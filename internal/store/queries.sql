-- name: GetSetting :one
SELECT value
FROM settings
WHERE key = sqlc.arg(key)
LIMIT 1;

-- name: SetSetting :exec
INSERT INTO settings (key, value)
VALUES (sqlc.arg(key), sqlc.arg(value))
ON CONFLICT (key) DO UPDATE SET value = excluded.value;
