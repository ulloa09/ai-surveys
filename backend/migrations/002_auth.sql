-- +goose NO TRANSACTION

-- +goose Up

-- ALTER TYPE ... ADD VALUE cannot run inside a transaction in PostgreSQL.
ALTER TYPE user_role ADD VALUE IF NOT EXISTS 'super_admin' BEFORE 'admin';

ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;

CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT        PRIMARY KEY,
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id    ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- +goose Down

DROP TABLE IF EXISTS sessions;

ALTER TABLE users DROP COLUMN IF EXISTS password_hash;

-- NOTE: PostgreSQL does not support removing enum values once added.
-- super_admin remains in user_role after rolling back this migration.
