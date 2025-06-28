package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/syossan27/k8s-pending-resource-inspector/internal"
	"github.com/syossan27/k8s-pending-resource-inspector/pkg/utils"
)

var (
	namespace     string
	includeLimits bool
	outputFormat  string
	alertSlack    string
	logLevel      string
	logFormat     string
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
	rootCmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level: debug, info, warn, error")
	rootCmd.Flags().StringVar(&logFormat, "log-format", "text", "Log format: text, json")
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

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[logLevel] {
		return fmt.Errorf("unsupported log level: %s (supported: debug, info, warn, error)", logLevel)
	}

	validLogFormats := map[string]bool{"text": true, "json": true}
	if !validLogFormats[logFormat] {
		return fmt.Errorf("unsupported log format: %s (supported: text, json)", logFormat)
	}

	return nil
}

func setupLogging() error {
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}
	logrus.SetLevel(level)

	if logFormat == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	}

	return nil
}



func runAnalysis() error {
	if err := validateFlags(); err != nil {
		return err
	}

	if err := setupLogging(); err != nil {
		return err
	}

	ctx := context.Background()

	logrus.Info("Starting k8s-pending-resource-inspector analysis")

	if namespace != "" {
		logrus.WithField("namespace", namespace).Info("Analyzing specific namespace")
	} else {
		logrus.Info("Analyzing cluster-wide")
	}

	fetcher, err := internal.NewFetcherFromConfig()
	if err != nil {
		logrus.WithError(err).Error("Failed to create Kubernetes client")
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	logrus.Debug("Successfully created Kubernetes client")

	analyzer := internal.NewAnalyzer(fetcher)

	results, err := analyzer.AnalyzePodSchedulability(ctx, namespace, includeLimits)
	if err != nil {
		logrus.WithError(err).Error("Failed to analyze pod schedulability")
		return fmt.Errorf("failed to analyze pod schedulability: %w", err)
	}

	logrus.WithField("pending_pods_count", len(results)).Info("Pod schedulability analysis completed")

	nodes, err := fetcher.FetchNodes(ctx)
	if err != nil {
		logrus.WithError(err).Error("Failed to fetch nodes for metadata")
		return fmt.Errorf("failed to fetch nodes for metadata: %w", err)
	}

	logrus.WithField("nodes_count", len(nodes)).Debug("Fetched cluster nodes for metadata")

	clusterName := "unknown"

	var format internal.OutputFormat
	switch outputFormat {
	case "json":
		format = internal.OutputFormatJSON
	case "yaml":
		format = internal.OutputFormatYAML
	case "human":
		format = internal.OutputFormatHuman
	default:
		logrus.WithField("format", outputFormat).Error("Unsupported output format")
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}

	reporter := internal.NewReporter(os.Stdout, format)

	logrus.WithField("format", outputFormat).Info("Generating report")
	if err := reporter.GenerateReport(ctx, results, clusterName, len(nodes)); err != nil {
		logrus.WithError(err).Error("Failed to generate report")
		return fmt.Errorf("failed to generate report: %w", err)
	}

	if alertSlack != "" {
		logrus.WithField("webhook_url", utils.RedactWebhookURL(alertSlack)).Info("Sending Slack notification")
		if err := reporter.SendSlackNotification(ctx, alertSlack, results); err != nil {
			logrus.WithError(err).Error("Failed to send Slack notification")
			return fmt.Errorf("failed to send Slack notification: %w", err)
		}
	}

	logrus.Info("Analysis completed successfully")
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
