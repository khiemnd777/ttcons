package core

import (
	"context"
	"database/sql"
	"fmt"
)

type PipelineFormPublishExpectation struct {
	VersionID      string `json:"version_id"`
	DefinitionHash string `json:"definition_hash"`
	VersionStatus  string `json:"version_status"`
}

type PipelineFormPublishRequest struct {
	Versions []PipelineFormPublishExpectation `json:"versions"`
}

const pipelineFormPublishPreviewSQL = `
	SELECT v.id::text AS version_id, v.definition_hash, v.status AS version_status,
	       v.version_no, t.name AS template_name, t.status AS template_status,
	       COUNT(DISTINCT s.id)::int AS step_usage_count,
	       (SELECT COUNT(*)::int FROM workflow_form_fields f WHERE f.form_version_id=v.id) AS field_count
	FROM pipeline_template_steps s
	JOIN pipeline_template_nodes n ON n.id=s.node_id
	JOIN workflow_form_versions v ON v.id=s.form_version_id
	JOIN workflow_form_templates t ON t.id=v.template_id
	WHERE n.template_id=$1 AND s.form_version_id IS NOT NULL
	GROUP BY v.id, t.id
	ORDER BY v.id
`

func (r *Repository) PipelineFormPublishPreview(ctx context.Context, templateID string) (Record, error) {
	template, err := r.GetByID(ctx, "pipeline_templates", templateID)
	if err != nil {
		return nil, err
	}
	forms, err := r.Query(ctx, pipelineFormPublishPreviewSQL, templateID)
	if err != nil {
		return nil, err
	}
	return pipelineFormPublishPreviewRecord(template, forms), nil
}

func pipelineFormPublishPreviewRecord(template Record, forms []Record) Record {
	if forms == nil {
		forms = []Record{}
	}
	publishable, stepOccurrences := 0, 0
	for _, form := range forms {
		if form["version_status"] == "draft" && form["template_status"] == "active" && number(form["field_count"]) > 0 {
			publishable++
		}
		stepOccurrences += int(number(form["step_usage_count"]))
	}
	return Record{
		"template_id": template["id"], "template_name": template["name"],
		"template_status": template["status"], "forms": forms,
		"publishable_count": publishable, "step_occurrences": stepOccurrences,
	}
}

func (r *Repository) PublishPipelineForms(ctx context.Context, templateID string, request PipelineFormPublishRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	templates, err := queryTx(ctx, tx, `SELECT * FROM pipeline_templates WHERE id=$1 FOR UPDATE`, templateID)
	if err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		return nil, sql.ErrNoRows
	}
	if templates[0]["status"] != "draft" {
		return nil, validationError("Chỉ Pipeline nháp mới được phát hành biểu mẫu liên quan")
	}
	// Lock the graph so its existing nodes and steps cannot change during publication.
	if _, err := queryTx(ctx, tx, `SELECT id FROM pipeline_template_nodes WHERE template_id=$1 ORDER BY id FOR UPDATE`, templateID); err != nil {
		return nil, err
	}
	if _, err := queryTx(ctx, tx, `SELECT s.id FROM pipeline_template_steps s JOIN pipeline_template_nodes n ON n.id=s.node_id WHERE n.template_id=$1 ORDER BY s.id FOR UPDATE OF s`, templateID); err != nil {
		return nil, err
	}
	forms, err := queryTx(ctx, tx, pipelineFormPublishPreviewSQL, templateID)
	if err != nil {
		return nil, err
	}
	if err := validatePipelineFormPublishExpectation(forms, request.Versions); err != nil {
		return nil, err
	}

	publishedCount := 0
	for _, form := range forms {
		if form["template_status"] != "active" {
			return nil, validationError("Có biểu mẫu không còn active; hãy kiểm tra lại danh sách", Record{"version_id": form["version_id"]})
		}
		status := fmt.Sprint(form["version_status"])
		if status == "published" {
			continue
		}
		if status != "draft" || number(form["field_count"]) == 0 {
			return nil, validationError("Có biểu mẫu chưa đủ điều kiện phát hành", Record{"version_id": form["version_id"]})
		}
		versionID := fmt.Sprint(form["version_id"])
		// This row lock serializes edits and checks the actual definition, not only its stored hash.
		var lockedStatus, templateStatus string
		if err := tx.QueryRowContext(ctx, `SELECT v.status,t.status FROM workflow_form_versions v JOIN workflow_form_templates t ON t.id=v.template_id WHERE v.id=$1 FOR UPDATE OF v,t`, versionID).Scan(&lockedStatus, &templateStatus); err != nil {
			return nil, err
		}
		if lockedStatus != "draft" || templateStatus != "active" {
			return nil, validationError("Trạng thái biểu mẫu đã thay đổi; hãy xem trước lại", Record{"version_id": versionID})
		}
		hash, err := workflowFormDefinitionHashTx(ctx, tx, versionID)
		if err != nil {
			return nil, err
		}
		if hash != fmt.Sprint(form["definition_hash"]) {
			return nil, validationError("Nội dung biểu mẫu đã thay đổi; hãy xem trước lại", Record{"version_id": versionID})
		}
		if err := publishWorkflowFormVersionTx(ctx, tx, versionID, actorID, actorName); err != nil {
			return nil, err
		}
		publishedCount++
	}
	if publishedCount == 0 {
		return nil, validationError("Pipeline không có phiên bản biểu mẫu nháp để phát hành")
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return Record{"published_count": publishedCount, "template_id": templateID}, nil
}

func validatePipelineFormPublishExpectation(forms []Record, expected []PipelineFormPublishExpectation) error {
	if len(forms) != len(expected) {
		return validationError("Danh sách biểu mẫu đã thay đổi; hãy xem trước lại")
	}
	actual := make(map[string]Record, len(forms))
	for _, form := range forms {
		actual[fmt.Sprint(form["version_id"])] = form
	}
	seen := make(map[string]bool, len(expected))
	for _, item := range expected {
		id := cleanID(item.VersionID)
		form, ok := actual[id]
		if id == "" || !ok || seen[id] || item.VersionStatus != fmt.Sprint(form["version_status"]) || item.DefinitionHash != fmt.Sprint(form["definition_hash"]) {
			return validationError("Danh sách biểu mẫu đã thay đổi; hãy xem trước lại")
		}
		seen[id] = true
	}
	return nil
}
