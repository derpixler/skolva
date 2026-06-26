// Package jobs wraps the River job queue for PostgreSQL-backed background work.
//
// A single River client manages four named queues with independent
// worker counts. No job handlers are registered in Phase 1 — the
// queues are ready for future phases.
package jobs

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// Worker wraps a River client and its associated database pool.
type Worker struct {
	client     *river.Client[pgx.Tx]
	workerPool *pgxpool.Pool
}

// NewWorker creates a River client backed by the given pool.
// Four queues are configured but no job handlers are registered yet —
// they will be added as concrete jobs are implemented in later phases.
func NewWorker(ctx context.Context, workerPool *pgxpool.Pool) (*Worker, error) {
	workers := river.NewWorkers()

	client, err := river.NewClient[pgx.Tx](riverpgxv5.New(workerPool), &river.Config{
		Queues: map[string]river.QueueConfig{
			"default":  {MaxWorkers: 10},
			"webhooks": {MaxWorkers: 5},
			"mail":     {MaxWorkers: 3},
			"pdf":      {MaxWorkers: 2},
		},
		Workers: workers,
	})
	if err != nil {
		return nil, fmt.Errorf("create river client: %w", err)
	}

	return &Worker{client: client, workerPool: workerPool}, nil
}

// Start begins processing jobs. Blocks until ctx is cancelled
// or Stop is called.
func (w *Worker) Start(ctx context.Context) error {
	log.Println("starting river worker")
	return w.client.Start(ctx)
}

// Stop gracefully shuts down the worker, waiting for in-flight jobs
// to complete.
func (w *Worker) Stop(ctx context.Context) error {
	log.Println("stopping river worker")
	return w.client.Stop(ctx)
}

// Client returns the underlying River client for direct access
// (e.g., inserting jobs in the same database transaction).
func (w *Worker) Client() *river.Client[pgx.Tx] {
	return w.client
}
