-- Migration: Conversation Schema (UP)
-- Description: Creates conversation, message, and context tables for chat functionality
-- Author: System
-- Date: 2025-01-21

BEGIN;

-- Create conversation status enum
CREATE TYPE conversation_status AS ENUM ('active', 'archived', 'deleted');

-- Create message role enum  
CREATE TYPE message_role AS ENUM ('user', 'assistant', 'system');

-- Create conversation intent enum
CREATE TYPE conversation_intent AS ENUM ('recipe_creation', 'cooking_help', 'ingredient_substitution', 'meal_planning', 'general_question');

-- Conversations table
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255),
    intent conversation_intent,
    status conversation_status DEFAULT 'active',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

-- Add indexes for conversations
CREATE INDEX idx_conversations_user ON conversations(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_conversations_status ON conversations(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_conversations_intent ON conversations(intent) WHERE deleted_at IS NULL;
CREATE INDEX idx_conversations_updated ON conversations(updated_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_conversations_created ON conversations(created_at DESC) WHERE deleted_at IS NULL;

-- Messages table
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role message_role NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    tokens_used INTEGER DEFAULT 0,
    processing_time_ms INTEGER DEFAULT 0,
    model_used VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT messages_content_not_empty CHECK (LENGTH(TRIM(content)) > 0)
);

-- Add indexes for messages
CREATE INDEX idx_messages_conversation ON messages(conversation_id);
CREATE INDEX idx_messages_role ON messages(role);
CREATE INDEX idx_messages_created ON messages(created_at DESC);
CREATE INDEX idx_messages_conversation_created ON messages(conversation_id, created_at DESC);

-- Conversation context table for storing workflow state and extracted data
CREATE TABLE conversation_context (
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    context_type VARCHAR(50) NOT NULL,
    context_data JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    PRIMARY KEY (conversation_id, context_type)
);

-- Add indexes for conversation context
CREATE INDEX idx_conversation_context_type ON conversation_context(context_type);
CREATE INDEX idx_conversation_context_updated ON conversation_context(updated_at DESC);

-- Message attachments table for handling file uploads in chat
CREATE TABLE message_attachments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    file_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_size BIGINT NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add indexes for message attachments
CREATE INDEX idx_message_attachments_message ON message_attachments(message_id);
CREATE INDEX idx_message_attachments_uploaded ON message_attachments(uploaded_at DESC);

-- Conversation participants table (for future group chat functionality)
CREATE TABLE conversation_participants (
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) DEFAULT 'participant',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at TIMESTAMPTZ,
    
    PRIMARY KEY (conversation_id, user_id)
);

-- Add indexes for conversation participants
CREATE INDEX idx_conversation_participants_user ON conversation_participants(user_id);
CREATE INDEX idx_conversation_participants_joined ON conversation_participants(joined_at DESC);

-- Update conversation updated_at when messages are added
CREATE OR REPLACE FUNCTION update_conversation_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE conversations 
    SET updated_at = NOW() 
    WHERE id = NEW.conversation_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to update conversation timestamp on new messages
CREATE TRIGGER update_conversation_on_message 
    AFTER INSERT ON messages
    FOR EACH ROW 
    EXECUTE FUNCTION update_conversation_updated_at();

-- Trigger for updating conversation context timestamps
CREATE TRIGGER update_conversation_context_updated_at 
    BEFORE UPDATE ON conversation_context
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Function to automatically generate conversation titles from first message
CREATE OR REPLACE FUNCTION generate_conversation_title_from_message()
RETURNS TRIGGER AS $$
DECLARE
    first_message TEXT;
    conversation_record RECORD;
    generated_title TEXT;
BEGIN
    -- Only process user messages and if it's the first message in the conversation
    IF NEW.role = 'user' THEN
        -- Check if this is the first user message in the conversation
        SELECT COUNT(*) INTO conversation_record FROM messages 
        WHERE conversation_id = NEW.conversation_id AND role = 'user';
        
        -- If this is the first user message
        IF conversation_record.count = 1 THEN
            first_message := NEW.content;
            generated_title := '';
            
            -- Extract recipe/dish names from common patterns
            IF first_message ~* '\b(recipe\s+for|make|cook|create|prepare)\s+(.+?)(\.|$|\s+please|\s+with)' THEN
                generated_title := regexp_replace(
                    first_message, 
                    '.*\b(?:recipe\s+for|make|cook|create|prepare)\s+(.+?)(?:\.|$|\s+please|\s+with).*', 
                    'Recipe: \1', 
                    'i'
                );
            ELSIF first_message ~* '\b(how\s+to\s+make|how\s+to\s+cook)\s+(.+?)(\.|$|\s+\?)' THEN
                generated_title := regexp_replace(
                    first_message,
                    '.*\b(?:how\s+to\s+make|how\s+to\s+cook)\s+(.+?)(?:\.|$|\s+\?).*',
                    'How to make \1',
                    'i'
                );
            ELSIF first_message ~* '\b(substitute|replace|alternative)\b.*\b(for|instead of)\s+(.+?)(\.|$|\?)' THEN
                generated_title := 'Ingredient Substitution';
            ELSIF first_message ~* '\bmeal\s+plan' THEN
                generated_title := 'Meal Planning';
            ELSIF first_message ~* '\b(help|question|how)\b' THEN
                generated_title := 'Cooking Help';
            ELSE
                -- Extract first few meaningful words (max 50 chars)
                generated_title := CASE 
                    WHEN length(first_message) <= 50 THEN initcap(first_message)
                    ELSE initcap(substring(first_message from 1 for 47)) || '...'
                END;
            END IF;
            
            -- Clean up the title
            generated_title := regexp_replace(generated_title, '\s+', ' ', 'g');
            generated_title := trim(generated_title);
            
            -- Update the conversation title if it's still null or default
            UPDATE conversations 
            SET title = generated_title, updated_at = NOW()
            WHERE id = NEW.conversation_id 
            AND (title IS NULL OR title IN ('New Recipe Creation', 'Cooking Help', 'Ingredient Help', 'Meal Planning', 'Chat with AI Chef'));
        END IF;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to automatically generate conversation titles
CREATE OR REPLACE FUNCTION generate_conversation_title()
RETURNS TRIGGER AS $$
BEGIN
    -- Only generate title if it's null and we have the first user message
    IF NEW.title IS NULL AND NEW.intent IS NOT NULL THEN
        CASE NEW.intent
            WHEN 'recipe_creation' THEN
                NEW.title := 'New Recipe Creation';
            WHEN 'cooking_help' THEN 
                NEW.title := 'Cooking Help';
            WHEN 'ingredient_substitution' THEN
                NEW.title := 'Ingredient Help';
            WHEN 'meal_planning' THEN
                NEW.title := 'Meal Planning';
            ELSE
                NEW.title := 'Chat with AI Chef';
        END CASE;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to generate conversation titles
CREATE TRIGGER generate_conversation_title_trigger
    BEFORE INSERT ON conversations
    FOR EACH ROW
    EXECUTE FUNCTION generate_conversation_title();

-- Trigger to update conversation titles based on first message
CREATE TRIGGER generate_conversation_title_from_message_trigger
    AFTER INSERT ON messages
    FOR EACH ROW
    EXECUTE FUNCTION generate_conversation_title_from_message();

COMMIT;