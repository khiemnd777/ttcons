package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestProjectCreationCreatesStartupOwnerActionAndBlocksPrematurePipeline(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	suffix := newID()
	actorID := newID()
	actorEmail := "project-startup-" + suffix + "@test.local"
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:test','Startup Owner','active')`, actorID, actorEmail)

	var templateID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM pipeline_templates WHERE status='published' ORDER BY revision_no DESC,published_at DESC NULLS LAST,created_at DESC LIMIT 1`).Scan(&templateID); err != nil {
		t.Skip("a published pipeline template is required")
	}
	project, err := service.CreateProject(ctx, Record{
		"name": "Startup Coordination " + suffix[:8], "code": "START-" + suffix[:8],
		"client_name": "Test Client", "pipeline_template_id": templateID,
	}, actorID, actorEmail)
	if err != nil {
		t.Fatal(err)
	}
	projectID := cleanID(project["id"])
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM audit_logs WHERE project_id=$1`, projectID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id=$1`, actorID)
	})

	if cleanID(project["created_by"]) != actorID {
		t.Fatalf("project created_by = %v, want %s", project["created_by"], actorID)
	}
	var eventCount, auditCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_events WHERE event_key='project.setup_required' AND project_id=$1 AND actor_user_id=$2`, projectID, actorID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs WHERE event_type='project_created' AND project_id=$1 AND actor_id=$2`, projectID, actorID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 || auditCount != 1 {
		t.Fatalf("startup transaction event=%d audit=%d, want 1/1", eventCount, auditCount)
	}
	var setupEventID string
	var setupOutboxID int64
	if err := db.QueryRowContext(ctx, `
		SELECT event.id::text,outbox.id FROM notification_events event
		JOIN notification_outbox outbox ON outbox.event_id=event.id
		WHERE event.event_key='project.setup_required' AND event.project_id=$1
	`, projectID).Scan(&setupEventID, &setupOutboxID); err != nil {
		t.Fatal(err)
	}
	recipients, _, err := service.projectNotificationEvent(ctx, setupOutboxID, setupEventID)
	if err != nil {
		t.Fatal(err)
	}
	if len(recipients) != 1 || recipients[0] != actorID {
		t.Fatalf("setup notification recipients = %#v, want creator %s", recipients, actorID)
	}
	summary, err := repo.UserNotificationSummary(ctx, actorID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ActionRequired != 1 {
		t.Fatalf("creator notification summary = %#v, want one action", summary)
	}

	actions, err := service.MyProjectStartupActions(ctx, actorID, projectID, "")
	if err != nil {
		t.Fatal(err)
	}
	items := startupRecordSlice(actions["items"])
	if len(items) != 1 || fmt.Sprint(items[0]["action_kind"]) != "setup_required" {
		t.Fatalf("startup actions = %#v, want one setup_required", actions)
	}
	readiness, err := projectStartupReadiness(ctx, db, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if boolValue(readiness["ready_to_start"]) {
		t.Fatalf("readiness = %#v, want missing PM blocker", readiness)
	}
	if _, err := repo.StartPipeline(ctx, projectID, templateID, actorID, actorEmail); err == nil {
		t.Fatal("expected startup gate to reject pipeline without PM and first-step coverage")
	} else {
		var statusErr StatusError
		if !errors.As(err, &statusErr) || statusErr.Code != "VALIDATION_ERROR" {
			t.Fatalf("start pipeline error = %T %v, want VALIDATION_ERROR", err, err)
		}
	}
}

func TestProjectStartupHandoverStartsPipelineAndCreatesRealWorkNotifications(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	suffix := newID()
	actorID, actorStaffID := newID(), newID()
	pmUserID, pmStaffID := newID(), newID()
	roleID := ensureRole(t, db, "startup_flow_"+suffix[:8], "Startup Flow "+suffix[:8])
	actorEmail := "startup-actor-" + suffix + "@test.local"
	pmEmail := "startup-pm-" + suffix + "@test.local"
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:test','Startup Actor','active'),($3,$4,'demo:test','Startup PM','active')`, actorID, actorEmail, pmUserID, pmEmail)
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,department,position,status) VALUES ($1,$2,'Startup Actor',$3,'Dự án','Điều phối','active'),($4,$5,'Startup PM',$6,'Dự án','PM','active')`, actorStaffID, actorID, actorEmail, pmStaffID, pmUserID, pmEmail)
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) VALUES ($1,$3),($2,$3)`, actorID, pmUserID, roleID)

	var templateID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM pipeline_templates WHERE status='published' ORDER BY revision_no DESC,published_at DESC NULLS LAST,created_at DESC LIMIT 1`).Scan(&templateID); err != nil {
		t.Skip("a published pipeline template is required")
	}
	project, err := service.CreateProject(ctx, Record{
		"name": "Startup Ready " + suffix[:8], "code": "READY-" + suffix[:8],
		"client_name": "Test Client", "pipeline_template_id": templateID,
	}, actorID, actorEmail)
	if err != nil {
		t.Fatal(err)
	}
	projectID := cleanID(project["id"])
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM audit_logs WHERE project_id=$1`, projectID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM user_roles WHERE user_id IN ($1,$2)`, actorID, pmUserID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM role_permissions WHERE role_id=$1`, roleID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM roles WHERE id=$1`, roleID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM staff WHERE id IN ($1,$2)`, actorStaffID, pmStaffID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id IN ($1,$2)`, actorID, pmUserID)
	})

	readiness, err := projectStartupReadiness(ctx, db, projectID)
	if err != nil {
		t.Fatal(err)
	}
	roleLabels := []string{"PM"}
	permissionKeys := []string{"project.view", "project.member.manage", "pipeline.run"}
	for _, step := range startupRecordSlice(readiness["first_steps"]) {
		roleLabels = append(roleLabels, stringSlice(step["role_ids"])...)
		if permission := strings.TrimSpace(stringValue(step["required_permission"])); permission != "" {
			permissionKeys = append(permissionKeys, permission)
		}
	}
	roleLabels = normalizedRoleList(roleLabels)
	permissionKeys = normalizedRoleList(permissionKeys)
	for _, permissionKey := range permissionKeys {
		permissionID := ensurePermission(t, db, permissionKey, permissionKey, "project_startup_test")
		mustExec(t, db, `INSERT INTO role_permissions (role_id,permission_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, roleID, permissionID)
	}
	if _, err := service.SaveProjectMember(ctx, projectID, "", ProjectMemberMutationRequest{
		StaffID: pmStaffID, RoleIDs: roleLabels, ProjectRoleIDs: []string{roleID},
	}, actorID, actorEmail); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetProjectManager(ctx, projectID, pmStaffID, actorID, actorEmail); err != nil {
		t.Fatal(err)
	}
	readiness, err = service.ProjectStartupReadiness(ctx, projectID, actorID)
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(readiness["ready_to_start"]) {
		t.Fatalf("readiness = %#v, want ready", readiness)
	}
	if _, err := service.StartPipeline(ctx, projectID, templateID, actorID, actorEmail); err != nil {
		t.Fatal(err)
	}
	var readyItems, assignedEvents, completedEvents int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_items WHERE project_id=$1 AND status='ready'`, projectID).Scan(&readyItems); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_events WHERE project_id=$1 AND event_key='work_item.assigned'`, projectID).Scan(&assignedEvents); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_events WHERE project_id=$1 AND event_key='project.setup_completed'`, projectID).Scan(&completedEvents); err != nil {
		t.Fatal(err)
	}
	if readyItems == 0 || assignedEvents != readyItems || completedEvents != 1 {
		t.Fatalf("ready items=%d assigned events=%d completed events=%d", readyItems, assignedEvents, completedEvents)
	}
	var assignedEventID string
	var assignedOutboxID int64
	if err := db.QueryRowContext(ctx, `
		SELECT event.id::text,outbox.id
		FROM notification_events event
		JOIN notification_outbox outbox ON outbox.event_id=event.id
		WHERE event.project_id=$1 AND event.event_key='work_item.assigned'
		ORDER BY event.created_at,event.id LIMIT 1
	`, projectID).Scan(&assignedEventID, &assignedOutboxID); err != nil {
		t.Fatal(err)
	}
	recipients, _, err := service.projectNotificationEvent(ctx, assignedOutboxID, assignedEventID)
	if err != nil {
		t.Fatal(err)
	}
	if len(recipients) != 1 || recipients[0] != pmUserID {
		t.Fatalf("work item notification recipients = %#v, want PM %s", recipients, pmUserID)
	}
	notifications, err := repo.UserNotifications(ctx, pmUserID, NotificationFilter{State: "all", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	foundAssignedNotification := false
	for _, notification := range notifications.Items {
		if fmt.Sprint(notification["event_key"]) == "work_item.assigned" {
			foundAssignedNotification = true
			break
		}
	}
	if !foundAssignedNotification {
		t.Fatalf("PM notifications = %#v, want actionable work item notification", notifications.Items)
	}
	actions, err := service.MyProjectStartupActions(ctx, pmUserID, projectID, "")
	if err != nil {
		t.Fatal(err)
	}
	if number(actions["total"]) != 0 {
		t.Fatalf("startup actions after start = %#v, want empty", actions)
	}
}
