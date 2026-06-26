// ATLAS Backend — Entry point
// Servidor HTTP com integração real ao Proxmox + auto-bootstrap de VMs.
package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"golang.org/x/crypto/ssh"
)

// ─── Proxmox types ───────────────────────────────────────────

type PveNodeStatus struct {
	Name      string  `json:"node"`
	Status    string  `json:"status"`
	CPU       float64 `json:"cpu"`
	MaxCPU    int     `json:"maxcpu"`
	Mem       int64   `json:"mem"`
	MaxMem    int64   `json:"maxmem"`
	Disk      int64   `json:"disk"`
	MaxDisk   int64   `json:"maxdisk"`
	Uptime    int64   `json:"uptime"`
}

type PveVM struct {
	VMID    int     `json:"vmid"`
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	CPUs    int     `json:"cpus"`
	MaxMem  int64   `json:"maxmem"`
	MaxDisk int64   `json:"maxdisk"`
	NetIn   int64   `json:"netin"`
	NetOut  int64   `json:"netout"`
	Uptime  int64   `json:"uptime"`
	CPU     float64 `json:"cpu"`
	Mem     int64   `json:"mem"`
}

type PveResponse struct {
	Data json.RawMessage `json:"data"`
}

// ─── Proxmox client ──────────────────────────────────────────

type ProxmoxClient struct {
	Nodes       []string
	TokenID     string
	TokenSecret string
	httpClient  *http.Client
}

func NewProxmoxClient() *ProxmoxClient {
	nodes := strings.Split(getEnv("ATLAB_PROXMOX_NODES", "10.101.53.240,10.101.53.243,10.101.53.247"), ",")
	return &ProxmoxClient{
		Nodes:       nodes,
		TokenID:     getEnv("ATLAB_PROXMOX_TOKEN_ID", ""),
		TokenSecret: getEnv("ATLAB_PROXMOX_TOKEN_SECRET", ""),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Proxmox self-signed
			},
		},
	}
}

func (p *ProxmoxClient) request(nodeIP, path string) ([]byte, error) {
	url := fmt.Sprintf("https://%s:8006/api2/json%s", nodeIP, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", p.TokenID, p.TokenSecret))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func (p *ProxmoxClient) GetNodeStatus(nodeIP string) (*PveNodeStatus, error) {
	// First get node name
	data, err := p.request(nodeIP, "/nodes")
	if err != nil {
		return nil, err
	}

	var resp PveResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	var nodes []PveNodeStatus
	if err := json.Unmarshal(resp.Data, &nodes); err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes returned")
	}
	return &nodes[0], nil
}

func (p *ProxmoxClient) GetVMs(nodeIP, nodeName string) ([]PveVM, error) {
	data, err := p.request(nodeIP, fmt.Sprintf("/nodes/%s/qemu", nodeName))
	if err != nil {
		return nil, err
	}

	var resp PveResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	var vms []PveVM
	if err := json.Unmarshal(resp.Data, &vms); err != nil {
		return nil, err
	}
	return vms, nil
}

func (p *ProxmoxClient) GetContainers(nodeIP, nodeName string) ([]PveVM, error) {
	data, err := p.request(nodeIP, fmt.Sprintf("/nodes/%s/lxc", nodeName))
	if err != nil {
		return nil, err
	}

	var resp PveResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	var cts []PveVM
	if err := json.Unmarshal(resp.Data, &cts); err != nil {
		return nil, err
	}
	return cts, nil
}

// ─── VM Network Discovery ────────────────────────────────────

type VMInfo struct {
	VMID   int    `json:"vmid"`
	Name   string `json:"name"`
	Status string `json:"status"`
	IP     string `json:"ip"`
	Type   string `json:"type"` // vm or ct
	Node   string `json:"node"`
	// From SSH probe
	HasNodeExporter bool   `json:"has_node_exporter"`
	Bootstrapped    bool   `json:"bootstrapped"`
	OS              string `json:"os_detected"`
}

// GetVMIP tenta descobrir o IP de uma VM via QEMU guest agent
func (p *ProxmoxClient) GetVMIP(nodeIP, nodeName string, vmid int) string {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", nodeName, vmid)
	data, err := p.request(nodeIP, path)
	if err != nil {
		return ""
	}

	var resp PveResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return ""
	}

	// Parse interfaces
	var result struct {
		Result []struct {
			Name        string `json:"name"`
			IPAddresses []struct {
				IPAddress string `json:"ip-address"`
				IPType    string `json:"ip-address-type"`
			} `json:"ip-addresses"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return ""
	}

	// Find first non-loopback IPv4
	for _, iface := range result.Result {
		if iface.Name == "lo" {
			continue
		}
		for _, addr := range iface.IPAddresses {
			if addr.IPType == "ipv4" && !strings.HasPrefix(addr.IPAddress, "127.") {
				return addr.IPAddress
			}
		}
	}
	return ""
}

// ─── SSH Bootstrap ───────────────────────────────────────────

type BootstrapAgent struct {
	SSHUser     string
	SSHPassword string
	mu          sync.Mutex
	status      map[string]*BootstrapStatus // key = IP
}

type BootstrapStatus struct {
	IP              string `json:"ip"`
	Name            string `json:"name"`
	Connected       bool   `json:"ssh_connected"`
	OS              string `json:"os"`
	NodeExporter    bool   `json:"node_exporter_installed"`
	NodeExporterRun bool   `json:"node_exporter_running"`
	Bootstrapped    bool   `json:"bootstrapped"`
	LastCheck       int64  `json:"last_check"`
	Error           string `json:"error,omitempty"`
}

func NewBootstrapAgent() *BootstrapAgent {
	return &BootstrapAgent{
		SSHUser:     getEnv("ATLAB_SSH_USER", "root"),
		SSHPassword: getEnv("ATLAB_SSH_PASSWORD", "@tloginroot"),
		status:      make(map[string]*BootstrapStatus),
	}
}

// sshConnect establishes SSH connection with password auth
func (b *BootstrapAgent) sshConnect(ip string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: b.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(b.SSHPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	addr := net.JoinHostPort(ip, "22")
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH dial failed: %w", err)
	}
	return client, nil
}

// runCommand executes a command via SSH and returns output
func (b *BootstrapAgent) runCommand(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	return strings.TrimSpace(string(output)), err
}

// Probe checks a VM's status and installs node_exporter if needed
func (b *BootstrapAgent) Probe(ip, name string) *BootstrapStatus {
	b.mu.Lock()
	st, exists := b.status[ip]
	if !exists {
		st = &BootstrapStatus{IP: ip, Name: name}
		b.status[ip] = st
	}
	b.mu.Unlock()

	// Connect
	client, err := b.sshConnect(ip)
	if err != nil {
		st.Connected = false
		st.Error = err.Error()
		st.LastCheck = time.Now().Unix()
		return st
	}
	defer client.Close()

	st.Connected = true
	st.Error = ""

	// Detect OS
	osInfo, _ := b.runCommand(client, "cat /etc/os-release 2>/dev/null | grep ^PRETTY_NAME | cut -d= -f2 | tr -d '\"'")
	if osInfo == "" {
		osInfo, _ = b.runCommand(client, "uname -s")
	}
	st.OS = osInfo

	// Check if node_exporter is installed
	_, err = b.runCommand(client, "which node_exporter || command -v node_exporter")
	st.NodeExporter = err == nil

	// Check if node_exporter is running
	out, _ := b.runCommand(client, "systemctl is-active node_exporter 2>/dev/null || pgrep -x node_exporter")
	st.NodeExporterRun = (out == "active" || out != "")

	// If not installed/running, bootstrap it
	if !st.NodeExporter || !st.NodeExporterRun {
		log.Printf("[bootstrap] %s (%s): installing node_exporter...", name, ip)
		b.installNodeExporter(client, st)
	} else {
		st.Bootstrapped = true
	}

	st.LastCheck = time.Now().Unix()
	return st
}

func (b *BootstrapAgent) installNodeExporter(client *ssh.Client, st *BootstrapStatus) {
	commands := []string{
		// Download node_exporter
		"cd /tmp && curl -sLO https://github.com/prometheus/node_exporter/releases/download/v1.8.1/node_exporter-1.8.1.linux-amd64.tar.gz",
		// Extract
		"cd /tmp && tar xzf node_exporter-1.8.1.linux-amd64.tar.gz",
		// Install binary
		"cp /tmp/node_exporter-1.8.1.linux-amd64/node_exporter /usr/local/bin/ && chmod +x /usr/local/bin/node_exporter",
		// Create systemd service
		`cat > /etc/systemd/system/node_exporter.service << 'UNIT'
[Unit]
Description=Prometheus Node Exporter
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/node_exporter
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
UNIT`,
		// Enable and start
		"systemctl daemon-reload",
		"systemctl enable node_exporter",
		"systemctl start node_exporter",
		// Cleanup
		"rm -rf /tmp/node_exporter-*",
	}

	for _, cmd := range commands {
		_, err := b.runCommand(client, cmd)
		if err != nil {
			log.Printf("[bootstrap] %s: command failed: %s (error: %v)", st.Name, cmd[:min(50, len(cmd))], err)
			st.Error = fmt.Sprintf("bootstrap failed at: %s", cmd[:min(50, len(cmd))])
			return
		}
	}

	// Verify
	out, _ := b.runCommand(client, "systemctl is-active node_exporter")
	st.NodeExporter = true
	st.NodeExporterRun = (out == "active")
	st.Bootstrapped = st.NodeExporterRun
	if st.Bootstrapped {
		log.Printf("[bootstrap] %s (%s): node_exporter installed and running ✓", st.Name, st.IP)
		// Auto-register in Prometheus
		go registerInPrometheus(st.IP, st.Name)
	}
}

// registerInPrometheus adds the VM as a target in the Prometheus config via SSH
func registerInPrometheus(ip, hostname string) {
	promHost := getEnv("ATLAB_PROMETHEUS_HOST", "10.101.53.212")
	promUser := getEnv("ATLAB_PROMETHEUS_SSH_USER", "atlab")
	promPassword := getEnv("ATLAB_PROMETHEUS_SSH_PASSWORD", "@tloginroot")
	configPath := "/monitoramento/prometheus/prometheus.yml"

	// Connect to Prometheus server via SSH
	config := &ssh.ClientConfig{
		User: promUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(promPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	client, err := ssh.Dial("tcp", promHost+":22", config)
	if err != nil {
		log.Printf("[prometheus] Failed to connect to %s: %v", promHost, err)
		return
	}
	defer client.Close()

	// Check if target already exists
	session, _ := client.NewSession()
	out, _ := session.CombinedOutput(fmt.Sprintf("grep -c '%s:9100' %s", ip, configPath))
	session.Close()
	if strings.TrimSpace(string(out)) != "0" {
		log.Printf("[prometheus] %s (%s) already in config, skipping", hostname, ip)
		return
	}

	// Build the new target entry
	entry := fmt.Sprintf(`
    - targets: ["%s:9100"]
      labels:
        job: "vms"
        hostname: "%s"
        host_pai: "proxmox-alpha"
        managed_by: "atlas"`, ip, hostname)

	// Append to the node_exporter job section (before process_exporter section)
	appendCmd := fmt.Sprintf(`sed -i '/# CAMADA 2: PROCESS EXPORTER/i\%s' %s`, strings.ReplaceAll(entry, "\n", "\\n"), configPath)
	session2, _ := client.NewSession()
	_, err = session2.CombinedOutput(appendCmd)
	session2.Close()

	if err != nil {
		// Fallback: append at end of node_exporter section using echo
		fallbackCmd := fmt.Sprintf(`echo '%s' >> %s`, entry, configPath)
		session3, _ := client.NewSession()
		session3.CombinedOutput(fallbackCmd)
		session3.Close()
	}

	// Reload Prometheus
	session4, _ := client.NewSession()
	session4.CombinedOutput("curl -s -X POST http://localhost:9090/-/reload 2>/dev/null || kill -HUP $(pgrep prometheus) 2>/dev/null")
	session4.Close()

	log.Printf("[prometheus] %s (%s) registered as target ✓", hostname, ip)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─── Main ────────────────────────────────────────────────────

func main() {
	port := getEnv("ATLAB_PORT", "8080")
	pve := NewProxmoxClient()
	bootstrap := NewBootstrapAgent()

	// Known IPs (until guest agent works everywhere)
	knownIPs := map[int]string{
		100: "10.101.53.203", // CLUSTERLEX
		101: "10.101.53.202", // VM-GPU-ALPHA
		102: "10.101.53.218", // ATLAS
	}

	app := fiber.New(fiber.Config{
		AppName:      "ATLAS",
		ServerHeader: "ATLAS",
	})

	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
	}))

	// ─── Health ──────────────────────────────────────────────
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "atlas-backend",
			"version": "0.3.0",
		})
	})

	api := app.Group("/api")

	// ─── Proxmox Nodes (REAL) ────────────────────────────────
	api.Get("/proxmox/nodes", func(c *fiber.Ctx) error {
		type NodeResponse struct {
			Name     string  `json:"name"`
			Host     string  `json:"host"`
			Status   string  `json:"status"`
			CPU      float64 `json:"cpu_percent"`
			Cores    int     `json:"cores"`
			MemUsed  int64   `json:"mem_used_bytes"`
			MemTotal int64   `json:"mem_total_bytes"`
			MemPct   float64 `json:"mem_percent"`
			DiskUsed int64   `json:"disk_used_bytes"`
			DiskTotal int64  `json:"disk_total_bytes"`
			DiskPct  float64 `json:"disk_percent"`
			Uptime   int64   `json:"uptime_seconds"`
			Error    string  `json:"error,omitempty"`
		}

		results := make([]NodeResponse, 0, len(pve.Nodes))

		for _, nodeIP := range pve.Nodes {
			nr := NodeResponse{Host: nodeIP + ":8006"}

			status, err := pve.GetNodeStatus(nodeIP)
			if err != nil {
				nr.Status = "offline"
				nr.Error = err.Error()
				results = append(results, nr)
				continue
			}

			nr.Name = status.Name
			nr.Status = status.Status
			nr.CPU = status.CPU * 100
			nr.Cores = status.MaxCPU
			nr.MemUsed = status.Mem
			nr.MemTotal = status.MaxMem
			nr.DiskUsed = status.Disk
			nr.DiskTotal = status.MaxDisk
			nr.Uptime = status.Uptime

			if status.MaxMem > 0 {
				nr.MemPct = float64(status.Mem) / float64(status.MaxMem) * 100
			}
			if status.MaxDisk > 0 {
				nr.DiskPct = float64(status.Disk) / float64(status.MaxDisk) * 100
			}

			results = append(results, nr)
		}

		return c.JSON(fiber.Map{"nodes": results})
	})

	// ─── VMs de um nó (REAL) ─────────────────────────────────
	api.Get("/proxmox/nodes/:ip/vms", func(c *fiber.Ctx) error {
		nodeIP := c.Params("ip")

		// Get node name first
		status, err := pve.GetNodeStatus(nodeIP)
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": err.Error()})
		}

		vms, err := pve.GetVMs(nodeIP, status.Name)
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": err.Error()})
		}

		cts, err := pve.GetContainers(nodeIP, status.Name)
		if err != nil {
			cts = []PveVM{} // não fatal
		}

		type VMResponse struct {
			VMID     int     `json:"vmid"`
			Name     string  `json:"name"`
			Type     string  `json:"type"`
			Status   string  `json:"status"`
			CPUs     int     `json:"cpus"`
			CPU      float64 `json:"cpu_percent"`
			MemBytes int64   `json:"mem_bytes"`
			MaxMem   int64   `json:"max_mem_bytes"`
			MemPct   float64 `json:"mem_percent"`
			DiskBytes int64  `json:"disk_bytes"`
			NetIn    int64   `json:"net_in_bytes"`
			NetOut   int64   `json:"net_out_bytes"`
			Uptime   int64   `json:"uptime_seconds"`
		}

		results := make([]VMResponse, 0)

		for _, vm := range vms {
			r := VMResponse{
				VMID: vm.VMID, Name: vm.Name, Type: "vm", Status: vm.Status,
				CPUs: vm.CPUs, CPU: vm.CPU * 100, MemBytes: vm.Mem, MaxMem: vm.MaxMem,
				DiskBytes: vm.MaxDisk, NetIn: vm.NetIn, NetOut: vm.NetOut, Uptime: vm.Uptime,
			}
			if vm.MaxMem > 0 {
				r.MemPct = float64(vm.Mem) / float64(vm.MaxMem) * 100
			}
			results = append(results, r)
		}

		for _, ct := range cts {
			r := VMResponse{
				VMID: ct.VMID, Name: ct.Name, Type: "ct", Status: ct.Status,
				CPUs: ct.CPUs, CPU: ct.CPU * 100, MemBytes: ct.Mem, MaxMem: ct.MaxMem,
				DiskBytes: ct.MaxDisk, NetIn: ct.NetIn, NetOut: ct.NetOut, Uptime: ct.Uptime,
			}
			if ct.MaxMem > 0 {
				r.MemPct = float64(ct.Mem) / float64(ct.MaxMem) * 100
			}
			results = append(results, r)
		}

		return c.JSON(fiber.Map{
			"node":    status.Name,
			"machines": results,
			"total":   len(results),
		})
	})

	// ─── All machines (aggregate from all nodes + discover IPs) ─
	api.Get("/machines", func(c *fiber.Ctx) error {
		type MachineItem struct {
			VMID     int     `json:"vmid"`
			Name     string  `json:"name"`
			Type     string  `json:"type"`
			Status   string  `json:"status"`
			Node     string  `json:"node"`
			NodeIP   string  `json:"node_ip"`
			IP       string  `json:"ip"`
			CPUs     int     `json:"cpus"`
			CPU      float64 `json:"cpu_percent"`
			MemPct   float64 `json:"mem_percent"`
			MaxMemGB float64 `json:"max_mem_gb"`
			DiskGB   float64 `json:"disk_gb"`
			Uptime   int64   `json:"uptime_seconds"`
		}

		all := make([]MachineItem, 0)

		for _, nodeIP := range pve.Nodes {
			status, err := pve.GetNodeStatus(nodeIP)
			if err != nil {
				continue
			}

			vms, _ := pve.GetVMs(nodeIP, status.Name)
			cts, _ := pve.GetContainers(nodeIP, status.Name)

			for _, vm := range vms {
				memPct := float64(0)
				if vm.MaxMem > 0 {
					memPct = float64(vm.Mem) / float64(vm.MaxMem) * 100
				}
				// Discover IP: guest agent first, then known IPs fallback
				ip := pve.GetVMIP(nodeIP, status.Name, vm.VMID)
				if ip == "" {
					ip = knownIPs[vm.VMID]
				}
				all = append(all, MachineItem{
					VMID: vm.VMID, Name: vm.Name, Type: "vm", Status: vm.Status,
					Node: status.Name, NodeIP: nodeIP, IP: ip, CPUs: vm.CPUs, CPU: vm.CPU * 100,
					MemPct: memPct, MaxMemGB: float64(vm.MaxMem) / 1073741824,
					DiskGB: float64(vm.MaxDisk) / 1073741824, Uptime: vm.Uptime,
				})
			}
			for _, ct := range cts {
				memPct := float64(0)
				if ct.MaxMem > 0 {
					memPct = float64(ct.Mem) / float64(ct.MaxMem) * 100
				}
				ip := pve.GetVMIP(nodeIP, status.Name, ct.VMID)
				if ip == "" {
					ip = knownIPs[ct.VMID]
				}
				all = append(all, MachineItem{
					VMID: ct.VMID, Name: ct.Name, Type: "ct", Status: ct.Status,
					Node: status.Name, NodeIP: nodeIP, IP: ip, CPUs: ct.CPUs, CPU: ct.CPU * 100,
					MemPct: memPct, MaxMemGB: float64(ct.MaxMem) / 1073741824,
					DiskGB: float64(ct.MaxDisk) / 1073741824, Uptime: ct.Uptime,
				})
			}
		}

		return c.JSON(fiber.Map{"machines": all, "total": len(all)})
	})

	// ─── Bootstrap: probe + auto-install node_exporter ────────
	api.Post("/machines/bootstrap", func(c *fiber.Ctx) error {
		var body struct {
			IP   string `json:"ip"`
			Name string `json:"name"`
		}
		if err := c.BodyParser(&body); err != nil || body.IP == "" {
			return c.Status(400).JSON(fiber.Map{"error": "ip is required"})
		}

		// Run probe in background
		go bootstrap.Probe(body.IP, body.Name)

		return c.JSON(fiber.Map{
			"message": "bootstrap initiated",
			"ip":      body.IP,
		})
	})

	// ─── Bootstrap: probe all machines ───────────────────────
	api.Post("/machines/bootstrap-all", func(c *fiber.Ctx) error {
		for _, nodeIP := range pve.Nodes {
			status, err := pve.GetNodeStatus(nodeIP)
			if err != nil {
				continue
			}
			vms, _ := pve.GetVMs(nodeIP, status.Name)
			for _, vm := range vms {
				if vm.Status != "running" {
					continue
				}
				ip := pve.GetVMIP(nodeIP, status.Name, vm.VMID)
				if ip == "" {
					ip = knownIPs[vm.VMID]
				}
				if ip != "" {
					go bootstrap.Probe(ip, vm.Name)
				}
			}
		}
		return c.JSON(fiber.Map{"message": "bootstrap initiated for all running VMs"})
	})

	// ─── Bootstrap status ────────────────────────────────────
	api.Get("/machines/bootstrap-status", func(c *fiber.Ctx) error {
		bootstrap.mu.Lock()
		defer bootstrap.mu.Unlock()
		statuses := make([]*BootstrapStatus, 0, len(bootstrap.status))
		for _, s := range bootstrap.status {
			statuses = append(statuses, s)
		}
		return c.JSON(fiber.Map{"statuses": statuses})
	})

	// ─── Placeholder routes ──────────────────────────────────
	api.Get("/me", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "auth not implemented yet"})
	})
	api.Get("/alerts", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"alerts": []interface{}{}})
	})

	// ─── Prometheus metrics query ────────────────────────────
	promURL := getEnv("ATLAB_PROMETHEUS_URL", "http://10.101.53.212:9090")

	api.Get("/metrics", func(c *fiber.Ctx) error {
		ip := c.Query("ip")
		if ip == "" {
			return c.Status(400).JSON(fiber.Map{"error": "ip query param required"})
		}
		instance := ip + ":9100"

		type MetricResult struct {
			CPU          float64 `json:"cpu_percent"`
			MemTotal     float64 `json:"mem_total_bytes"`
			MemAvailable float64 `json:"mem_available_bytes"`
			MemPercent   float64 `json:"mem_percent"`
			DiskTotal    float64 `json:"disk_total_bytes"`
			DiskFree     float64 `json:"disk_free_bytes"`
			DiskPercent  float64 `json:"disk_percent"`
			Load1        float64 `json:"load_1"`
			Load5        float64 `json:"load_5"`
			Load15       float64 `json:"load_15"`
			NetRxBytes   float64 `json:"net_rx_bytes_per_sec"`
			NetTxBytes   float64 `json:"net_tx_bytes_per_sec"`
			Uptime       float64 `json:"uptime_seconds"`
		}

		result := MetricResult{}

		// CPU usage (percentage)
		cpuVal := queryPrometheus(promURL, fmt.Sprintf(
			`100 - (avg(rate(node_cpu_seconds_total{instance="%s",mode="idle"}[2m])) * 100)`, instance))
		result.CPU = cpuVal

		// Memory
		result.MemTotal = queryPrometheus(promURL, fmt.Sprintf(
			`node_memory_MemTotal_bytes{instance="%s"}`, instance))
		result.MemAvailable = queryPrometheus(promURL, fmt.Sprintf(
			`node_memory_MemAvailable_bytes{instance="%s"}`, instance))
		if result.MemTotal > 0 {
			result.MemPercent = (result.MemTotal - result.MemAvailable) / result.MemTotal * 100
		}

		// Disk (root filesystem)
		result.DiskTotal = queryPrometheus(promURL, fmt.Sprintf(
			`node_filesystem_size_bytes{instance="%s",mountpoint="/"}`, instance))
		result.DiskFree = queryPrometheus(promURL, fmt.Sprintf(
			`node_filesystem_avail_bytes{instance="%s",mountpoint="/"}`, instance))
		if result.DiskTotal > 0 {
			result.DiskPercent = (result.DiskTotal - result.DiskFree) / result.DiskTotal * 100
		}

		// Load average
		result.Load1 = queryPrometheus(promURL, fmt.Sprintf(
			`node_load1{instance="%s"}`, instance))
		result.Load5 = queryPrometheus(promURL, fmt.Sprintf(
			`node_load5{instance="%s"}`, instance))
		result.Load15 = queryPrometheus(promURL, fmt.Sprintf(
			`node_load15{instance="%s"}`, instance))

		// Network (rate per second, first non-lo interface)
		result.NetRxBytes = queryPrometheus(promURL, fmt.Sprintf(
			`rate(node_network_receive_bytes_total{instance="%s",device!="lo"}[2m])`, instance))
		result.NetTxBytes = queryPrometheus(promURL, fmt.Sprintf(
			`rate(node_network_transmit_bytes_total{instance="%s",device!="lo"}[2m])`, instance))

		// Uptime
		result.Uptime = queryPrometheus(promURL, fmt.Sprintf(
			`time() - node_boot_time_seconds{instance="%s"}`, instance))

		return c.JSON(result)
	})

	log.Printf("✓ ATLAS Backend v0.3.0 rodando em :%s", port)
	log.Printf("  Proxmox nodes: %v", pve.Nodes)
	log.Printf("  Known VMs: %v", knownIPs)

	// Auto-bootstrap all known VMs on startup
	go func() {
		time.Sleep(5 * time.Second)
		log.Println("[bootstrap] Iniciando auto-bootstrap de VMs conhecidas...")
		for vmid, ip := range knownIPs {
			if ip != "" {
				name := fmt.Sprintf("VM-%d", vmid)
				for _, nodeIP := range pve.Nodes {
					st, err := pve.GetNodeStatus(nodeIP)
					if err != nil {
						continue
					}
					vms, _ := pve.GetVMs(nodeIP, st.Name)
					for _, vm := range vms {
						if vm.VMID == vmid {
							name = vm.Name
						}
					}
				}
				go bootstrap.Probe(ip, name)
			}
		}
	}()

	log.Fatal(app.Listen(fmt.Sprintf(":%s", port)))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// queryPrometheus executa uma instant query e retorna o valor numérico.
func queryPrometheus(baseURL, query string) float64 {
	reqURL := fmt.Sprintf("%s/api/v1/query?query=%s", baseURL, url.QueryEscape(query))
	resp, err := http.Get(reqURL)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0
	}

	var result struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Value []interface{} `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0
	}

	if result.Status != "success" || len(result.Data.Result) == 0 {
		return 0
	}

	// Value is [timestamp, "value_string"]
	values := result.Data.Result[0].Value
	if len(values) < 2 {
		return 0
	}
	valStr, ok := values[1].(string)
	if !ok {
		return 0
	}

	var val float64
	fmt.Sscanf(valStr, "%f", &val)
	return val
}
