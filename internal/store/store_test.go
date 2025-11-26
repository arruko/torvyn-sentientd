package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/arruko/torvyn-sentientd/internal/domain"
)

// Helper function to create test incidents
func newTestIncident(id, correlationKey string, state domain.IncidentState, lastAlertAt time.Time) *domain.Incident {
	return &domain.Incident{
		ID:                 id,
		Service:            "test-service",
		Cluster:            "test-cluster",
		Environment:        "prod",
		Severity:           "critical",
		Alerts:             []domain.Alert{},
		CorrelationKey:     correlationKey,
		CorrelationVersion: 1,
		State:              state,
		CreatedAt:          time.Now().Add(-1 * time.Hour),
		UpdatedAt:          time.Now(),
		FirstAlertAt:       time.Now().Add(-1 * time.Hour),
		LastAlertAt:        lastAlertAt,
		TopologyContext:    nil,
	}
}

func TestInMemoryIncidentStore_SaveAndGetByID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(*InMemoryIncidentStore) *domain.Incident
		wantErr bool
	}{
		{
			name: "save and retrieve incident",
			setup: func(s *InMemoryIncidentStore) *domain.Incident {
				inc := newTestIncident("inc-1", "cluster/ns/svc", domain.IncidentStateOpen, time.Now())
				return inc
			},
			wantErr: false,
		},
		{
			name: "overwrite existing incident",
			setup: func(s *InMemoryIncidentStore) *domain.Incident {
				inc := newTestIncident("inc-1", "cluster/ns/svc", domain.IncidentStateOpen, time.Now())
				_ = s.Save(context.Background(), inc)
				inc.Severity = "warning"
				inc.CorrelationVersion = 2
				return inc
			},
			wantErr: false,
		},
		{
			name: "incident with alerts",
			setup: func(s *InMemoryIncidentStore) *domain.Incident {
				inc := newTestIncident("inc-2", "cluster/ns/svc", domain.IncidentStateOpen, time.Now())
				inc.Alerts = []domain.Alert{
					{
						Fingerprint: "fp-1",
						Labels:      map[string]string{"alertname": "HighCPU"},
						StartsAt:    time.Now(),
					},
				}
				return inc
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := NewInMemoryIncidentStore()
			ctx := context.Background()

			inc := tt.setup(store)

			// Save
			err := store.Save(ctx, inc)
			if (err != nil) != tt.wantErr {
				t.Errorf("Save() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Get
			got, err := store.GetByID(ctx, inc.ID)
			if err != nil {
				t.Fatalf("GetByID() error = %v", err)
			}

			if got == nil {
				t.Fatal("GetByID() returned nil")
			}

			if got.ID != inc.ID {
				t.Errorf("GetByID() ID = %v, want %v", got.ID, inc.ID)
			}

			if got.Severity != inc.Severity {
				t.Errorf("GetByID() Severity = %v, want %v", got.Severity, inc.Severity)
			}

			if got.CorrelationVersion != inc.CorrelationVersion {
				t.Errorf("GetByID() CorrelationVersion = %v, want %v", got.CorrelationVersion, inc.CorrelationVersion)
			}

			// Verify deep copy - modifying returned incident shouldn't affect store
			got.Severity = "modified"
			got2, _ := store.GetByID(ctx, inc.ID)
			if got2.Severity != inc.Severity {
				t.Errorf("Store not isolated, expected %v, got %v", inc.Severity, got2.Severity)
			}
		})
	}
}

func TestInMemoryIncidentStore_GetByID_NotFound(t *testing.T) {
	t.Parallel()

	store := NewInMemoryIncidentStore()
	ctx := context.Background()

	got, err := store.GetByID(ctx, "non-existent")
	if err != nil {
		t.Fatalf("GetByID() unexpected error = %v", err)
	}

	if got != nil {
		t.Errorf("GetByID() = %v, want nil for non-existent ID", got)
	}
}

func TestInMemoryIncidentStore_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(*InMemoryIncidentStore)
		want  int
	}{
		{
			name:  "empty store",
			setup: func(s *InMemoryIncidentStore) {},
			want:  0,
		},
		{
			name: "single incident",
			setup: func(s *InMemoryIncidentStore) {
				inc := newTestIncident("inc-1", "key1", domain.IncidentStateOpen, time.Now())
				_ = s.Save(context.Background(), inc)
			},
			want: 1,
		},
		{
			name: "multiple incidents",
			setup: func(s *InMemoryIncidentStore) {
				for i := 1; i <= 5; i++ {
					inc := newTestIncident("inc-"+string(rune('0'+i)), "key"+string(rune('0'+i)), domain.IncidentStateOpen, time.Now())
					_ = s.Save(context.Background(), inc)
				}
			},
			want: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := NewInMemoryIncidentStore()
			tt.setup(store)

			got, err := store.List(context.Background())
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}

			if len(got) != tt.want {
				t.Errorf("List() returned %v incidents, want %v", len(got), tt.want)
			}

			// Verify deep copy
			if len(got) > 0 {
				got[0].Severity = "modified"
				got2, _ := store.List(context.Background())
				if got2[0].Severity == "modified" {
					t.Error("List() not returning deep copy")
				}
			}
		})
	}
}

func TestInMemoryIncidentStore_FindOpenByCorrelationKey(t *testing.T) {
	t.Parallel()

	now := time.Now()
	window := 5 * time.Minute

	tests := []struct {
		name           string
		setup          func(*InMemoryIncidentStore)
		correlationKey string
		window         time.Duration
		wantID         string
		wantNil        bool
	}{
		{
			name:           "no incidents",
			setup:          func(s *InMemoryIncidentStore) {},
			correlationKey: "cluster/ns/svc",
			window:         window,
			wantNil:        true,
		}, {
			name: "matching incident within window",
			setup: func(s *InMemoryIncidentStore) {
				inc := newTestIncident("inc-1", "cluster/ns/svc", domain.IncidentStateOpen, now.Add(-2*time.Minute))
				_ = s.Save(context.Background(), inc)
			},
			correlationKey: "cluster/ns/svc",
			window:         window,
			wantID:         "inc-1",
		},
		{
			name: "incident outside window",
			setup: func(s *InMemoryIncidentStore) {
				inc := newTestIncident("inc-1", "cluster/ns/svc", domain.IncidentStateOpen, now.Add(-10*time.Minute))
				_ = s.Save(context.Background(), inc)
			},
			correlationKey: "cluster/ns/svc",
			window:         window,
			wantNil:        true,
		},
		{
			name: "resolved incident ignored",
			setup: func(s *InMemoryIncidentStore) {
				inc := newTestIncident("inc-1", "cluster/ns/svc", domain.IncidentStateResolved, now.Add(-2*time.Minute))
				_ = s.Save(context.Background(), inc)
			},
			correlationKey: "cluster/ns/svc",
			window:         window,
			wantNil:        true,
		},
		{
			name: "wrong correlation key",
			setup: func(s *InMemoryIncidentStore) {
				inc := newTestIncident("inc-1", "other/ns/svc", domain.IncidentStateOpen, now.Add(-2*time.Minute))
				_ = s.Save(context.Background(), inc)
			},
			correlationKey: "cluster/ns/svc",
			window:         window,
			wantNil:        true,
		},
		{
			name: "multiple matching incidents - returns most recent",
			setup: func(s *InMemoryIncidentStore) {
				inc1 := newTestIncident("inc-1", "cluster/ns/svc", domain.IncidentStateOpen, now.Add(-4*time.Minute))
				inc2 := newTestIncident("inc-2", "cluster/ns/svc", domain.IncidentStateOpen, now.Add(-2*time.Minute))
				inc3 := newTestIncident("inc-3", "cluster/ns/svc", domain.IncidentStateOpen, now.Add(-3*time.Minute))
				_ = s.Save(context.Background(), inc1)
				_ = s.Save(context.Background(), inc2)
				_ = s.Save(context.Background(), inc3)
			},
			correlationKey: "cluster/ns/svc",
			window:         window,
			wantID:         "inc-2",
		},
		{
			name: "investigating state is open",
			setup: func(s *InMemoryIncidentStore) {
				inc := newTestIncident("inc-1", "cluster/ns/svc", domain.IncidentStateInvestigating, now.Add(-2*time.Minute))
				_ = s.Save(context.Background(), inc)
			},
			correlationKey: "cluster/ns/svc",
			window:         window,
			wantID:         "inc-1",
		},
		{
			name: "mitigating state is open",
			setup: func(s *InMemoryIncidentStore) {
				inc := newTestIncident("inc-1", "cluster/ns/svc", domain.IncidentStateMitigating, now.Add(-2*time.Minute))
				_ = s.Save(context.Background(), inc)
			},
			correlationKey: "cluster/ns/svc",
			window:         window,
			wantID:         "inc-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := NewInMemoryIncidentStore()
			tt.setup(store)

			got, err := store.FindOpenByCorrelationKey(context.Background(), tt.correlationKey, tt.window)
			if err != nil {
				t.Fatalf("FindOpenByCorrelationKey() error = %v", err)
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("FindOpenByCorrelationKey() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("FindOpenByCorrelationKey() = nil, want non-nil")
			}

			if got.ID != tt.wantID {
				t.Errorf("FindOpenByCorrelationKey() ID = %v, want %v", got.ID, tt.wantID)
			}

			// Verify deep copy
			got.Severity = "modified"
			got2, _ := store.FindOpenByCorrelationKey(context.Background(), tt.correlationKey, tt.window)
			if got2 != nil && got2.Severity == "modified" {
				t.Error("FindOpenByCorrelationKey() not returning deep copy")
			}
		})
	}
}

func TestInMemoryIncidentStore_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	store := NewInMemoryIncidentStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				inc := newTestIncident("inc-"+string(rune('0'+id)), "key", domain.IncidentStateOpen, time.Now())
				inc.CorrelationVersion = j
				_ = store.Save(ctx, inc)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, _ = store.GetByID(ctx, "inc-"+string(rune('0'+id)))
				_, _ = store.List(ctx)
				_, _ = store.FindOpenByCorrelationKey(ctx, "key", 5*time.Minute)
			}
		}(i)
	}

	wg.Wait()

	// Verify store is still consistent
	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() after concurrent access failed: %v", err)
	}

	if len(list) != numGoroutines {
		t.Errorf("Expected %d incidents after concurrent writes, got %d", numGoroutines, len(list))
	}
}
