package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/derpixler/skolva/internal/app"
	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/core/ai"
	"github.com/derpixler/skolva/internal/core/config"
	"github.com/derpixler/skolva/internal/core/database"
	"github.com/derpixler/skolva/internal/core/hooks"
	"github.com/derpixler/skolva/internal/core/jobs"
	"github.com/derpixler/skolva/internal/core/middleware"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		log.Printf("failed to load config: %v", err)
		return
	}

	pools, err := database.NewPools(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Printf("failed to create database pools: %v", err)
		return
	}
	defer pools.Close()

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

	aiProvider := ai.NewNoopProvider()
	_ = aiProvider

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

	verify := func(token string) (*middleware.Actor, error) {
		claims, err := tokenManager.Verify(token)
		if err != nil {
			return nil, err
		}
		if claims.Kind != auth.TokenKindAccess {
			return nil, fmt.Errorf("token is not an access token")
		}
		return &middleware.Actor{
			UserID:      claims.Subject,
			Email:       claims.Email,
			Roles:       claims.Roles,
			Permissions: claims.Permissions,
		}, nil
	}

	router := app.NewRouter(pools, hookManager, worker, verify)

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
