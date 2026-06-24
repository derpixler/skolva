package errors

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
)

type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
	Err        error  `json:"-"`
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func New(code, message string, status int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: status,
	}
}

func NewNotFound(entity string) *AppError {
	return New("NOT_FOUND", fmt.Sprintf("%s not found", entity), http.StatusNotFound)
}

func NewValidation(message string) *AppError {
	return New("VALIDATION_ERROR", message, http.StatusUnprocessableEntity)
}

func NewConflict(message string) *AppError {
	return New("CONFLICT", message, http.StatusConflict)
}

func NewUnauthorized(message string) *AppError {
	if message == "" {
		message = "unauthorized"
	}
	return New("UNAUTHORIZED", message, http.StatusUnauthorized)
}

func NewForbidden(message string) *AppError {
	if message == "" {
		message = "forbidden"
	}
	return New("FORBIDDEN", message, http.StatusForbidden)
}

func NewInternal(message string) *AppError {
	if message == "" {
		message = "internal error"
	}
	return New("INTERNAL_ERROR", message, http.StatusInternalServerError)
}

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

func (e *AppError) Is(target error) bool {
	t, ok := target.(*AppError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}
