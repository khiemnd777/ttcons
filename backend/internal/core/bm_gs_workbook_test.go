package core

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestBMGSWorkbookAllTemplatesRemoveExamples(t *testing.T) {
	for code, l := range bmgsLayouts {
		t.Run(code, func(t *testing.T) {
			rows := []Record{}
			for i := 0; i < l.slots+2; i++ {
				r := Record{}
				for _, k := range l.keys {
					r[k] = "actual-" + k
				}
				rows = append(rows, r)
			}
			s := BMGSSheet{Name: code, DateLabel: "08/10/2026", Rows: rows, Metadata: Record{}}
			if code == "BM-GS-02" {
				s.CrewRows = []Record{{"partner": "Actual crew", "worker_count": 3}}
				s.CriteriaRows = []Record{{"code": "progress", "assessment": "not_assessed", "note": "Actual note"}}
			}
			images := []BMGSImage{}
			if code == "BM-GS-07" {
				images = bmgsTestImagesForRows(t, s.Rows)
			}
			out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: code, ProjectCode: "P001", ProjectName: "Actual project", Sheets: []BMGSSheet{s}, Images: images})
			if err != nil {
				t.Fatal(err)
			}
			z, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range z.File {
				rd, _ := entry.Open()
				raw, _ := io.ReadAll(rd)
				rd.Close()
				for _, sample := range []string{"CHATEAU_Q7", "Minh Phát", "Nguyễn Văn A", "150m2", "[ X ]", "17/09/2026", "Trinh Quoc Tuan", "ICV"} {
					if bytes.Contains(raw, []byte(sample)) {
						t.Fatalf("sample leaked: %s in %s", sample, entry.Name)
					}
				}
			}
			f, err := excelize.OpenReader(bytes.NewReader(out))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if len(f.GetSheetList()) != 1 {
				t.Fatal(f.GetSheetList())
			}
			header, _ := f.GetCellValue(code, "A1")
			if !strings.Contains(header, "THỦ THIÊM") {
				t.Fatal(header)
			}
			style, _ := f.GetCellStyle(code, fmt.Sprintf("B%d", l.start))
			if style == 0 {
				t.Fatal("lost template cell style")
			}
			merges, _ := f.GetMergeCells(code)
			if len(merges) < 3 {
				t.Fatal("lost template merges")
			}
			if code != "BM-GS-02" {
				v, _ := f.GetCellValue(code, fmt.Sprintf("B%d", l.start+l.slots+1))
				if v != "actual-"+l.keys[0] {
					t.Fatalf("dynamic row missing: %q", v)
				}
				if code == "BM-GS-04" {
					summary, err := f.CalcCellValue(code, "A15")
					if err != nil {
						t.Fatal(err)
					}
					if summary != "TỔNG HỢP: Tổng lỗi: 5 | Đã khắc phục: 0 | Đang xử lý: 5" {
						t.Fatal("summary did not count expanded rows", summary)
					}
				}
			}
			if code == "BM-GS-02" {
				v, _ := f.GetCellValue(code, "C10")
				if v != "Chưa đánh giá" {
					t.Fatal(v)
				}
				v, _ = f.GetCellValue(code, "C11")
				if v != "" {
					t.Fatal("invented assessment")
				}
				formula, _ := f.GetCellFormula(code, "C28")
				if formula != `IF(COUNT(C25:C27)=0,"",SUM(C25:C27))` {
					t.Fatal(formula)
				}
			}
		})
	}
}

func TestBMGS04SummaryCountsExportedActions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rows    []Record
		footer  string
		summary string
	}{
		{
			name:   "blank_code_with_unused_template_rows",
			rows:   []Record{{"description": "Uncoded fault", "status": "open"}},
			footer: "A13", summary: "TỔNG HỢP: Tổng lỗi: 1 | Đã khắc phục: 0 | Đang xử lý: 1",
		},
		{
			name:   "all_codes_blank",
			rows:   []Record{{"status": "open"}, {"status": "done"}, {"status": "waiting_confirmation"}},
			footer: "A13", summary: "TỔNG HỢP: Tổng lỗi: 3 | Đã khắc phục: 1 | Đang xử lý: 2",
		},
		{
			name:   "mixed_codes_and_dynamic_rows",
			rows:   []Record{{"fault_code": "F001", "status": "open"}, {"status": "done"}, {"fault_code": "F003", "status": "waiting_confirmation"}, {"status": "open"}, {"fault_code": "F005", "status": "done"}},
			footer: "A15", summary: "TỔNG HỢP: Tổng lỗi: 5 | Đã khắc phục: 2 | Đang xử lý: 3",
		},
		{
			name:   "empty_register",
			footer: "A13", summary: "TỔNG HỢP: Tổng lỗi: 0 | Đã khắc phục: 0 | Đang xử lý: 0",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-04", Sheets: []BMGSSheet{{Rows: tc.rows}}})
			if err != nil {
				t.Fatal(err)
			}
			f, err := excelize.OpenReader(bytes.NewReader(out))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			sheet := f.GetSheetList()[0]
			got, err := f.CalcCellValue(sheet, tc.footer)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.summary {
				t.Fatalf("calculated exported summary: got %q, want %q", got, tc.summary)
			}
		})
	}
}

func TestBMGSNumericQuantityAndDate(t *testing.T) {
	detected := time.Date(2026, 10, 8, 9, 30, 0, 0, time.UTC)
	out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-03", Sheets: []BMGSSheet{{Rows: []Record{{"title": "Quantity", "design_quantity": 25.75, "actual_quantity": 23.5, "unit": "m2"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	raw, _ := f.GetCellValue("BM-GS-03", "E9", excelize.Options{RawCellValue: true})
	if raw != "25.75" {
		t.Fatal("quantity lost numeric value", raw)
	}
	formatted, _ := f.GetCellValue("BM-GS-03", "E9")
	if formatted != "25.75 m2" {
		t.Fatal(formatted)
	}
	out, err = generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-05", Sheets: []BMGSSheet{{Rows: []Record{{"risk_code": "00001", "detected_at": detected}}}}})
	if err != nil {
		t.Fatal(err)
	}
	g, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	id, _ := g.GetCellValue("BM-GS-05", "B8")
	if id != "00001" {
		t.Fatal("identifier zeroes lost")
	}
	raw, _ = g.GetCellValue("BM-GS-05", "C8", excelize.Options{RawCellValue: true})
	if strings.Contains(raw, "2026") || raw == "" {
		t.Fatal("date is not stored as typed Excel serial", raw)
	}
}

func TestBMGSWorkbookFrozenProjectIdentity(t *testing.T) {
	data := BMGSWorkbookData{TemplateCode: "BM-GS-02", ProjectCode: "LIVE", ProjectName: "Renamed live project", Address: "Current address", Sheets: []BMGSSheet{
		{Name: "Historical", Metadata: Record{"historical": true, "project_code": "OLD", "project_name": "Original name", "address": "Original address"}},
		{Name: "Legacy missing", Metadata: Record{"historical": true}},
		{Name: "Legacy partial", Metadata: Record{"historical": true, "project_code": "OLDER", "project_name": nil}},
		{Name: "Current"},
		{Name: "Explicit source", Metadata: Record{"project_code": "EXPLICIT", "project_name": "Source name"}},
	}}
	out, err := generateBMGSWorkbook(data)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for sheet, want := range map[string]string{"Historical": "[OLD] Original name", "Legacy missing": "", "Legacy partial": "[OLDER] ", "Current": "[LIVE] Renamed live project", "Explicit source": "[EXPLICIT] Source name"} {
		got, _ := f.GetCellValue(sheet, "B5")
		if got != want {
			t.Fatalf("%s identity %q, want %q", sheet, got, want)
		}
	}
	data.TemplateCode = "BM-GS-01"
	out, err = generateBMGSWorkbook(data)
	if err != nil {
		t.Fatal(err)
	}
	g, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	for cell, want := range map[string]string{"B5": "Original name", "F5": "OLD", "B6": "Original address"} {
		got, _ := g.GetCellValue("Historical", cell)
		if got != want {
			t.Fatalf("%s %q want %q", cell, got, want)
		}
	}
	for _, cell := range []string{"B5", "F5", "B6"} {
		got, _ := g.GetCellValue("Legacy missing", cell)
		if got != "" {
			t.Fatalf("legacy missing %s acquired live value %q", cell, got)
		}
	}
}

func TestBMGSWorkbookPreservesClientPrintSetup(t *testing.T) {
	template, err := excelize.OpenReader(bytes.NewReader(bmgsTemplateWorkbook))
	if err != nil {
		t.Fatal(err)
	}
	defer template.Close()
	for code, l := range bmgsLayouts {
		t.Run(code, func(t *testing.T) {
			original, e := template.GetPageLayout(code)
			if e != nil {
				t.Fatal(e)
			}
			out, e := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: code, Sheets: []BMGSSheet{{}}})
			if e != nil {
				t.Fatal(e)
			}
			f, e := excelize.OpenReader(bytes.NewReader(out))
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			got, e := f.GetPageLayout(code)
			if e != nil {
				t.Fatal(e)
			}
			if *got.Orientation != *original.Orientation || *got.Size != *original.Size || *got.AdjustTo != *original.AdjustTo {
				t.Fatalf("client print setup changed: got orientation=%s,paper=%d,scale=%d; template orientation=%s,paper=%d,scale=%d", *got.Orientation, *got.Size, *got.AdjustTo, *original.Orientation, *original.Size, *original.AdjustTo)
			}
			if got.FitToWidth == nil || *got.FitToWidth != 1 || got.FitToHeight == nil || *got.FitToHeight != 0 {
				t.Fatal("dynamic fit dimensions lost", got)
			}
			props, e := f.GetSheetProps(code)
			if e != nil || props.FitToPage == nil || !*props.FitToPage {
				t.Fatal("fit to page inactive", e)
			}
			for c := 1; c <= l.columns; c++ {
				column, _ := excelize.ColumnNumberToName(c)
				width, e := f.GetColWidth(code, column)
				if e != nil {
					t.Fatal(e)
				}
				clientWidth, e := template.GetColWidth(code, column)
				if code == "BM-GS-07" && column == "F" {
					clientWidth = 24
				}
				if e != nil {
					t.Fatal(e)
				}
				if width != clientWidth {
					t.Fatalf("column%s dimension changed %f vs %f", column, width, clientWidth)
				}
			}
		})
	}
}

func TestBMGSLocalizedRecordedStates(t *testing.T) {
	for _, tc := range []struct {
		code, key, column, kind string
		values, wants           []string
	}{
		{"BM-GS-03", "result", "G", "", []string{"pass", "fail", "pending", "conditional_pass", "Đo lại vào buổi sáng", "future_state"}, []string{"Đạt", "Không đạt", "Chưa đánh giá", "Đạt có điều kiện", "Đo lại vào buổi sáng", "future_state"}},
		{"BM-GS-06", "status", "J", "", []string{"in_progress", "waiting_evidence", "waiting_review", "completed", "future_state"}, []string{"Đang xử lý", "Chờ minh chứng", "Chờ rà soát", "Hoàn tất", "future_state"}},
		{"BM-GS-05", "status", "K", "survey", []string{"draft", "submitted", "approved", "reopened"}, []string{"Bản nháp biểu mẫu", "Biểu mẫu đã gửi", "Biểu mẫu đã duyệt", "Biểu mẫu mở lại"}},
		{"BM-GS-05", "status", "K", "variation", []string{"draft", "qs_review", "approved", "incorporated"}, []string{"Phát sinh hợp đồng: Nháp", "Phát sinh hợp đồng: QS đang rà soát", "Phát sinh hợp đồng: Đã duyệt", "Phát sinh hợp đồng: Đã tạo phiên bản điều chỉnh"}},
		{"BM-GS-04", "status", "I", "action", []string{"done", "waiting_confirmation", "open", "Đã khắc phục"}, []string{"Đã khắc phục", "Chờ xác nhận khắc phục", "Mới", "Đã khắc phục"}},
	} {
		t.Run(tc.code+tc.kind, func(t *testing.T) {
			rows := make([]Record, len(tc.values))
			for i, value := range tc.values {
				rows[i] = Record{tc.key: value, "source_kind": tc.kind, "fault_code": fmt.Sprintf("E%d", i)}
			}
			out, e := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: tc.code, Sheets: []BMGSSheet{{Rows: rows, Metadata: Record{"conclusion": "pending"}}}})
			if e != nil {
				t.Fatal(e)
			}
			f, e := excelize.OpenReader(bytes.NewReader(out))
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			start := bmgsLayouts[tc.code].start
			for i, want := range tc.wants {
				got, _ := f.GetCellValue(tc.code, fmt.Sprintf("%s%d", tc.column, start+i))
				if got != want {
					t.Fatalf("%s %q => %q, want %q", tc.key, tc.values[i], got, want)
				}
			}
			if tc.code == "BM-GS-03" {
				got, _ := f.GetCellValue(tc.code, "A16")
				if got != "KẾT LUẬN NGHIỆM THU: [   ] ĐỒNG Ý NGHIỆM THU  |  [   ] CẦN SỬA CHỮA TỒN TẠI" {
					t.Fatal("pending conclusion became assessed", got)
				}
			}
			if tc.code == "BM-GS-04" {
				formula, _ := f.GetCellFormula(tc.code, "A14")
				if !strings.Contains(formula, `COUNTIF(I8:I11,"Đã khắc phục")`) {
					t.Fatal("resolved counter changed", formula)
				}
			}
		})
	}
}

func TestBMGSWebP(t *testing.T) {
	// Single-pixel opaque VP8 WebP; validate MIME and normalize for Excel.
	raw, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	pic, err := prepareBMGSImage(raw, "image/webp")
	if err != nil {
		t.Fatal(err)
	}
	if pic.MIMEType != "image/jpeg" || pic.Width != 1 || pic.Height != 1 {
		t.Fatal(pic.MIMEType, pic.Width, pic.Height)
	}
}

// Optional retained synthetic fixtures support offline inspection without source/customer data.
func TestBMGSQAFixtures(t *testing.T) {
	dir := os.Getenv("BMGS_QA_DIR")
	if dir == "" {
		t.Skip("BMGS_QA_DIR not requested")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	im := image.NewRGBA(image.Rect(0, 0, 2000, 1200))
	seed := uint32(42)
	for y := 0; y < 1200; y++ {
		for x := 0; x < 2000; x++ {
			seed = seed*1664525 + 1013904223
			v := uint8(seed >> 24)
			im.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	for y := 100; y < 600; y++ {
		for x := 100; x < 1900; x++ {
			im.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	fd := font.Drawer{Dst: im, Src: image.NewUniform(color.Black), Face: basicfont.Face7x13, Dot: fixed.P(120, 130)}
	fd.DrawString("SYNTHETIC QA: technical detail / 00123 / 25.75 mm")
	for y := 180; y < 550; y += 30 {
		for x := 120; x < 1850; x++ {
			im.SetRGBA(x, y, color.RGBA{20, 20, 20, 255})
		}
	}
	var raw bytes.Buffer
	jpeg.Encode(&raw, im, &jpeg.Options{Quality: 95})
	pic, err := prepareBMGSImage(raw.Bytes(), "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	pic.EvidenceID = "synthetic-only"
	pic.Filename = "synthetic-detail.jpg"
	pic.Caption = "SYNTHETIC QA ONLY"
	pic.SourceLabel = "QA source"
	pic.Location = "QA location"
	pic.Phase = "during"
	if err = os.WriteFile(filepath.Join(dir, "synthetic-original.jpg"), raw.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "synthetic-optimized.jpg"), pic.Bytes, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("synthetic JPEG bytes %d -> %d; pixels 2000x1200 -> %dx%d", raw.Len(), len(pic.Bytes), pic.Width, pic.Height)
	for code, l := range bmgsLayouts {
		rows := []Record{}
		for i := 0; i < l.slots+2; i++ {
			r := Record{}
			for _, k := range l.keys {
				r[k] = "QA " + k
			}
			if code == "BM-GS-03" {
				r["design_quantity"] = 25.75
				r["actual_quantity"] = 23.5
				r["unit"] = "m2"
			}
			rows = append(rows, r)
		}
		s := BMGSSheet{Name: code, DateLabel: "08/10/2026", Rows: rows}
		if code == "BM-GS-02" {
			s.CriteriaRows = bmgsQACriteria()
			s.CrewRows = []Record{{"partner": "QA crew", "worker_count": 3}}
			s.MaterialRows = []Record{{"title": "QA delivery", "quantity": 4.5, "unit": "m2"}}
		}
		images := []BMGSImage{pic}
		if code == "BM-GS-07" {
			images = bmgsTestImagesForRows(t, s.Rows)
		}
		out, e := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: code, ProjectCode: "QA001", ProjectName: "Synthetic project", Sheets: []BMGSSheet{s}, Images: images})
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, code+".xlsx"), out, 0600); e != nil {
			t.Fatal(e)
		}
	}
}

func TestBMGSWorkbookDailySheetsAndImages(t *testing.T) {
	im := image.NewNRGBA(image.Rect(0, 0, 60, 40))
	im.Set(0, 0, color.NRGBA{R: 255, A: 128})
	var raw bytes.Buffer
	png.Encode(&raw, im)
	pic, err := prepareBMGSImage(raw.Bytes(), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	pic.EvidenceID = "exact-evidence"
	pic.Filename = "photo.png"
	pic.Caption = "actual technical caption"
	out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-02", Sheets: []BMGSSheet{{Name: "08/10/2026"}, {Name: "08/10/2026"}}, Images: []BMGSImage{pic, pic}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	names := f.GetSheetList()
	if len(names) != 3 || names[0] == names[1] {
		t.Fatal(names)
	}
	anchors, err := f.GetPictureCells("Hình ảnh")
	if err != nil || len(anchors) != 1 {
		t.Fatalf("picture anchors %v: %v", anchors, err)
	}
	images, err := f.GetPictures("Hình ảnh", anchors[0])
	if err != nil || len(images) != 1 {
		t.Fatalf("images %d %v", len(images), err)
	}
	if !bytes.Equal(images[0].File, pic.Bytes) {
		t.Fatal("media bytes changed")
	}
	caption, _ := f.GetCellValue("Hình ảnh", "A1")
	if !strings.Contains(caption, pic.EvidenceID) || !strings.Contains(caption, pic.Caption) {
		t.Fatal(caption)
	}
	pic.Bytes = []byte("invalid")
	if _, err = generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-02", Sheets: []BMGSSheet{{}}, Images: []BMGSImage{pic}}); err == nil {
		t.Fatal("invalid image silently omitted")
	}
}

func bmgsQACriteria() []Record {
	codes := []string{"progress", "workforce", "deliveries", "quality", "completed_quantity", "variation", "mep", "safety", "acceptance", "risk", "images", "instructions"}
	notes := []string{"QA: Đối chiếu mốc tiến độ với kế hoạch tuần đã ghi nhận.", "QA: Tổ đội có ba công nhân tại thời điểm báo cáo.", "QA: Phiếu giao nhận ghi nhận 4,5 m2 vật tư nhập trường.", "QA: Hai vị trí cần kiểm tra lại bề mặt hoàn thiện.", "QA: Khối lượng hoàn thành chưa được đo và xác nhận.", "QA: Không áp dụng mục phát sinh cho nội dung báo cáo này.", "QA: Đã đối chiếu vị trí chờ kỹ thuật với bản vẽ.", "QA: Thiết bị bảo hộ được kiểm tra trong ca.", "QA: Công tác nghiệm thu đang chờ kiểm tra hiện trường.", "QA: Có ghi nhận nguy cơ chậm giao vật tư cần theo dõi.", "QA: Ảnh hiện trường đã được người lập chọn làm bằng chứng.", "QA: Chỉ dẫn thi công được ghi nhận tại nguồn công việc."}
	states := []string{"pass", "pass", "pass", "fail", "not_assessed", "not_applicable", "pass", "pass", "not_assessed", "fail", "pass", "pass"}
	rows := make([]Record, len(codes))
	for i, code := range codes {
		rows[i] = Record{"code": code, "assessment": states[i], "note": notes[i]}
	}
	return rows
}

func TestBMGSAllDailyCriteriaAndLongText(t *testing.T) {
	criteria := bmgsQACriteria()
	criteria[0]["note"] = strings.Repeat("Ghi nhận kỹ thuật tại hiện trường. ", 25)
	data := BMGSWorkbookData{TemplateCode: "BM-GS-02", Sheets: []BMGSSheet{{CriteriaRows: criteria}}}
	out, err := generateBMGSWorkbook(data)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	labels := map[string]string{"pass": "Đạt", "fail": "Không đạt", "not_applicable": "Không áp dụng", "not_assessed": "Chưa đánh giá"}
	for i, r := range criteria {
		row := 10 + i
		got, _ := f.GetCellValue("BM-GS-02", fmt.Sprintf("C%d", row))
		if got != labels[r["assessment"].(string)] {
			t.Fatalf("criterion %s at row%d: %s", r["code"], row, got)
		}
		got, _ = f.GetCellValue("BM-GS-02", fmt.Sprintf("D%d", row))
		if got != r["note"] {
			t.Fatalf("criterion note lost at row%d", row)
		}
	}
	height, _ := f.GetRowHeight("BM-GS-02", 10)
	if height <= 42 {
		t.Fatal("long criterion note height unchanged", height)
	}
	data.TemplateCode = "BM-GS-01"
	rows := make([]Record, 7)
	for i := range rows {
		rows[i] = Record{"location": "Synthetic location", "note": strings.Repeat("Ghi nhận. ", 25)}
	}
	data.Sheets = []BMGSSheet{{Rows: rows}}
	out, err = generateBMGSWorkbook(data)
	if err != nil {
		t.Fatal(err)
	}
	g, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	height, _ = g.GetRowHeight("BM-GS-01", 16)
	if height <= 45 {
		t.Fatal("dynamic long text clipped", height)
	}
	found := false
	for _, n := range g.GetDefinedName() {
		if n.Name == "_xlnm.Print_Area" && n.Scope == "BM-GS-01" {
			found = true
			if n.RefersTo != "'BM-GS-01'!$A$1:$G$24" {
				t.Fatal("print area not expanded", n)
			}
		}
	}
	if !found {
		t.Fatal("missing print area")
	}
	rows[0]["note"] = strings.Repeat("text ", 1000)
	if _, err = generateBMGSWorkbook(data); err == nil || !strings.Contains(err.Error(), "printable Excel row height") {
		t.Fatal("unprintable content silently clipped", err)
	}
}

func TestBMGSLongCaptionMovesImageAnchors(t *testing.T) {
	im := image.NewNRGBA(image.Rect(0, 0, 60, 40))
	var raw bytes.Buffer
	png.Encode(&raw, im)
	pic, err := prepareBMGSImage(raw.Bytes(), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	pic.EvidenceID = "longcaption"
	pic.Caption = strings.Repeat("Ghi nhận nội dung ảnh kỹ thuật. ", 30)
	second := pic
	second.EvidenceID = "second"
	out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-02", Sheets: []BMGSSheet{{}}, Images: []BMGSImage{pic, second}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	anchors, err := f.GetPictureCells("Hình ảnh")
	if err != nil || len(anchors) != 2 {
		t.Fatal(anchors, err)
	}
	_, firstRow, _ := excelize.CellNameToCoordinates(anchors[0])
	_, secondRow, _ := excelize.CellNameToCoordinates(anchors[1])
	if firstRow <= 4 || secondRow-firstRow < 34 {
		t.Fatal("long captions overlap images", anchors)
	}
	caption, _ := f.GetCellValue("Hình ảnh", "A1")
	if !strings.Contains(caption, pic.Caption) {
		t.Fatal("caption truncated")
	}
}

func TestBMGSImageAppendixLocalizedCaptionIdentity(t *testing.T) {
	const sourceID = "10000000-0000-4000-8000-000000000001"
	cases := []struct{ phase, want, kind, sourceLabel string }{
		{"before", "Trước thi công", "work_report", "Báo cáo công việc"},
		{"during", "Trong thi công", "daily_report", "Báo cáo hằng ngày"},
		{"after", "Sau thi công", "evidence", "Minh chứng"},
		{"Giai đoạn do người lập ghi nhận", "Giai đoạn do người lập ghi nhận", "survey", "Khảo sát hiện trạng"},
		{"future_phase", "future_phase", "variation", "Phát sinh hợp đồng"},
	}
	im := image.NewNRGBA(image.Rect(0, 0, 30, 20))
	var raw bytes.Buffer
	if err := png.Encode(&raw, im); err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareBMGSImage(raw.Bytes(), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	for i, tc := range cases {
		t.Run(tc.phase, func(t *testing.T) {
			pic := prepared
			pic.EvidenceID = fmt.Sprintf("caption-evidence-%d", i)
			pic.Filename = "recorded-photo.png"
			pic.Phase = tc.phase
			pic.CapturedAt = "08/10/2026 09:30"
			pic.SourceLabel = tc.kind + " " + sourceID + " lần 7"
			pic.Location = "Vị trí ghi nhận"
			pic.Caption = "Nội dung ảnh gốc"
			out, e := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-02", Sheets: []BMGSSheet{{}}, Images: []BMGSImage{pic}})
			if e != nil {
				t.Fatal(e)
			}
			f, e := excelize.OpenReader(bytes.NewReader(out))
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			caption, e := f.GetCellValue("Hình ảnh", "A1")
			if e != nil {
				t.Fatal(e)
			}
			want := pic.Filename + " • " + pic.EvidenceID + "\n" + tc.sourceLabel + " " + sourceID + " lần 7 • " + pic.Location + "\n" + pic.CapturedAt + " • " + tc.want + "\n" + pic.Caption
			if caption != want {
				t.Fatalf("caption source/phase changed incorrectly:\n%s\nwant:\n%s", caption, want)
			}
			if pic.Phase != tc.phase || pic.SourceLabel != tc.kind+" "+sourceID+" lần 7" {
				t.Fatal("renderer mutated original identity")
			}
		})
	}
	for _, label := range []string{"Nguồn ghi nhận riêng", "survey nơi khảo sát", "future_kind " + sourceID + " lần 7", "work_report " + sourceID + " lần chưa ghi nhận"} {
		if got := bmgsImageSourceLabel(label); got != label {
			t.Fatalf("arbitrary source label changed %q => %q", label, got)
		}
	}
	for kind, want := range map[string]string{"acceptance": "Nghiệm thu nội bộ", "action": "Lỗi và tồn tại", "site_work": "Đầu việc hiện trường", "handover": "Bàn giao"} {
		if got := bmgsImageSourceLabel(kind + " " + sourceID + " lần 0"); got != want+" "+sourceID+" lần 0" {
			t.Fatal(got)
		}
	}
}

func TestBMGSPrepareImageBoundsAndOrientation(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 2000, 1000))
	var raw bytes.Buffer
	jpeg.Encode(&raw, im, &jpeg.Options{Quality: 95})
	pic, err := prepareBMGSImage(raw.Bytes(), "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if pic.Width != 1280 || pic.Height != 640 {
		t.Fatal(pic.Width, pic.Height)
	}
	if len(pic.Bytes) >= len(raw.Bytes()) {
		t.Fatal("photo did not compress")
	}
	if _, err = prepareBMGSImage(raw.Bytes(), "image/png"); err == nil {
		t.Fatal("MIME spoof accepted")
	}
	if _, err = prepareBMGSImage(make([]byte, bmgsMaxInputImageBytes+1), "image/jpeg"); err == nil {
		t.Fatal("oversize accepted")
	}
	small := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	small.Set(0, 0, color.NRGBA{R: 255, A: 255})
	raw.Reset()
	png.Encode(&raw, small)
	pic, err = prepareBMGSImage(raw.Bytes(), "image/png")
	if err != nil || pic.Width != 3 || pic.Height != 2 {
		t.Fatal("upscaled", err)
	}
	decoded, _, _ := image.Decode(bytes.NewReader(pic.Bytes))
	_, _, _, alpha := decoded.At(1, 1).RGBA()
	if alpha != 0 {
		t.Fatal("transparency lost")
	}
	tooLarge := append([]byte{}, raw.Bytes()...)
	binary.BigEndian.PutUint32(tooLarge[16:20], 10000)
	binary.BigEndian.PutUint32(tooLarge[20:24], 10000)
	binary.BigEndian.PutUint32(tooLarge[29:33], crc32.ChecksumIEEE(tooLarge[12:29]))
	if _, err = prepareBMGSImage(tooLarge, "image/png"); err == nil || !strings.Contains(err.Error(), "pixel limit") {
		t.Fatal("pixel budget not checked before decoding", err)
	}
	for o := 1; o <= 8; o++ {
		got := bmgsOrient(small, o)
		w, h := 3, 2
		if o >= 5 {
			w, h = 2, 3
		}
		if got.Bounds().Dx() != w || got.Bounds().Dy() != h {
			t.Fatal("bad orientation", o)
		}
	}
	// APP1 TIFF orientation=6, followed by the original valid JPEG.
	raw.Reset()
	jpeg.Encode(&raw, small, nil)
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	oriented := append([]byte{0xff, 0xd8, 0xff, 0xe1, 0, byte(len(exif) + 2)}, exif...)
	oriented = append(oriented, raw.Bytes()[2:]...)
	pic, err = prepareBMGSImage(oriented, "image/jpeg")
	if err != nil || pic.Width != 2 || pic.Height != 3 {
		t.Fatal("EXIF ignored", err, pic.Width, pic.Height)
	}
}
