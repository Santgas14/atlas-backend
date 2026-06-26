// Package middleware — Middlewares HTTP (auth, RBAC, rate limiting).
package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// Claims armazena os dados do JWT emitido após login OIDC.
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// RequireAuth verifica o JWT no header Authorization.
func RequireAuth(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if header == "" {
			return c.Status(401).JSON(fiber.Map{"error": "Token não fornecido"})
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		if tokenStr == header {
			return c.Status(401).JSON(fiber.Map{"error": "Formato inválido (use Bearer)"})
		}

		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			return c.Status(401).JSON(fiber.Map{"error": "Token inválido ou expirado"})
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "Claims inválidos"})
		}

		// Armazena no contexto para handlers usarem
		c.Locals("user_id", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("role", claims.Role)
		c.Locals("name", claims.Name)

		return c.Next()
	}
}

// RequireRole verifica se o usuário tem uma das roles permitidas.
func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole, _ := c.Locals("role").(string)
		for _, r := range roles {
			if userRole == r {
				return c.Next()
			}
		}
		return c.Status(403).JSON(fiber.Map{
			"error": "Permissão negada",
			"required": roles,
			"your_role": userRole,
		})
	}
}
