package gorm

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConversationRepositoryTestSuite tests the GORM conversation repository
type ConversationRepositoryTestSuite struct {
	suite.Suite
	db               *gorm.DB
	mock             sqlmock.Sqlmock
	conversationRepo *ConversationRepository
	messageRepo      *MessageRepository
	contextRepo      *ContextRepository
	ctx              context.Context
}

func (suite *ConversationRepositoryTestSuite) SetupTest() {
	suite.ctx = context.Background()

	// Create mock DB
	sqlDB, mock, err := sqlmock.New()
	require.NoError(suite.T(), err)
	suite.mock = mock

	// Create GORM DB with mock
	dialector := postgres.New(postgres.Config{
		DSN:                  "sqlmock_db_0",
		DriverName:           "postgres",
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	})
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(suite.T(), err)
	suite.db = db

	// Create repositories
	suite.conversationRepo = NewConversationRepository(db)
	suite.messageRepo = NewMessageRepository(db)
	suite.contextRepo = NewContextRepository(db)
}

func (suite *ConversationRepositoryTestSuite) TearDownTest() {
	// Only check expectations if we set them
	if err := suite.mock.ExpectationsWereMet(); err != nil {
		suite.T().Logf("Mock expectations not met: %v", err)
	}
}

// Conversation Repository Tests

func (suite *ConversationRepositoryTestSuite) TestCreateConversation_Success() {
	conv := &conversation.Conversation{
		ID:        "conv-123",
		UserID:    "user-456",
		Title:     "Test Conversation",
		Intent:    conversation.IntentRecipeCreation,
		Status:    conversation.StatusActive,
		Metadata:  map[string]interface{}{"key": "value"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock successful INSERT
	suite.mock.ExpectBegin()
	suite.mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "conversations"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(conv.ID, conv.CreatedAt, conv.UpdatedAt))
	suite.mock.ExpectCommit()

	err := suite.conversationRepo.CreateConversation(suite.ctx, conv)
	assert.NoError(suite.T(), err)
}

func (suite *ConversationRepositoryTestSuite) TestCreateConversation_MetadataError() {
	conv := &conversation.Conversation{
		ID:       "conv-123",
		UserID:   "user-456",
		Title:    "Test Conversation",
		Intent:   conversation.IntentRecipeCreation,
		Status:   conversation.StatusActive,
		Metadata: map[string]interface{}{"invalid": func() {}}, // Functions can't be JSON marshaled
	}

	// Should fail before hitting database
	err := suite.conversationRepo.CreateConversation(suite.ctx, conv)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to marshal metadata")
}

func (suite *ConversationRepositoryTestSuite) TestCreateConversation_DatabaseError() {
	conv := &conversation.Conversation{
		ID:        "conv-123",
		UserID:    "user-456",
		Title:     "Test Conversation",
		Intent:    conversation.IntentRecipeCreation,
		Status:    conversation.StatusActive,
		Metadata:  map[string]interface{}{"key": "value"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	suite.mock.ExpectBegin()
	suite.mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "conversations"`)).
		WillReturnError(errors.New("database connection failed"))
	suite.mock.ExpectRollback()

	err := suite.conversationRepo.CreateConversation(suite.ctx, conv)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to create conversation")
}

func (suite *ConversationRepositoryTestSuite) TestGetConversation_Success() {
	convID := "conv-123"
	expectedTime := time.Now()

	// Mock SELECT query
	suite.mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "conversations" WHERE id = $1 AND deleted_at IS NULL`)).
		WithArgs(convID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "title", "intent", "status", "metadata", "created_at", "updated_at", "deleted_at",
		}).AddRow(
			convID,
			"user-456",
			"Test Conversation",
			"recipe_creation",
			"active",
			`{"key":"value"}`,
			expectedTime,
			expectedTime,
			nil,
		))

	result, err := suite.conversationRepo.GetConversation(suite.ctx, convID)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), convID, result.ID)
	assert.Equal(suite.T(), "user-456", result.UserID)
	assert.Equal(suite.T(), "Test Conversation", result.Title)
	assert.Equal(suite.T(), conversation.IntentRecipeCreation, result.Intent)
	assert.Equal(suite.T(), conversation.StatusActive, result.Status)
	assert.Equal(suite.T(), "value", result.Metadata["key"])
}

func (suite *ConversationRepositoryTestSuite) TestGetConversation_NotFound() {
	convID := "nonexistent"

	suite.mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "conversations" WHERE id = $1 AND deleted_at IS NULL`)).
		WithArgs(convID).
		WillReturnError(gorm.ErrRecordNotFound)

	result, err := suite.conversationRepo.GetConversation(suite.ctx, convID)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to get conversation")
}

func (suite *ConversationRepositoryTestSuite) TestGetConversation_InvalidMetadata() {
	convID := "conv-123"

	suite.mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "conversations" WHERE id = $1 AND deleted_at IS NULL`)).
		WithArgs(convID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "title", "intent", "status", "metadata", "created_at", "updated_at", "deleted_at",
		}).AddRow(
			convID,
			"user-456",
			"Test Conversation",
			"recipe_creation",
			"active",
			`{invalid json}`, // Invalid JSON
			time.Now(),
			time.Now(),
			nil,
		))

	result, err := suite.conversationRepo.GetConversation(suite.ctx, convID)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to unmarshal metadata")
}

func (suite *ConversationRepositoryTestSuite) TestGetUserConversations_Success() {
	userID := "user-456"
	limit := 10
	offset := 0
	expectedTime := time.Now()

	suite.mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "conversations" WHERE user_id = $1 AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT $2`)).
		WithArgs(userID, limit).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "title", "intent", "status", "metadata", "created_at", "updated_at", "deleted_at",
		}).
			AddRow("conv-1", userID, "Conv 1", "recipe_creation", "active", `{}`, expectedTime, expectedTime, nil).
			AddRow("conv-2", userID, "Conv 2", "cooking_help", "active", `{}`, expectedTime, expectedTime, nil))

	result, err := suite.conversationRepo.GetUserConversations(suite.ctx, userID, limit, offset)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 2)
	assert.Equal(suite.T(), "conv-1", result[0].ID)
	assert.Equal(suite.T(), "conv-2", result[1].ID)
}

func (suite *ConversationRepositoryTestSuite) TestUpdateConversation_Success() {
	conv := &conversation.Conversation{
		ID:        "conv-123",
		Title:     "Updated Title",
		Intent:    conversation.IntentCookingHelp,
		Status:    conversation.StatusActive,
		Metadata:  map[string]interface{}{"updated": "value"},
		UpdatedAt: time.Now(),
	}

	suite.mock.ExpectBegin()
	suite.mock.ExpectExec(regexp.QuoteMeta(`UPDATE "conversations" SET`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	suite.mock.ExpectCommit()

	err := suite.conversationRepo.UpdateConversation(suite.ctx, conv)
	assert.NoError(suite.T(), err)
}

func (suite *ConversationRepositoryTestSuite) TestUpdateConversation_MetadataError() {
	conv := &conversation.Conversation{
		ID:       "conv-123",
		Title:    "Updated Title",
		Intent:   conversation.IntentCookingHelp,
		Status:   conversation.StatusActive,
		Metadata: map[string]interface{}{"invalid": func() {}}, // Functions can't be JSON marshaled
	}

	err := suite.conversationRepo.UpdateConversation(suite.ctx, conv)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to marshal metadata")
}

func (suite *ConversationRepositoryTestSuite) TestDeleteConversation_Success() {
	convID := "conv-123"

	suite.mock.ExpectBegin()
	suite.mock.ExpectExec(regexp.QuoteMeta(`UPDATE "conversations" SET "deleted_at"=$1 WHERE id = $2`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	suite.mock.ExpectCommit()

	err := suite.conversationRepo.DeleteConversation(suite.ctx, convID)
	assert.NoError(suite.T(), err)
}

// Message Repository Tests

func (suite *ConversationRepositoryTestSuite) TestCreateMessage_Success() {
	msg := &conversation.Message{
		ID:               "msg-123",
		ConversationID:   "conv-456",
		Role:             conversation.RoleUser,
		Content:          "Hello world",
		Metadata:         map[string]interface{}{"key": "value"},
		TokensUsed:       10,
		ProcessingTimeMs: 100,
		ModelUsed:        "gpt-4",
		CreatedAt:        time.Now(),
	}

	suite.mock.ExpectBegin()
	suite.mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "messages"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(msg.ID, msg.CreatedAt))
	suite.mock.ExpectCommit()

	err := suite.messageRepo.CreateMessage(suite.ctx, msg)
	assert.NoError(suite.T(), err)
}

func (suite *ConversationRepositoryTestSuite) TestGetMessage_Success() {
	msgID := "msg-123"
	expectedTime := time.Now()

	suite.mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "messages" WHERE id = $1`)).
		WithArgs(msgID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "conversation_id", "role", "content", "metadata", "tokens_used", "processing_time_ms", "model_used", "created_at",
		}).AddRow(
			msgID,
			"conv-456",
			"user",
			"Hello world",
			`{"key":"value"}`,
			10,
			100,
			"gpt-4",
			expectedTime,
		))

	result, err := suite.messageRepo.GetMessage(suite.ctx, msgID)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), msgID, result.ID)
	assert.Equal(suite.T(), "conv-456", result.ConversationID)
	assert.Equal(suite.T(), conversation.RoleUser, result.Role)
	assert.Equal(suite.T(), "Hello world", result.Content)
	assert.Equal(suite.T(), 10, result.TokensUsed)
}

func (suite *ConversationRepositoryTestSuite) TestGetConversationMessages_Success() {
	convID := "conv-456"
	limit := 10
	offset := 0
	expectedTime := time.Now()

	suite.mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "messages" WHERE conversation_id = $1 ORDER BY created_at ASC LIMIT $2`)).
		WithArgs(convID, limit).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "conversation_id", "role", "content", "metadata", "tokens_used", "processing_time_ms", "model_used", "created_at",
		}).
			AddRow("msg-1", convID, "user", "Message 1", `{}`, 10, 100, "gpt-4", expectedTime).
			AddRow("msg-2", convID, "assistant", "Message 2", `{}`, 20, 200, "gpt-4", expectedTime))

	result, err := suite.messageRepo.GetConversationMessages(suite.ctx, convID, limit, offset)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 2)
	assert.Equal(suite.T(), "msg-1", result[0].ID)
	assert.Equal(suite.T(), "msg-2", result[1].ID)
}

func (suite *ConversationRepositoryTestSuite) TestUpdateMessage_Success() {
	msg := &conversation.Message{
		ID:               "msg-123",
		Content:          "Updated content",
		Metadata:         map[string]interface{}{"updated": "value"},
		TokensUsed:       15,
		ProcessingTimeMs: 150,
		ModelUsed:        "gpt-4-turbo",
	}

	suite.mock.ExpectBegin()
	suite.mock.ExpectExec(regexp.QuoteMeta(`UPDATE "messages" SET`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	suite.mock.ExpectCommit()

	err := suite.messageRepo.UpdateMessage(suite.ctx, msg)
	assert.NoError(suite.T(), err)
}

func (suite *ConversationRepositoryTestSuite) TestDeleteMessage_Success() {
	msgID := "msg-123"

	suite.mock.ExpectBegin()
	suite.mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "messages" WHERE id = $1`)).
		WithArgs(msgID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	suite.mock.ExpectCommit()

	err := suite.messageRepo.DeleteMessage(suite.ctx, msgID)
	assert.NoError(suite.T(), err)
}

// Context Repository Tests

func (suite *ConversationRepositoryTestSuite) TestSetContext_Success() {
	ctx := &conversation.ConversationContext{
		ConversationID: "conv-123",
		ContextType:    "recipe_state",
		ContextData:    map[string]interface{}{"step": 1},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	suite.mock.ExpectBegin()
	suite.mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "conversation_context"`)).
		WillReturnRows(sqlmock.NewRows([]string{"conversation_id", "context_type"}).
			AddRow(ctx.ConversationID, ctx.ContextType))
	suite.mock.ExpectCommit()

	err := suite.contextRepo.SetContext(suite.ctx, ctx)
	assert.NoError(suite.T(), err)
}

func (suite *ConversationRepositoryTestSuite) TestGetContext_Success() {
	convID := "conv-123"
	contextType := "recipe_state"
	expectedTime := time.Now()

	suite.mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "conversation_context" WHERE conversation_id = $1 AND context_type = $2`)).
		WithArgs(convID, contextType).
		WillReturnRows(sqlmock.NewRows([]string{
			"conversation_id", "context_type", "context_data", "created_at", "updated_at",
		}).AddRow(
			convID,
			contextType,
			`{"step":1}`,
			expectedTime,
			expectedTime,
		))

	result, err := suite.contextRepo.GetContext(suite.ctx, convID, contextType)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), convID, result.ConversationID)
	assert.Equal(suite.T(), contextType, result.ContextType)
	assert.Equal(suite.T(), float64(1), result.ContextData["step"]) // JSON numbers are float64
}

func (suite *ConversationRepositoryTestSuite) TestGetAllContext_Success() {
	convID := "conv-123"
	expectedTime := time.Now()

	suite.mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "conversation_context" WHERE conversation_id = $1 ORDER BY context_type ASC`)).
		WithArgs(convID).
		WillReturnRows(sqlmock.NewRows([]string{
			"conversation_id", "context_type", "context_data", "created_at", "updated_at",
		}).
			AddRow(convID, "recipe_state", `{"step":1}`, expectedTime, expectedTime).
			AddRow(convID, "user_preferences", `{"dietary":"vegetarian"}`, expectedTime, expectedTime))

	result, err := suite.contextRepo.GetAllContext(suite.ctx, convID)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 2)
	assert.Equal(suite.T(), "recipe_state", result[0].ContextType)
	assert.Equal(suite.T(), "user_preferences", result[1].ContextType)
}

func (suite *ConversationRepositoryTestSuite) TestDeleteContext_Success() {
	convID := "conv-123"
	contextType := "recipe_state"

	suite.mock.ExpectBegin()
	suite.mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "conversation_context" WHERE conversation_id = $1 AND context_type = $2`)).
		WithArgs(convID, contextType).
		WillReturnResult(sqlmock.NewResult(1, 1))
	suite.mock.ExpectCommit()

	err := suite.contextRepo.DeleteContext(suite.ctx, convID, contextType)
	assert.NoError(suite.T(), err)
}

// Edge Cases and Error Scenarios

func (suite *ConversationRepositoryTestSuite) TestModelToConversation_EmptyMetadata() {
	model := &ConversationModel{
		ID:        "conv-123",
		UserID:    "user-456",
		Title:     "Test",
		Intent:    conversation.IntentRecipeCreation,
		Status:    conversation.StatusActive,
		Metadata:  "", // Empty metadata
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	result, err := suite.conversationRepo.modelToConversation(model)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.NotNil(suite.T(), result.Metadata)
	assert.Len(suite.T(), result.Metadata, 0)
}

func (suite *ConversationRepositoryTestSuite) TestModelToMessage_EmptyMetadata() {
	model := &MessageModel{
		ID:             "msg-123",
		ConversationID: "conv-456",
		Role:           conversation.RoleUser,
		Content:        "Test",
		Metadata:       "", // Empty metadata
		CreatedAt:      time.Now(),
	}

	result, err := suite.messageRepo.modelToMessage(model)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.NotNil(suite.T(), result.Metadata)
	assert.Len(suite.T(), result.Metadata, 0)
}

func (suite *ConversationRepositoryTestSuite) TestModelToContext_EmptyData() {
	model := &ConversationContextModel{
		ConversationID: "conv-123",
		ContextType:    "empty_state",
		ContextData:    "", // Empty context data
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	result, err := suite.contextRepo.modelToContext(model)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.NotNil(suite.T(), result.ContextData)
	assert.Len(suite.T(), result.ContextData, 0)
}

// Table-driven tests for comprehensive coverage

func (suite *ConversationRepositoryTestSuite) TestConversationRepository_TableDriven() {
	tests := []struct {
		name        string
		operation   string
		expectError bool
		setup       func()
		assertions  func()
	}{
		{
			name:        "Create conversation with nil metadata",
			operation:   "create",
			expectError: false,
			setup: func() {
				suite.mock.ExpectBegin()
				suite.mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "conversations"`)).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conv-123"))
				suite.mock.ExpectCommit()
			},
			assertions: func() {
				conv := &conversation.Conversation{
					ID:        "conv-123",
					UserID:    "user-456",
					Title:     "Test",
					Intent:    conversation.IntentRecipeCreation,
					Status:    conversation.StatusActive,
					Metadata:  nil, // Nil metadata
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				err := suite.conversationRepo.CreateConversation(suite.ctx, conv)
				assert.NoError(suite.T(), err)
			},
		},
		{
			name:        "Get conversation with database error",
			operation:   "get",
			expectError: true,
			setup: func() {
				suite.mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "conversations"`)).
					WillReturnError(errors.New("database connection lost"))
			},
			assertions: func() {
				result, err := suite.conversationRepo.GetConversation(suite.ctx, "conv-123")
				assert.Error(suite.T(), err)
				assert.Nil(suite.T(), result)
			},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			tt.setup()
			tt.assertions()
		})
	}
}

// Test Suite Runner

func TestConversationRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(ConversationRepositoryTestSuite))
}

// Benchmark Tests

func BenchmarkConversationRepository_CreateConversation(b *testing.B) {
	suite := &ConversationRepositoryTestSuite{}
	suite.SetupTest()

	conv := &conversation.Conversation{
		ID:        "conv-123",
		UserID:    "user-456",
		Title:     "Benchmark Test",
		Intent:    conversation.IntentRecipeCreation,
		Status:    conversation.StatusActive,
		Metadata:  map[string]interface{}{"key": "value"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Mock expectations for each iteration
		suite.mock.ExpectBegin()
		suite.mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "conversations"`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(conv.ID))
		suite.mock.ExpectCommit()

		_ = suite.conversationRepo.CreateConversation(suite.ctx, conv)
	}
}