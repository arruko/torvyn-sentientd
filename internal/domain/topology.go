package domain

type TopologyNodeType string

const (
	NodeTypeService  TopologyNodeType = "service"
	NodeTypeDatabase TopologyNodeType = "database"
	NodeTypeQueue    TopologyNodeType = "queue"
	NodeTypeCluster  TopologyNodeType = "cluster"
)

type TopologyNode struct {
	ID   string           `json:"id"`   // e.g. "svc:payment-api"
	Type TopologyNodeType `json:"type"` // service, db, queue, etc.
}

type TopologyEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"` // depends_on, calls, hosts, etc.
}

type TopologySlice struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}
