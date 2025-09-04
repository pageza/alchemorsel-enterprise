// Package memory provides in-memory implementations for conversation persistence
package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
)

// ConversationRepository implements in-memory conversation persistence
type ConversationRepository struct {
	conversations map[string]*conversation.Conversation
	mu            sync.RWMutex
}

// MessageRepository implements in-memory message persistence
type MessageRepository struct {
	messages map[string]*conversation.Message
	mu       sync.RWMutex
}

// ContextRepository implements in-memory context persistence
type ContextRepository struct {
	contexts map[string]*conversation.ConversationContext // key: conversationID:contextType
	mu       sync.RWMutex
}

// NewConversationRepository creates a new in-memory conversation repository
func NewConversationRepository() conversation.ConversationRepository {
	return &ConversationRepository{
		conversations: make(map[string]*conversation.Conversation),
	}
}

// NewMessageRepository creates a new in-memory message repository
func NewMessageRepository() conversation.MessageRepository {
	return &MessageRepository{
		messages: make(map[string]*conversation.Message),
	}
}

// NewContextRepository creates a new in-memory context repository
func NewContextRepository() conversation.ContextRepository {
	return &ContextRepository{
		contexts: make(map[string]*conversation.ConversationContext),
	}
}

// Conversation Repository Implementation

// CreateConversation creates a new conversation
func (r *ConversationRepository) CreateConversation(ctx context.Context, conv *conversation.Conversation) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.conversations[conv.ID]; exists {
		return fmt.Errorf("conversation with ID %s already exists", conv.ID)
	}

	// Create a copy to avoid reference issues
	convCopy := *conv
	r.conversations[conv.ID] = &convCopy

	return nil
}

// GetConversation retrieves a conversation by ID
func (r *ConversationRepository) GetConversation(ctx context.Context, id string) (*conversation.Conversation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	conv, exists := r.conversations[id]
	if !exists {
		return nil, fmt.Errorf("conversation with ID %s not found", id)
	}

	// Return a copy to avoid reference issues
	convCopy := *conv
	return &convCopy, nil
}

// GetUserConversations retrieves all conversations for a user
func (r *ConversationRepository) GetUserConversations(ctx context.Context, userID string, limit, offset int) ([]*conversation.Conversation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var userConversations []*conversation.Conversation
	for _, conv := range r.conversations {
		if conv.UserID == userID && conv.Status != conversation.StatusDeleted {
			convCopy := *conv
			userConversations = append(userConversations, &convCopy)
		}
	}

	// Sort by UpdatedAt descending (newest first)
	sort.Slice(userConversations, func(i, j int) bool {
		return userConversations[i].UpdatedAt.After(userConversations[j].UpdatedAt)
	})

	// Apply pagination
	start := offset
	if start > len(userConversations) {
		return []*conversation.Conversation{}, nil
	}

	end := start + limit
	if end > len(userConversations) {
		end = len(userConversations)
	}

	return userConversations[start:end], nil
}

// UpdateConversation updates an existing conversation
func (r *ConversationRepository) UpdateConversation(ctx context.Context, conv *conversation.Conversation) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.conversations[conv.ID]; !exists {
		return fmt.Errorf("conversation with ID %s not found", conv.ID)
	}

	conv.UpdatedAt = time.Now()
	convCopy := *conv
	r.conversations[conv.ID] = &convCopy

	return nil
}

// DeleteConversation deletes a conversation (soft delete by marking as deleted)
func (r *ConversationRepository) DeleteConversation(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	conv, exists := r.conversations[id]
	if !exists {
		return fmt.Errorf("conversation with ID %s not found", id)
	}

	conv.Status = conversation.StatusDeleted
	conv.UpdatedAt = time.Now()

	return nil
}

// Message Repository Implementation

// CreateMessage creates a new message
func (r *MessageRepository) CreateMessage(ctx context.Context, msg *conversation.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.messages[msg.ID]; exists {
		return fmt.Errorf("message with ID %s already exists", msg.ID)
	}

	// Create a copy to avoid reference issues
	msgCopy := *msg
	r.messages[msg.ID] = &msgCopy

	return nil
}

// GetMessage retrieves a message by ID
func (r *MessageRepository) GetMessage(ctx context.Context, id string) (*conversation.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	msg, exists := r.messages[id]
	if !exists {
		return nil, fmt.Errorf("message with ID %s not found", id)
	}

	// Return a copy to avoid reference issues
	msgCopy := *msg
	return &msgCopy, nil
}

// GetConversationMessages retrieves all messages for a conversation
func (r *MessageRepository) GetConversationMessages(ctx context.Context, conversationID string, limit, offset int) ([]*conversation.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var conversationMessages []*conversation.Message
	for _, msg := range r.messages {
		if msg.ConversationID == conversationID {
			msgCopy := *msg
			conversationMessages = append(conversationMessages, &msgCopy)
		}
	}

	// Sort by CreatedAt ascending (oldest first - conversation order)
	sort.Slice(conversationMessages, func(i, j int) bool {
		return conversationMessages[i].CreatedAt.Before(conversationMessages[j].CreatedAt)
	})

	// Apply pagination
	start := offset
	if start > len(conversationMessages) {
		return []*conversation.Message{}, nil
	}

	end := start + limit
	if end > len(conversationMessages) {
		end = len(conversationMessages)
	}

	return conversationMessages[start:end], nil
}

// UpdateMessage updates an existing message
func (r *MessageRepository) UpdateMessage(ctx context.Context, msg *conversation.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.messages[msg.ID]; !exists {
		return fmt.Errorf("message with ID %s not found", msg.ID)
	}

	msgCopy := *msg
	r.messages[msg.ID] = &msgCopy

	return nil
}

// DeleteMessage deletes a message
func (r *MessageRepository) DeleteMessage(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.messages[id]; !exists {
		return fmt.Errorf("message with ID %s not found", id)
	}

	delete(r.messages, id)
	return nil
}

// Context Repository Implementation

// SetContext sets context data for a conversation
func (r *ContextRepository) SetContext(ctx context.Context, context *conversation.ConversationContext) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%s", context.ConversationID, context.ContextType)
	contextCopy := *context
	r.contexts[key] = &contextCopy

	return nil
}

// GetContext retrieves context data for a conversation
func (r *ContextRepository) GetContext(ctx context.Context, conversationID, contextType string) (*conversation.ConversationContext, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", conversationID, contextType)
	context, exists := r.contexts[key]
	if !exists {
		return nil, fmt.Errorf("context %s for conversation %s not found", contextType, conversationID)
	}

	// Return a copy to avoid reference issues
	contextCopy := *context
	return &contextCopy, nil
}

// GetAllContext retrieves all context data for a conversation
func (r *ContextRepository) GetAllContext(ctx context.Context, conversationID string) ([]*conversation.ConversationContext, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var contexts []*conversation.ConversationContext
	for _, context := range r.contexts {
		if context.ConversationID == conversationID {
			contextCopy := *context
			contexts = append(contexts, &contextCopy)
		}
	}

	// Sort by CreatedAt ascending
	sort.Slice(contexts, func(i, j int) bool {
		return contexts[i].CreatedAt.Before(contexts[j].CreatedAt)
	})

	return contexts, nil
}

// DeleteContext deletes context data for a conversation
func (r *ContextRepository) DeleteContext(ctx context.Context, conversationID, contextType string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%s", conversationID, contextType)
	if _, exists := r.contexts[key]; !exists {
		return fmt.Errorf("context %s for conversation %s not found", contextType, conversationID)
	}

	delete(r.contexts, key)
	return nil
}
