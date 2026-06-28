// Skolva API server — a self-hosted community management platform.
//
// The composition root: it loads config, opens the database pools, builds the
// module assembly and its dependency bundle, drives the module lifecycle
// (hooks, activation) and starts the HTTP server and background worker.
package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/derpixler/skolva/internal/app"
	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/core/ai"
	"github.com/derpixler/skolva/internal/core/cache"
	"github.com/derpixler/skolva/internal/core/config"
	"github.com/derpixler/skolva/internal/core/database"
	"github.com/derpixler/skolva/internal/core/events"
	"github.com/derpixler/skolva/internal/core/hooks"
	"github.com/derpixler/skolva/internal/core/jobs"
	"github.com/derpixler/skolva/internal/core/mail"
	"github.com/derpixler/skolva/internal/core/module"
	"github.com/derpixler/skolva/internal/core/search"
	"github.com/derpixler/skolva/internal/core/secrets"
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

	// Extensibility bus shared by all modules.
	hookManager := hooks.NewHookManager()

	// AI provider — no-op until the AI module lands.
	aiProvider := ai.NewNoopProvider()
	_ = aiProvider

	// Background jobs — River worker.
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

	tokenManager, err := auth.NewTokenManager(cfg.JWTSecret, time.Duration(cfg.JWTExpiryHours)*time.Hour)
	if err != nil {
		log.Printf("failed to create token manager: %v", err)
		return
	}
	verify := auth.NewVerifier(tokenManager)

	cipher, err := secrets.NewCipher(cfg.EncryptionKey)
	if err != nil {
		log.Printf("failed to create secrets cipher: %v", err)
		return
	}

	mailer := mail.NewSMTPMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPFrom)

	// Module assembly + typed dependency bundle (composition root).
	deps := module.Deps{
		DB:     pools.Web,
		Hooks:  hookManager,
		Cipher: cipher,
		Mailer: mailer,
		Events: events.NewInProc(hookManager),
		Cache:  cache.NewMemory(),
		Search: search.NewService(pools.Web),
		Logger: slog.Default(),
	}
	registry := app.DefaultRegistry(tokenManager)

	if err := registry.RegisterHooks(hookManager); err != nil {
		log.Printf("failed to register module hooks: %v", err)
		return
	}
	if err := registry.ActivateAll(ctx, deps); err != nil {
		log.Printf("failed to activate modules: %v", err)
		return
	}

	router := app.NewRouter(pools, registry, deps, verify)

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
	_ = registry.DeactivateAll(context.Background())
}
