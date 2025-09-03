-- Migration: Enhanced Conversation Title Generation (DOWN)
-- Description: Reverts enhanced title generation improvements
-- Author: System
-- Date: 2025-01-21

BEGIN;

-- Drop enhanced triggers and functions
DROP TRIGGER IF EXISTS generate_conversation_title_from_message_trigger ON messages;
DROP FUNCTION IF EXISTS generate_conversation_title_from_message();

-- Drop added indexes
DROP INDEX IF EXISTS idx_conversations_title_search;
DROP INDEX IF EXISTS idx_conversations_user_intent;

-- Restore original simple title generation function (from migration 000002)
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

-- Recreate original trigger
CREATE TRIGGER generate_conversation_title_from_message_trigger
    AFTER INSERT ON messages
    FOR EACH ROW
    EXECUTE FUNCTION generate_conversation_title_from_message();

COMMIT;