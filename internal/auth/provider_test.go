package auth_test

import (
	"testing"

	"github.com/derpixler/skolva/internal/auth"
)

func TestProviderForFallsBackToLocal(t *testing.T) {
	for _, name := range []string{"", "local", "unknown", "keycloak"} {
		if got := auth.ProviderFor(name).Name(); got != "local" {
			t.Errorf("ProviderFor(%q): want local, got %q", name, got)
		}
	}
}
