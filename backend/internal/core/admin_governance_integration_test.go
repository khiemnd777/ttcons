package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"consflow/backend/internal/shared/middleware"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

func TestCurrentUserRolesDeduplicateDisplayNamesAcrossProjectScopes(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	suffix := newID()
	userID := newID()
	roleID := newID()
	firstProjectID := newID()
	secondProjectID := newID()
	roleDisplayName := "Scoped PM " + suffix[:8]

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id IN ($1,$2)`, firstProjectID, secondProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, roleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})

	mustExec(t, db, `
		INSERT INTO users (id,email,password_hash,full_name,status)
		VALUES ($1,$2,'demo:test','Scoped Role User','active')
	`, userID, "scoped-role-user-"+suffix[:8]+"@test.local")
	mustExec(t, db, `
		INSERT INTO roles (id,name,display_name,system_role)
		VALUES ($1,$2,$3,false)
	`, roleID, "scoped_role_"+suffix[:8], roleDisplayName)
	mustExec(t, db, `
		INSERT INTO projects (id,name,code,client_name,status) VALUES
			($1,'Scoped Role Project One',$2,'Client','planning'),
			($3,'Scoped Role Project Two',$4,'Client','planning')
	`, firstProjectID, "SR1-"+suffix[:8], secondProjectID, "SR2-"+suffix[:8])
	mustExec(t, db, `
		INSERT INTO user_roles (user_id,role_id,project_id) VALUES
			($1,$2,$3),
			($1,$2,$4)
	`, userID, roleID, firstProjectID, secondProjectID)

	roles, err := repo.UserRoles(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0] != roleDisplayName {
		t.Fatalf("repository roles=%#v, want one %q display name", roles, roleDisplayName)
	}

	profile, err := service.Me(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	profileRoles, ok := profile["roles"].([]string)
	if !ok || len(profileRoles) != 1 || profileRoles[0] != roleDisplayName {
		t.Fatalf("current-user roles=%#v, want one %q display name", profile["roles"], roleDisplayName)
	}
}

func TestPartnerCatalogGroupFilters(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	suffix := newID()
	clientID := newID()
	supplierID := newID()
	mixedID := newID()
	inactiveOutputID := newID()
	partnerIDs := []string{clientID, supplierID, mixedID, inactiveOutputID}

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM partners WHERE id = ANY($1::uuid[])`, partnerIDs)
	})

	mustExec(t, db, `
		INSERT INTO partners (id,partner_code,name,partner_types,active) VALUES
			($1,$2,'Input Client','["client"]'::jsonb,true),
			($3,$4,'Output Supplier','["supplier"]'::jsonb,true),
			($5,$6,'Mixed Partner','["client","supplier"]'::jsonb,true),
			($7,$8,'Inactive Output','["service_provider"]'::jsonb,false)
	`, clientID, "INPUT-"+suffix[:8], supplierID, "OUTPUT-"+suffix[:8], mixedID, "MIXED-"+suffix[:8], inactiveOutputID, "INACTIVE-"+suffix[:8])

	inputRows, err := service.ListPartners(ctx, PartnerFilter{Group: "input", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if recordByID(inputRows, "id", clientID) == nil || recordByID(inputRows, "id", mixedID) == nil {
		t.Fatalf("input group must contain client and mixed partners: %#v", inputRows)
	}
	if recordByID(inputRows, "id", supplierID) != nil || recordByID(inputRows, "id", inactiveOutputID) != nil {
		t.Fatalf("input group returned an output-only or inactive partner: %#v", inputRows)
	}

	outputRows, err := service.ListPartners(ctx, PartnerFilter{Group: "output", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if recordByID(outputRows, "id", supplierID) == nil || recordByID(outputRows, "id", mixedID) == nil {
		t.Fatalf("output group must contain supplier and mixed partners: %#v", outputRows)
	}
	if recordByID(outputRows, "id", clientID) != nil || recordByID(outputRows, "id", inactiveOutputID) != nil {
		t.Fatalf("output group returned a client-only or inactive partner: %#v", outputRows)
	}

	supplierRows, err := service.ListPartners(ctx, PartnerFilter{Group: "output", Type: "supplier", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if recordByID(supplierRows, "id", supplierID) == nil || recordByID(supplierRows, "id", mixedID) == nil {
		t.Fatalf("output supplier filter must retain supplier and mixed partners: %#v", supplierRows)
	}

	inactiveRows, err := service.ListPartners(ctx, PartnerFilter{Group: "output", Status: "inactive"})
	if err != nil {
		t.Fatal(err)
	}
	if recordByID(inactiveRows, "id", inactiveOutputID) == nil || recordByID(inactiveRows, "id", supplierID) != nil {
		t.Fatalf("inactive output filter returned an unexpected set: %#v", inactiveRows)
	}

	_, err = service.ListPartners(ctx, PartnerFilter{Group: "unknown"})
	assertValidationError(t, err)
	_, err = service.ListPartners(ctx, PartnerFilter{Group: "input", Type: "supplier"})
	assertValidationError(t, err)
	_, err = service.ListPartners(ctx, PartnerFilter{Group: "output", Type: "client"})
	assertValidationError(t, err)
}

func TestAdminStaffAccountSecurityManagement(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	suffix := newID()
	actorID := newID()
	targetUserID := newID()
	targetStaffID := newID()
	actorStaffID := newID()
	unlinkedStaffID := newID()
	limitedRoleID := ensureRole(t, db, "staff_security_limited_"+suffix[:8], "Staff Security Limited")
	staffViewPermissionID := ensurePermission(t, db, "staff.view", "Xem nhân sự", "staff")
	staffCreatePermissionID := ensurePermission(t, db, "staff.create", "Tạo nhân sự", "staff")
	staffUpdatePermissionID := ensurePermission(t, db, "staff.update", "Sửa nhân sự", "staff")
	staffSecurityPermissionID := ensurePermission(t, db, "staff.account_security.manage", "Quản trị bảo mật tài khoản nhân sự", "staff")
	actorEmail := "staff-security-actor-" + suffix[:8] + "@test.local"
	targetEmail := "staff-security-target-" + suffix[:8] + "@test.local"
	recoveryEmail := "staff-security-recovery-" + suffix[:8] + "@example.com"
	createdEmail := "staff-created-" + suffix[:8] + "@test.local"
	createdRecoveryEmail := "staff-created-recovery-" + suffix[:8] + "@example.com"
	createdPassword := "Created-Staff-123!"
	newPassword := "Admin-Reset-123!"

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE actor_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth_email_tokens WHERE user_id IN ($1,$2)`, actorID, targetUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id IN ($1,$2)`, actorID, targetUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE email=$1`, createdEmail)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id IN ($1,$2,$3)`, targetStaffID, actorStaffID, unlinkedStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE email=$1`, createdEmail)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, actorID, targetUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, limitedRoleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, limitedRoleID)
	})

	mustExec(t, db, `
		INSERT INTO role_permissions (role_id,permission_id) VALUES ($1,$2),($1,$3),($1,$4)
		ON CONFLICT DO NOTHING
	`, limitedRoleID, staffViewPermissionID, staffUpdatePermissionID, staffCreatePermissionID)
	mustExec(t, db, `
		INSERT INTO users (id,email,password_hash,full_name,status) VALUES
			($1,$2,'demo:actor-password','Security Actor','active'),
			($3,$4,'demo:target-password','Security Target','inactive')
	`, actorID, actorEmail, targetUserID, targetEmail)
	mustExec(t, db, `
		INSERT INTO staff (id,user_id,full_name,email,department,position,status) VALUES
			($1,$2,'Security Target',$3,'Dự án','Nhân viên','inactive'),
			($4,$5,'Security Actor',$6,'Công nghệ','Admin','active'),
			($7,NULL,'Unlinked Staff',$8,'Dự án','Nhân viên','active')
	`, targetStaffID, targetUserID, targetEmail, actorStaffID, actorID, actorEmail, unlinkedStaffID, "unlinked-"+suffix[:8]+"@test.local")
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) VALUES ($1,$2)`, actorID, limitedRoleID)
	mustExec(t, db, `
		INSERT INTO auth_sessions (user_id,refresh_token_hash,access_token_id,expires_at)
		VALUES ($1,$2,$3,now()+interval '1 day')
	`, targetUserID, strings.Repeat("a", 56)+suffix[:8], "target-session-"+suffix)
	mustExec(t, db, `
		INSERT INTO auth_email_tokens (user_id,purpose,token_hash,target_email,expires_at)
		VALUES ($1,'reset_password',$2,$3,now()+interval '15 minutes')
	`, targetUserID, strings.Repeat("b", 56)+suffix[:8], "old-recovery-"+suffix[:8]+"@example.com")

	handler := NewHandler(service)
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", actorID)
		c.Locals("email", actorEmail)
		return c.Next()
	})
	app.Get("/staff", handler.ListStaff)
	app.Post("/staff", handler.CreateStaff)
	app.Put("/staff/:id", handler.UpdateStaff)
	createBody := fmt.Sprintf(`{"full_name":"Created Staff","email":"%s","phone":"0901234567","department":"   ","position":"","status":"active","password":"%s","recovery_email":"%s"}`, createdEmail, createdPassword, createdRecoveryEmail)
	deniedCreateRequest := httptest.NewRequest(http.MethodPost, "/staff", strings.NewReader(createBody))
	deniedCreateRequest.Header.Set("Content-Type", "application/json")
	deniedCreateResponse, err := app.Test(deniedCreateRequest)
	if err != nil {
		t.Fatal(err)
	}
	if deniedCreateResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("staff create status=%d, want %d without staff.account_security.manage", deniedCreateResponse.StatusCode, http.StatusForbidden)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM users WHERE email=$1`, 0, createdEmail)
	assertCount(t, db, `SELECT COUNT(*) FROM staff WHERE email=$1`, 0, createdEmail)

	deniedRequest := httptest.NewRequest(http.MethodPut, "/staff/"+targetStaffID, strings.NewReader(`{"new_password":"Denied-Reset-123!"}`))
	deniedRequest.Header.Set("Content-Type", "application/json")
	deniedResponse, err := app.Test(deniedRequest)
	if err != nil {
		t.Fatal(err)
	}
	if deniedResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("sensitive staff update status=%d, want %d without staff.account_security.manage", deniedResponse.StatusCode, http.StatusForbidden)
	}

	fullName := "Security Target Updated"
	blankOrganizationValue := "   "
	result, err := service.UpdateStaff(ctx, targetStaffID, actorID, actorEmail, UpdateStaffRequest{
		FullName:                &fullName,
		Department:              &blankOrganizationValue,
		Position:                &blankOrganizationValue,
		NewPassword:             newPassword,
		NewPasswordConfirmation: newPassword,
		RecoveryEmail:           recoveryEmail,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result["password_changed"] != true || result["recovery_email_changed"] != true || result["reauth_required"] != false || result["has_recovery_email"] != true {
		t.Fatalf("unexpected security update response: %#v", result)
	}

	var passwordHash, storedRecoveryEmail, targetStatus, targetStaffStatus, verifiedAt string
	var targetDepartmentMissing, targetPositionMissing bool
	if err := db.QueryRowContext(ctx, `
		SELECT account.password_hash,account.recovery_email,account.status,person.status,account.recovery_email_verified_at::text,
			person.department IS NULL,person.position IS NULL
		FROM users account
		JOIN staff person ON person.user_id=account.id
		WHERE account.id=$1 AND person.id=$2
	`, targetUserID, targetStaffID).Scan(&passwordHash, &storedRecoveryEmail, &targetStatus, &targetStaffStatus, &verifiedAt, &targetDepartmentMissing, &targetPositionMissing); err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(newPassword)) != nil {
		t.Fatal("admin password reset was not stored as a bcrypt hash")
	}
	if storedRecoveryEmail != recoveryEmail || verifiedAt == "" {
		t.Fatalf("recovery email state=%q verified_at=%q", storedRecoveryEmail, verifiedAt)
	}
	if targetStatus != "inactive" || targetStaffStatus != "inactive" {
		t.Fatalf("security update changed inactive state: user=%q staff=%q", targetStatus, targetStaffStatus)
	}
	if !targetDepartmentMissing || !targetPositionMissing {
		t.Fatalf("blank Staff organization values were not cleared to NULL: department_missing=%t position_missing=%t", targetDepartmentMissing, targetPositionMissing)
	}

	readStaff := func() []Record {
		t.Helper()
		listResponse, err := app.Test(httptest.NewRequest(http.MethodGet, "/staff", nil))
		if err != nil {
			t.Fatal(err)
		}
		if listResponse.StatusCode != http.StatusOK {
			t.Fatalf("list staff status=%d, want %d", listResponse.StatusCode, http.StatusOK)
		}
		var body struct {
			Data []Record `json:"data"`
		}
		if err := json.NewDecoder(listResponse.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		return body.Data
	}
	findTargetStaff := func(rows []Record) Record {
		t.Helper()
		for _, row := range rows {
			if fmt.Sprint(row["id"]) == targetStaffID {
				return row
			}
		}
		t.Fatalf("target staff %s missing from list", targetStaffID)
		return nil
	}

	limitedRow := findTargetStaff(readStaff())
	if limitedRow["has_recovery_email"] != true {
		t.Fatalf("limited staff list recovery state=%#v, want true", limitedRow["has_recovery_email"])
	}
	if _, exposed := limitedRow["recovery_email_masked"]; exposed {
		t.Fatal("masked recovery email exposed without staff.account_security.manage")
	}

	mustExec(t, db, `INSERT INTO role_permissions (role_id,permission_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, limitedRoleID, staffSecurityPermissionID)
	createRequest := httptest.NewRequest(http.MethodPost, "/staff", strings.NewReader(createBody))
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse, err := app.Test(createRequest)
	if err != nil {
		t.Fatal(err)
	}
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("staff create status=%d, want %d", createResponse.StatusCode, http.StatusCreated)
	}
	var createdUserID, createdStaffID, createdPasswordHash, storedCreatedRecovery, createdVerifiedAt string
	var createdDepartmentMissing, createdPositionMissing bool
	if err := db.QueryRowContext(ctx, `
		SELECT account.id::text,person.id::text,account.password_hash,account.recovery_email,account.recovery_email_verified_at::text,
			person.department IS NULL,person.position IS NULL
		FROM users account JOIN staff person ON person.user_id=account.id
		WHERE account.email=$1 AND person.email=$1 AND account.full_name=person.full_name AND account.status=person.status
	`, createdEmail).Scan(&createdUserID, &createdStaffID, &createdPasswordHash, &storedCreatedRecovery, &createdVerifiedAt, &createdDepartmentMissing, &createdPositionMissing); err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(createdPasswordHash), []byte(createdPassword)) != nil {
		t.Fatal("created staff password was not stored as bcrypt")
	}
	if storedCreatedRecovery != createdRecoveryEmail || createdVerifiedAt == "" {
		t.Fatalf("created recovery email=%q verified_at=%q", storedCreatedRecovery, createdVerifiedAt)
	}
	if !createdDepartmentMissing || !createdPositionMissing {
		t.Fatalf("created blank Staff organization values were not stored as NULL: department_missing=%t position_missing=%t", createdDepartmentMissing, createdPositionMissing)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1`, 0, createdUserID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE actor_id=$1 AND target_id IN ($2,$3) AND event_type IN ('user_account_created_by_admin','staff_created_with_account')`, 2, actorID, createdUserID, createdStaffID)
	duplicateCreateRequest := httptest.NewRequest(http.MethodPost, "/staff", strings.NewReader(createBody))
	duplicateCreateRequest.Header.Set("Content-Type", "application/json")
	duplicateCreateResponse, err := app.Test(duplicateCreateRequest)
	if err != nil {
		t.Fatal(err)
	}
	if duplicateCreateResponse.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate staff create status=%d, want %d", duplicateCreateResponse.StatusCode, http.StatusConflict)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM users WHERE email=$1`, 1, createdEmail)
	assertCount(t, db, `SELECT COUNT(*) FROM staff WHERE email=$1`, 1, createdEmail)

	privilegedRow := findTargetStaff(readStaff())
	if got, want := fmt.Sprint(privilegedRow["recovery_email_masked"]), "sta***@e**.com"; got != want {
		t.Fatalf("masked recovery email=%q, want %q", got, want)
	}
	if _, exposed := privilegedRow["recovery_email_private"]; exposed {
		t.Fatal("private recovery email leaked through staff list")
	}
	if _, exposed := privilegedRow["recovery_email"]; exposed {
		t.Fatal("full recovery email leaked through staff list")
	}

	assertCount(t, db, `SELECT COUNT(*) FROM auth_sessions WHERE user_id=$1 AND revoked_at IS NOT NULL`, 1, targetUserID)
	assertCount(t, db, `SELECT COUNT(*) FROM auth_email_tokens WHERE user_id=$1 AND invalidated_at IS NOT NULL`, 1, targetUserID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE actor_id=$1 AND event_type IN ('staff_updated_by_admin','user_password_changed_by_admin','user_recovery_email_set_by_admin')`, 3, actorID)

	var auditMetadata string
	if err := db.QueryRowContext(ctx, `SELECT string_agg(metadata_json::text,' ') FROM audit_logs WHERE actor_id=$1`, actorID).Scan(&auditMetadata); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(auditMetadata, newPassword) || strings.Contains(auditMetadata, recoveryEmail) || strings.Contains(auditMetadata, createdPassword) || strings.Contains(auditMetadata, createdRecoveryEmail) {
		t.Fatalf("security secret leaked into audit metadata: %s", auditMetadata)
	}
	if _, err := service.UpdateStaff(ctx, targetStaffID, actorID, actorEmail, UpdateStaffRequest{NewPassword: newPassword, NewPasswordConfirmation: newPassword}); err == nil {
		t.Fatal("admin reset accepted the current password")
	}

	mustExec(t, db, `
		INSERT INTO auth_sessions (user_id,refresh_token_hash,access_token_id,expires_at)
		VALUES ($1,$2,$3,now()+interval '1 day')
	`, targetUserID, strings.Repeat("c", 56)+suffix[:8], "identity-session-"+suffix)
	identityEmail := "staff-security-renamed-" + suffix[:8] + "@test.local"
	activeStatus := "active"
	identityResult, err := service.UpdateStaff(ctx, targetStaffID, actorID, actorEmail, UpdateStaffRequest{Email: &identityEmail, Status: &activeStatus})
	if err != nil {
		t.Fatal(err)
	}
	if identityResult["identity_changed"] != true || identityResult["reauth_required"] != false {
		t.Fatalf("unexpected identity sync response: %#v", identityResult)
	}
	assertCount(t, db, `
		SELECT COUNT(*) FROM users account JOIN staff person ON person.user_id=account.id
		WHERE account.id=$1 AND person.id=$2 AND account.email=$3 AND person.email=$3 AND account.status='active' AND person.status='active'
	`, 1, targetUserID, targetStaffID, identityEmail)
	assertCount(t, db, `SELECT COUNT(*) FROM auth_sessions WHERE access_token_id=$1 AND revoked_at IS NOT NULL`, 1, "identity-session-"+suffix)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE actor_id=$1 AND target_id=$2 AND event_type='user_identity_synced_by_admin'`, 2, actorID, targetUserID)

	if _, err := service.UpdateStaff(ctx, unlinkedStaffID, actorID, actorEmail, UpdateStaffRequest{NewPassword: newPassword, NewPasswordConfirmation: newPassword}); err == nil {
		t.Fatal("expected unlinked staff account-security update to fail")
	}
	selfResult, err := service.UpdateStaff(ctx, actorStaffID, actorID, actorEmail, UpdateStaffRequest{NewPassword: "Actor-Reset-123!", NewPasswordConfirmation: "Actor-Reset-123!"})
	if err != nil {
		t.Fatal(err)
	}
	if selfResult["reauth_required"] != true {
		t.Fatalf("self password reset response=%#v, want reauth_required=true", selfResult)
	}
}

func TestProjectOperationsAccessPolicy(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	suffix := newID()
	projectID := newID()
	cancelledProjectID := newID()
	adminUserID := newID()
	directorUserID := newID()
	pmUserID := newID()
	pmStaffID := newID()
	noScopeUserID := newID()
	projectRoleNoPermissionUserID := newID()
	projectRolePermissionUserID := newID()
	var projectPermissionRoleID string

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id IN ($1,$2,$3,$4,$5,$6)`, adminUserID, directorUserID, pmUserID, noScopeUserID, projectRoleNoPermissionUserID, projectRolePermissionUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id IN ($1,$2)`, projectID, cancelledProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, pmStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2,$3,$4,$5,$6)`, adminUserID, directorUserID, pmUserID, noScopeUserID, projectRoleNoPermissionUserID, projectRolePermissionUserID)
		if projectPermissionRoleID != "" {
			_, _ = db.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, projectPermissionRoleID)
			_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, projectPermissionRoleID)
		}
	})

	operationViewID := ensurePermission(t, db, "operation.view", "Xem thi công", "operation")
	adminRoleID := ensureRole(t, db, "admin", "Quản trị")
	directorRoleID := ensureRole(t, db, "director", "Giám đốc")
	pmRoleID := ensureRole(t, db, "pm", "PM")
	noPermissionRoleID := ensureRole(t, db, "batch2_viewer_"+suffix[:8], "Batch2 Viewer")
	projectPermissionRoleID = ensureRole(t, db, "batch2_project_viewer_"+suffix[:8], "Batch2 Project Viewer")
	mustExec(t, db, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2),($3,$2),($4,$2) ON CONFLICT DO NOTHING`, directorRoleID, operationViewID, pmRoleID, projectPermissionRoleID)

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Admin','active')`, adminUserID, "admin-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Director','active')`, directorUserID, "director-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','PM','active')`, pmUserID, "pm-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','No Scope','active')`, noScopeUserID, "noscope-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Project Role No Permission','active')`, projectRoleNoPermissionUserID, "project-role-no-perm-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Project Role Permission','active')`, projectRolePermissionUserID, "project-role-perm-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, position, status) VALUES ($1,$2,'PM',$3,'Dự án','PM','active')`, pmStaffID, pmUserID, "pm-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Ops Access',$2,'Client','planning'),($3,'Ops Cancelled',$4,'Client','cancelled')`, projectID, "OPA-"+suffix[:8], cancelledProjectID, "OPC-"+suffix[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["PM"]'::jsonb,true)`, projectID, pmStaffID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id) VALUES ($1,$2),($3,$4),($5,$6),($7,$6)`, adminUserID, adminRoleID, directorUserID, directorRoleID, pmUserID, pmRoleID, noScopeUserID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id, project_id) VALUES ($1,$2,$3)`, projectRoleNoPermissionUserID, noPermissionRoleID, projectID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id, project_id) VALUES ($1,$2,$3)`, projectRolePermissionUserID, projectPermissionRoleID, projectID)

	assertCanViewProjectOperations(t, repo, ctx, projectID, adminUserID, true)
	assertCanViewProjectOperations(t, repo, ctx, projectID, directorUserID, true)
	assertCanViewProjectOperations(t, repo, ctx, projectID, pmUserID, true)
	assertCanViewProjectOperations(t, repo, ctx, projectID, noScopeUserID, false)
	assertCanViewProjectOperations(t, repo, ctx, projectID, projectRoleNoPermissionUserID, false)
	assertCanViewProjectOperations(t, repo, ctx, projectID, projectRolePermissionUserID, true)
	assertCanViewProjectOperations(t, repo, ctx, cancelledProjectID, directorUserID, false)

	visibilityCases := []struct {
		name   string
		userID string
		want   bool
	}{
		{name: "admin", userID: adminUserID, want: true},
		{name: "director", userID: directorUserID, want: true},
		{name: "member", userID: pmUserID, want: true},
		{name: "project role", userID: projectRolePermissionUserID, want: true},
		{name: "no project scope", userID: noScopeUserID, want: false},
		{name: "project role without permission", userID: projectRoleNoPermissionUserID, want: false},
	}
	for _, test := range visibilityCases {
		t.Run("snapshot parity "+test.name, func(t *testing.T) {
			visibleProjects, err := repo.ProjectOperationsVisibleProjectIDs(ctx, test.userID)
			if err != nil {
				t.Fatal(err)
			}
			_, visible := visibleProjects[projectID]
			if visible != test.want {
				t.Fatalf("active project snapshot visibility=%v, want %v", visible, test.want)
			}
			if _, visible := visibleProjects[cancelledProjectID]; visible {
				t.Fatal("cancelled project leaked into activity visibility snapshot")
			}
			canView, err := repo.CanViewProjectOperations(ctx, projectID, test.userID)
			if err != nil {
				t.Fatal(err)
			}
			if visible != canView {
				t.Fatalf("snapshot visibility=%v differs from CanViewProjectOperations=%v", visible, canView)
			}
		})
	}
}

func TestBatch4ProjectListIsScopedByMembership(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	fixture := setupProjectScopeFixture(t, db, []string{"project.view"})

	rows, err := repo.ListProjects(ctx, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly one scoped project, got %d", len(rows))
	}
	if rows[0]["id"] != fixture.projectID {
		t.Fatalf("project id = %v, want %s", rows[0]["id"], fixture.projectID)
	}
}

func TestBatch4ProjectCreateRequiresModulePermission(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	handler := NewHandler(NewService(repo, "test-secret"))
	fixture := setupProjectScopeFixture(t, db, []string{"project.view", "boq.view"})
	allowed, err := repo.CanAccessProjectPermission(ctx, fixture.userID, fixture.projectID, []string{"boq.create"})
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("expected direct RBAC helper to deny boq.create")
	}
	app := batch4TestApp(fixture.userID, func(app *fiber.App, h *Handler) {
		app.Post("/projects/:id/boq", h.CreateBOQ)
	}, handler)

	req := httptest.NewRequest(http.MethodPost, "/projects/"+fixture.projectID+"/boq", strings.NewReader(`{"category":"A","work_item_name":"Denied","unit":"m2","quantity":1,"unit_price":10}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestAuthSessionRefreshAndLogoutRevokesAccess(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	userID := newID()
	suffix := userID[:8]
	email := "auth-session-" + suffix + "@test.local"
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Auth Session','active')`, userID, email)

	login, err := service.Login(ctx, LoginRequest{Email: email, Password: "pw"}, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	accessToken := fmt.Sprint(login["token"])
	refreshToken := fmt.Sprint(login["refresh_token"])
	if accessToken == "" || refreshToken == "" {
		t.Fatalf("login tokens missing: %#v", login)
	}

	handler := NewHandler(service)
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	app.Use(middleware.AuthRequired("test-secret", repo))
	app.Get("/me", handler.Me)
	assertBearerStatus(t, app, "/me", accessToken, http.StatusOK)

	refreshed, err := service.RefreshSession(ctx, refreshToken, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	nextAccessToken := fmt.Sprint(refreshed["token"])
	nextRefreshToken := fmt.Sprint(refreshed["refresh_token"])
	assertBearerStatus(t, app, "/me", accessToken, http.StatusUnauthorized)
	assertBearerStatus(t, app, "/me", nextAccessToken, http.StatusOK)

	if err := service.LogoutByRefreshToken(ctx, nextRefreshToken); err != nil {
		t.Fatal(err)
	}
	assertBearerStatus(t, app, "/me", nextAccessToken, http.StatusUnauthorized)
}

func TestExternalOnlyRoleDeniesAuthenticationSessionsAndRBACAssignment(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	suffix := newID()
	externalRoleID := newID()
	userID := newID()
	assignmentTargetID := newID()
	permissionID := ensurePermission(t, db, "external.only.permission."+suffix[:8], "External only permission", "rbac")
	email := "external-only-" + suffix[:8] + "@test.local"
	externalDisplayName := "Đối tác ngoài App " + suffix[:8]
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id IN ($1,$2)`, userID, assignmentTargetID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id IN ($1,$2)`, userID, assignmentTargetID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, userID, assignmentTargetID)
		_, _ = db.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, externalRoleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, externalRoleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM permissions WHERE id=$1`, permissionID)
	})
	mustExec(t, db, `INSERT INTO roles (id,name,display_name,system_role,external_only) VALUES ($1,$2,$3,true,true)`, externalRoleID, "external_only_"+suffix[:8], externalDisplayName)
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','External Session','active'),($3,$4,'demo:pw','Assignment Target','active')`, userID, email, assignmentTargetID, "assignment-target-"+suffix[:8]+"@test.local")
	if _, err := service.UpdateRole(ctx, externalRoleID, Record{"display_name": "Renamed external role"}); err == nil {
		t.Fatal("expected external-only role update to be denied")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM roles WHERE id=$1 AND display_name=$2`, 1, externalRoleID, externalDisplayName)

	login, err := service.Login(ctx, LoginRequest{Email: email, Password: "pw"}, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	accessToken := fmt.Sprint(login["token"])
	refreshToken := fmt.Sprint(login["refresh_token"])
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) VALUES ($1,$2)`, userID, externalRoleID)

	if _, err := service.Login(ctx, LoginRequest{Email: email, Password: "pw"}, "test-agent", "127.0.0.1"); err == nil {
		t.Fatal("expected external-only user login to be denied")
	}
	if _, err := service.RefreshSession(ctx, refreshToken, "test-agent", "127.0.0.1"); err == nil {
		t.Fatal("expected external-only refresh session to be denied")
	}
	handler := NewHandler(service)
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	app.Use(middleware.AuthRequired("test-secret", repo))
	app.Get("/me", handler.Me)
	assertBearerStatus(t, app, "/me", accessToken, http.StatusUnauthorized)

	if _, err := repo.ReplaceRolePermissions(ctx, externalRoleID, []string{permissionID}); err == nil {
		t.Fatal("expected external-only role permission assignment to be denied")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM role_permissions WHERE role_id=$1`, 0, externalRoleID)
	if _, err := repo.ReplaceUserRoles(ctx, assignmentTargetID, []string{externalRoleID}, ""); err == nil {
		t.Fatal("expected external-only user role assignment to be denied")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1`, 0, assignmentTargetID)
}

func TestRBACAssignmentsReplaceSnapshots(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	suffix := newID()
	roleID := ensureRole(t, db, "rbac_replace_"+suffix[:8], "RBAC Replace")
	firstPermissionID := ensurePermission(t, db, "rbac.replace.first."+suffix[:8], "RBAC Replace First", "rbac")
	secondPermissionID := ensurePermission(t, db, "rbac.replace.second."+suffix[:8], "RBAC Replace Second", "rbac")
	userID := newID()
	projectID := newID()
	staffID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, roleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, roleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, staffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','RBAC User','active')`, userID, "rbac-user-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, position, status) VALUES ($1,$2,'RBAC User',$3,'Dự án','Member','active')`, staffID, userID, "rbac-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'RBAC Scoped',$2,'Client','planning')`, projectID, "RBA-"+suffix[:8])

	if _, err := repo.ReplaceRolePermissions(ctx, roleID, []string{firstPermissionID, secondPermissionID}); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM role_permissions WHERE role_id=$1`, 2, roleID)
	if _, err := repo.ReplaceRolePermissions(ctx, roleID, []string{secondPermissionID}); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM role_permissions WHERE role_id=$1 AND permission_id=$2`, 0, roleID, firstPermissionID)
	assertCount(t, db, `SELECT COUNT(*) FROM role_permissions WHERE role_id=$1 AND permission_id=$2`, 1, roleID, secondPermissionID)

	if _, err := repo.ReplaceUserRoles(ctx, userID, []string{roleID}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReplaceUserRoles(ctx, userID, []string{roleID}, projectID); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1 AND project_id IS NULL`, 1, userID)
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1 AND project_id=$2`, 1, userID, projectID)
	if _, err := repo.ReplaceUserRoles(ctx, userID, []string{}, ""); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1 AND project_id IS NULL`, 0, userID)
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1 AND project_id=$2`, 1, userID, projectID)
}
