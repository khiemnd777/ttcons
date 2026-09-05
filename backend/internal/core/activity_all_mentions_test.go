package core

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestNormalizeAllActivityMention(t *testing.T) {
	mentions, err := normalizeActivityMentions([]ActivityMentionRequest{
		{Type: " All ", ID: "ALL", Label: "untrusted label"},
		{Type: "all", ID: "all"},
	})
	if err != nil || len(mentions) != 1 || mentions[0] != (ActivityMentionRequest{Type: "all", ID: "all", Label: "all"}) {
		t.Fatalf("normalized all = %#v, %v", mentions, err)
	}
	for _, id := range []string{"", "other-project", newID(), "allabc"} {
		_, err := normalizeActivityMentions([]ActivityMentionRequest{{Type: "all", ID: id}})
		assertValidationError(t, err)
	}
}

func addAllMentionTestMember(t *testing.T, db *sql.DB, projectID, roleID string) (string, string) {
	t.Helper()
	ctx := context.Background()
	userID, staffID := newID(), newID()
	mustExec(t, db, `INSERT INTO users(id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','All Mention Member','active')`, userID, "all-"+userID+"@test.local")
	mustExec(t, db, `INSERT INTO staff(id,user_id,full_name,email,status) VALUES ($1,$2,'All Mention Member',$3,'active')`, staffID, userID, "all-staff-"+staffID+"@test.local")
	mustExec(t, db, `INSERT INTO project_members(project_id,staff_id,role_ids,active) VALUES ($1,$2,'["Member"]',true)`, projectID, staffID)
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id,project_id) VALUES ($1,$2,$3)`, userID, roleID, projectID)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE staff_id=$1`, staffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, staffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})
	return userID, staffID
}

func TestAllActivityMentionsMembershipDeliveryAndReply(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	fixture := setupProjectScopeFixture(t, db, []string{"operation.view", "project.view"})
	service := NewService(NewRepository(db), "test-secret")
	memberID, memberStaffID := addAllMentionTestMember(t, db, fixture.projectID, fixture.roleID)
	outsideID, _ := addAllMentionTestMember(t, db, fixture.otherProjectID, fixture.roleID)
	inactiveMemberID, inactiveMemberStaffID := addAllMentionTestMember(t, db, fixture.projectID, fixture.roleID)
	inactiveUserID, _ := addAllMentionTestMember(t, db, fixture.projectID, fixture.roleID)
	inactiveStaffUserID, inactiveStaffID := addAllMentionTestMember(t, db, fixture.projectID, fixture.roleID)
	deletedUserID, _ := addAllMentionTestMember(t, db, fixture.projectID, fixture.roleID)
	externalID, _ := addAllMentionTestMember(t, db, fixture.projectID, fixture.roleID)
	mustExec(t, db, `UPDATE project_members SET active=false WHERE project_id=$1 AND staff_id=$2`, fixture.projectID, inactiveMemberStaffID)
	mustExec(t, db, `UPDATE users SET status='inactive' WHERE id=$1`, inactiveUserID)
	mustExec(t, db, `UPDATE staff SET status='inactive' WHERE id=$1`, inactiveStaffID)
	mustExec(t, db, `UPDATE users SET status='inactive',deleted_at=now() WHERE id=$1`, deletedUserID)
	externalRoleID := ensureRole(t, db, "all_external_"+newID(), "External")
	mustExec(t, db, `UPDATE roles SET external_only=true WHERE id=$1`, externalRoleID)
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id) VALUES ($1,$2)`, externalID, externalRoleID)
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, externalRoleID) })
	request := WorkItemActivityRequest{
		Comment: "@All kiểm tra hiện trường", Visibility: "internal",
		Mentions: []ActivityMentionRequest{{Type: "all", ID: "all"}, {Type: "staff", ID: memberStaffID}, {Type: "user", ID: memberID}},
	}
	activity, err := service.AddProjectActivity(ctx, fixture.projectID, request, fixture.userID, "Sender")
	if err != nil {
		t.Fatal(err)
	}
	activityID := cleanID(activity["id"])
	assertCount(t, db, `SELECT COUNT(DISTINCT recipient_user_id) FROM activity_mentions WHERE activity_id=$1`, 2, activityID)
	assertCount(t, db, `SELECT COUNT(*) FROM activity_mentions WHERE activity_id=$1 AND recipient_user_id=$2 AND mention_type='staff'`, 1, activityID, memberID)
	assertCount(t, db, `SELECT COUNT(*) FROM activity_read_states WHERE activity_id=$1 AND user_id=$2 AND read_at IS NOT NULL`, 1, activityID, fixture.userID)
	assertCount(t, db, `SELECT COUNT(*) FROM activity_read_states WHERE activity_id=$1 AND user_id=$2 AND read_at IS NULL`, 1, activityID, memberID)
	mustDispatchNotifications(t, service)
	mustDispatchNotifications(t, service)
	assertCount(t, db, `SELECT COUNT(*) FROM user_notifications WHERE source_id=$1 AND event_key='activity.mentioned'`, 1, activityID)
	assertCount(t, db, `SELECT COUNT(*) FROM user_notifications WHERE source_id=$1 AND recipient_user_id=$2`, 1, activityID, memberID)
	for _, excluded := range []string{fixture.userID, outsideID, inactiveMemberID, inactiveUserID, inactiveStaffUserID, deletedUserID, externalID} {
		assertCount(t, db, `SELECT COUNT(*) FROM user_notifications WHERE source_id=$1 AND recipient_user_id=$2`, 0, activityID, excluded)
	}
	// Membership is resolved anew on send; no frontend member list is accepted as the all audience.
	mustExec(t, db, `UPDATE project_members SET active=true WHERE project_id=$1 AND staff_id=$2`, fixture.projectID, inactiveMemberStaffID)
	reply, err := service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{
		Comment: "@all phản hồi", Visibility: "project", ParentActivityID: activityID,
		Mentions: []ActivityMentionRequest{{Type: "all", ID: "all"}},
	}, fixture.userID, "Sender")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(reply["visibility"]) != "internal" || cleanID(reply["parent_activity_id"]) != activityID {
		t.Fatalf("reply changed historical context: %#v", reply)
	}
	assertCount(t, db, `SELECT COUNT(DISTINCT recipient_user_id) FROM activity_mentions WHERE activity_id=$1`, 3, cleanID(reply["id"]))
	mustDispatchNotifications(t, service)
	assertCount(t, db, `SELECT COUNT(*) FROM user_notifications WHERE source_id=$1 AND recipient_user_id=$2`, 1, cleanID(reply["id"]), inactiveMemberID)
	_, err = service.UpdateActivityMessage(ctx, activityID, ActivityMessageUpdateRequest{Comment: "@all nội dung đã sửa"}, fixture.userID, "Sender")
	if err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(DISTINCT recipient_user_id) FROM activity_mentions WHERE activity_id=$1`, 2, activityID)
	mustDispatchNotifications(t, service)
	assertCount(t, db, `SELECT COUNT(*) FROM user_notifications WHERE source_id=$1`, 1, activityID)
	_, err = service.AddProjectActivity(ctx, fixture.otherProjectID, request, fixture.userID, "Sender")
	if err == nil {
		t.Fatal("all mention bypassed project mutation scope")
	}
}

func TestAllActivityMentionsLimitRollsBackWholeActivity(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	fixture := setupProjectScopeFixture(t, db, []string{"operation.view"})
	service := NewService(NewRepository(db), "test-secret")
	suffix := newID()
	// 500 recipients plus the actor is accepted; the actor is excluded from notification fan-out.
	mustExec(t, db, `
		WITH accounts AS (
			INSERT INTO users(email,password_hash,full_name,status)
			SELECT 'all-limit-' || $2 || '-' || n || '@test.local','demo:pw','Limit Member','active'
			FROM generate_series(1,500) n RETURNING id,email
		), people AS (
			INSERT INTO staff(user_id,full_name,email,status)
			SELECT id,'Limit Member',email,'active' FROM accounts RETURNING id
		)
		INSERT INTO project_members(project_id,staff_id,role_ids,active)
		SELECT $1::uuid,id,'["Member"]'::jsonb,true FROM people
	`, fixture.projectID, suffix)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE email LIKE $1`, "all-limit-"+suffix+"-%")
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE email LIKE $1`, "all-limit-"+suffix+"-%")
	})
	request := WorkItemActivityRequest{Comment: "@all boundary", Mentions: []ActivityMentionRequest{{Type: "all", ID: "all"}}}
	activity, err := service.AddProjectActivity(ctx, fixture.projectID, request, fixture.userID, "Sender")
	if err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(DISTINCT recipient_user_id) FROM activity_mentions WHERE activity_id=$1`, 501, cleanID(activity["id"]))
	addAllMentionTestMember(t, db, fixture.projectID, fixture.roleID)
	request.Comment = "@all over limit must roll back"
	_, err = service.AddProjectActivity(ctx, fixture.projectID, request, fixture.userID, "Sender")
	assertValidationError(t, err)
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected clear recipient limit error, got %v", err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE project_id=$1`, 1, fixture.projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM notification_events WHERE project_id=$1 AND event_key='activity.mentioned'`, 1, fixture.projectID)
	assertCount(t, db, `SELECT COUNT(DISTINCT activity_id) FROM activity_read_states WHERE project_id=$1`, 1, fixture.projectID)
}
