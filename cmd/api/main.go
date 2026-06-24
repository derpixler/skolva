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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pools, err := database.NewPools(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to create database pools: %v", err)
	}
	defer pools.Close()

	hookManager := hooks.NewHookManager()
	pluginRegistry := hooks.NewPluginRegistry()

	if err := pluginRegistry.RegisterAll(hookManager); err != nil {
		log.Fatalf("failed to register plugins: %v", err)
	}

	if err := pluginRegistry.ActivateAll(pools.Web); err != nil {
		log.Fatalf("failed to activate plugins: %v", err)
	}

	aiProvider := ai.NewNoopProvider()
	_ = aiProvider

	worker, err := jobs.NewWorker(ctx, pools.Worker)
	if err != nil {
		log.Fatalf("failed to create worker: %v", err)
	}

	go func() {
		if err := worker.Start(ctx); err != nil {
			log.Printf("worker stopped: %v", err)
		}
	}()

	jobs.StartScheduler(ctx, worker.Client())

	router := app.NewRouter(pools, hookManager, worker)

	go func() {
		addr := ":" + cfg.Port
		log.Printf("server starting on %s", addr)
		if err := router.Run(addr); err != nil {
			log.Fatalf("server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	_ = worker.Stop(context.Background())
	_ = pluginRegistry.DeactivateAll()
}
