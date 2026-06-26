-- ATLAB Platform — Schema Inicial
-- Executado automaticamente na primeira subida do PostgreSQL

-- ─── Extensions ──────────────────────────────────────────────
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ─── Enum Types ──────────────────────────────────────────────
CREATE TYPE machine_type AS ENUM ('physical', 'vm', 'ct');
CREATE TYPE machine_status AS ENUM ('online', 'offline', 'unknown');
CREATE TYPE user_role AS ENUM ('admin', 'devops', 'developer', 'viewer');
CREATE TYPE credential_type AS ENUM ('ssh_key', 'password', 'token');
CREATE TYPE alert_severity AS ENUM ('info', 'warning', 'critical');
CREATE TYPE task_status AS ENUM ('idle', 'running', 'success', 'failed');
CREATE TYPE session_status AS ENUM ('active', 'closed');

-- ─── Users (espelho do Authentik, syncado no login) ──────────
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    authentik_id VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    avatar_url TEXT,
    role user_role NOT NULL DEFAULT 'viewer',
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Access Groups ───────────────────────────────────────────
CREATE TABLE access_groups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    permissions JSONB NOT NULL DEFAULT '[]',
    -- ex: ["ssh", "sudo", "provision", "shutdown"]
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE user_groups (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    group_id UUID REFERENCES access_groups(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, group_id)
);

-- ─── Proxmox Nodes ───────────────────────────────────────────
CREATE TABLE proxmox_nodes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    host VARCHAR(255) NOT NULL,    -- ex: 10.101.53.240:8006
    token_id VARCHAR(255),         -- ex: root@pam!atlab
    token_secret TEXT,             -- encrypted
    status machine_status NOT NULL DEFAULT 'unknown',
    version VARCHAR(50),
    cpu_model VARCHAR(100),
    cores INTEGER,
    ram_total_gb INTEGER,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Machines ────────────────────────────────────────────────
CREATE TABLE machines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    proxmox_vmid INTEGER,          -- VMID no Proxmox (null se cadastro manual)
    node_id UUID REFERENCES proxmox_nodes(id),
    name VARCHAR(100) NOT NULL,
    ip VARCHAR(45),                 -- IPv4 ou IPv6
    mac_address VARCHAR(17),        -- para WoL
    type machine_type NOT NULL,
    os VARCHAR(100),
    os_version VARCHAR(50),
    kernel VARCHAR(100),
    cores INTEGER,
    ram_gb INTEGER,
    disk_gb INTEGER,
    ssh_port INTEGER NOT NULL DEFAULT 22,
    status machine_status NOT NULL DEFAULT 'unknown',
    tags TEXT[] DEFAULT '{}',
    -- Security config (synced periodicamente)
    root_login_enabled BOOLEAN DEFAULT false,
    ssh_password_auth BOOLEAN DEFAULT false,
    firewall_enabled BOOLEAN DEFAULT false,
    last_patched_at TIMESTAMPTZ,
    --
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Relação grupo ↔ máquina (quais máquinas cada grupo pode acessar)
CREATE TABLE group_machines (
    group_id UUID REFERENCES access_groups(id) ON DELETE CASCADE,
    machine_id UUID REFERENCES machines(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, machine_id)
);

-- ─── Subnets (IPAM) ─────────────────────────────────────────
CREATE TABLE subnets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    network CIDR NOT NULL,        -- ex: 10.101.53.0/24
    description VARCHAR(255),
    vlan VARCHAR(50),
    gateway INET,
    dhcp_start INET,
    dhcp_end INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── IP Allocations ──────────────────────────────────────────
CREATE TABLE ip_allocations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    subnet_id UUID REFERENCES subnets(id),
    ip INET NOT NULL UNIQUE,
    machine_id UUID REFERENCES machines(id) ON DELETE SET NULL,
    hostname VARCHAR(100),
    allocated_by UUID REFERENCES users(id),
    allocated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    notes TEXT
);

-- ─── Credentials Vault ───────────────────────────────────────
CREATE TABLE credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    label VARCHAR(255) NOT NULL,
    type credential_type NOT NULL,
    username VARCHAR(100),
    -- secret é encrypted via pgcrypto
    secret_encrypted BYTEA,
    target_pattern VARCHAR(255),   -- ex: "srv-web*" ou machine_id
    created_by UUID REFERENCES users(id),
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── SSH Sessions (auditoria) ────────────────────────────────
CREATE TABLE ssh_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id),
    machine_id UUID REFERENCES machines(id),
    from_ip INET,
    status session_status NOT NULL DEFAULT 'active',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ
);

-- Cada comando executado na sessão
CREATE TABLE ssh_commands (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID REFERENCES ssh_sessions(id) ON DELETE CASCADE,
    command TEXT NOT NULL,
    exit_code INTEGER,
    executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    flagged BOOLEAN DEFAULT false     -- marcado como suspeito pelo motor
);

-- ─── Alerts ──────────────────────────────────────────────────
CREATE TABLE alerts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    severity alert_severity NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT,
    source VARCHAR(100),           -- machine name ou "system"
    machine_id UUID REFERENCES machines(id),
    acknowledged BOOLEAN DEFAULT false,
    acknowledged_by UUID REFERENCES users(id),
    acknowledged_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Activity Log ────────────────────────────────────────────
CREATE TABLE activity_log (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    action VARCHAR(255) NOT NULL,
    target VARCHAR(255),
    target_type VARCHAR(50),       -- 'machine', 'credential', 'group', etc.
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Automation Tasks ────────────────────────────────────────
CREATE TABLE automation_tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    task_type VARCHAR(50) NOT NULL,  -- 'backup', 'healthcheck', 'cleanup', 'deploy'
    schedule VARCHAR(100),           -- cron expression
    enabled BOOLEAN DEFAULT true,
    status task_status DEFAULT 'idle',
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    config JSONB,                    -- task-specific config
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Scheduled Shutdowns ─────────────────────────────────────
CREATE TABLE scheduled_shutdowns (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    reason TEXT NOT NULL,
    scheduled_for TIMESTAMPTZ NOT NULL,
    machine_ids UUID[] NOT NULL,
    created_by UUID REFERENCES users(id),
    status VARCHAR(20) DEFAULT 'pending',  -- pending, executing, completed, cancelled
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Indexes ─────────────────────────────────────────────────
CREATE INDEX idx_machines_node ON machines(node_id);
CREATE INDEX idx_machines_status ON machines(status);
CREATE INDEX idx_machines_type ON machines(type);
CREATE INDEX idx_ssh_sessions_user ON ssh_sessions(user_id);
CREATE INDEX idx_ssh_sessions_machine ON ssh_sessions(machine_id);
CREATE INDEX idx_ssh_commands_session ON ssh_commands(session_id);
CREATE INDEX idx_alerts_severity ON alerts(severity) WHERE NOT acknowledged;
CREATE INDEX idx_activity_log_user ON activity_log(user_id);
CREATE INDEX idx_activity_log_created ON activity_log(created_at DESC);
CREATE INDEX idx_ip_allocations_subnet ON ip_allocations(subnet_id);

-- ─── Seed: Initial subnets (ATLAB real) ──────────────────────
INSERT INTO subnets (network, description, vlan, gateway, dhcp_start, dhcp_end) VALUES
    ('10.101.53.0/24', 'Rede principal ATLAB', NULL, '10.101.53.1', '10.101.53.21', '10.101.53.200'),
    ('200.19.187.64/28', 'IP Real GREAT', NULL, '200.19.187.65', NULL, NULL),
    ('200.19.187.80/28', 'IP Real ATLab', NULL, '200.19.187.81', NULL, NULL);

-- ─── Seed: Initial Proxmox nodes ─────────────────────────────
INSERT INTO proxmox_nodes (name, host, cores, ram_total_gb, cpu_model) VALUES
    ('proxmox-alpha', '10.101.53.240:8006', 6, 31, NULL),
    ('proxmox-redragon', '10.101.53.247:8006', 24, 126, NULL),
    ('proxmox-tau', '10.101.53.243:8006', 12, 63, NULL);
