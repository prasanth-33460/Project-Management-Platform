package apperror

import "fmt"

// AppError is a domain error that carries an HTTP status code.
type AppError struct {
	Code    int         `json:"-"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func New(code int, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func WithDetails(code int, message string, details interface{}) *AppError {
	return &AppError{Code: code, Message: message, Details: details}
}

// Common sentinel errors
var (
	ErrNotFound     = New(404, "resource not found")
	ErrUnauthorized = New(401, "unauthorized")
	ErrForbidden    = New(403, "forbidden")
	ErrConflict     = New(409, "concurrent update detected, please retry")
)
