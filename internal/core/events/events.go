// Package events provides an event bus seam. The default implementation is
// in-process, backed by the hooks action bus; alternative backends (Redis
// Streams, NATS, RabbitMQ) can implement Bus without changing module code.
package events

import (
	"context"

	"github.com/derpixler/skolva/internal/core/hooks"
)

// Handler reacts to a published event payload.
type Handler func(ctx context.Context, payload any) error

// Bus publishes events to topics and lets subscribers react. It is the
// swappable seam for inter-module, decoupled communication.
type Bus interface {
	Publish(ctx context.Context, topic string, payload any) error
	Subscribe(topic string, h Handler)
}

const payloadKey = "payload"

// inProc is the default in-process bus over the hooks action manager.
type inProc struct {
	hm *hooks.HookManager
}

// NewInProc returns the default in-process event bus.
func NewInProc(hm *hooks.HookManager) Bus {
	return &inProc{hm: hm}
}

func (b *inProc) Publish(ctx context.Context, topic string, payload any) error {
	return b.hm.DoActions(ctx, topic, hooks.HookContext{payloadKey: payload})
}

func (b *inProc) Subscribe(topic string, h Handler) {
	b.hm.AddAction(topic, 100, "eventbus", func(ctx context.Context, hc hooks.HookContext) error {
		return h(ctx, hc[payloadKey])
	})
}
