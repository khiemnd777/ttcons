package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"consflow/backend/internal/shared/mailer"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

const forgotPasswordAcceptedMessage = "Nếu tài khoản hợp lệ, bộ phận hỗ trợ sẽ xem xét yêu cầu khôi phục."

func (s *Service) shapeOwnProfile(profile Record) {
	recoveryEmail := strings.TrimSpace(fmt.Sprint(profile["recovery_email_private"]))
	pendingEmail := strings.TrimSpace(fmt.Sprint(profile["pending_recovery_email_private"]))
	delete(profile, "recovery_email_private")
	delete(profile, "pending_recovery_email_private")
	profile["recovery_email_available"] = s.accountRecoveryReady()
	profile["recovery_email_verified"] = recoveryEmail != ""
	if recoveryEmail != "" {
		profile["recovery_email_masked"] = maskEmail(recoveryEmail)
	}
	if pendingEmail != "" {
		profile["pending_recovery_email_masked"] = maskEmail(pendingEmail)
	}
}

func maskEmail(value string) string {
	parts := strings.SplitN(strings.TrimSpace(value), "@", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "***"
	}
	local := []rune(parts[0])
	visibleLocal := len(local)
	if visibleLocal > 3 {
		visibleLocal = 3
	}
	domainParts := strings.Split(parts[1], ".")
	domainLabel := []rune(domainParts[0])
	maskedDomain := "***"
	if len(domainLabel) > 0 {
		maskedDomain = string(domainLabel[:1]) + "**"
	}
	if len(domainParts) > 1 && domainParts[len(domainParts)-1] != "" {
		maskedDomain += "." + domainParts[len(domainParts)-1]
	}
	return string(local[:visibleLocal]) + "***@" + maskedDomain
}

func (s *Service) RequestRecoveryEmailVerification(ctx context.Context, userID string, req UpdateRecoveryEmailRequest) (Record, error) {
	if !s.accountRecoveryReady() {
		return nil, recoveryUnavailableError()
	}
	recoveryEmail, err := normalizeRecoveryEmail(req.RecoveryEmail)
	if err != nil {
		return nil, err
	}
	account, err := s.repo.GetByID(ctx, "users", userID)
	if err != nil {
		return nil, err
	}
	currentPasswordHash := fmt.Sprint(account["password_hash"])
	if !passwordMatches(currentPasswordHash, req.CurrentPassword) {
		return nil, validationError("Mật khẩu hiện tại không đúng", Record{"field": "current_password"})
	}
	if strings.EqualFold(fmt.Sprint(account["email"]), recoveryEmail) {
		return nil, validationError("Email khôi phục phải khác email đăng nhập", Record{"field": "recovery_email"})
	}
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(account["recovery_email"])), recoveryEmail) && account["recovery_email_verified_at"] != nil {
		return nil, validationError("Email này đã được xác minh làm email khôi phục", Record{"field": "recovery_email"})
	}
	token, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	tokenHash := hashSessionToken(token)
	if err := s.repo.CreateRecoveryVerificationToken(ctx, userID, currentPasswordHash, tokenHash, recoveryEmail, time.Now().Add(s.recoveryVerifyTTL)); err != nil {
		if recoveryTokenNotFound(err) {
			return nil, validationError("Tài khoản đã thay đổi, vui lòng xác nhận lại mật khẩu hiện tại", Record{"field": "current_password"})
		}
		return nil, err
	}
	link := s.frontendOrigin + "/verify-recovery-email#token=" + token
	message := mailer.Message{
		To:      recoveryEmail,
		Subject: "Xác minh email khôi phục Consflow",
		Text:    fmt.Sprintf("Xin chào %s,\n\nMở liên kết sau để xác minh email khôi phục cho tài khoản Consflow của anh/chị:\n%s\n\nLiên kết này có thời hạn và chỉ sử dụng được một lần. Nếu không thực hiện yêu cầu này, anh/chị có thể bỏ qua email.", strings.TrimSpace(fmt.Sprint(account["full_name"])), link),
	}
	if err := s.mailSender.Send(ctx, message); err != nil {
		_ = s.repo.InvalidateAuthEmailToken(ctx, tokenHash)
		return nil, StatusError{Status: fiber.StatusServiceUnavailable, Code: "EMAIL_DELIVERY_FAILED", Message: "Chưa thể gửi email xác minh, vui lòng thử lại sau"}
	}
	return Record{"verification_sent": true, "pending_recovery_email_masked": maskEmail(recoveryEmail)}, nil
}

func (s *Service) VerifyRecoveryEmail(ctx context.Context, req VerifyRecoveryEmailRequest) (Record, error) {
	if !s.accountRecoveryReady() {
		return nil, recoveryUnavailableError()
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		return nil, invalidRecoveryTokenError()
	}
	if err := s.repo.VerifyRecoveryEmail(ctx, hashSessionToken(token)); err != nil {
		if recoveryTokenNotFound(err) {
			return nil, invalidRecoveryTokenError()
		}
		return nil, err
	}
	return Record{"recovery_email_verified": true}, nil
}

func (s *Service) RemoveRecoveryEmail(ctx context.Context, userID, actorName string, req RemoveRecoveryEmailRequest) (Record, error) {
	if !s.accountRecoveryReady() {
		return nil, recoveryUnavailableError()
	}
	account, err := s.repo.GetByID(ctx, "users", userID)
	if err != nil {
		return nil, err
	}
	currentPasswordHash := fmt.Sprint(account["password_hash"])
	if !passwordMatches(currentPasswordHash, req.CurrentPassword) {
		return nil, validationError("Mật khẩu hiện tại không đúng", Record{"field": "current_password"})
	}
	if err := s.repo.RemoveRecoveryEmail(ctx, userID, actorName, currentPasswordHash); err != nil {
		if recoveryTokenNotFound(err) {
			return nil, validationError("Tài khoản đã thay đổi, vui lòng xác nhận lại mật khẩu hiện tại", Record{"field": "current_password"})
		}
		return nil, err
	}
	return Record{"recovery_email_removed": true}, nil
}

func (s *Service) ForgotPassword(ctx context.Context, req ForgotPasswordRequest) (Record, error) {
	started := time.Now()
	defer func() {
		if remaining := 150*time.Millisecond - time.Since(started); remaining > 0 {
			time.Sleep(remaining)
		}
	}()
	accepted := Record{"accepted": true, "message": forgotPasswordAcceptedMessage}
	loginEmail, err := normalizeProfileEmail(req.Email)
	if err != nil {
		return accepted, nil
	}
	if err := s.repo.CreateDeveloperRecoveryRequest(ctx, loginEmail); err != nil {
		return nil, err
	}
	return accepted, nil
}

func (s *Service) ResetPassword(ctx context.Context, req ResetPasswordRequest) (Record, error) {
	if strings.TrimSpace(req.Token) == "" {
		return nil, invalidRecoveryTokenError()
	}
	if err := validateNewPassword(req.NewPassword, req.NewPasswordConfirmation); err != nil {
		return nil, err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ResetPasswordWithToken(ctx, hashSessionToken(strings.TrimSpace(req.Token)), string(passwordHash), req.NewPassword); err != nil {
		if recoveryTokenNotFound(err) {
			return nil, invalidRecoveryTokenError()
		}
		return nil, err
	}
	return Record{"password_reset": true, "reauth_required": true}, nil
}

func (s *Service) accountRecoveryReady() bool {
	return s.recoveryEnabled && s.mailSender != nil && s.frontendOrigin != ""
}

func (s *Service) developerRecoveryReady() bool {
	return s.frontendOrigin != ""
}

func normalizeRecoveryEmail(raw string) (string, error) {
	email, err := normalizeProfileEmail(raw)
	if err != nil {
		return "", validationError("Email khôi phục không hợp lệ", Record{"field": "recovery_email"})
	}
	return email, nil
}

func recoveryUnavailableError() StatusError {
	return StatusError{Status: fiber.StatusServiceUnavailable, Code: "RECOVERY_UNAVAILABLE", Message: "Tính năng khôi phục tài khoản chưa sẵn sàng"}
}

func invalidRecoveryTokenError() StatusError {
	return StatusError{Status: fiber.StatusBadRequest, Code: "INVALID_OR_EXPIRED_TOKEN", Message: "Liên kết không hợp lệ, đã hết hạn hoặc đã được sử dụng"}
}
