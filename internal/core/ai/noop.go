package ai

import "context"

type NoopProvider struct{}

func NewNoopProvider() *NoopProvider {
	return &NoopProvider{}
}

func (p *NoopProvider) Complete(ctx context.Context, prompt string, opts CompletionOpts) (string, error) {
	return "", nil
}

func (p *NoopProvider) Classify(ctx context.Context, text string, categories []string) (string, float64, error) {
	return "", 0, nil
}

func (p *NoopProvider) Extract(ctx context.Context, text string, schema any) (any, error) {
	return nil, nil
}
