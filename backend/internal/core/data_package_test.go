package core

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNormalizeDataPackageImportSource(t *testing.T) {
	format, version, err := normalizeDataPackageImportSource("json", "1", "", "package.json")
	if err != nil || format != "json" || version != "1" {
		t.Fatalf("unexpected normalized source format=%q version=%q err=%v", format, version, err)
	}
	if _, _, err := normalizeDataPackageImportSource("json", "1", "", "package.xlsx"); err == nil {
		t.Fatal("JSON source accepted an XLSX filename")
	}
	if _, _, err := normalizeDataPackageImportSource("yaml", "1", "", "package.yaml"); err == nil {
		t.Fatal("unsupported source format was accepted")
	}
	if _, _, err := normalizeDataPackageImportSource("json", "2", "", "package.json"); err == nil {
		t.Fatal("unsupported package version was accepted")
	}
	if _, _, err := normalizeDataPackageImportSource("json", "", "1", "package.json"); err == nil {
		t.Fatal("JSON source without package_version was accepted")
	}
	if _, _, err := normalizeDataPackageImportSource("xlsx", "1", "2", "package.xlsx"); err == nil {
		t.Fatal("mismatched workbook and package versions were accepted")
	}
}

func TestDecodeStrictJSONRejectsAmbiguousOrUnknownInput(t *testing.T) {
	target := struct {
		Format string `json:"format"`
	}{}
	for _, body := range []string{
		`{"format":"json","format":"xlsx"}`,
		`{"format":"json","unknown":true}`,
		`{"format":"json","constructor":{}}`,
		`{"format":"json"} {}`,
	} {
		if err := decodeStrictJSON([]byte(body), &target); err == nil {
			t.Fatalf("strict decoder accepted %s", body)
		}
	}
	if err := decodeStrictJSON([]byte(`{"format":"json"}`), &target); err != nil || target.Format != "json" {
		t.Fatalf("strict decoder rejected valid input target=%#v err=%v", target, err)
	}
}

func TestValidateDataPackageRecordKeysRejectsUnknownNestedFields(t *testing.T) {
	if err := validateDataPackageRecordKeys("pipelines", []Record{{"pipeline_key": "pipeline:test", "unexpected": true}}, "pipeline_key"); err == nil {
		t.Fatal("unknown nested record field was accepted")
	}
	if err := validateDataPackageRecordKeys("pipelines", []Record{{"pipeline_key": "pipeline:test"}}, "pipeline_key"); err != nil {
		t.Fatalf("valid nested record was rejected: %v", err)
	}
}

func TestGenerateProjectJSONPackageUsesPortableSchemaAndExactStrings(t *testing.T) {
	catalog := projectExcelCatalog{
		Readiness:            Record{"ready": true},
		ReferenceFingerprint: strings.Repeat("a", 64),
		Pipelines:            []Record{{"pipeline_ref": "family@r1#hash", "ready": true}},
		Forms:                []Record{},
		Fields:               []Record{},
		Partners:             []Record{},
	}
	result, err := generateProjectJSONPackage(catalog, []Record{{
		"project_code": "CT-001", "project_name": "Công trình 1", "estimated_budget_vnd": int64(9_000_000_000_000_001),
		"pipeline_ref": "family@r1#hash", "planned_start_date": nil,
	}}, time.Date(2026, 8, 23, 8, 0, 0, 0, time.UTC), true)
	if err != nil {
		t.Fatal(err)
	}
	if result.ContentType != exportJSONContentType || !strings.HasSuffix(result.Filename, ".json") || strings.Contains(string(result.Data), "<nil>") {
		t.Fatalf("unexpected JSON export filename=%q content_type=%q data=%s", result.Filename, result.ContentType, result.Data)
	}
	var packageData struct {
		Schema        string   `json:"schema"`
		SchemaVersion int      `json:"schema_version"`
		Projects      []Record `json:"projects"`
	}
	if err := json.Unmarshal(result.Data, &packageData); err != nil {
		t.Fatal(err)
	}
	if packageData.Schema != "consflow.project-package" || packageData.SchemaVersion != 1 || exportCellText(packageData.Projects[0]["estimated_budget_vnd"]) != "9000000000000001" {
		t.Fatalf("unexpected project package: %#v", packageData)
	}
}

func TestProjectExcelVNDCellRemovesOnlyZeroDatabaseScale(t *testing.T) {
	if got := projectExcelVNDCell("1500000000.00"); got != "1500000000" {
		t.Fatalf("unexpected whole VND value %q", got)
	}
	if got := projectExcelVNDCell("1500000000.25"); got != "1500000000.25" {
		t.Fatalf("fractional persisted value was silently changed: %q", got)
	}
}

func TestGenerateWorkflowDefinitionJSONPackageUsesNestedSchema(t *testing.T) {
	packageData := workflowDefinitionPackage{Scope: "bundle", Pipelines: []Record{{
		"pipeline_key": "pipeline:test", "name": "Test", "source_family_id": "family", "source_revision_no": 1,
		"nodes": []Record{{"node_key": "start", "steps": []Record{}}}, "edges": []Record{},
	}}, Forms: []Record{{
		"form_key": "form:test", "template_code": "test", "name": "Test", "fields": []Record{},
	}}}
	catalog := workflowDefinitionCatalog{Fingerprint: strings.Repeat("b", 64)}
	result, err := generateWorkflowDefinitionJSONPackage(packageData, catalog, time.Date(2026, 8, 23, 8, 0, 0, 0, time.UTC), "xuat")
	if err != nil {
		t.Fatal(err)
	}
	if result.ContentType != exportJSONContentType || !strings.HasSuffix(result.Filename, ".json") {
		t.Fatalf("unexpected JSON result filename=%q content_type=%q", result.Filename, result.ContentType)
	}
	var envelope struct {
		Schema        string   `json:"schema"`
		SchemaVersion int      `json:"schema_version"`
		Pipelines     []Record `json:"pipelines"`
		Forms         []Record `json:"forms"`
	}
	if err := json.Unmarshal(result.Data, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Schema != "consflow.workflow-definition-package" || envelope.SchemaVersion != 1 || len(envelope.Pipelines) != 1 || len(envelope.Forms) != 1 {
		t.Fatalf("unexpected workflow definition package: %#v", envelope)
	}
}
