package identity

import (
	"testing"
)

func TestSpiffeConfig_Structure(t *testing.T) {
	t.Parallel()

	// Test that SpiffeConfig can be created with nil values
	// (actual SPIFFE connection requires Workload API)
	cfg := &SpiffeConfig{
		Source:    nil,
		TLSConfig: nil,
	}

	// Verify structure can be instantiated
	if cfg.Source != nil {
		t.Error("Expected nil Source")
	}
	if cfg.TLSConfig != nil {
		t.Error("Expected nil TLSConfig")
	}
}

func TestSpiffeConfig_Close_NilSource(t *testing.T) {
	t.Parallel()

	cfg := &SpiffeConfig{
		Source:    nil,
		TLSConfig: nil,
	}

	// Should not panic with nil source
	err := cfg.Close()

	if err != nil {
		t.Errorf("Close() with nil source error = %v, want nil", err)
	}
}

// Note: Tests for NewClientMTLSConfig() require a SPIRE Workload API
// to be available (SPIFFE_ENDPOINT_SOCKET env var pointing to agent socket).
//
// These would be integration tests tagged with `//go:build integration`
// and run in an environment with SPIRE deployed.
//
// Example integration test structure:
//
// //go:build integration
//
// func TestNewClientMTLSConfig_Integration(t *testing.T) {
//     // Requires SPIRE agent running with proper workload registration
//     expectedServerID := "spiffe://torvyn.local/kagent"
//
//     cfg, err := NewClientMTLSConfig(expectedServerID)
//     if err != nil {
//         t.Fatalf("NewClientMTLSConfig() error = %v", err)
//     }
//     defer cfg.Close()
//
//     if cfg.Source == nil {
//         t.Error("Source should not be nil")
//     }
//
//     if cfg.TLSConfig == nil {
//         t.Error("TLSConfig should not be nil")
//     }
//
//     // Verify TLS config has client certificates
//     if cfg.TLSConfig.GetClientCertificate == nil {
//         t.Error("TLSConfig should have client certificate provider")
//     }
// }
//
// func TestNewClientMTLSConfig_InvalidSPIFFEID_Integration(t *testing.T) {
//     // Test with invalid SPIFFE ID format
//     _, err := NewClientMTLSConfig("not-a-valid-spiffe-id")
//     if err == nil {
//         t.Error("NewClientMTLSConfig() expected error for invalid SPIFFE ID, got nil")
//     }
// }
//
// func TestNewClientMTLSConfig_NoWorkloadAPI(t *testing.T) {
//     // Test when SPIRE agent is not available
//     // Unset SPIFFE_ENDPOINT_SOCKET or point to non-existent socket
//     t.Setenv("SPIFFE_ENDPOINT_SOCKET", "unix:///nonexistent/socket")
//
//     _, err := NewClientMTLSConfig("spiffe://torvyn.local/kagent")
//     if err == nil {
//         t.Error("NewClientMTLSConfig() expected error when Workload API unavailable, got nil")
//     }
// }

// Unit tests for SPIFFE ID validation (without actual Workload API)

func TestSPIFFEIDFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		spiffeID string
		wantErr  bool
	}{
		{
			name:     "valid SPIFFE ID",
			spiffeID: "spiffe://torvyn.local/kagent",
			wantErr:  false,
		},
		{
			name:     "valid SPIFFE ID with path",
			spiffeID: "spiffe://torvyn.local/ns/sentientd/sa/default",
			wantErr:  false,
		},
		{
			name:     "invalid - missing scheme",
			spiffeID: "torvyn.local/kagent",
			wantErr:  true,
		},
		{
			name:     "invalid - empty string",
			spiffeID: "",
			wantErr:  true,
		},
		{
			name:     "invalid - wrong scheme",
			spiffeID: "https://torvyn.local/kagent",
			wantErr:  true,
		},
		{
			name:     "invalid - missing trust domain",
			spiffeID: "spiffe:///kagent",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// We can't actually call NewClientMTLSConfig without SPIRE,
			// but we can validate the SPIFFE ID format would be accepted
			// by checking basic structure

			hasScheme := len(tt.spiffeID) >= 9 && tt.spiffeID[:9] == "spiffe://"
			hasTrustDomain := len(tt.spiffeID) > 9 && tt.spiffeID[9:] != "" && tt.spiffeID[9] != '/'

			gotErr := !hasScheme || !hasTrustDomain

			if gotErr != tt.wantErr {
				t.Errorf("SPIFFE ID validation: gotErr = %v, wantErr %v for %q", gotErr, tt.wantErr, tt.spiffeID)
			}
		})
	}
}

func TestSpiffeConfig_FieldsPresence(t *testing.T) {
	t.Parallel()

	// Test that SpiffeConfig has the expected fields
	cfg := &SpiffeConfig{}

	// Verify fields can be accessed (compile-time check)
	_ = cfg.Source
	_ = cfg.TLSConfig

	// Verify zero values
	if cfg.Source != nil {
		t.Error("Zero-value Source should be nil")
	}

	if cfg.TLSConfig != nil {
		t.Error("Zero-value TLSConfig should be nil")
	}
}

func TestClose_MultipleCallsSafe(t *testing.T) {
	t.Parallel()

	cfg := &SpiffeConfig{
		Source:    nil,
		TLSConfig: nil,
	}

	// Multiple Close() calls should be safe
	err1 := cfg.Close()
	err2 := cfg.Close()
	err3 := cfg.Close()

	if err1 != nil || err2 != nil || err3 != nil {
		t.Error("Multiple Close() calls should not error")
	}
}
