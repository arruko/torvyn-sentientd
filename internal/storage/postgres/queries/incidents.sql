-- name: GetIncidentByID :one
SELECT * FROM incidents
WHERE id = $1;

-- name: CreateIncident :one
INSERT INTO incidents (
    id,
    service,
    cluster,
    environment,
    severity,
    correlation_key,
    correlation_version,
    state,
    created_at,
    updated_at,
    first_alert_at,
    last_alert_at,
    topology_snapshot,
    remediation_attempts,
    github_pr_url,
    rca_summary
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
)
RETURNING *;

-- name: UpdateIncident :one
UPDATE incidents
SET
    service = $2,
    cluster = $3,
    environment = $4,
    severity = $5,
    correlation_key = $6,
    correlation_version = $7,
    state = $8,
    updated_at = $9,
    first_alert_at = $10,
    last_alert_at = $11,
    topology_snapshot = $12,
    remediation_attempts = $13,
    github_pr_url = $14,
    rca_summary = $15
WHERE id = $1
RETURNING *;

-- name: FindOpenByCorrelationKey :one
SELECT * FROM incidents
WHERE correlation_key = $1
  AND state != 'resolved'
  AND last_alert_at > $2
ORDER BY last_alert_at DESC
LIMIT 1;

-- name: ListIncidents :many
SELECT * FROM incidents
ORDER BY created_at DESC;

-- name: ListIncidentsByState :many
SELECT * FROM incidents
WHERE state = $1
ORDER BY created_at DESC;

-- name: IncrementRemediationAttempts :one
UPDATE incidents
SET remediation_attempts = remediation_attempts + 1,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateIncidentState :one
UPDATE incidents
SET state = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateIncidentRCA :one
UPDATE incidents
SET rca_summary = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateIncidentPR :one
UPDATE incidents
SET github_pr_url = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;
