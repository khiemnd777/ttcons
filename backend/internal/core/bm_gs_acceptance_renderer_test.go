package core

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestBMGSNativeAcceptanceCapturedFactsWorkbook(t *testing.T) {
	for _, decision := range []string{"accepted", "rework_required", "", "pass"} {
		t.Run(decision, func(t *testing.T) {
			rows := []Record{{"title": "Measured", "quality_result": "pass", "quality_note": "Measured assessment", "status": "done", "owner_response": "accepted", "comment": "Legacy comment"}, {"title": "Defect", "quality_result": "fail", "quality_note": "Leak"}, {"title": "Legacy", "status": "done", "owner_response": "accepted"}, {"title": "Fourth"}}
			s, err := nativeBMGSProjectSheet("BM-GS-03", BMGSSourceRef{Kind: "subcontractor_checklist"}, Record{"reference_no": "wrong header", "answers_snapshot_json": Record{"reference_no": Record{"value": "NT-042"}, "check_date": Record{"value": "2026-10-09"}, "work_package": Record{"value": "Waterproofing"}, "subcontractor_name": Record{"value": "Native subcontractor"}, "acceptance_conclusion": Record{"value": decision}, "checklist_items": rows}})
			if err != nil {
				t.Fatal(err)
			}
			f := bmgsAcceptanceReopen(t, "BM-GS-03", s)
			defer f.Close()
			for cell, want := range map[string]string{"B5": "NT-042", "F5": "2026-10-09", "B6": "Waterproofing", "F6": "Native subcontractor", "G9": "Đạt · Measured assessment", "G10": "Không đạt · Leak", "G11": "Chưa đánh giá"} {
				got, _ := f.GetCellValue(f.GetSheetList()[0], cell)
				if got != want {
					t.Errorf("%s = %q, want %q", cell, got, want)
				}
			}
			bmgsAssertConclusion(t, f, "A14", "ĐỒNG Ý NGHIỆM THU", "CẦN SỬA CHỮA TỒN TẠI", decision, "accepted", "rework_required")
			text := bmgsWorkbookText(t, f)
			if !strings.Contains(text, "Legacy comment") || !strings.Contains(text, "GIÁM SÁT CÔNG TRÌNH") || !strings.Contains(text, "ĐẠI DIỆN THẦU PHỤ") {
				t.Fatal("legacy facts or signatures lost")
			}
		})
	}
}

func TestBMGSNativeChecklistQualityIsExplicit(t *testing.T) {
	for _, row := range []Record{
		{"status": "done", "owner_response": "accepted"},
		{"quality_result": "accepted", "status": "done"},
		{"quality_result": "not_assessed"},
	} {
		if got := nativeBMGSChecklistResult(row); got != "Chưa đánh giá" {
			t.Fatalf("legacy state became quality result: %q", got)
		}
	}
	row := Record{"quality_result": Record{"value": "fail"}, "quality_note": Record{"value": "Pressure test failed"}, "comment": "Pressure test failed"}
	if got := nativeBMGSChecklistResult(row); got != "Không đạt · Pressure test failed" {
		t.Fatal(got)
	}
	if got := nativeBMGSChecklistSourceFacts(row); got != "" {
		t.Fatalf("duplicated assessment note: %q", got)
	}
	if got := nativeBMGSChecklistSourceFacts(Record{"quality_result": "legacy-result"}); !strings.Contains(got, "legacy-result") {
		t.Fatal("unknown source value lost")
	}
}

func TestBMGSNativeAcceptancePinnedScopeLabel(t *testing.T) {
	for _, key := range []string{"work_package", "work_scope", "scope"} {
		for _, value := range []string{"saved-key", "Custom scope"} {
			source := Record{"answers_snapshot_json": Record{key: Record{"value": value}}, "fields": []Record{{"field_key": key, "options_json": []Record{{"value": "saved-key", "label": "Pinned scope label"}}}}}
			sheet, err := nativeBMGSProjectSheet("BM-GS-03", BMGSSourceRef{Kind: "subcontractor_checklist"}, source)
			if err != nil {
				t.Fatal(err)
			}
			f := bmgsAcceptanceReopen(t, "BM-GS-03", sheet)
			got, _ := f.GetCellValue(f.GetSheetList()[0], "B6")
			f.Close()
			want := value
			if value == "saved-key" {
				want = "Pinned scope label"
			}
			if got != want {
				t.Fatalf("%s = %q, want %q", key, got, want)
			}
		}
	}
	// An absent pinned schema cannot justify inventing a catalog label.
	if got := nativeBMGSPinnedOptionLabel(Record{}, "work_package", "unrecognized"); got != "unrecognized" {
		t.Fatal(got)
	}
}

func TestBMGSNativeAcceptanceQuantityFormatCapturedPrecision(t *testing.T) {
	rows := []Record{}
	for _, quantity := range []float64{24, 0, 25.75, 23.5, 0.000000001} {
		rows = append(rows, Record{"title": "Measured", "actual_quantity": quantity, "unit": "m2"})
	}
	f := bmgsAcceptanceReopen(t, "BM-GS-03", BMGSSheet{Rows: rows})
	defer f.Close()
	for i, want := range []string{`0 "m2"`, `0 "m2"`, `0.00 "m2"`, `0.0 "m2"`, `0.000000000 "m2"`} {
		cell := fmt.Sprintf("F%d", 9+i)
		styleID, _ := f.GetCellStyle(f.GetSheetList()[0], cell)
		style, _ := f.GetStyle(styleID)
		if style.CustomNumFmt == nil || *style.CustomNumFmt != want {
			t.Fatalf("%s format=%v, want %q", cell, style.CustomNumFmt, want)
		}
		kind, _ := f.GetCellType(f.GetSheetList()[0], cell)
		if kind == excelize.CellTypeSharedString || kind == excelize.CellTypeInlineString {
			t.Fatalf("%s quantity became text", cell)
		}
	}
	got, _ := f.GetCellValue(f.GetSheetList()[0], "F9")
	if got != "24 m2" {
		t.Fatal(got)
	}
	// The other templates keep their existing quantity format.
	g := bmgsAcceptanceReopen(t, "BM-GS-02", BMGSSheet{MaterialRows: []Record{{"title": "Material", "quantity": 24.0, "unit": "m2"}}})
	defer g.Close()
	id, _ := g.GetCellStyle(g.GetSheetList()[0], "G25")
	style, _ := g.GetStyle(id)
	if style.CustomNumFmt == nil || *style.CustomNumFmt != `0.######## "m2"` {
		t.Fatal("unrelated template format changed")
	}
}

func TestBMGSNativeHandoverCapturedDecisionAndDateWorkbook(t *testing.T) {
	for _, decision := range []string{"eligible", "not_eligible", "", "handed_over"} {
		t.Run(decision, func(t *testing.T) {
			rows := []Record{{"title": "First", "physical_result": "fail", "requirement": "Pressure", "defect": "Leak", "due_date": "2026-10-10T00:00:00+07:00"}}
			for len(rows) < 6 {
				rows = append(rows, Record{"title": "Next", "status": "verified"})
			}
			s, err := nativeBMGSProjectSheet("BM-GS-08", BMGSSourceRef{Kind: "handover"}, Record{"handover_stage": "Architecture", "handover_conclusion": decision, "status": "handed_over", "checklist_items": rows})
			if err != nil {
				t.Fatal(err)
			}
			f := bmgsAcceptanceReopen(t, "BM-GS-08", s)
			defer f.Close()
			for cell, want := range map[string]string{"F5": "Architecture", "C8": "Pressure", "D8": "Không đạt", "E8": "Leak", "F8": "2026-10-10"} {
				got, _ := f.GetCellValue(f.GetSheetList()[0], cell)
				if got != want {
					t.Errorf("%s = %q, want %q", cell, got, want)
				}
			}
			kind, _ := f.GetCellType(f.GetSheetList()[0], "F8")
			if kind == excelize.CellTypeSharedString || kind == excelize.CellTypeInlineString {
				t.Fatal("due date exported as text")
			}
			bmgsAssertConclusion(t, f, "A15", "ĐỦ ĐIỀU KIỆN BÀN GIAO", "CHƯA ĐỦ ĐIỀU KIỆN (CHỜ KHẮC PHỤC TỒN TẠI)", decision, "eligible", "not_eligible")
			text := bmgsWorkbookText(t, f)
			if !strings.Contains(text, "GIÁM SÁT CÔNG TRÌNH") || !strings.Contains(text, "ĐẠI DIỆN CĐT / KHÁCH HÀNG") {
				t.Fatal("signatures lost")
			}
		})
	}
}

func bmgsAcceptanceReopen(t *testing.T, code string, s BMGSSheet) *excelize.File {
	t.Helper()
	out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: code, Sheets: []BMGSSheet{s}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func bmgsAssertConclusion(t *testing.T, f *excelize.File, cell, first, second, decision, firstKey, secondKey string) {
	t.Helper()
	name := f.GetSheetList()[0]
	got, _ := f.GetCellValue(name, cell)
	mark := func(key string) string {
		if decision == key {
			return "[ X ] "
		}
		return "[   ] "
	}
	want := mark(firstKey) + first + "  |  " + mark(secondKey) + second
	if !strings.HasSuffix(got, want) {
		t.Errorf("conclusion = %q, want suffix %q", got, want)
	}
	styleID, _ := f.GetCellStyle(name, cell)
	style, _ := f.GetStyle(styleID)
	if style.Alignment == nil || !style.Alignment.WrapText {
		t.Fatal("conclusion does not wrap")
	}
	height, _ := f.GetRowHeight(name, mustBMGSRow(cell))
	if height < 22 {
		t.Fatalf("conclusion height too small: %v", height)
	}
	merges, _ := f.GetMergeCells(name)
	found := false
	for _, m := range merges {
		if m.GetStartAxis() == cell {
			found = true
		}
	}
	if !found {
		t.Fatal("dynamic conclusion merge lost")
	}
	rows, err := f.GetRows(name)
	if err != nil {
		t.Fatal(err)
	}
	printArea := ""
	for _, defined := range f.GetDefinedName() {
		if defined.Scope == name && defined.Name == "_xlnm.Print_Area" {
			printArea = defined.RefersTo
		}
	}
	end := strings.LastIndex(printArea, "$")
	if end < 0 {
		t.Fatal("print area missing")
	}
	var last int
	if _, err := fmt.Sscanf(printArea[end+1:], "%d", &last); err != nil || last < len(rows) {
		t.Fatalf("print area clips worksheet: %q, rows=%d", printArea, len(rows))
	}
}

func mustBMGSRow(cell string) int { _, row, _ := excelize.CellNameToCoordinates(cell); return row }

func bmgsWorkbookText(t *testing.T, f *excelize.File) string {
	t.Helper()
	rows, err := f.GetRows(f.GetSheetList()[0])
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, row := range rows {
		lines = append(lines, strings.Join(row, " "))
	}
	return strings.Join(lines, "\n")
}
