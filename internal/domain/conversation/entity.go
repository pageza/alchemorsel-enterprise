// Package conversation contains the core domain logic for chat conversation management.
// This follows Domain-Driven Design principles with rich domain models.
package conversation

import (
	"fmt"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/domain/shared"
)

// ConversationStatus represents the status of a conversation
type ConversationStatus string

const (
	StatusActive   ConversationStatus = "active"
	StatusArchived ConversationStatus = "archived"
	StatusDeleted  ConversationStatus = "deleted"
)

// ConversationIntent represents the purpose of a conversation
type ConversationIntent string

const (
	IntentRecipeCreation        ConversationIntent = "recipe_creation"
	IntentCookingHelp          ConversationIntent = "cooking_help"
	IntentIngredientSubstitution ConversationIntent = "ingredient_substitution"
	IntentMealPlanning         ConversationIntent = "meal_planning"
	IntentGeneralQuestion      ConversationIntent = "general_question"
)

// MessageRole represents the role of a message sender
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
)

// Conversation represents a chat conversation between a user and AI assistant
type Conversation struct {
	// Aggregate root identifier
	id      string
	userID  string
	
	// Conversation metadata
	title      string
	intent     *ConversationIntent
	status     ConversationStatus
	metadata   map[string]interface{}
	
	// Messages in this conversation
	messages   []Message
	
	// Timestamps
	createdAt  time.Time
	updatedAt  time.Time
	archivedAt *time.Time
	deletedAt  *time.Time
	
	// Domain events
	events []shared.DomainEvent
}

// Message represents a single message within a conversation
type Message struct {
	id             string
	conversationID string
	role           MessageRole
	content        string
	metadata       map[string]interface{}
	
	// AI-specific fields
	tokensUsed       int
	processingTimeMs int
	modelUsed        string
	
	createdAt time.Time
}

// ConversationContext stores workflow state and extracted data
type ConversationContext struct {
	conversationID string
	contextType    string
	contextData    map[string]interface{}
	createdAt      time.Time
	updatedAt      time.Time
}

// NewConversation creates a new conversation
func NewConversation(userID string, intent *ConversationIntent) *Conversation {
	now := time.Now()
	
	conversation := &Conversation{
		id:        generateID(),
		userID:    userID,
		intent:    intent,
		status:    StatusActive,
		metadata:  make(map[string]interface{}),
		messages:  make([]Message, 0),
		createdAt: now,
		updatedAt: now,
		events:    make([]shared.DomainEvent, 0),
	}
	
	// Generate default title based on intent
	conversation.generateDefaultTitle()
	
	// Record domain event
	conversation.recordEvent(shared.ConversationCreated{
		ConversationID: conversation.id,
		UserID:        userID,
		Intent:        intent,
		CreatedAt:     now,
	})
	
	return conversation
}

// ID returns the conversation ID
func (c *Conversation) ID() string {
	return c.id
}

// UserID returns the user ID
func (c *Conversation) UserID() string {
	return c.userID
}

// Title returns the conversation title
func (c *Conversation) Title() string {
	return c.title
}

// Status returns the conversation status
func (c *Conversation) Status() ConversationStatus {
	return c.status
}

// Intent returns the conversation intent
func (c *Conversation) Intent() *ConversationIntent {
	return c.intent
}

// Messages returns all messages in the conversation
func (c *Conversation) Messages() []Message {
	return c.messages
}

// CreatedAt returns the creation timestamp
func (c *Conversation) CreatedAt() time.Time {
	return c.createdAt
}

// UpdatedAt returns the last update timestamp
func (c *Conversation) UpdatedAt() time.Time {
	return c.updatedAt
}

// AddMessage adds a new message to the conversation
func (c *Conversation) AddMessage(role MessageRole, content string, metadata map[string]interface{}) (*Message, error) {
	if strings.TrimSpace(content) == "" {
		return nil, NewValidationError("message content cannot be empty")
	}
	
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	
	message := Message{
		id:             generateID(),
		conversationID: c.id,
		role:           role,
		content:        content,
		metadata:       metadata,
		createdAt:      time.Now(),
	}
	
	c.messages = append(c.messages, message)
	c.updatedAt = time.Now()
	
	// Update title from first user message if needed
	if role == RoleUser && c.shouldUpdateTitle() {
		c.generateTitleFromMessage(content)
	}
	
	// Record domain event
	c.recordEvent(shared.MessageAdded{
		ConversationID: c.id,
		MessageID:      message.id,
		Role:          string(role),
		Content:       content,
		CreatedAt:     message.createdAt,
	})
	
	return &message, nil
}

// UpdateTitle updates the conversation title
func (c *Conversation) UpdateTitle(title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return NewValidationError("title cannot be empty")
	}
	
	if len(title) > 255 {
		return NewValidationError("title cannot exceed 255 characters")
	}
	
	oldTitle := c.title
	c.title = title
	c.updatedAt = time.Now()
	
	// Record domain event
	c.recordEvent(shared.ConversationTitleUpdated{
		ConversationID: c.id,
		OldTitle:      oldTitle,
		NewTitle:      title,
		UpdatedAt:     c.updatedAt,
	})
	
	return nil
}

// Archive archives the conversation
func (c *Conversation) Archive() error {
	if c.status == StatusDeleted {
		return NewDomainError("cannot archive a deleted conversation")
	}
	
	if c.status == StatusArchived {
		return nil // Already archived
	}
	
	now := time.Now()
	c.status = StatusArchived
	c.archivedAt = &now
	c.updatedAt = now
	
	c.recordEvent(shared.ConversationArchived{
		ConversationID: c.id,
		ArchivedAt:     now,
	})
	
	return nil
}

// Delete soft-deletes the conversation
func (c *Conversation) Delete() error {
	if c.status == StatusDeleted {
		return nil // Already deleted
	}
	
	now := time.Now()
	c.status = StatusDeleted
	c.deletedAt = &now
	c.updatedAt = now
	
	c.recordEvent(shared.ConversationDeleted{
		ConversationID: c.id,
		DeletedAt:      now,
	})
	
	return nil
}

// IsDeleted returns true if the conversation is deleted
func (c *Conversation) IsDeleted() bool {
	return c.status == StatusDeleted
}

// DomainEvents returns all uncommitted domain events
func (c *Conversation) DomainEvents() []shared.DomainEvent {
	return c.events
}

// MarkEventsAsCommitted clears all domain events
func (c *Conversation) MarkEventsAsCommitted() {
	c.events = make([]shared.DomainEvent, 0)
}

// Private methods

func (c *Conversation) generateDefaultTitle() {
	if c.intent != nil {
		switch *c.intent {
		case IntentRecipeCreation:
			c.title = "New Recipe Creation"
		case IntentCookingHelp:
			c.title = "Cooking Help"
		case IntentIngredientSubstitution:
			c.title = "Ingredient Help"
		case IntentMealPlanning:
			c.title = "Meal Planning"
		default:
			c.title = "Chat with AI Chef"
		}
	} else {
		c.title = "Chat with AI Chef"
	}
}

func (c *Conversation) shouldUpdateTitle() bool {
	// Update title if it's still the default or if we only have one user message
	userMessageCount := 0
	for _, msg := range c.messages {
		if msg.role == RoleUser {
			userMessageCount++
		}
	}
	
	return userMessageCount == 1 && (c.title == "" || 
		c.title == "New Recipe Creation" || 
		c.title == "Cooking Help" || 
		c.title == "Ingredient Help" || 
		c.title == "Meal Planning" || 
		c.title == "Chat with AI Chef")
}

func (c *Conversation) generateTitleFromMessage(content string) {
	// Simple title generation logic - can be enhanced
	content = strings.TrimSpace(content)
	if len(content) > 50 {
		c.title = content[:47] + "..."
	} else {
		c.title = content
	}
	
	// Clean up title
	c.title = strings.Title(strings.ToLower(c.title))
}

func (c *Conversation) recordEvent(event shared.DomainEvent) {
	c.events = append(c.events, event)
}

// generateID generates a new UUID string
func generateID() string {
	// This is a simple implementation - in production you'd use proper UUID generation
	// For now, we'll use a timestamp-based approach
	return fmt.Sprintf("%d", time.Now().UnixNano())
}