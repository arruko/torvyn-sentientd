# Kubernetes Deployment Guide

This directory contains Kubernetes manifests for deploying sentientd in production.

## Overview

sentientd runs as a **Deployment** in its own namespace with:
- **ServiceAccount** with RBAC permissions for:
  - Reading ServiceGraph CRDs
  - Creating/managing Argo Workflows
  - Optional: Reading Pods/Events for debugging
- **PostgreSQL** database for incident storage
- **Argo Workflows** for DAG execution
- **kagent** integration for AI analysis

## Prerequisites

1. **Kubernetes cluster** (1.24+)
2. **Argo Workflows** installed and running
3. **PostgreSQL** database (can be in-cluster or external)
4. **kubectl** and **kustomize** CLI tools

## Quick Start

### 1. Install with Kustomize

```bash
# Review manifests
kubectl kustomize deploy/kubernetes

# Apply manifests
kubectl apply -k deploy/kubernetes

# Verify deployment
kubectl -n sentientd get pods
kubectl -n sentientd logs -l app.kubernetes.io/name=sentientd
```

### 2. Configure Secrets

**Edit `deploy/kubernetes/secret.yaml`** with actual credentials:

```bash
# Generate base64-encoded secrets
echo -n "postgres://user:pass@host:5432/db" | base64
echo -n "ghp_YourGitHubToken" | base64
echo -n "https://hooks.slack.com/..." | base64

# Apply secrets
kubectl apply -f deploy/kubernetes/secret.yaml
```

**Or use kubectl create secret:**

```bash
kubectl create secret generic sentientd-secrets \
  --from-literal=DATABASE_URL="postgres://..." \
  --from-literal=GITHUB_TOKEN="ghp_..." \
  --from-literal=SLACK_WEBHOOK_URL="https://..." \
  -n sentientd
```

### 3. Verify Deployment

```bash
# Check pod status
kubectl -n sentientd get pods

# Check logs
kubectl -n sentientd logs -f deployment/sentientd

# Check RBAC
kubectl -n sentientd auth can-i create workflows.argoproj.io --as=system:serviceaccount:sentientd:sentientd
# Expected: yes

# Port-forward for testing
kubectl -n sentientd port-forward svc/sentientd 8080:8080

# Test health endpoint
curl http://localhost:8080/healthz
# Expected: ok
```

## Architecture

### Components

```
┌─────────────────────────────────────────────────┐
│              Alertmanager                        │
│         (sends webhook to sentientd)            │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────────┐
│            sentientd Pod                          │
│  ┌────────────────────────────────────────────┐  │
│  │  ServiceAccount: sentientd                 │  │
│  │  RBAC: read ServiceGraph, create Workflows │  │
│  └────────────────────────────────────────────┘  │
│                                                   │
│  Correlates alerts → Creates incidents →         │
│  Triggers investigation → Creates Workflows      │
└──┬─────────┬─────────┬──────────┬───────────────┘
   │         │         │          │
   │         │         │          │
   ▼         ▼         ▼          ▼
┌────┐  ┌────────┐ ┌────────┐ ┌──────────┐
│ DB │  │ kagent │ │  Argo  │ │  Slack   │
│    │  │        │ │Workflow│ │          │
└────┘  └────────┘ └────────┘ └──────────┘
```

### RBAC Permissions

sentientd requires these permissions in the `sentientd` namespace:

| Resource | API Group | Verbs | Purpose |
|----------|-----------|-------|---------|
| `servicegraphs` | `torvyn.io` | `get`, `list`, `watch` | Read topology for correlation |
| `workflows` | `argoproj.io` | `create`, `get`, `list`, `watch`, `update`, `patch` | Create and monitor remediation workflows |
| `pods`, `events` | core | `get`, `list`, `watch` | Optional debugging |

## Configuration

### Environment Variables

All configuration is via environment variables (see [deployment.yaml](deployment.yaml)):

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | - | **Required**: Postgres connection string |
| `SENTIENTD_LISTEN_ADDR` | `:8080` | HTTP server listen address |
| `SENTIENTD_KAGENT_URL` | `http://kagent.sentientd.svc.cluster.local:8080` | kagent service URL |
| `SENTIENTD_ARGO_NAMESPACE` | `argo` | Namespace where Argo Workflows runs |
| `SENTIENTD_INITIAL_WINDOW` | `30s` | Initial correlation time window |
| `SENTIENTD_EXPANSION_WINDOW` | `5m` | Correlation expansion window |
| `SENTIENTD_SLACK_WEBHOOK_URL` | - | Optional: Slack webhook for notifications |
| `GITHUB_TOKEN` | - | Optional: GitHub PAT for PR creation |

### Resource Requests/Limits

Default resource allocation:

```yaml
resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

Adjust based on your incident volume:
- **Low volume** (< 100 incidents/day): Use defaults
- **Medium volume** (100-1000 incidents/day): Double memory to 512Mi/1Gi
- **High volume** (> 1000 incidents/day): Consider horizontal scaling (replicas: 2+)

## Database Setup

### Option 1: External PostgreSQL

Use an external managed database (recommended for production):

```yaml
# In secret.yaml
DATABASE_URL: "postgres://sentientd:password@postgres.example.com:5432/sentientd?sslmode=require"
```

### Option 2: In-Cluster PostgreSQL

Deploy Postgres in Kubernetes:

```bash
# Using Bitnami Helm chart
helm install postgres bitnami/postgresql \
  --namespace sentientd \
  --set auth.username=sentientd \
  --set auth.password=sentientd \
  --set auth.database=sentientd

# Update secret.yaml
DATABASE_URL: "postgres://sentientd:sentientd@postgres-postgresql.sentientd.svc.cluster.local:5432/sentientd?sslmode=disable"
```

### Run Migrations

Migrations must run before sentientd starts:

```bash
# Option 1: Kubernetes Job
kubectl create -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: sentientd-migrate
  namespace: sentientd
spec:
  template:
    spec:
      containers:
      - name: migrate
        image: migrate/migrate
        args:
          - -path=/migrations
          - -database=\$(DATABASE_URL)
          - up
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: sentientd-secrets
              key: DATABASE_URL
        volumeMounts:
        - name: migrations
          mountPath: /migrations
      volumes:
      - name: migrations
        configMap:
          name: sentientd-migrations
      restartPolicy: Never
EOF

# Option 2: Manual (from local machine)
export DATABASE_URL="postgres://..."
make db-migrate
```

## Argo Workflows Integration

sentientd creates Argo Workflows dynamically for each incident remediation.

### Prerequisites

1. **Argo Workflows** installed:
   ```bash
   kubectl create namespace argo
   kubectl apply -n argo -f https://github.com/argoproj/argo-workflows/releases/latest/download/install.yaml
   ```

2. **DAG Runner Container Image** (TODO: Build this):
   ```bash
   # Build torvyn-dag-runner image
   docker build -t ghcr.io/arruko/torvyn-dag-runner:latest ./dag-runner
   docker push ghcr.io/arruko/torvyn-dag-runner:latest
   ```

3. **ServiceAccount for Workflows**:
   ```bash
   kubectl create serviceaccount argo-workflow -n argo
   # Grant necessary RBAC permissions for remediation tasks
   ```

### Workflow Creation Flow

1. **Alert ingested** → Incident created
2. **Investigation triggered** → kagent analyzes
3. **DAG generated** → Converted to Argo Workflow
4. **Workflow submitted** → Argo executes DAG
5. **Status monitored** → sentientd tracks progress

Example generated Workflow:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  name: remediation-incident-123-1706012345
  namespace: argo
  labels:
    torvyn.io/incident-id: incident-123
spec:
  serviceAccountName: argo-workflow
  entrypoint: remediation-dag
  templates:
    - name: remediation-dag
      dag:
        tasks:
          - name: scale-deployment
            template: dag-task-runner
            arguments:
              parameters:
                - name: task_type
                  value: kubectl
                - name: payload
                  value: "scale deployment api-server --replicas=3"
```

## Monitoring & Observability

### Health Checks

```bash
# Liveness probe
curl http://sentientd:8080/healthz

# Readiness probe (same endpoint for v1)
curl http://sentientd:8080/healthz
```

### Logs

```bash
# Stream logs
kubectl -n sentientd logs -f deployment/sentientd

# Search for errors
kubectl -n sentientd logs deployment/sentientd | grep ERROR

# Follow specific incident
kubectl -n sentientd logs deployment/sentientd | grep "incident-123"
```

### Metrics (TODO: Implement in Milestone 4)

```bash
# Prometheus metrics endpoint (planned)
curl http://sentientd:8080/metrics
```

## Scaling

### Horizontal Scaling

For high incident volume:

```yaml
# In deployment.yaml
spec:
  replicas: 3  # Increase replicas
```

**Note**: Current v1 uses polling, which may cause duplicate investigations with multiple replicas. Switch to event-driven mode (NATS/Kafka) for true horizontal scaling.

### Vertical Scaling

Increase resources:

```yaml
resources:
  requests:
    cpu: 500m
    memory: 512Mi
  limits:
    cpu: 1000m
    memory: 1Gi
```

## Security Best Practices

### 1. Network Policies

Restrict traffic to sentientd:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: sentientd-network-policy
  namespace: sentientd
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: sentientd
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
      - namespaceSelector:
          matchLabels:
            name: monitoring  # Alertmanager
      ports:
        - protocol: TCP
          port: 8080
  egress:
    - to:
      - namespaceSelector:
          matchLabels:
            name: sentientd  # Postgres, kagent
    - to:
      - namespaceSelector:
          matchLabels:
            name: argo  # Argo Workflows
```

### 2. Pod Security

The Deployment already includes:
- `runAsNonRoot: true`
- `readOnlyRootFilesystem: true`
- `allowPrivilegeEscalation: false`
- Drop all capabilities

### 3. Secrets Management

Use **External Secrets Operator** or **Sealed Secrets**:

```bash
# Example with External Secrets
kubectl apply -f - <<EOF
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: sentientd-secrets
  namespace: sentientd
spec:
  secretStoreRef:
    name: vault
    kind: SecretStore
  target:
    name: sentientd-secrets
  data:
    - secretKey: DATABASE_URL
      remoteRef:
        key: sentientd/database-url
EOF
```

## Troubleshooting

### Pod not starting

```bash
kubectl -n sentientd describe pod -l app.kubernetes.io/name=sentientd
kubectl -n sentientd logs -l app.kubernetes.io/name=sentientd
```

Common issues:
- **Database connection failed**: Check `DATABASE_URL` in secret
- **ImagePullBackOff**: Update image in kustomization.yaml
- **CrashLoopBackOff**: Check logs for application errors

### RBAC permission denied

```bash
# Test ServiceAccount permissions
kubectl -n sentientd auth can-i create workflows.argoproj.io \
  --as=system:serviceaccount:sentientd:sentientd

# If denied, verify Role and RoleBinding
kubectl -n sentientd get role sentientd -o yaml
kubectl -n sentientd get rolebinding sentientd -o yaml
```

### Workflows not being created

```bash
# Check if Argo client initialized
kubectl -n sentientd logs deployment/sentientd | grep "Argo Workflows client initialized"

# Check Argo Workflows is running
kubectl -n argo get pods

# Verify RBAC permissions
kubectl -n argo auth can-i create workflows --as=system:serviceaccount:sentientd:sentientd
```

## Uninstall

```bash
# Delete all resources
kubectl delete -k deploy/kubernetes

# Or manually
kubectl delete namespace sentientd
```

## Next Steps

- [ ] Build and push sentientd container image
- [ ] Build torvyn-dag-runner container image
- [ ] Configure Alertmanager webhook to point to sentientd
- [ ] Deploy ServiceGraph CRD and operator
- [ ] Set up monitoring dashboards
- [ ] Configure GitOps (Flux/ArgoCD) for deployments

---

**Status**: Milestone 2 Step 1 Complete - Kubernetes manifests ready for deployment
