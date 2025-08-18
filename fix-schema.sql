-- Add missing columns to match Go structs
ALTER TABLE users ADD COLUMN IF NOT EXISTS name text;
ALTER TABLE recipes ADD COLUMN IF NOT EXISTS prep_time integer;

-- Update existing users with name if they don't have one
UPDATE users SET name = 'Demo User' WHERE name IS NULL;