// Package handlers — HTTP handlers da API ATLAB.
package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/atlab-ufc/atlab-backend/internal/config"
	"github.com/atlab-ufc/atlab-backend/internal/middleware"
)

type AuthHandler struct {
	cfg *config.Config
	db  *pgxpool.Pool
}

func NewAuthHandler(cfg *config.Config, db *pgxpool.Pool) *AuthHandler {
	return &AuthHandler{cfg: cfg, db: db}
}

// LoginRedirect redireciona o browser para o Authentik authorize endpoint.
func (h *AuthHandler) LoginRedirect(c *fiber.Ctx) error {
	// Em produção: construir URL do Authentik OIDC
	// GET https://authentik.atlab.ufc.br/application/o/authorize/
	//   ?client_id=atlab-platform
	//   &redirect_uri=https://atlab.ufc.br/api/auth/callback
	//   &response_type=code
	//   &scope=openid+profile+email+groups
	//   &state=<random>
	return c.JSON(fiber.Map{
		"redirect_url": h.cfg.AuthIssuer + "authorize?client_id=" + h.cfg.AuthClientID +
			"&redirect_uri=https://atlab.ufc.br/api/auth/callback" +
			"&response_type=code&scope=openid+profile+email+groups",
	})
}

// Callback recebe o code do Authentik, troca por token, cria sessão.
func (h *AuthHandler) Callback(c *fiber.Ctx) error {
	code := c.Query("code")
	if code == "" {
		return c.Status(400).JSON(fiber.Map{"error": "code não recebido"})
	}

	// TODO: Em produção:
	// 1. Trocar code por access_token via POST ao token endpoint do Authentik
	// 2. Validar id_token
	// 3. Extrair claims (email, name, groups)
	// 4. Upsert user no banco
	// 5. Gerar JWT interno

	// Placeholder: gerar JWT de exemplo
	token := h.generateJWT("placeholder-id", "admin@atlab.local", "Administrador", "admin")

	// Redirect para frontend com token
	return c.Redirect("https://atlab.ufc.br/auth/callback?token=" + token)
}

// Me retorna dados do usuário logado.
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"id":    c.Locals("user_id"),
		"email": c.Locals("email"),
		"name":  c.Locals("name"),
		"role":  c.Locals("role"),
	})
}

// Refresh renova o JWT (se ainda válido).
func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	// TODO: validar refresh token, emitir novo JWT
	return c.JSON(fiber.Map{"message": "not implemented yet"})
}

// Logout invalida a sessão.
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	// TODO: invalidar token no Redis (blacklist)
	return c.JSON(fiber.Map{"message": "logged out"})
}

func (h *AuthHandler) generateJWT(userID, email, name, role string) string {
	claims := middleware.Claims{
		UserID: userID,
		Email:  email,
		Name:   name,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "atlab-backend",
		},
	}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(h.cfg.JWTSecret))
	return token
}
