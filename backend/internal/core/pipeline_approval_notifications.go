package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func emitPipelineNodeApprovalRequestedTx(ctx context.Context, tx *sql.Tx, request, node Record, actorID, actorName, sourceEventID string, stage int, resolvePriorStage bool) error {
	approvalID := cleanID(request["id"])
	nodeID := cleanID(node["id"])
	projectID := cleanID(request["project_id"])
	if approvalID == "" || nodeID == "" || projectID == "" {
		return fmt.Errorf("pipeline node approval notification context is incomplete")
	}
	if sourceEventID = cleanID(sourceEventID); sourceEventID == "" {
		sourceEventID = approvalID
	}
	audience := Record{
		"approval_request_id": approvalID,
		"expected_stage":      stage,
		"exclude_actor":       true,
	}
	if resolvePriorStage {
		audience["resolve_thread_for_all"] = true
	}
	_, err := emitNotificationEventTx(ctx, tx, NotificationDomainEvent{
		Key:            "workflow.node_approval_requested",
		AggregateType:  "pipeline_run_node",
		AggregateID:    nodeID,
		SourceEventID:  sourceEventID,
		ProjectID:      projectID,
		ActorUserID:    actorID,
		CorrelationKey: "pipeline_node_approval:" + approvalID,
		IdempotencyKey: fmt.Sprintf("pipeline_node_approval:%s:stage:%d:requested", approvalID, stage),
		Payload: Record{
			"actor_name":          actorName,
			"node_name":           fmt.Sprint(node["name"]),
			"node_id":             nodeID,
			"project_id":          projectID,
			"approval_request_id": approvalID,
		},
		Audience: audience,
	})
	return err
}

func emitPipelineNodeApprovalActionTx(
	ctx context.Context,
	tx *sql.Tx,
	request Record,
	actionID, action, comment, actorID, actorName string,
	currentStage, nextStage int,
	status string,
) error {
	if fmt.Sprint(request["target_type"]) != "pipeline_run_node" {
		return nil
	}
	approvalID := cleanID(request["id"])
	nodeID := cleanID(request["target_id"])
	projectID := cleanID(request["project_id"])
	rows, err := queryTx(ctx, tx, `SELECT * FROM pipeline_run_nodes WHERE id=$1::uuid AND project_id=$2::uuid`, nodeID, projectID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("pipeline run node %s is not visible in approval project scope", nodeID)
	}
	node := rows[0]
	if status == "pending" && nextStage > currentStage {
		return emitPipelineNodeApprovalRequestedTx(ctx, tx, request, node, actorID, actorName, actionID, nextStage, true)
	}

	eventKey := "workflow.node_approval_progressed"
	if status == "approved" {
		eventKey = "workflow.node_approval_approved"
	} else if status == "rejected" {
		eventKey = "workflow.node_approval_rejected"
	} else if status == "changes_requested" {
		eventKey = "workflow.node_approval_changes_requested"
	}
	requestedBy := cleanID(request["requested_by"])
	audience := Record{
		"user_ids":         []string{requestedBy},
		"resolve_user_ids": []string{actorID},
		"exclude_actor":    true,
	}
	if status == "pending" {
		audience["approval_request_id"] = approvalID
		audience["expected_stage"] = currentStage
		audience["require_pending_approval"] = true
	} else {
		audience["resolve_thread_for_all"] = true
	}
	_, err = emitNotificationEventTx(ctx, tx, NotificationDomainEvent{
		Key:            eventKey,
		AggregateType:  "pipeline_run_node",
		AggregateID:    nodeID,
		SourceEventID:  actionID,
		ProjectID:      projectID,
		ActorUserID:    actorID,
		CorrelationKey: "pipeline_node_approval:" + approvalID,
		IdempotencyKey: "pipeline_node_approval_action:" + actionID + ":notification",
		Payload: Record{
			"actor_name":          actorName,
			"node_name":           fmt.Sprint(node["name"]),
			"node_id":             nodeID,
			"project_id":          projectID,
			"approval_request_id": approvalID,
			"comment":             strings.TrimSpace(comment),
			"action":              action,
		},
		Audience: audience,
	})
	return err
}
