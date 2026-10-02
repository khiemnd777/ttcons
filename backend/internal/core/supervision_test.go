package core

import (
	"context"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestSupervisionFilterIsBounded(t *testing.T) {
	for _, filter := range []SupervisionWorkFilter{{Limit: 51}, {Limit: -1}, {Offset: -1}, {Offset: 10001}, {SupervisorID: "bad"}, {Status: "cancelled"}} {
		if _, err := normalizeSupervisionWorkFilter(filter); err == nil {
			t.Fatalf("accepted invalid filter: %#v", filter)
		}
	}
	filter, err := normalizeSupervisionWorkFilter(SupervisionWorkFilter{Status: "pending"})
	if err != nil || filter.Limit != 25 {
		t.Fatalf("default filter %#v: %v", filter, err)
	}
	profile := workItemAccessProfile{CanSuperviseAllProjects: true}
	if supervisionCompanyScopeAllowed(profile, []string{"project_finance.view"}, []string{"project_finance.view"}) {
		t.Fatal("supervision widened finance scope")
	}
	if !supervisionCompanyScopeAllowed(profile, []string{"operation.view"}, []string{"operation.view"}) {
		t.Fatal("supervision read scope missing")
	}
}

func TestSupervisionOptionsMatchCanonicalClassification(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	supervisorRoleID := ensureRole(t, db, "supervisor", "Giám sát")
	// Sample identities are unique per role; reuse a seeded identity without
	// mutating it when this test runs against an already populated test DB.
	sampleUsers, err := repo.Query(ctx, `SELECT id::text FROM users WHERE is_sample AND sample_role_id=$1`, supervisorRoleID)
	if err != nil {
		t.Fatal(err)
	}
	otherRoleID := ensureRole(t, db, "options-other-"+newID(), "Giám sát")
	projectIDs := []string{newID(), newID(), newID()}
	var userIDs, staffIDs []string
	t.Cleanup(func() {
		for _, id := range projectIDs {
			_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, id)
		}
		for _, id := range staffIDs {
			_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, id)
		}
		for _, id := range userIDs {
			// Remove grants while the user still exists: explorer invalidation
			// on a cascaded role deletion would otherwise reference a deleted user.
			_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, id)
			_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, id)
		}
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, otherRoleID)
	})
	for i, id := range projectIDs {
		status := "planning"
		if i == 2 {
			status = "cancelled"
		}
		mustExec(t, db, `INSERT INTO projects(id,name,code,client_name,status) VALUES($1,'Options fixture',$2,'Test',$3)`, id, "OPTIONS-"+id, status)
	}
	createUser := func(name, status string, deleted bool) string {
		id := newID()
		userIDs = append(userIDs, id)
		mustExec(t, db, `INSERT INTO users(id,email,password_hash,full_name,status,deleted_at) VALUES($1,$2,'demo:pw',$3,$4,CASE WHEN $5 THEN now() END)`, id, "options-"+id+"@test.local", name, status, deleted)
		return id
	}
	deputyID := createUser("Deputy options", "active", false)
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id) SELECT $1,id FROM roles WHERE name='deputy_manager'`, deputyID)
	type optionCase struct {
		name, grant, userStatus, staffStatus string
		project                              int
		sample, deleted, inactiveMember      bool
		want                                 bool
	}
	cases := []optionCase{
		{name: "Global", grant: "global", want: true},
		{name: "Scoped real", grant: "scoped", want: true},
		{name: "Scoped sample", grant: "scoped", project: 1},
		{name: "Scoped cancelled", grant: "scoped", project: 2},
		{name: "Member label", grant: "Giám sát", want: true},
		{name: "Member key", grant: "supervisor", want: true},
		{name: "Member UUID", grant: supervisorRoleID, want: true},
		{name: "Member sample", grant: "Giám sát", project: 1},
		{name: "Member cancelled", grant: "Giám sát", project: 2},
		{name: "Inactive membership", grant: "Giám sát", inactiveMember: true},
		{name: "Deleted staff", grant: "Giám sát", staffStatus: "deleted"},
		// Canonical classification accepts non-deleted staff, even when inactive;
		// query optimization must not silently change that existing contract.
		{name: "Inactive staff", grant: "Giám sát", staffStatus: "inactive", want: true},
		{name: "Inactive account", grant: "global", userStatus: "inactive"},
		{name: "Deleted account", grant: "global", userStatus: "inactive", deleted: true},
		{name: "Sample account", grant: "scoped", project: 1, sample: true},
		{name: "Position only", grant: "position"},
		{name: "Noncanonical role display", grant: "other"},
		{name: "Multiple grants", grant: "multiple", want: true},
	}
	caseIDs := make([]string, len(cases))
	for i, tc := range cases {
		if tc.sample && len(sampleUsers) > 0 {
			caseIDs[i] = cleanID(sampleUsers[0]["id"])
			continue
		}
		userStatus := tc.userStatus
		if userStatus == "" {
			userStatus = "active"
		}
		id := createUser(tc.name, userStatus, tc.deleted)
		caseIDs[i] = id
		switch tc.grant {
		case "global", "multiple":
			mustExec(t, db, `INSERT INTO user_roles(user_id,role_id) VALUES($1,$2)`, id, supervisorRoleID)
		case "scoped":
			mustExec(t, db, `INSERT INTO user_roles(user_id,role_id,project_id) VALUES($1,$2,$3)`, id, supervisorRoleID, projectIDs[tc.project])
		case "other":
			mustExec(t, db, `INSERT INTO user_roles(user_id,role_id) VALUES($1,$2)`, id, otherRoleID)
		}
		if tc.grant == "global" || tc.grant == "scoped" || tc.grant == "other" {
			continue
		}
		staffID := newID()
		staffIDs = append(staffIDs, staffID)
		staffStatus := tc.staffStatus
		if staffStatus == "" {
			staffStatus = "active"
		}
		mustExec(t, db, `INSERT INTO staff(id,user_id,full_name,email,position,status,deleted_at) VALUES($1,$2,$3,$4,'Giám sát',$5,CASE WHEN $5='deleted' THEN now() END)`, staffID, id, tc.name, "staff-options-"+id+"@test.local", staffStatus)
		grant := tc.grant
		if grant == "multiple" {
			grant = "Giám sát"
			mustExec(t, db, `INSERT INTO user_roles(user_id,role_id,project_id) VALUES($1,$2,$3)`, id, supervisorRoleID, projectIDs[0])
		}
		mustExec(t, db, `INSERT INTO project_members(project_id,staff_id,role_ids,active) VALUES($1,$2,jsonb_build_array($3::text),$4)`, projectIDs[tc.project], staffID, grant, !tc.inactiveMember)
	}
	// Historical grants can remain after changing a project's sample flag. The
	// options must honor the current project boundary, not assignment-time scope.
	mustExec(t, db, `UPDATE projects SET is_sample=true WHERE id=$1`, projectIDs[1])
	for i, tc := range cases {
		if tc.sample && len(sampleUsers) == 0 {
			mustExec(t, db, `UPDATE users SET is_sample=true,sample_role_id=$2 WHERE id=$1`, caseIDs[i], supervisorRoleID)
		}
	}
	options, err := service.SupervisionOptions(ctx, deputyID)
	if err != nil {
		t.Fatal(err)
	}
	supervisors := recordSlice(options["supervisors"])
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := recordByID(supervisors, "id", caseIDs[i]) != nil; got != tc.want {
				t.Fatalf("included=%t, want=%t", got, tc.want)
			}
		})
	}
	// Compare the entire ordered result against the canonical report classifier,
	// so duplicate grants and membership aliases cannot drift from report scope.
	canonical, err := repo.Query(ctx, `SELECT u.id::text,u.full_name FROM users u WHERE u.status='active' AND u.deleted_at IS NULL AND NOT u.is_sample
		AND EXISTS (SELECT 1 FROM projects p WHERE p.status <> 'cancelled' AND NOT p.is_sample AND `+supervisorUserSQL("u.id", "p.id")+`) ORDER BY lower(u.full_name),u.id`)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(supervisors, canonical) {
		t.Fatalf("options differ from canonical ordered classification: got %d rows, want %d", len(supervisors), len(canonical))
	}
	projects := recordSlice(options["projects"])
	if recordByID(projects, "id", projectIDs[0]) == nil || recordByID(projects, "id", projectIDs[1]) != nil || recordByID(projects, "id", projectIDs[2]) != nil {
		t.Fatal("options leaked sample/cancelled projects or omitted the real project")
	}
	if _, err := service.SupervisionOptions(ctx, caseIDs[0]); err == nil {
		t.Fatal("Supervisor accessed company options")
	}
	// A global role alone is insufficient when no real active project exists.
	// This transaction runs only in the integration test DB and is rolled back.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET status='cancelled' WHERE status <> 'cancelled' AND NOT is_sample`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM (`+supervisionSupervisorOptionsSQL()+`) options`, deputyID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("Supervisor options survived without a real active project")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `DELETE FROM user_roles WHERE user_id=$1`, deputyID)
	if _, err := service.SupervisionOptions(ctx, deputyID); err == nil {
		t.Fatal("revoked Deputy accessed company options")
	}
	rows, err := repo.Query(ctx, supervisionSupervisorOptionsSQL(), deputyID)
	if err != nil || len(rows) != 0 {
		t.Fatalf("query exposed options to a revoked actor: rows=%d, err=%v", len(rows), err)
	}
}

func TestDeputyCompanyScopeReadDiscussionAndMutationBoundaries(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)
	deputyID := newID()
	mustExec(t, db, `INSERT INTO users(id,email,password_hash,full_name,status) VALUES($1,$2,'demo:pw','Deputy Fixture','active')`, deputyID, "deputy-"+deputyID+"@test.local")
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM work_item_activities WHERE actor_id=$1`, deputyID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, deputyID)
	})
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id) SELECT $1,id FROM roles WHERE name='deputy_manager'`, deputyID)
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id) SELECT $1,id FROM roles WHERE name='supervisor'`, fixture.workerID)
	mustExec(t, db, `UPDATE work_items SET assignee_staff_id=(SELECT id FROM staff WHERE user_id=$2) WHERE id=$1`, fixture.itemID, fixture.workerID)
	profile, err := repo.WorkItemAccessProfile(ctx, deputyID)
	if err != nil {
		t.Fatal(err)
	}
	if !profile.CanSuperviseAllProjects || profile.IsAdmin || profile.IsDirector || profile.IsManager || profile.CanReconcileAllProjects {
		t.Fatalf("deputy profile %#v", profile)
	}
	for _, key := range []string{"project.view", "pipeline.view", "planning.view", "operation.view", "work_report.review"} {
		allowed, err := repo.CanAccessProjectPermission(ctx, deputyID, fixture.projectID, []string{key})
		if err != nil || !allowed {
			t.Fatalf("missing %s scope: %v", key, err)
		}
	}
	for _, key := range []string{"project.update", "project.member.manage", "pipeline.run", "pipeline.advance_node", "planning.edit", "project_finance.view", "accounting.view", "operation.report"} {
		allowed, err := repo.CanAccessProjectPermission(ctx, deputyID, fixture.projectID, []string{key})
		if err != nil || allowed {
			t.Fatalf("unexpected %s scope: %v", key, err)
		}
	}
	canAct, err := repo.CanActOnWorkItem(ctx, fixture.itemID, deputyID)
	if err != nil || canAct {
		t.Fatalf("deputy executes work: %v", err)
	}
	detail, err := service.WorkItemDetail(ctx, fixture.itemID, deputyID)
	if err != nil {
		t.Fatal(err)
	}
	if boolValue(detail["can_act"]) || boolValue(detail["can_decide"]) {
		t.Fatal("oversight detail grants mutation")
	}
	if _, err = service.WorkItemAction(ctx, fixture.itemID, "start", WorkItemActionRequest{}, deputyID, "Deputy"); err == nil {
		t.Fatal("deputy starts Supervisor work")
	}
	queue, err := service.SupervisionWork(ctx, deputyID, SupervisionWorkFilter{ProjectID: fixture.projectID, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(recordSlice(queue["items"])) != 1 {
		t.Fatalf("Supervisor queue %#v", queue)
	}
	if _, err := service.SupervisionWork(ctx, fixture.workerID, SupervisionWorkFilter{}); err == nil {
		t.Fatal("worker accessed company queue")
	}
	options, err := service.SupervisionOptions(ctx, deputyID)
	if err != nil {
		t.Fatal(err)
	}
	if recordByID(recordSlice(options["supervisors"]), "id", fixture.workerID) == nil {
		t.Fatalf("Supervisor missing from options %#v", options)
	}
	activityOptions, err := service.ProjectActivityOptions(ctx, fixture.projectID, deputyID)
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(activityOptions["comment_only"]) || len(recordSlice(activityOptions["attachments"])) != 0 {
		t.Fatal("company deputy composer exposes task mutations")
	}
	if _, err := service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{ActivityType: "comment", Comment: "Theo dõi tiến độ", ScopeType: "work_item", ScopeID: fixture.itemID}, deputyID, "Deputy"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{ActivityType: "decision", Comment: "Duyệt công việc"}, deputyID, "Deputy"); err == nil {
		t.Fatal("company oversight allowed decision activity")
	}
	question, err := service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{ActivityType: "question", Comment: "Cần phản hồi tiến độ", Mentions: []ActivityMentionRequest{{Type: "user", ID: deputyID}}}, fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResolveActivity(ctx, cleanID(question["id"]), ActivityResolutionRequest{}, deputyID, "Deputy"); err == nil || !strings.Contains(err.Error(), "không xử lý trạng thái hoạt động") {
		t.Fatalf("mentioned Deputy resolved project question: %v", err)
	}
	if _, err := service.AddWorkItemActivity(ctx, fixture.itemID, WorkItemActivityRequest{ActivityType: "comment", Comment: "Trao đổi tiến độ"}, deputyID, "Deputy"); err != nil {
		t.Fatal(err)
	}
	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{EvidenceType: "photo", Name: "Supervisor evidence", FileURL: "https://example.com/supervision.jpg", Visibility: "project"}, fixture.workerID, "Supervisor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{ActivityType: "comment", Comment: "Attachment adapter", Metadata: Record{"evidence_id": cleanID(evidence["id"])}}, deputyID, "Deputy"); err == nil || !strings.Contains(err.Error(), "Minh chứng công việc phải do người được phân công cập nhật") {
		t.Fatalf("legacy metadata bypassed company comment attachment boundary: %v", err)
	}
	// A second unrelated project appears automatically; no membership is inserted.
	other := setupWorkItemFlowFixture(t, db, false)
	board, err := repo.ProjectProgressBoard(ctx, deputyID)
	if err != nil {
		t.Fatal(err)
	}
	if recordByID(board, "id", other.projectID) == nil {
		t.Fatal("new project requires manual membership")
	}
	// A financial role on the same actor does not turn supervision into accounting scope.
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id) SELECT $1,id FROM roles WHERE name='pm'`, deputyID)
	financeAllowed, err := repo.CanAccessProjectPermission(ctx, deputyID, other.projectID, []string{"project_finance.view"})
	if err != nil || financeAllowed {
		t.Fatalf("mixed roles widened finance scope: %v", err)
	}
	mustExec(t, db, `DELETE FROM user_roles WHERE user_id=$1 AND role_id IN(SELECT id FROM roles WHERE name='pm')`, deputyID)
	// Global observation excludes sample projects and disabled or scoped-only deputies.
	mustExec(t, db, `UPDATE projects SET is_sample=true WHERE id=$1`, other.projectID)
	canView, err := repo.CanViewWorkItem(ctx, other.itemID, deputyID)
	if err != nil || canView {
		t.Fatalf("sample work leaked: %v", err)
	}
	board, err = repo.ProjectProgressBoard(ctx, deputyID)
	if err != nil {
		t.Fatal(err)
	}
	if recordByID(board, "id", other.projectID) != nil {
		t.Fatal("sample progress leaked")
	}
	mustExec(t, db, `UPDATE users SET status='inactive' WHERE id=$1`, deputyID)
	if _, err := service.SupervisionWork(ctx, deputyID, SupervisionWorkFilter{}); err == nil {
		t.Fatal("inactive deputy accessed queue")
	}
	mustExec(t, db, `UPDATE users SET status='active' WHERE id=$1`, deputyID)
	mustExec(t, db, `UPDATE user_roles SET project_id=$2 WHERE user_id=$1`, deputyID, fixture.projectID)
	if _, err := service.SupervisionWork(ctx, deputyID, SupervisionWorkFilter{}); err == nil {
		t.Fatal("project-scoped deputy accessed company queue")
	}
	app := batch4TestApp(fixture.workerID, func(app *fiber.App, h *Handler) { app.Get("/me/supervision-work", h.SupervisionWork) }, NewHandler(service))
	response, err := app.Test(httptest.NewRequest("GET", "/me/supervision-work", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatalf("scope handler status %d", response.StatusCode)
	}
}
