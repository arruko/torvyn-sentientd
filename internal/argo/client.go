package argo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/arruko/torvyn-sentientd/internal/domain"
	"github.com/arruko/torvyn-sentientd/internal/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// WorkflowGVR defines the GroupVersionResource for Argo Workflows
var WorkflowGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "workflows",
}

// Client manages Argo Workflow creation and monitoring
type Client struct {
	Namespace      string
	dynamicClient  dynamic.Interface
	workflowClient dynamic.ResourceInterface
}

// New creates a new Argo client using in-cluster or kubeconfig
func New(namespace string) *Client {
	return &Client{
		Namespace: namespace,
	}
}

// NewWithKubeconfig creates a client with explicit kubeconfig (for testing)
func NewWithKubeconfig(namespace, kubeconfigPath string) (*Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &Client{
		Namespace:      namespace,
		dynamicClient:  dynClient,
		workflowClient: dynClient.Resource(WorkflowGVR).Namespace(namespace),
	}, nil
}

// Initialize sets up the Kubernetes client (call this in main.go after RBAC is ready)
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
	c.workflowClient = dynClient.Resource(WorkflowGVR).Namespace(c.Namespace)

	logging.Info.Printf("Argo Workflows client initialized for namespace: %s", c.Namespace)
	return nil
}

// CreateWorkflowForDAG creates an Argo Workflow from a DAG
func (c *Client) CreateWorkflowForDAG(ctx context.Context, dag domain.DAG) error {
	if c.workflowClient == nil {
		// If not initialized (local dev), just log
		dagJSON, _ := json.Marshal(dag)
		logging.Info.Printf("Would create Argo Workflow in ns %s with DAG: %s", c.Namespace, string(dagJSON))
		return nil
	}

	// Generate workflow name
	workflowName := fmt.Sprintf("remediation-%s-%d", dag.IncidentID, time.Now().Unix())

	// Convert DAG to Argo Workflow spec
	workflow, err := c.dagToWorkflow(workflowName, dag)
	if err != nil {
		return fmt.Errorf("failed to convert DAG to workflow: %w", err)
	}

	// Create workflow
	created, err := c.workflowClient.Create(ctx, workflow, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create workflow: %w", err)
	}

	workflowUID := created.GetUID()
	logging.Info.Printf("Created Argo Workflow: %s (UID: %s) for incident: %s", workflowName, workflowUID, dag.IncidentID)

	return nil
}

// dagToWorkflow converts a domain.DAG into an Argo Workflow unstructured object
func (c *Client) dagToWorkflow(name string, dag domain.DAG) (*unstructured.Unstructured, error) {
	// Build Argo DAG tasks from domain.DAG
	var argoTasks []map[string]any

	for _, task := range dag.Tasks {
		// Marshal inputs to JSON string
		inputsJSON, _ := json.Marshal(task.Inputs)

		argoTask := map[string]any{
			"name":     task.ID, // Use task ID as unique name
			"template": "dag-task-runner",
			"arguments": map[string]any{
				"parameters": []map[string]any{
					{
						"name":  "task_id",
						"value": task.ID,
					},
					{
						"name":  "task_name",
						"value": task.Name,
					},
					{
						"name":  "task_tool",
						"value": task.Tool,
					},
					{
						"name":  "task_inputs",
						"value": string(inputsJSON),
					},
				},
			},
		}

		// Add dependencies (runAfter)
		if len(task.RunAfter) > 0 {
			argoTask["dependencies"] = task.RunAfter
		}

		argoTasks = append(argoTasks, argoTask)
	}

	// Build Workflow manifest
	workflow := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Workflow",
			"metadata": map[string]any{
				"name":      name,
				"namespace": c.Namespace,
				"labels": map[string]any{
					"app.kubernetes.io/name":      "sentientd",
					"app.kubernetes.io/component": "remediation",
					"torvyn.io/incident-id":       dag.IncidentID,
				},
				"annotations": map[string]any{
					"torvyn.io/incident-id":  dag.IncidentID,
					"torvyn.io/generated-by": "sentientd",
				},
			},
			"spec": map[string]any{
				"serviceAccountName": "argo-workflow", // TODO: Make configurable
				"entrypoint":         "remediation-dag",
				"templates": []map[string]any{
					{
						"name": "remediation-dag",
						"dag": map[string]any{
							"tasks": argoTasks,
						},
					},
					{
						"name": "dag-task-runner",
						"inputs": map[string]any{
							"parameters": []map[string]any{
								{"name": "task_id"},
								{"name": "task_name"},
								{"name": "task_tool"},
								{"name": "task_inputs"},
							},
						},
						"container": map[string]any{
							"image":   "ghcr.io/arruko/torvyn-dag-runner:latest",
							"command": []string{"/app/dag-runner"},
							"args": []string{
								"--task-id={{inputs.parameters.task_id}}",
								"--task-name={{inputs.parameters.task_name}}",
								"--tool={{inputs.parameters.task_tool}}",
								"--inputs={{inputs.parameters.task_inputs}}",
							},
							"env": []map[string]any{
								{
									"name":  "INCIDENT_ID",
									"value": dag.IncidentID,
								},
								{
									"name":  "PLAN_ID",
									"value": dag.PlanID,
								},
								{
									"name":  "ALLOW_SCRIPT",
									"value": "false", // Script execution disabled by default
								},
							},
							"securityContext": map[string]any{
								"runAsNonRoot": true,
								"runAsUser":    int64(1000),
								"fsGroup":      int64(1000),
								"runAsGroup":   int64(1000),
								"seccompProfile": map[string]any{
									"type": "RuntimeDefault",
								},
								"readOnlyRootFilesystem":   true,
								"allowPrivilegeEscalation": false,
								"capabilities": map[string]any{
									"drop": []string{"ALL"},
								},
							},
							"resources": map[string]any{
								"requests": map[string]any{
									"cpu":    "100m",
									"memory": "128Mi",
								},
								"limits": map[string]any{
									"cpu":    "500m",
									"memory": "512Mi",
								},
							},
						},
					},
				},
			},
		},
	}

	return workflow, nil
}

// GetWorkflowStatus retrieves the status of a workflow
func (c *Client) GetWorkflowStatus(ctx context.Context, workflowName string) (string, error) {
	if c.workflowClient == nil {
		return "unknown", fmt.Errorf("workflow client not initialized")
	}

	workflow, err := c.workflowClient.Get(ctx, workflowName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get workflow: %w", err)
	}

	// Extract status.phase
	status, found, err := unstructured.NestedString(workflow.Object, "status", "phase")
	if err != nil || !found {
		return "unknown", nil
	}

	return status, nil
}

func jsonClone[T any](in T) T {
	b, _ := json.Marshal(in)
	var out T
	_ = json.Unmarshal(b, &out)
	return out
}
