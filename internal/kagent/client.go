package kagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/arruko/torvyn-sentientd/internal/domain"
)

type Client struct {
	baseURL string
	client  *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

type InvestigationAndPlanRequest struct {
	Incident            domain.Incident       `json:"incident"`
	Topology            *domain.TopologySlice `json:"topology,omitempty"`
	Policies            map[string]any        `json:"policies,omitempty"`
	InvestigationConfig InvestigationConfig   `json:"investigationConfig"`
}

type InvestigationConfig struct {
	MaxSteps          int     `json:"maxSteps"`
	MaxDurationSec    int     `json:"maxDurationSeconds"`
	MinRCAConfidence  float64 `json:"minRcaConfidence"`
	MinPlanConfidence float64 `json:"minPlanConfidence"`
}

// Response matches what we described before.
type InvestigationAndPlanResponse struct {
	IncidentID string     `json:"incidentId"`
	PlanID     string     `json:"planId"`
	RCA        domain.RCA `json:"rca"`
	Plan       struct {
		DAG                      domain.DAG `json:"dag"`
		Confidence               float64    `json:"confidence"`
		HumanApprovalRecommended bool       `json:"humanApprovalRecommended"`
		NotesForHuman            []string   `json:"notesForHuman,omitempty"`
	} `json:"plan"`
}

func (c *Client) InvestigateAndPlan(ctx context.Context, req InvestigationAndPlanRequest) (*domain.InvestigationResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/incidents/plan", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var kpResp InvestigationAndPlanResponse
	if err := json.NewDecoder(resp.Body).Decode(&kpResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	result := &domain.InvestigationResult{
		IncidentID:          kpResp.IncidentID,
		PlanID:              kpResp.PlanID,
		RCA:                 kpResp.RCA,
		DAG:                 kpResp.Plan.DAG,
		PlanConfidence:      kpResp.Plan.Confidence,
		HumanApprovalNeeded: kpResp.Plan.HumanApprovalRecommended,
	}

	return result, nil
}
