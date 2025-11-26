package topology

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceGraph represents the topology of services and their dependencies
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Nodes",type=integer,JSONPath=`.status.nodeCount`
// +kubebuilder:printcolumn:name="Edges",type=integer,JSONPath=`.status.edgeCount`
// +kubebuilder:printcolumn:name="Last Updated",type=string,JSONPath=`.status.lastUpdated`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type ServiceGraph struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceGraphSpec   `json:"spec,omitempty"`
	Status ServiceGraphStatus `json:"status,omitempty"`
}

// ServiceGraphSpec defines the desired state of ServiceGraph
type ServiceGraphSpec struct {
	// Nodes represent services, databases, queues, and other components
	Nodes []Node `json:"nodes,omitempty"`

	// Edges represent dependencies between nodes
	Edges []Edge `json:"edges,omitempty"`
}

// Node represents a component in the service graph
type Node struct {
	// ID is the unique identifier for the node (e.g., "svc:payment-api")
	ID string `json:"id"`

	// Type of node (service, database, queue, cache, external)
	Type NodeType `json:"type"`

	// Labels are key-value pairs for the node
	Labels map[string]string `json:"labels,omitempty"`

	// Metadata contains additional information
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NodeType represents the type of a node
// +kubebuilder:validation:Enum=service;database;queue;cache;external
type NodeType string

const (
	NodeTypeService  NodeType = "service"
	NodeTypeDatabase NodeType = "database"
	NodeTypeQueue    NodeType = "queue"
	NodeTypeCache    NodeType = "cache"
	NodeTypeExternal NodeType = "external"
)

// Edge represents a dependency between two nodes
type Edge struct {
	// From is the source node ID
	From string `json:"from"`

	// To is the target node ID
	To string `json:"to"`

	// Type of dependency
	Type EdgeType `json:"type"`

	// Weight is an optional weight for the edge (e.g., request rate)
	Weight *int `json:"weight,omitempty"`

	// Labels are key-value pairs for the edge
	Labels map[string]string `json:"labels,omitempty"`
}

// EdgeType represents the type of an edge
// +kubebuilder:validation:Enum=depends_on;calls;publishes_to;subscribes_to
type EdgeType string

const (
	EdgeTypeDependsOn    EdgeType = "depends_on"
	EdgeTypeCalls        EdgeType = "calls"
	EdgeTypePublishesTo  EdgeType = "publishes_to"
	EdgeTypeSubscribesTo EdgeType = "subscribes_to"
)

// ServiceGraphStatus defines the observed state of ServiceGraph
type ServiceGraphStatus struct {
	// LastUpdated is the timestamp of last update
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`

	// NodeCount is the total number of nodes
	NodeCount int `json:"nodeCount,omitempty"`

	// EdgeCount is the total number of edges
	EdgeCount int `json:"edgeCount,omitempty"`

	// Conditions represent the latest available observations of the ServiceGraph's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ServiceGraphList contains a list of ServiceGraph
// +kubebuilder:object:root=true
type ServiceGraphList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceGraph `json:"items"`
}
