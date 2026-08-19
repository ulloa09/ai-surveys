-- +goose Up

-- Extensión para generar UUIDs automáticamente con gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- === users ===
-- Usuarios de la plataforma. El rol determina qué puede hacer cada uno.
CREATE TYPE user_role AS ENUM ('super_admin', 'admin', 'viewer');

CREATE TABLE users (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email        TEXT        NOT NULL UNIQUE,
    display_name TEXT        NOT NULL,
    role         user_role   NOT NULL DEFAULT 'viewer',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- === teams ===
CREATE TABLE teams (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    created_by UUID        NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- === team_members ===
-- Tabla de unión users ↔ teams. Un usuario puede tener distinto rol en cada equipo.
CREATE TYPE team_role AS ENUM ('owner', 'editor', 'viewer');

CREATE TABLE team_members (
    team_id UUID      NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role    team_role NOT NULL DEFAULT 'viewer',
    PRIMARY KEY (team_id, user_id)  -- un usuario no puede estar dos veces en el mismo equipo
);

-- === surveys ===
CREATE TYPE survey_status   AS ENUM ('draft', 'open', 'closed', 'archived');
CREATE TYPE survey_mode     AS ENUM ('conversational', 'form', 'prompt_only');
CREATE TYPE anonymity_level AS ENUM ('none', 'partial', 'full');
CREATE TYPE termination_mode AS ENUM ('turn_limit', 'time_limit', 'response_cap', 'manual');

CREATE TABLE surveys (
    id                   UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    title                TEXT            NOT NULL,
    description          TEXT,
    owner_id             UUID            NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    team_id              UUID            REFERENCES teams(id) ON DELETE SET NULL,
    status               survey_status   NOT NULL DEFAULT 'draft',
    mode                 survey_mode     NOT NULL DEFAULT 'form',
    system_prompt        TEXT,
    anonymity_level      anonymity_level NOT NULL DEFAULT 'none',
    available_languages  TEXT[]          NOT NULL DEFAULT '{en}',
    default_language     TEXT            NOT NULL DEFAULT 'en',
    termination_mode     termination_mode NOT NULL DEFAULT 'manual',
    turn_limit           INT,
    time_estimate_minutes INT,
    response_cap         INT,
    allow_revisit        BOOL            NOT NULL DEFAULT FALSE,
    optional_registration BOOL           NOT NULL DEFAULT FALSE,
    opens_at             TIMESTAMPTZ,
    closes_at            TIMESTAMPTZ,
    public_token         UUID            NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    qr_png_url           TEXT,
    qr_svg_url           TEXT,
    created_at           TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

-- === questions ===
CREATE TYPE question_type AS ENUM (
    'open_ended',
    'single_choice',
    'multi_choice',
    'true_false',
    'linear_scale',
    'ranking',
    'matrix'
);

CREATE TABLE questions (
    id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    survey_id   UUID          NOT NULL REFERENCES surveys(id) ON DELETE CASCADE,
    type        question_type NOT NULL DEFAULT 'open_ended',
    text        TEXT          NOT NULL,
    required    BOOL          NOT NULL DEFAULT TRUE,
    ai_followup BOOL          NOT NULL DEFAULT FALSE,
    options     JSONB,        -- para multiple_choice: [{label: "Sí", value: "yes"}, ...]
    order_index INT           NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- === responses ===
-- Una respuesta = una sesión de un usuario respondiendo una encuesta.
CREATE TYPE response_status AS ENUM ('in_progress', 'submitted', 'abandoned');

CREATE TABLE responses (
    id                    UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    survey_id             UUID            NOT NULL REFERENCES surveys(id) ON DELETE CASCADE,
    user_id               UUID            REFERENCES users(id) ON DELETE SET NULL, -- nullable si anónimo
    fingerprint_hash      TEXT,           -- identifica dispositivo en respuestas anónimas
    status                response_status NOT NULL DEFAULT 'in_progress',
    language              TEXT            NOT NULL DEFAULT 'en',
    started_at            TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    submitted_at          TIMESTAMPTZ,
    current_question_index INT            NOT NULL DEFAULT 0,
    turn_count            INT             NOT NULL DEFAULT 0,
    registered_name       TEXT,
    registered_email      TEXT
);

-- === turns ===
-- Mensajes del chat conversacional (user ↔ AI) dentro de una respuesta.
CREATE TYPE turn_role AS ENUM ('user', 'assistant');

CREATE TABLE turns (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    response_id UUID        NOT NULL REFERENCES responses(id) ON DELETE CASCADE,
    role        turn_role   NOT NULL,
    content     TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- === answers ===
-- Respuesta concreta a cada pregunta dentro de una sesión.
CREATE TABLE answers (
    id              UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    response_id     UUID    NOT NULL REFERENCES responses(id) ON DELETE CASCADE,
    question_id     UUID    NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    raw_value       TEXT    NOT NULL,
    sentiment_score FLOAT,  -- null hasta que corra el análisis
    sentiment_label TEXT,
    topic_tags      TEXT[],
    is_outlier      BOOL    NOT NULL DEFAULT FALSE
);

-- === analysis_results ===
-- Resultados del análisis AI por encuesta/pregunta. Se genera en batch, no en tiempo real.
CREATE TABLE analysis_results (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    survey_id             UUID        NOT NULL REFERENCES surveys(id) ON DELETE CASCADE,
    question_id           UUID        REFERENCES questions(id) ON DELETE SET NULL,
    summary_text          TEXT,
    sentiment_distribution JSONB,     -- ej: {"positive": 0.6, "neutral": 0.3, "negative": 0.1}
    topic_clusters        JSONB,      -- ej: [{"label": "precio", "count": 42}, ...]
    analysed_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── survey_access_tokens ===
-- Tokens de acceso temporal para encuestas (links de un solo uso, QR, etc.)
CREATE TABLE survey_access_tokens (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    response_id UUID        REFERENCES responses(id) ON DELETE CASCADE,
    survey_id   UUID        NOT NULL REFERENCES surveys(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── platform_settings ===
-- Configuración global de la plataforma en formato clave-valor.
-- Ej: key='maintenance_mode', value='true'
CREATE TABLE platform_settings (
    key        TEXT        PRIMARY KEY,
    value      TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── índices ===
-- Los índices aceleran las queries más frecuentes. Sin ellos, cada query escanea toda la tabla.
CREATE INDEX idx_surveys_owner      ON surveys(owner_id);
CREATE INDEX idx_surveys_team       ON surveys(team_id);
CREATE INDEX idx_surveys_status     ON surveys(status);
CREATE INDEX idx_questions_survey   ON questions(survey_id, order_index);
CREATE INDEX idx_responses_survey   ON responses(survey_id);
CREATE INDEX idx_responses_user     ON responses(user_id);
CREATE INDEX idx_turns_response     ON turns(response_id, created_at);
CREATE INDEX idx_answers_response   ON answers(response_id);
CREATE INDEX idx_answers_question   ON answers(question_id);
CREATE INDEX idx_analysis_survey    ON analysis_results(survey_id);


-- +goose Down

-- Se eliminan en orden inverso para respetar las foreign keys
DROP TABLE IF EXISTS platform_settings;
DROP TABLE IF EXISTS survey_access_tokens;
DROP TABLE IF EXISTS analysis_results;
DROP TABLE IF EXISTS answers;
DROP TABLE IF EXISTS turns;
DROP TABLE IF EXISTS responses;
DROP TABLE IF EXISTS questions;
DROP TABLE IF EXISTS surveys;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS user_role;
DROP TYPE IF EXISTS team_role;
DROP TYPE IF EXISTS survey_status;
DROP TYPE IF EXISTS survey_mode;
DROP TYPE IF EXISTS anonymity_level;
DROP TYPE IF EXISTS termination_mode;
DROP TYPE IF EXISTS question_type;
DROP TYPE IF EXISTS response_status;
DROP TYPE IF EXISTS turn_role;