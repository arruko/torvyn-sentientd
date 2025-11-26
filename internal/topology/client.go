package topology

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/arruko/torvyn-sentientd/internal/domain"
	"github.com/arruko/torvyn-sentientd/internal/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// ServiceGraphGVR defines the GroupVersionResource for ServiceGraph CRD
var ServiceGraphGVR = schema.GroupVersionResource{
	Group:    "torvyn.io",
	Version:  "v1alpha1",
	Resource: "servicegraphs",
}

// Client loads and watches ServiceGraph CRDs
type Client struct {
	namespace     string
	dynamicClient dynamic.Interface
	sgClient      dynamic.ResourceInterface
	graph         *InMemoryGraph
	mu            sync.RWMutex
	informer      cache.SharedIndexInformer
	stopCh        chan struct{}
}

// NewClient creates a new ServiceGraph client
func NewClient(namespace string) *Client {
	return &Client{
		namespace: namespace,
		graph:     NewInMemoryGraph(),
		stopCh:    make(chan struct{}),
	}
}

// Initialize sets up the Kubernetes client and starts watching
func (c *Client) Initialize() error {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	c.dynamicClient = dynClient
	c.sgClient = dynClient.Resource(ServiceGraphGVR).Namespace(c.namespace)

	logging.Info.Printf("ServiceGraph client initialized for namespace: %s", c.namespace)

	// Load initial ServiceGraph
	if err := c.loadServiceGraph(context.Background()); err != nil {
		logging.Error.Printf("Failed to load initial ServiceGraph: %v", err)
		// Don't fail initialization, just log the error
	}

	// Start watcher
	go c.startWatch()

	return nil
}

// loadServiceGraph fetches the ServiceGraph CRD and loads it into memory
func (c *Client) loadServiceGraph(ctx context.Context) error {
	// Try to get "default" ServiceGraph
	obj, err := c.sgClient.Get(ctx, "default", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ServiceGraph: %w", err)
	}

	return c.updateGraph(obj)
}

// updateGraph updates the in-memory graph from an unstructured object
func (c *Client) updateGraph(obj *unstructured.Unstructured) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Convert unstructured to ServiceGraph
	var sg ServiceGraph
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &sg); err != nil {
		return fmt.Errorf("failed to convert unstructured to ServiceGraph: %w", err)
	}

	// Load into graph
	c.graph.LoadFromServiceGraph(&sg)

	nodeCount, edgeCount := c.graph.Stats()
	logging.Info.Printf("ServiceGraph loaded: %d nodes, %d edges", nodeCount, edgeCount)

	return nil
}

// startWatch starts watching for ServiceGraph changes
func (c *Client) startWatch() {
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return c.sgClient.List(context.Background(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return c.sgClient.Watch(context.Background(), options)
		},
	}

	informer := cache.NewSharedIndexInformer(
		lw,
		&unstructured.Unstructured{},
		0, // No resync
		cache.Indexers{},
	)

	_, _ = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if u, ok := obj.(*unstructured.Unstructured); ok {
				logging.Info.Printf("ServiceGraph added: %s", u.GetName())
				if err := c.updateGraph(u); err != nil {
					logging.Error.Printf("Failed to update graph on add: %v", err)
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if u, ok := newObj.(*unstructured.Unstructured); ok {
				logging.Info.Printf("ServiceGraph updated: %s", u.GetName())
				if err := c.updateGraph(u); err != nil {
					logging.Error.Printf("Failed to update graph on update: %v", err)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			if u, ok := obj.(*unstructured.Unstructured); ok {
				logging.Info.Printf("ServiceGraph deleted: %s", u.GetName())
				// Clear the graph
				c.mu.Lock()
				c.graph = NewInMemoryGraph()
				c.mu.Unlock()
			}
		},
	})

	c.informer = informer

	logging.Info.Println("Starting ServiceGraph watcher...")
	informer.Run(c.stopCh)
}

// Stop stops the watcher
func (c *Client) Stop() {
	close(c.stopCh)
	logging.Info.Println("ServiceGraph watcher stopped")
}

// SubgraphForService implements the Graph interface
func (c *Client) SubgraphForService(service string) (*domain.TopologySlice, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Build service ID (e.g., "svc:payment-api")
	serviceID := service
	if !strings.HasPrefix(service, "svc:") {
		serviceID = fmt.Sprintf("svc:%s", service)
	}

	return c.graph.GetSubgraphForService(serviceID)
}

// GetGraph returns the underlying graph (for testing)
func (c *Client) GetGraph() *InMemoryGraph {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.graph
}
