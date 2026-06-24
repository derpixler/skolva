// Package database manages PostgreSQL connection pools via pgx.
//
// Two isolated pools are maintained: a web pool (max 20 connections) for
// HTTP request handling, and a worker pool (max 5) for background jobs.
// This prevents a stuck worker from starving web requests.
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pools holds two isolated pgx connection pools sharing the same database.
type Pools struct {
	Web    *pgxpool.Pool // HTTP request handling, max 20 connections
	Worker *pgxpool.Pool // background jobs (river), max 5 connections
}

// NewPools creates both connection pools from the same database URL.
// The web pool is created first; if worker pool creation fails,
// the web pool is closed before returning the error.
func NewPools(ctx context.Context, databaseURL string) (*Pools, error) {
	webCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse web pool config: %w", err)
	}
	webCfg.MaxConns = 20

	webPool, err := pgxpool.NewWithConfig(ctx, webCfg)
	if err != nil {
		return nil, fmt.Errorf("create web pool: %w", err)
	}

	if err := webPool.Ping(ctx); err != nil {
		webPool.Close()
		return nil, fmt.Errorf("ping web pool: %w", err)
	}

	workerCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		webPool.Close()
		return nil, fmt.Errorf("parse worker pool config: %w", err)
	}
	workerCfg.MaxConns = 5

	workerPool, err := pgxpool.NewWithConfig(ctx, workerCfg)
	if err != nil {
		webPool.Close()
		return nil, fmt.Errorf("create worker pool: %w", err)
	}

	if err := workerPool.Ping(ctx); err != nil {
		webPool.Close()
		workerPool.Close()
		return nil, fmt.Errorf("ping worker pool: %w", err)
	}

	return &Pools{Web: webPool, Worker: workerPool}, nil
}

// Close shuts down both pools.
func (p *Pools) Close() {
	if p.Web != nil {
		p.Web.Close()
	}
	if p.Worker != nil {
		p.Worker.Close()
	}
}

// Health pings both pools and returns the first error encountered.
func (p *Pools) Health(ctx context.Context) error {
	if err := p.Web.Ping(ctx); err != nil {
		return fmt.Errorf("web pool: %w", err)
	}
	if err := p.Worker.Ping(ctx); err != nil {
		return fmt.Errorf("worker pool: %w", err)
	}
	return nil
}
