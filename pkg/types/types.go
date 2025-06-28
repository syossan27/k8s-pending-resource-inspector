package types

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

type LogFormat string

const (
	LogFormatJSON LogFormat = "json"
	LogFormatText LogFormat = "text"
)

type NodeInfo struct {
	Name              string            `json:"name" yaml:"name"`
	AllocatableCPU    resource.Quantity `json:"allocatableCpu" yaml:"allocatableCpu"`
	AllocatableMemory resource.Quantity `json:"allocatableMemory" yaml:"allocatableMemory"`
	Taints            []corev1.Taint    `json:"taints,omitempty" yaml:"taints,omitempty"`
	Labels            map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

type PodInfo struct {
	Name           string               `json:"name" yaml:"name"`
	Namespace      string               `json:"namespace" yaml:"namespace"`
	RequestsCPU    resource.Quantity    `json:"requestsCpu" yaml:"requestsCpu"`
	RequestsMemory resource.Quantity    `json:"requestsMemory" yaml:"requestsMemory"`
	LimitsCPU      resource.Quantity    `json:"limitsCpu,omitempty" yaml:"limitsCpu,omitempty"`
	LimitsMemory   resource.Quantity    `json:"limitsMemory,omitempty" yaml:"limitsMemory,omitempty"`
	NodeAffinity   *corev1.NodeAffinity `json:"nodeAffinity,omitempty" yaml:"nodeAffinity,omitempty"`
	Tolerations    []corev1.Toleration  `json:"tolerations,omitempty" yaml:"tolerations,omitempty"`
}

type AnalysisResult struct {
	Pod                PodInfo           `json:"pod" yaml:"pod"`
	IsSchedulable      bool              `json:"isSchedulable" yaml:"isSchedulable"`
	Reason             string            `json:"reason,omitempty" yaml:"reason,omitempty"`
	Suggestion         string            `json:"suggestion,omitempty" yaml:"suggestion,omitempty"`
	MaxAvailableCPU    resource.Quantity `json:"maxAvailableCpu" yaml:"maxAvailableCpu"`
	MaxAvailableMemory resource.Quantity `json:"maxAvailableMemory" yaml:"maxAvailableMemory"`
}

type ClusterAnalysis struct {
	Timestamp         time.Time        `json:"timestamp" yaml:"timestamp"`
	ClusterName       string           `json:"clusterName" yaml:"clusterName"`
	TotalNodes        int              `json:"totalNodes" yaml:"totalNodes"`
	TotalPendingPods  int              `json:"totalPendingPods" yaml:"totalPendingPods"`
	UnschedulablePods []AnalysisResult `json:"unschedulablePods" yaml:"unschedulablePods"`
	Summary           string           `json:"summary" yaml:"summary"`
}
