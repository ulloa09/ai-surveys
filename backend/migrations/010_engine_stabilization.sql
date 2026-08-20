-- +goose Up

-- Orden estable de turnos: created_at puede empatar cuando un intercambio
-- (user + assistant) se inserta en la misma transacción. seq garantiza el
-- orden de inserción real al reconstruir el transcript.
ALTER TABLE turns ADD COLUMN IF NOT EXISTS seq BIGSERIAL;

CREATE INDEX IF NOT EXISTS idx_turns_response_seq ON turns (response_id, seq);

-- Razón de cierre de una respuesta cuando no termina por submit normal
-- (p. ej. 'low_quality_answers' cuando el respondiente manda basura repetida).
ALTER TABLE responses ADD COLUMN IF NOT EXISTS closed_reason TEXT;

-- Seguimiento del followup de recuperación para respuestas basura:
-- cuántos intentos inválidos lleva y sobre qué pregunta fue el último.
ALTER TABLE responses ADD COLUMN IF NOT EXISTS low_quality_attempts INT NOT NULL DEFAULT 0;
ALTER TABLE responses ADD COLUMN IF NOT EXISTS low_quality_question_id UUID;

-- +goose Down

ALTER TABLE responses DROP COLUMN IF EXISTS low_quality_question_id;
ALTER TABLE responses DROP COLUMN IF EXISTS low_quality_attempts;
ALTER TABLE responses DROP COLUMN IF EXISTS closed_reason;
DROP INDEX IF EXISTS idx_turns_response_seq;
ALTER TABLE turns DROP COLUMN IF EXISTS seq;
