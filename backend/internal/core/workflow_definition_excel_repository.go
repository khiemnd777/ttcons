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

func (r *Repository) WorkflowDefinitionCatalog(ctx context.Context) (workflowDefinitionCatalog, error) {
	permissions, err := r.Query(ctx, `SELECT key,name,module FROM permissions ORDER BY key`)
	if err != nil {
		return workflowDefinitionCatalog{}, err
	}
	roles, err := r.Query(ctx, `SELECT name,display_name,external_only FROM roles ORDER BY name`)
	if err != nil {
		return workflowDefinitionCatalog{}, err
	}
	forms, err := r.Query(ctx, `
		SELECT template.id::text AS template_id,template.code,template.name,template.status AS template_status,
			COUNT(version.id) FILTER (WHERE version.status='draft')::int AS draft_count,
			COALESCE(MAX(version.version_no),0)::int AS max_version_no
		FROM workflow_form_templates template
		LEFT JOIN workflow_form_versions version ON version.template_id=template.id
		GROUP BY template.id
		ORDER BY template.code
	`)
	if err != nil {
		return workflowDefinitionCatalog{}, err
	}
	formVersions, err := r.Query(ctx, `
		SELECT version.id::text AS version_id,template.id::text AS template_id,template.code,template.name,
			template.status AS template_status,version.version_no,version.status AS version_status,
			COALESCE(version.definition_hash,'') AS definition_hash,COUNT(field.id)::int AS field_count,
			COALESCE(jsonb_agg(field.field_key ORDER BY field.order_index,field.created_at,field.id)
				FILTER (WHERE field.id IS NOT NULL),'[]'::jsonb) AS field_keys
		FROM workflow_form_versions version
		JOIN workflow_form_templates template ON template.id=version.template_id
		LEFT JOIN workflow_form_fields field ON field.form_version_id=version.id
		GROUP BY version.id,template.id
		HAVING COUNT(field.id)>0
		ORDER BY template.code,version.version_no
	`)
	if err != nil {
		return workflowDefinitionCatalog{}, err
	}
	families, err := r.Query(ctx, `
		SELECT COALESCE(template_family_id,id)::text AS family_id,
			(MAX(name) FILTER (WHERE revision_no=(SELECT MAX(inner_template.revision_no) FROM pipeline_templates inner_template WHERE COALESCE(inner_template.template_family_id,inner_template.id)=COALESCE(template.template_family_id,template.id) AND inner_template.status <> 'deleted'))) AS name,
			COUNT(*) FILTER (WHERE status='draft')::int AS draft_count,
			MAX(revision_no)::int AS max_revision_no
		FROM pipeline_templates template
		WHERE status <> 'deleted'
		GROUP BY COALESCE(template_family_id,id)
		ORDER BY family_id
	`)
	if err != nil {
		return workflowDefinitionCatalog{}, err
	}
	catalog := workflowDefinitionCatalog{
		Permissions: permissions, Roles: roles, Forms: formVersions, PipelineFamilies: families,
		PermissionByKey: map[string]Record{}, RoleByName: map[string]Record{}, RoleNameByDisplayName: map[string]string{},
		FormByCode: map[string]Record{}, PublishedFormByRef: map[string]Record{}, PipelineFamilyByID: map[string]Record{},
	}
	for _, row := range permissions {
		catalog.PermissionByKey[workflowDefinitionText(row["key"])] = row
	}
	for _, row := range roles {
		name := strings.ToLower(workflowDefinitionText(row["name"]))
		catalog.RoleByName[name] = row
		catalog.RoleNameByDisplayName[strings.ToLower(workflowDefinitionText(row["display_name"]))] = workflowDefinitionText(row["name"])
	}
	for _, row := range forms {
		catalog.FormByCode[strings.ToLower(workflowDefinitionText(row["code"]))] = row
	}
	for _, row := range formVersions {
		if workflowDefinitionText(row["template_status"]) != "active" || workflowDefinitionText(row["version_status"]) != "published" {
			continue
		}
		ref := fmt.Sprintf("%s@v%d", workflowDefinitionText(row["code"]), workflowDefinitionInt(row["version_no"], 0))
		row["form_ref"] = ref
		catalog.PublishedFormByRef[strings.ToLower(ref)] = row
	}
	for _, row := range families {
		catalog.PipelineFamilyByID[cleanID(row["family_id"])] = row
	}
	raw, err := json.Marshal(Record{"permissions": permissions, "roles": roles, "forms": forms, "form_versions": formVersions, "pipeline_families": families})
	if err != nil {
		return workflowDefinitionCatalog{}, err
	}
	sum := sha256.Sum256(raw)
	catalog.Fingerprint = hex.EncodeToString(sum[:])
	return catalog, nil
}

func (r *Repository) WorkflowDefinitionImportResult(ctx context.Context, actorID, idempotencyKey, previewHash string) (Record, bool, error) {
	var storedActorID sql.NullString
	var storedHash string
	var sourceFormat string
	var storedJSON []byte
	err := r.db.QueryRowContext(ctx, `SELECT actor_id::text,preview_hash,source_format,result_json FROM workflow_definition_imports WHERE idempotency_key=$1`, idempotencyKey).Scan(&storedActorID, &storedHash, &sourceFormat, &storedJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !storedActorID.Valid || storedActorID.String != actorID || previewHash == "" || storedHash != previewHash {
		return nil, false, conflictError("Mã chống tạo trùng đã được dùng cho một gói workflow khác", Record{"field": "idempotency_key"})
	}
	result := Record{}
	if err := json.Unmarshal(storedJSON, &result); err != nil {
		return nil, false, err
	}
	result["source_format"] = sourceFormat
	return result, true, nil
}

func (r *Repository) ExecuteWorkflowDefinitionImport(ctx context.Context, actorID, actorName, idempotencyKey, sourceFormat, sourceFilename string, sourceSizeBytes int64, previewHash, referenceFingerprint string, packageData workflowDefinitionPackage) (result Record, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, idempotencyKey); err != nil {
		return nil, err
	}
	var storedActorID sql.NullString
	var storedHash string
	var storedJSON []byte
	if err := tx.QueryRowContext(ctx, `SELECT actor_id::text,preview_hash,result_json FROM workflow_definition_imports WHERE idempotency_key=$1`, idempotencyKey).Scan(&storedActorID, &storedHash, &storedJSON); err == nil {
		if !storedActorID.Valid || storedActorID.String != actorID || storedHash != previewHash {
			return nil, conflictError("Mã chống tạo trùng đã được dùng cho một gói workflow khác", Record{"field": "idempotency_key"})
		}
		if err := json.Unmarshal(storedJSON, &result); err != nil {
			return nil, err
		}
		result["idempotent_replay"] = true
		return result, tx.Commit()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	createdForms := []Record{}
	formVersionIDs := map[string]string{}
	for _, form := range packageData.Forms {
		code := workflowDefinitionText(form["template_code"])
		templateID := cleanID(form["target_template_id"])
		if templateID == "" {
			if err := tx.QueryRowContext(ctx, `
				INSERT INTO workflow_form_templates (code,name,description,form_type,status,created_by)
				VALUES ($1,$2,$3,$4,'active',NULLIF($5,'')::uuid)
				RETURNING id::text
			`, code, form["name"], form["description"], form["form_type"], actorID).Scan(&templateID); err != nil {
				return nil, err
			}
		} else {
			var status string
			if err := tx.QueryRowContext(ctx, `SELECT status FROM workflow_form_templates WHERE id=$1 AND lower(code)=lower($2) FOR UPDATE`, templateID, code).Scan(&status); err != nil {
				return nil, conflictError("Form đích đã thay đổi sau bước xem trước", Record{"form_key": form["form_key"]})
			}
			if status != "active" {
				return nil, conflictError("Form đích không còn active", Record{"form_key": form["form_key"]})
			}
		}
		var draftCount, nextVersionNo int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FILTER (WHERE status='draft')::int,COALESCE(MAX(version_no),0)::int+1 FROM workflow_form_versions WHERE template_id=$1`, templateID).Scan(&draftCount, &nextVersionNo); err != nil {
			return nil, err
		}
		if draftCount > 0 {
			return nil, conflictError("Form đích đã có version nháp; import không ghi đè", Record{"form_key": form["form_key"]})
		}
		schemaRaw, _ := json.Marshal(defaultWorkflowDefinitionJSON(form["schema_json"], Record{}))
		validationRaw, _ := json.Marshal(defaultWorkflowDefinitionJSON(form["validation_json"], Record{}))
		versionID := newID()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workflow_form_versions (id,template_id,version_no,status,schema_json,validation_json,definition_hash,created_by)
			VALUES ($1,$2,$3,'draft',$4::jsonb,$5::jsonb,$6,NULLIF($7,'')::uuid)
		`, versionID, templateID, nextVersionNo, string(schemaRaw), string(validationRaw), form["definition_hash"], actorID); err != nil {
			return nil, err
		}
		if err := insertWorkflowFormFieldsTx(ctx, tx, versionID, workflowFieldRecords(form["fields"])); err != nil {
			return nil, err
		}
		if err := addAuditTx(ctx, tx, "", actorID, actorName, "workflow_form_definition_imported", "workflow_form_version", versionID, fmt.Sprintf("Import nháp biểu mẫu quy trình %s v%d", form["name"], nextVersionNo), Record{
			"import_idempotency_key": idempotencyKey, "template_id": templateID, "form_key": form["form_key"], "definition_hash": form["definition_hash"],
		}); err != nil {
			return nil, err
		}
		formKey := workflowDefinitionText(form["form_key"])
		formVersionIDs[formKey] = versionID
		createdForms = append(createdForms, Record{"form_key": formKey, "template_id": templateID, "version_id": versionID, "version_no": nextVersionNo, "status": "draft", "definition_hash": form["definition_hash"]})
	}

	createdPipelines := []Record{}
	for _, pipeline := range packageData.Pipelines {
		targetFamilyID := cleanID(pipeline["target_family_id"])
		templateID := newID()
		revisionNo := 1
		if targetFamilyID == "" {
			targetFamilyID = templateID
		} else {
			familyRows, err := queryTx(ctx, tx, `SELECT id FROM pipeline_templates WHERE COALESCE(template_family_id,id)=$1 AND status <> 'deleted' ORDER BY revision_no LIMIT 1 FOR UPDATE`, targetFamilyID)
			if err != nil {
				return nil, err
			}
			if len(familyRows) == 0 {
				return nil, conflictError("Family Pipeline đích đã thay đổi sau bước xem trước", Record{"pipeline_key": pipeline["pipeline_key"]})
			}
			var draftCount int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FILTER (WHERE status='draft')::int,COALESCE(MAX(revision_no),0)::int+1 FROM pipeline_templates WHERE COALESCE(template_family_id,id)=$1 AND status <> 'deleted'`, targetFamilyID).Scan(&draftCount, &revisionNo); err != nil {
				return nil, err
			}
			if draftCount > 0 {
				return nil, conflictError("Family Pipeline đích đã có revision nháp; import không ghi đè", Record{"pipeline_key": pipeline["pipeline_key"]})
			}
		}
		body := Record{
			"id": templateID, "name": pipeline["name"], "description": pipeline["description"], "project_type": pipeline["project_type"],
			"version": revisionNo, "status": "draft", "created_by": actorID, "template_family_id": targetFamilyID,
			"revision_no": revisionNo, "engine_version": pipeline["engine_version"], "report_config_json": pipeline["report_config_json"],
		}
		if _, err := insertRecord(ctx, tx, "pipeline_templates", templateColumns(), body); err != nil {
			return nil, err
		}
		nodeIDs := map[string]string{}
		for _, node := range workflowFieldRecords(pipeline["nodes"]) {
			nodeBody := Record{}
			for key, value := range node {
				if key != "node_key" && key != "steps" {
					nodeBody[key] = value
				}
			}
			nodeBody["template_id"] = templateID
			createdNode, err := insertRecord(ctx, tx, "pipeline_template_nodes", templateNodeColumns(), nodeBody)
			if err != nil {
				return nil, err
			}
			nodeID := cleanID(createdNode["id"])
			nodeIDs[workflowDefinitionText(node["node_key"])] = nodeID
			for _, step := range workflowFieldRecords(node["steps"]) {
				stepBody := Record{}
				for key, value := range step {
					if key != "step_key" && key != "form_ref" {
						stepBody[key] = value
					}
				}
				stepBody["node_id"] = nodeID
				if importedVersionID := formVersionIDs[workflowDefinitionText(step["form_ref"])]; importedVersionID != "" {
					stepBody["form_version_id"] = importedVersionID
				}
				if _, err := insertRecord(ctx, tx, "pipeline_template_steps", templateStepColumns(), stepBody); err != nil {
					return nil, err
				}
			}
		}
		for _, edge := range workflowFieldRecords(pipeline["edges"]) {
			fromID := nodeIDs[workflowDefinitionText(edge["from_node_key"])]
			toID := nodeIDs[workflowDefinitionText(edge["to_node_key"])]
			if fromID == "" || toID == "" {
				return nil, conflictError("Connector không còn khớp Node sau bước xem trước", Record{"pipeline_key": pipeline["pipeline_key"], "edge_key": edge["edge_key"]})
			}
			if _, err := insertRecord(ctx, tx, "pipeline_template_edges", edgeColumns(), Record{
				"template_id": templateID, "from_node_id": fromID, "to_node_id": toID, "condition_type": edge["condition_type"],
				"condition_config": edge["condition_config"], "label": edge["label"], "priority": edge["priority"],
			}); err != nil {
				return nil, err
			}
		}
		if err := addAuditTx(ctx, tx, "", actorID, actorName, "pipeline_definition_imported", "pipeline_template", templateID, fmt.Sprintf("Import revision nháp Pipeline %s", pipeline["name"]), Record{
			"import_idempotency_key": idempotencyKey, "pipeline_key": pipeline["pipeline_key"], "template_family_id": targetFamilyID, "revision_no": revisionNo, "engine_version": pipeline["engine_version"],
		}); err != nil {
			return nil, err
		}
		createdPipelines = append(createdPipelines, Record{"pipeline_key": pipeline["pipeline_key"], "template_id": templateID, "template_family_id": targetFamilyID, "revision_no": revisionNo, "status": "draft", "engine_version": pipeline["engine_version"]})
	}

	importID := newID()
	result = Record{
		"import_id": importID, "source_format": sourceFormat, "scope": packageData.Scope, "pipeline_count": len(createdPipelines), "form_count": len(createdForms),
		"pipelines": createdPipelines, "forms": createdForms, "status": "draft", "idempotent_replay": false,
	}
	resultRaw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_definition_imports (
			id,idempotency_key,actor_id,scope,source_filename,source_size_bytes,workbook_version,source_format,
			preview_hash,reference_fingerprint,pipeline_count,form_count,result_json
		) VALUES ($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb)
	`, importID, idempotencyKey, actorID, packageData.Scope, sourceFilename, sourceSizeBytes, workflowDefinitionWorkbookVersion, sourceFormat,
		previewHash, referenceFingerprint, len(createdPipelines), len(createdForms), string(resultRaw)); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "workflow_definitions_imported", "workflow_definition_import", importID, "Import gói định nghĩa workflow thành bản nháp", Record{
		"scope": packageData.Scope, "source_format": sourceFormat, "pipeline_count": len(createdPipelines), "form_count": len(createdForms), "preview_hash": previewHash,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
