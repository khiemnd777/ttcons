package core

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
)

func (r *Repository) ProjectExcelCatalog(ctx context.Context) (projectExcelCatalog, error) {
	pipelines, err := r.Query(ctx, `
		WITH field_counts AS (
			SELECT form_version_id,COUNT(*)::int AS field_count
			FROM workflow_form_fields
			GROUP BY form_version_id
		)
		SELECT
			pt.id::text AS template_id,
			COALESCE(pt.template_family_id::text,'') AS template_family_id,
			pt.name,
			COALESCE(pt.project_type,'') AS project_type,
			pt.revision_no,
			pt.engine_version,
			COALESCE(pt.definition_hash,'') AS definition_hash,
			COUNT(DISTINCT node.id)::int AS node_count,
			COUNT(DISTINCT step.id)::int AS step_count,
			COUNT(DISTINCT step.id) FILTER (WHERE step.form_required)::int AS required_form_count,
			COUNT(DISTINCT step.id) FILTER (
				WHERE step.form_required AND (
					version.id IS NULL OR version.status <> 'published' OR template.status <> 'active'
				)
			)::int AS invalid_form_reference_count,
			COUNT(DISTINCT step.id) FILTER (
				WHERE step.form_required AND COALESCE(field_counts.field_count,0)=0
			)::int AS zero_field_reference_count
		FROM pipeline_templates pt
		LEFT JOIN pipeline_template_nodes node ON node.template_id=pt.id
		LEFT JOIN pipeline_template_steps step ON step.node_id=node.id
		LEFT JOIN workflow_form_versions version ON version.id=step.form_version_id
		LEFT JOIN workflow_form_templates template ON template.id=version.template_id
		LEFT JOIN field_counts ON field_counts.form_version_id=version.id
		WHERE pt.status='published'
		GROUP BY pt.id
		ORDER BY pt.name,pt.revision_no,pt.id
	`)
	if err != nil {
		return projectExcelCatalog{}, err
	}
	forms, err := r.Query(ctx, `
		SELECT
			template.id::text AS form_template_id,
			template.code AS form_template_code,
			template.name AS form_template_name,
			template.form_type,
			version.id::text AS form_version_id,
			version.version_no,
			version.status,
			COUNT(field.id)::int AS field_count
		FROM workflow_form_templates template
		JOIN workflow_form_versions version ON version.template_id=template.id AND version.status='published'
		LEFT JOIN workflow_form_fields field ON field.form_version_id=version.id
		WHERE template.status='active'
		GROUP BY template.id,version.id
		ORDER BY template.code,version.version_no,version.id
	`)
	if err != nil {
		return projectExcelCatalog{}, err
	}
	fields, err := r.Query(ctx, `
		SELECT
			template.code AS form_template_code,
			version.id::text AS form_version_id,
			version.version_no,
			field.field_key,
			field.label,
			field.field_type,
			field.required,
			field.options_json,
			field.validation_json,
			field.visibility_condition_json,
			field.order_index
		FROM workflow_form_templates template
		JOIN workflow_form_versions version ON version.template_id=template.id AND version.status='published'
		JOIN workflow_form_fields field ON field.form_version_id=version.id
		WHERE template.status='active'
		ORDER BY template.code,version.version_no,field.order_index,field.id
	`)
	if err != nil {
		return projectExcelCatalog{}, err
	}
	partners, err := r.Query(ctx, `
		SELECT id::text,partner_code,name,COALESCE(tax_code,'') AS tax_code,updated_at
		FROM partners
		WHERE active=true AND partner_types ? 'client'
		ORDER BY partner_code,id
	`)
	if err != nil {
		return projectExcelCatalog{}, err
	}

	pipelineByReference := map[string]Record{}
	readyPipelineCount := 0
	invalidPipelineCount := 0
	for _, pipeline := range pipelines {
		ready := number(pipeline["node_count"]) > 0 &&
			number(pipeline["step_count"]) > 0 &&
			number(pipeline["invalid_form_reference_count"]) == 0 &&
			number(pipeline["zero_field_reference_count"]) == 0 &&
			strings.TrimSpace(fmt.Sprint(pipeline["definition_hash"])) != ""
		pipeline["ready"] = ready
		pipeline["pipeline_ref"] = projectExcelPipelineReference(pipeline)
		if ready {
			readyPipelineCount++
			pipelineByReference[fmt.Sprint(pipeline["pipeline_ref"])] = pipeline
		} else {
			invalidPipelineCount++
		}
	}
	formTemplateIDs := map[string]bool{}
	validFormVersionCount := 0
	for _, form := range forms {
		form["form_ref"] = projectExcelFormReference(form)
		formTemplateIDs[cleanID(form["form_template_id"])] = true
		if number(form["field_count"]) > 0 {
			validFormVersionCount++
		}
	}
	for _, field := range fields {
		field["form_ref"] = projectExcelFormReference(field)
	}
	partnerByCode := map[string]Record{}
	for _, partner := range partners {
		partnerByCode[strings.ToLower(strings.TrimSpace(fmt.Sprint(partner["partner_code"])))] = partner
	}
	blockers := []Record{}
	warnings := []Record{}
	if readyPipelineCount == 0 {
		blockers = append(blockers, Record{"code": "PIPELINE_DATA_MISSING", "message": "Chưa có Pipeline published và sẵn sàng", "route": "/pipeline-builder"})
	}
	if len(forms) == 0 || validFormVersionCount == 0 || len(fields) == 0 {
		blockers = append(blockers, Record{"code": "WORKFLOW_FORM_DATA_MISSING", "message": "Chưa có Form version published kèm Fields", "route": "/workflow-forms"})
	}
	if invalidPipelineCount > 0 {
		warnings = append(warnings, Record{"code": "PIPELINE_FORM_REFERENCE_INVALID", "message": "Có Pipeline published không đủ điều kiện và đã bị loại khỏi danh mục import", "count": invalidPipelineCount, "route": "/pipeline-builder"})
	}
	readiness := Record{
		"ready":                        len(blockers) == 0,
		"published_pipeline_count":     len(pipelines),
		"ready_pipeline_count":         readyPipelineCount,
		"invalid_pipeline_count":       invalidPipelineCount,
		"active_form_template_count":   len(formTemplateIDs),
		"published_form_version_count": len(forms),
		"valid_form_version_count":     validFormVersionCount,
		"field_count":                  len(fields),
		"active_client_partner_count":  len(partners),
		"blockers":                     blockers,
		"warnings":                     warnings,
		"pipeline_builder_route":       "/pipeline-builder",
		"workflow_forms_route":         "/workflow-forms",
	}
	fingerprintPayload, err := json.Marshal(Record{"pipelines": pipelines, "forms": forms, "fields": fields, "partners": partners})
	if err != nil {
		return projectExcelCatalog{}, err
	}
	fingerprint := sha256.Sum256(fingerprintPayload)
	readiness["reference_fingerprint"] = hex.EncodeToString(fingerprint[:])
	return projectExcelCatalog{
		Pipelines: pipelines, Forms: forms, Fields: fields, Partners: partners,
		Readiness: readiness, ReferenceFingerprint: fmt.Sprint(readiness["reference_fingerprint"]),
		PipelineByReference: pipelineByReference, PartnerByCode: partnerByCode,
	}, nil
}

func (r *Repository) ExistingProjectExcelCodes(ctx context.Context, normalizedCodes []string) (map[string]bool, error) {
	result := map[string]bool{}
	if len(normalizedCodes) == 0 {
		return result, nil
	}
	placeholders := make([]string, len(normalizedCodes))
	args := make([]any, len(normalizedCodes))
	for index, code := range normalizedCodes {
		placeholders[index] = fmt.Sprintf("$%d", index+1)
		args[index] = code
	}
	rows, err := r.db.QueryContext(ctx, `SELECT lower(code) FROM projects WHERE lower(code) IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		result[code] = true
	}
	return result, rows.Err()
}

func (r *Repository) ProjectExcelImportResult(ctx context.Context, actorID, idempotencyKey, previewHash string) (Record, bool, error) {
	var storedActorID sql.NullString
	var storedHash string
	var sourceFormat string
	var storedJSON []byte
	err := r.db.QueryRowContext(ctx, `SELECT actor_id::text,preview_hash,source_format,result_json FROM project_excel_imports WHERE idempotency_key=$1`, idempotencyKey).Scan(&storedActorID, &storedHash, &sourceFormat, &storedJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !storedActorID.Valid || storedActorID.String != actorID || previewHash == "" || storedHash != previewHash {
		return nil, false, conflictError("Mã chống tạo trùng đã được dùng cho một danh sách import khác", Record{"field": "idempotency_key"})
	}
	var result Record
	if err := json.Unmarshal(storedJSON, &result); err != nil {
		return nil, false, err
	}
	result["source_format"] = sourceFormat
	return result, true, nil
}

func (r *Repository) ExecuteProjectExcelImport(ctx context.Context, actorID, actorName, idempotencyKey, sourceFormat, sourceFilename string, sourceSizeBytes int64, previewHash, referenceFingerprint string, rows []normalizedProjectExcelImportRow) (result Record, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, idempotencyKey); err != nil {
		return nil, err
	}
	var storedHash string
	var storedActorID sql.NullString
	var storedJSON []byte
	if err := tx.QueryRowContext(ctx, `SELECT actor_id::text,preview_hash,result_json FROM project_excel_imports WHERE idempotency_key=$1`, idempotencyKey).Scan(&storedActorID, &storedHash, &storedJSON); err == nil {
		if !storedActorID.Valid || storedActorID.String != actorID || storedHash != previewHash {
			return nil, conflictError("Mã chống tạo trùng đã được dùng cho một danh sách import khác", Record{"field": "idempotency_key"})
		}
		if err := json.Unmarshal(storedJSON, &result); err != nil {
			return nil, err
		}
		result["idempotent_replay"] = true
		return result, tx.Commit()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	importID := uuid.NewString()
	created := make([]Record, 0, len(rows))
	for _, row := range rows {
		var revision int
		var definitionHash string
		var pipelineReady bool
		if err := tx.QueryRowContext(ctx, `
			SELECT template.revision_no,COALESCE(template.definition_hash,''),
				EXISTS (SELECT 1 FROM pipeline_template_nodes node WHERE node.template_id=template.id)
				AND EXISTS (
					SELECT 1 FROM pipeline_template_steps step
					JOIN pipeline_template_nodes node ON node.id=step.node_id
					WHERE node.template_id=template.id
				)
				AND NOT EXISTS (
					SELECT 1
					FROM pipeline_template_steps step
					JOIN pipeline_template_nodes node ON node.id=step.node_id
					LEFT JOIN workflow_form_versions version ON version.id=step.form_version_id
					LEFT JOIN workflow_form_templates form_template ON form_template.id=version.template_id
					WHERE node.template_id=template.id AND step.form_required AND (
						version.id IS NULL OR version.status <> 'published' OR form_template.status <> 'active'
						OR NOT EXISTS (SELECT 1 FROM workflow_form_fields field WHERE field.form_version_id=version.id)
					)
				)
			FROM pipeline_templates template
			WHERE template.id=$1 AND template.status='published'
			FOR SHARE
		`, row.PipelineID).Scan(&revision, &definitionHash, &pipelineReady); err != nil {
			return nil, conflictError("Pipeline reference đã thay đổi hoặc không còn được phát hành", Record{"row_number": row.RowNumber, "field": "pipeline_ref"})
		}
		if !pipelineReady || projectExcelPipelineReferenceFromValues(row.PipelineID, revision, definitionHash) != row.PipelineRef {
			return nil, conflictError("Pipeline reference đã thay đổi sau bước xem trước", Record{"row_number": row.RowNumber, "field": "pipeline_ref"})
		}
		if row.PartnerID != "" {
			var partnerCode, partnerName string
			if err := tx.QueryRowContext(ctx, `SELECT partner_code,name FROM partners WHERE id=$1 AND active=true AND partner_types ? 'client' FOR SHARE`, row.PartnerID).Scan(&partnerCode, &partnerName); err != nil || !strings.EqualFold(partnerCode, row.PartnerCode) || partnerName != row.ClientName {
				return nil, conflictError("Chủ đầu tư đã thay đổi sau bước xem trước", Record{"row_number": row.RowNumber, "field": "client_partner_code"})
			}
		}
		var projectID string
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO projects (
				name,code,partner_id,client_name,address,planned_start_date,planned_end_date,
				estimated_budget,contract_amount,status,pipeline_template_id,progress_percent,created_by
			)
			VALUES ($1,$2,NULLIF($3,'')::uuid,$4,NULLIF($5,''),NULLIF($6,'')::date,NULLIF($7,'')::date,$8,0,'draft',$9,0,NULLIF($10,'')::uuid)
			RETURNING id::text
		`, row.ProjectName, row.ProjectCode, row.PartnerID, row.ClientName, row.Address, row.PlannedStartDate, row.PlannedEndDate, row.EstimatedBudgetVND, row.PipelineID, actorID).Scan(&projectID); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return nil, conflictError("Mã công trình đã được sử dụng trong lúc import", Record{"row_number": row.RowNumber, "field": "project_code"})
			}
			return nil, err
		}
		if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "project_imported", "project", projectID, "Tạo công trình từ gói dữ liệu", Record{
			"import_id": importID, "source_filename": strings.TrimSpace(sourceFilename), "row_number": row.RowNumber,
			"project_code": row.ProjectCode, "pipeline_ref": row.PipelineRef, "partner_code": row.PartnerCode,
			"pipeline_started": false, "source_format": sourceFormat,
		}); err != nil {
			return nil, err
		}
		created = append(created, Record{"row_number": row.RowNumber, "project_id": projectID, "project_code": row.ProjectCode, "pipeline_ref": row.PipelineRef})
	}
	result = Record{
		"import_id": importID, "source_format": sourceFormat, "source_filename": strings.TrimSpace(sourceFilename), "created_count": len(rows),
		"pipeline_started_count": 0, "items": created, "idempotent_replay": false,
	}
	storedResult := Record{
		"import_id": importID, "source_format": sourceFormat, "source_filename": strings.TrimSpace(sourceFilename), "created_count": len(rows),
		"pipeline_started_count": 0, "items": created,
	}
	resultJSON, err := json.Marshal(storedResult)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_excel_imports (
			id,idempotency_key,actor_id,source_filename,source_size_bytes,workbook_version,source_format,
			preview_hash,reference_fingerprint,row_count,created_count,result_json
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10,$11::jsonb)
	`, importID, idempotencyKey, actorID, strings.TrimSpace(sourceFilename), sourceSizeBytes, projectExcelWorkbookVersion, sourceFormat, previewHash, referenceFingerprint, len(rows), resultJSON); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "project_data_import_completed", "project_excel_import", importID, "Hoàn tất import công trình từ gói dữ liệu", Record{
		"source_filename": strings.TrimSpace(sourceFilename), "source_format": sourceFormat, "row_count": len(rows), "created_count": len(rows), "pipeline_started_count": 0,
	}); err != nil {
		return nil, err
	}
	if _, err := emitNotificationEventTx(ctx, tx, NotificationDomainEvent{
		Key: "project.setup_batch_required", AggregateType: "project_excel_import", AggregateID: importID, ActorUserID: actorID,
		Payload:  Record{"project_count": len(rows), "import_id": importID},
		Audience: Record{"user_ids": []string{actorID}}, CorrelationKey: "project_setup_batch:" + importID,
		IdempotencyKey: "project_setup_batch:" + importID,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *Repository) ProjectExcelExportRows(ctx context.Context, actorID string, req ProjectExcelExportRequest) ([]Record, error) {
	scopeSQL, args, err := r.ProjectAccessFilter(ctx, actorID, "project", 1)
	if err != nil {
		return nil, err
	}
	where := []string{"project.status <> 'cancelled'", scopeSQL}
	if req.Search != "" {
		args = append(args, "%"+escapeLikePattern(req.Search)+"%")
		placeholder := len(args)
		where = append(where, fmt.Sprintf(`(
			LOWER(project.name) LIKE LOWER($%[1]d) ESCAPE E'\\' OR
			LOWER(project.code) LIKE LOWER($%[1]d) ESCAPE E'\\' OR
			LOWER(project.client_name) LIKE LOWER($%[1]d) ESCAPE E'\\' OR
			LOWER(COALESCE(s.full_name,'')) LIKE LOWER($%[1]d) ESCAPE E'\\'
		)`, placeholder))
	}
	if req.Status != "" {
		args = append(args, req.Status)
		where = append(where, fmt.Sprintf("project.status=$%d", len(args)))
	}
	if req.Scope == "selected" {
		placeholders := make([]string, 0, len(req.ProjectIDs))
		for _, projectID := range req.ProjectIDs {
			args = append(args, projectID)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		where = append(where, "project.id::text IN ("+strings.Join(placeholders, ",")+")")
	}
	explorerRequest := ProjectExplorerRequest{Sort: req.Sort, Direction: req.Direction}
	orderSQL := projectExplorerOrderSQLForAlias(explorerRequest, "project", "s")
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT
			project.id::text AS project_id,
			project.code AS project_code,
			project.name AS project_name,
			COALESCE(partner.partner_code,'') AS client_partner_code,
			CASE WHEN partner.id IS NULL THEN project.client_name ELSE '' END AS client_name_unregistered,
			project.client_name,
			COALESCE(project.address,'') AS address,
			project.planned_start_date,
			project.planned_end_date,
			project.estimated_budget AS estimated_budget_vnd,
			project.contract_amount,
			project.status,
			project.progress_percent,
			COALESCE(project.pm_staff_id::text,'') AS pm_staff_id,
			COALESCE(s.full_name,'') AS pm_name,
			project.created_at,
			project.updated_at,
			COALESCE(selected_template.id::text,'') AS selected_template_id,
			COALESCE(selected_template.revision_no,0) AS selected_template_revision,
			COALESCE(selected_template.definition_hash,'') AS selected_template_definition_hash,
			COALESCE(selected_template.name,'') AS selected_template_name,
			COALESCE(run.id::text,'') AS runtime_id,
			COALESCE(run_template.id::text,'') AS runtime_template_id,
			COALESCE(run_template.revision_no,0) AS runtime_template_revision,
			COALESCE(run.template_definition_hash,run_template.definition_hash,'') AS runtime_template_definition_hash,
			COALESCE(run_template.name,'') AS runtime_template_name,
			COALESCE(run.engine_version,0) AS runtime_engine_version
		FROM projects project
		LEFT JOIN staff s ON s.id=project.pm_staff_id
		LEFT JOIN partners partner ON partner.id=project.partner_id
		LEFT JOIN pipeline_templates selected_template ON selected_template.id=project.pipeline_template_id
		LEFT JOIN pipeline_runs run ON run.id=project.current_pipeline_run_id
		LEFT JOIN pipeline_templates run_template ON run_template.id=run.template_id
		WHERE %s
		ORDER BY %s
		LIMIT %d
	`, strings.Join(where, " AND "), orderSQL, projectExcelMaximumExportRows+1), args...)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		selectedRef := projectExcelPipelineReferenceFromValues(cleanID(row["selected_template_id"]), int(number(row["selected_template_revision"])), strings.TrimSpace(fmt.Sprint(row["selected_template_definition_hash"])))
		runtimeRef := projectExcelPipelineReferenceFromValues(cleanID(row["runtime_template_id"]), int(number(row["runtime_template_revision"])), strings.TrimSpace(fmt.Sprint(row["runtime_template_definition_hash"])))
		row["selected_pipeline_ref"] = selectedRef
		row["runtime_pipeline_ref"] = runtimeRef
		row["pipeline_ref"] = selectedRef
		if selectedRef == "" {
			row["pipeline_ref"] = runtimeRef
		}
		switch {
		case selectedRef == "" && runtimeRef == "":
			row["reference_state"] = "missing"
		case selectedRef != "" && runtimeRef != "" && selectedRef != runtimeRef:
			row["reference_state"] = "runtime_mismatch"
		case selectedRef == "" && runtimeRef != "":
			row["reference_state"] = "runtime_only"
		default:
			row["reference_state"] = "ready"
		}
	}
	return rows, nil
}

func projectExplorerOrderSQLForAlias(request ProjectExplorerRequest, projectAlias, staffAlias string) string {
	direction := "ASC"
	if request.Direction == "desc" {
		direction = "DESC"
	}
	tieBreak := fmt.Sprintf("LOWER(%s.code) ASC,LOWER(%s.name) ASC,%s.id ASC", projectAlias, projectAlias, projectAlias)
	switch request.Sort {
	case "project":
		return fmt.Sprintf("LOWER(%s.code) %s,LOWER(%s.name) %s,%s.id ASC", projectAlias, direction, projectAlias, direction, projectAlias)
	case "status":
		return fmt.Sprintf("LOWER(%s) %s,%s", projectStatusLabelSQL(projectAlias+".status"), direction, tieBreak)
	case "owner":
		return fmt.Sprintf("(NULLIF(BTRIM(%s.full_name),'') IS NULL) ASC,LOWER(%s.full_name) %s,%s", staffAlias, staffAlias, direction, tieBreak)
	default:
		return fmt.Sprintf("%s.created_at %s,%s", projectAlias, direction, tieBreak)
	}
}

func projectExcelPipelineReference(row Record) string {
	return projectExcelPipelineReferenceFromValues(cleanID(row["template_id"]), int(number(row["revision_no"])), strings.TrimSpace(fmt.Sprint(row["definition_hash"])))
}

func projectExcelPipelineReferenceFromValues(templateID string, revision int, definitionHash string) string {
	if templateID == "" || revision < 1 || definitionHash == "" {
		return ""
	}
	return fmt.Sprintf("%s@r%d#%s", templateID, revision, definitionHash)
}

func projectExcelFormReference(row Record) string {
	code := strings.TrimSpace(fmt.Sprint(row["form_template_code"]))
	versionID := cleanID(row["form_version_id"])
	version := int(number(row["version_no"]))
	if code == "" || versionID == "" || version < 1 {
		return ""
	}
	return fmt.Sprintf("%s@v%d#%s", code, version, versionID)
}
