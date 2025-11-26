package store

import (
	"context"
	"sync"
	"time"

	"github.com/arruko/torvyn-sentientd/internal/domain"
)

type IncidentStore interface {
	GetByID(ctx context.Context, id string) (*domain.Incident, error)
	Save(ctx context.Context, inc *domain.Incident) error
	FindOpenByCorrelationKey(ctx context.Context, key string, window time.Duration) (*domain.Incident, error)
	List(ctx context.Context) ([]*domain.Incident, error)
}

type InMemoryIncidentStore struct {
	mu        sync.RWMutex
	incidents map[string]*domain.Incident
}

func NewInMemoryIncidentStore() *InMemoryIncidentStore {
	return &InMemoryIncidentStore{
		incidents: make(map[string]*domain.Incident),
	}
}

func (s *InMemoryIncidentStore) GetByID(_ context.Context, id string) (*domain.Incident, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inc, ok := s.incidents[id]
	if !ok {
		return nil, nil
	}
	cpy := *inc
	return &cpy, nil
}

func (s *InMemoryIncidentStore) Save(_ context.Context, inc *domain.Incident) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cpy := *inc
	s.incidents[inc.ID] = &cpy
	return nil
}

func (s *InMemoryIncidentStore) FindOpenByCorrelationKey(_ context.Context, key string, window time.Duration) (*domain.Incident, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	var candidate *domain.Incident
	for _, inc := range s.incidents {
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

func (s *InMemoryIncidentStore) List(_ context.Context) ([]*domain.Incident, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.Incident, 0, len(s.incidents))
	for _, inc := range s.incidents {
		cpy := *inc
		out = append(out, &cpy)
	}
	return out, nil
}
