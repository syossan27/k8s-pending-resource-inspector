//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syossan27/k8s-pending-resource-inspector/internal"
	"github.com/syossan27/k8s-pending-resource-inspector/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"gopkg.in/yaml.v3"
)

type IntegrationTestSuite struct {
	ctx context.Context
}

func TestIntegrationSuite(t *testing.T) {
	suite := &IntegrationTestSuite{
		ctx: context.Background(),
	}
	
	t.Run("EndToEndWorkflow", suite.TestEndToEndWorkflow)
	t.Run("AllOutputFormats", suite.TestAllOutputFormats)
	t.Run("DifferentScenarios", suite.TestDifferentScenarios)
	t.Run("ErrorConditions", suite.TestErrorConditions)
	t.Run("CLIFlagCombinations", suite.TestCLIFlagCombinations)
	t.Run("LargeClusterPerformance", suite.TestLargeClusterPerformance)
	t.Run("SlackNotificationIntegration", suite.TestSlackNotificationIntegration)
}

func (suite *IntegrationTestSuite) TestEndToEndWorkflow(t *testing.T) {
	tests := []struct {
		name          string
		nodes         []*corev1.Node
		pods          []*corev1.Pod
		namespace     string
		includeLimits bool
		expectedCount int
		expectedUnschedulable int
	}{
		{
			name: "schedulable pods scenario",
			nodes: []*corev1.Node{
				createNode("node1", "4", "8Gi", nil),
				createNode("node2", "2", "4Gi", nil),
			},
			pods: []*corev1.Pod{
				createPendingPod("schedulable-pod-1", "default", "100m", "128Mi", "", ""),
				createPendingPod("schedulable-pod-2", "default", "500m", "512Mi", "", ""),
			},
			namespace:     "",
			includeLimits: false,
			expectedCount: 2,
			expectedUnschedulable: 0,
		},
		{
			name: "unschedulable pods scenario",
			nodes: []*corev1.Node{
				createNode("small-node-1", "1", "2Gi", nil),
				createNode("small-node-2", "1", "2Gi", nil),
			},
			pods: []*corev1.Pod{
				createPendingPod("cpu-hungry-pod", "default", "2", "1Gi", "", ""),
				createPendingPod("memory-hungry-pod", "default", "500m", "4Gi", "", ""),
			},
			namespace:     "",
			includeLimits: false,
			expectedCount: 2,
			expectedUnschedulable: 2,
		},
		{
			name: "mixed scenario",
			nodes: []*corev1.Node{
				createNode("medium-node", "2", "4Gi", nil),
			},
			pods: []*corev1.Pod{
				createPendingPod("schedulable-pod", "default", "1", "2Gi", "", ""),
				createPendingPod("unschedulable-cpu-pod", "default", "4", "1Gi", "", ""),
				createPendingPod("unschedulable-memory-pod", "default", "500m", "8Gi", "", ""),
			},
			namespace:     "",
			includeLimits: false,
			expectedCount: 3,
			expectedUnschedulable: 2,
		},
		{
			name: "namespace specific scenario",
			nodes: []*corev1.Node{
				createNode("node1", "2", "4Gi", nil),
			},
			pods: []*corev1.Pod{
				createPendingPod("pod-in-target-ns", "target-namespace", "100m", "128Mi", "", ""),
				createPendingPod("pod-in-other-ns", "other-namespace", "100m", "128Mi", "", ""),
			},
			namespace:     "target-namespace",
			includeLimits: false,
			expectedCount: 1,
			expectedUnschedulable: 0,
		},
		{
			name: "limits scenario",
			nodes: []*corev1.Node{
				createNode("node1", "2", "4Gi", nil),
			},
			pods: []*corev1.Pod{
				createPendingPod("pod-with-limits", "default", "100m", "128Mi", "3", "1Gi"),
			},
			namespace:     "",
			includeLimits: true,
			expectedCount: 1,
			expectedUnschedulable: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			
			for _, node := range tt.nodes {
				_, err := clientset.CoreV1().Nodes().Create(suite.ctx, node, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			
			for _, pod := range tt.pods {
				_, err := clientset.CoreV1().Pods(pod.Namespace).Create(suite.ctx, pod, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			fetcher := internal.NewFetcher(clientset)
			analyzer := internal.NewAnalyzer(fetcher)

			results, err := analyzer.AnalyzePodSchedulability(suite.ctx, tt.namespace, tt.includeLimits)
			require.NoError(t, err)

			assert.Len(t, results, tt.expectedCount)

			unschedulableCount := 0
			for _, result := range results {
				if !result.IsSchedulable {
					unschedulableCount++
					assert.NotEmpty(t, result.Reason)
					assert.NotEmpty(t, result.Suggestion)
				}
			}
			assert.Equal(t, tt.expectedUnschedulable, unschedulableCount)
		})
	}
}

func (suite *IntegrationTestSuite) TestAllOutputFormats(t *testing.T) {
	nodes := []*corev1.Node{
		createNode("test-node", "2", "4Gi", nil),
	}
	pods := []*corev1.Pod{
		createPendingPod("schedulable-pod", "default", "100m", "128Mi", "", ""),
		createPendingPod("unschedulable-pod", "default", "4", "1Gi", "", ""),
	}

	clientset := fake.NewSimpleClientset()
	for _, node := range nodes {
		_, err := clientset.CoreV1().Nodes().Create(suite.ctx, node, metav1.CreateOptions{})
		require.NoError(t, err)
	}
	for _, pod := range pods {
		_, err := clientset.CoreV1().Pods(pod.Namespace).Create(suite.ctx, pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	fetcher := internal.NewFetcher(clientset)
	analyzer := internal.NewAnalyzer(fetcher)
	results, err := analyzer.AnalyzePodSchedulability(suite.ctx, "", false)
	require.NoError(t, err)

	t.Run("human readable output", func(t *testing.T) {
		var buf bytes.Buffer
		reporter := internal.NewReporter(&buf, internal.OutputFormatHuman)
		
		err := reporter.GenerateReport(suite.ctx, results, "test-cluster", len(nodes))
		require.NoError(t, err)
		
		output := buf.String()
		assert.Contains(t, output, "Found 2 pending pod(s) for analysis:")
		assert.Contains(t, output, "[✓] Pod: schedulable-pod - Schedulable")
		assert.Contains(t, output, "[✗] Pod: unschedulable-pod")
		assert.Contains(t, output, "→ Reason:")
		assert.Contains(t, output, "→ Suggested:")
	})

	t.Run("JSON output", func(t *testing.T) {
		var buf bytes.Buffer
		reporter := internal.NewReporter(&buf, internal.OutputFormatJSON)
		
		err := reporter.GenerateReport(suite.ctx, results, "test-cluster", len(nodes))
		require.NoError(t, err)
		
		var analysis types.ClusterAnalysis
		err = json.Unmarshal(buf.Bytes(), &analysis)
		require.NoError(t, err)
		
		assert.Equal(t, "test-cluster", analysis.ClusterName)
		assert.Equal(t, len(nodes), analysis.TotalNodes)
		assert.Equal(t, 2, analysis.TotalPendingPods)
		assert.Len(t, analysis.UnschedulablePods, 1)
		assert.Equal(t, "unschedulable-pod", analysis.UnschedulablePods[0].Pod.Name)
		assert.NotZero(t, analysis.Timestamp)
	})

	t.Run("YAML output", func(t *testing.T) {
		var buf bytes.Buffer
		reporter := internal.NewReporter(&buf, internal.OutputFormatYAML)
		
		err := reporter.GenerateReport(suite.ctx, results, "test-cluster", len(nodes))
		require.NoError(t, err)
		
		var analysis types.ClusterAnalysis
		err = yaml.Unmarshal(buf.Bytes(), &analysis)
		require.NoError(t, err)
		
		assert.Equal(t, "test-cluster", analysis.ClusterName)
		assert.Equal(t, len(nodes), analysis.TotalNodes)
		assert.Equal(t, 2, analysis.TotalPendingPods)
		assert.Len(t, analysis.UnschedulablePods, 1)
	})
}

func (suite *IntegrationTestSuite) TestDifferentScenarios(t *testing.T) {
	t.Run("no pods scenario", func(t *testing.T) {
		nodes := []*corev1.Node{
			createNode("node1", "2", "4Gi", nil),
		}
		
		clientset := fake.NewSimpleClientset()
		for _, node := range nodes {
			_, err := clientset.CoreV1().Nodes().Create(suite.ctx, node, metav1.CreateOptions{})
			require.NoError(t, err)
		}

		fetcher := internal.NewFetcher(clientset)
		analyzer := internal.NewAnalyzer(fetcher)
		results, err := analyzer.AnalyzePodSchedulability(suite.ctx, "", false)
		require.NoError(t, err)
		assert.Empty(t, results)

		var buf bytes.Buffer
		reporter := internal.NewReporter(&buf, internal.OutputFormatHuman)
		err = reporter.GenerateReport(suite.ctx, results, "test-cluster", len(nodes))
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "No pending pods found in the specified scope.")
	})

	t.Run("different cluster configurations", func(t *testing.T) {
		testCases := []struct {
			name  string
			nodes []*corev1.Node
		}{
			{
				name: "small cluster",
				nodes: []*corev1.Node{
					createNode("small-node", "1", "2Gi", nil),
				},
			},
			{
				name: "medium cluster",
				nodes: []*corev1.Node{
					createNode("medium-node-1", "4", "8Gi", nil),
					createNode("medium-node-2", "4", "8Gi", nil),
				},
			},
			{
				name: "heterogeneous cluster",
				nodes: []*corev1.Node{
					createNode("cpu-optimized", "8", "4Gi", nil),
					createNode("memory-optimized", "2", "32Gi", nil),
					createNode("balanced", "4", "16Gi", nil),
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				clientset := fake.NewSimpleClientset()
				for _, node := range tc.nodes {
					_, err := clientset.CoreV1().Nodes().Create(suite.ctx, node, metav1.CreateOptions{})
					require.NoError(t, err)
				}

				pod := createPendingPod("test-pod", "default", "2", "4Gi", "", "")
				_, err := clientset.CoreV1().Pods(pod.Namespace).Create(suite.ctx, pod, metav1.CreateOptions{})
				require.NoError(t, err)

				fetcher := internal.NewFetcher(clientset)
				analyzer := internal.NewAnalyzer(fetcher)
				results, err := analyzer.AnalyzePodSchedulability(suite.ctx, "", false)
				require.NoError(t, err)
				assert.Len(t, results, 1)
			})
		}
	})

	t.Run("node with taints", func(t *testing.T) {
		taints := []corev1.Taint{
			{
				Key:    "node-role.kubernetes.io/master",
				Effect: corev1.TaintEffectNoSchedule,
			},
		}
		
		nodes := []*corev1.Node{
			createNode("tainted-node", "4", "8Gi", taints),
		}
		pods := []*corev1.Pod{
			createPendingPod("test-pod", "default", "1", "2Gi", "", ""),
		}

		clientset := fake.NewSimpleClientset()
		for _, node := range nodes {
			_, err := clientset.CoreV1().Nodes().Create(suite.ctx, node, metav1.CreateOptions{})
			require.NoError(t, err)
		}
		for _, pod := range pods {
			_, err := clientset.CoreV1().Pods(pod.Namespace).Create(suite.ctx, pod, metav1.CreateOptions{})
			require.NoError(t, err)
		}

		fetcher := internal.NewFetcher(clientset)
		analyzer := internal.NewAnalyzer(fetcher)
		results, err := analyzer.AnalyzePodSchedulability(suite.ctx, "", false)
		require.NoError(t, err)
		assert.Len(t, results, 1)
	})
}

func (suite *IntegrationTestSuite) TestErrorConditions(t *testing.T) {
	t.Run("empty cluster", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		
		pod := createPendingPod("test-pod", "default", "1", "2Gi", "", "")
		_, err := clientset.CoreV1().Pods(pod.Namespace).Create(suite.ctx, pod, metav1.CreateOptions{})
		require.NoError(t, err)

		fetcher := internal.NewFetcher(clientset)
		analyzer := internal.NewAnalyzer(fetcher)
		results, err := analyzer.AnalyzePodSchedulability(suite.ctx, "", false)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.False(t, results[0].IsSchedulable)
		assert.Contains(t, results[0].Reason, "exceed all node allocatable resources")
	})

	t.Run("unsupported output format", func(t *testing.T) {
		var buf bytes.Buffer
		reporter := internal.NewReporter(&buf, internal.OutputFormat("unsupported"))
		
		results := []types.AnalysisResult{
			{
				Pod: types.PodInfo{Name: "test-pod", Namespace: "default"},
				IsSchedulable: false,
			},
		}
		err := reporter.GenerateReport(suite.ctx, results, "test-cluster", 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported output format: unsupported")
	})
}

func (suite *IntegrationTestSuite) TestCLIFlagCombinations(t *testing.T) {
	nodes := []*corev1.Node{
		createNode("node1", "2", "4Gi", nil),
	}
	pods := []*corev1.Pod{
		createPendingPod("pod-ns1", "namespace1", "100m", "128Mi", "500m", "256Mi"),
		createPendingPod("pod-ns2", "namespace2", "100m", "128Mi", "3", "1Gi"),
		createPendingPod("pod-default", "default", "100m", "128Mi", "", ""),
	}

	clientset := fake.NewSimpleClientset()
	for _, node := range nodes {
		_, err := clientset.CoreV1().Nodes().Create(suite.ctx, node, metav1.CreateOptions{})
		require.NoError(t, err)
	}
	for _, pod := range pods {
		_, err := clientset.CoreV1().Pods(pod.Namespace).Create(suite.ctx, pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	fetcher := internal.NewFetcher(clientset)
	analyzer := internal.NewAnalyzer(fetcher)

	testCases := []struct {
		name          string
		namespace     string
		includeLimits bool
		outputFormat  internal.OutputFormat
		expectedCount int
	}{
		{
			name:          "cluster-wide with requests",
			namespace:     "",
			includeLimits: false,
			outputFormat:  internal.OutputFormatHuman,
			expectedCount: 3,
		},
		{
			name:          "specific namespace with requests",
			namespace:     "namespace1",
			includeLimits: false,
			outputFormat:  internal.OutputFormatJSON,
			expectedCount: 1,
		},
		{
			name:          "cluster-wide with limits",
			namespace:     "",
			includeLimits: true,
			outputFormat:  internal.OutputFormatYAML,
			expectedCount: 3,
		},
		{
			name:          "specific namespace with limits",
			namespace:     "namespace2",
			includeLimits: true,
			outputFormat:  internal.OutputFormatHuman,
			expectedCount: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := analyzer.AnalyzePodSchedulability(suite.ctx, tc.namespace, tc.includeLimits)
			require.NoError(t, err)
			assert.Len(t, results, tc.expectedCount)

			var buf bytes.Buffer
			reporter := internal.NewReporter(&buf, tc.outputFormat)
			err = reporter.GenerateReport(suite.ctx, results, "test-cluster", len(nodes))
			require.NoError(t, err)
			assert.NotEmpty(t, buf.String())
		})
	}
}

func (suite *IntegrationTestSuite) TestLargeClusterPerformance(t *testing.T) {
	const nodeCount = 100
	const podCount = 50

	nodes := make([]*corev1.Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = createNode(
			fmt.Sprintf("node-%d", i),
			"2",
			"4Gi",
			nil,
		)
	}

	pods := make([]*corev1.Pod, podCount)
	for i := 0; i < podCount; i++ {
		pods[i] = createPendingPod(
			fmt.Sprintf("pod-%d", i),
			"default",
			"100m",
			"128Mi",
			"",
			"",
		)
	}

	clientset := fake.NewSimpleClientset()
	for _, node := range nodes {
		_, err := clientset.CoreV1().Nodes().Create(suite.ctx, node, metav1.CreateOptions{})
		require.NoError(t, err)
	}
	for _, pod := range pods {
		_, err := clientset.CoreV1().Pods(pod.Namespace).Create(suite.ctx, pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	fetcher := internal.NewFetcher(clientset)
	analyzer := internal.NewAnalyzer(fetcher)

	results, err := analyzer.AnalyzePodSchedulability(suite.ctx, "", false)
	require.NoError(t, err)
	assert.Len(t, results, podCount)

	for _, result := range results {
		assert.True(t, result.IsSchedulable)
	}

	var buf bytes.Buffer
	reporter := internal.NewReporter(&buf, internal.OutputFormatJSON)
	err = reporter.GenerateReport(suite.ctx, results, "large-test-cluster", len(nodes))
	require.NoError(t, err)

	var analysis types.ClusterAnalysis
	err = json.Unmarshal(buf.Bytes(), &analysis)
	require.NoError(t, err)
	assert.Equal(t, nodeCount, analysis.TotalNodes)
	assert.Equal(t, podCount, analysis.TotalPendingPods)
}

func (suite *IntegrationTestSuite) TestSlackNotificationIntegration(t *testing.T) {
	nodes := []*corev1.Node{
		createNode("node1", "1", "2Gi", nil),
	}
	pods := []*corev1.Pod{
		createPendingPod("unschedulable-pod", "default", "2", "4Gi", "", ""),
	}

	clientset := fake.NewSimpleClientset()
	for _, node := range nodes {
		_, err := clientset.CoreV1().Nodes().Create(suite.ctx, node, metav1.CreateOptions{})
		require.NoError(t, err)
	}
	for _, pod := range pods {
		_, err := clientset.CoreV1().Pods(pod.Namespace).Create(suite.ctx, pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	fetcher := internal.NewFetcher(clientset)
	analyzer := internal.NewAnalyzer(fetcher)
	results, err := analyzer.AnalyzePodSchedulability(suite.ctx, "", false)
	require.NoError(t, err)

	var buf bytes.Buffer
	reporter := internal.NewReporter(&buf, internal.OutputFormatJSON)
	
	err = reporter.SendSlackNotification(suite.ctx, "https://hooks.slack.com/services/test", results)
	assert.NoError(t, err)
}

func createNode(name, cpu, memory string, taints []corev1.Taint) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"kubernetes.io/os": "linux",
			},
		},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpu),
				corev1.ResourceMemory: resource.MustParse(memory),
			},
		},
		Spec: corev1.NodeSpec{
			Taints: taints,
		},
	}
}

func createPendingPod(name, namespace, requestCPU, requestMemory, limitCPU, limitMemory string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "container1",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{},
						Limits:   corev1.ResourceList{},
					},
				},
			},
		},
	}

	if requestCPU != "" {
		pod.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU] = resource.MustParse(requestCPU)
	}
	if requestMemory != "" {
		pod.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory] = resource.MustParse(requestMemory)
	}
	if limitCPU != "" {
		pod.Spec.Containers[0].Resources.Limits[corev1.ResourceCPU] = resource.MustParse(limitCPU)
	}
	if limitMemory != "" {
		pod.Spec.Containers[0].Resources.Limits[corev1.ResourceMemory] = resource.MustParse(limitMemory)
	}

	return pod
}
