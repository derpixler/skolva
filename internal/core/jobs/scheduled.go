package jobs

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

// ScheduledJob represents a recurring job with a name and interval.
type ScheduledJob struct {
	Name     string
	Schedule time.Duration
	Job      river.JobArgs
}

// defaultJobs is empty in Phase 1. Jobs will be added as features are
// implemented: OverdueLendingCheck, DunningEscalation, WorkHourDeadline,
// etc.
var defaultJobs = []ScheduledJob{}

// RegisterScheduledJobs logs the names of scheduled jobs. In a future
// phase this will register them with River's periodic job system.
func RegisterScheduledJobs(client *river.Client[pgx.Tx], jobs []ScheduledJob) {
	for _, j := range jobs {
		log.Printf("registered scheduled job: %s (interval: %s)", j.Name, j.Schedule)
	}
}

// StartScheduler launches a background goroutine that waits for
// context cancellation. Scheduling logic will be added in later phases.
func StartScheduler(ctx context.Context, client *river.Client[pgx.Tx]) {
	go func() {
		log.Println("scheduler started")
		<-ctx.Done()
		log.Println("scheduler stopped")
	}()
}

// DefaultScheduledJobs returns the (currently empty) list of jobs.
func DefaultScheduledJobs() []ScheduledJob {
	return defaultJobs
}
