package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syossan27/k8s-pending-resource-inspector/pkg/types"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewReporter(&buf, OutputFormatJSON)

	results := []types.AnalysisResult{
		{
			Pod: types.PodInfo{
				Name:           "test-pod",
				Namespace:      "default",
				RequestsCPU:    resource.MustParse("100m"),
				RequestsMemory: resource.MustParse("128Mi"),
			},
			IsSchedulable:      false,
			Reason:             "Insufficient CPU",
			Suggestion:         "Add more nodes",
			MaxAvailableCPU:    resource.MustParse("50m"),
			MaxAvailableMemory: resource.MustParse("256Mi"),
		},
	}

	err := reporter.GenerateReport(context.Background(), results, "test-cluster", 3)
	require.NoError(t, err, "Failed to generate JSON report")

	var analysis types.ClusterAnalysis
	err = json.Unmarshal(buf.Bytes(), &analysis)
	require.NoError(t, err, "Generated output is not valid JSON")

	assert.Equal(t, "test-cluster", analysis.ClusterName)
	assert.Equal(t, 3, analysis.TotalNodes)
	assert.Equal(t, 1, analysis.TotalPendingPods)
	assert.Len(t, analysis.UnschedulablePods, 1)
}

func TestYAMLOutput(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewReporter(&buf, OutputFormatYAML)

	results := []types.AnalysisResult{
		{
			Pod: types.PodInfo{
				Name:           "test-pod",
				Namespace:      "default",
				RequestsCPU:    resource.MustParse("100m"),
				RequestsMemory: resource.MustParse("128Mi"),
			},
			IsSchedulable: true,
		},
	}

	err := reporter.GenerateReport(context.Background(), results, "test-cluster", 2)
	require.NoError(t, err, "Failed to generate YAML report")

	var analysis types.ClusterAnalysis
	err = yaml.Unmarshal(buf.Bytes(), &analysis)
	require.NoError(t, err, "Generated output is not valid YAML")

	assert.Equal(t, "test-cluster", analysis.ClusterName)
}

func TestEmptyResults(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewReporter(&buf, OutputFormatJSON)

	results := []types.AnalysisResult{}

	err := reporter.GenerateReport(context.Background(), results, "test-cluster", 1)
	if err != nil {
		t.Fatalf("Failed to generate report for empty results: %v", err)
	}

	output := buf.String()
	assert.Equal(t, "No pending pods found in the specified scope.\n", output)
}

func TestNewReporter(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewReporter(&buf, OutputFormatJSON)
	
	assert.NotNil(t, reporter)
	assert.Equal(t, &buf, reporter.writer)
	assert.Equal(t, OutputFormatJSON, reporter.format)
}

func TestGenerateHumanReport(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewReporter(&buf, OutputFormatHuman)

	results := []types.AnalysisResult{
		{
			Pod: types.PodInfo{
				Name:      "schedulable-pod",
				Namespace: "default",
			},
			IsSchedulable: true,
		},
		{
			Pod: types.PodInfo{
				Name:      "unschedulable-pod",
				Namespace: "default",
			},
			IsSchedulable: false,
			Reason:        "Insufficient CPU",
			Suggestion:    "Add more nodes",
		},
	}

	err := reporter.GenerateReport(context.Background(), results, "test-cluster", 2)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Found 2 pending pod(s) for analysis:")
	assert.Contains(t, output, "[✓] Pod: schedulable-pod - Schedulable")
	assert.Contains(t, output, "[✗] Pod: unschedulable-pod")
	assert.Contains(t, output, "→ Reason: Insufficient CPU")
	assert.Contains(t, output, "→ Suggested: Add more nodes")
}

func TestGenerateReport_UnsupportedFormat(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewReporter(&buf, OutputFormat("unsupported"))

	results := []types.AnalysisResult{
		{
			Pod: types.PodInfo{
				Name:      "test-pod",
				Namespace: "default",
			},
			IsSchedulable: true,
		},
	}

	err := reporter.GenerateReport(context.Background(), results, "test-cluster", 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format: unsupported")
}

func TestSendSlackNotification(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewReporter(&buf, OutputFormatJSON)

	results := []types.AnalysisResult{
		{
			Pod: types.PodInfo{
				Name:      "test-pod",
				Namespace: "default",
			},
			IsSchedulable: false,
		},
	}

	err := reporter.SendSlackNotification(context.Background(), "https://hooks.slack.com/test", results)
	assert.NoError(t, err)
}

func TestSendPrometheusMetrics(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewReporter(&buf, OutputFormatJSON)

	// Create test data
	results := []types.AnalysisResult{
		{
			Pod: types.PodInfo{
				Name:      "test-pod",
				Namespace: "default",
			},
			IsSchedulable: false,
			Reason:       "Insufficient resources",
			Suggestion:   "Add more nodes",
		},
	}

	err := reporter.SendPrometheusMetrics(context.Background(), "", results, "test-cluster")
	assert.NoError(t, err) // Should succeed with empty URL (no-op)
}

func TestBuildClusterAnalysis(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewReporter(&buf, OutputFormatJSON)

	results := []types.AnalysisResult{
		{
			Pod: types.PodInfo{
				Name:      "schedulable-pod",
				Namespace: "default",
			},
			IsSchedulable: true,
		},
		{
			Pod: types.PodInfo{
				Name:      "unschedulable-pod-1",
				Namespace: "default",
			},
			IsSchedulable: false,
			Reason:        "Insufficient CPU",
		},
		{
			Pod: types.PodInfo{
				Name:      "unschedulable-pod-2",
				Namespace: "test",
			},
			IsSchedulable: false,
			Reason:        "Insufficient Memory",
		},
	}

	analysis := reporter.buildClusterAnalysis(results, "test-cluster", 5)

	assert.Equal(t, "test-cluster", analysis.ClusterName)
	assert.Equal(t, 5, analysis.TotalNodes)
	assert.Equal(t, 3, analysis.TotalPendingPods)
	assert.Len(t, analysis.UnschedulablePods, 2)
	assert.Equal(t, "Found 3 pending pods, 2 unschedulable due to resource constraints", analysis.Summary)
	assert.NotZero(t, analysis.Timestamp)

	assert.Equal(t, "unschedulable-pod-1", analysis.UnschedulablePods[0].Pod.Name)
	assert.Equal(t, "unschedulable-pod-2", analysis.UnschedulablePods[1].Pod.Name)
}
