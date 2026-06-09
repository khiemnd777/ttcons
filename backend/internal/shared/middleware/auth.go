package middleware

import (
	"strings"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID string   `json:"user_id"`
	Email  string   `json:"email"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

func AuthRequired(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			return response.Fail(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Bạn cần đăng nhập để thực hiện thao tác này")
		}

		tokenText := strings.TrimPrefix(header, "Bearer ")
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenText, claims, func(token *jwt.Token) (any, error) {
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			return response.Fail(c, fiber.StatusUnauthorized, "INVALID_TOKEN", "Phiên đăng nhập không hợp lệ hoặc đã hết hạn")
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("roles", claims.Roles)
		return c.Next()
	}
}
