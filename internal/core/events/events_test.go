package events_test

import (
	"context"
	"testing"

	"github.com/derpixler/skolva/internal/core/events"
	"github.com/derpixler/skolva/internal/core/hooks"
)

func TestInProcPublishSubscribe(t *testing.T) {
	bus := events.NewInProc(hooks.NewHookManager())

	var got any
	bus.Subscribe("user.created", func(_ context.Context, payload any) error {
		got = payload
		return nil
	})

	if err := bus.Publish(context.Background(), "user.created", "u-1"); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if got != "u-1" {
		t.Errorf("subscriber payload: want %q, got %v", "u-1", got)
	}

	// publishing to a topic with no subscribers is a no-op
	if err := bus.Publish(context.Background(), "no.subscribers", 42); err != nil {
		t.Errorf("publish to empty topic: %v", err)
	}
}
