package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/atlab-ufc/atlab-backend/internal/config"
)

type MachineHandler struct {
	cfg *config.Config
	db  *pgxpool.Pool
}

func NewMachineHandler(cfg *config.Config, db *pgxpool.Pool) *MachineHandler {
	return &MachineHandler{cfg: cfg, db: db}
}

// List retorna todas as máquinas visíveis para o usuário (filtrado por grupo).
func (h *MachineHandler) List(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	userID, _ := c.Locals("user_id").(string)

	// Admin vê tudo, outros filtram por grupo
	_ = role
	_ = userID

	// TODO: query machines com join em group_machines + user_groups
	// SELECT m.* FROM machines m
	// JOIN group_machines gm ON gm.machine_id = m.id
	// JOIN user_groups ug ON ug.group_id = gm.group_id
	// WHERE ug.user_id = $1 OR $2 = 'admin'

	return c.JSON(fiber.Map{"machines": []interface{}{}, "total": 0})
}

// Get retorna detalhes de uma máquina específica.
func (h *MachineHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")
	_ = id
	// TODO: query machine by ID + check permission
	return c.JSON(fiber.Map{"machine": nil})
}

// Metrics retorna métricas em tempo real via query ao Prometheus.
func (h *MachineHandler) Metrics(c *fiber.Ctx) error {
	id := c.Params("id")
	_ = id

	// TODO: query Prometheus API
	// GET http://10.101.53.212:9000/api/v1/query
	//   ?query=node_cpu_seconds_total{instance="10.101.53.X:9100"}
	//
	// Queries úteis:
	// - CPU: 100 - (avg(rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)
	// - RAM: (node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) / node_memory_MemTotal_bytes * 100
	// - Disk: node_filesystem_avail_bytes / node_filesystem_size_bytes
	// - Load: node_load1, node_load5, node_load15
	// - Net: rate(node_network_receive_bytes_total[5m])

	return c.JSON(fiber.Map{"metrics": nil})
}

// Power controla ligar/desligar/reiniciar uma máquina.
func (h *MachineHandler) Power(c *fiber.Ctx) error {
	id := c.Params("id")

	var body struct {
		Action string `json:"action"` // "start", "stop", "reboot"
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "body inválido"})
	}

	_ = id
	// TODO:
	// - Para VMs/CTs: chamar Proxmox API POST /nodes/{node}/qemu/{vmid}/status/{action}
	// - Para baremetals: IPMI/iDRAC ou WoL (wake) / SSH shutdown (stop)
	// - Logar ação no activity_log
	// - Enviar notificação se for baremetal

	return c.JSON(fiber.Map{"message": "ação executada", "action": body.Action})
}
