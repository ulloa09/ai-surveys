-- +goose Up

ALTER TABLE answers ADD CONSTRAINT answers_response_question_unique
UNIQUE (response_id, question_id);

-- Extender el enum response_status con los nuevos estados del engine.
-- PostgreSQL no permite eliminar valores de un enum, solo agregar.
-- El estado 'abandoned' se mantiene por compatibilidad con datos existentes.
ALTER TYPE response_status ADD VALUE IF NOT EXISTS 'not_started';
ALTER TYPE response_status ADD VALUE IF NOT EXISTS 'pending_submission';
ALTER TYPE response_status ADD VALUE IF NOT EXISTS 'analysing';
ALTER TYPE response_status ADD VALUE IF NOT EXISTS 'complete';

-- Índices para el engine: búsqueda de respuestas por encuesta y estado.
CREATE INDEX IF NOT EXISTS idx_responses_survey_status
    ON responses (survey_id, status);

CREATE INDEX IF NOT EXISTS idx_responses_status
    ON responses (status);

-- Índice en answers para verificar cobertura de preguntas rápidamente.
CREATE INDEX IF NOT EXISTS idx_answers_response
    ON answers (response_id);

CREATE INDEX IF NOT EXISTS idx_answers_question
    ON answers (response_id, question_id);

ALTER TABLE responses ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- +goose Down

DROP INDEX IF EXISTS idx_answers_question;
DROP INDEX IF EXISTS idx_answers_response;
DROP INDEX IF EXISTS idx_responses_status;
DROP INDEX IF EXISTS idx_responses_survey_status;
ALTER TABLE responses DROP COLUMN IF EXISTS updated_at;