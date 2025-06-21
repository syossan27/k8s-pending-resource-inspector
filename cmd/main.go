package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/syossan27/k8s-pending-resource-inspector/internal"
)

var (
	namespace     string
	includeLimits bool
	outputFormat  string
	alertSlack    string
)

var rootCmd = &cobra.Command{
	Use:   "k8s-pending-resource-inspector",
	Short: "A CLI tool to inspect Kubernetes Pods stuck in Pending state due to resource constraints",
	Long: `k8s-pending-resource-inspector analyzes Kubernetes clusters to identify Pods that remain 
in Pending state because their CPU or memory requests exceed the allocatable capacity 
of all available nodes.

Examples:
  # Analyze all namespaces
  k8s-pending-resource-inspector

  # Analyze specific namespace with JSON output
  k8s-pending-resource-inspector --namespace my-app --output json

  # Include limits and send Slack notification
  k8s-pending-resource-inspector --include-limits --alert-slack https://hooks.slack.com/services/XXX`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAnalysis()
	},
}

func init() {
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Target namespace to analyze (empty for cluster-wide)")
	rootCmd.Flags().BoolVar(&includeLimits, "include-limits", false, "Use resource limits instead of requests for analysis")
	rootCmd.Flags().StringVarP(&outputFormat, "output", "o", "human", "Output format: human, json, yaml")
	rootCmd.Flags().StringVar(&alertSlack, "alert-slack", "", "Slack webhook URL for notifications (optional)")
}

func validateFlags() error {
	validFormats := map[string]bool{"human": true, "json": true, "yaml": true}
	if !validFormats[outputFormat] {
		return fmt.Errorf("unsupported output format: %s (supported: human, json, yaml)", outputFormat)
	}
	
	if alertSlack != "" {
		if !strings.HasPrefix(alertSlack, "https://hooks.slack.com/") {
			return fmt.Errorf("invalid Slack webhook URL: must start with https://hooks.slack.com/")
		}
	}
	
	return nil
}

func runAnalysis() error {
	if err := validateFlags(); err != nil {
		return err
	}
	
	ctx := context.Background()

	fetcher, err := internal.NewFetcherFromConfig()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	analyzer := internal.NewAnalyzer(fetcher)

	results, err := analyzer.AnalyzePodSchedulability(ctx, namespace, includeLimits)
	if err != nil {
		return fmt.Errorf("failed to analyze pod schedulability: %w", err)
	}

	var format internal.OutputFormat
	switch outputFormat {
	case "json":
		format = internal.OutputFormatJSON
	case "yaml":
		format = internal.OutputFormatYAML
	case "human":
		format = internal.OutputFormatHuman
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}

	reporter := internal.NewReporter(os.Stdout, format)
	
	if err := reporter.GenerateReport(ctx, results); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}
	
	if alertSlack != "" {
		if err := reporter.SendSlackNotification(ctx, alertSlack, results); err != nil {
			return fmt.Errorf("failed to send Slack notification: %w", err)
		}
	}

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
