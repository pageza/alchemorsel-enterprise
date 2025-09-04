package conversation

import "fmt"

// ConversationError represents a domain-specific error in the conversation context
type ConversationError struct {
	Code    string
	Message string
	Cause   error
}

func (e ConversationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Domain error constructors

func NewValidationError(message string) error {
	return ConversationError{
		Code:    "VALIDATION_ERROR",
		Message: message,
	}
}

func NewDomainError(message string) error {
	return ConversationError{
		Code:    "DOMAIN_ERROR",
		Message: message,
	}
}

func NewNotFoundError(message string) error {
	return ConversationError{
		Code:    "NOT_FOUND",
		Message: message,
	}
}

func NewPermissionError(message string) error {
	return ConversationError{
		Code:    "PERMISSION_DENIED",
		Message: message,
	}
}

// Common error instances

var (
	ErrConversationNotFound = NewNotFoundError("conversation not found")
	ErrMessageNotFound      = NewNotFoundError("message not found")
	ErrInvalidUserID        = NewValidationError("invalid user ID")
	ErrEmptyContent         = NewValidationError("message content cannot be empty")
	ErrTitleTooLong         = NewValidationError("title cannot exceed 255 characters")
)
