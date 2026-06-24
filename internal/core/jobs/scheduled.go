package jobs

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

type ScheduledJob struct {
	Name     string
	Schedule time.Duration
	Job      river.JobArgs
}

var defaultJobs = []ScheduledJob{}

func RegisterScheduledJobs(client *river.Client[pgx.Tx], jobs []ScheduledJob) {
	for _, j := range jobs {
		log.Printf("registered scheduled job: %s (interval: %s)", j.Name, j.Schedule)
	}
}

func StartScheduler(ctx context.Context, client *river.Client[pgx.Tx]) {
	go func() {
		log.Println("scheduler started")
		<-ctx.Done()
		log.Println("scheduler stopped")
	}()
}

func DefaultScheduledJobs() []ScheduledJob {
	return defaultJobs
}
