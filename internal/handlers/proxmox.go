package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/atlab-ufc/atlab-backend/internal/config"
)

type ProxmoxHandler struct {
	cfg *config.Config
	db  *pgxpool.Pool
}

func NewProxmoxHandler(cfg *config.Config, db *pgxpool.Pool) *ProxmoxHandler {
	return &ProxmoxHandler{cfg: cfg, db: db}
}

// ListNodes retorna status de todos os nós Proxmox.
func (h *ProxmoxHandler) ListNodes(c *fiber.Ctx) error {
	// TODO: Para cada nó configurado, chamar:
	// GET https://{node}:8006/api2/json/nodes
	// Header: Authorization: PVEAPIToken={tokenid}={secret}
	//
	// Resposta inclui: cpu, maxcpu, mem, maxmem, disk, maxdisk, uptime, status

	return c.JSON(fiber.Map{"nodes": []interface{}{}})
}

// ListVMs retorna VMs e containers de um nó específico.
func (h *ProxmoxHandler) ListVMs(c *fiber.Ctx) error {
	nodeID := c.Params("id")
	_ = nodeID

	// TODO:
	// GET https://{node}:8006/api2/json/nodes/{nodename}/qemu — lista VMs
	// GET https://{node}:8006/api2/json/nodes/{nodename}/lxc  — lista CTs
	//
	// Cada VM/CT retorna: vmid, name, status, cpus, maxmem, maxdisk, netin, netout

	return c.JSON(fiber.Map{"vms": []interface{}{}, "containers": []interface{}{}})
}

// Provision cria uma nova VM ou CT no Proxmox.
func (h *ProxmoxHandler) Provision(c *fiber.Ctx) error {
	var body struct {
		Type    string `json:"type"`    // "vm" ou "ct"
		Name    string `json:"name"`
		Node    string `json:"node"`    // nome do nó
		OS      string `json:"os"`      // template/iso
		Cores   int    `json:"cores"`
		RamMB   int    `json:"ram_mb"`
		DiskGB  int    `json:"disk_gb"`
		Network string `json:"network"` // bridge
		IP      string `json:"ip"`      // vazio = DHCP
		// Cloud-init
		Username  string `json:"username"`
		Password  string `json:"password"`
		SSHPubKey string `json:"ssh_pub_key"`
		// Software
		Packages []string `json:"packages"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "body inválido"})
	}

	// TODO:
	// 1. Pegar próximo VMID disponível: GET /cluster/nextid
	// 2. Criar VM: POST /nodes/{node}/qemu (ou /lxc)
	//    Params: vmid, name, cores, memory, scsi0, net0, ide2 (cloudinit)
	// 3. Se cloud-init: configurar user, sshkeys, nameserver, ipconfig0
	// 4. Se packages: criar snippet de cloud-init com runcmd
	// 5. Start VM: POST /nodes/{node}/qemu/{vmid}/status/start
	// 6. Registrar máquina no banco
	// 7. Alocar IP no IPAM
	// 8. Logar atividade

	return c.Status(201).JSON(fiber.Map{
		"message": "Provisionamento iniciado",
		"name":    body.Name,
	})
}
