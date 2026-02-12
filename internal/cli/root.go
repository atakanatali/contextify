package cli

import (
	"github.com/spf13/cobra"
)

var (
	flagServerURL string
	flagConfig    string
	flagVerbose   bool
)

func NewRootCmd(version string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "contextify",
		Short: "Shared memory system for AI agents",
		Long: `Contextify - Shared memory system for AI agents.

Manage your Contextify installation, configure AI tools,
and interact with the memory system from the command line.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().StringVar(&flagServerURL, "server", "", "server URL (default http://localhost:8420)")
	rootCmd.PersistentFlags().StringVar(&flagConfig, "config", "", "config file (default ~/.contextify/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "V", false, "verbose output")

	// Management commands
	rootCmd.AddCommand(newVersionCmd(version))
	rootCmd.AddCommand(newStartCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newRestartCmd())
	rootCmd.AddCommand(newLogsCmd())
	rootCmd.AddCommand(newUpdateCmd(version))
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newInstallCmd())
	rootCmd.AddCommand(newUninstallCmd())

	// Memory commands
	rootCmd.AddCommand(newStoreCmd())
	rootCmd.AddCommand(newRecallCmd())
	rootCmd.AddCommand(newSearchCmd())
	rootCmd.AddCommand(newGetCmd())
	rootCmd.AddCommand(newDeleteCmd())
	rootCmd.AddCommand(newPromoteCmd())
	rootCmd.AddCommand(newStatsCmd())
	rootCmd.AddCommand(newContextCmd())

	return rootCmd
}

func Execute(version string) error {
	return NewRootCmd(version).Execute()
}
