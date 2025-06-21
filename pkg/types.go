package types

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)


type NodeInfo struct {
	Name              string
	AllocatableCPU    resource.Quantity
	AllocatableMemory resource.Quantity
	Taints            []corev1.Taint
	Labels            map[string]string
}


type PodInfo struct {
	Name         string
	Namespace    string
	RequestsCPU  resource.Quantity
	RequestsMemory resource.Quantity
	LimitsCPU    resource.Quantity
	LimitsMemory resource.Quantity
	NodeAffinity *corev1.NodeAffinity
	Tolerations  []corev1.Toleration
}


type AnalysisResult struct {
	Pod                PodInfo
	IsSchedulable      bool
	Reason             string
	Suggestion         string
	MaxAvailableCPU    resource.Quantity
	MaxAvailableMemory resource.Quantity
}


type ClusterAnalysis struct {
	Timestamp        time.Time
	ClusterName      string
	TotalNodes       int
	TotalPendingPods int
	UnschedulablePods []AnalysisResult
	Summary          string
}
