package core

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestNativeDailyFormExportUsesActualSavedAnswers(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_daily_form_export_", []string{"workflow_form_instances", "operation_daily_reports"})
	s := NewService(NewRepository(db), "test-secret")
	s.ConfigureUploads(t.TempDir(), 12*1024*1024)
	f := setupWorkItemFlowFixture(t, db, false)
	ctx := context.Background()
	var raw bytes.Buffer
	if err := png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	stored, _, err := s.storeGeneratedFile(ctx, "evidence/daily-form.png", raw.Bytes(), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	photoID := newID()
	mustExec(t, db, `INSERT INTO evidence_records(id,project_id,work_item_id,evidence_type,name,status,uploaded_by,storage_provider,storage_key,mime_type,file_size,checksum_sha256)VALUES($1,$2,$3,'photo','daily-form.png','submitted',$4,$5,$6,'image/png',$7,$8)`, photoID, f.projectID, f.itemID, f.workerID, stored.StorageProvider, stored.StorageKey, len(raw.Bytes()), sha256Hex(raw.Bytes()))
	formID := newID()
	mustExec(t, db, `INSERT INTO workflow_form_instances(id,project_id,work_item_id,form_template_id,form_version_id,form_type,title,created_by)SELECT $1,$2,$3,t.id,v.id,t.form_type,'Saved daily source',$4 FROM workflow_form_templates t JOIN LATERAL(SELECT id FROM workflow_form_versions WHERE template_id=t.id AND status='published' ORDER BY version_no DESC LIMIT 1)v ON true WHERE t.code='builtin_construction_daily_report'`, formID, f.projectID, f.itemID, f.workerID)
	answers := Record{
		"report_date": "2026-10-08", "reported_by": "Giám sát được ghi nhận", "construction_scope": "civil", "weather": "normal",
		"completed_items":        []any{map[string]any{"title": "Hoàn thiện sảnh đã lưu", "result": "done", "evidence_id": photoID}},
		"crew_items":             []any{map[string]any{"crew_name": "Tổ xây nguồn", "worker_count": 3, "work_hours": 8, "representative": "Đại diện nguồn", "contact": "0900000000"}},
		"material_arrival_items": []any{map[string]any{"material_name": "Gạch đến hiện trường", "quantity": 12, "unit": "viên", "note": "Ghi chú vật tư nguồn"}},
		"blockers":               []any{map[string]any{"content": "Vướng mặt bằng nguồn", "severity": "high", "impact": "access", "risk_code": "RR-NATIVE", "cause": "Chưa dọn mặt bằng", "owner": "PM nguồn", "status": "Đang theo dõi"}},
		"next_plan_items":        []any{map[string]any{"title": "Giao việc nguồn", "owner_role": "PM", "due_date": "2026-10-09", "priority": "normal", "receiver": "Đại diện nhận việc nguồn", "progress_percent": 0}},
	}
	saved, err := s.SaveWorkflowFormDraft(ctx, formID, WorkflowFormSaveRequest{Answers: answers}, f.workerID, "Creator profile")
	if err != nil {
		nativeDailyFormFatal(t, "draft", err)
	}
	canonical := recordFromAny(saved["answers"])
	if _, ok := canonical["report_date"].(string); !ok {
		t.Fatalf("actual save date shape is not raw: %#v", canonical["report_date"])
	}
	for _, purpose := range []string{"default", "risks", "next_plan"} {
		result, err := s.ExportNativeWorkflowForm(ctx, formID, purpose, f.workerID, "Export actor")
		if err != nil {
			t.Fatalf("draft %s: %v", purpose, err)
		}
		if !bytes.HasPrefix(result.Data, []byte("PK")) {
			t.Fatal("draft export is not XLSX")
		}
	}
	submitted, err := s.SubmitWorkflowForm(ctx, formID, WorkflowFormSaveRequest{Answers: canonical}, f.workerID, "Creator profile")
	if err != nil {
		nativeDailyFormFatal(t, "submit", err)
	}
	canonical = recordFromAny(submitted["answers"])
	if cleanText(canonical["reported_by"]) != "Giám sát được ghi nhận" {
		t.Fatalf("captured author changed: %v", canonical["reported_by"])
	}
	// A synchronized source is deliberately different. Exports must use the
	// selected saved form, not search for a current daily-report substitute.
	mustExec(t, db, `UPDATE operation_daily_reports SET work_summary='UNRELATED LIVE DAILY',structured_json='{}'::jsonb WHERE workflow_form_instance_id=$1`, formID)
	denied := setupProjectScopeFixture(t, db, []string{"operation.view"})
	for _, purpose := range []string{"default", "risks", "next_plan"} {
		result, err := s.ExportNativeWorkflowForm(ctx, formID, purpose, f.workerID, "Export actor")
		if err != nil {
			t.Fatalf("submitted %s: %v", purpose, err)
		}
		book, err := excelize.OpenReader(bytes.NewReader(result.Data))
		if err != nil {
			t.Fatal(err)
		}
		all := strings.Builder{}
		for _, sheet := range book.GetSheetList() {
			rows, _ := book.GetRows(sheet)
			for _, row := range rows {
				all.WriteString(strings.Join(row, " ") + "\n")
			}
		}
		text := all.String()
		if strings.Contains(text, "UNRELATED LIVE DAILY") {
			t.Fatal("export substituted synchronized live daily source")
		}
		switch purpose {
		case "default":
			if got, _ := book.GetCellValue(book.GetSheetList()[0], "G6"); got != "Giám sát được ghi nhận" {
				t.Fatalf("source author=%q", got)
			}
			for _, expected := range []string{"Hoàn thiện sảnh đã lưu", "Tổ xây nguồn", "Đại diện nguồn", "Gạch đến hiện trường", "Ghi chú vật tư nguồn", "Chưa dọn mặt bằng", "PM nguồn", "Đại diện nhận việc nguồn", "0%"} {
				if !strings.Contains(text, expected) {
					t.Fatalf("BM02 lost saved fact %s", expected)
				}
			}
		case "risks":
			if !strings.Contains(text, "RR-NATIVE") || !strings.Contains(text, "Vướng mặt bằng nguồn") {
				t.Fatal("BM05 lost native blockers")
			}
		case "next_plan":
			if !strings.Contains(text, "Giao việc nguồn") || !strings.Contains(text, "Đại diện nhận việc nguồn") {
				t.Fatal("BM06 lost saved next plan")
			}
		}
		book.Close()
		archive, err := zip.NewReader(bytes.NewReader(result.Data), int64(len(result.Data)))
		if err != nil {
			t.Fatal(err)
		}
		pictures := 0
		for _, file := range archive.File {
			if strings.HasPrefix(file.Name, "xl/drawings/drawing") && strings.HasSuffix(file.Name, ".xml") {
				r, _ := file.Open()
				data, _ := io.ReadAll(r)
				r.Close()
				pictures += bytes.Count(data, []byte("<xdr:pic>"))
			}
		}
		if pictures != 1 {
			t.Fatalf("%s lost exact saved linked photo: %d", purpose, pictures)
		}
		path := "/workflow-form-instances/" + formID + "/export.xlsx"
		if purpose == "risks" {
			path = "/workflow-form-instances/" + formID + "/risks/export.xlsx"
		}
		if purpose == "next_plan" {
			path = "/workflow-form-instances/" + formID + "/next-plan/export.xlsx"
		}
		nativeBMGSHTTP(t, s, path, f.workerID, 200)
		nativeBMGSHTTP(t, s, path, denied.userID, 403)
	}
}

func nativeDailyFormFatal(t *testing.T, phase string, err error) {
	t.Helper()
	var status StatusError
	if errors.As(err, &status) {
		t.Fatalf("%s %s details=%+v: %v", phase, status.Code, status.Details, err)
	}
	t.Fatal(err)
}

func TestNativeDailyFormAdapterRawAndWrappedAnswersNoFallback(t *testing.T) {
	answers := Record{"report_date": "2026-10-08", "reported_by": "Captured author", "weather": "rain", "progress_percent": 0, "notes": "Captured notes", "crew_items": []Record{{"crew_name": "Captured crew"}}}
	for _, wrapped := range []bool{false, true} {
		input := Record{}
		for key, value := range answers {
			if wrapped {
				input[key] = Record{"value": value}
			} else {
				input[key] = value
			}
		}
		source := Record{"answers_snapshot_json": input, "author_name": "Live creator", "report_date": "2099-01-01", "progress_percent": 99, "photos": []string{"source-photo"}}
		before, _ := json.Marshal(source)
		row := nativeWorkflowDailyExportRow(source)
		after, _ := json.Marshal(source)
		if !bytes.Equal(before, after) {
			t.Fatal("adapter changed saved answers")
		}
		if row["author_name"] != "Captured author" || row["report_date"] != "2026-10-08" || row["progress_percent"] != 0 || row["notes"] != "Captured notes" {
			t.Fatalf("captured headers changed: %#v", row)
		}
		if len(workflowAnswerRecordList(recordFromAny(row["structured_json"])["crew_items"])) != 1 {
			t.Fatal("crew data lost")
		}
	}
	missing := nativeWorkflowDailyExportRow(Record{"answers_snapshot_json": Record{}, "author_name": "Live creator", "report_date": "2099-01-01", "progress_percent": 99})
	if missing["author_name"] != nil || missing["report_date"] != nil || missing["progress_percent"] != nil {
		t.Fatal("uncaptured daily headers fabricated")
	}
}
