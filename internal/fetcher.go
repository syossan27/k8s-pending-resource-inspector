package internal

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/syossan27/k8s-pending-resource-inspector/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Fetcher provides methods to fetch Kubernetes resources from a cluster.
// It wraps a Kubernetes clientset to retrieve node and pod information
// for resource inspection and analysis.
type Fetcher struct {
	clientset kubernetes.Interface
}

// NewFetcher creates a new Fetcher instance with the provided Kubernetes clientset.
// The clientset is used to interact with the Kubernetes API server.
//
// Parameters:
//   - clientset: A Kubernetes client interface for API operations
//
// Returns:
//   - *Fetcher: A new Fetcher instance
func NewFetcher(clientset kubernetes.Interface) *Fetcher {
	return &Fetcher{
		clientset: clientset,
	}
}

// NewFetcherFromConfig creates a new Fetcher instance using automatic Kubernetes configuration.
// It first attempts to use in-cluster configuration (when running inside a pod),
// then falls back to the default kubeconfig file (~/.kube/config) if in-cluster config fails.
//
// Returns:
//   - *Fetcher: A new Fetcher instance configured with the detected Kubernetes client
//   - error: An error if both in-cluster and kubeconfig configurations fail
func NewFetcherFromConfig() (*Fetcher, error) {
	var config *rest.Config
	var err error

	logrus.Debug("Attempting to create Kubernetes configuration")

	config, err = rest.InClusterConfig()
	if err != nil {
		logrus.Debug("In-cluster config failed, trying kubeconfig file")
		inClusterErr := err
		config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"in_cluster_error": inClusterErr.Error(),
				"kubeconfig_error": err.Error(),
			}).Error("Failed to create Kubernetes configuration")
			return nil, fmt.Errorf("failed to create kubernetes config: in-cluster error: %v, fallback error: %w",
				inClusterErr, err)
		}
		logrus.Debug("Successfully loaded kubeconfig file")
	} else {
		logrus.Debug("Successfully loaded in-cluster configuration")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.WithError(err).Error("Failed to create Kubernetes clientset")
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	logrus.Debug("Successfully created Kubernetes clientset")
	return NewFetcher(clientset), nil
}

// FetchNodes retrieves information about all nodes in the Kubernetes cluster.
// It fetches node details including allocatable resources, taints, and labels
// which are essential for pod scheduling analysis.
//
// Parameters:
//   - ctx: Context for the API request, used for cancellation and timeout
//
// Returns:
//   - []types.NodeInfo: A slice of NodeInfo containing node details
//   - error: An error if the node listing operation fails
func (f *Fetcher) FetchNodes(ctx context.Context) ([]types.NodeInfo, error) {
	logrus.Debug("Fetching cluster nodes")

	nodes, err := f.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logrus.WithError(err).Error("Failed to list nodes from Kubernetes API")
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	logrus.WithField("nodes_count", len(nodes.Items)).Info("Successfully fetched cluster nodes")

	nodeInfos := make([]types.NodeInfo, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		nodeInfo := types.NodeInfo{
			Name:              node.Name,
			AllocatableCPU:    node.Status.Allocatable.Cpu().DeepCopy(),
			AllocatableMemory: node.Status.Allocatable.Memory().DeepCopy(),
			Taints:            node.Spec.Taints,
			Labels:            node.Labels,
		}

		logrus.WithFields(logrus.Fields{
			"node_name":          node.Name,
			"allocatable_cpu":    nodeInfo.AllocatableCPU.String(),
			"allocatable_memory": nodeInfo.AllocatableMemory.String(),
			"taints_count":       len(node.Spec.Taints),
		}).Debug("Processed node information")

		nodeInfos = append(nodeInfos, nodeInfo)
	}

	return nodeInfos, nil
}

// FetchPendingPods retrieves all pods in Pending state from the specified namespace or cluster-wide.
// Pending pods are those that have not been scheduled to a node yet, often due to
// resource constraints, node affinity rules, or taints/tolerations mismatches.
//
// Parameters:
//   - ctx: Context for the API request, used for cancellation and timeout
//   - namespace: Target namespace to search for pending pods. If empty, searches cluster-wide
//
// Returns:
//   - []types.PodInfo: A slice of PodInfo containing pending pod details and resource requirements
//   - error: An error if the pod listing operation fails
func (f *Fetcher) FetchPendingPods(ctx context.Context, namespace string) ([]types.PodInfo, error) {
	var listOptions metav1.ListOptions
	listOptions.FieldSelector = "status.phase=Pending"

	var pods *corev1.PodList
	var err error

	if namespace == "" {
		logrus.Debug("Fetching pending pods cluster-wide")
		pods, err = f.clientset.CoreV1().Pods("").List(ctx, listOptions)
	} else {
		logrus.WithField("namespace", namespace).Debug("Fetching pending pods from specific namespace")
		pods, err = f.clientset.CoreV1().Pods(namespace).List(ctx, listOptions)
	}

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"namespace": namespace,
			"error":     err.Error(),
		}).Error("Failed to list pending pods from Kubernetes API")
		return nil, fmt.Errorf("failed to list pending pods: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"namespace":          namespace,
		"pending_pods_count": len(pods.Items),
	}).Info("Successfully fetched pending pods")

	podInfos := make([]types.PodInfo, 0, len(pods.Items))
	for _, pod := range pods.Items {
		podInfo := f.parsePodResources(pod)

		logrus.WithFields(logrus.Fields{
			"pod_name":        pod.Name,
			"pod_namespace":   pod.Namespace,
			"requests_cpu":    podInfo.RequestsCPU.String(),
			"requests_memory": podInfo.RequestsMemory.String(),
		}).Debug("Processed pending pod information")

		podInfos = append(podInfos, podInfo)
	}

	return podInfos, nil
}

// parsePodResources extracts and aggregates resource information from a pod specification.
// It calculates total CPU and memory requests/limits across all containers in the pod,
// and extracts scheduling constraints like node affinity and tolerations.
//
// Parameters:
//   - pod: The Kubernetes pod object to parse
//
// Returns:
//   - types.PodInfo: Structured pod information including aggregated resources and scheduling constraints
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
