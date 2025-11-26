package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arruko/torvyn-sentientd/internal/correlation"
	"github.com/arruko/torvyn-sentientd/internal/domain"
	"github.com/arruko/torvyn-sentientd/internal/queue"
)

// Mock correlator for testing
type mockCorrelator struct {
	processedAlerts []domain.Alert
	processError    error
}

func (m *mockCorrelator) ProcessAlert(_ context.Context, alert domain.Alert) error {
	m.processedAlerts = append(m.processedAlerts, alert)
	return m.processError
}

var _ correlation.Correlator = (*mockCorrelator)(nil)

// Mock queue for testing
type mockQueue struct {
	publishedMessages []mockPublishedMessage
	publishError      error
}

type mockPublishedMessage struct {
	subject string
	data    []byte
}

func (m *mockQueue) Publish(_ context.Context, subject string, data []byte) error {
	m.publishedMessages = append(m.publishedMessages, mockPublishedMessage{
		subject: subject,
		data:    data,
	})
	return m.publishError
}

func (m *mockQueue) Subscribe(subject string, handler queue.HandlerFunc) error {
	return nil
}

func (m *mockQueue) Close() error {
	return nil
}

func TestHandleAlertmanagerWebhook_ValidPayload(t *testing.T) {
	t.Parallel()

	correlator := &mockCorrelator{}
	server := &Server{
		correlator: correlator,
	}

	payload := alertmanagerWebhookPayload{
		Receiver: "sentientd",
		Status:   "firing",
		Alerts: []alertmanagerAlertPayload{
			{
				Status: "firing",
				Labels: map[string]string{
					"alertname":   "HighCPU",
					"service":     "payment-api",
					"severity":    "critical",
					"cluster":     "prod-us",
					"namespace":   "payments",
					"environment": "production",
				},
				Annotations: map[string]string{
					"summary":     "CPU usage high",
					"description": "CPU usage is above 80%",
				},
				StartsAt:     time.Now().Format(time.RFC3339Nano),
				EndsAt:       "",
				GeneratorURL: "http://prometheus/alerts",
				Fingerprint:  "fp-12345",
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/alertmanager/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	server.handleAlertmanagerWebhook(rr, req)

	// Verify response
	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
	}

	if rr.Body.String() != "ok\n" {
		t.Errorf("Body = %q, want %q", rr.Body.String(), "ok\n")
	}

	// Verify correlator was called
	if len(correlator.processedAlerts) != 1 {
		t.Fatalf("ProcessAlert called %d times, want 1", len(correlator.processedAlerts))
	}

	alert := correlator.processedAlerts[0]

	if alert.Fingerprint != "fp-12345" {
		t.Errorf("Fingerprint = %v, want fp-12345", alert.Fingerprint)
	}

	if alert.Labels["alertname"] != "HighCPU" {
		t.Errorf("alertname = %v, want HighCPU", alert.Labels["alertname"])
	}

	if alert.Labels["service"] != "payment-api" {
		t.Errorf("service = %v, want payment-api", alert.Labels["service"])
	}

	if alert.Annotations["summary"] != "CPU usage high" {
		t.Errorf("summary = %v, want 'CPU usage high'", alert.Annotations["summary"])
	}
}

func TestHandleAlertmanagerWebhook_MultipleAlerts(t *testing.T) {
	t.Parallel()

	correlator := &mockCorrelator{}
	server := &Server{
		correlator: correlator,
	}

	payload := alertmanagerWebhookPayload{
		Receiver: "sentientd",
		Status:   "firing",
		Alerts: []alertmanagerAlertPayload{
			{
				Status: "firing",
				Labels: map[string]string{
					"alertname": "HighCPU",
					"service":   "payment-api",
				},
				Fingerprint: "fp-1",
				StartsAt:    time.Now().Format(time.RFC3339Nano),
			},
			{
				Status: "firing",
				Labels: map[string]string{
					"alertname": "HighMemory",
					"service":   "payment-api",
				},
				Fingerprint: "fp-2",
				StartsAt:    time.Now().Format(time.RFC3339Nano),
			},
			{
				Status: "firing",
				Labels: map[string]string{
					"alertname": "DatabaseDown",
					"service":   "postgres",
				},
				Fingerprint: "fp-3",
				StartsAt:    time.Now().Format(time.RFC3339Nano),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/alertmanager/webhook", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	server.handleAlertmanagerWebhook(rr, req)

	// Verify all alerts were processed
	if len(correlator.processedAlerts) != 3 {
		t.Errorf("ProcessAlert called %d times, want 3", len(correlator.processedAlerts))
	}

	// Verify fingerprints are unique
	fingerprints := make(map[string]bool)
	for _, alert := range correlator.processedAlerts {
		fingerprints[alert.Fingerprint] = true
	}

	if len(fingerprints) != 3 {
		t.Errorf("Unique fingerprints = %d, want 3", len(fingerprints))
	}

	if !fingerprints["fp-1"] || !fingerprints["fp-2"] || !fingerprints["fp-3"] {
		t.Errorf("Fingerprints = %v, want [fp-1, fp-2, fp-3]", fingerprints)
	}
}

func TestHandleAlertmanagerWebhook_WithQueue(t *testing.T) {
	t.Parallel()

	correlator := &mockCorrelator{}
	queue := &mockQueue{}

	server := &Server{
		correlator: correlator,
		queue:      queue,
	}

	payload := alertmanagerWebhookPayload{
		Receiver: "sentientd",
		Status:   "firing",
		Alerts: []alertmanagerAlertPayload{
			{
				Status:      "firing",
				Labels:      map[string]string{"alertname": "Test"},
				Fingerprint: "fp-1",
				StartsAt:    time.Now().Format(time.RFC3339Nano),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/alertmanager/webhook", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	server.handleAlertmanagerWebhook(rr, req)

	// Verify queue was called
	if len(queue.publishedMessages) != 1 {
		t.Fatalf("Queue Publish called %d times, want 1", len(queue.publishedMessages))
	}

	msg := queue.publishedMessages[0]

	if msg.subject != "alerts.ingest" {
		t.Errorf("Subject = %v, want alerts.ingest", msg.subject)
	}

	// Verify message structure
	var alertBatch map[string]interface{}
	if err := json.Unmarshal(msg.data, &alertBatch); err != nil {
		t.Fatalf("Failed to unmarshal queue message: %v", err)
	}

	if alertBatch["receiver"] != "sentientd" {
		t.Errorf("receiver = %v, want sentientd", alertBatch["receiver"])
	}

	if alertBatch["status"] != "firing" {
		t.Errorf("status = %v, want firing", alertBatch["status"])
	}
}

func TestHandleAlertmanagerWebhook_InvalidJSON(t *testing.T) {
	t.Parallel()

	correlator := &mockCorrelator{}
	server := &Server{
		correlator: correlator,
	}

	req := httptest.NewRequest(http.MethodPost, "/alertmanager/webhook", bytes.NewReader([]byte("invalid json")))
	rr := httptest.NewRecorder()

	server.handleAlertmanagerWebhook(rr, req)

	// Verify bad request response
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	// Verify correlator was not called
	if len(correlator.processedAlerts) != 0 {
		t.Errorf("ProcessAlert called %d times, want 0", len(correlator.processedAlerts))
	}
}

func TestHandleAlertmanagerWebhook_CorrelatorError(t *testing.T) {
	t.Parallel()

	correlator := &mockCorrelator{
		processError: context.DeadlineExceeded,
	}

	server := &Server{
		correlator: correlator,
	}

	payload := alertmanagerWebhookPayload{
		Receiver: "sentientd",
		Status:   "firing",
		Alerts: []alertmanagerAlertPayload{
			{
				Status:      "firing",
				Labels:      map[string]string{"alertname": "Test"},
				Fingerprint: "fp-1",
				StartsAt:    time.Now().Format(time.RFC3339Nano),
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/alertmanager/webhook", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	server.handleAlertmanagerWebhook(rr, req)

	// Verify request still succeeds (correlator errors are logged but don't fail the request)
	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d (errors should not fail the request)", rr.Code, http.StatusOK)
	}
}

func TestHandleAlertmanagerWebhook_TimestampParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		startsAt  string
		endsAt    string
		wantError bool
	}{
		{
			name:      "valid RFC3339Nano timestamps",
			startsAt:  "2024-01-15T10:30:00.123456789Z",
			endsAt:    "2024-01-15T10:35:00.987654321Z",
			wantError: false,
		},
		{
			name:      "valid RFC3339 timestamps",
			startsAt:  "2024-01-15T10:30:00Z",
			endsAt:    "2024-01-15T10:35:00Z",
			wantError: false,
		},
		{
			name:      "empty endsAt",
			startsAt:  "2024-01-15T10:30:00Z",
			endsAt:    "",
			wantError: false,
		},
		{
			name:      "with timezone offset",
			startsAt:  "2024-01-15T10:30:00-05:00",
			endsAt:    "",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			correlator := &mockCorrelator{}
			server := &Server{
				correlator: correlator,
			}

			payload := alertmanagerWebhookPayload{
				Receiver: "sentientd",
				Status:   "firing",
				Alerts: []alertmanagerAlertPayload{
					{
						Status:      "firing",
						Labels:      map[string]string{"alertname": "Test"},
						Fingerprint: "fp-1",
						StartsAt:    tt.startsAt,
						EndsAt:      tt.endsAt,
					},
				},
			}

			body, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/alertmanager/webhook", bytes.NewReader(body))
			rr := httptest.NewRecorder()

			server.handleAlertmanagerWebhook(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
			}

			if len(correlator.processedAlerts) != 1 {
				t.Fatalf("ProcessAlert called %d times, want 1", len(correlator.processedAlerts))
			}

			alert := correlator.processedAlerts[0]

			// Verify StartsAt was parsed
			if alert.StartsAt.IsZero() {
				t.Error("StartsAt is zero, should be parsed")
			}

			// Verify EndsAt handling
			if tt.endsAt == "" {
				if alert.EndsAt != nil {
					t.Error("EndsAt should be nil for empty string")
				}
			} else {
				if alert.EndsAt == nil {
					t.Error("EndsAt should not be nil")
				}
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "RFC3339Nano",
			input:     "2024-01-15T10:30:00.123456789Z",
			wantError: false,
		},
		{
			name:      "RFC3339",
			input:     "2024-01-15T10:30:00Z",
			wantError: false,
		},
		{
			name:      "with timezone",
			input:     "2024-01-15T10:30:00-05:00",
			wantError: false,
		},
		{
			name:      "invalid format",
			input:     "2024-01-15 10:30:00",
			wantError: true,
		},
		{
			name:      "empty string",
			input:     "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := parseTime(tt.input)

			if tt.wantError {
				if err == nil {
					t.Error("parseTime() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("parseTime() unexpected error: %v", err)
				}

				if result.IsZero() {
					t.Error("parseTime() returned zero time")
				}
			}
		})
	}
}

func TestHandleAlertmanagerWebhook_EmptyAlerts(t *testing.T) {
	t.Parallel()

	correlator := &mockCorrelator{}
	server := &Server{
		correlator: correlator,
	}

	payload := alertmanagerWebhookPayload{
		Receiver: "sentientd",
		Status:   "resolved",
		Alerts:   []alertmanagerAlertPayload{},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/alertmanager/webhook", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	server.handleAlertmanagerWebhook(rr, req)

	// Verify success even with no alerts
	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
	}

	// Verify correlator was not called
	if len(correlator.processedAlerts) != 0 {
		t.Errorf("ProcessAlert called %d times, want 0", len(correlator.processedAlerts))
	}
}
