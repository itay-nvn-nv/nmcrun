package main

import (
	"fmt"
	"os"

	"nmcrun/internal/collector"
	"nmcrun/internal/updater"
	"nmcrun/internal/version"

	"github.com/spf13/cobra"
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
		collector, err := collector.New()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing collector: %v\n", err)
			os.Exit(1)
		}
		if err := collector.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test environment and connectivity for RunAI log collection",
	Long: `Tests Kubernetes cluster connectivity and displays RunAI cluster information 
including control plane and cluster URLs. No external tools required.`,
	Run: func(cmd *cobra.Command, args []string) {
		collector, err := collector.New()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing collector: %v\n", err)
			os.Exit(1)
		}
		if err := collector.RunTests(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var workloadsCmd = &cobra.Command{
	Use:   "workloads",
	Short: "Collect detailed information about a specific RunAI workload",
	Long: `Collects comprehensive information about a RunAI workload including YAML manifests,
pod logs, and related resources. Creates a timestamped archive for analysis.`,
	Run: func(cmd *cobra.Command, args []string) {
		project, _ := cmd.Flags().GetString("project")
		workloadType, _ := cmd.Flags().GetString("type")
		name, _ := cmd.Flags().GetString("name")

		if project == "" || workloadType == "" || name == "" {
			fmt.Fprintf(os.Stderr, "Error: --project, --type, and --name are required\n")
			cmd.Usage()
			os.Exit(1)
		}

		collector, err := collector.New()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing collector: %v\n", err)
			os.Exit(1)
		}
		if err := collector.CollectWorkloadInfo(project, workloadType, name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var schedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "Collect RunAI scheduler information and resources",
	Long: `Collects comprehensive RunAI scheduler information including projects, queues,
nodepools, and departments. Creates a timestamped archive with all resources.`,
	Run: func(cmd *cobra.Command, args []string) {
		collector, err := collector.New()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing collector: %v\n", err)
			os.Exit(1)
		}
		if err := collector.CollectSchedulerInfo(); err != nil {
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
	// Add flags for workloads command
	workloadsCmd.Flags().StringP("project", "p", "", "RunAI project name (required)")
	workloadsCmd.Flags().StringP("type", "t", "", "Workload type: tw, iw, infw, dw, dinfw, ew (required)")
	workloadsCmd.Flags().StringP("name", "n", "", "Workload name (required)")
	workloadsCmd.MarkFlagRequired("project")
	workloadsCmd.MarkFlagRequired("type")
	workloadsCmd.MarkFlagRequired("name")

	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(workloadsCmd)
	rootCmd.AddCommand(schedulerCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(upgradeCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
