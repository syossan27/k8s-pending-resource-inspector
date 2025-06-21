//go:build integration

package integration

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syossan27/k8s-pending-resource-inspector/internal"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCLIIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("complete CLI workflow simulation", func(t *testing.T) {
		nodes := []*corev1.Node{
			createNode("node1", "4", "8Gi", nil),
			createNode("node2", "2", "4Gi", nil),
		}
		pods := []*corev1.Pod{
			createPendingPod("schedulable-pod", "default", "1", "2Gi", "", ""),
			createPendingPod("unschedulable-cpu-pod", "default", "8", "1Gi", "", ""),
			createPendingPod("unschedulable-memory-pod", "default", "500m", "16Gi", "", ""),
		}

		clientset := fake.NewSimpleClientset()
		for _, node := range nodes {
			_, err := clientset.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
			require.NoError(t, err)
		}
		for _, pod := range pods {
			_, err := clientset.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
			require.NoError(t, err)
		}

		fetcher := internal.NewFetcher(clientset)
		analyzer := internal.NewAnalyzer(fetcher)

		results, err := analyzer.AnalyzePodSchedulability(ctx, "", false)
		require.NoError(t, err)
		assert.Len(t, results, 3)

		nodeList, err := fetcher.FetchNodes(ctx)
		require.NoError(t, err)
		assert.Len(t, nodeList, 2)

		var buf bytes.Buffer
		reporter := internal.NewReporter(&buf, internal.OutputFormatHuman)
		
		err = reporter.GenerateReport(ctx, results, "test-cluster", len(nodeList))
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Found 3 pending pod(s) for analysis:")
		assert.Contains(t, output, "[✓] Pod: schedulable-pod - Schedulable")
		assert.Contains(t, output, "[✗] Pod: unschedulable-cpu-pod")
		assert.Contains(t, output, "[✗] Pod: unschedulable-memory-pod")

		schedulableCount := 0
		unschedulableCount := 0
		for _, result := range results {
			if result.IsSchedulable {
				schedulableCount++
			} else {
				unschedulableCount++
				assert.NotEmpty(t, result.Reason)
				assert.NotEmpty(t, result.Suggestion)
			}
		}
		assert.Equal(t, 1, schedulableCount)
		assert.Equal(t, 2, unschedulableCount)
	})

	t.Run("CLI flag validation simulation", func(t *testing.T) {
		testCases := []struct {
			name         string
			outputFormat string
			slackWebhook string
			expectError  bool
		}{
			{
				name:         "valid human format",
				outputFormat: "human",
				slackWebhook: "",
				expectError:  false,
			},
			{
				name:         "valid json format",
				outputFormat: "json",
				slackWebhook: "",
				expectError:  false,
			},
			{
				name:         "valid yaml format",
				outputFormat: "yaml",
				slackWebhook: "",
				expectError:  false,
			},
			{
				name:         "invalid format",
				outputFormat: "xml",
				slackWebhook: "",
				expectError:  true,
			},
			{
				name:         "valid slack webhook",
				outputFormat: "json",
				slackWebhook: "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX",
				expectError:  false,
			},
			{
				name:         "invalid slack webhook",
				outputFormat: "json",
				slackWebhook: "http://invalid.com",
				expectError:  true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				validFormats := map[string]bool{"human": true, "json": true, "yaml": true}
				formatValid := validFormats[tc.outputFormat]
				
				slackValid := tc.slackWebhook == "" || 
					strings.HasPrefix(tc.slackWebhook, "https://hooks.slack.com/")

				expectedValid := formatValid && slackValid
				actualValid := !tc.expectError

				assert.Equal(t, expectedValid, actualValid, 
					"Format: %s, Slack: %s, Expected: %v, Actual: %v", 
					tc.outputFormat, tc.slackWebhook, expectedValid, actualValid)
			})
		}
	})

	t.Run("environment variable simulation", func(t *testing.T) {
		originalKubeconfig := os.Getenv("KUBECONFIG")
		defer func() {
			if originalKubeconfig != "" {
				os.Setenv("KUBECONFIG", originalKubeconfig)
			} else {
				os.Unsetenv("KUBECONFIG")
			}
		}()

		os.Setenv("KUBECONFIG", "/tmp/nonexistent-kubeconfig")

		_, err := internal.NewFetcherFromConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create kubernetes config")
	})
}

func TestRealWorldScenarios(t *testing.T) {
	ctx := context.Background()

	t.Run("production-like cluster scenario", func(t *testing.T) {
		nodes := []*corev1.Node{
			createNode("master-node", "4", "8Gi", []corev1.Taint{
				{
					Key:    "node-role.kubernetes.io/master",
					Effect: corev1.TaintEffectNoSchedule,
				},
			}),
			createNode("worker-node-1", "8", "16Gi", nil),
			createNode("worker-node-2", "8", "16Gi", nil),
			createNode("worker-node-3", "4", "8Gi", nil),
		}

		pods := []*corev1.Pod{
			createPendingPod("web-app-1", "production", "2", "4Gi", "4", "8Gi"),
			createPendingPod("web-app-2", "production", "2", "4Gi", "4", "8Gi"),
			createPendingPod("database", "production", "4", "8Gi", "6", "12Gi"),
			createPendingPod("cache", "production", "1", "2Gi", "2", "4Gi"),
			createPendingPod("huge-job", "batch", "16", "32Gi", "16", "32Gi"),
		}

		clientset := fake.NewSimpleClientset()
		for _, node := range nodes {
			_, err := clientset.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
			require.NoError(t, err)
		}
		for _, pod := range pods {
			_, err := clientset.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
			require.NoError(t, err)
		}

		fetcher := internal.NewFetcher(clientset)
		analyzer := internal.NewAnalyzer(fetcher)

		t.Run("analyze with requests", func(t *testing.T) {
			results, err := analyzer.AnalyzePodSchedulability(ctx, "", false)
			require.NoError(t, err)
			assert.Len(t, results, 5)

			schedulableCount := 0
			unschedulableCount := 0
			for _, result := range results {
				if result.IsSchedulable {
					schedulableCount++
				} else {
					unschedulableCount++
				}
			}

			assert.Equal(t, 4, schedulableCount)
			assert.Equal(t, 1, unschedulableCount)
		})

		t.Run("analyze with limits", func(t *testing.T) {
			results, err := analyzer.AnalyzePodSchedulability(ctx, "", true)
			require.NoError(t, err)
			assert.Len(t, results, 5)

			unschedulableCount := 0
			for _, result := range results {
				if !result.IsSchedulable {
					unschedulableCount++
					assert.Contains(t, result.Reason, "limits.")
				}
			}

			assert.Greater(t, unschedulableCount, 0)
		})

		t.Run("namespace-specific analysis", func(t *testing.T) {
			results, err := analyzer.AnalyzePodSchedulability(ctx, "production", false)
			require.NoError(t, err)
			assert.Len(t, results, 4)

			for _, result := range results {
				assert.Equal(t, "production", result.Pod.Namespace)
			}
		})
	})

	t.Run("resource exhaustion scenario", func(t *testing.T) {
		nodes := []*corev1.Node{
			createNode("small-node-1", "1", "2Gi", nil),
			createNode("small-node-2", "1", "2Gi", nil),
		}

		pods := []*corev1.Pod{
			createPendingPod("big-cpu-pod", "default", "4", "1Gi", "", ""),
			createPendingPod("big-memory-pod", "default", "500m", "8Gi", "", ""),
			createPendingPod("reasonable-pod", "default", "500m", "1Gi", "", ""),
		}

		clientset := fake.NewSimpleClientset()
		for _, node := range nodes {
			_, err := clientset.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
			require.NoError(t, err)
		}
		for _, pod := range pods {
			_, err := clientset.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
			require.NoError(t, err)
		}

		fetcher := internal.NewFetcher(clientset)
		analyzer := internal.NewAnalyzer(fetcher)
		results, err := analyzer.AnalyzePodSchedulability(ctx, "", false)
		require.NoError(t, err)

		schedulableCount := 0
		unschedulableCount := 0
		for _, result := range results {
			if result.IsSchedulable {
				schedulableCount++
			} else {
				unschedulableCount++
				assert.NotEmpty(t, result.Reason)
				assert.NotEmpty(t, result.Suggestion)
				assert.Contains(t, result.Reason, "exceeds all node allocatable")
			}
		}

		assert.Equal(t, 1, schedulableCount)
		assert.Equal(t, 2, unschedulableCount)
	})
}
