# Chat-Centric Interface Integration Plan

## Components Created

### 1. Database Schema (✅ Complete)
- **Location**: `internal/infrastructure/persistence/migrations/sql/000002_conversation_schema.up.sql`
- **Features**: Conversations, messages, context tables with proper indexing

### 2. WebSocket Infrastructure (✅ Complete)
- **Location**: `internal/infrastructure/websocket/manager.go`
- **Features**: Connection management, message routing, heartbeat

### 3. Conversation Service (✅ Complete)
- **Location**: `internal/application/conversation/service.go`
- **Features**: Intent classification, conversation management, context tracking

### 4. AI Service Integration (✅ Complete)
- **Location**: `internal/application/conversation/ai_service.go`
- **Features**: Conversational AI, recipe extraction, multi-turn dialogue

### 5. Repository Implementation (✅ Complete)
- **Location**: `internal/infrastructure/persistence/gorm/conversation_repository.go`
- **Features**: GORM-based data persistence for conversations

### 6. WebSocket Handler (✅ Complete)
- **Location**: `internal/infrastructure/http/handlers/websocket.go`
- **Features**: WebSocket upgrade, message processing

### 7. HTTP Chat Handler (✅ Complete)
- **Location**: `internal/infrastructure/http/handlers/chat.go`
- **Features**: REST API endpoints, HTMX integration

### 8. AI Adapter (✅ Complete)
- **Location**: `internal/infrastructure/ai/conversation_adapter.go`
- **Features**: Ollama/OpenAI integration for conversations

### 9. Chat UI Template (✅ Complete)
- **Location**: `internal/infrastructure/http/server/templates/pages/chat.html`
- **Features**: Full-page conversational interface with WebSocket support

## Integration Steps

### Step 1: Update main.go Dependencies
1. Add new imports for conversation services
2. Initialize conversation repositories
3. Create AI service instances
4. Initialize WebSocket manager
5. Create conversation service with dependencies

### Step 2: Add Routes
1. Add `/chat` route for chat page
2. Add `/ws/chat` route for WebSocket connections
3. Add HTMX endpoints for chat functionality
4. Add conversation management endpoints

### Step 3: Update Templates
1. Ensure chat.html template is properly integrated
2. Update template rendering in main.go
3. Add CSS for chat interface

### Step 4: Database Migration
1. Run migration to create conversation tables
2. Update auto-migration in main.go

### Step 5: Testing
1. Test WebSocket connections
2. Test conversation creation and management
3. Test AI integration
4. Test UI functionality

## Files to Modify

### main.go - Major updates needed:
- Add imports for new services
- Initialize WebSocket manager
- Initialize conversation services
- Add new routes
- Update template rendering

### Go Modules
- Ensure gorilla/websocket dependency is available
- No new external dependencies needed

## Environment Variables
- ALCHEMORSEL_OLLAMA_HOST (default: http://localhost:11434)
- ALCHEMORSEL_OLLAMA_MODEL (default: llama3.2:3b)
- OPENAI_API_KEY (optional, for fallback)

## Testing Checklist
- [ ] Database migration runs successfully
- [ ] WebSocket connections establish correctly
- [ ] Chat interface loads and functions
- [ ] Messages send and receive via WebSocket
- [ ] AI responses generate correctly
- [ ] Conversation history persists
- [ ] Intent classification works
- [ ] Recipe creation workflow functions
- [ ] Fallback to HTTP works when WebSocket unavailable
- [ ] UI is responsive and accessible

## Post-Integration Tasks
1. Performance testing with multiple concurrent connections
2. Security review of WebSocket implementation
3. Rate limiting for AI requests
4. Logging and monitoring setup
5. Error handling improvements
6. UI/UX refinements based on testing