-- 000003_seed_demo_users.down.sql  
-- Removes demo users and their associated data

-- Remove recipes created by demo users
DELETE FROM recipes WHERE author_id IN (
    SELECT id FROM users WHERE email IN (
        'chef@alchemorsel.com',
        'user@alchemorsel.com', 
        'admin@alchemorsel.com'
    )
);

-- Remove demo users
DELETE FROM users WHERE email IN (
    'chef@alchemorsel.com',
    'user@alchemorsel.com',
    'admin@alchemorsel.com'
);

-- Log the removal
DO $$
BEGIN
    RAISE NOTICE 'Demo users and their data removed successfully';
END $$;