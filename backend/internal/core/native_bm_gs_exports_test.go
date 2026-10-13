package core

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
)

func nativeBMGSHTTP(t *testing.T, s *Service, path, actor string, status int) {
	t.Helper()
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", actor)
		c.Locals("full_name", "Synthetic export actor")
		return c.Next()
	})
	registerBMGSExportRoutes(app, NewHandler(s))
	response, err := app.Test(httptest.NewRequest("GET", path, nil), 10000)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != status {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("%s status=%d expected=%d body=%s", path, response.StatusCode, status, body)
	}
	if status == 200 {
		raw, _ := io.ReadAll(response.Body)
		book, err := excelize.OpenReader(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		book.Close()
	}
}

func nativeBMGSArtifact(t *testing.T, name string, result tabularExportResult) {
	t.Helper()
	dir := os.Getenv("CONSFLOW_BMGS_ARTIFACT_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".xlsx"), result.Data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestNativeBMGSOwnerQueriesPinnedFormsAndScope(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_bmgs_owners_", []string{"workflow_form_fields", "handover_checklist_items"})
	s := NewService(NewRepository(db), "test-secret")
	s.ConfigureUploads(t.TempDir(), 12*1024*1024)
	ctx := context.Background()
	denied := setupProjectScopeFixture(t, db, []string{"operation.view"})
	cases := []struct{ code, purpose, answers string }{
		{"client_site_measurement_record", "", `{"dimension_items":[{"area":"Sảnh nguồn","dimension":"3 x 4","unit":"m"}]}`},
		{"client_survey_work_package", "risks", `{"risk_items":[{"risk":"Mặt bằng nguồn","severity":"high","mitigation":"Rào khu vực"}]}`},
		{"client_site_risk_assessment", "", `{"risk_items":[{"risk":"Rủi ro nguồn","severity":"high"}]}`},
		{"builtin_subcontractor_checklist", "", `{"checklist_items":[{"title":"Hạng mục nguồn","actual_quantity":3,"design_quantity":4}]}`},
		{"client_subcontractor_acceptance_package", "", `{"checklist_items":[{"title":"Nghiệm thu nguồn","actual_quantity":5}]}`},
		{"builtin_construction_daily_report", "next_plan", `{"next_plan_items":[{"title":"Kế hoạch nguồn","receiver":"Đội nguồn","progress_percent":0}]}`},
	}
	for _, c := range cases {
		t.Run(c.code+c.purpose, func(t *testing.T) {
			f := setupWorkItemFlowFixture(t, db, false)
			formID := newID()
			mustExec(t, db, `INSERT INTO workflow_form_instances(id,project_id,work_item_id,form_template_id,form_version_id,form_type,title,answers_snapshot_json,created_by) SELECT $1,$2,$3,template.id,version.id,template.form_type,'Phiếu nguồn',$4::jsonb,$5 FROM workflow_form_templates template JOIN LATERAL(SELECT id FROM workflow_form_versions WHERE template_id=template.id AND status='published' ORDER BY version_no DESC LIMIT 1)version ON true WHERE template.code=$6`, formID, f.projectID, f.itemID, c.answers, f.workerID, c.code)
			result, err := s.ExportNativeWorkflowForm(ctx, formID, c.purpose, f.workerID, "Exporter")
			if err != nil {
				t.Fatal(err)
			}
			book, err := excelize.OpenReader(bytes.NewReader(result.Data))
			if err != nil {
				t.Fatal(err)
			}
			book.Close()
			artifact := map[string]string{"client_site_measurement_record": "BM-GS-01", "builtin_subcontractor_checklist": "BM-GS-03", "client_site_risk_assessment": "BM-GS-05", "builtin_construction_daily_report": "BM-GS-06"}[c.code]
			if artifact != "" {
				nativeBMGSArtifact(t, artifact, result)
			}
			path := "/workflow-form-instances/" + formID + "/export.xlsx"
			if c.purpose == "next_plan" {
				path = "/workflow-form-instances/" + formID + "/next-plan/export.xlsx"
			}
			if c.purpose == "risks" {
				path = "/workflow-form-instances/" + formID + "/risks/export.xlsx"
			}
			nativeBMGSHTTP(t, s, path, f.workerID, 200)
			nativeBMGSHTTP(t, s, path, denied.userID, 403)
			if _, err = s.ExportNativeWorkflowForm(ctx, formID, c.purpose, denied.userID, "Denied"); err == nil {
				t.Fatal("wrong project actor exported pinned form")
			}
			row, err := s.nativeWorkflowExportSource(ctx, formID, f.workerID)
			if err != nil {
				t.Fatal(err)
			}
			if cleanText(row["form_code"]) != c.code {
				t.Fatalf("wrong pinned identity: %v", row["form_code"])
			}
		})
	}
	f := setupWorkItemFlowFixture(t, db, false)
	minutesID := newID()
	mustExec(t, db, `INSERT INTO workflow_form_instances(id,project_id,work_item_id,form_template_id,form_version_id,form_type,title,created_by) SELECT $1,$2,$3,t.id,v.id,t.form_type,'Minutes',$4 FROM workflow_form_templates t JOIN LATERAL(SELECT id FROM workflow_form_versions WHERE template_id=t.id ORDER BY version_no DESC LIMIT 1)v ON true WHERE t.code='client_site_survey_minutes'`, minutesID, f.projectID, f.itemID, f.workerID)
	if _, err := s.ExportNativeWorkflowForm(ctx, minutesID, "", f.workerID, "Worker"); err == nil {
		t.Fatal("minutes falsely mapped to dimension survey")
	}
	actionID := newID()
	mustExec(t, db, `INSERT INTO construction_action_items(id,project_id,work_item_id,title,created_by,fault_code,location)VALUES($1,$2,$3,'Lỗi nguồn',$4,'L-01','Sảnh')`, actionID, f.projectID, f.itemID, f.workerID)
	if result, err := s.ExportNativeConstructionAction(ctx, actionID, f.workerID, "Worker"); err != nil {
		t.Fatal(err)
	} else {
		nativeBMGSArtifact(t, "BM-GS-04", result)
	}
	nativeBMGSHTTP(t, s, "/construction-actions/"+actionID+"/export.xlsx", f.workerID, 200)
	nativeBMGSHTTP(t, s, "/construction-actions/"+actionID+"/export.xlsx", denied.userID, 403)
	if _, err := s.ExportNativeConstructionAction(ctx, actionID, denied.userID, "Denied"); err == nil {
		t.Fatal("wrong project action exported")
	}
	privateItem, checkID, legacyAction := newID(), newID(), newID()
	mustExec(t, db, `INSERT INTO work_items(id,project_id,source_type,source_id,title,work_type,status,role_ids,created_by)VALUES($1,$2,'issue',$3,'Restricted check','task','ready','["Restricted team"]',$4)`, privateItem, f.projectID, newID(), f.pmID)
	mustExec(t, db, `INSERT INTO construction_checks(id,project_id,work_item_id,title)VALUES($1,$2,$3,'Restricted check')`, checkID, f.projectID, privateItem)
	mustExec(t, db, `INSERT INTO construction_action_items(id,project_id,source_check_id,title)VALUES($1,$2,$3,'Legacy no-photo restricted action')`, legacyAction, f.projectID, checkID)
	if _, err := s.ExportNativeConstructionAction(ctx, legacyAction, f.workerID, "Worker"); err == nil {
		t.Fatal("legacy action bypassed check work-item scope")
	}
	visibleActions, err := s.repo.nativeConstructionActions(ctx, f.projectID, f.workerID, NativeConstructionActionFilter{Queue: "all"})
	if err != nil || len(visibleActions) != 1 {
		t.Fatalf("action list revealed private check: rows=%d err=%v", len(visibleActions), err)
	}
	handoverOwner := setupProjectScopeFixture(t, db, []string{"handover.view"})
	handoverID := newID()
	mustExec(t, db, `INSERT INTO handover_records(id,project_id,handover_code) VALUES($1,$2,'HO-NATIVE')`, handoverID, handoverOwner.projectID)
	mustExec(t, db, `INSERT INTO handover_checklist_items(handover_id,project_id,item_code,category,title,status,physical_result,defect) VALUES($1,$2,'native','other','Mối nối nguồn','pending','fail','Chưa hoàn thiện')`, handoverID, handoverOwner.projectID)
	if result, err := s.ExportNativeHandover(ctx, handoverID, handoverOwner.userID, "Owner"); err != nil {
		t.Fatal(err)
	} else {
		nativeBMGSArtifact(t, "BM-GS-08", result)
	}
	nativeBMGSHTTP(t, s, "/handover/"+handoverID+"/export.xlsx", handoverOwner.userID, 200)
	nativeBMGSHTTP(t, s, "/handover/"+handoverID+"/export.xlsx", denied.userID, 403)
	if _, err := s.ExportNativeHandover(ctx, handoverID, denied.userID, "Denied"); err == nil {
		t.Fatal("wrong project handover exported")
	}
}

func TestNativeBMGSLinkedWorkflowEvidenceAndPhotoOwner(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_bmgs_links_", []string{"workflow_form_fields", "evidence_records"})
	s := NewService(NewRepository(db), "test-secret")
	s.ConfigureUploads(t.TempDir(), 12*1024*1024)
	f := setupWorkItemFlowFixture(t, db, false)
	ctx := context.Background()
	var raw bytes.Buffer
	if err := png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	stored, _, err := s.storeGeneratedFile(ctx, "evidence/source-linked.png", raw.Bytes(), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	photoID := newID()
	mustExec(t, db, `INSERT INTO evidence_records(id,project_id,work_item_id,evidence_type,name,status,uploaded_by,storage_provider,storage_key,mime_type,file_size,checksum_sha256,caption)VALUES($1,$2,$3,'photo','linked.png','submitted',$4,$5,$6,'image/png',$7,$8,'Chú thích nguồn')`, photoID, f.projectID, f.itemID, f.workerID, stored.StorageProvider, stored.StorageKey, len(raw.Bytes()), sha256Hex(raw.Bytes()))
	formID := newID()
	answers, _ := json.Marshal(Record{"linked_evidence_id": Record{"value": photoID}, "dimension_items": []Record{{"area": "Sảnh", "dimension": "4 x 5"}}})
	mustExec(t, db, `INSERT INTO workflow_form_instances(id,project_id,work_item_id,form_template_id,form_version_id,form_type,title,answers_snapshot_json,created_by)SELECT $1,$2,$3,t.id,v.id,t.form_type,'Measurement',$4::jsonb,$5 FROM workflow_form_templates t JOIN workflow_form_versions v ON v.template_id=t.id AND v.version_no=1 WHERE t.code='client_site_measurement_record'`, formID, f.projectID, f.itemID, string(answers), f.workerID)
	result, err := s.ExportNativeWorkflowForm(ctx, formID, "", f.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(result.Data), int64(len(result.Data)))
	if err != nil {
		t.Fatal(err)
	}
	media := 0
	var allXML strings.Builder
	for _, file := range archive.File {
		if strings.HasPrefix(file.Name, "xl/media/") {
			media++
		}
		if strings.HasSuffix(file.Name, ".xml") {
			reader, _ := file.Open()
			data, _ := io.ReadAll(reader)
			reader.Close()
			allXML.Write(data)
		}
	}
	if media != 1 {
		t.Fatalf("canonical linked_evidence_id omitted: media=%d", media)
	}
	if strings.Contains(allXML.String(), "lần gửi 0") {
		t.Fatal("workflow photo caption invents report revision")
	}
	if result, err = s.ExportNativeWorkItemPhotos(ctx, f.itemID, f.workerID, "Worker"); err != nil {
		t.Fatal(err)
	} else {
		nativeBMGSArtifact(t, "BM-GS-07", result)
	}
	other := setupProjectScopeFixture(t, db, []string{"operation.view"})
	if _, err = s.ExportNativeWorkItemPhotos(ctx, f.itemID, other.userID, "Other"); err == nil {
		t.Fatal("wrong actor photo owner exported")
	}
	nativeBMGSHTTP(t, s, "/work-items/"+f.itemID+"/photos/export.xlsx", f.workerID, 200)
	nativeBMGSHTTP(t, s, "/work-items/"+f.itemID+"/photos/export.xlsx", other.userID, 403)
}

func TestNativeBMGSChecklistInheritsMoreThanFiftyPhotos(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_bmgs_checkphotos_", []string{"workflow_form_fields", "evidence_records"})
	s := NewService(NewRepository(db), "test-secret")
	s.ConfigureUploads(t.TempDir(), 12*1024*1024)
	ctx := context.Background()
	f := setupWorkItemFlowFixture(t, db, false)
	var raw bytes.Buffer
	_ = png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	items := []Record{}
	for index := 0; index < 51; index++ {
		stored, _, err := s.storeGeneratedFile(ctx, fmt.Sprintf("evidence/check-%d.png", index), raw.Bytes(), "image/png")
		if err != nil {
			t.Fatal(err)
		}
		id := newID()
		mustExec(t, db, `INSERT INTO evidence_records(id,project_id,work_item_id,evidence_type,name,status,uploaded_by,storage_provider,storage_key,mime_type,file_size,checksum_sha256)VALUES($1,$2,$3,'photo','check.png','submitted',$4,$5,$6,'image/png',$7,$8)`, id, f.projectID, f.itemID, f.workerID, stored.StorageProvider, stored.StorageKey, len(raw.Bytes()), sha256Hex(raw.Bytes()))
		items = append(items, Record{"title": fmt.Sprintf("Dòng checklist %d", index), "evidence_id": id})
	}
	answers, _ := json.Marshal(Record{"checklist_items": items})
	formID := newID()
	mustExec(t, db, `INSERT INTO workflow_form_instances(id,project_id,work_item_id,form_template_id,form_version_id,form_type,title,answers_snapshot_json,created_by)SELECT $1,$2,$3,t.id,v.id,t.form_type,'51 linked images',$4::jsonb,$5 FROM workflow_form_templates t JOIN LATERAL(SELECT id FROM workflow_form_versions WHERE template_id=t.id AND status='published' ORDER BY version_no DESC LIMIT 1)v ON true WHERE t.code='builtin_subcontractor_checklist'`, formID, f.projectID, f.itemID, string(answers), f.workerID)
	result, err := s.ExportNativeWorkflowForm(ctx, formID, "", f.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(result.Data), int64(len(result.Data)))
	if err != nil {
		t.Fatal(err)
	}
	pics := 0
	for _, file := range archive.File {
		if strings.HasPrefix(file.Name, "xl/drawings/drawing") && strings.HasSuffix(file.Name, ".xml") {
			r, _ := file.Open()
			b, _ := io.ReadAll(r)
			r.Close()
			pics += bytes.Count(b, []byte("<xdr:pic>"))
		}
	}
	if pics != 51 {
		t.Fatalf("checklist photos omitted: %d", pics)
	}
}

func TestNativeBMGSActionListFiltersScopeAndStableSort(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_bmgs_actionlist_", []string{"construction_action_items"})
	s := NewService(NewRepository(db), "test-secret")
	f := setupWorkItemFlowFixture(t, db, false)
	ctx := context.Background()
	for index := 0; index < 55; index++ {
		mustExec(t, db, `INSERT INTO construction_action_items(id,project_id,work_item_id,title,priority,impact_scope,assigned_user_id,due_at)VALUES($1,$2,$3,$4,'urgent','timeline',$5,CASE WHEN $6::int=0 THEN NULL ELSE '2026-10-08'::timestamptz END)`, newID(), f.projectID, f.itemID, fmt.Sprintf("Lỗi sảnh %d", index), f.workerID, index)
	}
	filter := NativeConstructionActionFilter{Queue: "open", Search: "SẢNH", AssigneeID: f.workerID, Priority: "urgent", ImpactScope: "timeline"}
	rows, err := s.repo.nativeConstructionActions(ctx, f.projectID, f.workerID, filter)
	if err != nil || len(rows) != 55 {
		t.Fatalf("filtered rows=%d err=%v", len(rows), err)
	}
	result, err := s.ExportNativeConstructionActions(ctx, f.projectID, filter, f.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if len(book.GetSheetList()) != 1 {
		t.Fatal("register split across sources")
	}
	filter.Search = "không có"
	if _, err = s.ExportNativeConstructionActions(ctx, f.projectID, filter, f.workerID, "Worker"); err == nil {
		t.Fatal("empty filter silently exports other sources")
	}
	other := setupProjectScopeFixture(t, db, []string{"operation.view"})
	if _, err = s.ExportNativeConstructionActions(ctx, f.projectID, NativeConstructionActionFilter{Queue: "all"}, other.userID, "Other"); err == nil {
		t.Fatal("cross-project action list exported")
	}
	orderedIDs := []string{"a9000000-0000-4000-8000-000000000001", "a9000000-0000-4000-8000-000000000002", "a9000000-0000-4000-8000-000000000003", "a9000000-0000-4000-8000-000000000004"}
	for index, id := range orderedIDs {
		state, due := "open", "2026-10-08T08:00:00Z"
		if index == 2 {
			due = ""
		}
		if index == 3 {
			state = "done"
			due = "2020-01-01T08:00:00Z"
		}
		mustExec(t, db, `INSERT INTO construction_action_items(id,project_id,work_item_id,title,fault_code,priority,status,due_at,created_at)VALUES($1,$2,$3,'Thứ tự kiểm chứng',$4,'urgent',$5,NULLIF($6,'')::timestamptz,'2026-10-07T08:00:00Z')`, id, f.projectID, f.itemID, fmt.Sprintf("ORDER-%d", index+1), state, due)
	}
	orderFilter := NativeConstructionActionFilter{Queue: "all", Search: "thứ tự kiểm chứng"}
	ordered, err := s.repo.nativeConstructionActions(ctx, f.projectID, f.workerID, orderFilter)
	if err != nil || len(ordered) != 4 {
		t.Fatalf("sort fixture rows=%d err=%v", len(ordered), err)
	}
	for index, row := range ordered {
		if cleanID(row["id"]) != orderedIDs[index] {
			t.Fatalf("wrong lexicographic sort at %d: %s", index, cleanID(row["id"]))
		}
	}
	sorted, err := s.ExportNativeConstructionActions(ctx, f.projectID, orderFilter, f.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	sortedBook, err := excelize.OpenReader(bytes.NewReader(sorted.Data))
	if err != nil {
		t.Fatal(err)
	}
	defer sortedBook.Close()
	sheetRows, err := sortedBook.GetRows(sortedBook.GetSheetList()[0])
	if err != nil {
		t.Fatal(err)
	}
	actual := []string{}
	for _, row := range sheetRows {
		for _, cell := range row {
			if strings.HasPrefix(cell, "ORDER-") {
				actual = append(actual, cell)
			}
		}
	}
	if strings.Join(actual, ",") != "ORDER-1,ORDER-2,ORDER-3,ORDER-4" {
		t.Fatalf("XLSX order differs from query: %v", actual)
	}
}

func TestNativeBMGSDailyInheritsFiftyPhotosWithoutSupplementalGates(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_bmgs_photos_", []string{"operation_daily_reports", "evidence_records"})
	service := NewService(NewRepository(db), "test-secret")
	service.ConfigureUploads(t.TempDir(), 12*1024*1024)
	fixture := setupWorkItemFlowFixture(t, db, false)
	ctx := context.Background()
	var raw bytes.Buffer
	if err := png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for index := 0; index < 50; index++ {
		stored, _, err := service.storeGeneratedFile(ctx, fmt.Sprintf("evidence/native-original-%02d.png", index), raw.Bytes(), "image/png")
		if err != nil {
			t.Fatal(err)
		}
		id := newID()
		ids = append(ids, id)
		mustExec(t, db, `INSERT INTO evidence_records(id,project_id,work_item_id,evidence_type,name,status,uploaded_by,storage_provider,storage_key,mime_type,file_size,checksum_sha256,caption,payload_json) VALUES($1,$2,$3,'photo',$4,'submitted',$5,$6,$7,'image/png',$8,$9,'Ảnh đã liên kết', '{"capture_metadata":{"location":"Sảnh","photo_phase":"defect"}}'::jsonb)`, id, fixture.projectID, fixture.itemID, fmt.Sprintf("native-%02d.png", index), fixture.workerID, stored.StorageProvider, stored.StorageKey, len(raw.Bytes()), sha256Hex(raw.Bytes()))
	}
	photoJSON, _ := json.Marshal(ids)
	sourceID := newID()
	mustExec(t, db, `INSERT INTO operation_daily_reports(id,project_id,work_item_id,report_date,reported_by,photos,work_summary,structured_json) VALUES($1,$2,$3,'2026-10-08',$4,$5::jsonb,'Nội dung báo cáo gốc','{"done_items":[{"title":"Hoàn thiện khu vực sảnh"}]}'::jsonb)`, sourceID, fixture.projectID, fixture.itemID, fixture.workerID, string(photoJSON))
	result, err := service.ExportNativeDailyReport(ctx, sourceID, fixture.workerID, "Người bấm xuất")
	if err != nil {
		t.Fatal(err)
	}
	nativeBMGSArtifact(t, "BM-GS-02", result)
	nativeBMGSHTTP(t, service, "/daily-reports/"+sourceID+"/export.xlsx", fixture.workerID, 200)
	archive, err := zip.NewReader(bytes.NewReader(result.Data), int64(len(result.Data)))
	if err != nil {
		t.Fatal(err)
	}
	images := 0
	for _, file := range archive.File {
		if strings.HasPrefix(file.Name, "xl/drawings/drawing") && strings.HasSuffix(file.Name, ".xml") {
			reader, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			drawing, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				t.Fatal(err)
			}
			images += bytes.Count(drawing, []byte("<xdr:pic>"))
		}
	}
	if images != 50 {
		t.Fatalf("linked photos silently omitted: drawing pictures=%d", images)
	}
	book, err := excelize.OpenReader(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if got, _ := book.GetCellValue(book.GetSheetList()[0], "G6"); got != "Worker" {
		t.Fatalf("wrong source author: %q", got)
	}
	other := setupProjectScopeFixture(t, db, []string{"operation.view"})
	nativeBMGSHTTP(t, service, "/daily-reports/"+sourceID+"/export.xlsx", other.userID, 403)
	if _, err = service.ExportNativeDailyReport(ctx, sourceID, other.userID, "Other"); err == nil {
		t.Fatal("cross-project source exported")
	}
	ids = append(ids, newID())
	photoJSON, _ = json.Marshal(ids)
	mustExec(t, db, `UPDATE operation_daily_reports SET photos=$2::jsonb WHERE id=$1`, sourceID, string(photoJSON))
	if _, err = service.ExportNativeDailyReport(ctx, sourceID, fixture.workerID, "Worker"); err == nil {
		t.Fatal("missing linked photo silently omitted")
	}
	photoJSON, _ = json.Marshal(ids[:50])
	mustExec(t, db, `UPDATE operation_daily_reports SET photos=$2::jsonb WHERE id=$1`, sourceID, string(photoJSON))
	mustExec(t, db, `UPDATE evidence_records SET checksum_sha256=$2 WHERE id=$1`, ids[0], strings.Repeat("0", 64))
	if _, err = service.ExportNativeDailyReport(ctx, sourceID, fixture.workerID, "Worker"); err == nil {
		t.Fatal("tampered original image checksum silently exported")
	}
}

func TestNativeBMGSDailyListFiltersAndCapacity(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_bmgs_range_", []string{"operation_daily_reports"})
	service := NewService(NewRepository(db), "test-secret")
	owner := setupProjectScopeFixture(t, db, []string{"operation.view"})
	ctx := context.Background()
	for index := 0; index < 51; index++ {
		mustExec(t, db, `INSERT INTO operation_daily_reports(id,project_id,report_date,reported_by,work_summary) VALUES($1,$2,$3::date,$4,$5)`, newID(), owner.projectID, map[bool]string{true: "2026-10-07", false: "2026-10-08"}[index == 0], owner.userID, fmt.Sprintf("Source %d", index))
	}
	filter := NativeDailyReportFilter{DateFrom: "2026-10-08", DateTo: "2026-10-08"}
	rows, err := service.ListNativeDailyReports(ctx, owner.projectID, owner.userID, filter)
	if err != nil || len(rows) != 50 {
		t.Fatalf("native filtered list rows=%d err=%v", len(rows), err)
	}
	result, err := service.ExportNativeDailyReports(ctx, owner.projectID, filter, owner.userID, "Owner")
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if len(book.GetSheetList()) != 50 {
		t.Fatalf("matching report sheets=%d", len(book.GetSheetList()))
	}
	if _, err = service.ExportNativeDailyReports(ctx, owner.projectID, NativeDailyReportFilter{}, owner.userID, "Owner"); err == nil {
		t.Fatal("over-capacity range silently truncated")
	}
}

func TestNativeBMGSWorkReportUsesFrozenIdentityAndPhotoMetadata(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_bmgs_revision_", []string{"work_report_revisions", "evidence_records"})
	service := NewService(NewRepository(db), "test-secret")
	service.ConfigureUploads(t.TempDir(), 12*1024*1024)
	fixture := setupWorkItemFlowFixture(t, db, false)
	ctx := context.Background()
	mustExec(t, db, `INSERT INTO role_permissions(role_id,permission_id) SELECT role_id,permission.id FROM user_roles assignment CROSS JOIN permissions permission WHERE assignment.user_id=$1 AND permission.key IN ('work_report.view','work_report.export') ON CONFLICT DO NOTHING`, fixture.workerID)
	var raw bytes.Buffer
	png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	stored, _, err := service.storeGeneratedFile(ctx, "evidence/frozen-original.png", raw.Bytes(), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	photoID, sourceID, reportID := newID(), newID(), newID()
	mustExec(t, db, `INSERT INTO evidence_records(id,project_id,work_item_id,evidence_type,name,status,uploaded_by,storage_provider,storage_key,mime_type,file_size,checksum_sha256,caption,payload_json) VALUES($1,$2,$3,'photo','CURRENT NAME','submitted',$4,$5,$6,'image/png',$7,$8,'CURRENT CAPTION','{"capture_metadata":{"location":"CURRENT LOCATION"}}'::jsonb)`, photoID, fixture.projectID, fixture.itemID, fixture.workerID, stored.StorageProvider, stored.StorageKey, len(raw.Bytes()), sha256Hex(raw.Bytes()))
	mustExec(t, db, `INSERT INTO operation_daily_reports(id,project_id,work_item_id,report_date,reported_by,work_summary) VALUES($1,$2,$3,'2026-10-08',$4,'CURRENT CONTENT')`, sourceID, fixture.projectID, fixture.itemID, fixture.workerID)
	mustExec(t, db, `INSERT INTO work_reports(id,source_kind,source_id,project_id,report_type,report_date,author_id,recipient_id,current_revision_no) VALUES($1,'operation_daily_report',$2,$3,'construction_daily','2026-10-07',$4,$4,1)`, reportID, sourceID, fixture.projectID, fixture.workerID)
	snapshot := Record{"project_id": fixture.projectID, "work_item_id": fixture.itemID, "project_code": "FROZEN-CODE", "project_name": "Frozen project", "address": "Frozen address", "author_name": "Frozen supervisor", "report_date": "2026-10-07", "work_summary": "FROZEN CONTENT", "photos": []string{photoID}, "bm_gs_evidence": []Record{{"id": photoID, "project_id": fixture.projectID, "work_item_id": fixture.itemID, "evidence_type": "photo", "name": "Frozen filename.png", "caption": "Frozen caption", "storage_provider": stored.StorageProvider, "storage_key": stored.StorageKey, "mime_type": "image/png", "file_size": len(raw.Bytes()), "checksum_sha256": sha256Hex(raw.Bytes()), "payload_json": Record{"capture_metadata": Record{"location": "Frozen location"}}}}}
	encoded, _ := json.Marshal(snapshot)
	mustExec(t, db, `INSERT INTO work_report_revisions(report_id,revision_no,snapshot_json,snapshot_sha256,submitted_by) VALUES($1,1,$2::jsonb,$3,$4)`, reportID, string(encoded), sha256Hex(encoded), fixture.workerID)
	var changed bytes.Buffer
	if err = png.Encode(&changed, image.NewRGBA(image.Rect(0, 0, 17, 17))); err != nil {
		t.Fatal(err)
	}
	currentFile, _, err := service.storeGeneratedFile(ctx, "evidence/current-replacement.png", changed.Bytes(), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE evidence_records SET storage_key=$2,file_size=$3,checksum_sha256=$4 WHERE id=$1`, photoID, currentFile.StorageKey, len(changed.Bytes()), sha256Hex(changed.Bytes()))
	detail, err := service.WorkReportDetail(ctx, sourceID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	revisions := workflowAnswerRecordList(detail["revisions"])
	if len(revisions) != 1 {
		t.Fatalf("revisions=%d", len(revisions))
	}
	frozenRecords := workflowAnswerRecordList(revisions[0]["evidence_records"])
	if len(frozenRecords) != 1 || cleanText(frozenRecords[0]["caption"]) != "Frozen caption" || cleanText(frozenRecords[0]["storage_key"]) != stored.StorageKey {
		t.Fatalf("viewer imported live metadata/locator: %#v", frozenRecords)
	}
	frozenFile, err := service.WorkReportEvidenceFile(ctx, reportID, 1, photoID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	frozenBytes, err := io.ReadAll(frozenFile.Reader)
	frozenFile.Reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(frozenBytes, raw.Bytes()) || frozenFile.Filename != "Frozen filename.png" {
		t.Fatalf("historical file changed: filename=%s bytes=%d", frozenFile.Filename, len(frozenBytes))
	}
	result, err := service.ExportNativeWorkReport(ctx, reportID, 1, fixture.workerID, "Exporter")
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if got, _ := book.GetCellValue(book.GetSheetList()[0], "G6"); got != "Frozen supervisor" {
		t.Fatal(got)
	}
	all := []string{}
	for _, sheet := range book.GetSheetList() {
		rows, _ := book.GetRows(sheet)
		for _, row := range rows {
			all = append(all, strings.Join(row, " "))
		}
	}
	text := strings.Join(all, "\n")
	if !strings.Contains(text, "Frozen caption") || !strings.Contains(text, "Frozen location") || strings.Contains(text, "CURRENT CAPTION") || strings.Contains(text, "CURRENT LOCATION") {
		t.Fatalf("history metadata bleed: %s", text)
	}
	if _, err = service.ExportNativeWorkReport(ctx, reportID, 2, fixture.workerID, "Exporter"); err == nil {
		t.Fatal("missing revision fell back to current source")
	}
	delete(snapshot, "bm_gs_evidence")
	encoded, _ = json.Marshal(snapshot)
	mustExec(t, db, `INSERT INTO work_report_revisions(report_id,revision_no,snapshot_json,snapshot_sha256,submitted_by) VALUES($1,2,$2::jsonb,$3,$4)`, reportID, string(encoded), sha256Hex(encoded), fixture.workerID)
	legacy, err := service.ExportNativeWorkReport(ctx, reportID, 2, fixture.workerID, "Exporter")
	if err != nil {
		t.Fatal(err)
	}
	legacyBook, err := excelize.OpenReader(bytes.NewReader(legacy.Data))
	if err != nil {
		t.Fatal(err)
	}
	defer legacyBook.Close()
	for _, sheet := range legacyBook.GetSheetList() {
		rows, _ := legacyBook.GetRows(sheet)
		for _, row := range rows {
			line := strings.Join(row, " ")
			if strings.Contains(line, "CURRENT NAME") || strings.Contains(line, "CURRENT CAPTION") || strings.Contains(line, "CURRENT LOCATION") {
				t.Fatalf("legacy revision imported current metadata: %s", line)
			}
		}
	}
}

func TestNativeBMGSSupervisionExportIgnoresListPagination(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_bmgs_supervision_", []string{"work_report_revisions"})
	s := NewService(NewRepository(db), "test-secret")
	ctx := context.Background()
	owner := setupProjectScopeFixture(t, db, []string{"work_report.view", "work_report.export"})
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id)SELECT $1,id FROM roles WHERE name='supervisor' ON CONFLICT DO NOTHING`, owner.userID)
	deputy := newID()
	mustExec(t, db, `INSERT INTO users(id,email,password_hash,full_name,status)VALUES($1,$2,'demo:pw','Deputy','active')`, deputy, "native-deputy-"+deputy+"@test.local")
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id)SELECT $1,id FROM roles WHERE name='deputy_manager'`, deputy)
	for i := 0; i < 3; i++ {
		source, report := newID(), newID()
		mustExec(t, db, `INSERT INTO operation_daily_reports(id,project_id,report_date,reported_by,work_summary,report_delivery_eligible)VALUES($1,$2,'2026-10-08',$3,'Supervisor source',true)`, source, owner.projectID, owner.userID)
		mustExec(t, db, `INSERT INTO work_reports(id,source_kind,source_id,project_id,report_type,report_date,author_id,recipient_id,current_revision_no,is_supervisor_report)VALUES($1,'operation_daily_report',$2,$3,'construction_daily','2026-10-08',$4,$4,1,true)`, report, source, owner.projectID, owner.userID)
		snapshot, _ := json.Marshal(Record{"project_id": owner.projectID, "author_name": "Supervisor", "report_date": "2026-10-08", "work_summary": fmt.Sprintf("Frozen source %d", i)})
		mustExec(t, db, `INSERT INTO work_report_revisions(report_id,revision_no,snapshot_json,snapshot_sha256,submitted_by)VALUES($1,1,$2::jsonb,$3,$4)`, report, string(snapshot), sha256Hex(snapshot), owner.userID)
	}
	filter := WorkReportFilter{Scope: "supervision", Limit: 1, Offset: 1, DateFrom: "2026-10-08", DateTo: "2026-10-08"}
	page, err := s.WorkReports(ctx, filter, deputy)
	if err != nil || len(page) != 1 {
		t.Fatalf("native page=%d err=%v", len(page), err)
	}
	result, err := s.ExportNativeWorkReports(ctx, filter, deputy, "Deputy")
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if len(book.GetSheetList()) != 3 {
		t.Fatalf("export retained pagination: sheets=%d", len(book.GetSheetList()))
	}
	rows, err := s.workReports(ctx, filter, deputy, -1)
	if err != nil || len(rows) != 3 {
		t.Fatalf("legacy full-filter register rows=%d err=%v", len(rows), err)
	}
}

func TestNativeBMGSSnapshotComparisonIgnoresRetiredSupplementalFields(t *testing.T) {
	previous := Record{"work_summary": "Nguồn", "bm_gs_json": Record{"note": "legacy"}, "bm_gs_version": 3, "bm_gs_evidence": []Record{{"id": "source-photo", "caption": "Native caption", "bm_gs_json": Record{"location": "legacy"}}}}
	current := Record{"work_summary": "Nguồn", "bm_gs_evidence": []Record{{"id": "source-photo", "caption": "Native caption"}}}
	changed, err := workReportNativeSnapshotChanged(current, previous)
	if err != nil || changed {
		t.Fatalf("retired fields fabricate source correction: changed=%v err=%v", changed, err)
	}
	current["work_summary"] = "Nguồn được bổ sung"
	changed, err = workReportNativeSnapshotChanged(current, previous)
	if err != nil || !changed {
		t.Fatal("native fact change not detected")
	}
	if _, exists := previous["bm_gs_json"]; !exists {
		t.Fatal("comparison mutated immutable snapshot")
	}
}

func TestNativeBMGSSnapshotComparisonPreservesInputEvidenceMaps(t *testing.T) {
	for _, shape := range []string{"records", "json_maps"} {
		t.Run(shape, func(t *testing.T) {
			previousPhoto := Record{"id": "photo", "bm_gs_json": Record{"old": "previous"}, "bm_gs_version": 2, "payload_json": Record{"capture_metadata": Record{"location": "Sảnh"}}}
			currentPhoto := Record{"id": "photo", "bm_gs_json": Record{"old": "current"}, "bm_gs_version": 3, "payload_json": Record{"capture_metadata": Record{"location": "Sảnh"}}}
			previous, current := Record{"bm_gs_evidence": []Record{previousPhoto}}, Record{"bm_gs_evidence": []Record{currentPhoto}}
			if shape == "json_maps" {
				previous["bm_gs_evidence"] = []any{map[string]any(previousPhoto)}
				current["bm_gs_evidence"] = []any{map[string]any(currentPhoto)}
			}
			beforePrevious, _ := json.Marshal(previous)
			beforeCurrent, _ := json.Marshal(current)
			changed, err := workReportNativeSnapshotChanged(current, previous)
			if err != nil || changed {
				t.Fatalf("retired difference changed=%v err=%v", changed, err)
			}
			afterPrevious, _ := json.Marshal(previous)
			afterCurrent, _ := json.Marshal(current)
			if !bytes.Equal(beforePrevious, afterPrevious) || !bytes.Equal(beforeCurrent, afterCurrent) {
				t.Fatal("comparison mutated snapshot evidence maps")
			}
			currentPhoto["payload_json"] = Record{"capture_metadata": Record{"location": "Tầng một"}}
			beforeCurrent, _ = json.Marshal(current)
			changed, err = workReportNativeSnapshotChanged(current, previous)
			if err != nil || !changed {
				t.Fatalf("native payload change missed: %v %v", changed, err)
			}
			afterCurrent, _ = json.Marshal(current)
			afterPrevious, _ = json.Marshal(previous)
			if !bytes.Equal(beforePrevious, afterPrevious) || !bytes.Equal(beforeCurrent, afterCurrent) {
				t.Fatal("native comparison mutated snapshot evidence maps")
			}
		})
	}
}

func TestNativeBMGSRetiredChooserRoutesAbsent(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	registerBMGSExportRoutes(app, NewHandler(nil))
	for _, path := range []string{"/projects/project/bm-gs/options", "/projects/project/bm-gs/preview", "/projects/project/bm-gs/export", "/bm-gs/sources/daily_report/source"} {
		for _, method := range []string{"GET", "POST", "PUT"} {
			response, err := app.Test(httptest.NewRequest(method, path, nil))
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != 404 {
				t.Fatalf("retired route %s %s is public: %d", method, path, response.StatusCode)
			}
		}
	}
}
