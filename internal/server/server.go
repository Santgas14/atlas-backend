// Package server — HTTP server com Fiber, rotas e middlewares.
package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/atlab-ufc/atlab-backend/internal/config"
	"github.com/atlab-ufc/atlab-backend/internal/handlers"
	"github.com/atlab-ufc/atlab-backend/internal/middleware"
)

// New cria e configura o servidor Fiber com todas as rotas.
func New(cfg *config.Config, db *pgxpool.Pool) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "ATLAB Platform",
		ServerHeader: "ATLAB",
		BodyLimit:    10 * 1024 * 1024, // 10MB
	})

	// ─── Middlewares globais ──────────────────────────────────
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} | ${status} | ${latency} | ${method} ${path}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "https://atlab.ufc.br,http://localhost:5173,http://localhost:3000",
		AllowCredentials: true,
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
	}))

	// ─── Health check ────────────────────────────────────────
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "atlab-backend"})
	})

	// ─── Auth routes (public) ────────────────────────────────
	auth := handlers.NewAuthHandler(cfg, db)
	app.Get("/api/auth/login", auth.LoginRedirect)
	app.Get("/api/auth/callback", auth.Callback)
	app.Post("/api/auth/refresh", auth.Refresh)
	app.Post("/api/auth/logout", auth.Logout)

	// ─── Protected API routes ────────────────────────────────
	api := app.Group("/api", middleware.RequireAuth(cfg.JWTSecret))

	// User / Session
	api.Get("/me", auth.Me)

	// Machines
	machines := handlers.NewMachineHandler(cfg, db)
	api.Get("/machines", machines.List)
	api.Get("/machines/:id", machines.Get)
	api.Get("/machines/:id/metrics", machines.Metrics)
	api.Post("/machines/:id/power", middleware.RequireRole("admin"), machines.Power)

	// Proxmox
	proxmox := handlers.NewProxmoxHandler(cfg, db)
	api.Get("/proxmox/nodes", proxmox.ListNodes)
	api.Get("/proxmox/nodes/:id/vms", proxmox.ListVMs)
	api.Post("/proxmox/provision", middleware.RequireRole("admin"), proxmox.Provision)

	// IPAM
	ipam := handlers.NewIPAMHandler(db)
	api.Get("/ipam/subnets", ipam.ListSubnets)
	api.Get("/ipam/subnets/:id/allocations", ipam.ListAllocations)
	api.Post("/ipam/allocate", middleware.RequireRole("admin", "devops"), ipam.Allocate)

	// SSH WebSocket (handled separately)
	ssh := handlers.NewSSHHandler(cfg, db)
	api.Get("/ssh/connect/:machineId", ssh.Connect)

	// Alerts
	alerts := handlers.NewAlertHandler(db)
	api.Get("/alerts", alerts.List)
	api.Post("/alerts/:id/ack", alerts.Acknowledge)

	// Activity log
	api.Get("/activity", handlers.NewActivityHandler(db).List)

	// Groups
	groups := handlers.NewGroupHandler(db)
	api.Get("/groups", groups.List)
	api.Post("/groups", middleware.RequireRole("admin"), groups.Create)
	api.Put("/groups/:id", middleware.RequireRole("admin"), groups.Update)

	// Credentials
	creds := handlers.NewCredentialHandler(db)
	api.Get("/credentials", middleware.RequireRole("admin", "devops"), creds.List)
	api.Post("/credentials", middleware.RequireRole("admin"), creds.Create)

	// Automation
	tasks := handlers.NewAutomationHandler(db)
	api.Get("/automation/tasks", tasks.List)
	api.Post("/automation/tasks/:id/run", middleware.RequireRole("admin", "devops"), tasks.Run)

	// Reports
	api.Get("/reports/:type", handlers.NewReportHandler(db).Generate)

	// Notifications config
	api.Get("/notifications/channels", handlers.NewNotificationHandler(cfg, db).ListChannels)
	api.Put("/notifications/channels/:id", middleware.RequireRole("admin"), handlers.NewNotificationHandler(cfg, db).UpdateChannel)

	return app
}
