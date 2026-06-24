package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/derpixler/skolva/internal/core/jobs"
)

func TestDefaultScheduledJobs(t *testing.T) {
	j := jobs.DefaultScheduledJobs()
	if j == nil {
		t.Fatal("expected non-nil slice")
	}
}

func TestScheduledJobStruct(t *testing.T) {
	job := jobs.ScheduledJob{
		Name:     "test-job",
		Schedule: time.Hour,
	}

	if job.Name != "test-job" {
		t.Errorf("expected test-job, got %s", job.Name)
	}
	if job.Schedule != time.Hour {
		t.Errorf("expected 1h, got %v", job.Schedule)
	}
}

func TestRegisterScheduledJobs(t *testing.T) {
	jobs.RegisterScheduledJobs(nil, []jobs.ScheduledJob{
		{Name: "test-job", Schedule: time.Hour},
	})
}

func TestSchedulerContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		jobs.StartScheduler(ctx, nil)
	}()

	cancel()
	time.Sleep(10 * time.Millisecond)
}
