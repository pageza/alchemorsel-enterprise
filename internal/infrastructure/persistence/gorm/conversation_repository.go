package gorm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"gorm.io/gorm"
)

// ConversationModel represents the GORM model for conversations
type ConversationModel struct {
	ID        string                          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    string                          `gorm:"type:uuid;not null"`
	Title     string                          `gorm:"type:varchar(255)"`
	Intent    conversation.ConversationIntent `gorm:"type:conversation_intent"`
	Status    conversation.ConversationStatus `gorm:"type:conversation_status;default:'active'"`
	Metadata  string                          `gorm:"type:jsonb"`
	CreatedAt time.Time                       `gorm:"not null;default:now()"`
	UpdatedAt time.Time                       `gorm:"not null;default:now()"`
	DeletedAt *time.Time                      `gorm:"index"`
}

// TableName returns the table name for ConversationModel
func (ConversationModel) TableName() string {
	return "conversations"
}

// MessageModel represents the GORM model for messages
type MessageModel struct {
	ID               string                   `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ConversationID   string                   `gorm:"type:uuid;not null"`
	Role             conversation.MessageRole `gorm:"type:message_role;not null"`
	Content          string                   `gorm:"type:text;not null"`
	Metadata         string                   `gorm:"type:jsonb;default:'{}'"`
	TokensUsed       int                      `gorm:"default:0"`
	ProcessingTimeMs int                      `gorm:"default:0"`
	ModelUsed        string                   `gorm:"type:varchar(100)"`
	CreatedAt        time.Time                `gorm:"not null;default:now()"`
}

// TableName returns the table name for MessageModel
func (MessageModel) TableName() string {
	return "messages"
}

// ConversationContextModel represents the GORM model for conversation context
type ConversationContextModel struct {
	ConversationID string    `gorm:"type:uuid;not null;primaryKey"`
	ContextType    string    `gorm:"type:varchar(50);not null;primaryKey"`
	ContextData    string    `gorm:"type:jsonb;not null"`
	CreatedAt      time.Time `gorm:"not null;default:now()"`
	UpdatedAt      time.Time `gorm:"not null;default:now()"`
}

// TableName returns the table name for ConversationContextModel
func (ConversationContextModel) TableName() string {
	return "conversation_context"
}

// ConversationRepository implements the conversation repository interface
type ConversationRepository struct {
	db *gorm.DB
}

// NewConversationRepository creates a new conversation repository
func NewConversationRepository(db *gorm.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

// CreateConversation creates a new conversation
func (r *ConversationRepository) CreateConversation(ctx context.Context, conv *conversation.Conversation) error {
	metadataJSON, err := json.Marshal(conv.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	model := &ConversationModel{
		ID:        conv.ID,
		UserID:    conv.UserID,
		Title:     conv.Title,
		Intent:    conv.Intent,
		Status:    conv.Status,
		Metadata:  string(metadataJSON),
		CreatedAt: conv.CreatedAt,
		UpdatedAt: conv.UpdatedAt,
	}

	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return fmt.Errorf("failed to create conversation: %w", err)
	}

	return nil
}

// GetConversation retrieves a conversation by ID
func (r *ConversationRepository) GetConversation(ctx context.Context, id string) (*conversation.Conversation, error) {
	var model ConversationModel

	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&model).Error; err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	return r.modelToConversation(&model)
}

// GetUserConversations retrieves all conversations for a user
func (r *ConversationRepository) GetUserConversations(ctx context.Context, userID string, limit, offset int) ([]*conversation.Conversation, error) {
	var models []ConversationModel

	query := r.db.WithContext(ctx).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Order("updated_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&models).Error; err != nil {
		return nil, fmt.Errorf("failed to get user conversations: %w", err)
	}

	conversations := make([]*conversation.Conversation, len(models))
	for i, model := range models {
		conv, err := r.modelToConversation(&model)
		if err != nil {
			return nil, fmt.Errorf("failed to convert conversation %s: %w", model.ID, err)
		}
		conversations[i] = conv
	}

	return conversations, nil
}

// UpdateConversation updates a conversation
func (r *ConversationRepository) UpdateConversation(ctx context.Context, conv *conversation.Conversation) error {
	metadataJSON, err := json.Marshal(conv.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	updates := map[string]interface{}{
		"title":      conv.Title,
		"intent":     conv.Intent,
		"status":     conv.Status,
		"metadata":   string(metadataJSON),
		"updated_at": conv.UpdatedAt,
	}

	if err := r.db.WithContext(ctx).Model(&ConversationModel{}).Where("id = ?", conv.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update conversation: %w", err)
	}

	return nil
}

// DeleteConversation soft deletes a conversation
func (r *ConversationRepository) DeleteConversation(ctx context.Context, id string) error {
	now := time.Now()
	if err := r.db.WithContext(ctx).Model(&ConversationModel{}).Where("id = ?", id).Update("deleted_at", now).Error; err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	return nil
}

// modelToConversation converts a ConversationModel to a Conversation
func (r *ConversationRepository) modelToConversation(model *ConversationModel) (*conversation.Conversation, error) {
	var metadata map[string]interface{}
	if model.Metadata != "" {
		if err := json.Unmarshal([]byte(model.Metadata), &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	return &conversation.Conversation{
		ID:        model.ID,
		UserID:    model.UserID,
		Title:     model.Title,
		Intent:    model.Intent,
		Status:    model.Status,
		Metadata:  metadata,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}, nil
}

// MessageRepository implements the message repository interface
type MessageRepository struct {
	db *gorm.DB
}

// NewMessageRepository creates a new message repository
func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// CreateMessage creates a new message
func (r *MessageRepository) CreateMessage(ctx context.Context, msg *conversation.Message) error {
	metadataJSON, err := json.Marshal(msg.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	model := &MessageModel{
		ID:               msg.ID,
		ConversationID:   msg.ConversationID,
		Role:             msg.Role,
		Content:          msg.Content,
		Metadata:         string(metadataJSON),
		TokensUsed:       msg.TokensUsed,
		ProcessingTimeMs: msg.ProcessingTimeMs,
		ModelUsed:        msg.ModelUsed,
		CreatedAt:        msg.CreatedAt,
	}

	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	return nil
}

// GetMessage retrieves a message by ID
func (r *MessageRepository) GetMessage(ctx context.Context, id string) (*conversation.Message, error) {
	var model MessageModel

	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	return r.modelToMessage(&model)
}

// GetConversationMessages retrieves all messages for a conversation
func (r *MessageRepository) GetConversationMessages(ctx context.Context, conversationID string, limit, offset int) ([]*conversation.Message, error) {
	var models []MessageModel

	query := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("created_at ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&models).Error; err != nil {
		return nil, fmt.Errorf("failed to get conversation messages: %w", err)
	}

	messages := make([]*conversation.Message, len(models))
	for i, model := range models {
		msg, err := r.modelToMessage(&model)
		if err != nil {
			return nil, fmt.Errorf("failed to convert message %s: %w", model.ID, err)
		}
		messages[i] = msg
	}

	return messages, nil
}

// UpdateMessage updates a message
func (r *MessageRepository) UpdateMessage(ctx context.Context, msg *conversation.Message) error {
	metadataJSON, err := json.Marshal(msg.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	updates := map[string]interface{}{
		"content":            msg.Content,
		"metadata":           string(metadataJSON),
		"tokens_used":        msg.TokensUsed,
		"processing_time_ms": msg.ProcessingTimeMs,
		"model_used":         msg.ModelUsed,
	}

	if err := r.db.WithContext(ctx).Model(&MessageModel{}).Where("id = ?", msg.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}

	return nil
}

// DeleteMessage deletes a message
func (r *MessageRepository) DeleteMessage(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&MessageModel{}).Error; err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	return nil
}

// modelToMessage converts a MessageModel to a Message
func (r *MessageRepository) modelToMessage(model *MessageModel) (*conversation.Message, error) {
	var metadata map[string]interface{}
	if model.Metadata != "" {
		if err := json.Unmarshal([]byte(model.Metadata), &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	return &conversation.Message{
		ID:               model.ID,
		ConversationID:   model.ConversationID,
		Role:             model.Role,
		Content:          model.Content,
		Metadata:         metadata,
		TokensUsed:       model.TokensUsed,
		ProcessingTimeMs: model.ProcessingTimeMs,
		ModelUsed:        model.ModelUsed,
		CreatedAt:        model.CreatedAt,
	}, nil
}

// ContextRepository implements the context repository interface
type ContextRepository struct {
	db *gorm.DB
}

// NewContextRepository creates a new context repository
func NewContextRepository(db *gorm.DB) *ContextRepository {
	return &ContextRepository{db: db}
}

// SetContext sets context data for a conversation
func (r *ContextRepository) SetContext(ctx context.Context, convContext *conversation.ConversationContext) error {
	contextDataJSON, err := json.Marshal(convContext.ContextData)
	if err != nil {
		return fmt.Errorf("failed to marshal context data: %w", err)
	}

	model := &ConversationContextModel{
		ConversationID: convContext.ConversationID,
		ContextType:    convContext.ContextType,
		ContextData:    string(contextDataJSON),
		CreatedAt:      convContext.CreatedAt,
		UpdatedAt:      convContext.UpdatedAt,
	}

	// Use UPSERT (ON CONFLICT UPDATE)
	if err := r.db.WithContext(ctx).Save(model).Error; err != nil {
		return fmt.Errorf("failed to set context: %w", err)
	}

	return nil
}

// GetContext retrieves context data for a conversation
func (r *ContextRepository) GetContext(ctx context.Context, conversationID, contextType string) (*conversation.ConversationContext, error) {
	var model ConversationContextModel

	if err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND context_type = ?", conversationID, contextType).
		First(&model).Error; err != nil {
		return nil, fmt.Errorf("failed to get context: %w", err)
	}

	return r.modelToContext(&model)
}

// GetAllContext retrieves all context data for a conversation
func (r *ContextRepository) GetAllContext(ctx context.Context, conversationID string) ([]*conversation.ConversationContext, error) {
	var models []ConversationContextModel

	if err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("context_type ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("failed to get all context: %w", err)
	}

	contexts := make([]*conversation.ConversationContext, len(models))
	for i, model := range models {
		convContext, err := r.modelToContext(&model)
		if err != nil {
			return nil, fmt.Errorf("failed to convert context %s/%s: %w", model.ConversationID, model.ContextType, err)
		}
		contexts[i] = convContext
	}

	return contexts, nil
}

// DeleteContext deletes context data for a conversation
func (r *ContextRepository) DeleteContext(ctx context.Context, conversationID, contextType string) error {
	if err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND context_type = ?", conversationID, contextType).
		Delete(&ConversationContextModel{}).Error; err != nil {
		return fmt.Errorf("failed to delete context: %w", err)
	}

	return nil
}

// modelToContext converts a ConversationContextModel to a ConversationContext
func (r *ContextRepository) modelToContext(model *ConversationContextModel) (*conversation.ConversationContext, error) {
	var contextData map[string]interface{}
	if model.ContextData != "" {
		if err := json.Unmarshal([]byte(model.ContextData), &contextData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal context data: %w", err)
		}
	}

	if contextData == nil {
		contextData = make(map[string]interface{})
	}

	return &conversation.ConversationContext{
		ConversationID: model.ConversationID,
		ContextType:    model.ContextType,
		ContextData:    contextData,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}, nil
}
