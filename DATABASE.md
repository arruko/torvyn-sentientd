# Database Setup Guide

This guide covers the Postgres setup for sentientd's production storage.

## Overview

sentientd uses **PostgreSQL** as its primary data store for incidents and alerts. The schema is managed with [golang-migrate](https://github.com/golang-migrate/migrate) and queries are type-safe via [sqlc](https://sqlc.dev/).

## Quick Start (Local Development)

### 1. Install dependencies

```bash
# Install golang-migrate (macOS)
brew install golang-migrate

# Install sqlc (already done via go install)
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

### 2. Start Postgres

```bash
make db-up
```

This starts a Postgres 16 container via Docker Compose with:
- **User**: `sentientd`
- **Password**: `sentientd`
- **Database**: `sentientd`
- **Port**: `5432`

### 3. Run migrations

```bash
make db-migrate
```

This applies all migrations from the `migrations/` directory.

### 4. Verify setup

```bash
# Connect to Postgres
docker exec -it sentientd-postgres psql -U sentientd

# Check tables
\dt

# Check incidents schema
\d incidents

# Exit
\q
```

You should see:
- `incidents` table
- `alerts` table
- `schema_migrations` table (managed by golang-migrate)

### 5. Run sentientd with Postgres

```bash
export DATABASE_URL="postgres://sentientd:sentientd@localhost:5432/sentientd?sslmode=disable"
make run
```

Or use in-memory mode for testing:

```bash
export USE_INMEMORY_STORE=true
make run
```

## Schema Overview

### `incidents` table

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | Primary key, unique incident ID |
| `service` | TEXT | Service name (e.g., `api-server`) |
| `cluster` | TEXT | Cluster name (e.g., `prod-us-east-1`) |
| `environment` | TEXT | Environment (e.g., `production`) |
| `severity` | TEXT | Severity level (e.g., `critical`, `warning`) |
| `correlation_key` | TEXT | Correlation key for grouping (indexed) |
| `correlation_version` | INTEGER | Version counter for correlation rounds |
| `state` | TEXT | Incident state (`open`, `investigating`, `mitigating`, `monitoring`, `resolved`) |
| `created_at` | TIMESTAMPTZ | When incident was first created |
| `updated_at` | TIMESTAMPTZ | Last update timestamp |
| `first_alert_at` | TIMESTAMPTZ | Timestamp of first alert |
| `last_alert_at` | TIMESTAMPTZ | Timestamp of most recent alert (indexed) |
| `topology_snapshot` | JSONB | ServiceGraph snapshot at incident creation |
| `remediation_attempts` | INTEGER | Number of remediation attempts (default: 0) |
| `github_pr_url` | TEXT | URL of GitHub PR for remediation (nullable) |
| `rca_summary` | TEXT | Root cause analysis summary (nullable) |

**Key indexes:**
- `correlation_key` - for fast correlation lookups
- `state` - for filtering by incident state
- `last_alert_at` - for time-based queries
- Composite index on `(correlation_key, state, last_alert_at)` for `FindOpenByCorrelationKey`

### `alerts` table

| Column | Type | Description |
|--------|------|-------------|
| `id` | SERIAL | Auto-incrementing primary key |
| `incident_id` | TEXT | Foreign key to `incidents.id` |
| `fingerprint` | TEXT | Unique alert fingerprint from Alertmanager |
| `labels` | JSONB | Alert labels (e.g., `{"severity": "critical"}`) |
| `annotations` | JSONB | Alert annotations (e.g., `{"description": "..."}`) |
| `starts_at` | TIMESTAMPTZ | When alert started firing |
| `ends_at` | TIMESTAMPTZ | When alert stopped firing (nullable) |
| `created_at` | TIMESTAMPTZ | When alert was ingested |

**Constraints:**
- Unique constraint on `(incident_id, fingerprint)` - prevents duplicate alerts per incident
- Foreign key to `incidents(id)` with `ON DELETE CASCADE`

## SQL Queries (sqlc)

All SQL operations are type-safe via sqlc. See:
- `internal/storage/postgres/queries/incidents.sql` - Incident queries
- `internal/storage/postgres/queries/alerts.sql` - Alert queries

Generated Go code is in `internal/storage/postgres/`.

### Common operations

```go
// Get incident by ID
inc, err := store.GetByID(ctx, "incident-123")

// Save incident (upsert)
err := store.Save(ctx, incident)

// Find open incident by correlation key within time window
inc, err := store.FindOpenByCorrelationKey(ctx, "prod:api:cpu", 5*time.Minute)

// List all incidents
incidents, err := store.List(ctx)
```

## Migrations

### Create a new migration

```bash
migrate create -ext sql -dir migrations -seq add_new_field
```

This creates:
- `migrations/000002_add_new_field.up.sql`
- `migrations/000002_add_new_field.down.sql`

### Apply migrations

```bash
make db-migrate
```

### Rollback migrations

```bash
make db-migrate-down
```

### Reset database

```bash
make db-reset
```

## Production Deployment

### Environment Variables

```bash
# Required
DATABASE_URL=postgres://user:pass@host:5432/dbname?sslmode=require

# Optional
USE_INMEMORY_STORE=false  # Set to true to disable Postgres
```

### Connection Pool Settings

Default pool settings (see `internal/database/database.go`):
- **MaxConns**: 25
- **MinConns**: 5
- **MaxConnLifetime**: 1 hour
- **MaxConnIdleTime**: 30 minutes

Adjust via `database.Config` if needed.

### Running migrations in production

#### Option 1: Kubernetes Job (recommended)

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: sentientd-migrate
spec:
  template:
    spec:
      containers:
      - name: migrate
        image: migrate/migrate
        args:
          - -path=/migrations
          - -database=$(DATABASE_URL)
          - up
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: sentientd-db
              key: url
        volumeMounts:
        - name: migrations
          mountPath: /migrations
      volumes:
      - name: migrations
        configMap:
          name: sentientd-migrations
      restartPolicy: Never
```

#### Option 2: On startup (not recommended for production)

Set `RUN_MIGRATIONS_ON_STARTUP=true` environment variable.

**⚠️ Warning**: This can cause issues with multiple replicas.

## Troubleshooting

### "connection refused"

```bash
# Check if Postgres is running
docker ps | grep sentientd-postgres

# Check Postgres logs
docker logs sentientd-postgres

# Restart Postgres
make db-down && make db-up
```

### "relation does not exist"

Migrations haven't been applied:

```bash
make db-migrate
```

### "duplicate key value violates unique constraint"

This is expected behavior when:
- Trying to insert the same alert twice into the same incident (alerts are upserted)
- Trying to create an incident with duplicate ID

### Reset everything

```bash
make db-down
docker volume rm torvyn-sentientd_postgres_data
make db-up db-migrate
```

## Testing

### Unit tests (with in-memory store)

```bash
go test ./internal/store/...
```

### Integration tests (with real Postgres)

```bash
make test-integration
```

This:
1. Starts Postgres
2. Runs migrations
3. Runs tests with `DATABASE_URL` set
4. Tears down Postgres

## Architecture Decisions

### Why Postgres over CRD?

**v1**: Postgres gives us:
- Strong consistency for correlation
- Rich querying (time ranges, ordering, aggregations)
- Transaction support
- Proven reliability

**v2/v3**: We may add a **read-only CRD view** for kubectl-based observability, but Postgres remains the source of truth.

### Why sqlc over ORM?

- Type-safe without runtime reflection
- Explicit SQL (easier to optimize)
- No magic, no N+1 queries
- Minimal dependencies

### Why separate `alerts` table?

- Enables querying individual alerts by fingerprint
- Supports alert deduplication
- Better for debugging correlation issues
- Scales better than JSONB array in `incidents`

## Next Steps

After Milestone 1 is complete:
- [ ] Add incident state transitions with validation
- [ ] Add metrics collection (Prometheus)
- [ ] Add distributed tracing (OpenTelemetry)
- [ ] Add read replicas for high-availability
- [ ] Add pg_partman for time-based partitioning (if needed at scale)
