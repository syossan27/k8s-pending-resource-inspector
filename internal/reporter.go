package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/syossan27/k8s-pending-resource-inspector/pkg/types"
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
	if len(results) == 0 {
		fmt.Fprintln(r.writer, "No pending pods found in the specified scope.")
		return nil
	}

	switch r.format {
	case OutputFormatHuman:
		return r.generateHumanReport(results)
	case OutputFormatJSON:
		return r.generateJSONReport(results, clusterName, totalNodes)
	case OutputFormatYAML:
		return r.generateYAMLReport(results, clusterName, totalNodes)
	default:
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
	return encoder.Encode(analysis)
}

func (r *Reporter) generateYAMLReport(results []types.AnalysisResult, clusterName string, totalNodes int) error {
	analysis := r.buildClusterAnalysis(results, clusterName, totalNodes)
	data, err := yaml.Marshal(analysis)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}
	_, err = r.writer.Write(data)
	return err
}

// SlackMessage represents the structure of a Slack webhook message
type SlackMessage struct {
	Text        string              `json:"text"`
	Attachments []SlackAttachment   `json:"attachments,omitempty"`
}

// SlackAttachment represents a Slack message attachment
type SlackAttachment struct {
	Color    string       `json:"color"`
	Title    string       `json:"title"`
	Text     string       `json:"text"`
	Fields   []SlackField `json:"fields,omitempty"`
}

// SlackField represents a field within a Slack attachment
type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// SendSlackNotification sends analysis results as a notification to a Slack channel.
func (r *Reporter) SendSlackNotification(ctx context.Context, webhookURL string, results []types.AnalysisResult) error {
	if len(results) == 0 {
		return nil
	}

	unschedulablePods := make([]types.AnalysisResult, 0)
	for _, result := range results {
		if !result.IsSchedulable {
			unschedulablePods = append(unschedulablePods, result)
		}
	}

	message := r.buildSlackMessage(results, unschedulablePods)

	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Slack API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// buildSlackMessage creates a Slack message from analysis results
func (r *Reporter) buildSlackMessage(results []types.AnalysisResult, unschedulablePods []types.AnalysisResult) SlackMessage {
	totalPods := len(results)
	unschedulableCount := len(unschedulablePods)
	
	var color string
	var title string
	
	if unschedulableCount == 0 {
		color = "good"
		title = fmt.Sprintf("✅ All %d pending pods are schedulable", totalPods)
	} else {
		color = "danger"
		title = fmt.Sprintf("⚠️ %d of %d pending pods are unschedulable", unschedulableCount, totalPods)
	}

	attachment := SlackAttachment{
		Color: color,
		Title: title,
	}

	if unschedulableCount > 0 {
		fields := make([]SlackField, 0)
		
		for i, pod := range unschedulablePods {
			if i >= 5 { // Limit to first 5 pods to avoid message length issues
				fields = append(fields, SlackField{
					Title: "Additional Issues",
					Value: fmt.Sprintf("... and %d more unschedulable pods", unschedulableCount-5),
					Short: false,
				})
				break
			}
			
			fields = append(fields, SlackField{
				Title: fmt.Sprintf("Pod: %s", pod.Pod.Name),
				Value: fmt.Sprintf("Reason: %s\nSuggestion: %s", pod.Reason, pod.Suggestion),
				Short: false,
			})
		}
		
		attachment.Fields = fields
	}

	return SlackMessage{
		Text:        "Kubernetes Pending Pod Analysis Report",
		Attachments: []SlackAttachment{attachment},
	}
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
// This method converts pod schedulability analysis into metrics and pushes them for monitoring.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and timeout
//   - pushGatewayURL: The Prometheus Push Gateway URL to send metrics to
//   - results: Analysis results to convert to metrics
//   - clusterName: Name of the cluster for metric labeling
//
// Returns:
//   - error: Error if metrics push fails
func (r *Reporter) SendPrometheusMetrics(ctx context.Context, pushGatewayURL string, results []types.AnalysisResult, clusterName string) error {
	if pushGatewayURL == "" {
		return nil
	}

	metrics := r.buildPrometheusMetrics(results, clusterName)
	
	req, err := http.NewRequestWithContext(ctx, "POST", pushGatewayURL+"/metrics/job/k8s-pending-resource-inspector", bytes.NewBufferString(metrics))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to push metrics to Prometheus: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Prometheus Push Gateway returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// buildPrometheusMetrics converts analysis results into Prometheus metrics format
func (r *Reporter) buildPrometheusMetrics(results []types.AnalysisResult, clusterName string) string {
	var metrics strings.Builder
	
	totalPods := len(results)
	unschedulablePods := 0
	schedulablePods := 0
	
	for _, result := range results {
		if result.IsSchedulable {
			schedulablePods++
		} else {
			unschedulablePods++
		}
	}
	
	timestamp := time.Now().Unix()
	
	// Total pending pods metric
	metrics.WriteString(fmt.Sprintf("# HELP k8s_pending_pods_total Total number of pending pods analyzed\n"))
	metrics.WriteString(fmt.Sprintf("# TYPE k8s_pending_pods_total gauge\n"))
	metrics.WriteString(fmt.Sprintf("k8s_pending_pods_total{cluster=\"%s\"} %d %d\n", clusterName, totalPods, timestamp))
	
	// Schedulable pods metric
	metrics.WriteString(fmt.Sprintf("# HELP k8s_pending_pods_schedulable Number of pending pods that are schedulable\n"))
	metrics.WriteString(fmt.Sprintf("# TYPE k8s_pending_pods_schedulable gauge\n"))
	metrics.WriteString(fmt.Sprintf("k8s_pending_pods_schedulable{cluster=\"%s\"} %d %d\n", clusterName, schedulablePods, timestamp))
	
	// Unschedulable pods metric
	metrics.WriteString(fmt.Sprintf("# HELP k8s_pending_pods_unschedulable Number of pending pods that are unschedulable due to resource constraints\n"))
	metrics.WriteString(fmt.Sprintf("# TYPE k8s_pending_pods_unschedulable gauge\n"))
	metrics.WriteString(fmt.Sprintf("k8s_pending_pods_unschedulable{cluster=\"%s\"} %d %d\n", clusterName, unschedulablePods, timestamp))
	
	// Per-pod metrics
	for _, result := range results {
		schedulableStatus := 0
		if result.IsSchedulable {
			schedulableStatus = 1
		}
		
		podName := result.Pod.Name
		namespace := result.Pod.Namespace
		
		metrics.WriteString(fmt.Sprintf("# HELP k8s_pod_schedulable Whether a specific pod is schedulable (1) or not (0)\n"))
		metrics.WriteString(fmt.Sprintf("# TYPE k8s_pod_schedulable gauge\n"))
		metrics.WriteString(fmt.Sprintf("k8s_pod_schedulable{cluster=\"%s\",pod=\"%s\",namespace=\"%s\"} %d %d\n", 
			clusterName, podName, namespace, schedulableStatus, timestamp))
	}
	
	return metrics.String()
}
