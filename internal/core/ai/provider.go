package ai

import "context"

type Provider interface {
	Complete(ctx context.Context, prompt string, opts CompletionOpts) (string, error)
	Classify(ctx context.Context, text string, categories []string) (string, float64, error)
	Extract(ctx context.Context, text string, schema any) (any, error)
}

type CompletionOpts struct {
	Temperature float64
	MaxTokens   int
	Model       string
}
