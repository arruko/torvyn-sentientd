package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/arruko/torvyn-sentientd/internal/domain"
)

// Helper to create a temporary policy directory with test policies
func createTestPolicyDir(t *testing.T, policies map[string]string) string {
	t.Helper()

	tmpDir := t.TempDir()

	for name, content := range policies {
		policyPath := filepath.Join(tmpDir, name)
		if err := os.WriteFile(policyPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test policy %s: %v", name, err)
		}
	}

	return tmpDir
}

func TestNewEngine_ValidPolicy(t *testing.T) {
	t.Parallel()

	// Create a simple allow-all policy
	allowPolicy := `
package sentientd

default allow_dag = true
`

	policyDir := createTestPolicyDir(t, map[string]string{
		"allow.rego": allowPolicy,
	})

	engine, err := NewEngine(policyDir)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	if engine == nil {
		t.Fatal("NewEngine() returned nil")
	}
}

func TestNewEngine_NoPolicies(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	_, err := NewEngine(tmpDir)
	if err == nil {
		t.Error("NewEngine() expected error for empty policy directory, got nil")
	}
}

func TestNewEngine_InvalidPolicyDir(t *testing.T) {
	t.Parallel()

	_, err := NewEngine("/nonexistent/directory")
	if err == nil {
		t.Error("NewEngine() expected error for nonexistent directory, got nil")
	}
}

func TestValidateDAG_AllowPolicy(t *testing.T) {
	t.Parallel()

	// Policy that always allows
	allowPolicy := `
package sentientd

default allow_dag = true
`

	policyDir := createTestPolicyDir(t, map[string]string{
		"allow.rego": allowPolicy,
	})

	engine, err := NewEngine(policyDir)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	incident := domain.Incident{
		ID:          "inc-123",
		Service:     "payment-api",
		Environment: "production",
		Severity:    "critical",
	}

	dag := domain.DAG{
		IncidentID: "inc-123",
		PlanID:     "plan-456",
		Tasks: []domain.DAGTask{
			{
				ID:   "task-1",
				Tool: "k8s.pod.list",
				Inputs: map[string]interface{}{
					"namespace": "production",
				},
			},
		},
	}

	err = engine.ValidateDAG(context.Background(), incident, dag)
	if err != nil {
		t.Errorf("ValidateDAG() error = %v, expected nil for allow policy", err)
	}
}

func TestValidateDAG_DenyPolicy(t *testing.T) {
	t.Parallel()

	// Policy that always denies
	denyPolicy := `
package sentientd

default allow_dag = false
deny_reason = "Policy denies all DAG executions"
`

	policyDir := createTestPolicyDir(t, map[string]string{
		"deny.rego": denyPolicy,
	})

	engine, err := NewEngine(policyDir)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	incident := domain.Incident{
		ID:          "inc-123",
		Service:     "payment-api",
		Environment: "production",
	}

	dag := domain.DAG{
		IncidentID: "inc-123",
		PlanID:     "plan-456",
		Tasks:      []domain.DAGTask{},
	}

	err = engine.ValidateDAG(context.Background(), incident, dag)
	if err == nil {
		t.Error("ValidateDAG() expected error for deny policy, got nil")
	}

	// Verify error message - the deny_reason is a variable, not a rule with body,
	// so it won't be in bindings. We'll just get "policy denied DAG execution"
	if err != nil && err.Error() != "policy denied DAG execution" {
		t.Errorf("Error message = %v, want 'policy denied DAG execution'", err.Error())
	}
}

func TestValidateDAG_ConditionalPolicy(t *testing.T) {
	t.Parallel()

	// Policy that denies production environment
	conditionalPolicy := `
package sentientd

default allow_dag = true

allow_dag = false if {
    input.incident.environment == "production"
}

deny_reason = "DAG execution not allowed in production" if {
    not allow_dag
}
`

	policyDir := createTestPolicyDir(t, map[string]string{
		"conditional.rego": conditionalPolicy,
	})

	engine, err := NewEngine(policyDir)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	tests := []struct {
		name        string
		environment string
		wantErr     bool
	}{
		{
			name:        "staging environment allowed",
			environment: "staging",
			wantErr:     false,
		},
		{
			name:        "production environment denied",
			environment: "production",
			wantErr:     true,
		},
		{
			name:        "development environment allowed",
			environment: "development",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incident := domain.Incident{
				ID:          "inc-123",
				Service:     "test-service",
				Environment: tt.environment,
			}

			dag := domain.DAG{
				IncidentID: "inc-123",
				PlanID:     "plan-456",
				Tasks:      []domain.DAGTask{},
			}

			err := engine.ValidateDAG(context.Background(), incident, dag)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDAG() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDAG_ForbiddenToolsPolicy(t *testing.T) {
	t.Parallel()

	// Policy that checks for forbidden tools
	forbiddenToolsPolicy := `
package sentientd

default allow_dag = true

forbidden_tools := ["k8s.applyManifest", "k8s.patchResource", "k8s.exec"]

allow_dag = false if {
    some task in input.dag.tasks
    task.tool in forbidden_tools
}

deny_reason = reason if {
    not allow_dag
    some task in input.dag.tasks
    task.tool in forbidden_tools
    reason := sprintf("Tool '%s' is forbidden", [task.tool])
}
`

	policyDir := createTestPolicyDir(t, map[string]string{
		"forbidden_tools.rego": forbiddenToolsPolicy,
	})

	engine, err := NewEngine(policyDir)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	incident := domain.Incident{
		ID:      "inc-123",
		Service: "test-service",
	}

	tests := []struct {
		name    string
		tool    string
		wantErr bool
	}{
		{
			name:    "allowed tool k8s.pod.list",
			tool:    "k8s.pod.list",
			wantErr: false,
		},
		{
			name:    "forbidden tool k8s.exec",
			tool:    "k8s.exec",
			wantErr: true,
		},
		{
			name:    "forbidden tool k8s.applyManifest",
			tool:    "k8s.applyManifest",
			wantErr: true,
		},
		{
			name:    "allowed tool prom.query",
			tool:    "prom.query",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dag := domain.DAG{
				IncidentID: "inc-123",
				PlanID:     "plan-456",
				Tasks: []domain.DAGTask{
					{
						ID:     "task-1",
						Tool:   tt.tool,
						Inputs: map[string]interface{}{},
					},
				},
			}

			err := engine.ValidateDAG(context.Background(), incident, dag)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDAG() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDAG_MultiplePolices(t *testing.T) {
	t.Parallel()

	// Multiple policy files that work together
	envPolicy := `
package sentientd

default allow_dag = true

allow_dag = false if {
    input.incident.environment == "production"
    input.incident.severity != "critical"
}
`

	toolPolicy := `
package sentientd

forbidden_tools := ["k8s.delete"]

allow_dag = false if {
    some task in input.dag.tasks
    task.tool in forbidden_tools
}
`

	policyDir := createTestPolicyDir(t, map[string]string{
		"env.rego":  envPolicy,
		"tool.rego": toolPolicy,
	})

	engine, err := NewEngine(policyDir)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	// Test: critical incident in production with safe tool - should allow
	incident1 := domain.Incident{
		ID:          "inc-123",
		Environment: "production",
		Severity:    "critical",
	}

	dag1 := domain.DAG{
		IncidentID: "inc-123",
		Tasks: []domain.DAGTask{
			{ID: "task-1", Tool: "k8s.pod.list", Inputs: map[string]interface{}{}},
		},
	}

	err = engine.ValidateDAG(context.Background(), incident1, dag1)
	if err != nil {
		t.Errorf("ValidateDAG() for critical production = %v, want nil", err)
	}

	// Test: warning incident in production - should deny
	incident2 := domain.Incident{
		ID:          "inc-456",
		Environment: "production",
		Severity:    "warning",
	}

	dag2 := domain.DAG{
		IncidentID: "inc-456",
		Tasks: []domain.DAGTask{
			{ID: "task-1", Tool: "k8s.pod.list", Inputs: map[string]interface{}{}},
		},
	}

	err = engine.ValidateDAG(context.Background(), incident2, dag2)
	if err == nil {
		t.Error("ValidateDAG() for warning in production expected error, got nil")
	}

	// Test: forbidden tool - should deny
	incident3 := domain.Incident{
		ID:          "inc-789",
		Environment: "staging",
		Severity:    "critical",
	}

	dag3 := domain.DAG{
		IncidentID: "inc-789",
		Tasks: []domain.DAGTask{
			{ID: "task-1", Tool: "k8s.delete", Inputs: map[string]interface{}{}},
		},
	}

	err = engine.ValidateDAG(context.Background(), incident3, dag3)
	if err == nil {
		t.Error("ValidateDAG() for forbidden tool expected error, got nil")
	}
}
