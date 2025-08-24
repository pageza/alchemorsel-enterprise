-- 000003_seed_demo_users.up.sql
-- Seeds demo users for development and testing

-- Insert demo chef user
INSERT INTO users (id, email, name, password_hash, role, is_active, created_at, updated_at)
VALUES (
    gen_random_uuid(),
    'chef@alchemorsel.com',
    'Demo Chef',
    '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', -- bcrypt hash for 'password'
    'chef',
    true,
    NOW(),
    NOW()
) ON CONFLICT (email) DO NOTHING;

-- Insert demo regular user  
INSERT INTO users (id, email, name, password_hash, role, is_active, created_at, updated_at)
VALUES (
    gen_random_uuid(),
    'user@alchemorsel.com', 
    'Demo User',
    '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', -- bcrypt hash for 'password'
    'user',
    true,
    NOW(),
    NOW()
) ON CONFLICT (email) DO NOTHING;

-- Insert demo admin user for testing admin features
INSERT INTO users (id, email, name, password_hash, role, is_active, created_at, updated_at)
VALUES (
    gen_random_uuid(),
    'admin@alchemorsel.com',
    'Demo Admin',
    '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', -- bcrypt hash for 'password'
    'admin',
    true,
    NOW(),
    NOW()
) ON CONFLICT (email) DO NOTHING;

-- Add some sample recipes for the chef user (for demo purposes)
WITH chef_user AS (
    SELECT id FROM users WHERE email = 'chef@alchemorsel.com' LIMIT 1
)
INSERT INTO recipes (id, title, description, author_id, cuisine, difficulty, prep_time_minutes, cook_time_minutes, servings, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    'Classic Margherita Pizza',
    'A simple yet delicious pizza with fresh tomatoes, mozzarella, and basil. Perfect for beginners!',
    chef_user.id,
    'Italian',
    'Easy',
    20,
    15,
    4,
    NOW(),
    NOW()
FROM chef_user
ON CONFLICT DO NOTHING;

WITH chef_user AS (
    SELECT id FROM users WHERE email = 'chef@alchemorsel.com' LIMIT 1
)
INSERT INTO recipes (id, title, description, author_id, cuisine, difficulty, prep_time_minutes, cook_time_minutes, servings, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    'Chocolate Chip Cookies',
    'Crispy on the outside, chewy on the inside. The perfect chocolate chip cookie recipe.',
    chef_user.id,
    'American',
    'Easy',
    15,
    12,
    24,
    NOW(),
    NOW()
FROM chef_user
ON CONFLICT DO NOTHING;

-- Log the seeding
DO $$
BEGIN
    RAISE NOTICE 'Demo users seeded successfully:';
    RAISE NOTICE '- chef@alchemorsel.com / password (Chef role)';
    RAISE NOTICE '- user@alchemorsel.com / password (User role)';  
    RAISE NOTICE '- admin@alchemorsel.com / password (Admin role)';
    RAISE NOTICE '- Added 2 sample recipes for demo chef';
END $$;