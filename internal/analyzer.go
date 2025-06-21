package internal

import (
	"context"
	"fmt"
	
	"github.com/syossan27/k8s-pending-resource-inspector/pkg/types"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Analyzer struct {
	fetcher *Fetcher
}

func NewAnalyzer(fetcher *Fetcher) *Analyzer {
	return &Analyzer{
		fetcher: fetcher,
	}
}

func (a *Analyzer) AnalyzePodSchedulability(ctx context.Context, namespace string, includeLimits bool) ([]types.AnalysisResult, error) {
	pods, err := a.fetcher.FetchPendingPods(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pending pods: %w", err)
	}
	
	nodes, err := a.fetcher.FetchNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch nodes: %w", err)
	}
	
	var results []types.AnalysisResult
	for _, pod := range pods {
		result := a.analyzeSinglePod(pod, nodes, includeLimits)
		results = append(results, result)
	}
	
	return results, nil
}

// it analyzes resource limits instead of requests. Returns an AnalysisResult
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
		if !cpuFits && !memoryFits {
			reason = fmt.Sprintf("%s.cpu = %s and %s.memory = %s exceed all node allocatable resources (max CPU: %s, max memory: %s)",
				resourceType, podCPU.String(), resourceType, podMemory.String(),
				maxAvailableCPU.String(), maxAvailableMemory.String())
			suggestion = fmt.Sprintf("Lower %s.cpu to <= %s and %s.memory to <= %s, or add nodes with higher capacity",
				resourceType, maxAvailableCPU.String(), resourceType, maxAvailableMemory.String())
		} else if !cpuFits {
			reason = fmt.Sprintf("%s.cpu = %s exceeds all node allocatable.cpu (max: %s)",
				resourceType, podCPU.String(), maxAvailableCPU.String())
			suggestion = fmt.Sprintf("Lower %s.cpu to <= %s or add higher-CPU node",
				resourceType, maxAvailableCPU.String())
		} else {
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

func (a *Analyzer) EvaluateResourceConstraints(ctx context.Context) error {
	return nil
}
