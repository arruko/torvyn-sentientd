package investigation

import (
	"context"
	"testing"
	"time"

	"github.com/arruko/torvyn-sentientd/internal/argo"
	"github.com/arruko/torvyn-sentientd/internal/domain"
	"github.com/arruko/torvyn-sentientd/internal/kagent"
	"github.com/arruko/torvyn-sentientd/internal/policy"
	"github.com/arruko/torvyn-sentientd/internal/slack"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// TestNewEngine_AllNilClients tests engine creation with all nil clients
func TestNewEngine_AllNilClients(t *testing.T) {
	t.Parallel()

	mockStore := &mockIncidentStore{incidents: make(map[string]*domain.Incident)}

	// All clients nil
	engine := NewEngine(mockStore, nil, nil, nil, nil)

	if engine == nil {
		t.Fatal("NewEngine() returned nil")
	}

	if engine.store == nil {
		t.Error("store should not be nil")
	}

	// All optional clients can be nil
	if engine.kagentClient != nil {
		t.Error("kagentClient should be nil")
	}

	if engine.argoClient != nil {
		t.Error("argoClient should be nil")
	}

	if engine.slackClient != nil {
		t.Error("slackClient should be nil")
	}

	if engine.policyEngine != nil {
		t.Error("policyEngine should be nil")
	}
}

// TestNewEngine_SelectiveNilClients tests engine with some nil clients
func TestNewEngine_SelectiveNilClients(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		kagent        *kagent.Client
		argo          *argo.Client
		slack         *slack.Client
		policy        *policy.Engine
	}{
		{
			name:          "only kagent",
			kagent:        &kagent.Client{},
			argo:          nil,
			slack:         nil,
			policy:        nil,
		},
		{
			name:          "kagent and argo",
			kagent:        &kagent.Client{},
			argo:          &argo.Client{},
			slack:         nil,
			policy:        nil,
		},
		{
			name:          "kagent and slack",
			kagent:        &kagent.Client{},
			argo:          nil,
			slack:         &slack.Client{},
			policy:        nil,
		},
		{
			name:          "kagent and policy",
			kagent:        &kagent.Client{},
			argo:          nil,
			slack:         nil,
			policy:        &policy.Engine{},
		},
		{
			name:          "all except kagent",
			kagent:        nil,
			argo:          &argo.Client{},
			slack:         &slack.Client{},
			policy:        &policy.Engine{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockStore := &mockIncidentStore{incidents: make(map[string]*domain.Incident)}

			engine := NewEngine(mockStore, tt.kagent, tt.argo, tt.slack, tt.policy)

			if engine == nil {
				t.Fatal("NewEngine() returned nil")
			}

			// Verify each client matches what was passed
			if (engine.kagentClient != nil) != (tt.kagent != nil) {
				t.Error("kagentClient not set correctly")
			}

			if (engine.argoClient != nil) != (tt.argo != nil) {
				t.Error("argoClient not set correctly")
			}

			if (engine.slackClient != nil) != (tt.slack != nil) {
				t.Error("slackClient not set correctly")
			}

			if (engine.policyEngine != nil) != (tt.policy != nil) {
				t.Error("policyEngine not set correctly")
			}
		})
	}
}

// TestInvestigateIncident_StoreEdgeCases tests various store edge cases
func TestInvestigateIncident_StoreEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		incidentID  string
		setupStore  func(*mockIncidentStore)
		expectError bool
	}{
		{
			name:       "empty incident ID",
			incidentID: "",
			setupStore: func(s *mockIncidentStore) {},
			expectError: false, // GetByID returns nil for empty ID
		},
		{
			name:       "very long incident ID",
			incidentID: "inc-" + string(make([]byte, 1000)),
			setupStore: func(s *mockIncidentStore) {},
			expectError: false, // GetByID returns nil for non-existent ID
		},
		{
			name:       "special characters in ID",
			incidentID: "inc-$pecial-ch@rs-123!",
			setupStore: func(s *mockIncidentStore) {},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockStore := &mockIncidentStore{
				incidents: make(map[string]*domain.Incident),
			}
			tt.setupStore(mockStore)

			engine := NewEngine(mockStore, &kagent.Client{}, &argo.Client{}, &slack.Client{}, &policy.Engine{})

			err := engine.InvestigateIncident(context.Background(), tt.incidentID)

			if tt.expectError && err == nil {
				t.Error("InvestigateIncident() expected error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("InvestigateIncident() unexpected error: %v", err)
			}
		})
	}
}

// TestInvestigateIncident_ContextHandling tests context propagation
func TestInvestigateIncident_ContextHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func() context.Context
		wantErr bool
	}{
		{
			name: "normal context",
			setup: func() context.Context {
				return context.Background()
			},
			wantErr: false,
		},
		{
			name: "cancelled context",
			setup: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: false, // Should still return nil for non-existent incident
		},
		{
			name: "expired context",
			setup: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
				defer cancel()
				time.Sleep(1 * time.Millisecond)
				return ctx
			},
			wantErr: false, // Should still return nil for non-existent incident
		},
		{
			name: "context with value",
			setup: func() context.Context {
				key := contextKey("testKey")
				return context.WithValue(context.Background(), key, "value")
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockStore := &mockIncidentStore{
				incidents: make(map[string]*domain.Incident),
			}

			engine := NewEngine(mockStore, &kagent.Client{}, &argo.Client{}, &slack.Client{}, &policy.Engine{})

			ctx := tt.setup()
			err := engine.InvestigateIncident(ctx, "non-existent")

			if tt.wantErr && err == nil {
				t.Error("InvestigateIncident() expected error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("InvestigateIncident() unexpected error: %v", err)
			}
		})
	}
}

// TestInvestigateIncident_IncidentStates tests different incident states
func TestInvestigateIncident_IncidentStates(t *testing.T) {
	t.Parallel()

	now := time.Now()
	states := []domain.IncidentState{
		domain.IncidentStateOpen,
		domain.IncidentStateInvestigating,
		domain.IncidentStateMitigating,
		domain.IncidentStateMonitoring,
		domain.IncidentStateResolved,
	}

	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			t.Parallel()

			incident := &domain.Incident{
				ID:          "inc-state-test",
				Service:     "test-service",
				Cluster:     "test-cluster",
				Environment: "test",
				Severity:    "critical",
				State:       state,
				CreatedAt:   now,
				UpdatedAt:   now,
				FirstAlertAt: now,
				LastAlertAt: now,
			}

			mockStore := &mockIncidentStore{
				incidents: map[string]*domain.Incident{
					"inc-state-test": incident,
				},
			}

			engine := NewEngine(mockStore, kagent.New("http://invalid-test-url"), &argo.Client{}, nil, nil)

			// The engine should process incidents in any state
			// Will fail at kagent but shouldn't panic
			_ = engine.InvestigateIncident(context.Background(), "inc-state-test")
		})
	}
}

// TestInvestigateIncident_IncidentWithTopology tests incidents with topology context
func TestInvestigateIncident_IncidentWithTopology(t *testing.T) {
	t.Parallel()

	topology := &domain.TopologySlice{
		Nodes: []domain.TopologyNode{
			{ID: "node1", Type: domain.NodeTypeService},
			{ID: "node2", Type: domain.NodeTypeDatabase},
		},
		Edges: []domain.TopologyEdge{
			{From: "node1", To: "node2", Type: "depends_on"},
		},
	}

	incident := &domain.Incident{
		ID:              "inc-with-topology",
		Service:         "test-service",
		Cluster:         "test-cluster",
		Environment:     "test",
		Severity:        "critical",
		State:           domain.IncidentStateOpen,
		TopologyContext: topology,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		FirstAlertAt:    time.Now(),
		LastAlertAt:     time.Now(),
	}

	mockStore := &mockIncidentStore{
		incidents: map[string]*domain.Incident{
			"inc-with-topology": incident,
		},
	}

	engine := NewEngine(mockStore, kagent.New("http://invalid-test-url"), &argo.Client{}, nil, nil)

	// The engine should handle incidents with topology context
	// Will fail at kagent.InvestigateAndPlan due to invalid URL, but shouldn't panic
	err := engine.InvestigateIncident(context.Background(), "inc-with-topology")

	// Expect error from kagent client (network error)
	if err == nil {
		t.Error("Expected error from invalid kagent URL")
	}
}

// TestInvestigateIncident_IncidentWithAlerts tests incidents with multiple alerts
func TestInvestigateIncident_IncidentWithAlerts(t *testing.T) {
	t.Parallel()

	now := time.Now()
	endsAt := now.Add(1 * time.Hour)

	incident := &domain.Incident{
		ID:          "inc-with-alerts",
		Service:     "test-service",
		Cluster:     "test-cluster",
		Environment: "test",
		Severity:    "critical",
		State:       domain.IncidentStateOpen,
		Alerts: []domain.Alert{
			{
				Fingerprint: "fp-1",
				Labels: map[string]string{
					"alertname": "HighCPU",
					"severity":  "critical",
				},
				Annotations: map[string]string{
					"summary": "CPU usage high",
				},
				StartsAt: now,
				EndsAt:   nil,
			},
			{
				Fingerprint: "fp-2",
				Labels: map[string]string{
					"alertname": "HighMemory",
					"severity":  "warning",
				},
				Annotations: map[string]string{
					"summary": "Memory usage high",
				},
				StartsAt: now,
				EndsAt:   &endsAt,
			},
		},
		CreatedAt:    now,
		UpdatedAt:    now,
		FirstAlertAt: now,
		LastAlertAt:  now,
	}

	mockStore := &mockIncidentStore{
		incidents: map[string]*domain.Incident{
			"inc-with-alerts": incident,
		},
	}

	engine := NewEngine(mockStore, kagent.New("http://invalid-test-url"), &argo.Client{}, nil, nil)

	// The engine should handle incidents with multiple alerts
	// Will fail at kagent.InvestigateAndPlan due to invalid URL, but shouldn't panic
	err := engine.InvestigateIncident(context.Background(), "inc-with-alerts")

	// Expect error from kagent client (network error)
	if err == nil {
		t.Error("Expected error from invalid kagent URL")
	}
}

// TestInvestigateIncident_DifferentEnvironments tests different environment values
func TestInvestigateIncident_DifferentEnvironments(t *testing.T) {
	t.Parallel()

	environments := []string{"dev", "staging", "production", "test", ""}

	for _, env := range environments {
		t.Run("env-"+env, func(t *testing.T) {
			t.Parallel()

			incident := &domain.Incident{
				ID:           "inc-env-test",
				Service:      "test-service",
				Cluster:      "test-cluster",
				Environment:  env,
				Severity:     "critical",
				State:        domain.IncidentStateOpen,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
				FirstAlertAt: time.Now(),
				LastAlertAt:  time.Now(),
			}

			mockStore := &mockIncidentStore{
				incidents: map[string]*domain.Incident{
					"inc-env-test": incident,
				},
			}

			engine := NewEngine(mockStore, kagent.New("http://invalid-test-url"), &argo.Client{}, nil, nil)

			// The engine should process incidents from any environment
			// Will fail at kagent but shouldn't panic
			_ = engine.InvestigateIncident(context.Background(), "inc-env-test")
		})
	}
}

// TestInvestigateIncident_DifferentSeverities tests different severity values
func TestInvestigateIncident_DifferentSeverities(t *testing.T) {
	t.Parallel()

	severities := []string{"critical", "warning", "info", "debug", ""}

	for _, severity := range severities {
		t.Run("severity-"+severity, func(t *testing.T) {
			t.Parallel()

			incident := &domain.Incident{
				ID:           "inc-severity-test",
				Service:      "test-service",
				Cluster:      "test-cluster",
				Environment:  "test",
				Severity:     severity,
				State:        domain.IncidentStateOpen,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
				FirstAlertAt: time.Now(),
				LastAlertAt:  time.Now(),
			}

			mockStore := &mockIncidentStore{
				incidents: map[string]*domain.Incident{
					"inc-severity-test": incident,
				},
			}

			engine := NewEngine(mockStore, kagent.New("http://invalid-test-url"), &argo.Client{}, nil, nil)

			// The engine should process incidents of any severity
			// Will fail at kagent but shouldn't panic
			_ = engine.InvestigateIncident(context.Background(), "inc-severity-test")
		})
	}
}

// TestInvestigateIncident_CorrelationVersions tests different correlation versions
func TestInvestigateIncident_CorrelationVersions(t *testing.T) {
	t.Parallel()

	versions := []int{0, 1, 5, 10, 100}

	for _, version := range versions {
		t.Run("version", func(t *testing.T) {
			t.Parallel()

			incident := &domain.Incident{
				ID:                 "inc-version-test",
				Service:            "test-service",
				Cluster:            "test-cluster",
				Environment:        "test",
				Severity:           "critical",
				State:              domain.IncidentStateOpen,
				CorrelationVersion: version,
				CreatedAt:          time.Now(),
				UpdatedAt:          time.Now(),
				FirstAlertAt:       time.Now(),
				LastAlertAt:        time.Now(),
			}

			mockStore := &mockIncidentStore{
				incidents: map[string]*domain.Incident{
					"inc-version-test": incident,
				},
			}

			engine := NewEngine(mockStore, kagent.New("http://invalid-test-url"), &argo.Client{}, nil, nil)

			// The engine should process incidents with any correlation version
			// Will fail at kagent but shouldn't panic
			_ = engine.InvestigateIncident(context.Background(), "inc-version-test")
		})
	}
}

// TestEngine_StructureAndFields tests engine structure
func TestEngine_StructureAndFields(t *testing.T) {
	t.Parallel()

	mockStore := &mockIncidentStore{incidents: make(map[string]*domain.Incident)}
	kagentClient := &kagent.Client{}
	argoClient := &argo.Client{}
	slackClient := &slack.Client{}
	policyEngine := &policy.Engine{}

	engine := NewEngine(mockStore, kagentClient, argoClient, slackClient, policyEngine)

	// Verify all fields are accessible (testing struct composition)
	if engine.store == nil {
		t.Error("store field not accessible")
	}

	if engine.kagentClient == nil {
		t.Error("kagentClient field not accessible")
	}

	if engine.argoClient == nil {
		t.Error("argoClient field not accessible")
	}

	if engine.slackClient == nil {
		t.Error("slackClient field not accessible")
	}

	if engine.policyEngine == nil {
		t.Error("policyEngine field not accessible")
	}
}

// TestEngine_MultipleIncidents tests that engine can process multiple incidents
func TestEngine_MultipleIncidents(t *testing.T) {
	t.Parallel()

	now := time.Now()

	mockStore := &mockIncidentStore{
		incidents: map[string]*domain.Incident{
			"inc-1": {
				ID:           "inc-1",
				Service:      "service-1",
				Cluster:      "cluster-1",
				Environment:  "prod",
				Severity:     "critical",
				State:        domain.IncidentStateOpen,
				CreatedAt:    now,
				UpdatedAt:    now,
				FirstAlertAt: now,
				LastAlertAt:  now,
			},
			"inc-2": {
				ID:           "inc-2",
				Service:      "service-2",
				Cluster:      "cluster-2",
				Environment:  "staging",
				Severity:     "warning",
				State:        domain.IncidentStateInvestigating,
				CreatedAt:    now,
				UpdatedAt:    now,
				FirstAlertAt: now,
				LastAlertAt:  now,
			},
			"inc-3": {
				ID:           "inc-3",
				Service:      "service-3",
				Cluster:      "cluster-3",
				Environment:  "dev",
				Severity:     "info",
				State:        domain.IncidentStateResolved,
				CreatedAt:    now,
				UpdatedAt:    now,
				FirstAlertAt: now,
				LastAlertAt:  now,
			},
		},
	}

	engine := NewEngine(mockStore, kagent.New("http://invalid-test-url"), &argo.Client{}, nil, nil)

	// Process multiple incidents
	incidentIDs := []string{"inc-1", "inc-2", "inc-3"}

	for _, id := range incidentIDs {
		// Should not panic for any incident (will fail at kagent but that's ok)
		_ = engine.InvestigateIncident(context.Background(), id)
	}
}
