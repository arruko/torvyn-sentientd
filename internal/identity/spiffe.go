package identity

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

// SpiffeConfig holds the SPIFFE X509Source and mTLS configuration
// for authenticated communication between sentientd components.
type SpiffeConfig struct {
	Source    *workloadapi.X509Source
	TLSConfig *tls.Config
}

// NewClientMTLSConfig creates a SPIFFE-based mTLS client configuration.
// It connects to the SPIRE Workload API (via Unix socket) to obtain X.509-SVIDs
// and build a TLS config that authenticates the specified server SPIFFE ID.
//
// Parameters:
//   - expectedServerID: The SPIFFE ID of the server to authenticate (e.g., "spiffe://torvyn.local/kagent")
//
// Returns:
//   - SpiffeConfig with X509Source and TLS config, or error if initialization fails
//
// Environment:
//   - SPIFFE_ENDPOINT_SOCKET: Path to SPIRE agent socket (default: unix:///run/spire/sockets/agent.sock)
//
// Example:
//
//	cfg, err := identity.NewClientMTLSConfig("spiffe://torvyn.local/kagent")
//	if err != nil {
//	    return err
//	}
//	defer cfg.Source.Close()
//
//	client := &http.Client{
//	    Transport: &http.Transport{TLSClientConfig: cfg.TLSConfig},
//	}
func NewClientMTLSConfig(expectedServerID string) (*SpiffeConfig, error) {
	// Parse the expected server SPIFFE ID
	serverID, err := spiffeid.FromString(expectedServerID)
	if err != nil {
		return nil, fmt.Errorf("invalid server SPIFFE ID %q: %w", expectedServerID, err)
	}

	// Create context with timeout for Workload API connection
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect to SPIRE Workload API to fetch X.509-SVIDs
	// This reads SPIFFE_ENDPOINT_SOCKET env var (defaults to unix:///run/spire/sockets/agent.sock)
	source, err := workloadapi.NewX509Source(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create X509Source from Workload API: %w", err)
	}

	// Build mTLS client config that:
	// 1. Presents our SVID as client certificate
	// 2. Validates server SVID matches expectedServerID
	// 3. Uses SPIFFE trust bundle for verification
	tlsConfig := tlsconfig.MTLSClientConfig(source, source, tlsconfig.AuthorizeID(serverID))

	return &SpiffeConfig{
		Source:    source,
		TLSConfig: tlsConfig,
	}, nil
}

// Close releases resources held by the SpiffeConfig.
// Must be called when the config is no longer needed.
func (c *SpiffeConfig) Close() error {
	if c.Source != nil {
		return c.Source.Close()
	}
	return nil
}
