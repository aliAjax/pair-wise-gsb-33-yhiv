package util

import "fmt"

// AppError is a business error carrying an HTTP status and a business code.
// Data optionally carries an existing resource (e.g. the already-submitted
// plan returned on a concurrent duplicate submission).
type AppError struct {
	HTTPStatus int
	Code       int
	Message    string
	Data       any
}

func (e *AppError) Error() string {
	return fmt.Sprintf("code=%d message=%s", e.Code, e.Message)
}

// NewAppError builds an AppError with the given status/code/message.
func NewAppError(httpStatus, code int, message string) *AppError {
	return &AppError{HTTPStatus: httpStatus, Code: code, Message: message}
}

// WithData attaches a payload to the error and returns the same error.
func (e *AppError) WithData(data any) *AppError {
	e.Data = data
	return e
}
