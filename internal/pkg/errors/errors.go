package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode represents a standardized error code
type ErrorCode string

const (
	// Authentication errors
	ErrUnauthorized   ErrorCode = "UNAUTHORIZED"
	ErrTokenExpired   ErrorCode = "TOKEN_EXPIRED"
	ErrTokenInvalid   ErrorCode = "TOKEN_INVALID"
	ErrCertInvalid    ErrorCode = "CERT_INVALID"
	ErrCertCNInvalid  ErrorCode = "CERT_CN_INVALID"
	ErrSiteMismatch   ErrorCode = "SITE_MISMATCH"
	ErrTenantMismatch ErrorCode = "TENANT_MISMATCH"

	// Authorization errors
	ErrForbidden        ErrorCode = "FORBIDDEN"
	ErrTenantRequired   ErrorCode = "TENANT_REQUIRED"
	ErrInsufficientRole ErrorCode = "INSUFFICIENT_ROLE"

	// Validation errors
	ErrValidation   ErrorCode = "VALIDATION_ERROR"
	ErrInvalidInput ErrorCode = "INVALID_INPUT"
	ErrMissingField ErrorCode = "MISSING_FIELD"

	// Resource errors
	ErrNotFound      ErrorCode = "NOT_FOUND"
	ErrAlreadyExists ErrorCode = "ALREADY_EXISTS"
	ErrConflict      ErrorCode = "CONFLICT"

	// Database errors
	ErrDatabaseError    ErrorCode = "DATABASE_ERROR"
	ErrConnectionFailed ErrorCode = "CONNECTION_FAILED"

	// External service errors
	ErrKeycloakError ErrorCode = "KEYCLOAK_ERROR"
	ErrKOSError      ErrorCode = "KOS_ERROR"

	// Internal errors
	ErrInternal ErrorCode = "INTERNAL_ERROR"
)

// APIError represents a structured API error
type APIError struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	Details    any       `json:"details,omitempty"`
	HTTPStatus int       `json:"-"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// New creates a new APIError
func New(code ErrorCode, message string, httpStatus int) *APIError {
	return &APIError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
	}
}

// WithDetails adds details to an error
func (e *APIError) WithDetails(details any) *APIError {
	e.Details = details
	return e
}

// Common error constructors
func Unauthorized(message string) *APIError {
	return New(ErrUnauthorized, message, http.StatusUnauthorized)
}

func Forbidden(message string) *APIError {
	return New(ErrForbidden, message, http.StatusForbidden)
}

func NotFound(resource string) *APIError {
	return New(ErrNotFound, fmt.Sprintf("%s not found", resource), http.StatusNotFound)
}

func AlreadyExists(resource string) *APIError {
	return New(ErrAlreadyExists, fmt.Sprintf("%s already exists", resource), http.StatusConflict)
}

func Validation(message string) *APIError {
	return New(ErrValidation, message, http.StatusBadRequest)
}

func InvalidInput(message string) *APIError {
	return New(ErrInvalidInput, message, http.StatusBadRequest)
}

func Internal(message string) *APIError {
	return New(ErrInternal, message, http.StatusInternalServerError)
}

func DatabaseError(err error) *APIError {
	return New(ErrDatabaseError, "database operation failed", http.StatusInternalServerError).WithDetails(err.Error())
}

func KeycloakError(err error) *APIError {
	return New(ErrKeycloakError, "keycloak operation failed", http.StatusInternalServerError).WithDetails(err.Error())
}

// ErrorResponse is the standard API error response format
type ErrorResponse struct {
	Success bool      `json:"success"`
	Error   *APIError `json:"error"`
}

// NewErrorResponse creates a new error response
func NewErrorResponse(err *APIError) *ErrorResponse {
	return &ErrorResponse{
		Success: false,
		Error:   err,
	}
}
