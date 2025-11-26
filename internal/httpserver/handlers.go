package httpserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/arruko/torvyn-sentientd/internal/domain"
	"github.com/arruko/torvyn-sentientd/internal/logging"
)

type alertmanagerWebhookPayload struct {
	Receiver          string                     `json:"receiver"`
	Status            string                     `json:"status"`
	Alerts            []alertmanagerAlertPayload `json:"alerts"`
	ExternalURL       string                     `json:"externalURL"`
	GroupLabels       map[string]string          `json:"groupLabels"`
	CommonLabels      map[string]string          `json:"commonLabels"`
	CommonAnnotations map[string]string          `json:"commonAnnotations"`
	Version           string                     `json:"version"`
	GroupKey          string                     `json:"groupKey"`
	TruncatedAlerts   int                        `json:"truncatedAlerts"`
}

type alertmanagerAlertPayload struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"startsAt"`
	EndsAt       string            `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

func (s *Server) handleAlertmanagerWebhook(w http.ResponseWriter, r *http.Request) {
	var payload alertmanagerWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		logging.Error.Printf("failed to decode alertmanager payload: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	alerts := make([]domain.Alert, 0, len(payload.Alerts))
	for _, a := range payload.Alerts {
		start, _ := parseTime(a.StartsAt)
		var endPtr *time.Time
		if a.EndsAt != "" {
			if end, err := parseTime(a.EndsAt); err == nil {
				endPtr = &end
			}
		}

		alerts = append(alerts, domain.Alert{
			Fingerprint: a.Fingerprint,
			Labels:      a.Labels,
			Annotations: a.Annotations,
			StartsAt:    start,
			EndsAt:      endPtr,
		})
	}

	// Process alerts synchronously (existing behavior)
	for _, alert := range alerts {
		if err := s.correlator.ProcessAlert(r.Context(), alert); err != nil {
			logging.Error.Printf("correlator failed: %v", err)
			// Do not fail the whole batch for one alert; just log.
		}
	}

	// If NATS is enabled, also publish to queue for async processing
	if s.queue != nil {
		// Create alert batch structure for queue
		alertBatch := map[string]interface{}{
			"receiver":  payload.Receiver,
			"status":    payload.Status,
			"alerts":    alerts,
			"timestamp": time.Now().UTC(),
		}

		batchJSON, err := json.Marshal(alertBatch)
		if err != nil {
			logging.Error.Printf("failed to serialize alert batch for queue: %v", err)
		} else {
			if err := s.queue.Publish(r.Context(), "alerts.ingest", batchJSON); err != nil {
				logging.Error.Printf("failed to publish to queue: %v", err)
				// Don't fail the request - sync processing already succeeded
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func parseTime(s string) (time.Time, error) {
	// Alertmanager uses RFC3339 with timezone.
	return time.Parse(time.RFC3339Nano, s)
}
