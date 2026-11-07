-- Identity queries (users, sessions, refresh tokens, devices, email codes, step-up challenges).

-- name: InsertUser :exec
INSERT INTO users (id, email, password_hash)
VALUES (sqlc.arg(id), sqlc.arg(email), sqlc.arg(password_hash));

-- name: GetUserByID :one
SELECT * FROM users WHERE id = sqlc.arg(id);

-- name: GetUserByEmail :one
SELECT * FROM users WHERE lower(email) = lower(sqlc.arg(email));

-- name: MarkEmailVerified :exec
UPDATE users SET email_verified_at = sqlc.arg(verified_at), updated_at = sqlc.arg(verified_at)
WHERE id = sqlc.arg(id) AND email_verified_at IS NULL;

-- name: InsertEmailCode :exec
INSERT INTO email_codes (id, user_id, purpose, code_hash, expires_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(purpose), sqlc.arg(code_hash), sqlc.arg(expires_at));

-- name: GetActiveEmailCodeForUpdate :one
SELECT * FROM email_codes
WHERE user_id = sqlc.arg(user_id)
  AND purpose = sqlc.arg(purpose)
  AND consumed_at IS NULL
  AND expires_at > sqlc.arg(now)
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE;

-- name: IncrementEmailCodeAttempts :exec
UPDATE email_codes SET attempts = attempts + 1 WHERE id = sqlc.arg(id);

-- name: ConsumeEmailCode :exec
UPDATE email_codes SET consumed_at = sqlc.arg(consumed_at) WHERE id = sqlc.arg(id) AND consumed_at IS NULL;

-- name: InsertSession :exec
INSERT INTO sessions (id, user_id, device_id) VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.narg(device_id));

-- name: GetSession :one
SELECT * FROM sessions WHERE id = sqlc.arg(id);

-- name: RevokeSession :exec
UPDATE sessions SET revoked_at = sqlc.arg(revoked_at) WHERE id = sqlc.arg(id) AND revoked_at IS NULL;

-- name: InsertRefreshToken :exec
INSERT INTO refresh_tokens (id, session_id, token_hash, expires_at)
VALUES (sqlc.arg(id), sqlc.arg(session_id), sqlc.arg(token_hash), sqlc.arg(expires_at));

-- name: GetRefreshTokenByHashForUpdate :one
SELECT * FROM refresh_tokens WHERE token_hash = sqlc.arg(token_hash) FOR UPDATE;

-- name: MarkRefreshTokenUsed :exec
UPDATE refresh_tokens SET used_at = sqlc.arg(used_at), replaced_by = sqlc.narg(replaced_by)
WHERE id = sqlc.arg(id) AND used_at IS NULL;

-- name: InsertDevice :exec
INSERT INTO devices (id, user_id, public_key, name)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(public_key), sqlc.arg(name));

-- name: GetDevice :one
SELECT * FROM devices WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND revoked_at IS NULL;

-- name: InsertStepUpChallenge :exec
INSERT INTO step_up_challenges (id, user_id, session_id, device_id, nonce, expires_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(session_id), sqlc.narg(device_id), sqlc.arg(nonce), sqlc.arg(expires_at));

-- name: GetStepUpChallengeForUpdate :one
SELECT * FROM step_up_challenges
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND session_id = sqlc.arg(session_id)
FOR UPDATE;

-- name: MarkStepUpChallengeUsed :exec
UPDATE step_up_challenges SET used_at = sqlc.arg(used_at) WHERE id = sqlc.arg(id) AND used_at IS NULL;
