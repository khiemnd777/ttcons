package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	maxPipelineTemplateLayoutUpdates = 500
	maxPipelineDiagramCoordinate     = 1_000_000
)

var pipelineDiagramPortSides = map[string]bool{
	"top":    true,
	"right":  true,
	"bottom": true,
	"left":   true,
}

func (s *Service) UpdateTemplateLayout(
	ctx context.Context,
	templateID string,
	req PipelineTemplateLayoutRequest,
	actorID string,
	actorName string,
) (Record, error) {
	templateID = strings.TrimSpace(templateID)
	if _, err := uuid.Parse(templateID); err != nil {
		return nil, validationError("ID mẫu quy trình không hợp lệ", Record{"template_id": templateID})
	}
	normalized, err := normalizePipelineTemplateLayoutRequest(req)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateTemplateLayout(ctx, templateID, normalized, actorID, actorName)
}

func normalizePipelineTemplateLayoutRequest(req PipelineTemplateLayoutRequest) (PipelineTemplateLayoutRequest, error) {
	if len(req.Nodes)+len(req.Edges) == 0 {
		return PipelineTemplateLayoutRequest{}, validationError("Cần ít nhất một thay đổi bố cục workflow")
	}
	if len(req.Nodes)+len(req.Edges) > maxPipelineTemplateLayoutUpdates {
		return PipelineTemplateLayoutRequest{}, validationError("Một lần chỉ được cập nhật tối đa 500 phần tử bố cục")
	}

	normalized := PipelineTemplateLayoutRequest{
		Nodes: make([]PipelineTemplateNodeLayoutRequest, 0, len(req.Nodes)),
		Edges: make([]PipelineTemplateEdgeLayoutRequest, 0, len(req.Edges)),
	}
	nodeIDs := map[string]bool{}
	for _, node := range req.Nodes {
		node.ID = strings.TrimSpace(node.ID)
		if _, err := uuid.Parse(node.ID); err != nil {
			return PipelineTemplateLayoutRequest{}, validationError("ID node trong bố cục không hợp lệ", Record{"node_id": node.ID})
		}
		if nodeIDs[node.ID] {
			return PipelineTemplateLayoutRequest{}, validationError("Bố cục chứa node trùng lặp", Record{"node_id": node.ID})
		}
		if !validPipelineDiagramCoordinate(node.PositionX) || !validPipelineDiagramCoordinate(node.PositionY) {
			return PipelineTemplateLayoutRequest{}, validationError("Tọa độ node vượt phạm vi cho phép", Record{"node_id": node.ID})
		}
		nodeIDs[node.ID] = true
		normalized.Nodes = append(normalized.Nodes, node)
	}

	edgeIDs := map[string]bool{}
	for _, edge := range req.Edges {
		edge.ID = strings.TrimSpace(edge.ID)
		edge.SourceSide = strings.ToLower(strings.TrimSpace(edge.SourceSide))
		edge.TargetSide = strings.ToLower(strings.TrimSpace(edge.TargetSide))
		if _, err := uuid.Parse(edge.ID); err != nil {
			return PipelineTemplateLayoutRequest{}, validationError("ID connector trong bố cục không hợp lệ", Record{"edge_id": edge.ID})
		}
		if edgeIDs[edge.ID] {
			return PipelineTemplateLayoutRequest{}, validationError("Bố cục chứa connector trùng lặp", Record{"edge_id": edge.ID})
		}
		if !pipelineDiagramPortSides[edge.SourceSide] || !pipelineDiagramPortSides[edge.TargetSide] {
			return PipelineTemplateLayoutRequest{}, validationError("Cạnh bám connector không hợp lệ", Record{
				"edge_id":     edge.ID,
				"source_side": edge.SourceSide,
				"target_side": edge.TargetSide,
			})
		}
		edgeIDs[edge.ID] = true
		normalized.Edges = append(normalized.Edges, edge)
	}

	return normalized, nil
}

func validPipelineDiagramCoordinate(value int) bool {
	return value >= -maxPipelineDiagramCoordinate && value <= maxPipelineDiagramCoordinate
}

func (r *Repository) UpdateTemplateLayout(
	ctx context.Context,
	templateID string,
	req PipelineTemplateLayoutRequest,
	actorID string,
	actorName string,
) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var templateName string
	var templateStatus string
	if err := tx.QueryRowContext(ctx, `
		SELECT name, status
		FROM pipeline_templates
		WHERE id=$1
		FOR UPDATE
	`, templateID).Scan(&templateName, &templateStatus); err != nil {
		return nil, err
	}
	templateStatus = strings.TrimSpace(templateStatus)
	if templateStatus != "draft" && templateStatus != "published" {
		return nil, validationError("Chỉ revision nháp hoặc đã phát hành mới được cập nhật bố cục", Record{
			"template_id": templateID,
			"status":      templateStatus,
		})
	}

	nodeIDs := make([]string, 0, len(req.Nodes))
	for _, node := range req.Nodes {
		var updatedID string
		err := tx.QueryRowContext(ctx, `
			UPDATE pipeline_template_nodes
			SET position_x=$3, position_y=$4, updated_at=now()
			WHERE template_id=$1 AND id=$2
			RETURNING id
		`, templateID, node.ID, node.PositionX, node.PositionY).Scan(&updatedID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, validationError("Node không thuộc mẫu quy trình đang cập nhật", Record{"node_id": node.ID, "template_id": templateID})
			}
			return nil, err
		}
		nodeIDs = append(nodeIDs, updatedID)
	}

	edgeIDs := make([]string, 0, len(req.Edges))
	for _, edge := range req.Edges {
		var updatedID string
		err := tx.QueryRowContext(ctx, `
			UPDATE pipeline_template_edges
			SET condition_config=jsonb_set(
				CASE
					WHEN jsonb_typeof(condition_config) = 'object' THEN condition_config
					ELSE '{}'::jsonb
				END,
				'{diagram}',
				jsonb_build_object(
					'version', 1,
					'portMode', 'manual',
					'source', jsonb_build_object('nodeId', from_node_id::text, 'side', $3::text),
					'target', jsonb_build_object('nodeId', to_node_id::text, 'side', $4::text)
				),
				true
			)
			WHERE template_id=$1 AND id=$2
			RETURNING id
		`, templateID, edge.ID, edge.SourceSide, edge.TargetSide).Scan(&updatedID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, validationError("Connector không thuộc mẫu quy trình đang cập nhật", Record{"edge_id": edge.ID, "template_id": templateID})
			}
			return nil, err
		}
		edgeIDs = append(edgeIDs, updatedID)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE pipeline_templates SET updated_at=now() WHERE id=$1`, templateID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "pipeline_template_layout_updated", "pipeline_template", templateID, fmt.Sprintf("Cập nhật bố cục mẫu quy trình %s", templateName), Record{
		"template_status": templateStatus,
		"node_ids":        nodeIDs,
		"edge_ids":        edgeIDs,
		"node_count":      len(nodeIDs),
		"edge_count":      len(edgeIDs),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetTemplateDetail(ctx, templateID)
}
