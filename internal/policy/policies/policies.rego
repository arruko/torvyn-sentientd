package sentientd

# Default deny - must explicitly allow
default allow_dag = false

# Allow DAG if no deny rules match
allow_dag {
    not deny_prod_autoremediation
    not deny_forbidden_tools
    not deny_missing_approval
}

# Deny autoremediation in production without approval
deny_prod_autoremediation {
    input.incident.Environment == "prod"
    some i
    task := input.dag.Tasks[i]
    not task.Approval.Required
}

# Deny forbidden tools
deny_forbidden_tools {
    some i
    task := input.dag.Tasks[i]
    forbidden_tool(task.Tool)
}

# Deny if any critical task is missing approval in production
deny_missing_approval {
    input.incident.Environment == "prod"
    some i
    task := input.dag.Tasks[i]
    critical_tool(task.Tool)
    not task.Approval.Required
}

# Define forbidden tools - these should NEVER be used
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

# Define critical tools that require approval in production
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

# Compute denial reason for better error messages
deny_reason = msg {
    deny_prod_autoremediation
    msg := "production incidents require approval for all remediation tasks"
}

deny_reason = msg {
    deny_forbidden_tools
    some i
    task := input.dag.Tasks[i]
    forbidden_tool(task.Tool)
    msg := sprintf("forbidden tool detected: %s", [task.Tool])
}

deny_reason = msg {
    deny_missing_approval
    some i
    task := input.dag.Tasks[i]
    critical_tool(task.Tool)
    not task.Approval.Required
    msg := sprintf("critical tool %s requires approval in production", [task.Tool])
}
