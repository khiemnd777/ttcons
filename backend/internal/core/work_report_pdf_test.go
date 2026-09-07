package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"codeberg.org/go-pdf/fpdf"
)

func workReportPDFFixture() Record {
	return Record{
		"revision_no": 1, "submitted_at": "2026-08-26T03:31:01Z", "recipient_name": "PM",
		"snapshot_sha256": "bec26905f83eb9c50cd246273e0df0a988eef3a7e910ae9711ca25033b9ad4ce",
		"snapshot_json": Record{
			"report_date":  "2026-08-25T00:00:00Z",
			"project_name": "Công trình mẫu đang lập kế hoạch 65% · 24/08/2026 02:43",
			"author_name":  "Giám sát", "weather": "Nắng", "progress_percent": 10,
			"structured_json": Record{
				"completed_items": []Record{{"title": "TEST", "result": "done"}, {"title": "TESTING", "result": "partial"}},
				"next_plan_items": []Record{{"title": "ON GOING TEST"}},
			},
		},
	}
}

func TestWorkReportPDFPageBoundaryKeepsLabelsWithContent(t *testing.T) {
	for _, kind := range []string{"section", "summary", "verification"} {
		t.Run(kind, func(t *testing.T) {
			pdf := fpdf.New("P", "mm", "A4", "")
			pdf.SetMargins(workReportPDFMargin, 17, workReportPDFMargin)
			pdf.SetAutoPageBreak(false, 25)
			pdf.SetCellMargin(0)
			pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "", pipelinePDFRegularFont)
			pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "B", pipelinePDFBoldFont)
			pdf.AddPage()
			pdf.SetY(249) // 23mm remain: not enough for the complete opening block.
			renderer := workReportPDFRenderer{pdf: pdf}
			switch kind {
			case "section":
				renderer.workTable([3][]string{{"Đã kiểm tra hiện trường."}, nil, {"Kiểm tra tiếp."}})
			case "summary":
				renderer.summary(Record{"weather": "Nắng", "progress_percent": 0})
			case "verification":
				renderer.verification(workReportPDFFixture()["snapshot_sha256"])
			}
			if pdf.PageNo() != 2 || pdf.GetY() > workReportPDFBottom {
				t.Fatalf("opening block must move intact to page 2: page=%d y=%f", pdf.PageNo(), pdf.GetY())
			}
			if err := pdf.Error(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWorkReportPDFDateFilename(t *testing.T) {
	for _, tc := range []struct{ date, want string }{
		{"2026-08-25T00:00:00Z", "25-08-2026"},
		{"2026-08-25", "25-08-2026"},
		{"2026-08-25T00:00:00.123456Z", "25-08-2026"},
		{"2026-08-25T18:00:00Z", "26-08-2026"},
		{"2026-08-25T00:00:00+07:00", "25-08-2026"},
		{"", "khong-ro-ngay"},
		{"2026-02-30", "khong-ro-ngay"},
		{"../../invalid", "khong-ro-ngay"},
	} {
		t.Run(tc.date, func(t *testing.T) {
			revision := workReportPDFFixture()
			recordFromAny(revision["snapshot_json"])["report_date"] = tc.date
			result, err := generateWorkReportPDF(revision)
			if err != nil {
				t.Fatal(err)
			}
			if want := "bao-cao-thi-cong-" + tc.want + "-lan-1.pdf"; result.Filename != want {
				t.Fatalf("filename = %q, want %q", result.Filename, want)
			}
		})
	}
}

// Optional artifacts allow repeatable PDF extraction and visual QA without a
// database, browser, network access or writing to the production report ledger.
func TestWorkReportPDFFixtures(t *testing.T) {
	for _, kind := range []string{"short", "long", "empty", "overflow", "columns", "legacy"} {
		t.Run(kind, func(t *testing.T) {
			revision := workReportPDFFixture()
			snapshot := recordFromAny(revision["snapshot_json"])
			switch kind {
			case "long":
				snapshot["project_name"] = strings.Repeat("Công trình cải tạo khối văn phòng và khu dịch vụ tại Thủ Thiêm. ", 4)
				snapshot["author_name"] = "Nguyễn Văn Giám Sát - Phụ trách đội thi công hoàn thiện khu vực phía Đông"
				revision["recipient_name"] = "Trần Quản Lý - Ban quản lý công trình và điều phối thi công"
				snapshot["weather"] = "Mưa vào buổi sáng, nắng nhẹ buổi chiều; gió mạnh tại các tầng cao."
				snapshot["progress_percent"] = 62.5
				items := make([]Record, 0, 16)
				for i := 1; i <= 16; i++ {
					items = append(items, Record{"result": "done", "title": fmt.Sprintf("Hạng mục %02d: %s Kết thúc hạng mục %02d.", i, strings.Repeat("Thi công hoàn thiện tường, kiểm tra cao độ và bảo vệ vật tư tại hiện trường. ", 3), i)})
				}
				snapshot["structured_json"] = Record{
					"completed_items": items,
					"next_plan_items": []Record{{"title": "Tiếp tục nghiệm thu các khu vực đủ điều kiện."}},
					"blockers":        []Record{{"content": strings.Repeat("Cần xử lý thấm tại khu vực tiếp giáp trước khi hoàn thiện. ", 8)}},
				}
				snapshot["manpower_summary"] = strings.Repeat("Đội xây dựng: 12 người. Đội điện nước: 8 người.\n", 5)
				snapshot["material_usage_summary"] = strings.Repeat("Xi măng: 30 bao; thép: 250 kg; gạch: 2.000 viên.\n", 5)
				snapshot["issues_summary"] = "Ghi chú cuối báo cáo dài: bảo vệ khu vực đã hoàn thiện."
			case "empty":
				revision["snapshot_json"] = Record{}
				revision["submitted_at"] = nil
				revision["recipient_name"] = nil
			case "overflow":
				snapshot["structured_json"] = Record{"blockers": []Record{{"content": strings.Repeat("Rủi ro cần phối hợp xử lý, ghi nhận đầy đủ diễn biến tại công trường. ", 100) + "KẾT THÚC TRỞ NGẠI."}}}
				snapshot["issues_summary"] = strings.Repeat("CHUOIKHONGBATKYDAUCACH", 60) + " KẾT THÚC GHI CHÚ."
			case "columns":
				snapshot["structured_json"] = Record{
					"completed_items": []Record{
						{"result": "done", "title": "DONE-BEGIN " + strings.Repeat("Hoàn thiện trát tường khu vực phía đông. ", 12) + "DONE-END"},
						{"result": "partial", "title": "CURRENT-BEGIN " + strings.Repeat("Đang kiểm tra đường ống và vị trí đấu nối. ", 50) + "CURRENT-END"},
					},
					"next_plan_items": []Record{{"title": "NEXT-BEGIN " + strings.Repeat("Nghiệm thu trước khi chuyển công tác tiếp theo. ", 70) + "NEXT-END"}},
				}
			case "legacy":
				snapshot["structured_json"] = nil
				snapshot["work_summary"] = "Nội dung báo cáo cũ chưa phân loại: khảo sát khu vực thi công."
			}
			before, _ := json.Marshal(revision)
			result, err := generateWorkReportPDF(revision)
			if err != nil {
				t.Fatal(err)
			}
			after, _ := json.Marshal(revision)
			if !bytes.Equal(before, after) {
				t.Fatal("PDF rendering mutated the immutable revision")
			}
			if !bytes.HasPrefix(result.Data, []byte("%PDF-")) || result.ContentType != "application/pdf" {
				t.Fatal("expected real PDF bytes and MIME")
			}
			pages := len(regexp.MustCompile(`/Type /Page\b`).FindAll(result.Data, -1))
			if (kind == "short" || kind == "empty" || kind == "legacy") && pages != 1 {
				t.Fatalf("compact report has %d pages, want 1", pages)
			}
			if (kind == "long" || kind == "overflow" || kind == "columns") && pages < 2 {
				t.Fatalf("long report has %d pages, want multiple", pages)
			}
			if outputDir := os.Getenv("WORK_REPORT_PDF_ARTIFACT_DIR"); outputDir != "" {
				if err := os.MkdirAll(outputDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(outputDir, kind+".pdf"), result.Data, 0644); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
