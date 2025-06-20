# k8s-pending-resource-inspector

A command-line diagnostic tool for Kubernetes cluster administrators to identify and troubleshoot Pods that remain in `Pending` state due to insufficient CPU or memory resources.

## Overview

This tool analyzes whether pending Pods have resource requests (CPU/memory) that exceed the allocatable capacity of all available cluster nodes, providing actionable insights for resolving scheduling issues.

## Features

- Scans Kubernetes clusters for Pods stuck in Pending state
- Compares Pod resource requests against node allocatable capacity
- Generates human-readable diagnostics with suggested remediation actions
- Supports structured output (JSON/YAML) for automation integration
- Optional notifications via Slack webhooks and Prometheus metrics
- Considers advanced scheduling constraints like NodeAffinity and taints/tolerations

## Installation

### From Source
```bash
git clone https://github.com/syossan27/k8s-pending-resource-inspector.git
cd k8s-pending-resource-inspector
go build -o k8s-pending-resource-inspector ./cmd
```

## Usage

### Basic Usage
```bash
# Analyze all namespaces
./k8s-pending-resource-inspector

# Analyze specific namespace
./k8s-pending-resource-inspector --namespace my-namespace

# Include resource limits in analysis
./k8s-pending-resource-inspector --include-limits

# Output in JSON format
./k8s-pending-resource-inspector --output json
```

### Output Formats
- **Human-readable** (default): Clear diagnostic messages with suggestions
- **JSON**: Structured data for automation and tooling integration
- **YAML**: Alternative structured format

### Example Output
```
[✗] Pod: frontend-app-7899f7
→ Reason: requests.memory = 10Gi exceeds all node allocatable.memory (max: 8Gi)
→ Suggested: Lower requests.memory to <= 8Gi or add higher-memory node
```

## Requirements

- Go 1.21+
- Kubernetes cluster access with appropriate RBAC permissions
- Read access to Pods and Nodes resources

## Development

This project follows a modular architecture:
- `cmd/main.go`: CLI entry point
- `internal/fetcher.go`: Kubernetes API data retrieval
- `internal/analyzer.go`: Resource analysis and scheduling logic
- `internal/reporter.go`: Output formatting and notifications
- `pkg/types.go`: Shared data structures

## License

MIT License - see [LICENSE](LICENSE) file for details.
