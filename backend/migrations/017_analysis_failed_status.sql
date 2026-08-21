-- +goose Up

-- El Analysis Engine (#15) dejaba encuestas atascadas en 'analysing' para
-- siempre cuando el job fallaba a mitad de camino (proveedor de IA caído,
-- JSON inválido, timeout, crash del proceso): no había ningún estado
-- terminal de error, así que la UI mostraba "Analizando" indefinidamente y
-- no existía manera de reintentar. 'failed' es ese estado terminal.
ALTER TYPE survey_status ADD VALUE IF NOT EXISTS 'failed';

-- +goose Down

-- NOTE: PostgreSQL no soporta remover valores de un enum una vez agregados.
-- 'failed' permanece en survey_status tras el rollback.
