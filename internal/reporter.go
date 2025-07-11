package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/syossan27/k8s-pending-resource-inspector/pkg/types"
	"github.com/syossan27/k8s-pending-resource-inspector/pkg/utils"
	"gopkg.in/yaml.v3"
)

// OutputFormat represents the different output formats supported for generating reports.
// It defines how analysis results should be formatted and presented to users.
type OutputFormat string

// Supported output formats for analysis reports.
const (
	// OutputFormatHuman provides human-readable output with formatted tables and text
	OutputFormatHuman OutputFormat = "human"
	// OutputFormatJSON provides structured JSON output for programmatic consumption
	OutputFormatJSON OutputFormat = "json"
	// OutputFormatYAML provides structured YAML output for configuration and automation
	OutputFormatYAML OutputFormat = "yaml"
)



// Reporter handles the generation and delivery of analysis reports in various formats.
// It can output results to different destinations and formats, and supports
// integration with external systems like Slack and Prometheus.
type Reporter struct {
	writer io.Writer
	format OutputFormat
}

// NewReporter creates a new Reporter instance with the specified output writer and format.
// The writer determines where the report output will be sent (e.g., stdout, file),
// and the format determines how the data will be structured.
//
// Parameters:
//   - writer: The destination for report output (e.g., os.Stdout, file handle)
//   - format: The output format to use (human, json, or yaml)
//
// Returns:
//   - *Reporter: A new Reporter instance configured with the specified writer and format
func NewReporter(writer io.Writer, format OutputFormat) *Reporter {
	return &Reporter{
		writer: writer,
		format: format,
	}
}

// GenerateReport generates and outputs a formatted report based on analysis results.
func (r *Reporter) GenerateReport(ctx context.Context, results []types.AnalysisResult, clusterName string, totalNodes int) error {
	logrus.WithFields(logrus.Fields{
		"results_count": len(results),
		"cluster_name":  clusterName,
		"total_nodes":   totalNodes,
		"format":        r.format,
	}).Info("Generating analysis report")

	if len(results) == 0 {
		logrus.Info("No pending pods found in the specified scope")
		fmt.Fprintln(r.writer, "No pending pods found in the specified scope.")
		return nil
	}

	switch r.format {
	case OutputFormatHuman:
		logrus.Debug("Generating human-readable report")
		return r.generateHumanReport(results)
	case OutputFormatJSON:
		logrus.Debug("Generating JSON report")
		return r.generateJSONReport(results, clusterName, totalNodes)
	case OutputFormatYAML:
		logrus.Debug("Generating YAML report")
		return r.generateYAMLReport(results, clusterName, totalNodes)
	default:
		logrus.WithField("format", r.format).Error("Unsupported output format")
		return fmt.Errorf("unsupported output format: %s", r.format)
	}
}

func (r *Reporter) generateHumanReport(results []types.AnalysisResult) error {
	fmt.Fprintf(r.writer, "Found %d pending pod(s) for analysis:\n\n", len(results))
	for _, result := range results {
		if result.IsSchedulable {
			fmt.Fprintf(r.writer, "[✓] Pod: %s - Schedulable\n", result.Pod.Name)
		} else {
			fmt.Fprintf(r.writer, "[✗] Pod: %s\n", result.Pod.Name)
			fmt.Fprintf(r.writer, "→ Reason: %s\n", result.Reason)
			fmt.Fprintf(r.writer, "→ Suggested: %s\n", result.Suggestion)
		}
		fmt.Fprintln(r.writer)
	}
	return nil
}

func (r *Reporter) generateJSONReport(results []types.AnalysisResult, clusterName string, totalNodes int) error {
	analysis := r.buildClusterAnalysis(results, clusterName, totalNodes)
	encoder := json.NewEncoder(r.writer)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(analysis)
	if err != nil {
		logrus.WithError(err).Error("Failed to encode JSON report")
		return err
	}
	logrus.Debug("Successfully generated JSON report")
	return nil
}

func (r *Reporter) generateYAMLReport(results []types.AnalysisResult, clusterName string, totalNodes int) error {
	analysis := r.buildClusterAnalysis(results, clusterName, totalNodes)
	data, err := yaml.Marshal(analysis)
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal YAML report")
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}
	_, err = r.writer.Write(data)
	if err != nil {
		logrus.WithError(err).Error("Failed to write YAML report")
		return err
	}
	logrus.Debug("Successfully generated YAML report")
	return nil
}

// SendSlackNotification sends analysis results as a notification to a Slack channel.
func (r *Reporter) SendSlackNotification(ctx context.Context, webhookURL string, results []types.AnalysisResult) error {
	unschedulableCount := 0
	for _, result := range results {
		if !result.IsSchedulable {
			unschedulableCount++
		}
	}

	logrus.WithFields(logrus.Fields{
		"webhook_url":        utils.RedactWebhookURL(webhookURL),
		"total_results":      len(results),
		"unschedulable_pods": unschedulableCount,
	}).Info("Sending Slack notification")

	logrus.Debug("Slack notification feature is currently a placeholder implementation")
	fmt.Printf("Slack notification would be sent to: %s with %d results (%d unschedulable)\n",
		webhookURL, len(results), unschedulableCount)
	return nil
}

func (r *Reporter) buildClusterAnalysis(results []types.AnalysisResult, clusterName string, totalNodes int) types.ClusterAnalysis {
	unschedulablePods := make([]types.AnalysisResult, 0)
	for _, result := range results {
		if !result.IsSchedulable {
			unschedulablePods = append(unschedulablePods, result)
		}
	}

	summary := fmt.Sprintf("Found %d pending pods, %d unschedulable due to resource constraints",
		len(results), len(unschedulablePods))

	return types.ClusterAnalysis{
		Timestamp:         time.Now(),
		ClusterName:       clusterName,
		TotalNodes:        totalNodes,
		TotalPendingPods:  len(results),
		UnschedulablePods: unschedulablePods,
		Summary:           summary,
	}
}

// SendPrometheusMetrics sends analysis results as metrics to a Prometheus Push Gateway.
// This method is currently a placeholder for future implementation of Prometheus integration
// that will convert pod schedulability analysis into metrics and push them for monitoring.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and timeout
//   - pushGatewayURL: The Prometheus Push Gateway URL to send metrics to
//
// Returns:
//   - error: Currently always returns nil (not implemented)
func (r *Reporter) SendPrometheusMetrics(ctx context.Context, pushGatewayURL string) error {
	return nil
}
