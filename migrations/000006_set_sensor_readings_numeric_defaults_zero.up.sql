-- Update any NULL values to 0
UPDATE sensor_readings SET soil_moisture = 0 WHERE soil_moisture IS NULL;
UPDATE sensor_readings SET temperature = 0 WHERE temperature IS NULL;
UPDATE sensor_readings SET humidity = 0 WHERE humidity IS NULL;

-- Set default 0 and enforce NOT NULL on numeric columns
ALTER TABLE sensor_readings ALTER COLUMN soil_moisture SET DEFAULT 0;
ALTER TABLE sensor_readings ALTER COLUMN temperature SET DEFAULT 0;
ALTER TABLE sensor_readings ALTER COLUMN humidity SET DEFAULT 0;

ALTER TABLE sensor_readings ALTER COLUMN soil_moisture SET NOT NULL;
ALTER TABLE sensor_readings ALTER COLUMN temperature SET NOT NULL;
ALTER TABLE sensor_readings ALTER COLUMN humidity SET NOT NULL;
