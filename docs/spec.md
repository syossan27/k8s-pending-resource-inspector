# k8s-pending-resource-inspector - Specification Document

## Overview

This CLI tool is designed to detect situations in a Kubernetes cluster where `Pod` resources remain in a `Pending` state due to excessive CPU or memory `requests` (and optionally `limits`) that exceed the capacity of all available nodes.

It is implemented in Go using the `client-go` library, and intended for use in automated diagnostics within Kubernetes environments.

---

## Core Features

### 1. Fetch Node Allocatable Resources

Uses the Kubernetes API via `client-go` to fetch `status.allocatable.cpu` and `status.allocatable.memory` from all nodes in the cluster.

### 2. Analyze Pod Resource Requests

- Targets `Pending` Pods from all namespaces or a specific one.
- Parses:
  - `spec.containers[].resources.requests`
  - `spec.containers[].resources.limits` (optional)

### 3. Evaluation Logic

- For each Pending Pod:
  - If both CPU and memory requests exceed the allocatable resources of **all nodes**, the Pod is marked as unschedulable.
- Optional considerations:
  - `NodeAffinity`
  - `taints` / `tolerations`

### 4. Reporting

- Default output: Human-readable message in standard output.
- Optional: JSON or YAML format for automated pipelines.

#### Example Output

[✗] Pod: frontend-app-7899f7
→ Reason: requests.memory = 10Gi exceeds all node allocatable.memory (max: 8Gi)
→ Suggested: Lower requests.memory to <= 8Gi or add higher-memory node

### 5. Notifications (Optional)

- Slack Webhook support
- Prometheus PushGateway support for custom metrics export

---

## Technology Stack

| Item         | Description                                                   |
|--------------|---------------------------------------------------------------|
| Language     | Go (v1.22+)                                                   |
| Libraries    | `k8s.io/client-go`, `spf13/cobra`, `sirupsen/logrus`, etc.    |
| Runtime      | Standalone binary or container                                |
| Permissions  | Read-only access to Pods and Nodes (`get`, `list`, `watch`)   |

---

## CLI Example

```bash
k8s-pending-resource-inspector \
  --namespace=prod \
  --include-limits \
  --output=json \
  --alert-slack=https://hooks.slack.com/services/XXXX

## Project Structure (for Go implementation)

.
├── cmd/
│   └── main.go               // CLI entry point
├── internal/
│   ├── fetcher.go            // Retrieves Pod and Node info
│   ├── analyzer.go           // Scheduling logic and resource comparison
│   └── reporter.go           // Output and notification handling
├── pkg/
│   └── types.go              // Shared type definitions
├── go.mod
└── README.md

## Non-Functional Requirements

 - Must complete within ~10 seconds even for clusters with 100+ nodes.
 - Supports structured log output (JSON or readable format).
 - Runs as a non-root user.
 - Can operate with minimal RBAC permissions (read-only).
