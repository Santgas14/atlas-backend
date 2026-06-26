package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/atlab-ufc/atlab-backend/internal/config"
)

type SSHHandler struct {
	cfg *config.Config
	db  *pgxpool.Pool
}

func NewSSHHandler(cfg *config.Config, db *pgxpool.Pool) *SSHHandler {
	return &SSHHandler{cfg: cfg, db: db}
}

// Connect estabelece uma sessão SSH via WebSocket.
//
// Fluxo:
// 1. Frontend abre WebSocket em /api/ssh/connect/{machineId}
// 2. Backend verifica permissão do usuário para aquela máquina
// 3. Backend abre conexão SSH para a máquina (usando chave do service account)
// 4. Cria entrada em ssh_sessions
// 5. Proxy bidirecional: WebSocket ↔ SSH channel
// 6. Cada linha recebida do usuário é logada em ssh_commands
// 7. Motor de segurança roda em tempo real nos comandos (flag suspicious)
// 8. Ao fechar: atualiza ssh_sessions com ended_at
func (h *SSHHandler) Connect(c *fiber.Ctx) error {
	machineID := c.Params("machineId")
	userRole, _ := c.Locals("role").(string)

	// Verificar permissão
	if userRole == "viewer" {
		return c.Status(403).JSON(fiber.Map{"error": "Viewers não podem abrir sessões SSH"})
	}

	_ = machineID

	// TODO: Implementar com gofiber/contrib/websocket:
	//
	// return websocket.New(func(ws *websocket.Conn) {
	//     // 1. Buscar IP e porta da máquina
	//     // 2. Carregar chave privada do service account
	//     // 3. Dial SSH: ssh.Dial("tcp", ip+":"+port, sshConfig)
	//     // 4. Abrir session: client.NewSession()
	//     // 5. RequestPty("xterm-256color", 80, 24, modes)
	//     // 6. Shell()
	//     // 7. Loop: ws.ReadMessage() → stdin | stdout → ws.WriteMessage()
	//     // 8. Log cada comando parsed (por newline)
	//     // 9. Cleanup on close
	// })(c)

	return c.JSON(fiber.Map{
		"message": "WebSocket SSH endpoint — upgrade para ws:// necessário",
		"machine": machineID,
	})
}
