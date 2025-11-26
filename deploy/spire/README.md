# SPIRE Deployment for sentientd (Kind/Dev)

This directory contains SPIRE Server and Agent deployment manifests for local development with Kind.

## Overview

SPIRE (SPIFFE Runtime Environment) provides workload identity attestation and X.509-SVID issuance for mTLS communication between sentientd components.

**Trust Domain**: `torvyn.local`

## Components

- **SPIRE Server**: Issues SVIDs and manages trust bundles
- **SPIRE Agent**: Runs as DaemonSet, provides Workload API on each node

## Deployment

Apply the manifests in order:

```bash
# 1. Deploy SPIRE Server
kubectl apply -f deploy/spire/spire-server.yaml

# 2. Wait for server to be ready
kubectl -n spire wait --for=condition=ready pod -l app=spire-server --timeout=120s

# 3. Deploy SPIRE Agent
kubectl apply -f deploy/spire/spire-agent.yaml

# 4. Wait for agents to be ready
kubectl -n spire wait --for=condition=ready pod -l app=spire-agent --timeout=120s
```

## Verification

Check that SPIRE components are running:

```bash
# Check server
kubectl -n spire get statefulset spire-server
kubectl -n spire logs -l app=spire-server --tail=50

# Check agents
kubectl -n spire get daemonset spire-agent
kubectl -n spire logs -l app=spire-agent --tail=50
```

## Registering Workload Identities

After deploying your workloads, you need to register them with SPIRE to receive SVIDs.

Example: Register sentientd

```bash
kubectl -n spire exec -it spire-server-0 -- \
  /opt/spire/bin/spire-server entry create \
  -spiffeID spiffe://torvyn.local/sentientd \
  -parentID spiffe://torvyn.local/spire/agent/k8s_psat/kind-sentientd/node \
  -selector k8s:ns:sentientd \
  -selector k8s:sa:sentientd
```

Example: Register DAG Runner

```bash
kubectl -n spire exec -it spire-server-0 -- \
  /opt/spire/bin/spire-server entry create \
  -spiffeID spiffe://torvyn.local/dag-runner \
  -parentID spiffe://torvyn.local/spire/agent/k8s_psat/kind-sentientd/node \
  -selector k8s:ns:argo \
  -selector k8s:pod-label:app:dag-runner
```

## Accessing Workload API

Workloads can access the SPIRE Agent's Workload API via the Unix domain socket:

- **Socket Path**: `/run/spire/sockets/agent.sock`
- **Environment Variable**: `SPIFFE_ENDPOINT_SOCKET=unix:///run/spire/sockets/agent.sock`

To mount the socket in your workload pods, add this volume configuration:

```yaml
volumes:
  - name: spire-agent-socket
    hostPath:
      path: /run/spire/sockets
      type: Directory

volumeMounts:
  - name: spire-agent-socket
    mountPath: /run/spire/sockets
    readOnly: true
```

## Configuration Notes

### Trust Domain
- **Dev/Kind**: `torvyn.local`
- **Production**: Should use actual domain (e.g., `torvyn.io`)

### Node Attestation
- Uses `k8s_psat` (Kubernetes Projected Service Account Token)
- Cluster name: `kind-sentientd` (matches Kind cluster name)

### SVID TTLs
- CA TTL: 168h (7 days)
- Default X.509-SVID TTL: 24h (rotated automatically)

### Storage
- **Dev**: SQLite3 (single-node, stateful)
- **Production**: Use PostgreSQL plugin for HA

## Troubleshooting

**Agent can't connect to server**:
```bash
# Check server service
kubectl -n spire get svc spire-server

# Check network connectivity
kubectl -n spire exec -it <agent-pod> -- nc -zv spire-server.spire 8081
```

**Workload can't access socket**:
```bash
# Verify socket exists
kubectl exec -it <workload-pod> -- ls -la /run/spire/sockets/

# Check agent logs
kubectl -n spire logs -l app=spire-agent
```

**Registration entry not working**:
```bash
# List all entries
kubectl -n spire exec -it spire-server-0 -- \
  /opt/spire/bin/spire-server entry show

# Check agent logs for attestation errors
kubectl -n spire logs -l app=spire-agent | grep -i error
```

## Security Notes

- This configuration is **dev-only** (single-node SPIRE server, SQLite storage)
- Agent runs as `privileged: true` and `runAsUser: 0` (required for node attestation)
- `skip_kubelet_verification = true` is enabled for Kind compatibility
- Production deployments should:
  - Use PostgreSQL for SPIRE server storage
  - Deploy multiple SPIRE servers for HA
  - Enable kubelet verification
  - Use proper CA key management (not in-memory)

## References

- [SPIFFE Spec](https://github.com/spiffe/spiffe)
- [SPIRE Docs](https://spiffe.io/docs/latest/spire/)
- [SPIRE Kubernetes Quickstart](https://spiffe.io/docs/latest/try/getting-started-k8s/)
