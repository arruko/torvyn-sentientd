package correlation

import (
	"context"
	"fmt"
	"time"

	"github.com/arruko/torvyn-sentientd/internal/domain"
	"github.com/arruko/torvyn-sentientd/internal/logging"
	"github.com/arruko/torvyn-sentientd/internal/store"
	"github.com/arruko/torvyn-sentientd/internal/topology"
)

type Correlator interface {
	ProcessAlert(ctx context.Context, alert domain.Alert) error
}

type Config struct {
	InitialWindow   time.Duration
	ExpansionWindow time.Duration
}

type correlator struct {
	cfg      Config
	store    store.IncidentStore
	graph    topology.Graph
	idSource IDSource
}

type IDSource interface {
	NewIncidentID() string
}

func New(cfg Config, store store.IncidentStore, graph topology.Graph, idSource IDSource) Correlator {
	return &correlator{
		cfg:      cfg,
		store:    store,
		graph:    graph,
		idSource: idSource,
	}
}

func (c *correlator) ProcessAlert(ctx context.Context, alert domain.Alert) error {
	key := buildCorrelationKey(alert)

	now := time.Now()

	inc, err := c.store.FindOpenByCorrelationKey(ctx, key, c.cfg.ExpansionWindow)
	if err != nil {
		return fmt.Errorf("find incident: %w", err)
	}

	if inc == nil {
		// new incident
		id := c.idSource.NewIncidentID()
		service := primaryServiceFromLabels(alert.Labels)
		env := alert.Labels["environment"]
		cluster := alert.Labels["cluster"]

		topSlice, _ := c.graph.SubgraphForService(service)

		inc = &domain.Incident{
			ID:                 id,
			Service:            service,
			Cluster:            cluster,
			Environment:        env,
			Severity:           severityFromLabels(alert.Labels),
			Alerts:             []domain.Alert{alert},
			CorrelationKey:     key,
			CorrelationVersion: 1,
			State:              domain.IncidentStateOpen,
			CreatedAt:          now,
			UpdatedAt:          now,
			FirstAlertAt:       alert.StartsAt,
			LastAlertAt:        now,
			TopologyContext:    topSlice,
		}
		logging.Info.Printf("Created new incident %s for key %s", id, key)
		return c.store.Save(ctx, inc)
	}

	// update existing incident
	inc.Alerts = append(inc.Alerts, alert)
	inc.LastAlertAt = now
	inc.UpdatedAt = now
	// update severity if needed
	s := severityFromLabels(alert.Labels)
	if s > inc.Severity {
		inc.Severity = s
	}
	inc.CorrelationVersion++

	logging.Info.Printf("Updated incident %s for key %s", inc.ID, key)
	return c.store.Save(ctx, inc)
}

// Simple correlation key based on cluster + namespace + service.
func buildCorrelationKey(alert domain.Alert) string {
	cluster := alert.Labels["cluster"]
	namespace := alert.Labels["namespace"]
	service := primaryServiceFromLabels(alert.Labels)
	return fmt.Sprintf("%s/%s/%s", cluster, namespace, service)
}

func primaryServiceFromLabels(labels map[string]string) string {
	if s, ok := labels["service"]; ok && s != "" {
		return s
	}
	if a, ok := labels["app"]; ok && a != "" {
		return a
	}
	if d, ok := labels["deployment"]; ok && d != "" {
		return d
	}
	return "unknown"
}

func severityFromLabels(labels map[string]string) string {
	if s, ok := labels["severity"]; ok && s != "" {
		return s
	}
	return "unknown"
}
