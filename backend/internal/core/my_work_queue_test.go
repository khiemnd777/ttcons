package core

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestMyWorkQueueSourcePrecedenceOnlyForDefaultSort(t *testing.T) {
	for _, sortKind := range []string{"", "priority"} {
		if !strings.HasPrefix(myWorkQueueOrderSQL(sortKind), "CASE WHEN entry_kind='accounting_handoff' THEN 0 ELSE 1 END, ") {
			t.Fatalf("missing source precedence for %q", sortKind)
		}
	}
	for _, sortKind := range []string{"due_asc", "newest"} {
		if myWorkQueueOrderSQL(sortKind) != myWorkItemsOrderSQL(sortKind)+", entry_kind" {
			t.Fatalf("changed explicit sort %q", sortKind)
		}
	}
}

func TestMyWorkQueuePrioritizesAccountingBeforePagination(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	f := setupProjectScopeFixture(t, db, []string{"project.view", "accounting_handoff.view", "accounting_handoff.acknowledge", "operation.report"})
	s := NewService(NewRepository(db), "test-secret")
	// All workflow items would have outranked handoffs under the original priority
	// sort. Their later creation times and deadlines also exercise explicit sorts.
	mustExec(t, db, `INSERT INTO work_items(id,project_id,source_type,source_id,title,work_type,status,priority,assignee_staff_id,evidence_requirement_json,created_by,created_at,due_at)
		SELECT gen_random_uuid(),$1,'issue',gen_random_uuid(),'Priority work '||n,'task','blocked','urgent',$2,'{"required_permission":"operation.report"}',$3,
		'2026-02-01'::timestamptz + n*interval '1 day','2026-03-01'::timestamptz + n*interval '1 day'
		FROM generate_series(1,35) n`, f.projectID, f.staffID, f.userID)
	mustExec(t, db, `INSERT INTO accounting_handoffs(id,project_id,direction,source_type,source_id,status,handed_off_by,handed_off_at)
		SELECT gen_random_uuid(),$1,'payable','procurement_request',gen_random_uuid(),'handed_off',$2,
		'2026-01-01'::timestamptz + n*interval '1 day' FROM generate_series(1,3) n`, f.projectID, f.userID)
	read := func(sortKind, cursor string, limit int, kind string) Record {
		t.Helper()
		page, err := s.MyWorkItems(ctx, MyWorkItemsFilter{Unified: true, ProjectID: f.projectID, Queue: "all", Bucket: "all", Sort: sortKind, Cursor: cursor, Limit: limit, EntryKind: kind}, f.userID)
		if err != nil {
			t.Fatal(err)
		}
		return page
	}
	var kinds []string
	seen := map[string]bool{}
	cursor := ""
	for {
		page := read("priority", cursor, 2, "")
		if number(page["total"]) != 38 {
			t.Fatalf("changed count: %v", page["total"])
		}
		for _, entry := range page["entries"].([]Record) {
			key := cleanID(entry["kind"]) + ":" + cleanID(recordValue(entry["item"])["id"])
			if seen[key] {
				t.Fatalf("duplicate %s", key)
			}
			seen[key] = true
			kinds = append(kinds, cleanID(entry["kind"]))
		}
		if !boolValue(page["has_more"]) {
			break
		}
		cursor = cleanID(page["next_cursor"])
	}
	for index, kind := range kinds {
		want := "work_item"
		if index < 3 {
			want = "accounting_handoff"
		}
		if kind != want {
			t.Fatalf("position %d kind=%s want=%s", index, kind, want)
		}
	}
	if len(kinds) != 38 {
		t.Fatalf("lost entries: %d", len(kinds))
	}
	for _, sortKind := range []string{"due_asc", "newest"} {
		page := read(sortKind, "", 1, "")
		if page["entries"].([]Record)[0]["kind"] != "work_item" {
			t.Fatalf("source precedence leaked into %s", sortKind)
		}
	}
	// Internal order within the workflow group must stay identical to WorkItem-only.
	mixed := read("priority", "", 100, "")["entries"].([]Record)
	workOnly := read("priority", "", 100, "work_item")["entries"].([]Record)
	for i, entry := range workOnly {
		if cleanID(recordValue(entry["item"])["id"]) != cleanID(recordValue(mixed[i+3]["item"])["id"]) {
			t.Fatal("changed workflow relative order")
		}
	}
}

func TestMyWorkQueueFilterValidation(t *testing.T) {
	for _, status := range []string{"handed_off", "returned", "acknowledged"} {
		if _, _, err := normalizeMyWorkItemsFilter(MyWorkItemsFilter{Unified: true, Status: status, Bucket: "waiting_supplement", EntryKind: "accounting_handoff"}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := normalizeMyWorkItemsFilter(MyWorkItemsFilter{Status: status}); err == nil {
			t.Fatal("legacy work status contract expanded")
		}
	}
	if _, _, err := normalizeMyWorkItemsFilter(MyWorkItemsFilter{Unified: true, EntryKind: "invalid"}); err == nil {
		t.Fatal("invalid kind accepted")
	}
}

func TestMyWorkQueueMixedPaginationScopeAndCanonicalActions(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	permissions := []string{"project.view", "project.reconciliation.view", "accounting_handoff.view", "accounting_handoff.acknowledge", "operation.report"}
	f := setupProjectScopeFixture(t, db, permissions)
	s := NewService(NewRepository(db), "test-secret")
	workID := newID()
	mustExec(t, db, `INSERT INTO work_items(id,project_id,source_type,source_id,title,work_type,status,assignee_staff_id,evidence_requirement_json,created_by)
		VALUES ($1,$2,'issue',$3,'Mixed work','task','ready',$4,'{"required_permission":"operation.report"}',$5)`, workID, f.projectID, newID(), f.staffID, f.userID)
	handoffIDs := []string{}
	for _, status := range []string{"handed_off", "returned", "acknowledged"} {
		id := newID()
		handoffIDs = append(handoffIDs, id)
		mustExec(t, db, `INSERT INTO accounting_handoffs(id,project_id,direction,source_type,source_id,source_snapshot_json,status,handed_off_by,returned_by,returned_at,return_reason,acknowledged_by,acknowledged_at)
			VALUES ($1,$2,'payable','procurement_request',$3,'{"source_code":"MIX-PO","source_title":"Mixed accounting","counterparty_name":"Queue vendor","reference_amount":12500,"currency":"USD"}',$4,$5,
			CASE WHEN $4='returned' THEN $5::uuid END, CASE WHEN $4='returned' THEN now() END, CASE WHEN $4='returned' THEN 'Bổ sung PO' END,
			CASE WHEN $4='acknowledged' THEN $5::uuid END, CASE WHEN $4='acknowledged' THEN now() END)`, id, f.projectID, newID(), status, f.userID)
	}
	otherID := newID()
	mustExec(t, db, `INSERT INTO accounting_handoffs(id,project_id,direction,source_type,source_id,status,handed_off_by)
		VALUES ($1,$2,'payable','procurement_request',$3,'handed_off',$4)`, otherID, f.otherProjectID, newID(), f.userID)
	// Company observation must not turn somebody else's workflow item into personal work.
	mustExec(t, db, `INSERT INTO work_items(id,project_id,source_type,source_id,title,work_type,status,created_by)
		VALUES ($1,$2,'issue',$3,'Outside membership work','task','ready',$4)`, newID(), f.otherProjectID, newID(), f.userID)

	pageFor := func(filter MyWorkItemsFilter) Record {
		t.Helper()
		filter.Unified = true
		if filter.Queue == "" {
			filter.Queue = "all"
		}
		if filter.ProjectID == "" {
			filter.ProjectID = f.projectID
		}
		page, err := s.MyWorkItems(ctx, filter, f.userID)
		if err != nil {
			t.Fatal(err)
		}
		return page
	}
	seen := map[string]bool{}
	cursor := ""
	for {
		page := pageFor(MyWorkItemsFilter{Bucket: "all", Limit: 1, Cursor: cursor, Sort: "newest"})
		if number(page["total"]) != 4 {
			t.Fatalf("mixed total: %#v", page)
		}
		entries := page["entries"].([]Record)
		if len(entries) != 1 {
			t.Fatalf("page size: %#v", entries)
		}
		entry := entries[0]
		item := recordValue(entry["item"])
		key := cleanID(entry["kind"]) + ":" + cleanID(item["id"])
		if seen[key] {
			t.Fatalf("duplicate page entry %s", key)
		}
		seen[key] = true
		if entry["kind"] == "accounting_handoff" {
			if item["currency"] != "USD" || item["source_code"] != "MIX-PO" {
				t.Fatalf("lost source fields: %#v", item)
			}
			for _, field := range []string{"due_at", "assignee_staff_id", "execution_target", "capabilities"} {
				if _, exists := item[field]; exists {
					t.Fatalf("fake workflow field %s", field)
				}
			}
		} else if _, ok := item["capabilities"].(Record); !ok {
			t.Fatalf("missing work capabilities: %#v", item)
		}
		if !boolValue(page["has_more"]) {
			break
		}
		cursor = cleanID(page["next_cursor"])
	}
	if len(seen) != 4 {
		t.Fatalf("seen %d", len(seen))
	}
	for _, tc := range []struct {
		filter MyWorkItemsFilter
		total  int
	}{
		{MyWorkItemsFilter{Bucket: "need_action"}, 2},
		{MyWorkItemsFilter{Bucket: "waiting_supplement"}, 1},
		{MyWorkItemsFilter{Bucket: "completed"}, 1},
		{MyWorkItemsFilter{Bucket: "all", EntryKind: "work_item"}, 1},
		{MyWorkItemsFilter{Bucket: "all", EntryKind: "accounting_handoff"}, 3},
		{MyWorkItemsFilter{Search: "Queue vendor"}, 3},
		{MyWorkItemsFilter{Status: "acknowledged"}, 1},
		{MyWorkItemsFilter{EntryKind: "accounting_handoff", Deadline: "no_due"}, 0},
		{MyWorkItemsFilter{EntryKind: "accounting_handoff", Queue: "mine"}, 0},
		{MyWorkItemsFilter{EntryKind: "accounting_handoff", Queue: "department"}, 0},
		{MyWorkItemsFilter{EntryKind: "accounting_handoff", Queue: "watching"}, 0},
		{MyWorkItemsFilter{ProjectID: f.otherProjectID, Queue: "responsible", Bucket: "need_action"}, 0},
	} {
		page := pageFor(tc.filter)
		if int(number(page["total"])) != tc.total {
			t.Fatalf("filter %+v: got %v want %d", tc.filter, page["total"], tc.total)
		}
	}
	summary := recordValue(pageFor(MyWorkItemsFilter{})["summary"])
	if number(summary["need_action"]) != 2 || number(summary["waiting_supplement"]) != 1 || number(summary["completed"]) != 1 {
		t.Fatalf("summary: %#v", summary)
	}

	adminRoleID := ensureRole(t, db, "admin", "Quản trị")
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, f.userID, adminRoleID)
	isAccounting, err := s.repo.UserHasGlobalRole(ctx, f.userID, accountingRoleName)
	if err != nil || isAccounting {
		t.Fatalf("Admin incorrectly classified as Accounting: assigned=%v err=%v", isAccounting, err)
	}
	adminResponsiblePage := pageFor(MyWorkItemsFilter{Queue: "responsible", Bucket: "need_action", EntryKind: "accounting_handoff"})
	if number(adminResponsiblePage["total"]) != 0 {
		t.Fatalf("Admin accounting responsibility leaked from permissions: %#v", adminResponsiblePage)
	}
	adminScopeSummary, err := s.MyWorkScopeSummary(ctx, f.userID, f.projectID)
	if err != nil {
		t.Fatal(err)
	}
	if number(recordValue(adminScopeSummary["responsible"])["total"]) != 1 || number(recordValue(adminScopeSummary["all"])["total"]) != 2 {
		t.Fatalf("Admin scope summary = %#v, want responsible work only and accounting visible in all", adminScopeSummary)
	}
	adminNavigation, err := s.MyWorkNavigationSummary(ctx, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	adminBaseWork := number(adminNavigation["work_items_need_action"]) + number(adminNavigation["workflow_actions_ready"]) + number(adminNavigation["project_startups_pending"])
	if number(adminNavigation["accounting_total"]) < 1 || number(adminNavigation["work_total"]) != adminBaseWork {
		t.Fatalf("Admin navigation = %#v, want visible Accounting breakdown excluded from personal work", adminNavigation)
	}

	accountingRoleID := ensureRole(t, db, "accounting", "Kế toán")
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id,project_id) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, f.userID, accountingRoleID, f.projectID)
	isAccounting, err = s.repo.UserHasGlobalRole(ctx, f.userID, accountingRoleName)
	if err != nil || isAccounting {
		t.Fatalf("project-scoped Accounting role incorrectly became company responsibility: assigned=%v err=%v", isAccounting, err)
	}
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, f.userID, accountingRoleID)
	isAccounting, err = s.repo.UserHasGlobalRole(ctx, f.userID, accountingRoleName)
	if err != nil || !isAccounting {
		t.Fatalf("Accounting role was not recognized: assigned=%v err=%v", isAccounting, err)
	}
	accountingResponsiblePage := pageFor(MyWorkItemsFilter{Queue: "responsible", Bucket: "need_action", EntryKind: "accounting_handoff"})
	if number(accountingResponsiblePage["total"]) != 1 {
		t.Fatalf("Accounting handoff missing from Accounting responsibility: %#v", accountingResponsiblePage)
	}
	accountingResponsibleHistory := pageFor(MyWorkItemsFilter{Queue: "responsible", Bucket: "all", EntryKind: "accounting_handoff"})
	if number(accountingResponsibleHistory["total"]) != 1 {
		t.Fatalf("returned/acknowledged accounting history leaked into current responsibility: %#v", accountingResponsibleHistory)
	}
	accountingScopeSummary, err := s.MyWorkScopeSummary(ctx, f.userID, f.projectID)
	if err != nil {
		t.Fatal(err)
	}
	if number(recordValue(accountingScopeSummary["responsible"])["total"]) != 2 || number(recordValue(accountingScopeSummary["all"])["total"]) != 2 {
		t.Fatalf("Accounting scope summary = %#v, want handoff in responsible and all", accountingScopeSummary)
	}
	accountingNavigation, err := s.MyWorkNavigationSummary(ctx, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	accountingBaseWork := number(accountingNavigation["work_items_need_action"]) + number(accountingNavigation["workflow_actions_ready"]) + number(accountingNavigation["project_startups_pending"])
	if number(accountingNavigation["accounting_total"]) < 1 || number(accountingNavigation["work_total"]) != accountingBaseWork+number(accountingNavigation["accounting_total"]) {
		t.Fatalf("Accounting navigation = %#v, want visible Accounting breakdown included in personal work", accountingNavigation)
	}

	legacy, err := s.MyWorkItems(ctx, MyWorkItemsFilter{ProjectID: f.projectID}, f.userID)
	if err != nil || number(legacy["total"]) != 1 {
		t.Fatalf("legacy: %#v %v", legacy, err)
	}
	if _, exists := legacy["entries"]; exists {
		t.Fatal("changed legacy shape")
	}

	unprivileged := setupProjectScopeFixture(t, db, []string{"project.view", "accounting_handoff.view"})
	deniedPage, err := s.MyWorkItems(ctx, MyWorkItemsFilter{Unified: true, IncludeAccounting: true, AccountingResponsible: true, EntryKind: "accounting_handoff"}, unprivileged.userID)
	if err != nil || number(deniedPage["total"]) != 0 {
		t.Fatalf("permission bypass: %#v %v", deniedPage, err)
	}
	// Mutation authorization lives at the canonical HTTP handler boundary.
	deniedApp := batch4TestApp(unprivileged.userID, func(app *fiber.App, h *Handler) {
		app.Post("/accounting-handoffs/:id/:action", h.AccountingHandoffAction)
	}, NewHandler(s))
	deniedResponse, err := deniedApp.Test(httptest.NewRequest("POST", "/accounting-handoffs/"+handoffIDs[0]+"/acknowledge", nil))
	if err != nil {
		t.Fatal(err)
	}
	_ = deniedResponse.Body.Close()
	if deniedResponse.StatusCode != 403 {
		t.Fatalf("unauthorized acknowledge HTTP %d", deniedResponse.StatusCode)
	}

	second := setupProjectScopeFixture(t, db, permissions)
	results := make(chan error, 2)
	for _, actor := range []string{f.userID, second.userID} {
		go func(actor string) {
			_, err := s.AccountingHandoffAction(ctx, handoffIDs[0], "acknowledge", AccountingHandoffActionRequest{}, actor, "Accounting")
			results <- err
		}(actor)
	}
	successes := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent successes %d", successes)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE target_id=$1 AND target_type='accounting_handoff'`, 1, handoffIDs[0])
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND status='ready'`, 1, workID)
	if number(pageFor(MyWorkItemsFilter{Bucket: "need_action", EntryKind: "accounting_handoff"})["total"]) != 0 {
		t.Fatal("acknowledged remained actionable")
	}

	app := batch4TestApp(f.userID, func(app *fiber.App, h *Handler) { app.Get("/me/work-queue", h.MyWorkQueue) }, NewHandler(s))
	response, err := app.Test(httptest.NewRequest("GET", "/me/work-queue?entry_kind=invalid", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode < 400 || response.StatusCode >= 500 {
		t.Fatalf("invalid filter HTTP %d", response.StatusCode)
	}
}
