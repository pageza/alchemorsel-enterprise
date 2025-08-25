-- Migration: Conversation Schema (DOWN)
-- Description: Removes conversation, message, and context tables
-- Author: System
-- Date: 2025-01-21

BEGIN;

-- Drop triggers
DROP TRIGGER IF EXISTS update_conversation_on_message ON messages;
DROP TRIGGER IF EXISTS update_conversation_context_updated_at ON conversation_context;
DROP TRIGGER IF EXISTS generate_conversation_title_trigger ON conversations;

-- Drop functions
DROP FUNCTION IF EXISTS update_conversation_updated_at();
DROP FUNCTION IF EXISTS generate_conversation_title();

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS message_attachments CASCADE;
DROP TABLE IF EXISTS conversation_participants CASCADE;
DROP TABLE IF EXISTS conversation_context CASCADE;
DROP TABLE IF EXISTS messages CASCADE;
DROP TABLE IF EXISTS conversations CASCADE;

-- Drop custom types
DROP TYPE IF EXISTS conversation_intent;
DROP TYPE IF EXISTS message_role;
DROP TYPE IF EXISTS conversation_status;

COMMIT;