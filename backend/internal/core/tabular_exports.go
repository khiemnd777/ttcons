package core

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"

	"codeberg.org/go-pdf/fpdf"
)

const (
	exportCSVContentType  = "text/csv; charset=utf-8"
	exportXLSXContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	exportPDFContentType  = "application/pdf"
)

type tabularColumn struct {
	Key     string
	Header  string
	Numeric bool
}

type tabularDefinition struct {
	Title    string
	FileSlug string
	Columns  []tabularColumn
}

type tabularExportResult struct {
	Data        []byte
	Filename    string
	ContentType string
}

func statisticDefinition(kind string) (tabularDefinition, error) {
	definitions := map[string]tabularDefinition{
		"projects": {
			Title: "Thống kê công trình và tiến độ", FileSlug: "thong-ke-cong-trinh",
			Columns: []tabularColumn{{"name", "Công trình", false}, {"is_sample", "Dữ liệu mẫu", false}, {"status", "Trạng thái", false}, {"progress_percent", "Tiến độ (%)", true}, {"open_issues", "Vấn đề mở", true}, {"pending_approvals", "Chờ duyệt", true}},
		},
		"warehouse": {
			Title: "Thống kê vật tư và kho", FileSlug: "thong-ke-vat-tu-kho",
			Columns: []tabularColumn{{"name", "Vật tư", false}, {"category", "Nhóm", false}, {"stock_quantity", "Tồn", true}, {"surplus_quantity", "Khối lượng dư", true}},
		},
		"procurement": {
			Title: "Thống kê mua sắm", FileSlug: "thong-ke-mua-sam",
			Columns: []tabularColumn{{"status", "Trạng thái", false}, {"budget_status", "Ngân sách", false}, {"total", "Số yêu cầu", true}, {"amount", "Giá trị", true}},
		},
		"accounting": {
			Title: "Thống kê công nợ", FileSlug: "thong-ke-cong-no",
			Columns: []tabularColumn{{"name", "Công trình", false}, {"payable_remaining", "Còn phải trả", true}, {"receivable_remaining", "Còn phải thu", true}, {"used_budget", "Ngân sách đã dùng", true}},
		},
		"staff": {
			Title: "Thống kê nhân sự công trình", FileSlug: "thong-ke-nhan-su-cong-trinh",
			Columns: []tabularColumn{{"full_name", "Nhân sự", false}, {"department", "Phòng ban", false}, {"position", "Chức vụ", false}, {"project_count", "Số công trình", true}},
		},
	}
	definition, ok := definitions[kind]
	if !ok {
		return tabularDefinition{}, validationError("Loại thống kê không hợp lệ", Record{"kind": kind})
	}
	return definition, nil
}

func generateTabularExport(definition tabularDefinition, rows []Record, format string, generatedAt time.Time) (tabularExportResult, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "csv":
		data, err := generateTabularCSV(definition, rows)
		return tabularExportResult{Data: data, Filename: definition.FileSlug + ".csv", ContentType: exportCSVContentType}, err
	case "xlsx", "excel":
		data, err := generateTabularXLSX(definition, rows, generatedAt)
		return tabularExportResult{Data: data, Filename: definition.FileSlug + ".xlsx", ContentType: exportXLSXContentType}, err
	case "pdf":
		data, err := generateTabularPDF(definition, rows, generatedAt)
		return tabularExportResult{Data: data, Filename: definition.FileSlug + ".pdf", ContentType: exportPDFContentType}, err
	default:
		return tabularExportResult{}, validationError("Định dạng xuất dữ liệu không hợp lệ", Record{"format": format, "allowed": []string{"pdf", "xlsx", "csv"}})
	}
}

func generateTabularCSV(definition tabularDefinition, rows []Record) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(&buffer)
	headers := make([]string, len(definition.Columns))
	for index, column := range definition.Columns {
		headers[index] = column.Header
	}
	if err := writer.Write(headers); err != nil {
		return nil, err
	}
	for _, row := range rows {
		values := make([]string, len(definition.Columns))
		for index, column := range definition.Columns {
			values[index] = exportCellText(row[column.Key])
		}
		if err := writer.Write(values); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func generateTabularXLSX(definition tabularDefinition, rows []Record, generatedAt time.Time) ([]byte, error) {
	return generateWorkbookXLSX(definition.Title, []xlsxWorkbookSheet{{
		Name: "Thống kê", Columns: definition.Columns, Rows: rows,
	}}, generatedAt)
}

func generateWorksheetXML(definition tabularDefinition, rows []Record) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols>`)
	for index := range definition.Columns {
		builder.WriteString(fmt.Sprintf(`<col min="%d" max="%d" width="22" customWidth="1"/>`, index+1, index+1))
	}
	builder.WriteString(`</cols><sheetData>`)
	builder.WriteString(`<row r="1">`)
	for index, column := range definition.Columns {
		builder.WriteString(xlsxInlineCell(index, 1, column.Header, 1))
	}
	builder.WriteString(`</row>`)
	for rowIndex, row := range rows {
		excelRow := rowIndex + 2
		builder.WriteString(fmt.Sprintf(`<row r="%d">`, excelRow))
		for columnIndex, column := range definition.Columns {
			value := row[column.Key]
			if column.Numeric {
				if number, ok := exportNumber(value); ok {
					builder.WriteString(fmt.Sprintf(`<c r="%s%d" s="2"><v>%s</v></c>`, excelColumnName(columnIndex), excelRow, strconv.FormatFloat(number, 'f', -1, 64)))
					continue
				}
			}
			builder.WriteString(xlsxInlineCell(columnIndex, excelRow, exportCellText(value), 0))
		}
		builder.WriteString(`</row>`)
	}
	builder.WriteString(`</sheetData><autoFilter ref="A1:` + excelColumnName(len(definition.Columns)-1) + `1"/></worksheet>`)
	return builder.String()
}

func xlsxInlineCell(columnIndex, rowIndex int, value string, style int) string {
	return fmt.Sprintf(`<c r="%s%d" s="%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, excelColumnName(columnIndex), rowIndex, style, xmlEscape(value))
}

func excelColumnName(index int) string {
	index++
	name := ""
	for index > 0 {
		index--
		name = string(rune('A'+index%26)) + name
		index /= 26
	}
	return name
}

func generateTabularPDF(definition tabularDefinition, rows []Record, generatedAt time.Time) ([]byte, error) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(10, 10, 10)
	pdf.SetAutoPageBreak(true, 12)
	pdf.SetCompression(true)
	pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "", pipelinePDFRegularFont)
	pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "B", pipelinePDFBoldFont)
	pdf.SetTitle(definition.Title, true)
	pdf.SetCreator("Thủ Thiêm Workflow", true)
	pdf.SetLang("vi-VN")
	pdf.SetCreationDate(generatedAt.UTC())
	pdf.SetFooterFunc(func() {
		pdf.SetY(-8)
		pdf.SetFont(pipelinePDFFontFamily, "", 7)
		pdf.SetTextColor(100, 116, 139)
		pdf.CellFormat(0, 4, fmt.Sprintf("Trang %d", pdf.PageNo()), "", 0, "R", false, 0, "")
	})
	pdf.AddPage()
	pdf.SetFont(pipelinePDFFontFamily, "B", 16)
	pdf.SetTextColor(15, 23, 42)
	pdf.CellFormat(0, 9, definition.Title, "", 1, "L", false, 0, "")
	pdf.SetFont(pipelinePDFFontFamily, "", 8)
	pdf.SetTextColor(71, 85, 105)
	pdf.CellFormat(0, 6, "Xuất lúc "+generatedAt.In(time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)).Format("02/01/2006 15:04")+fmt.Sprintf(" · %d dòng", len(rows)), "", 1, "L", false, 0, "")
	pdf.Ln(2)
	width := 277.0 / float64(len(definition.Columns))
	drawHeader := func() {
		pdf.SetFont(pipelinePDFFontFamily, "B", 8)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFillColor(37, 99, 235)
		for _, column := range definition.Columns {
			pdf.CellFormat(width, 7, truncateExportText(column.Header, 42), "1", 0, "L", true, 0, "")
		}
		pdf.Ln(-1)
	}
	drawHeader()
	for _, row := range rows {
		if pdf.GetY() > 188 {
			pdf.AddPage()
			drawHeader()
		}
		pdf.SetFont(pipelinePDFFontFamily, "", 7.5)
		pdf.SetTextColor(30, 41, 59)
		for _, column := range definition.Columns {
			align := "L"
			if column.Numeric {
				align = "R"
			}
			pdf.CellFormat(width, 6.5, truncateExportText(exportCellText(row[column.Key]), 48), "1", 0, align, false, 0, "")
		}
		pdf.Ln(-1)
	}
	var buffer bytes.Buffer
	if err := pdf.Output(&buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func exportCellText(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func exportNumber(value any) (float64, bool) {
	switch number := value.(type) {
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	case float32:
		return float64(number), true
	case float64:
		return number, true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(number), 64)
		return parsed, err == nil
	default:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
		return parsed, err == nil
	}
}

func xmlEscape(value string) string {
	var buffer bytes.Buffer
	_ = xml.EscapeText(&buffer, []byte(value))
	return buffer.String()
}

func truncateExportText(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes-1]) + "…"
}
