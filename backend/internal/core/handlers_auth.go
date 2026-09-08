package core

import (
	"consflow/backend/internal/shared/response"
	"encoding/json"
	"fmt"
	fiber "github.com/gofiber/fiber/v2"
	"time"
)

func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đăng nhập không hợp lệ")
	}
	data, err := h.service.Login(c.Context(), req, c.Get("User-Agent"), c.IP())
	if err != nil {
		return response.Fail(c, fiber.StatusUnauthorized, "INVALID_CREDENTIALS", "Email hoặc mật khẩu không đúng")
	}
	h.setRefreshCookie(c, fmt.Sprint(data["refresh_token"]), fmt.Sprint(data["refresh_expires_at"]))
	delete(data, "refresh_token")
	delete(data, "refresh_expires_at")
	return response.OK(c, data, "Đăng nhập thành công")
}

func (h *Handler) Refresh(c *fiber.Ctx) error {
	data, err := h.service.RefreshSession(c.Context(), c.Cookies(refreshCookieName), c.Get("User-Agent"), c.IP())
	if err != nil {
		h.clearRefreshCookie(c)
		return response.Fail(c, fiber.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Phiên đăng nhập đã hết hạn, vui lòng đăng nhập lại")
	}
	h.setRefreshCookie(c, fmt.Sprint(data["refresh_token"]), fmt.Sprint(data["refresh_expires_at"]))
	delete(data, "refresh_token")
	delete(data, "refresh_expires_at")
	return response.OK(c, data)
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	if err := h.service.LogoutByRefreshToken(c.Context(), c.Cookies(refreshCookieName)); err != nil {
		return err
	}
	h.clearRefreshCookie(c)
	return response.OK(c, Record{"logged_out": true})
}

func (h *Handler) Me(c *fiber.Ctx) error {
	user, err := h.service.Me(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, user)
}

func (h *Handler) UpdateOwnProfile(c *fiber.Ctx) error {
	var req UpdateOwnProfileRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu hồ sơ cá nhân không hợp lệ")
	}
	result, err := h.service.UpdateOwnProfile(c.Context(), actorID(c), actorName(c), req)
	if err != nil {
		return err
	}
	if boolValue(result["reauth_required"]) {
		h.clearRefreshCookie(c)
	}
	return response.OK(c, result, "Đã cập nhật hồ sơ cá nhân")
}

func (h *Handler) UploadOwnAvatar(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Ảnh đại diện là bắt buộc")
	}
	user, err := h.service.UpdateOwnAvatar(c.Context(), actorID(c), actorName(c), fileHeader)
	if err != nil {
		return err
	}
	return response.OK(c, user, "Đã cập nhật ảnh đại diện")
}

func (h *Handler) GetOwnAvatar(c *fiber.Ctx) error {
	file, err := h.service.OwnAvatarFile(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	return sendProtectedFile(c, file, "anh-dai-dien.png")
}

func (h *Handler) ChangeOwnPassword(c *fiber.Ctx) error {
	var req ChangeOwnPasswordRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đổi mật khẩu không hợp lệ")
	}
	result, err := h.service.ChangeOwnPassword(c.Context(), actorID(c), actorName(c), req)
	if err != nil {
		return err
	}
	h.clearRefreshCookie(c)
	return response.OK(c, result, "Đã đổi mật khẩu")
}

func (h *Handler) UpdateRecoveryEmail(c *fiber.Ctx) error {
	var req UpdateRecoveryEmailRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu email khôi phục không hợp lệ")
	}
	result, err := h.service.RequestRecoveryEmailVerification(c.Context(), actorID(c), req)
	if err != nil {
		return err
	}
	return response.OK(c, result, "Đã gửi email xác minh")
}

func (h *Handler) RemoveRecoveryEmail(c *fiber.Ctx) error {
	var req RemoveRecoveryEmailRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu gỡ email khôi phục không hợp lệ")
	}
	result, err := h.service.RemoveRecoveryEmail(c.Context(), actorID(c), actorName(c), req)
	if err != nil {
		return err
	}
	return response.OK(c, result, "Đã gỡ email khôi phục")
}

func (h *Handler) VerifyRecoveryEmail(c *fiber.Ctx) error {
	var req VerifyRecoveryEmailRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xác minh không hợp lệ")
	}
	result, err := h.service.VerifyRecoveryEmail(c.Context(), req)
	if err != nil {
		return err
	}
	return response.OK(c, result, "Đã xác minh email khôi phục")
}

func (h *Handler) ForgotPassword(c *fiber.Ctx) error {
	var req ForgotPasswordRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu yêu cầu khôi phục không hợp lệ")
	}
	return response.OK(c, h.service.ForgotPassword(c.Context(), req), forgotPasswordAcceptedMessage)
}

func (h *Handler) ResetPassword(c *fiber.Ctx) error {
	var req ResetPasswordRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đặt lại mật khẩu không hợp lệ")
	}
	result, err := h.service.ResetPassword(c.Context(), req)
	if err != nil {
		return err
	}
	h.clearRefreshCookie(c)
	return response.OK(c, result, "Đã đặt lại mật khẩu")
}

func (h *Handler) setRefreshCookie(c *fiber.Ctx, token string, expiresAtText string) {
	expiresAt, err := time.Parse(time.RFC3339, expiresAtText)
	if err != nil {
		expiresAt = time.Now().Add(h.service.refreshTokenTTL)
	}
	c.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Expires:  expiresAt,
		HTTPOnly: true,
		Secure:   h.service.refreshCookieSecure,
		SameSite: h.service.refreshCookieSameSite,
		Path:     "/api/auth",
	})
}

func (h *Handler) clearRefreshCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   h.service.refreshCookieSecure,
		SameSite: h.service.refreshCookieSameSite,
		Path:     "/api/auth",
	})
}
