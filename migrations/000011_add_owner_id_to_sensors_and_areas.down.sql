DROP INDEX IF EXISTS idx_areas_owner_id;
ALTER TABLE areas DROP COLUMN IF EXISTS owner_id;

DROP INDEX IF EXISTS idx_sensors_owner_id;
ALTER TABLE sensors DROP COLUMN IF EXISTS owner_id;
