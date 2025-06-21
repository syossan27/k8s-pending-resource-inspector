package internal

import (
	"context"
	"fmt"

	"github.com/syossan27/k8s-pending-resource-inspector/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Fetcher struct {
	clientset kubernetes.Interface
}

func NewFetcher(clientset kubernetes.Interface) *Fetcher {
	return &Fetcher{
		clientset: clientset,
	}
}

func NewFetcherFromConfig() (*Fetcher, error) {
	var config *rest.Config
	var err error

	config, err = rest.InClusterConfig()
	if err != nil {
		inClusterErr := err // Preserve the error from InClusterConfig
		config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes config: in-cluster error: %v, fallback error: %w", inClusterErr, err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	return NewFetcher(clientset), nil
}

func (f *Fetcher) FetchNodes(ctx context.Context) ([]types.NodeInfo, error) {
	nodes, err := f.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var nodeInfos []types.NodeInfo
	for _, node := range nodes.Items {
		nodeInfo := types.NodeInfo{
			Name:              node.Name,
			AllocatableCPU:    node.Status.Allocatable.Cpu().DeepCopy(),
			AllocatableMemory: node.Status.Allocatable.Memory().DeepCopy(),
			Taints:            node.Spec.Taints,
			Labels:            node.Labels,
		}
		nodeInfos = append(nodeInfos, nodeInfo)
	}

	return nodeInfos, nil
}

func (f *Fetcher) FetchPendingPods(ctx context.Context, namespace string) ([]types.PodInfo, error) {
	var listOptions metav1.ListOptions
	listOptions.FieldSelector = "status.phase=Pending"
	
	var pods *corev1.PodList
	var err error
	
	if namespace == "" {
		pods, err = f.clientset.CoreV1().Pods("").List(ctx, listOptions)
	} else {
		pods, err = f.clientset.CoreV1().Pods(namespace).List(ctx, listOptions)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to list pending pods: %w", err)
	}
	
	var podInfos []types.PodInfo
	for _, pod := range pods.Items {
		podInfo := f.parsePodResources(pod)
		podInfos = append(podInfos, podInfo)
	}
	
	return podInfos, nil
}

func (f *Fetcher) parsePodResources(pod corev1.Pod) types.PodInfo {
	var totalRequestsCPU, totalRequestsMemory resource.Quantity
	var totalLimitsCPU, totalLimitsMemory resource.Quantity
	
	for _, container := range pod.Spec.Containers {
		if container.Resources.Requests != nil {
			if cpu := container.Resources.Requests.Cpu(); cpu != nil {
				totalRequestsCPU.Add(*cpu)
			}
			if memory := container.Resources.Requests.Memory(); memory != nil {
				totalRequestsMemory.Add(*memory)
			}
		}
		
		if container.Resources.Limits != nil {
			if cpu := container.Resources.Limits.Cpu(); cpu != nil {
				totalLimitsCPU.Add(*cpu)
			}
			if memory := container.Resources.Limits.Memory(); memory != nil {
				totalLimitsMemory.Add(*memory)
			}
		}
	}
	
	var nodeAffinity *corev1.NodeAffinity
	if pod.Spec.Affinity != nil {
		nodeAffinity = pod.Spec.Affinity.NodeAffinity
	}
	
	return types.PodInfo{
		Name:           pod.Name,
		Namespace:      pod.Namespace,
		RequestsCPU:    totalRequestsCPU,
		RequestsMemory: totalRequestsMemory,
		LimitsCPU:      totalLimitsCPU,
		LimitsMemory:   totalLimitsMemory,
		NodeAffinity:   nodeAffinity,
		Tolerations:    pod.Spec.Tolerations,
	}
}
