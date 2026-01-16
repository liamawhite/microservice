package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "microservice",
	Short: "A composable HTTP proxy service for testing microservice topologies",
	Long: `A Go-based HTTP proxy service that creates composable mock microservice topologies.

This service acts as an HTTP proxy that can chain requests through multiple services,
allowing you to simulate complex microservice architectures. Perfect for testing
distributed system behaviors in development and CI/CD pipelines.

The service supports:
  - Request chaining through multiple services
  - Fault injection for testing resilience
  - Configurable timeouts and logging
  - Health check endpoints

Examples:
  # Start the server
  microservice serve -p 8080

  # With debug logging
  microservice serve -p 8080 -l debug

  # Show version information
  microservice version`,
	Version: Version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Add subcommands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)

	// Custom version template to match our version command output
	rootCmd.SetVersionTemplate(fmt.Sprintf("microservice version %s\n  commit: %s\n  built:  %s\n", Version, Commit, BuildDate))
}
