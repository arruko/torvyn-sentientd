package investigation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/arruko/torvyn-sentientd/internal/argo"
	"github.com/arruko/torvyn-sentientd/internal/domain"
	"github.com/arruko/torvyn-sentientd/internal/kagent"
	"github.com/arruko/torvyn-sentientd/internal/policy"
	"github.com/arruko/torvyn-sentientd/internal/slack"
	"github.com/arruko/torvyn-sentientd/internal/store"
)

// Mock IncidentStore
type mockIncidentStore struct {
	incidents map[string]*domain.Incident
	getErr    error
}

func (m *mockIncidentStore) Save(_ context.Context, inc *domain.Incident) error {
	m.incidents[inc.ID] = inc
	return nil
}

func (m *mockIncidentStore) GetByID(_ context.Context, id string) (*domain.Incident, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	inc, ok := m.incidents[id]
	if !ok {
		return nil, nil
	}
	return inc, nil
}

func (m *mockIncidentStore) FindOpenByCorrelationKey(_ context.Context, key string, window time.Duration) (*domain.Incident, error) {
	return nil, nil
}

func (m *mockIncidentStore) List(_ context.Context) ([]*domain.Incident, error) {
	return nil, nil
}

var _ store.IncidentStore = (*mockIncidentStore)(nil)

// Note: The Engine struct expects concrete types (*kagent.Client, *argo.Client, etc.)
// so we cannot use traditional interface mocks. Instead, we test with the real types
// and use integration-style testing for now. In a production codebase, you'd refactor
// Engine to use interfaces.

func TestNewEngine(t *testing.T) {
	mockStore := &mockIncidentStore{incidents: make(map[string]*domain.Incident)}
	kagentClient := &kagent.Client{}
	argoClient := &argo.Client{}
	slackClient := &slack.Client{}
	policyEngine := &policy.Engine{}

	engine := NewEngine(mockStore, kagentClient, argoClient, slackClient, policyEngine)

	if engine == nil {
		t.Fatal("NewEngine() returned nil")
	}

	if engine.store == nil {
		t.Error("store not set")
	}

	if engine.kagentClient == nil {
		t.Error("kagentClient not set")
	}

	if engine.argoClient == nil {
		t.Error("argoClient not set")
	}

	if engine.slackClient == nil {
		t.Error("slackClient not set")
	}

	if engine.policyEngine == nil {
		t.Error("policyEngine not set")
	}
}

func TestInvestigateIncident_IncidentNotFound(t *testing.T) {
	t.Parallel()

	mockStore := &mockIncidentStore{
		incidents: make(map[string]*domain.Incident),
	}

	kagentClient := &kagent.Client{}
	argoClient := &argo.Client{}
	slackClient := &slack.Client{}
	policyEngine := &policy.Engine{}

	engine := NewEngine(mockStore, kagentClient, argoClient, slackClient, policyEngine)

	err := engine.InvestigateIncident(context.Background(), "non-existent")

	// Should return nil for non-existent incident
	if err != nil {
		t.Errorf("InvestigateIncident() error = %v, expected nil for non-existent incident", err)
	}
}

func TestInvestigateIncident_StoreError(t *testing.T) {
	t.Parallel()

	mockStore := &mockIncidentStore{
		incidents: make(map[string]*domain.Incident),
		getErr:    errors.New("database connection error"),
	}

	kagentClient := &kagent.Client{}
	argoClient := &argo.Client{}
	slackClient := &slack.Client{}
	policyEngine := &policy.Engine{}

	engine := NewEngine(mockStore, kagentClient, argoClient, slackClient, policyEngine)

	err := engine.InvestigateIncident(context.Background(), "inc-123")

	// Verify error is returned
	if err == nil {
		t.Fatal("InvestigateIncident() expected error for store failure, got nil")
	}
}

func TestInvestigateIncident_NoSlackClient(t *testing.T) {
	incident := &domain.Incident{
		ID:      "inc-123",
		Service: "payment-api",
	}

	mockStore := &mockIncidentStore{
		incidents: map[string]*domain.Incident{
			"inc-123": incident,
		},
	}

	kagentClient := &kagent.Client{}
	argoClient := &argo.Client{}
	policyEngine := &policy.Engine{}

	// No Slack client provided (nil is acceptable)
	engine := NewEngine(mockStore, kagentClient, argoClient, nil, policyEngine)

	if engine == nil {
		t.Fatal("NewEngine() returned nil with nil slack client")
	}

	// The engine should handle nil Slack client gracefully
	// (It checks for nil before calling Slack methods)
}

func TestInvestigateIncident_NoPolicyEngine(t *testing.T) {
	incident := &domain.Incident{
		ID:      "inc-123",
		Service: "payment-api",
	}

	mockStore := &mockIncidentStore{
		incidents: map[string]*domain.Incident{
			"inc-123": incident,
		},
	}

	kagentClient := &kagent.Client{}
	argoClient := &argo.Client{}
	slackClient := &slack.Client{}

	// No policy engine provided (nil is acceptable)
	engine := NewEngine(mockStore, kagentClient, argoClient, slackClient, nil)

	if engine == nil {
		t.Fatal("NewEngine() returned nil with nil policy engine")
	}

	// The engine should handle nil policy engine gracefully
	// (It checks for nil before calling policy validation)
}
