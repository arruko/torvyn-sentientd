package topology

import (
	"fmt"
	"sync"

	"github.com/arruko/torvyn-sentientd/internal/domain"
)

// InMemoryGraph is an in-memory representation of the service topology
type InMemoryGraph struct {
	mu    sync.RWMutex
	nodes map[string]*Node  // node ID -> Node
	edges map[string][]*Edge // from node ID -> edges
	rev   map[string][]*Edge // to node ID -> edges (reverse lookup)
}

// NewInMemoryGraph creates a new empty graph
func NewInMemoryGraph() *InMemoryGraph {
	return &InMemoryGraph{
		nodes: make(map[string]*Node),
		edges: make(map[string][]*Edge),
		rev:   make(map[string][]*Edge),
	}
}

// LoadFromServiceGraph loads a ServiceGraph CRD into the in-memory graph
func (g *InMemoryGraph) LoadFromServiceGraph(sg *ServiceGraph) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Clear existing graph
	g.nodes = make(map[string]*Node)
	g.edges = make(map[string][]*Edge)
	g.rev = make(map[string][]*Edge)

	// Load nodes
	for i := range sg.Spec.Nodes {
		node := &sg.Spec.Nodes[i]
		g.nodes[node.ID] = node
	}

	// Load edges
	for i := range sg.Spec.Edges {
		edge := &sg.Spec.Edges[i]
		g.edges[edge.From] = append(g.edges[edge.From], edge)
		g.rev[edge.To] = append(g.rev[edge.To], edge)
	}
}

// GetNode retrieves a node by ID
func (g *InMemoryGraph) GetNode(id string) (*Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, ok := g.nodes[id]
	return node, ok
}

// GetSubgraphForService returns a topology slice centered on a service
// This includes the service itself + all 1-hop neighbors (dependencies + dependents)
func (g *InMemoryGraph) GetSubgraphForService(serviceID string) (*domain.TopologySlice, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Check if service exists
	service, ok := g.nodes[serviceID]
	if !ok {
		// Return empty slice instead of error for non-existent services
		return &domain.TopologySlice{
			Nodes: []domain.TopologyNode{},
			Edges: []domain.TopologyEdge{},
		}, nil
	}

	// Build topology slice
	slice := &domain.TopologySlice{
		Nodes: []domain.TopologyNode{},
		Edges: []domain.TopologyEdge{},
	}

	// Add the service itself as a node
	slice.Nodes = append(slice.Nodes, domain.TopologyNode{
		ID:   service.ID,
		Type: domain.TopologyNodeType(service.Type),
	})

	// Get all neighbors (1-hop)
	neighbors := g.getNeighborsUnlocked(serviceID)

	// Add neighbor nodes
	for _, neighborID := range neighbors {
		node, ok := g.nodes[neighborID]
		if !ok {
			continue
		}

		slice.Nodes = append(slice.Nodes, domain.TopologyNode{
			ID:   node.ID,
			Type: domain.TopologyNodeType(node.Type),
		})
	}

	// Add edges (both outgoing and incoming)
	seen := make(map[string]bool) // Track edges we've added

	// Outgoing edges
	for _, edge := range g.edges[serviceID] {
		edgeKey := fmt.Sprintf("%s->%s", edge.From, edge.To)
		if !seen[edgeKey] {
			slice.Edges = append(slice.Edges, domain.TopologyEdge{
				From: edge.From,
				To:   edge.To,
				Type: string(edge.Type),
			})
			seen[edgeKey] = true
		}
	}

	// Incoming edges
	for _, edge := range g.rev[serviceID] {
		edgeKey := fmt.Sprintf("%s->%s", edge.From, edge.To)
		if !seen[edgeKey] {
			slice.Edges = append(slice.Edges, domain.TopologyEdge{
				From: edge.From,
				To:   edge.To,
				Type: string(edge.Type),
			})
			seen[edgeKey] = true
		}
	}

	return slice, nil
}

// getNeighborsUnlocked returns all node IDs connected to the given node (internal, unlocked)
func (g *InMemoryGraph) getNeighborsUnlocked(nodeID string) []string {
	seen := make(map[string]bool)

	// Outgoing edges (dependencies)
	for _, edge := range g.edges[nodeID] {
		if !seen[edge.To] {
			seen[edge.To] = true
		}
	}

	// Incoming edges (dependents)
	for _, edge := range g.rev[nodeID] {
		if !seen[edge.From] {
			seen[edge.From] = true
		}
	}

	neighbors := make([]string, 0, len(seen))
	for id := range seen {
		neighbors = append(neighbors, id)
	}

	return neighbors
}

// FindServiceByLabels finds services matching the given labels
func (g *InMemoryGraph) FindServiceByLabels(labels map[string]string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var matches []string

	for id, node := range g.nodes {
		if node.Type != NodeTypeService {
			continue
		}

		// Check if all provided labels match
		allMatch := true
		for k, v := range labels {
			if node.Labels[k] != v {
				allMatch = false
				break
			}
		}

		if allMatch {
			matches = append(matches, id)
		}
	}

	return matches
}

// Stats returns statistics about the graph
func (g *InMemoryGraph) Stats() (nodeCount, edgeCount int) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodeCount = len(g.nodes)
	edgeCount = 0
	for _, edges := range g.edges {
		edgeCount += len(edges)
	}

	return nodeCount, edgeCount
}
