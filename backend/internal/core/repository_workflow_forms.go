package core

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func (r *Repository) GetWorkflowFormInstanceDetail(ctx context.Context, instanceID string) (Record, error) {
	return getWorkflowFormInstanceDetail(ctx, r.db, instanceID)
}

func getWorkflowFormInstanceDetail(ctx context.Context, db queryContext, instanceID string) (Record, error) {
	instance, err := queryOneRecord(ctx, db, `
		SELECT
			wfi.*,
			wft.code AS form_code,
			wft.name AS form_name,
			wft.description AS form_description,
			wfv.version_no AS form_version_no,
			wfv.status AS form_version_status,
			COALESCE(wfv.schema_json->>'variant','') AS form_version_variant,
			wi.title AS work_item_title,
			wi.status AS work_item_status
		FROM workflow_form_instances wfi
		JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
		JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		LEFT JOIN work_items wi ON wi.id=wfi.work_item_id AND wi.project_id=wfi.project_id
		WHERE wfi.id=$1
	`, instanceID)
	if err != nil {
		return nil, err
	}
	fields, err := queryRecords(ctx, db, `
		SELECT id, form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index, visibility_condition_json
		FROM workflow_form_fields
		WHERE form_version_id=$1
		ORDER BY order_index, created_at, id
	`, instance["form_version_id"])
	if err != nil {
		return nil, err
	}
	fields = applyOperationalWorkflowFormFieldPresentation(
		fmt.Sprint(instance["form_version_variant"]),
		fmt.Sprint(instance["form_code"]),
		fields,
	)
	delete(instance, "form_version_variant")
	answerRows, err := queryRecords(ctx, db, `
		SELECT field_key, value_json, revision_no, updated_by, updated_at
		FROM workflow_form_answers
		WHERE instance_id=$1
		ORDER BY field_key
	`, instanceID)
	if err != nil {
		return nil, err
	}
	answers := Record{}
	for _, row := range answerRows {
		fieldKey := fmt.Sprint(row["field_key"])
		value, decodeErr := decodeWorkflowFormAnswerValue(row["value_json"])
		if decodeErr != nil {
			return nil, fmt.Errorf("decode workflow form answer %s: %w", fieldKey, decodeErr)
		}
		answers[fieldKey] = value
	}
	events, err := queryRecords(ctx, db, `
		SELECT wfe.*, u.full_name AS actor_name, COALESCE(u.deleted_at IS NOT NULL, false) AS actor_account_deleted
		FROM workflow_form_events wfe
		LEFT JOIN users u ON u.id=wfe.actor_id
		WHERE wfe.instance_id=$1
		ORDER BY wfe.created_at DESC
		LIMIT 30
	`, instanceID)
	if err != nil {
		return nil, err
	}
	signatures, err := queryRecords(ctx, db, `
		SELECT signature.*, signature.form_revision_no=$2 AS is_current_revision
		FROM workflow_form_signatures signature
		WHERE signature.form_instance_id=$1
		ORDER BY signature.form_revision_no DESC, signature.signed_at DESC, signature.created_at DESC, signature.id
	`, instanceID, instance["current_revision"])
	if err != nil {
		return nil, err
	}
	sealApplications, err := queryRecords(ctx, db, `
		SELECT
			application.*,
			asset.name AS seal_asset_name,
			asset.status AS seal_asset_status,
			asset.storage_provider,
			asset.storage_key,
			asset.mime_type,
			asset.file_size,
			asset.width_px,
			asset.height_px,
			asset.checksum_sha256,
			application.form_revision_no=$2 AS is_current_revision
		FROM workflow_form_seal_applications application
		JOIN seal_assets asset ON asset.id=application.seal_asset_id
		WHERE application.form_instance_id=$1
		ORDER BY application.applied_at DESC, application.created_at DESC, application.id
	`, instanceID, instance["current_revision"])
	if err != nil {
		return nil, err
	}
	pdfConfirmationPreferences, err := queryRecords(ctx, db, `
		SELECT *
		FROM workflow_form_pdf_confirmation_preferences
		WHERE form_instance_id=$1
		ORDER BY purpose, created_at, id
	`, instanceID)
	if err != nil {
		return nil, err
	}
	pdfExports, err := queryRecords(ctx, db, `
		SELECT
			wfpe.*,
			wfpe.form_revision_no=$2 AS is_current_revision,
			COALESCE(deletion.status, '') AS deletion_status,
			(
				SELECT COUNT(*)
				FROM acceptance_records acceptance
				WHERE acceptance.pdf_export_id=wfpe.id
			) AS acceptance_reference_count,
			wfpe.form_revision_no=$2 AND NOT EXISTS (
				SELECT 1
				FROM acceptance_records acceptance
				WHERE acceptance.pdf_export_id=wfpe.id
			) AS can_delete
		FROM workflow_form_pdf_exports wfpe
		LEFT JOIN workflow_form_pdf_export_deletions deletion ON deletion.pdf_export_id=wfpe.id
		WHERE wfpe.form_instance_id=$1
		  AND COALESCE(deletion.status, '') <> 'deleted'
		ORDER BY wfpe.generated_at DESC, wfpe.created_at DESC
	`, instanceID, instance["current_revision"])
	if err != nil {
		return nil, err
	}
	return Record{
		"instance": instance,
		"template": Record{
			"id":          instance["form_template_id"],
			"code":        instance["form_code"],
			"name":        instance["form_name"],
			"description": instance["form_description"],
			"form_type":   instance["form_type"],
		},
		"version": Record{
			"id":         instance["form_version_id"],
			"version_no": instance["form_version_no"],
			"status":     instance["form_version_status"],
		},
		"fields":                       fields,
		"answers":                      answers,
		"events":                       events,
		"signatures":                   signatures,
		"seal_applications":            sealApplications,
		"pdf_confirmation_preferences": pdfConfirmationPreferences,
		"pdf_exports":                  pdfExports,
	}, nil
}

func decodeWorkflowFormAnswerValue(value any) (any, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case string:
		var decoded any
		if err := json.Unmarshal([]byte(typed), &decoded); err != nil {
			return nil, err
		}
		return decoded, nil
	case []byte:
		var decoded any
		if err := json.Unmarshal(typed, &decoded); err != nil {
			return nil, err
		}
		return decoded, nil
	default:
		return typed, nil
	}
}

func (r *Repository) HasSubmittedWorkflowFormForWorkItem(ctx context.Context, workItemID string) (bool, error) {
	rows, err := r.Query(ctx, `
		SELECT COUNT(*) AS total
		FROM workflow_form_instances
		WHERE work_item_id=$1 AND status='submitted'
	`, workItemID)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && number(rows[0]["total"]) > 0, nil
}

func (r *Repository) WorkflowFormCatalog(ctx context.Context) ([]Record, error) {
	return r.Query(ctx, `
		WITH latest_published AS (
			SELECT DISTINCT ON (template_id)
				id,
				template_id
			FROM workflow_form_versions
			WHERE status='published'
			  AND COALESCE(schema_json->>'variant','')<>'operational_capture_v3'
			ORDER BY template_id, version_no DESC, published_at DESC NULLS LAST, created_at DESC
		),
		step_usage AS (
			SELECT form_version_id, COUNT(*)::int AS step_usage_count
			FROM pipeline_template_steps
			WHERE form_version_id IS NOT NULL
			GROUP BY form_version_id
		)
		SELECT
			t.id AS template_id,
			t.code AS template_code,
			t.name AS template_name,
			t.description AS template_description,
			t.form_type,
			t.status AS template_status,
			v.id AS version_id,
			v.version_no,
			v.status AS version_status,
			v.published_at,
			(v.id=lp.id) AS latest_published,
			COALESCE(su.step_usage_count, 0) AS step_usage_count,
			COUNT(f.id)::int AS field_count,
			COALESCE(
				jsonb_agg(f.field_key ORDER BY f.order_index, f.created_at, f.id) FILTER (WHERE f.id IS NOT NULL AND f.required),
				'[]'::jsonb
			) AS required_field_keys,
			COALESCE(
				jsonb_agg(
					jsonb_build_object(
						'field_key', f.field_key,
						'label', f.label,
						'field_type', f.field_type,
						'required', f.required,
						'options_json', f.options_json,
						'validation_json', f.validation_json,
						'visibility_condition_json', f.visibility_condition_json,
						'order_index', f.order_index
					)
					ORDER BY f.order_index, f.created_at, f.id
				) FILTER (WHERE f.id IS NOT NULL),
				'[]'::jsonb
			) AS fields
		FROM workflow_form_templates t
		JOIN workflow_form_versions v ON v.template_id=t.id AND v.status='published'
		  AND COALESCE(v.schema_json->>'variant','')<>'operational_capture_v3'
		JOIN latest_published lp ON lp.template_id=t.id
		LEFT JOIN workflow_form_fields f ON f.form_version_id=v.id
		LEFT JOIN step_usage su ON su.form_version_id=v.id
		WHERE t.status='active'
		GROUP BY t.id, t.code, t.name, t.description, t.form_type, t.status, v.id, v.version_no, v.status, v.published_at, lp.id, su.step_usage_count
		ORDER BY t.form_type, t.name, v.version_no DESC
	`)
}

func (r *Repository) WorkflowFormTemplateLibrary(ctx context.Context) ([]Record, error) {
	return r.Query(ctx, `
		WITH latest_published AS (
			SELECT DISTINCT ON (template_id)
				id,
				template_id,
				version_no,
				status,
				published_at
			FROM workflow_form_versions
			WHERE status='published'
			ORDER BY template_id, version_no DESC, published_at DESC NULLS LAST, created_at DESC
		),
		version_counts AS (
			SELECT
				template_id,
				COUNT(*)::int AS version_count,
				COUNT(*) FILTER (WHERE status='published')::int AS published_version_count,
				COUNT(*) FILTER (WHERE status='draft')::int AS draft_version_count
			FROM workflow_form_versions
			GROUP BY template_id
		),
		field_counts AS (
			SELECT v.template_id, COUNT(f.id)::int AS field_count
			FROM workflow_form_versions v
			LEFT JOIN workflow_form_fields f ON f.form_version_id=v.id
			GROUP BY v.template_id
		),
		step_usage AS (
			SELECT v.template_id, COUNT(pts.id)::int AS step_usage_count
			FROM workflow_form_versions v
			LEFT JOIN pipeline_template_steps pts ON pts.form_version_id=v.id
			GROUP BY v.template_id
		),
		instance_usage AS (
			SELECT form_template_id AS template_id, COUNT(*)::int AS instance_count
			FROM workflow_form_instances
			WHERE status <> 'cancelled'
			GROUP BY form_template_id
		)
		SELECT
			t.id AS template_id,
			t.code AS template_code,
			t.name AS template_name,
			t.description AS template_description,
			t.form_type,
			t.status AS template_status,
			t.created_at,
			t.updated_at,
			lp.id AS latest_version_id,
			lp.version_no AS latest_version_no,
			lp.status AS latest_version_status,
			lp.published_at AS latest_published_at,
			COALESCE(vc.version_count, 0) AS version_count,
			COALESCE(vc.published_version_count, 0) AS published_version_count,
			COALESCE(vc.draft_version_count, 0) AS draft_version_count,
			COALESCE(fc.field_count, 0) AS field_count,
			COALESCE(su.step_usage_count, 0) AS step_usage_count,
			COALESCE(iu.instance_count, 0) AS instance_count
		FROM workflow_form_templates t
		LEFT JOIN latest_published lp ON lp.template_id=t.id
		LEFT JOIN version_counts vc ON vc.template_id=t.id
		LEFT JOIN field_counts fc ON fc.template_id=t.id
		LEFT JOIN step_usage su ON su.template_id=t.id
		LEFT JOIN instance_usage iu ON iu.template_id=t.id
		ORDER BY CASE WHEN t.status='active' THEN 0 ELSE 1 END, t.form_type, t.name
	`)
}

func (r *Repository) WorkflowFormTemplateDetail(ctx context.Context, templateID string) (Record, error) {
	template, err := r.QueryOne(ctx, `
		WITH step_usage AS (
			SELECT v.template_id, COUNT(pts.id)::int AS step_usage_count
			FROM workflow_form_versions v
			LEFT JOIN pipeline_template_steps pts ON pts.form_version_id=v.id
			GROUP BY v.template_id
		),
		instance_usage AS (
			SELECT form_template_id AS template_id, COUNT(*)::int AS instance_count
			FROM workflow_form_instances
			WHERE status <> 'cancelled'
			GROUP BY form_template_id
		)
		SELECT
			t.id AS template_id,
			t.code AS template_code,
			t.name AS template_name,
			t.description AS template_description,
			t.form_type,
			t.status AS template_status,
			t.created_by,
			t.created_at,
			t.updated_at,
			COALESCE(su.step_usage_count, 0) AS step_usage_count,
			COALESCE(iu.instance_count, 0) AS instance_count
		FROM workflow_form_templates t
		LEFT JOIN step_usage su ON su.template_id=t.id
		LEFT JOIN instance_usage iu ON iu.template_id=t.id
		WHERE t.id=$1
	`, templateID)
	if err != nil {
		return nil, err
	}
	versions, err := r.workflowFormVersions(ctx, templateID)
	if err != nil {
		return nil, err
	}
	template["versions"] = versions
	return template, nil
}

func (r *Repository) WorkflowFormVersionDetail(ctx context.Context, versionID string) (Record, error) {
	rows, err := r.Query(ctx, workflowFormVersionDetailSQL("WHERE v.id=$1"), versionID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	return rows[0], nil
}

func (r *Repository) workflowFormVersions(ctx context.Context, templateID string) ([]Record, error) {
	return r.Query(ctx, workflowFormVersionDetailSQL("WHERE v.template_id=$1"), templateID)
}

func workflowFormVersionDetailSQL(where string) string {
	return fmt.Sprintf(`
		WITH latest_published AS (
			SELECT DISTINCT ON (template_id)
				id,
				template_id
			FROM workflow_form_versions
			WHERE status='published'
			ORDER BY template_id, version_no DESC, published_at DESC NULLS LAST, created_at DESC
		),
		step_usage AS (
			SELECT form_version_id, COUNT(*)::int AS step_usage_count
			FROM pipeline_template_steps
			WHERE form_version_id IS NOT NULL
			GROUP BY form_version_id
		),
		instance_usage AS (
			SELECT form_version_id, COUNT(*)::int AS instance_count
			FROM workflow_form_instances
			WHERE status <> 'cancelled'
			GROUP BY form_version_id
		)
		SELECT
			v.id AS version_id,
			v.template_id,
			t.code AS template_code,
			t.name AS template_name,
			t.description AS template_description,
			t.form_type,
			t.status AS template_status,
			v.version_no,
			v.status AS version_status,
			v.schema_json,
			v.validation_json,
			COALESCE(v.definition_hash,'') AS definition_hash,
			v.published_at,
			v.created_by,
			v.created_at,
			v.updated_at,
			(v.id=lp.id) AS latest_published,
			COALESCE(su.step_usage_count, 0) AS step_usage_count,
			COALESCE(iu.instance_count, 0) AS instance_count,
			COUNT(f.id)::int AS field_count,
			COALESCE(
				jsonb_agg(
					jsonb_build_object(
						'id', f.id,
						'form_version_id', f.form_version_id,
						'field_key', f.field_key,
						'label', f.label,
						'field_type', f.field_type,
						'required', f.required,
						'options_json', f.options_json,
						'validation_json', f.validation_json,
						'help_text', f.help_text,
						'order_index', f.order_index,
						'visibility_condition_json', f.visibility_condition_json,
						'created_at', f.created_at
					)
					ORDER BY f.order_index, f.created_at, f.id
				) FILTER (WHERE f.id IS NOT NULL),
				'[]'::jsonb
			) AS fields
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		LEFT JOIN latest_published lp ON lp.template_id=v.template_id
		LEFT JOIN step_usage su ON su.form_version_id=v.id
		LEFT JOIN instance_usage iu ON iu.form_version_id=v.id
		LEFT JOIN workflow_form_fields f ON f.form_version_id=v.id
		%s
		GROUP BY v.id, t.id, t.code, t.name, t.form_type, t.status, lp.id, su.step_usage_count, iu.instance_count
		ORDER BY v.version_no DESC
	`, where)
}

func (r *Repository) CreateWorkflowFormTemplate(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	row, err := r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		rows, queryErr := queryTx(ctx, tx, `
			INSERT INTO workflow_form_templates (code, name, description, form_type, status, created_by)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6,'')::uuid)
			RETURNING id
		`, body["code"], body["name"], body["description"], body["form_type"], body["status"], actorID)
		if queryErr != nil {
			return nil, queryErr
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		return rows[0], nil
	}, func(row Record) auditEvent {
		return auditEvent{
			ActorID: actorID, ActorName: actorName, EventType: "workflow_form_template_created",
			TargetType: "workflow_form_template", Description: fmt.Sprintf("Tạo mẫu biểu mẫu quy trình %s", body["name"]),
			Metadata: Record{"code": body["code"], "form_type": body["form_type"]},
		}
	})
	if err != nil {
		return nil, err
	}
	templateID := cleanID(row["id"])
	return r.WorkflowFormTemplateDetail(ctx, templateID)
}

func (r *Repository) UpdateWorkflowFormTemplate(ctx context.Context, templateID string, body Record, actorID, actorName string) (Record, error) {
	if fmt.Sprint(body["status"]) == "archived" {
		usageRows, err := r.Query(ctx, `
			SELECT COUNT(*) AS total
			FROM pipeline_template_steps pts
			JOIN workflow_form_versions v ON v.id=pts.form_version_id
			WHERE v.template_id=$1
		`, templateID)
		if err != nil {
			return nil, err
		}
		if len(usageRows) > 0 && number(usageRows[0]["total"]) > 0 {
			return nil, validationError("Không thể lưu trữ mẫu biểu mẫu quy trình đang được bước quy trình sử dụng", Record{"step_usage_count": usageRows[0]["total"]})
		}
	}
	_, err := r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		rows, queryErr := queryTx(ctx, tx, `
			UPDATE workflow_form_templates
			SET name=$2,
				description=$3,
				status=$4,
				updated_at=now()
			WHERE id=$1
			RETURNING id, name
		`, templateID, body["name"], body["description"], body["status"])
		if queryErr != nil {
			return nil, queryErr
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		return rows[0], nil
	}, func(row Record) auditEvent {
		return auditEvent{
			ActorID: actorID, ActorName: actorName, EventType: "workflow_form_template_updated",
			TargetType: "workflow_form_template", TargetID: templateID,
			Description: fmt.Sprintf("Cập nhật mẫu biểu mẫu quy trình %s", row["name"]), Metadata: Record{"status": body["status"]},
		}
	})
	if err != nil {
		return nil, err
	}
	return r.WorkflowFormTemplateDetail(ctx, templateID)
}

func (r *Repository) ArchiveWorkflowFormTemplate(ctx context.Context, templateID string, actorID, actorName string) (Record, error) {
	usageRows, err := r.Query(ctx, `
		SELECT COUNT(*) AS total
		FROM pipeline_template_steps pts
		JOIN workflow_form_versions v ON v.id=pts.form_version_id
		WHERE v.template_id=$1
	`, templateID)
	if err != nil {
		return nil, err
	}
	if len(usageRows) > 0 && number(usageRows[0]["total"]) > 0 {
		return nil, validationError("Không thể lưu trữ mẫu biểu mẫu quy trình đang được bước quy trình sử dụng", Record{"step_usage_count": usageRows[0]["total"]})
	}
	_, err = r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		rows, queryErr := queryTx(ctx, tx, `
			UPDATE workflow_form_templates
			SET status='archived', updated_at=now()
			WHERE id=$1
			RETURNING name
		`, templateID)
		if queryErr != nil {
			return nil, queryErr
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		return rows[0], nil
	}, func(row Record) auditEvent {
		return auditEvent{
			ActorID: actorID, ActorName: actorName, EventType: "workflow_form_template_archived",
			TargetType: "workflow_form_template", TargetID: templateID,
			Description: fmt.Sprintf("Lưu trữ mẫu biểu mẫu quy trình %s", row["name"]), Metadata: Record{},
		}
	})
	if err != nil {
		return nil, err
	}
	return r.WorkflowFormTemplateDetail(ctx, templateID)
}

func (r *Repository) CreateWorkflowFormVersion(ctx context.Context, templateID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var templateName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM workflow_form_templates WHERE id=$1 AND status='active' FOR UPDATE`, templateID).Scan(&templateName); err != nil {
		return nil, err
	}
	sourceVersionID := cleanID(body["source_version_id"])
	schemaJSON := body["schema_json"]
	validationJSON := body["validation_json"]
	fields := workflowFieldRecords(body["fields"])
	if sourceVersionID != "" {
		sourceRows, err := tx.QueryContext(ctx, `
			SELECT schema_json, validation_json
			FROM workflow_form_versions
			WHERE id=$1 AND template_id=$2
		`, sourceVersionID, templateID)
		if err != nil {
			return nil, err
		}
		source, err := scanRows(sourceRows)
		sourceRows.Close()
		if err != nil {
			return nil, err
		}
		if len(source) == 0 {
			return nil, sql.ErrNoRows
		}
		if recordEmpty(schemaJSON) {
			schemaJSON = source[0]["schema_json"]
		}
		if recordEmpty(validationJSON) {
			validationJSON = source[0]["validation_json"]
		}
		if len(fields) == 0 {
			copied, err := workflowFormFieldRowsTx(ctx, tx, sourceVersionID)
			if err != nil {
				return nil, err
			}
			fields = copied
		}
	}
	nextVersionNo := 1
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_no),0)+1 FROM workflow_form_versions WHERE template_id=$1`, templateID).Scan(&nextVersionNo); err != nil {
		return nil, err
	}
	rawSchema, _ := json.Marshal(defaultRecord(schemaJSON))
	rawValidation, _ := json.Marshal(defaultRecord(validationJSON))
	var versionID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO workflow_form_versions (template_id, version_no, status, schema_json, validation_json, created_by)
		VALUES ($1, $2, 'draft', $3::jsonb, $4::jsonb, NULLIF($5,'')::uuid)
		RETURNING id
	`, templateID, nextVersionNo, string(rawSchema), string(rawValidation), actorID).Scan(&versionID); err != nil {
		return nil, err
	}
	if err := insertWorkflowFormFieldsTx(ctx, tx, versionID, fields); err != nil {
		return nil, err
	}
	definitionHash, err := workflowFormDefinitionHash(schemaJSON, validationJSON, fields)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workflow_form_versions SET definition_hash=$2 WHERE id=$1`, versionID, definitionHash); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "workflow_form_version_created", "workflow_form_version", versionID, fmt.Sprintf("Tạo nháp biểu mẫu quy trình %s v%d", templateName, nextVersionNo), Record{"template_id": templateID, "source_version_id": sourceVersionID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.WorkflowFormVersionDetail(ctx, versionID)
}

func (r *Repository) UpdateWorkflowFormVersion(ctx context.Context, versionID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var templateID, templateName, status string
	var versionNo int
	if err := tx.QueryRowContext(ctx, `
		SELECT v.template_id::text, t.name, v.version_no, v.status
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$1
		FOR UPDATE
	`, versionID).Scan(&templateID, &templateName, &versionNo, &status); err != nil {
		return nil, err
	}
	if status != "draft" {
		return nil, validationError("Chỉ phiên bản nháp biểu mẫu quy trình mới được chỉnh sửa", Record{"status": status})
	}
	rawSchema, _ := json.Marshal(defaultRecord(body["schema_json"]))
	rawValidation, _ := json.Marshal(defaultRecord(body["validation_json"]))
	if _, err := tx.ExecContext(ctx, `
		UPDATE workflow_form_versions
		SET schema_json=$2::jsonb,
			validation_json=$3::jsonb,
			updated_at=now()
		WHERE id=$1
	`, versionID, string(rawSchema), string(rawValidation)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM workflow_form_fields WHERE form_version_id=$1`, versionID); err != nil {
		return nil, err
	}
	if err := insertWorkflowFormFieldsTx(ctx, tx, versionID, workflowFieldRecords(body["fields"])); err != nil {
		return nil, err
	}
	definitionHash, err := workflowFormDefinitionHash(body["schema_json"], body["validation_json"], workflowFieldRecords(body["fields"]))
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workflow_form_versions SET definition_hash=$2 WHERE id=$1`, versionID, definitionHash); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "workflow_form_version_updated", "workflow_form_version", versionID, fmt.Sprintf("Cập nhật nháp biểu mẫu quy trình %s v%d", templateName, versionNo), Record{"template_id": templateID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.WorkflowFormVersionDetail(ctx, versionID)
}

func (r *Repository) PublishWorkflowFormVersion(ctx context.Context, versionID string, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var templateID, templateName, status string
	var versionNo int
	if err := tx.QueryRowContext(ctx, `
		SELECT v.template_id::text, t.name, v.version_no, v.status
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$1
		FOR UPDATE
	`, versionID).Scan(&templateID, &templateName, &versionNo, &status); err != nil {
		return nil, err
	}
	if status != "draft" {
		return nil, validationError("Chỉ phiên bản nháp biểu mẫu quy trình mới được phát hành", Record{"status": status})
	}
	var fieldCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_form_fields WHERE form_version_id=$1`, versionID).Scan(&fieldCount); err != nil {
		return nil, err
	}
	if fieldCount == 0 {
		return nil, validationError("Phiên bản biểu mẫu quy trình cần ít nhất một trường trước khi phát hành")
	}
	definitionHash, err := workflowFormDefinitionHashTx(ctx, tx, versionID)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE workflow_form_versions
		SET status='published',
			definition_hash=$2,
			published_at=COALESCE(published_at, now()),
			updated_at=now()
		WHERE id=$1
	`, versionID, definitionHash); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "workflow_form_version_published", "workflow_form_version", versionID, fmt.Sprintf("Phát hành biểu mẫu quy trình %s v%d", templateName, versionNo), Record{"template_id": templateID, "field_count": fieldCount}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.WorkflowFormVersionDetail(ctx, versionID)
}

func (r *Repository) ArchiveWorkflowFormVersion(ctx context.Context, versionID string, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var templateID, templateName, status string
	var versionNo int
	if err := tx.QueryRowContext(ctx, `
		SELECT v.template_id::text, t.name, v.version_no, v.status
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$1
		FOR UPDATE
	`, versionID).Scan(&templateID, &templateName, &versionNo, &status); err != nil {
		return nil, err
	}
	if status == "archived" {
		return r.WorkflowFormVersionDetail(ctx, versionID)
	}
	var stepUsage int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM pipeline_template_steps WHERE form_version_id=$1`, versionID).Scan(&stepUsage); err != nil {
		return nil, err
	}
	if stepUsage > 0 {
		return nil, validationError("Không thể lưu trữ phiên bản biểu mẫu quy trình đang được bước quy trình sử dụng", Record{"step_usage_count": stepUsage})
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workflow_form_versions SET status='archived', updated_at=now() WHERE id=$1`, versionID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "workflow_form_version_archived", "workflow_form_version", versionID, fmt.Sprintf("Lưu trữ biểu mẫu quy trình %s v%d", templateName, versionNo), Record{"template_id": templateID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.WorkflowFormVersionDetail(ctx, versionID)
}

func workflowFieldRecords(value any) []Record {
	fields, ok := value.([]Record)
	if ok {
		return fields
	}
	rawFields, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]Record, 0, len(rawFields))
	for _, raw := range rawFields {
		if record, ok := raw.(Record); ok {
			out = append(out, record)
			continue
		}
		if record, ok := raw.(map[string]any); ok {
			out = append(out, Record(record))
		}
	}
	return out
}

func workflowFormFieldRowsTx(ctx context.Context, tx *sql.Tx, versionID string) ([]Record, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT field_key, label, field_type, required, options_json, validation_json, help_text, order_index, visibility_condition_json
		FROM workflow_form_fields
		WHERE form_version_id=$1
		ORDER BY order_index, created_at, id
	`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func insertWorkflowFormFieldsTx(ctx context.Context, tx *sql.Tx, versionID string, fields []Record) error {
	for index, field := range fields {
		orderIndex := int(number(field["order_index"]))
		if orderIndex <= 0 {
			orderIndex = (index + 1) * 10
		}
		rawOptions, _ := json.Marshal(defaultArray(field["options_json"]))
		rawValidation, _ := json.Marshal(defaultRecord(field["validation_json"]))
		rawVisibility, _ := json.Marshal(defaultRecord(field["visibility_condition_json"]))
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workflow_form_fields
				(form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index, visibility_condition_json)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, NULLIF($8,''), $9, $10::jsonb)
		`, versionID, field["field_key"], field["label"], field["field_type"], boolValue(field["required"]), string(rawOptions), string(rawValidation), cleanOptionalText(field["help_text"]), orderIndex, string(rawVisibility)); err != nil {
			return err
		}
	}
	return nil
}

func defaultRecord(value any) any {
	if value == nil {
		return Record{}
	}
	return value
}

func defaultArray(value any) any {
	if value == nil {
		return []any{}
	}
	return value
}

func recordEmpty(value any) bool {
	if value == nil {
		return true
	}
	if record, ok := value.(Record); ok {
		return len(record) == 0
	}
	if record, ok := value.(map[string]any); ok {
		return len(record) == 0
	}
	return false
}

func (r *Repository) DefaultWorkflowFormVersionID(ctx context.Context, stepType string, checklistRequired bool, stepContext ...string) (string, error) {
	stepName := ""
	if len(stepContext) > 0 {
		stepName = stepContext[0]
	}
	code := defaultWorkflowFormCode(stepType, checklistRequired, stepName)
	row, err := r.QueryOne(ctx, `
		SELECT v.id
		FROM workflow_form_templates t
		JOIN workflow_form_versions v ON v.template_id=t.id
		WHERE t.code=$1 AND t.status='active' AND v.status='published'
		  AND COALESCE(v.schema_json->>'variant','')<>'operational_capture_v3'
		ORDER BY v.version_no DESC
		LIMIT 1
	`, code)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return cleanID(row["id"]), nil
}

func defaultWorkflowFormCode(stepType string, checklistRequired bool, stepName string) string {
	normalizedName := normalizedWorkflowStepName(stepName)
	if stepType == "daily_report" {
		if isConstructionDailyReportStep(normalizedName) {
			return "builtin_construction_daily_report"
		}
		return "builtin_daily_report"
	}
	if stepType == "meeting_minutes" {
		return "builtin_meeting_minutes"
	}
	if statusIn(stepType, "checklist", "acceptance_check", "handover_check") || checklistRequired {
		if strings.Contains(normalizedName, "chu dau tu") || strings.Contains(normalizedName, "client") || strings.Contains(normalizedName, "owner") {
			return "builtin_owner_client_checklist"
		}
		if strings.Contains(normalizedName, "thau phu") || strings.Contains(normalizedName, "subcontractor") {
			return "builtin_subcontractor_checklist"
		}
		return "builtin_checklist"
	}
	return "builtin_generic_submission"
}

func normalizedWorkflowStepName(stepName string) string {
	return strings.ToLower(strings.TrimSpace(asciiFold(stepName)))
}

func isConstructionDailyReportStep(normalizedName string) bool {
	return strings.Contains(normalizedName, "thi cong") ||
		strings.Contains(normalizedName, "construction") ||
		strings.Contains(normalizedName, "cong truong") ||
		strings.Contains(normalizedName, "site report")
}

func (r *Repository) TemplatePublishFormIssues(ctx context.Context, templateID string) ([]Record, error) {
	return r.Query(ctx, templatePublishFormIssuesSQL(), templateID)
}

func templatePublishFormIssuesSQL() string {
	return `
		SELECT
			n.id AS node_id,
			n.name AS node_name,
			pts.id AS step_id,
			pts.name AS step_name,
			pts.step_type,
			pts.form_required,
			pts.form_version_id,
			COALESCE(wfv.status, 'missing') AS form_version_status,
			wfv.version_no AS form_version_no,
			wft.name AS form_name,
			wft.status AS form_template_status
		FROM pipeline_template_steps pts
		JOIN pipeline_template_nodes n ON n.id=pts.node_id
		LEFT JOIN workflow_form_versions wfv ON wfv.id=pts.form_version_id
		LEFT JOIN workflow_form_templates wft ON wft.id=wfv.template_id
		WHERE n.template_id=$1
		  AND COALESCE(pts.form_required, false)
		  AND (
			pts.form_version_id IS NULL
			OR wfv.id IS NULL
			OR wfv.status <> 'published'
			OR wft.status <> 'active'
		  )
		ORDER BY n.order_index, pts.order_index, pts.created_at, pts.id
	`
}

func (r *Repository) SaveWorkflowForm(ctx context.Context, instanceID string, answers Record, comment string, submit bool, actorID, actorName string) (Record, error) {
	if answers == nil {
		answers = Record{}
	}
	rawSnapshot, _ := json.Marshal(answers)
	eventType := "draft_saved"
	toStatus := "draft"
	if submit {
		eventType = "submitted"
		toStatus = "submitted"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		SELECT wfi.*, wft.code AS form_code
		FROM workflow_form_instances wfi
		JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		WHERE wfi.id=$1
		FOR UPDATE OF wfi
	`, instanceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	instance := rows[0]
	fromStatus := fmt.Sprint(instance["status"])
	if statusIn(fromStatus, "cancelled") || (!submit && fromStatus == "submitted") || (submit && fromStatus == "submitted") {
		return nil, validationError("Trạng thái biểu mẫu quy trình đã thay đổi, vui lòng tải lại", Record{"status": fromStatus})
	}
	if err := startWorkflowFormWorkItemTx(ctx, tx, instance, submit, actorID, actorName); err != nil {
		return nil, err
	}
	revisionNo := int(number(instance["current_revision"]))
	if submit {
		revisionNo++
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM workflow_form_answers wfa
		WHERE wfa.instance_id=$1
		  AND NOT EXISTS (
			SELECT 1
			FROM jsonb_object_keys($2::jsonb) AS submitted(field_key)
			WHERE submitted.field_key=wfa.field_key
		  )
	`, instanceID, string(rawSnapshot)); err != nil {
		return nil, err
	}
	for key, value := range answers {
		rawValue, _ := json.Marshal(value)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workflow_form_answers (instance_id, field_key, field_type, value_json, revision_no, updated_by)
			SELECT $1, wff.field_key, wff.field_type, $3::jsonb, $4, NULLIF($5,'')::uuid
			FROM workflow_form_fields wff
			WHERE wff.form_version_id=$2 AND wff.field_key=$6
			ON CONFLICT (instance_id, field_key)
			DO UPDATE SET value_json=EXCLUDED.value_json, field_type=EXCLUDED.field_type, revision_no=EXCLUDED.revision_no, updated_by=EXCLUDED.updated_by, updated_at=now()
		`, instanceID, instance["form_version_id"], string(rawValue), revisionNo, actorID, key); err != nil {
			return nil, err
		}
	}
	if submit {
		if _, err := tx.ExecContext(ctx, `
			UPDATE workflow_form_instances
			SET status='submitted',
				current_revision=$2,
				answers_snapshot_json=$3::jsonb,
				submitted_by=NULLIF($4,'')::uuid,
				submitted_at=now(),
				updated_at=now()
			WHERE id=$1
		`, instanceID, revisionNo, string(rawSnapshot), actorID); err != nil {
			return nil, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE workflow_form_instances
			SET answers_snapshot_json=$2::jsonb,
				updated_at=now()
			WHERE id=$1
		`, instanceID, string(rawSnapshot)); err != nil {
			return nil, err
		}
	}
	rawMetadata, _ := json.Marshal(Record{
		"form_type":       instance["form_type"],
		"form_version_id": instance["form_version_id"],
		"revision_no":     revisionNo,
	})
	workItemID := cleanID(instance["work_item_id"])
	projectID := cleanID(instance["project_id"])
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_form_events (instance_id, project_id, work_item_id, actor_id, event_type, from_status, to_status, comment, metadata_json)
		VALUES ($1, $2, NULLIF($3,'')::uuid, NULLIF($4,'')::uuid, $5, $6, $7, NULLIF($8,''), $9::jsonb)
	`, instanceID, projectID, workItemID, actorID, eventType, fromStatus, toStatus, comment, string(rawMetadata)); err != nil {
		return nil, err
	}
	if workItemID != "" {
		activityComment := comment
		if activityComment == "" {
			if submit {
				activityComment = "Đã gửi biểu mẫu quy trình"
			} else {
				activityComment = "Đã lưu nháp biểu mẫu quy trình"
			}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, $5::jsonb)
		`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
			return nil, err
		}
		if submit {
			generatedWorkItemIDs := []string{}
			if fmt.Sprint(instance["form_type"]) == "meeting_minutes" {
				var err error
				generatedWorkItemIDs, err = createMeetingMinutesActionItemsTx(ctx, tx, instance, answers, actorID)
				if err != nil {
					return nil, err
				}
			} else if fmt.Sprint(instance["form_type"]) == "daily_report" {
				if strings.TrimSpace(fmt.Sprint(instance["form_code"])) == "builtin_construction_daily_report" {
					if err := syncConstructionDailyReportTx(ctx, tx, instance, answers, actorID); err != nil {
						return nil, err
					}
				}
				var err error
				generatedWorkItemIDs, err = createDailyReportNextItemsTx(ctx, tx, instance, answers, actorID)
				if err != nil {
					return nil, err
				}
			}
			payload := Record{
				"workflow_form_instance_id": instanceID,
				"workflow_form_version_id":  instance["form_version_id"],
				"workflow_form_type":        instance["form_type"],
				"workflow_form_answers":     answers,
			}
			if len(generatedWorkItemIDs) > 0 {
				payload["generated_work_item_ids"] = generatedWorkItemIDs
			}
			if fmt.Sprint(instance["form_type"]) == "checklist" {
				if checklistItems, ok := answers["checklist_items"]; ok {
					payload["checklist_items"] = checklistItems
					payload["checklist"] = checklistItems
				} else {
					payload["checklist"] = []Record{{
						"label":    "Đã gửi biểu mẫu quy trình",
						"checked":  true,
						"source":   "workflow_form",
						"form_id":  instanceID,
						"revision": revisionNo,
					}}
				}
			}
			rawPayload, _ := json.Marshal(payload)
			if _, err := tx.ExecContext(ctx, `
				UPDATE work_items
				SET completion_payload_json=COALESCE(completion_payload_json, '{}'::jsonb) || $2::jsonb,
					updated_by=NULLIF($3,'')::uuid,
					updated_at=now()
				WHERE id=$1
			`, workItemID, string(rawPayload), actorID); err != nil {
				return nil, err
			}
		}
		auditDescription := "Lưu nháp biểu mẫu quy trình"
		if submit {
			auditDescription = "Gửi biểu mẫu quy trình"
		}
		if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "workflow_form_"+eventType, "workflow_form_instance", instanceID, auditDescription, Record{"work_item_id": workItemID, "form_type": instance["form_type"], "revision_no": revisionNo}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetWorkflowFormInstanceDetail(ctx, instanceID)
}

func startWorkflowFormWorkItemTx(ctx context.Context, tx *sql.Tx, instance Record, submit bool, actorID, actorName string) error {
	workItemID := cleanID(instance["work_item_id"])
	if workItemID == "" {
		return nil
	}
	rows, err := queryTx(ctx, tx, `
		SELECT
			wi.*,
			COALESCE(pr.engine_version, 1) AS pipeline_engine_version,
			pr.current_node_id AS pipeline_current_node_id,
			pr.status AS pipeline_run_status,
			current_node.status AS pipeline_current_node_status,
			target_node.status AS run_node_status
		FROM work_items wi
		LEFT JOIN pipeline_runs pr ON pr.id=wi.pipeline_run_id
		LEFT JOIN pipeline_run_nodes current_node ON current_node.id=pr.current_node_id
		LEFT JOIN pipeline_run_nodes target_node ON target_node.id=wi.run_node_id
		WHERE wi.id=$1
		FOR UPDATE OF wi
	`, workItemID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return sql.ErrNoRows
	}
	item := rows[0]
	fromStatus := fmt.Sprint(item["status"])
	if !statusIn(fromStatus, "ready", "needs_changes") {
		return nil
	}
	if int(number(item["pipeline_engine_version"])) >= 2 && cleanID(item["pipeline_run_id"]) != "" {
		currentNodeID := cleanID(item["pipeline_current_node_id"])
		currentNodeStatus := fmt.Sprint(item["pipeline_current_node_status"])
		targetNodeStatus := fmt.Sprint(item["run_node_status"])
		targetIsCurrent := currentNodeID != "" && currentNodeID == cleanID(item["run_node_id"])
		allowedEarlyExecution := workItemHasStarted(item) && !targetIsCurrent &&
			statusIn(currentNodeStatus, "in_progress", "waiting_approval", "blocked") &&
			statusIn(targetNodeStatus, "pending", "in_progress", "waiting_approval", "blocked")
		if statusIn(fmt.Sprint(item["pipeline_run_status"]), "completed", "cancelled") ||
			(!targetIsCurrent && !allowedEarlyExecution) ||
			(targetIsCurrent && !statusIn(currentNodeStatus, "in_progress", "waiting_approval", "blocked")) {
			return validationError("Đầu việc không còn thuộc node đang thực hiện của quy trình", Record{
				"work_item_id":    workItemID,
				"run_node_id":     item["run_node_id"],
				"current_node_id": currentNodeID,
			})
		}
	}

	formAction := "draft_saved"
	comment := "Bắt đầu đầu việc khi lưu nháp hồ sơ nghiệp vụ"
	if submit {
		formAction = "submitted"
		comment = "Bắt đầu đầu việc khi chốt hồ sơ nghiệp vụ"
	}
	metadata := Record{
		"action":                    "started",
		"source":                    "workflow_form_first_write",
		"workflow_form_action":      formAction,
		"workflow_form_instance_id": instance["id"],
		"workflow_form_version_id":  instance["form_version_id"],
	}
	return persistWorkItemStartedTx(ctx, tx, item, actorID, actorName, comment, metadata)
}

func persistWorkItemStartedTx(ctx context.Context, tx *sql.Tx, item Record, actorID, actorName, comment string, metadata Record) error {
	workItemID := cleanID(item["id"])
	projectID := cleanID(item["project_id"])
	fromStatus := fmt.Sprint(item["status"])
	if workItemID == "" || projectID == "" {
		return validationError("Đầu việc không hợp lệ", Record{"work_item_id": workItemID, "project_id": projectID})
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE work_items
		SET status='in_progress',
			started_at=COALESCE(started_at, now()),
			updated_by=NULLIF($3,'')::uuid,
			updated_at=now()
		WHERE id=$1 AND status=$2
		RETURNING id
	`, workItemID, fromStatus, actorID)
	if err != nil {
		return err
	}
	if len(updated) == 0 {
		return validationError("Trạng thái đầu việc đã thay đổi, vui lòng tải lại trước khi lưu hồ sơ", Record{"expected_status": fromStatus})
	}
	if err := applyRunStepStatusTx(ctx, tx, item, "in_progress", actorID); err != nil {
		return err
	}
	if metadata == nil {
		metadata = Record{}
	}
	metadata["action"] = "started"
	rawMetadata, _ := json.Marshal(metadata)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, from_status, to_status, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'started', $4, $5, 'in_progress', $6::jsonb)
	`, workItemID, projectID, actorID, comment, fromStatus, string(rawMetadata)); err != nil {
		return err
	}
	return addAuditTx(
		ctx,
		tx,
		projectID,
		actorID,
		actorName,
		"work_item_started",
		"work_item",
		workItemID,
		workItemAuditDescription("started", item, ""),
		metadata,
	)
}

func (r *Repository) AmendWorkflowForm(ctx context.Context, instanceID string, answers Record, reason string, expectedRevision int, actorID, actorName string) (Record, error) {
	if answers == nil {
		answers = Record{}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		SELECT wfi.*, wft.code AS form_code
		FROM workflow_form_instances wfi
		JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		WHERE wfi.id=$1
		FOR UPDATE OF wfi
	`, instanceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	instance := rows[0]
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, conflictError("Trạng thái hồ sơ đã thay đổi, vui lòng tải lại", Record{"status": instance["status"]})
	}
	constructionDailyReport := fmt.Sprint(instance["form_type"]) == "daily_report" && strings.TrimSpace(fmt.Sprint(instance["form_code"])) == "builtin_construction_daily_report"
	if constructionDailyReport {
		if err := ensureConstructionDailyReportAmendableTx(ctx, tx, instanceID); err != nil {
			return nil, err
		}
	}
	currentRevision := int(number(instance["current_revision"]))
	if currentRevision != expectedRevision {
		return nil, conflictError("Phiên bản hồ sơ đã thay đổi, vui lòng tải lại trước khi lưu", Record{
			"expected_revision": expectedRevision,
			"current_revision":  currentRevision,
		})
	}
	fields, err := workflowFormFieldRowsTx(ctx, tx, cleanID(instance["form_version_id"]))
	if err != nil {
		return nil, err
	}
	previousAnswers := recordFromAny(instance["answers_snapshot_json"])
	changes := workflowFormAnswerChanges(previousAnswers, answers, fields)
	if len(changes) == 0 {
		return nil, validationError("Hồ sơ chưa có nội dung thay đổi", Record{"reason": "no_changes"})
	}
	nextRevision := currentRevision + 1
	rawSnapshot, _ := json.Marshal(answers)
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM workflow_form_answers wfa
		WHERE wfa.instance_id=$1
		  AND NOT EXISTS (
			SELECT 1
			FROM jsonb_object_keys($2::jsonb) AS amended(field_key)
			WHERE amended.field_key=wfa.field_key
		  )
	`, instanceID, string(rawSnapshot)); err != nil {
		return nil, err
	}
	for key, value := range answers {
		rawValue, _ := json.Marshal(value)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workflow_form_answers (instance_id, field_key, field_type, value_json, revision_no, updated_by)
			SELECT $1, wff.field_key, wff.field_type, $3::jsonb, $4, NULLIF($5,'')::uuid
			FROM workflow_form_fields wff
			WHERE wff.form_version_id=$2 AND wff.field_key=$6
			ON CONFLICT (instance_id, field_key)
			DO UPDATE SET value_json=EXCLUDED.value_json, field_type=EXCLUDED.field_type, revision_no=EXCLUDED.revision_no, updated_by=EXCLUDED.updated_by, updated_at=now()
		`, instanceID, instance["form_version_id"], string(rawValue), nextRevision, actorID, key); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE workflow_form_instances
		SET current_revision=$2,
			answers_snapshot_json=$3::jsonb,
			updated_at=now()
		WHERE id=$1
	`, instanceID, nextRevision, string(rawSnapshot)); err != nil {
		return nil, err
	}

	workItemID := cleanID(instance["work_item_id"])
	projectID := cleanID(instance["project_id"])
	completionPatch := Record{
		"workflow_form_instance_id": instanceID,
		"workflow_form_version_id":  instance["form_version_id"],
		"workflow_form_type":        instance["form_type"],
		"workflow_form_answers":     answers,
		"workflow_form_revision":    nextRevision,
	}
	if fmt.Sprint(instance["form_type"]) == "checklist" {
		if checklistItems, ok := answers["checklist_items"]; ok {
			completionPatch["checklist_items"] = checklistItems
			completionPatch["checklist"] = checklistItems
		}
	}
	rawCompletionPatch, _ := json.Marshal(completionPatch)
	if workItemID != "" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE work_items
			SET completion_payload_json=COALESCE(completion_payload_json, '{}'::jsonb) || $2::jsonb,
				updated_by=NULLIF($3,'')::uuid,
				updated_at=now()
			WHERE id=$1
		`, workItemID, string(rawCompletionPatch), actorID); err != nil {
			return nil, err
		}
	}
	if constructionDailyReport {
		if err := syncConstructionDailyReportTx(ctx, tx, instance, answers, actorID); err != nil {
			return nil, err
		}
	}

	metadata := Record{
		"workflow_form_instance_id": instanceID,
		"form_type":                 instance["form_type"],
		"form_version_id":           instance["form_version_id"],
		"from_revision":             currentRevision,
		"to_revision":               nextRevision,
		"reason":                    reason,
		"changed_fields":            changes,
		"downstream_reprocessed":    false,
	}
	rawMetadata, _ := json.Marshal(metadata)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_form_events (instance_id, project_id, work_item_id, actor_id, event_type, from_status, to_status, comment, metadata_json)
		VALUES ($1,$2,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,'amended','submitted','submitted',$5,$6::jsonb)
	`, instanceID, projectID, workItemID, actorID, reason, string(rawMetadata)); err != nil {
		return nil, err
	}
	if workItemID != "" {
		activityComment := fmt.Sprintf(
			"Đã chỉnh sửa hồ sơ nghiệp vụ v%d → v%d: %s. Lý do: %s",
			currentRevision,
			nextRevision,
			workflowFormChangedFieldLabels(changes),
			reason,
		)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
			VALUES ($1,$2,'work_item',$1,NULLIF($3,'')::uuid,'system_event',$4,'internal',$5::jsonb)
		`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "workflow_form_amended", "workflow_form_instance", instanceID, "Chỉnh sửa hồ sơ nghiệp vụ đã chốt", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetWorkflowFormInstanceDetail(ctx, instanceID)
}

func workflowFormAnswerChanges(before, after Record, fields []Record) []Record {
	changes := make([]Record, 0)
	for _, field := range fields {
		key := strings.TrimSpace(fmt.Sprint(field["field_key"]))
		if key == "" {
			continue
		}
		beforeValue, beforePresent := before[key]
		afterValue, afterPresent := after[key]
		if beforePresent == afterPresent && workflowFormJSONValuesEqual(beforeValue, afterValue) {
			continue
		}
		changes = append(changes, Record{
			"field_key":      key,
			"field_label":    strings.TrimSpace(fmt.Sprint(field["label"])),
			"field_type":     field["field_type"],
			"before":         beforeValue,
			"after":          afterValue,
			"before_present": beforePresent,
			"after_present":  afterPresent,
		})
	}
	return changes
}

func workflowFormJSONValuesEqual(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}

func workflowFormChangedFieldLabels(changes []Record) string {
	labels := make([]string, 0, len(changes))
	for _, change := range changes {
		label := strings.TrimSpace(fmt.Sprint(change["field_label"]))
		if label == "" {
			label = strings.TrimSpace(fmt.Sprint(change["field_key"]))
		}
		if label != "" {
			labels = append(labels, label)
		}
	}
	if len(labels) == 0 {
		return "nội dung hồ sơ"
	}
	return strings.Join(labels, ", ")
}

func syncConstructionDailyReportTx(ctx context.Context, tx *sql.Tx, instance Record, answers Record, actorID string) error {
	completedItems := workflowAnswerRecordList(answers["completed_items"])
	crewItems := workflowAnswerRecordList(answers["crew_items"])
	materialItems := workflowAnswerRecordList(answers["material_usage_items"])
	blockers := workflowAnswerRecordList(answers["blockers"])
	workTitles := make([]string, 0, len(completedItems))
	for _, item := range completedItems {
		if title := strings.TrimSpace(fmt.Sprint(item["title"])); title != "" && title != "<nil>" {
			workTitles = append(workTitles, title)
		}
	}
	workSummary := strings.Join(workTitles, "; ")
	if workSummary == "" {
		workSummary = fmt.Sprintf("%d hạng mục thi công", len(completedItems))
	}
	manpower := 0.0
	for _, item := range crewItems {
		manpower += number(item["worker_count"])
	}
	materialNames := make([]string, 0, len(materialItems))
	for _, item := range materialItems {
		if name := strings.TrimSpace(fmt.Sprint(item["material_name"])); name != "" && name != "<nil>" {
			materialNames = append(materialNames, name)
		}
	}
	blockerTitles := make([]string, 0, len(blockers))
	for _, item := range blockers {
		title := strings.TrimSpace(fmt.Sprint(item["content"]))
		if title == "" || title == "<nil>" {
			title = strings.TrimSpace(fmt.Sprint(item["title"]))
		}
		if title != "" && title != "<nil>" {
			blockerTitles = append(blockerTitles, title)
		}
	}
	primaryEvidenceID := cleanID(answers["site_photo_evidence_id"])
	photos := []string{}
	if primaryEvidenceID != "" {
		photos = append(photos, primaryEvidenceID)
	}
	structuredRaw, _ := json.Marshal(answers)
	photosRaw, _ := json.Marshal(photos)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO operation_daily_reports (
			project_id,report_date,reported_by,weather,work_summary,manpower_summary,material_usage_summary,
			issues_summary,progress_percent,photos,workflow_form_instance_id,work_item_id,structured_json,primary_evidence_id
		) VALUES ($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7,$8,$9,$10::jsonb,$11,NULLIF($12,'')::uuid,$13::jsonb,NULLIF($14,'')::uuid)
		ON CONFLICT (workflow_form_instance_id) WHERE workflow_form_instance_id IS NOT NULL DO UPDATE SET
			report_date=EXCLUDED.report_date,reported_by=EXCLUDED.reported_by,weather=EXCLUDED.weather,
			work_summary=EXCLUDED.work_summary,manpower_summary=EXCLUDED.manpower_summary,
			material_usage_summary=EXCLUDED.material_usage_summary,issues_summary=EXCLUDED.issues_summary,
			progress_percent=EXCLUDED.progress_percent,photos=EXCLUDED.photos,structured_json=EXCLUDED.structured_json,
			primary_evidence_id=EXCLUDED.primary_evidence_id
	`, instance["project_id"], answers["report_date"], actorID, answers["weather"], workSummary,
		fmt.Sprintf("%.0f nhân sự / %d đội", manpower, len(crewItems)), strings.Join(materialNames, "; "),
		strings.Join(blockerTitles, "; "), int(number(answers["progress_percent"])), string(photosRaw), instance["id"], cleanID(instance["work_item_id"]), string(structuredRaw), primaryEvidenceID)
	return err
}

func ensureConstructionDailyReportAmendableTx(ctx context.Context, tx *sql.Tx, workflowFormInstanceID string) error {
	var sourceID string
	var deliveryEligible bool
	err := tx.QueryRowContext(ctx, `
		SELECT id::text,report_delivery_eligible
		FROM operation_daily_reports
		WHERE workflow_form_instance_id=$1
		FOR UPDATE
	`, workflowFormInstanceID).Scan(&sourceID, &deliveryEligible)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !deliveryEligible {
		return validationError("Nhật ký lịch sử này chỉ dùng để tra cứu và không thể chỉnh sửa", Record{"workflow_form_instance_id": workflowFormInstanceID})
	}
	var status string
	err = tx.QueryRowContext(ctx, `
		SELECT status
		FROM work_reports
		WHERE source_kind='operation_daily_report' AND source_id=$1
		FOR UPDATE
	`, sourceID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if status != "needs_changes" {
		return validationError("Báo cáo chỉ được chỉnh sửa sau khi người nhận yêu cầu bổ sung", Record{"workflow_form_instance_id": workflowFormInstanceID, "status": status})
	}
	return nil
}

func workflowAnswerRecordList(value any) []Record {
	switch typed := value.(type) {
	case []Record:
		return typed
	case []any:
		out := make([]Record, 0, len(typed))
		for _, item := range typed {
			if record, ok := item.(map[string]any); ok {
				out = append(out, Record(record))
			}
		}
		return out
	default:
		return nil
	}
}

func (r *Repository) ListSealAssets(ctx context.Context, includeArchived bool) ([]Record, error) {
	statusFilter := "WHERE asset.status='active'"
	if includeArchived {
		statusFilter = ""
	}
	return r.Query(ctx, `
		SELECT asset.*,COALESCE(uploader.deleted_at IS NOT NULL,false) AS uploaded_by_account_deleted
		FROM seal_assets asset
		LEFT JOIN users uploader ON uploader.id=asset.uploaded_by
		`+statusFilter+`
		ORDER BY CASE asset.status WHEN 'active' THEN 0 ELSE 1 END, asset.created_at DESC, asset.id
	`)
}

func (r *Repository) GetSealAsset(ctx context.Context, sealAssetID string) (Record, error) {
	return r.QueryOne(ctx, `SELECT * FROM seal_assets WHERE id=$1`, sealAssetID)
}

func (r *Repository) CreateSealAsset(
	ctx context.Context,
	name, originalName, sourceMIME string,
	file StoredFileMetadata,
	width, height int,
	actorID, actorName string,
) (result Record, err error) {
	tracker := storedMutationTracker{}
	defer tracker.wrap(&err)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		INSERT INTO seal_assets (
			name, original_name, source_mime_type,
			storage_provider, storage_key, mime_type, file_size,
			width_px, height_px, checksum_sha256, uploaded_by, uploaded_by_name
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NULLIF($11,'')::uuid,$12)
		RETURNING *
	`, name, originalName, sourceMIME, file.StorageProvider, file.StorageKey, file.MIMEType, file.FileSize, width, height, file.ChecksumSHA256, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	asset := rows[0]
	assetID := cleanID(asset["id"])
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "seal_asset_uploaded", "seal_asset", assetID, "Tải tài nguyên con dấu công ty", Record{
		"name":             name,
		"source_mime_type": sourceMIME,
		"mime_type":        file.MIMEType,
		"file_size":        file.FileSize,
		"width_px":         width,
		"height_px":        height,
		"checksum_sha256":  file.ChecksumSHA256,
	}); err != nil {
		return nil, err
	}
	return finalizeStoredMutation(&tracker, func() (Record, error) { return asset, nil }, tx.Commit)
}

func (r *Repository) ArchiveSealAsset(ctx context.Context, sealAssetID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM seal_assets WHERE id=$1 FOR UPDATE`, sealAssetID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if fmt.Sprint(rows[0]["status"]) == "archived" {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return rows[0], nil
	}
	updatedRows, err := queryTx(ctx, tx, `
		UPDATE seal_assets
		SET status='archived', archived_by=NULLIF($2,'')::uuid, archived_by_name=$3, archived_at=now(), updated_at=now()
		WHERE id=$1 AND status='active'
		RETURNING *
	`, sealAssetID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if len(updatedRows) == 0 {
		return nil, sql.ErrNoRows
	}
	asset := updatedRows[0]
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "seal_asset_archived", "seal_asset", sealAssetID, "Lưu trữ tài nguyên con dấu công ty", Record{
		"name":            asset["name"],
		"checksum_sha256": asset["checksum_sha256"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return asset, nil
}

func (r *Repository) ApplyWorkflowFormSeal(ctx context.Context, instanceID, sealAssetID, purpose, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		SELECT
			wfi.*,
			wft.code AS form_code,
			wft.name AS form_name,
			wfv.version_no AS form_version_no
		FROM workflow_form_instances wfi
		JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
		JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		WHERE wfi.id=$1
		FOR UPDATE
	`, instanceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	instance := rows[0]
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Biểu mẫu quy trình phải được chốt trước khi đóng dấu", Record{"status": instance["status"]})
	}
	projectID := cleanID(instance["project_id"])
	workItemID := cleanID(instance["work_item_id"])
	formVersionID := cleanID(instance["form_version_id"])
	formRevisionNo := int(number(instance["current_revision"]))
	if projectID == "" || workItemID == "" || formVersionID == "" {
		return nil, validationError("Biểu mẫu quy trình thiếu liên kết để đóng dấu", Record{"workflow_form_instance_id": instanceID})
	}

	assetRows, err := queryTx(ctx, tx, `SELECT * FROM seal_assets WHERE id=$1 AND status='active' FOR SHARE`, sealAssetID)
	if err != nil {
		return nil, err
	}
	if len(assetRows) == 0 {
		return nil, validationError("Con dấu không tồn tại hoặc đã được lưu trữ", Record{"seal_asset_id": sealAssetID})
	}
	asset := assetRows[0]

	duplicateRows, err := queryTx(ctx, tx, `
		SELECT COUNT(*) AS total
		FROM workflow_form_seal_applications
		WHERE form_instance_id=$1 AND form_revision_no=$2 AND purpose=$3 AND seal_asset_id=$4
	`, instanceID, formRevisionNo, purpose, sealAssetID)
	if err != nil {
		return nil, err
	}
	if len(duplicateRows) > 0 && number(duplicateRows[0]["total"]) > 0 {
		return nil, validationError("Con dấu này đã được áp dụng cho cùng mục đích trên hồ sơ", Record{"purpose": purpose, "seal_asset_id": sealAssetID})
	}

	roleRows, err := queryTx(ctx, tx, `
		SELECT COALESCE(jsonb_agg(role_name ORDER BY role_name), '[]'::jsonb) AS roles
		FROM (
			SELECT DISTINCT role.display_name AS role_name
			FROM user_roles assignment
			JOIN roles role ON role.id=assignment.role_id
			WHERE assignment.user_id=$1
		) actor_roles
	`, actorID)
	if err != nil {
		return nil, err
	}
	actorRoles := any([]any{})
	if len(roleRows) > 0 && roleRows[0]["roles"] != nil {
		actorRoles = roleRows[0]["roles"]
	}
	appliedByName := strings.TrimSpace(actorName)
	if appliedByName == "" {
		appliedByName = actorID
	}
	payload := Record{
		"source":                    "workflow_form_seal_application",
		"workflow_form_instance_id": instanceID,
		"workflow_form_version_id":  formVersionID,
		"workflow_form_revision_no": formRevisionNo,
		"workflow_form_type":        instance["form_type"],
		"workflow_form_code":        instance["form_code"],
		"workflow_form_title":       instance["title"],
		"answers_snapshot":          instance["answers_snapshot_json"],
		"project_id":                projectID,
		"work_item_id":              workItemID,
		"purpose":                   purpose,
		"comment":                   comment,
		"applied_by":                actorID,
		"applied_by_name":           appliedByName,
		"applied_by_roles":          actorRoles,
		"seal_asset": Record{
			"id":              sealAssetID,
			"name":            asset["name"],
			"mime_type":       asset["mime_type"],
			"file_size":       asset["file_size"],
			"width_px":        asset["width_px"],
			"height_px":       asset["height_px"],
			"checksum_sha256": asset["checksum_sha256"],
		},
	}
	rawPayload, _ := json.Marshal(payload)
	hashBytes := sha256.Sum256(rawPayload)
	payloadHash := hex.EncodeToString(hashBytes[:])
	rawRoles, _ := json.Marshal(actorRoles)

	applicationRows, err := queryTx(ctx, tx, `
		INSERT INTO workflow_form_seal_applications (
			project_id, work_item_id, form_instance_id, form_version_id, form_revision_no,
			seal_asset_id, purpose, applied_by, applied_by_name, applied_by_roles_json,
			comment, payload_json, payload_hash
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::uuid,$9,$10::jsonb,NULLIF($11,''),$12::jsonb,$13)
		RETURNING *
	`, projectID, workItemID, instanceID, formVersionID, formRevisionNo, sealAssetID, purpose, actorID, appliedByName, string(rawRoles), comment, string(rawPayload), payloadHash)
	if err != nil {
		return nil, err
	}
	if len(applicationRows) == 0 {
		return nil, sql.ErrNoRows
	}
	applicationID := cleanID(applicationRows[0]["id"])
	metadata := Record{
		"workflow_form_instance_id": instanceID,
		"seal_application_id":       applicationID,
		"seal_asset_id":             sealAssetID,
		"seal_asset_name":           asset["name"],
		"purpose":                   purpose,
		"payload_hash":              payloadHash,
		"form_revision_no":          formRevisionNo,
		"checksum_sha256":           asset["checksum_sha256"],
	}
	rawMetadata, _ := json.Marshal(metadata)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_form_events (instance_id, project_id, work_item_id, actor_id, event_type, from_status, to_status, comment, metadata_json)
		VALUES ($1,$2,$3,NULLIF($4,'')::uuid,'seal_applied','submitted','submitted',$5,$6::jsonb)
	`, instanceID, projectID, workItemID, actorID, fmt.Sprintf("Đã áp dụng con dấu: %s", asset["name"]), string(rawMetadata)); err != nil {
		return nil, err
	}
	activityComment := fmt.Sprintf("Đã áp dụng con dấu công ty: %s", asset["name"])
	if comment != "" {
		activityComment = comment
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES ($1,$2,'work_item',$1,NULLIF($3,'')::uuid,'system_event',$4,'internal',$5::jsonb)
	`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "workflow_form_seal_applied", "workflow_form_seal_application", applicationID, "Áp dụng con dấu công ty vào biểu mẫu quy trình", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetWorkflowFormInstanceDetail(ctx, instanceID)
}

func (r *Repository) GetWorkflowFormSealApplicationDetail(ctx context.Context, applicationID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			application.*,
			asset.name AS seal_asset_name,
			asset.status AS seal_asset_status,
			asset.storage_provider,
			asset.storage_key,
			asset.mime_type,
			asset.file_size,
			asset.width_px,
			asset.height_px,
			asset.checksum_sha256
		FROM workflow_form_seal_applications application
		JOIN seal_assets asset ON asset.id=application.seal_asset_id
		JOIN work_items item ON item.id=application.work_item_id AND item.project_id=application.project_id
		JOIN projects project ON project.id=application.project_id
		WHERE application.id=$1 AND item.status <> 'cancelled' AND project.status <> 'cancelled'
	`, applicationID)
}

func (r *Repository) SignWorkflowForm(ctx context.Context, instanceID, purpose, comment string, visualFile StoredFileMetadata, strokes []WorkflowFormSignatureStroke, actorID, actorName string) (result Record, err error) {
	tracker := storedMutationTracker{}
	defer tracker.wrap(&err)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		SELECT
			wfi.*,
			wft.code AS form_code,
			wft.name AS form_name,
			wfv.version_no AS form_version_no
		FROM workflow_form_instances wfi
		JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
		JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		WHERE wfi.id=$1
		FOR UPDATE
	`, instanceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	instance := rows[0]
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Biểu mẫu quy trình phải được gửi trước khi ký nội bộ", Record{"status": instance["status"]})
	}
	projectID := cleanID(instance["project_id"])
	workItemID := cleanID(instance["work_item_id"])
	formVersionID := cleanID(instance["form_version_id"])
	revisionNo := int(number(instance["current_revision"]))
	if workItemID == "" || projectID == "" || formVersionID == "" {
		return nil, validationError("Biểu mẫu quy trình thiếu liên kết để ký", Record{"workflow_form_instance_id": instanceID})
	}

	duplicateRows, err := queryTx(ctx, tx, `
		SELECT COUNT(*) AS total
		FROM workflow_form_signatures
		WHERE form_instance_id=$1 AND form_revision_no=$2 AND signer_user_id=$3 AND purpose=$4
	`, instanceID, revisionNo, actorID, purpose)
	if err != nil {
		return nil, err
	}
	if len(duplicateRows) > 0 && number(duplicateRows[0]["total"]) > 0 {
		return nil, validationError("Bạn đã ký biểu mẫu quy trình này cho mục đích này", Record{"purpose": purpose})
	}

	roleRows, err := queryTx(ctx, tx, `
		SELECT COALESCE(jsonb_agg(role_name ORDER BY role_name), '[]'::jsonb) AS roles
		FROM (
			SELECT DISTINCT r.display_name AS role_name
			FROM user_roles ur
			JOIN roles r ON r.id=ur.role_id
			WHERE ur.user_id=$1
		) signer_roles
	`, actorID)
	if err != nil {
		return nil, err
	}
	signerRoles := any([]any{})
	if len(roleRows) > 0 && roleRows[0]["roles"] != nil {
		signerRoles = roleRows[0]["roles"]
	}
	signerName := strings.TrimSpace(actorName)
	if signerName == "" {
		signerName = actorID
	}
	payload := Record{
		"source":                    "workflow_form_signature",
		"workflow_form_instance_id": instanceID,
		"workflow_form_version_id":  formVersionID,
		"workflow_form_revision_no": revisionNo,
		"workflow_form_type":        instance["form_type"],
		"workflow_form_code":        instance["form_code"],
		"workflow_form_title":       instance["title"],
		"answers_snapshot":          instance["answers_snapshot_json"],
		"project_id":                projectID,
		"work_item_id":              workItemID,
		"purpose":                   purpose,
		"comment":                   comment,
		"signer_user_id":            actorID,
		"signer_name":               signerName,
		"signer_roles":              signerRoles,
		"visual_signature": Record{
			"storage_provider": visualFile.StorageProvider,
			"storage_key":      visualFile.StorageKey,
			"mime_type":        visualFile.MIMEType,
			"file_size":        visualFile.FileSize,
			"checksum_sha256":  visualFile.ChecksumSHA256,
			"strokes":          strokes,
		},
	}
	rawPayload, _ := json.Marshal(payload)
	hashBytes := sha256.Sum256(rawPayload)
	payloadHash := hex.EncodeToString(hashBytes[:])
	rawRoles, _ := json.Marshal(signerRoles)

	signatureRows, err := queryTx(ctx, tx, `
		INSERT INTO workflow_form_signatures
			(project_id, work_item_id, form_instance_id, form_version_id, form_revision_no, purpose, signer_user_id, signer_name, signer_roles_json, comment, payload_json, payload_hash, visual_storage_provider, visual_storage_key, visual_mime_type, visual_file_size, visual_checksum_sha256)
		VALUES
			($1, $2, $3, $4, $5, $6, NULLIF($7,'')::uuid, $8, $9::jsonb, NULLIF($10,''), $11::jsonb, $12, $13, $14, $15, $16, $17)
		RETURNING *
	`, projectID, workItemID, instanceID, formVersionID, revisionNo, purpose, actorID, signerName, string(rawRoles), comment, string(rawPayload), payloadHash, visualFile.StorageProvider, visualFile.StorageKey, visualFile.MIMEType, visualFile.FileSize, visualFile.ChecksumSHA256)
	if err != nil {
		return nil, err
	}
	if len(signatureRows) == 0 {
		return nil, sql.ErrNoRows
	}
	signatureID := cleanID(signatureRows[0]["id"])
	rawMetadata, _ := json.Marshal(Record{
		"workflow_form_instance_id": instanceID,
		"signature_id":              signatureID,
		"purpose":                   purpose,
		"payload_hash":              payloadHash,
		"form_revision_no":          revisionNo,
		"visual_checksum_sha256":    visualFile.ChecksumSHA256,
	})
	eventComment := comment
	if eventComment == "" {
		eventComment = "Đã ký biểu mẫu quy trình"
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_form_events (instance_id, project_id, work_item_id, actor_id, event_type, from_status, to_status, comment, metadata_json)
		VALUES ($1, $2, $3, NULLIF($4,'')::uuid, 'signed', 'submitted', 'submitted', $5, $6::jsonb)
	`, instanceID, projectID, workItemID, actorID, eventComment, string(rawMetadata)); err != nil {
		return nil, err
	}
	activityComment := fmt.Sprintf("Đã ký biểu mẫu quy trình: %s", purpose)
	if comment != "" {
		activityComment = comment
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'internal', $5::jsonb)
	`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "workflow_form_signed", "workflow_form_signature", signatureID, "Ký nội bộ biểu mẫu quy trình", Record{
		"work_item_id":              workItemID,
		"workflow_form_instance_id": instanceID,
		"purpose":                   purpose,
		"payload_hash":              payloadHash,
		"form_revision_no":          revisionNo,
	}); err != nil {
		return nil, err
	}
	return finalizeStoredMutation(&tracker, func() (Record, error) {
		return getWorkflowFormInstanceDetail(ctx, tx, instanceID)
	}, tx.Commit)
}

func (r *Repository) SaveWorkflowFormPDFConfirmationPreference(ctx context.Context, instanceID, purpose string, includeSignatures, includeSeals bool, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	instance, err := queryOneRecord(ctx, tx, `
		SELECT id, status, current_revision
		FROM workflow_form_instances
		WHERE id=$1
		FOR UPDATE
	`, instanceID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Biểu mẫu quy trình phải được chốt trước khi chọn xác nhận hiển thị trong PDF", Record{"status": instance["status"]})
	}
	availableKinds, err := queryOneRecord(ctx, tx, `
		SELECT
			EXISTS (
				SELECT 1
				FROM workflow_form_signatures
				WHERE form_instance_id=$1 AND form_revision_no=$3 AND purpose=$2
			) AS has_signatures,
			EXISTS (
				SELECT 1
				FROM workflow_form_seal_applications
				WHERE form_instance_id=$1 AND form_revision_no=$3 AND purpose=$2
			) AS has_seals
	`, instanceID, purpose, instance["current_revision"])
	if err != nil {
		return nil, err
	}
	if !boolValue(availableKinds["has_signatures"]) || !boolValue(availableKinds["has_seals"]) {
		return nil, validationError("Chỉ chọn thành phần PDF khi cùng mục đích đã có cả chữ ký cá nhân và con dấu công ty", Record{
			"purpose":        purpose,
			"has_signatures": availableKinds["has_signatures"],
			"has_seals":      availableKinds["has_seals"],
		})
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_form_pdf_confirmation_preferences
			(form_instance_id, purpose, include_signatures, include_seals, updated_by, updated_by_name)
		VALUES
			($1, $2, $3, $4, NULLIF($5, '')::uuid, NULLIF($6, ''))
		ON CONFLICT (form_instance_id, purpose) DO UPDATE
		SET include_signatures=EXCLUDED.include_signatures,
			include_seals=EXCLUDED.include_seals,
			updated_by=EXCLUDED.updated_by,
			updated_by_name=EXCLUDED.updated_by_name,
			updated_at=now()
	`, instanceID, purpose, includeSignatures, includeSeals, actorID, actorName); err != nil {
		return nil, err
	}
	result, err := getWorkflowFormInstanceDetail(ctx, tx, instanceID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *Repository) CreateWorkflowFormPDFExport(ctx context.Context, instanceID, fileName, title string, pdfFile StoredFileMetadata, payloadHash string, confirmationSelection Record, actorID, actorName string) (result Record, err error) {
	tracker := storedMutationTracker{}
	defer tracker.wrap(&err)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		SELECT
			wfi.*,
			wft.code AS form_code,
			wft.name AS form_name,
			wfv.version_no AS form_version_no
		FROM workflow_form_instances wfi
		JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
		JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		WHERE wfi.id=$1
		FOR UPDATE
	`, instanceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	instance := rows[0]
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Biểu mẫu quy trình phải được gửi trước khi xuất PDF", Record{"status": instance["status"]})
	}
	projectID := cleanID(instance["project_id"])
	workItemID := cleanID(instance["work_item_id"])
	formVersionID := cleanID(instance["form_version_id"])
	revisionNo := int(number(instance["current_revision"]))
	if workItemID == "" || projectID == "" || formVersionID == "" {
		return nil, validationError("Biểu mẫu quy trình thiếu liên kết để xuất PDF", Record{"workflow_form_instance_id": instanceID})
	}
	if strings.TrimSpace(title) == "" {
		title = fmt.Sprint(instance["title"])
	}
	if strings.TrimSpace(fileName) == "" {
		fileName = fmt.Sprintf("workflow-form-rev-%d.pdf", revisionNo)
	}
	metadata := Record{
		"workflow_form_type":       instance["form_type"],
		"workflow_form_code":       instance["form_code"],
		"workflow_form_title":      instance["title"],
		"workflow_form_version_no": instance["form_version_no"],
		"form_revision_no":         revisionNo,
		"confirmation_selection":   confirmationSelection,
	}
	rawMetadata, _ := json.Marshal(metadata)

	exportRows, err := queryTx(ctx, tx, `
		INSERT INTO workflow_form_pdf_exports
			(project_id, work_item_id, form_instance_id, form_version_id, form_revision_no, title, file_name, storage_provider, storage_key, mime_type, file_size, checksum_sha256, payload_hash, metadata_json, generated_by, generated_by_name)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, NULLIF($15,'')::uuid, NULLIF($16,''))
		RETURNING *
	`, projectID, workItemID, instanceID, formVersionID, revisionNo, title, fileName, pdfFile.StorageProvider, pdfFile.StorageKey, pdfFile.MIMEType, pdfFile.FileSize, pdfFile.ChecksumSHA256, payloadHash, string(rawMetadata), actorID, actorName)
	if err != nil {
		return nil, err
	}
	if len(exportRows) == 0 {
		return nil, sql.ErrNoRows
	}
	exportID := cleanID(exportRows[0]["id"])
	eventMetadata := Record{
		"workflow_form_instance_id": instanceID,
		"pdf_export_id":             exportID,
		"payload_hash":              payloadHash,
		"checksum_sha256":           pdfFile.ChecksumSHA256,
		"form_revision_no":          revisionNo,
		"confirmation_selection":    confirmationSelection,
	}
	rawEventMetadata, _ := json.Marshal(eventMetadata)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_form_events (instance_id, project_id, work_item_id, actor_id, event_type, from_status, to_status, comment, metadata_json)
		VALUES ($1, $2, $3, NULLIF($4,'')::uuid, 'pdf_exported', 'submitted', 'submitted', $5, $6::jsonb)
	`, instanceID, projectID, workItemID, actorID, fmt.Sprintf("Xuất PDF biểu mẫu quy trình: %s", fileName), string(rawEventMetadata)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'internal', $5::jsonb)
	`, workItemID, projectID, actorID, fmt.Sprintf("Đã xuất PDF biểu mẫu quy trình: %s", fileName), string(rawEventMetadata)); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "workflow_form_pdf_exported", "workflow_form_pdf_export", exportID, "Xuất PDF biểu mẫu quy trình", eventMetadata); err != nil {
		return nil, err
	}
	return finalizeStoredMutation(&tracker, func() (Record, error) {
		return getWorkflowFormInstanceDetail(ctx, tx, instanceID)
	}, tx.Commit)
}

func (r *Repository) GetWorkflowFormSignatureDetail(ctx context.Context, signatureID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT wfs.*, wi.status AS work_item_status
		FROM workflow_form_signatures wfs
		JOIN work_items wi ON wi.id=wfs.work_item_id AND wi.project_id=wfs.project_id
		JOIN projects p ON p.id=wfs.project_id
		WHERE wfs.id=$1 AND wi.status <> 'cancelled' AND p.status <> 'cancelled'
	`, signatureID)
}

func (r *Repository) GetWorkflowFormPDFExportDetail(ctx context.Context, exportID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			wfpe.*,
			wi.status AS work_item_status,
			COALESCE(deletion.status, '') AS deletion_status,
			(
				SELECT COUNT(*)
				FROM acceptance_records acceptance
				WHERE acceptance.pdf_export_id=wfpe.id
			) AS acceptance_reference_count
		FROM workflow_form_pdf_exports wfpe
		JOIN work_items wi ON wi.id=wfpe.work_item_id AND wi.project_id=wfpe.project_id
		JOIN projects p ON p.id=wfpe.project_id
		LEFT JOIN workflow_form_pdf_export_deletions deletion ON deletion.pdf_export_id=wfpe.id
		WHERE wfpe.id=$1
		  AND wi.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		  AND COALESCE(deletion.status, '') NOT IN ('pending', 'deleted')
	`, exportID)
}

func (r *Repository) GetWorkflowFormPDFExportForDeletion(ctx context.Context, exportID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			wfpe.*,
			wi.status AS work_item_status,
			COALESCE(deletion.status, '') AS deletion_status,
			(
				SELECT COUNT(*)
				FROM acceptance_records acceptance
				WHERE acceptance.pdf_export_id=wfpe.id
			) AS acceptance_reference_count
		FROM workflow_form_pdf_exports wfpe
		JOIN work_items wi ON wi.id=wfpe.work_item_id AND wi.project_id=wfpe.project_id
		JOIN projects p ON p.id=wfpe.project_id
		LEFT JOIN workflow_form_pdf_export_deletions deletion ON deletion.pdf_export_id=wfpe.id
		WHERE wfpe.id=$1 AND wi.status <> 'cancelled' AND p.status <> 'cancelled'
	`, exportID)
}

func (r *Repository) PrepareWorkflowFormPDFExportDeletion(ctx context.Context, exportID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	exportRows, err := queryTx(ctx, tx, `
		SELECT wfpe.*, wfi.current_revision
		FROM workflow_form_pdf_exports wfpe
		JOIN workflow_form_instances wfi ON wfi.id=wfpe.form_instance_id
		WHERE wfpe.id=$1
		FOR UPDATE OF wfpe, wfi
	`, exportID)
	if err != nil {
		return nil, err
	}
	if len(exportRows) == 0 {
		return nil, sql.ErrNoRows
	}
	export := exportRows[0]
	if int(number(export["form_revision_no"])) != int(number(export["current_revision"])) {
		return nil, conflictError("PDF của phiên bản trước được giữ bất biến trong lịch sử hồ sơ", Record{
			"pdf_export_id":       exportID,
			"form_revision_no":    export["form_revision_no"],
			"current_revision_no": export["current_revision"],
		})
	}

	var referenceCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM acceptance_records
		WHERE pdf_export_id=$1
	`, exportID).Scan(&referenceCount); err != nil {
		return nil, err
	}
	if referenceCount > 0 {
		return nil, conflictError("Không thể xoá PDF đã được liên kết với hồ sơ nghiệm thu đầu ra", Record{
			"pdf_export_id":              exportID,
			"acceptance_reference_count": referenceCount,
		})
	}

	deletionRows, err := queryTx(ctx, tx, `
		SELECT status
		FROM workflow_form_pdf_export_deletions
		WHERE pdf_export_id=$1
		FOR UPDATE
	`, exportID)
	if err != nil {
		return nil, err
	}
	if len(deletionRows) > 0 && fmt.Sprint(deletionRows[0]["status"]) == "deleted" {
		export["deletion_status"] = "deleted"
		export["already_deleted"] = true
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return export, nil
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_form_pdf_export_deletions
			(pdf_export_id, status, requested_by, requested_by_name, requested_at, attempt_count, last_error, storage_deleted_at, completed_at, updated_at)
		VALUES
			($1, 'pending', NULLIF($2,'')::uuid, NULLIF($3,''), now(), 1, NULL, NULL, NULL, now())
		ON CONFLICT (pdf_export_id) DO UPDATE
		SET status='pending',
			requested_by=EXCLUDED.requested_by,
			requested_by_name=EXCLUDED.requested_by_name,
			requested_at=now(),
			attempt_count=workflow_form_pdf_export_deletions.attempt_count + 1,
			last_error=NULL,
			storage_deleted_at=NULL,
			completed_at=NULL,
			updated_at=now()
	`, exportID, actorID, actorName); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	export["deletion_status"] = "pending"
	export["acceptance_reference_count"] = referenceCount
	return export, nil
}

func (r *Repository) MarkWorkflowFormPDFExportDeletionFailed(ctx context.Context, exportID, errorMessage string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE workflow_form_pdf_export_deletions
		SET status='failed', last_error=NULLIF($2,''), completed_at=NULL, updated_at=now()
		WHERE pdf_export_id=$1 AND status <> 'deleted'
	`, exportID, errorMessage)
	return err
}

func (r *Repository) CompleteWorkflowFormPDFExportDeletion(ctx context.Context, exportID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	exportRows, err := queryTx(ctx, tx, `
		SELECT wfpe.*
		FROM workflow_form_pdf_exports wfpe
		WHERE wfpe.id=$1
		FOR UPDATE
	`, exportID)
	if err != nil {
		return nil, err
	}
	if len(exportRows) == 0 {
		return nil, sql.ErrNoRows
	}
	export := exportRows[0]

	completion, err := tx.ExecContext(ctx, `
		UPDATE workflow_form_pdf_export_deletions
		SET status='deleted',
			last_error=NULL,
			storage_deleted_at=COALESCE(storage_deleted_at, now()),
			completed_at=COALESCE(completed_at, now()),
			updated_at=now()
		WHERE pdf_export_id=$1 AND status <> 'deleted'
	`, exportID)
	if err != nil {
		return nil, err
	}
	rowsAffected, err := completion.RowsAffected()
	if err != nil {
		return nil, err
	}

	instanceID := cleanID(export["form_instance_id"])
	if rowsAffected > 0 {
		projectID := cleanID(export["project_id"])
		workItemID := cleanID(export["work_item_id"])
		fileName := strings.TrimSpace(fmt.Sprint(export["file_name"]))
		eventMetadata := Record{
			"workflow_form_instance_id": instanceID,
			"pdf_export_id":             exportID,
			"file_name":                 fileName,
			"checksum_sha256":           export["checksum_sha256"],
			"form_revision_no":          export["form_revision_no"],
		}
		rawEventMetadata, _ := json.Marshal(eventMetadata)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workflow_form_events (instance_id, project_id, work_item_id, actor_id, event_type, from_status, to_status, comment, metadata_json)
			VALUES ($1, $2, $3, NULLIF($4,'')::uuid, 'pdf_export_deleted', 'submitted', 'submitted', $5, $6::jsonb)
		`, instanceID, projectID, workItemID, actorID, fmt.Sprintf("Xoá PDF biểu mẫu quy trình: %s", fileName), string(rawEventMetadata)); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'internal', $5::jsonb)
		`, workItemID, projectID, actorID, fmt.Sprintf("Đã xoá PDF biểu mẫu quy trình: %s", fileName), string(rawEventMetadata)); err != nil {
			return nil, err
		}
		if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "workflow_form_pdf_export_deleted", "workflow_form_pdf_export", exportID, "Xoá PDF biểu mẫu quy trình", eventMetadata); err != nil {
			return nil, err
		}
	}

	detail, err := getWorkflowFormInstanceDetail(ctx, tx, instanceID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return detail, nil
}

func createMeetingMinutesActionItemsTx(ctx context.Context, tx *sql.Tx, instance Record, answers Record, actorID string) ([]string, error) {
	rawActionItems, ok := answers["action_items"]
	if !ok || workflowFormValueEmpty(Record{"field_type": "structured_list"}, rawActionItems) {
		return []string{}, nil
	}
	actionItems, err := workflowRecordList(rawActionItems)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(instance["project_id"])
	instanceID := cleanID(instance["id"])
	parentWorkItemID := cleanID(instance["work_item_id"])
	meetingTitle := strings.TrimSpace(fmt.Sprint(answers["meeting_title"]))
	createdIDs := make([]string, 0, len(actionItems))
	for index, item := range actionItems {
		title := strings.TrimSpace(fmt.Sprint(item["title"]))
		if title == "" || title == "<nil>" {
			continue
		}
		ownerRole := strings.TrimSpace(fmt.Sprint(item["owner_role"]))
		ownerName := strings.TrimSpace(fmt.Sprint(item["owner_name"]))
		dueDate := strings.TrimSpace(fmt.Sprint(item["due_date"]))
		priority := strings.TrimSpace(fmt.Sprint(item["priority"]))
		if !statusIn(priority, "low", "normal", "high", "urgent") {
			priority = "normal"
		}
		descriptionParts := []string{}
		if meetingTitle != "" && meetingTitle != "<nil>" {
			descriptionParts = append(descriptionParts, "Biên bản họp: "+meetingTitle)
		}
		if ownerName != "" && ownerName != "<nil>" {
			descriptionParts = append(descriptionParts, "Owner: "+ownerName)
		}
		if note := strings.TrimSpace(fmt.Sprint(item["description"])); note != "" && note != "<nil>" {
			descriptionParts = append(descriptionParts, note)
		}
		description := strings.Join(descriptionParts, "\n")
		roleIDs := []string{}
		if ownerRole != "" && ownerRole != "<nil>" {
			roleIDs = append(roleIDs, ownerRole)
		}
		rawRoles, _ := json.Marshal(roleIDs)
		rawMetadata, _ := json.Marshal(Record{
			"source":                    "meeting_minutes",
			"workflow_form_instance_id": instanceID,
			"parent_work_item_id":       parentWorkItemID,
			"action_item_index":         index,
			"action_item":               item,
		})
		workItemID := newID()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_items
				(id, project_id, source_type, source_id, title, description, work_type, status, priority, role_ids, due_at, evidence_requirement_json, created_by, updated_by)
			VALUES
				($1, $2, 'workflow_form_action_item', $3, $4, NULLIF($5,''), 'task', 'ready', $6, $7::jsonb, NULLIF($8,'')::timestamptz, '{}'::jsonb, NULLIF($9,'')::uuid, NULLIF($9,'')::uuid)
		`, workItemID, projectID, instanceID, title, description, priority, string(rawRoles), dueDate, actorID); err != nil {
			return nil, err
		}
		if ownerRole != "" && ownerRole != "<nil>" {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO work_item_assignments (work_item_id, assignment_type, role_name, created_by)
				VALUES ($1, 'role', $2, NULLIF($3,'')::uuid)
			`, workItemID, ownerRole, actorID); err != nil {
				return nil, err
			}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, to_status, comment, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'created', 'ready', 'Đầu việc được sinh từ biên bản họp', $4::jsonb)
		`, workItemID, projectID, actorID, string(rawMetadata)); err != nil {
			return nil, err
		}
		createdIDs = append(createdIDs, workItemID)
	}
	if len(createdIDs) > 0 && parentWorkItemID != "" {
		rawMetadata, _ := json.Marshal(Record{
			"source":                    "meeting_minutes",
			"workflow_form_instance_id": instanceID,
			"generated_work_item_ids":   createdIDs,
		})
		comment := fmt.Sprintf("Biên bản họp đã sinh %d đầu việc", len(createdIDs))
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, $5::jsonb)
		`, parentWorkItemID, projectID, actorID, comment, string(rawMetadata)); err != nil {
			return nil, err
		}
	}
	return createdIDs, nil
}

func createDailyReportNextItemsTx(ctx context.Context, tx *sql.Tx, instance Record, answers Record, actorID string) ([]string, error) {
	rawNextItems, ok := answers["next_plan_items"]
	if !ok || workflowFormValueEmpty(Record{"field_type": "structured_list"}, rawNextItems) {
		return []string{}, nil
	}
	nextItems, err := workflowRecordList(rawNextItems)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(instance["project_id"])
	instanceID := cleanID(instance["id"])
	parentWorkItemID := cleanID(instance["work_item_id"])
	reportDate := strings.TrimSpace(fmt.Sprint(answers["report_date"]))
	reportedBy := strings.TrimSpace(fmt.Sprint(answers["reported_by"]))
	reportSource, reportLabel, scopeKey, scopeValue := dailyReportNextItemContext(instance, answers)
	createdIDs := make([]string, 0, len(nextItems))
	for index, item := range nextItems {
		title := strings.TrimSpace(fmt.Sprint(item["title"]))
		if title == "" || title == "<nil>" {
			continue
		}
		ownerRole := strings.TrimSpace(fmt.Sprint(item["owner_role"]))
		ownerName := strings.TrimSpace(fmt.Sprint(item["owner_name"]))
		dueDate := strings.TrimSpace(fmt.Sprint(item["due_date"]))
		priority := strings.TrimSpace(fmt.Sprint(item["priority"]))
		if !statusIn(priority, "low", "normal", "high", "urgent") {
			priority = "normal"
		}
		descriptionParts := []string{}
		if reportDate != "" && reportDate != "<nil>" {
			descriptionParts = append(descriptionParts, reportLabel+": "+reportDate)
		}
		if scopeValue != "" && scopeValue != "<nil>" {
			descriptionParts = append(descriptionParts, "Phạm vi: "+scopeValue)
		}
		if reportedBy != "" && reportedBy != "<nil>" {
			descriptionParts = append(descriptionParts, "Người báo cáo: "+reportedBy)
		}
		if ownerName != "" && ownerName != "<nil>" {
			descriptionParts = append(descriptionParts, "Người phụ trách: "+ownerName)
		}
		if note := strings.TrimSpace(fmt.Sprint(item["description"])); note != "" && note != "<nil>" {
			descriptionParts = append(descriptionParts, note)
		}
		description := strings.Join(descriptionParts, "\n")
		roleIDs := []string{}
		if ownerRole != "" && ownerRole != "<nil>" {
			roleIDs = append(roleIDs, ownerRole)
		}
		rawRoles, _ := json.Marshal(roleIDs)
		metadata := Record{
			"source":                    reportSource,
			"workflow_form_instance_id": instanceID,
			"parent_work_item_id":       parentWorkItemID,
			"next_plan_item_index":      index,
			"next_plan_item":            item,
			"report_date":               reportDate,
		}
		if scopeKey != "" {
			metadata[scopeKey] = scopeValue
		}
		rawMetadata, _ := json.Marshal(metadata)
		workItemID := newID()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_items
				(id, project_id, source_type, source_id, title, description, work_type, status, priority, role_ids, due_at, evidence_requirement_json, created_by, updated_by)
			VALUES
				($1, $2, 'workflow_form_action_item', $3, $4, NULLIF($5,''), 'task', 'ready', $6, $7::jsonb, NULLIF($8,'')::timestamptz, '{}'::jsonb, NULLIF($9,'')::uuid, NULLIF($9,'')::uuid)
		`, workItemID, projectID, instanceID, title, description, priority, string(rawRoles), dueDate, actorID); err != nil {
			return nil, err
		}
		if ownerRole != "" && ownerRole != "<nil>" {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO work_item_assignments (work_item_id, assignment_type, role_name, created_by)
				VALUES ($1, 'role', $2, NULLIF($3,'')::uuid)
			`, workItemID, ownerRole, actorID); err != nil {
				return nil, err
			}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, to_status, comment, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'created', 'ready', $4, $5::jsonb)
		`, workItemID, projectID, actorID, "Đầu việc kế hoạch tiếp theo được sinh từ "+strings.ToLower(reportLabel), string(rawMetadata)); err != nil {
			return nil, err
		}
		createdIDs = append(createdIDs, workItemID)
	}
	if len(createdIDs) > 0 && parentWorkItemID != "" {
		rawMetadata, _ := json.Marshal(Record{
			"source":                    reportSource,
			"workflow_form_instance_id": instanceID,
			"generated_work_item_ids":   createdIDs,
		})
		comment := fmt.Sprintf("%s đã sinh %d đầu việc kế hoạch tiếp theo", reportLabel, len(createdIDs))
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, $5::jsonb)
		`, parentWorkItemID, projectID, actorID, comment, string(rawMetadata)); err != nil {
			return nil, err
		}
	}
	return createdIDs, nil
}

func dailyReportNextItemContext(instance Record, answers Record) (string, string, string, string) {
	formCode := strings.TrimSpace(fmt.Sprint(instance["form_code"]))
	if formCode == "builtin_construction_daily_report" {
		return "construction_daily_report", "Nhật ký thi công", "construction_scope", strings.TrimSpace(fmt.Sprint(answers["construction_scope"]))
	}
	if rawScope, ok := answers["construction_scope"]; ok {
		return "construction_daily_report", "Nhật ký thi công", "construction_scope", strings.TrimSpace(fmt.Sprint(rawScope))
	}
	return "planning_daily_report", "Nhật ký kế hoạch", "planning_scope", strings.TrimSpace(fmt.Sprint(answers["planning_scope"]))
}
