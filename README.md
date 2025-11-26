# torvyn-sentientd

**Autonomous incident response platform for Kubernetes** - Correlates alerts, performs root cause analysis, and executes remediation workflows with policy-driven safety guardrails.

[![CI](https://github.com/arruko/torvyn-sentientd/actions/workflows/ci.yml/badge.svg)](https://github.com/arruko/torvyn-sentientd/actions/workflows/ci.yml)
[![Security](https://github.com/arruko/torvyn-sentientd/actions/workflows/security.yml/badge.svg)](https://github.com/arruko/torvyn-sentientd/actions/workflows/security.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/arruko/torvyn-sentientd)](https://goreportcard.com/report/github.com/arruko/torvyn-sentientd)

---

## Features

### 🔍 **Intelligent Alert Correlation**
- Time-window based alert correlation
- Service topology-aware grouping
- Automatic incident creation and deduplication

### 🤖 **Autonomous Investigation**
- AI-powered root cause analysis via kagent
- Service dependency graph integration
- Confidence-scored remediation plans

### 🛡️ **Policy-Driven Safety**
- OPA (Open Policy Agent) policy enforcement
- Production safety guardrails
- Multi-layer approval workflows
- Forbidden tool blocking (`k8s.exec`, `k8s.patch`, etc.)

### ⚡ **Reliable Execution**
- Argo Workflows for DAG execution
- Hardened DAG runner with runtime guardrails
- Script execution disabled by default
- Comprehensive structured logging

### 🔐 **Enterprise Security**
- SPIFFE/SPIRE workload identity
- Non-root containers with read-only filesystems
- Seccomp profiles and capability dropping
- Full Trivy security compliance

### 📊 **Async Processing**
- NATS JetStream message queue
- Graceful degradation (queue optional)
- Retry logic with acknowledgments

---

## Architecture

```
Alertmanager → sentientd webhook → Correlator → Incident Store
                         ↓
                    NATS Queue (optional)
                         ↓
                   Ingest Worker

Incident (state=open) → Investigation Engine
                         ↓
                    kagent (RCA + DAG)
                         ↓
                    OPA Policy Engine
                         ↓
                 ✅ Allow / ❌ Deny
                         ↓
                 Argo Workflow Creation
                         ↓
                   DAG Runner Pods
                         ↓
                 Task Execution (with safety guardrails)
```

---

## Quick Start

### Prerequisites

- Kubernetes cluster (Kind, GKE, EKS, AKS)
- PostgreSQL (for production) or in-memory store (for dev)
- Argo Workflows
- Optional: NATS JetStream, SPIRE

### Deploy to Kind (Development)

```bash
# Create Kind cluster
kind create cluster --name sentientd

# Deploy Argo Workflows
kubectl create namespace argo
kubectl apply -n argo -f https://github.com/argoproj/argo-workflows/releases/download/v3.5.11/install.yaml

# Deploy PostgreSQL
kubectl create namespace sentientd
kubectl apply -f deploy/postgres/

# Deploy NATS JetStream (optional)
kubectl apply -f deploy/nats/nats-jetstream.yaml

# Deploy SPIRE (optional)
kubectl apply -f deploy/spire/spire-server.yaml
kubectl apply -f deploy/spire/spire-agent.yaml

# Build and deploy sentientd
make docker-build
kind load docker-image ghcr.io/arruko/torvyn-sentientd:latest --name sentientd
kubectl apply -f deploy/sentientd/

# Build and load DAG runner
make dag-runner-build
kind load docker-image ghcr.io/arruko/torvyn-dag-runner:latest --name sentientd
```

### Send Test Alert

```bash
kubectl -n sentientd port-forward svc/sentientd 8080:8080

curl -X POST http://localhost:8080/alertmanager/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "alerts": [{
      "fingerprint": "test-001",
      "labels": {
        "alertname": "HighCPU",
        "severity": "critical",
        "service": "payment-api",
        "cluster": "kind-sentientd",
        "namespace": "payments",
        "environment": "dev"
      },
      "annotations": {
        "description": "CPU usage above 90%"
      },
      "startsAt": "2025-01-24T12:00:00Z"
    }]
  }'
```

---

## Configuration

sentientd is configured via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `SENTIENTD_LISTEN_ADDR` | HTTP server address | `:8080` |
| `SENTIENTD_INITIAL_WINDOW` | Initial correlation window | `30s` |
| `SENTIENTD_EXPANSION_WINDOW` | Window expansion time | `5m` |
| `SENTIENTD_KAGENT_URL` | kagent API endpoint | `` |
| `SENTIENTD_ARGO_NAMESPACE` | Argo Workflows namespace | `argo` |
| `SENTIENTD_SLACK_WEBHOOK_URL` | Slack webhook for notifications | `` |
| `SENTIENTD_ENVIRONMENT` | Environment (dev/staging/prod) | `dev` |
| `DATABASE_URL` | PostgreSQL connection string | `` |
| `USE_INMEMORY_STORE` | Use in-memory store instead of DB | `false` |
| `NATS_URL` | NATS server URL | `` |
| `NATS_ENABLED` | Enable NATS queue | `false` |
| `POLICY_DIR` | OPA policy directory | `internal/policy/policies` |

---

## Security

### Production-Ready Security Hardening

This project follows strict security best practices:

- ✅ **Non-root containers** (UID 1000)
- ✅ **Read-only root filesystems**
- ✅ **Dropped capabilities** (all non-essential)
- ✅ **Seccomp profiles** enabled
- ✅ **Trivy security scanning** in CI
- ✅ **golangci-lint** code quality checks
- ✅ **OPA policy enforcement** at runtime

See [SECURITY_HARDENING.md](SECURITY_HARDENING.md) for detailed security documentation.

### Trivy Compliance

All security vulnerabilities have been addressed. The [`.trivyignore.yaml`](.trivyignore.yaml) file documents intentional exceptions for SPIRE Agent (required for workload attestation).

```bash
# Run security scan
trivy config deploy/
```

---

## Policy Examples

### Forbidden Tools (Always Blocked)

```rego
forbidden_tools := {
    "k8s.exec",
    "k8s.patchResource",
    "script.shell",
    "k8s.applyManifest"
}
```

### Production Approval Requirement

```rego
deny[msg] {
    input.incident.environment == "prod"
    not all_tasks_have_approval
    msg := "production incidents require approval for all remediation tasks"
}
```

### Critical Tool Protection

```rego
deny[msg] {
    input.incident.environment == "prod"
    some task
    task := input.dag.tasks[_]
    critical_tools[task.tool]
    not task.requiresApproval
    msg := sprintf("critical tool %s requires approval in production", [task.tool])
}
```

---

## DAG Runner Safety

The DAG runner includes runtime safety guardrails:

### Script Execution Disabled by Default

```go
allowScript := os.Getenv("ALLOW_SCRIPT")
if allowScript != "true" {
    return fmt.Errorf("script tool disabled by runtime policy")
}
```

### Security Context

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL
```

---

## Development

### Prerequisites

- Go 1.23+
- Docker
- Kind (for local Kubernetes)
- golangci-lint
- Trivy

### Build

```bash
# Build sentientd
make build

# Build DAG runner
make dag-runner-build

# Run tests
make test

# Run linter
make lint

# Security scan
make security-scan
```

### Testing

The project includes a comprehensive unit test suite across all components. Tests are pure unit tests with no external dependencies (no K8s API, Postgres, NATS, or OPA server required).

```bash
# Run all tests with race detector
go test ./... -race

# Run tests with coverage
go test ./... -race -coverprofile=coverage.out

# View coverage report
go tool cover -html=coverage.out

# Run tests for specific package
go test ./internal/correlation/... -v -race
```

#### Test Coverage by Component

| Component | Test File | Coverage |
|-----------|-----------|----------|
| **Storage Layer** | `internal/store/store_test.go` | Incident storage, correlation windows, concurrent access |
| **Alert Correlation** | `internal/correlation/correlator_test.go` | Correlation logic, severity escalation, topology integration |
| **HTTP Handlers** | `internal/httpserver/handlers_test.go` | Webhook handling, timestamp parsing, queue integration |
| **Topology** | `internal/topology/memory_graph_test.go` | Graph operations, subgraph extraction, label matching |
| **Argo Client** | `internal/argo/client_test.go` | DAG to Workflow conversion, dependencies, security context |
| **OPA Policy Engine** | `internal/policy/engine_test.go` | Allow/deny policies, conditional rules, forbidden tools |
| **Investigation Engine** | `internal/investigation/engine_test.go` | Engine initialization, nil handling |
| **DAG Runner** | `cmd/dag-runner/main_test.go` | Task execution, tool routing, ALLOW_SCRIPT guardrail |

#### Test Characteristics

- ✅ **Pure Unit Tests**: No external dependencies
- ✅ **Table-Driven Tests**: Using `t.Run()` for parallel execution
- ✅ **Race Detector**: All tests run with `-race` flag
- ✅ **Mock Implementations**: Custom mocks for all interfaces
- ✅ **Parallel Execution**: Most tests run in parallel with `t.Parallel()`
- ✅ **Concurrency Tests**: Validating thread-safe operations

#### Known Limitations

1. **Argo Client Tests**: Limited validation of nested unstructured K8s objects due to deep copy limitations in K8s SDK
2. **Investigation Engine Tests**: Uses concrete types instead of interfaces (refactoring needed for full mock support)

#### CI/CD Integration

Tests run automatically in GitHub Actions on every push and pull request:

```yaml
# See .github/workflows/tests.yaml
- run: go test ./... -race -coverprofile=coverage.out
```

### Project Structure

```
torvyn-sentientd/
├── cmd/
│   ├── main.go              # sentientd entrypoint
│   └── dag-runner/          # DAG task executor
├── internal/
│   ├── argo/                # Argo Workflows client
│   ├── correlation/         # Alert correlation engine
│   ├── database/            # PostgreSQL client
│   ├── domain/              # Core domain models
│   ├── httpserver/          # HTTP API server
│   ├── identity/            # SPIFFE/SPIRE integration
│   ├── ingest/              # NATS queue worker
│   ├── investigation/       # Investigation engine
│   ├── kagent/              # kagent API client
│   ├── policy/              # OPA policy engine
│   ├── queue/               # Queue abstraction
│   ├── slack/               # Slack notifications
│   ├── store/               # Incident persistence
│   └── topology/            # ServiceGraph integration
├── deploy/
│   ├── nats/                # NATS JetStream manifests
│   ├── postgres/            # PostgreSQL manifests
│   ├── spire/               # SPIRE Server + Agent
│   └── sentientd/           # sentientd deployment
└── docs/
    ├── MILESTONE4_PLAN.md   # Milestone 4 implementation guide
    └── SECURITY_HARDENING.md # Security documentation
```

---

## Milestones

### ✅ Milestone 4: Secure Identity + Queue + Policy Foundation (COMPLETE)

- ✅ SPIFFE/SPIRE workload identity
- ✅ NATS JetStream async processing
- ✅ OPA policy engine with production safety rules
- ✅ Hardened DAG runner with runtime guardrails
- ✅ Full security hardening (Trivy + golangci-lint clean)

See [MILESTONE4_PLAN.md](MILESTONE4_PLAN.md) for details.

### 🔜 Milestone 5 Candidates

1. **Production Deployment** - Helm chart, multi-environment config
2. **Observability** - Prometheus metrics, OpenTelemetry tracing
3. **GitHub Integration** - PR creation for remediation plans
4. **Advanced Policies** - Custom policies per team/service
5. **Chaos Testing** - Failure simulation and resilience verification

---

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Code Quality Standards

- All code must pass `golangci-lint`
- Security scans must be clean (Trivy)
- Tests required for new features
- Security-sensitive changes require review

---

## License

[MIT License](LICENSE)

---

## Support

- **Issues**: [GitHub Issues](https://github.com/arruko/torvyn-sentientd/issues)
- **Discussions**: [GitHub Discussions](https://github.com/arruko/torvyn-sentientd/discussions)
- **Security**: See [SECURITY.md](SECURITY.md) for reporting vulnerabilities

---

**Built with ❤️ for reliable, secure, autonomous incident response**
