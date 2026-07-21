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
	return func(c *fiber.Ctx) error {
		verified, err := verifyAuthClaims(c, secret)
		if err != nil || !verified {
			return err
		}
		active, err := validateAuthSession(c, firstSessionValidator(validators))
		if err != nil || !active {
			return err
		}
		return c.Next()
	}
}

func AuthClaimsRequired(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		verified, err := verifyAuthClaims(c, secret)
		if err != nil || !verified {
			return err
		}
		return c.Next()
	}
}

func AuthSessionRequired(validators ...SessionValidator) fiber.Handler {
	validator := firstSessionValidator(validators)
	return func(c *fiber.Ctx) error {
		active, err := validateAuthSession(c, validator)
		if err != nil || !active {
			return err
		}
		return c.Next()
	}
}

func verifyAuthClaims(c *fiber.Ctx, secret string) (bool, error) {
	header := c.Get("Authorization")
	if header == "" || !strings.HasPrefix(header, "Bearer ") {
		return false, response.Fail(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Bạn cần đăng nhập để thực hiện thao tác này")
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
		return false, response.Fail(c, fiber.StatusUnauthorized, "INVALID_TOKEN", "Phiên đăng nhập không hợp lệ hoặc đã hết hạn")
	}
	if claims.UserID == "" || claims.SessionID == "" || claims.ID == "" {
		return false, response.Fail(c, fiber.StatusUnauthorized, "INVALID_TOKEN", "Phiên đăng nhập không hợp lệ hoặc đã hết hạn")
	}
	c.Locals("user_id", claims.UserID)
	c.Locals("email", claims.Email)
	c.Locals("roles", claims.Roles)
	c.Locals("session_id", claims.SessionID)
	c.Locals("token_id", claims.ID)
	return true, nil
}

func validateAuthSession(c *fiber.Ctx, validator SessionValidator) (bool, error) {
	if validator == nil {
		return true, nil
	}
	userID := fmt.Sprint(c.Locals("user_id"))
	sessionID := fmt.Sprint(c.Locals("session_id"))
	tokenID := fmt.Sprint(c.Locals("token_id"))
	if userID == "" || sessionID == "" || tokenID == "" {
		return false, response.Fail(c, fiber.StatusUnauthorized, "INVALID_TOKEN", "Phiên đăng nhập không hợp lệ hoặc đã hết hạn")
	}
	active, err := validator.AuthSessionActive(c.Context(), userID, sessionID, tokenID)
	if err != nil {
		return false, err
	}
	if !active {
		return false, response.Fail(c, fiber.StatusUnauthorized, "INVALID_TOKEN", "Phiên đăng nhập không hợp lệ hoặc đã hết hạn")
	}
	return true, nil
}

func firstSessionValidator(validators []SessionValidator) SessionValidator {
	if len(validators) == 0 {
		return nil
	}
	return validators[0]
}
