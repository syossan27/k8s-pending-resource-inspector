package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

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
	if err != nil {
		t.Fatalf("Failed to generate JSON report: %v", err)
	}

	var analysis types.ClusterAnalysis
	if err := json.Unmarshal(buf.Bytes(), &analysis); err != nil {
		t.Fatalf("Generated output is not valid JSON: %v", err)
	}

	if analysis.ClusterName != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got '%s'", analysis.ClusterName)
	}
	if analysis.TotalNodes != 3 {
		t.Errorf("Expected 3 total nodes, got %d", analysis.TotalNodes)
	}
	if analysis.TotalPendingPods != 1 {
		t.Errorf("Expected 1 pending pod, got %d", analysis.TotalPendingPods)
	}
	if len(analysis.UnschedulablePods) != 1 {
		t.Errorf("Expected 1 unschedulable pod, got %d", len(analysis.UnschedulablePods))
	}
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
	if err != nil {
		t.Fatalf("Failed to generate YAML report: %v", err)
	}

	var analysis types.ClusterAnalysis
	if err := yaml.Unmarshal(buf.Bytes(), &analysis); err != nil {
		t.Fatalf("Generated output is not valid YAML: %v", err)
	}

	if analysis.ClusterName != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got '%s'", analysis.ClusterName)
	}
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
	if output != "No pending pods found in the specified scope.\n" {
		t.Errorf("Expected empty results message, got: %s", output)
	}
}
