package config

import (
	"os"
	"testing"
	"time"
)

func TestFromEnv_Defaults(t *testing.T) {
	// Clear all relevant env vars
	envVars := []string{
		"SENTIENTD_LISTEN_ADDR",
		"SENTIENTD_INITIAL_WINDOW",
		"SENTIENTD_EXPANSION_WINDOW",
		"SENTIENTD_KAGENT_URL",
		"SENTIENTD_ARGO_NAMESPACE",
		"SENTIENTD_SLACK_WEBHOOK_URL",
		"SENTIENTD_SERVICE_NAME",
		"SENTIENTD_ENVIRONMENT",
		"DATABASE_URL",
		"USE_INMEMORY_STORE",
		"NATS_URL",
		"NATS_ENABLED",
		"POLICY_DIR",
	}

	for _, v := range envVars {
		_ = os.Unsetenv(v)
	}

	cfg := FromEnv()

	tests := []struct {
		name     string
		got      interface{}
		want     interface{}
	}{
		{"ListenAddr", cfg.ListenAddr, ":8080"},
		{"AlertPath", cfg.AlertPath, "/alertmanager/webhook"},
		{"InitialWindow", cfg.InitialWindow, 30 * time.Second},
		{"ExpansionWindow", cfg.ExpansionWindow, 5 * time.Minute},
		{"KagentURL", cfg.KagentURL, "http://kagent:8080"},
		{"ArgoNamespace", cfg.ArgoNamespace, "argo"},
		{"SlackWebhookURL", cfg.SlackWebhookURL, ""},
		{"ServiceName", cfg.ServiceName, "sentientd"},
		{"Environment", cfg.Environment, "dev"},
		{"DatabaseURL", cfg.DatabaseURL, ""},
		{"UseInMemoryStore", cfg.UseInMemoryStore, false},
		{"NATSURL", cfg.NATSURL, ""},
		{"NATSEnabled", cfg.NATSEnabled, false},
		{"PolicyDir", cfg.PolicyDir, "internal/policy/policies"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestFromEnv_CustomValues(t *testing.T) {
	// Set custom env vars
	_ = os.Setenv("SENTIENTD_LISTEN_ADDR", ":9000")
	_ = os.Setenv("SENTIENTD_INITIAL_WINDOW", "1m")
	_ = os.Setenv("SENTIENTD_EXPANSION_WINDOW", "10m")
	_ = os.Setenv("SENTIENTD_KAGENT_URL", "http://custom-kagent:9090")
	_ = os.Setenv("SENTIENTD_ARGO_NAMESPACE", "custom-argo")
	_ = os.Setenv("SENTIENTD_SLACK_WEBHOOK_URL", "https://hooks.slack.com/test")
	_ = os.Setenv("SENTIENTD_SERVICE_NAME", "custom-service")
	_ = os.Setenv("SENTIENTD_ENVIRONMENT", "production")
	_ = os.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test")
	_ = os.Setenv("USE_INMEMORY_STORE", "true")
	_ = os.Setenv("NATS_URL", "nats://localhost:4222")
	_ = os.Setenv("NATS_ENABLED", "true")
	_ = os.Setenv("POLICY_DIR", "/custom/policies")

	defer func() {
		// Cleanup
		_ = os.Unsetenv("SENTIENTD_LISTEN_ADDR")
		_ = os.Unsetenv("SENTIENTD_INITIAL_WINDOW")
		_ = os.Unsetenv("SENTIENTD_EXPANSION_WINDOW")
		_ = os.Unsetenv("SENTIENTD_KAGENT_URL")
		_ = os.Unsetenv("SENTIENTD_ARGO_NAMESPACE")
		_ = os.Unsetenv("SENTIENTD_SLACK_WEBHOOK_URL")
		_ = os.Unsetenv("SENTIENTD_SERVICE_NAME")
		_ = os.Unsetenv("SENTIENTD_ENVIRONMENT")
		_ = os.Unsetenv("DATABASE_URL")
		_ = os.Unsetenv("USE_INMEMORY_STORE")
		_ = os.Unsetenv("NATS_URL")
		_ = os.Unsetenv("NATS_ENABLED")
		_ = os.Unsetenv("POLICY_DIR")
	}()

	cfg := FromEnv()

	if cfg.ListenAddr != ":9000" {
		t.Errorf("ListenAddr = %v, want :9000", cfg.ListenAddr)
	}

	if cfg.InitialWindow != 1*time.Minute {
		t.Errorf("InitialWindow = %v, want 1m", cfg.InitialWindow)
	}

	if cfg.ExpansionWindow != 10*time.Minute {
		t.Errorf("ExpansionWindow = %v, want 10m", cfg.ExpansionWindow)
	}

	if cfg.KagentURL != "http://custom-kagent:9090" {
		t.Errorf("KagentURL = %v, want http://custom-kagent:9090", cfg.KagentURL)
	}

	if cfg.ArgoNamespace != "custom-argo" {
		t.Errorf("ArgoNamespace = %v, want custom-argo", cfg.ArgoNamespace)
	}

	if cfg.SlackWebhookURL != "https://hooks.slack.com/test" {
		t.Errorf("SlackWebhookURL = %v, want https://hooks.slack.com/test", cfg.SlackWebhookURL)
	}

	if cfg.ServiceName != "custom-service" {
		t.Errorf("ServiceName = %v, want custom-service", cfg.ServiceName)
	}

	if cfg.Environment != "production" {
		t.Errorf("Environment = %v, want production", cfg.Environment)
	}

	if cfg.DatabaseURL != "postgres://test:test@localhost:5432/test" {
		t.Errorf("DatabaseURL = %v", cfg.DatabaseURL)
	}

	if !cfg.UseInMemoryStore {
		t.Error("UseInMemoryStore = false, want true")
	}

	if cfg.NATSURL != "nats://localhost:4222" {
		t.Errorf("NATSURL = %v, want nats://localhost:4222", cfg.NATSURL)
	}

	if !cfg.NATSEnabled {
		t.Error("NATSEnabled = false, want true")
	}

	if cfg.PolicyDir != "/custom/policies" {
		t.Errorf("PolicyDir = %v, want /custom/policies", cfg.PolicyDir)
	}
}

func TestGetEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		def     string
		envVal  string
		setEnv  bool
		want    string
	}{
		{
			name:   "env var set",
			key:    "TEST_VAR_1",
			def:    "default",
			envVal: "custom",
			setEnv: true,
			want:   "custom",
		},
		{
			name:   "env var not set",
			key:    "TEST_VAR_2",
			def:    "default",
			setEnv: false,
			want:   "default",
		},
		{
			name:   "env var empty string",
			key:    "TEST_VAR_3",
			def:    "default",
			envVal: "",
			setEnv: true,
			want:   "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				_ = os.Setenv(tt.key, tt.envVal)
				defer func() { _ = os.Unsetenv(tt.key) }()
			} else {
				_ = os.Unsetenv(tt.key)
			}

			got := getEnv(tt.key, tt.def)
			if got != tt.want {
				t.Errorf("getEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetDurationEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		def     time.Duration
		envVal  string
		setEnv  bool
		want    time.Duration
	}{
		{
			name:   "valid duration",
			key:    "TEST_DURATION_1",
			def:    5 * time.Second,
			envVal: "10s",
			setEnv: true,
			want:   10 * time.Second,
		},
		{
			name:   "env var not set",
			key:    "TEST_DURATION_2",
			def:    5 * time.Second,
			setEnv: false,
			want:   5 * time.Second,
		},
		{
			name:   "invalid duration format",
			key:    "TEST_DURATION_3",
			def:    5 * time.Second,
			envVal: "invalid",
			setEnv: true,
			want:   5 * time.Second,
		},
		{
			name:   "empty string",
			key:    "TEST_DURATION_4",
			def:    5 * time.Second,
			envVal: "",
			setEnv: true,
			want:   5 * time.Second,
		},
		{
			name:   "complex duration",
			key:    "TEST_DURATION_5",
			def:    1 * time.Second,
			envVal: "1h30m45s",
			setEnv: true,
			want:   1*time.Hour + 30*time.Minute + 45*time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				_ = os.Setenv(tt.key, tt.envVal)
				defer func() { _ = os.Unsetenv(tt.key) }()
			} else {
				_ = os.Unsetenv(tt.key)
			}

			got := getDurationEnv(tt.key, tt.def)
			if got != tt.want {
				t.Errorf("getDurationEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromEnv_BooleanParsing(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		wantBool bool
	}{
		{"true value", "USE_INMEMORY_STORE", "true", true},
		{"false value", "USE_INMEMORY_STORE", "false", false},
		{"empty value", "USE_INMEMORY_STORE", "", false},
		{"random value", "USE_INMEMORY_STORE", "yes", false},
		{"TRUE uppercase", "USE_INMEMORY_STORE", "TRUE", false}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				_ = os.Setenv(tt.envKey, tt.envValue)
			} else {
				_ = os.Unsetenv(tt.envKey)
			}
			defer func() { _ = os.Unsetenv(tt.envKey) }()

			cfg := FromEnv()

			if cfg.UseInMemoryStore != tt.wantBool {
				t.Errorf("UseInMemoryStore = %v, want %v", cfg.UseInMemoryStore, tt.wantBool)
			}
		})
	}
}

func TestFromEnv_NATSEnabled(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{"enabled", "true", true},
		{"disabled", "false", false},
		{"empty", "", false},
		{"invalid", "maybe", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				_ = os.Setenv("NATS_ENABLED", tt.envValue)
			} else {
				_ = os.Unsetenv("NATS_ENABLED")
			}
			defer func() { _ = os.Unsetenv("NATS_ENABLED") }()

			cfg := FromEnv()

			if cfg.NATSEnabled != tt.want {
				t.Errorf("NATSEnabled = %v, want %v", cfg.NATSEnabled, tt.want)
			}
		})
	}
}
