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

type Worker struct {
	client     *river.Client[pgx.Tx]
	workerPool *pgxpool.Pool
}

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

	return &Worker{
		client:     client,
		workerPool: workerPool,
	}, nil
}

func (w *Worker) Start(ctx context.Context) error {
	log.Println("starting river worker")
	return w.client.Start(ctx)
}

func (w *Worker) Stop(ctx context.Context) error {
	log.Println("stopping river worker")
	return w.client.Stop(ctx)
}

func (w *Worker) Client() *river.Client[pgx.Tx] {
	return w.client
}
