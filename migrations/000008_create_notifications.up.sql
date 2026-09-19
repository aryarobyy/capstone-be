CREATE TABLE device_tokens (
 id BIGSERIAL PRIMARY KEY,
 user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 token TEXT NOT NULL UNIQUE,
 device_type TEXT NOT NULL CHECK (device_type IN ('android', 'ios', 'web')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX device_tokens_user_id_idx ON device_tokens(user_id);
CREATE TABLE notifications (
 id BIGSERIAL PRIMARY KEY,
 user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 title TEXT NOT NULL,
 body TEXT NOT NULL,
 data JSONB NOT NULL DEFAULT '{}',
 firebase_msg_id TEXT,
 is_read BOOLEAN NOT NULL DEFAULT FALSE,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX notifications_user_id_id_idx ON notifications(user_id, id DESC);
