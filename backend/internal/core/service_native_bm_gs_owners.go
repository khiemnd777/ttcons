package core

import (
	"context"
	"fmt"
	"strings"
)

func (s *Service) nativeWorkflowExportSource(ctx context.Context, id, actorID string) (Record, error) {
	if !validActivityUUID(id) {
		return nil, validationError("Phiếu không hợp lệ")
	}
	identity, err := s.repo.QueryOne(ctx, `SELECT project_id::text,work_item_id::text,status FROM workflow_form_instances WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	if statusIn(cleanText(identity["status"]), "cancelled", "deleted") {
		return nil, validationError("Phiếu đã hủy")
	}
	workItemID := cleanID(identity["work_item_id"])
	if workItemID == "" {
		return nil, validationError("Phiếu chưa liên kết Đầu việc nghiệp vụ")
	}
	allowed, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem phiếu của Đầu việc này")
	}
	row, err := s.repo.QueryOne(ctx, `SELECT instance.*,template.code AS form_code,project.code AS project_code,project.name AS project_name,project.address AS project_address,author.full_name AS author_name
	 FROM workflow_form_instances instance JOIN workflow_form_templates template ON template.id=instance.form_template_id
	 JOIN workflow_form_versions version ON version.id=instance.form_version_id AND version.template_id=instance.form_template_id
	 JOIN projects project ON project.id=instance.project_id LEFT JOIN users author ON author.id=instance.created_by
	 WHERE instance.id=$1 AND project.status <> 'cancelled'`, id)
	if err != nil {
		return nil, err
	}
	answers := Record{}
	for key, value := range recordFromAny(row["answers_snapshot_json"]) {
		answers[key] = nativeBMGSAnswer(value)
	}
	fields, err := s.repo.Query(ctx, `SELECT field_key,field_type,options_json,validation_json FROM workflow_form_fields WHERE form_version_id=$1 ORDER BY order_index,id`, cleanID(row["form_version_id"]))
	if err != nil {
		return nil, err
	}
	// Export presentation uses the instance's immutable pinned options. The
	// current catalog revision must never relabel a historical saved selection.
	row["fields"] = fields
	ids := []string{}
	seen := map[string]bool{}
	appendEvidence := func(value any) error {
		id := cleanID(nativeBMGSAnswer(value))
		if id == "" {
			return nil
		}
		if !validActivityUUID(id) {
			return validationError("Ảnh liên kết trong phiếu không hợp lệ")
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
		return nil
	}
	for _, field := range fields {
		value := answers[cleanText(field["field_key"])]
		if cleanText(field["field_type"]) == "evidence_reference" {
			if err = appendEvidence(value); err != nil {
				return nil, err
			}
		}
		if cleanText(field["field_type"]) == "structured_list" {
			for _, itemField := range workflowAnswerRecordList(recordFromAny(field["validation_json"])["item_fields"]) {
				if cleanText(itemField["type"]) != "evidence_reference" {
					continue
				}
				for _, item := range workflowAnswerRecordList(value) {
					if err = appendEvidence(item[cleanText(itemField["key"])]); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	row["photos"] = ids
	return row, nil
}

func (s *Service) ExportNativeConstructionActions(ctx context.Context, projectID string, filter NativeConstructionActionFilter, actorID, actorName string) (tabularExportResult, error) {
	filter.Search = strings.TrimSpace(filter.Search)
	if len([]rune(filter.Search)) > 120 {
		return tabularExportResult{}, validationError("Từ khóa tìm kiếm không được vượt quá 120 ký tự")
	}
	if filter.Queue == "" {
		filter.Queue = "open"
	}
	if !statusIn(filter.Queue, "open", "waiting_confirmation", "done", "all") {
		return tabularExportResult{}, validationError("Trạng thái danh sách cần xử lý không hợp lệ")
	}
	if filter.AssigneeID != "" && !validActivityUUID(filter.AssigneeID) {
		return tabularExportResult{}, validationError("Người xử lý không hợp lệ")
	}
	if filter.Priority != "" && !statusIn(filter.Priority, "urgent", "normal") {
		return tabularExportResult{}, validationError("Ưu tiên không hợp lệ")
	}
	if filter.ImpactScope != "" && !statusIn(filter.ImpactScope, "none", "timeline", "budget", "design_scope") {
		return tabularExportResult{}, validationError("Ảnh hưởng không hợp lệ")
	}
	if _, err := s.nativeExportProject(ctx, projectID, actorID, "operation.view"); err != nil {
		return tabularExportResult{}, err
	}
	rows, err := s.repo.nativeConstructionActions(ctx, projectID, actorID, filter)
	if err != nil {
		return tabularExportResult{}, err
	}
	if len(rows) > 5000 {
		return tabularExportResult{}, validationError("Phạm vi có hơn 5000 lỗi; thu hẹp bộ lọc tại danh sách")
	}
	records := []nativeBMGSRecord{}
	for _, candidate := range rows {
		row, e := s.nativeConstructionActionSource(ctx, cleanID(candidate["id"]), actorID)
		if e != nil {
			return tabularExportResult{}, e
		}
		records = append(records, nativeBMGSRecord{BMGSSourceRef{Kind: "action", ID: cleanID(candidate["id"])}, row})
	}
	return s.exportNativeRecords(ctx, "BM-GS-04", records, actorID, actorName)
}

func (s *Service) ExportNativeWorkflowForm(ctx context.Context, id, purpose, actorID, actorName string) (tabularExportResult, error) {
	row, err := s.nativeWorkflowExportSource(ctx, id, actorID)
	if err != nil {
		return tabularExportResult{}, err
	}
	formCode := cleanText(row["form_code"])
	if formCode == "builtin_construction_daily_report" {
		row = nativeWorkflowDailyExportRow(row)
		if err = s.nativeWorkflowDailySources(ctx, row, actorID); err != nil {
			return tabularExportResult{}, err
		}
	}
	code, kind := "", "workflow_form"
	switch purpose {
	case "risks":
		if !statusIn(formCode, "client_site_risk_assessment", "client_survey_work_package", "builtin_construction_daily_report") {
			return tabularExportResult{}, validationError("Phiếu này không phải nguồn rủi ro khảo sát hoặc vướng mắc báo cáo ngày")
		}
		code = "BM-GS-05"
	case "next_plan":
		if formCode != "builtin_construction_daily_report" {
			return tabularExportResult{}, validationError("Phiếu này không phải kế hoạch hiện trường của báo cáo ngày")
		}
		code = "BM-GS-06"
	default:
		switch formCode {
		case "builtin_construction_daily_report":
			code = "BM-GS-02"
		case "client_survey_work_package", "client_site_measurement_record":
			code = "BM-GS-01"
		case "client_site_risk_assessment":
			code = "BM-GS-05"
		case "builtin_subcontractor_checklist", "client_subcontractor_acceptance_package":
			code = "BM-GS-03"
			kind = "subcontractor_checklist"
		default:
			return tabularExportResult{}, validationError("Phiếu này chưa có mapping biểu mẫu client tương đương")
		}
	}
	return s.exportNativeRecords(ctx, code, []nativeBMGSRecord{{BMGSSourceRef{Kind: kind, ID: id}, row}}, actorID, actorName)
}

// Daily form exports retain the pinned saved answers. A synchronized daily row
// and the creator's current profile are not substitutes for captured answers.
func nativeWorkflowDailyExportRow(source Record) Record {
	row := bmgsCopy(source)
	answers := Record{}
	for key, value := range recordFromAny(source["answers_snapshot_json"]) {
		answers[key] = nativeBMGSAnswer(value)
	}
	row["structured_json"] = answers
	for _, key := range []string{"report_date", "reported_by", "weather", "progress_percent", "work_summary", "manpower_summary", "material_usage_summary", "issues_summary", "field_notes", "notes"} {
		row[key] = answers[key]
	}
	row["author_name"] = answers["reported_by"]
	return row
}

func (s *Service) ExportNativeDailyBusiness(ctx context.Context, id, purpose, actorID, actorName string) (tabularExportResult, error) {
	row, err := s.nativeDailySource(ctx, id, actorID)
	if err != nil {
		return tabularExportResult{}, err
	}
	code := "BM-GS-05"
	if purpose == "next_plan" {
		code = "BM-GS-06"
	}
	return s.exportNativeRecords(ctx, code, []nativeBMGSRecord{{BMGSSourceRef{Kind: "daily_report", ID: id}, row}}, actorID, actorName)
}

func (s *Service) ExportNativeConstructionAction(ctx context.Context, id, actorID, actorName string) (tabularExportResult, error) {
	row, err := s.nativeConstructionActionSource(ctx, id, actorID)
	if err != nil {
		return tabularExportResult{}, err
	}
	return s.exportNativeRecords(ctx, "BM-GS-04", []nativeBMGSRecord{{BMGSSourceRef{Kind: "action", ID: id}, row}}, actorID, actorName)
}

func (s *Service) nativeConstructionActionSource(ctx context.Context, id, actorID string) (Record, error) {
	if !validActivityUUID(id) {
		return nil, validationError("Hồ sơ cần xử lý không hợp lệ")
	}
	// Empty native location writes become SQL NULL. The existing audited field
	// presence distinguishes an explicit clear from an action with no override.
	row, err := s.repo.QueryOne(ctx, `SELECT action.*,COALESCE(action.work_item_id,check_source.work_item_id)::text AS effective_work_item_id,
		CASE WHEN action.location IS NOT NULL OR EXISTS (
			SELECT 1 FROM audit_logs location_write
			WHERE location_write.project_id=action.project_id AND location_write.target_id=action.id
			AND location_write.target_type='construction_action' AND location_write.event_type='construction_action_field_context_updated'
			AND location_write.metadata_json->'after' ? 'location'
		) THEN action.location ELSE check_source.location END AS export_location,
		project.code AS project_code,project.name AS project_name,project.address AS project_address
		FROM construction_action_items action
		LEFT JOIN construction_checks check_source ON check_source.id=action.source_check_id AND check_source.project_id=action.project_id AND check_source.status <> 'cancelled'
		JOIN projects project ON project.id=action.project_id
		WHERE action.id=$1 AND action.status <> 'cancelled' AND project.status <> 'cancelled'`, id)
	if err != nil {
		return nil, err
	}
	if _, err = s.nativeExportProject(ctx, cleanID(row["project_id"]), actorID, "operation.view"); err != nil {
		return nil, err
	}
	if workItemID := cleanID(row["effective_work_item_id"]); workItemID != "" {
		item, e := s.repo.QueryOne(ctx, `SELECT project_id::text FROM work_items WHERE id=$1 AND status <> 'cancelled'`, workItemID)
		if e != nil {
			return nil, e
		}
		if cleanID(item["project_id"]) != cleanID(row["project_id"]) {
			return nil, forbiddenError("Đầu việc không thuộc công trình của lỗi")
		}
		allowed, e := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
		if e != nil {
			return nil, e
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem Đầu việc của lỗi")
		}
		row["work_item_id"] = workItemID
	}
	row["photos"] = []string{cleanID(row["before_evidence_id"]), cleanID(row["resolution_evidence_id"])}
	return row, nil
}

func (s *Service) ExportNativeWorkItemPhotos(ctx context.Context, id, actorID, actorName string) (tabularExportResult, error) {
	if !validActivityUUID(id) {
		return tabularExportResult{}, validationError("Đầu việc không hợp lệ")
	}
	allowed, err := s.repo.CanViewWorkItem(ctx, id, actorID)
	if err != nil {
		return tabularExportResult{}, err
	}
	if !allowed {
		return tabularExportResult{}, forbiddenError("Bạn không có quyền xem ảnh của Đầu việc")
	}
	project, err := s.repo.QueryOne(ctx, `SELECT project.id::text,project.code,project.name,project.address FROM work_items item JOIN projects project ON project.id=item.project_id WHERE item.id=$1 AND item.status <> 'cancelled' AND project.status <> 'cancelled'`, id)
	if err != nil {
		return tabularExportResult{}, err
	}
	photos, err := s.repo.Query(ctx, `SELECT id::text FROM evidence_records WHERE work_item_id=$1 AND evidence_type='photo' AND status NOT IN ('rejected','deleted','cancelled') ORDER BY created_at,id LIMIT 251`, id)
	if err != nil {
		return tabularExportResult{}, err
	}
	if len(photos) > nativeBMGSMaxImages {
		return tabularExportResult{}, validationError("Đầu việc có hơn 250 ảnh; phạm vi vượt giới hạn workbook")
	}
	records := []nativeBMGSRecord{}
	for _, candidate := range photos {
		photo, e := s.EvidenceDetail(ctx, cleanID(candidate["id"]), actorID)
		if e != nil {
			return tabularExportResult{}, e
		}
		if cleanID(photo["project_id"]) != cleanID(project["id"]) {
			return tabularExportResult{}, forbiddenError("Ảnh không thuộc công trình của Đầu việc")
		}
		photo["project_code"] = project["code"]
		photo["project_name"] = project["name"]
		photo["project_address"] = project["address"]
		photo["photos"] = []string{cleanID(photo["id"])}
		records = append(records, nativeBMGSRecord{BMGSSourceRef{Kind: "evidence", ID: cleanID(photo["id"])}, photo})
	}
	return s.exportNativeRecords(ctx, "BM-GS-07", records, actorID, actorName)
}

func (s *Service) ExportNativeHandover(ctx context.Context, id, actorID, actorName string) (tabularExportResult, error) {
	if !validActivityUUID(id) {
		return tabularExportResult{}, validationError("Hồ sơ bàn giao không hợp lệ")
	}
	identity, err := s.repo.QueryOne(ctx, `SELECT project_id::text FROM handover_records WHERE id=$1 AND status NOT IN ('deleted','cancelled')`, id)
	if err != nil {
		return tabularExportResult{}, err
	}
	project, err := s.nativeExportProject(ctx, cleanID(identity["project_id"]), actorID, "handover.view")
	if err != nil {
		return tabularExportResult{}, err
	}
	row, err := s.repo.GetHandoverDetail(ctx, id)
	if err != nil {
		return tabularExportResult{}, err
	}
	row["project_code"] = project["code"]
	row["project_name"] = project["name"]
	row["project_address"] = project["address"]
	ids := []string{}
	for _, item := range workflowAnswerRecordList(row["checklist_items"]) {
		if evidenceID := cleanID(item["evidence_id"]); evidenceID != "" {
			ids = append(ids, evidenceID)
		}
	}
	row["photos"] = ids
	return s.exportNativeRecords(ctx, "BM-GS-08", []nativeBMGSRecord{{BMGSSourceRef{Kind: "handover", ID: id}, row}}, actorID, actorName)
}

func nativeSourceDescription(ref BMGSSourceRef) string {
	if ref.Kind == "work_report" {
		return fmt.Sprintf("Báo cáo %s · lần gửi %d", ref.ID, ref.RevisionNo)
	}
	labels := map[string]string{"daily_report": "Báo cáo ngày", "workflow_form": "Phiếu nghiệp vụ", "subcontractor_checklist": "Checklist nhà thầu phụ", "action": "Lỗi và tồn tại", "evidence": "Ảnh hiện trường", "handover": "Hồ sơ bàn giao"}
	return fmt.Sprintf("%s · %s", labels[ref.Kind], ref.ID)
}
