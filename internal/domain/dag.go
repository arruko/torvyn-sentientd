package domain

import "time"

type DAGVersion string

const DAGVersionV1 DAGVersion = "v1"

type DAG struct {
	Version     DAGVersion        `json:"version"`
	IncidentID  string            `json:"incidentId"`
	PlanID      string            `json:"planId"`
	Description string            `json:"description"`
	Tasks       []DAGTask         `json:"tasks"`
	Constraints DAGConstraints    `json:"constraints"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type DAGTask struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Tool        string                 `json:"tool"`
	Inputs      map[string]interface{} `json:"inputs"`
	RunAfter    []string               `json:"runAfter,omitempty"`
	TimeoutSec  int                    `json:"timeoutSeconds,omitempty"`
	Retry       *DAGTaskRetry          `json:"retry,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Approval    *DAGTaskApproval       `json:"approval,omitempty"`
}

type DAGTaskRetry struct {
	MaxAttempts    int `json:"maxAttempts"`
	BackoffSeconds int `json:"backoffSeconds,omitempty"`
}

type DAGTaskApproval struct {
	Required     bool   `json:"required"`
	Mode         string `json:"mode,omitempty"` // e.g. "human-or-policy"
	SlackChannel string `json:"slackChannel,omitempty"`
}

type DAGConstraints struct {
	MaxParallel        int            `json:"maxParallel"`
	PerToolConcurrency map[string]int `json:"perToolConcurrency,omitempty"`
	MustNotUseTools    []string       `json:"mustNotUseTools,omitempty"`
}

type PlanMetadata struct {
	CreatedBy    string    `json:"createdBy"`
	CreatedAt    time.Time `json:"createdAt"`
	ChangePolicy string    `json:"changePolicy,omitempty"`
}
