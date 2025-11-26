# Security Hardening Summary

This document describes the security hardening applied to the Kubernetes deployments based on Trivy security scanning results.

## Overview

All Kubernetes manifests have been hardened to follow security best practices while maintaining required functionality for components like SPIRE that need elevated privileges for workload attestation.

---

## NATS JetStream Hardening

**File**: [deploy/nats/nats-jetstream.yaml](deploy/nats/nats-jetstream.yaml)

### Security Improvements

#### Pod-level Security Context
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  runAsGroup: 1000
  fsGroup: 1000
  seccompProfile:
    type: RuntimeDefault
```

#### Container-level Security Context
```yaml
securityContext:
  allowPrivilegeEscalation: false
  runAsNonRoot: true
  runAsUser: 1000
  capabilities:
    drop:
      - ALL
  readOnlyRootFilesystem: true
```

#### Volume Mounts
- Added `/tmp` emptyDir volume for writable temporary space (required by NATS)
- Config mounted as read-only
- Data volume for persistent storage

### Trivy Issues Resolved
- ✅ AVD-KSV-0014: Read-only root filesystem enabled
- ✅ AVD-KSV-0118: Non-root user configured (UID 1000)
- ✅ Security context properly defined

---

## SPIRE Server Hardening

**File**: [deploy/spire/spire-server.yaml](deploy/spire/spire-server.yaml)

### Security Improvements

#### Pod-level Security Context
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  runAsGroup: 1000
  fsGroup: 1000
  seccompProfile:
    type: RuntimeDefault
```

#### Container-level Security Context
```yaml
securityContext:
  allowPrivilegeEscalation: false
  runAsNonRoot: true
  runAsUser: 1000
  capabilities:
    drop:
      - ALL
  readOnlyRootFilesystem: true
```

#### Volume Mounts
- Added `/tmp` emptyDir volume for writable temporary space
- Config mounted as read-only
- Data volume for SPIRE database
- Socket directory for SPIRE API

### Trivy Issues Resolved
- ✅ AVD-KSV-0014: Read-only root filesystem enabled
- ✅ AVD-KSV-0118: Non-root user configured (UID 1000)
- ✅ Security context properly defined

---

## SPIRE Agent Hardening

**File**: [deploy/spire/spire-agent.yaml](deploy/spire/spire-agent.yaml)

### Security Improvements

#### Pod-level Security Context
```yaml
securityContext:
  seccompProfile:
    type: RuntimeDefault
```

#### Init Container Security Context
```yaml
securityContext:
  runAsUser: 0
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
  readOnlyRootFilesystem: true
```

#### Main Container Security Context
```yaml
securityContext:
  # REQUIRED for workload attestation - cannot be changed
  runAsUser: 0
  privileged: true
  # Dropped capabilities where possible
  capabilities:
    drop:
      - NET_RAW
```

#### Volume Mounts
- Added `/tmp` emptyDir volume for temporary space
- Config mounted as read-only

### Intentional Security Exceptions (Required for SPIRE Functionality)

The following security configurations **cannot be changed** as they are required for SPIRE Agent to perform workload attestation:

1. **hostPID: true** - Required to access workload process information for attestation
2. **hostNetwork: true** - Required to communicate with kubelet for workload verification
3. **privileged: true** - Required for the main container to perform workload attestation
4. **runAsUser: 0** - Required for privileged operations
5. **nodes/proxy permission** - Required for workload attestation (Trivy AVD-KSV-0047)

These are **documented SPIRE requirements** and cannot be removed without breaking functionality.

**Reference**: https://spiffe.io/docs/latest/deploying/spire_agent/

### Trivy Issues Resolved
- ✅ AVD-KSV-0014: Read-only root filesystem enabled for init container
- ⚠️ AVD-KSV-0014: Main container cannot use read-only filesystem (SPIRE requirement)
- ⚠️ AVD-KSV-0009: hostNetwork required for workload attestation
- ⚠️ AVD-KSV-0010: hostPID required for workload attestation
- ⚠️ AVD-KSV-0017: privileged mode required for workload attestation
- ⚠️ AVD-KSV-0047: nodes/proxy permission required for workload attestation
- ⚠️ AVD-KSV-0118: Root user required for privileged operations

---

## Security Hardening Principles Applied

### 1. Principle of Least Privilege
- All containers run as non-root users (except where functionally required)
- Capabilities dropped to minimum required set
- Read-only root filesystems where possible

### 2. Defense in Depth
- Pod-level and container-level security contexts
- Seccomp profiles applied
- Privilege escalation disabled

### 3. Immutable Infrastructure
- Read-only root filesystems prevent tampering
- Configuration mounted as read-only
- Writable space limited to designated volumes

### 4. Documentation of Exceptions
- All security exceptions documented with reasons
- References to official documentation provided
- Clear markers in YAML comments

---

## Verification

### Run Trivy Scan
```bash
trivy config deploy/
```

### Trivy Ignore File

A [`.trivyignore.yaml`](.trivyignore.yaml) file has been created to suppress expected security findings for SPIRE Agent. This file documents why specific security exceptions are required and cannot be fixed without breaking SPIRE functionality.

**Ignored findings**:
- `AVD-KSV-0009` - hostNetwork (required for kubelet communication)
- `AVD-KSV-0010` - hostPID (required for process attestation)
- `AVD-KSV-0014` - Read-only root filesystem (SPIRE Agent main container)
- `AVD-KSV-0017` - Privileged mode (required for attestation)
- `AVD-KSV-0047` - nodes/proxy permission (required for Kubernetes attestation)

### Expected Results
- **NATS JetStream**: All issues resolved ✅
- **SPIRE Server**: All issues resolved ✅
- **SPIRE Agent**: Documented exceptions in `.trivyignore.yaml` ⚠️

### Test Deployments
```bash
# Deploy NATS
kubectl apply -f deploy/nats/nats-jetstream.yaml

# Deploy SPIRE
kubectl apply -f deploy/spire/spire-server.yaml
kubectl apply -f deploy/spire/spire-agent.yaml

# Verify pods are running
kubectl -n nats get pods
kubectl -n spire get pods
```

---

## Production Recommendations

1. **Pod Security Standards**: Consider enforcing Kubernetes Pod Security Standards at the namespace level
2. **Network Policies**: Add NetworkPolicy resources to restrict traffic between components
3. **Resource Limits**: All deployments include resource requests/limits
4. **Monitoring**: Add security monitoring for privileged containers (SPIRE Agent)
5. **Admission Control**: Use admission controllers (OPA Gatekeeper, Kyverno) to enforce security policies

---

## Summary

This security hardening brings the sentientd infrastructure deployments to production-ready security standards:

- ✅ **NATS JetStream**: Fully hardened with no security exceptions
- ✅ **SPIRE Server**: Fully hardened with no security exceptions
- ⚠️ **SPIRE Agent**: Hardened where possible; documented exceptions for required privileged operations

All security exceptions are intentional, documented, and required for proper SPIRE functionality. The platform maintains strong security posture while enabling workload identity capabilities.
