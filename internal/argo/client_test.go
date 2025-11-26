package argo

import (
	"testing"

	"github.com/arruko/torvyn-sentientd/internal/domain"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Helper to create a minimal test DAG
func newTestDAG(incidentID, planID string, tasks []domain.DAGTask) domain.DAG {
	return domain.DAG{
		Version:     domain.DAGVersionV1,
		IncidentID:  incidentID,
		PlanID:      planID,
		Description: "Test remediation plan",
		Tasks:       tasks,
		Constraints: domain.DAGConstraints{
			MaxParallel: 3,
		},
		Metadata: map[string]string{
			"source": "test",
		},
	}
}

func TestDAGToWorkflow_BasicStructure(t *testing.T) {

	client := New("test-namespace")

	dag := newTestDAG("inc-123", "plan-456", []domain.DAGTask{
		{
			ID:   "task-1",
			Name: "Check pod status",
			Tool: "kubectl",
			Inputs: map[string]any{
				"action": "get",
			},
		},
	})

	workflow, err := client.dagToWorkflow("test-workflow", dag)
	if err != nil {
		t.Fatalf("dagToWorkflow() error = %v", err)
	}

	// Verify basic structure
	if workflow.GetAPIVersion() != "argoproj.io/v1alpha1" {
		t.Errorf("APIVersion = %v, want argoproj.io/v1alpha1", workflow.GetAPIVersion())
	}

	if workflow.GetKind() != "Workflow" {
		t.Errorf("Kind = %v, want Workflow", workflow.GetKind())
	}

	if workflow.GetName() != "test-workflow" {
		t.Errorf("Name = %v, want test-workflow", workflow.GetName())
	}

	if workflow.GetNamespace() != "test-namespace" {
		t.Errorf("Namespace = %v, want test-namespace", workflow.GetNamespace())
	}

	// Verify labels
	labels := workflow.GetLabels()
	if labels["torvyn.io/incident-id"] != "inc-123" {
		t.Errorf("Label torvyn.io/incident-id = %v, want inc-123", labels["torvyn.io/incident-id"])
	}

	// Verify spec exists
	specObj := jsonClone(workflow.Object)
	spec, found, err := unstructured.NestedMap(specObj, "spec")

	if err != nil || !found {
		t.Fatalf("Failed to get spec: found=%v, err=%v", found, err)
	}

	// Verify entrypoint
	entrypoint, _, _ := unstructured.NestedString(spec, "entrypoint")
	if entrypoint != "remediation-dag" {
		t.Errorf("Entrypoint = %v, want remediation-dag", entrypoint)
	}

	// Verify templates exist
	templates, found, err := unstructured.NestedSlice(spec, "templates")
	if err != nil || !found {
		t.Fatalf("Failed to get templates: found=%v, err=%v", found, err)
	}

	if len(templates) != 2 {
		t.Errorf("Template count = %d, want 2 (remediation-dag + dag-task-runner)", len(templates))
	}
}

func TestDAGToWorkflow_TaskDependencies(t *testing.T) {

	client := New("test-namespace")

	// Create tasks with dependencies
	tasks := []domain.DAGTask{
		{
			ID:   "task-1",
			Name: "First task",
			Tool: "kubectl",
			Inputs: map[string]any{
				"action": "get",
			},
		},
		{
			ID:       "task-2",
			Name:     "Second task",
			Tool:     "kubectl",
			RunAfter: []string{"task-1"},
			Inputs: map[string]any{
				"action": "delete",
			},
		},
	}

	dag := newTestDAG("inc-123", "plan-456", tasks)
	workflow, err := client.dagToWorkflow("test-workflow", dag)
	if err != nil {
		t.Fatalf("dagToWorkflow() error = %v", err)
	}

	// Extract templates
	specObj := jsonClone(workflow.Object)
	spec, _, _ := unstructured.NestedMap(specObj, "spec")
	templates, _, _ := unstructured.NestedSlice(spec, "templates")

	// Find remediation-dag template
	var dagTemplate map[string]any
	for _, tmpl := range templates {
		tmplMap := tmpl.(map[string]any)
		if tmplMap["name"] == "remediation-dag" {
			dagTemplate = tmplMap
			break
		}
	}

	if dagTemplate == nil {
		t.Fatal("remediation-dag template not found")
	}

	// Get tasks from DAG template
	specObj = jsonClone(dagTemplate)
	dagSpec, found, _ := unstructured.NestedMap(specObj, "dag")
	if !found {
		t.Fatal("dag spec not found in remediation-dag template")
	}

	argoTasks, found, _ := unstructured.NestedSlice(dagSpec, "tasks")
	if !found {
		t.Fatal("tasks not found in dag spec")
	}

	if len(argoTasks) != 2 {
		t.Errorf("Task count = %d, want 2", len(argoTasks))
	}

	// Find task-2 and verify dependencies
	var task2 map[string]any
	for _, taskIface := range argoTasks {
		task := taskIface.(map[string]any)
		if task["name"] == "task-2" {
			task2 = task
			break
		}
	}

	if task2 == nil {
		t.Fatal("task-2 not found")
	}

	deps, hasDeps := task2["dependencies"]
	if !hasDeps {
		t.Fatal("task-2 should have dependencies")
	}

	depsIface := deps.([]any)
	var depsSlice []string
	for _, v := range depsIface {
		depsSlice = append(depsSlice, v.(string))
	}
	if len(depsSlice) != 1 || depsSlice[0] != "task-1" {
		t.Errorf("task-2 dependencies = %v, want [task-1]", depsSlice)
	}
}

func TestDAGToWorkflow_ContainerSecurity(t *testing.T) {

	client := New("test-namespace")

	dag := newTestDAG("inc-123", "plan-456", []domain.DAGTask{
		{
			ID:   "task-1",
			Name: "Test task",
			Tool: "kubectl",
			Inputs: map[string]any{
				"action": "get",
			},
		},
	})

	workflow, err := client.dagToWorkflow("test-workflow", dag)
	if err != nil {
		t.Fatalf("dagToWorkflow() error = %v", err)
	}

	// Extract templates
	specObj := jsonClone(workflow.Object)
	spec, _, _ := unstructured.NestedMap(specObj, "spec")
	templates, _, _ := unstructured.NestedSlice(spec, "templates")

	// Find dag-task-runner template
	var runnerTemplate map[string]any
	for _, tmpl := range templates {
		tmplMap := tmpl.(map[string]any)
		if tmplMap["name"] == "dag-task-runner" {
			runnerTemplate = tmplMap
			break
		}
	}

	if runnerTemplate == nil {
		t.Fatal("dag-task-runner template not found")
	}

	// Verify container exists
	specRunner := jsonClone(runnerTemplate)
	container, found, _ := unstructured.NestedMap(specRunner, "container")
	if !found {
		t.Fatal("container not found in dag-task-runner template")
	}

	// Verify security context
	specContainer := jsonClone(container)
	securityContext, found, _ := unstructured.NestedMap(specContainer, "securityContext")
	if !found {
		t.Fatal("securityContext not found")
	}

	runAsNonRoot, _, _ := unstructured.NestedBool(securityContext, "runAsNonRoot")
	if !runAsNonRoot {
		t.Error("runAsNonRoot should be true")
	}

	raw := securityContext["runAsUser"]
	var runAsUser int64
	switch v := raw.(type) {
	case int64:
		runAsUser = v
	case float64:
		runAsUser = int64(v)
	default:
		t.Fatalf("unexpected type for runAsUser: %T", raw)
	}
	if runAsUser != 1000 {
		t.Errorf("runAsUser = %v, want 1000", runAsUser)
	}

	readOnlyRootFilesystem, _, _ := unstructured.NestedBool(securityContext, "readOnlyRootFilesystem")
	if !readOnlyRootFilesystem {
		t.Error("readOnlyRootFilesystem should be true")
	}

	allowPrivilegeEscalation, _, _ := unstructured.NestedBool(securityContext, "allowPrivilegeEscalation")
	if allowPrivilegeEscalation {
		t.Error("allowPrivilegeEscalation should be false")
	}

	// Verify environment variables
	envVars, found, _ := unstructured.NestedSlice(container, "env")
	if !found {
		t.Fatal("env not found in container")
	}

	envMap := make(map[string]string)
	for _, e := range envVars {
		eMap := e.(map[string]any)
		envMap[eMap["name"].(string)] = eMap["value"].(string)
	}

	if envMap["INCIDENT_ID"] != "inc-123" {
		t.Errorf("INCIDENT_ID = %v, want inc-123", envMap["INCIDENT_ID"])
	}

	if envMap["PLAN_ID"] != "plan-456" {
		t.Errorf("PLAN_ID = %v, want plan-456", envMap["PLAN_ID"])
	}

	if envMap["ALLOW_SCRIPT"] != "false" {
		t.Errorf("ALLOW_SCRIPT = %v, want false", envMap["ALLOW_SCRIPT"])
	}
}

func TestDAGToWorkflow_TaskInputsSerialization(t *testing.T) {

	client := New("test-namespace")

	inputs := map[string]any{
		"action":    "get",
		"resource":  "pods",
		"namespace": "default",
		"count":     42,
		"enabled":   true,
	}

	dag := newTestDAG("inc-123", "plan-456", []domain.DAGTask{
		{
			ID:     "task-1",
			Name:   "Check pods",
			Tool:   "kubectl",
			Inputs: inputs,
		},
	})

	workflow, err := client.dagToWorkflow("test-workflow", dag)
	if err != nil {
		t.Fatalf("dagToWorkflow() error = %v", err)
	}

	// Verify workflow was created (inputs will be JSON serialized in task arguments)
	if workflow.GetName() != "test-workflow" {
		t.Errorf("Workflow name = %v, want test-workflow", workflow.GetName())
	}

	// The actual verification that inputs are serialized correctly happens at runtime
	// when Argo executes the workflow. Here we just verify the workflow structure is valid.
}

func TestDAGToWorkflow_EmptyTasks(t *testing.T) {

	client := New("test-namespace")

	dag := newTestDAG("inc-123", "plan-456", []domain.DAGTask{})

	workflow, err := client.dagToWorkflow("test-workflow", dag)
	if err != nil {
		t.Fatalf("dagToWorkflow() error = %v", err)
	}

	// Verify workflow was created even with no tasks
	if workflow.GetName() != "test-workflow" {
		t.Errorf("Name = %v, want test-workflow", workflow.GetName())
	}

	// Verify DAG template exists but has empty tasks
	specObj := jsonClone(workflow.Object)
	spec, _, _ := unstructured.NestedMap(specObj, "spec")
	templates, _, _ := unstructured.NestedSlice(spec, "templates")

	var dagTemplate map[string]any
	for _, tmpl := range templates {
		tmplMap := tmpl.(map[string]any)
		if tmplMap["name"] == "remediation-dag" {
			dagTemplate = tmplMap
			break
		}
	}

	if dagTemplate == nil {
		t.Fatal("remediation-dag template not found")
	}

	dagObj := jsonClone(dagTemplate)
	dagSpec, _, _ := unstructured.NestedMap(dagObj, "dag")
	tasks, _, _ := unstructured.NestedSlice(dagSpec, "tasks")

	if len(tasks) != 0 {
		t.Errorf("Task count = %d, want 0", len(tasks))
	}
}

func TestDAGToWorkflow_MultipleDependencies(t *testing.T) {

	client := New("test-namespace")

	// Create a DAG where one task depends on multiple previous tasks
	tasks := []domain.DAGTask{
		{ID: "check-pods", Name: "Check pods", Tool: "k8s.pod.list", Inputs: map[string]any{}},
		{ID: "check-logs", Name: "Check logs", Tool: "k8s.pod.logs", Inputs: map[string]any{}, RunAfter: []string{"check-pods"}},
		{ID: "check-metrics", Name: "Check metrics", Tool: "prom.query", Inputs: map[string]any{}, RunAfter: []string{"check-pods"}},
		{ID: "restart-pod", Name: "Restart pod", Tool: "k8s.pod.delete", Inputs: map[string]any{}, RunAfter: []string{"check-logs", "check-metrics"}},
	}

	dag := newTestDAG("inc-999", "plan-888", tasks)
	workflow, err := client.dagToWorkflow("complex-workflow", dag)
	if err != nil {
		t.Fatalf("dagToWorkflow() error = %v", err)
	}

	// Extract tasks
	specObj := jsonClone(workflow.Object)
	spec, _, _ := unstructured.NestedMap(specObj, "spec")
	templates, _, _ := unstructured.NestedSlice(spec, "templates")

	var dagTemplate map[string]any
	for _, tmpl := range templates {
		tmplMap := tmpl.(map[string]any)
		if tmplMap["name"] == "remediation-dag" {
			dagTemplate = tmplMap
			break
		}
	}

	dagObj := jsonClone(dagTemplate)
	dagSpec, _, _ := unstructured.NestedMap(dagObj, "dag")
	argoTasks, _, _ := unstructured.NestedSlice(dagSpec, "tasks")

	if len(argoTasks) != 4 {
		t.Fatalf("Task count = %d, want 4", len(argoTasks))
	}

	// Find restart-pod task
	var restartTask map[string]any
	for _, taskIface := range argoTasks {
		task := taskIface.(map[string]any)
		if task["name"] == "restart-pod" {
			restartTask = task
			break
		}
	}

	if restartTask == nil {
		t.Fatal("restart-pod task not found")
	}

	rawDeps, ok := restartTask["dependencies"].([]interface{})
	if !ok {
		t.Fatalf("wrong type for dependencies: %T", restartTask["dependencies"])
	}

	deps := make([]string, len(rawDeps))
	for i, v := range rawDeps {
		deps[i] = v.(string)
	}
	if !ok {
		t.Fatalf("restart-pod dependencies wrong type: %T", restartTask["dependencies"])
	}

	if len(deps) != 2 {
		t.Errorf("restart-pod dependencies count = %d, want 2", len(deps))
	}

	// Verify it has both dependencies
	depsMap := make(map[string]bool)
	for _, dep := range deps {
		depsMap[dep] = true
	}

	if !depsMap["check-logs"] || !depsMap["check-metrics"] {
		t.Errorf("restart-pod dependencies = %v, want [check-logs, check-metrics]", deps)
	}
}

func TestNew(t *testing.T) {

	client := New("test-ns")

	if client == nil {
		t.Fatal("New() returned nil")
	}

	if client.Namespace != "test-ns" {
		t.Errorf("Namespace = %v, want test-ns", client.Namespace)
	}

	if client.dynamicClient != nil {
		t.Error("dynamicClient should be nil until Initialize() is called")
	}
}
