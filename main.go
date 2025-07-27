package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"nmcrun/internal/collector"
	"nmcrun/internal/updater"
	"nmcrun/internal/version"
)

var rootCmd = &cobra.Command{
	Use:   "nmcrun",
	Short: "RunAI log collector and environment diagnostic tool",
	Long: `nmcrun is a tool that collects logs and environment details from RunAI deployments
and archives them for support analysis.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default action: show help
		cmd.Help()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("nmcrun version %s\n", version.Get())
		fmt.Printf("Build date: %s\n", version.GetBuildDate())
		fmt.Printf("Git commit: %s\n", version.GetCommit())
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Collect logs and environment details from RunAI deployment",
	Long: `Collects logs from RunAI pods, cluster configuration, and environment details.
Creates timestamped archives for each namespace (runai and runai-backend).`,
	Run: func(cmd *cobra.Command, args []string) {
		collector := collector.New()
		if err := collector.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Check for updates and upgrade to latest version",
	Run: func(cmd *cobra.Command, args []string) {
		updater := updater.New()
		if err := updater.CheckAndUpgrade(); err != nil {
			fmt.Fprintf(os.Stderr, "Error during upgrade: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(upgradeCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
} 