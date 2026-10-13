package core

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"time"
)

var constructionSupervisionCodes = []string{"progress", "workforce", "deliveries", "quality", "completed_quantity", "variation", "mep", "safety", "acceptance", "risk", "images", "instructions"}

func nativeCaptureText(value, field string, limit int) (string, error) {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > limit {
		return "", validationError("Nội dung vượt giới hạn ký tự", Record{"field": field, "max": limit})
	}
	return value, nil
}

var constructionBlockerCaptureKeys = []string{"risk_code", "detected_at", "cause", "impact", "severity", "proposed_action", "owner", "target_date", "status"}
var constructionNextCaptureKeys = []string{"instruction_code", "location", "subcontractor", "receiver", "contact", "issued_date", "due_date", "progress_percent", "state"}

func constructionRowHasCaptureFacts(row Record, keys []string) bool {
	for _, key := range keys {
		if value, exists := row[key]; exists && value != nil {
			if text, ok := value.(string); !ok || strings.TrimSpace(text) != "" {
				return true
			}
		}
	}
	return false
}

func constructionCaptureRowID(value string, seen map[string]bool) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = newID()
	}
	if len(value) != 36 || !validActivityUUID(value) || value == "00000000-0000-0000-0000-000000000000" || seen[value] {
		return "", validationError("Mã dòng không hợp lệ hoặc trùng", Record{"field": "row_id"})
	}
	seen[value] = true
	return value, nil
}

func normalizeConstructionCaptureField(row Record, key string, value *string, limit int, date bool) error {
	if value == nil {
		return nil
	}
	text, err := nativeCaptureText(*value, key, limit)
	if err != nil {
		return err
	}
	if date && text != "" {
		if _, err := time.Parse("2006-01-02", text); err != nil {
			return validationError("Ngày ghi nhận không hợp lệ", Record{"field": key})
		}
	}
	row[key] = text
	return nil
}

func normalizeConstructionNextCapture(row Record, item ConstructionDailyReportWorkItemRequest) error {
	for _, field := range []struct {
		key   string
		value *string
		limit int
		date  bool
	}{{"instruction_code", item.InstructionCode, 80, false}, {"location", item.Location, 200, false}, {"subcontractor", item.Subcontractor, 160, false}, {"receiver", item.Receiver, 120, false}, {"contact", item.Contact, 100, false}, {"issued_date", item.IssuedDate, 10, true}, {"due_date", item.DueDate, 10, true}, {"state", item.State, 80, false}} {
		if err := normalizeConstructionCaptureField(row, field.key, field.value, field.limit, field.date); err != nil {
			return err
		}
	}
	if item.ProgressPercent != nil {
		value := *item.ProgressPercent
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 {
			return validationError("Tiến độ công việc phải từ 0 đến 100", Record{"field": "next_items.progress_percent"})
		}
		row["progress_percent"] = value
	} else if item.progressPresent {
		row["progress_percent"] = nil
	}
	return nil
}

func normalizeConstructionBlockerCapture(row Record, item ConstructionDailyReportBlockerRequest) error {
	for _, field := range []struct {
		key   string
		value *string
		limit int
		date  bool
	}{{"risk_code", item.RiskCode, 80, false}, {"detected_at", item.DetectedAt, 10, true}, {"cause", item.Cause, 1000, false}, {"impact", item.Impact, 1000, false}, {"severity", item.Severity, 20, false}, {"proposed_action", item.ProposedAction, 1000, false}, {"owner", item.Owner, 160, false}, {"target_date", item.TargetDate, 10, true}, {"status", item.Status, 80, false}} {
		if err := normalizeConstructionCaptureField(row, field.key, field.value, field.limit, field.date); err != nil {
			return err
		}
	}
	if item.Severity != nil && cleanText(row["severity"]) != "" && !statusIn(cleanText(row["severity"]), "low", "medium", "high", "critical") {
		return validationError("Mức độ rủi ro không hợp lệ", Record{"field": "blockers.severity"})
	}
	return nil
}

// Preserve only omitted capture facts from the row with the same source-owned
// identity. The caller holds the report lock; absence from the new list removes
// a row and explicit blank/null clears a field.
func mergeConstructionCapturedRows(structured, previous Record) {
	for section, keys := range map[string][]string{"blockers": constructionBlockerCaptureKeys, "next_plan_items": constructionNextCaptureKeys} {
		oldRows := map[string]Record{}
		for _, row := range workflowAnswerRecordList(previous[section]) {
			if id := cleanID(row["row_id"]); id != "" {
				oldRows[id] = row
			}
		}
		for _, row := range workflowAnswerRecordList(structured[section]) {
			old := oldRows[cleanID(row["row_id"])]
			for _, key := range keys {
				if _, supplied := row[key]; !supplied {
					if value, existed := old[key]; existed {
						row[key] = value
					}
				}
			}
		}
	}
}

func constructionRiskPlanAuditRows(structured Record) Record {
	return Record{"blockers": structured["blockers"], "next_plan_items": structured["next_plan_items"]}
}

func normalizeConstructionCrewContact(row Record, representative, contact *string) error {
	for _, field := range []struct {
		key   string
		value *string
		limit int
	}{{"representative", representative, 120}, {"contact", contact, 100}} {
		if field.value == nil {
			continue
		}
		value, err := nativeCaptureText(*field.value, field.key, field.limit)
		if err != nil {
			return err
		}
		row[field.key] = value
	}
	return nil
}

func normalizeConstructionSupervisionAssessments(items []ConstructionSupervisionAssessment) ([]Record, error) {
	if len(items) > 12 {
		return nil, validationError("Báo cáo chỉ có tối đa 12 nội dung giám sát", nil)
	}
	rows := make([]Record, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		code := strings.TrimSpace(item.Code)
		known := false
		for _, candidate := range constructionSupervisionCodes {
			if code == candidate {
				known = true
				break
			}
		}
		if !known || seen[code] {
			return nil, validationError("Nội dung giám sát không hợp lệ hoặc trùng", Record{"code": code})
		}
		assessment := strings.TrimSpace(item.Assessment)
		if !statusIn(assessment, "pass", "fail", "not_applicable", "not_assessed") {
			return nil, validationError("Đánh giá giám sát không hợp lệ", Record{"code": code, "assessment": assessment})
		}
		note, err := nativeCaptureText(item.Note, "supervision_assessments.note", 1000)
		if err != nil {
			return nil, err
		}
		rows = append(rows, Record{"code": code, "assessment": assessment, "note": note})
		seen[code] = true
	}
	return rows, nil
}

func normalizeConstructionMaterialArrivals(items []ConstructionMaterialArrival) ([]Record, error) {
	if len(items) > 200 {
		return nil, validationError("Báo cáo có quá nhiều dòng vật tư nhập hiện trường", nil)
	}
	rows := make([]Record, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		row := Record{}
		for _, field := range []struct {
			key, value string
			limit      int
		}{{"material_name", item.MaterialName, 160}, {"specification", item.Specification, 300}, {"origin", item.Origin, 200}, {"unit", item.Unit, 30}, {"note", item.Note, 800}} {
			value, err := nativeCaptureText(field.value, "material_arrival_items."+field.key, field.limit)
			if err != nil {
				return nil, err
			}
			if value != "" {
				row[field.key] = value
			}
		}
		if len(row) == 0 && item.Quantity == nil {
			continue
		}
		if cleanID(row["material_name"]) == "" {
			return nil, validationError("Dòng nhập hiện trường cần tên vật tư hoặc thiết bị", nil)
		}
		rowID := cleanID(item.RowID)
		if rowID == "" {
			rowID = newID()
		}
		if !validActivityUUID(rowID) || seen[rowID] {
			return nil, validationError("Mã dòng nhập hiện trường không hợp lệ hoặc trùng", nil)
		}
		if item.Quantity != nil {
			if math.IsNaN(*item.Quantity) || math.IsInf(*item.Quantity, 0) || *item.Quantity < 0 {
				return nil, validationError("Số lượng nhập hiện trường phải là số không âm", nil)
			}
			if cleanID(row["unit"]) == "" {
				return nil, validationError("Có số lượng thì phải ghi đơn vị tính", nil)
			}
			row["quantity"] = *item.Quantity
		}
		row["row_id"] = rowID
		seen[rowID] = true
		rows = append(rows, row)
	}
	return rows, nil
}

type EvidenceCaptureMetadata struct {
	CapturedAt   string `json:"captured_at,omitempty"`
	Location     string `json:"location,omitempty"`
	PhotoPhase   string `json:"photo_phase,omitempty"`
	HandoverNote string `json:"handover_note,omitempty"`
}

func normalizeNativeDailyCaptureAnswers(answers Record) error {
	// Native workflow rows keep their complete schema-defined role, date and
	// task fields. Only their hidden identity is normalized here.
	for _, key := range []string{"blockers", "next_plan_items"} {
		if value, exists := answers[key]; exists {
			rows := workflowAnswerRecordList(value)
			seen := map[string]bool{}
			for _, row := range rows {
				id, err := constructionCaptureRowID(cleanID(row["row_id"]), seen)
				if err != nil {
					return err
				}
				row["row_id"] = id
			}
			answers[key] = rows
		}
	}
	if value, exists := answers["supervision_assessments"]; exists {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		var items []ConstructionSupervisionAssessment
		if err = json.Unmarshal(raw, &items); err != nil {
			return validationError("Đánh giá giám sát không hợp lệ", nil)
		}
		rows, err := normalizeConstructionSupervisionAssessments(items)
		if err != nil {
			return err
		}
		answers["supervision_assessments"] = rows
	}
	if value, exists := answers["material_arrival_items"]; exists {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		var items []ConstructionMaterialArrival
		if err = json.Unmarshal(raw, &items); err != nil {
			return validationError("Dòng vật tư nhập hiện trường không hợp lệ", nil)
		}
		rows, err := normalizeConstructionMaterialArrivals(items)
		if err != nil {
			return err
		}
		answers["material_arrival_items"] = rows
	}
	return nil
}

// The existing native upload payload carries capture facts. Upload time and
// file bytes are independent; neither is rewritten or inferred from these fields.
func normalizeEvidenceCaptureMetadata(req *WorkItemEvidenceRequest) error {
	value, exists := req.Payload["capture_metadata"]
	if !exists {
		return nil
	}
	if req.EvidenceType != "photo" {
		return validationError("Thông tin chụp chỉ áp dụng cho ảnh", nil)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return validationError("Thông tin chụp không hợp lệ", nil)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return validationError("Thông tin chụp phải là đối tượng", nil)
	}
	metadata := EvidenceCaptureMetadata{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&metadata); err != nil {
		return validationError("Thông tin chụp có trường hoặc kiểu dữ liệu không hợp lệ", nil)
	}
	metadata.CapturedAt = strings.TrimSpace(metadata.CapturedAt)
	if metadata.CapturedAt != "" {
		if _, err := time.Parse(time.RFC3339, metadata.CapturedAt); err != nil {
			return validationError("Thời điểm chụp cần ngày giờ và múi giờ hợp lệ", nil)
		}
	}
	metadata.Location, err = nativeCaptureText(metadata.Location, "capture_metadata.location", 200)
	if err != nil {
		return err
	}
	metadata.HandoverNote, err = nativeCaptureText(metadata.HandoverNote, "capture_metadata.handover_note", 800)
	if err != nil {
		return err
	}
	metadata.PhotoPhase = strings.TrimSpace(metadata.PhotoPhase)
	if metadata.PhotoPhase != "" && !statusIn(metadata.PhotoPhase, "before", "during", "after", "defect") {
		return validationError("Giai đoạn hoặc mục đích ảnh không hợp lệ", nil)
	}
	// Clone before normalization so validation does not mutate caller-owned payloads.
	payload := Record{}
	for key, value := range req.Payload {
		payload[key] = value
	}
	data, _ := json.Marshal(metadata)
	normalized := Record{}
	_ = json.Unmarshal(data, &normalized)
	payload["capture_metadata"] = normalized
	req.Payload = payload
	return nil
}
