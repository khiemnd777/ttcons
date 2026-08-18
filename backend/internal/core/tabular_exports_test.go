package core

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"io"
	"strings"
	"testing"
	"time"
)

func TestGenerateTabularExportProducesRealFormats(t *testing.T) {
	definition, err := statisticDefinition("projects")
	if err != nil {
		t.Fatal(err)
	}
	rows := []Record{{"name": "Công trình Thủ Thiêm", "status": "active", "progress_percent": 42.5, "open_issues": 2, "pending_approvals": 1}}
	generatedAt := time.Date(2026, 8, 18, 2, 30, 0, 0, time.UTC)

	csvExport, err := generateTabularExport(definition, rows, "csv", generatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(csvExport.Data, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatal("CSV must carry a UTF-8 BOM for Vietnamese Excel users")
	}
	csvRows, err := csv.NewReader(bytes.NewReader(csvExport.Data[3:])).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(csvRows) != 2 || csvRows[0][0] != "Công trình" || csvRows[1][0] != "Công trình Thủ Thiêm" {
		t.Fatalf("unexpected CSV rows: %#v", csvRows)
	}

	xlsxExport, err := generateTabularExport(definition, rows, "xlsx", generatedAt)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(xlsxExport.Data), int64(len(xlsxExport.Data)))
	if err != nil {
		t.Fatalf("XLSX must be a valid ZIP package: %v", err)
	}
	worksheet := zipFileText(t, archive, "xl/worksheets/sheet1.xml")
	if !strings.Contains(worksheet, "Công trình Thủ Thiêm") || !strings.Contains(worksheet, `<v>42.5</v>`) {
		t.Fatalf("XLSX worksheet does not preserve text and numeric cells: %s", worksheet)
	}

	pdfExport, err := generateTabularExport(definition, rows, "pdf", generatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdfExport.Data, []byte("%PDF-")) {
		t.Fatal("PDF export must contain a real PDF document")
	}
	if pdfExport.ContentType != exportPDFContentType || xlsxExport.ContentType != exportXLSXContentType || csvExport.ContentType != exportCSVContentType {
		t.Fatal("export content types must match the generated format")
	}
}

func TestGenerateTabularExportRejectsUnknownFormat(t *testing.T) {
	definition, err := statisticDefinition("staff")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := generateTabularExport(definition, nil, "xls", time.Now()); err == nil {
		t.Fatal("legacy XLS is not an accepted alias for the XLSX implementation")
	}
}

func TestNormalizeWorkReportFilter(t *testing.T) {
	filter, err := normalizeWorkReportFilter(WorkReportFilter{Search: "  tiến độ  "})
	if err != nil {
		t.Fatal(err)
	}
	if filter.Scope != "mine" || filter.Search != "tiến độ" {
		t.Fatalf("unexpected normalized filter: %#v", filter)
	}
	for _, invalid := range []WorkReportFilter{
		{Scope: "team"},
		{Status: "approved"},
		{DateFrom: "2026-08-20", DateTo: "2026-08-18"},
		{ProjectID: "not-a-uuid"},
	} {
		if _, err := normalizeWorkReportFilter(invalid); err == nil {
			t.Fatalf("filter should be rejected: %#v", invalid)
		}
	}
}

func TestFilterAndSortStatisticRowsMatchesExportScope(t *testing.T) {
	rows := []Record{
		{"name": "Công trình B", "status": "active", "progress_percent": 20, "open_issues": 3, "pending_approvals": 0},
		{"name": "Công trình A", "status": "planning", "progress_percent": 80, "open_issues": 1, "pending_approvals": 2},
		{"name": "Kho khác", "status": "active", "progress_percent": 90, "open_issues": 0, "pending_approvals": 0},
	}
	filtered, err := filterAndSortStatisticRows("projects", rows, OperationalReportRequest{Search: "công trình", Sort: "progress_percent", Direction: "desc"})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 2 || filtered[0]["name"] != "Công trình A" || filtered[1]["name"] != "Công trình B" {
		t.Fatalf("filtered/sorted statistics = %#v", filtered)
	}
	if _, err := filterAndSortStatisticRows("projects", rows, OperationalReportRequest{Sort: "secret"}); err == nil {
		t.Fatal("unknown sort columns must be rejected")
	}
}

func zipFileText(t *testing.T, archive *zip.Reader, name string) string {
	t.Helper()
	for _, file := range archive.File {
		if file.Name != name {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		content, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	t.Fatalf("missing ZIP entry %s", name)
	return ""
}
