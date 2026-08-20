-- +goose Up
ALTER TABLE answers
ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
 
-- +goose Down
ALTER TABLE answers DROP COLUMN IF EXISTS created_at;
 