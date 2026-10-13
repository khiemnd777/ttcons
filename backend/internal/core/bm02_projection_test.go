package core

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestBM02DailyFactsPopulateOriginalCriteriaCells(t *testing.T) {
	row := Record{"report_date": "2026-10-12", "workflow_form_instance_id": "source", "progress_percent": 0, "structured_json": Record{
		"crew_items":           []Record{{"crew_name": "Tổ nguồn", "worker_count": 3, "work_hours": 8}},
		"completed_items":      []Record{{"title": "Công việc nguồn", "actual_quantity": 2, "unit": "m2", "result": "done"}, {"title": "Công việc nguồn", "actual_quantity": 0, "unit": "m3"}},
		"material_usage_items": []Record{{"material_name": "USED ONLY", "quantity": 9}},
		"blockers":             []Record{{"content": "Vướng mắc nguồn", "cause": "Nguyên nhân nguồn"}},
		"next_plan_items":      []Record{{"title": "Kế hoạch nguồn", "progress_percent": 0, "state": "Đang làm"}},
	}}
	sheet, err := nativeBMGSDailySheet(BMGSSourceRef{Kind: "daily_report", ID: "one"}, row)
	if err != nil {
		t.Fatal(err)
	}
	photos := []Record{{"id": "photo-exact", "original_filename": "frozen.jpg", "caption": "Chú thích nguồn", "payload_json": Record{"capture_metadata": Record{"location": "Vị trí nguồn"}}}}
	nativeBMGSProjectDailyPhotos(&sheet, row, photos)
	bookBytes, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-02", Sheets: []BMGSSheet{sheet}})
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(bookBytes))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	name := book.GetSheetList()[0]
	for cell, fragments := range map[string][]string{"D11": {"Tổ nguồn", "Số người: 3", "Giờ làm việc: 8"}, "D14": {"Khối lượng đã ghi nhận: 2", "m2", "Khối lượng đã ghi nhận: 0", "m3"}, "D19": {"Vướng mắc nguồn", "Nguyên nhân nguồn"}, "D20": {"Ảnh 1", "frozen.jpg", "Chú thích nguồn", "Vị trí nguồn"}, "D21": {"Kế hoạch nguồn", "0%", "Đang làm"}} {
		value, _ := book.GetCellValue(name, cell)
		for _, fragment := range fragments {
			if !strings.Contains(value, fragment) {
				t.Fatalf("%s lost %q: %s", cell, fragment, value)
			}
		}
	}
	for n := 10; n <= 21; n++ {
		value, _ := book.GetCellValue(name, fmt.Sprintf("C%d", n))
		if value != "Chưa đánh giá" {
			t.Fatal(n, value)
		}
		styleID, _ := book.GetCellStyle(name, fmt.Sprintf("C%d", n))
		style, err := book.GetStyle(styleID)
		if err != nil || len(style.Fill.Color) != 1 || style.Fill.Color[0] != "F2F2F2" {
			t.Fatalf("unknown status has non-neutral fill: row %d, style=%+v, err=%v", n, style, err)
		}
	}
	for _, cell := range []string{"D10", "D12", "D13", "D15", "D16", "D17", "D18"} {
		value, _ := book.GetCellValue(name, cell)
		if !strings.HasPrefix(value, "Chưa có ghi nhận") {
			t.Fatal(cell, value)
		}
	}
	structured := recordFromAny(row["structured_json"])
	structured["progress_percent"] = 0
	structured["material_arrival_items"] = []Record{{"material_name": "Actual arrival", "quantity": 0, "unit": "kg"}}
	other, _ := nativeBMGSDailySheet(BMGSSourceRef{Kind: "daily_report", ID: "two"}, row)
	if other.CriteriaRows[0]["note"] != "0%" || strings.Contains(fmt.Sprint(other.CriteriaRows[2]["note"]), "USED ONLY") {
		t.Fatal(other.CriteriaRows)
	}
	if !strings.HasPrefix(fmt.Sprint(sheet.CriteriaRows[0]["note"]), "Chưa có ghi nhận") {
		t.Fatal("previous source changed")
	}
	if strings.Contains(fmt.Sprint(sheet.CriteriaRows[10]["note"]), "photo-exact") {
		t.Fatal("photo identity leaked into primary note")
	}
}

func TestBM02ThirdArrivalFullSourceRetainsUnit(t *testing.T) {
	row := Record{"structured_json": Record{"material_arrival_items": []Record{{"material_name": "Arrival", "quantity": 1, "unit": "kg"}, {"material_name": "Arrival", "quantity": 2, "unit": "m"}, {"material_name": "Third", "quantity": 3, "unit": "UNIQUE_THIRD_UNIT", "specification": "Third spec", "origin": "Third origin", "note": "Third note"}}}}
	sheet, err := nativeBMGSDailySheet(BMGSSourceRef{Kind: "daily_report"}, row)
	if err != nil {
		t.Fatal(err)
	}
	out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-02", Sheets: []BMGSSheet{sheet}})
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	name := book.GetSheetList()[0]
	note, _ := book.GetCellValue(name, "D12")
	if !strings.Contains(note, "3 ghi nhận; xem đầy đủ tại Vật tư nhập trường đã ghi nhận") {
		t.Fatal(note)
	}
	rows, _ := book.GetRows(name)
	text := fmt.Sprint(rows)
	for _, fact := range []string{"UNIQUE_THIRD_UNIT", "Third spec", "Third origin", "Third note"} {
		if !strings.Contains(text, fact) {
			t.Fatal("full third arrival fact lost", fact)
		}
	}
}

func TestBM02ManualCriteriaAndFrozenPhotoFacts(t *testing.T) {
	for _, status := range []string{"pass", "fail", "not_applicable", "not_assessed"} {
		row := Record{"structured_json": Record{"supervision_assessments": []Record{{"code": "images", "assessment": status, "note": "  Exact manual note  "}, {"code": "quality", "assessment": status, "note": "Exact quality note"}}}}
		sheet, _ := nativeBMGSDailySheet(BMGSSourceRef{Kind: "work_report", RevisionNo: 2}, row)
		nativeBMGSProjectDailyPhotos(&sheet, row, []Record{{"id": "unrelated replacement"}})
		for _, index := range []int{3, 10} {
			if sheet.CriteriaRows[index]["assessment"] != status {
				t.Fatal(sheet.CriteriaRows[index])
			}
		}
		if sheet.CriteriaRows[10]["note"] != "  Exact manual note  " || sheet.CriteriaRows[3]["note"] != "Exact quality note" {
			t.Fatal(sheet.CriteriaRows)
		}
	}
	row := Record{"structured_json": Record{}}
	sheet, _ := nativeBMGSDailySheet(BMGSSourceRef{Kind: "work_report", RevisionNo: 2}, row)
	photo := Record{"id": "exact-frozen", "name": "old.jpg", "caption": "old caption"}
	nativeBMGSProjectDailyPhotos(&sheet, row, []Record{photo})
	photo["caption"] = "MUTATED LIVE"
	if strings.Contains(fmt.Sprint(sheet.CriteriaRows), "MUTATED LIVE") || !strings.Contains(fmt.Sprint(sheet.CriteriaRows[10]["note"]), "old caption") || !strings.Contains(fmt.Sprint(sheet.CriteriaRows[10]["note"]), "old.jpg") {
		t.Fatal(sheet.CriteriaRows)
	}
	lines := []string{strings.Repeat("a", 1000), "duplicate", "duplicate"}
	note := nativeBMGSCriterionSummary(lines, "full section")
	if len([]rune(note)) > 800 || !strings.Contains(note, "3 ghi nhận; xem đầy đủ tại full section") {
		t.Fatal(note)
	}
}
