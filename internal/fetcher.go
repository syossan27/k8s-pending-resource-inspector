package internal

import (
	"context"
	"fmt"

	"github.com/syossan27/k8s-pending-resource-inspector/pkg/types"
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

func (f *Fetcher) FetchPendingPods(ctx context.Context, namespace string) error {
	return nil
}
