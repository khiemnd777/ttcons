package core

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"time"

	"codeberg.org/go-pdf/fpdf"
)

const (
	workReportPDFMargin = 17.0
	workReportPDFWidth  = 176.0
	workReportPDFBottom = 272.0
	workReportPDFLine   = 5.6
)

var workReportPDFTimezone = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)

func generateWorkReportPDF(revision Record) (tabularExportResult, error) {
	snapshot := recordFromAny(revision["snapshot_json"])
	revisionNo := int(number(revision["revision_no"]))
	submittedAt := workReportTime(revision["submitted_at"])
	reportDate := workReportPDFDate(snapshot["report_date"])
	structured := recordFromAny(snapshot["structured_json"])

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(workReportPDFMargin, 17, workReportPDFMargin)
	// All body lines are paginated explicitly, including shaded blockers and
	// metadata pairs. This keeps labels with content and footers out of the body.
	pdf.SetAutoPageBreak(false, 25)
	pdf.SetCellMargin(0)
	pdf.SetCompression(true)
	pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "", pipelinePDFRegularFont)
	pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "B", pipelinePDFBoldFont)
	pdf.SetTitle("Báo cáo thi công ngày "+workReportPDFValue(reportDate), true)
	pdf.SetSubject("Báo cáo công việc đã gửi", true)
	pdf.SetCreator("Thủ Thiêm Workflow", true)
	pdf.SetLang("vi-VN")
	if !submittedAt.IsZero() {
		pdf.SetCreationDate(submittedAt.UTC())
		pdf.SetModificationDate(submittedAt.UTC())
	}
	pdf.AliasNbPages("{nb}")
	pdf.SetHeaderFunc(func() {
		if pdf.PageNo() == 1 {
			return
		}
		pdf.SetFont(pipelinePDFFontFamily, "B", 9)
		pdf.SetTextColor(23, 46, 70)
		pdf.CellFormat(92, 5, "BÁO CÁO THI CÔNG · TIẾP THEO", "", 0, "L", false, 0, "")
		pdf.SetFont(pipelinePDFFontFamily, "", 8)
		pdf.CellFormat(84, 5, fmt.Sprintf("%s · Lần gửi %02d", workReportPDFValue(reportDate), revisionNo), "", 1, "R", false, 0, "")
		pdf.SetDrawColor(216, 225, 233)
		pdf.Line(workReportPDFMargin, 25, 193, 25)
		pdf.SetXY(workReportPDFMargin, 30)
	})
	pdf.SetFooterFunc(func() {
		pdf.SetDrawColor(216, 225, 233)
		pdf.Line(workReportPDFMargin, 280, 193, 280)
		pdf.SetXY(workReportPDFMargin, 283)
		pdf.SetFont(pipelinePDFFontFamily, "", 8)
		pdf.SetTextColor(86, 103, 120)
		pdf.CellFormat(140, 4, fmt.Sprintf("Báo cáo ngày %s · Lần gửi %02d", workReportPDFValue(reportDate), revisionNo), "", 0, "L", false, 0, "")
		pdf.CellFormat(36, 4, fmt.Sprintf("Trang %d/{nb}", pdf.PageNo()), "", 0, "R", false, 0, "")
	})
	pdf.AddPage()
	renderer := workReportPDFRenderer{pdf: pdf}
	renderer.heading(reportDate)
	renderer.label("CÔNG TRÌNH")
	renderer.paragraph(workReportPDFValue(snapshot["project_name"]), workReportPDFMargin, workReportPDFWidth, "B", 12, false)
	pdf.Ln(4)
	renderer.metaPair("Người lập", workReportPDFValue(snapshot["author_name"]), "Người nhận", workReportPDFValue(revision["recipient_name"]))
	submittedLabel := "Chưa ghi nhận"
	if !submittedAt.IsZero() {
		submittedLabel = submittedAt.In(workReportPDFTimezone).Format("15:04 02-01-2006")
	}
	renderer.metaPair("Lần gửi", fmt.Sprintf("%02d", revisionNo), "Gửi lúc", submittedLabel)
	renderer.summary(snapshot)

	completed := workReportRecordList(structured["completed_items"])
	done := workReportItemsByResult(completed, "done")
	current := workReportItemsByResult(completed, "partial")
	blocked := workReportItemsByResult(completed, "blocked")
	next := workReportItemTitles(workReportRecordList(structured["next_plan_items"]))
	blockers := append(blocked, workReportBlockerTitles(workReportRecordList(structured["blockers"]))...)
	if len(done)+len(current)+len(next) == 0 && exportCellText(snapshot["work_summary"]) != "" {
		// Legacy free-text summaries do not identify a work state. Keep that
		// content readable without assigning it to an invented table column.
		renderer.section("01", "CÔNG VIỆC TRONG NGÀY")
		renderer.paragraph(workReportPDFValue(snapshot["work_summary"]), workReportPDFMargin, workReportPDFWidth, "", 10.5, false)
	} else {
		renderer.workTable([3][]string{done, current, next})
	}
	renderer.section("02", "VẤN ĐỀ VÀ TRỞ NGẠI")
	if len(blockers) == 0 {
		renderer.paragraph("Chưa ghi nhận vấn đề hoặc trở ngại.", workReportPDFMargin, workReportPDFWidth, "", 10.5, false)
	} else {
		for _, blocker := range blockers {
			renderer.paragraph("• "+blocker, workReportPDFMargin+3, workReportPDFWidth-6, "", 10.5, true)
		}
	}
	renderer.section("03", "NGUỒN LỰC")
	renderer.resource("Nhân lực", snapshot["manpower_summary"])
	renderer.resource("Vật tư sử dụng", snapshot["material_usage_summary"])
	renderer.section("04", "GHI CHÚ HIỆN TRƯỜNG")
	renderer.paragraph(workReportPDFValue(snapshot["issues_summary"]), workReportPDFMargin, workReportPDFWidth, "", 10.5, false)
	renderer.verification(revision["snapshot_sha256"])

	var buffer bytes.Buffer
	if err := pdf.Output(&buffer); err != nil {
		return tabularExportResult{}, err
	}
	fileDate := reportDate
	if fileDate == "" {
		fileDate = "khong-ro-ngay"
	}
	return tabularExportResult{Data: buffer.Bytes(), Filename: fmt.Sprintf("bao-cao-thi-cong-%s-lan-%d.pdf", fileDate, revisionNo), ContentType: exportPDFContentType}, nil
}

type workReportPDFRenderer struct {
	pdf *fpdf.Fpdf
}

func (r workReportPDFRenderer) ensure(height float64) {
	if r.pdf.GetY()+height > workReportPDFBottom {
		r.pdf.AddPage()
	}
}

func (r workReportPDFRenderer) heading(date string) {
	pdf := r.pdf
	pdf.SetFillColor(31, 89, 119)
	pdf.Rect(workReportPDFMargin, 17, 12, 1.2, "F")
	pdf.SetXY(workReportPDFMargin, 23)
	pdf.SetFont(pipelinePDFFontFamily, "B", 22)
	pdf.SetTextColor(23, 46, 70)
	pdf.CellFormat(120, 10, "BÁO CÁO THI CÔNG", "", 0, "L", false, 0, "")
	pdf.SetXY(139, 21)
	pdf.SetFont(pipelinePDFFontFamily, "B", 8)
	pdf.SetTextColor(86, 103, 120)
	pdf.CellFormat(54, 5, "NGÀY BÁO CÁO", "", 1, "R", false, 0, "")
	pdf.SetXY(139, 27)
	pdf.SetFont(pipelinePDFFontFamily, "B", 16)
	pdf.SetTextColor(23, 46, 70)
	pdf.CellFormat(54, 8, workReportPDFValue(date), "", 1, "R", false, 0, "")
	pdf.SetXY(workReportPDFMargin, 35)
	pdf.SetFont(pipelinePDFFontFamily, "", 10)
	pdf.SetTextColor(86, 103, 120)
	pdf.CellFormat(0, 6, "Báo cáo công việc trong ngày", "", 1, "L", false, 0, "")
	pdf.Ln(7)
}

func (r workReportPDFRenderer) label(label string) {
	r.ensure(5 + workReportPDFLine)
	r.pdf.SetFont(pipelinePDFFontFamily, "B", 8)
	r.pdf.SetTextColor(86, 103, 120)
	r.pdf.CellFormat(0, 5, label, "", 1, "L", false, 0, "")
}

func (r workReportPDFRenderer) paragraph(text string, x, width float64, style string, size float64, warning bool) {
	pdf := r.pdf
	pdf.SetFont(pipelinePDFFontFamily, style, size)
	lines := pdf.SplitText(strings.ReplaceAll(text, "\r", ""), width)
	for _, line := range lines {
		r.ensure(workReportPDFLine)
		y := pdf.GetY()
		if warning {
			pdf.SetFillColor(255, 248, 231)
			pdf.Rect(workReportPDFMargin, y, workReportPDFWidth, workReportPDFLine, "F")
			pdf.SetFillColor(183, 119, 24)
			pdf.Rect(workReportPDFMargin, y, 0.7, workReportPDFLine, "F")
		}
		pdf.SetFont(pipelinePDFFontFamily, style, size)
		pdf.SetTextColor(38, 53, 68)
		pdf.SetXY(x, y)
		pdf.CellFormat(width, workReportPDFLine, line, "", 0, "L", false, 0, "")
		pdf.SetXY(workReportPDFMargin, y+workReportPDFLine)
	}
}

func (r workReportPDFRenderer) metaPair(leftLabel, leftValue, rightLabel, rightValue string) {
	pdf := r.pdf
	r.ensure(5 + workReportPDFLine)
	pdf.SetFont(pipelinePDFFontFamily, "B", 8)
	pdf.SetTextColor(86, 103, 120)
	pdf.CellFormat(91, 5, leftLabel, "", 0, "L", false, 0, "")
	pdf.CellFormat(85, 5, rightLabel, "", 1, "L", false, 0, "")
	pdf.SetFont(pipelinePDFFontFamily, "", 10.5)
	leftLines, rightLines := pdf.SplitText(leftValue, 85), pdf.SplitText(rightValue, 85)
	for i := 0; i < max(len(leftLines), len(rightLines)); i++ {
		r.ensure(workReportPDFLine)
		y := pdf.GetY()
		pdf.SetFont(pipelinePDFFontFamily, "", 10.5)
		pdf.SetTextColor(38, 53, 68)
		if i < len(leftLines) {
			pdf.CellFormat(85, workReportPDFLine, leftLines[i], "", 0, "L", false, 0, "")
		}
		pdf.SetXY(108, y)
		if i < len(rightLines) {
			pdf.CellFormat(85, workReportPDFLine, rightLines[i], "", 0, "L", false, 0, "")
		}
		pdf.SetXY(workReportPDFMargin, y+workReportPDFLine)
	}
	pdf.Ln(2)
}

func (r workReportPDFRenderer) summary(snapshot Record) {
	pdf := r.pdf
	pdf.SetFont(pipelinePDFFontFamily, "", 10.5)
	weather := workReportPDFValue(snapshot["weather"])
	weatherLines := pdf.SplitText(weather, 82)
	// Reserve the normal summary together. Unusually long free text still uses
	// the line paginator instead of overflowing a fixed-height panel.
	r.ensure(math.Min(math.Max(34, 16+float64(len(weatherLines))*workReportPDFLine), 45))
	pdf.Ln(3)
	y := pdf.GetY()
	pdf.SetDrawColor(216, 225, 233)
	pdf.Line(workReportPDFMargin, y, 193, y)
	pdf.SetXY(workReportPDFMargin, y+4)
	pdf.SetFont(pipelinePDFFontFamily, "B", 8)
	pdf.SetTextColor(86, 103, 120)
	pdf.CellFormat(91, 5, "TIẾN ĐỘ GHI NHẬN", "", 0, "L", false, 0, "")
	pdf.CellFormat(85, 5, "THỜI TIẾT", "", 1, "L", false, 0, "")
	valueY := pdf.GetY()
	progress, valid := exportNumber(snapshot["progress_percent"])
	valid = valid && !math.IsNaN(progress) && !math.IsInf(progress, 0) && progress >= 0 && progress <= 100
	label := "Chưa ghi nhận"
	pdf.SetFont(pipelinePDFFontFamily, "B", 11)
	if valid {
		label = fmt.Sprintf("%g%%", progress)
		pdf.SetFont(pipelinePDFFontFamily, "B", 20)
	}
	pdf.SetTextColor(23, 46, 70)
	pdf.CellFormat(85, 9, label, "", 0, "L", false, 0, "")
	if valid {
		pdf.SetFillColor(229, 235, 240)
		pdf.Rect(workReportPDFMargin, valueY+12, 76, 2, "F")
		if progress > 0 {
			pdf.SetFillColor(31, 89, 119)
			pdf.Rect(workReportPDFMargin, valueY+12, 76*progress/100, 2, "F")
		}
	}
	pdf.SetXY(workReportPDFMargin, valueY)
	startPage := pdf.PageNo()
	r.paragraph(weather, 108, 85, "", 10.5, false)
	if pdf.PageNo() == startPage {
		pdf.SetY(math.Max(pdf.GetY(), valueY+16))
	}
	pdf.SetDrawColor(216, 225, 233)
	pdf.Line(workReportPDFMargin, pdf.GetY()+2, 193, pdf.GetY()+2)
	pdf.Ln(4)
}

func (r workReportPDFRenderer) section(number, title string) {
	r.ensure(26)
	r.pdf.Ln(5)
	r.pdf.SetFont(pipelinePDFFontFamily, "B", 11)
	r.pdf.SetTextColor(31, 89, 119)
	r.pdf.CellFormat(10, 7, number, "", 0, "L", false, 0, "")
	r.pdf.SetTextColor(23, 46, 70)
	r.pdf.CellFormat(workReportPDFWidth-10, 7, title, "", 1, "L", false, 0, "")
	r.pdf.Ln(2)
}

func (r workReportPDFRenderer) workTable(groups [3][]string) {
	const (
		columnWidth  = workReportPDFWidth / 3
		padding      = 3.0
		headerHeight = 9.0
	)
	pdf := r.pdf
	var columns [3][]string
	maxLines := 0
	pdf.SetFont(pipelinePDFFontFamily, "", 10.5)
	for column, items := range groups {
		if len(items) == 0 {
			columns[column] = []string{"Chưa ghi nhận"}
		}
		for _, item := range items {
			columns[column] = append(columns[column], pdf.SplitText("• "+strings.ReplaceAll(item, "\r", ""), columnWidth-2*padding)...)
		}
		maxLines = max(maxLines, len(columns[column]))
	}
	// Keep the section title, table header and at least one content line on
	// the same page. Each column is a separate list, not a correlated row.
	r.ensure(14 + headerHeight + 2*padding + workReportPDFLine)
	r.section("01", "CÔNG VIỆC TRONG NGÀY")
	for offset := 0; offset < maxLines; {
		top := pdf.GetY()
		availableLines := int(math.Floor((workReportPDFBottom - top - headerHeight - 2*padding) / workReportPDFLine))
		lineCount := min(maxLines-offset, availableLines)
		if lineCount < 1 {
			pdf.AddPage()
			continue
		}
		height := headerHeight + 2*padding + float64(lineCount)*workReportPDFLine
		pdf.SetFillColor(237, 243, 247)
		pdf.Rect(workReportPDFMargin, top, workReportPDFWidth, headerHeight, "F")
		for column, title := range [3]string{"ĐÃ LÀM", "ĐANG LÀM", "SẼ LÀM"} {
			x := workReportPDFMargin + float64(column)*columnWidth
			pdf.SetXY(x+padding, top)
			pdf.SetFont(pipelinePDFFontFamily, "B", 9)
			pdf.SetTextColor(23, 46, 70)
			pdf.CellFormat(columnWidth-2*padding, headerHeight, title, "", 0, "L", false, 0, "")
			pdf.SetFont(pipelinePDFFontFamily, "", 10.5)
			pdf.SetTextColor(38, 53, 68)
			for row := 0; row < lineCount && offset+row < len(columns[column]); row++ {
				pdf.SetXY(x+padding, top+headerHeight+padding+float64(row)*workReportPDFLine)
				pdf.CellFormat(columnWidth-2*padding, workReportPDFLine, columns[column][offset+row], "", 0, "L", false, 0, "")
			}
		}
		pdf.SetDrawColor(203, 215, 225)
		pdf.SetLineWidth(0.2)
		pdf.Rect(workReportPDFMargin, top, workReportPDFWidth, height, "D")
		pdf.Line(workReportPDFMargin, top+headerHeight, workReportPDFMargin+workReportPDFWidth, top+headerHeight)
		for column := 1; column < 3; column++ {
			x := workReportPDFMargin + float64(column)*columnWidth
			pdf.Line(x, top, x, top+height)
		}
		pdf.SetXY(workReportPDFMargin, top+height)
		offset += lineCount
		if offset < maxLines {
			pdf.AddPage()
		}
	}
	pdf.Ln(3)
}

func (r workReportPDFRenderer) resource(label string, value any) {
	r.ensure(workReportPDFLine)
	y := r.pdf.GetY()
	r.pdf.SetFont(pipelinePDFFontFamily, "B", 9)
	r.pdf.SetTextColor(86, 103, 120)
	r.pdf.CellFormat(34, workReportPDFLine, label, "", 0, "L", false, 0, "")
	r.pdf.SetXY(workReportPDFMargin, y)
	r.paragraph(workReportPDFValue(value), workReportPDFMargin+34, workReportPDFWidth-34, "", 10.5, false)
	r.pdf.Ln(2)
}

func (r workReportPDFRenderer) verification(hash any) {
	r.ensure(25)
	r.pdf.Ln(6)
	r.pdf.SetDrawColor(216, 225, 233)
	r.pdf.Line(workReportPDFMargin, r.pdf.GetY(), 193, r.pdf.GetY())
	r.pdf.Ln(3)
	r.label("ĐỐI CHIẾU BẢN GỬI")
	r.paragraph("SHA-256: "+workReportPDFValue(hash), workReportPDFMargin, workReportPDFWidth, "", 8, false)
}

func workReportPDFValue(value any) string {
	text := exportCellText(value)
	if text == "" || text == "<nil>" {
		return "Chưa ghi nhận"
	}
	return text
}

// PDF presentation only: do not rewrite the snapshotted source date, its hash,
// or the shared notification date formatter.
func workReportPDFDate(value any) string {
	if date, err := time.Parse("2006-01-02", exportCellText(value)); err == nil {
		return date.Format("02-01-2006")
	}
	if date := workReportTime(value); !date.IsZero() {
		return date.In(workReportPDFTimezone).Format("02-01-2006")
	}
	return ""
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
	if parsed, ok := value.(time.Time); ok {
		return parsed
	}
	parsed, _ := time.Parse(time.RFC3339Nano, exportCellText(value))
	return parsed
}

func workReportDisplayDate(value string) string {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return value
	}
	return parsed.Format("02/01/2006")
}
