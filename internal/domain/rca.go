package domain

type EvidenceType string

const (
	EvidenceTypeMetric EvidenceType = "metric"
	EvidenceTypeLog    EvidenceType = "log"
	EvidenceTypeEvent  EvidenceType = "event"
)

type EvidenceItem struct {
	Type        EvidenceType `json:"type"`
	Source      string       `json:"source"`
	Description string       `json:"description"`
	Link        string       `json:"link,omitempty"`
}

type RCA struct {
	Summary             string         `json:"summary"`
	RootCauseType       string         `json:"rootCauseType"`
	Confidence          float64        `json:"confidence"`
	ContributingFactors []string       `json:"contributingFactors,omitempty"`
	Evidence            []EvidenceItem `json:"evidence,omitempty"`
}

type InvestigationResult struct {
	IncidentID          string  `json:"incidentId"`
	PlanID              string  `json:"planId"`
	RCA                 RCA     `json:"rca"`
	DAG                 DAG     `json:"dag"`
	PlanConfidence      float64 `json:"planConfidence"`
	HumanApprovalNeeded bool    `json:"humanApprovalNeeded"`
}
