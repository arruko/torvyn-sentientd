package kagent

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
		name    string
		baseURL string
		wantNil bool
	}{
		{
			name:    "valid URL",
			baseURL: "http://localhost:8080",
			wantNil: false,
		},
		{
			name:    "URL with trailing slash",
			baseURL: "http://localhost:8080/",
			wantNil: false,
		},
		{
			name:    "empty URL creates client",
			baseURL: "",
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := New(tt.baseURL)

			if (client == nil) != tt.wantNil {
				t.Errorf("New() = %v, wantNil %v", client, tt.wantNil)
			}

			if client != nil && client.baseURL != tt.baseURL {
				t.Errorf("baseURL = %v, want %v", client.baseURL, tt.baseURL)
			}

			if client != nil && client.client == nil {
				t.Error("client should not be nil")
			}
		})
	}
}

func TestClient_InvestigateAndPlan_Success(t *testing.T) {
	t.Parallel()

	// Expected response
	expectedResult := InvestigationAndPlanResponse{
		IncidentID: "inc-123",
		PlanID:     "plan-456",
		RCA: domain.RCA{
			Summary:       "High CPU usage detected in pods",
			RootCauseType: "resource_exhaustion",
			Confidence:    0.95,
			Evidence: []domain.EvidenceItem{
				{Type: domain.EvidenceTypeMetric, Description: "Pod metrics show CPU at 95%", Source: "prometheus"},
			},
		},
	}
	expectedResult.Plan.Confidence = 0.92
	expectedResult.Plan.HumanApprovalRecommended = false

	// Create test server
	var receivedRequest InvestigationAndPlanRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != http.MethodPost {
			t.Errorf("Request method = %v, want POST", r.Method)
		}

		// Verify path
		if r.URL.Path != "/v1/incidents/plan" {
			t.Errorf("Request path = %v, want /v1/incidents/plan", r.URL.Path)
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

		if err := json.Unmarshal(body, &receivedRequest); err != nil {
			t.Fatalf("Failed to unmarshal request: %v", err)
		}

		// Send success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(expectedResult)
	}))
	defer server.Close()

	client := New(server.URL)

	req := InvestigationAndPlanRequest{
		Incident: domain.Incident{
			ID:       "inc-123",
			Cluster:  "prod-us-west",
			Service:  "payment-api",
			Severity: "critical",
		},
		InvestigationConfig: InvestigationConfig{
			MaxSteps:          10,
			MaxDurationSec:    300,
			MinRCAConfidence:  0.8,
			MinPlanConfidence: 0.8,
		},
	}

	result, err := client.InvestigateAndPlan(context.Background(), req)

	if err != nil {
		t.Errorf("InvestigateAndPlan() error = %v", err)
	}

	// Verify received request
	if receivedRequest.Incident.ID != req.Incident.ID {
		t.Errorf("Received incident ID = %v, want %v", receivedRequest.Incident.ID, req.Incident.ID)
	}

	// Verify result
	if result.RCA.Summary != expectedResult.RCA.Summary {
		t.Errorf("RCA Summary = %v, want %v", result.RCA.Summary, expectedResult.RCA.Summary)
	}

	if result.RCA.Confidence != expectedResult.RCA.Confidence {
		t.Errorf("RCA Confidence = %v, want %v", result.RCA.Confidence, expectedResult.RCA.Confidence)
	}

	if result.PlanConfidence != expectedResult.Plan.Confidence {
		t.Errorf("PlanConfidence = %v, want %v", result.PlanConfidence, expectedResult.Plan.Confidence)
	}

	if result.HumanApprovalNeeded != expectedResult.Plan.HumanApprovalRecommended {
		t.Errorf("HumanApprovalNeeded = %v, want %v", result.HumanApprovalNeeded, expectedResult.Plan.HumanApprovalRecommended)
	}
}

func TestClient_InvestigateAndPlan_HTTPError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		wantErrContain string
	}{
		{
			name:           "500 internal server error",
			statusCode:     http.StatusInternalServerError,
			responseBody:   `{"error": "internal error"}`,
			wantErrContain: "unexpected status",
		},
		{
			name:           "400 bad request",
			statusCode:     http.StatusBadRequest,
			responseBody:   `{"error": "bad request"}`,
			wantErrContain: "unexpected status",
		},
		{
			name:           "404 not found",
			statusCode:     http.StatusNotFound,
			responseBody:   `{"error": "not found"}`,
			wantErrContain: "unexpected status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := New(server.URL)

			req := InvestigationAndPlanRequest{
				Incident: domain.Incident{ID: "inc-123"},
			}

			_, err := client.InvestigateAndPlan(context.Background(), req)

			if err == nil {
				t.Error("InvestigateAndPlan() expected error, got nil")
			}

			if err != nil && !contains(err.Error(), tt.wantErrContain) {
				t.Errorf("Error = %v, want to contain %q", err, tt.wantErrContain)
			}
		})
	}
}

func TestClient_InvestigateAndPlan_InvalidJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	client := New(server.URL)

	req := InvestigationAndPlanRequest{
		Incident: domain.Incident{ID: "inc-123"},
	}

	_, err := client.InvestigateAndPlan(context.Background(), req)

	if err == nil {
		t.Error("InvestigateAndPlan() expected error for invalid JSON, got nil")
	}
}

func TestClient_InvestigateAndPlan_InvalidURL(t *testing.T) {
	t.Parallel()

	client := New("http://invalid-url-that-does-not-exist:99999")

	req := InvestigationAndPlanRequest{
		Incident: domain.Incident{ID: "inc-123"},
	}

	_, err := client.InvestigateAndPlan(context.Background(), req)

	if err == nil {
		t.Error("InvestigateAndPlan() expected error for invalid URL, got nil")
	}
}

func TestClient_InvestigateAndPlan_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Create server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wait for context cancellation
		<-r.Context().Done()
	}))
	defer server.Close()

	client := New(server.URL)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := InvestigationAndPlanRequest{
		Incident: domain.Incident{ID: "inc-123"},
	}

	_, err := client.InvestigateAndPlan(ctx, req)

	if err == nil {
		t.Error("InvestigateAndPlan() expected error for cancelled context, got nil")
	}
}

func TestClient_InvestigateAndPlan_RequestMarshaling(t *testing.T) {
	t.Parallel()

	var receivedRequest InvestigationAndPlanRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedRequest)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(InvestigationAndPlanResponse{})
	}))
	defer server.Close()

	client := New(server.URL)

	req := InvestigationAndPlanRequest{
		Incident: domain.Incident{
			ID:       "inc-456",
			Cluster:  "staging-eu",
			Service:  "auth-service",
			Severity: "warning",
		},
		InvestigationConfig: InvestigationConfig{
			MaxSteps:          5,
			MaxDurationSec:    120,
			MinRCAConfidence:  0.7,
			MinPlanConfidence: 0.7,
		},
	}

	_, err := client.InvestigateAndPlan(context.Background(), req)

	if err != nil {
		t.Errorf("InvestigateAndPlan() error = %v", err)
	}

	// Verify request was properly marshaled
	if receivedRequest.Incident.ID != req.Incident.ID {
		t.Errorf("Received ID = %v, want %v", receivedRequest.Incident.ID, req.Incident.ID)
	}

	if receivedRequest.Incident.Cluster != req.Incident.Cluster {
		t.Errorf("Received Cluster = %v, want %v", receivedRequest.Incident.Cluster, req.Incident.Cluster)
	}

	if receivedRequest.InvestigationConfig.MaxSteps != req.InvestigationConfig.MaxSteps {
		t.Errorf("Received MaxSteps = %v, want %v", receivedRequest.InvestigationConfig.MaxSteps, req.InvestigationConfig.MaxSteps)
	}
}

func TestClient_InvestigateAndPlan_ComplexResult(t *testing.T) {
	t.Parallel()

	expectedResult := InvestigationAndPlanResponse{
		IncidentID: "inc-789",
		PlanID:     "plan-999",
		RCA: domain.RCA{
			Summary:             "Memory leak in application",
			RootCauseType:       "memory_leak",
			Confidence:          0.89,
			ContributingFactors: []string{"Goroutine leak", "Unbounded cache"},
			Evidence: []domain.EvidenceItem{
				{Type: domain.EvidenceTypeMetric, Description: "Memory usage increasing over time", Source: "prometheus"},
				{Type: domain.EvidenceTypeLog, Description: "Goroutine count increasing", Source: "pprof"},
			},
		},
	}
	expectedResult.Plan.Confidence = 0.87
	expectedResult.Plan.HumanApprovalRecommended = true
	expectedResult.Plan.NotesForHuman = []string{"Verify impact before restarting", "Check for data loss"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(expectedResult)
	}))
	defer server.Close()

	client := New(server.URL)

	req := InvestigationAndPlanRequest{
		Incident: domain.Incident{ID: "inc-789"},
	}

	result, err := client.InvestigateAndPlan(context.Background(), req)

	if err != nil {
		t.Errorf("InvestigateAndPlan() error = %v", err)
	}

	// Verify RCA
	if result.RCA.Summary != expectedResult.RCA.Summary {
		t.Errorf("RCA Summary = %v, want %v", result.RCA.Summary, expectedResult.RCA.Summary)
	}

	if result.RCA.RootCauseType != expectedResult.RCA.RootCauseType {
		t.Errorf("RCA RootCauseType = %v, want %v", result.RCA.RootCauseType, expectedResult.RCA.RootCauseType)
	}

	if len(result.RCA.Evidence) != len(expectedResult.RCA.Evidence) {
		t.Errorf("Evidence count = %d, want %d", len(result.RCA.Evidence), len(expectedResult.RCA.Evidence))
	}

	if len(result.RCA.ContributingFactors) != len(expectedResult.RCA.ContributingFactors) {
		t.Errorf("ContributingFactors count = %d, want %d", len(result.RCA.ContributingFactors), len(expectedResult.RCA.ContributingFactors))
	}

	// Verify flags
	if result.HumanApprovalNeeded != expectedResult.Plan.HumanApprovalRecommended {
		t.Errorf("HumanApprovalNeeded = %v, want %v", result.HumanApprovalNeeded, expectedResult.Plan.HumanApprovalRecommended)
	}
}

func TestInvestigationConfig_Structure(t *testing.T) {
	t.Parallel()

	config := InvestigationConfig{
		MaxSteps:          10,
		MaxDurationSec:    300,
		MinRCAConfidence:  0.8,
		MinPlanConfidence: 0.75,
	}

	if config.MaxSteps <= 0 {
		t.Error("MaxSteps should be positive")
	}

	if config.MaxDurationSec <= 0 {
		t.Error("MaxDurationSec should be positive")
	}

	if config.MinRCAConfidence < 0 || config.MinRCAConfidence > 1 {
		t.Error("MinRCAConfidence should be between 0 and 1")
	}

	if config.MinPlanConfidence < 0 || config.MinPlanConfidence > 1 {
		t.Error("MinPlanConfidence should be between 0 and 1")
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
