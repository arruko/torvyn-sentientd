package main

import (
	"os"
	"testing"
	"time"
)

// --- test setup: override sleepFn to avoid real delays ---
func init() {
	sleepFn = func(d time.Duration) {}
}

// ---------------------------------------------------------
// KUBECTL TESTS
// ---------------------------------------------------------

func TestExecuteTask_Kubectl(t *testing.T) {
	tests := []struct {
		name      string
		task      TaskInput
		wantError bool
	}{
		{
			name: "valid kubectl get",
			task: TaskInput{
				TaskID:   "task-1",
				TaskName: "Get pods",
				Tool:     "kubectl",
				Inputs: map[string]interface{}{
					"action":   "get",
					"resource": "pods",
				},
			},
			wantError: false,
		},
		{
			name: "kubectl missing action",
			task: TaskInput{
				TaskID:   "task-2",
				TaskName: "Get pods",
				Tool:     "kubectl",
				Inputs: map[string]interface{}{
					"resource": "pods",
				},
			},
			wantError: true,
		},
		{
			name: "kubectl missing resource",
			task: TaskInput{
				TaskID:   "task-3",
				TaskName: "Get pods",
				Tool:     "kubectl",
				Inputs: map[string]interface{}{
					"action": "get",
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		err := executeKubectl(tt.task)
		if (err != nil) != tt.wantError {
			t.Errorf("executeKubectl() error = %v, wantError %v", err, tt.wantError)
		}
	}
}

func TestExecuteKubectl_Actions(t *testing.T) {
	actions := []string{"get", "describe", "logs", "delete", "apply"}

	for _, action := range actions {
		task := TaskInput{
			TaskID:   "task-1",
			TaskName: "Kubectl " + action,
			Tool:     "kubectl",
			Inputs: map[string]interface{}{
				"action":   action,
				"resource": "pods",
			},
		}

		if err := executeKubectl(task); err != nil {
			t.Errorf("executeKubectl() with action %s error = %v", action, err)
		}
	}
}

// ---------------------------------------------------------
// HELM TESTS
// ---------------------------------------------------------

func TestExecuteTask_Helm(t *testing.T) {
	tests := []struct {
		name      string
		task      TaskInput
		wantError bool
	}{
		{
			name: "valid helm upgrade",
			task: TaskInput{
				TaskID:   "task-1",
				TaskName: "Upgrade release",
				Tool:     "helm",
				Inputs: map[string]interface{}{
					"action":  "upgrade",
					"release": "my-app",
				},
			},
			wantError: false,
		},
		{
			name: "helm missing action",
			task: TaskInput{
				TaskID:   "task-2",
				TaskName: "Upgrade release",
				Tool:     "helm",
				Inputs: map[string]interface{}{
					"release": "my-app",
				},
			},
			wantError: true,
		},
		{
			name: "helm missing release",
			task: TaskInput{
				TaskID:   "task-3",
				TaskName: "Upgrade release",
				Tool:     "helm",
				Inputs: map[string]interface{}{
					"action": "upgrade",
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		err := executeHelm(tt.task)
		if (err != nil) != tt.wantError {
			t.Errorf("executeHelm() error = %v, wantError %v", err, tt.wantError)
		}
	}
}

func TestExecuteHelm_Actions(t *testing.T) {
	actions := []string{"install", "upgrade", "rollback", "uninstall", "list"}

	for _, action := range actions {
		task := TaskInput{
			TaskID:   "task-1",
			TaskName: "Helm " + action,
			Tool:     "helm",
			Inputs: map[string]interface{}{
				"action":  action,
				"release": "my-release",
			},
		}

		if err := executeHelm(task); err != nil {
			t.Errorf("executeHelm() action %s error = %v", action, err)
		}
	}
}

// ---------------------------------------------------------
// HTTP TESTS
// ---------------------------------------------------------

func TestExecuteTask_HTTP(t *testing.T) {
	tests := []struct {
		name      string
		task      TaskInput
		wantError bool
	}{
		{
			name: "valid HTTP GET",
			task: TaskInput{
				TaskID:   "task-1",
				TaskName: "Health check",
				Tool:     "http",
				Inputs: map[string]interface{}{
					"method": "GET",
					"url":    "http://example.com/health",
				},
			},
			wantError: false,
		},
		{
			name: "HTTP default method",
			task: TaskInput{
				TaskID:   "task-2",
				TaskName: "Health check",
				Tool:     "http",
				Inputs: map[string]interface{}{
					"url": "http://example.com/health",
				},
			},
			wantError: false,
		},
		{
			name: "HTTP missing url",
			task: TaskInput{
				TaskID:   "task-3",
				TaskName: "Health check",
				Tool:     "http",
				Inputs: map[string]interface{}{
					"method": "GET",
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		err := executeHTTP(tt.task)
		if (err != nil) != tt.wantError {
			t.Errorf("executeHTTP() error = %v, wantError %v", err, tt.wantError)
		}
	}
}

func TestExecuteHTTP_Methods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, m := range methods {
		task := TaskInput{
			TaskID:   "task-1",
			TaskName: "HTTP " + m,
			Tool:     "http",
			Inputs: map[string]interface{}{
				"method": m,
				"url":    "http://example.com/api",
			},
		}

		if err := executeHTTP(task); err != nil {
			t.Errorf("executeHTTP() method %s error = %v", m, err)
		}
	}
}

// ---------------------------------------------------------
// SCRIPT TESTS
// ---------------------------------------------------------

func TestExecuteTask_Script_Blocked(t *testing.T) {
	_ = os.Unsetenv("ALLOW_SCRIPT")

	task := TaskInput{
		TaskID:   "task-1",
		TaskName: "Run script",
		Tool:     "script",
		Inputs: map[string]interface{}{
			"script": "echo hello",
		},
	}

	err := executeScript(task)
	if err == nil {
		t.Fatal("expected error when script execution is disabled")
	}
}

func TestExecuteTask_Script_Enabled(t *testing.T) {
	_ = os.Setenv("ALLOW_SCRIPT", "true")
	defer func() { _ = os.Unsetenv("ALLOW_SCRIPT") }()

	task := TaskInput{
		TaskID:   "task-1",
		TaskName: "Run script",
		Tool:     "script",
		Inputs: map[string]interface{}{
			"script": "echo hello",
		},
	}

	if err := executeScript(task); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecuteTask_Script_MissingScript(t *testing.T) {
	_ = os.Setenv("ALLOW_SCRIPT", "true")
	defer func() { _ = os.Unsetenv("ALLOW_SCRIPT") }()

	task := TaskInput{
		TaskID:   "task-1",
		TaskName: "Run script",
		Tool:     "script",
		Inputs:   map[string]interface{}{},
	}

	err := executeScript(task)
	if err == nil {
		t.Fatal("expected missing script error")
	}
}

// ---------------------------------------------------------
// WAIT TESTS
// ---------------------------------------------------------

func TestExecuteTask_Wait(t *testing.T) {
	tests := []struct {
		name      string
		task      TaskInput
		wantError bool
	}{
		{
			name: "valid wait",
			task: TaskInput{
				TaskID:   "task-1",
				TaskName: "Wait",
				Tool:     "wait",
				Inputs: map[string]interface{}{
					"duration": "100ms",
				},
			},
			wantError: false,
		},
		{
			name: "invalid duration",
			task: TaskInput{
				TaskID:   "task-2",
				TaskName: "Wait invalid",
				Tool:     "wait",
				Inputs: map[string]interface{}{
					"duration": "not-a-duration",
				},
			},
			wantError: true,
		},
		{
			name: "missing duration",
			task: TaskInput{
				TaskID:   "task-3",
				TaskName: "Wait missing",
				Tool:     "wait",
				Inputs:   map[string]interface{}{},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		err := executeWait(tt.task)
		if (err != nil) != tt.wantError {
			t.Errorf("executeWait() error = %v, wantError %v", err, tt.wantError)
		}
	}
}

// ---------------------------------------------------------
// ROUTING
// ---------------------------------------------------------

func TestExecuteTask_UnknownTool(t *testing.T) {
	task := TaskInput{
		TaskID:   "t",
		TaskName: "Unknown",
		Tool:     "unknown-tool",
		Inputs:   map[string]interface{}{},
	}

	err := executeTask(task)
	if err == nil {
		t.Fatal("expected unknown tool error")
	}
}

func TestExecuteTask_Routing(t *testing.T) {
	tests := []struct {
		name      string
		tool      string
		inputs    map[string]interface{}
		wantError bool
	}{
		{
			name: "route kubectl",
			tool: "kubectl",
			inputs: map[string]interface{}{
				"action":   "get",
				"resource": "pods",
			},
			wantError: false,
		},
		{
			name: "route helm",
			tool: "helm",
			inputs: map[string]interface{}{
				"action":  "list",
				"release": "r",
			},
			wantError: false,
		},
		{
			name: "route http",
			tool: "http",
			inputs: map[string]interface{}{
				"url": "http://example.com",
			},
			wantError: false,
		},
		{
			name: "route wait",
			tool: "wait",
			inputs: map[string]interface{}{
				"duration": "100ms",
			},
			wantError: false,
		},
		{
			name:      "route unknown",
			tool:      "unknown",
			inputs:    map[string]interface{}{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		task := TaskInput{
			TaskID:   "t",
			TaskName: tt.name,
			Tool:     tt.tool,
			Inputs:   tt.inputs,
		}

		err := executeTask(task)
		if (err != nil) != tt.wantError {
			t.Errorf("%s: error = %v, wantError %v", tt.name, err, tt.wantError)
		}
	}
}

// ---------------------------------------------------------
// STRUCT TEST
// ---------------------------------------------------------

func TestTaskInput_Structure(t *testing.T) {
	task := TaskInput{
		TaskID:     "task-123",
		TaskName:   "Test task",
		Tool:       "kubectl",
		Inputs:     map[string]interface{}{"key": "value"},
		IncidentID: "inc-456",
		PlanID:     "plan-789",
	}

	if task.TaskID != "task-123" {
		t.Errorf("TaskID = %v", task.TaskID)
	}
	if task.TaskName != "Test task" {
		t.Errorf("TaskName = %v", task.TaskName)
	}
	if task.Tool != "kubectl" {
		t.Errorf("Tool = %v", task.Tool)
	}
	if task.IncidentID != "inc-456" {
		t.Errorf("IncidentID = %v", task.IncidentID)
	}
	if task.PlanID != "plan-789" {
		t.Errorf("PlanID = %v", task.PlanID)
	}
	if task.Inputs["key"] != "value" {
		t.Errorf("Inputs[key] = %v", task.Inputs["key"])
	}
}
