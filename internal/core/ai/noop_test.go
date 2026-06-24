package ai_test

import (
	"context"
	"testing"

	"github.com/derpixler/skolva/internal/core/ai"
)

func TestNoopComplete(t *testing.T) {
	provider := ai.NewNoopProvider()

	result, err := provider.Complete(context.Background(), "some prompt", ai.CompletionOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got '%s'", result)
	}
}

func TestNoopClassify(t *testing.T) {
	provider := ai.NewNoopProvider()

	category, confidence, err := provider.Classify(context.Background(), "text", []string{"a", "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if category != "" {
		t.Errorf("expected empty category, got '%s'", category)
	}
	if confidence != 0 {
		t.Errorf("expected 0 confidence, got %f", confidence)
	}
}

func TestNoopExtract(t *testing.T) {
	provider := ai.NewNoopProvider()

	result, err := provider.Extract(context.Background(), "text", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result")
	}
}

func TestNoopProviderImplementsInterface(t *testing.T) {
	var _ ai.Provider = ai.NewNoopProvider()
}
