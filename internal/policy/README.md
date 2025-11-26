# Policy Engine

OPA-based policy validation for DAG execution.

## Overview

The policy engine validates remediation DAGs before they are converted to Argo Workflows. Policies are written in [Rego](https://www.openpolicyagent.org/docs/latest/policy-language/) and evaluate both the incident context and the proposed DAG.

## Policy Entrypoint

**Query**: `data.sentientd.allow_dag`

**Input**:
```json
{
  "incident": {
    "ID": "incident-123",
    "Environment": "prod",
    "Service": "payment-api",
    "State": "investigating",
    ...
  },
  "dag": {
    "Tasks": [
      {
        "ID": "task-1",
        "Name": "Scale deployment",
        "Tool": "kubectl.scale",
        "Inputs": {...},
        "Approval": {
          "Required": false
        }
      }
    ],
    "Edges": [...]
  }
}
```

**Output**:
- `allow_dag = true` → DAG is allowed to execute
- `allow_dag = false` → DAG is denied
- `deny_reason` (optional) → Human-readable denial message

## Policy Rules

### 1. Production Autoremediation
**Rule**: `deny_prod_autoremediation`

Production incidents require approval for ALL remediation tasks.

```rego
deny_prod_autoremediation {
    input.incident.Environment == "prod"
    some i
    task := input.dag.Tasks[i]
    not task.Approval.Required
}
```

### 2. Forbidden Tools
**Rule**: `deny_forbidden_tools`

Certain tools are completely forbidden and will never be allowed:
- `k8s.applyManifest` - Direct manifest application (too broad)
- `k8s.patchResource` - Arbitrary resource patching (dangerous)
- `k8s.exec` - Pod exec (shell access)
- `script.shell` - Arbitrary shell script execution
- `helm.delete` - Helm chart deletion (destructive)

```rego
forbidden_tool(tool) {
    forbidden_tools := [
        "k8s.applyManifest",
        "k8s.patchResource",
        "k8s.exec",
        "script.shell",
        "helm.delete"
    ]
    tool == forbidden_tools[_]
}
```

### 3. Critical Tools
**Rule**: `deny_missing_approval`

Critical tools require approval in production:
- `kubectl.scale` - Pod scaling operations
- `kubectl.delete` - Resource deletion
- `kubectl.apply` - Apply configuration
- `helm.rollback` - Helm rollbacks
- `helm.upgrade` - Helm upgrades

```rego
critical_tool(tool) {
    critical_tools := [
        "kubectl.scale",
        "kubectl.delete",
        "kubectl.apply",
        "helm.rollback",
        "helm.upgrade"
    ]
    tool == critical_tools[_]
}
```

## Usage

```go
import "github.com/arruko/torvyn-sentientd/internal/policy"

// Initialize policy engine
engine, err := policy.NewEngine("internal/policy/policies")
if err != nil {
    log.Fatal(err)
}

// Validate a DAG
err = engine.ValidateDAG(ctx, incident, dag)
if err != nil {
    // DAG denied: err contains denial reason
    log.Printf("Policy denied DAG: %v", err)
    return
}

// DAG allowed, proceed with workflow creation
```

## Adding New Policies

### Example: Deny DAGs with >10 tasks

Create `internal/policy/policies/max_tasks.rego`:

```rego
package sentientd

deny_too_many_tasks {
    count(input.dag.Tasks) > 10
}

deny_reason = msg {
    deny_too_many_tasks
    msg := sprintf("DAG has %d tasks, maximum is 10", [count(input.dag.Tasks)])
}
```

Then update `allow_dag`:

```rego
allow_dag {
    not deny_prod_autoremediation
    not deny_forbidden_tools
    not deny_missing_approval
    not deny_too_many_tasks
}
```

### Example: Require specific labels

```rego
deny_missing_labels {
    not input.incident.Labels.team
}

deny_reason = msg {
    deny_missing_labels
    msg := "incident missing required label: team"
}
```

## Testing Policies

Use OPA CLI to test policies:

```bash
# Install OPA
brew install opa

# Test a policy with sample input
cat > test_input.json <<EOF
{
  "incident": {
    "Environment": "prod",
    "Service": "payment-api"
  },
  "dag": {
    "Tasks": [
      {
        "Tool": "kubectl.scale",
        "Approval": {
          "Required": false
        }
      }
    ]
  }
}
EOF

# Evaluate policy
opa eval -d internal/policy/policies/policies.rego \
  -i test_input.json \
  'data.sentientd.allow_dag'

# Output:
# {
#   "result": [
#     {
#       "expressions": [
#         {
#           "value": false,
#           "text": "data.sentientd.allow_dag"
#         }
#       ]
#     }
#   ]
# }
```

## Policy Bypass (Development Only)

For local development, you can disable policy enforcement:

```bash
export POLICY_DIR=""  # Empty directory = no policies loaded
./bin/sentientd
# Policy engine will fail to initialize, investigation proceeds without validation
```

**WARNING**: Never disable policies in production!

## Future Enhancements

1. **Policy Versioning**: Load policies from ConfigMaps with version tracking
2. **Policy Testing**: Unit tests for Rego policies
3. **Policy Metrics**: Track deny reasons in Prometheus
4. **Policy Audit Log**: Record all policy decisions
5. **Dynamic Policies**: Hot-reload policies without restart
6. **Policy Simulation**: Dry-run mode to test policies against historical DAGs

## References

- [OPA Documentation](https://www.openpolicyagent.org/docs/latest/)
- [Rego Language](https://www.openpolicyagent.org/docs/latest/policy-language/)
- [Rego Testing](https://www.openpolicyagent.org/docs/latest/policy-testing/)
