package errorcode

import "net/http"

const (
	// CodeGeneralError normal or unclassified error
	CodeGeneralError = 1

	// CodeBadBinding binding failed
	CodeBadBinding = 600001
	// CodeUnauthorized stands for invalid token,
	// which is an umbrella error exposed to public
	CodeUnauthorized = 600401
)

var (
	// ErrUnauthorized stands for invalid token, which is an umbrella error exposed to public
	ErrUnauthorized = newError(http.StatusForbidden, CodeUnauthorized, "Unauthorized")
)

// Error standard API error
type Error struct {
	// http status code
	statusCode int
	// business layer code
	code    int
	message string
}

// newError create a new API error
func newError(statusCode, code int, message string) *Error {
	return &Error{
		statusCode: statusCode,
		code:       code,
		message:    message,
	}
}

// Code returns error code
func (e *Error) Code() int {
	return e.code
}

// StatusCode returns http status code
func (e *Error) StatusCode() int {
	return e.statusCode
}

// Error implements error interface
func (e *Error) Error() string {
	return e.message
}
