-- Drop foreign key constraint on area_id if it exists to allow 0 (no area)
ALTER TABLE sensors DROP CONSTRAINT IF EXISTS fk_sensors_area;

-- Update existing NULL area_id to 0
UPDATE sensors SET area_id = 0 WHERE area_id IS NULL;

-- Set default 0 and enforce NOT NULL
ALTER TABLE sensors ALTER COLUMN area_id SET DEFAULT 0;
ALTER TABLE sensors ALTER COLUMN area_id SET NOT NULL;

-- Ensure code column exists
ALTER TABLE sensors ADD COLUMN IF NOT EXISTS code VARCHAR(100);
