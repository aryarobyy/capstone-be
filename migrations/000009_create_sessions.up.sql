CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id BIGINT REFERENCES device_tokens(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON sessions(user_id);
CREATE INDEX IF NOT EXISTS sessions_device_id_idx ON sessions(device_id);
CREATE INDEX IF NOT EXISTS sessions_expires_at_idx ON sessions(expires_at) WHERE revoked_at IS NULL;
