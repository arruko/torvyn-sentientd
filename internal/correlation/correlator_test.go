package correlation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/arruko/torvyn-sentientd/internal/domain"
)

// Mock implementations for testing

type mockIDSource struct {
	counter int
}

func (m *mockIDSource) NewIncidentID() string {
	m.counter++
	return "inc-" + string(rune('0'+m.counter))
}

type mockIncidentStore struct {
	incidents map[string]*domain.Incident
	saveCalls int
}

func newMockIncidentStore() *mockIncidentStore {
	return &mockIncidentStore{
		incidents: make(map[string]*domain.Incident),
	}
}

func (m *mockIncidentStore) GetByID(_ context.Context, id string) (*domain.Incident, error) {
	inc, ok := m.incidents[id]
	if !ok {
		return nil, nil
	}
	cpy := *inc
	return &cpy, nil
}

func (m *mockIncidentStore) Save(_ context.Context, inc *domain.Incident) error {
	m.saveCalls++
	cpy := *inc
	m.incidents[inc.ID] = &cpy
	return nil
}

func (m *mockIncidentStore) FindOpenByCorrelationKey(_ context.Context, key string, window time.Duration) (*domain.Incident, error) {
	now := time.Now()
	var candidate *domain.Incident
	for _, inc := range m.incidents {
		if inc.CorrelationKey != key {
			continue
		}
		if inc.State == domain.IncidentStateResolved {
			continue
		}
		if now.Sub(inc.LastAlertAt) > window {
			continue
		}
		if candidate == nil || inc.LastAlertAt.After(candidate.LastAlertAt) {
			candidate = inc
		}
	}
	if candidate == nil {
		return nil, nil
	}
	cpy := *candidate
	return &cpy, nil
}

func (m *mockIncidentStore) List(_ context.Context) ([]*domain.Incident, error) {
	out := make([]*domain.Incident, 0, len(m.incidents))
	for _, inc := range m.incidents {
		cpy := *inc
		out = append(out, &cpy)
	}
	return out, nil
}

type mockGraph struct {
	subgraphs map[string]*domain.TopologySlice
}

func newMockGraph() *mockGraph {
	return &mockGraph{
		subgraphs: make(map[string]*domain.TopologySlice),
	}
}

func (m *mockGraph) SubgraphForService(service string) (*domain.TopologySlice, error) {
	if slice, ok := m.subgraphs[service]; ok {
		return slice, nil
	}
	return nil, nil
}

// Helper function to create test alerts
func newTestAlert(fingerprint string, labels map[string]string) domain.Alert {
	return domain.Alert{
		Fingerprint: fingerprint,
		Labels:      labels,
		Annotations: map[string]string{},
		StartsAt:    time.Now(),
		EndsAt:      nil,
	}
}

func TestBuildCorrelationKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		alert  domain.Alert
		want   string
	}{
		{
			name: "all labels present",
			alert: newTestAlert("fp-1", map[string]string{
				"cluster":   "prod-us",
				"namespace": "payments",
				"service":   "payment-api",
			}),
			want: "prod-us/payments/payment-api",
		},
		{
			name: "missing namespace",
			alert: newTestAlert("fp-1", map[string]string{
				"cluster": "prod-us",
				"service": "payment-api",
			}),
			want: "prod-us//payment-api",
		},
		{
			name: "missing service, uses app",
			alert: newTestAlert("fp-1", map[string]string{
				"cluster":   "prod-us",
				"namespace": "payments",
				"app":       "payment-service",
			}),
			want: "prod-us/payments/payment-service",
		},
		{
			name: "missing service and app, uses deployment",
			alert: newTestAlert("fp-1", map[string]string{
				"cluster":    "prod-us",
				"namespace":  "payments",
				"deployment": "payment-deploy",
			}),
			want: "prod-us/payments/payment-deploy",
		},
		{
			name: "no service identifiers",
			alert: newTestAlert("fp-1", map[string]string{
				"cluster":   "prod-us",
				"namespace": "payments",
			}),
			want: "prod-us/payments/unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildCorrelationKey(tt.alert)
			if got != tt.want {
				t.Errorf("buildCorrelationKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrimaryServiceFromLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{
			name:   "service label present",
			labels: map[string]string{"service": "payment-api"},
			want:   "payment-api",
		},
		{
			name:   "app label present, no service",
			labels: map[string]string{"app": "payment-service"},
			want:   "payment-service",
		},
		{
			name:   "deployment label present, no service or app",
			labels: map[string]string{"deployment": "payment-deploy"},
			want:   "payment-deploy",
		},
		{
			name:   "service takes precedence over app",
			labels: map[string]string{"service": "svc", "app": "app"},
			want:   "svc",
		},
		{
			name:   "app takes precedence over deployment",
			labels: map[string]string{"app": "app", "deployment": "deploy"},
			want:   "app",
		},
		{
			name:   "empty service label is ignored",
			labels: map[string]string{"service": "", "app": "payment-service"},
			want:   "payment-service",
		},
		{
			name:   "no service identifiers",
			labels: map[string]string{"cluster": "prod"},
			want:   "unknown",
		},
		{
			name:   "empty labels",
			labels: map[string]string{},
			want:   "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := primaryServiceFromLabels(tt.labels)
			if got != tt.want {
				t.Errorf("primaryServiceFromLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeverityFromLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{
			name:   "severity present",
			labels: map[string]string{"severity": "critical"},
			want:   "critical",
		},
		{
			name:   "severity warning",
			labels: map[string]string{"severity": "warning"},
			want:   "warning",
		},
		{
			name:   "empty severity",
			labels: map[string]string{"severity": ""},
			want:   "unknown",
		},
		{
			name:   "no severity label",
			labels: map[string]string{"alertname": "HighCPU"},
			want:   "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := severityFromLabels(tt.labels)
			if got != tt.want {
				t.Errorf("severityFromLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCorrelator_ProcessAlert_NewIncident(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		alert         domain.Alert
		wantService   string
		wantSeverity  string
		wantAlertCnt  int
		wantCorrKey   string
	}{
		{
			name: "new alert creates incident",
			alert: newTestAlert("fp-1", map[string]string{
				"cluster":     "prod-us",
				"namespace":   "payments",
				"service":     "payment-api",
				"severity":    "critical",
				"environment": "production",
			}),
			wantService:  "payment-api",
			wantSeverity: "critical",
			wantAlertCnt: 1,
			wantCorrKey:  "prod-us/payments/payment-api",
		},
		{
			name: "new alert with app label",
			alert: newTestAlert("fp-2", map[string]string{
				"cluster":     "prod-us",
				"namespace":   "payments",
				"app":         "payment-service",
				"severity":    "warning",
				"environment": "staging",
			}),
			wantService:  "payment-service",
			wantSeverity: "warning",
			wantAlertCnt: 1,
			wantCorrKey:  "prod-us/payments/payment-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newMockIncidentStore()
			graph := newMockGraph()
			idSource := &mockIDSource{}

			cfg := Config{
				InitialWindow:   5 * time.Minute,
				ExpansionWindow: 10 * time.Minute,
			}

			correlator := New(cfg, store, graph, idSource)

			err := correlator.ProcessAlert(context.Background(), tt.alert)
			if err != nil {
				t.Fatalf("ProcessAlert() error = %v", err)
			}

			// Verify incident was created
			if store.saveCalls != 1 {
				t.Errorf("Expected 1 save call, got %d", store.saveCalls)
			}

			// Verify incident properties
			incidents, _ := store.List(context.Background())
			if len(incidents) != 1 {
				t.Fatalf("Expected 1 incident, got %d", len(incidents))
			}

			inc := incidents[0]
			if inc.Service != tt.wantService {
				t.Errorf("Service = %v, want %v", inc.Service, tt.wantService)
			}

			if inc.Severity != tt.wantSeverity {
				t.Errorf("Severity = %v, want %v", inc.Severity, tt.wantSeverity)
			}

			if len(inc.Alerts) != tt.wantAlertCnt {
				t.Errorf("Alert count = %v, want %v", len(inc.Alerts), tt.wantAlertCnt)
			}

			if inc.CorrelationKey != tt.wantCorrKey {
				t.Errorf("CorrelationKey = %v, want %v", inc.CorrelationKey, tt.wantCorrKey)
			}

			if inc.State != domain.IncidentStateOpen {
				t.Errorf("State = %v, want %v", inc.State, domain.IncidentStateOpen)
			}

			if inc.CorrelationVersion != 1 {
				t.Errorf("CorrelationVersion = %v, want 1", inc.CorrelationVersion)
			}

			if !strings.HasPrefix(inc.ID, "inc-") {
				t.Errorf("ID = %v, expected to start with 'inc-'", inc.ID)
			}
		})
	}
}

func TestCorrelator_ProcessAlert_UpdateExisting(t *testing.T) {
	t.Parallel()

	store := newMockIncidentStore()
	graph := newMockGraph()
	idSource := &mockIDSource{}

	cfg := Config{
		InitialWindow:   5 * time.Minute,
		ExpansionWindow: 10 * time.Minute,
	}

	correlator := New(cfg, store, graph, idSource)
	ctx := context.Background()

	// First alert creates incident
	alert1 := newTestAlert("fp-1", map[string]string{
		"cluster":   "prod-us",
		"namespace": "payments",
		"service":   "payment-api",
		"severity":  "low",
	})

	err := correlator.ProcessAlert(ctx, alert1)
	if err != nil {
		t.Fatalf("ProcessAlert() first alert error = %v", err)
	}

	incidents, _ := store.List(ctx)
	if len(incidents) != 1 {
		t.Fatalf("Expected 1 incident after first alert, got %d", len(incidents))
	}

	initialIncID := incidents[0].ID
	initialVersion := incidents[0].CorrelationVersion

	// Second alert with same correlation key and higher severity (lexicographically)
	alert2 := newTestAlert("fp-2", map[string]string{
		"cluster":   "prod-us",
		"namespace": "payments",
		"service":   "payment-api",
		"severity":  "medium",
	})

	time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamp

	err = correlator.ProcessAlert(ctx, alert2)
	if err != nil {
		t.Fatalf("ProcessAlert() second alert error = %v", err)
	}

	// Verify incident was updated, not duplicated
	incidents, _ = store.List(ctx)
	if len(incidents) != 1 {
		t.Fatalf("Expected 1 incident after second alert, got %d", len(incidents))
	}

	inc := incidents[0]

	// Verify same incident ID
	if inc.ID != initialIncID {
		t.Errorf("Incident ID changed from %v to %v", initialIncID, inc.ID)
	}

	// Verify alert count
	if len(inc.Alerts) != 2 {
		t.Errorf("Alert count = %v, want 2", len(inc.Alerts))
	}

	// Verify severity escalation (lexicographic: "medium" > "low")
	if inc.Severity != "medium" {
		t.Errorf("Severity = %v, want medium (escalated from low)", inc.Severity)
	}

	// Verify version increment
	if inc.CorrelationVersion != initialVersion+1 {
		t.Errorf("CorrelationVersion = %v, want %v", inc.CorrelationVersion, initialVersion+1)
	}

	// Verify save was called twice
	if store.saveCalls != 2 {
		t.Errorf("Expected 2 save calls, got %d", store.saveCalls)
	}
}

func TestCorrelator_ProcessAlert_WindowExpired(t *testing.T) {
	t.Parallel()

	store := newMockIncidentStore()
	graph := newMockGraph()
	idSource := &mockIDSource{}

	cfg := Config{
		InitialWindow:   5 * time.Minute,
		ExpansionWindow: 1 * time.Millisecond, // Very short window
	}

	correlator := New(cfg, store, graph, idSource)
	ctx := context.Background()

	// First alert
	alert1 := newTestAlert("fp-1", map[string]string{
		"cluster":   "prod-us",
		"namespace": "payments",
		"service":   "payment-api",
		"severity":  "warning",
	})

	err := correlator.ProcessAlert(ctx, alert1)
	if err != nil {
		t.Fatalf("ProcessAlert() first alert error = %v", err)
	}

	// Wait for window to expire
	time.Sleep(10 * time.Millisecond)

	// Second alert with same correlation key
	alert2 := newTestAlert("fp-2", map[string]string{
		"cluster":   "prod-us",
		"namespace": "payments",
		"service":   "payment-api",
		"severity":  "critical",
	})

	err = correlator.ProcessAlert(ctx, alert2)
	if err != nil {
		t.Fatalf("ProcessAlert() second alert error = %v", err)
	}

	// Verify two separate incidents were created
	incidents, _ := store.List(ctx)
	if len(incidents) != 2 {
		t.Errorf("Expected 2 incidents (window expired), got %d", len(incidents))
	}
}

func TestCorrelator_ProcessAlert_ResolvedIncidentIgnored(t *testing.T) {
	t.Parallel()

	store := newMockIncidentStore()
	graph := newMockGraph()
	idSource := &mockIDSource{}

	cfg := Config{
		InitialWindow:   5 * time.Minute,
		ExpansionWindow: 10 * time.Minute,
	}

	// Pre-populate store with resolved incident
	resolvedInc := &domain.Incident{
		ID:                 "inc-resolved",
		Service:            "payment-api",
		Cluster:            "prod-us",
		Environment:        "prod",
		Severity:           "critical",
		Alerts:             []domain.Alert{},
		CorrelationKey:     "prod-us/payments/payment-api",
		CorrelationVersion: 1,
		State:              domain.IncidentStateResolved,
		CreatedAt:          time.Now().Add(-1 * time.Hour),
		UpdatedAt:          time.Now(),
		FirstAlertAt:       time.Now().Add(-1 * time.Hour),
		LastAlertAt:        time.Now(),
	}
	_ = store.Save(context.Background(), resolvedInc)

	correlator := New(cfg, store, graph, idSource)

	// New alert with same correlation key
	alert := newTestAlert("fp-1", map[string]string{
		"cluster":   "prod-us",
		"namespace": "payments",
		"service":   "payment-api",
		"severity":  "warning",
	})

	err := correlator.ProcessAlert(context.Background(), alert)
	if err != nil {
		t.Fatalf("ProcessAlert() error = %v", err)
	}

	// Verify new incident was created (resolved incident ignored)
	incidents, _ := store.List(context.Background())
	if len(incidents) != 2 {
		t.Fatalf("Expected 2 incidents (1 resolved + 1 new), got %d", len(incidents))
	}

	// Find the non-resolved incident
	var openInc *domain.Incident
	for _, inc := range incidents {
		if inc.State != domain.IncidentStateResolved {
			openInc = inc
			break
		}
	}

	if openInc == nil {
		t.Fatal("No open incident found")
	}

	if openInc.ID == resolvedInc.ID {
		t.Error("Resolved incident was reused instead of creating new one")
	}
}

func TestCorrelator_ProcessAlert_WithTopologyContext(t *testing.T) {
	t.Parallel()

	store := newMockIncidentStore()
	graph := newMockGraph()
	idSource := &mockIDSource{}

	// Setup mock graph with topology
	topologySlice := &domain.TopologySlice{
		Nodes: []domain.TopologyNode{
			{ID: "svc:payment-api", Type: domain.NodeTypeService},
		},
		Edges: []domain.TopologyEdge{
			{From: "svc:payment-api", To: "db:postgres", Type: "depends_on"},
		},
	}
	graph.subgraphs["payment-api"] = topologySlice

	cfg := Config{
		InitialWindow:   5 * time.Minute,
		ExpansionWindow: 10 * time.Minute,
	}

	correlator := New(cfg, store, graph, idSource)

	alert := newTestAlert("fp-1", map[string]string{
		"cluster":   "prod-us",
		"namespace": "payments",
		"service":   "payment-api",
		"severity":  "critical",
	})

	err := correlator.ProcessAlert(context.Background(), alert)
	if err != nil {
		t.Fatalf("ProcessAlert() error = %v", err)
	}

	incidents, _ := store.List(context.Background())
	if len(incidents) != 1 {
		t.Fatalf("Expected 1 incident, got %d", len(incidents))
	}

	inc := incidents[0]

	if inc.TopologyContext == nil {
		t.Fatal("TopologyContext is nil, expected non-nil")
	}

	if len(inc.TopologyContext.Nodes) != 1 {
		t.Errorf("TopologyContext.Nodes count = %v, want 1", len(inc.TopologyContext.Nodes))
	}

	if len(inc.TopologyContext.Edges) != 1 {
		t.Errorf("TopologyContext.Edges count = %v, want 1", len(inc.TopologyContext.Edges))
	}
}
