-- Cross-cutting tables: idempotency keys, audit log, notification outbox and push tokens.

-- +goose Up
CREATE TABLE idempotency_keys (
    scope           text        NOT NULL CHECK (char_length(scope) BETWEEN 1 AND 200),
    key             text        NOT NULL CHECK (char_length(key) BETWEEN 16 AND 128),
    request_hash    bytea       NOT NULL CHECK (octet_length(request_hash) = 32),
    response_status integer     CHECK (response_status BETWEEN 100 AND 599),
    response_body   bytea,
    created_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL,
    PRIMARY KEY (scope, key)
);
CREATE INDEX idempotency_keys_expires_at_idx ON idempotency_keys (expires_at);

CREATE TABLE audit_log (
    id              uuid        PRIMARY KEY,
    org_id          uuid        REFERENCES organizations (id) ON DELETE RESTRICT,
    actor_id        uuid        REFERENCES users (id) ON DELETE RESTRICT,
    actor_kind      text        NOT NULL CHECK (actor_kind IN ('user', 'system', 'internal', 'psp')),
    action          text        NOT NULL CHECK (char_length(action) BETWEEN 1 AND 80),
    target_type     text        NOT NULL CHECK (char_length(target_type) BETWEEN 1 AND 40),
    target_id       uuid,
    request_id      text        CHECK (char_length(request_id) <= 100),
    idempotency_key text        CHECK (char_length(idempotency_key) <= 128),
    payload_hash    bytea       CHECK (octet_length(payload_hash) = 32),
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_org_id_idx ON audit_log (org_id, created_at DESC);
CREATE INDEX audit_log_actor_id_idx ON audit_log (actor_id);
CREATE INDEX audit_log_target_idx ON audit_log (target_type, target_id);
CREATE TRIGGER audit_log_append_only
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION reject_append_only_mutation();

CREATE TABLE notifications (
    id         uuid        PRIMARY KEY,
    org_id     uuid        REFERENCES organizations (id) ON DELETE CASCADE,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind       text        NOT NULL CHECK (char_length(kind) BETWEEN 1 AND 60),
    data       jsonb       NOT NULL DEFAULT '{}'::jsonb,
    status     text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    attempts   integer     NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at    timestamptz
);
CREATE INDEX notifications_user_id_idx ON notifications (user_id, created_at DESC);
CREATE INDEX notifications_org_id_idx ON notifications (org_id);
CREATE INDEX notifications_pending_idx ON notifications (created_at) WHERE status = 'pending';

CREATE TABLE push_tokens (
    id           uuid        PRIMARY KEY,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token        text        NOT NULL UNIQUE CHECK (char_length(token) BETWEEN 1 AND 200),
    platform     text        NOT NULL CHECK (platform IN ('ios', 'android')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX push_tokens_user_id_idx ON push_tokens (user_id);

-- +goose Down
DROP TABLE push_tokens;
DROP TABLE notifications;
DROP TRIGGER audit_log_append_only ON audit_log;
DROP TABLE audit_log;
DROP TABLE idempotency_keys;
