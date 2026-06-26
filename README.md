# ATLAB Backend

> API REST + WebSocket do ATLAB Platform — Go + Fiber + PostgreSQL + Redis

## Arquitetura

```
cmd/server/main.go          → Entry point
internal/
├── config/config.go        → Env vars (ATLAB_*)
├── database/postgres.go    → Pool pgx
├── middleware/auth.go      → JWT validation + RBAC
├── handlers/
│   ├── auth.go             → OIDC login/callback (Authentik)
│   ├── machines.go         → CRUD + métricas (via Prometheus)
│   ├── proxmox.go          → API Proxmox (nodes, VMs, provisioning)
│   ├── ssh.go              → WebSocket SSH proxy
│   ├── ipam.go             → Subnets + IP allocations
│   └── alerts.go           → Alerts, activity, groups, creds, automation
├── services/               → (futuro) business logic isolada
└── models/                 → (futuro) structs do domínio
migrations/
└── init.sql                → Schema completo do PostgreSQL
```

## Endpoints

| Método | Rota | Auth | Role | Descrição |
|--------|------|------|------|-----------|
| GET | /health | ✗ | — | Health check |
| GET | /api/auth/login | ✗ | — | Redirect pro Authentik |
| GET | /api/auth/callback | ✗ | — | Recebe code OIDC |
| GET | /api/me | ✓ | * | Dados do usuário logado |
| GET | /api/machines | ✓ | * | Listar máquinas (filtrado por grupo) |
| GET | /api/machines/:id | ✓ | * | Detalhes da máquina |
| GET | /api/machines/:id/metrics | ✓ | * | Métricas via Prometheus |
| POST | /api/machines/:id/power | ✓ | admin | Ligar/desligar/reiniciar |
| GET | /api/proxmox/nodes | ✓ | * | Status dos nós |
| GET | /api/proxmox/nodes/:id/vms | ✓ | * | VMs de um nó |
| POST | /api/proxmox/provision | ✓ | admin | Criar VM/CT |
| GET | /api/ipam/subnets | ✓ | * | Sub-redes |
| POST | /api/ipam/allocate | ✓ | admin,devops | Alocar IP |
| GET | /api/ssh/connect/:id | ✓ | !viewer | WebSocket SSH |
| GET | /api/alerts | ✓ | * | Alertas |
| POST | /api/alerts/:id/ack | ✓ | * | Reconhecer alerta |
| GET | /api/activity | ✓ | * | Log de atividades |
| GET | /api/groups | ✓ | * | Grupos de acesso |
| GET | /api/automation/tasks | ✓ | * | Tarefas agendadas |
| POST | /api/automation/tasks/:id/run | ✓ | admin,devops | Executar task |
| GET | /api/reports/:type | ✓ | * | Gerar relatório |

## Deploy

```bash
# 1. Copie e preencha .env
cp .env.example .env
nano .env

# 2. Suba tudo
docker compose up -d

# 3. Verifique
curl http://localhost:8080/health
```

## Pré-requisitos no Proxmox

1. Criar API Token:
   - Datacenter > Permissions > API Tokens
   - User: root@pam, Token ID: atlab
   - Desmarcar "Privilege Separation" (para ter acesso total)
   - Copiar o secret para PROXMOX_TOKEN_SECRET

2. Criar API Token em CADA nó (ou usar um centralizado se tiver cluster)

## Pré-requisitos no Authentik

1. Criar Application: "ATLAB Platform"
2. Criar Provider: OAuth2/OpenID Connect
   - Client ID: atlab-platform
   - Redirect URI: https://atlab.ufc.br/api/auth/callback
   - Scopes: openid, profile, email, groups
3. Copiar Client Secret para AUTHENTIK_CLIENT_SECRET

## Infra real ATLAB

```
Prometheus: http://10.101.53.212:9000
Authentik:  https://authentik.atlab.ufc.br
Proxmox:    10.101.53.240 (alpha, 6c/31G)
            10.101.53.247 (redragon, 24c/126G)
            10.101.53.243 (tau, 12c/63G)
Rede:       10.101.53.0/24 (principal)
            200.19.187.64/28 (real GREAT)
            200.19.187.80/28 (real ATLab)
```
