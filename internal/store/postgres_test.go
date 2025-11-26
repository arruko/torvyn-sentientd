package store

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/arruko/torvyn-sentientd/internal/domain"
	pgstore "github.com/arruko/torvyn-sentientd/internal/storage/postgres"
	"github.com/jackc/pgx/v5/pgtype"
)

// TestRowToIncident tests the rowToIncident conversion function
func TestRowToIncident(t *testing.T) {
	t.Parallel()

	now := time.Now()
	topology := &domain.TopologySlice{
		Nodes: []domain.TopologyNode{
			{ID: "node1", Type: "service"},
		},
	}
	topologyJSON, _ := json.Marshal(topology)

	tests := []struct {
		name      string
		row       pgstore.Incident
		alerts    []pgstore.Alert
		wantErr   bool
		validate  func(*testing.T, *domain.Incident)
	}{
		{
			name: "complete incident with alerts",
			row: pgstore.Incident{
				ID:                  "inc-123",
				Service:             "payment-api",
				Cluster:             "prod-us",
				Environment:         "production",
				Severity:            "critical",
				CorrelationKey:      "prod-us:production:payment-api",
				CorrelationVersion:  2,
				State:               "open",
				CreatedAt:           now.Add(-1 * time.Hour),
				UpdatedAt:           now,
				FirstAlertAt:        now.Add(-1 * time.Hour),
				LastAlertAt:         now,
				TopologySnapshot:    topologyJSON,
				RemediationAttempts: 1,
			},
			alerts: []pgstore.Alert{
				{
					ID:          1,
					IncidentID:  "inc-123",
					Fingerprint: "fp-1",
					Labels:      json.RawMessage(`{"alertname":"HighCPU","severity":"critical"}`),
					Annotations: json.RawMessage(`{"summary":"CPU high","description":"CPU usage above 80%"}`),
					StartsAt:    now.Add(-30 * time.Minute),
					EndsAt: pgtype.Timestamptz{
						Time:  now,
						Valid: true,
					},
					CreatedAt: now.Add(-30 * time.Minute),
				},
			},
			wantErr: false,
			validate: func(t *testing.T, inc *domain.Incident) {
				if inc.ID != "inc-123" {
					t.Errorf("ID = %v, want inc-123", inc.ID)
				}
				if inc.Service != "payment-api" {
					t.Errorf("Service = %v, want payment-api", inc.Service)
				}
				if inc.CorrelationVersion != 2 {
					t.Errorf("CorrelationVersion = %v, want 2", inc.CorrelationVersion)
				}
				if len(inc.Alerts) != 1 {
					t.Fatalf("Alerts count = %d, want 1", len(inc.Alerts))
				}
				if inc.Alerts[0].Fingerprint != "fp-1" {
					t.Errorf("Alert fingerprint = %v, want fp-1", inc.Alerts[0].Fingerprint)
				}
				if inc.Alerts[0].Labels["alertname"] != "HighCPU" {
					t.Errorf("Alert alertname = %v, want HighCPU", inc.Alerts[0].Labels["alertname"])
				}
				if inc.Alerts[0].EndsAt == nil {
					t.Error("Alert EndsAt should not be nil")
				}
				if inc.TopologyContext == nil {
					t.Error("TopologyContext should not be nil")
				}
			},
		},
		{
			name: "incident without topology",
			row: pgstore.Incident{
				ID:                  "inc-456",
				Service:             "auth-service",
				Cluster:             "staging",
				Environment:         "staging",
				Severity:            "warning",
				CorrelationKey:      "staging:staging:auth-service",
				CorrelationVersion:  1,
				State:               "investigating",
				CreatedAt:           now,
				UpdatedAt:           now,
				FirstAlertAt:        now,
				LastAlertAt:         now,
				TopologySnapshot:    nil,
				RemediationAttempts: 0,
			},
			alerts:  []pgstore.Alert{},
			wantErr: false,
			validate: func(t *testing.T, inc *domain.Incident) {
				if inc.TopologyContext != nil {
					t.Error("TopologyContext should be nil")
				}
				if len(inc.Alerts) != 0 {
					t.Errorf("Alerts count = %d, want 0", len(inc.Alerts))
				}
			},
		},
		{
			name: "alert with nil EndsAt",
			row: pgstore.Incident{
				ID:                  "inc-789",
				Service:             "test",
				Cluster:             "test",
				Environment:         "test",
				Severity:            "info",
				CorrelationKey:      "test",
				CorrelationVersion:  1,
				State:               "open",
				CreatedAt:           now,
				UpdatedAt:           now,
				FirstAlertAt:        now,
				LastAlertAt:         now,
				TopologySnapshot:    nil,
				RemediationAttempts: 0,
			},
			alerts: []pgstore.Alert{
				{
					ID:          1,
					IncidentID:  "inc-789",
					Fingerprint: "fp-2",
					Labels:      json.RawMessage(`{"alertname":"Test"}`),
					Annotations: json.RawMessage(`{}`),
					StartsAt:    now,
					EndsAt: pgtype.Timestamptz{
						Valid: false,
					},
					CreatedAt: now,
				},
			},
			wantErr: false,
			validate: func(t *testing.T, inc *domain.Incident) {
				if len(inc.Alerts) != 1 {
					t.Fatalf("Alerts count = %d, want 1", len(inc.Alerts))
				}
				if inc.Alerts[0].EndsAt != nil {
					t.Error("Alert EndsAt should be nil when pgtype.Timestamptz.Valid is false")
				}
			},
		},
		{
			name: "multiple alerts",
			row: pgstore.Incident{
				ID:                  "inc-multi",
				Service:             "multi-service",
				Cluster:             "test",
				Environment:         "test",
				Severity:            "critical",
				CorrelationKey:      "test",
				CorrelationVersion:  1,
				State:               "open",
				CreatedAt:           now,
				UpdatedAt:           now,
				FirstAlertAt:        now,
				LastAlertAt:         now,
				TopologySnapshot:    nil,
				RemediationAttempts: 0,
			},
			alerts: []pgstore.Alert{
				{
					ID:          1,
					IncidentID:  "inc-multi",
					Fingerprint: "fp-a",
					Labels:      json.RawMessage(`{"alertname":"Alert1"}`),
					Annotations: json.RawMessage(`{}`),
					StartsAt:    now.Add(-10 * time.Minute),
					EndsAt:      pgtype.Timestamptz{Valid: false},
					CreatedAt:   now.Add(-10 * time.Minute),
				},
				{
					ID:          2,
					IncidentID:  "inc-multi",
					Fingerprint: "fp-b",
					Labels:      json.RawMessage(`{"alertname":"Alert2"}`),
					Annotations: json.RawMessage(`{}`),
					StartsAt:    now.Add(-5 * time.Minute),
					EndsAt:      pgtype.Timestamptz{Valid: false},
					CreatedAt:   now.Add(-5 * time.Minute),
				},
				{
					ID:          3,
					IncidentID:  "inc-multi",
					Fingerprint: "fp-c",
					Labels:      json.RawMessage(`{"alertname":"Alert3"}`),
					Annotations: json.RawMessage(`{}`),
					StartsAt:    now,
					EndsAt:      pgtype.Timestamptz{Valid: false},
					CreatedAt:   now,
				},
			},
			wantErr: false,
			validate: func(t *testing.T, inc *domain.Incident) {
				if len(inc.Alerts) != 3 {
					t.Errorf("Alerts count = %d, want 3", len(inc.Alerts))
				}
				fingerprints := make(map[string]bool)
				for _, alert := range inc.Alerts {
					fingerprints[alert.Fingerprint] = true
				}
				if !fingerprints["fp-a"] || !fingerprints["fp-b"] || !fingerprints["fp-c"] {
					t.Error("Missing expected fingerprints")
				}
			},
		},
		{
			name: "invalid topology JSON",
			row: pgstore.Incident{
				ID:                  "inc-bad-topology",
				Service:             "test",
				Cluster:             "test",
				Environment:         "test",
				Severity:            "info",
				CorrelationKey:      "test",
				CorrelationVersion:  1,
				State:               "open",
				CreatedAt:           now,
				UpdatedAt:           now,
				FirstAlertAt:        now,
				LastAlertAt:         now,
				TopologySnapshot:    []byte(`{invalid json`),
				RemediationAttempts: 0,
			},
			alerts:  []pgstore.Alert{},
			wantErr: true,
		},
		{
			name: "invalid labels JSON",
			row: pgstore.Incident{
				ID:                  "inc-test",
				Service:             "test",
				Cluster:             "test",
				Environment:         "test",
				Severity:            "info",
				CorrelationKey:      "test",
				CorrelationVersion:  1,
				State:               "open",
				CreatedAt:           now,
				UpdatedAt:           now,
				FirstAlertAt:        now,
				LastAlertAt:         now,
				TopologySnapshot:    nil,
				RemediationAttempts: 0,
			},
			alerts: []pgstore.Alert{
				{
					ID:          1,
					IncidentID:  "inc-test",
					Fingerprint: "fp-bad",
					Labels:      json.RawMessage(`{invalid json`),
					Annotations: json.RawMessage(`{}`),
					StartsAt:    now,
					EndsAt:      pgtype.Timestamptz{Valid: false},
					CreatedAt:   now,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid annotations JSON",
			row: pgstore.Incident{
				ID:                  "inc-test",
				Service:             "test",
				Cluster:             "test",
				Environment:         "test",
				Severity:            "info",
				CorrelationKey:      "test",
				CorrelationVersion:  1,
				State:               "open",
				CreatedAt:           now,
				UpdatedAt:           now,
				FirstAlertAt:        now,
				LastAlertAt:         now,
				TopologySnapshot:    nil,
				RemediationAttempts: 0,
			},
			alerts: []pgstore.Alert{
				{
					ID:          1,
					IncidentID:  "inc-test",
					Fingerprint: "fp-bad",
					Labels:      json.RawMessage(`{}`),
					Annotations: json.RawMessage(`{invalid json`),
					StartsAt:    now,
					EndsAt:      pgtype.Timestamptz{Valid: false},
					CreatedAt:   now,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inc, err := rowToIncident(tt.row, tt.alerts)

			if tt.wantErr {
				if err == nil {
					t.Error("rowToIncident() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("rowToIncident() unexpected error: %v", err)
			}

			if inc == nil {
				t.Fatal("rowToIncident() returned nil")
			}

			if tt.validate != nil {
				tt.validate(t, inc)
			}
		})
	}
}

// TestRowToIncident_StateConversion tests incident state string conversion
func TestRowToIncident_StateConversion(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name      string
		stateStr  string
		wantState domain.IncidentState
	}{
		{"open state", "open", domain.IncidentStateOpen},
		{"investigating state", "investigating", domain.IncidentStateInvestigating},
		{"mitigating state", "mitigating", domain.IncidentStateMitigating},
		{"monitoring state", "monitoring", domain.IncidentStateMonitoring},
		{"resolved state", "resolved", domain.IncidentStateResolved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			row := pgstore.Incident{
				ID:                  "inc-state-test",
				Service:             "test",
				Cluster:             "test",
				Environment:         "test",
				Severity:            "info",
				CorrelationKey:      "test",
				CorrelationVersion:  1,
				State:               tt.stateStr,
				CreatedAt:           now,
				UpdatedAt:           now,
				FirstAlertAt:        now,
				LastAlertAt:         now,
				TopologySnapshot:    nil,
				RemediationAttempts: 0,
			}

			inc, err := rowToIncident(row, []pgstore.Alert{})

			if err != nil {
				t.Fatalf("rowToIncident() unexpected error: %v", err)
			}

			if inc.State != tt.wantState {
				t.Errorf("State = %v, want %v", inc.State, tt.wantState)
			}
		})
	}
}

// TestRowToIncident_AlertFieldMapping tests all alert fields are properly mapped
func TestRowToIncident_AlertFieldMapping(t *testing.T) {
	t.Parallel()

	now := time.Now()
	endsAt := now.Add(1 * time.Hour)

	row := pgstore.Incident{
		ID:                  "inc-alert-mapping",
		Service:             "test",
		Cluster:             "test",
		Environment:         "test",
		Severity:            "info",
		CorrelationKey:      "test",
		CorrelationVersion:  1,
		State:               "open",
		CreatedAt:           now,
		UpdatedAt:           now,
		FirstAlertAt:        now,
		LastAlertAt:         now,
		TopologySnapshot:    nil,
		RemediationAttempts: 0,
	}

	alerts := []pgstore.Alert{
		{
			ID:          1,
			IncidentID:  "inc-alert-mapping",
			Fingerprint: "test-fingerprint-123",
			Labels: json.RawMessage(`{
				"alertname": "TestAlert",
				"severity": "critical",
				"cluster": "prod",
				"namespace": "default",
				"service": "api"
			}`),
			Annotations: json.RawMessage(`{
				"summary": "Test alert summary",
				"description": "Detailed description",
				"runbook": "http://runbook.example.com"
			}`),
			StartsAt: now,
			EndsAt: pgtype.Timestamptz{
				Time:  endsAt,
				Valid: true,
			},
			CreatedAt: now,
		},
	}

	inc, err := rowToIncident(row, alerts)

	if err != nil {
		t.Fatalf("rowToIncident() unexpected error: %v", err)
	}

	if len(inc.Alerts) != 1 {
		t.Fatalf("Alerts count = %d, want 1", len(inc.Alerts))
	}

	alert := inc.Alerts[0]

	// Verify fingerprint
	if alert.Fingerprint != "test-fingerprint-123" {
		t.Errorf("Fingerprint = %v, want test-fingerprint-123", alert.Fingerprint)
	}

	// Verify all labels
	expectedLabels := map[string]string{
		"alertname": "TestAlert",
		"severity":  "critical",
		"cluster":   "prod",
		"namespace": "default",
		"service":   "api",
	}

	for key, expectedValue := range expectedLabels {
		if alert.Labels[key] != expectedValue {
			t.Errorf("Label[%s] = %v, want %v", key, alert.Labels[key], expectedValue)
		}
	}

	// Verify all annotations
	expectedAnnotations := map[string]string{
		"summary":     "Test alert summary",
		"description": "Detailed description",
		"runbook":     "http://runbook.example.com",
	}

	for key, expectedValue := range expectedAnnotations {
		if alert.Annotations[key] != expectedValue {
			t.Errorf("Annotation[%s] = %v, want %v", key, alert.Annotations[key], expectedValue)
		}
	}

	// Verify timestamps
	if !alert.StartsAt.Equal(now) {
		t.Errorf("StartsAt = %v, want %v", alert.StartsAt, now)
	}

	if alert.EndsAt == nil {
		t.Fatal("EndsAt should not be nil")
	}

	if !alert.EndsAt.Equal(endsAt) {
		t.Errorf("EndsAt = %v, want %v", *alert.EndsAt, endsAt)
	}
}

// TestRowToIncident_EmptyMaps tests handling of empty labels and annotations
func TestRowToIncident_EmptyMaps(t *testing.T) {
	t.Parallel()

	now := time.Now()

	row := pgstore.Incident{
		ID:                  "inc-empty-maps",
		Service:             "test",
		Cluster:             "test",
		Environment:         "test",
		Severity:            "info",
		CorrelationKey:      "test",
		CorrelationVersion:  1,
		State:               "open",
		CreatedAt:           now,
		UpdatedAt:           now,
		FirstAlertAt:        now,
		LastAlertAt:         now,
		TopologySnapshot:    nil,
		RemediationAttempts: 0,
	}

	alerts := []pgstore.Alert{
		{
			ID:          1,
			IncidentID:  "inc-empty-maps",
			Fingerprint: "fp-empty",
			Labels:      json.RawMessage(`{}`),
			Annotations: json.RawMessage(`{}`),
			StartsAt:    now,
			EndsAt:      pgtype.Timestamptz{Valid: false},
			CreatedAt:   now,
		},
	}

	inc, err := rowToIncident(row, alerts)

	if err != nil {
		t.Fatalf("rowToIncident() unexpected error: %v", err)
	}

	if len(inc.Alerts) != 1 {
		t.Fatalf("Alerts count = %d, want 1", len(inc.Alerts))
	}

	alert := inc.Alerts[0]

	if alert.Labels == nil {
		t.Error("Labels should not be nil, should be empty map")
	}

	if len(alert.Labels) != 0 {
		t.Errorf("Labels length = %d, want 0", len(alert.Labels))
	}

	if alert.Annotations == nil {
		t.Error("Annotations should not be nil, should be empty map")
	}

	if len(alert.Annotations) != 0 {
		t.Errorf("Annotations length = %d, want 0", len(alert.Annotations))
	}
}

// TestNewPostgresIncidentStore tests constructor
func TestNewPostgresIncidentStore(t *testing.T) {
	t.Parallel()

	// Test with nil pool (won't work in practice but tests constructor)
	store := NewPostgresIncidentStore(nil)

	if store == nil {
		t.Fatal("NewPostgresIncidentStore() returned nil")
	}

	// Store is guaranteed non-nil after the check above
	if store.pool != nil {
		t.Error("Expected nil pool")
	}

	if store.q == nil {
		t.Error("Queries should be initialized even with nil pool")
	}
}
