package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestStaffAccountImportPreviewExecuteIdempotencyAndInitialPasswordGate(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	suffix := strings.ReplaceAll(newID(), "-", "")[:10]
	actorID := newID()
	actorEmail := "staff-import-actor-" + suffix + "@test.local"
	loginEmail := "staff-import-user-" + suffix + "@test.local"
	recoveryEmail := "staff-import-recovery-" + suffix + "@example.com"
	initialPassword := "Init-123!"
	newPassword := "Changed-456!"
	idempotencyKey := newID()

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE actor_id=$1 OR target_id IN (SELECT id FROM users WHERE email=$2)`, actorID, loginEmail)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff_account_imports WHERE actor_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id IN (SELECT id FROM users WHERE email IN ($1,$2))`, actorEmail, loginEmail)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE email IN ($1,$2))`, actorEmail, loginEmail)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE email IN ($1,$2)`, actorEmail, loginEmail)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE email IN ($1,$2)`, actorEmail, loginEmail)
	})

	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:actor-password','Import Actor','active')`, actorID, actorEmail)

	request := StaffAccountImportPreviewRequest{
		SourceFilename:  "accounts.xlsx",
		SourceSizeBytes: 2048,
		Rows: []StaffAccountImportRowRequest{{
			RowNumber: 2, FullName: "  Nhân viên Import  ", LoginEmail: strings.ToUpper(loginEmail),
			Password: initialPassword, RecoveryEmail: recoveryEmail, Phone: "0901 234 567",
			Department: "  Dự án ", Position: "  Kỹ sư QS ", RoleCode: "QS",
		}},
	}
	preview, err := service.PreviewStaffAccountImport(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if number(preview["valid_count"]) != 1 || number(preview["invalid_count"]) != 0 || fmt.Sprint(preview["preview_hash"]) == "" {
		t.Fatalf("unexpected preview: %#v", preview)
	}
	serializedPreview, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serializedPreview), initialPassword) || strings.Contains(string(serializedPreview), recoveryEmail) {
		t.Fatalf("preview leaked a secret value: %s", serializedPreview)
	}
	if !strings.Contains(string(serializedPreview), `"errors":[]`) || !strings.Contains(string(serializedPreview), `"warnings":[]`) {
		t.Fatalf("preview must expose empty issue arrays, got %s", serializedPreview)
	}

	executeRequest := StaffAccountImportExecuteRequest{
		StaffAccountImportPreviewRequest: request,
		PreviewHash:                      fmt.Sprint(preview["preview_hash"]),
		IdempotencyKey:                   idempotencyKey,
	}
	result, err := service.ExecuteStaffAccountImport(ctx, actorID, actorEmail, executeRequest)
	if err != nil {
		t.Fatal(err)
	}
	if number(result["created_count"]) != 1 || number(result["activated_count"]) != 1 || number(result["must_change_password_count"]) != 1 {
		t.Fatalf("unexpected execute result: %#v", result)
	}

	var userID, staffID, roleCode, userStatus, staffStatus, storedRecovery string
	var mustChange bool
	var department, position *string
	if err := db.QueryRowContext(ctx, `
		SELECT account.id::text,person.id::text,role.name,account.status,person.status,
			account.must_change_password,account.recovery_email,person.department,person.position
		FROM users account
		JOIN staff person ON person.user_id=account.id
		JOIN user_roles assignment ON assignment.user_id=account.id AND assignment.project_id IS NULL
		JOIN roles role ON role.id=assignment.role_id
		WHERE account.email=$1
	`, loginEmail).Scan(&userID, &staffID, &roleCode, &userStatus, &staffStatus, &mustChange, &storedRecovery, &department, &position); err != nil {
		t.Fatal(err)
	}
	if roleCode != "qs" || userStatus != "active" || staffStatus != "active" || !mustChange || storedRecovery != recoveryEmail {
		t.Fatalf("unexpected persisted import role=%q user=%q staff=%q must_change=%t recovery=%q", roleCode, userStatus, staffStatus, mustChange, storedRecovery)
	}
	if department == nil || *department != "Dự án" || position == nil || *position != "Kỹ sư QS" {
		t.Fatalf("unexpected optional profile values department=%v position=%v", department, position)
	}

	replay, err := service.ExecuteStaffAccountImport(ctx, actorID, actorEmail, executeRequest)
	if err != nil {
		t.Fatal(err)
	}
	if replay["idempotent_replay"] != true || number(replay["created_count"]) != 1 {
		t.Fatalf("unexpected idempotent replay: %#v", replay)
	}
	if _, err := service.ExecuteStaffAccountImport(ctx, newID(), "Another Import Actor", executeRequest); err == nil {
		t.Fatal("idempotent replay was accepted from a different actor")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM users WHERE email=$1`, 1, loginEmail)
	assertCount(t, db, `SELECT COUNT(*) FROM staff WHERE email=$1`, 1, loginEmail)
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1 AND project_id IS NULL`, 1, userID)

	if _, err := service.ChangeOwnPassword(ctx, userID, loginEmail, ChangeOwnPasswordRequest{
		CurrentPassword: initialPassword, NewPassword: newPassword, NewPasswordConfirmation: newPassword,
	}); err != nil {
		t.Fatal(err)
	}
	required, err := repo.InitialPasswordChangeRequired(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if required {
		t.Fatal("password change did not clear must_change_password")
	}

	var auditText string
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(string_agg(metadata_json::text,' '),'') FROM audit_logs WHERE actor_id=$1 OR target_id IN ($2,$3)`, actorID, userID, staffID).Scan(&auditText); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(auditText, initialPassword) || strings.Contains(auditText, recoveryEmail) {
		t.Fatalf("import secret leaked into audit metadata: %s", auditText)
	}
}

func TestStaffAccountImportPreviewRejectsDuplicateAndRestrictedRole(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	service := NewService(NewRepository(db), "test-secret")
	assertCount(t, db, `
		SELECT COUNT(*) FROM role_permissions assignment
		JOIN roles role ON role.id=assignment.role_id
		JOIN permissions permission ON permission.id=assignment.permission_id
		WHERE role.name='developer' AND permission.key='staff.import'
	`, 1)
	assertCount(t, db, `
		SELECT COUNT(*) FROM role_permissions assignment
		JOIN roles role ON role.id=assignment.role_id
		JOIN permissions permission ON permission.id=assignment.permission_id
		WHERE role.name='admin' AND permission.key='staff.import'
	`, 0)
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO role_permissions (role_id,permission_id)
		SELECT role.id,permission.id FROM roles role CROSS JOIN permissions permission
		WHERE role.name='admin' AND permission.key='staff.import'
	`); err == nil {
		t.Fatal("database trigger allowed staff.import to be assigned to Admin")
	}
	email := "duplicate-" + strings.ReplaceAll(newID(), "-", "")[:10] + "@test.local"
	preview, err := service.PreviewStaffAccountImport(context.Background(), StaffAccountImportPreviewRequest{
		SourceFilename: "accounts.csv", SourceSizeBytes: 300,
		Rows: []StaffAccountImportRowRequest{
			{RowNumber: 2, FullName: "Duplicate One", LoginEmail: email, Password: "Password-1!", RoleCode: "admin"},
			{RowNumber: 3, FullName: "Duplicate Two", LoginEmail: strings.ToUpper(email), Password: "Password-2!", RoleCode: "qs"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if number(preview["valid_count"]) != 0 || number(preview["invalid_count"]) != 2 || fmt.Sprint(preview["preview_hash"]) != "" {
		t.Fatalf("restricted/duplicate rows unexpectedly passed: %#v", preview)
	}
}
