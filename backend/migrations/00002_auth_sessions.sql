-- +goose Up

ALTER TABLE users
    ADD COLUMN role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin'));

CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    session_id UUID        NOT NULL,
    hash       TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_session_id_idx ON refresh_tokens (session_id);
CREATE INDEX refresh_tokens_expires_at_idx ON refresh_tokens (expires_at);

CREATE TABLE guest_sessions (
    id         UUID PRIMARY KEY,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX guest_sessions_expires_at_idx ON guest_sessions (expires_at);

ALTER TABLE sessions
    ALTER COLUMN user_id DROP NOT NULL,
    ADD COLUMN guest_session_id UUID REFERENCES guest_sessions (id) ON DELETE CASCADE,
    ADD CONSTRAINT sessions_owner_check
        CHECK (num_nonnulls(user_id, guest_session_id) = 1);

DROP INDEX sessions_single_active_idx;
CREATE UNIQUE INDEX sessions_single_active_idx
    ON sessions (COALESCE(user_id, guest_session_id), scenario_code)
    WHERE status = 'in_progress';

-- +goose Down

DROP INDEX sessions_single_active_idx;
CREATE UNIQUE INDEX sessions_single_active_idx
    ON sessions (user_id, scenario_code) WHERE status = 'in_progress';

ALTER TABLE sessions
    DROP CONSTRAINT sessions_owner_check,
    DROP COLUMN guest_session_id,
    ALTER COLUMN user_id SET NOT NULL;

DROP TABLE refresh_tokens;
DROP TABLE guest_sessions;

ALTER TABLE users DROP COLUMN role;
