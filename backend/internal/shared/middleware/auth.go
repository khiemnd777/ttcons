package middleware

import (
	"context"
	"fmt"
	"strings"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID    string   `json:"user_id"`
	Email     string   `json:"email"`
	Roles     []string `json:"roles"`
	SessionID string   `json:"session_id"`
	jwt.RegisteredClaims
}

type SessionValidator interface {
	AuthSessionActive(ctx context.Context, userID, sessionID, accessTokenID string) (bool, error)
}

func AuthRequired(secret string, validators ...SessionValidator) fiber.Handler {
	var validator SessionValidator
	if len(validators) > 0 {
		validator = validators[0]
	}
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			return response.Fail(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Bạn cần đăng nhập để thực hiện thao tác này")
		}

		tokenText := strings.TrimPrefix(header, "Bearer ")
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenText, claims, func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			return response.Fail(c, fiber.StatusUnauthorized, "INVALID_TOKEN", "Phiên đăng nhập không hợp lệ hoặc đã hết hạn")
		}
		if validator != nil {
			if claims.UserID == "" || claims.SessionID == "" || claims.ID == "" {
				return response.Fail(c, fiber.StatusUnauthorized, "INVALID_TOKEN", "Phiên đăng nhập không hợp lệ hoặc đã hết hạn")
			}
			active, err := validator.AuthSessionActive(c.Context(), claims.UserID, claims.SessionID, claims.ID)
			if err != nil {
				return err
			}
			if !active {
				return response.Fail(c, fiber.StatusUnauthorized, "INVALID_TOKEN", "Phiên đăng nhập không hợp lệ hoặc đã hết hạn")
			}
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("roles", claims.Roles)
		c.Locals("session_id", claims.SessionID)
		c.Locals("token_id", claims.ID)
		return c.Next()
	}
}
