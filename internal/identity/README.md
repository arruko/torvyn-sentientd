# Identity Package

This package provides SPIFFE-based workload identity and mTLS configuration for sentientd components.

## Overview

The `identity` package wraps the SPIRE Workload API to:
1. Obtain X.509-SVIDs for sentientd workloads
2. Generate mTLS client configurations for authenticated service-to-service communication
3. Validate peer identities using SPIFFE IDs

## Current Implementation

### `SpiffeConfig`

Holds the X509Source (connection to SPIRE Workload API) and a pre-configured `*tls.Config` for mTLS clients.

### `NewClientMTLSConfig(expectedServerID string)`

Creates an mTLS client configuration that:
- Connects to the local SPIRE Agent via Workload API
- Fetches the client's X.509-SVID
- Validates the server's SPIFFE ID matches `expectedServerID`
- Returns a `tls.Config` ready for use with HTTP or gRPC clients

**Example**:
```go
cfg, err := identity.NewClientMTLSConfig("spiffe://torvyn.local/kagent")
if err != nil {
    log.Fatal(err)
}
defer cfg.Source.Close()

client := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: cfg.TLSConfig,
    },
}
```

## Planned Usage (Future PRs)

This package will be integrated into the following clients:

### 1. kagent Client
**File**: `internal/kagent/client.go`

When sentientd sends RCA/DAG requests to kagent over HTTPS:
```go
spiffeCfg, err := identity.NewClientMTLSConfig("spiffe://torvyn.local/kagent")
defer spiffeCfg.Close()

httpClient := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: spiffeCfg.TLSConfig,
    },
}
client := kagent.NewClient(cfg.KagentURL, httpClient)
```

### 2. NATS Client
**File**: `internal/queue/nats.go` (to be created)

When sentientd publishes/subscribes to NATS with TLS:
```go
spiffeCfg, err := identity.NewClientMTLSConfig("spiffe://torvyn.local/nats")
defer spiffeCfg.Close()

nc, err := nats.Connect(
    natsURL,
    nats.Secure(spiffeCfg.TLSConfig),
)
```

### 3. Argo Workflows Client
**File**: `internal/argo/client.go`

If Argo Workflows API server is configured with mTLS:
```go
spiffeCfg, err := identity.NewClientMTLSConfig("spiffe://torvyn.local/argo-server")
defer spiffeCfg.Close()

transport := &http.Transport{
    TLSClientConfig: spiffeCfg.TLSConfig,
}
// Use with Kubernetes client-go rest.Config
```

### 4. DAG Runner Callback Client
**File**: `cmd/dag-runner/main.go`

When DAG Runner reports task status back to sentientd:
```go
spiffeCfg, err := identity.NewClientMTLSConfig("spiffe://torvyn.local/sentientd")
defer spiffeCfg.Close()

client := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: spiffeCfg.TLSConfig,
    },
}

// POST task results to sentientd
resp, err := client.Post(callbackURL, "application/json", bytes.NewReader(payload))
```

## Environment Variables

- **SPIFFE_ENDPOINT_SOCKET**: Path to SPIRE Agent socket
  - Default: `unix:///run/spire/sockets/agent.sock`
  - Must be mounted as a volume in Kubernetes pods

## Kubernetes Integration

To use this package in a pod, ensure the SPIRE agent socket is mounted:

```yaml
volumeMounts:
  - name: spire-agent-socket
    mountPath: /run/spire/sockets
    readOnly: true

volumes:
  - name: spire-agent-socket
    hostPath:
      path: /run/spire/sockets
      type: Directory
```

And the workload is registered with SPIRE:

```bash
kubectl -n spire exec -it spire-server-0 -- \
  /opt/spire/bin/spire-server entry create \
  -spiffeID spiffe://torvyn.local/sentientd \
  -parentID spiffe://torvyn.local/spire/agent/k8s_psat/kind-sentientd/node \
  -selector k8s:ns:sentientd \
  -selector k8s:sa:sentientd
```

## Future Enhancements

- **Server-side mTLS**: Add `NewServerMTLSConfig()` for sentientd HTTP server
- **gRPC support**: Add helpers for gRPC credentials
- **Identity introspection**: Add methods to retrieve own SPIFFE ID
- **Policy enforcement**: Integrate with OPA for authorization decisions

## References

- [SPIFFE Specification](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE.md)
- [go-spiffe Documentation](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2)
- [SPIRE Workload API](https://spiffe.io/docs/latest/spire/using/workload-api/)
