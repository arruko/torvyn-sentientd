-- name: CreateAlert :one
INSERT INTO alerts (
    incident_id,
    fingerprint,
    labels,
    annotations,
    starts_at,
    ends_at
) VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (incident_id, fingerprint) DO UPDATE
SET
    labels = EXCLUDED.labels,
    annotations = EXCLUDED.annotations,
    starts_at = EXCLUDED.starts_at,
    ends_at = EXCLUDED.ends_at
RETURNING *;

-- name: GetAlertsByIncidentID :many
SELECT * FROM alerts
WHERE incident_id = $1
ORDER BY starts_at DESC;

-- name: GetAlertByFingerprint :one
SELECT * FROM alerts
WHERE fingerprint = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: ListRecentAlerts :many
SELECT * FROM alerts
WHERE starts_at > $1
ORDER BY starts_at DESC;
