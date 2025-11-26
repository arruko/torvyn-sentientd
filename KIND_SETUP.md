# Kind Cluster Setup Guide

This guide provides a complete setup for running sentientd in a local Kind (Kubernetes IN Docker) cluster for development and testing.

## Prerequisites

- Docker Desktop or Docker Engine
- kubectl (v1.24+)
- Kind (v0.20+)
- Helm (v3.12+)
- golang-migrate CLI

### Install Prerequisites (macOS)

```bash
# Install Kind
brew install kind

# Install kubectl
brew install kubectl

# Install Helm
brew install helm

# Install golang-migrate
brew install golang-migrate
```

---

## Quick Start

```bash
# 1. Create Kind cluster with Argo and sentientd
make dev-kind

# 2. Build and load sentientd image
make kind-load

# 3. Deploy sentientd
kubectl apply -k deploy/kubernetes

# 4. Verify
kubectl -n sentientd get pods
```

---

## Step-by-Step Setup

### 1. Create Kind Cluster

```bash
# Create cluster with custom configuration
cat <<EOF | kind create cluster --name sentientd --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
    extraPortMappings:
      - containerPort: 80
        hostPort: 80
        protocol: TCP
      - containerPort: 443
        hostPort: 443
        protocol: TCP
EOF
```

### 2. Install Argo Workflows

```bash
# Create argo namespace
kubectl create namespace argo

# Install Argo Workflows
kubectl apply -n argo -f https://github.com/argoproj/argo-workflows/releases/download/v3.5.5/install.yaml

# Wait for Argo to be ready
kubectl -n argo wait --for=condition=ready pod -l app=workflow-controller --timeout=300s

# Verify
kubectl -n argo get pods
```

### 3. Install PostgreSQL

```bash
# Add Bitnami Helm repo
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# Install PostgreSQL
helm install postgres bitnami/postgresql \
  --namespace sentientd \
  --create-namespace \
  --set auth.username=sentientd \
  --set auth.password=sentientd \
  --set auth.database=sentientd \
  --set primary.persistence.size=1Gi

# Wait for Postgres to be ready
kubectl -n sentientd wait --for=condition=ready pod -l app.kubernetes.io/name=postgresql --timeout=180s

# Get Postgres password
export POSTGRES_PASSWORD=$(kubectl get secret --namespace sentientd postgres-postgresql -o jsonpath="{.data.password}" | base64 -d)
```

### 4. Run Database Migrations

```bash
# Port-forward Postgres
kubectl -n sentientd port-forward svc/postgres-postgresql 5432:5432 &

# Run migrations
export DATABASE_URL="postgres://sentientd:sentientd@localhost:5432/sentientd?sslmode=disable"
make db-migrate

# Stop port-forward
pkill -f "port-forward svc/postgres-postgresql"
```

### 5. Deploy SPIRE (Workload Identity)

```bash
# Deploy SPIRE Server
kubectl apply -f deploy/spire/spire-server.yaml

# Wait for SPIRE Server to be ready
kubectl -n spire wait --for=condition=ready pod -l app=spire-server --timeout=120s

# Deploy SPIRE Agent (DaemonSet)
kubectl apply -f deploy/spire/spire-agent.yaml

# Wait for SPIRE Agents to be ready
kubectl -n spire wait --for=condition=ready pod -l app=spire-agent --timeout=120s

# Verify SPIRE is running
kubectl -n spire get pods
```

See [deploy/spire/README.md](deploy/spire/README.md) for details on registering workload identities.

### 6. Deploy NATS JetStream (Optional)

```bash
# Deploy NATS JetStream
kubectl apply -f deploy/nats/nats-jetstream.yaml

# Wait for NATS to be ready
kubectl -n nats wait --for=condition=ready pod -l app=nats --timeout=120s

# Verify NATS is running
kubectl -n nats get pods
kubectl -n nats get svc
```

See [deploy/nats/README.md](deploy/nats/README.md) for NATS configuration and monitoring details.

**Note**: NATS is optional. sentientd works with or without it. When enabled via `NATS_ENABLED=true`, alerts are published to the queue for async processing in addition to synchronous correlation.

### 7. Install ServiceGraph CRD

```bash
# Install the ServiceGraph CRD
kubectl apply -f crds/servicegraph.torvyn.io_servicegraphs.yaml

# Create example ServiceGraph
kubectl apply -f crds/examples/servicegraph-example.yaml

# Verify
kubectl -n sentientd get servicegraphs
```

### 8. Build and Load sentientd Image

```bash
# Build sentientd image
docker build -t sentientd:latest .

# Load image into Kind
kind load docker-image sentientd:latest --name sentientd

# Build DAG runner image
docker build -t ghcr.io/arruko/torvyn-dag-runner:latest -f cmd/dag-runner/Dockerfile .

# Load DAG runner into Kind
kind load docker-image ghcr.io/arruko/torvyn-dag-runner:latest --name sentientd
```

### 9. Update Secrets

```bash
# Create sentientd secrets
kubectl create secret generic sentientd-secrets \
  --from-literal=DATABASE_URL="postgres://sentientd:sentientd@postgres-postgresql.sentientd.svc.cluster.local:5432/sentientd?sslmode=disable" \
  --from-literal=NATS_URL="nats://nats.nats.svc.cluster.local:4222" \
  --from-literal=NATS_ENABLED="true" \
  --from-literal=GITHUB_TOKEN="" \
  --from-literal=SLACK_WEBHOOK_URL="" \
  -n sentientd \
  --dry-run=client -o yaml | kubectl apply -f -
```

### 10. Deploy sentientd

```bash
# Apply all manifests
kubectl apply -k deploy/kubernetes

# Wait for sentientd to be ready
kubectl -n sentientd wait --for=condition=ready pod -l app.kubernetes.io/name=sentientd --timeout=120s

# Check logs
kubectl -n sentientd logs -f deployment/sentientd
```

Expected output:
```
INFO: Connected to database successfully (max_conns=25, min_conns=5)
INFO: Using Postgres incident store
INFO: ServiceGraph client initialized for namespace: sentientd
INFO: ServiceGraph loaded: 8 nodes, 7 edges
INFO: Argo Workflows client initialized for namespace: argo
INFO: Starting HTTP server on :8080
```

---

## Testing the Full Stack

### 1. Send Test Alert

```bash
# Port-forward sentientd
kubectl -n sentientd port-forward svc/sentientd 8080:8080 &

# Send alert
curl -X POST http://localhost:8080/alertmanager/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "alerts": [{
      "fingerprint": "test-alert-001",
      "labels": {
        "alertname": "HighCPU",
        "severity": "critical",
        "service": "payment-api",
        "cluster": "kind-sentientd",
        "namespace": "payments"
      },
      "annotations": {
        "description": "CPU usage above 90%",
        "summary": "High CPU on payment-api"
      },
      "startsAt": "2025-01-23T12:00:00Z"
    }]
  }'
```

### 2. Verify Incident Created

```bash
# Check database
kubectl -n sentientd exec -it deployment/postgres-postgresql -- \
  psql -U sentientd -c "SELECT id, service, severity, state FROM incidents;"
```

### 3. Check Argo Workflows

```bash
# List workflows
kubectl -n argo get workflows

# Describe a workflow
kubectl -n argo describe workflow <workflow-name>

# Watch workflow progress
kubectl -n argo get workflows -w
```

### 4. View Logs

```bash
# sentientd logs
kubectl -n sentientd logs -f deployment/sentientd

# Argo workflow logs
kubectl -n argo logs -f <workflow-pod-name>
```

---

## Cleanup

### Remove sentientd

```bash
kubectl delete -k deploy/kubernetes
```

### Remove Everything

```bash
# Delete Kind cluster
kind delete cluster --name sentientd
```

---

## Makefile Targets

Add these targets to your Makefile:

```makefile
.PHONY: dev-kind kind-load kind-deploy kind-clean

dev-kind: ## Create Kind cluster with full stack
	@echo "Creating Kind cluster..."
	kind create cluster --name sentientd
	@echo "Installing Argo Workflows..."
	kubectl create namespace argo
	kubectl apply -n argo -f https://github.com/argoproj/argo-workflows/releases/download/v3.5.5/install.yaml
	@echo "Installing PostgreSQL..."
	helm repo add bitnami https://charts.bitnami.com/bitnami || true
	helm repo update
	helm install postgres bitnami/postgresql \
	  --namespace sentientd \
	  --create-namespace \
	  --set auth.username=sentientd \
	  --set auth.password=sentientd \
	  --set auth.database=sentientd \
	  --set primary.persistence.size=1Gi
	@echo "Waiting for Postgres..."
	kubectl -n sentientd wait --for=condition=ready pod -l app.kubernetes.io/name=postgresql --timeout=180s
	@echo "Installing ServiceGraph CRD..."
	kubectl apply -f crds/servicegraph.torvyn.io_servicegraphs.yaml
	@echo "Kind cluster ready!"

kind-load: ## Build and load images into Kind
	docker build -t sentientd:latest .
	docker build -t ghcr.io/arruko/torvyn-dag-runner:latest -f cmd/dag-runner/Dockerfile .
	kind load docker-image sentientd:latest --name sentientd
	kind load docker-image ghcr.io/arruko/torvyn-dag-runner:latest --name sentientd
	@echo "Images loaded into Kind cluster"

kind-deploy: ## Deploy sentientd to Kind cluster
	kubectl apply -k deploy/kubernetes
	@echo "Waiting for sentientd..."
	kubectl -n sentientd wait --for=condition=ready pod -l app.kubernetes.io/name=sentientd --timeout=120s
	@echo "sentientd deployed!"

kind-clean: ## Delete Kind cluster
	kind delete cluster --name sentientd
```

---

## Troubleshooting

### Postgres Connection Issues

```bash
# Check if Postgres is running
kubectl -n sentientd get pods -l app.kubernetes.io/name=postgresql

# Check Postgres logs
kubectl -n sentientd logs -l app.kubernetes.io/name=postgresql

# Test connection
kubectl -n sentientd exec -it deployment/postgres-postgresql -- psql -U sentientd -c '\l'
```

### ServiceGraph Not Loading

```bash
# Check if CRD is installed
kubectl get crd servicegraphs.torvyn.io

# Check ServiceGraph resource
kubectl -n sentientd get servicegraphs

# Check sentientd logs for ServiceGraph errors
kubectl -n sentientd logs deployment/sentientd | grep ServiceGraph
```

### Argo Workflows Not Creating

```bash
# Check RBAC permissions
kubectl -n sentientd auth can-i create workflows.argoproj.io \
  --as=system:serviceaccount:sentientd:sentientd

# Check Argo controller logs
kubectl -n argo logs deployment/workflow-controller

# List all workflows
kubectl -n argo get workflows --all-namespaces
```

### Image Pull Issues

```bash
# Verify images are loaded
docker exec -it sentientd-control-plane crictl images | grep sentientd

# Reload images if needed
make kind-load
```

---

## Development Workflow

### Rapid Iteration

```bash
# 1. Make code changes

# 2. Rebuild and reload
make build
make kind-load

# 3. Restart sentientd
kubectl -n sentientd rollout restart deployment/sentientd

# 4. Watch logs
kubectl -n sentientd logs -f deployment/sentientd
```

### Testing Correlation

```bash
# Send multiple alerts for the same service
for i in {1..3}; do
  curl -X POST http://localhost:8080/alertmanager/webhook \
    -H "Content-Type: application/json" \
    -d "{
      \"alerts\": [{
        \"fingerprint\": \"alert-$i\",
        \"labels\": {
          \"service\": \"payment-api\",
          \"severity\": \"critical\"
        },
        \"annotations\": {},
        \"startsAt\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"
      }]
    }"
  sleep 2
done

# Check if correlated into single incident
kubectl -n sentientd exec deployment/postgres-postgresql -- \
  psql -U sentientd -c "SELECT id, service, COUNT(*) as alert_count FROM incidents JOIN alerts ON incidents.id = alerts.incident_id GROUP BY incidents.id, service;"
```

---

## Next Steps

After successfully running in Kind:
1. Test end-to-end alert → incident → workflow flow
2. Verify ServiceGraph topology correlation
3. Test DAG execution with different task types
4. Add integration tests
5. Prepare for production deployment

---

**Cluster Ready!** 🎉 You now have a complete local Kubernetes environment for sentientd development.
