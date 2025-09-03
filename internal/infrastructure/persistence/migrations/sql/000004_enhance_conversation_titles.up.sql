-- Migration: Enhanced Conversation Title Generation (UP)
-- Description: Adds improved automatic title generation from message content
-- Author: System
-- Date: 2025-01-21

BEGIN;

-- Drop existing function and trigger to recreate with enhancements
DROP TRIGGER IF EXISTS generate_conversation_title_from_message_trigger ON messages;
DROP FUNCTION IF EXISTS generate_conversation_title_from_message();

-- Enhanced function to automatically generate conversation titles from first message
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
        SELECT COUNT(*) as count INTO conversation_record FROM messages 
        WHERE conversation_id = NEW.conversation_id AND role = 'user';
        
        -- If this is the first user message
        IF conversation_record.count = 1 THEN
            first_message := NEW.content;
            generated_title := '';
            
            -- Extract recipe/dish names from common patterns
            IF first_message ~* '\b(recipe\s+for|make\s+a?\s*recipe\s+for|cook|create|prepare)\s+(.+?)(\.|$|\s+please|\s+with|\?|\!)' THEN
                generated_title := regexp_replace(
                    first_message, 
                    '.*\b(?:recipe\s+for|make\s+a?\s*recipe\s+for|cook|create|prepare)\s+(.+?)(?:\.|$|\s+please|\s+with|\?|\!).*', 
                    'Recipe: \1', 
                    'i'
                );
                -- Clean up common words
                generated_title := regexp_replace(generated_title, '\b(a|an|some|the)\s+', '', 'gi');
                generated_title := regexp_replace(generated_title, '\s+', ' ', 'g');
                generated_title := trim(generated_title);
                generated_title := initcap(generated_title);
                
            ELSIF first_message ~* '\b(how\s+to\s+make|how\s+to\s+cook|how\s+do\s+i\s+make|how\s+do\s+i\s+cook)\s+(.+?)(\.|$|\s+\?|\?)' THEN
                generated_title := regexp_replace(
                    first_message,
                    '.*\b(?:how\s+to\s+make|how\s+to\s+cook|how\s+do\s+i\s+make|how\s+do\s+i\s+cook)\s+(.+?)(?:\.|$|\s+\?|\?).*',
                    'How to make \1',
                    'i'
                );
                generated_title := regexp_replace(generated_title, '\b(a|an|some|the)\s+', '', 'gi');
                generated_title := regexp_replace(generated_title, '\s+', ' ', 'g');
                generated_title := trim(generated_title);
                generated_title := initcap(generated_title);
                
            ELSIF first_message ~* '\b(substitute|replace|alternative|instead\s+of|swap)\b.*\b(for|with|in)\s+(.+?)(\.|$|\?)' THEN
                generated_title := 'Ingredient Substitution';
                
            ELSIF first_message ~* '\b(what\s+can\s+i\s+substitute|what\s+can\s+i\s+use\s+instead)\b' THEN
                generated_title := 'Ingredient Substitution';
                
            ELSIF first_message ~* '\bmeal\s+plan(ning)?\b|\bweek(ly)?\s+menu\b|\bmeal\s+prep\b' THEN
                generated_title := 'Meal Planning';
                
            ELSIF first_message ~* '\b(help|question|advice|tip|technique)\b.*\b(cooking|baking|kitchen)\b' THEN
                generated_title := 'Cooking Help';
                
            ELSIF first_message ~* '^\s*(what|how|why|when|where|can|should|would|could)' THEN
                -- For questions, use first meaningful part
                generated_title := CASE 
                    WHEN length(first_message) <= 50 THEN initcap(first_message)
                    ELSE initcap(substring(first_message from 1 for 47)) || '...'
                END;
                
            ELSE
                -- Extract first few meaningful words (max 50 chars)
                generated_title := CASE 
                    WHEN length(first_message) <= 50 THEN initcap(first_message)
                    ELSE initcap(substring(first_message from 1 for 47)) || '...'
                END;
            END IF;
            
            -- Final cleanup
            generated_title := regexp_replace(generated_title, '\s+', ' ', 'g');
            generated_title := trim(generated_title);
            
            -- Ensure title is not empty
            IF generated_title = '' OR generated_title IS NULL THEN
                generated_title := 'Chat with AI Chef';
            END IF;
            
            -- Update the conversation title if it's still null or default
            UPDATE conversations 
            SET title = generated_title, updated_at = NOW()
            WHERE id = NEW.conversation_id 
            AND (title IS NULL OR title IN ('New Recipe Creation', 'Cooking Help', 'Ingredient Help', 'Meal Planning', 'Chat with AI Chef', ''));
        END IF;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Recreate the trigger with improved function
CREATE TRIGGER generate_conversation_title_from_message_trigger
    AFTER INSERT ON messages
    FOR EACH ROW
    EXECUTE FUNCTION generate_conversation_title_from_message();

-- Add index for conversation title searches (if not exists)
CREATE INDEX IF NOT EXISTS idx_conversations_title_search 
ON conversations USING gin(to_tsvector('english', title)) 
WHERE deleted_at IS NULL;

-- Add index for intent-based queries
CREATE INDEX IF NOT EXISTS idx_conversations_user_intent 
ON conversations(user_id, intent, updated_at DESC) 
WHERE deleted_at IS NULL;

COMMIT;