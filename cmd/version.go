package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	// Version information - set via ldflags during build
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Display the version, commit hash, and build date of the microservice.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("microservice version %s\n", Version)
		fmt.Printf("  commit: %s\n", Commit)
		fmt.Printf("  built:  %s\n", BuildDate)

		// If version is still "dev", try to get from build info
		if Version == "dev" {
			if info, ok := debug.ReadBuildInfo(); ok {
				fmt.Printf("  module: %s\n", info.Main.Path)
				if info.Main.Version != "" && info.Main.Version != "(devel)" {
					fmt.Printf("  go version: %s\n", info.Main.Version)
				}
			}
		}
	},
}
