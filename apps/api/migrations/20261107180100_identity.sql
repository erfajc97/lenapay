-- Identity: users, sessions, rotating refresh tokens, device keys, email codes, step-up challenges.
-- Ids are UUIDv7 generated in Go; there are no DB defaults for ids on purpose.

-- +goose Up
CREATE TABLE users (
    id                uuid        PRIMARY KEY,
    email             text        NOT NULL CHECK (char_length(email) BETWEEN 3 AND 254),
    password_hash     text        NOT NULL CHECK (char_length(password_hash) <= 255),
    email_verified_at timestamptz,
    totp_secret       bytea,
    totp_enabled_at   timestamptz,
    status            text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email));

CREATE TABLE devices (
    id         uuid        PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    public_key bytea       NOT NULL CHECK (octet_length(public_key) = 32),
    name       text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);
CREATE INDEX devices_user_id_idx ON devices (user_id);

CREATE TABLE sessions (
    id           uuid        PRIMARY KEY,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    device_id    uuid        REFERENCES devices (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    revoked_at   timestamptz
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_device_id_idx ON sessions (device_id);

CREATE TABLE refresh_tokens (
    id          uuid        PRIMARY KEY,
    session_id  uuid        NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    token_hash  bytea       NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz,
    replaced_by uuid        REFERENCES refresh_tokens (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_session_id_idx ON refresh_tokens (session_id);
CREATE INDEX refresh_tokens_replaced_by_idx ON refresh_tokens (replaced_by);

CREATE TABLE email_codes (
    id          uuid        PRIMARY KEY,
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose     text        NOT NULL CHECK (purpose IN ('verify_email')),
    code_hash   bytea       NOT NULL,
    attempts    integer     NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_codes_user_id_idx ON email_codes (user_id, purpose);

CREATE TABLE step_up_challenges (
    id         uuid        PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    session_id uuid        NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    device_id  uuid        REFERENCES devices (id) ON DELETE CASCADE,
    nonce      bytea       NOT NULL CHECK (octet_length(nonce) >= 32),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX step_up_challenges_user_id_idx ON step_up_challenges (user_id);
CREATE INDEX step_up_challenges_session_id_idx ON step_up_challenges (session_id);
CREATE INDEX step_up_challenges_device_id_idx ON step_up_challenges (device_id);

-- +goose Down
DROP TABLE step_up_challenges;
DROP TABLE email_codes;
DROP TABLE refresh_tokens;
DROP TABLE sessions;
DROP TABLE devices;
DROP TABLE users;
