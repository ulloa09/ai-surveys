-- +goose NO TRANSACTION

-- +goose Up

ALTER TYPE termination_mode ADD VALUE IF NOT EXISTS 'question_coverage';
ALTER TYPE termination_mode ADD VALUE IF NOT EXISTS 'time_estimate';
ALTER TYPE termination_mode ADD VALUE IF NOT EXISTS 'combination';

ALTER TABLE surveys
  ALTER COLUMN mode SET DEFAULT 'conversational',
  ALTER COLUMN termination_mode SET DEFAULT 'turn_limit',
  ALTER COLUMN turn_limit SET DEFAULT 12;

UPDATE surveys
SET termination_mode = 'turn_limit',
    turn_limit = COALESCE(turn_limit, 12)
WHERE termination_mode::TEXT IN ('manual', 'time_limit', 'response_cap');

UPDATE surveys
SET turn_limit = 12
WHERE termination_mode = 'turn_limit'
  AND turn_limit IS NULL;

ALTER TABLE surveys
  ADD CONSTRAINT surveys_termination_mode_supported
    CHECK (termination_mode::TEXT IN ('turn_limit', 'question_coverage', 'time_estimate', 'combination')),
  ADD CONSTRAINT surveys_turn_limit_positive
    CHECK (turn_limit IS NULL OR turn_limit > 0),
  ADD CONSTRAINT surveys_time_estimate_minutes_positive
    CHECK (time_estimate_minutes IS NULL OR time_estimate_minutes > 0),
  ADD CONSTRAINT surveys_turn_limit_required_for_termination
    CHECK (termination_mode::TEXT NOT IN ('turn_limit', 'combination') OR turn_limit IS NOT NULL),
  ADD CONSTRAINT surveys_time_estimate_required_for_termination
    CHECK (termination_mode::TEXT NOT IN ('time_estimate', 'combination') OR time_estimate_minutes IS NOT NULL);

-- +goose Down

ALTER TABLE surveys
  DROP CONSTRAINT IF EXISTS surveys_time_estimate_required_for_termination,
  DROP CONSTRAINT IF EXISTS surveys_turn_limit_required_for_termination,
  DROP CONSTRAINT IF EXISTS surveys_time_estimate_minutes_positive,
  DROP CONSTRAINT IF EXISTS surveys_turn_limit_positive,
  DROP CONSTRAINT IF EXISTS surveys_termination_mode_supported,
  ALTER COLUMN mode SET DEFAULT 'form',
  ALTER COLUMN termination_mode SET DEFAULT 'manual',
  ALTER COLUMN turn_limit DROP DEFAULT;

-- NOTE: PostgreSQL does not support removing enum values once added.
