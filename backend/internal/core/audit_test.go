package core

import (
	"context"
	"testing"
)

func TestNormalizeAuditLogFilter(t *testing.T) {
	filter, err := normalizeAuditLogFilter(AuditLogFilter{
		ProjectID:  "30000000-0000-0000-0000-000000000022",
		EventType:  " Node_Started ",
		TargetType: " Pipeline_Run ",
		ActorName:  "  Nguyễn Minh PM  ",
		Search:     "  Khảo sát  ",
		DateFrom:   "2026-07-01",
		DateTo:     "2026-07-13",
		Page:       -1,
		PageSize:   500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filter.EventType != "node_started" || filter.TargetType != "pipeline_run" || filter.ActorName != "Nguyễn Minh PM" || filter.Search != "Khảo sát" {
		t.Fatalf("normalized audit filter = %#v", filter)
	}
	if filter.Page != 1 || filter.PageSize != 25 {
		t.Fatalf("normalized audit paging = %#v", filter)
	}

	if _, err := normalizeAuditLogFilter(AuditLogFilter{ProjectID: "not-a-project"}); err == nil {
		t.Fatal("expected invalid project id to fail")
	}
	if _, err := normalizeAuditLogFilter(AuditLogFilter{DateFrom: "2026-07-14", DateTo: "2026-07-13"}); err == nil {
		t.Fatal("expected invalid date range to fail")
	}
}

func TestAuditLogPageFiltersAndPagination(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	projectID := newID()
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status) VALUES ($1,'Audit Filter Project',$2,'Client','planning')`, projectID, "AUD-"+projectID[:8])
	t.Cleanup(func() { _, _ = db.ExecContext(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID) })
	mustExec(t, db, `
		INSERT INTO audit_logs (project_id,actor_name,event_type,target_type,description,metadata_json,created_at)
		VALUES
			($1,'Nguyễn Minh PM','node_started','pipeline_run','Mở node Khảo sát','{"node":1}',now()-interval '1 minute'),
			($1,'Admin','project_note','project','Ghi chú công trình','{}',now())
	`, projectID)

	page, err := NewService(NewRepository(db), "test-secret").AuditLogs(ctx, projectID, "", AuditLogFilter{
		Search:   "Khảo sát",
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if number(page["total"]) != 1 || number(page["total_pages"]) != 1 {
		t.Fatalf("audit page totals = %#v", page)
	}
	items, ok := page["items"].([]Record)
	if !ok || len(items) != 1 || items[0]["event_type"] != "node_started" {
		t.Fatalf("audit page items = %#v", page["items"])
	}
}
