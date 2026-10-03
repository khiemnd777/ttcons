package core

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestProjectManagerLabelCompatibility(t *testing.T) {
	aliases := []string{"PM", "pm", " Project Manager ", "Quản lý dự án"}
	for _, actor := range aliases {
		for _, required := range aliases {
			if !roleLabelsIntersect([]string{actor}, []string{required}) {
				t.Fatalf("PM aliases do not match: %q / %q", actor, required)
			}
		}
	}
	for _, label := range []string{"Phó Quản Lý", "deputy_manager", "Giám sát", "QS"} {
		if roleLabelsIntersect([]string{label}, aliases) {
			t.Fatalf("unrelated role %q became Project Manager", label)
		}
	}
}

func TestProjectManagerLegacySampleAndExportLabels(t *testing.T) {
	actor := sampleDataStaffActor{RoleID: "pm-role", RoleKey: "pm", RoleName: "Quản lý dự án", UserID: "pm-user", StaffID: "pm-staff"}
	blueprint := sampleDataBlueprint{
		Roles:  []sampleDataRoleAccount{{RoleID: actor.RoleID, RoleKey: actor.RoleKey, RoleName: actor.RoleName}, {RoleID: "deputy", RoleKey: "deputy_manager", RoleName: "Phó Quản Lý"}},
		Actors: map[string][]sampleDataStaffActor{actor.RoleName: {actor}},
	}
	catalog := workflowDefinitionCatalog{RoleByName: map[string]Record{"pm": {"name": "pm", "display_name": "Quản lý dự án"}}, RoleNameByDisplayName: map[string]string{"quản lý dự án": "pm"}}
	for _, label := range []string{"PM", "Project Manager", "Quản lý dự án"} {
		matches := sampleRoleMatches(blueprint, label)
		actors := sampleActorsForRoleLabel(blueprint, label)
		if len(matches) != 1 || matches[0].RoleKey != "pm" || len(actors) != 1 || actors[0].UserID != actor.UserID {
			t.Fatalf("legacy sample label %q did not resolve canonical PM", label)
		}
		exported := workflowDefinitionExportRoleNames([]string{label}, catalog)
		if len(exported) != 1 || exported[0] != "pm" {
			t.Fatalf("legacy export %q = %#v", label, exported)
		}
	}
	// Preserve the fail-closed ambiguous-label detection used by provisioning.
	blueprint.Roles = append(blueprint.Roles, sampleDataRoleAccount{RoleID: "custom", RoleKey: "custom", RoleName: "PM"})
	blueprint.Actors["PM"] = []sampleDataStaffActor{{RoleID: "custom", RoleKey: "custom", RoleName: "PM", UserID: "custom-user"}}
	if len(sampleRoleMatches(blueprint, "PM")) != 2 {
		t.Fatal("ambiguous role display name was silently accepted")
	}
	if len(sampleActorsForRoleLabel(blueprint, "PM")) != 0 || sampleActorsForRoleLabel(blueprint, "Quản lý dự án")[0].UserID != actor.UserID {
		t.Fatal("ambiguous/custom PM actor overrode canonical Project Manager")
	}
	catalog.RoleNameByDisplayName["pm"] = "custom"
	if exported := workflowDefinitionExportRoleNames([]string{"PM"}, catalog); exported[0] != "custom" {
		t.Fatal("legacy export overrode an exact current catalog mapping")
	}
	if len(sampleActorsForRoleLabel(blueprint, "Phó Quản Lý")) != 0 {
		t.Fatal("Deputy Manager resolved a PM sample actor")
	}
}

func TestProjectManagerLabelCompatibilityIntegration(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	fixture := setupProjectScopeFixture(t, db, []string{"operation.view"})
	var displayName string
	if err := db.QueryRowContext(ctx, `SELECT display_name FROM roles WHERE name='pm'`).Scan(&displayName); err != nil || displayName != "Quản lý dự án" {
		t.Fatalf("migrated PM display = %q, error = %v", displayName, err)
	}
	for _, label := range []string{"PM", "Project Manager", "Quản lý dự án"} {
		raw, _ := json.Marshal([]string{label})
		mustExec(t, db, `UPDATE project_members SET role_ids=$3::jsonb WHERE project_id=$1 AND staff_id=$2`, fixture.projectID, fixture.staffID, string(raw))
		manager, err := repo.UserIsProjectManager(ctx, fixture.userID, fixture.projectID)
		if err != nil || !manager {
			t.Fatalf("member %q PM identity = %v, error = %v", label, manager, err)
		}
		for _, workLabel := range []string{"PM", "Quản lý dự án"} {
			workRaw, _ := json.Marshal([]string{workLabel})
			query := `SELECT ` + myWorkRoleResponsibilitySQL("wi", 1, 2) + ` FROM (SELECT $3::uuid AS id,$4::uuid AS project_id,$5::jsonb AS role_ids) wi`
			var responsible bool
			if err := db.QueryRowContext(ctx, query, fixture.userID, fixture.staffID, newID(), fixture.projectID, string(workRaw)).Scan(&responsible); err != nil || !responsible {
				t.Fatalf("member %q / work %q responsibility = %v, error = %v", label, workLabel, responsible, err)
			}
			if err := db.QueryRowContext(ctx, query, fixture.userID, fixture.staffID, newID(), fixture.otherProjectID, string(workRaw)).Scan(&responsible); err != nil || responsible {
				t.Fatalf("PM alias crossed project boundary: %v, error = %v", responsible, err)
			}
		}
	}
	for _, label := range []string{"Phó Quản Lý", "deputy_manager", "QS", " QS "} {
		var key string
		if err := db.QueryRowContext(ctx, `SELECT consflow_role_label_key($1)`, label).Scan(&key); err != nil || key != label {
			t.Fatalf("SQL compatibility changed unrelated label %q to %q: %v", label, key, err)
		}
	}
	raw, _ := json.Marshal([]string{"Phó Quản Lý"})
	mustExec(t, db, `UPDATE project_members SET role_ids=$3::jsonb WHERE project_id=$1 AND staff_id=$2`, fixture.projectID, fixture.staffID, string(raw))
	manager, err := repo.UserIsProjectManager(ctx, fixture.userID, fixture.projectID)
	if err != nil || manager {
		t.Fatalf("Deputy member became PM: %v, %v", manager, err)
	}
	// Parse the shared queue's full SQL on the migrated database too.
	if _, err := repo.MyWorkItemsPage(ctx, fixture.userID, MyWorkItemsFilter{Queue: "responsible", Bucket: "all", Limit: 25}, 0); err != nil {
		t.Fatal(fmt.Errorf("migrated My Work queue: %w", err))
	}
	memberUserID, memberStaffID := addAllMentionTestMember(t, db, fixture.projectID, fixture.roleID)
	mustExec(t, db, `UPDATE project_members SET role_ids='["Quản lý dự án"]'::jsonb WHERE project_id=$1 AND staff_id=$2`, fixture.projectID, memberStaffID)
	service := NewService(repo, "pm-label-test-secret")
	activity, err := service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{
		Comment: "Project Manager label compatibility", Visibility: "project",
		Mentions: []ActivityMentionRequest{{Type: "role", ID: "PM", Label: "Quản lý dự án"}},
	}, fixture.userID, "Sender")
	if err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM activity_mentions WHERE activity_id=$1 AND recipient_user_id=$2`, 1, cleanID(activity["id"]), memberUserID)
	assertCount(t, db, `SELECT COUNT(*) FROM activity_mentions WHERE activity_id=$1 AND recipient_user_id=$2`, 0, cleanID(activity["id"]), fixture.userID)
	profile, err := repo.WorkItemAccessProfile(ctx, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	options, err := repo.ProjectActivityOptions(ctx, fixture.projectID, profile)
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range options["roles"].([]Record) {
		if normalizeRoleLabel(fmt.Sprint(option["id"])) == "pm" && option["label"] != "Quản lý dự án" {
			t.Fatalf("mention option retained ambiguous label: %#v", option)
		}
	}
}
