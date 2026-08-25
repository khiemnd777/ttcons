package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (r *Repository) ListPublishedSampleDataBlueprints(ctx context.Context) ([]sampleDataBlueprint, error) {
	templates, err := r.Query(ctx, `
		SELECT id::text
		FROM pipeline_templates
		WHERE status='published'
		ORDER BY lower(name),revision_no DESC,id
	`)
	if err != nil {
		return nil, err
	}
	blueprints := make([]sampleDataBlueprint, 0, len(templates))
	for _, template := range templates {
		blueprint, err := r.LoadSampleDataBlueprint(ctx, cleanID(template["id"]))
		if err != nil {
			return nil, err
		}
		blueprints = append(blueprints, blueprint)
	}
	return blueprints, nil
}

func (r *Repository) LoadSampleDataBlueprint(ctx context.Context, templateID string) (sampleDataBlueprint, error) {
	template, err := r.QueryOne(ctx, `
		SELECT id::text,name,status,revision_no,engine_version,definition_hash,template_family_id::text
		FROM pipeline_templates
		WHERE id=$1
	`, templateID)
	if err != nil {
		return sampleDataBlueprint{}, err
	}
	nodeRows, err := r.Query(ctx, `
		SELECT * FROM pipeline_template_nodes
		WHERE template_id=$1 AND status='active'
		ORDER BY order_index,created_at,id
	`, templateID)
	if err != nil {
		return sampleDataBlueprint{}, err
	}
	edges, err := r.Query(ctx, `
		SELECT * FROM pipeline_template_edges
		WHERE template_id=$1
		ORDER BY priority,created_at,id
	`, templateID)
	if err != nil {
		return sampleDataBlueprint{}, err
	}
	blueprint := sampleDataBlueprint{Template: template, Edges: edges, Actors: map[string][]sampleDataStaffActor{}}
	for _, nodeRow := range nodeRows {
		steps, err := r.Query(ctx, `
			SELECT step.*,COALESCE(version.status,'') AS form_version_status
			FROM pipeline_template_steps step
			LEFT JOIN workflow_form_versions version ON version.id=step.form_version_id
			WHERE step.node_id=$1
			ORDER BY step.order_index,step.created_at,step.id
		`, nodeRow["id"])
		if err != nil {
			return sampleDataBlueprint{}, err
		}
		node := sampleDataNodeDefinition{Record: nodeRow}
		for _, stepRow := range steps {
			step := sampleDataStepDefinition{Record: stepRow}
			if formVersionID := cleanID(stepRow["form_version_id"]); formVersionID != "" {
				step.Fields, err = r.Query(ctx, `
					SELECT field_key,label,field_type,required,options_json,validation_json,help_text,
						order_index,visibility_condition_json
					FROM workflow_form_fields
					WHERE form_version_id=$1
					ORDER BY order_index,id
				`, formVersionID)
				if err != nil {
					return sampleDataBlueprint{}, err
				}
			}
			node.Steps = append(node.Steps, step)
		}
		blueprint.Nodes = append(blueprint.Nodes, node)
	}
	roleRows, err := r.Query(ctx, `
		SELECT role.id::text AS role_id,role.name AS role_key,role.display_name AS role_name,role.external_only,
			COALESCE(account.id::text,'') AS user_id,COALESCE(account.email,'') AS email,
			COALESCE(account.status,'') AS user_status,(account.deleted_at IS NOT NULL) AS user_deleted,
			COALESCE(profile.id::text,'') AS staff_id,COALESCE(profile.full_name,'') AS full_name,
			COALESCE(profile.department,'') AS department,COALESCE(profile.status,'') AS staff_status,
			(profile.deleted_at IS NOT NULL) AS staff_deleted,
			EXISTS (
				SELECT 1 FROM users conflicting
				WHERE lower(conflicting.email)=lower(role.name || '@dev.local')
				  AND conflicting.id IS DISTINCT FROM account.id
			) AS email_conflict
		FROM roles role
		LEFT JOIN users account ON account.is_sample AND account.sample_role_id=role.id
		LEFT JOIN LATERAL (
			SELECT staff.id,staff.full_name,staff.department,staff.status,staff.deleted_at
			FROM staff
			WHERE staff.user_id=account.id
			ORDER BY CASE WHEN staff.status='active' AND staff.deleted_at IS NULL THEN 0 ELSE 1 END,
				staff.created_at,staff.id
			LIMIT 1
		) profile ON true
		ORDER BY lower(role.display_name),role.id
	`)
	if err != nil {
		return sampleDataBlueprint{}, err
	}
	for _, row := range roleRows {
		role := sampleDataRoleAccount{
			RoleID: cleanID(row["role_id"]), RoleKey: strings.TrimSpace(fmt.Sprint(row["role_key"])),
			RoleName: strings.TrimSpace(fmt.Sprint(row["role_name"])), ExternalOnly: boolValue(row["external_only"]),
			UserID: cleanID(row["user_id"]), StaffID: cleanID(row["staff_id"]), Email: strings.TrimSpace(fmt.Sprint(row["email"])),
			UserStatus: strings.TrimSpace(fmt.Sprint(row["user_status"])), StaffStatus: strings.TrimSpace(fmt.Sprint(row["staff_status"])),
			UserDeleted: boolValue(row["user_deleted"]), StaffDeleted: boolValue(row["staff_deleted"]),
			EmailConflict: boolValue(row["email_conflict"]),
		}
		blueprint.Roles = append(blueprint.Roles, role)
		if !sampleRoleAccountReady(role) {
			continue
		}
		actor := sampleDataStaffActor{
			StaffID: role.StaffID, UserID: role.UserID, FullName: strings.TrimSpace(fmt.Sprint(row["full_name"])),
			RoleID: role.RoleID, RoleName: role.RoleName, RoleKey: role.RoleKey,
			Department: strings.TrimSpace(fmt.Sprint(row["department"])),
		}
		blueprint.Actors[role.RoleName] = []sampleDataStaffActor{actor}
	}
	return blueprint, nil
}

func sampleRoleAccountReady(role sampleDataRoleAccount) bool {
	return role.UserID != "" && role.StaffID != "" && role.UserStatus == "active" && role.StaffStatus == "active" &&
		!role.UserDeleted && !role.StaffDeleted && !role.EmailConflict && strings.EqualFold(role.Email, sampleRoleEmail(role.RoleKey))
}

func sampleRoleEmail(roleKey string) string {
	return strings.ToLower(strings.TrimSpace(roleKey)) + "@dev.local"
}

func (r *Repository) ListSampleDataAccountingContexts(ctx context.Context) ([]sampleDataAccountingContext, error) {
	rows, err := r.Query(ctx, `
		SELECT book.id::text,book.book_code,book.name,book.base_currency,
			COALESCE(MAX(CASE WHEN account.account_code='1121' AND account.active AND account.is_postable THEN account.id::text END),'') AS bank_account_id,
			COALESCE(MAX(CASE WHEN account.account_code='154' AND account.active AND account.is_postable THEN account.id::text END),'') AS cost_account_id,
			COALESCE(MAX(CASE WHEN account.account_code='5111' AND account.active AND account.is_postable THEN account.id::text END),'') AS revenue_account_id,
			EXISTS (
				SELECT 1 FROM accounting_periods period
				WHERE period.book_id=book.id
				  AND CURRENT_DATE BETWEEN period.date_from AND period.date_to
				  AND period.status='open'
			) AS has_open_period
		FROM accounting_books book
		LEFT JOIN accounting_accounts account ON account.book_id=book.id
		WHERE book.active
		GROUP BY book.id,book.book_code,book.name,book.base_currency
		ORDER BY lower(book.name),book.id
	`)
	if err != nil {
		return nil, err
	}
	contexts := make([]sampleDataAccountingContext, 0, len(rows))
	for _, row := range rows {
		contexts = append(contexts, sampleDataAccountingContext{
			ID: cleanID(row["id"]), Code: strings.TrimSpace(fmt.Sprint(row["book_code"])),
			Name: strings.TrimSpace(fmt.Sprint(row["name"])), BaseCurrency: strings.TrimSpace(fmt.Sprint(row["base_currency"])),
			BankAccountID: cleanID(row["bank_account_id"]), CostAccountID: cleanID(row["cost_account_id"]),
			RevenueAccountID: cleanID(row["revenue_account_id"]), HasOpenPeriod: boolValue(row["has_open_period"]),
		})
	}
	return contexts, nil
}

func sampleRequiredRoles(blueprint sampleDataBlueprint) []string {
	seen := map[string]bool{}
	roles := []string{}
	appendRoles := func(labels []string) {
		for _, label := range labels {
			label = strings.TrimSpace(label)
			if label != "" && !seen[label] {
				seen[label] = true
				roles = append(roles, label)
			}
		}
	}
	for _, node := range blueprint.Nodes {
		if strings.TrimSpace(fmt.Sprint(node.Record["business_code"])) == "boq_without_design" {
			continue
		}
		if strings.TrimSpace(fmt.Sprint(node.Record["approval_rule"])) != "" {
			appendRoles(stringSlice(node.Record["responsible_role_ids"]))
		}
		for _, step := range node.Steps {
			labels := stringSlice(step.Record["required_role_ids"])
			if len(labels) == 0 {
				labels = stringSlice(node.Record["responsible_role_ids"])
			}
			appendRoles(labels)
		}
	}
	sort.Strings(roles)
	return roles
}

func (r *Repository) CreateSampleDataUsers(ctx context.Context, expectedRoles []sampleDataRoleAccount, passwordHash, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('consflow-sample-users'))`); err != nil {
		return nil, err
	}

	createdCount := 0
	existingCount := 0
	items := make([]Record, 0, len(expectedRoles))
	roleKeys := make([]string, 0, len(expectedRoles))
	for _, expected := range expectedRoles {
		var roleKey, roleName string
		var externalOnly bool
		if err := tx.QueryRowContext(ctx, `
			SELECT name,display_name,external_only
			FROM roles WHERE id=$1 FOR SHARE
		`, expected.RoleID).Scan(&roleKey, &roleName, &externalOnly); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, validationError("Vai trò của Pipeline không còn tồn tại", Record{"role_id": expected.RoleID})
			}
			return nil, err
		}
		roleKey = strings.TrimSpace(roleKey)
		roleName = strings.TrimSpace(roleName)
		if !strings.EqualFold(roleKey, expected.RoleKey) || !strings.EqualFold(roleName, expected.RoleName) {
			return nil, conflictError("Vai trò đã thay đổi trong lúc tạo tài khoản mẫu", Record{"role_id": expected.RoleID})
		}
		if externalOnly || sampleRoleRestricted(roleKey) || !validSampleRoleKey(roleKey) {
			return nil, validationError("Vai trò không đủ điều kiện tạo tài khoản mẫu", Record{"role_id": expected.RoleID, "role_key": roleKey})
		}
		email := sampleRoleEmail(roleKey)
		roleKeys = append(roleKeys, roleKey)

		var userID, currentEmail, userStatus string
		var userDeleted bool
		err := tx.QueryRowContext(ctx, `
			SELECT id::text,email,status,(deleted_at IS NOT NULL)
			FROM users
			WHERE is_sample AND sample_role_id=$1
			FOR UPDATE
		`, expected.RoleID).Scan(&userID, &currentEmail, &userStatus, &userDeleted)
		if err == nil {
			var conflictingEmail bool
			if err := tx.QueryRowContext(ctx, `
				SELECT EXISTS(SELECT 1 FROM users WHERE lower(email)=lower($1) AND id<>$2)
			`, email, userID).Scan(&conflictingEmail); err != nil {
				return nil, err
			}
			var staffID, staffStatus string
			var staffDeleted bool
			staffErr := tx.QueryRowContext(ctx, `
				SELECT id::text,status,(deleted_at IS NOT NULL)
				FROM staff
				WHERE user_id=$1
				ORDER BY CASE WHEN status='active' AND deleted_at IS NULL THEN 0 ELSE 1 END,created_at,id
				LIMIT 1
				FOR UPDATE
			`, userID).Scan(&staffID, &staffStatus, &staffDeleted)
			if staffErr != nil && !errors.Is(staffErr, sql.ErrNoRows) {
				return nil, staffErr
			}
			if userStatus != "active" || userDeleted || conflictingEmail || !strings.EqualFold(currentEmail, email) || staffErr != nil || staffStatus != "active" || staffDeleted {
				return nil, conflictError("Tài khoản mẫu hiện có không còn đúng cấu hình chuẩn", Record{"role_id": expected.RoleID, "role": roleName, "email": email})
			}
			existingCount++
			items = append(items, Record{"role_id": expected.RoleID, "role_key": roleKey, "role_name": roleName, "email": email, "user_id": userID, "staff_id": staffID, "status": "existing"})
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		var emailInUse bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE lower(email)=lower($1))`, email).Scan(&emailInUse); err != nil {
			return nil, err
		}
		if emailInUse {
			return nil, conflictError("Email dành cho tài khoản mẫu đã được tài khoản khác sử dụng", Record{"role_id": expected.RoleID, "role": roleName, "email": email})
		}
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO users (email,password_hash,full_name,status,must_change_password,is_sample,sample_role_id)
			VALUES ($1,$2,$3,'active',false,true,$4)
			RETURNING id::text
		`, email, passwordHash, roleName, expected.RoleID).Scan(&userID); err != nil {
			return nil, err
		}
		var staffID string
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO staff (user_id,full_name,email,department,position,status)
			VALUES ($1,$2,$3,'Dữ liệu mẫu',$2,'active')
			RETURNING id::text
		`, userID, roleName, email).Scan(&staffID); err != nil {
			return nil, err
		}
		if err := addAuditTx(ctx, tx, "", actorID, actorName, "sample_user_created", "user_account", userID, "Nhà phát triển tạo tài khoản mẫu theo vai trò Pipeline", Record{
			"staff_id": staffID, "role_id": expected.RoleID, "role_key": roleKey, "email": email, "must_change_password": false,
		}); err != nil {
			return nil, err
		}
		createdCount++
		items = append(items, Record{"role_id": expected.RoleID, "role_key": roleKey, "role_name": roleName, "email": email, "user_id": userID, "staff_id": staffID, "status": "created"})
	}

	batchID := newID()
	message := fmt.Sprintf("Đã tạo %d tài khoản mẫu; %d tài khoản đã tồn tại", createdCount, existingCount)
	if createdCount == 0 {
		message = fmt.Sprintf("Đã có đủ tài khoản mẫu cho %d/%d vai trò", existingCount, len(expectedRoles))
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "sample_users_provisioned", "sample_user_batch", batchID, message, Record{
		"created_count": createdCount, "existing_count": existingCount, "role_count": len(expectedRoles), "role_keys": roleKeys,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return Record{
		"batch_id": batchID, "created_count": createdCount, "existing_count": existingCount,
		"role_count": len(expectedRoles), "ready": true, "message": message, "items": items,
	}, nil
}

func (r *Repository) FindSampleDataGeneration(ctx context.Context, idempotencyKey string) (Record, error) {
	row, err := r.QueryOne(ctx, `
		SELECT generation.*,project.name AS project_name,project.code AS project_code,
			project.status AS project_status,project.progress_percent
		FROM sample_data_generations generation
		LEFT JOIN projects project ON project.id=generation.project_id
		WHERE generation.idempotency_key=$1
	`, idempotencyKey)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return row, err
}

func (r *Repository) CreateSampleData(ctx context.Context, plan sampleDataPlan) (result Record, err error) {
	tracker := storedMutationTracker{}
	defer tracker.wrap(&err)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('consflow-sample-data'))`); err != nil {
		return nil, err
	}
	var existingActor, existingProfile, existingProject, existingTemplate, existingAccountingBook string
	var existingProgress int
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(actor_id::text,''),profile,progress_percent,COALESCE(project_id::text,''),
			COALESCE(template_id::text,''),COALESCE(accounting_book_id::text,'')
		FROM sample_data_generations WHERE idempotency_key=$1
	`, plan.IdempotencyKey).Scan(&existingActor, &existingProfile, &existingProgress, &existingProject, &existingTemplate, &existingAccountingBook)
	if err == nil {
		sourceMismatch := existingTemplate != plan.TemplateID
		if plan.Profile == "completed" {
			sourceMismatch = sourceMismatch || existingAccountingBook != plan.AccountingBookID
		}
		if existingActor != plan.GeneratorID || existingProfile != plan.Profile || existingProgress != plan.ProgressPercent || sourceMismatch {
			return nil, conflictError("Khoá idempotency đã được dùng cho một yêu cầu tạo dữ liệu mẫu khác")
		}
		if existingProject == "" {
			return nil, conflictError("Yêu cầu tạo dữ liệu mẫu trước đó chưa có kết quả công trình")
		}
		if err := tracker.commit(tx.Commit); err != nil {
			return nil, err
		}
		return r.sampleDataProjectResult(ctx, existingProject, true)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO projects (
			id,name,code,client_name,address,planned_start_date,planned_end_date,
			estimated_budget,contract_amount,status,pipeline_template_id,accounting_book_id,pm_staff_id,
			progress_percent,workflow_entry_mode,is_sample,sample_profile,sample_blueprint_version
		) VALUES (
			$1,$2,$3,'Công ty Demo Consflow','Công trình mẫu · TP.HCM',CURRENT_DATE-interval '180 days',CURRENT_DATE+interval '180 days',
			3600000000,5200000000,$4,$5,$6,NULLIF($7,'')::uuid,$8,'survey_required',true,$9,$10
		)
	`, plan.ProjectID, plan.ProjectName, plan.ProjectCode, plan.ProjectStatus, plan.TemplateID, plan.AccountingBookID,
		samplePMStaffID(plan), plan.ProgressPercent, plan.Profile, sampleDataBlueprintVersion); err != nil {
		return nil, err
	}
	if err := insertSampleProjectMembersTx(ctx, tx, plan); err != nil {
		return nil, fmt.Errorf("insert sample project members: %w", err)
	}
	if plan.Profile != "draft" {
		if err := insertSampleRuntimeTx(ctx, tx, plan); err != nil {
			return nil, fmt.Errorf("insert sample runtime: %w", err)
		}
		if err := insertSampleFormsEvidenceApprovalsTx(ctx, tx, plan); err != nil {
			return nil, fmt.Errorf("insert sample forms, evidence and approvals: %w", err)
		}
		if err := insertSampleDomainHistoryTx(ctx, tx, plan); err != nil {
			return nil, fmt.Errorf("insert sample domain history: %w", err)
		}
		if err := ensureProjectRunStepWorkItemCompletionSyncTx(ctx, tx, plan.ProjectID); err != nil {
			return nil, fmt.Errorf("validate sample workflow lifecycle: %w", err)
		}
	}
	result = Record{
		"project_id": plan.ProjectID, "project_code": plan.ProjectCode, "project_name": plan.ProjectName,
		"profile": plan.Profile, "progress_percent": plan.ProgressPercent, "is_sample": true,
		"pipeline_template_id": plan.TemplateID, "accounting_book_id": plan.AccountingBookID,
	}
	resultJSON, _ := json.Marshal(result)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sample_data_generations (
			id,idempotency_key,actor_id,profile,progress_percent,blueprint_version,
			template_id,template_definition_hash,accounting_book_id,project_id,result_json
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb)
	`, plan.GenerationID, plan.IdempotencyKey, plan.GeneratorID, plan.Profile, plan.ProgressPercent,
		sampleDataBlueprintVersion, plan.TemplateID, plan.TemplateHash, plan.AccountingBookID, plan.ProjectID, string(resultJSON)); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, plan.ProjectID, plan.GeneratorID, plan.GeneratorName,
		"sample_project_generated", "project", plan.ProjectID,
		"Nhà phát triển đã tạo công trình dữ liệu mẫu",
		Record{"simulated": true, "profile": plan.Profile, "progress_percent": plan.ProgressPercent, "blueprint_version": sampleDataBlueprintVersion}); err != nil {
		return nil, err
	}
	if err := tracker.commit(tx.Commit); err != nil {
		return nil, err
	}
	return r.sampleDataProjectResult(ctx, plan.ProjectID, false)
}

func (r *Repository) sampleDataProjectResult(ctx context.Context, projectID string, replayed bool) (Record, error) {
	row, err := r.QueryOne(ctx, `
		SELECT id::text,name,code,status,progress_percent,is_sample,sample_profile,
			sample_blueprint_version,current_pipeline_run_id::text,pipeline_template_id::text,accounting_book_id::text
		FROM projects WHERE id=$1 AND status <> 'cancelled'
	`, projectID)
	if err != nil {
		return nil, err
	}
	row["replayed"] = replayed
	return row, nil
}

func samplePMStaffID(plan sampleDataPlan) string {
	for _, actor := range plan.AllActors {
		if strings.EqualFold(strings.TrimSpace(actor.RoleKey), "pm") {
			return actor.StaffID
		}
	}
	return ""
}

func insertSampleProjectMembersTx(ctx context.Context, tx *sql.Tx, plan sampleDataPlan) error {
	type membership struct {
		staffID string
		userID  string
		roles   []string
		roleIDs []string
	}
	byStaff := map[string]*membership{}
	for roleName, actors := range plan.Actors {
		for _, actor := range actors {
			item := byStaff[actor.StaffID]
			if item == nil {
				item = &membership{staffID: actor.StaffID, userID: actor.UserID}
				byStaff[actor.StaffID] = item
			}
			item.roles = append(item.roles, roleName)
			item.roleIDs = append(item.roleIDs, actor.RoleID)
		}
	}
	staffIDs := make([]string, 0, len(byStaff))
	for staffID := range byStaff {
		staffIDs = append(staffIDs, staffID)
	}
	sort.Strings(staffIDs)
	for _, staffID := range staffIDs {
		item := byStaff[staffID]
		sort.Strings(item.roles)
		raw, _ := json.Marshal(item.roles)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO project_members (project_id,staff_id,role_ids,assigned_nodes,active)
			VALUES ($1,$2,$3::jsonb,'[]'::jsonb,true)
		`, plan.ProjectID, staffID, string(raw)); err != nil {
			return err
		}
		for _, roleID := range item.roleIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO user_roles (user_id,role_id,project_id)
				VALUES ($1,$2,$3)
				ON CONFLICT DO NOTHING
			`, item.userID, roleID, plan.ProjectID); err != nil {
				return err
			}
		}
	}
	return nil
}

func insertSampleRuntimeTx(ctx context.Context, tx *sql.Tx, plan sampleDataPlan) error {
	runStatus := "in_progress"
	var completedAt any
	if plan.Profile == "completed" {
		runStatus = "completed"
		completedAt = time.Now()
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pipeline_runs (
			id,project_id,template_id,current_node_id,status,started_at,completed_at,
			engine_version,template_definition_hash
		) SELECT $1,$2,template.id,NULLIF($4,'')::uuid,$5,now()-interval '180 days',$6,
			template.engine_version,template.definition_hash
		  FROM pipeline_templates template WHERE template.id=$3
	`, plan.RunID, plan.ProjectID, plan.TemplateID, plan.CurrentNodeID, runStatus, completedAt); err != nil {
		return err
	}
	nodeIDByTemplateID := map[string]string{}
	for _, node := range plan.Nodes {
		nodeIDByTemplateID[cleanID(node.Definition.Record["id"])] = node.RunNodeID
		roles, _ := json.Marshal(node.Definition.Record["responsible_role_ids"])
		permissions, _ := json.Marshal(nodePermissions(node.Definition.Record))
		exitConditions, _ := json.Marshal(node.Definition.Record["exit_conditions"])
		approvalPolicy, _ := json.Marshal(node.Definition.Record["approval_policy_json"])
		var startedAt, completedAt any
		if node.Status == "in_progress" || node.Status == "completed" {
			startedAt = time.Now().Add(-120 * 24 * time.Hour)
		}
		if node.Status == "completed" {
			completedAt = time.Now().Add(-24 * time.Hour)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO pipeline_run_nodes (
				id,run_id,project_id,template_node_id,business_code,name,description,type,order_index,
				responsible_role_ids,required_permission,permissions,exit_conditions,approval_rule,
				approval_policy_json,fail_path,status,started_at,completed_at
			) VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,$10::jsonb,$11,$12::jsonb,$13::jsonb,$14,$15::jsonb,$16,$17,$18,$19)
		`, node.RunNodeID, plan.RunID, plan.ProjectID, node.Definition.Record["id"], cleanOptionalText(node.Definition.Record["business_code"]),
			node.Definition.Record["name"], node.Definition.Record["description"], node.Definition.Record["type"], node.Definition.Record["order_index"],
			string(roles), node.Definition.Record["required_permission"], string(permissions), string(exitConditions), node.Definition.Record["approval_rule"],
			string(approvalPolicy), node.Definition.Record["fail_path"], node.Status, startedAt, completedAt); err != nil {
			return err
		}
		for _, step := range node.Steps {
			if err := insertSampleRuntimeStepTx(ctx, tx, plan, node, step); err != nil {
				return err
			}
		}
	}
	for _, edge := range plan.Edges {
		fromID := nodeIDByTemplateID[cleanID(edge["from_node_id"])]
		toID := nodeIDByTemplateID[cleanID(edge["to_node_id"])]
		if fromID == "" || toID == "" {
			continue
		}
		config, _ := json.Marshal(edge["condition_config"])
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO pipeline_run_edges (
				id,run_id,project_id,template_edge_id,from_run_node_id,to_run_node_id,
				condition_type,condition_config,label,priority
			) VALUES (gen_random_uuid(),$1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9)
		`, plan.RunID, plan.ProjectID, edge["id"], fromID, toID, edgeConditionType(edge), string(config), edge["label"], edgePriority(edge)); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE projects SET current_pipeline_run_id=$1,updated_at=now() WHERE id=$2
	`, plan.RunID, plan.ProjectID); err != nil {
		return err
	}
	return nil
}

func insertSampleRuntimeStepTx(ctx context.Context, tx *sql.Tx, plan sampleDataPlan, node sampleDataPlannedNode, step sampleDataPlannedStep) error {
	roles, _ := json.Marshal(step.Definition.Record["required_role_ids"])
	var submittedBy, submittedAt, reviewedAt, startedAt, completedAt any
	if step.Completed {
		submittedBy = step.Actor.UserID
		submittedAt = time.Now().Add(-48 * time.Hour)
		reviewedAt = time.Now().Add(-47 * time.Hour)
		startedAt = time.Now().Add(-72 * time.Hour)
		completedAt = time.Now().Add(-47 * time.Hour)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pipeline_run_steps (
			id,run_node_id,project_id,template_step_id,business_code,name,description,step_type,
			required_role_ids,required_permission,checklist_required,document_required,photo_required,
			form_required,form_version_id,surface_kind,domain_action_key,order_index,status,
			submitted_by,submitted_at,reviewed_at
		) VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9::jsonb,$10,$11,$12,$13,$14,NULLIF($15,'')::uuid,NULLIF($16,''),NULLIF($17,''),$18,$19,NULLIF($20,'')::uuid,$21,$22)
	`, step.RunStepID, node.RunNodeID, plan.ProjectID, step.Definition.Record["id"], cleanOptionalText(step.Definition.Record["business_code"]),
		step.Definition.Record["name"], step.Definition.Record["description"], step.Definition.Record["step_type"], string(roles),
		step.Definition.Record["required_permission"], step.Definition.Record["checklist_required"], step.Definition.Record["document_required"],
		step.Definition.Record["photo_required"], step.Definition.Record["form_required"], cleanID(step.Definition.Record["form_version_id"]),
		cleanOptionalText(step.Definition.Record["surface_kind"]), cleanOptionalText(step.Definition.Record["domain_action_key"]),
		step.Definition.Record["order_index"], step.Status, cleanID(submittedBy), submittedAt, reviewedAt); err != nil {
		return err
	}
	evidenceRequirement, _ := json.Marshal(Record{
		"checklist_required": step.Definition.Record["checklist_required"], "document_required": step.Definition.Record["document_required"],
		"photo_required": step.Definition.Record["photo_required"], "form_required": step.Definition.Record["form_required"],
		"form_version_id": cleanID(step.Definition.Record["form_version_id"]), "required_permission": step.Definition.Record["required_permission"],
	})
	completion, _ := json.Marshal(Record{"simulated": true, "result": "Hoàn tất dữ liệu mẫu", "blueprint_version": sampleDataBlueprintVersion})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_items (
			id,project_id,pipeline_run_id,run_node_id,run_step_id,source_type,source_id,title,description,
			work_type,status,priority,department,role_ids,assignee_staff_id,started_at,completed_at,
			completion_payload_json,evidence_requirement_json,created_by,updated_by
		) VALUES ($1,$2,$3,$4,$5,'pipeline_run_step',$5,$6,$7,$8,$9,'normal',NULLIF($10,''),$11::jsonb,NULLIF($12,'')::uuid,$13,$14,$15::jsonb,$16::jsonb,$17,$17)
	`, step.WorkItemID, plan.ProjectID, plan.RunID, node.RunNodeID, step.RunStepID, step.Definition.Record["name"],
		step.Definition.Record["description"], workItemType(fmt.Sprint(step.Definition.Record["step_type"])), step.WorkStatus,
		step.Actor.Department, string(roles), step.Actor.StaffID, startedAt, completedAt, string(completion), string(evidenceRequirement), plan.ActorID); err != nil {
		return err
	}
	for _, roleLabel := range stringSlice(step.Definition.Record["required_role_ids"]) {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_assignments (work_item_id,assignment_type,role_name,created_by)
			VALUES ($1,'role',$2,$3)
		`, step.WorkItemID, roleLabel, plan.ActorID); err != nil {
			return err
		}
	}
	if step.Actor.StaffID != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_assignments (work_item_id,assignment_type,staff_id,created_by)
			VALUES ($1,'assignee',$2,$3) ON CONFLICT DO NOTHING
		`, step.WorkItemID, step.Actor.StaffID, plan.ActorID); err != nil {
			return err
		}
	}
	activityType := "created"
	comment := "Đầu việc dữ liệu mẫu được tạo từ Pipeline đã chọn"
	if step.Completed {
		activityType = "completed"
		comment = "Đầu việc dữ liệu mẫu đã hoàn tất và có hồ sơ lịch sử"
	}
	metadata, _ := json.Marshal(Record{"simulated": true, "generated_by": plan.ActorID, "blueprint_version": sampleDataBlueprintVersion})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (
			work_item_id,project_id,scope_type,scope_id,actor_id,activity_type,to_status,comment,metadata_json
		) VALUES ($1,$2,'work_item',$1,$3,$4,$5,$6,$7::jsonb)
	`, step.WorkItemID, plan.ProjectID, plan.ActorID, activityType, step.WorkStatus, comment, string(metadata)); err != nil {
		return err
	}
	return nil
}

func insertSampleFormsEvidenceApprovalsTx(ctx context.Context, tx *sql.Tx, plan sampleDataPlan) error {
	for _, node := range plan.Nodes {
		for _, step := range node.Steps {
			for index, artifact := range step.Evidence {
				evidenceType := "document"
				documentType := "record"
				if strings.HasPrefix(artifact.MIMEType, "image/") {
					evidenceType, documentType = "photo", "photo"
				}
				payload, _ := json.Marshal(Record{"simulated": true, "generated_by": plan.ActorID, "blueprint_version": sampleDataBlueprintVersion})
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO evidence_records (
						id,work_item_id,project_id,target_type,target_id,evidence_type,document_type,name,title,
						description,file_url,mime_type,file_size,caption,revision_no,visibility,payload_json,status,
						uploaded_by,submitted_by,reviewed_by,reviewed_at,storage_provider,storage_key,checksum_sha256
					) VALUES (
						$1,$2,$3,'work_item',$2,$4,$5,$6,$7,$8,$9,$10,$11,$12,1,'internal',$13::jsonb,'accepted',
						$14,$14,$14,now()-interval '47 hours',$15,$16,$17
					)
				`, artifact.ID, step.WorkItemID, plan.ProjectID, evidenceType, documentType, artifact.Name,
					fmt.Sprintf("Minh chứng %02d · %s", index+1, step.Definition.Record["name"]),
					"Minh chứng hợp lệ của dữ liệu mẫu, lưu trong storage hiện hành.",
					"/evidence/"+artifact.ID+"/file", artifact.MIMEType, artifact.Size,
					"Dữ liệu mẫu · "+fmt.Sprint(step.Definition.Record["name"]), string(payload), plan.ActorID,
					artifact.Provider, artifact.Key, artifact.Checksum); err != nil {
					return fmt.Errorf("insert evidence for step %s: %w", step.RunStepID, err)
				}
			}
			if step.FormInstanceID == "" {
				continue
			}
			roles, _ := json.Marshal(step.Definition.Record["required_role_ids"])
			formStatus := "draft"
			currentRevision := 0
			var submittedBy, submittedAt any
			if step.Completed {
				formStatus, currentRevision = "submitted", 1
				submittedBy, submittedAt = step.Actor.UserID, time.Now().Add(-48*time.Hour)
			}
			answersJSON := sampleAnswersJSON(step.Answers)
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO workflow_form_instances (
					id,project_id,work_item_id,pipeline_run_id,run_node_id,run_step_id,template_step_id,
					form_template_id,form_version_id,form_type,title,status,current_revision,
					answers_snapshot_json,required_role_ids,created_by,submitted_by,submitted_at
				)
				SELECT $1,$2,$3,$4,$5,$6,$7,version.template_id,version.id,template.form_type,$8,$9,$10,
					$11::jsonb,$12::jsonb,$13,NULLIF($14,'')::uuid,$15
				FROM workflow_form_versions version
				JOIN workflow_form_templates template ON template.id=version.template_id
				WHERE version.id=$16
			`, step.FormInstanceID, plan.ProjectID, step.WorkItemID, plan.RunID, node.RunNodeID, step.RunStepID,
				step.Definition.Record["id"], step.Definition.Record["name"], formStatus, currentRevision, answersJSON,
				string(roles), plan.ActorID, cleanID(submittedBy), submittedAt, cleanID(step.Definition.Record["form_version_id"])); err != nil {
				return fmt.Errorf("insert form instance for step %s: %w", step.RunStepID, err)
			}
			createdMetadata, _ := json.Marshal(Record{"simulated": true, "source": "sample_data_generator"})
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO workflow_form_events (
					instance_id,project_id,work_item_id,actor_id,event_type,to_status,comment,metadata_json
				) VALUES ($1,$2,$3,$4,'created','draft','Khởi tạo biểu mẫu dữ liệu mẫu',$5::jsonb)
			`, step.FormInstanceID, plan.ProjectID, step.WorkItemID, plan.ActorID, string(createdMetadata)); err != nil {
				return fmt.Errorf("insert form created event for step %s: %w", step.RunStepID, err)
			}
			if !step.Completed {
				continue
			}
			for _, field := range step.Definition.Fields {
				key := strings.TrimSpace(fmt.Sprint(field["field_key"]))
				valueJSON, _ := json.Marshal(step.Answers[key])
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO workflow_form_answers (
						instance_id,field_key,field_type,value_json,revision_no,updated_by
					) VALUES ($1,$2,$3,$4::jsonb,1,$5)
				`, step.FormInstanceID, key, field["field_type"], string(valueJSON), plan.ActorID); err != nil {
					return fmt.Errorf("insert form answer %s for step %s: %w", key, step.RunStepID, err)
				}
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO workflow_form_events (
					instance_id,project_id,work_item_id,actor_id,event_type,from_status,to_status,
					comment,metadata_json
				) VALUES ($1,$2,$3,$4,'submitted','draft','submitted','Đã nộp biểu mẫu dữ liệu mẫu',$5::jsonb)
			`, step.FormInstanceID, plan.ProjectID, step.WorkItemID, plan.ActorID, string(createdMetadata)); err != nil {
				return fmt.Errorf("insert form submitted event for step %s: %w", step.RunStepID, err)
			}
			signaturePayload, _ := json.Marshal(Record{"simulated": true, "form_instance_id": step.FormInstanceID, "generated_by": plan.ActorID})
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO workflow_form_signatures (
					project_id,work_item_id,form_instance_id,form_version_id,form_revision_no,purpose,
					signer_user_id,signer_name,signer_roles_json,comment,payload_json,payload_hash,visual_storage_provider
				) VALUES ($1,$2,$3,$4,1,'approved_by',$5,$6,$7::jsonb,
					'Chữ ký xác nhận dữ liệu mẫu',$8::jsonb,encode(digest($9 || ':sample-signature','sha256'),'hex'),'none')
			`, plan.ProjectID, step.WorkItemID, step.FormInstanceID, step.Definition.Record["form_version_id"],
				step.Actor.UserID, step.Actor.FullName, string(roles), string(signaturePayload), step.FormInstanceID); err != nil {
				return fmt.Errorf("insert form signature for step %s: %w", step.RunStepID, err)
			}
			if step.FormPDF != nil {
				pdfMetadata, _ := json.Marshal(Record{"simulated": true, "blueprint_version": sampleDataBlueprintVersion})
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO workflow_form_pdf_exports (
						id,project_id,work_item_id,form_instance_id,form_version_id,form_revision_no,title,file_name,
						storage_provider,storage_key,mime_type,file_size,checksum_sha256,payload_hash,metadata_json,
						generated_by,generated_by_name
					) VALUES ($1,$2,$3,$4,$5,1,$6,$7,$8,$9,$10,$11,$12,
						encode(digest($16 || ':sample-pdf','sha256'),'hex'),$13::jsonb,$14,$15)
				`, step.FormPDF.ID, plan.ProjectID, step.WorkItemID, step.FormInstanceID, step.Definition.Record["form_version_id"],
					"Hồ sơ mẫu · "+fmt.Sprint(step.Definition.Record["name"]), step.FormPDF.Name, step.FormPDF.Provider,
					step.FormPDF.Key, step.FormPDF.MIMEType, step.FormPDF.Size, step.FormPDF.Checksum,
					string(pdfMetadata), plan.ActorID, plan.ActorName, step.FormInstanceID); err != nil {
					return fmt.Errorf("insert form PDF for step %s: %w", step.RunStepID, err)
				}
			}
		}
		if !node.Selected || strings.TrimSpace(fmt.Sprint(node.Definition.Record["approval_rule"])) == "" {
			continue
		}
		approvalStatus := "pending"
		if node.Status == "completed" {
			approvalStatus = "approved"
		}
		if node.Status == "pending" {
			continue
		}
		requestID := newID()
		requiredRoles, _ := json.Marshal(node.Definition.Record["responsible_role_ids"])
		var resolvedAt any
		if approvalStatus == "approved" {
			resolvedAt = time.Now().Add(-47 * time.Hour)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO approval_requests (
				id,target_type,target_id,project_id,requested_by,status,reason,required_roles,resolved_at
			) VALUES ($1,'pipeline_run_node',$2,$3,$4,$5,
				'Yêu cầu phê duyệt được giả lập cho công trình mẫu',$6::jsonb,$7)
		`, requestID, node.RunNodeID, plan.ProjectID, plan.ActorID, approvalStatus, string(requiredRoles), resolvedAt); err != nil {
			return fmt.Errorf("insert approval request for node %s: %w", node.RunNodeID, err)
		}
		if len(node.Steps) > 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE work_items SET approval_request_id=$1 WHERE id=$2`, requestID, node.Steps[0].WorkItemID); err != nil {
				return err
			}
		}
		if approvalStatus == "approved" {
			approvalActor := sampleFirstActorForRoles(plan, stringSlice(node.Definition.Record["responsible_role_ids"]))
			if approvalActor.UserID == "" || approvalActor.RoleID == "" {
				return fmt.Errorf("resolve approval actor for node %s", node.RunNodeID)
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO approval_actions (approval_request_id,user_id,role_id,action,comment,created_at)
				VALUES ($1,$2,$3,'approved','Đã phê duyệt dữ liệu mẫu',now()-interval '47 hours')
			`, requestID, approvalActor.UserID, approvalActor.RoleID); err != nil {
				return err
			}
		}
	}
	return nil
}

func sampleFirstActorForRoles(plan sampleDataPlan, roleNames []string) sampleDataStaffActor {
	for _, roleName := range roleNames {
		if actors := plan.Actors[strings.TrimSpace(roleName)]; len(actors) > 0 {
			return actors[0]
		}
	}
	return sampleDataStaffActor{}
}
