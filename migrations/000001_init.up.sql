-- incidents table: main incident metadata
CREATE TABLE IF NOT EXISTS incidents (
    id TEXT PRIMARY KEY,
    service TEXT NOT NULL,
    cluster TEXT NOT NULL,
    environment TEXT NOT NULL,
    severity TEXT NOT NULL,
    correlation_key TEXT NOT NULL,
    correlation_version INTEGER NOT NULL DEFAULT 1,
    state TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    first_alert_at TIMESTAMPTZ NOT NULL,
    last_alert_at TIMESTAMPTZ NOT NULL,

    -- New fields for production
    topology_snapshot JSONB,
    remediation_attempts INTEGER NOT NULL DEFAULT 0,
    github_pr_url TEXT,
    rca_summary TEXT,

    -- Indexes for common queries
    CONSTRAINT valid_state CHECK (state IN ('open', 'investigating', 'mitigating', 'monitoring', 'resolved'))
);

-- Index for correlation queries (most critical for performance)
CREATE INDEX idx_incidents_correlation_key ON incidents(correlation_key);
CREATE INDEX idx_incidents_state ON incidents(state);
CREATE INDEX idx_incidents_last_alert_at ON incidents(last_alert_at DESC);
CREATE INDEX idx_incidents_created_at ON incidents(created_at DESC);

-- Composite index for FindOpenByCorrelationKey query
CREATE INDEX idx_incidents_correlation_open ON incidents(correlation_key, state, last_alert_at)
    WHERE state != 'resolved';

-- alerts table: 1:N relationship with incidents
CREATE TABLE IF NOT EXISTS alerts (
    id SERIAL PRIMARY KEY,
    incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    fingerprint TEXT NOT NULL,
    labels JSONB NOT NULL,
    annotations JSONB NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure no duplicate fingerprints per incident
    CONSTRAINT unique_alert_per_incident UNIQUE(incident_id, fingerprint)
);

-- Index for alert lookups
CREATE INDEX idx_alerts_incident_id ON alerts(incident_id);
CREATE INDEX idx_alerts_fingerprint ON alerts(fingerprint);
CREATE INDEX idx_alerts_starts_at ON alerts(starts_at DESC);
