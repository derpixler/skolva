// Package ai defines the interface for AI/LLM providers and contains
// a no-operation implementation for phases where AI is not yet active.
//
// The Provider interface is intentionally minimal — only Complete,
// Classify, and Extract are supported. Specific prompt engineering
// and domain logic lives in the consuming modules.
package ai

import "context"

// Provider is the interface that all AI backends must implement.
// OpenAI-compatible APIs are the primary target, with Ollama as
// the local/offline fallback.
type Provider interface {
	// Complete generates text from a prompt. Used for drafting letters,
	// suggesting event dates, and other open-ended generation tasks.
	Complete(ctx context.Context, prompt string, opts CompletionOpts) (string, error)

	// Classify assigns one of the given categories to the input text,
	// returning the category name and a confidence score between 0 and 1.
	Classify(ctx context.Context, text string, categories []string) (string, float64, error)

	// Extract parses structured data from unstructured text according to
	// the provided schema. Schema is provider-specific (JSON Schema for
	// OpenAI, or a typed struct).
	Extract(ctx context.Context, text string, schema any) (any, error)
}

// CompletionOpts controls text generation behaviour.
type CompletionOpts struct {
	Temperature float64 // 0 = deterministic, 1 = creative
	MaxTokens   int     // maximum tokens in the response
	Model       string  // overrides the default model, e.g. "gpt-4o"
}
