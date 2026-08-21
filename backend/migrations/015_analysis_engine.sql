-- +goose Up

-- Extender el enum survey_status con los estados del Analysis Engine (#15).
-- PostgreSQL no permite eliminar valores de un enum, solo agregar.
-- No se usan estos valores en esta misma migración, así que no hace falta
-- correr fuera de transacción (mismo criterio que 008_survey_engine.sql).
ALTER TYPE survey_status ADD VALUE IF NOT EXISTS 'analysing';
ALTER TYPE survey_status ADD VALUE IF NOT EXISTS 'complete';

-- Constraint de unicidad para que el job de análisis pueda hacer upsert
-- (ON CONFLICT) por pregunta y así ser idempotente al re-ejecutarse.
ALTER TABLE analysis_results
    ADD CONSTRAINT analysis_results_survey_question_unique UNIQUE (survey_id, question_id);

-- +goose Down

ALTER TABLE analysis_results DROP CONSTRAINT IF EXISTS analysis_results_survey_question_unique;

-- NOTE: PostgreSQL no soporta remover valores de un enum una vez agregados.
-- 'analysing' y 'complete' permanecen en survey_status tras el rollback.
