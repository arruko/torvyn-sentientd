package slack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arruko/torvyn-sentientd/internal/domain"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		webhookURL string
		wantNil    bool
	}{
		{
			name:       "valid webhook URL",
			webhookURL: "https://hooks.slack.com/services/TEST",
			wantNil:    false,
		},
		{
			name:       "empty webhook URL returns nil",
			webhookURL: "",
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := New(tt.webhookURL)

			if (client == nil) != tt.wantNil {
				t.Errorf("New() = %v, wantNil %v", client, tt.wantNil)
			}

			if client != nil && client.webhookURL != tt.webhookURL {
				t.Errorf("webhookURL = %v, want %v", client.webhookURL, tt.webhookURL)
			}
		})
	}
}

func TestClient_PostMessage_NilClient(t *testing.T) {
	t.Parallel()

	var client *Client = nil

	err := client.PostMessage(context.Background(), "test message")

	if err != nil {
		t.Errorf("PostMessage() on nil client error = %v, want nil", err)
	}
}

func TestClient_PostMessage_Success(t *testing.T) {
	t.Parallel()

	// Create test server
	var receivedPayload slackPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != http.MethodPost {
			t.Errorf("Request method = %v, want POST", r.Method)
		}

		// Verify Content-Type header
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %v, want application/json", r.Header.Get("Content-Type"))
		}

		// Read and parse body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}

		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Fatalf("Failed to unmarshal payload: %v", err)
		}

		// Send success response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(server.URL)
	testMessage := "Test alert message"

	err := client.PostMessage(context.Background(), testMessage)

	if err != nil {
		t.Errorf("PostMessage() error = %v", err)
	}

	if receivedPayload.Text != testMessage {
		t.Errorf("Received text = %v, want %v", receivedPayload.Text, testMessage)
	}
}

func TestClient_PostMessage_HTTPError(t *testing.T) {
	t.Parallel()

	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := New(server.URL)

	// Should not return error (logs it instead)
	err := client.PostMessage(context.Background(), "test")

	if err != nil {
		t.Errorf("PostMessage() error = %v, want nil (errors are logged, not returned)", err)
	}
}

func TestClient_PostMessage_InvalidURL(t *testing.T) {
	t.Parallel()

	client := New("http://invalid-url-that-does-not-exist:99999")

	err := client.PostMessage(context.Background(), "test")

	if err == nil {
		t.Error("PostMessage() expected error for invalid URL, got nil")
	}
}

func TestClient_PostRCA_NilClient(t *testing.T) {
	t.Parallel()

	var client *Client = nil

	incident := domain.Incident{ID: "inc-123"}
	rca := domain.RCA{Summary: "Test RCA"}

	err := client.PostRCA(context.Background(), incident, rca, 0.8, false)

	if err != nil {
		t.Errorf("PostRCA() on nil client error = %v, want nil", err)
	}
}

func TestClient_PostRCA_Success(t *testing.T) {
	t.Parallel()

	var receivedText string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload slackPayload
		_ = json.Unmarshal(body, &payload)
		receivedText = payload.Text
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(server.URL)

	incident := domain.Incident{
		ID:       "inc-456",
		Cluster:  "prod-us-west",
		Service:  "payment-api",
		Severity: "critical",
	}

	rca := domain.RCA{
		Summary:    "High CPU usage due to memory leak",
		Confidence: 0.85,
	}

	err := client.PostRCA(context.Background(), incident, rca, 0.9, true)

	if err != nil {
		t.Errorf("PostRCA() error = %v", err)
	}

	// Verify message contains expected fields
	expectedSubstrings := []string{
		"inc-456",
		"prod-us-west",
		"payment-api",
		"critical",
		"High CPU usage due to memory leak",
		"0.85",
		"true", // humanNeeded
	}

	for _, substr := range expectedSubstrings {
		if len(receivedText) == 0 || !contains(receivedText, substr) {
			t.Errorf("PostRCA() message missing expected substring %q\nFull message: %s", substr, receivedText)
		}
	}
}

func TestClient_PostRCA_FormattedMessage(t *testing.T) {
	t.Parallel()

	var receivedPayload slackPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(server.URL)

	incident := domain.Incident{
		ID:       "inc-789",
		Cluster:  "staging-eu",
		Service:  "auth-service",
		Severity: "warning",
	}

	rca := domain.RCA{
		Summary:    "Database connection pool exhausted",
		Confidence: 0.92,
	}

	planConfidence := 0.88
	humanNeeded := false

	err := client.PostRCA(context.Background(), incident, rca, planConfidence, humanNeeded)

	if err != nil {
		t.Errorf("PostRCA() error = %v", err)
	}

	// Verify message structure
	if receivedPayload.Text == "" {
		t.Error("PostRCA() message is empty")
	}

	// Verify markdown formatting
	if !contains(receivedPayload.Text, "*Incident*") {
		t.Error("Message missing *Incident* markdown")
	}

	if !contains(receivedPayload.Text, "*Severity*") {
		t.Error("Message missing *Severity* markdown")
	}

	if !contains(receivedPayload.Text, "*RCA") {
		t.Error("Message missing *RCA* markdown")
	}
}

func TestClient_PostMessage_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Create server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't respond immediately
		<-r.Context().Done()
	}))
	defer server.Close()

	client := New(server.URL)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := client.PostMessage(ctx, "test")

	if err == nil {
		t.Error("PostMessage() expected error for cancelled context, got nil")
	}
}

func TestSlackPayload_JSONSerialization(t *testing.T) {
	t.Parallel()

	payload := slackPayload{
		Text: "Test message with special chars: @user #channel",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	var decoded slackPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if decoded.Text != payload.Text {
		t.Errorf("Decoded text = %v, want %v", decoded.Text, payload.Text)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexSubstring(s, substr) >= 0)
}

func indexSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
