package plugins_test

import (
	"testing"

	"github.com/derpixler/skolva/plugins"
)

func TestAll(t *testing.T) {
	result := plugins.All()
	if result == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(result))
	}
}
