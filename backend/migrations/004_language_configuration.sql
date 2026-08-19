-- +goose Up

UPDATE surveys
SET available_languages = ARRAY['es']::TEXT[],
    default_language = 'es'
WHERE available_languages = ARRAY['en']::TEXT[]
  AND default_language = 'en';

UPDATE responses
SET language = 'es'
WHERE language = 'en'
  AND survey_id IN (
    SELECT id
    FROM surveys
    WHERE available_languages = ARRAY['es']::TEXT[]
      AND default_language = 'es'
  );

ALTER TABLE surveys
  ALTER COLUMN available_languages SET DEFAULT ARRAY['es']::TEXT[],
  ALTER COLUMN default_language SET DEFAULT 'es';

ALTER TABLE responses
  ALTER COLUMN language SET DEFAULT 'es';

ALTER TABLE surveys
  ADD CONSTRAINT surveys_available_languages_not_empty
    CHECK (cardinality(available_languages) > 0),
  ADD CONSTRAINT surveys_available_languages_supported
    CHECK (available_languages <@ ARRAY['es', 'en']::TEXT[]),
  ADD CONSTRAINT surveys_default_language_supported
    CHECK (default_language IN ('es', 'en')),
  ADD CONSTRAINT surveys_default_language_available
    CHECK (default_language = ANY(available_languages));

ALTER TABLE responses
  ADD CONSTRAINT responses_language_supported
    CHECK (language IN ('es', 'en'));

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION validate_response_language()
RETURNS TRIGGER AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM surveys
    WHERE id = NEW.survey_id
      AND NEW.language = ANY(available_languages)
  ) THEN
    RAISE EXCEPTION 'response language must be available for the survey'
      USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER responses_language_available_trigger
BEFORE INSERT OR UPDATE OF survey_id, language ON responses
FOR EACH ROW
EXECUTE FUNCTION validate_response_language();

-- +goose Down

DROP TRIGGER IF EXISTS responses_language_available_trigger ON responses;
DROP FUNCTION IF EXISTS validate_response_language();

ALTER TABLE responses
  DROP CONSTRAINT IF EXISTS responses_language_supported;

ALTER TABLE surveys
  DROP CONSTRAINT IF EXISTS surveys_default_language_available,
  DROP CONSTRAINT IF EXISTS surveys_default_language_supported,
  DROP CONSTRAINT IF EXISTS surveys_available_languages_supported,
  DROP CONSTRAINT IF EXISTS surveys_available_languages_not_empty;

ALTER TABLE responses
  ALTER COLUMN language SET DEFAULT 'en';

ALTER TABLE surveys
  ALTER COLUMN available_languages SET DEFAULT ARRAY['en']::TEXT[],
  ALTER COLUMN default_language SET DEFAULT 'en';
