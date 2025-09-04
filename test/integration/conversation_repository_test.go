package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/alchemorsel/v3/internal/infrastructure/persistence/gorm"
	"github.com/alchemorsel/v3/test/testutils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	gormdb "gorm.io/gorm"
)

// ConversationRepositoryTestSuite tests conversation repository with real database
type ConversationRepositoryTestSuite struct {
	suite.Suite
	testDB           *testutils.TestDatabase
	db               *gormdb.DB
	conversationRepo *gorm.ConversationRepository
	messageRepo      *gorm.MessageRepository
	contextRepo      *gorm.ContextRepository
	ctx              context.Context
	testUsers        []string
}

// SetupSuite initializes the test suite with a test database
func (suite *ConversationRepositoryTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	
	// Setup test database
	suite.testDB = testutils.SetupTestDatabase(suite.T())
	suite.db = suite.testDB.GormDB
	
	// Create repositories
	suite.conversationRepo = gorm.NewConversationRepository(suite.db)
	suite.messageRepo = gorm.NewMessageRepository(suite.db)
	suite.contextRepo = gorm.NewContextRepository(suite.db)
	
	// Create test users
	suite.testUsers = []string{
		"550e8400-e29b-41d4-a716-446655440001",
		"550e8400-e29b-41d4-a716-446655440002",
		"550e8400-e29b-41d4-a716-446655440003",
	}
}

// TearDownSuite cleans up the test database
func (suite *ConversationRepositoryTestSuite) TearDownSuite() {
	suite.testDB.Cleanup()
}

// SetupTest cleans up data before each test
func (suite *ConversationRepositoryTestSuite) SetupTest() {
	suite.testDB.TruncateAllTables()
}

// TestConversationCRUD tests basic CRUD operations for conversations
func (suite *ConversationRepositoryTestSuite) TestConversationCRUD() {
	userID := suite.testUsers[0]

	// Test Create
	suite.Run("Create Conversation", func() {
		conv := &conversation.Conversation{
			ID:     uuid.New().String(),
			UserID: userID,
			Title:  "Test Recipe Creation",
			Intent: conversation.IntentRecipeCreation,
			Status: conversation.StatusActive,
			Metadata: map[string]interface{}{
				"source": "test",
				"tags":   []string{"pasta", "italian"},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err := suite.conversationRepo.CreateConversation(suite.ctx, conv)
		suite.NoError(err)

		// Verify creation by reading back
		retrieved, err := suite.conversationRepo.GetConversation(suite.ctx, conv.ID)
		suite.NoError(err)
		suite.NotNil(retrieved)
		suite.Equal(conv.ID, retrieved.ID)
		suite.Equal(conv.UserID, retrieved.UserID)
		suite.Equal(conv.Title, retrieved.Title)
		suite.Equal(conv.Intent, retrieved.Intent)
		suite.Equal(conv.Status, retrieved.Status)
		suite.NotNil(retrieved.Metadata)
	})

	// Test Read
	suite.Run("Get Conversation", func() {
		// Create a conversation first
		conv := suite.createTestConversation(userID, conversation.IntentCookingHelp)

		// Read it back
		retrieved, err := suite.conversationRepo.GetConversation(suite.ctx, conv.ID)
		suite.NoError(err)
		suite.NotNil(retrieved)
		suite.Equal(conv.ID, retrieved.ID)
		suite.Equal(conv.Title, retrieved.Title)
	})

	// Test Update
	suite.Run("Update Conversation", func() {
		conv := suite.createTestConversation(userID, conversation.IntentRecipeCreation)

		// Update the conversation
		conv.Title = "Updated Title"
		conv.Status = conversation.StatusArchived
		conv.UpdatedAt = time.Now()

		err := suite.conversationRepo.UpdateConversation(suite.ctx, conv)
		suite.NoError(err)

		// Verify update
		updated, err := suite.conversationRepo.GetConversation(suite.ctx, conv.ID)
		suite.NoError(err)
		suite.Equal("Updated Title", updated.Title)
		suite.Equal(conversation.StatusArchived, updated.Status)
	})

	// Test Delete (Soft Delete)
	suite.Run("Delete Conversation", func() {
		conv := suite.createTestConversation(userID, conversation.IntentRecipeCreation)

		// Delete the conversation
		err := suite.conversationRepo.DeleteConversation(suite.ctx, conv.ID)
		suite.NoError(err)

		// Verify it's no longer accessible
		_, err = suite.conversationRepo.GetConversation(suite.ctx, conv.ID)
		suite.Error(err)
	})
}

// TestGetUserConversations tests retrieving conversations by user
func (suite *ConversationRepositoryTestSuite) TestGetUserConversations() {
	userID1 := suite.testUsers[0]
	userID2 := suite.testUsers[1]

	// Create conversations for both users
	_ = suite.createTestConversation(userID1, conversation.IntentRecipeCreation)
	_ = suite.createTestConversation(userID1, conversation.IntentCookingHelp)
	conv3 := suite.createTestConversation(userID2, conversation.IntentMealPlanning)

	// Test getting conversations for user1
	suite.Run("Get User1 Conversations", func() {
		conversations, err := suite.conversationRepo.GetUserConversations(suite.ctx, userID1, 10, 0)
		suite.NoError(err)
		suite.Len(conversations, 2)

		// Verify conversations are ordered by updated_at DESC
		suite.True(conversations[0].UpdatedAt.After(conversations[1].UpdatedAt) || 
				  conversations[0].UpdatedAt.Equal(conversations[1].UpdatedAt))

		// Verify all conversations belong to user1
		for _, conv := range conversations {
			suite.Equal(userID1, conv.UserID)
		}
	})

	// Test getting conversations for user2
	suite.Run("Get User2 Conversations", func() {
		conversations, err := suite.conversationRepo.GetUserConversations(suite.ctx, userID2, 10, 0)
		suite.NoError(err)
		suite.Len(conversations, 1)
		suite.Equal(conv3.ID, conversations[0].ID)
	})

	// Test pagination
	suite.Run("Pagination", func() {
		// Create more conversations for user1
		for i := 0; i < 5; i++ {
			suite.createTestConversation(userID1, conversation.IntentRecipeCreation)
			time.Sleep(10 * time.Millisecond) // Ensure different timestamps
		}

		// Get first page
		page1, err := suite.conversationRepo.GetUserConversations(suite.ctx, userID1, 3, 0)
		suite.NoError(err)
		suite.Len(page1, 3)

		// Get second page
		page2, err := suite.conversationRepo.GetUserConversations(suite.ctx, userID1, 3, 3)
		suite.NoError(err)
		suite.Len(page2, 3)

		// Verify no overlap
		for _, conv1 := range page1 {
			for _, conv2 := range page2 {
				suite.NotEqual(conv1.ID, conv2.ID)
			}
		}
	})
}

// TestMessageCRUD tests basic CRUD operations for messages
func (suite *ConversationRepositoryTestSuite) TestMessageCRUD() {
	userID := suite.testUsers[0]
	conv := suite.createTestConversation(userID, conversation.IntentRecipeCreation)

	// Test Create Message
	suite.Run("Create Message", func() {
		msg := &conversation.Message{
			ID:             uuid.New().String(),
			ConversationID: conv.ID,
			Role:           conversation.RoleUser,
			Content:        "I want to make pasta carbonara",
			Metadata: map[string]interface{}{
				"timestamp": time.Now().Unix(),
				"client":    "web",
			},
			TokensUsed:       25,
			ProcessingTimeMs: 150,
			ModelUsed:        "gpt-3.5-turbo",
			CreatedAt:        time.Now(),
		}

		err := suite.messageRepo.CreateMessage(suite.ctx, msg)
		suite.NoError(err)

		// Verify creation
		retrieved, err := suite.messageRepo.GetMessage(suite.ctx, msg.ID)
		suite.NoError(err)
		suite.Equal(msg.Content, retrieved.Content)
		suite.Equal(msg.Role, retrieved.Role)
		suite.Equal(msg.TokensUsed, retrieved.TokensUsed)
	})

	// Test Get Message
	suite.Run("Get Message", func() {
		msg := suite.createTestMessage(conv.ID, conversation.RoleAssistant, "Sure! I'll help you make carbonara.")

		retrieved, err := suite.messageRepo.GetMessage(suite.ctx, msg.ID)
		suite.NoError(err)
		suite.Equal(msg.Content, retrieved.Content)
		suite.Equal(msg.Role, retrieved.Role)
	})

	// Test Update Message
	suite.Run("Update Message", func() {
		msg := suite.createTestMessage(conv.ID, conversation.RoleUser, "Original content")

		// Update message
		msg.Content = "Updated content"
		msg.TokensUsed = 30
		msg.ProcessingTimeMs = 200

		err := suite.messageRepo.UpdateMessage(suite.ctx, msg)
		suite.NoError(err)

		// Verify update
		updated, err := suite.messageRepo.GetMessage(suite.ctx, msg.ID)
		suite.NoError(err)
		suite.Equal("Updated content", updated.Content)
		suite.Equal(30, updated.TokensUsed)
		suite.Equal(200, updated.ProcessingTimeMs)
	})

	// Test Delete Message
	suite.Run("Delete Message", func() {
		msg := suite.createTestMessage(conv.ID, conversation.RoleUser, "Message to delete")

		err := suite.messageRepo.DeleteMessage(suite.ctx, msg.ID)
		suite.NoError(err)

		// Verify deletion
		_, err = suite.messageRepo.GetMessage(suite.ctx, msg.ID)
		suite.Error(err)
	})
}

// TestGetConversationMessages tests retrieving messages for a conversation
func (suite *ConversationRepositoryTestSuite) TestGetConversationMessages() {
	userID := suite.testUsers[0]
	conv := suite.createTestConversation(userID, conversation.IntentRecipeCreation)

	// Create messages in conversation
	_ = []*conversation.Message{
		suite.createTestMessage(conv.ID, conversation.RoleUser, "I want to make pasta"),
		suite.createTestMessage(conv.ID, conversation.RoleAssistant, "What type of pasta?"),
		suite.createTestMessage(conv.ID, conversation.RoleUser, "Carbonara"),
		suite.createTestMessage(conv.ID, conversation.RoleAssistant, "Great choice!"),
	}

	// Test getting all messages
	suite.Run("Get All Messages", func() {
		retrieved, err := suite.messageRepo.GetConversationMessages(suite.ctx, conv.ID, 100, 0)
		suite.NoError(err)
		suite.Len(retrieved, 4)

		// Verify messages are ordered by created_at ASC
		for i := 1; i < len(retrieved); i++ {
			suite.True(retrieved[i].CreatedAt.After(retrieved[i-1].CreatedAt) ||
					  retrieved[i].CreatedAt.Equal(retrieved[i-1].CreatedAt))
		}
	})

	// Test pagination
	suite.Run("Paginated Messages", func() {
		// Get first 2 messages
		page1, err := suite.messageRepo.GetConversationMessages(suite.ctx, conv.ID, 2, 0)
		suite.NoError(err)
		suite.Len(page1, 2)

		// Get next 2 messages
		page2, err := suite.messageRepo.GetConversationMessages(suite.ctx, conv.ID, 2, 2)
		suite.NoError(err)
		suite.Len(page2, 2)

		// Verify order and no overlap
		suite.True(page2[0].CreatedAt.After(page1[1].CreatedAt) ||
				  page2[0].CreatedAt.Equal(page1[1].CreatedAt))
	})

	// Test with different conversation
	suite.Run("Different Conversation", func() {
		conv2 := suite.createTestConversation(userID, conversation.IntentCookingHelp)
		suite.createTestMessage(conv2.ID, conversation.RoleUser, "How do I boil water?")

		// Should only get messages for conv2
		messages, err := suite.messageRepo.GetConversationMessages(suite.ctx, conv2.ID, 100, 0)
		suite.NoError(err)
		suite.Len(messages, 1)
		suite.Equal("How do I boil water?", messages[0].Content)
	})
}

// TestConversationContextCRUD tests context repository operations
func (suite *ConversationRepositoryTestSuite) TestConversationContextCRUD() {
	userID := suite.testUsers[0]
	conv := suite.createTestConversation(userID, conversation.IntentRecipeCreation)

	// Test Set Context
	suite.Run("Set Context", func() {
		contextData := map[string]interface{}{
			"recipe_progress": "gathering_ingredients",
			"ingredients":     []string{"pasta", "eggs", "cheese"},
			"servings":        4,
		}

		ctx := &conversation.ConversationContext{
			ConversationID: conv.ID,
			ContextType:    "recipe_creation",
			ContextData:    contextData,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		err := suite.contextRepo.SetContext(suite.ctx, ctx)
		suite.NoError(err)

		// Verify context was set
		retrieved, err := suite.contextRepo.GetContext(suite.ctx, conv.ID, "recipe_creation")
		suite.NoError(err)
		suite.Equal(conv.ID, retrieved.ConversationID)
		suite.Equal("recipe_creation", retrieved.ContextType)
		suite.Equal("gathering_ingredients", retrieved.ContextData["recipe_progress"])
	})

	// Test Update Context (Upsert)
	suite.Run("Update Context", func() {
		// Set initial context
		initialData := map[string]interface{}{
			"step": 1,
			"data": "initial",
		}

		ctx1 := &conversation.ConversationContext{
			ConversationID: conv.ID,
			ContextType:    "workflow",
			ContextData:    initialData,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		err := suite.contextRepo.SetContext(suite.ctx, ctx1)
		suite.NoError(err)

		// Update same context
		updatedData := map[string]interface{}{
			"step": 2,
			"data": "updated",
		}

		ctx2 := &conversation.ConversationContext{
			ConversationID: conv.ID,
			ContextType:    "workflow",
			ContextData:    updatedData,
			CreatedAt:      ctx1.CreatedAt,
			UpdatedAt:      time.Now(),
		}

		err = suite.contextRepo.SetContext(suite.ctx, ctx2)
		suite.NoError(err)

		// Verify update
		retrieved, err := suite.contextRepo.GetContext(suite.ctx, conv.ID, "workflow")
		suite.NoError(err)
		suite.Equal(float64(2), retrieved.ContextData["step"]) // JSON numbers are float64
		suite.Equal("updated", retrieved.ContextData["data"])
	})

	// Test Get All Context
	suite.Run("Get All Context", func() {
		// Set multiple contexts
		contexts := []struct {
			contextType string
			data        map[string]interface{}
		}{
			{"recipe_creation", map[string]interface{}{"step": "ingredients"}},
			{"user_preferences", map[string]interface{}{"dietary": "vegetarian"}},
			{"ai_metadata", map[string]interface{}{"model": "gpt-3.5"}},
		}

		for _, ctx := range contexts {
			contextObj := &conversation.ConversationContext{
				ConversationID: conv.ID,
				ContextType:    ctx.contextType,
				ContextData:    ctx.data,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}
			err := suite.contextRepo.SetContext(suite.ctx, contextObj)
			suite.NoError(err)
		}

		// Get all contexts
		allContexts, err := suite.contextRepo.GetAllContext(suite.ctx, conv.ID)
		suite.NoError(err)
		suite.Len(allContexts, 3)

		// Verify all contexts are present
		contextTypes := make(map[string]bool)
		for _, ctx := range allContexts {
			contextTypes[ctx.ContextType] = true
		}

		suite.True(contextTypes["recipe_creation"])
		suite.True(contextTypes["user_preferences"])
		suite.True(contextTypes["ai_metadata"])
	})

	// Test Delete Context
	suite.Run("Delete Context", func() {
		// Set context first
		ctx := &conversation.ConversationContext{
			ConversationID: conv.ID,
			ContextType:    "temporary",
			ContextData:    map[string]interface{}{"temp": "data"},
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		err := suite.contextRepo.SetContext(suite.ctx, ctx)
		suite.NoError(err)

		// Verify it exists
		_, err = suite.contextRepo.GetContext(suite.ctx, conv.ID, "temporary")
		suite.NoError(err)

		// Delete it
		err = suite.contextRepo.DeleteContext(suite.ctx, conv.ID, "temporary")
		suite.NoError(err)

		// Verify it's gone
		_, err = suite.contextRepo.GetContext(suite.ctx, conv.ID, "temporary")
		suite.Error(err)
	})
}

// TestRepositoryTransactions tests transactional behavior
func (suite *ConversationRepositoryTestSuite) TestRepositoryTransactions() {
	userID := suite.testUsers[0]

	suite.Run("Transaction Rollback", func() {
		// Start transaction
		tx := suite.db.Begin()
		defer tx.Rollback()

		txConversationRepo := gorm.NewConversationRepository(tx)
		txMessageRepo := gorm.NewMessageRepository(tx)

		// Create conversation in transaction
		conv := &conversation.Conversation{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     "Transaction Test",
			Intent:    conversation.IntentRecipeCreation,
			Status:    conversation.StatusActive,
			Metadata:  make(map[string]interface{}),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err := txConversationRepo.CreateConversation(suite.ctx, conv)
		suite.NoError(err)

		// Create message in transaction
		msg := &conversation.Message{
			ID:             uuid.New().String(),
			ConversationID: conv.ID,
			Role:           conversation.RoleUser,
			Content:        "Test message",
			Metadata:       make(map[string]interface{}),
			CreatedAt:      time.Now(),
		}

		err = txMessageRepo.CreateMessage(suite.ctx, msg)
		suite.NoError(err)

		// Verify they exist in transaction
		_, err = txConversationRepo.GetConversation(suite.ctx, conv.ID)
		suite.NoError(err)

		_, err = txMessageRepo.GetMessage(suite.ctx, msg.ID)
		suite.NoError(err)

		// Rollback transaction
		tx.Rollback()

		// Verify they don't exist outside transaction
		_, err = suite.conversationRepo.GetConversation(suite.ctx, conv.ID)
		suite.Error(err)

		_, err = suite.messageRepo.GetMessage(suite.ctx, msg.ID)
		suite.Error(err)
	})

	suite.Run("Transaction Commit", func() {
		// Start transaction
		tx := suite.db.Begin()

		txConversationRepo := gorm.NewConversationRepository(tx)
		txMessageRepo := gorm.NewMessageRepository(tx)

		// Create conversation in transaction
		conv := &conversation.Conversation{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     "Commit Test",
			Intent:    conversation.IntentRecipeCreation,
			Status:    conversation.StatusActive,
			Metadata:  make(map[string]interface{}),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err := txConversationRepo.CreateConversation(suite.ctx, conv)
		suite.NoError(err)

		// Create message in transaction
		msg := &conversation.Message{
			ID:             uuid.New().String(),
			ConversationID: conv.ID,
			Role:           conversation.RoleUser,
			Content:        "Commit test message",
			Metadata:       make(map[string]interface{}),
			CreatedAt:      time.Now(),
		}

		err = txMessageRepo.CreateMessage(suite.ctx, msg)
		suite.NoError(err)

		// Commit transaction
		err = tx.Commit().Error
		suite.NoError(err)

		// Verify they exist outside transaction
		retrievedConv, err := suite.conversationRepo.GetConversation(suite.ctx, conv.ID)
		suite.NoError(err)
		suite.Equal(conv.Title, retrievedConv.Title)

		retrievedMsg, err := suite.messageRepo.GetMessage(suite.ctx, msg.ID)
		suite.NoError(err)
		suite.Equal(msg.Content, retrievedMsg.Content)
	})
}

// TestRepositoryPerformance tests repository performance characteristics
func (suite *ConversationRepositoryTestSuite) TestRepositoryPerformance() {
	userID := suite.testUsers[0]

	suite.Run("Large Dataset Performance", func() {
		// Create a conversation with many messages
		conv := suite.createTestConversation(userID, conversation.IntentRecipeCreation)

		// Create 1000 messages
		const numMessages = 1000
		start := time.Now()

		for i := 0; i < numMessages; i++ {
			role := conversation.RoleUser
			if i%2 == 1 {
				role = conversation.RoleAssistant
			}

			msg := &conversation.Message{
				ID:             uuid.New().String(),
				ConversationID: conv.ID,
				Role:           role,
				Content:        fmt.Sprintf("Message %d", i+1),
				Metadata:       make(map[string]interface{}),
				CreatedAt:      time.Now().Add(time.Duration(i) * time.Millisecond),
			}

			err := suite.messageRepo.CreateMessage(suite.ctx, msg)
			suite.NoError(err)
		}

		insertDuration := time.Since(start)
		suite.T().Logf("Inserted %d messages in %v", numMessages, insertDuration)

		// Test retrieval performance
		start = time.Now()
		messages, err := suite.messageRepo.GetConversationMessages(suite.ctx, conv.ID, numMessages, 0)
		retrievalDuration := time.Since(start)

		suite.NoError(err)
		suite.Len(messages, numMessages)
		suite.T().Logf("Retrieved %d messages in %v", numMessages, retrievalDuration)

		// Performance assertions (adjust based on your requirements)
		suite.Less(retrievalDuration, 5*time.Second, "Retrieval should be fast even with many messages")
	})

	suite.Run("Concurrent Operations", func() {
		const numGoroutines = 10
		const operationsPerGoroutine = 10

		results := make(chan error, numGoroutines*operationsPerGoroutine)

		// Launch concurrent operations
		for i := 0; i < numGoroutines; i++ {
			go func(routineID int) {
				for j := 0; j < operationsPerGoroutine; j++ {
					// Create conversation
					conv := &conversation.Conversation{
						ID:        uuid.New().String(),
						UserID:    userID,
						Title:     fmt.Sprintf("Concurrent Conv %d-%d", routineID, j),
						Intent:    conversation.IntentRecipeCreation,
						Status:    conversation.StatusActive,
						Metadata:  make(map[string]interface{}),
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					}

					err := suite.conversationRepo.CreateConversation(suite.ctx, conv)
					if err != nil {
						results <- err
						continue
					}

					// Create message
					msg := &conversation.Message{
						ID:             uuid.New().String(),
						ConversationID: conv.ID,
						Role:           conversation.RoleUser,
						Content:        fmt.Sprintf("Concurrent message %d-%d", routineID, j),
						Metadata:       make(map[string]interface{}),
						CreatedAt:      time.Now(),
					}

					err = suite.messageRepo.CreateMessage(suite.ctx, msg)
					results <- err
				}
			}(i)
		}

		// Collect results
		for i := 0; i < numGoroutines*operationsPerGoroutine; i++ {
			err := <-results
			suite.NoError(err, "Concurrent operation should succeed")
		}
	})
}

// TestRepositoryErrorHandling tests error handling scenarios
func (suite *ConversationRepositoryTestSuite) TestRepositoryErrorHandling() {
	userID := suite.testUsers[0]

	suite.Run("Duplicate ID Error", func() {
		conv := suite.createTestConversation(userID, conversation.IntentRecipeCreation)

		// Try to create another conversation with same ID
		duplicateConv := &conversation.Conversation{
			ID:        conv.ID, // Same ID
			UserID:    userID,
			Title:     "Duplicate",
			Intent:    conversation.IntentCookingHelp,
			Status:    conversation.StatusActive,
			Metadata:  make(map[string]interface{}),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err := suite.conversationRepo.CreateConversation(suite.ctx, duplicateConv)
		suite.Error(err, "Should error on duplicate ID")
	})

	suite.Run("Invalid Foreign Key", func() {
		nonExistentConvID := uuid.New().String()

		msg := &conversation.Message{
			ID:             uuid.New().String(),
			ConversationID: nonExistentConvID, // Non-existent conversation
			Role:           conversation.RoleUser,
			Content:        "Test message",
			Metadata:       make(map[string]interface{}),
			CreatedAt:      time.Now(),
		}

		err := suite.messageRepo.CreateMessage(suite.ctx, msg)
		// Note: Depending on your foreign key constraints, this might or might not error
		// If you have FK constraints, it should error
		if err != nil {
			suite.T().Logf("Foreign key constraint working: %v", err)
		}
	})

	suite.Run("Get Non-Existent Record", func() {
		nonExistentID := uuid.New().String()

		_, err := suite.conversationRepo.GetConversation(suite.ctx, nonExistentID)
		suite.Error(err, "Should error when getting non-existent conversation")

		_, err = suite.messageRepo.GetMessage(suite.ctx, nonExistentID)
		suite.Error(err, "Should error when getting non-existent message")

		_, err = suite.contextRepo.GetContext(suite.ctx, nonExistentID, "test")
		suite.Error(err, "Should error when getting non-existent context")
	})
}

// Helper methods

// createTestConversation creates a test conversation and saves it to the database
func (suite *ConversationRepositoryTestSuite) createTestConversation(userID string, intent conversation.ConversationIntent) *conversation.Conversation {
	conv := &conversation.Conversation{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     fmt.Sprintf("Test %s Conversation", intent),
		Intent:    intent,
		Status:    conversation.StatusActive,
		Metadata:  make(map[string]interface{}),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := suite.conversationRepo.CreateConversation(suite.ctx, conv)
	suite.Require().NoError(err)

	return conv
}

// createTestMessage creates a test message and saves it to the database
func (suite *ConversationRepositoryTestSuite) createTestMessage(conversationID string, role conversation.MessageRole, content string) *conversation.Message {
	msg := &conversation.Message{
		ID:             uuid.New().String(),
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		Metadata:       make(map[string]interface{}),
		CreatedAt:      time.Now(),
	}

	err := suite.messageRepo.CreateMessage(suite.ctx, msg)
	suite.Require().NoError(err)

	return msg
}

// Run the test suite
func TestConversationRepositorySuite(t *testing.T) {
	suite.Run(t, new(ConversationRepositoryTestSuite))
}

// TestRepositoryDataIntegrity tests data integrity scenarios
func TestRepositoryDataIntegrity(t *testing.T) {
	testDB := testutils.SetupTestDatabase(t)
	defer testDB.Cleanup()

	conversationRepo := gorm.NewConversationRepository(testDB.GormDB)
	messageRepo := gorm.NewMessageRepository(testDB.GormDB)
	ctx := context.Background()

	t.Run("JSON Metadata Handling", func(t *testing.T) {
		userID := "550e8400-e29b-41d4-a716-446655440001"

		// Test complex metadata
		complexMetadata := map[string]interface{}{
			"string_field":  "test string",
			"int_field":     42,
			"float_field":   3.14,
			"bool_field":    true,
			"array_field":   []string{"a", "b", "c"},
			"object_field": map[string]interface{}{
				"nested_string": "nested value",
				"nested_int":    123,
			},
			"null_field": nil,
		}

		conv := &conversation.Conversation{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     "JSON Metadata Test",
			Intent:    conversation.IntentRecipeCreation,
			Status:    conversation.StatusActive,
			Metadata:  complexMetadata,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err := conversationRepo.CreateConversation(ctx, conv)
		require.NoError(t, err)

		// Retrieve and verify metadata
		retrieved, err := conversationRepo.GetConversation(ctx, conv.ID)
		require.NoError(t, err)

		assert.Equal(t, "test string", retrieved.Metadata["string_field"])
		assert.Equal(t, float64(42), retrieved.Metadata["int_field"]) // JSON numbers are float64
		assert.Equal(t, 3.14, retrieved.Metadata["float_field"])
		assert.Equal(t, true, retrieved.Metadata["bool_field"])
		assert.Equal(t, []interface{}{"a", "b", "c"}, retrieved.Metadata["array_field"])
		
		nestedObj := retrieved.Metadata["object_field"].(map[string]interface{})
		assert.Equal(t, "nested value", nestedObj["nested_string"])
		assert.Equal(t, float64(123), nestedObj["nested_int"])
		
		assert.Nil(t, retrieved.Metadata["null_field"])
	})

	t.Run("Unicode Content Handling", func(t *testing.T) {
		userID := "550e8400-e29b-41d4-a716-446655440001"

		conv := &conversation.Conversation{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     "Unicode Test 🍝🍕🥘",
			Intent:    conversation.IntentRecipeCreation,
			Status:    conversation.StatusActive,
			Metadata:  make(map[string]interface{}),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err := conversationRepo.CreateConversation(ctx, conv)
		require.NoError(t, err)

		// Create message with unicode content
		unicodeContent := "Let's make 🍝 pasta! 今日はパスタを作りましょう! Давайте приготовим пасту! مرحبا دعونا نجعل المعكرونة!"

		msg := &conversation.Message{
			ID:             uuid.New().String(),
			ConversationID: conv.ID,
			Role:           conversation.RoleUser,
			Content:        unicodeContent,
			Metadata:       make(map[string]interface{}),
			CreatedAt:      time.Now(),
		}

		err = messageRepo.CreateMessage(ctx, msg)
		require.NoError(t, err)

		// Retrieve and verify unicode handling
		retrievedConv, err := conversationRepo.GetConversation(ctx, conv.ID)
		require.NoError(t, err)
		assert.Equal(t, "Unicode Test 🍝🍕🥘", retrievedConv.Title)

		retrievedMsg, err := messageRepo.GetMessage(ctx, msg.ID)
		require.NoError(t, err)
		assert.Equal(t, unicodeContent, retrievedMsg.Content)
	})

	t.Run("Large Content Handling", func(t *testing.T) {
		userID := "550e8400-e29b-41d4-a716-446655440001"

		conv := &conversation.Conversation{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     "Large Content Test",
			Intent:    conversation.IntentRecipeCreation,
			Status:    conversation.StatusActive,
			Metadata:  make(map[string]interface{}),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err := conversationRepo.CreateConversation(ctx, conv)
		require.NoError(t, err)

		// Create message with large content (10KB)
		largeContent := strings.Repeat("This is a very long recipe instruction that repeats many times. ", 150)

		msg := &conversation.Message{
			ID:             uuid.New().String(),
			ConversationID: conv.ID,
			Role:           conversation.RoleAssistant,
			Content:        largeContent,
			Metadata:       make(map[string]interface{}),
			CreatedAt:      time.Now(),
		}

		err = messageRepo.CreateMessage(ctx, msg)
		require.NoError(t, err)

		// Retrieve and verify large content
		retrievedMsg, err := messageRepo.GetMessage(ctx, msg.ID)
		require.NoError(t, err)
		assert.Equal(t, largeContent, retrievedMsg.Content)
		assert.Greater(t, len(retrievedMsg.Content), 10000) // Should be > 10KB
	})
}