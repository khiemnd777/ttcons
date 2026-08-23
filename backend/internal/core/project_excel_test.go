package core

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestGenerateWorkbookXLSXIncludesAllSheetsAndEscapesValues(t *testing.T) {
	data, err := generateWorkbookXLSX("Công trình & Reference", []xlsxWorkbookSheet{
		{Name: "Cong_trinh", Columns: []tabularColumn{{Key: "name", Header: "project_name"}}, Rows: []Record{{"name": "A&B <Test>"}}},
		{Name: "Ref_Pipeline", Columns: []tabularColumn{{Key: "ref", Header: "pipeline_ref"}}, Rows: []Record{{"ref": "template@r1#hash"}}},
	}, time.Date(2026, 8, 22, 8, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	files := unzipXLSXFiles(t, data)
	for _, name := range []string{"[Content_Types].xml", "xl/workbook.xml", "xl/worksheets/sheet1.xml", "xl/worksheets/sheet2.xml"} {
		if _, ok := files[name]; !ok {
			t.Fatalf("xlsx missing %s", name)
		}
	}
	if workbook := files["xl/workbook.xml"]; !strings.Contains(workbook, `name="Cong_trinh"`) || !strings.Contains(workbook, `name="Ref_Pipeline"`) {
		t.Fatalf("unexpected workbook sheets: %s", workbook)
	}
	if sheet := files["xl/worksheets/sheet1.xml"]; !strings.Contains(sheet, "A&amp;B &lt;Test&gt;") {
		t.Fatalf("worksheet did not escape cell text: %s", sheet)
	}
}

func TestNormalizeProjectExcelExportRequestRejectsInvalidScopeAndSelection(t *testing.T) {
	if _, err := normalizeProjectExcelExportRequest(ProjectExcelExportRequest{Scope: "all"}); err == nil {
		t.Fatal("invalid export scope was accepted")
	}
	if _, err := normalizeProjectExcelExportRequest(ProjectExcelExportRequest{Scope: "selected"}); err == nil {
		t.Fatal("empty selected export was accepted")
	}
	if _, err := normalizeProjectExcelExportRequest(ProjectExcelExportRequest{Scope: "selected", ProjectIDs: []string{"not-a-uuid"}}); err == nil {
		t.Fatal("invalid project id was accepted")
	}
}

func TestProjectDataImportExportReadinessPersistenceAndIdempotency(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "project-excel-test-secret")
	suffix := strings.ReplaceAll(newID(), "-", "")[:10]
	actorID := newID()
	actorEmail := "project-excel-actor-" + suffix + "@test.local"
	projectCode := "XLSX-" + suffix
	idempotencyKey := newID()

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM project_excel_imports WHERE actor_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE actor_id=$1 OR target_id IN (SELECT id FROM projects WHERE code=$2)`, actorID, projectCode)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE code=$1`, projectCode)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})

	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:test','Project Excel Actor','active')`, actorID, actorEmail)
	mustExec(t, db, `
		INSERT INTO user_roles (user_id,role_id)
		SELECT $1,id FROM roles WHERE name='admin'
	`, actorID)

	catalog, err := repo.ProjectExcelCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(catalog.Readiness["ready"]) || len(catalog.PipelineByReference) == 0 {
		t.Fatalf("database reference catalog is not ready: %#v", catalog.Readiness)
	}
	var pipelineRef string
	for reference := range catalog.PipelineByReference {
		pipelineRef = reference
		break
	}
	request := ProjectExcelImportPreviewRequest{
		PackageVersion:  dataPackageVersionV1,
		SourceFormat:    dataPackageFormatJSON,
		SourceFilename:  "du-an-moi.json",
		SourceSizeBytes: 4096,
		Rows: []ProjectExcelImportRowRequest{{
			RowNumber: 2, ProjectCode: projectCode, ProjectName: "Công trình import " + suffix,
			ClientNameUnregistered: "Chủ đầu tư chưa có danh mục", Address: "Thủ Thiêm",
			PlannedStartDate: "2026-09-01", PlannedEndDate: "2026-12-31",
			EstimatedBudgetVND: "1500000000", PipelineRef: pipelineRef,
		}},
	}
	preview, err := service.PreviewProjectExcelImport(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if number(preview["valid_count"]) != 1 || number(preview["invalid_count"]) != 0 || fmt.Sprint(preview["preview_hash"]) == "" {
		t.Fatalf("unexpected import preview: %#v", preview)
	}

	execute := ProjectExcelImportExecuteRequest{
		ProjectExcelImportPreviewRequest: request,
		PreviewHash:                      fmt.Sprint(preview["preview_hash"]),
		IdempotencyKey:                   idempotencyKey,
	}
	result, err := service.ExecuteProjectExcelImport(ctx, actorID, actorEmail, execute)
	if err != nil {
		t.Fatal(err)
	}
	if number(result["created_count"]) != 1 || number(result["pipeline_started_count"]) != 0 || boolValue(result["idempotent_replay"]) {
		t.Fatalf("unexpected execute result: %#v", result)
	}
	items, ok := result["items"].([]Record)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected imported items: %#v", result["items"])
	}
	projectID := cleanID(items[0]["project_id"])
	var status, selectedTemplateID string
	var currentRunID *string
	var estimatedBudget, contractAmount int64
	if err := db.QueryRowContext(ctx, `
		SELECT status,pipeline_template_id::text,current_pipeline_run_id::text,estimated_budget::bigint,contract_amount::bigint
		FROM projects WHERE id=$1
	`, projectID).Scan(&status, &selectedTemplateID, &currentRunID, &estimatedBudget, &contractAmount); err != nil {
		t.Fatal(err)
	}
	if status != "draft" || selectedTemplateID != cleanID(catalog.PipelineByReference[pipelineRef]["template_id"]) || currentRunID != nil || estimatedBudget != 1_500_000_000 || contractAmount != 0 {
		t.Fatalf("unexpected imported project status=%q template=%q run=%v budget=%d contract=%d", status, selectedTemplateID, currentRunID, estimatedBudget, contractAmount)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM pipeline_runs WHERE project_id=$1`, 0, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE project_id=$1`, 0, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_instances WHERE project_id=$1`, 0, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM project_members WHERE project_id=$1`, 0, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM project_excel_imports WHERE actor_id=$1 AND row_count=1 AND created_count=1 AND source_format='json'`, 1, actorID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE actor_id=$1 AND event_type IN ('project_imported','project_data_import_completed')`, 2, actorID)

	replay, err := service.ExecuteProjectExcelImport(ctx, actorID, actorEmail, execute)
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(replay["idempotent_replay"]) || number(replay["created_count"]) != 1 {
		t.Fatalf("unexpected idempotent replay: %#v", replay)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM projects WHERE code=$1`, 1, projectCode)

	duplicate, err := service.PreviewProjectExcelImport(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if number(duplicate["invalid_count"]) != 1 || fmt.Sprint(duplicate["preview_hash"]) != "" {
		t.Fatalf("existing project code unexpectedly passed preview: %#v", duplicate)
	}

	export, err := service.ExportProjectsExcel(ctx, actorID, actorEmail, ProjectExcelExportRequest{
		Scope: "selected", ProjectIDs: []string{projectID}, Sort: "created_at", Direction: "desc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(export.Data) < 4 || !bytes.Equal(export.Data[:2], []byte("PK")) || !strings.HasSuffix(export.Filename, ".xlsx") {
		t.Fatalf("unexpected export result filename=%q bytes=%d", export.Filename, len(export.Data))
	}
	files := unzipXLSXFiles(t, export.Data)
	workbook := files["xl/workbook.xml"]
	for _, sheet := range []string{"Huong_dan", "Cong_trinh", "Thong_tin_xuat", "Ref_Pipeline", "Ref_Bieu_mau", "Ref_Fields", "Ref_Chu_dau_tu"} {
		if !strings.Contains(workbook, `name="`+sheet+`"`) {
			t.Fatalf("export workbook missing sheet %s: %s", sheet, workbook)
		}
	}
	if projectSheet := files["xl/worksheets/sheet2.xml"]; !strings.Contains(projectSheet, `t="inlineStr"`) || !strings.Contains(projectSheet, "1500000000") {
		t.Fatalf("project budget must be emitted as an exact integer string: %s", projectSheet)
	}
	jsonExport, err := service.ExportProjectsExcel(ctx, actorID, actorEmail, ProjectExcelExportRequest{
		Format: dataPackageFormatJSON, Scope: "selected", ProjectIDs: []string{projectID}, Sort: "created_at", Direction: "desc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if jsonExport.ContentType != exportJSONContentType || !strings.Contains(string(jsonExport.Data), `"schema": "consflow.project-package"`) || !strings.Contains(string(jsonExport.Data), `"estimated_budget_vnd": "1500000000"`) {
		t.Fatalf("unexpected project JSON export: %s", jsonExport.Data)
	}
}

func TestProjectExcelPermissionMigrationAssignmentsAreFailClosed(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	for _, test := range []struct {
		role       string
		permission string
		want       int
	}{
		{"admin", "project.import", 1}, {"admin", "project.export", 1},
		{"developer", "project.import", 1}, {"developer", "project.export", 1},
		{"pm", "project.import", 0}, {"pm", "project.export", 1},
	} {
		assertCount(t, db, `
			SELECT COUNT(*) FROM role_permissions assignment
			JOIN roles role ON role.id=assignment.role_id
			JOIN permissions permission ON permission.id=assignment.permission_id
			WHERE role.name=$1 AND permission.key=$2
		`, test.want, test.role, test.permission)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO role_permissions (role_id,permission_id)
		SELECT role.id,permission.id FROM roles role CROSS JOIN permissions permission
		WHERE role.name='pm' AND permission.key='project.import'
	`); err == nil {
		t.Fatal("database trigger allowed project.import to be assigned to PM")
	}
}

func unzipXLSXFiles(t *testing.T, data []byte) map[string]string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string]string, len(reader.File))
	for _, entry := range reader.File {
		stream, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, readErr := io.ReadAll(stream)
		closeErr := stream.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		files[entry.Name] = string(content)
	}
	return files
}
