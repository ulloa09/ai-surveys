-- +goose Up

-- El ciclo de vida (#08) reutiliza columnas que ya existen en 001_init.sql:
-- status, opens_at, closes_at, response_cap, public_token, qr_png_url, qr_svg_url.
-- Aquí solo añadimos las restricciones de integridad y los índices que el
-- scheduler necesita para encontrar encuestas por abrir/cerrar de forma eficiente.

ALTER TABLE surveys
  ADD CONSTRAINT surveys_response_cap_positive
    CHECK (response_cap IS NULL OR response_cap > 0),
  ADD CONSTRAINT surveys_schedule_order
    CHECK (opens_at IS NULL OR closes_at IS NULL OR opens_at < closes_at);

-- Índices parciales: solo indexan las filas con fecha programada, que son las
-- únicas que el scheduler consulta cada minuto.
CREATE INDEX IF NOT EXISTS idx_surveys_opens_at
  ON surveys (opens_at) WHERE opens_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_surveys_closes_at
  ON surveys (closes_at) WHERE closes_at IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS idx_surveys_closes_at;
DROP INDEX IF EXISTS idx_surveys_opens_at;

ALTER TABLE surveys
  DROP CONSTRAINT IF EXISTS surveys_schedule_order,
  DROP CONSTRAINT IF EXISTS surveys_response_cap_positive;
