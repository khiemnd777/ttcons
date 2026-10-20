package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"consflow/backend/internal/shared/mailer"
)

type captureMailSender struct {
	messages chan mailer.Message
	err      error
}

func (s *captureMailSender) Send(_ context.Context, message mailer.Message) error {
	if s.err != nil {
		return s.err
	}
	s.messages <- message
	return nil
}

func TestProfileInputNormalization(t *testing.T) {
	email, err := normalizeProfileEmail("  USER@Example.COM ")
	if err != nil {
		t.Fatal(err)
	}
	if email != "user@example.com" {
		t.Fatalf("email = %q", email)
	}
	phone, err := normalizeProfilePhone("(+84) 901-234-567")
	if err != nil {
		t.Fatal(err)
	}
	if phone != "+84901234567" {
		t.Fatalf("phone = %q", phone)
	}
	if _, err := normalizeProfileEmail("not-an-email"); err == nil {
		t.Fatal("expected invalid email error")
	}
	if _, err := normalizeProfilePhone("phone-number"); err == nil {
		t.Fatal("expected invalid phone error")
	}
	if !passwordMatches("demo:secret", "secret") || passwordMatches("demo:secret", "wrong") {
		t.Fatal("demo password compatibility mismatch")
	}
}

func TestOwnAvatarUploadNormalizesProtectsAndReplacesImage(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	storageRoot := t.TempDir()
	service.ConfigureUploads(storageRoot, defaultMaxUploadBytes)
	userID := newID()
	email := "avatar-" + userID[:8] + "@test.local"
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE actor_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:avatar-secret','Avatar User','active')`, userID, email)

	first := testMultipartFileHeader(t, "file", "avatar.jpg", avatarImageBytes(t, color.RGBA{R: 12, G: 86, B: 212, A: 255}))
	profile, err := service.UpdateOwnAvatar(ctx, userID, "Avatar User", first)
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(profile["has_avatar"]) || profile["avatar_updated_at"] == nil {
		t.Fatalf("avatar fields missing from own profile: %#v", profile)
	}
	if _, exposed := profile["avatar_storage_key"]; exposed {
		t.Fatalf("own profile must not expose avatar storage metadata: %#v", profile)
	}
	var firstProvider, firstKey, firstMIME string
	if err := db.QueryRowContext(ctx, `SELECT avatar_storage_provider,avatar_storage_key,avatar_mime_type FROM users WHERE id=$1`, userID).Scan(&firstProvider, &firstKey, &firstMIME); err != nil {
		t.Fatal(err)
	}
	if firstProvider != "local" || firstMIME != "image/png" || firstKey == "" {
		t.Fatalf("stored avatar metadata = provider=%q key=%q mime=%q", firstProvider, firstKey, firstMIME)
	}
	file, err := service.OwnAvatarFile(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	content, readErr := io.ReadAll(file.Reader)
	closeErr := file.Reader.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read avatar file: read=%v close=%v", readErr, closeErr)
	}
	if file.MIMEType != "image/png" {
		t.Fatalf("avatar MIME = %q, want image/png", file.MIMEType)
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(content)); err != nil {
		t.Fatalf("stored avatar is not decodable: %v", err)
	}

	firstPath := filepath.Join(storageRoot, filepath.FromSlash(firstKey))
	second := testMultipartFileHeader(t, "file", "replacement.jpg", avatarJPEGBytes(t, color.RGBA{R: 42, G: 161, B: 152, A: 255}))
	if _, err := service.UpdateOwnAvatar(ctx, userID, "Avatar User", second); err != nil {
		t.Fatal(err)
	}
	var secondKey string
	if err := db.QueryRowContext(ctx, `SELECT avatar_storage_key FROM users WHERE id=$1`, userID).Scan(&secondKey); err != nil {
		t.Fatal(err)
	}
	if secondKey == firstKey {
		t.Fatal("replacement avatar must receive a new storage key")
	}
	if _, err := os.Stat(firstPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("previous avatar object stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(filepath.Join(storageRoot, filepath.FromSlash(secondKey))); err != nil {
		t.Fatalf("replacement avatar object stat error = %v", err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE actor_id=$1 AND event_type='user_avatar_updated'`, 2, userID)

	invalid := testMultipartFileHeader(t, "file", "avatar.txt", []byte("not an image"))
	if _, err := service.UpdateOwnAvatar(ctx, userID, "Avatar User", invalid); err == nil {
		t.Fatal("expected invalid avatar type to be rejected")
	}
}

func avatarImageBytes(t *testing.T, fill color.RGBA) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			canvas.SetRGBA(x, y, fill)
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func avatarJPEGBytes(t *testing.T, fill color.RGBA) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			canvas.SetRGBA(x, y, fill)
		}
	}
	var output bytes.Buffer
	if err := jpeg.Encode(&output, canvas, nil); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestOwnProfileUpdateAndPasswordChange(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	userID := newID()
	staffID := newID()
	conflictUserID := newID()
	originalEmail := "profile-" + userID[:8] + "@test.local"
	nextEmail := "profile-next-" + userID[:8] + "@test.local"
	conflictEmail := "profile-conflict-" + userID[:8] + "@test.local"

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id IN ($1,$2)`, userID, conflictUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE actor_id=$1 AND target_type='user_account'`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, staffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, userID, conflictUserID)
	})
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:old-secret','Profile User','active'),($3,$4,'demo:other-secret','Conflict User','active')`, userID, originalEmail, conflictUserID, conflictEmail)
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,phone,department,position,status) VALUES ($1,$2,'Profile User',$3,'0901234567','Dự án','Project Member','active')`, staffID, userID, originalEmail)

	login, err := service.Login(ctx, LoginRequest{Email: originalEmail, Password: "old-secret"}, "profile-test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if phone := fmt.Sprint(login["user"].(Record)["phone"]); phone != "0901234567" {
		t.Fatalf("login profile phone = %q", phone)
	}

	updated, err := service.UpdateOwnProfile(ctx, userID, originalEmail, UpdateOwnProfileRequest{
		FullName: "  Profile   Renamed ",
		Email:    originalEmail,
		Phone:    "(+84) 901-222-333",
	})
	if err != nil {
		t.Fatal(err)
	}
	if boolValue(updated["reauth_required"]) {
		t.Fatal("name/phone update must preserve sessions")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM users WHERE id=$1 AND full_name='Profile Renamed' AND email=$2`, 1, userID, originalEmail)
	assertCount(t, db, `SELECT COUNT(*) FROM staff WHERE id=$1 AND full_name='Profile Renamed' AND email=$2 AND phone='+84901222333'`, 1, staffID, originalEmail)
	assertCount(t, db, `SELECT COUNT(*) FROM auth_sessions WHERE user_id=$1 AND revoked_at IS NULL`, 1, userID)

	if _, err := service.UpdateOwnProfile(ctx, userID, originalEmail, UpdateOwnProfileRequest{
		FullName:        "Profile Renamed",
		Email:           nextEmail,
		Phone:           "+84901222333",
		CurrentPassword: "wrong",
	}); err == nil {
		t.Fatal("expected current-password validation for email change")
	}
	if _, err := service.UpdateOwnProfile(ctx, userID, originalEmail, UpdateOwnProfileRequest{
		FullName:        "Profile Renamed",
		Email:           conflictEmail,
		Phone:           "+84901222333",
		CurrentPassword: "old-secret",
	}); err == nil {
		t.Fatal("expected duplicate-email conflict")
	} else {
		var statusErr StatusError
		if !errors.As(err, &statusErr) || statusErr.Code != "CONFLICT" {
			t.Fatalf("duplicate email error = %#v", err)
		}
	}

	updated, err = service.UpdateOwnProfile(ctx, userID, originalEmail, UpdateOwnProfileRequest{
		FullName:        "Profile Renamed",
		Email:           nextEmail,
		Phone:           "+84901222333",
		CurrentPassword: "old-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(updated["reauth_required"]) {
		t.Fatal("email change must require reauthentication")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM users WHERE id=$1 AND email=$2`, 1, userID, nextEmail)
	assertCount(t, db, `SELECT COUNT(*) FROM staff WHERE id=$1 AND email=$2`, 1, staffID, nextEmail)
	assertCount(t, db, `SELECT COUNT(*) FROM auth_sessions WHERE user_id=$1 AND revoked_at IS NULL`, 0, userID)
	if _, err := service.Login(ctx, LoginRequest{Email: originalEmail, Password: "old-secret"}, "profile-test", "127.0.0.1"); err == nil {
		t.Fatal("old email must stop authenticating")
	}
	if _, err := service.Login(ctx, LoginRequest{Email: nextEmail, Password: "old-secret"}, "profile-test", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}

	if _, err := service.ChangeOwnPassword(ctx, userID, nextEmail, ChangeOwnPasswordRequest{
		CurrentPassword:         "wrong",
		NewPassword:             "new-secret-123",
		NewPasswordConfirmation: "new-secret-123",
	}); err == nil {
		t.Fatal("expected wrong-current-password validation")
	}
	if _, err := service.ChangeOwnPassword(ctx, userID, nextEmail, ChangeOwnPasswordRequest{
		CurrentPassword:         "old-secret",
		NewPassword:             "new-secret-123",
		NewPasswordConfirmation: "different-secret",
	}); err == nil {
		t.Fatal("expected password-confirmation validation")
	}
	if _, err := service.ChangeOwnPassword(ctx, userID, nextEmail, ChangeOwnPasswordRequest{
		CurrentPassword:         "old-secret",
		NewPassword:             "new-secret-123",
		NewPasswordConfirmation: "new-secret-123",
	}); err != nil {
		t.Fatal(err)
	}
	var passwordHash string
	if err := db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id=$1`, userID).Scan(&passwordHash); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(passwordHash, "demo:") || !passwordMatches(passwordHash, "new-secret-123") {
		t.Fatal("password was not persisted as a bcrypt hash")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM auth_sessions WHERE user_id=$1 AND revoked_at IS NULL`, 0, userID)
	if _, err := service.Login(ctx, LoginRequest{Email: nextEmail, Password: "old-secret"}, "profile-test", "127.0.0.1"); err == nil {
		t.Fatal("old password must stop authenticating")
	}
	if _, err := service.Login(ctx, LoginRequest{Email: nextEmail, Password: "new-secret-123"}, "profile-test", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}

	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE actor_id=$1 AND event_type='user_profile_updated'`, 2, userID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE actor_id=$1 AND event_type='user_password_changed'`, 1, userID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE actor_id=$1 AND metadata_json::text ILIKE '%secret%'`, 0, userID)
}

func TestOwnProfileDoesNotCreateMissingStaffRecord(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	userID := newID()
	email := "profile-unlinked-" + userID[:8] + "@test.local"
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE actor_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:old-secret','Unlinked Profile','active')`, userID, email)

	if _, err := service.UpdateOwnProfile(ctx, userID, email, UpdateOwnProfileRequest{
		FullName: "Unlinked Profile",
		Email:    email,
		Phone:    "0901234567",
	}); err == nil {
		t.Fatal("expected missing-staff validation")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM staff WHERE user_id=$1`, 0, userID)
}

func TestRecoveryEmailAndPasswordResetLifecycle(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	sender := &captureMailSender{messages: make(chan mailer.Message, 8)}
	service := NewService(repo, "test-secret")
	service.ConfigureAccountRecovery(true, sender, "https://workflow.test", 15*time.Minute, 24*time.Hour)
	userID := newID()
	loginEmail := "recovery-login-" + userID[:8] + "@test.local"
	firstRecoveryEmail := "recovery-one-" + userID[:8] + "@example.com"
	nextRecoveryEmail := "recovery-two-" + userID[:8] + "@example.com"
	developerID := newID()
	developerEmail := "recovery-developer-" + developerID[:8] + "@test.local"

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM notification_events WHERE aggregate_type='account_recovery_request' AND aggregate_id IN (SELECT id FROM account_recovery_requests WHERE user_id=$1)`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM account_recovery_requests WHERE user_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth_email_tokens WHERE user_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE actor_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE actor_id=$1`, developerID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, developerID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, developerID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:old-secret','Recovery User','active')`, userID, loginEmail)
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:developer','Recovery Developer','active')`, developerID, developerEmail)
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) SELECT $1,id FROM roles WHERE name='developer' AND system_role`, developerID)
	staleTokenHash := hashSessionToken("stale-recovery-token-fixture")
	mustExec(t, db, `
		INSERT INTO auth_email_tokens (user_id,purpose,token_hash,target_email,expires_at,consumed_at,created_at)
		VALUES ($1,'reset_password',$2,$3,now()-interval '31 days',now()-interval '31 days',now()-interval '31 days')
	`, userID, staleTokenHash, firstRecoveryEmail)

	if _, err := service.RequestRecoveryEmailVerification(ctx, userID, UpdateRecoveryEmailRequest{RecoveryEmail: firstRecoveryEmail, CurrentPassword: "wrong"}); err == nil {
		t.Fatal("expected current-password validation before recovery email setup")
	}
	if _, err := service.RequestRecoveryEmailVerification(ctx, userID, UpdateRecoveryEmailRequest{RecoveryEmail: loginEmail, CurrentPassword: "old-secret"}); err == nil {
		t.Fatal("login email must not be accepted as recovery email")
	}
	if _, err := service.RequestRecoveryEmailVerification(ctx, userID, UpdateRecoveryEmailRequest{RecoveryEmail: firstRecoveryEmail, CurrentPassword: "old-secret"}); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM auth_email_tokens WHERE token_hash=$1`, 0, staleTokenHash)
	verificationMessage := receiveMail(t, sender.messages)
	if verificationMessage.To != firstRecoveryEmail || strings.Contains(verificationMessage.Text, loginEmail) {
		t.Fatalf("unexpected verification message: %#v", verificationMessage)
	}

	profile, err := service.Me(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if _, leaked := profile["recovery_email_private"]; leaked {
		t.Fatal("private recovery email key leaked through own-profile response")
	}
	if fmt.Sprint(profile["pending_recovery_email_masked"]) == firstRecoveryEmail {
		t.Fatal("pending recovery email was not masked")
	}
	if err := repo.VerifyRecoveryEmail(ctx, hashSessionToken(tokenFromMail(t, verificationMessage))); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM users WHERE id=$1 AND recovery_email=$2 AND recovery_email_verified_at IS NOT NULL`, 1, userID, firstRecoveryEmail)

	if _, err := service.RequestRecoveryEmailVerification(ctx, userID, UpdateRecoveryEmailRequest{RecoveryEmail: nextRecoveryEmail, CurrentPassword: "old-secret"}); err != nil {
		t.Fatal(err)
	}
	replacementMessage := receiveMail(t, sender.messages)
	assertCount(t, db, `SELECT COUNT(*) FROM users WHERE id=$1 AND recovery_email=$2`, 1, userID, firstRecoveryEmail)
	if _, err := service.VerifyRecoveryEmail(ctx, VerifyRecoveryEmailRequest{Token: tokenFromMail(t, replacementMessage)}); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM users WHERE id=$1 AND recovery_email=$2 AND recovery_email_verified_at IS NOT NULL`, 1, userID, nextRecoveryEmail)

	if _, err := service.Login(ctx, LoginRequest{Email: loginEmail, Password: "old-secret"}, "recovery-test", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	unknownResult, err := service.ForgotPassword(ctx, ForgotPasswordRequest{Email: "unknown@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	knownResult, err := service.ForgotPassword(ctx, ForgotPasswordRequest{Email: loginEmail})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(unknownResult["message"]) != fmt.Sprint(knownResult["message"]) || unknownResult["accepted"] != knownResult["accepted"] {
		t.Fatal("forgot-password response enumerates account availability")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM account_recovery_requests WHERE user_id=$1 AND status='pending'`, 1, userID)
	select {
	case <-sender.messages:
		t.Fatal("forgot-password sent an automatic reset email")
	default:
	}
	var requestID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM account_recovery_requests WHERE user_id=$1`, userID).Scan(&requestID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.IssueDeveloperRecoveryLink(ctx, requestID, userID, "Recovery User"); err == nil {
		t.Fatal("ordinary user issued a recovery link")
	}
	issued, err := service.IssueDeveloperRecoveryLink(ctx, requestID, developerID, "Recovery Developer")
	if err != nil {
		t.Fatal(err)
	}
	resetToken := strings.TrimPrefix(fmt.Sprint(issued["link"]), "https://workflow.test/reset-password#token=")
	if resetToken == fmt.Sprint(issued["link"]) {
		t.Fatal("reset link missing token fragment")
	}
	if _, err := service.ResetPassword(ctx, ResetPasswordRequest{Token: resetToken, NewPassword: "old-secret", NewPasswordConfirmation: "old-secret"}); err == nil {
		t.Fatal("recovery reset accepted the current password")
	}
	if _, err := service.ResetPassword(ctx, ResetPasswordRequest{Token: resetToken, NewPassword: "new-secret-123", NewPasswordConfirmation: "new-secret-123"}); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM auth_sessions WHERE user_id=$1 AND revoked_at IS NULL`, 0, userID)
	if _, err := service.ResetPassword(ctx, ResetPasswordRequest{Token: resetToken, NewPassword: "another-secret-123", NewPasswordConfirmation: "another-secret-123"}); err == nil {
		t.Fatal("one-time reset token was accepted twice")
	}
	if _, err := service.Login(ctx, LoginRequest{Email: loginEmail, Password: "old-secret"}, "recovery-test", "127.0.0.1"); err == nil {
		t.Fatal("old password still authenticates after recovery reset")
	}
	if _, err := service.Login(ctx, LoginRequest{Email: loginEmail, Password: "new-secret-123"}, "recovery-test", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RemoveRecoveryEmail(ctx, userID, loginEmail, RemoveRecoveryEmailRequest{CurrentPassword: "new-secret-123"}); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM users WHERE id=$1 AND recovery_email IS NULL AND recovery_email_verified_at IS NULL`, 1, userID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE actor_id=$1 AND metadata_json::text LIKE '%' || $2 || '%'`, 0, userID, nextRecoveryEmail)
}

func receiveMail(t *testing.T, messages <-chan mailer.Message) mailer.Message {
	t.Helper()
	select {
	case message := <-messages:
		return message
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for recovery email")
		return mailer.Message{}
	}
}

func tokenFromMail(t *testing.T, message mailer.Message) string {
	t.Helper()
	const marker = "#token="
	position := strings.Index(message.Text, marker)
	if position < 0 {
		t.Fatalf("message does not contain a fragment token: %q", message.Text)
	}
	fields := strings.Fields(message.Text[position+len(marker):])
	if len(fields) == 0 || fields[0] == "" {
		t.Fatal("message fragment token is empty")
	}
	return fields[0]
}
