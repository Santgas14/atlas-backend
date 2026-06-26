// Package main — Entry point do backend ATLAB Platform.
//
// Inicializa: config, database, redis, auth provider,
// Proxmox clients, e sobe o servidor HTTP/WebSocket.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/atlab-ufc/atlab-backend/internal/config"
	"github.com/atlab-ufc/atlab-backend/internal/database"
	"github.com/atlab-ufc/atlab-backend/internal/server"
)

func main() {
	// Load configuration from environment
	cfg := config.Load()

	// Connect to PostgreSQL
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Falha ao conectar no PostgreSQL: %v", err)
	}
	defer db.Close()
	log.Println("✓ PostgreSQL conectado")

	// Build and start server
	srv := server.New(cfg, db)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("✓ ATLAB Backend rodando em :%s", cfg.Port)
		if err := srv.Listen(":" + cfg.Port); err != nil {
			log.Fatalf("Servidor encerrou: %v", err)
		}
	}()

	<-quit
	log.Println("Desligando servidor...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.ShutdownWithContext(ctx)
	log.Println("✓ Servidor encerrado com sucesso")
}
