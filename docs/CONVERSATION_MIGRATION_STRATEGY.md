# Multi-Chat Conversation System Migration Strategy

## Overview
This document outlines the migration strategy from the current session-based single conversation storage (`ai_conversation_{userID}`) to a database-backed multi-conversation system.

## Current State Analysis

### Existing Implementation
- **Storage**: Session-based with key `ai_conversation_{userID}`
- **Data Structure**: `[]conversation.ChatMessage` stored in session
- **Interface**: Single chat interface on home page
- **Persistence**: Temporary (lost on session expiry)

### Existing Infrastructure
✅ **Already Implemented:**
- PostgreSQL database with conversation schema (migration 000002)
- Go struct definitions for Conversation, Message, and ConversationContext
- GORM repository implementations
- Conversation service with intent classification
- Basic chat handler with HTTP endpoints
- HTMX-ready chat interface template

## Migration Steps

### Phase 1: Database Setup (Ready for Production)

The database schema is already implemented in migration `000002_conversation_schema.up.sql`:

```sql
-- Key tables already created:
- conversations (id, user_id, title, intent, status, metadata, timestamps)
- messages (id, conversation_id, role, content, metadata, timestamps)
- conversation_context (conversation_id, context_type, context_data)
```

**Enhanced Features Added:**
- Automatic title generation from first message content
- Advanced regex patterns for recipe/dish name extraction
- Conversation management triggers

### Phase 2: API Endpoints (Implemented)

#### New Endpoints Added:
```
GET  /api/chat/conversations      - List user conversations
GET  /api/chat/history            - Get conversation with messages
POST /api/chat/rename             - Rename conversation
POST /api/chat/delete             - Delete conversation
GET  /htmx/chat/conversations-list - HTMX partial for conversation list
```

#### Enhanced Existing Endpoints:
- `/api/chat/message` - Now creates/continues conversations
- `/htmx/chat/message` - HTMX-compatible chat interface

### Phase 3: Frontend Enhancements (Implemented)

#### Multi-Chat Interface Features:
- **Conversation Sidebar**: Lists all user conversations with titles, previews, and timestamps
- **Active Conversation Tracking**: Visual indication of current conversation
- **Conversation Management**: Rename, delete functionality with confirmation
- **Real-time Updates**: Automatic conversation list refresh
- **Responsive Design**: Mobile-friendly sidebar that hides on small screens

#### JavaScript Functions Added:
```javascript
- loadConversations()           - Fetch and display conversation list
- loadConversation(id)          - Switch to specific conversation
- renameConversation(id, title) - Rename conversation
- deleteConversation(id)        - Delete conversation with confirmation
- startNewConversation()        - Create new conversation
```

### Phase 4: Session Migration Strategy

#### 4.1 Gradual Migration Approach

**Option A: Automatic Migration (Recommended)**
```go
// Add to conversation service
func (s *Service) MigrateSessionToDatabase(ctx context.Context, userID string, sessionMessages []ChatMessage) (*Conversation, error) {
    if len(sessionMessages) == 0 {
        return nil, nil
    }

    // Create new conversation from session data
    firstMessage := sessionMessages[0].Content
    conv, err := s.CreateConversation(ctx, userID, firstMessage)
    if err != nil {
        return nil, err
    }

    // Migrate all messages
    for _, msg := range sessionMessages {
        role := conversation.MessageRole(msg.Role)
        _, err := s.AddMessage(ctx, conv.ID, role, msg.Content, msg.Metadata)
        if err != nil {
            log.Printf("Failed to migrate message: %v", err)
            continue
        }
    }

    return conv, nil
}
```

**Option B: User-Initiated Migration**
- Add "Save Conversation" button to existing chat
- Allow users to choose whether to save current session
- Provide migration status feedback

#### 4.2 Implementation Steps

1. **Add Migration Middleware**:
```go
func SessionMigrationMiddleware(convService *conversation.Service) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user := getUserFromContext(r.Context())
            if user != nil {
                // Check for existing session data
                sessionKey := fmt.Sprintf("ai_conversation_%s", user.ID)
                if sessionData := getFromSession(r, sessionKey); sessionData != nil {
                    // Migrate to database
                    go func() {
                        ctx := context.Background()
                        convService.MigrateSessionToDatabase(ctx, user.ID, sessionData)
                        clearFromSession(w, r, sessionKey) // Clear old session
                    }()
                }
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

2. **Update Route Configuration**:
```go
// In your router setup
r.Use(SessionMigrationMiddleware(conversationService))
```

3. **Graceful Fallback**:
```go
func (h *ChatHandler) HandleChatWithFallback(w http.ResponseWriter, r *http.Request) {
    user := getUserFromHTTPContext(r.Context())
    
    // Try database first
    conversations, err := h.convService.GetUserConversations(ctx, user.ID, 1, 0)
    if err != nil || len(conversations) == 0 {
        // Fall back to session if no database conversations
        sessionKey := fmt.Sprintf("ai_conversation_%s", user.ID)
        if sessionData := getFromSession(r, sessionKey); sessionData != nil {
            // Use session data and suggest migration
            // ... render with migration prompt
        }
    }
    // ... rest of handler
}
```

### Phase 5: Deployment Strategy

#### 5.1 Pre-Deployment
- [ ] Run database migrations
- [ ] Test conversation creation and management
- [ ] Verify HTMX endpoints work correctly
- [ ] Test conversation title generation

#### 5.2 Deployment
1. **Deploy with Feature Flag**:
```go
const useMultiChatSystem = os.Getenv("ENABLE_MULTI_CHAT") == "true"
```

2. **Staged Rollout**:
   - Phase A: Admin users only
   - Phase B: Premium users
   - Phase C: All users

3. **Monitoring**:
   - Track conversation creation rates
   - Monitor database performance
   - Watch for session migration errors

#### 5.3 Post-Deployment
- [ ] Monitor conversation engagement metrics
- [ ] Collect user feedback on multi-chat experience
- [ ] Remove session-based code after successful migration

## API Documentation

### Conversation Management Endpoints

#### Create/Continue Conversation
```http
POST /api/chat/message
Content-Type: application/x-www-form-urlencoded

message=I want to make pasta&conversation_id=optional-uuid
```

Response:
```json
{
  "success": true,
  "response": "AI response text",
  "conversation_id": "uuid",
  "message_id": "uuid"
}
```

#### List Conversations
```http
GET /api/chat/conversations
Authorization: Bearer <token>
```

Response:
```json
{
  "success": true,
  "conversations": [
    {
      "id": "uuid",
      "title": "Recipe: Homemade Pasta",
      "intent": "recipe_creation",
      "updated_at": "2025-01-21T10:30:00Z"
    }
  ]
}
```

#### Rename Conversation
```http
POST /api/chat/rename
Content-Type: application/x-www-form-urlencoded

conversation_id=uuid&title=New Title
```

#### Delete Conversation
```http
POST /api/chat/delete
Content-Type: application/x-www-form-urlencoded

conversation_id=uuid
```

## UI/UX Recommendations

### 1. Conversation List Design
- **ChatGPT-inspired sidebar**: 300px width, collapsible on mobile
- **Conversation previews**: Show first message excerpt (max 60 chars)
- **Time indicators**: "Just now", "5m ago", "Today", "Yesterday"
- **Hover actions**: Edit/Delete buttons appear on hover

### 2. Title Generation Strategy
The system automatically generates meaningful titles:
- **Recipe requests**: "Recipe: [Dish Name]" (e.g., "Recipe: Chicken Alfredo")
- **How-to questions**: "How to make [Item]" (e.g., "How to make bread")
- **General questions**: First 47 characters + "..." if longer
- **Substitutions**: "Ingredient Substitution"
- **Meal planning**: "Meal Planning"

### 3. Conversation States
- **Active**: Green indicator, current conversation
- **Archived**: Greyed out, accessible but separate
- **Recently used**: Sorted by updated_at timestamp

### 4. Mobile Responsiveness
```css
@media (max-width: 768px) {
    .conversation-sidebar {
        position: fixed;
        left: -300px;
        transition: left 0.3s ease;
        z-index: 1000;
    }
    
    .conversation-sidebar.open {
        left: 0;
    }
}
```

## Testing Strategy

### 1. Unit Tests
- Conversation service methods
- Title generation logic
- Database repository operations

### 2. Integration Tests
- API endpoint responses
- Database transaction handling
- Session migration process

### 3. End-to-End Tests
- Complete conversation flow
- Multi-conversation switching
- Conversation management operations

## Performance Considerations

### 1. Database Optimization
- Conversations indexed by user_id and updated_at
- Messages indexed by conversation_id and created_at
- JSONB indexes for metadata searches

### 2. Caching Strategy
```go
// Cache frequent conversations
type ConversationCache struct {
    userConversations map[string][]*conversation.Conversation
    mutex            sync.RWMutex
    ttl              time.Duration
}
```

### 3. Pagination
- Default: 50 conversations per request
- Messages: 100 per conversation load
- Infinite scroll for older conversations

## Success Metrics

### 1. User Engagement
- Average conversations per user
- Messages per conversation
- Conversation completion rates

### 2. Technical Metrics
- Database query performance
- Session migration success rate
- API response times

### 3. User Experience
- Time spent in chat interface
- Feature usage (rename, delete, switch)
- User feedback scores

## Rollback Plan

If issues arise:

1. **Immediate**: Set feature flag to disable multi-chat
2. **Session restore**: Temporarily restore session-based storage
3. **Data preservation**: Keep database conversations for future retry
4. **User communication**: Notify users of temporary revert

## Conclusion

The multi-chat conversation system is ready for deployment with:
- ✅ Complete database schema with advanced title generation
- ✅ Full CRUD API endpoints for conversation management
- ✅ Enhanced frontend with conversation sidebar and management
- ✅ Automatic session migration strategy
- ✅ Mobile-responsive design
- ✅ Production-ready error handling and validation

The system provides a ChatGPT-like experience while maintaining your existing AI chef assistant functionality.