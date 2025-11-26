package topology

import "github.com/arruko/torvyn-sentientd/internal/domain"

// Graph is an abstraction over your service graph / CMDB / ServiceGraph CRD.
type Graph interface {
	SubgraphForService(service string) (*domain.TopologySlice, error)
}
