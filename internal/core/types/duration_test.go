package types_test

import (
	"testing"
	"time"

	"github.com/derpixler/skolva/internal/core/types"
)

func TestParseDurationStandard(t *testing.T) {
	d, err := types.ParseDuration("1h30m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 90*time.Minute {
		t.Errorf("expected 90m, got %v", d)
	}
}

func TestParseDurationDays(t *testing.T) {
	d, err := types.ParseDuration("3d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 72*time.Hour {
		t.Errorf("expected 72h, got %v", d)
	}
}

func TestParseDurationDaysAndHours(t *testing.T) {
	d, err := types.ParseDuration("2d5h")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 53*time.Hour {
		t.Errorf("expected 53h, got %v", d)
	}
}

func TestParseDurationMonths(t *testing.T) {
	d, err := types.ParseDuration("1M")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 30*24*time.Hour {
		t.Errorf("expected 720h, got %v", d)
	}
}

func TestParseDurationComplex(t *testing.T) {
	d, err := types.ParseDuration("1d3M2h30m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := 24*time.Hour + 3*30*24*time.Hour + 2*time.Hour + 30*time.Minute
	if d != expected {
		t.Errorf("expected %v, got %v", expected, d)
	}
}

func TestParseDurationEmpty(t *testing.T) {
	_, err := types.ParseDuration("")
	if err == nil {
		t.Error("expected error for empty duration")
	}
}

func TestParseDurationInvalid(t *testing.T) {
	_, err := types.ParseDuration("xyz")
	if err == nil {
		t.Error("expected error for invalid duration")
	}
}

func TestMustParseDuration(t *testing.T) {
	d := types.MustParseDuration("1d")
	if d != 24*time.Hour {
		t.Errorf("expected 24h, got %v", d)
	}
}

func TestMustParseDurationPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid duration")
		}
	}()
	types.MustParseDuration("not-valid")
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{0, "0s"},
		{30 * time.Minute, "30m"},
		{2*time.Hour + 30*time.Minute, "2h30m"},
		{25 * time.Hour, "1d1h0m"},
	}

	for _, tt := range tests {
		result := types.FormatDuration(tt.input)
		if result != tt.expected {
			t.Errorf("FormatDuration(%v) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}
