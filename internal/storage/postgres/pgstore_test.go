package pgstore

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// TestAlert_Structure tests Alert model structure
func TestAlert_Structure(t *testing.T) {
	t.Parallel()

	labels := json.RawMessage(`{"team":"platform"}`)
	annotations := json.RawMessage(`{"runbook":"http://runbook.example.com"}`)
	now := time.Now()

	alert := Alert{
		ID:          12345,
		IncidentID:  "inc-123",
		Fingerprint: "abc123",
		Labels:      labels,
		Annotations: annotations,
		StartsAt:    now,
		CreatedAt:   now,
	}

	if alert.ID == 0 {
		t.Error("Alert ID should not be 0")
	}

	if alert.IncidentID == "" {
		t.Error("Alert IncidentID should not be empty")
	}

	if alert.Fingerprint == "" {
		t.Error("Alert Fingerprint should not be empty")
	}
}

// TestAlert_Fields verifies Alert has expected fields
func TestAlert_Fields(t *testing.T) {
	t.Parallel()

	alert := Alert{}

	// Verify zero values
	if alert.ID != 0 {
		t.Error("Default Alert ID should be 0")
	}

	if alert.IncidentID != "" {
		t.Error("Default Alert IncidentID should be empty")
	}

	if alert.Labels != nil {
		t.Error("Default Alert Labels should be nil")
	}

	if alert.Annotations != nil {
		t.Error("Default Alert Annotations should be nil")
	}
}

// TestIncident_Structure tests Incident model structure
func TestIncident_Structure(t *testing.T) {
	t.Parallel()

	now := time.Now()

	incident := Incident{
		ID:                  "inc-456",
		Service:             "auth-service",
		Cluster:             "staging-cluster",
		Environment:         "staging",
		Severity:            "warning",
		CorrelationKey:      "staging:default:auth-service",
		CorrelationVersion:  1,
		State:               "open",
		CreatedAt:           now,
		UpdatedAt:           now,
		FirstAlertAt:        now.Add(-10 * time.Minute),
		LastAlertAt:         now.Add(-5 * time.Minute),
		TopologySnapshot:    []byte(`{}`),
		RemediationAttempts: 0,
	}

	if incident.ID == "" {
		t.Error("Incident ID should not be empty")
	}

	if incident.Cluster == "" {
		t.Error("Incident Cluster should not be empty")
	}

	if incident.State == "" {
		t.Error("Incident State should not be empty")
	}

	if incident.Service == "" {
		t.Error("Incident Service should not be empty")
	}
}

// TestIncident_Fields verifies Incident has expected fields
func TestIncident_Fields(t *testing.T) {
	t.Parallel()

	incident := Incident{}

	// Verify zero values
	if incident.ID != "" {
		t.Error("Default Incident ID should be empty")
	}

	if incident.Cluster != "" {
		t.Error("Default Incident Cluster should be empty")
	}

	if incident.RcaSummary != nil {
		t.Error("Default Incident RcaSummary should be nil")
	}

	if incident.GithubPrUrl != nil {
		t.Error("Default Incident GithubPrUrl should be nil")
	}

	if incident.RemediationAttempts != 0 {
		t.Error("Default Incident RemediationAttempts should be 0")
	}
}

// TestIncident_PointerFields tests pointer fields can be set
func TestIncident_PointerFields(t *testing.T) {
	t.Parallel()

	rcaSummary := "High CPU usage"
	prUrl := "https://github.com/org/repo/pull/123"

	incident := Incident{
		ID:          "inc-789",
		Service:     "api",
		Cluster:     "prod",
		Environment: "production",
		Severity:    "critical",
		State:       "investigating",
		RcaSummary:  &rcaSummary,
		GithubPrUrl: &prUrl,
	}

	if incident.RcaSummary == nil {
		t.Error("RcaSummary should not be nil")
	}

	if *incident.RcaSummary != rcaSummary {
		t.Errorf("RcaSummary = %v, want %v", *incident.RcaSummary, rcaSummary)
	}

	if incident.GithubPrUrl == nil {
		t.Error("GithubPrUrl should not be nil")
	}

	if *incident.GithubPrUrl != prUrl {
		t.Errorf("GithubPrUrl = %v, want %v", *incident.GithubPrUrl, prUrl)
	}
}

// TestQuerier_Interface verifies the Querier interface exists
func TestQuerier_Interface(t *testing.T) {
	t.Parallel()

	// This test verifies that the Querier interface is defined
	// We can't test methods without a database, but we can verify the type exists
	var querier Querier

	// Verify nil interface value
	_ = querier
}

// TestQueries_Structure verifies the Queries struct exists
func TestQueries_Structure(t *testing.T) {
	t.Parallel()

	// Test that we can create a Queries instance (will have nil db)
	queries := &Queries{}

	// Verify struct can be instantiated (db will be nil)
	if queries.db != nil {
		t.Error("Expected nil db")
	}
}

// TestNew_NilDB tests New with nil database
func TestNew_NilDB(t *testing.T) {
	t.Parallel()

	queries := New(nil)

	if queries == nil {
		t.Error("New() should not return nil even with nil db")
	}
}

// TestWithTx_NilDB tests WithTx with nil database
func TestWithTx_NilDB(t *testing.T) {
	t.Parallel()

	queries := &Queries{}
	newQueries := queries.WithTx(nil)

	if newQueries == nil {
		t.Error("WithTx() should not return nil")
	}
}

// TestIncidentSeverity_Values tests valid severity values
func TestIncidentSeverity_Values(t *testing.T) {
	t.Parallel()

	validSeverities := []string{"critical", "warning", "info"}

	for _, severity := range validSeverities {
		incident := Incident{
			ID:       "inc-test",
			Service:  "test",
			Cluster:  "test",
			Severity: severity,
		}

		if incident.Severity != severity {
			t.Errorf("Incident Severity = %v, want %v", incident.Severity, severity)
		}
	}
}

// TestIncidentState_Values tests valid state values
func TestIncidentState_Values(t *testing.T) {
	t.Parallel()

	validStates := []string{"open", "investigating", "mitigating", "monitoring", "resolved"}

	for _, state := range validStates {
		incident := Incident{
			ID:      "inc-test",
			Service: "test",
			Cluster: "test",
			State:   state,
		}

		if incident.State != state {
			t.Errorf("Incident State = %v, want %v", incident.State, state)
		}
	}
}

// TestTimestamps_Ordering tests that timestamps maintain proper ordering
func TestTimestamps_Ordering(t *testing.T) {
	t.Parallel()

	now := time.Now()
	firstAlert := now.Add(-10 * time.Minute)
	lastAlert := now.Add(-5 * time.Minute)
	created := now.Add(-15 * time.Minute)
	updated := now

	incident := Incident{
		ID:           "inc-test",
		Service:      "test",
		Cluster:      "test",
		FirstAlertAt: firstAlert,
		LastAlertAt:  lastAlert,
		CreatedAt:    created,
		UpdatedAt:    updated,
	}

	if incident.FirstAlertAt.After(incident.LastAlertAt) {
		t.Error("FirstAlertAt should not be after LastAlertAt")
	}

	if incident.CreatedAt.After(incident.UpdatedAt) {
		t.Error("CreatedAt should not be after UpdatedAt")
	}
}

// TestJSONFields_ValidJSON tests that JSON fields accept valid JSON
func TestJSONFields_ValidJSON(t *testing.T) {
	t.Parallel()

	validLabels := json.RawMessage(`{"key": "value", "number": 123}`)
	validAnnotations := json.RawMessage(`{"runbook": "http://example.com"}`)

	alert := Alert{
		ID:          1,
		IncidentID:  "inc-test",
		Fingerprint: "test",
		Labels:      validLabels,
		Annotations: validAnnotations,
		StartsAt:    time.Now(),
		CreatedAt:   time.Now(),
	}

	if len(alert.Labels) == 0 {
		t.Error("Alert Labels should not be empty")
	}

	if len(alert.Annotations) == 0 {
		t.Error("Alert Annotations should not be empty")
	}

	// Test JSON unmarshaling
	var labelsMap map[string]interface{}
	if err := json.Unmarshal(alert.Labels, &labelsMap); err != nil {
		t.Errorf("Failed to unmarshal Labels: %v", err)
	}

	if labelsMap["key"] != "value" {
		t.Errorf("Labels[key] = %v, want 'value'", labelsMap["key"])
	}
}

// TestAlert_EndsAtTimestamp tests the pgtype.Timestamptz field
func TestAlert_EndsAtTimestamp(t *testing.T) {
	t.Parallel()

	now := time.Now()
	endsAt := pgtype.Timestamptz{
		Time:  now.Add(5 * time.Minute),
		Valid: true,
	}

	alert := Alert{
		ID:          1,
		IncidentID:  "inc-test",
		Fingerprint: "test",
		StartsAt:    now,
		EndsAt:      endsAt,
		CreatedAt:   now,
	}

	if !alert.EndsAt.Valid {
		t.Error("EndsAt should be valid")
	}

	if alert.EndsAt.Time.Before(alert.StartsAt) {
		t.Error("EndsAt should not be before StartsAt")
	}
}

// TestIncident_TopologySnapshot tests topology snapshot field
func TestIncident_TopologySnapshot(t *testing.T) {
	t.Parallel()

	topology := []byte(`{"nodes":[{"id":"node1","type":"service"}]}`)

	incident := Incident{
		ID:               "inc-test",
		Service:          "test",
		Cluster:          "test",
		TopologySnapshot: topology,
	}

	if len(incident.TopologySnapshot) == 0 {
		t.Error("TopologySnapshot should not be empty")
	}

	// Test JSON unmarshaling
	var topologyMap map[string]interface{}
	if err := json.Unmarshal(incident.TopologySnapshot, &topologyMap); err != nil {
		t.Errorf("Failed to unmarshal TopologySnapshot: %v", err)
	}

	if topologyMap["nodes"] == nil {
		t.Error("TopologySnapshot should contain 'nodes'")
	}
}

// TestIncident_CorrelationKey tests correlation key format
func TestIncident_CorrelationKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		cluster        string
		environment    string
		service        string
		wantPrefix     string
	}{
		{
			name:        "production key",
			cluster:     "prod",
			environment: "production",
			service:     "api",
			wantPrefix:  "prod",
		},
		{
			name:        "staging key",
			cluster:     "staging",
			environment: "staging",
			service:     "auth",
			wantPrefix:  "staging",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			correlationKey := tt.cluster + ":" + tt.environment + ":" + tt.service

			incident := Incident{
				ID:             "inc-test",
				Service:        tt.service,
				Cluster:        tt.cluster,
				Environment:    tt.environment,
				CorrelationKey: correlationKey,
			}

			if incident.CorrelationKey == "" {
				t.Error("CorrelationKey should not be empty")
			}

			// Check if correlation key starts with cluster name
			if len(incident.CorrelationKey) < len(tt.wantPrefix) ||
				incident.CorrelationKey[:len(tt.wantPrefix)] != tt.wantPrefix {
				t.Errorf("CorrelationKey should start with %q, got %q", tt.wantPrefix, incident.CorrelationKey)
			}
		})
	}
}

// TestIncident_RemediationAttempts tests remediation attempts counter
func TestIncident_RemediationAttempts(t *testing.T) {
	t.Parallel()

	incident := Incident{
		ID:                  "inc-test",
		Service:             "test",
		Cluster:             "test",
		RemediationAttempts: 3,
	}

	if incident.RemediationAttempts < 0 {
		t.Error("RemediationAttempts should not be negative")
	}

	if incident.RemediationAttempts != 3 {
		t.Errorf("RemediationAttempts = %d, want 3", incident.RemediationAttempts)
	}
}

// Note: Integration tests for actual database operations would require a test database
// and should be tagged with `//go:build integration` and run separately.
//
// Example integration test structure:
//
// //go:build integration
//
// func TestCreateIncident_Integration(t *testing.T) {
//     ctx := context.Background()
//     db, err := pgxpool.New(ctx, "postgres://test:test@localhost:5432/test")
//     if err != nil {
//         t.Fatalf("Failed to connect: %v", err)
//     }
//     defer db.Close()
//
//     queries := New(db)
//
//     params := CreateIncidentParams{
//         ID:          uuid.NewString(),
//         Service:     "test-service",
//         Cluster:     "test-cluster",
//         Environment: "test",
//         Severity:    "critical",
//         State:       "open",
//     }
//
//     incident, err := queries.CreateIncident(ctx, params)
//     if err != nil {
//         t.Fatalf("CreateIncident() error = %v", err)
//     }
//
//     if incident.ID != params.ID {
//         t.Errorf("Created incident ID = %v, want %v", incident.ID, params.ID)
//     }
// }
