// Package errors provides structured application errors with HTTP status mapping.
//
// Every error carries a machine-readable Code, a human-readable Message,
// and an HTTPStatus for the API layer. PostgreSQL errors are automatically
// translated via pgErrorMap.
package errors

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
)

// AppError is a structured error that maps cleanly to HTTP responses.
type AppError struct {
	Code       string `json:"code"`    // machine-readable, e.g. "NOT_FOUND"
	Message    string `json:"message"` // human-readable
	HTTPStatus int    `json:"-"`       // HTTP status code for the response
	Err        error  `json:"-"`       // underlying error, if any
}

// Error returns the human-readable message.
func (e *AppError) Error() string { return e.Message }

// Unwrap returns the underlying error for errors.Is/errors.As.
func (e *AppError) Unwrap() error { return e.Err }

// New creates a new AppError with the given code, message, and HTTP status.
func New(code, message string, status int) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: status}
}

// NewNotFound returns a 404 AppError for the given entity name.
func NewNotFound(entity string) *AppError {
	return New("NOT_FOUND", fmt.Sprintf("%s not found", entity), http.StatusNotFound)
}

// NewValidation returns a 422 AppError for invalid input.
func NewValidation(message string) *AppError {
	return New("VALIDATION_ERROR", message, http.StatusUnprocessableEntity)
}

// NewConflict returns a 409 AppError for resource conflicts.
func NewConflict(message string) *AppError {
	return New("CONFLICT", message, http.StatusConflict)
}

// NewUnauthorized returns a 401 AppError. Falls back to "unauthorized" if
// message is empty.
func NewUnauthorized(message string) *AppError {
	if message == "" {
		message = "unauthorized"
	}
	return New("UNAUTHORIZED", message, http.StatusUnauthorized)
}

// NewForbidden returns a 403 AppError. Falls back to "forbidden" if message
// is empty.
func NewForbidden(message string) *AppError {
	if message == "" {
		message = "forbidden"
	}
	return New("FORBIDDEN", message, http.StatusForbidden)
}

// NewInternal returns a 500 AppError. Falls back to "internal error" if
// message is empty.
func NewInternal(message string) *AppError {
	if message == "" {
		message = "internal error"
	}
	return New("INTERNAL_ERROR", message, http.StatusInternalServerError)
}

// pgErrorMap translates well-known PostgreSQL error codes to HTTP statuses.
// Detail and Hint from the PG error are used when available.
var pgErrorMap = map[string]struct {
	Status  int
	Message string
}{
	"23505": {http.StatusConflict, "This record already exists (duplicate)."},
	"23503": {http.StatusBadRequest, "Referenced record does not exist."},
	"23514": {http.StatusUnprocessableEntity, "Value violates check constraint."},
	"23P01": {http.StatusConflict, "Time range overlaps with an existing record."},
	"P0001": {http.StatusUnprocessableEntity, "Database constraint violated."},
}

// FromPGError attempts to translate a PostgreSQL error into an AppError.
// Returns nil if the error is not a *pgconn.PgError or has no known mapping.
func FromPGError(err error) *AppError {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if m, ok := pgErrorMap[pgErr.Code]; ok {
			msg := m.Message
			if pgErr.Detail != "" {
				msg = pgErr.Detail
			}
			if pgErr.Hint != "" {
				msg += " " + pgErr.Hint
			}
			return New(pgErr.Code, msg, m.Status)
		}
		return New("PG_ERROR", pgErr.Message, http.StatusInternalServerError)
	}
	return nil
}

// FromError converts any error into an AppError. PostgreSQL errors are
// translated; existing AppErrors are returned as-is; everything else
// becomes a 500 internal error.
func FromError(err error) *AppError {
	if appErr := FromPGError(err); appErr != nil {
		return appErr
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return NewInternal(err.Error())
}

// Is compares two AppErrors by their Code field.
func (e *AppError) Is(target error) bool {
	t, ok := target.(*AppError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}
