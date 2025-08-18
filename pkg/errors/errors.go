// Package errors provides structured error handling for the application
// Following enterprise patterns for error management and observability
package errors

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// ErrorCode represents an error code
type ErrorCode string

// Common error codes following RESTful API conventions
const (
	// Client errors (4xx)
	CodeBadRequest          ErrorCode = "BAD_REQUEST"
	CodeUnauthorized        ErrorCode = "UNAUTHORIZED"
	CodeForbidden           ErrorCode = "FORBIDDEN"
	CodeNotFound            ErrorCode = "NOT_FOUND"
	CodeConflict            ErrorCode = "CONFLICT"
	CodeValidationFailed    ErrorCode = "VALIDATION_FAILED"
	CodeTooManyRequests     ErrorCode = "TOO_MANY_REQUESTS"
	
	// Server errors (5xx)
	CodeInternal            ErrorCode = "INTERNAL_ERROR"
	CodeServiceUnavailable  ErrorCode = "SERVICE_UNAVAILABLE"
	CodeDatabaseError       ErrorCode = "DATABASE_ERROR"
	CodeExternalServiceError ErrorCode = "EXTERNAL_SERVICE_ERROR"
	
	// Business logic errors
	CodeRecipeNotFound      ErrorCode = "RECIPE_NOT_FOUND"
	CodeUserNotFound        ErrorCode = "USER_NOT_FOUND"
	CodeInvalidCredentials  ErrorCode = "INVALID_CREDENTIALS"
	CodeEmailAlreadyExists  ErrorCode = "EMAIL_ALREADY_EXISTS"
	CodeUsernameAlreadyExists ErrorCode = "USERNAME_ALREADY_EXISTS"
	CodeInsufficientPermissions ErrorCode = "INSUFFICIENT_PERMISSIONS"
	CodeResourceLocked      ErrorCode = "RESOURCE_LOCKED"
	CodeQuotaExceeded       ErrorCode = "QUOTA_EXCEEDED"
)

// AppError represents an application error with structured information
type AppError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Cause      error                  `json:"-"`
	StackTrace string                 `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// StatusCode returns the appropriate HTTP status code
func (e *AppError) StatusCode() int {
	switch e.Code {
	case CodeBadRequest, CodeValidationFailed:
		return http.StatusBadRequest
	case CodeUnauthorized, CodeInvalidCredentials:
		return http.StatusUnauthorized
	case CodeForbidden, CodeInsufficientPermissions:
		return http.StatusForbidden
	case CodeNotFound, CodeRecipeNotFound, CodeUserNotFound:
		return http.StatusNotFound
	case CodeConflict, CodeEmailAlreadyExists, CodeUsernameAlreadyExists, CodeResourceLocked:
		return http.StatusConflict
	case CodeTooManyRequests, CodeQuotaExceeded:
		return http.StatusTooManyRequests
	case CodeServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// WithMetadata adds metadata to the error
func (e *AppError) WithMetadata(key string, value interface{}) *AppError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// WithCause adds a cause error
func (e *AppError) WithCause(cause error) *AppError {
	e.Cause = cause
	return e
}

// NewAppError creates a new application error
func NewAppError(code ErrorCode, message, details string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		Details:    details,
		StackTrace: getStackTrace(),
	}
}

// Predefined error constructors for common scenarios

// NewBadRequestError creates a bad request error
func NewBadRequestError(message string) *AppError {
	return NewAppError(CodeBadRequest, message, "")
}

// NewValidationError creates a validation error
func NewValidationError(details string) *AppError {
	return NewAppError(CodeValidationFailed, "Validation failed", details)
}

// NewUnauthorizedError creates an unauthorized error
func NewUnauthorizedError(message string) *AppError {
	if message == "" {
		message = "Authentication required"
	}
	return NewAppError(CodeUnauthorized, message, "")
}

// NewForbiddenError creates a forbidden error
func NewForbiddenError(message string) *AppError {
	if message == "" {
		message = "Access forbidden"
	}
	return NewAppError(CodeForbidden, message, "")
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource string) *AppError {
	message := "Resource not found"
	if resource != "" {
		message = fmt.Sprintf("%s not found", strings.Title(resource))
	}
	return NewAppError(CodeNotFound, message, "")
}

// NewConflictError creates a conflict error
func NewConflictError(message string) *AppError {
	return NewAppError(CodeConflict, message, "")
}

// NewInternalError creates an internal server error
func NewInternalError(message string) *AppError {
	if message == "" {
		message = "An unexpected error occurred"
	}
	return NewAppError(CodeInternal, message, "")
}

// NewDatabaseError creates a database error
func NewDatabaseError(operation string, cause error) *AppError {
	return NewAppError(
		CodeDatabaseError,
		"Database operation failed",
		fmt.Sprintf("Failed to %s", operation),
	).WithCause(cause)
}

// NewExternalServiceError creates an external service error
func NewExternalServiceError(service string, cause error) *AppError {
	return NewAppError(
		CodeExternalServiceError,
		"External service error",
		fmt.Sprintf("Failed to communicate with %s", service),
	).WithCause(cause)
}

// Business domain specific errors

// NewRecipeNotFoundError creates a recipe not found error
func NewRecipeNotFoundError(recipeID string) *AppError {
	return NewAppError(
		CodeRecipeNotFound,
		"Recipe not found",
		fmt.Sprintf("Recipe with ID %s does not exist", recipeID),
	).WithMetadata("recipe_id", recipeID)
}

// NewUserNotFoundError creates a user not found error
func NewUserNotFoundError(userID string) *AppError {
	return NewAppError(
		CodeUserNotFound,
		"User not found",
		fmt.Sprintf("User with ID %s does not exist", userID),
	).WithMetadata("user_id", userID)
}

// NewEmailAlreadyExistsError creates an email already exists error
func NewEmailAlreadyExistsError(email string) *AppError {
	return NewAppError(
		CodeEmailAlreadyExists,
		"Email already exists",
		"An account with this email address already exists",
	).WithMetadata("email", email)
}

// NewUsernameAlreadyExistsError creates a username already exists error
func NewUsernameAlreadyExistsError(username string) *AppError {
	return NewAppError(
		CodeUsernameAlreadyExists,
		"Username already exists",
		"This username is already taken",
	).WithMetadata("username", username)
}

// NewInvalidCredentialsError creates an invalid credentials error
func NewInvalidCredentialsError() *AppError {
	return NewAppError(
		CodeInvalidCredentials,
		"Invalid credentials",
		"The provided email/username or password is incorrect",
	)
}

// NewInsufficientPermissionsError creates an insufficient permissions error
func NewInsufficientPermissionsError(action string) *AppError {
	return NewAppError(
		CodeInsufficientPermissions,
		"Insufficient permissions",
		fmt.Sprintf("You don't have permission to %s", action),
	).WithMetadata("action", action)
}

// NewResourceLockedError creates a resource locked error
func NewResourceLockedError(resource string) *AppError {
	return NewAppError(
		CodeResourceLocked,
		"Resource locked",
		fmt.Sprintf("The %s is currently locked by another operation", resource),
	).WithMetadata("resource", resource)
}

// NewQuotaExceededError creates a quota exceeded error
func NewQuotaExceededError(quotaType string, limit int) *AppError {
	return NewAppError(
		CodeQuotaExceeded,
		"Quota exceeded",
		fmt.Sprintf("You have exceeded your %s quota of %d", quotaType, limit),
	).WithMetadata("quota_type", quotaType).WithMetadata("limit", limit)
}

// Utility functions

// Wrap wraps an error as an internal error if it's not already an AppError
func Wrap(err error, message string) *AppError {
	if err == nil {
		return nil
	}
	
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	
	return NewInternalError(message).WithCause(err)
}

// Is checks if an error is of a specific error code
func Is(err error, code ErrorCode) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == code
	}
	return false
}

// GetCode extracts the error code from an error
func GetCode(err error) ErrorCode {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code
	}
	return CodeInternal
}

// getStackTrace captures the current stack trace
func getStackTrace() string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	
	var builder strings.Builder
	for {
		frame, more := frames.Next()
		if !strings.Contains(frame.File, "pkg/errors") {
			builder.WriteString(fmt.Sprintf("%s:%d %s\n", frame.File, frame.Line, frame.Function))
		}
		if !more {
			break
		}
	}
	
	return builder.String()
}

// ValidationError represents a field validation error
type ValidationError struct {
	Field   string      `json:"field"`
	Value   interface{} `json:"value"`
	Tag     string      `json:"tag"`
	Message string      `json:"message"`
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

// Error implements the error interface
func (v ValidationErrors) Error() string {
	if len(v) == 0 {
		return "validation failed"
	}
	
	if len(v) == 1 {
		return v[0].Message
	}
	
	var messages []string
	for _, err := range v {
		messages = append(messages, err.Message)
	}
	
	return strings.Join(messages, "; ")
}

// NewValidationErrors creates validation errors from validator errors
func NewValidationErrors(errors []ValidationError) *AppError {
	validationErrs := ValidationErrors(errors)
	
	return NewAppError(
		CodeValidationFailed,
		"Validation failed",
		validationErrs.Error(),
	).WithMetadata("validation_errors", validationErrs)
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error ErrorDetails `json:"error"`
}

// ErrorDetails represents the error details in API responses
type ErrorDetails struct {
	Code      ErrorCode              `json:"code"`
	Message   string                 `json:"message"`
	Details   string                 `json:"details,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	Timestamp string                 `json:"timestamp"`
}

// ToErrorResponse converts an AppError to an API error response
func ToErrorResponse(err *AppError, requestID string) ErrorResponse {
	return ErrorResponse{
		Error: ErrorDetails{
			Code:      err.Code,
			Message:   err.Message,
			Details:   err.Details,
			Metadata:  err.Metadata,
			RequestID: requestID,
			Timestamp: fmt.Sprintf("%d", time.Now().Unix()),
		},
	}
}