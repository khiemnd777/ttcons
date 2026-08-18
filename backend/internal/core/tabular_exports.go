package core

import (
	"archive/zip"
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
			Columns: []tabularColumn{{"name", "Công trình", false}, {"status", "Trạng thái", false}, {"progress_percent", "Tiến độ (%)", true}, {"open_issues", "Vấn đề mở", true}, {"pending_approvals", "Chờ duyệt", true}},
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
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	files := map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/><Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/><Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/></Types>`,
		"_rels/.rels":                `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/><Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/></Relationships>`,
		"xl/workbook.xml":            `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Thống kê" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"xl/styles.xml":              `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="2"><font><sz val="11"/><name val="Arial"/></font><font><b/><color rgb="FFFFFFFF"/><sz val="11"/><name val="Arial"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF2563EB"/><bgColor indexed="64"/></patternFill></fill></fills><borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="3"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="1" borderId="0" xfId="0" applyFont="1" applyFill="1"/><xf numFmtId="4" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/></cellXfs></styleSheet>`,
		"docProps/app.xml":           `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"><Application>Thủ Thiêm Workflow</Application></Properties>`,
	}
	coreTitle := xmlEscape(definition.Title)
	timestamp := generatedAt.UTC().Format(time.RFC3339)
	files["docProps/core.xml"] = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><dc:title>%s</dc:title><dc:creator>Thủ Thiêm Workflow</dc:creator><dcterms:created xsi:type="dcterms:W3CDTF">%s</dcterms:created></cp:coreProperties>`, coreTitle, timestamp)
	files["xl/worksheets/sheet1.xml"] = generateWorksheetXML(definition, rows)

	order := []string{"[Content_Types].xml", "_rels/.rels", "docProps/app.xml", "docProps/core.xml", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/styles.xml", "xl/worksheets/sheet1.xml"}
	for _, name := range order {
		entry, err := archive.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write([]byte(files[name])); err != nil {
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
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
