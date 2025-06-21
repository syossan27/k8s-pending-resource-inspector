package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syossan27/k8s-pending-resource-inspector/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)



func TestNewFetcher(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	fetcher := NewFetcher(clientset)
	
	assert.NotNil(t, fetcher)
	assert.Equal(t, clientset, fetcher.clientset)
}

func TestFetchNodes_Success(t *testing.T) {
	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
			Labels: map[string]string{
				"kubernetes.io/os": "linux",
			},
		},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("4Gi"),
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    "node-role.kubernetes.io/master",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
		},
	}
	
	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2",
			Labels: map[string]string{
				"kubernetes.io/os": "linux",
			},
		},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
		},
	}
	
	clientset := fake.NewSimpleClientset(node1, node2)
	fetcher := NewFetcher(clientset)
	ctx := context.Background()
	
	nodes, err := fetcher.FetchNodes(ctx)
	
	require.NoError(t, err)
	assert.Len(t, nodes, 2)
	
	assert.Equal(t, "node1", nodes[0].Name)
	assert.True(t, resource.MustParse("2").Equal(nodes[0].AllocatableCPU))
	assert.True(t, resource.MustParse("4Gi").Equal(nodes[0].AllocatableMemory))
	assert.Len(t, nodes[0].Taints, 1)
	assert.Equal(t, "node-role.kubernetes.io/master", nodes[0].Taints[0].Key)
	
	assert.Equal(t, "node2", nodes[1].Name)
	assert.True(t, resource.MustParse("4").Equal(nodes[1].AllocatableCPU))
	assert.True(t, resource.MustParse("8Gi").Equal(nodes[1].AllocatableMemory))
	assert.Len(t, nodes[1].Taints, 0)
}

func TestFetchPendingPods_ClusterWide(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pending-pod-1",
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "container1",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "kubernetes.io/os",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"linux"},
									},
								},
							},
						},
					},
				},
			},
			Tolerations: []corev1.Toleration{
				{
					Key:      "node.kubernetes.io/not-ready",
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoExecute,
				},
			},
		},
	}
	
	clientset := fake.NewSimpleClientset(pod)
	fetcher := NewFetcher(clientset)
	ctx := context.Background()
	
	pods, err := fetcher.FetchPendingPods(ctx, "")
	
	require.NoError(t, err)
	assert.Len(t, pods, 1)
	
	podInfo := pods[0]
	assert.Equal(t, "pending-pod-1", podInfo.Name)
	assert.Equal(t, "default", podInfo.Namespace)
	assert.True(t, resource.MustParse("100m").Equal(podInfo.RequestsCPU))
	assert.True(t, resource.MustParse("128Mi").Equal(podInfo.RequestsMemory))
	assert.True(t, resource.MustParse("200m").Equal(podInfo.LimitsCPU))
	assert.True(t, resource.MustParse("256Mi").Equal(podInfo.LimitsMemory))
	assert.NotNil(t, podInfo.NodeAffinity)
	assert.Len(t, podInfo.Tolerations, 1)
}

func TestFetchPendingPods_SpecificNamespace(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pending-pod-2",
			Namespace: "my-namespace",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "container1",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
		},
	}
	
	clientset := fake.NewSimpleClientset(pod)
	fetcher := NewFetcher(clientset)
	ctx := context.Background()
	
	pods, err := fetcher.FetchPendingPods(ctx, "my-namespace")
	
	require.NoError(t, err)
	assert.Len(t, pods, 1)
	
	podInfo := pods[0]
	assert.Equal(t, "pending-pod-2", podInfo.Name)
	assert.Equal(t, "my-namespace", podInfo.Namespace)
	assert.True(t, resource.MustParse("500m").Equal(podInfo.RequestsCPU))
	assert.True(t, resource.MustParse("1Gi").Equal(podInfo.RequestsMemory))
}

func TestParsePodResources(t *testing.T) {
	tests := []struct {
		name     string
		pod      corev1.Pod
		expected types.PodInfo
	}{
		{
			name: "pod with requests and limits",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				},
			},
			expected: types.PodInfo{
				Name:           "test-pod",
				Namespace:      "test-ns",
				RequestsCPU:    resource.MustParse("100m"),
				RequestsMemory: resource.MustParse("128Mi"),
				LimitsCPU:      resource.MustParse("200m"),
				LimitsMemory:   resource.MustParse("256Mi"),
			},
		},
		{
			name: "pod with multiple containers",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-container-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				},
			},
			expected: types.PodInfo{
				Name:           "multi-container-pod",
				Namespace:      "default",
				RequestsCPU:    resource.MustParse("300m"),
				RequestsMemory: resource.MustParse("384Mi"),
			},
		},
		{
			name: "pod with no resources",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-resources-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{},
						},
					},
				},
			},
			expected: types.PodInfo{
				Name:      "no-resources-pod",
				Namespace: "default",
			},
		},
	}
	
	fetcher := NewFetcher(fake.NewSimpleClientset())
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fetcher.parsePodResources(tt.pod)
			
			assert.Equal(t, tt.expected.Name, result.Name)
			assert.Equal(t, tt.expected.Namespace, result.Namespace)
			assert.True(t, tt.expected.RequestsCPU.Equal(result.RequestsCPU))
			assert.True(t, tt.expected.RequestsMemory.Equal(result.RequestsMemory))
			assert.True(t, tt.expected.LimitsCPU.Equal(result.LimitsCPU))
			assert.True(t, tt.expected.LimitsMemory.Equal(result.LimitsMemory))
		})
	}
}
