package types_test

import (
	"encoding/json"
	"testing"

	"github.com/derpixler/skolva/internal/core/types"
)

func TestNewDecimal(t *testing.T) {
	d, err := types.NewDecimal("123.45")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.String() != "123.45" {
		t.Errorf("expected 123.45, got %s", d.String())
	}
}

func TestMustDecimal(t *testing.T) {
	d := types.MustDecimal("67.89")
	if d.String() != "67.89" {
		t.Errorf("expected 67.89, got %s", d.String())
	}
}

func TestMustDecimalPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid decimal")
		}
	}()
	types.MustDecimal("not-a-decimal")
}

func TestDecimalJSONRoundTrip(t *testing.T) {
	type Container struct {
		Amount types.Decimal `json:"amount"`
	}

	d := types.MustDecimal("99.99")
	c := Container{Amount: d}

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var c2 Container
	if err := json.Unmarshal(data, &c2); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if c2.Amount.String() != "99.99" {
		t.Errorf("expected 99.99, got %s", c2.Amount.String())
	}
}

func TestDecimalPrecision(t *testing.T) {
	a := types.MustDecimal("0.1")
	b := types.MustDecimal("0.2")
	sum := a.Add(b)

	if sum.String() != "0.3" {
		t.Errorf("expected 0.3, got %s", sum.String())
	}
}

func TestZeroDecimal(t *testing.T) {
	if !types.Zero.IsZero() {
		t.Error("expected Zero to be zero")
	}
}
