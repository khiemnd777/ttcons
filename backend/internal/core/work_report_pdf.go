package core

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"codeberg.org/go-pdf/fpdf"
)

func generateWorkReportPDF(revision Record) (tabularExportResult, error) {
	snapshot := recordFromAny(revision["snapshot_json"])
	revisionNo := int(number(revision["revision_no"]))
	submittedAt := workReportTime(revision["submitted_at"])
	reportDate := strings.TrimSpace(fmt.Sprint(snapshot["report_date"]))
	projectName := strings.TrimSpace(fmt.Sprint(snapshot["project_name"]))
	authorName := strings.TrimSpace(fmt.Sprint(snapshot["author_name"]))
	recipientName := strings.TrimSpace(fmt.Sprint(revision["recipient_name"]))
	structured := recordFromAny(snapshot["structured_json"])

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 14, 15)
	pdf.SetAutoPageBreak(true, 16)
	pdf.SetCompression(true)
	pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "", pipelinePDFRegularFont)
	pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "B", pipelinePDFBoldFont)
	pdf.SetTitle("Báo cáo thi công ngày "+reportDate, true)
	pdf.SetSubject("Báo cáo công việc đã gửi", true)
	pdf.SetCreator("Thủ Thiêm Workflow", true)
	pdf.SetLang("vi-VN")
	pdf.SetCreationDate(submittedAt.UTC())
	pdf.SetModificationDate(submittedAt.UTC())
	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont(pipelinePDFFontFamily, "", 7)
		pdf.SetTextColor(100, 116, 139)
		pdf.CellFormat(0, 4, fmt.Sprintf("Revision %d · Trang %d", revisionNo, pdf.PageNo()), "", 0, "R", false, 0, "")
	})
	pdf.AddPage()
	pdf.SetFont(pipelinePDFFontFamily, "B", 16)
	pdf.SetTextColor(15, 23, 42)
	pdf.MultiCell(0, 8, "BÁO CÁO THI CÔNG NGÀY "+workReportDisplayDate(reportDate), "", "L", false)
	pdf.SetFont(pipelinePDFFontFamily, "", 8)
	pdf.SetTextColor(71, 85, 105)
	pdf.CellFormat(0, 5, fmt.Sprintf("Mã revision: %d · Gửi lúc %s", revisionNo, submittedAt.In(time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)).Format("02/01/2006 15:04")), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	workReportPDFMeta(pdf, "Công trình", projectName)
	workReportPDFMeta(pdf, "Người lập", authorName)
	workReportPDFMeta(pdf, "Gửi đến", recipientName)
	workReportPDFMeta(pdf, "Thời tiết", exportCellText(snapshot["weather"]))
	workReportPDFMeta(pdf, "Tiến độ ghi nhận", exportCellText(snapshot["progress_percent"])+"%")
	pdf.Ln(3)

	completed := workReportRecordList(structured["completed_items"])
	done := workReportItemsByResult(completed, "done")
	current := workReportItemsByResult(completed, "partial")
	blocked := workReportItemsByResult(completed, "blocked")
	next := workReportItemTitles(workReportRecordList(structured["next_plan_items"]))
	blockers := append(blocked, workReportBlockerTitles(workReportRecordList(structured["blockers"]))...)
	if len(done)+len(current)+len(next)+len(blockers) == 0 {
		workReportPDFSection(pdf, "NỘI DUNG CÔNG VIỆC", []string{exportCellText(snapshot["work_summary"])}, "Chưa có nội dung công việc.")
	} else {
		workReportPDFSection(pdf, "ĐÃ LÀM", done, "Chưa ghi hạng mục hoàn tất.")
		workReportPDFSection(pdf, "ĐANG LÀM", current, "Không có hạng mục đang tiếp diễn.")
		workReportPDFSection(pdf, "SẼ LÀM", next, "Chưa ghi kế hoạch tiếp theo.")
		workReportPDFSection(pdf, "VẤN ĐỀ / TRỞ NGẠI", blockers, "Không có vấn đề hoặc trở ngại.")
	}
	workReportPDFSection(pdf, "NHÂN LỰC", []string{exportCellText(snapshot["manpower_summary"])}, "Chưa ghi nhân lực.")
	workReportPDFSection(pdf, "VẬT TƯ SỬ DỤNG", []string{exportCellText(snapshot["material_usage_summary"])}, "Chưa ghi vật tư.")
	workReportPDFSection(pdf, "GHI CHÚ HIỆN TRƯỜNG", []string{exportCellText(snapshot["issues_summary"])}, "Không có ghi chú.")

	pdf.SetDrawColor(203, 213, 225)
	pdf.SetFillColor(248, 250, 252)
	pdf.Rect(15, pdf.GetY()+2, 180, 18, "DF")
	pdf.SetXY(19, pdf.GetY()+5)
	pdf.SetFont(pipelinePDFFontFamily, "B", 8)
	pdf.SetTextColor(51, 65, 85)
	pdf.CellFormat(0, 4, "XÁC THỰC BẢN GỬI", "", 1, "L", false, 0, "")
	pdf.SetX(19)
	pdf.SetFont(pipelinePDFFontFamily, "", 7)
	pdf.SetTextColor(71, 85, 105)
	pdf.MultiCell(172, 4, "SHA-256: "+exportCellText(revision["snapshot_sha256"]), "", "L", false)

	var buffer bytes.Buffer
	if err := pdf.Output(&buffer); err != nil {
		return tabularExportResult{}, err
	}
	fileDate := strings.ReplaceAll(reportDate, "/", "-")
	if fileDate == "" {
		fileDate = "khong-ro-ngay"
	}
	return tabularExportResult{Data: buffer.Bytes(), Filename: fmt.Sprintf("bao-cao-thi-cong-%s-rev-%d.pdf", fileDate, revisionNo), ContentType: exportPDFContentType}, nil
}

func workReportPDFMeta(pdf *fpdf.Fpdf, label, value string) {
	pdf.SetFont(pipelinePDFFontFamily, "B", 8)
	pdf.SetTextColor(71, 85, 105)
	pdf.CellFormat(34, 6, label, "", 0, "L", false, 0, "")
	pdf.SetFont(pipelinePDFFontFamily, "", 9)
	pdf.SetTextColor(15, 23, 42)
	pdf.CellFormat(0, 6, value, "", 1, "L", false, 0, "")
}

func workReportPDFSection(pdf *fpdf.Fpdf, title string, items []string, empty string) {
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		if text := strings.TrimSpace(item); text != "" && text != "<nil>" {
			filtered = append(filtered, text)
		}
	}
	pdf.Ln(2)
	pdf.SetFont(pipelinePDFFontFamily, "B", 9)
	pdf.SetTextColor(30, 64, 175)
	pdf.CellFormat(0, 6, title, "B", 1, "L", false, 0, "")
	pdf.SetFont(pipelinePDFFontFamily, "", 8.5)
	if len(filtered) == 0 {
		pdf.SetTextColor(100, 116, 139)
		pdf.MultiCell(0, 5, empty, "", "L", false)
		return
	}
	pdf.SetTextColor(30, 41, 59)
	for _, item := range filtered {
		pdf.MultiCell(0, 5, "• "+item, "", "L", false)
	}
}

func workReportRecordList(value any) []Record {
	items, ok := value.([]any)
	if !ok {
		if records, ok := value.([]Record); ok {
			return records
		}
		return nil
	}
	result := make([]Record, 0, len(items))
	for _, item := range items {
		if record := recordFromAny(item); len(record) > 0 {
			result = append(result, record)
		}
	}
	return result
}

func workReportItemsByResult(items []Record, result string) []string {
	filtered := make([]Record, 0)
	for _, item := range items {
		if fmt.Sprint(item["result"]) == result {
			filtered = append(filtered, item)
		}
	}
	return workReportItemTitles(filtered)
}

func workReportItemTitles(items []Record) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(fmt.Sprint(item["title"]))
		if title != "" && title != "<nil>" {
			result = append(result, title)
		}
	}
	return result
}

func workReportBlockerTitles(items []Record) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(fmt.Sprint(item["content"]))
		if text == "" || text == "<nil>" {
			text = strings.TrimSpace(fmt.Sprint(item["title"]))
		}
		if text != "" && text != "<nil>" {
			result = append(result, text)
		}
	}
	return result
}

func workReportTime(value any) time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(fmt.Sprint(value)))
	if err != nil {
		return time.Now()
	}
	return parsed
}

func workReportDisplayDate(value string) string {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return value
	}
	return parsed.Format("02/01/2006")
}
