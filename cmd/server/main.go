// ATLAS Backend — Entry point mínimo (compilável)
// Sobe um servidor HTTP com health check e placeholder de rotas.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	port := getEnv("ATLAB_PORT", "8080")

	app := fiber.New(fiber.Config{
		AppName:      "ATLAS",
		ServerHeader: "ATLAS",
	})

	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowCredentials: false,
	}))

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "atlas-backend",
			"version": "0.1.0",
		})
	})

	// Placeholder API routes
	api := app.Group("/api")

	api.Get("/me", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "auth not implemented yet"})
	})

	api.Get("/machines", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"machines": []interface{}{}, "message": "proxmox integration pending"})
	})

	api.Get("/proxmox/nodes", func(c *fiber.Ctx) error {
		// TODO: query real Proxmox API
		nodes := []fiber.Map{
			{"name": "proxmox-alpha", "host": "10.101.53.240:8006", "status": "pending"},
			{"name": "proxmox-tau", "host": "10.101.53.243:8006", "status": "pending"},
			{"name": "proxmox-redragon", "host": "10.101.53.247:8006", "status": "pending"},
		}
		return c.JSON(fiber.Map{"nodes": nodes})
	})

	api.Get("/alerts", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"alerts": []interface{}{}})
	})

	log.Printf("✓ ATLAS Backend rodando em :%s", port)
	log.Fatal(app.Listen(fmt.Sprintf(":%s", port)))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
