package core

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"codeberg.org/go-pdf/fpdf"
)

func TestWorkReportPDFRecordedProgressAuthority(t *testing.T) {
	cases := []struct {
		name             string
		header, captured any
		workflow         bool
		label            string
		bars             int
	}{
		{"workflow_absent", 0, nil, true, "Chưa ghi tiến độ", 0},
		{"workflow_omitted", 0, nil, true, "Chưa ghi tiến độ", 0},
		{"workflow_header_conflict", 88, "", true, "Chưa ghi tiến độ", 0},
		{"workflow_zero", 88, 0, true, "0%", 1},
		{"workflow_decimal", 0, 42.5, true, "42.5%", 2},
		{"workflow_numeric_string", 0, "35", true, "35%", 2},
		{"workflow_json_number", 0, json.Number("25.5"), true, "25.5%", 2},
		{"native_daily_form", 88, 37.5, true, "37.5%", 2},
		{"workflow_legacy_wrapper", 88, Record{"value": "0"}, true, "0%", 1},
		{"workflow_nested_wrapper", 88, wrappedProgressTestValue("42.5", 2), true, "42.5%", 2},
		{"workflow_wrapper_boundary", 88, wrappedProgressTestValue(0, 8), true, "0%", 1},
		{"workflow_wrapper_overflow", 88, wrappedProgressTestValue(0, 9), true, "Chưa ghi tiến độ", 0},
		{"workflow_invalid_text", 0, "N/A", true, "Chưa ghi tiến độ", 0},
		{"workflow_invalid_hex", 0, "0x10", true, "Chưa ghi tiến độ", 0},
		{"workflow_invalid_bool", 0, true, true, "Chưa ghi tiến độ", 0},
		{"workflow_invalid_range", 0, 101, true, "Chưa ghi tiến độ", 0},
		{"workflow_invalid_negative", 0, -1, true, "Chưa ghi tiến độ", 0},
		{"workflow_blank", 88, " ", true, "Chưa ghi tiến độ", 0},
		{"workflow_null_wrapper", 88, Record{"value": nil}, true, "Chưa ghi tiến độ", 0},
		{"workflow_maximum", 0, 100, true, "100%", 2},
		{"manual_zero", 0, 88, false, "0%", 1},
		{"manual_header", 65, 88, false, "65%", 2},
		{"manual_numeric_string", "65", 88, false, "65%", 2},
		{"manual_missing", nil, 88, false, "Chưa ghi tiến độ", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			revision := workReportPDFFixture()
			snapshot := recordFromAny(revision["snapshot_json"])
			snapshot["progress_percent"] = tc.header
			snapshot["structured_json"] = Record{"progress_percent": tc.captured}
			if tc.name == "workflow_omitted" {
				delete(recordFromAny(snapshot["structured_json"]), "progress_percent")
			}
			if tc.workflow {
				snapshot["workflow_form_instance_id"] = "saved-form"
			}
			if tc.name == "native_daily_form" {
				delete(snapshot, "workflow_form_instance_id")
				snapshot["form_code"] = "builtin_construction_daily_report"
			}
			before, _ := json.Marshal(revision)
			result, err := generateWorkReportPDF(revision)
			if err != nil {
				t.Fatal(err)
			}
			if !weatherPDFHasText(result.Data, tc.label) {
				t.Fatalf("PDF lacks progress label %q", tc.label)
			}
			pdf := fpdf.New("P", "mm", "A4", "")
			pdf.SetMargins(workReportPDFMargin, 17, workReportPDFMargin)
			pdf.SetAutoPageBreak(false, 25)
			pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "", pipelinePDFRegularFont)
			pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "B", pipelinePDFBoldFont)
			pdf.AddPage()
			workReportPDFRenderer{pdf: pdf}.summary(snapshot)
			var summary bytes.Buffer
			if err := pdf.Output(&summary); err != nil {
				t.Fatal(err)
			}
			bars := 0
			for _, stream := range regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`).FindAllSubmatch(summary.Bytes(), -1) {
				reader, err := zlib.NewReader(bytes.NewReader(stream[1]))
				if err != nil {
					continue
				}
				decoded, err := io.ReadAll(reader)
				reader.Close()
				if err != nil {
					t.Fatal(err)
				}
				bars += bytes.Count(decoded, []byte(" re f"))
			}
			if bars != tc.bars {
				t.Fatalf("progress rectangles=%d want=%d", bars, tc.bars)
			}
			after, _ := json.Marshal(revision)
			if !bytes.Equal(before, after) || sha256Hex(before) != sha256Hex(after) {
				t.Fatal("PDF changed immutable snapshot content or hash")
			}
			if outputDir := os.Getenv("WORK_REPORT_PDF_ARTIFACT_DIR"); outputDir != "" {
				if err := os.MkdirAll(outputDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(outputDir, "progress-"+tc.name+".pdf"), result.Data, 0644); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func wrappedProgressTestValue(value any, depth int) any {
	for i := 0; i < depth; i++ {
		value = Record{"value": value}
	}
	return value
}
