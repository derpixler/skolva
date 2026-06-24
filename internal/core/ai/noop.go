package ai

import "context"

// NoopProvider implements Provider with no-op methods that return
// zero values. It is used during Phase 1 when no AI backend is
// configured and will be replaced by a real provider in Phase 9.
type NoopProvider struct{}

// NewNoopProvider returns a Provider that always returns zero values.
func NewNoopProvider() *NoopProvider { return &NoopProvider{} }

// Complete returns an empty string and nil error.
func (p *NoopProvider) Complete(ctx context.Context, prompt string, opts CompletionOpts) (string, error) {
	return "", nil
}

// Classify returns an empty category, 0 confidence, and nil error.
func (p *NoopProvider) Classify(ctx context.Context, text string, categories []string) (string, float64, error) {
	return "", 0, nil
}

// Extract returns nil data and nil error.
func (p *NoopProvider) Extract(ctx context.Context, text string, schema any) (any, error) {
	return nil, nil
}
