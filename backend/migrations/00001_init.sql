-- +goose Up

CREATE TABLE users (
    id            UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    nickname      TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL, -- только bcrypt-хеш (SEC1)
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Сюда попадает jti токена при выходе.
CREATE TABLE revoked_tokens (
    jti        TEXT PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX revoked_tokens_expires_at_idx ON revoked_tokens (expires_at);

CREATE TABLE risk_signals (
    code             TEXT PRIMARY KEY,
    side             TEXT   NOT NULL CHECK (side IN ('buyer', 'seller')),
    title            TEXT   NOT NULL,
    summary          TEXT   NOT NULL,
    description      TEXT   NOT NULL,
    how_to_recognize TEXT[] NOT NULL DEFAULT '{}',
    how_to_act       TEXT   NOT NULL
);

-- Сценарии версионируются: сессия ссылается на конкретную версию, поэтому
-- правка контента не меняет завершённые сессии (FR32).
CREATE TABLE scenarios (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code              TEXT        NOT NULL,
    version           INT         NOT NULL CHECK (version >= 1),
    role              TEXT        NOT NULL CHECK (role IN ('buyer', 'seller')),
    title             TEXT        NOT NULL,
    description       TEXT        NOT NULL,
    intro             TEXT        NOT NULL DEFAULT '',
    difficulty        TEXT        NOT NULL CHECK (difficulty IN ('basic', 'advanced', 'demo')),
    steps_count       INT         NOT NULL CHECK (steps_count BETWEEN 3 AND 8),
    estimated_minutes INT         NOT NULL DEFAULT 3,
    is_active         BOOLEAN     NOT NULL DEFAULT TRUE,
    -- Хеш файла: версия поднимается только при изменении контента.
    content_hash      TEXT        NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (code, version)
);

CREATE UNIQUE INDEX scenarios_active_code_idx ON scenarios (code) WHERE is_active;

CREATE TABLE steps (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    scenario_id       BIGINT  NOT NULL REFERENCES scenarios (id) ON DELETE CASCADE,
    code              TEXT    NOT NULL,
    type              TEXT    NOT NULL CHECK (type IN ('dialog', 'terminal')),
    position          INT     NOT NULL,
    -- JSONB: структура контента может меняться без миграции схемы.
    content           JSONB   NOT NULL,
    risk_signal_codes TEXT[]  NOT NULL DEFAULT '{}',
    is_start          BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (scenario_id, code)
);

CREATE UNIQUE INDEX steps_single_start_idx ON steps (scenario_id) WHERE is_start;

CREATE TABLE options (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    step_id        BIGINT NOT NULL REFERENCES steps (id) ON DELETE CASCADE,
    code           TEXT   NOT NULL,
    text           TEXT   NOT NULL,
    outcome        TEXT   NOT NULL CHECK (outcome IN ('safe', 'risky', 'critical')),
    score          INT    NOT NULL CHECK (score IN (10, 0, -10)),
    feedback       TEXT   NOT NULL CHECK (feedback <> ''),
    next_step_code TEXT,
    position       INT    NOT NULL,
    UNIQUE (step_id, code)
);

CREATE TABLE sessions (
    id                UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    user_id           UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    scenario_id       BIGINT      NOT NULL REFERENCES scenarios (id),
    scenario_code     TEXT        NOT NULL,
    scenario_version  INT         NOT NULL,
    status            TEXT        NOT NULL CHECK (status IN ('in_progress', 'completed', 'abandoned')),
    current_step_code TEXT,
    score             INT         NOT NULL DEFAULT 0,
    started_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at       TIMESTAMPTZ,
    CHECK ((status = 'in_progress') = (finished_at IS NULL))
);

-- Не более одной незавершённой сессии на сценарий, иначе «продолжить
-- тренировку» становится неоднозначным (FR12).
CREATE UNIQUE INDEX sessions_single_active_idx
    ON sessions (user_id, scenario_code) WHERE status = 'in_progress';

CREATE INDEX sessions_history_idx
    ON sessions (user_id, scenario_code, finished_at DESC) WHERE status = 'completed';

CREATE TABLE answers (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_id        UUID        NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    step_code         TEXT        NOT NULL,
    option_code       TEXT        NOT NULL,
    outcome           TEXT        NOT NULL CHECK (outcome IN ('safe', 'risky', 'critical')),
    score_delta       INT         NOT NULL CHECK (score_delta IN (10, 0, -10)),
    -- Признаки риска копируются в ответ: разбор остаётся воспроизводимым
    -- после выхода новой версии сценария.
    risk_signal_codes TEXT[]      NOT NULL DEFAULT '{}',
    position          INT         NOT NULL,
    answered_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Изменить сделанный выбор нельзя: FR13 гарантируется схемой.
    UNIQUE (session_id, step_code)
);

CREATE INDEX answers_session_idx ON answers (session_id, position);

-- +goose Down

DROP TABLE IF EXISTS answers;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS options;
DROP TABLE IF EXISTS steps;
DROP TABLE IF EXISTS scenarios;
DROP TABLE IF EXISTS risk_signals;
DROP TABLE IF EXISTS revoked_tokens;
DROP TABLE IF EXISTS users;
