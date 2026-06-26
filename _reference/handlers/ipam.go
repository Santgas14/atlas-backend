package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IPAMHandler struct {
	db *pgxpool.Pool
}

func NewIPAMHandler(db *pgxpool.Pool) *IPAMHandler {
	return &IPAMHandler{db: db}
}

// ListSubnets retorna todas as sub-redes cadastradas.
func (h *IPAMHandler) ListSubnets(c *fiber.Ctx) error {
	// TODO: SELECT * FROM subnets
	// + contagem de IPs alocados por subnet
	return c.JSON(fiber.Map{"subnets": []interface{}{}})
}

// ListAllocations retorna IPs alocados em uma sub-rede.
func (h *IPAMHandler) ListAllocations(c *fiber.Ctx) error {
	subnetID := c.Params("id")
	_ = subnetID
	// TODO: SELECT ia.*, m.name FROM ip_allocations ia
	// LEFT JOIN machines m ON m.id = ia.machine_id
	// WHERE ia.subnet_id = $1
	return c.JSON(fiber.Map{"allocations": []interface{}{}})
}

// Allocate registra um novo IP alocado.
func (h *IPAMHandler) Allocate(c *fiber.Ctx) error {
	var body struct {
		SubnetID  string `json:"subnet_id"`
		IP        string `json:"ip"`
		MachineID string `json:"machine_id"`
		Hostname  string `json:"hostname"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "body inválido"})
	}

	// TODO:
	// 1. Verificar se IP está dentro da subnet
	// 2. Verificar se IP não está alocado
	// 3. INSERT INTO ip_allocations
	// 4. Atualizar machine.ip se machine_id fornecido

	return c.Status(201).JSON(fiber.Map{"message": "IP alocado", "ip": body.IP})
}
