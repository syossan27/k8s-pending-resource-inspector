package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "k8s-pending-resource-inspector",
	Short: "A CLI tool to inspect Kubernetes Pods stuck in Pending state due to resource constraints",
	Long: `k8s-pending-resource-inspector analyzes Kubernetes clusters to identify Pods that remain 
in Pending state because their CPU or memory requests exceed the allocatable capacity 
of all available nodes.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("k8s-pending-resource-inspector - analyzing cluster...")
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
