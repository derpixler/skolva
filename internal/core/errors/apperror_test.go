package errors_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	apperrors "github.com/derpixler/skolva/internal/core/errors"
)

func TestNewNotFound(t *testing.T) {
	err := apperrors.NewNotFound("User")
	if err.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND, got %s", err.Code)
	}
	if err.HTTPStatus != http.StatusNotFound {
		t.Errorf("expected 404, got %d", err.HTTPStatus)
	}
	if err.Message != "User not found" {
		t.Errorf("unexpected message: %s", err.Message)
	}
}

func TestNewValidation(t *testing.T) {
	err := apperrors.NewValidation("email is required")
	if err.Code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %s", err.Code)
	}
	if err.HTTPStatus != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", err.HTTPStatus)
	}
}

func TestNewConflict(t *testing.T) {
	err := apperrors.NewConflict("overlap detected")
	if err.HTTPStatus != http.StatusConflict {
		t.Errorf("expected 409, got %d", err.HTTPStatus)
	}
}

func TestNewUnauthorized(t *testing.T) {
	err := apperrors.NewUnauthorized("")
	if err.Message != "unauthorized" {
		t.Errorf("expected default message, got %s", err.Message)
	}
}

func TestNewForbidden(t *testing.T) {
	err := apperrors.NewForbidden("insufficient permissions")
	if err.HTTPStatus != http.StatusForbidden {
		t.Errorf("expected 403, got %d", err.HTTPStatus)
	}
}

func TestNewInternal(t *testing.T) {
	err := apperrors.NewInternal("")
	if err.Message != "internal error" {
		t.Errorf("expected default message, got %s", err.Message)
	}
}

func TestFromPGErrorUniqueViolation(t *testing.T) {
	pgErr := &pgconn.PgError{
		Code:    "23505",
		Message: "duplicate key value violates unique constraint",
	}

	err := apperrors.FromPGError(pgErr)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.HTTPStatus != http.StatusConflict {
		t.Errorf("expected 409, got %d", err.HTTPStatus)
	}
}

func TestFromPGErrorForeignKeyViolation(t *testing.T) {
	pgErr := &pgconn.PgError{
		Code:    "23503",
		Message: "insert or update violates foreign key constraint",
	}

	err := apperrors.FromPGError(pgErr)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.HTTPStatus != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", err.HTTPStatus)
	}
}

func TestFromPGErrorCheckViolation(t *testing.T) {
	pgErr := &pgconn.PgError{
		Code:    "23514",
		Message: "new row violates check constraint",
	}

	err := apperrors.FromPGError(pgErr)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.HTTPStatus != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", err.HTTPStatus)
	}
}

func TestFromPGErrorExclusionViolation(t *testing.T) {
	pgErr := &pgconn.PgError{
		Code:    "23P01",
		Message: "conflicting key value violates exclusion constraint",
		Detail:  "Pachtvertrag ueberschneidet sich zeitlich.",
	}

	err := apperrors.FromPGError(pgErr)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.HTTPStatus != http.StatusConflict {
		t.Errorf("expected 409, got %d", err.HTTPStatus)
	}
	if err.Message != "Pachtvertrag ueberschneidet sich zeitlich." {
		t.Errorf("unexpected message: %s", err.Message)
	}
}

func TestFromPGErrorUnknown(t *testing.T) {
	pgErr := &pgconn.PgError{
		Code:    "XX999",
		Message: "some unknown error",
	}

	err := apperrors.FromPGError(pgErr)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.HTTPStatus != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", err.HTTPStatus)
	}
}

func TestFromErrorWithAppError(t *testing.T) {
	original := apperrors.NewNotFound("User")
	result := apperrors.FromError(original)
	if result.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND, got %s", result.Code)
	}
}

func TestFromErrorWithGenericError(t *testing.T) {
	original := errors.New("something went wrong")
	result := apperrors.FromError(original)
	if result.Code != "INTERNAL_ERROR" {
		t.Errorf("expected INTERNAL_ERROR, got %s", result.Code)
	}
	if result.HTTPStatus != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", result.HTTPStatus)
	}
}

func TestAppErrorIs(t *testing.T) {
	err := apperrors.NewNotFound("User")
	if !errors.Is(err, &apperrors.AppError{Code: "NOT_FOUND"}) {
		t.Error("expected Is to match on code")
	}
}

func TestAppErrorErrorString(t *testing.T) {
	err := apperrors.NewConflict("overlap")
	if err.Error() != "overlap" {
		t.Errorf("unexpected error string: %s", err.Error())
	}
}
