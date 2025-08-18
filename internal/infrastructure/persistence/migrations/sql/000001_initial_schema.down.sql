-- Migration: Initial Schema (DOWN)
-- Description: Rollback for initial schema
-- Author: System
-- Date: 2024-01-15

BEGIN;

-- Drop triggers
DROP TRIGGER IF EXISTS update_recipe_ratings_metrics ON recipe_ratings;
DROP TRIGGER IF EXISTS update_recipe_likes_count ON recipe_likes;
DROP TRIGGER IF EXISTS update_collections_updated_at ON collections;
DROP TRIGGER IF EXISTS update_user_preferences_updated_at ON user_preferences;
DROP TRIGGER IF EXISTS update_recipe_ratings_updated_at ON recipe_ratings;
DROP TRIGGER IF EXISTS update_nutrition_info_updated_at ON nutrition_info;
DROP TRIGGER IF EXISTS update_instructions_updated_at ON instructions;
DROP TRIGGER IF EXISTS update_ingredients_updated_at ON ingredients;
DROP TRIGGER IF EXISTS update_recipes_updated_at ON recipes;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

-- Drop functions
DROP FUNCTION IF EXISTS update_recipe_metrics();
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS collection_recipes;
DROP TABLE IF EXISTS collections;
DROP TABLE IF EXISTS user_preferences;
DROP TABLE IF EXISTS user_follows;
DROP TABLE IF EXISTS recipe_views;
DROP TABLE IF EXISTS recipe_likes;
DROP TABLE IF EXISTS recipe_ratings;
DROP TABLE IF EXISTS recipe_images;
DROP TABLE IF EXISTS nutrition_info;
DROP TABLE IF EXISTS instructions;
DROP TABLE IF EXISTS ingredients;
DROP TABLE IF EXISTS recipe_tags;
DROP TABLE IF EXISTS recipes;
DROP TABLE IF EXISTS users;

-- Drop custom types
DROP TYPE IF EXISTS notification_type;
DROP TYPE IF EXISTS user_role;
DROP TYPE IF EXISTS temperature_unit;
DROP TYPE IF EXISTS measurement_unit;
DROP TYPE IF EXISTS difficulty_level;
DROP TYPE IF EXISTS category_type;
DROP TYPE IF EXISTS cuisine_type;
DROP TYPE IF EXISTS recipe_status;

-- Drop extensions if they're not needed elsewhere
-- Note: Be careful with this in production
-- DROP EXTENSION IF EXISTS "btree_gin";
-- DROP EXTENSION IF EXISTS "pg_trgm";
-- DROP EXTENSION IF EXISTS "uuid-ossp";

COMMIT;