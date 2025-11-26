package database

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	url := "postgres://user:pass@localhost:5432/testdb"
	cfg := DefaultConfig(url)

	if cfg.URL != url {
		t.Errorf("URL = %v, want %v", cfg.URL, url)
	}

	if cfg.MaxConns != 25 {
		t.Errorf("MaxConns = %v, want 25", cfg.MaxConns)
	}

	if cfg.MinConns != 5 {
		t.Errorf("MinConns = %v, want 5", cfg.MinConns)
	}

	if cfg.MaxConnLifetime != 1*time.Hour {
		t.Errorf("MaxConnLifetime = %v, want 1h", cfg.MaxConnLifetime)
	}

	if cfg.MaxConnIdleTime != 30*time.Minute {
		t.Errorf("MaxConnIdleTime = %v, want 30m", cfg.MaxConnIdleTime)
	}
}

func TestConfig_Structure(t *testing.T) {
	t.Parallel()

	cfg := Config{
		URL:             "postgres://test:test@localhost:5432/test",
		MaxConns:        50,
		MinConns:        10,
		MaxConnLifetime: 2 * time.Hour,
		MaxConnIdleTime: 1 * time.Hour,
	}

	if cfg.URL == "" {
		t.Error("URL should not be empty")
	}

	if cfg.MaxConns <= 0 {
		t.Error("MaxConns should be positive")
	}

	if cfg.MinConns < 0 {
		t.Error("MinConns should not be negative")
	}

	if cfg.MaxConnLifetime <= 0 {
		t.Error("MaxConnLifetime should be positive")
	}

	if cfg.MaxConnIdleTime <= 0 {
		t.Error("MaxConnIdleTime should be positive")
	}
}

func TestDefaultConfig_DifferentURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "localhost",
			url:  "postgres://user:pass@localhost:5432/db",
		},
		{
			name: "remote host",
			url:  "postgres://user:pass@db.example.com:5432/production",
		},
		{
			name: "with SSL mode",
			url:  "postgres://user:pass@localhost:5432/db?sslmode=require",
		},
		{
			name: "empty URL",
			url:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := DefaultConfig(tt.url)

			if cfg.URL != tt.url {
				t.Errorf("URL = %v, want %v", cfg.URL, tt.url)
			}

			// Verify defaults are always applied
			if cfg.MaxConns != 25 {
				t.Errorf("MaxConns = %v, want 25 (default)", cfg.MaxConns)
			}

			if cfg.MinConns != 5 {
				t.Errorf("MinConns = %v, want 5 (default)", cfg.MinConns)
			}
		})
	}
}

func TestConfig_CustomPoolSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		maxConns        int
		minConns        int
		maxConnLifetime time.Duration
		maxConnIdleTime time.Duration
	}{
		{
			name:            "small pool",
			maxConns:        10,
			minConns:        2,
			maxConnLifetime: 30 * time.Minute,
			maxConnIdleTime: 10 * time.Minute,
		},
		{
			name:            "large pool",
			maxConns:        100,
			minConns:        20,
			maxConnLifetime: 4 * time.Hour,
			maxConnIdleTime: 2 * time.Hour,
		},
		{
			name:            "minimal pool",
			maxConns:        5,
			minConns:        1,
			maxConnLifetime: 15 * time.Minute,
			maxConnIdleTime: 5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := Config{
				URL:             "postgres://test:test@localhost:5432/test",
				MaxConns:        tt.maxConns,
				MinConns:        tt.minConns,
				MaxConnLifetime: tt.maxConnLifetime,
				MaxConnIdleTime: tt.maxConnIdleTime,
			}

			if cfg.MaxConns != tt.maxConns {
				t.Errorf("MaxConns = %v, want %v", cfg.MaxConns, tt.maxConns)
			}

			if cfg.MinConns != tt.minConns {
				t.Errorf("MinConns = %v, want %v", cfg.MinConns, tt.minConns)
			}

			if cfg.MaxConnLifetime != tt.maxConnLifetime {
				t.Errorf("MaxConnLifetime = %v, want %v", cfg.MaxConnLifetime, tt.maxConnLifetime)
			}

			if cfg.MaxConnIdleTime != tt.maxConnIdleTime {
				t.Errorf("MaxConnIdleTime = %v, want %v", cfg.MaxConnIdleTime, tt.maxConnIdleTime)
			}
		})
	}
}

func TestClose_NilPool(t *testing.T) {
	t.Parallel()

	// Should not panic with nil pool
	Close(nil)
}

// Note: Tests for Connect(), Health(), and actual database operations
// require integration tests with a real Postgres instance.
// Those would be tagged with `//go:build integration`
// and run separately with `go test -tags=integration`
//
// Example integration test structure:
//
// //go:build integration
//
// func TestConnect_Integration(t *testing.T) {
//     cfg := DefaultConfig("postgres://test:test@localhost:5432/test")
//     pool, err := Connect(context.Background(), cfg)
//     if err != nil {
//         t.Fatalf("Connect() error = %v", err)
//     }
//     defer Close(pool)
//
//     if err := Health(context.Background(), pool); err != nil {
//         t.Errorf("Health() error = %v", err)
//     }
// }
