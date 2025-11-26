package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/arruko/torvyn-sentientd/internal/domain"
	pgstore "github.com/arruko/torvyn-sentientd/internal/storage/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresIncidentStore implements IncidentStore using Postgres + sqlc
type PostgresIncidentStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewPostgresIncidentStore creates a new Postgres-backed incident store
func NewPostgresIncidentStore(pool *pgxpool.Pool) *PostgresIncidentStore {
	return &PostgresIncidentStore{
		pool: pool,
		q:    pgstore.New(pool),
	}
}

// GetByID retrieves an incident by ID with all its alerts
func (s *PostgresIncidentStore) GetByID(ctx context.Context, id string) (*domain.Incident, error) {
	row, err := s.q.GetIncidentByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get incident: %w", err)
	}

	alerts, err := s.q.GetAlertsByIncidentID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get alerts for incident: %w", err)
	}

	return rowToIncident(row, alerts)
}

// Save creates or updates an incident with all its alerts
func (s *PostgresIncidentStore) Save(ctx context.Context, inc *domain.Incident) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qtx := s.q.WithTx(tx)

	// Check if incident exists
	existing, err := qtx.GetIncidentByID(ctx, inc.ID)
	isNew := err == pgx.ErrNoRows
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("failed to check existing incident: %w", err)
	}

	var topologyJSON []byte
	if inc.TopologyContext != nil {
		topologyJSON, err = json.Marshal(inc.TopologyContext)
		if err != nil {
			return fmt.Errorf("failed to marshal topology: %w", err)
		}
	}

	if isNew {
		// Create new incident
		_, err = qtx.CreateIncident(ctx, pgstore.CreateIncidentParams{
			ID:                  inc.ID,
			Service:             inc.Service,
			Cluster:             inc.Cluster,
			Environment:         inc.Environment,
			Severity:            inc.Severity,
			CorrelationKey:      inc.CorrelationKey,
			CorrelationVersion:  int32(inc.CorrelationVersion),
			State:               string(inc.State),
			CreatedAt:           inc.CreatedAt,
			UpdatedAt:           inc.UpdatedAt,
			FirstAlertAt:        inc.FirstAlertAt,
			LastAlertAt:         inc.LastAlertAt,
			TopologySnapshot:    topologyJSON,
			RemediationAttempts: 0, // Start at 0
			GithubPrUrl:         nil,
			RcaSummary:          nil,
		})
		if err != nil {
			return fmt.Errorf("failed to create incident: %w", err)
		}
	} else {
		// Update existing incident
		_, err = qtx.UpdateIncident(ctx, pgstore.UpdateIncidentParams{
			ID:                  inc.ID,
			Service:             inc.Service,
			Cluster:             inc.Cluster,
			Environment:         inc.Environment,
			Severity:            inc.Severity,
			CorrelationKey:      inc.CorrelationKey,
			CorrelationVersion:  int32(inc.CorrelationVersion),
			State:               string(inc.State),
			UpdatedAt:           time.Now(),
			FirstAlertAt:        inc.FirstAlertAt,
			LastAlertAt:         inc.LastAlertAt,
			TopologySnapshot:    topologyJSON,
			RemediationAttempts: existing.RemediationAttempts, // Preserve existing count
			GithubPrUrl:         existing.GithubPrUrl,
			RcaSummary:          existing.RcaSummary,
		})
		if err != nil {
			return fmt.Errorf("failed to update incident: %w", err)
		}
	}

	// Save all alerts (upsert based on fingerprint)
	for _, alert := range inc.Alerts {
		labelsJSON, err := json.Marshal(alert.Labels)
		if err != nil {
			return fmt.Errorf("failed to marshal labels: %w", err)
		}

		annotationsJSON, err := json.Marshal(alert.Annotations)
		if err != nil {
			return fmt.Errorf("failed to marshal annotations: %w", err)
		}

		var endsAt pgtype.Timestamptz
		if alert.EndsAt != nil {
			endsAt = pgtype.Timestamptz{
				Time:  *alert.EndsAt,
				Valid: true,
			}
		}

		_, err = qtx.CreateAlert(ctx, pgstore.CreateAlertParams{
			IncidentID:  inc.ID,
			Fingerprint: alert.Fingerprint,
			Labels:      labelsJSON,
			Annotations: annotationsJSON,
			StartsAt:    alert.StartsAt,
			EndsAt:      endsAt,
		})
		if err != nil {
			return fmt.Errorf("failed to save alert: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// FindOpenByCorrelationKey finds the most recent open incident matching the correlation key
func (s *PostgresIncidentStore) FindOpenByCorrelationKey(ctx context.Context, key string, window time.Duration) (*domain.Incident, error) {
	cutoff := time.Now().Add(-window)

	row, err := s.q.FindOpenByCorrelationKey(ctx, pgstore.FindOpenByCorrelationKeyParams{
		CorrelationKey: key,
		LastAlertAt:    cutoff,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find open incident: %w", err)
	}

	alerts, err := s.q.GetAlertsByIncidentID(ctx, row.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get alerts: %w", err)
	}

	return rowToIncident(row, alerts)
}

// List returns all incidents ordered by creation time
func (s *PostgresIncidentStore) List(ctx context.Context) ([]*domain.Incident, error) {
	rows, err := s.q.ListIncidents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list incidents: %w", err)
	}

	incidents := make([]*domain.Incident, 0, len(rows))
	for _, row := range rows {
		alerts, err := s.q.GetAlertsByIncidentID(ctx, row.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get alerts for incident %s: %w", row.ID, err)
		}

		inc, err := rowToIncident(row, alerts)
		if err != nil {
			return nil, err
		}
		incidents = append(incidents, inc)
	}

	return incidents, nil
}

// rowToIncident converts a DB row and alerts to a domain.Incident
func rowToIncident(row pgstore.Incident, alertRows []pgstore.Alert) (*domain.Incident, error) {
	var topology *domain.TopologySlice
	if row.TopologySnapshot != nil {
		if err := json.Unmarshal(row.TopologySnapshot, &topology); err != nil {
			return nil, fmt.Errorf("failed to unmarshal topology: %w", err)
		}
	}

	alerts := make([]domain.Alert, len(alertRows))
	for i, a := range alertRows {
		var labels map[string]string
		if err := json.Unmarshal(a.Labels, &labels); err != nil {
			return nil, fmt.Errorf("failed to unmarshal labels: %w", err)
		}

		var annotations map[string]string
		if err := json.Unmarshal(a.Annotations, &annotations); err != nil {
			return nil, fmt.Errorf("failed to unmarshal annotations: %w", err)
		}

		var endsAt *time.Time
		if a.EndsAt.Valid {
			endsAt = &a.EndsAt.Time
		}

		alerts[i] = domain.Alert{
			Fingerprint: a.Fingerprint,
			Labels:      labels,
			Annotations: annotations,
			StartsAt:    a.StartsAt,
			EndsAt:      endsAt,
		}
	}

	return &domain.Incident{
		ID:                 row.ID,
		Service:            row.Service,
		Cluster:            row.Cluster,
		Environment:        row.Environment,
		Severity:           row.Severity,
		Alerts:             alerts,
		CorrelationKey:     row.CorrelationKey,
		CorrelationVersion: int(row.CorrelationVersion),
		State:              domain.IncidentState(row.State),
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
		FirstAlertAt:       row.FirstAlertAt,
		LastAlertAt:        row.LastAlertAt,
		TopologyContext:    topology,
	}, nil
}

// Health checks the database connection
func (s *PostgresIncidentStore) Health(ctx context.Context) error {
	return s.pool.Ping(ctx)
}
