// Package config — Carrega configuração do ambiente.
package config

import "os"

type Config struct {
	Env  string
	Port string

	// Database
	DatabaseURL string
	RedisURL    string

	// Auth (Authentik OIDC)
	AuthIssuer       string
	AuthClientID     string
	AuthClientSecret string
	JWTSecret        string

	// Prometheus
	PrometheusURL string

	// Proxmox
	ProxmoxNodes       string // comma-separated IPs
	ProxmoxTokenID     string
	ProxmoxTokenSecret string

	// Notifications
	EvolutionAPIURL string
	EvolutionAPIKey string
	TelegramToken   string
	TelegramChatID  string
}

// Load lê todas as variáveis de ambiente com prefix ATLAB_.
func Load() *Config {
	return &Config{
		Env:  getEnv("ATLAB_ENV", "development"),
		Port: getEnv("ATLAB_PORT", "8080"),

		DatabaseURL: getEnv("ATLAB_DB_URL", "postgres://atlab:atlab@localhost:5432/atlab?sslmode=disable"),
		RedisURL:    getEnv("ATLAB_REDIS_URL", "redis://localhost:6379/0"),

		AuthIssuer:       getEnv("ATLAB_AUTHENTIK_ISSUER", "https://authentik.atlab.ufc.br/application/o/atlab-platform/"),
		AuthClientID:     getEnv("ATLAB_AUTHENTIK_CLIENT_ID", "atlab-platform"),
		AuthClientSecret: getEnv("ATLAB_AUTHENTIK_CLIENT_SECRET", ""),
		JWTSecret:        getEnv("ATLAB_JWT_SECRET", "dev-secret-change-me"),

		PrometheusURL: getEnv("ATLAB_PROMETHEUS_URL", "http://10.101.53.212:9000"),

		ProxmoxNodes:       getEnv("ATLAB_PROXMOX_NODES", "10.101.53.240,10.101.53.243,10.101.53.247"),
		ProxmoxTokenID:     getEnv("ATLAB_PROXMOX_TOKEN_ID", ""),
		ProxmoxTokenSecret: getEnv("ATLAB_PROXMOX_TOKEN_SECRET", ""),

		EvolutionAPIURL: getEnv("ATLAB_EVOLUTION_API_URL", ""),
		EvolutionAPIKey: getEnv("ATLAB_EVOLUTION_API_KEY", ""),
		TelegramToken:   getEnv("ATLAB_TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:  getEnv("ATLAB_TELEGRAM_CHAT_ID", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
