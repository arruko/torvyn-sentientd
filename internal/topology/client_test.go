package topology

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("test-namespace")

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.namespace != "test-namespace" {
		t.Errorf("namespace = %v, want test-namespace", client.namespace)
	}

	if client.graph == nil {
		t.Fatal("graph should be initialized")
	}

	// Verify graph is empty initially
	nodeCount, edgeCount := client.graph.Stats()
	if nodeCount != 0 {
		t.Errorf("Initial node count = %d, want 0", nodeCount)
	}

	if edgeCount != 0 {
		t.Errorf("Initial edge count = %d, want 0", edgeCount)
	}
}

func TestClient_SubgraphForService(t *testing.T) {
	client := NewClient("test-namespace")

	// Manually load a test graph
	sg := &ServiceGraph{
		Spec: ServiceGraphSpec{
			Nodes: []Node{
				{ID: "svc:api", Type: NodeTypeService},
				{ID: "db:postgres", Type: NodeTypeDatabase},
			},
			Edges: []Edge{
				{From: "svc:api", To: "db:postgres", Type: EdgeTypeDependsOn},
			},
		},
	}

	client.graph.LoadFromServiceGraph(sg)

	// Test getting subgraph
	slice, err := client.SubgraphForService("svc:api")
	if err != nil {
		t.Fatalf("SubgraphForService() error = %v", err)
	}

	if slice == nil {
		t.Fatal("SubgraphForService() returned nil")
	}

	if len(slice.Nodes) != 2 {
		t.Errorf("Node count = %d, want 2", len(slice.Nodes))
	}

	if len(slice.Edges) != 1 {
		t.Errorf("Edge count = %d, want 1", len(slice.Edges))
	}
}

func TestClient_GetGraph(t *testing.T) {
	client := NewClient("test-namespace")

	graph := client.GetGraph()

	if graph == nil {
		t.Fatal("GetGraph() returned nil")
	}

	// Should be the same instance
	if graph != client.graph {
		t.Error("GetGraph() returned different instance")
	}
}
