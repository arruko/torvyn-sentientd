package topology

import (
	"testing"

	"github.com/arruko/torvyn-sentientd/internal/domain"
)

func newTestServiceGraph() *ServiceGraph {
	return &ServiceGraph{
		Spec: ServiceGraphSpec{
			Nodes: []Node{
				{
					ID:   "svc:payment-api",
					Type: NodeTypeService,
					Labels: map[string]string{
						"team":        "payments",
						"environment": "production",
					},
				},
				{
					ID:   "svc:user-service",
					Type: NodeTypeService,
					Labels: map[string]string{
						"team": "platform",
					},
				},
				{
					ID:   "db:postgres",
					Type: NodeTypeDatabase,
				},
				{
					ID:   "queue:nats",
					Type: NodeTypeQueue,
				},
			},
			Edges: []Edge{
				{
					From: "svc:payment-api",
					To:   "db:postgres",
					Type: EdgeTypeDependsOn,
				},
				{
					From: "svc:payment-api",
					To:   "queue:nats",
					Type: EdgeTypePublishesTo,
				},
				{
					From: "svc:user-service",
					To:   "svc:payment-api",
					Type: EdgeTypeCalls,
				},
			},
		},
	}
}

func TestNewInMemoryGraph(t *testing.T) {
	t.Parallel()

	graph := NewInMemoryGraph()

	if graph == nil {
		t.Fatal("NewInMemoryGraph() returned nil")
	}

	nodeCount, edgeCount := graph.Stats()
	if nodeCount != 0 {
		t.Errorf("New graph node count = %d, want 0", nodeCount)
	}

	if edgeCount != 0 {
		t.Errorf("New graph edge count = %d, want 0", edgeCount)
	}
}

func TestInMemoryGraph_LoadFromServiceGraph(t *testing.T) {
	t.Parallel()

	graph := NewInMemoryGraph()
	sg := newTestServiceGraph()

	graph.LoadFromServiceGraph(sg)

	nodeCount, edgeCount := graph.Stats()

	if nodeCount != 4 {
		t.Errorf("Node count = %d, want 4", nodeCount)
	}

	if edgeCount != 3 {
		t.Errorf("Edge count = %d, want 3", edgeCount)
	}
}

func TestInMemoryGraph_GetNode(t *testing.T) {
	t.Parallel()

	graph := NewInMemoryGraph()
	sg := newTestServiceGraph()
	graph.LoadFromServiceGraph(sg)

	tests := []struct {
		name     string
		nodeID   string
		wantType NodeType
		wantOK   bool
	}{
		{
			name:     "existing service node",
			nodeID:   "svc:payment-api",
			wantType: NodeTypeService,
			wantOK:   true,
		},
		{
			name:     "existing database node",
			nodeID:   "db:postgres",
			wantType: NodeTypeDatabase,
			wantOK:   true,
		},
		{
			name:   "non-existent node",
			nodeID: "svc:non-existent",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			node, ok := graph.GetNode(tt.nodeID)

			if ok != tt.wantOK {
				t.Errorf("GetNode() ok = %v, want %v", ok, tt.wantOK)
			}

			if tt.wantOK {
				if node == nil {
					t.Fatal("GetNode() returned nil node")
				}

				if node.Type != tt.wantType {
					t.Errorf("Node type = %v, want %v", node.Type, tt.wantType)
				}

				if node.ID != tt.nodeID {
					t.Errorf("Node ID = %v, want %v", node.ID, tt.nodeID)
				}
			}
		})
	}
}

func TestInMemoryGraph_GetSubgraphForService(t *testing.T) {
	t.Parallel()

	graph := NewInMemoryGraph()
	sg := newTestServiceGraph()
	graph.LoadFromServiceGraph(sg)

	tests := []struct {
		name           string
		serviceID      string
		wantNodes      int
		wantEdges      int
		validateFunc   func(t *testing.T, slice *domain.TopologySlice)
	}{
		{
			name:      "payment-api with dependencies",
			serviceID: "svc:payment-api",
			wantNodes: 4, // payment-api + db + queue + user-service
			wantEdges: 3, // all 3 edges connected to payment-api
			validateFunc: func(t *testing.T, slice *domain.TopologySlice) {
				// Verify service node is included
				found := false
				for _, node := range slice.Nodes {
					if node.ID == "svc:payment-api" {
						found = true
						if node.Type != domain.NodeTypeService {
							t.Errorf("payment-api type = %v, want service", node.Type)
						}
					}
				}
				if !found {
					t.Error("payment-api node not found in slice")
				}

				// Verify dependencies
				nodeIDs := make(map[string]bool)
				for _, node := range slice.Nodes {
					nodeIDs[node.ID] = true
				}

				if !nodeIDs["db:postgres"] {
					t.Error("db:postgres not in subgraph")
				}
				if !nodeIDs["queue:nats"] {
					t.Error("queue:nats not in subgraph")
				}
				if !nodeIDs["svc:user-service"] {
					t.Error("svc:user-service not in subgraph (dependent)")
				}
			},
		},
		{
			name:      "user-service",
			serviceID: "svc:user-service",
			wantNodes: 2, // user-service + payment-api
			wantEdges: 1, // user-service -> payment-api
			validateFunc: func(t *testing.T, slice *domain.TopologySlice) {
				nodeIDs := make(map[string]bool)
				for _, node := range slice.Nodes {
					nodeIDs[node.ID] = true
				}

				if !nodeIDs["svc:user-service"] {
					t.Error("svc:user-service not in subgraph")
				}
				if !nodeIDs["svc:payment-api"] {
					t.Error("svc:payment-api not in subgraph (dependency)")
				}
			},
		},
		{
			name:      "non-existent service returns empty",
			serviceID: "svc:non-existent",
			wantNodes: 0,
			wantEdges: 0,
			validateFunc: func(t *testing.T, slice *domain.TopologySlice) {
				if len(slice.Nodes) != 0 {
					t.Errorf("Non-existent service returned %d nodes, want 0", len(slice.Nodes))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			slice, err := graph.GetSubgraphForService(tt.serviceID)
			if err != nil {
				t.Fatalf("GetSubgraphForService() error = %v", err)
			}

			if slice == nil {
				t.Fatal("GetSubgraphForService() returned nil")
			}

			if len(slice.Nodes) != tt.wantNodes {
				t.Errorf("Node count = %d, want %d", len(slice.Nodes), tt.wantNodes)
			}

			if len(slice.Edges) != tt.wantEdges {
				t.Errorf("Edge count = %d, want %d", len(slice.Edges), tt.wantEdges)
			}

			tt.validateFunc(t, slice)
		})
	}
}

func TestInMemoryGraph_FindServiceByLabels(t *testing.T) {
	t.Parallel()

	graph := NewInMemoryGraph()
	sg := newTestServiceGraph()
	graph.LoadFromServiceGraph(sg)

	tests := []struct {
		name       string
		labels     map[string]string
		wantCount  int
		wantIDs    []string
	}{
		{
			name:      "match single service by team",
			labels:    map[string]string{"team": "payments"},
			wantCount: 1,
			wantIDs:   []string{"svc:payment-api"},
		},
		{
			name:      "match by environment",
			labels:    map[string]string{"environment": "production"},
			wantCount: 1,
			wantIDs:   []string{"svc:payment-api"},
		},
		{
			name:      "match multiple labels",
			labels:    map[string]string{"team": "payments", "environment": "production"},
			wantCount: 1,
			wantIDs:   []string{"svc:payment-api"},
		},
		{
			name:      "no matches",
			labels:    map[string]string{"team": "nonexistent"},
			wantCount: 0,
			wantIDs:   []string{},
		},
		{
			name:      "empty labels match all services",
			labels:    map[string]string{},
			wantCount: 2, // Only services, not databases/queues
			wantIDs:   []string{"svc:payment-api", "svc:user-service"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			matches := graph.FindServiceByLabels(tt.labels)

			if len(matches) != tt.wantCount {
				t.Errorf("Match count = %d, want %d", len(matches), tt.wantCount)
			}

			if tt.wantCount > 0 {
				matchMap := make(map[string]bool)
				for _, id := range matches {
					matchMap[id] = true
				}

				for _, wantID := range tt.wantIDs {
					if !matchMap[wantID] {
						t.Errorf("Expected ID %s not found in matches", wantID)
					}
				}
			}
		})
	}
}

func TestInMemoryGraph_Stats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setup         func(*InMemoryGraph)
		wantNodes     int
		wantEdges     int
	}{
		{
			name:      "empty graph",
			setup:     func(g *InMemoryGraph) {},
			wantNodes: 0,
			wantEdges: 0,
		},
		{
			name: "graph with nodes and edges",
			setup: func(g *InMemoryGraph) {
				sg := newTestServiceGraph()
				g.LoadFromServiceGraph(sg)
			},
			wantNodes: 4,
			wantEdges: 3,
		},
		{
			name: "reload replaces graph",
			setup: func(g *InMemoryGraph) {
				sg1 := newTestServiceGraph()
				g.LoadFromServiceGraph(sg1)

				// Reload with different graph
				sg2 := &ServiceGraph{
					Spec: ServiceGraphSpec{
						Nodes: []Node{
							{ID: "svc:test", Type: NodeTypeService},
						},
						Edges: []Edge{},
					},
				}
				g.LoadFromServiceGraph(sg2)
			},
			wantNodes: 1,
			wantEdges: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			graph := NewInMemoryGraph()
			tt.setup(graph)

			nodeCount, edgeCount := graph.Stats()

			if nodeCount != tt.wantNodes {
				t.Errorf("Node count = %d, want %d", nodeCount, tt.wantNodes)
			}

			if edgeCount != tt.wantEdges {
				t.Errorf("Edge count = %d, want %d", edgeCount, tt.wantEdges)
			}
		})
	}
}

func TestInMemoryGraph_ComplexTopology(t *testing.T) {
	t.Parallel()

	graph := NewInMemoryGraph()

	// Create a more complex service graph
	sg := &ServiceGraph{
		Spec: ServiceGraphSpec{
			Nodes: []Node{
				{ID: "svc:api-gateway", Type: NodeTypeService},
				{ID: "svc:auth-service", Type: NodeTypeService},
				{ID: "svc:payment-service", Type: NodeTypeService},
				{ID: "svc:order-service", Type: NodeTypeService},
				{ID: "db:postgres-auth", Type: NodeTypeDatabase},
				{ID: "db:postgres-orders", Type: NodeTypeDatabase},
				{ID: "cache:redis", Type: NodeTypeCache},
				{ID: "queue:kafka", Type: NodeTypeQueue},
			},
			Edges: []Edge{
				// API Gateway calls all services
				{From: "svc:api-gateway", To: "svc:auth-service", Type: EdgeTypeCalls},
				{From: "svc:api-gateway", To: "svc:payment-service", Type: EdgeTypeCalls},
				{From: "svc:api-gateway", To: "svc:order-service", Type: EdgeTypeCalls},
				// Services depend on databases
				{From: "svc:auth-service", To: "db:postgres-auth", Type: EdgeTypeDependsOn},
				{From: "svc:order-service", To: "db:postgres-orders", Type: EdgeTypeDependsOn},
				// All services use cache
				{From: "svc:auth-service", To: "cache:redis", Type: EdgeTypeDependsOn},
				{From: "svc:payment-service", To: "cache:redis", Type: EdgeTypeDependsOn},
				{From: "svc:order-service", To: "cache:redis", Type: EdgeTypeDependsOn},
				// Order service publishes to queue
				{From: "svc:order-service", To: "queue:kafka", Type: EdgeTypePublishesTo},
				{From: "svc:payment-service", To: "queue:kafka", Type: EdgeTypeSubscribesTo},
			},
		},
	}

	graph.LoadFromServiceGraph(sg)

	// Test API Gateway neighbors
	slice, err := graph.GetSubgraphForService("svc:api-gateway")
	if err != nil {
		t.Fatalf("GetSubgraphForService() error = %v", err)
	}

	// API Gateway has 3 outgoing edges, so it should have itself + 3 neighbors = 4 nodes
	if len(slice.Nodes) != 4 {
		t.Errorf("API Gateway subgraph nodes = %d, want 4", len(slice.Nodes))
	}

	// Test order service (has multiple edges, including bidirectional with payment via queue)
	slice, err = graph.GetSubgraphForService("svc:order-service")
	if err != nil {
		t.Fatalf("GetSubgraphForService() error = %v", err)
	}

	// order-service connects to: api-gateway (incoming), db, cache, queue
	// It should also include payment-service (which subscribes to the same queue)
	if len(slice.Nodes) < 4 {
		t.Errorf("Order service subgraph nodes = %d, want at least 4", len(slice.Nodes))
	}
}
