package core

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestBMGS05ProjectHeaderMergedWorkbook(t *testing.T) {
	template, err := excelize.OpenReader(bytes.NewReader(bmgsTemplateWorkbook))
	if err != nil {
		t.Fatal(err)
	}
	defer template.Close()
	const code = "BM-GS-05"
	for _, tc := range []struct {
		name, project string
	}{
		{"short", "Công trình Thu Thiêm"},
		{"long", strings.Repeat("Công trình cải tạo và hoàn thiện Thu Thiêm ", 4)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			detected := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
			due := detected.AddDate(0, 0, 7)
			data := BMGSWorkbookData{TemplateCode: code, ProjectCode: "LIVE", ProjectName: "Current renamed project", Sheets: []BMGSSheet{{
				Metadata: Record{"historical": true, "project_code": "OLD-05", "project_name": tc.project},
				Rows:     []Record{{"risk_code": "00005", "detected_at": detected, "description": "Nguy cơ thấm", "cause": "Mưa lớn", "impact": "Chậm thi công", "risk_level": "Cao", "proposed_action": "Che chắn", "owner": "Giám sát", "due_at": due, "status": "Đang theo dõi tại hiện trường"}},
			}}}
			before, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			out, err := generateBMGSWorkbook(data)
			if err != nil {
				t.Fatal(err)
			}
			after, err := json.Marshal(data)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("source input changed: %v", err)
			}
			f, err := excelize.OpenReader(bytes.NewReader(out))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			merges, err := f.GetMergeCells(code)
			if err != nil {
				t.Fatal(err)
			}
			merged := false
			for _, m := range merges {
				merged = merged || m.GetStartAxis() == "B5" && m.GetEndAxis() == "K5"
			}
			if !merged {
				t.Error("project header requires B5:K5 merged area")
			}
			value, err := f.GetCellValue(code, "B5")
			if err != nil || value != "[OLD-05] "+tc.project {
				t.Fatalf("frozen project name lost: %q, %v", value, err)
			}
			height, err := f.GetRowHeight(code, 5)
			if err != nil || height > 60 {
				t.Errorf("project header height=%g, want <=60pt for merged-area fixture: %v", height, err)
			}
			t.Logf("full project name retained; row5 height=%gpt", height)
			for col := 1; col <= 11; col++ {
				letter, _ := excelize.ColumnNumberToName(col)
				want, err := template.GetColWidth(code, letter)
				if err != nil {
					t.Fatal(err)
				}
				got, err := f.GetColWidth(code, letter)
				if err != nil || got != want {
					t.Fatalf("risk column %s width changed: %g, want %g: %v", letter, got, want, err)
				}
				cell := letter + "7"
				wantHeader, _ := template.GetCellValue(code, cell)
				gotHeader, err := f.GetCellValue(code, cell)
				if err != nil || gotHeader != wantHeader {
					t.Fatalf("risk header %s changed: %q, want %q: %v", cell, gotHeader, wantHeader, err)
				}
			}
			for _, cell := range []string{"A5", "A12", "D12", "G12", "A13", "D13", "G13"} {
				want, _ := template.GetCellValue(code, cell)
				got, err := f.GetCellValue(code, cell)
				if err != nil || got != want || want == "" {
					t.Fatalf("client label/signature %s changed: %q, want %q: %v", cell, got, want, err)
				}
			}
			for cell, want := range map[string]string{"A8": "1", "B8": "00005", "D8": "Nguy cơ thấm", "E8": "Mưa lớn", "F8": "Chậm thi công", "G8": "Cao", "H8": "Che chắn", "I8": "Giám sát", "K8": "Đang theo dõi tại hiện trường"} {
				got, err := f.GetCellValue(code, cell)
				if err != nil || got != want {
					t.Fatalf("risk data %s changed: %q, want %q: %v", cell, got, want, err)
				}
			}
			for cell, date := range map[string]time.Time{"C8": detected, "J8": due} {
				serial := date.Sub(time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)).Hours() / 24
				got, err := f.GetCellValue(code, cell, excelize.Options{RawCellValue: true})
				if err != nil || got != strconv.FormatFloat(serial, 'f', -1, 64) {
					t.Fatalf("date %s changed: %q, want %v: %v", cell, got, serial, err)
				}
			}
		})
	}
}

func TestBMGS05HeaderRepairPreservesOtherTemplateMerges(t *testing.T) {
	template, err := excelize.OpenReader(bytes.NewReader(bmgsTemplateWorkbook))
	if err != nil {
		t.Fatal(err)
	}
	defer template.Close()
	for code := range bmgsLayouts {
		t.Run(code, func(t *testing.T) {
			out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: code, Sheets: []BMGSSheet{{}}})
			if err != nil {
				t.Fatal(err)
			}
			f, err := excelize.OpenReader(bytes.NewReader(out))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			want, err := template.GetMergeCells(code)
			if err != nil {
				t.Fatal(err)
			}
			got, err := f.GetMergeCells(code)
			if err != nil {
				t.Fatal(err)
			}
			// Compare geometry, since export data intentionally replaces template values.
			geometry := func(merges []excelize.MergeCell) []string {
				areas := make([]string, len(merges))
				for i, m := range merges {
					areas[i] = m.GetStartAxis() + ":" + m.GetEndAxis()
				}
				sort.Strings(areas)
				return areas
			}
			wantAreas := geometry(want)
			if area, ok := map[string]string{"BM-GS-05": "B5:K5", "BM-GS-06": "B5:J5", "BM-GS-07": "B5:G5"}[code]; ok {
				wantAreas = append(wantAreas, area)
				sort.Strings(wantAreas)
			}
			if !reflect.DeepEqual(geometry(got), wantAreas) {
				t.Fatalf("template merges changed beyond approved headers: got %v, want %v", geometry(got), wantAreas)
			}
		})
	}
}

func TestBMGS06And07ProjectHeaderMergedWorkbook(t *testing.T) {
	template, err := excelize.OpenReader(bytes.NewReader(bmgsTemplateWorkbook))
	if err != nil {
		t.Fatal(err)
	}
	defer template.Close()
	for _, code := range []string{"BM-GS-06", "BM-GS-07"} {
		for _, tc := range []struct {
			name, project, expected string
			metadata                Record
		}{
			{"current_short", "Thu Thiêm", "[LIVE] Thu Thiêm", nil},
			{"frozen_long", "Renamed live project", "[OLD] " + strings.Repeat("Công trình cải tạo Thu Thiêm ", 6), Record{"historical": true, "project_code": "OLD", "project_name": strings.Repeat("Công trình cải tạo Thu Thiêm ", 6)}},
			{"historical_missing", "Renamed live project", "", Record{"historical": true}},
		} {
			t.Run(code+"/"+tc.name, func(t *testing.T) {
				row := Record{"instruction_code": "00006", "title": "Che chắn", "location": "Tầng 2", "partner": "Thầu phụ", "representative": "Giám sát", "issued_at": "2026-10-13", "due_at": "2026-10-20", "progress_percent": 0, "status": "Đang theo dõi", "captured_at": "2026-10-13", "technical_content": "Kiểm tra che chắn", "photo_phase": "Trước thi công", "handover_note": "Giữ nguyên ảnh nguồn"}
				data := BMGSWorkbookData{TemplateCode: code, ProjectCode: "LIVE", ProjectName: tc.project, Sheets: []BMGSSheet{{Metadata: tc.metadata, Rows: []Record{row}}}}
				if code == "BM-GS-07" {
					data.Images = bmgsTestImagesForRows(t, data.Sheets[0].Rows)
				}
				before, _ := json.Marshal(data)
				out, err := generateBMGSWorkbook(data)
				if err != nil {
					t.Fatal(err)
				}
				after, _ := json.Marshal(data)
				if !bytes.Equal(before, after) {
					t.Fatal("source identity/data/images mutated")
				}
				f, err := excelize.OpenReader(bytes.NewReader(out))
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				value, err := f.GetCellValue(code, "B5")
				if err != nil || value != tc.expected {
					t.Fatalf("project identity %q, want %q: %v", value, tc.expected, err)
				}
				height, err := f.GetRowHeight(code, 5)
				if err != nil || height > 60 {
					t.Errorf("header height=%gpt, want <=60pt: %v", height, err)
				}
				t.Logf("full identity retained; row5=%gpt", height)
				end := map[string]string{"BM-GS-06": "J5", "BM-GS-07": "G5"}[code]
				merges, err := f.GetMergeCells(code)
				if err != nil {
					t.Fatal(err)
				}
				merged := false
				for _, m := range merges {
					merged = merged || m.GetStartAxis() == "B5" && m.GetEndAxis() == end
				}
				if !merged {
					t.Errorf("missing B5:%s header merge", end)
				}
				for col := 1; col <= bmgsLayouts[code].columns; col++ {
					letter, _ := excelize.ColumnNumberToName(col)
					wantWidth, _ := template.GetColWidth(code, letter)
					if code == "BM-GS-07" && letter == "F" {
						wantWidth = 24 // Existing cell-image layout contract.
					}
					gotWidth, err := f.GetColWidth(code, letter)
					if err != nil || gotWidth != wantWidth {
						t.Fatalf("table column %s width=%g, want %g: %v", letter, gotWidth, wantWidth, err)
					}
					wantHeader, _ := template.GetCellValue(code, letter+"7")
					gotHeader, err := f.GetCellValue(code, letter+"7")
					if err != nil || gotHeader != wantHeader {
						t.Fatalf("table header %s changed: %q, want %q: %v", letter, gotHeader, wantHeader, err)
					}
				}
				labels := []string{"A5", "A12", "D12", "G12", "A13", "D13", "G13"}
				values := map[string]string{"A8": "1", "B8": "00006", "C8": "Che chắn", "D8": "Tầng 2", "E8": "Thầu phụ", "F8": "Giám sát", "G8": "2026-10-13", "H8": "2026-10-20", "I8": "0", "J8": "Đang theo dõi"}
				if code == "BM-GS-07" {
					labels = []string{"A5", "A13", "D13", "A14", "D14"}
					values = map[string]string{"A8": "1", "B8": "Tầng 2", "C8": "2026-10-13", "D8": "Kiểm tra che chắn", "E8": "Trước thi công", "F8": "", "G8": "Giữ nguyên ảnh nguồn"}
					bmgsAssertCompatibleImages(t, out, []bmgsCellImage{{Sheet: code, Cell: "F8", Image: data.Images[0]}})
					baseline, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: code, Sheets: []BMGSSheet{{Rows: data.Sheets[0].Rows}}, Images: data.Images})
					if err != nil {
						t.Fatal(err)
					}
					base, err := excelize.OpenReader(bytes.NewReader(baseline))
					if err != nil {
						t.Fatal(err)
					}
					defer base.Close()
					wantHeight, _ := base.GetRowHeight(code, 8)
					gotHeight, err := f.GetRowHeight(code, 8)
					if err != nil || gotHeight != wantHeight {
						t.Fatalf("header identity changed photo row height: %g, want %g: %v", gotHeight, wantHeight, err)
					}
					baseParts := bmgsTestZipParts(t, baseline)
					for name, part := range bmgsTestZipParts(t, out) {
						if (strings.HasPrefix(name, "xl/drawings/") || strings.HasPrefix(name, "xl/media/")) && !bytes.Equal(part, baseParts[name]) {
							t.Fatalf("header identity changed image geometry/media: %s", name)
						}
					}
				}
				for _, cell := range labels {
					values[cell], _ = template.GetCellValue(code, cell)
				}
				for cell, want := range values {
					got, err := f.GetCellValue(code, cell)
					if err != nil || got != want {
						t.Fatalf("cell %s=%q, want %q: %v", cell, got, want, err)
					}
				}
			})
		}
	}
}
