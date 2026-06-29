package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryGetSetDelete(t *testing.T) {
	ctx := context.Background()
	c := NewMemory()

	if _, ok, _ := c.Get(ctx, "missing"); ok {
		t.Error("expected miss for unknown key")
	}

	if err := c.Set(ctx, "k", []byte("v"), 0); err != nil {
		t.Fatalf("set: %v", err)
	}
	if v, ok, _ := c.Get(ctx, "k"); !ok || string(v) != "v" {
		t.Errorf("get: want v/true, got %q/%v", v, ok)
	}

	_ = c.Delete(ctx, "k")
	if _, ok, _ := c.Get(ctx, "k"); ok {
		t.Error("expected miss after delete")
	}
}

func TestMemoryTTLExpiry(t *testing.T) {
	ctx := context.Background()
	c := NewMemory()
	base := time.Now()
	c.now = func() time.Time { return base }

	if err := c.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, ok, _ := c.Get(ctx, "k"); !ok {
		t.Error("expected hit before expiry")
	}

	c.now = func() time.Time { return base.Add(2 * time.Minute) }
	if _, ok, _ := c.Get(ctx, "k"); ok {
		t.Error("expected miss after expiry")
	}
}
