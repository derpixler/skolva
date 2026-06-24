// Skolva API server — a self-hosted community management platform.
//
// Phase 1 (Walking Skeleton): starts a Gin HTTP server with a health
// endpoint, dual database pools, an empty plugin registry, and a
// River job worker. No real business logic yet — just the foundation.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/derpixler/skolva/internal/app"
	"github.com/derpixler/skolva/internal/core/ai"
	"github.com/derpixler/skolva/internal/core/config"
	"github.com/derpixler/skolva/internal/core/database"
	"github.com/derpixler/skolva/internal/core/hooks"
	"github.com/derpixler/skolva/internal/core/jobs"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		log.Printf("failed to load config: %v", err)
		return
	}

	// Database — two isolated connection pools on the same PostgreSQL instance.
	pools, err := database.NewPools(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Printf("failed to create database pools: %v", err)
		return
	}
	defer pools.Close()

	// Plugin system — empty in Phase 1, ready for module registration.
	hookManager := hooks.NewHookManager()
	pluginRegistry := hooks.NewPluginRegistry()

	if err := pluginRegistry.RegisterAll(hookManager); err != nil {
		log.Printf("failed to register plugins: %v", err)
		return
	}

	if err := pluginRegistry.ActivateAll(pools.Web); err != nil {
		log.Printf("failed to activate plugins: %v", err)
		return
	}

	// AI provider — no-op until Phase 9.
	aiProvider := ai.NewNoopProvider()
	_ = aiProvider

	// Background jobs — River worker with empty handler set.
	worker, err := jobs.NewWorker(ctx, pools.Worker)
	if err != nil {
		log.Printf("failed to create worker: %v", err)
		return
	}

	go func() {
		if err := worker.Start(ctx); err != nil {
			log.Printf("worker stopped: %v", err)
		}
	}()

	jobs.StartScheduler(ctx, worker.Client())

	// HTTP server.
	router := app.NewRouter(pools, hookManager, worker)

	go func() {
		addr := ":" + cfg.Port
		log.Printf("server starting on %s", addr)
		if err := router.Run(addr); err != nil {
			log.Printf("server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	_ = worker.Stop(context.Background())
	_ = pluginRegistry.DeactivateAll()
}
