package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type ApprovalPolicyStage struct {
	Roles  []string `json:"roles"`
	Quorum int      `json:"quorum"`
}

type ApprovalPolicy struct {
	Mode                   string                `json:"mode"`
	Roles                  []string              `json:"roles"`
	Quorum                 int                   `json:"quorum"`
	Stages                 []ApprovalPolicyStage `json:"stages,omitempty"`
	RequesterCannotApprove bool                  `json:"requester_cannot_approve"`
	CommentRequired        bool                  `json:"comment_required"`
	SignatureRequired      bool                  `json:"signature_required"`
}

func normalizeApprovalPolicyValue(value any) (Record, error) {
	if value == nil {
		return Record{}, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, validationError("Approval policy không hợp lệ")
	}
	policy := ApprovalPolicy{}
	if err := json.Unmarshal(raw, &policy); err != nil {
		return nil, validationError("Approval policy không hợp lệ")
	}
	policy.Mode = strings.TrimSpace(policy.Mode)
	if policy.Mode == "" || policy.Mode == "none" {
		return Record{"mode": "none"}, nil
	}
	if !statusIn(policy.Mode, "any", "all", "quorum", "ordered") {
		return nil, validationError("Chế độ approval policy không hợp lệ", Record{"mode": policy.Mode})
	}
	policy.Roles = normalizedRoleList(policy.Roles)
	if policy.Mode == "ordered" && len(policy.Stages) == 0 {
		for _, role := range policy.Roles {
			policy.Stages = append(policy.Stages, ApprovalPolicyStage{Roles: []string{role}, Quorum: 1})
		}
	}
	if policy.Mode == "ordered" {
		if len(policy.Stages) == 0 {
			return nil, validationError("Approval policy ordered cần ít nhất một stage")
		}
		allRoles := []string{}
		for index := range policy.Stages {
			policy.Stages[index].Roles = normalizedRoleList(policy.Stages[index].Roles)
			if len(policy.Stages[index].Roles) == 0 {
				return nil, validationError("Approval stage cần ít nhất một role", Record{"stage": index})
			}
			if policy.Stages[index].Quorum <= 0 {
				policy.Stages[index].Quorum = 1
			}
			if policy.Stages[index].Quorum > len(policy.Stages[index].Roles) {
				return nil, validationError("Quorum của approval stage vượt số role", Record{"stage": index})
			}
			allRoles = append(allRoles, policy.Stages[index].Roles...)
		}
		policy.Roles = normalizedRoleList(allRoles)
	} else {
		if len(policy.Roles) == 0 {
			return nil, validationError("Approval policy cần ít nhất một role")
		}
		if policy.Quorum <= 0 {
			policy.Quorum = 1
		}
		if policy.Mode == "all" {
			policy.Quorum = len(policy.Roles)
		}
		if policy.Quorum > len(policy.Roles) {
			return nil, validationError("Quorum vượt số role trong approval policy")
		}
	}
	normalizedRaw, _ := json.Marshal(policy)
	out := Record{}
	_ = json.Unmarshal(normalizedRaw, &out)
	return out, nil
}

func approvalPolicyFromValue(value any) ApprovalPolicy {
	normalized, err := normalizeApprovalPolicyValue(value)
	if err != nil {
		return ApprovalPolicy{}
	}
	raw, _ := json.Marshal(normalized)
	policy := ApprovalPolicy{}
	_ = json.Unmarshal(raw, &policy)
	return policy
}

func (p ApprovalPolicy) enabled() bool {
	return statusIn(p.Mode, "any", "all", "quorum", "ordered")
}

func (p ApprovalPolicy) rolesForStage(stage int) []string {
	if p.Mode == "ordered" && stage >= 0 && stage < len(p.Stages) {
		return p.Stages[stage].Roles
	}
	return p.Roles
}

func (p ApprovalPolicy) quorumForStage(stage int) int {
	if p.Mode == "ordered" && stage >= 0 && stage < len(p.Stages) {
		return p.Stages[stage].Quorum
	}
	if p.Mode == "all" {
		return len(p.Roles)
	}
	if p.Quorum > 0 {
		return p.Quorum
	}
	return 1
}

func matchingApprovalRole(actorLabels, allowedRoles []string) string {
	for _, role := range allowedRoles {
		if roleLabelsIntersect(actorLabels, []string{role}) {
			return role
		}
	}
	return ""
}

func (r *Repository) approvalActorLabels(ctx context.Context, actorID, projectID string) ([]string, error) {
	profile, err := r.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	labels := append([]string{}, profile.RoleLabels...)
	if projectID != "" {
		projectLabels, err := r.UserProjectRoleLabels(ctx, actorID, projectID)
		if err != nil {
			return nil, err
		}
		labels = append(labels, projectLabels...)
	}
	return normalizedRoleList(labels), nil
}

func (r *Repository) ApplyApprovalPolicyAction(ctx context.Context, request Record, action, comment, actorID, actorName string) (Record, error) {
	policy := approvalPolicyFromValue(request["policy_json"])
	if !policy.enabled() {
		return r.ApprovalAction(ctx, cleanID(request["id"]), action, comment, actorID, actorName)
	}
	if fmt.Sprint(request["status"]) != "pending" {
		return nil, validationError("Yêu cầu phê duyệt không còn ở trạng thái chờ", Record{"status": request["status"]})
	}
	comment = strings.TrimSpace(comment)
	if policy.CommentRequired && comment == "" {
		return nil, validationError("Ý kiến phê duyệt là bắt buộc")
	}
	if policy.RequesterCannotApprove && cleanID(request["requested_by"]) == actorID {
		return nil, forbiddenError("Người gửi yêu cầu không được tự phê duyệt")
	}
	projectID := cleanID(request["project_id"])
	actorLabels, err := r.approvalActorLabels(ctx, actorID, projectID)
	if err != nil {
		return nil, err
	}
	currentStage := int(number(request["current_stage"]))
	allowedRoles := policy.rolesForStage(currentStage)
	roleLabel := matchingApprovalRole(actorLabels, allowedRoles)
	if roleLabel == "" {
		return nil, forbiddenError("Vai trò hiện tại không thuộc stage phê duyệt", Record{"required_roles": allowedRoles, "current_stage": currentStage})
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	locked, err := queryTx(ctx, tx, `SELECT * FROM approval_requests WHERE id=$1 FOR UPDATE`, request["id"])
	if err != nil {
		return nil, err
	}
	if len(locked) == 0 {
		return nil, sql.ErrNoRows
	}
	if fmt.Sprint(locked[0]["status"]) != "pending" || int(number(locked[0]["current_stage"])) != currentStage {
		return nil, validationError("Stage phê duyệt đã thay đổi, vui lòng tải lại")
	}
	duplicate, err := queryTx(ctx, tx, `
		SELECT id FROM approval_actions
		WHERE approval_request_id=$1 AND user_id=NULLIF($2,'')::uuid AND action='approve'
		LIMIT 1
	`, request["id"], actorID)
	if err != nil {
		return nil, err
	}
	if action == "approve" && len(duplicate) > 0 {
		return nil, validationError("Người dùng đã phê duyệt yêu cầu này")
	}
	var approvalActionID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO approval_actions (approval_request_id, user_id, action, comment, role_label)
		VALUES ($1, NULLIF($2,'')::uuid, $3, NULLIF($4,''), $5)
		RETURNING id::text
	`, request["id"], actorID, action, comment, roleLabel).Scan(&approvalActionID); err != nil {
		return nil, err
	}

	status := "pending"
	nextStage := currentStage
	if action == "reject" {
		status = "rejected"
	} else if action == "request-change" {
		status = "changes_requested"
	} else {
		actions, err := queryTx(ctx, tx, `
			SELECT DISTINCT ON (user_id) user_id, role_label
			FROM approval_actions
			WHERE approval_request_id=$1 AND action='approve'
			ORDER BY user_id, created_at
		`, request["id"])
		if err != nil {
			return nil, err
		}
		stageApprovers := map[string]bool{}
		approvedRoles := map[string]bool{}
		for _, approval := range actions {
			approvalRole := strings.TrimSpace(fmt.Sprint(approval["role_label"]))
			if matchingApprovalRole([]string{approvalRole}, allowedRoles) != "" {
				stageApprovers[cleanID(approval["user_id"])] = true
			}
			approvedRoles[normalizeRoleLabel(approvalRole)] = true
		}
		satisfied := len(stageApprovers) >= policy.quorumForStage(currentStage)
		if policy.Mode == "all" {
			satisfied = true
			for _, role := range policy.Roles {
				if !approvedRoles[normalizeRoleLabel(role)] {
					satisfied = false
					break
				}
			}
		}
		if satisfied {
			if policy.Mode == "ordered" && currentStage+1 < len(policy.Stages) {
				nextStage = currentStage + 1
			} else {
				status = "approved"
			}
		}
	}
	resolved := status != "pending"
	if _, err := tx.ExecContext(ctx, `
		UPDATE approval_requests
		SET status=$2, current_stage=$3, resolved_at=CASE WHEN $4 THEN now() ELSE NULL END
		WHERE id=$1
	`, request["id"], status, nextStage, resolved); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "approval_"+strings.ReplaceAll(action, "-", "_"), fmt.Sprint(request["target_type"]), cleanID(request["target_id"]), fmt.Sprintf("%s phê duyệt stage %d", action, currentStage+1), Record{"approval_request_id": request["id"], "action": action, "role_label": roleLabel, "stage": currentStage, "next_stage": nextStage, "status": status}); err != nil {
		return nil, err
	}
	if err := emitPipelineNodeApprovalActionTx(ctx, tx, locked[0], approvalActionID, action, comment, actorID, actorName, currentStage, nextStage, status); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, "approval_requests", cleanID(request["id"]))
}
