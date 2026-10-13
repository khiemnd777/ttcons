package core

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Native projections read only the supplied source or exact immutable snapshot.
// The former enrichment ledger is intentionally not a fallback for any field.
func nativeBMGSDailySheet(ref BMGSSourceRef, row Record) (BMGSSheet, error) {
	if ref.Kind != "daily_report" && ref.Kind != "work_report" && !(ref.Kind == "workflow_form" && nativeBMGSIsDailyForm(row)) {
		return BMGSSheet{}, fmt.Errorf("daily workbook requires a daily report or exact submitted revision")
	}
	if ref.Kind == "work_report" && ref.RevisionNo < 1 {
		return BMGSSheet{}, fmt.Errorf("historical report requires an exact revision")
	}
	s := nativeBMGSBaseSheet("BM-GS-02", ref, row)
	weather := nativeBMGSAnswer(row["weather"])
	if label, pinned := nativeBMGSPinnedOptionLabelMatch(row, "weather", weather); pinned {
		s.Metadata["weather"] = label
	} else {
		s.Metadata["weather"] = constructionReportWeatherLabel(weather)
	}
	s.Metadata["supervisor"] = row["author_name"]
	structured := recordFromAny(row["structured_json"])
	for _, r := range workflowAnswerRecordList(structured["crew_items"]) {
		s.CrewRows = append(s.CrewRows, Record{"partner": r["crew_name"], "worker_count": r["worker_count"], "representative": r["representative"], "contact": r["contact"], "note": r["note"]})
	}
	for _, r := range workflowAnswerRecordList(structured["material_arrival_items"]) {
		s.MaterialRows = append(s.MaterialRows, Record{"title": r["material_name"], "quantity": r["quantity"], "unit": r["unit"], "specification": r["specification"], "origin": r["origin"], "note": r["note"]})
	}
	nativeBMGSAddText(&s, "Nội dung báo cáo đã lưu", row["work_summary"])
	if value, recorded := nativeBMGSDailyRecordedProgress(row); recorded {
		nativeBMGSAddText(&s, "Tiến độ được người lập ghi nhận", strconv.FormatFloat(value, 'f', -1, 64)+"%")
	}
	completed := []string{}
	for _, r := range workflowAnswerRecordList(structured["completed_items"]) {
		facts := bmgsCopy(r)
		if result := cleanText(r["result"]); result != "" {
			label := map[string]string{"done": "Đã làm", "partial": "Đang làm"}[result]
			if label == "" {
				label = result
			}
			facts["result"] = label
		}
		if line := nativeBMGSDailyFactLine(facts, "title", [][2]string{{"result", "Kết quả đã ghi nhận"}, {"actual_quantity", "Khối lượng đã ghi nhận"}, {"unit", "Đơn vị"}, {"work_area", "Khu vực công việc"}, {"crew_name", "Tổ đội"}, {"material_used", "Vật tư đã dùng"}, {"note", "Ghi chú"}}); line != "" {
			completed = append(completed, line)
		}
	}
	nativeBMGSAddLines(&s, "Công việc và khối lượng đã ghi nhận", completed)
	usage := []string{}
	for _, r := range workflowAnswerRecordList(structured["material_usage_items"]) {
		usage = append(usage, bmgsJoin(r["material_name"], bmgsJoin(r["quantity"], r["unit"]))+nativeBMGSNote(r["note"]))
	}
	nativeBMGSAddLines(&s, "Vật tư sử dụng (không phải vật tư nhập trường)", usage)
	nativeBMGSAddText(&s, "Tóm tắt vật tư sử dụng", row["material_usage_summary"])
	plan := []string{}
	for _, r := range workflowAnswerRecordList(structured["next_plan_items"]) {
		if line := nativeBMGSDailyFactLine(r, "title", [][2]string{{"instruction_code", "Mã chỉ dẫn"}, {"location", "Vị trí"}, {"subcontractor", "Thầu phụ"}, {"receiver", "Người nhận"}, {"contact", "Liên hệ"}, {"issued_date", "Ngày giao đã ghi nhận"}, {"due_date", "Hạn thực hiện"}, {"progress_percent", "Tiến độ đã ghi nhận"}, {"state", "Trạng thái đã ghi nhận"}, {"owner_role", "Vai trò phụ trách"}, {"owner_name", "Người phụ trách"}, {"priority", "Ưu tiên"}, {"description", "Mô tả"}}); line != "" {
			plan = append(plan, line)
		}
	}
	nativeBMGSAddLines(&s, "Kế hoạch tiếp theo đã ghi nhận", plan)
	blockers := []string{}
	for _, r := range workflowAnswerRecordList(structured["blockers"]) {
		if line := nativeBMGSDailyFactLine(r, "content", [][2]string{{"risk_code", "Mã rủi ro"}, {"detected_at", "Ngày phát hiện đã ghi nhận"}, {"cause", "Nguyên nhân"}, {"impact", "Ảnh hưởng"}, {"severity", "Mức độ"}, {"proposed_action", "Biện pháp đề xuất"}, {"owner", "Người phụ trách"}, {"owner_role", "Vai trò phụ trách"}, {"target_date", "Hạn xử lý"}, {"status", "Trạng thái đã ghi nhận"}}); line != "" {
			blockers = append(blockers, line)
		}
	}
	nativeBMGSAddLines(&s, "Vướng mắc đã ghi nhận", blockers)
	nativeBMGSAddText(&s, "Tóm tắt vướng mắc", row["issues_summary"])
	nativeBMGSAddText(&s, "Ghi nhận hiện trường", row["field_notes"])
	nativeBMGSAddText(&s, "Ghi chú báo cáo", row["notes"])
	nativeBMGSAddText(&s, "Tóm tắt nhân lực", row["manpower_summary"])
	nativeBMGSProjectDailyCriteria(&s, row)
	return s, nil
}

// Captured states remain source text, separate from report lifecycle statuses.
func nativeBMGSDailyFactLine(row Record, main string, fields [][2]string) string {
	parts := []string{}
	if !bmgsBlank(row[main]) {
		parts = append(parts, fmt.Sprint(row[main]))
	}
	for _, field := range fields {
		value := row[field[0]]
		if bmgsBlank(value) {
			continue
		}
		text := fmt.Sprint(value)
		if field[0] == "severity" {
			text = fmt.Sprint(bmgsEnum(value, map[string]string{"low": "Thấp", "medium": "Trung bình", "high": "Cao", "critical": "Nghiêm trọng"}))
		}
		if field[0] == "progress_percent" {
			progress, recorded := nativeBMGSDailyProgress(nativeBMGSAnswer(value))
			if !recorded {
				continue
			}
			text = strconv.FormatFloat(progress, 'f', -1, 64) + "%"
		}
		parts = append(parts, field[1]+": "+text)
	}
	return strings.Join(parts, " · ")
}

// Only finite percentages within the source admission range prove progress.
// The source value remains unchanged, including in historical snapshots.
var nativeBMGSDecimalProgress = regexp.MustCompile(`^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$`)

// Shared display authority for native daily exports and immutable report PDFs.
// A synchronized workflow header default does not prove recorded progress.
func nativeBMGSDailyRecordedProgress(source Record) (float64, bool) {
	progress := source["progress_percent"]
	if !bmgsBlank(source["workflow_form_instance_id"]) || nativeBMGSIsDailyForm(source) {
		progress = recordFromAny(source["structured_json"])["progress_percent"]
		// Match the frontend's bounded legacy-wrapper admission without
		// changing the generic answer unwrap used by other source fields.
		for depth := 0; depth < 8; depth++ {
			value, wrapped := recordFromAny(progress)["value"]
			if !wrapped {
				break
			}
			progress = value
		}
	}
	return nativeBMGSDailyProgress(progress)
}

func nativeBMGSDailyProgress(value any) (float64, bool) {
	if value == nil {
		return 0, false
	}
	var text string
	switch v := value.(type) {
	case string:
		text = strings.TrimSpace(v)
		if !nativeBMGSDecimalProgress.MatchString(text) {
			return 0, false
		}
	case json.Number:
		text = string(v)
		if !nativeBMGSDecimalProgress.MatchString(text) {
			return 0, false
		}
	default:
		kind := reflect.TypeOf(value).Kind()
		if kind < reflect.Int || kind > reflect.Float64 || kind == reflect.Uintptr {
			return 0, false
		}
		text = fmt.Sprint(value)
	}
	if text == "" {
		return 0, false
	}
	number, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 || number > 100 {
		return 0, false
	}
	return number, true
}

func nativeBMGSBaseSheet(code string, ref BMGSSourceRef, row Record) BMGSSheet {
	date := bmgsBusinessDate(row["report_date"])
	id := ref.ID
	if len(id) > 8 {
		id = id[:8]
	}
	name := date
	if name == "" {
		name = code
	}
	if id != "" {
		name += "-" + id
	}
	metadata := Record{"source_kind": ref.Kind, "source_id": ref.ID, "revision_no": ref.RevisionNo, "project_id": row["project_id"], "project_code": row["project_code"], "project_name": row["project_name"]}
	historical, _ := row["historical"].(bool)
	if ref.Kind == "work_report" || historical {
		metadata["historical"] = true
		metadata["address"] = row["address"]
	} else {
		metadata["address"] = row["project_address"]
		if value, ok := row["address"]; ok {
			metadata["address"] = value
		}
	}
	return BMGSSheet{Name: name, DateLabel: date, Title: code, Metadata: metadata, Rows: []Record{}}
}

func nativeBMGSProjectSheet(code string, ref BMGSSourceRef, canonical Record) (BMGSSheet, error) {
	if ref.Kind == "work_report" && ref.RevisionNo < 1 {
		return BMGSSheet{}, fmt.Errorf("historical report requires an exact revision")
	}
	if code == "BM-GS-02" {
		return nativeBMGSDailySheet(ref, canonical)
	}
	s := nativeBMGSBaseSheet(code, ref, canonical)
	answers := recordFromAny(canonical["answers_snapshot_json"])
	if pinned, exists := canonical["checklist_answers"]; exists {
		answers = recordFromAny(pinned)
	}
	switch code {
	case "BM-GS-01":
		if ref.Kind != "survey" && ref.Kind != "workflow_form" {
			return s, fmt.Errorf("survey export requires the native survey form")
		}
		s.DateLabel = bmgsBusinessDate(nativeBMGSAnswer(answers["measurement_date"]))
		if s.DateLabel == "" {
			s.DateLabel = bmgsBusinessDate(nativeBMGSAnswer(answers["survey_date"]))
		}
		s.Metadata["survey_unit"] = nativeBMGSAnswer(answers["survey_unit"])
		s.Metadata["client_representative"] = nativeBMGSAnswer(answers["client_representative"])
		for _, r := range workflowAnswerRecordList(nativeBMGSAnswer(answers["dimension_items"])) {
			dimension := bmgsJoin(r["dimension"], r["unit"])
			s.Rows = append(s.Rows, Record{"location": r["area"], "dimensions": dimension, "mep": r["mep"], "existing_condition": r["existing_condition"], "assessment": r["assessment"], "note": r["note"], "source_id": ref.ID})
		}
	case "BM-GS-03":
		formCode := cleanText(canonical["form_code"])
		if formCode == "" {
			formCode = cleanText(canonical["code"])
		}
		if ref.Kind != "subcontractor_checklist" && ref.Kind != "subcontractor_reconciliation" && formCode != "builtin_subcontractor_checklist" {
			return s, fmt.Errorf("internal acceptance export requires the pinned subcontractor checklist; generic acceptance is not equivalent")
		}
		s.DateLabel = bmgsBusinessDate(nativeBMGSAnswer(answers["check_date"]))
		s.Metadata["partner"] = nativeBMGSAnswer(answers["subcontractor_name"])
		for _, key := range []string{"work_package", "work_scope", "scope"} {
			if value := nativeBMGSAnswer(answers[key]); !bmgsBlank(value) {
				s.Metadata["item_title"] = nativeBMGSPinnedOptionLabel(canonical, key, value)
				break
			}
		}
		s.Metadata["reference_no"] = nativeBMGSAnswer(answers["reference_no"])
		s.Metadata["conclusion"] = nativeBMGSAnswer(answers["acceptance_conclusion"])
		quantities := []string{}
		checklistFacts := []string{}
		for _, r := range workflowAnswerRecordList(nativeBMGSAnswer(answers["checklist_items"])) {
			result := nativeBMGSChecklistResult(r)
			if facts := nativeBMGSChecklistSourceFacts(r); facts != "" {
				checklistFacts = append(checklistFacts, cleanText(r["title"])+" · "+facts)
			}
			s.Rows = append(s.Rows, Record{"title": r["title"], "standard": r["standard_reference"], "drawing_revision": r["drawing_reference"], "technical_requirement": r["technical_requirement"], "design_quantity": r["design_quantity"], "actual_quantity": r["actual_quantity"], "unit": r["unit"], "result": result, "source_id": ref.ID})
			parts := []string{}
			for _, pair := range [][2]string{{"claimed_quantity", "Khối lượng đề nghị"}, {"confirmed_quantity", "Khối lượng xác nhận đối soát"}, {"boq_item_ref", "Tham chiếu BOQ"}, {"measurement_note", "Ghi chú đo"}} {
				if v, exists := r[pair[0]]; exists && !bmgsBlank(v) {
					parts = append(parts, pair[1]+": "+fmt.Sprint(v))
				}
			}
			if len(parts) > 0 {
				quantities = append(quantities, cleanText(r["title"])+" · "+strings.Join(parts, " · "))
			}
		}
		nativeBMGSAddLines(&s, "Thông tin đối soát nguồn (không phải khối lượng đo thực tế)", quantities)
		nativeBMGSAddLines(&s, "Trạng thái checklist và phản hồi đối soát nguồn (không thay thế đánh giá chất lượng)", checklistFacts)
		nativeBMGSAddText(&s, "Đại diện thầu phụ đã ghi nhận", nativeBMGSAnswer(answers["subcontractor_representative"]))
		nativeBMGSAddText(&s, "Khu vực thi công", nativeBMGSAnswer(answers["construction_area"]))
		nativeBMGSAddText(&s, "Ghi chú checklist", nativeBMGSAnswer(answers["overall_comment"]))
	case "BM-GS-04":
		if ref.Kind != "action" {
			return s, fmt.Errorf("punch list requires native construction actions")
		}
		s.DateLabel = bmgsVietnamDate(canonical["updated_at"])
		resolutionNote := ""
		if !bmgsBlank(canonical["resolution_note"]) {
			resolutionNote = "Ghi chú khắc phục: " + fmt.Sprint(canonical["resolution_note"])
		}
		location := canonical["location"]
		if inherited, exists := canonical["export_location"]; exists {
			location = inherited
		}
		s.Rows = []Record{{"fault_code": canonical["fault_code"], "location": location, "description": canonical["description"], "severity": canonical["severity"], "partner": canonical["responsible_unit"], "representative": canonical["representative"], "contact": canonical["contact"], "due_at": canonical["due_at"], "status": canonical["status"], "note": resolutionNote, "source_kind": ref.Kind, "source_id": ref.ID}}
		nativeBMGSAddText(&s, "Nội dung lỗi đã ghi nhận", canonical["title"])
	case "BM-GS-05":
		dailyForm := ref.Kind == "workflow_form" && nativeBMGSIsDailyForm(canonical)
		if ref.Kind == "survey" || ref.Kind == "workflow_form" && !dailyForm {
			s.DateLabel = bmgsBusinessDate(nativeBMGSAnswer(answers["measurement_date"]))
			if s.DateLabel == "" {
				s.DateLabel = bmgsBusinessDate(nativeBMGSAnswer(answers["survey_date"]))
			}
			for _, r := range workflowAnswerRecordList(nativeBMGSAnswer(answers["risk_items"])) {
				s.Rows = append(s.Rows, Record{"risk_code": r["issue_code"], "detected_at": r["detected_date"], "description": r["risk"], "cause": r["cause"], "impact": r["impact"], "risk_level": r["severity"], "proposed_action": r["mitigation"], "owner": r["owner"], "due_at": r["due_date"], "status": r["status"], "source_kind": "survey", "source_id": ref.ID})
				s.Rows[len(s.Rows)-1]["status_is_captured_text"] = true
			}
		}
		if ref.Kind == "daily_report" || ref.Kind == "work_report" || dailyForm {
			structured := recordFromAny(canonical["structured_json"])
			for _, r := range workflowAnswerRecordList(structured["blockers"]) {
				s.Rows = append(s.Rows, Record{"risk_code": r["risk_code"], "detected_at": r["detected_at"], "description": r["content"], "cause": r["cause"], "impact": r["impact"], "risk_level": r["severity"], "proposed_action": r["proposed_action"], "owner": r["owner"], "due_at": r["target_date"], "status": r["status"], "source_kind": ref.Kind, "source_id": ref.ID})
				s.Rows[len(s.Rows)-1]["status_is_captured_text"] = true
			}
		}
		if ref.Kind != "survey" && ref.Kind != "workflow_form" && ref.Kind != "daily_report" && ref.Kind != "work_report" {
			return s, fmt.Errorf("risk register requires native survey risks or daily blockers; commercial variations are not interchangeable")
		}
	case "BM-GS-06":
		dailyForm := ref.Kind == "workflow_form" && nativeBMGSIsDailyForm(canonical)
		if ref.Kind == "daily_report" || ref.Kind == "work_report" || dailyForm {
			structured := recordFromAny(canonical["structured_json"])
			for _, r := range workflowAnswerRecordList(structured["next_plan_items"]) {
				s.Rows = append(s.Rows, Record{"instruction_code": r["instruction_code"], "title": r["title"], "location": r["location"], "partner": r["subcontractor"], "representative": r["receiver"], "contact": r["contact"], "issued_at": r["issued_date"], "due_at": r["due_date"], "progress_percent": r["progress_percent"], "status": r["state"], "source_kind": ref.Kind, "source_id": ref.ID})
				s.Rows[len(s.Rows)-1]["status_is_captured_text"] = true
			}
		}
		if ref.Kind == "site_instruction" {
			if cleanText(canonical["source_field_key"]) != "next_plan_items" {
				return s, fmt.Errorf("assignment is not a generated next-plan instruction")
			}
			s.Rows = []Record{{"instruction_code": canonical["instruction_code"], "title": canonical["title"], "location": canonical["location"], "partner": canonical["responsible_unit"], "representative": canonical["representative"], "contact": canonical["contact"], "issued_at": canonical["issued_at"], "due_at": canonical["due_date"], "progress_percent": canonical["progress_percent"], "status": canonical["status"], "source_kind": ref.Kind, "source_id": ref.ID}}
			nativeBMGSAddText(&s, "Người chịu trách nhiệm nội bộ (không phải đại diện thầu phụ)", canonical["assigned_user_name"])
		}
		if ref.Kind != "daily_report" && ref.Kind != "work_report" && ref.Kind != "site_instruction" && !dailyForm {
			return s, fmt.Errorf("site instructions require a native next plan or generated next-plan assignment")
		}
		for _, row := range s.Rows {
			if progress, recorded := nativeBMGSDailyProgress(nativeBMGSAnswer(row["progress_percent"])); recorded {
				row["progress_percent"] = progress
			} else {
				row["progress_percent"] = nil
			}
		}
	case "BM-GS-07":
		if ref.Kind != "evidence" {
			return s, fmt.Errorf("image log requires exact native evidence metadata")
		}
		if strings.TrimSpace(ref.ID) == "" || ref.RevisionNo < 0 {
			return s, fmt.Errorf("image log requires an exact evidence identity")
		}
		if id := cleanID(canonical["id"]); id != "" && id != ref.ID {
			return s, fmt.Errorf("image log evidence identity does not match its source")
		}
		capture := recordFromAny(recordFromAny(canonical["payload_json"])["capture_metadata"])
		s.DateLabel = bmgsVietnamDate(capture["captured_at"])
		technical := canonical["caption"]
		if bmgsBlank(technical) {
			technical = canonical["description"]
		}
		s.Rows = []Record{{"location": capture["location"], "captured_at": capture["captured_at"], "technical_content": technical, "photo_phase": capture["photo_phase"], "filename": canonical["name"], "handover_note": capture["handover_note"], "source_kind": ref.Kind, "source_id": ref.ID}}
		s.Rows[0]["image_key"] = nativeBMGSImageIdentity(ref, ref.ID)
	case "BM-GS-08":
		if ref.Kind != "handover" {
			return s, fmt.Errorf("handover export requires its exact native checklist")
		}
		// Document type (for example partial/final) does not identify the
		// architectural/MEP handover stage requested by the client header.
		s.Metadata["stage"] = canonical["handover_stage"]
		if conclusion := canonical["handover_conclusion"]; !bmgsBlank(conclusion) {
			s.Metadata["conclusion"] = conclusion
		}
		s.DateLabel = bmgsBusinessDate(canonical["handover_date"])
		states := []string{}
		itemNotes := []string{}
		for _, r := range workflowAnswerRecordList(canonical["checklist_items"]) {
			s.Rows = append(s.Rows, Record{"title": r["title"], "requirement": r["requirement"], "result": r["physical_result"], "defect": r["defect"], "due_at": bmgsBusinessDate(r["due_date"]), "source_id": ref.ID})
			if !bmgsBlank(r["status"]) {
				states = append(states, cleanText(r["title"])+" · Trạng thái hồ sơ: "+cleanText(r["status"]))
			}
			if note := cleanText(r["notes"]); note != "" {
				itemNotes = append(itemNotes, cleanText(r["title"])+" · Ghi chú nguồn: "+note)
			}
		}
		nativeBMGSAddLines(&s, "Trạng thái hồ sơ nguồn (không thay thế đánh giá thực tế)", states)
		nativeBMGSAddLines(&s, "Ghi chú hạng mục bàn giao nguồn (không thay thế đánh giá thực tế)", itemNotes)
		nativeBMGSAddText(&s, "Ghi chú bàn giao", canonical["notes"])
		nativeBMGSAddText(&s, "Loại hồ sơ bàn giao nguồn", canonical["type"])
	default:
		return s, fmt.Errorf("unsupported native workbook template")
	}
	return s, nil
}

func nativeBMGSImageIdentity(ref BMGSSourceRef, evidenceID string) string {
	return fmt.Sprintf("%s:%s:%d:%s", ref.Kind, ref.ID, ref.RevisionNo, evidenceID)
}

func nativeBMGSAnswer(v any) any {
	if r := recordFromAny(v); r != nil {
		if answer, exists := r["value"]; exists {
			return answer
		}
	}
	return v
}

// The instance's pinned field options own display labels. Unknown/free-text
// values remain recorded source text; current catalog options are not a fallback.
func nativeBMGSPinnedOptionLabel(canonical Record, key string, value any) any {
	if label, found := nativeBMGSPinnedOptionLabelMatch(canonical, key, value); found {
		return label
	}
	return value
}

func nativeBMGSPinnedOptionLabelMatch(canonical Record, key string, value any) (any, bool) {
	for _, field := range workflowAnswerRecordList(canonical["fields"]) {
		if cleanText(field["field_key"]) != key {
			continue
		}
		for _, option := range workflowAnswerRecordList(field["options_json"]) {
			if cleanText(option["value"]) == cleanText(value) && !bmgsBlank(option["label"]) {
				return option["label"], true
			}
		}
		break
	}
	return nil, false
}

func nativeBMGSIsDailyForm(row Record) bool {
	return cleanText(row["form_code"]) == "builtin_construction_daily_report" || cleanText(row["code"]) == "builtin_construction_daily_report"
}
func nativeBMGSNote(v any) string {
	if bmgsBlank(v) {
		return ""
	}
	return " · " + fmt.Sprint(v)
}
func nativeBMGSAddText(s *BMGSSheet, title string, v any) {
	if !bmgsBlank(v) {
		nativeBMGSAddLines(s, title, []string{fmt.Sprint(v)})
	}
}
func nativeBMGSAddLines(s *BMGSSheet, title string, lines []string) {
	if len(lines) > 0 {
		s.SourceSections = append(s.SourceSections, BMGSSourceSection{Title: title, Lines: lines})
	}
}
func nativeBMGSChecklistResult(r Record) string {
	// Quality is explicitly captured. Completion and reconciliation responses
	// cannot establish a physical assessment, including for legacy records.
	result := map[string]string{"pass": "Đạt", "fail": "Không đạt", "not_assessed": "Chưa đánh giá"}[cleanText(nativeBMGSAnswer(r["quality_result"]))]
	if result == "" {
		result = "Chưa đánh giá"
	}
	if note := cleanText(nativeBMGSAnswer(r["quality_note"])); note != "" {
		result += " · " + note
	}
	return result
}

func nativeBMGSChecklistSourceFacts(r Record) string {
	parts := []string{}
	if status := cleanText(r["status"]); status != "" {
		label := map[string]string{"done": "Đã thực hiện", "not_applicable": "Không áp dụng", "needs_follow_up": "Cần theo dõi", "blocked": "Bị chặn"}[status]
		if label == "" {
			label = status
		}
		parts = append(parts, "Trạng thái checklist: "+label)
	}
	if response := cleanText(r["owner_response"]); response != "" {
		label := map[string]string{"accepted": "Chấp nhận", "adjust_quantity": "Điều chỉnh khối lượng", "rework_required": "Yêu cầu làm lại", "waiting_evidence": "Chờ minh chứng"}[response]
		if label == "" {
			label = response
		}
		parts = append(parts, "Phản hồi đối soát: "+label)
	}
	if comment := cleanText(nativeBMGSAnswer(r["comment"])); comment != "" && comment != cleanText(nativeBMGSAnswer(r["quality_note"])) && comment != cleanText(nativeBMGSAnswer(r["measurement_note"])) {
		parts = append(parts, "Ghi chú nguồn: "+comment)
	}
	if quality := cleanText(nativeBMGSAnswer(r["quality_result"])); quality != "" && quality != "pass" && quality != "fail" && quality != "not_assessed" {
		parts = append(parts, "Giá trị đánh giá nguồn chưa được hỗ trợ: "+quality)
	}
	return strings.Join(parts, " · ")
}

// A register is one scoped table, whereas daily reports and independent survey,
// subcontractor and handover documents retain their own sheet identity.
func nativeBMGSWorkbookSheets(code string, sheets []BMGSSheet) ([]BMGSSheet, error) {
	if len(sheets) < 2 || code != "BM-GS-04" && code != "BM-GS-05" && code != "BM-GS-06" && code != "BM-GS-07" {
		return sheets, nil
	}
	combined := sheets[0]
	combined.Name = code
	combined.Rows = []Record{}
	combined.SourceSections = nil
	projectID := ""
	dates := map[string]bool{}
	for _, s := range sheets {
		id := cleanID(s.Metadata["project_id"])
		if id != "" {
			if projectID != "" && id != projectID {
				return nil, fmt.Errorf("register source project scope mismatch")
			}
			projectID = id
		}
		combined.Rows = append(combined.Rows, s.Rows...)
		combined.SourceSections = append(combined.SourceSections, s.SourceSections...)
		if s.DateLabel != "" {
			dates[s.DateLabel] = true
		}
	}
	values := []string{}
	for date := range dates {
		values = append(values, date)
	}
	sort.Strings(values)
	if len(values) > 1 {
		combined.DateLabel = values[0] + " – " + values[len(values)-1]
	}
	return []BMGSSheet{combined}, nil
}

func bmgsBlank(v any) bool {
	return v == nil || strings.TrimSpace(fmt.Sprint(v)) == "" || fmt.Sprint(v) == "<nil>"
}
func bmgsCopy(r Record) Record {
	out := Record{}
	for key, value := range r {
		out[key] = value
	}
	return out
}

func bmgsVietnamDate(value any) string {
	if date, ok := value.(time.Time); ok {
		if date.IsZero() {
			return ""
		}
		return date.In(time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)).Format("2006-01-02")
	}
	date, err := time.Parse(time.RFC3339, cleanText(value))
	if err != nil {
		return ""
	}
	return date.In(time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)).Format("2006-01-02")
}

func bmgsBusinessDate(value any) string {
	if record := recordFromAny(value); record != nil {
		if actual, ok := record["value"]; ok {
			value = actual
		}
	}
	text := dateOnly(value)
	if _, err := time.Parse("2006-01-02", text); err != nil {
		return ""
	}
	return text
}
