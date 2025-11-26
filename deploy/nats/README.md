# NATS JetStream Deployment

This directory contains NATS JetStream deployment manifests for sentientd message queue.

## Overview

NATS JetStream provides:
- Persistent message streaming
- At-least-once delivery guarantees
- Durable consumers for reliable message processing
- Low-latency pub/sub for alert ingestion

## Components

- **NATS Server**: Single-node JetStream with persistent storage
- **Client Port**: 4222 (NATS protocol)
- **Monitoring Port**: 8222 (HTTP metrics/health)

## Deployment

```bash
# Deploy NATS JetStream
kubectl apply -f deploy/nats/nats-jetstream.yaml

# Wait for NATS to be ready
kubectl -n nats wait --for=condition=ready pod -l app=nats --timeout=120s

# Verify NATS is running
kubectl -n nats get pods
kubectl -n nats get svc
```

## Configuration

### JetStream Settings
- **Store directory**: `/data/jetstream`
- **Max memory store**: 1Gi
- **Max file store**: 10Gi
- **Max payload**: 8MB

### Resource Limits
- **CPU**: 100m request, 500m limit
- **Memory**: 256Mi request, 1Gi limit
- **Storage**: 10Gi PVC

## Verification

Check NATS health and JetStream status:

```bash
# Check server health
kubectl -n nats exec -it nats-0 -- nats server check

# Check JetStream info
kubectl -n nats exec -it nats-0 -- nats server info

# View monitoring endpoint
kubectl -n nats port-forward svc/nats 8222:8222 &
curl http://localhost:8222/varz
```

## Creating Streams

sentientd will auto-create streams, but you can manually create them:

```bash
# Create alerts stream
kubectl -n nats exec -it nats-0 -- nats stream add \
  --subjects "alerts.*" \
  --storage file \
  --retention limits \
  --max-msgs=-1 \
  --max-age=168h \
  --max-bytes=5Gi \
  alerts

# List streams
kubectl -n nats exec -it nats-0 -- nats stream list

# View stream info
kubectl -n nats exec -it nats-0 -- nats stream info alerts
```

## Creating Consumers

View or create durable consumers:

```bash
# List consumers for alerts stream
kubectl -n nats exec -it nats-0 -- nats consumer list alerts

# Create a consumer
kubectl -n nats exec -it nats-0 -- nats consumer add alerts \
  --pull \
  --deliver all \
  --max-deliver=-1 \
  --ack explicit \
  sentientd-ingest

# View consumer info
kubectl -n nats exec -it nats-0 -- nats consumer info alerts sentientd-ingest
```

## Monitoring

### Health Checks
```bash
# Liveness probe
curl http://<nats-pod-ip>:8222/healthz

# Readiness probe (same endpoint)
curl http://<nats-pod-ip>:8222/healthz
```

### Metrics Endpoints
```bash
# General server stats
curl http://localhost:8222/varz

# Connection stats
curl http://localhost:8222/connz

# Subscription stats
curl http://localhost:8222/subsz

# JetStream stats
curl http://localhost:8222/jsz
```

## Connecting from Pods

Applications in the cluster can connect to NATS using:

**URL**: `nats://nats.nats.svc.cluster.local:4222`

Example Go client connection:
```go
nc, err := nats.Connect("nats://nats.nats.svc.cluster.local:4222")
if err != nil {
    log.Fatal(err)
}
defer nc.Close()

js, err := nc.JetStream()
if err != nil {
    log.Fatal(err)
}
```

## Subjects

sentientd uses the following subject hierarchy:

- `alerts.ingest` - Raw alert batches from Alertmanager
- `alerts.processed` - Correlated incidents (future)
- `incidents.investigating` - Incidents sent to kagent (future)
- `incidents.resolved` - Resolved incidents (future)

## Troubleshooting

**Pod won't start**:
```bash
# Check logs
kubectl -n nats logs -l app=nats

# Check PVC
kubectl -n nats get pvc
```

**Connection refused**:
```bash
# Verify service
kubectl -n nats get svc nats

# Test from another pod
kubectl run -it --rm debug --image=alpine --restart=Never -- sh
apk add curl
curl http://nats.nats.svc.cluster.local:8222/varz
```

**JetStream not enabled**:
```bash
# Check JetStream status
kubectl -n nats exec -it nats-0 -- nats account info

# Verify config
kubectl -n nats get configmap nats-config -o yaml
```

**Storage full**:
```bash
# Check disk usage
kubectl -n nats exec -it nats-0 -- df -h /data

# Resize PVC (if supported by storage class)
kubectl -n nats patch pvc data-nats-0 -p '{"spec":{"resources":{"requests":{"storage":"20Gi"}}}}'
```

## Production Considerations

This is a **dev configuration**. For production:

1. **High Availability**: Use NATS clustering (3+ nodes)
2. **Storage**: Use production-grade storage class with snapshots
3. **TLS**: Enable TLS for client connections (integrate with SPIFFE)
4. **Authentication**: Enable NATS account-based auth
5. **Monitoring**: Export metrics to Prometheus
6. **Resource Limits**: Tune based on message volume
7. **Backup**: Regular JetStream state backups

Example production setup:
```yaml
# 3-node cluster with TLS
replicas: 3
jetstream:
  max_file_store: 100Gi
tls:
  enabled: true
  cert: /etc/nats-tls/tls.crt
  key: /etc/nats-tls/tls.key
  ca: /etc/nats-tls/ca.crt
```

## Cleanup

```bash
# Delete NATS deployment
kubectl delete -f deploy/nats/nats-jetstream.yaml

# Verify PVC is deleted
kubectl -n nats get pvc
```

## References

- [NATS JetStream Documentation](https://docs.nats.io/nats-concepts/jetstream)
- [NATS Configuration](https://docs.nats.io/running-a-nats-service/configuration)
- [JetStream Go Client](https://github.com/nats-io/nats.go)
