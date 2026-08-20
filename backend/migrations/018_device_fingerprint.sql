-- +goose Up

-- Prevención de envíos duplicados en encuestas pseudónimas (#09).
--
-- La columna responses.fingerprint_hash existe desde 001_init pero nunca se
-- escribió: hasta ahora las encuestas 'partial' no tenían NINGUNA protección
-- contra duplicados. El chequeo de duplicados en ResponseService.Create
-- comparaba por user_id, y 'partial' justamente no guarda user_id (para no
-- vincular la respuesta a la persona), así que la cuenta siempre daba 0 y el
-- mismo alumno podía contestar cuantas veces quisiera.
--
-- Ahora 'partial' guarda aquí el HMAC del device_id (un UUID aleatorio por
-- navegador, ver internal/fingerprint), y este índice es el que hace barata la
-- consulta de duplicados por encuesta. Parcial (WHERE ... IS NOT NULL) porque
-- solo las respuestas pseudónimas tienen hash: las de 'none' se deduplican por
-- user_id y las de 'full' no se deduplican en absoluto — no guardan nada.
CREATE INDEX IF NOT EXISTS idx_responses_survey_fingerprint
    ON responses (survey_id, fingerprint_hash)
    WHERE fingerprint_hash IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS idx_responses_survey_fingerprint;
