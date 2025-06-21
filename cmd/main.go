package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/syossan27/k8s-pending-resource-inspector/internal"
)

var (
	namespace     string
	includeLimits bool
	outputFormat  string
)

var rootCmd = &cobra.Command{
	Use:   "k8s-pending-resource-inspector",
	Short: "A CLI tool to inspect Kubernetes Pods stuck in Pending state due to resource constraints",
	Long: `k8s-pending-resource-inspector analyzes Kubernetes clusters to identify Pods that remain 
in Pending state because their CPU or memory requests exceed the allocatable capacity 
of all available nodes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAnalysis()
	},
}

func init() {
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Target namespace to analyze (empty for cluster-wide)")
	rootCmd.Flags().BoolVar(&includeLimits, "include-limits", false, "Use resource limits instead of requests for analysis")
	rootCmd.Flags().StringVarP(&outputFormat, "output", "o", "human", "Output format: human, json, yaml")
}

func runAnalysis() error {
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
	_ = reporter

	if len(results) == 0 {
		fmt.Println("No pending pods found in the specified scope.")
		return nil
	}

	fmt.Printf("Found %d pending pod(s) for analysis:\n\n", len(results))
	for _, result := range results {
		if result.IsSchedulable {
			fmt.Printf("[✓] Pod: %s/%s - Schedulable\n", result.Pod.Namespace, result.Pod.Name)
		} else {
			fmt.Printf("[✗] Pod: %s/%s\n", result.Pod.Namespace, result.Pod.Name)
			fmt.Printf("→ Reason: %s\n", result.Reason)
			fmt.Printf("→ Suggested: %s\n", result.Suggestion)
		}
		fmt.Println()
	}

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
