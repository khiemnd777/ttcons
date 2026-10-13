package core

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestBMGSNativeDailyKeepsSourceFactsAndUnassessedCriteria(t *testing.T) {
	row := Record{"report_date": "2026-10-08", "project_code": "OLD", "project_name": "Frozen project", "address": "Frozen address", "work_summary": "Native saved work summary", "material_usage_summary": "Native usage summary", "weather": "Nắng", "author_name": "Người lập đã lưu", "structured_json": Record{
		"crew_items":      []Record{{"crew_name": "Native crew", "worker_count": 0, "representative": "Native representative", "contact": "Native contact"}},
		"completed_items": []Record{{"title": "Native completed item", "result": "done"}}, "next_plan_items": []Record{{"title": "Native next plan"}}, "blockers": []Record{{"content": "Native blocker"}},
		"material_usage_items":   []Record{{"material_name": "Used concrete", "quantity": 3.5, "unit": "m3"}},
		"material_arrival_items": []Record{{"material_name": "Arrived pipe", "quantity": nil, "unit": "m", "origin": "Native origin"}},
	}, "bm_gs_json": Record{"criteria": []Record{{"code": "progress", "assessment": "pass", "note": "REJECTED SIDECAR"}}, "crews": []Record{{"partner": "REJECTED SIDECAR"}}, "deliveries": []Record{{"title": "REJECTED SIDECAR", "quantity": 999}}}}
	s, err := nativeBMGSDailySheet(BMGSSourceRef{Kind: "work_report", ID: "report-source-1", RevisionNo: 2}, row)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.CriteriaRows) != 12 {
		t.Fatal("missing native assessment acquired sidecar/pass")
	}
	if len(s.CrewRows) != 1 || s.CrewRows[0]["partner"] != "Native crew" || s.CrewRows[0]["worker_count"] != 0 {
		t.Fatal(s.CrewRows)
	}
	if len(s.MaterialRows) != 1 || s.MaterialRows[0]["title"] != "Arrived pipe" || s.MaterialRows[0]["quantity"] != nil {
		t.Fatal("usage became an arrival or null became zero", s.MaterialRows)
	}
	out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-02", ProjectName: "Renamed live project", Sheets: []BMGSSheet{s}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	name := f.GetSheetList()[0]
	project, _ := f.GetCellValue(name, "B5")
	if project != "[OLD] Frozen project" {
		t.Fatal(project)
	}
	values, _ := f.GetRows(name)
	text := fmt.Sprint(values)
	for _, value := range []string{"Native saved work summary", "Native completed item", "Native next plan", "Native blocker", "Used concrete", "Vật tư sử dụng (không phải vật tư nhập trường)", "Arrived pipe", "Native representative"} {
		if !strings.Contains(text, value) {
			t.Fatal("native fact lost", value)
		}
	}
	if strings.Contains(text, "REJECTED SIDECAR") {
		t.Fatal("sidecar data exported")
	}
	for r := 10; r <= 21; r++ {
		status, _ := f.GetCellValue(name, fmt.Sprintf("C%d", r))
		if status != "Chưa đánh giá" {
			t.Fatal("unrecorded assessment invented", r, status)
		}
	}
	if _, err = nativeBMGSDailySheet(BMGSSourceRef{Kind: "work_report", RevisionNo: 0}, row); err == nil {
		t.Fatal("unversioned history accepted")
	}
}

func TestBMGSNativeDailyHistoricalRiskPlanDetailsWorkbook(t *testing.T) {
	plan := Record{"title": "Frozen plan", "instruction_code": "CV-009", "location": "Frozen location", "subcontractor": "Frozen contractor", "receiver": "Frozen receiver", "contact": "0900000001", "issued_date": "2026-10-01", "due_date": "2026-10-09", "progress_percent": 0.0, "state": "submitted", "owner_role": "Frozen internal role", "owner_name": "Frozen internal owner", "priority": "Frozen priority", "description": "Frozen description"}
	blocker := Record{"content": "Frozen blocker", "risk_code": "RR-009", "detected_at": "2026-10-02", "cause": "Frozen cause", "impact": "Frozen impact", "severity": "high", "proposed_action": "Frozen mitigation", "owner": "Frozen risk owner", "owner_role": "Frozen risk role", "target_date": "2026-10-10", "status": "reopened"}
	row := Record{"project_name": "Frozen project", "report_date": "2026-10-08", "status": "acknowledged", "progress_percent": 88, "structured_json": Record{"completed_items": []Record{{"title": "Frozen completed work", "result": "partial", "actual_quantity": 0.0, "unit": "m2", "work_area": "Frozen work area", "crew_name": "Frozen crew", "material_used": "Frozen material", "note": "Frozen work note"}, {"title": "Legacy blank work"}}, "next_plan_items": []Record{plan, plan, {"title": "Legacy blank plan"}}, "blockers": []Record{blocker, {"content": "Legacy blank blocker"}}}, "bm_gs_json": Record{"receiver": "LIVE FALLBACK"}}
	s, err := nativeBMGSDailySheet(BMGSSourceRef{Kind: "work_report", ID: "frozen-report", RevisionNo: 7}, row)
	if err != nil {
		t.Fatal(err)
	}
	for _, criterion := range s.CriteriaRows {
		if criterion["assessment"] != "not_assessed" {
			t.Fatal("supplemental facts invented assessments", criterion)
		}
	}
	var planLines, blockerLines []string
	for _, section := range s.SourceSections {
		if section.Title == "Kế hoạch tiếp theo đã ghi nhận" {
			planLines = section.Lines
		}
		if section.Title == "Vướng mắc đã ghi nhận" {
			blockerLines = section.Lines
		}
	}
	if len(planLines) != 3 || planLines[0] != planLines[1] || planLines[2] != "Legacy blank plan" || len(blockerLines) != 2 || blockerLines[1] != "Legacy blank blocker" {
		t.Fatal("duplicates/order/blank fields changed", planLines, blockerLines)
	}
	out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-02", ProjectName: "LIVE FALLBACK", Sheets: []BMGSSheet{s}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := f.GetRows(f.GetSheetList()[0])
	if err != nil {
		t.Fatal(err)
	}
	text := fmt.Sprint(rows)
	for _, want := range []string{"Công việc và khối lượng đã ghi nhận", "Frozen completed work", "Kết quả đã ghi nhận: Đang làm", "Khối lượng đã ghi nhận: 0", "Đơn vị: m2", "Khu vực công việc: Frozen work area", "Tổ đội: Frozen crew", "Vật tư đã dùng: Frozen material", "Ghi chú: Frozen work note", "Legacy blank work"} {
		if !strings.Contains(text, want) {
			t.Fatal("captured completed work detail omitted", want)
		}
	}
	for _, want := range []string{"Frozen project", "Mã chỉ dẫn: CV-009", "Vị trí: Frozen location", "Thầu phụ: Frozen contractor", "Người nhận: Frozen receiver", "Liên hệ: 0900000001", "Ngày giao đã ghi nhận: 2026-10-01", "Hạn thực hiện: 2026-10-09", "Tiến độ đã ghi nhận: 0%", "Trạng thái đã ghi nhận: submitted", "Vai trò phụ trách: Frozen internal role", "Người phụ trách: Frozen internal owner", "Ưu tiên: Frozen priority", "Mô tả: Frozen description", "Mã rủi ro: RR-009", "Ngày phát hiện đã ghi nhận: 2026-10-02", "Nguyên nhân: Frozen cause", "Ảnh hưởng: Frozen impact", "Mức độ: Cao", "Biện pháp đề xuất: Frozen mitigation", "Người phụ trách: Frozen risk owner", "Vai trò phụ trách: Frozen risk role", "Hạn xử lý: 2026-10-10", "Trạng thái đã ghi nhận: reopened"} {
		if !strings.Contains(text, want) {
			t.Fatal("frozen native detail omitted", want)
		}
	}
	for _, unwanted := range []string{"LIVE FALLBACK", "Biểu mẫu đã gửi", "Biểu mẫu mở lại", "Người nhận: Frozen internal owner", "Tiến độ đã ghi nhận: 88%"} {
		if strings.Contains(text, unwanted) {
			t.Fatal("source semantics overwritten", unwanted)
		}
	}
}

func TestBMGSNativeDailyWorkflowProgressAuthorityWorkbook(t *testing.T) {
	for _, kind := range []string{"daily_report", "work_report", "workflow_form"} {
		for _, tc := range []struct {
			name       string
			structured Record
			want       string
		}{{"missing", Record{}, ""}, {"null", Record{"progress_percent": nil}, ""}, {"blank", Record{"progress_percent": " "}, ""}, {"zero", Record{"progress_percent": 0.0}, "0%"}, {"wrapped_zero", Record{"progress_percent": Record{"value": 0.0}}, "0%"}, {"nonnumeric", Record{"progress_percent": "N/A"}, ""}, {"hex_float", Record{"progress_percent": "0x1p2"}, ""}, {"underscore", Record{"progress_percent": "1_0"}, ""}, {"boolean", Record{"progress_percent": false}, ""}, {"wrapped_nonnumeric", Record{"progress_percent": Record{"value": "N/A"}}, ""}, {"infinite", Record{"progress_percent": math.Inf(1)}, ""}, {"nan", Record{"progress_percent": math.NaN()}, ""}, {"positive", Record{"progress_percent": 37.5}, "37.5%"}, {"wrapped_positive", Record{"progress_percent": Record{"value": 37.5}}, "37.5%"}, {"numeric_text", Record{"progress_percent": " 37.5 "}, "37.5%"}, {"negative", Record{"progress_percent": -1.0}, ""}, {"above_range", Record{"progress_percent": 101.0}, ""}} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				row := Record{"workflow_form_instance_id": "native-form", "progress_percent": 0.0, "structured_json": tc.structured}
				if kind == "workflow_form" {
					delete(row, "workflow_form_instance_id")
					row["form_code"] = "builtin_construction_daily_report"
				}
				before := fmt.Sprint(row)
				s, err := nativeBMGSDailySheet(BMGSSourceRef{Kind: kind, ID: "source", RevisionNo: 3}, row)
				if err != nil {
					t.Fatal(err)
				}
				out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-02", Sheets: []BMGSSheet{s}})
				if err != nil {
					t.Fatal(err)
				}
				f, err := excelize.OpenReader(bytes.NewReader(out))
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				rows, err := f.GetRows(f.GetSheetList()[0])
				if err != nil {
					t.Fatal(err)
				}
				text := fmt.Sprint(rows)
				if strings.Contains(text, "Tiến độ được người lập ghi nhận") != (tc.want != "") || tc.want != "" && !strings.Contains(text, tc.want) || tc.want == "" && strings.Contains(text, "%") || tc.want == "37.5%" && strings.Contains(text, "0%") {
					t.Fatal("default header zero mistaken for captured progress", tc.name, text)
				}
				if fmt.Sprint(row) != before {
					t.Fatal("source snapshot mutated")
				}
			})
		}
	}
	manual, err := nativeBMGSDailySheet(BMGSSourceRef{Kind: "daily_report"}, Record{"progress_percent": 0.0, "structured_json": Record{}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, section := range manual.SourceSections {
		if section.Title == "Tiến độ được người lập ghi nhận" && len(section.Lines) == 1 && section.Lines[0] == "0%" {
			found = true
		}
	}
	if !found {
		t.Fatal("manual Operations canonical zero lost")
	}
}

func TestBMGSNativePlanProgressWorkbookEligibility(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
		want  string
	}{{"missing", nil, ""}, {"blank", " ", ""}, {"nonnumeric", "N/A", ""}, {"boolean", false, ""}, {"wrapped_nonnumeric", Record{"value": "N/A"}, ""}, {"negative", -1, ""}, {"above_range", 101, ""}, {"zero", 0, "0"}, {"positive", 37.5, "37.5"}, {"wrapped_positive", Record{"value": 37.5}, "37.5"}} {
		for _, code := range []string{"BM-GS-02", "BM-GS-06"} {
			t.Run(code+"/"+tc.name, func(t *testing.T) {
				row := Record{"workflow_form_instance_id": "frozen-form", "progress_percent": 0, "structured_json": Record{"next_plan_items": []Record{{"title": "Captured plan", "progress_percent": tc.value}}}}
				s, err := nativeBMGSProjectSheet(code, BMGSSourceRef{Kind: "work_report", ID: "report", RevisionNo: 4}, row)
				if err != nil {
					t.Fatal(err)
				}
				out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: code, Sheets: []BMGSSheet{s}})
				if err != nil {
					t.Fatal(err)
				}
				f, err := excelize.OpenReader(bytes.NewReader(out))
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				name := f.GetSheetList()[0]
				if code == "BM-GS-06" {
					cell, err := f.GetCellValue(name, "I8")
					if err != nil || cell != tc.want {
						t.Fatal("invalid plan progress exported or captured value lost", cell, tc.want, err)
					}
				} else {
					rows, _ := f.GetRows(name)
					text := fmt.Sprint(rows)
					if strings.Contains(text, "Tiến độ đã ghi nhận:") != (tc.want != "") || tc.want != "" && !strings.Contains(text, "Tiến độ đã ghi nhận: "+tc.want+"%") || tc.want == "" && strings.Contains(text, "%") {
						t.Fatal("invalid supplemental plan progress exported", tc.name, text)
					}
				}
			})
		}
	}
}

func TestBMGSNativeSurveySubcontractorAndHandoverSemantics(t *testing.T) {
	survey := Record{"answers_snapshot_json": Record{"dimension_items": []Record{{"area": "Native location", "dimension": "12x15", "unit": "m", "note": "Native note", "mep": "Native MEP"}}, "survey_unit": "Native survey unit"}, "bm_gs_json": Record{"locations": []Record{{"location": "REJECTED"}}}}
	s, err := nativeBMGSProjectSheet("BM-GS-01", BMGSSourceRef{Kind: "survey", ID: "survey-source"}, survey)
	if err != nil || s.Rows[0]["location"] != "Native location" || s.Rows[0]["mep"] != "Native MEP" || s.Rows[0]["existing_condition"] != nil {
		t.Fatal(s, err)
	}
	checklist := Record{"form_code": "builtin_subcontractor_checklist", "answers_snapshot_json": Record{"checklist_items": []Record{{"title": "N/A native item", "status": "not_applicable", "owner_response": "accepted", "claimed_quantity": 15.0, "confirmed_quantity": 15.0, "boq_item_ref": "BOQ-001", "unit": "m2"}, {"title": "Measured native item", "status": "done", "design_quantity": 0.0, "actual_quantity": 0.0, "standard_reference": "Native standard", "drawing_reference": "REV-02", "unit": "m2"}}}}
	s, err = nativeBMGSProjectSheet("BM-GS-03", BMGSSourceRef{Kind: "subcontractor_checklist", ID: "checklist-source"}, checklist)
	if err != nil {
		t.Fatal(err)
	}
	if s.Rows[0]["design_quantity"] != nil || s.Rows[0]["actual_quantity"] != nil {
		t.Fatal("claimed/confirmed/BOQ became measured/design quantity", s.Rows[0])
	}
	if s.Rows[0]["result"] != "Chưa đánh giá" || !strings.Contains(fmt.Sprint(s.SourceSections), "Không áp dụng") {
		t.Fatal("N/A+accepted became physical pass", s.Rows[0])
	}
	if s.Rows[1]["design_quantity"] != 0.0 || s.Rows[1]["actual_quantity"] != 0.0 {
		t.Fatal("explicit zero lost")
	}
	if _, err = nativeBMGSProjectSheet("BM-GS-03", BMGSSourceRef{Kind: "acceptance"}, Record{}); err == nil {
		t.Fatal("generic acceptance treated as internal subcontractor acceptance")
	}
	checklist["checklist_answers"] = Record{}
	pinned, err := nativeBMGSProjectSheet("BM-GS-03", BMGSSourceRef{Kind: "subcontractor_reconciliation"}, checklist)
	if err != nil || len(pinned.Rows) != 0 {
		t.Fatal("empty pinned checklist fell back to another/current form", pinned, err)
	}
	handover := Record{"status": "handed_over", "checklist_items": []Record{{"title": "Verified document", "status": "verified", "physical_result": nil}, {"title": "Physical test", "status": "pending", "physical_result": "fail", "requirement": "Native requirement", "defect": "Native defect"}}}
	s, err = nativeBMGSProjectSheet("BM-GS-08", BMGSSourceRef{Kind: "handover"}, handover)
	if err != nil {
		t.Fatal(err)
	}
	if s.Rows[0]["result"] != nil || s.Metadata["conclusion"] != nil {
		t.Fatal("document verified or handed-over became physical pass/eligibility", s)
	}
	if s.Rows[1]["result"] != "fail" || s.Rows[1]["defect"] != "Native defect" {
		t.Fatal(s.Rows[1])
	}
}

func TestBMGSNativeRiskNextPlanAndRegisterGrouping(t *testing.T) {
	row := Record{"project_id": "one-project", "report_date": "2026-10-08", "structured_json": Record{"blockers": []Record{{"content": "Native issue", "severity": "high", "impact": "Native impact", "owner": "Native owner", "target_date": "2026-10-10"}}, "next_plan_items": []Record{{"title": "Native next item"}}}, "status": "acknowledged", "progress_percent": 80}
	risk, err := nativeBMGSProjectSheet("BM-GS-05", BMGSSourceRef{Kind: "daily_report", ID: "daily-one"}, row)
	if err != nil {
		t.Fatal(err)
	}
	if risk.Rows[0]["description"] != "Native issue" || risk.Rows[0]["status"] != nil || risk.Rows[0]["detected_at"] != nil {
		t.Fatal("report status/date leaked as risk outcome/detection", risk.Rows)
	}
	if _, err = nativeBMGSProjectSheet("BM-GS-05", BMGSSourceRef{Kind: "variation"}, Record{}); err == nil {
		t.Fatal("arbitrary variation accepted as field risk")
	}
	plan, err := nativeBMGSProjectSheet("BM-GS-06", BMGSSourceRef{Kind: "daily_report", ID: "daily-one"}, row)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"issued_at", "representative", "progress_percent", "status"} {
		if plan.Rows[0][key] != nil {
			t.Fatal("next plan invented assignment fact", key, plan.Rows[0])
		}
	}
	if _, err = nativeBMGSProjectSheet("BM-GS-06", BMGSSourceRef{Kind: "site_work"}, Record{"title": "Arbitrary WI"}); err == nil {
		t.Fatal("arbitrary WI accepted as site instruction")
	}
	second := risk
	second.Metadata = bmgsCopy(risk.Metadata)
	second.Rows = []Record{{"description": "Second native issue"}}
	sheets, err := nativeBMGSWorkbookSheets("BM-GS-05", []BMGSSheet{risk, second})
	if err != nil || len(sheets) != 1 || len(sheets[0].Rows) != 2 {
		t.Fatal(sheets, err)
	}
	second.Metadata["project_id"] = "other-project"
	if _, err = nativeBMGSWorkbookSheets("BM-GS-05", []BMGSSheet{risk, second}); err == nil {
		t.Fatal("mixed project register accepted")
	}
	daily, _ := nativeBMGSDailySheet(BMGSSourceRef{Kind: "daily_report", ID: "one"}, row)
	sheets, err = nativeBMGSWorkbookSheets("BM-GS-02", []BMGSSheet{daily, daily})
	if err != nil || len(sheets) != 2 {
		t.Fatal("independent same-day daily reports merged")
	}
	action, err := nativeBMGSProjectSheet("BM-GS-04", BMGSSourceRef{Kind: "action"}, Record{"priority": "high", "description": "Native defect", "status": "done"})
	if err != nil || action.Rows[0]["severity"] != nil {
		t.Fatal("priority inferred fault severity", action, err)
	}
}

func TestBMGSNativeActionResolutionNoteWorksheet(t *testing.T) {
	s, err := nativeBMGSProjectSheet("BM-GS-04", BMGSSourceRef{Kind: "action", ID: "native-action"}, Record{"description": "Native defect", "status": "done", "resolution_note": "Đã thay đoạn ống bị hỏng", "notes": "REJECTED NONCANONICAL NOTE"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-04", Sheets: []BMGSSheet{s}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	note, err := f.GetCellValue(f.GetSheetList()[0], "J8")
	if err != nil || note != "Ghi chú khắc phục: Đã thay đoạn ống bị hỏng" {
		t.Fatal("canonical remediation note lost", note, err)
	}
	rows, _ := f.GetRows(f.GetSheetList()[0])
	if strings.Contains(fmt.Sprint(rows), "REJECTED NONCANONICAL NOTE") {
		t.Fatal("noncanonical notes substituted for remediation")
	}
}

func TestBMGSNativeWorkflowDailyFieldMapping(t *testing.T) {
	row := Record{"form_code": "builtin_construction_daily_report", "report_date": "2026-10-08", "structured_json": Record{"next_plan_items": []Record{{"title": "Native assignment", "instruction_code": "CV-001", "location": "Native location", "subcontractor": "Native subcontractor", "receiver": "External receiver", "owner_name": "Internal supervisor", "issued_date": "2026-10-07", "due_date": "2026-10-09", "state": "Đã giao", "progress_percent": 0.0}}, "blockers": []Record{{"content": "Native blocker"}}}}
	ref := BMGSSourceRef{Kind: "workflow_form", ID: "native-form"}
	daily, err := nativeBMGSDailySheet(ref, row)
	if err != nil {
		t.Fatal(err)
	}
	if daily.Metadata["source_kind"] != "workflow_form" {
		t.Fatal("native form identity was disguised")
	}
	plan, err := nativeBMGSProjectSheet("BM-GS-06", ref, row)
	if err != nil {
		t.Fatal(err)
	}
	expected := Record{"title": "Native assignment", "partner": "Native subcontractor", "representative": "External receiver", "issued_at": "2026-10-07", "due_at": "2026-10-09", "progress_percent": 0.0, "status": "Đã giao"}
	for key, want := range expected {
		if plan.Rows[0][key] != want {
			t.Fatal("native next-plan field mapping drift", key, plan.Rows[0][key], want)
		}
	}
	risk, err := nativeBMGSProjectSheet("BM-GS-05", ref, row)
	if err != nil || len(risk.Rows) != 1 || risk.Rows[0]["description"] != "Native blocker" {
		t.Fatal("workflow daily blocker lost", risk, err)
	}
	row["form_code"] = "unrelated_form"
	if _, err = nativeBMGSDailySheet(ref, row); err == nil {
		t.Fatal("unrelated form presented as daily report")
	}
	survey := Record{"answers_snapshot_json": Record{"risk_items": []Record{{"risk": "Native risk", "issue_code": "R-001", "detected_date": "2026-10-06", "due_date": "2026-10-10", "severity": "high", "mitigation": "Native mitigation"}}}}
	s, err := nativeBMGSProjectSheet("BM-GS-05", BMGSSourceRef{Kind: "survey"}, survey)
	if err != nil || s.Rows[0]["risk_code"] != "R-001" || s.Rows[0]["detected_at"] != "2026-10-06" || s.Rows[0]["due_at"] != "2026-10-10" {
		t.Fatal("native risk keys drift", s, err)
	}
	s.Rows[0]["status"] = "submitted"
	out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-05", Sheets: []BMGSSheet{s}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, _ := f.GetCellValue(f.GetSheetList()[0], "K8")
	if got != "submitted" {
		t.Fatal("captured risk status became survey-form lifecycle", got)
	}
}

func TestBMGSNativeImagesAllVersionsAndGIF(t *testing.T) {
	im := image.NewPaletted(image.Rect(0, 0, 30, 20), color.Palette{color.Transparent, color.White})
	im.SetColorIndex(1, 1, 1)
	var raw bytes.Buffer
	if err := gif.Encode(&raw, im, nil); err != nil {
		t.Fatal(err)
	}
	pic, err := prepareBMGSImage(raw.Bytes(), "image/gif")
	if err != nil {
		t.Fatal(err)
	}
	if pic.MIMEType != "image/png" || pic.DerivativeNote != "GIF: khung hình đầu tiên" {
		t.Fatal(pic.MIMEType, pic.DerivativeNote)
	}
	decoded, err := png.Decode(bytes.NewReader(pic.Bytes))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, alpha := decoded.At(0, 0).RGBA()
	if alpha != 0 {
		t.Fatal("GIF transparency lost")
	}
	frame := image.NewPaletted(image.Rect(5, 5, 15, 15), color.Palette{color.Transparent, color.White})
	frame.SetColorIndex(5, 5, 1)
	raw.Reset()
	if err := gif.EncodeAll(&raw, &gif.GIF{Image: []*image.Paletted{frame}, Delay: []int{0}, Config: image.Config{ColorModel: frame.Palette, Width: 30, Height: 20}}); err != nil {
		t.Fatal(err)
	}
	offset, err := prepareBMGSImage(raw.Bytes(), "image/gif")
	if err != nil || offset.Width != 30 || offset.Height != 20 {
		t.Fatal("GIF first frame cropped logical canvas", offset.Width, offset.Height, err)
	}
	images := []BMGSImage{}
	for i := 0; i < 51; i++ {
		p := pic
		p.EvidenceID = "same-original"
		p.SourceRevisionKey = fmt.Sprintf("work_report:report:%d", i+1)
		p.SourceLabel = fmt.Sprintf("work_report 10000000-0000-4000-8000-000000000001 lần %d", i+1)
		p.Caption = fmt.Sprintf("Frozen caption version%d", i+1)
		p.Phase = "defect"
		images = append(images, p)
	}
	out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-02", Sheets: []BMGSSheet{{}}, Images: images})
	if err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("BMGS_QA_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "BM-GS-02-native-51-images.xlsx"), out, 0600); err != nil {
			t.Fatal(err)
		}
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	anchors, err := f.GetPictureCells("Hình ảnh")
	if err != nil || len(anchors) != 51 {
		t.Fatal("linked images/versions truncated or deduplicated across revisions", len(anchors), err)
	}
	rows, _ := f.GetRows("Hình ảnh")
	text := fmt.Sprint(rows)
	for _, want := range []string{"Frozen caption version1", "Frozen caption version25", "Frozen caption version51", "lần 51", "Phát hiện lỗi", "GIF: khung hình đầu tiên"} {
		if !strings.Contains(text, want) {
			t.Fatal("image provenance omitted", want)
		}
	}
	if _, err = generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-07", Sheets: []BMGSSheet{{}}, Images: make([]BMGSImage, bmgsMaxImages+1)}); err == nil {
		t.Fatal("capacity overflow silently accepted")
	}
}
