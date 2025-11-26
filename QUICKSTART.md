# 🚀 sentientd Quick Start

Get sentientd running locally in 3 minutes.

## Prerequisites

- Go 1.21+
- Docker & Docker Compose
- Make

## Steps

### 1. Clone and setup

```bash
cd /path/to/torvyn-sentientd

# Install sqlc and golang-migrate
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
brew install golang-migrate  # macOS

# Setup local environment (starts Postgres + runs migrations)
make dev-setup
```

### 2. Run sentientd

```bash
# Option A: With Postgres (recommended)
export DATABASE_URL="postgres://sentientd:sentientd@localhost:5432/sentientd?sslmode=disable"
make run

# Option B: In-memory mode (testing only)
export USE_INMEMORY_STORE=true
make run
```

Expected output:
```
INFO: Connecting to Postgres...
INFO: Connected to database successfully (max_conns=25, min_conns=5)
INFO: Using Postgres incident store
INFO: Starting HTTP server on :8080
```

### 3. Test it

```bash
# Health check
curl http://localhost:8080/healthz
# Expected: ok

# Send a test alert
curl -X POST http://localhost:8080/alertmanager/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "alerts": [{
      "fingerprint": "test-alert-001",
      "labels": {
        "alertname": "HighCPU",
        "severity": "critical",
        "service": "api-server",
        "cluster": "prod-us-east-1",
        "namespace": "default"
      },
      "annotations": {
        "description": "CPU usage above 90%",
        "summary": "High CPU on api-server"
      },
      "startsAt": "2025-01-23T12:00:00Z"
    }]
  }'
```

### 4. Verify in database

```bash
# Connect to Postgres
docker exec -it sentientd-postgres psql -U sentientd

# Query incidents
SELECT id, service, severity, state, created_at FROM incidents;

# Query alerts
SELECT incident_id, fingerprint, labels->>'severity' as severity FROM alerts;

# Exit
\q
```

## What Just Happened?

1. **Alertmanager webhook received** → Alert ingested
2. **Correlator analyzed** → Incident created (or existing one updated)
3. **Stored in Postgres** → `incidents` and `alerts` tables
4. **Investigation loop** → Will trigger kagent analysis (when configured)

## Next Steps

- **Configure kagent**: Set `SENTIENTD_KAGENT_URL` to your kagent instance
- **Add Slack notifications**: Set `SENTIENTD_SLACK_WEBHOOK_URL`
- **Connect to Argo**: Set `SENTIENTD_ARGO_NAMESPACE`
- **Read [DATABASE.md](DATABASE.md)** for schema details
- **Read [MILESTONE1_COMPLETE.md](MILESTONE1_COMPLETE.md)** for full docs

## Useful Commands

```bash
# View logs (if running with make run)
# Ctrl+C to stop

# Check database
docker exec -it sentientd-postgres psql -U sentientd

# Reset everything
make db-down
docker volume rm torvyn-sentientd_postgres_data
make dev-setup

# Build binary
make build
./bin/sentientd
```

## Troubleshooting

**"connection refused"**
```bash
make db-up  # Ensure Postgres is running
```

**"relation does not exist"**
```bash
make db-migrate  # Run migrations
```

**"port 5432 already in use"**
```bash
# Stop existing Postgres or change port in docker-compose.yml
```

## Configuration Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | - | Postgres connection string (required for Postgres mode) |
| `USE_INMEMORY_STORE` | `false` | Set to `true` to disable Postgres |
| `SENTIENTD_LISTEN_ADDR` | `:8080` | HTTP server address |
| `SENTIENTD_KAGENT_URL` | `http://kagent:8080` | kagent service URL |
| `SENTIENTD_ARGO_NAMESPACE` | `argo` | Kubernetes namespace for Argo Workflows |
| `SENTIENTD_SLACK_WEBHOOK_URL` | - | Slack webhook for notifications |
| `SENTIENTD_INITIAL_WINDOW` | `30s` | Initial correlation window |
| `SENTIENTD_EXPANSION_WINDOW` | `5m` | Correlation expansion window |

---

**You're ready to go!** 🎉

For production deployment, see the Kubernetes manifests in Milestone 2.
