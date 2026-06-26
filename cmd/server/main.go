// ATLAS Backend — Entry point
// Sobe servidor HTTP com integração real ao Proxmox.
package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
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

// ─── Main ────────────────────────────────────────────────────

func main() {
	port := getEnv("ATLAB_PORT", "8080")
	pve := NewProxmoxClient()

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
			"version": "0.2.0",
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

	// ─── All machines (aggregate from all nodes) ─────────────
	api.Get("/machines", func(c *fiber.Ctx) error {
		type MachineItem struct {
			VMID     int     `json:"vmid"`
			Name     string  `json:"name"`
			Type     string  `json:"type"`
			Status   string  `json:"status"`
			Node     string  `json:"node"`
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
				all = append(all, MachineItem{
					VMID: vm.VMID, Name: vm.Name, Type: "vm", Status: vm.Status,
					Node: status.Name, CPUs: vm.CPUs, CPU: vm.CPU * 100,
					MemPct: memPct, MaxMemGB: float64(vm.MaxMem) / 1073741824,
					DiskGB: float64(vm.MaxDisk) / 1073741824, Uptime: vm.Uptime,
				})
			}
			for _, ct := range cts {
				memPct := float64(0)
				if ct.MaxMem > 0 {
					memPct = float64(ct.Mem) / float64(ct.MaxMem) * 100
				}
				all = append(all, MachineItem{
					VMID: ct.VMID, Name: ct.Name, Type: "ct", Status: ct.Status,
					Node: status.Name, CPUs: ct.CPUs, CPU: ct.CPU * 100,
					MemPct: memPct, MaxMemGB: float64(ct.MaxMem) / 1073741824,
					DiskGB: float64(ct.MaxDisk) / 1073741824, Uptime: ct.Uptime,
				})
			}
		}

		return c.JSON(fiber.Map{"machines": all, "total": len(all)})
	})

	// ─── Placeholder routes ──────────────────────────────────
	api.Get("/me", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "auth not implemented yet"})
	})
	api.Get("/alerts", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"alerts": []interface{}{}})
	})

	log.Printf("✓ ATLAS Backend v0.2.0 rodando em :%s", port)
	log.Printf("  Proxmox nodes: %v", pve.Nodes)
	log.Fatal(app.Listen(fmt.Sprintf(":%s", port)))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
