package policy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/arruko/torvyn-sentientd/internal/domain"
	"github.com/arruko/torvyn-sentientd/internal/logging"
	"github.com/open-policy-agent/opa/v1/rego"
)

// Engine validates DAGs against Rego policies.
type Engine struct {
	query rego.PreparedEvalQuery
}

// NewEngine creates a policy engine that loads and compiles Rego policies from a directory.
// It prepares a query against the `data.sentientd.allow_dag` entrypoint.
//
// Parameters:
//   - policyDir: Directory containing *.rego policy files
//
// Returns error if no policies found or compilation fails.
func NewEngine(policyDir string) (*Engine, error) {
	if policyDir == "" {
		return nil, fmt.Errorf("policy directory cannot be empty")
	}

	// Load all .rego files from the policy directory
	policyFiles, err := filepath.Glob(filepath.Join(policyDir, "*.rego"))
	if err != nil {
		return nil, fmt.Errorf("failed to glob policy files: %w", err)
	}

	if len(policyFiles) == 0 {
		return nil, fmt.Errorf("no .rego policy files found in %s", policyDir)
	}

	logging.Info.Printf("Loading %d policy files from %s", len(policyFiles), policyDir)

	// Build rego options with all policy files
	regoOpts := []func(*rego.Rego){
		rego.Query("data.sentientd.allow_dag"),
	}

	for _, policyFile := range policyFiles {
		policyBytes, err := os.ReadFile(policyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read policy file %s: %w", policyFile, err)
		}

		logging.Info.Printf("Loaded policy: %s (%d bytes)", filepath.Base(policyFile), len(policyBytes))
		regoOpts = append(regoOpts, rego.Module(policyFile, string(policyBytes)))
	}

	// Compile the query
	r := rego.New(regoOpts...)
	ctx := context.Background()

	query, err := r.PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to compile Rego policies: %w", err)
	}

	logging.Info.Printf("Policy engine initialized with entrypoint: data.sentientd.allow_dag")

	return &Engine{
		query: query,
	}, nil
}

// ValidateDAG evaluates the loaded policies against an incident and DAG.
// Returns nil if the DAG is allowed, or an error with denial reason if rejected.
//
// Input to Rego:
//
//	{
//	  "incident": { ... },
//	  "dag": { ... }
//	}
//
// Expected Rego output:
//   - data.sentientd.allow_dag = true  → DAG allowed
//   - data.sentientd.allow_dag = false → DAG denied
//   - data.sentientd.deny_reason (optional) → human-readable denial message
func (e *Engine) ValidateDAG(ctx context.Context, inc domain.Incident, dag domain.DAG) error {
	// Prepare input for Rego
	input := map[string]interface{}{
		"incident": inc,
		"dag":      dag,
	}

	// Evaluate the query
	results, err := e.query.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return fmt.Errorf("failed to evaluate policy: %w", err)
	}

	// Check if there are any results
	if len(results) == 0 {
		return fmt.Errorf("policy evaluation returned no results (check policy syntax)")
	}

	// Get the first result
	result := results[0]

	// Check if there are any expressions
	if len(result.Expressions) == 0 {
		return fmt.Errorf("policy evaluation returned no expressions")
	}

	// Extract the allow_dag value
	allowValue := result.Expressions[0].Value
	allow, ok := allowValue.(bool)
	if !ok {
		return fmt.Errorf("policy returned non-boolean value for allow_dag: %T", allowValue)
	}

	if !allow {
		// DAG is denied - try to extract denial reason
		denyReason := extractDenyReason(result.Bindings)
		if denyReason != "" {
			logging.Info.Printf("Policy denied DAG for incident %s: %s", inc.ID, denyReason)
			return fmt.Errorf("policy denied: %s", denyReason)
		}

		logging.Info.Printf("Policy denied DAG for incident %s (no reason provided)", inc.ID)
		return fmt.Errorf("policy denied DAG execution")
	}

	logging.Info.Printf("Policy allowed DAG for incident %s", inc.ID)
	return nil
}

// extractDenyReason attempts to extract a human-readable denial reason from policy bindings.
// Looks for data.sentientd.deny_reason in the bindings.
func extractDenyReason(bindings map[string]interface{}) string {
	if bindings == nil {
		return ""
	}

	// Try to get deny_reason from bindings
	if reason, ok := bindings["deny_reason"]; ok {
		if reasonStr, ok := reason.(string); ok {
			return reasonStr
		}
	}

	return ""
}
