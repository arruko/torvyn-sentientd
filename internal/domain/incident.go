package domain

import "time"

type IncidentState string

const (
	IncidentStateOpen          IncidentState = "open"
	IncidentStateInvestigating IncidentState = "investigating"
	IncidentStateMitigating    IncidentState = "mitigating"
	IncidentStateMonitoring    IncidentState = "monitoring"
	IncidentStateResolved      IncidentState = "resolved"
)

type Incident struct {
	ID                 string         `json:"id"`
	Service            string         `json:"service"`
	Cluster            string         `json:"cluster"`
	Environment        string         `json:"environment"`
	Severity           string         `json:"severity"`
	Alerts             []Alert        `json:"alerts"`
	CorrelationKey     string         `json:"correlationKey"` // cluster+ns+service, etc.
	CorrelationVersion int            `json:"correlationVersion"`
	State              IncidentState  `json:"state"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
	FirstAlertAt       time.Time      `json:"firstAlertAt"`
	LastAlertAt        time.Time      `json:"lastAlertAt"`
	TopologyContext    *TopologySlice `json:"topologyContext,omitempty"`
}
