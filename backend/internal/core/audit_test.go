package core

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"log"
	"strings"
	"testing"
)

type auditExecStub struct {
	err   error
	calls int
}

func (s *auditExecStub) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return driver.RowsAffected(1), nil
}

func TestWriteAuditPropagatesMetadataAndDatabaseFailures(t *testing.T) {
	marshalStub := &auditExecStub{}
	err := writeAudit(context.Background(), marshalStub, auditEvent{EventType: "critical_transition", Metadata: make(chan int)})
	if err == nil || !strings.Contains(err.Error(), "marshal audit metadata") {
		t.Fatalf("writeAudit metadata error = %v, want marshal failure", err)
	}
	if marshalStub.calls != 0 {
		t.Fatalf("audit executor called %d times after metadata failure, want 0", marshalStub.calls)
	}

	databaseStub := &auditExecStub{err: errors.New("injected audit insert failure")}
	err = writeAudit(context.Background(), databaseStub, auditEvent{EventType: "critical_transition", Metadata: Record{}})
	if err == nil || !strings.Contains(err.Error(), "injected audit insert failure") {
		t.Fatalf("writeAudit database error = %v, want injected failure", err)
	}
}

func TestWriteAuditBestEffortLogsStructuredFailure(t *testing.T) {
	stub := &auditExecStub{err: errors.New("injected best-effort failure")}
	var output bytes.Buffer
	originalWriter := log.Writer()
	originalFlags := log.Flags()
	originalPrefix := log.Prefix()
	log.SetOutput(&output)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
		log.SetPrefix(originalPrefix)
	})

	writeAuditBestEffort(context.Background(), stub, auditEvent{
		ProjectID: "project-1", ActorID: "actor-1", EventType: "daily_report_updated",
		TargetType: "daily_report", TargetID: "report-1", Metadata: Record{},
	})
	message := output.String()
	for _, field := range []string{"component=audit", "policy=best_effort", `event_type="daily_report_updated"`, `target_id="report-1"`, "injected best-effort failure"} {
		if !strings.Contains(message, field) {
			t.Fatalf("best-effort audit log %q missing %q", message, field)
		}
	}
}

func TestWithAuditTransactionRollsBackMutationWhenAuditFails(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	tests := []struct {
		name     string
		mutate   func(*auditEvent)
		wantText string
	}{
		{name: "metadata marshal", mutate: func(event *auditEvent) { event.Metadata = make(chan int) }, wantText: "marshal audit metadata"},
		{name: "invalid actor UUID", mutate: func(event *auditEvent) { event.ActorID = "invalid-actor" }, wantText: "write audit event"},
		{name: "invalid target UUID", mutate: func(event *auditEvent) { event.TargetID = "invalid-target" }, wantText: "write audit event"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			partnerCode := "AUDIT-ROLLBACK-" + newID()[:8]
			t.Cleanup(func() {
				_, _ = db.ExecContext(context.Background(), `DELETE FROM partners WHERE partner_code=$1`, partnerCode)
			})

			_, err := repo.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
				rows, queryErr := queryTx(ctx, tx, `
					INSERT INTO partners (partner_code, name, partner_types)
					VALUES ($1, 'Audit rollback probe', '["supplier"]'::jsonb)
					RETURNING id
				`, partnerCode)
				if queryErr != nil {
					return nil, queryErr
				}
				return rows[0], nil
			}, func(row Record) auditEvent {
				event := auditEvent{
					EventType: "partner_created", TargetType: "partner", TargetID: cleanID(row["id"]), Metadata: Record{},
				}
				tt.mutate(&event)
				return event
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("withAuditTransaction error = %v, want %q failure", err, tt.wantText)
			}

			var count int
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM partners WHERE partner_code=$1`, partnerCode).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("partner rows after audit failure = %d, want rollback", count)
			}
		})
	}
}

func TestUpdateIssueRollsBackStatusWhenAuditActorIsInvalid(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	projectID := newID()
	issueID := newID()
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status) VALUES ($1,'Audit Issue Project',$2,'Client','planning')`, projectID, "AUD-ISS-"+projectID[:8])
	mustExec(t, db, `INSERT INTO issues (id,project_id,title,severity,status) VALUES ($1,$2,'Audit issue','medium','open')`, issueID, projectID)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
	})

	_, err := repo.UpdateIssue(ctx, issueID, Record{"status": "closed"}, "invalid-actor", "Invalid Actor")
	if err == nil || !strings.Contains(err.Error(), "write audit event issue_updated") {
		t.Fatalf("UpdateIssue error = %v, want audit actor UUID failure", err)
	}
	var status string
	if err := db.QueryRowContext(ctx, `SELECT status FROM issues WHERE id=$1`, issueID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "open" {
		t.Fatalf("issue status after audit failure = %q, want rollback to open", status)
	}
}

func TestUpdateIssueNonStatusEditPersistsWhenAuditFails(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	projectID := newID()
	issueID := newID()
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status) VALUES ($1,'Audit Issue Edit Project',$2,'Client','planning')`, projectID, "AUD-EDIT-"+projectID[:8])
	mustExec(t, db, `INSERT INTO issues (id,project_id,title,severity,status) VALUES ($1,$2,'Original title','medium','open')`, issueID, projectID)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
	})

	var output bytes.Buffer
	originalWriter := log.Writer()
	originalFlags := log.Flags()
	originalPrefix := log.Prefix()
	log.SetOutput(&output)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
		log.SetPrefix(originalPrefix)
	})

	row, err := repo.UpdateIssue(ctx, issueID, Record{"title": "Updated title", "severity": "high"}, "invalid-actor", "Invalid Actor")
	if err != nil {
		t.Fatalf("UpdateIssue non-status error = %v, want best-effort success", err)
	}
	if row["title"] != "Updated title" || row["severity"] != "high" {
		t.Fatalf("UpdateIssue non-status row = %#v, want updated title and severity", row)
	}

	var title, severity, status string
	if err := db.QueryRowContext(ctx, `SELECT title, severity, status FROM issues WHERE id=$1`, issueID).Scan(&title, &severity, &status); err != nil {
		t.Fatal(err)
	}
	if title != "Updated title" || severity != "high" || status != "open" {
		t.Fatalf("persisted issue = title %q severity %q status %q, want updated/high/open", title, severity, status)
	}
	message := output.String()
	for _, field := range []string{"component=audit", "policy=best_effort", `event_type="issue_updated"`, `target_id="` + issueID + `"`, "invalid input syntax for type uuid"} {
		if !strings.Contains(message, field) {
			t.Fatalf("best-effort issue audit log %q missing %q", message, field)
		}
	}
}

func TestUpdateIssueNonStatusAuditPersistsTargetUUID(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	projectID := newID()
	issueID := newID()
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status) VALUES ($1,'Audit Issue Target Project',$2,'Client','planning')`, projectID, "AUD-TARGET-"+projectID[:8])
	mustExec(t, db, `INSERT INTO issues (id,project_id,title,severity,status) VALUES ($1,$2,'Original title','medium','open')`, issueID, projectID)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
	})

	if _, err := repo.UpdateIssue(ctx, issueID, Record{"title": "Updated title"}, "", "System"); err != nil {
		t.Fatal(err)
	}
	var count int
	var targetID string
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(MIN(target_id::text), '')
		FROM audit_logs
		WHERE project_id=$1 AND event_type='issue_updated'
	`, projectID).Scan(&count, &targetID); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("issue_updated event count = %d, want 1", count)
	}
	if targetID != issueID {
		t.Fatalf("issue_updated target_id = %q, want %q", targetID, issueID)
	}
}

func TestWithAuditTransactionPersistsTargetUUID(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	partnerCode := "AUDIT-TARGET-" + newID()[:8]
	row, err := repo.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		rows, queryErr := queryTx(ctx, tx, `
			INSERT INTO partners (partner_code, name, partner_types)
			VALUES ($1, 'Audit target probe', '["supplier"]'::jsonb)
			RETURNING id
		`, partnerCode)
		if queryErr != nil {
			return nil, queryErr
		}
		return rows[0], nil
	}, func(Record) auditEvent {
		return auditEvent{EventType: "partner_created", TargetType: "partner", Metadata: Record{}}
	})
	if err != nil {
		t.Fatal(err)
	}
	partnerID := cleanID(row["id"])
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM audit_logs WHERE event_type='partner_created' AND target_id=$1`, partnerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM partners WHERE id=$1`, partnerID)
	})

	var targetID string
	if err := db.QueryRowContext(ctx, `SELECT target_id::text FROM audit_logs WHERE event_type='partner_created' AND target_id=$1 ORDER BY created_at DESC LIMIT 1`, partnerID).Scan(&targetID); err != nil {
		t.Fatal(err)
	}
	if targetID != partnerID {
		t.Fatalf("audit target_id = %q, want %q", targetID, partnerID)
	}
}

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
