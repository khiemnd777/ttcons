package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"codeberg.org/go-pdf/fpdf"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var sampleRoleKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

func (s *Service) SampleDataOptions(ctx context.Context) (SampleDataOptions, error) {
	blueprints, err := s.repo.ListPublishedSampleDataBlueprints(ctx)
	if err != nil {
		return SampleDataOptions{}, err
	}
	options := SampleDataOptions{
		BlueprintVersion:  sampleDataBlueprintVersion,
		ProgressPresets:   append([]int(nil), sampleDataPlanningProgressPresets...),
		Pipelines:         []SampleDataPipelineOption{},
		AccountingBooks:   []SampleDataAccountingBookOption{},
		EnvironmentPolicy: "available_in_all_environments",
	}
	for _, blueprint := range blueprints {
		option := sampleDataPipelineCompatibility(blueprint)
		options.Pipelines = append(options.Pipelines, option)
		options.Ready = options.Ready || option.Compatible
	}
	accountingContexts, err := s.repo.ListSampleDataAccountingContexts(ctx)
	if err != nil {
		return SampleDataOptions{}, err
	}
	for _, accounting := range accountingContexts {
		options.AccountingBooks = append(options.AccountingBooks, sampleDataAccountingCompatibility(accounting))
	}
	options.Ready = options.Ready && len(options.AccountingBooks) > 0
	return options, nil
}

func (s *Service) CreateSampleDataUsers(ctx context.Context, req SampleDataUserCreateRequest, actorID, actorName string) (Record, error) {
	pipelineID, err := uuid.Parse(strings.TrimSpace(req.PipelineTemplateID))
	if err != nil {
		return nil, validationError("Pipeline tạo tài khoản mẫu không hợp lệ", Record{"field": "pipeline_template_id"})
	}
	blueprint, err := s.repo.LoadSampleDataBlueprint(ctx, pipelineID.String())
	if err != nil {
		return nil, err
	}
	option := sampleDataPipelineCompatibility(blueprint)
	if !option.SampleUsersProvisionable {
		return nil, validationError("Pipeline chưa đủ điều kiện tạo tài khoản mẫu", Record{
			"pipeline_template_id": pipelineID.String(), "blockers": option.Blockers,
		})
	}
	expectedRoles := make([]sampleDataRoleAccount, 0, len(option.RoleCoverage))
	for _, coverage := range option.RoleCoverage {
		for _, role := range blueprint.Roles {
			if role.RoleID == coverage.RoleID {
				expectedRoles = append(expectedRoles, role)
				break
			}
		}
	}
	if len(expectedRoles) == 0 {
		return nil, validationError("Pipeline chưa khai báo vai trò hợp lệ cho dữ liệu mẫu")
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(sampleDataUserPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateSampleDataUsers(ctx, expectedRoles, string(passwordHash), actorID, actorName)
}

func sampleDataPipelineCompatibility(blueprint sampleDataBlueprint) SampleDataPipelineOption {
	option := SampleDataPipelineOption{
		ID: cleanID(blueprint.Template["id"]), Name: strings.TrimSpace(fmt.Sprint(blueprint.Template["name"])),
		Revision: int(number(blueprint.Template["revision_no"])), DefinitionHash: cleanOptionalText(blueprint.Template["definition_hash"]),
		EngineVersion: int(number(blueprint.Template["engine_version"])), NodeCount: len(blueprint.Nodes),
		SelectedStepCount: sampleSelectedStepCount(blueprint), Blockers: []SampleDataBlocker{},
		RoleCoverage: []SampleDataRoleCoverage{}, MissingRoles: []string{},
	}
	structureCompatible := true
	usersProvisionable := true
	addStructureBlocker := func(code, message string) {
		option.Blockers = append(option.Blockers, SampleDataBlocker{Code: code, Message: message})
		structureCompatible = false
		usersProvisionable = false
	}
	addRoleBlocker := func(code, message string, blocksProvisioning bool) {
		option.Blockers = append(option.Blockers, SampleDataBlocker{Code: code, Message: message})
		if blocksProvisioning {
			usersProvisionable = false
		}
	}
	if strings.TrimSpace(fmt.Sprint(blueprint.Template["status"])) != "published" {
		addStructureBlocker("PIPELINE_NOT_PUBLISHED", "Pipeline chưa được phát hành")
	}
	if option.DefinitionHash == "" {
		addStructureBlocker("PIPELINE_HASH_MISSING", "Pipeline chưa có definition hash sau khi phát hành")
	}
	if option.SelectedStepCount == 0 {
		addStructureBlocker("DESIGN_BRANCH_EMPTY", "Nhánh Có thiết kế chưa có bước thực hiện")
	}
	if !sampleBlueprintHasNode(blueprint, "boq_without_design") {
		addStructureBlocker("NO_DESIGN_BRANCH_MISSING", "Thiếu nhánh Không thiết kế để đánh dấu bỏ qua")
	}
	for _, binding := range []struct {
		code, message, node, surface, action string
	}{
		{code: "CONSTRUCTION_BINDING_MISSING", message: "Thiếu Node nghiệp vụ thi công", node: "construction_execution"},
		{code: "INCOMING_ACCEPTANCE_BINDING_MISSING", message: "Thiếu binding nghiệm thu vật tư đầu vào", surface: "incoming_acceptance", action: "acceptance.incoming_material"},
		{code: "OUTGOING_ACCEPTANCE_BINDING_MISSING", message: "Thiếu Node nghiệm thu đầu ra", node: "outgoing_acceptance"},
		{code: "HANDOVER_BINDING_MISSING", message: "Thiếu binding xác nhận hoàn tất bàn giao", surface: "handover", action: "source.handover_completed"},
	} {
		present := sampleBlueprintHasNode(blueprint, binding.node)
		if binding.node == "" {
			present = sampleBlueprintHasStepBinding(blueprint, binding.surface, binding.action)
		}
		if !present {
			addStructureBlocker(binding.code, binding.message)
		}
	}
	for _, node := range blueprint.Nodes {
		if strings.TrimSpace(fmt.Sprint(node.Record["business_code"])) == "boq_without_design" {
			continue
		}
		for _, step := range node.Steps {
			if !boolValue(step.Record["form_required"]) {
				continue
			}
			if cleanID(step.Record["form_version_id"]) == "" {
				addStructureBlocker("FORM_REFERENCE_MISSING", fmt.Sprintf("Bước %s chưa tham chiếu phiên bản biểu mẫu", step.Record["name"]))
			} else if strings.TrimSpace(fmt.Sprint(step.Record["form_version_status"])) != "published" {
				addStructureBlocker("FORM_VERSION_NOT_PUBLISHED", fmt.Sprintf("Biểu mẫu của bước %s chưa được phát hành", step.Record["name"]))
			}
		}
	}
	requiredRoles := sampleRequiredRoles(blueprint)
	if len(requiredRoles) == 0 {
		addStructureBlocker("ROLE_REQUIREMENT_MISSING", "Nhánh Có thiết kế chưa khai báo vai trò thực hiện")
	}
	rolesReady := true
	for _, roleName := range requiredRoles {
		matches := sampleRoleMatches(blueprint, roleName)
		coverage := SampleDataRoleCoverage{RoleName: roleName, Status: "blocked", Message: "Vai trò chưa thể dùng để tạo tài khoản mẫu"}
		if len(matches) == 0 {
			rolesReady = false
			option.MissingRoles = append(option.MissingRoles, roleName)
			coverage.Message = "Không tìm thấy vai trò tương ứng trong hệ thống"
			addRoleBlocker("ROLE_NOT_FOUND", "Không tìm thấy vai trò hệ thống cho nhãn "+roleName, true)
			option.RoleCoverage = append(option.RoleCoverage, coverage)
			continue
		}
		if len(matches) > 1 {
			rolesReady = false
			coverage.Message = "Có nhiều vai trò trùng tên hiển thị"
			addRoleBlocker("ROLE_AMBIGUOUS", "Có nhiều vai trò hệ thống trùng nhãn "+roleName, true)
			option.RoleCoverage = append(option.RoleCoverage, coverage)
			continue
		}
		role := matches[0]
		coverage.RoleID, coverage.RoleKey, coverage.RoleName = role.RoleID, role.RoleKey, role.RoleName
		coverage.Email = sampleRoleEmail(role.RoleKey)
		if role.ExternalOnly || sampleRoleRestricted(role.RoleKey) {
			rolesReady = false
			coverage.Message = "Vai trò quản trị hoặc đối tác không được tạo tài khoản mẫu"
			addRoleBlocker("SAMPLE_ROLE_RESTRICTED", "Vai trò không được phép tạo tài khoản mẫu: "+role.RoleName, true)
		} else if !validSampleRoleKey(role.RoleKey) {
			rolesReady = false
			coverage.Message = "Mã vai trò không thể dùng để tạo email @dev.local"
			addRoleBlocker("SAMPLE_ROLE_EMAIL_INVALID", "Mã vai trò không hợp lệ để tạo email mẫu: "+role.RoleKey, true)
		} else if role.UserID == "" {
			rolesReady = false
			option.MissingRoles = append(option.MissingRoles, role.RoleName)
			if role.EmailConflict {
				coverage.Message = "Email mẫu đã được một tài khoản khác sử dụng"
				addRoleBlocker("SAMPLE_EMAIL_CONFLICT", "Email "+coverage.Email+" đã được một tài khoản khác sử dụng", true)
			} else {
				coverage.Status = "missing"
				coverage.Message = "Chưa tạo tài khoản mẫu"
				addRoleBlocker("SAMPLE_ACCOUNT_MISSING", "Chưa có tài khoản mẫu cho vai trò "+role.RoleName, false)
			}
		} else if !sampleRoleAccountReady(role) {
			rolesReady = false
			coverage.Message = "Tài khoản mẫu hiện có bị lệch email, trạng thái hoặc hồ sơ nhân sự"
			addRoleBlocker("SAMPLE_ACCOUNT_NOT_READY", "Tài khoản mẫu chưa đúng cấu hình cho vai trò "+role.RoleName, true)
		} else {
			coverage.Status = "ready"
			coverage.Message = "Tài khoản mẫu sẵn sàng"
			coverage.StaffCount = 1
			coverage.Ready = true
		}
		option.RoleCoverage = append(option.RoleCoverage, coverage)
	}
	option.StructureCompatible = structureCompatible
	option.SampleUsersProvisionable = structureCompatible && usersProvisionable
	option.SampleUsersReady = rolesReady && len(option.RoleCoverage) > 0
	option.Compatible = option.StructureCompatible && option.SampleUsersReady
	return option
}

func sampleRoleMatches(blueprint sampleDataBlueprint, roleLabel string) []sampleDataRoleAccount {
	matches := []sampleDataRoleAccount{}
	for _, role := range blueprint.Roles {
		if strings.EqualFold(strings.TrimSpace(role.RoleName), strings.TrimSpace(roleLabel)) ||
			(strings.EqualFold(role.RoleKey, "pm") && normalizeRoleLabel(roleLabel) == "pm") {
			matches = append(matches, role)
		}
	}
	return matches
}

func sampleRoleRestricted(roleKey string) bool {
	switch strings.ToLower(strings.TrimSpace(roleKey)) {
	case "admin", "developer":
		return true
	default:
		return false
	}
}

func validSampleRoleKey(roleKey string) bool {
	return sampleRoleKeyPattern.MatchString(strings.ToLower(strings.TrimSpace(roleKey)))
}

func sampleDataAccountingCompatibility(accounting sampleDataAccountingContext) SampleDataAccountingBookOption {
	option := SampleDataAccountingBookOption{
		ID: accounting.ID, Code: accounting.Code, Name: accounting.Name, BaseCurrency: accounting.BaseCurrency,
		Blockers: []SampleDataBlocker{},
	}
	for _, requirement := range []struct{ id, code, message string }{
		{accounting.BankAccountID, "ACCOUNT_1121_MISSING", "Thiếu tài khoản 1121 đang hoạt động và cho phép hạch toán"},
		{accounting.CostAccountID, "ACCOUNT_154_MISSING", "Thiếu tài khoản 154 đang hoạt động và cho phép hạch toán"},
		{accounting.RevenueAccountID, "ACCOUNT_5111_MISSING", "Thiếu tài khoản 5111 đang hoạt động và cho phép hạch toán"},
	} {
		if requirement.id == "" {
			option.Blockers = append(option.Blockers, SampleDataBlocker{Code: requirement.code, Message: requirement.message})
		}
	}
	if !accounting.HasOpenPeriod {
		option.Blockers = append(option.Blockers, SampleDataBlocker{Code: "OPEN_PERIOD_MISSING", Message: "Không có kỳ kế toán đang mở cho ngày hiện tại"})
	}
	option.Compatible = len(option.Blockers) == 0
	return option
}

func sampleBlueprintHasNode(blueprint sampleDataBlueprint, businessCode string) bool {
	if businessCode == "" {
		return false
	}
	for _, node := range blueprint.Nodes {
		if strings.TrimSpace(fmt.Sprint(node.Record["business_code"])) == businessCode {
			return true
		}
	}
	return false
}

func sampleBlueprintHasStepBinding(blueprint sampleDataBlueprint, surfaceKind, actionKey string) bool {
	for _, node := range blueprint.Nodes {
		if strings.TrimSpace(fmt.Sprint(node.Record["business_code"])) == "boq_without_design" {
			continue
		}
		for _, step := range node.Steps {
			if strings.TrimSpace(fmt.Sprint(step.Record["surface_kind"])) == surfaceKind &&
				strings.TrimSpace(fmt.Sprint(step.Record["domain_action_key"])) == actionKey {
				return true
			}
		}
	}
	return false
}

func sampleSelectedStepCount(blueprint sampleDataBlueprint) int {
	total := 0
	for _, node := range blueprint.Nodes {
		if strings.TrimSpace(fmt.Sprint(node.Record["business_code"])) == "boq_without_design" {
			continue
		}
		total += len(node.Steps)
	}
	return total
}

func (s *Service) CreateSampleDataProject(ctx context.Context, req SampleDataCreateRequest, actorID, actorName string) (Record, error) {
	req.Profile = strings.ToLower(strings.TrimSpace(req.Profile))
	req.PipelineTemplateID = strings.TrimSpace(req.PipelineTemplateID)
	req.AccountingBookID = strings.TrimSpace(req.AccountingBookID)
	if req.Profile == "draft" {
		req.ProgressPercent = 0
	} else if req.Profile == "completed" {
		req.ProgressPercent = 100
	}
	if !validSampleProfileProgress(req.Profile, req.ProgressPercent) {
		return nil, validationError("Loại hồ sơ hoặc tiến độ dữ liệu mẫu không hợp lệ", Record{
			"profile": req.Profile, "progress_percent": req.ProgressPercent, "planning_presets": sampleDataPlanningProgressPresets,
		})
	}
	pipelineID, err := uuid.Parse(req.PipelineTemplateID)
	if err != nil {
		return nil, validationError("Pipeline tạo dữ liệu mẫu không hợp lệ", Record{"field": "pipeline_template_id"})
	}
	req.PipelineTemplateID = pipelineID.String()
	requestedAccountingBookID := ""
	if req.Profile == "completed" {
		accountingBookID, parseErr := uuid.Parse(req.AccountingBookID)
		if parseErr != nil {
			return nil, validationError("Sổ kế toán tạo dữ liệu mẫu không hợp lệ", Record{"field": "accounting_book_id"})
		}
		requestedAccountingBookID = accountingBookID.String()
	}
	idempotencyKey, err := uuid.Parse(strings.TrimSpace(req.IdempotencyKey))
	if err != nil {
		return nil, validationError("Khoá idempotency không hợp lệ", Record{"field": "idempotency_key"})
	}
	if existing, err := s.repo.FindSampleDataGeneration(ctx, idempotencyKey.String()); err != nil {
		return nil, err
	} else if existing != nil {
		sourceMismatch := cleanID(existing["template_id"]) != req.PipelineTemplateID
		if req.Profile == "completed" {
			sourceMismatch = sourceMismatch || cleanID(existing["accounting_book_id"]) != requestedAccountingBookID
		}
		if cleanID(existing["actor_id"]) != actorID || strings.TrimSpace(fmt.Sprint(existing["profile"])) != req.Profile ||
			int(number(existing["progress_percent"])) != req.ProgressPercent || sourceMismatch {
			return nil, conflictError("Khoá idempotency đã được dùng cho một yêu cầu tạo dữ liệu mẫu khác")
		}
		return s.repo.sampleDataProjectResult(ctx, cleanID(existing["project_id"]), true)
	}
	accountingContexts, err := s.repo.ListSampleDataAccountingContexts(ctx)
	if err != nil {
		return nil, err
	}
	var accounting *sampleDataAccountingContext
	if req.Profile == "completed" {
		for index := range accountingContexts {
			if accountingContexts[index].ID == requestedAccountingBookID {
				accounting = &accountingContexts[index]
				break
			}
		}
		if accounting == nil {
			return nil, validationError("Sổ kế toán không hoạt động hoặc không tồn tại", Record{"accounting_book_id": requestedAccountingBookID})
		}
		accountingOption := sampleDataAccountingCompatibility(*accounting)
		if !accountingOption.Compatible {
			return nil, validationError("Sổ kế toán chưa đủ điều kiện tạo hồ sơ hoàn thành", Record{
				"accounting_book_id": requestedAccountingBookID, "blockers": accountingOption.Blockers,
			})
		}
	} else if len(accountingContexts) > 0 {
		accounting = &accountingContexts[0]
	} else {
		return nil, validationError("Không có sổ kế toán đang hoạt động để gắn với công trình mẫu", Record{"profile": req.Profile})
	}
	req.AccountingBookID = accounting.ID
	blueprint, err := s.repo.LoadSampleDataBlueprint(ctx, req.PipelineTemplateID)
	if err != nil {
		return nil, err
	}
	pipelineOption := sampleDataPipelineCompatibility(blueprint)
	if !pipelineOption.Compatible {
		return nil, validationError("Pipeline chưa đủ điều kiện tạo dữ liệu công trình mẫu", Record{
			"pipeline_template_id": req.PipelineTemplateID, "blockers": pipelineOption.Blockers,
		})
	}
	plan, err := buildSampleDataPlan(blueprint, accounting, req, actorID, actorName, idempotencyKey.String(), time.Now())
	if err != nil {
		return nil, err
	}
	objects, err := s.storeSampleDataArtifacts(ctx, &plan)
	if err != nil {
		for _, object := range objects {
			s.deleteStoredObject(object)
		}
		return nil, err
	}
	result, err := s.repo.CreateSampleData(ctx, plan)
	if err != nil {
		if storedMutationCleanupSafe(err) {
			for _, object := range objects {
				s.deleteStoredObject(object)
			}
		}
		return nil, err
	}
	return result, nil
}

func validSampleProfileProgress(profile string, progress int) bool {
	switch profile {
	case "draft":
		return progress == 0
	case "completed":
		return progress == 100
	case "planning":
		for _, preset := range sampleDataPlanningProgressPresets {
			if progress == preset {
				return true
			}
		}
	}
	return false
}

func buildSampleDataPlan(blueprint sampleDataBlueprint, accounting *sampleDataAccountingContext, req SampleDataCreateRequest, actorID, actorName, idempotencyKey string, now time.Time) (sampleDataPlan, error) {
	projectID := uuid.NewString()
	suffix := strings.ToUpper(strings.ReplaceAll(projectID[:8], "-", ""))
	prefix, title, status := "NEW", "Công trình mẫu mới", "draft"
	if req.Profile == "planning" {
		prefix, title, status = "PLAN", fmt.Sprintf("Công trình mẫu đang lập kế hoạch %d%%", req.ProgressPercent), "planning"
	} else if req.Profile == "completed" {
		prefix, title, status = "DONE", "Công trình mẫu đã hoàn thành", "completed"
	}
	plan := sampleDataPlan{
		GenerationID: uuid.NewString(), ProjectID: projectID, RunID: uuid.NewString(), Profile: req.Profile,
		ProgressPercent: req.ProgressPercent, ProjectCode: fmt.Sprintf("DEMO-%s-%s-%s", prefix, now.Format("060102150405"), suffix),
		ProjectName: title + " · " + now.Format("02/01/2006 15:04"), ProjectStatus: status,
		TemplateID: cleanID(blueprint.Template["id"]), TemplateHash: cleanOptionalText(blueprint.Template["definition_hash"]),
		TemplateRevision: int(number(blueprint.Template["revision_no"])), Edges: blueprint.Edges,
		Actors: map[string][]sampleDataStaffActor{}, GeneratorID: actorID, GeneratorName: actorName, IdempotencyKey: idempotencyKey,
		SelectedStepCount: sampleSelectedStepCount(blueprint),
	}
	if accounting != nil {
		plan.AccountingBookID = accounting.ID
		plan.BankAccountID = accounting.BankAccountID
		plan.CostAccountID = accounting.CostAccountID
		plan.RevenueAccountID = accounting.RevenueAccountID
	}
	if req.Profile == "completed" {
		plan.CompletedAt = now.Format(time.RFC3339)
	}
	for _, roleLabel := range sampleRequiredRoles(blueprint) {
		actors := sampleActorsForRoleLabel(blueprint, roleLabel)
		if len(actors) == 0 {
			return sampleDataPlan{}, validationError("Thiếu tài khoản mẫu đúng vai trò cho Pipeline", Record{"role": roleLabel})
		}
		plan.Actors[roleLabel] = []sampleDataStaffActor{actors[0]}
	}
	seenActors := map[string]bool{}
	for _, actors := range plan.Actors {
		for _, actor := range actors {
			key := actor.StaffID + ":" + actor.RoleID
			if !seenActors[key] {
				seenActors[key] = true
				plan.AllActors = append(plan.AllActors, actor)
			}
		}
	}
	sort.Slice(plan.AllActors, func(i, j int) bool {
		if plan.AllActors[i].StaffID == plan.AllActors[j].StaffID {
			return plan.AllActors[i].RoleName < plan.AllActors[j].RoleName
		}
		return plan.AllActors[i].StaffID < plan.AllActors[j].StaffID
	})
	if len(plan.AllActors) == 0 {
		return sampleDataPlan{}, validationError("Pipeline chưa có tài khoản mẫu sẵn sàng")
	}
	simulatedActor := plan.AllActors[0]
	for _, actor := range plan.AllActors {
		if strings.EqualFold(actor.RoleKey, "pm") {
			simulatedActor = actor
			break
		}
	}
	plan.ActorID = simulatedActor.UserID
	plan.ActorName = simulatedActor.FullName
	if req.Profile == "draft" {
		return plan, nil
	}
	completedStepCount := req.ProgressPercent * plan.SelectedStepCount / 100
	selectedOrdinal := 0
	for _, nodeDefinition := range blueprint.Nodes {
		selected := strings.TrimSpace(fmt.Sprint(nodeDefinition.Record["business_code"])) != "boq_without_design"
		node := sampleDataPlannedNode{Definition: nodeDefinition, RunNodeID: uuid.NewString(), Selected: selected, Status: "pending"}
		for _, stepDefinition := range nodeDefinition.Steps {
			step := sampleDataPlannedStep{
				Definition: stepDefinition, RunNodeID: node.RunNodeID, RunStepID: uuid.NewString(), WorkItemID: uuid.NewString(),
				Selected: selected, Status: "pending", WorkStatus: "pending", Answers: map[string]any{},
			}
			if formVersionID := cleanID(stepDefinition.Record["form_version_id"]); selected && formVersionID != "" {
				step.FormInstanceID = uuid.NewString()
			}
			if !selected {
				step.Status, step.WorkStatus = "skipped", "cancelled"
			} else {
				roles := stringSlice(stepDefinition.Record["required_role_ids"])
				if len(roles) == 0 {
					roles = stringSlice(nodeDefinition.Record["responsible_role_ids"])
				}
				actors := []sampleDataStaffActor{}
				if len(roles) > 0 {
					actors = sampleActorsForRoleLabel(blueprint, roles[0])
				}
				if len(roles) == 0 || len(actors) == 0 {
					return sampleDataPlan{}, validationError("Thiếu nhân sự đúng vai trò cho bước dữ liệu mẫu", Record{"step": stepDefinition.Record["name"], "roles": roles})
				}
				step.Actor = actors[0]
				step.Completed = selectedOrdinal < completedStepCount
				if step.Completed {
					step.Status, step.WorkStatus = "completed", "completed"
					for _, field := range stepDefinition.Fields {
						step.Answers[strings.TrimSpace(fmt.Sprint(field["field_key"]))] = sampleFormFieldValue(field, plan.ProjectCode, now)
					}
				} else if selectedOrdinal == completedStepCount {
					step.WorkStatus = "ready"
					plan.CurrentNodeID = node.RunNodeID
				}
				selectedOrdinal++
			}
			node.Steps = append(node.Steps, step)
		}
		if !selected {
			node.Status = "skipped"
		} else {
			allComplete, hasReady := true, false
			for _, step := range node.Steps {
				allComplete = allComplete && step.Completed
				hasReady = hasReady || step.WorkStatus == "ready"
			}
			if allComplete {
				node.Status = "completed"
			} else if hasReady {
				node.Status = "in_progress"
			}
		}
		plan.Nodes = append(plan.Nodes, node)
	}
	if req.Profile == "completed" {
		plan.CurrentNodeID = ""
	}
	return plan, nil
}

func sampleActorsForRoleLabel(blueprint sampleDataBlueprint, roleLabel string) []sampleDataStaffActor {
	if normalizeRoleLabel(roleLabel) == "pm" {
		matches := sampleRoleMatches(blueprint, roleLabel)
		if len(matches) != 1 {
			return nil
		}
		// Resolve through the catalog first; a custom PM-labelled actor must not
		// win a map iteration against the renamed canonical Project Manager.
		roleLabel = matches[0].RoleName
	}
	for name, actors := range blueprint.Actors {
		if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(roleLabel)) {
			return actors
		}
	}
	return nil
}

func sampleFormFieldValue(field Record, projectCode string, now time.Time) any {
	fieldType := strings.TrimSpace(fmt.Sprint(field["field_type"]))
	label := strings.TrimSpace(fmt.Sprint(field["label"]))
	switch fieldType {
	case "checkbox", "boolean":
		return true
	case "date":
		return now.Format("2006-01-02")
	case "datetime":
		return now.Format(time.RFC3339)
	case "number", "percentage":
		return 1
	case "currency":
		return 1000000
	case "select", "radio":
		return sampleFirstOption(field["options_json"])
	case "multi_select":
		return []any{sampleFirstOption(field["options_json"])}
	case "structured_list":
		row := Record{"title": "Hạng mục dữ liệu mẫu", "status": "done", "result": "pass", "note": "Đã kiểm tra"}
		validation, _ := field["validation_json"].(map[string]any)
		for _, spec := range recordsFromAny(validation["item_fields"]) {
			key := strings.TrimSpace(fmt.Sprint(spec["key"]))
			if key != "" {
				row[key] = sampleFormFieldValue(spec, projectCode, now)
			}
		}
		return []any{row}
	case "evidence_reference":
		return "sample-evidence-linked"
	case "textarea":
		return fmt.Sprintf("%s – %s đã được giả lập, kiểm tra và lưu lịch sử đầy đủ.", projectCode, label)
	default:
		return fmt.Sprintf("%s – %s", projectCode, label)
	}
}

func sampleFirstOption(raw any) any {
	options := recordsFromAny(raw)
	for _, option := range options {
		value := option["value"]
		if strings.TrimSpace(fmt.Sprint(value)) != "other" && strings.TrimSpace(fmt.Sprint(value)) != "" {
			return value
		}
	}
	if len(options) > 0 {
		return options[0]["value"]
	}
	if values, ok := raw.([]any); ok && len(values) > 0 {
		return values[0]
	}
	return "confirmed"
}

func recordsFromAny(value any) []Record {
	out := []Record{}
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if record, ok := item.(map[string]any); ok {
				out = append(out, Record(record))
			}
		}
	case []Record:
		out = typed
	}
	return out
}

func (s *Service) storeSampleDataArtifacts(ctx context.Context, plan *sampleDataPlan) ([]storedObjectRef, error) {
	if plan.Profile == "draft" {
		return nil, nil
	}
	objects := []storedObjectRef{}
	for nodeIndex := range plan.Nodes {
		for stepIndex := range plan.Nodes[nodeIndex].Steps {
			step := &plan.Nodes[nodeIndex].Steps[stepIndex]
			if !step.Completed {
				continue
			}
			pdfBytes, err := sampleDataPDF(plan.ProjectName, fmt.Sprint(step.Definition.Record["name"]), "Minh chứng lịch sử dữ liệu mẫu", plan.ActorName, time.Now())
			if err != nil {
				return objects, err
			}
			artifact, object, err := s.storeSampleArtifact(ctx, plan.ProjectID, step.WorkItemID, uuid.NewString(), "minh-chung.pdf", "application/pdf", pdfBytes, "evidence")
			if err != nil {
				return objects, err
			}
			objects = append(objects, object)
			step.Evidence = append(step.Evidence, artifact)
			for _, field := range step.Definition.Fields {
				if strings.TrimSpace(fmt.Sprint(field["field_type"])) == "evidence_reference" {
					step.Answers[strings.TrimSpace(fmt.Sprint(field["field_key"]))] = artifact.ID
				}
			}
			if boolValue(step.Definition.Record["photo_required"]) {
				pngBytes, pngErr := sampleDataPNG()
				if pngErr != nil {
					return objects, pngErr
				}
				photo, photoObject, putErr := s.storeSampleArtifact(ctx, plan.ProjectID, step.WorkItemID, uuid.NewString(), "anh-hien-truong.png", "image/png", pngBytes, "evidence")
				if putErr != nil {
					return objects, putErr
				}
				objects = append(objects, photoObject)
				step.Evidence = append(step.Evidence, photo)
			}
			if step.FormInstanceID != "" {
				formPDF, _, pdfErr := GenerateWorkflowFormPDFWithContext(sampleWorkflowPDFRecord(*plan, *step), WorkflowFormPDFContext{
					GeneratedAt: time.Now(), GeneratedBy: plan.ActorName, ProjectCode: plan.ProjectCode,
					ProjectName: plan.ProjectName, NodeName: fmt.Sprint(plan.Nodes[nodeIndex].Definition.Record["name"]),
					NodeOrder: nodeIndex + 1, StepName: fmt.Sprint(step.Definition.Record["name"]), StepOrder: stepIndex + 1,
					WorkItemTitle: fmt.Sprint(step.Definition.Record["name"]),
				})
				if pdfErr != nil {
					return objects, pdfErr
				}
				formArtifact, formObject, putErr := s.storeSampleArtifact(ctx, plan.ProjectID, step.WorkItemID, uuid.NewString(), "bieu-mau.pdf", "application/pdf", formPDF, "workflow-form-pdfs")
				if putErr != nil {
					return objects, putErr
				}
				objects = append(objects, formObject)
				step.FormPDF = &formArtifact
			}
		}
	}
	return objects, nil
}

func (s *Service) storeSampleArtifact(ctx context.Context, projectID, workItemID, artifactID, fileName, mimeType string, content []byte, kind string) (sampleDataArtifact, storedObjectRef, error) {
	storageKey := path.Join("sample-data", projectID, kind, workItemID, artifactID+path.Ext(fileName))
	stored, object, err := s.storeGeneratedFile(ctx, storageKey, content, mimeType)
	if err != nil {
		return sampleDataArtifact{}, storedObjectRef{}, err
	}
	return sampleDataArtifact{
		ID: artifactID, Provider: stored.StorageProvider, Key: stored.StorageKey, Name: fileName,
		MIMEType: stored.MIMEType, Size: stored.FileSize, Checksum: stored.ChecksumSHA256,
	}, object, nil
}

func sampleWorkflowPDFRecord(plan sampleDataPlan, step sampleDataPlannedStep) Record {
	answers := Record{}
	for key, value := range step.Answers {
		answers[key] = value
	}
	return Record{
		"instance": Record{"id": step.FormInstanceID, "project_id": plan.ProjectID, "work_item_id": step.WorkItemID,
			"title": step.Definition.Record["name"], "form_type": "generic_submission", "status": "submitted", "current_revision": 1,
			"submitted_at": time.Now().Add(-48 * time.Hour).Format(time.RFC3339)},
		"template": Record{"name": step.Definition.Record["name"], "code": step.Definition.Record["business_code"]},
		"version":  Record{"version_no": 1}, "answers": answers, "fields": sampleFieldRecords(step.Definition.Fields),
		"signatures":        []Record{{"purpose": "approved_by", "signer_name": step.Actor.FullName, "signed_at": time.Now().Add(-47 * time.Hour).Format(time.RFC3339), "payload_hash": strings.Repeat("a", 64)}},
		"seal_applications": []Record{}, "pdf_confirmation_preferences": []Record{},
	}
}

func sampleFieldRecords(fields []Record) []Record { return fields }

func sampleDataPDF(projectName, title, body, actor string, generatedAt time.Time) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(true)
	pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "", pipelinePDFRegularFont)
	pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "B", pipelinePDFBoldFont)
	pdf.SetCreationDate(generatedAt.UTC())
	pdf.AddPage()
	pdf.SetFont(pipelinePDFFontFamily, "B", 16)
	pdf.MultiCell(180, 9, title, "", "L", false)
	pdf.Ln(3)
	pdf.SetFont(pipelinePDFFontFamily, "", 10)
	pdf.MultiCell(180, 6, "Công trình: "+projectName, "", "L", false)
	pdf.MultiCell(180, 6, body, "", "L", false)
	pdf.MultiCell(180, 6, "Dữ liệu được giả lập bởi: "+actor, "", "L", false)
	var buffer bytes.Buffer
	if err := pdf.Output(&buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func sampleDataPNG() ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, 960, 540))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.RGBA{R: 235, G: 244, B: 255, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(48, 48, 912, 492), &image.Uniform{C: color.RGBA{R: 37, G: 99, B: 235, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(72, 72, 888, 468), &image.Uniform{C: color.RGBA{R: 255, G: 255, B: 255, A: 255}}, image.Point{}, draw.Src)
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func sampleAnswersJSON(answers map[string]any) string {
	raw, _ := json.Marshal(answers)
	return string(raw)
}
