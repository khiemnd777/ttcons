package core

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDeveloperRecoveryWithoutRecoveryEmail(t *testing.T) {
	db := openIsolatedProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	service.ConfigureAccountRecovery(false, nil, "https://workflow.test", 15*time.Minute, 24*time.Hour)
	userID, developerID := newID(), newID()
	loginEmail := "manual-recovery-" + userID[:8] + "@test.local"
	developerEmail := "manual-developer-" + developerID[:8] + "@test.local"
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM notification_events WHERE aggregate_type='account_recovery_request' AND aggregate_id IN (SELECT id FROM account_recovery_requests WHERE user_id=$1)`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM account_recovery_requests WHERE user_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE actor_id IN ($1,$2)`, userID, developerID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, developerID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, userID, developerID)
	})
	mustExec(t, db, `INSERT INTO users(id,email,password_hash,full_name,status) VALUES($1,$2,'demo:old-secret','Manual Recovery User','active')`, userID, loginEmail)
	mustExec(t, db, `INSERT INTO users(id,email,password_hash,full_name,status) VALUES($1,$2,'demo:developer','Manual Recovery Developer','active')`, developerID, developerEmail)
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id) SELECT $1,id FROM roles WHERE name='developer' AND system_role`, developerID)
	allowed, err := repo.DeveloperRecoveryAllowed(ctx, userID)
	if err != nil || allowed {
		t.Fatalf("ordinary user recovery authority=%v, err=%v", allowed, err)
	}
	allowed, err = repo.DeveloperRecoveryAllowed(ctx, developerID)
	if err != nil || !allowed {
		t.Fatalf("developer recovery authority=%v, err=%v", allowed, err)
	}
	if _, err := service.ForgotPassword(ctx, ForgotPasswordRequest{Email: loginEmail}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ForgotPassword(ctx, ForgotPasswordRequest{Email: loginEmail}); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM account_recovery_requests WHERE user_id=$1 AND status='pending'`, 1, userID)
	var requestID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM account_recovery_requests WHERE user_id=$1`, userID).Scan(&requestID); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM notification_events WHERE aggregate_type='account_recovery_request' AND aggregate_id=$1`, 1, requestID)
	if err := service.dispatchNotificationEvents(ctx); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM user_notifications WHERE recipient_user_id=$1 AND source_kind='account_recovery_request' AND source_id=$2 AND intent='action_required' AND resolved_at IS NULL AND action_href=$3`, 1, developerID, requestID, "/recovery-requests?request="+requestID)
	queue, err := service.ListDeveloperRecoveryRequests(ctx, developerID, "pending", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(queue["items"].([]Record)) != 1 {
		t.Fatalf("developer queue items=%d", len(queue["items"].([]Record)))
	}
	firstIssued, err := service.IssueDeveloperRecoveryLink(ctx, requestID, developerID, "Manual Recovery Developer")
	if err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM account_recovery_requests WHERE id=$1 AND status='issued' AND verification_method IS NULL`, 1, requestID)
	assertCount(t, db, `SELECT count(*) FROM audit_logs WHERE actor_id=$1 AND event_type='account_recovery_link_issued' AND metadata_json ? 'verification_method'`, 0, developerID)
	assertCount(t, db, `SELECT count(*) FROM user_notifications WHERE recipient_user_id=$1 AND source_id=$2 AND resolved_at IS NOT NULL`, 1, developerID, requestID)
	firstToken := strings.TrimPrefix(fmt.Sprint(firstIssued["link"]), "https://workflow.test/reset-password#token=")
	if _, err := service.IssueDeveloperRecoveryLink(ctx, requestID, developerID, "Manual Recovery Developer"); err == nil {
		t.Fatal("immediate duplicate issuance accepted")
	}
	mustExec(t, db, `UPDATE account_recovery_requests SET issued_at=now()-interval '2 minutes' WHERE id=$1`, requestID)
	issued, err := service.IssueDeveloperRecoveryLink(ctx, requestID, developerID, "Manual Recovery Developer")
	if err != nil {
		t.Fatal(err)
	}
	token := strings.TrimPrefix(fmt.Sprint(issued["link"]), "https://workflow.test/reset-password#token=")
	if token == fmt.Sprint(issued["link"]) {
		t.Fatal("issued link has no token fragment")
	}
	assertCount(t, db, `SELECT count(*) FROM auth_email_tokens WHERE recovery_request_id=$1 AND invalidated_at IS NOT NULL`, 1, requestID)
	if _, err := service.ResetPassword(ctx, ResetPasswordRequest{Token: firstToken, NewPassword: "new-secret-123", NewPasswordConfirmation: "new-secret-123"}); err == nil {
		t.Fatal("reissued link left its predecessor usable")
	}
	if _, err := service.RejectDeveloperRecoveryRequest(ctx, requestID, developerID, "Manual Recovery Developer"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResetPassword(ctx, ResetPasswordRequest{Token: token, NewPassword: "new-secret-123", NewPasswordConfirmation: "new-secret-123"}); err == nil {
		t.Fatal("rejected request token reset the password")
	}
	mustExec(t, db, `UPDATE account_recovery_requests SET requested_at=now()-interval '2 minutes' WHERE id=$1`, requestID)
	if _, err := service.ForgotPassword(ctx, ForgotPasswordRequest{Email: loginEmail}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM account_recovery_requests WHERE user_id=$1 AND status='pending'`, userID).Scan(&requestID); err != nil {
		t.Fatal(err)
	}
	issued, err = service.IssueDeveloperRecoveryLink(ctx, requestID, developerID, "Manual Recovery Developer")
	if err != nil {
		t.Fatal(err)
	}
	token = strings.TrimPrefix(fmt.Sprint(issued["link"]), "https://workflow.test/reset-password#token=")
	mustExec(t, db, `UPDATE auth_email_tokens SET expires_at=now()-interval '1 second' WHERE token_hash=$1`, hashSessionToken(token))
	if _, err := service.ResetPassword(ctx, ResetPasswordRequest{Token: token, NewPassword: "new-secret-123", NewPasswordConfirmation: "new-secret-123"}); err == nil {
		t.Fatal("expired link reset the password")
	}
	mustExec(t, db, `UPDATE account_recovery_requests SET requested_at=now()-interval '2 minutes' WHERE id=$1`, requestID)
	if _, err := service.ForgotPassword(ctx, ForgotPasswordRequest{Email: loginEmail}); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM account_recovery_requests WHERE id=$1 AND status='pending'`, 1, requestID)
	assertCount(t, db, `SELECT count(*) FROM notification_events WHERE aggregate_type='account_recovery_request' AND aggregate_id=$1`, 2, requestID)
	issued, err = service.IssueDeveloperRecoveryLink(ctx, requestID, developerID, "Manual Recovery Developer")
	if err != nil {
		t.Fatal(err)
	}
	token = strings.TrimPrefix(fmt.Sprint(issued["link"]), "https://workflow.test/reset-password#token=")
	if _, err := service.ResetPassword(ctx, ResetPasswordRequest{Token: token, NewPassword: "new-secret-123", NewPasswordConfirmation: "new-secret-123"}); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM account_recovery_requests WHERE id=$1 AND status='completed'`, 1, requestID)
	assertCount(t, db, `SELECT count(*) FROM audit_logs WHERE actor_id=$1 AND metadata_json::text LIKE '%' || $2 || '%'`, 0, developerID, token)
}
