ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE areas ADD COLUMN IF NOT EXISTS owner_id BIGINT REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS areas_owner_id_idx ON areas(owner_id);
ALTER TABLE sensors ADD COLUMN IF NOT EXISTS api_key_hash BYTEA;
CREATE UNIQUE INDEX IF NOT EXISTS sensors_api_key_hash_idx ON sensors(api_key_hash) WHERE api_key_hash IS NOT NULL;
ALTER TABLE device_tokens ADD COLUMN IF NOT EXISTS installation_id UUID;
CREATE UNIQUE INDEX IF NOT EXISTS device_tokens_installation_idx ON device_tokens(installation_id) WHERE installation_id IS NOT NULL;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS refresh_hash BYTEA;
CREATE UNIQUE INDEX IF NOT EXISTS sessions_refresh_hash_idx ON sessions(refresh_hash) WHERE refresh_hash IS NOT NULL;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS image_url TEXT NOT NULL DEFAULT '';
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS event_key TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS notifications_event_key_idx ON notifications(user_id,event_key) WHERE event_key IS NOT NULL;
CREATE TABLE IF NOT EXISTS notification_deliveries (
 id BIGSERIAL PRIMARY KEY,
 notification_id BIGINT NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
 device_id BIGINT REFERENCES device_tokens(id) ON DELETE SET NULL,
 status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','processing','retry','sent','failed','cancelled')),
 attempts INTEGER NOT NULL DEFAULT 0,
 next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 lease_id UUID,
 lease_until TIMESTAMPTZ,
 last_error TEXT,
 firebase_message_id TEXT,
 sent_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(notification_id,device_id)
);
CREATE INDEX IF NOT EXISTS notification_deliveries_ready_idx ON notification_deliveries(next_attempt_at) WHERE status IN ('pending','retry','processing');
CREATE TABLE IF NOT EXISTS alert_rules (
 sensor_id BIGINT NOT NULL REFERENCES sensors(id) ON DELETE CASCADE,
 parameter TEXT NOT NULL CHECK(parameter IN ('temperature','humidity','soil_moisture')),
 min_value DOUBLE PRECISION,
 max_value DOUBLE PRECISION,
 duration_seconds INTEGER NOT NULL DEFAULT 0 CHECK(duration_seconds BETWEEN 0 AND 86400),
 cooldown_seconds INTEGER NOT NULL DEFAULT 900 CHECK(cooldown_seconds BETWEEN 60 AND 604800),
 max_gap_seconds INTEGER NOT NULL DEFAULT 300 CHECK(max_gap_seconds BETWEEN 1 AND 86400),
 enabled BOOLEAN NOT NULL DEFAULT TRUE,
 PRIMARY KEY(sensor_id,parameter),
 CHECK(min_value IS NOT NULL OR max_value IS NOT NULL),
 CHECK(min_value IS NULL OR max_value IS NULL OR min_value < max_value)
);
CREATE TABLE IF NOT EXISTS alert_states (
 sensor_id BIGINT NOT NULL,
 parameter TEXT NOT NULL,
 started_at TIMESTAMPTZ,
 last_recorded_at TIMESTAMPTZ NOT NULL,
 last_notified_at TIMESTAMPTZ,
 anomaly_id BIGINT REFERENCES anomalies(id) ON DELETE SET NULL,
 PRIMARY KEY(sensor_id,parameter),
 FOREIGN KEY(sensor_id,parameter) REFERENCES alert_rules(sensor_id,parameter) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS sensor_readings_sensor_recorded_idx ON sensor_readings(sensor_id,recorded_at);
