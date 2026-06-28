package auth

import (
	"errors"
	"slices"
	"testing"
)

// Gated detail-logging + assertion helpers for the white-box (package auth)
// unit tests, mirroring the format used by the black-box HTTP tests
// (handler_test.go, package auth_test). Output appears only under
// `go test -v`, so plain `go test ./...` (coverage / full-check) stays quiet.

func tstep(t *testing.T, name string) {
	t.Helper()
	if testing.Verbose() {
		t.Logf("[step] %s", name)
	}
}

func tlog(t *testing.T, format string, args ...any) {
	t.Helper()
	if testing.Verbose() {
		t.Logf(format, args...)
	}
}

// assertErrIs checks errors.Is(got, want), logging want/got on success
// (verbose) and failing with want/got on mismatch.
func assertErrIs(t *testing.T, got, want error, desc string) {
	t.Helper()
	if errors.Is(got, want) {
		tlog(t, "[chk ] %s: want=%v got=%v  OK", desc, want, got)
		return
	}
	t.Errorf("[chk ] %s: want=%v got=%v  FAIL", desc, want, got)
}

// assertEq checks equality of comparable values with want/got logging.
func assertEq[T comparable](t *testing.T, got, want T, desc string) {
	t.Helper()
	if got == want {
		tlog(t, "[chk ] %s: want=%v got=%v  OK", desc, want, got)
		return
	}
	t.Errorf("[chk ] %s: want=%v got=%v  FAIL", desc, want, got)
}

// assertSliceEq checks slice equality with want/got logging.
func assertSliceEq[T comparable](t *testing.T, got, want []T, desc string) {
	t.Helper()
	if slices.Equal(got, want) {
		tlog(t, "[chk ] %s: want=%v got=%v  OK", desc, want, got)
		return
	}
	t.Errorf("[chk ] %s: want=%v got=%v  FAIL", desc, want, got)
}
