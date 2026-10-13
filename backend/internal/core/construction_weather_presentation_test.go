package core

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/xuri/excelize/v2"
)

func TestConstructionWeatherPresentationExportsPreserveSource(t *testing.T) {
	cases := []struct {
		value any
		label string
	}{
		{"normal", "Bình thường"}, {"hot", "Nắng nóng"}, {"rain", "Mưa"}, {"wind", "Gió mạnh"}, {"restricted_access", "Hạn chế mặt bằng"}, {"other", "Khác"},
		{"Nắng", "Nắng"}, {"Mưa nhẹ tại công trường", "Mưa nhẹ tại công trường"}, {"unknown_code", "unknown_code"}, {"  Custom weather  ", "  Custom weather  "}, {nil, "Chưa ghi thời tiết"}, {" ", "Chưa ghi thời tiết"}, {Record{"value": "hot"}, "Nắng nóng"}, {Record{"value": nil}, "Chưa ghi thời tiết"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if got := constructionReportWeatherLabel(tc.value); got != tc.label {
				t.Fatalf("label=%q want=%q", got, tc.label)
			}
			row := Record{"weather": tc.value, "report_date": "2026-10-12", "structured_json": Record{}}
			before, _ := json.Marshal(row)
			for _, kind := range []string{"daily_report", "work_report", "workflow_form"} {
				row["form_code"] = "builtin_construction_daily_report"
				sheet, err := nativeBMGSDailySheet(BMGSSourceRef{Kind: kind, ID: "weather-source", RevisionNo: 1}, row)
				if err != nil {
					t.Fatal(err)
				}
				data, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-02", Sheets: []BMGSSheet{sheet}})
				if err != nil {
					t.Fatal(err)
				}
				book, err := excelize.OpenReader(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				got, err := book.GetCellValue(book.GetSheetList()[0], "B6")
				book.Close()
				if err != nil || got != tc.label {
					t.Fatalf("%s weather cell=%q want=%q err=%v", kind, got, tc.label, err)
				}
			}
			delete(row, "form_code")
			revision := workReportPDFFixture()
			recordFromAny(revision["snapshot_json"])["weather"] = tc.value
			frozen, _ := json.Marshal(revision)
			pdf, err := generateWorkReportPDF(revision)
			if err != nil {
				t.Fatal(err)
			}
			if !weatherPDFHasText(pdf.Data, tc.label) {
				t.Fatalf("PDF lacks rendered label %q", tc.label)
			}
			after, _ := json.Marshal(row)
			afterRevision, _ := json.Marshal(revision)
			if !bytes.Equal(before, after) || !bytes.Equal(frozen, afterRevision) || sha256Hex(frozen) != sha256Hex(afterRevision) {
				t.Fatal("presentation changed raw source or snapshot hash")
			}
		})
	}
}

func weatherPDFHasText(data []byte, text string) bool {
	encoded := make([]byte, 0, len(text)*2)
	for _, unit := range utf16.Encode([]rune(text)) {
		encoded = binary.BigEndian.AppendUint16(encoded, unit)
	}
	for _, stream := range regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`).FindAllSubmatch(data, -1) {
		reader, err := zlib.NewReader(bytes.NewReader(stream[1]))
		if err != nil {
			continue
		}
		decoded, err := io.ReadAll(reader)
		reader.Close()
		if err == nil && bytes.Contains(decoded, encoded) {
			return true
		}
	}
	return false
}

func TestConstructionWeatherPresentationPinnedOptionsAndV20Parity(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "migrations", "V20__construction_daily_report_form.sql"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?s)'weather',.*?'(\[.*?\])'::jsonb`).FindSubmatch(data)
	if len(match) != 2 {
		t.Fatal("V20 weather option contract missing")
	}
	var options []Record
	if err := json.Unmarshal(match[1], &options); err != nil {
		t.Fatal(err)
	}
	if len(options) != 6 {
		t.Fatal("weather contract changed")
	}
	for _, option := range options {
		if got := constructionReportWeatherLabel(option["value"]); got != option["label"] {
			t.Fatalf("V20 parity failed: %v label=%s", option, got)
		}
	}
	for _, value := range []string{"hot", "Recorded custom weather"} {
		row := Record{"weather": value, "form_code": "builtin_construction_daily_report", "fields": []Record{{"field_key": "weather", "options_json": []Record{{"value": "hot", "label": "Pinned weather wording"}}}}}
		sheet, err := nativeBMGSDailySheet(BMGSSourceRef{Kind: "workflow_form", ID: "pinned-form"}, row)
		if err != nil {
			t.Fatal(err)
		}
		want := value
		if value == "hot" {
			want = "Pinned weather wording"
		}
		if sheet.Metadata["weather"] != want {
			t.Fatal("pinned or custom source label replaced", sheet.Metadata)
		}
	}
	// Even a pinned label that resembles another code remains exact pinned text.
	pinned := Record{"weather": "hot", "form_code": "builtin_construction_daily_report", "fields": []Record{{"field_key": "weather", "options_json": []Record{{"value": "hot", "label": "rain"}}}}}
	sheet, err := nativeBMGSDailySheet(BMGSSourceRef{Kind: "workflow_form", ID: "pinned-form"}, pinned)
	if err != nil || sheet.Metadata["weather"] != "rain" {
		t.Fatal("pinned label was interpreted as a fallback code", sheet.Metadata, err)
	}
	if strings.Contains(string(data), "Chưa ghi thời tiết") {
		t.Fatal("missing display label leaked into the stored migration contract")
	}
}
