package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func (s *Service) UpdateRole(ctx context.Context, roleID string, body Record) (Record, error) {
	externalOnly, err := s.repo.RoleExternalOnly(ctx, roleID)
	if err != nil {
		return nil, err
	}
	if externalOnly {
		return nil, validationError("Vai trò đối tác ngoài App chỉ được xem và không thể chỉnh sửa", Record{"role_id": roleID})
	}
	return s.repo.Update(ctx, "roles", roleColumns(), roleID, body)
}

func (s *Service) Login(ctx context.Context, req LoginRequest, userAgent string, ipAddress string) (Record, error) {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		return nil, errors.New("Email và mật khẩu là bắt buộc")
	}
	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	hash := fmt.Sprint(user["password_hash"])
	if !passwordMatches(hash, req.Password) {
		return nil, sql.ErrNoRows
	}
	roles, err := s.repo.UserRoles(ctx, fmt.Sprint(user["id"]))
	if err != nil {
		return nil, err
	}
	permissions, err := s.repo.UserPermissions(ctx, fmt.Sprint(user["id"]))
	if err != nil {
		return nil, err
	}
	profile, err := s.repo.GetOwnProfile(ctx, fmt.Sprint(user["id"]))
	if err != nil {
		return nil, err
	}
	s.shapeOwnProfile(profile)
	refreshToken, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	accessTokenID := uuid.NewString()
	refreshExpiresAt := time.Now().Add(s.refreshTokenTTL)
	session, err := s.repo.CreateAuthSession(ctx, fmt.Sprint(user["id"]), hashSessionToken(refreshToken), accessTokenID, userAgent, ipAddress, refreshExpiresAt)
	if err != nil {
		return nil, err
	}
	token, accessExpiresAt, err := s.signToken(fmt.Sprint(user["id"]), fmt.Sprint(user["email"]), roles, fmt.Sprint(session["id"]), accessTokenID)
	if err != nil {
		return nil, err
	}
	profile["roles"] = roles
	profile["permissions"] = permissions
	return Record{
		"token":              token,
		"access_expires_at":  accessExpiresAt.Format(time.RFC3339),
		"refresh_token":      refreshToken,
		"refresh_expires_at": refreshExpiresAt.Format(time.RFC3339),
		"user":               profile,
	}, nil
}

func (s *Service) Me(ctx context.Context, userID string) (Record, error) {
	user, err := s.repo.GetOwnProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	s.shapeOwnProfile(user)
	roles, _ := s.repo.UserRoles(ctx, userID)
	permissions, _ := s.repo.UserPermissions(ctx, userID)
	user["roles"] = roles
	user["permissions"] = permissions
	return user, nil
}

func (s *Service) UpdateOwnProfile(ctx context.Context, userID, actorName string, req UpdateOwnProfileRequest) (Record, error) {
	fullName := strings.Join(strings.Fields(req.FullName), " ")
	if count := len([]rune(fullName)); count < 2 || count > 120 {
		return nil, validationError("Họ tên phải có từ 2 đến 120 ký tự", Record{"field": "full_name"})
	}
	email, err := normalizeProfileEmail(req.Email)
	if err != nil {
		return nil, err
	}
	phone, err := normalizeProfilePhone(req.Phone)
	if err != nil {
		return nil, err
	}

	account, err := s.repo.GetByID(ctx, "users", userID)
	if err != nil {
		return nil, err
	}
	emailChanged := fmt.Sprint(account["email"]) != email
	if emailChanged && !passwordMatches(fmt.Sprint(account["password_hash"]), req.CurrentPassword) {
		return nil, validationError("Mật khẩu hiện tại không đúng", Record{"field": "current_password"})
	}

	reauthRequired, err := s.repo.UpdateOwnProfile(ctx, userID, actorName, fullName, email, phone)
	if err != nil {
		return nil, err
	}
	profile, err := s.Me(ctx, userID)
	if err != nil {
		return nil, err
	}
	return Record{"user": profile, "reauth_required": reauthRequired}, nil
}

func (s *Service) ChangeOwnPassword(ctx context.Context, userID, actorName string, req ChangeOwnPasswordRequest) (Record, error) {
	account, err := s.repo.GetByID(ctx, "users", userID)
	if err != nil {
		return nil, err
	}
	currentHash := fmt.Sprint(account["password_hash"])
	if !passwordMatches(currentHash, req.CurrentPassword) {
		return nil, validationError("Mật khẩu hiện tại không đúng", Record{"field": "current_password"})
	}
	if err := validateNewPassword(req.NewPassword, req.NewPasswordConfirmation); err != nil {
		return nil, err
	}
	if passwordMatches(currentHash, req.NewPassword) {
		return nil, validationError("Mật khẩu mới phải khác mật khẩu hiện tại", Record{"field": "new_password"})
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ChangeOwnPassword(ctx, userID, actorName, string(passwordHash)); err != nil {
		return nil, err
	}
	return Record{"password_changed": true, "reauth_required": true}, nil
}

func passwordMatches(hash, password string) bool {
	if password == "" {
		return false
	}
	if strings.HasPrefix(hash, "demo:") {
		return strings.TrimPrefix(hash, "demo:") == password
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func normalizeProfileEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > 254 || strings.ContainsAny(email, " \t\r\n") {
		return "", validationError("Email đăng nhập không hợp lệ", Record{"field": "email"})
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return "", validationError("Email đăng nhập không hợp lệ", Record{"field": "email"})
	}
	return email, nil
}

func normalizeProfilePhone(raw string) (string, error) {
	phone := strings.TrimSpace(raw)
	if phone == "" {
		return "", nil
	}
	phone = strings.NewReplacer(" ", "", "-", "", ".", "", "(", "", ")", "").Replace(phone)
	digits := phone
	if strings.HasPrefix(digits, "+") {
		digits = strings.TrimPrefix(digits, "+")
	}
	if len(digits) < 8 || len(digits) > 15 {
		return "", validationError("Số điện thoại phải có từ 8 đến 15 chữ số", Record{"field": "phone"})
	}
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return "", validationError("Số điện thoại chỉ được gồm chữ số và dấu + ở đầu", Record{"field": "phone"})
		}
	}
	return phone, nil
}

func (s *Service) RefreshSession(ctx context.Context, refreshToken string, userAgent string, ipAddress string) (Record, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, sql.ErrNoRows
	}
	nextRefreshToken, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	accessTokenID := uuid.NewString()
	refreshExpiresAt := time.Now().Add(s.refreshTokenTTL)
	session, err := s.repo.RotateAuthSession(ctx, hashSessionToken(refreshToken), hashSessionToken(nextRefreshToken), accessTokenID, userAgent, ipAddress, refreshExpiresAt)
	if err != nil {
		return nil, err
	}
	userID := fmt.Sprint(session["user_id"])
	user, err := s.Me(ctx, userID)
	if err != nil {
		return nil, err
	}
	roles, _ := user["roles"].([]string)
	token, accessExpiresAt, err := s.signToken(userID, fmt.Sprint(user["email"]), roles, fmt.Sprint(session["id"]), accessTokenID)
	if err != nil {
		return nil, err
	}
	return Record{
		"token":              token,
		"access_expires_at":  accessExpiresAt.Format(time.RFC3339),
		"refresh_token":      nextRefreshToken,
		"refresh_expires_at": refreshExpiresAt.Format(time.RFC3339),
		"user":               user,
	}, nil
}

func (s *Service) LogoutByRefreshToken(ctx context.Context, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil
	}
	return s.repo.RevokeAuthSessionByRefreshHash(ctx, hashSessionToken(refreshToken))
}

func (s *Service) signToken(userID, email string, roles []string, sessionID string, accessTokenID string) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.accessTokenTTL)
	claims := jwt.MapClaims{
		"user_id":    userID,
		"email":      email,
		"roles":      roles,
		"session_id": sessionID,
		"jti":        accessTokenID,
		"exp":        expiresAt.Unix(),
		"iat":        time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.jwtSecret))
	return signed, expiresAt, err
}

func newSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashSessionToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
