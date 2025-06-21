package internal

import (
	"context"
	"fmt"

	"github.com/syossan27/k8s-pending-resource-inspector/pkg/types"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Analyzer provides functionality to analyze pod schedulability and resource constraints
// in a Kubernetes cluster. It uses a Fetcher to retrieve cluster information and
// performs analysis to determine why pods might be pending.
type Analyzer struct {
	fetcher *Fetcher
}

// NewAnalyzer creates a new Analyzer instance with the provided Fetcher.
// The Fetcher is used to retrieve node and pod information from the Kubernetes cluster.
//
// Parameters:
//   - fetcher: A Fetcher instance for retrieving cluster resources
//
// Returns:
//   - *Analyzer: A new Analyzer instance
func NewAnalyzer(fetcher *Fetcher) *Analyzer {
	return &Analyzer{
		fetcher: fetcher,
	}
}

// AnalyzePodSchedulability analyzes all pending pods in the specified namespace (or cluster-wide)
// to determine their schedulability based on resource availability. It compares pod resource
// requirements against node allocatable resources to identify scheduling constraints.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and timeout
//   - namespace: Target namespace to analyze. If empty, analyzes cluster-wide
//   - includeLimits: If true, uses resource limits instead of requests for analysis
//
// Returns:
//   - []types.AnalysisResult: Analysis results for each pending pod, including schedulability status and suggestions
//   - error: An error if fetching pods or nodes fails
func (a *Analyzer) AnalyzePodSchedulability(ctx context.Context, namespace string, includeLimits bool) ([]types.AnalysisResult, error) {
	pods, err := a.fetcher.FetchPendingPods(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pending pods: %w", err)
	}

	nodes, err := a.fetcher.FetchNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch nodes: %w", err)
	}

	results := make([]types.AnalysisResult, 0, len(pods))
	for _, pod := range pods {
		result := a.analyzeSinglePod(pod, nodes, includeLimits)
		results = append(results, result)
	}

	return results, nil
}

// analyzeSinglePod performs schedulability analysis for a single pod against available nodes.
// It determines if the pod can be scheduled based on resource requirements and provides
// detailed reasons and suggestions when scheduling is not possible.
//
// Parameters:
//   - pod: The pod information to analyze
//   - nodes: Available nodes in the cluster with their resource information
//   - includeLimits: If true, uses resource limits instead of requests for comparison
//
// Returns:
//   - types.AnalysisResult: Detailed analysis result including schedulability status, reasons, and suggestions
func (a *Analyzer) analyzeSinglePod(pod types.PodInfo, nodes []types.NodeInfo, includeLimits bool) types.AnalysisResult {
	maxAvailableCPU, maxAvailableMemory := a.findMaxAvailableResources(nodes)

	podCPU := pod.RequestsCPU
	podMemory := pod.RequestsMemory
	resourceType := "requests"

	if includeLimits && (!pod.LimitsCPU.IsZero() || !pod.LimitsMemory.IsZero()) {
		if !pod.LimitsCPU.IsZero() {
			podCPU = pod.LimitsCPU
		}
		if !pod.LimitsMemory.IsZero() {
			podMemory = pod.LimitsMemory
		}
		resourceType = "limits"
	}

	cpuFits := podCPU.Cmp(maxAvailableCPU) <= 0
	memoryFits := podMemory.Cmp(maxAvailableMemory) <= 0

	isSchedulable := cpuFits && memoryFits

	var reason, suggestion string
	if !isSchedulable {
		switch {
		case !cpuFits && !memoryFits:
			reason = fmt.Sprintf("%s.cpu = %s and %s.memory = %s exceed all node allocatable resources (max CPU: %s, max memory: %s)",
				resourceType, podCPU.String(), resourceType, podMemory.String(),
				maxAvailableCPU.String(), maxAvailableMemory.String())
			suggestion = fmt.Sprintf("Lower %s.cpu to <= %s and %s.memory to <= %s, or add nodes with higher capacity",
				resourceType, maxAvailableCPU.String(), resourceType, maxAvailableMemory.String())
		case !cpuFits:
			reason = fmt.Sprintf("%s.cpu = %s exceeds all node allocatable.cpu (max: %s)",
				resourceType, podCPU.String(), maxAvailableCPU.String())
			suggestion = fmt.Sprintf("Lower %s.cpu to <= %s or add higher-CPU node",
				resourceType, maxAvailableCPU.String())
		default:
			reason = fmt.Sprintf("%s.memory = %s exceeds all node allocatable.memory (max: %s)",
				resourceType, podMemory.String(), maxAvailableMemory.String())
			suggestion = fmt.Sprintf("Lower %s.memory to <= %s or add higher-memory node",
				resourceType, maxAvailableMemory.String())
		}
	}

	return types.AnalysisResult{
		Pod:                pod,
		IsSchedulable:      isSchedulable,
		Reason:             reason,
		Suggestion:         suggestion,
		MaxAvailableCPU:    maxAvailableCPU,
		MaxAvailableMemory: maxAvailableMemory,
	}
}

// findMaxAvailableResources finds the maximum CPU and memory resources available
// across all nodes in the cluster. This represents the theoretical maximum resources
// that a single pod could request and still be schedulable.
//
// Parameters:
//   - nodes: Slice of node information containing allocatable resources
//
// Returns:
//   - resource.Quantity: Maximum allocatable CPU across all nodes
//   - resource.Quantity: Maximum allocatable memory across all nodes
func (a *Analyzer) findMaxAvailableResources(nodes []types.NodeInfo) (resource.Quantity, resource.Quantity) {
	var maxCPU, maxMemory resource.Quantity

	for _, node := range nodes {
		if node.AllocatableCPU.Cmp(maxCPU) > 0 {
			maxCPU = node.AllocatableCPU.DeepCopy()
		}
		if node.AllocatableMemory.Cmp(maxMemory) > 0 {
			maxMemory = node.AllocatableMemory.DeepCopy()
		}
	}

	return maxCPU, maxMemory
}

// EvaluateResourceConstraints evaluates cluster-wide resource constraints and bottlenecks.
// This method is currently a placeholder for future implementation of advanced constraint analysis.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and timeout
//
// Returns:
//   - error: Currently always returns nil (not implemented)
func (a *Analyzer) EvaluateResourceConstraints(ctx context.Context) error {
	return nil
}
