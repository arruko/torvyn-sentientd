package config

import (
	"os"
	"time"
)

type Config struct {
	ListenAddr       string
	AlertPath        string
	InitialWindow    time.Duration
	ExpansionWindow  time.Duration
	KagentURL        string
	ArgoNamespace    string
	SlackWebhookURL  string
	ServiceName      string
	Environment      string
	DatabaseURL      string
	UseInMemoryStore bool
	NATSURL          string
	NATSEnabled      bool
	PolicyDir        string
}

func FromEnv() Config {
	return Config{
		ListenAddr:       getEnv("SENTIENTD_LISTEN_ADDR", ":8080"),
		AlertPath:        "/alertmanager/webhook",
		InitialWindow:    getDurationEnv("SENTIENTD_INITIAL_WINDOW", 30*time.Second),
		ExpansionWindow:  getDurationEnv("SENTIENTD_EXPANSION_WINDOW", 5*time.Minute),
		KagentURL:        getEnv("SENTIENTD_KAGENT_URL", "http://kagent:8080"),
		ArgoNamespace:    getEnv("SENTIENTD_ARGO_NAMESPACE", "argo"),
		SlackWebhookURL:  getEnv("SENTIENTD_SLACK_WEBHOOK_URL", ""),
		ServiceName:      getEnv("SENTIENTD_SERVICE_NAME", "sentientd"),
		Environment:      getEnv("SENTIENTD_ENVIRONMENT", "dev"),
		DatabaseURL:      getEnv("DATABASE_URL", ""),
		UseInMemoryStore: getEnv("USE_INMEMORY_STORE", "false") == "true",
		NATSURL:          getEnv("NATS_URL", ""),
		NATSEnabled:      getEnv("NATS_ENABLED", "false") == "true",
		PolicyDir:        getEnv("POLICY_DIR", "internal/policy/policies"),
	}
}

func getEnv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func getDurationEnv(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
