-- +goose Up

-- Tokens de continuación (resume tokens, issue #09/#10).
-- La tabla survey_access_tokens ya existe (001_init.sql); aquí solo agregamos
-- los índices que el flujo de reanudación necesita:
--
--   idx_access_tokens_response — para encontrar el token de una respuesta dada.
--   idx_access_tokens_expires  — para que un eventual job de limpieza pueda
--                                barrer tokens vencidos sin escanear toda la tabla.
--
-- La búsqueda por id (sat.id = $1) ya usa la PRIMARY KEY, así que no requiere
-- índice adicional.

CREATE INDEX IF NOT EXISTS idx_access_tokens_response
  ON survey_access_tokens (response_id);
CREATE INDEX IF NOT EXISTS idx_access_tokens_expires
  ON survey_access_tokens (expires_at);

-- +goose Down

DROP INDEX IF EXISTS idx_access_tokens_expires;
DROP INDEX IF EXISTS idx_access_tokens_response;
