package search_test

import (
	"context"
	"errors"
	"testing"

	"github.com/derpixler/skolva/internal/core/search"
)

// TestServiceUnknownTable exercises the seam's validation path without a
// database: an unknown table is rejected before any query runs.
func TestServiceUnknownTable(t *testing.T) {
	svc := search.NewService(nil)
	_, err := svc.Search(context.Background(), "not_a_table", "query", 10)
	if !errors.Is(err, search.ErrUnknownTable) {
		t.Errorf("expected ErrUnknownTable, got %v", err)
	}
}
