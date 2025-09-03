package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	
	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/alchemorsel/v3/internal/infrastructure/http/handlers"
	gormRepo "github.com/alchemorsel/v3/internal/infrastructure/persistence/gorm"
	"gorm.io/gorm"
)

// Example router configuration for multi-chat conversation system
func setupConversationRoutes(r chi.Router, db *gorm.DB) {
	// Initialize repositories
	conversationRepo := gormRepo.NewConversationRepository(db)
	messageRepo := gormRepo.NewMessageRepository(db)
	contextRepo := gormRepo.NewContextRepository(db)
	
	// Initialize AI service (assuming you have this)
	// aiService := ai.NewService(...)
	
	// Initialize conversation service
	conversationService := conversation.NewService(
		conversationRepo,
		messageRepo,
		contextRepo,
		nil, // aiService - pass your AI service here
	)
	
	// Initialize chat handler
	chatHandler := handlers.NewChatHandler(conversationService)
	
	// Public routes (no authentication required)
	r.Route("/", func(r chi.Router) {
		// Chat page route
		r.Get("/chat", chatHandler.HandleChatPage)
	})
	
	// API routes (require authentication)
	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)
		// r.Use(AuthenticationMiddleware()) // Add your auth middleware
		
		r.Route("/chat", func(r chi.Router) {
			// Core chat functionality
			r.Post("/message", chatHandler.HandleChatMessage)
			r.Get("/conversations", chatHandler.HandleConversationList)
			r.Get("/history", chatHandler.HandleConversationHistory)
			
			// Conversation management
			r.Post("/rename", chatHandler.HandleConversationRename)
			r.Post("/delete", chatHandler.HandleConversationDelete)
			
			// Statistics
			r.Get("/stats", chatHandler.HandleConversationStats)
		})
	})
	
	// HTMX routes (for dynamic UI updates)
	r.Route("/htmx", func(r chi.Router) {
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)
		// r.Use(AuthenticationMiddleware()) // Add your auth middleware
		
		r.Route("/chat", func(r chi.Router) {
			// HTMX-compatible endpoints
			r.Post("/message", chatHandler.HandleAIChatHTMX)
			r.Get("/conversations-list", chatHandler.HandleConversationListHTMX)
		})
	})
	
	// WebSocket route (if you're using WebSockets)
	r.Route("/ws", func(r chi.Router) {
		// r.Use(WebSocketAuthMiddleware()) // Add WebSocket auth
		// r.Get("/chat", chatHandler.HandleWebSocketChat) // You'll need to implement this
	})
}

// Example middleware for session migration
func SessionMigrationMiddleware(convService *conversation.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := getUserFromContext(r.Context())
			if user != nil {
				// Check for existing session data
				sessionKey := fmt.Sprintf("ai_conversation_%s", user.ID)
				if sessionData := getFromSession(r, sessionKey); sessionData != nil {
					// Migrate to database asynchronously
					// TODO: Implement MigrateSessionToDatabase method in conversation service
					go func() {
						ctx := context.Background()
						// conv, err := convService.MigrateSessionToDatabase(ctx, user.ID, sessionData)
						// if err != nil {
						// 	log.Printf("Failed to migrate session for user %s: %v", user.ID, err)
						// } else if conv != nil {
						// 	log.Printf("Successfully migrated session to conversation %s for user %s", conv.ID, user.ID)
						// 	clearFromSession(w, r, sessionKey) // Clear old session
						// }
						log.Printf("Session migration not implemented for user %s", user.ID)
						_ = ctx
						_ = sessionData
					}()
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Example complete server setup
func main() {
	// Initialize database connection
	// db := initializeDatabase() // Your DB initialization - implement this function
	var db *gorm.DB // placeholder
	
	// Initialize router
	r := chi.NewRouter()
	
	// Global middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	
	// Add session migration middleware
	conversationService := initializeConversationService(db)
	r.Use(SessionMigrationMiddleware(conversationService))
	
	// Setup conversation routes
	setupConversationRoutes(r, db)
	
	// Other routes...
	
	// Start server
	log.Println("Server starting on :8080")
	http.ListenAndServe(":8080", r)
}

// Migration helper functions (implement these based on your session system)
func getFromSession(r *http.Request, key string) []conversation.ChatMessage {
	// Implement based on your session management system
	// Return existing session messages or nil
	return nil
}

func clearFromSession(w http.ResponseWriter, r *http.Request, key string) {
	// Implement session clearing logic
}

func getUserFromContext(ctx context.Context) *User {
	// Extract authenticated user from context
	if user, ok := ctx.Value("user").(*User); ok {
		return user
	}
	return nil
}

// Example conversation service initialization
func initializeConversationService(db *gorm.DB) *conversation.Service {
	conversationRepo := gormRepo.NewConversationRepository(db)
	messageRepo := gormRepo.NewMessageRepository(db)
	contextRepo := gormRepo.NewContextRepository(db)
	
	// Initialize with your AI service
	// aiService := ai.NewService(...)
	
	return conversation.NewService(
		conversationRepo,
		messageRepo,
		contextRepo,
		nil, // Replace with your AI service
	)
}

// Session migration implementation
// This would be implemented in the conversation service package
// func (s *conversation.Service) MigrateSessionToDatabase(ctx context.Context, userID string, sessionMessages []ChatMessage) (*conversation.Conversation, error) {
// 	if len(sessionMessages) == 0 {
// 		return nil, nil
// 	}

// 	// Create new conversation from session data
// 	firstMessage := sessionMessages[0].Content
// 	conv, err := s.CreateConversation(ctx, userID, firstMessage)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create conversation: %w", err)
// 	}

// 	// Migrate all messages
// 	for i, msg := range sessionMessages {
// 		role := conversation.MessageRole(strings.ToLower(msg.Role))
// 		if role != conversation.RoleUser && role != conversation.RoleAssistant && role != conversation.RoleSystem {
// 			log.Printf("Skipping message %d with invalid role: %s", i, msg.Role)
// 			continue
// 		}

// 		_, err := s.AddMessage(ctx, conv.ID, role, msg.Content, msg.Metadata)
// 		if err != nil {
// 			log.Printf("Failed to migrate message %d: %v", i, err)
// 			continue
// 		}
// 	}

// 	log.Printf("Successfully migrated %d messages to conversation %s", len(sessionMessages), conv.ID)
// 	return conv, nil
// }

// ChatMessage represents the old session-based message format
type ChatMessage struct {
	Role     string                 `json:"role"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
}

// User represents your user model
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// ... other user fields
}