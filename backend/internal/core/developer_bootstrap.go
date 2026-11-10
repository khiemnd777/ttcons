package core

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"consflow/backend/internal/shared/mailer"
	"consflow/backend/internal/shared/response"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

const developerBootstrapEmail = "khiemnd777@gmail.com"

type bootstrapQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func developerExists(ctx context.Context, db bootstrapQuery) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users u WHERE `+developerProjectSupportUserSQL("u.id")+`)`).Scan(&exists)
	return exists, err
}

func bootstrapGate(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		exists, err := developerExists(c.UserContext(), db)
		if err != nil {
			return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không kiểm tra được trạng thái hệ thống. Vui lòng thử lại.")
		}
		if !exists {
			return response.Fail(c, 423, "BOOTSTRAP_REQUIRED", "Hệ thống đang chờ thiết lập Developer.")
		}
		return c.Next()
	}
}

func (h *Handler) DeveloperBootstrapStatus(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	exists, err := developerExists(c.UserContext(), h.service.repo.db)
	if err != nil {
		return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không kiểm tra được trạng thái hệ thống. Vui lòng thử lại.")
	}
	return response.OK(c, fiber.Map{"required": !exists})
}

func bootstrapMAC(key []byte, code string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("consflow/developer-bootstrap/v1:" + code))
	return mac.Sum(nil)
}

func (h *Handler) SendDeveloperBootstrapOTP(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	s := h.service
	if s.bootstrapSender == nil || len(s.bootstrapKey) < 32 {
		return response.Fail(c, 503, "BOOTSTRAP_MAIL_UNAVAILABLE", "Dịch vụ gửi mã chưa sẵn sàng. Vui lòng liên hệ người vận hành.")
	}
	ctx := c.UserContext()
	tx, err := s.repo.db.BeginTx(ctx, nil)
	if err != nil {
		return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không thể gửi mã. Vui lòng thử lại.")
	}
	defer tx.Rollback()
	var sentAt sql.NullTime
	if err = tx.QueryRowContext(ctx, `SELECT sent_at FROM developer_bootstrap_otp WHERE singleton FOR UPDATE`).Scan(&sentAt); err != nil {
		return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không thể gửi mã. Vui lòng thử lại.")
	}
	exists, err := developerExists(ctx, tx)
	if err != nil {
		return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không thể kiểm tra hệ thống.")
	}
	if exists {
		return response.Fail(c, 409, "BOOTSTRAP_COMPLETE", "Hệ thống đã được thiết lập. Vui lòng đăng nhập.")
	}
	var emailExists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE lower(email)=lower($1))`, developerBootstrapEmail).Scan(&emailExists); err != nil {
		return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không thể kiểm tra tài khoản.")
	}
	if emailExists {
		return response.Fail(c, 409, "BOOTSTRAP_EMAIL_CONFLICT", "Email Developer đã thuộc một tài khoản. Cần người vận hành kiểm tra.")
	}
	if sentAt.Valid && time.Since(sentAt.Time) < time.Minute {
		return response.Fail(c, 429, "BOOTSTRAP_COOLDOWN", "Vui lòng đợi 60 giây trước khi gửi lại mã.")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(100000000))
	if err != nil {
		return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không thể tạo mã. Vui lòng thử lại.")
	}
	code := fmt.Sprintf("%08d", n.Int64())
	// A committed pending challenge invalidates the old code before SMTP. A crash
	// after delivery leaves this new code unusable until a later retry.
	codeMAC := bootstrapMAC(s.bootstrapKey, code)
	_, err = tx.ExecContext(ctx, `UPDATE developer_bootstrap_otp SET code_mac=$1, expires_at=now()+interval '10 minutes', sent_at=now(), attempts=0, delivery_pending=true WHERE singleton`, codeMAC)
	if err != nil || tx.Commit() != nil {
		return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không thể hoàn tất gửi mã. Vui lòng thử lại.")
	}
	sendErr := s.bootstrapSender.Send(ctx, mailer.Message{To: developerBootstrapEmail, Subject: "Mã xác minh thiết lập Consflow", Text: "Mã xác minh thiết lập Developer: " + code + "\nMã có hiệu lực 10 phút và chỉ sử dụng một lần."})
	if sendErr != nil {
		_, _ = s.repo.db.ExecContext(ctx, `UPDATE developer_bootstrap_otp SET code_mac=NULL,expires_at=NULL,delivery_pending=false WHERE singleton AND code_mac=$1 AND delivery_pending`, codeMAC)
		return response.Fail(c, 503, "BOOTSTRAP_MAIL_UNAVAILABLE", "Không gửi được mã xác minh. Vui lòng kiểm tra cấu hình email và thử lại.")
	}
	result, err := s.repo.db.ExecContext(ctx, `UPDATE developer_bootstrap_otp SET delivery_pending=false WHERE singleton AND code_mac=$1 AND delivery_pending`, codeMAC)
	if err != nil {
		return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không thể hoàn tất gửi mã. Vui lòng thử lại.")
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return response.Fail(c, 409, "BOOTSTRAP_SUPERSEDED", "Mã đã được thay thế. Vui lòng sử dụng mã mới nhất.")
	}
	return response.OK(c, fiber.Map{"sent": true})
}

type developerBootstrapRequest struct {
	OTP                  string `json:"otp"`
	FullName             string `json:"full_name"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

func (h *Handler) CompleteDeveloperBootstrap(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	var req developerBootstrapRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, 400, "VALIDATION_ERROR", "Dữ liệu không hợp lệ.")
	}
	req.FullName = strings.TrimSpace(req.FullName)
	if utf8.RuneCountInString(req.FullName) < 2 || utf8.RuneCountInString(req.FullName) > 160 || len(req.OTP) != 8 || strings.Trim(req.OTP, "0123456789") != "" || validateNewPassword(req.Password, req.PasswordConfirmation) != nil {
		return response.Fail(c, 400, "VALIDATION_ERROR", "Vui lòng kiểm tra mã xác minh, họ tên và mật khẩu.")
	}
	s := h.service
	if len(s.bootstrapKey) < 32 {
		return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Thiết lập chưa sẵn sàng. Vui lòng liên hệ người vận hành.")
	}
	ctx := c.UserContext()
	tx, err := s.repo.db.BeginTx(ctx, nil)
	if err != nil {
		return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không thể hoàn tất thiết lập.")
	}
	defer tx.Rollback()
	var stored []byte
	var expiry sql.NullTime
	var attempts int
	var pending bool
	if err = tx.QueryRowContext(ctx, `SELECT code_mac,expires_at,attempts,delivery_pending FROM developer_bootstrap_otp WHERE singleton FOR UPDATE`).Scan(&stored, &expiry, &attempts, &pending); err != nil {
		return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không thể hoàn tất thiết lập.")
	}
	exists, err := developerExists(ctx, tx)
	if err != nil {
		return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không thể kiểm tra hệ thống.")
	}
	if exists {
		return response.Fail(c, 409, "BOOTSTRAP_COMPLETE", "Hệ thống đã được thiết lập. Vui lòng đăng nhập.")
	}
	var emailExists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE lower(email)=lower($1))`, developerBootstrapEmail).Scan(&emailExists); err != nil {
		return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không thể kiểm tra tài khoản.")
	}
	if emailExists {
		return response.Fail(c, 409, "BOOTSTRAP_EMAIL_CONFLICT", "Email Developer đã thuộc một tài khoản. Cần người vận hành kiểm tra.")
	}
	if pending || attempts >= 5 || !expiry.Valid || !time.Now().Before(expiry.Time) || !hmac.Equal(stored, bootstrapMAC(s.bootstrapKey, req.OTP)) {
		if _, err = tx.ExecContext(ctx, `UPDATE developer_bootstrap_otp SET attempts=attempts+1 WHERE singleton`); err != nil || tx.Commit() != nil {
			return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không thể xác minh mã.")
		}
		return response.Fail(c, 400, "BOOTSTRAP_INVALID_OTP", "Mã không hợp lệ, đã hết hạn hoặc đã hết số lần thử. Vui lòng yêu cầu mã mới.")
	}
	var roleID string
	err = tx.QueryRowContext(ctx, `SELECT id::text FROM roles WHERE name='developer' AND system_role AND NOT external_only FOR SHARE`).Scan(&roleID)
	if err != nil {
		return response.Fail(c, 503, "BOOTSTRAP_ROLE_UNAVAILABLE", "Vai trò Developer chưa sẵn sàng. Vui lòng liên hệ người vận hành.")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không thể tạo tài khoản.")
	}
	var userID string
	err = tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,full_name,status) VALUES($1,$2,$3,'active') RETURNING id::text`, developerBootstrapEmail, string(hash), req.FullName).Scan(&userID)
	if err == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO staff(user_id,full_name,email,department,position,status) VALUES($1,$2,$3,'Công nghệ','Developer','active')`, userID, req.FullName, developerBootstrapEmail)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO user_roles(user_id,role_id,project_id) VALUES($1,$2,NULL)`, userID, roleID)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(actor_id,actor_name,event_type,target_type,target_id,description,metadata_json) VALUES(NULL,'system:bootstrap-developer','developer_bootstrap_created','user_account',$1,'Khởi tạo tài khoản Developer','{"source":"developer-bootstrap","scope":"global"}'::jsonb)`, userID)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE developer_bootstrap_otp SET code_mac=NULL,expires_at=NULL,attempts=0 WHERE singleton`)
	}
	if err != nil {
		return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không thể tạo tài khoản. Vui lòng liên hệ người vận hành.")
	}
	if err = tx.Commit(); err != nil {
		return response.Fail(c, 503, "BOOTSTRAP_UNAVAILABLE", "Không thể hoàn tất thiết lập.")
	}
	return response.OK(c, fiber.Map{"created": true})
}
