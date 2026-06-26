package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AlertHandler struct {
	db *pgxpool.Pool
}

func NewAlertHandler(db *pgxpool.Pool) *AlertHandler {
	return &AlertHandler{db: db}
}

func (h *AlertHandler) List(c *fiber.Ctx) error {
	// TODO: SELECT * FROM alerts ORDER BY created_at DESC LIMIT 100
	return c.JSON(fiber.Map{"alerts": []interface{}{}})
}

func (h *AlertHandler) Acknowledge(c *fiber.Ctx) error {
	id := c.Params("id")
	userID, _ := c.Locals("user_id").(string)
	_ = id
	_ = userID
	// TODO: UPDATE alerts SET acknowledged=true, acknowledged_by=$1, acknowledged_at=NOW() WHERE id=$2
	return c.JSON(fiber.Map{"message": "acknowledged"})
}

// ─── Activity Handler ─────────────────────────────────────────

type ActivityHandler struct {
	db *pgxpool.Pool
}

func NewActivityHandler(db *pgxpool.Pool) *ActivityHandler {
	return &ActivityHandler{db: db}
}

func (h *ActivityHandler) List(c *fiber.Ctx) error {
	// TODO: SELECT * FROM activity_log ORDER BY created_at DESC LIMIT 100
	return c.JSON(fiber.Map{"events": []interface{}{}})
}

// ─── Group Handler ────────────────────────────────────────────

type GroupHandler struct {
	db *pgxpool.Pool
}

func NewGroupHandler(db *pgxpool.Pool) *GroupHandler {
	return &GroupHandler{db: db}
}

func (h *GroupHandler) List(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"groups": []interface{}{}})
}

func (h *GroupHandler) Create(c *fiber.Ctx) error {
	return c.Status(201).JSON(fiber.Map{"message": "created"})
}

func (h *GroupHandler) Update(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"message": "updated"})
}

// ─── Credential Handler ───────────────────────────────────────

type CredentialHandler struct {
	db *pgxpool.Pool
}

func NewCredentialHandler(db *pgxpool.Pool) *CredentialHandler {
	return &CredentialHandler{db: db}
}

func (h *CredentialHandler) List(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"credentials": []interface{}{}})
}

func (h *CredentialHandler) Create(c *fiber.Ctx) error {
	return c.Status(201).JSON(fiber.Map{"message": "created"})
}

// ─── Automation Handler ───────────────────────────────────────

type AutomationHandler struct {
	db *pgxpool.Pool
}

func NewAutomationHandler(db *pgxpool.Pool) *AutomationHandler {
	return &AutomationHandler{db: db}
}

func (h *AutomationHandler) List(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"tasks": []interface{}{}})
}

func (h *AutomationHandler) Run(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"message": "task started"})
}

// ─── Report Handler ───────────────────────────────────────────

type ReportHandler struct {
	db *pgxpool.Pool
}

func NewReportHandler(db *pgxpool.Pool) *ReportHandler {
	return &ReportHandler{db: db}
}

func (h *ReportHandler) Generate(c *fiber.Ctx) error {
	reportType := c.Params("type")
	_ = reportType
	// TODO: gerar CSV/JSON conforme tipo
	return c.JSON(fiber.Map{"report": reportType, "data": nil})
}

// ─── Notification Handler ─────────────────────────────────────

type NotificationHandler struct {
	cfg *config.Config
	db  *pgxpool.Pool
}

func NewNotificationHandler(cfg *config.Config, db *pgxpool.Pool) *NotificationHandler {
	return &NotificationHandler{cfg: cfg, db: db}
}

func (h *NotificationHandler) ListChannels(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"channels": []interface{}{}})
}

func (h *NotificationHandler) UpdateChannel(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"message": "updated"})
}
