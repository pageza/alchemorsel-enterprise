-- Migration: Initial Schema (UP)
-- Description: Creates the initial database schema for Alchemorsel v3
-- Author: System
-- Date: 2024-01-15

BEGIN;

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
CREATE EXTENSION IF NOT EXISTS "btree_gin";

-- Create custom types
CREATE TYPE recipe_status AS ENUM ('draft', 'published', 'archived', 'deleted');
CREATE TYPE cuisine_type AS ENUM ('italian', 'french', 'chinese', 'japanese', 'indian', 'mexican', 'american', 'mediterranean', 'thai', 'other');
CREATE TYPE category_type AS ENUM ('appetizer', 'main_course', 'side_dish', 'dessert', 'beverage', 'breakfast', 'lunch', 'dinner', 'snack');
CREATE TYPE difficulty_level AS ENUM ('easy', 'medium', 'hard', 'expert');
CREATE TYPE measurement_unit AS ENUM ('tsp', 'tbsp', 'cup', 'oz', 'ml', 'l', 'g', 'kg', 'lb', 'piece', 'dash', 'pinch');
CREATE TYPE temperature_unit AS ENUM ('C', 'F', 'K');
CREATE TYPE user_role AS ENUM ('user', 'premium', 'moderator', 'admin');
CREATE TYPE notification_type AS ENUM ('recipe_liked', 'recipe_commented', 'new_follower', 'recipe_published');

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(100),
    bio TEXT,
    avatar_url VARCHAR(500),
    role user_role DEFAULT 'user',
    is_verified BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    email_verified_at TIMESTAMPTZ,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    
    -- Indexes
    CONSTRAINT users_email_check CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}$')
);

CREATE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_username ON users(username) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_created_at ON users(created_at DESC);
CREATE INDEX idx_users_role ON users(role) WHERE deleted_at IS NULL;

-- Recipes table
CREATE TABLE recipes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    version BIGINT NOT NULL DEFAULT 1,
    title VARCHAR(200) NOT NULL,
    description TEXT,
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cuisine cuisine_type,
    category category_type,
    difficulty difficulty_level,
    prep_time_minutes INTEGER,
    cook_time_minutes INTEGER,
    total_time_minutes INTEGER GENERATED ALWAYS AS (prep_time_minutes + cook_time_minutes) STORED,
    servings INTEGER CHECK (servings > 0),
    calories INTEGER CHECK (calories >= 0),
    status recipe_status DEFAULT 'draft',
    
    -- AI fields
    ai_generated BOOLEAN DEFAULT false,
    ai_prompt TEXT,
    ai_model VARCHAR(50),
    
    -- Social metrics
    likes_count INTEGER DEFAULT 0,
    views_count INTEGER DEFAULT 0,
    comments_count INTEGER DEFAULT 0,
    average_rating DECIMAL(2,1) DEFAULT 0.0 CHECK (average_rating >= 0 AND average_rating <= 5),
    ratings_count INTEGER DEFAULT 0,
    
    -- Timestamps
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    
    -- Constraints
    CONSTRAINT recipes_title_length CHECK (LENGTH(title) >= 3 AND LENGTH(title) <= 200)
);

-- Indexes for recipes
CREATE INDEX idx_recipes_author ON recipes(author_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_recipes_status ON recipes(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_recipes_cuisine ON recipes(cuisine) WHERE deleted_at IS NULL;
CREATE INDEX idx_recipes_category ON recipes(category) WHERE deleted_at IS NULL;
CREATE INDEX idx_recipes_difficulty ON recipes(difficulty) WHERE deleted_at IS NULL;
CREATE INDEX idx_recipes_published_at ON recipes(published_at DESC) WHERE status = 'published' AND deleted_at IS NULL;
CREATE INDEX idx_recipes_likes ON recipes(likes_count DESC) WHERE status = 'published' AND deleted_at IS NULL;
CREATE INDEX idx_recipes_rating ON recipes(average_rating DESC, ratings_count DESC) WHERE status = 'published' AND deleted_at IS NULL;
CREATE INDEX idx_recipes_search ON recipes USING gin(to_tsvector('english', title || ' ' || COALESCE(description, ''))) WHERE deleted_at IS NULL;

-- Recipe tags
CREATE TABLE recipe_tags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    tag VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(recipe_id, tag)
);

CREATE INDEX idx_recipe_tags_recipe ON recipe_tags(recipe_id);
CREATE INDEX idx_recipe_tags_tag ON recipe_tags(tag);

-- Ingredients table
CREATE TABLE ingredients (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    amount DECIMAL(10,2),
    unit measurement_unit,
    optional BOOLEAN DEFAULT false,
    notes TEXT,
    order_index INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ingredients_recipe ON ingredients(recipe_id);
CREATE INDEX idx_ingredients_name ON ingredients USING gin(to_tsvector('english', name));

-- Instructions table
CREATE TABLE instructions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    step_number INTEGER NOT NULL,
    description TEXT NOT NULL,
    duration_minutes INTEGER,
    temperature_value DECIMAL(5,1),
    temperature_unit temperature_unit,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(recipe_id, step_number)
);

CREATE INDEX idx_instructions_recipe ON instructions(recipe_id);

-- Nutrition information
CREATE TABLE nutrition_info (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    recipe_id UUID UNIQUE NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    calories INTEGER,
    protein_g DECIMAL(6,2),
    carbohydrates_g DECIMAL(6,2),
    fat_g DECIMAL(6,2),
    fiber_g DECIMAL(6,2),
    sugar_g DECIMAL(6,2),
    sodium_mg DECIMAL(8,2),
    cholesterol_mg DECIMAL(6,2),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Recipe images
CREATE TABLE recipe_images (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    url VARCHAR(500) NOT NULL,
    thumbnail_url VARCHAR(500),
    caption TEXT,
    is_primary BOOLEAN DEFAULT false,
    order_index INTEGER,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_recipe_images_recipe ON recipe_images(recipe_id);

-- Recipe ratings
CREATE TABLE recipe_ratings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating INTEGER NOT NULL CHECK (rating >= 1 AND rating <= 5),
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(recipe_id, user_id)
);

CREATE INDEX idx_recipe_ratings_recipe ON recipe_ratings(recipe_id);
CREATE INDEX idx_recipe_ratings_user ON recipe_ratings(user_id);
CREATE INDEX idx_recipe_ratings_rating ON recipe_ratings(rating);

-- Recipe likes
CREATE TABLE recipe_likes (
    recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    PRIMARY KEY(recipe_id, user_id)
);

CREATE INDEX idx_recipe_likes_user ON recipe_likes(user_id);
CREATE INDEX idx_recipe_likes_created ON recipe_likes(created_at DESC);

-- Recipe views
CREATE TABLE recipe_views (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    ip_address INET,
    user_agent TEXT,
    viewed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_recipe_views_recipe ON recipe_views(recipe_id);
CREATE INDEX idx_recipe_views_user ON recipe_views(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_recipe_views_date ON recipe_views(viewed_at DESC);

-- User follows
CREATE TABLE user_follows (
    follower_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    following_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    PRIMARY KEY(follower_id, following_id),
    CHECK(follower_id != following_id)
);

CREATE INDEX idx_user_follows_following ON user_follows(following_id);
CREATE INDEX idx_user_follows_created ON user_follows(created_at DESC);

-- User preferences
CREATE TABLE user_preferences (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    dietary_restrictions TEXT[],
    favorite_cuisines cuisine_type[],
    skill_level difficulty_level,
    email_notifications BOOLEAN DEFAULT true,
    push_notifications BOOLEAN DEFAULT true,
    newsletter_subscription BOOLEAN DEFAULT true,
    privacy_level VARCHAR(20) DEFAULT 'public',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Collections (recipe lists)
CREATE TABLE collections (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    is_public BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_collections_user ON collections(user_id);
CREATE INDEX idx_collections_public ON collections(is_public) WHERE is_public = true;

-- Collection recipes
CREATE TABLE collection_recipes (
    collection_id UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    order_index INTEGER,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    PRIMARY KEY(collection_id, recipe_id)
);

CREATE INDEX idx_collection_recipes_recipe ON collection_recipes(recipe_id);

-- Notifications
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type notification_type NOT NULL,
    title VARCHAR(200) NOT NULL,
    message TEXT,
    data JSONB,
    is_read BOOLEAN DEFAULT false,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user_unread ON notifications(user_id, created_at DESC) WHERE is_read = false;
CREATE INDEX idx_notifications_created ON notifications(created_at DESC);

-- Audit log
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50),
    entity_id UUID,
    old_values JSONB,
    new_values JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_user ON audit_logs(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id) WHERE entity_id IS NOT NULL;
CREATE INDEX idx_audit_logs_created ON audit_logs(created_at DESC);

-- Functions for updating timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Triggers for updating timestamps
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_recipes_updated_at BEFORE UPDATE ON recipes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_ingredients_updated_at BEFORE UPDATE ON ingredients
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_instructions_updated_at BEFORE UPDATE ON instructions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_nutrition_info_updated_at BEFORE UPDATE ON nutrition_info
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_recipe_ratings_updated_at BEFORE UPDATE ON recipe_ratings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_preferences_updated_at BEFORE UPDATE ON user_preferences
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_collections_updated_at BEFORE UPDATE ON collections
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Function to update recipe metrics
CREATE OR REPLACE FUNCTION update_recipe_metrics()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_TABLE_NAME = 'recipe_likes' THEN
        IF TG_OP = 'INSERT' THEN
            UPDATE recipes SET likes_count = likes_count + 1 WHERE id = NEW.recipe_id;
        ELSIF TG_OP = 'DELETE' THEN
            UPDATE recipes SET likes_count = likes_count - 1 WHERE id = OLD.recipe_id;
        END IF;
    ELSIF TG_TABLE_NAME = 'recipe_ratings' THEN
        UPDATE recipes SET 
            average_rating = (SELECT AVG(rating) FROM recipe_ratings WHERE recipe_id = COALESCE(NEW.recipe_id, OLD.recipe_id)),
            ratings_count = (SELECT COUNT(*) FROM recipe_ratings WHERE recipe_id = COALESCE(NEW.recipe_id, OLD.recipe_id))
        WHERE id = COALESCE(NEW.recipe_id, OLD.recipe_id);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Triggers for updating recipe metrics
CREATE TRIGGER update_recipe_likes_count AFTER INSERT OR DELETE ON recipe_likes
    FOR EACH ROW EXECUTE FUNCTION update_recipe_metrics();

CREATE TRIGGER update_recipe_ratings_metrics AFTER INSERT OR UPDATE OR DELETE ON recipe_ratings
    FOR EACH ROW EXECUTE FUNCTION update_recipe_metrics();

COMMIT;