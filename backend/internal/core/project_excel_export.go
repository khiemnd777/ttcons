package core

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func generateProjectExcelWorkbook(catalog projectExcelCatalog, projects []Record, generatedAt time.Time, exported bool) (tabularExportResult, error) {
	missingReferences := 0
	runtimeMismatches := 0
	projectRows := make([]Record, 0, len(projects))
	metadataRows := make([]Record, 0, len(projects))
	for _, project := range projects {
		state := strings.TrimSpace(fmt.Sprint(project["reference_state"]))
		if state == "missing" {
			missingReferences++
		}
		if state == "runtime_mismatch" {
			runtimeMismatches++
		}
		projectRows = append(projectRows, Record{
			"project_code":             exportCellText(project["project_code"]),
			"project_name":             exportCellText(project["project_name"]),
			"client_partner_code":      exportCellText(project["client_partner_code"]),
			"client_name_unregistered": exportCellText(project["client_name_unregistered"]),
			"address":                  exportCellText(project["address"]),
			"planned_start_date":       projectExcelDateCell(project["planned_start_date"]),
			"planned_end_date":         projectExcelDateCell(project["planned_end_date"]),
			"estimated_budget_vnd":     projectExcelVNDCell(project["estimated_budget_vnd"]),
			"pipeline_ref":             exportCellText(project["pipeline_ref"]),
		})
		metadataRows = append(metadataRows, Record{
			"project_id": project["project_id"], "project_code": project["project_code"], "status": project["status"],
			"progress_percent": project["progress_percent"], "contract_amount_snapshot": projectExcelVNDCell(project["contract_amount"]),
			"selected_pipeline_ref": project["selected_pipeline_ref"], "selected_pipeline_name": project["selected_template_name"],
			"runtime_pipeline_ref": project["runtime_pipeline_ref"], "runtime_pipeline_name": project["runtime_template_name"],
			"runtime_id": project["runtime_id"], "runtime_engine_version": project["runtime_engine_version"],
			"runtime_definition_hash": project["runtime_template_definition_hash"], "reference_state": state,
			"pm_staff_id": project["pm_staff_id"], "pm_name": project["pm_name"],
			"created_at": project["created_at"], "updated_at": project["updated_at"],
		})
	}

	guideRows := []Record{
		{"muc": "workbook_version", "noi_dung": projectExcelWorkbookVersion},
		{"muc": "generated_at", "noi_dung": generatedAt.UTC().Format(time.RFC3339)},
		{"muc": "pham_vi", "noi_dung": map[bool]string{true: "File xuất công trình", false: "File mẫu import công trình"}[exported]},
		{"muc": "quy_tac_1", "noi_dung": "Chỉ chỉnh sửa và import sheet Cong_trinh; giữ nguyên đúng chín tiêu đề cột."},
		{"muc": "quy_tac_2", "noi_dung": "project_code, project_name và pipeline_ref là bắt buộc. Mỗi lần tối đa 100 công trình, file tối đa 5 MB."},
		{"muc": "quy_tac_3", "noi_dung": "pipeline_ref phải lấy nguyên văn từ Ref_Pipeline và chỉ dòng ready=true mới được dùng."},
		{"muc": "quy_tac_4", "noi_dung": "client_partner_code lấy từ Ref_Chu_dau_tu; nếu bỏ trống thì phải nhập client_name_unregistered."},
		{"muc": "quy_tac_5", "noi_dung": "Import chỉ tạo master Công trình ở trạng thái draft; không khởi chạy Pipeline, không tạo runtime, Form answers, thành viên hoặc dữ liệu nghiệp vụ."},
		{"muc": "quy_tac_6", "noi_dung": "contract_amount_snapshot chỉ đọc. Nguồn chính thức là hợp đồng client đã ký; không nhập giá trị này từ Excel."},
		{"muc": "readiness", "noi_dung": fmt.Sprintf("ready=%t; pipelines=%d; forms=%d; fields=%d", boolValue(catalog.Readiness["ready"]), int(number(catalog.Readiness["ready_pipeline_count"])), int(number(catalog.Readiness["published_form_version_count"])), int(number(catalog.Readiness["field_count"])))},
		{"muc": "reference_fingerprint", "noi_dung": catalog.ReferenceFingerprint},
	}
	if exported {
		guideRows = append(guideRows,
			Record{"muc": "exported_project_count", "noi_dung": strconv.Itoa(len(projects))},
			Record{"muc": "missing_reference_count", "noi_dung": strconv.Itoa(missingReferences)},
			Record{"muc": "runtime_mismatch_count", "noi_dung": strconv.Itoa(runtimeMismatches)},
			Record{"muc": "can_import_again", "noi_dung": "File xuất là dữ liệu tham khảo. Import create-only sẽ từ chối mã công trình đã tồn tại; dòng thiếu pipeline_ref phải được cập nhật trước."},
		)
	}

	pipelineRows := make([]Record, 0, len(catalog.Pipelines))
	for _, source := range catalog.Pipelines {
		pipelineRows = append(pipelineRows, Record{
			"pipeline_ref": source["pipeline_ref"], "template_id": source["template_id"], "template_family_id": source["template_family_id"],
			"name": source["name"], "project_type": source["project_type"], "revision_no": source["revision_no"],
			"engine_version": source["engine_version"], "definition_hash": source["definition_hash"], "node_count": source["node_count"],
			"step_count": source["step_count"], "required_form_count": source["required_form_count"],
			"invalid_form_reference_count": source["invalid_form_reference_count"], "zero_field_reference_count": source["zero_field_reference_count"],
			"ready": source["ready"],
		})
	}
	formRows := make([]Record, 0, len(catalog.Forms))
	for _, source := range catalog.Forms {
		formRows = append(formRows, Record{
			"form_ref": source["form_ref"], "form_template_id": source["form_template_id"], "form_template_code": source["form_template_code"],
			"form_template_name": source["form_template_name"], "form_type": source["form_type"], "form_version_id": source["form_version_id"],
			"version_no": source["version_no"], "status": source["status"], "field_count": source["field_count"],
		})
	}
	fieldRows := make([]Record, 0, len(catalog.Fields))
	for _, source := range catalog.Fields {
		fieldRows = append(fieldRows, Record{
			"form_ref": source["form_ref"], "form_template_code": source["form_template_code"], "form_version_id": source["form_version_id"],
			"version_no": source["version_no"], "field_key": source["field_key"], "label": source["label"], "field_type": source["field_type"],
			"required": source["required"], "options_json": projectExcelJSONText(source["options_json"]),
			"validation_json": projectExcelJSONText(source["validation_json"]), "visibility_condition_json": projectExcelJSONText(source["visibility_condition_json"]),
			"order_index": source["order_index"],
		})
	}
	partnerRows := make([]Record, 0, len(catalog.Partners))
	for _, source := range catalog.Partners {
		partnerRows = append(partnerRows, Record{
			"partner_code": source["partner_code"], "name": source["name"], "tax_code": source["tax_code"], "updated_at": source["updated_at"],
		})
	}

	sheets := []xlsxWorkbookSheet{
		{Name: "Huong_dan", Columns: []tabularColumn{{"muc", "muc", false}, {"noi_dung", "noi_dung", false}}, Rows: guideRows},
		{Name: "Cong_trinh", Columns: projectExcelImportColumns(), Rows: projectRows},
		{Name: "Thong_tin_xuat", Columns: []tabularColumn{
			{"project_id", "project_id", false}, {"project_code", "project_code", false}, {"status", "status", false}, {"progress_percent", "progress_percent", true},
			{"contract_amount_snapshot", "contract_amount_snapshot", false}, {"selected_pipeline_ref", "selected_pipeline_ref", false},
			{"selected_pipeline_name", "selected_pipeline_name", false}, {"runtime_pipeline_ref", "runtime_pipeline_ref", false},
			{"runtime_pipeline_name", "runtime_pipeline_name", false}, {"runtime_id", "runtime_id", false}, {"runtime_engine_version", "runtime_engine_version", true},
			{"runtime_definition_hash", "runtime_definition_hash", false}, {"reference_state", "reference_state", false},
			{"pm_staff_id", "pm_staff_id", false}, {"pm_name", "pm_name", false}, {"created_at", "created_at", false}, {"updated_at", "updated_at", false},
		}, Rows: metadataRows},
		{Name: "Ref_Pipeline", Columns: []tabularColumn{
			{"pipeline_ref", "pipeline_ref", false}, {"template_id", "template_id", false}, {"template_family_id", "template_family_id", false},
			{"name", "name", false}, {"project_type", "project_type", false}, {"revision_no", "revision_no", true}, {"engine_version", "engine_version", true},
			{"definition_hash", "definition_hash", false}, {"node_count", "node_count", true}, {"step_count", "step_count", true},
			{"required_form_count", "required_form_count", true}, {"invalid_form_reference_count", "invalid_form_reference_count", true},
			{"zero_field_reference_count", "zero_field_reference_count", true}, {"ready", "ready", false},
		}, Rows: pipelineRows},
		{Name: "Ref_Bieu_mau", Columns: []tabularColumn{
			{"form_ref", "form_ref", false}, {"form_template_id", "form_template_id", false}, {"form_template_code", "form_template_code", false},
			{"form_template_name", "form_template_name", false}, {"form_type", "form_type", false}, {"form_version_id", "form_version_id", false},
			{"version_no", "version_no", true}, {"status", "status", false}, {"field_count", "field_count", true},
		}, Rows: formRows},
		{Name: "Ref_Fields", Columns: []tabularColumn{
			{"form_ref", "form_ref", false}, {"form_template_code", "form_template_code", false}, {"form_version_id", "form_version_id", false},
			{"version_no", "version_no", true}, {"field_key", "field_key", false}, {"label", "label", false}, {"field_type", "field_type", false},
			{"required", "required", false}, {"options_json", "options_json", false}, {"validation_json", "validation_json", false},
			{"visibility_condition_json", "visibility_condition_json", false}, {"order_index", "order_index", true},
		}, Rows: fieldRows},
		{Name: "Ref_Chu_dau_tu", Columns: []tabularColumn{
			{"partner_code", "partner_code", false}, {"name", "name", false}, {"tax_code", "tax_code", false}, {"updated_at", "updated_at", false},
		}, Rows: partnerRows},
	}
	data, err := generateWorkbookXLSX("Import/Export Công trình", sheets, generatedAt)
	if err != nil {
		return tabularExportResult{}, err
	}
	filename := "mau-import-cong-trinh.xlsx"
	if exported {
		filename = "cong-trinh-" + generatedAt.In(time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)).Format("20060102-150405") + ".xlsx"
	}
	return tabularExportResult{Data: data, Filename: filename, ContentType: exportXLSXContentType}, nil
}

func generateProjectJSONPackage(catalog projectExcelCatalog, projects []Record, generatedAt time.Time, exported bool) (tabularExportResult, error) {
	projectRows := make([]Record, 0, len(projects))
	snapshots := make([]Record, 0, len(projects))
	for _, project := range projects {
		projectRows = append(projectRows, Record{
			"project_code":             fmt.Sprint(project["project_code"]),
			"project_name":             fmt.Sprint(project["project_name"]),
			"client_partner_code":      fmt.Sprint(project["client_partner_code"]),
			"client_name_unregistered": fmt.Sprint(project["client_name_unregistered"]),
			"address":                  fmt.Sprint(project["address"]),
			"planned_start_date":       projectExcelDateCell(project["planned_start_date"]),
			"planned_end_date":         projectExcelDateCell(project["planned_end_date"]),
			"estimated_budget_vnd":     projectExcelVNDCell(project["estimated_budget_vnd"]),
			"pipeline_ref":             fmt.Sprint(project["pipeline_ref"]),
		})
		snapshots = append(snapshots, Record{
			"project_id": project["project_id"], "project_code": project["project_code"], "status": project["status"],
			"progress_percent": project["progress_percent"], "contract_amount_snapshot": projectExcelVNDCell(project["contract_amount"]),
			"selected_pipeline_ref": project["selected_pipeline_ref"], "selected_pipeline_name": project["selected_template_name"],
			"runtime_pipeline_ref": project["runtime_pipeline_ref"], "runtime_pipeline_name": project["runtime_template_name"],
			"runtime_id": project["runtime_id"], "runtime_engine_version": project["runtime_engine_version"],
			"runtime_definition_hash": project["runtime_template_definition_hash"], "reference_state": project["reference_state"],
			"pm_staff_id": project["pm_staff_id"], "pm_name": project["pm_name"],
			"created_at": project["created_at"], "updated_at": project["updated_at"],
		})
	}

	pipelines := make([]Record, 0, len(catalog.Pipelines))
	for _, source := range catalog.Pipelines {
		pipelines = append(pipelines, Record{
			"pipeline_ref": source["pipeline_ref"], "template_id": source["template_id"], "template_family_id": source["template_family_id"],
			"name": source["name"], "project_type": source["project_type"], "revision_no": source["revision_no"],
			"engine_version": source["engine_version"], "definition_hash": source["definition_hash"], "node_count": source["node_count"],
			"step_count": source["step_count"], "required_form_count": source["required_form_count"],
			"invalid_form_reference_count": source["invalid_form_reference_count"], "zero_field_reference_count": source["zero_field_reference_count"],
			"ready": source["ready"],
		})
	}
	forms := make([]Record, 0, len(catalog.Forms))
	for _, source := range catalog.Forms {
		forms = append(forms, Record{
			"form_ref": source["form_ref"], "form_template_id": source["form_template_id"], "form_template_code": source["form_template_code"],
			"form_template_name": source["form_template_name"], "form_type": source["form_type"], "form_version_id": source["form_version_id"],
			"version_no": source["version_no"], "status": source["status"], "field_count": source["field_count"],
		})
	}
	fields := make([]Record, 0, len(catalog.Fields))
	for _, source := range catalog.Fields {
		fields = append(fields, Record{
			"form_ref": source["form_ref"], "form_template_code": source["form_template_code"], "form_version_id": source["form_version_id"],
			"version_no": source["version_no"], "field_key": source["field_key"], "label": source["label"], "field_type": source["field_type"],
			"required": source["required"], "options_json": defaultWorkflowDefinitionJSON(source["options_json"], []any{}),
			"validation_json":           defaultWorkflowDefinitionJSON(source["validation_json"], Record{}),
			"visibility_condition_json": defaultWorkflowDefinitionJSON(source["visibility_condition_json"], Record{}), "order_index": source["order_index"],
		})
	}
	clients := make([]Record, 0, len(catalog.Partners))
	for _, source := range catalog.Partners {
		clients = append(clients, Record{
			"partner_code": source["partner_code"], "name": source["name"], "tax_code": source["tax_code"], "updated_at": source["updated_at"],
		})
	}
	mode := "template"
	if exported {
		mode = "export"
	}
	data, err := marshalDataPackageJSON(struct {
		Schema               string   `json:"schema"`
		SchemaVersion        int      `json:"schema_version"`
		Mode                 string   `json:"mode"`
		GeneratedAt          string   `json:"generated_at"`
		ReferenceFingerprint string   `json:"reference_fingerprint"`
		Instructions         []string `json:"instructions"`
		Readiness            Record   `json:"readiness"`
		Projects             []Record `json:"projects"`
		ProjectSnapshots     []Record `json:"project_snapshots"`
		References           struct {
			Pipelines []Record `json:"pipelines"`
			Forms     []Record `json:"forms"`
			Fields    []Record `json:"fields"`
			Clients   []Record `json:"clients"`
		} `json:"references"`
	}{
		Schema: "consflow.project-package", SchemaVersion: 1, Mode: mode, GeneratedAt: generatedAt.UTC().Format(time.RFC3339),
		ReferenceFingerprint: catalog.ReferenceFingerprint,
		Instructions: []string{
			"Chỉ chỉnh mảng projects; project_snapshots và references là dữ liệu chỉ đọc.",
			"Import chỉ tạo master Công trình ở trạng thái draft; không khởi chạy Pipeline hoặc tạo dữ liệu runtime.",
		},
		Readiness: catalog.Readiness, Projects: projectRows, ProjectSnapshots: snapshots,
		References: struct {
			Pipelines []Record `json:"pipelines"`
			Forms     []Record `json:"forms"`
			Fields    []Record `json:"fields"`
			Clients   []Record `json:"clients"`
		}{Pipelines: pipelines, Forms: forms, Fields: fields, Clients: clients},
	})
	if err != nil {
		return tabularExportResult{}, err
	}
	filename := "mau-import-cong-trinh.json"
	if exported {
		filename = "cong-trinh-" + generatedAt.In(time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)).Format("20060102-150405") + ".json"
	}
	return tabularExportResult{Data: data, Filename: filename, ContentType: exportJSONContentType}, nil
}

func projectExcelImportColumns() []tabularColumn {
	return []tabularColumn{
		{"project_code", "project_code", false}, {"project_name", "project_name", false},
		{"client_partner_code", "client_partner_code", false}, {"client_name_unregistered", "client_name_unregistered", false},
		{"address", "address", false}, {"planned_start_date", "planned_start_date", false}, {"planned_end_date", "planned_end_date", false},
		{"estimated_budget_vnd", "estimated_budget_vnd", false}, {"pipeline_ref", "pipeline_ref", false},
	}
}

func projectExcelDateCell(value any) string {
	text := exportCellText(value)
	if len(text) >= 10 {
		return text[:10]
	}
	return text
}

func projectExcelVNDCell(value any) string {
	text := exportCellText(value)
	separator := strings.IndexByte(text, '.')
	if separator < 0 {
		return text
	}
	for _, digit := range text[separator+1:] {
		if digit != '0' {
			return text
		}
	}
	return text[:separator]
}

func projectExcelJSONText(value any) string {
	if value == nil {
		return ""
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return string(encoded)
}
