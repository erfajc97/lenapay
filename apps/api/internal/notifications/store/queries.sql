-- Notification outbox and push token queries.

-- name: InsertNotification :exec
INSERT INTO notifications (id, org_id, user_id, kind, data)
VALUES (sqlc.arg(id), sqlc.narg(org_id), sqlc.arg(user_id), sqlc.arg(kind), sqlc.arg(data));

-- name: GetNotification :one
SELECT * FROM notifications WHERE id = sqlc.arg(id);

-- name: ListStalePendingNotifications :many
-- The sweep re-queues deliveries whose job was lost (oldest first, bounded).
SELECT id FROM notifications
WHERE status = 'pending' AND created_at < sqlc.arg(created_before)
ORDER BY created_at, id
LIMIT sqlc.arg(page_limit);

-- name: MarkNotificationSent :execrows
UPDATE notifications SET status = 'sent', sent_at = sqlc.arg(sent_at), attempts = attempts + 1
WHERE id = sqlc.arg(id) AND status = 'pending';

-- name: MarkNotificationAttemptFailed :exec
UPDATE notifications
SET attempts = attempts + 1,
    status = CASE WHEN attempts + 1 >= sqlc.arg(max_attempts)::integer THEN 'failed' ELSE 'pending' END
WHERE id = sqlc.arg(id) AND status = 'pending';

-- name: UpsertPushToken :exec
INSERT INTO push_tokens (id, user_id, token, platform)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(token), sqlc.arg(platform))
ON CONFLICT (token) DO UPDATE
SET user_id = EXCLUDED.user_id, platform = EXCLUDED.platform, last_seen_at = now();

-- name: ListPushTokensForUser :many
SELECT * FROM push_tokens WHERE user_id = sqlc.arg(user_id) ORDER BY created_at, id;
