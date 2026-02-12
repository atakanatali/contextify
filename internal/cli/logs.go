package cli

import (
	"fmt"

	"github.com/atakanatali/contextify/internal/docker"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show Contextify container logs",
		RunE:  runLogs,
	}
	cmd.Flags().BoolP("follow", "f", false, "Follow log output")
	cmd.Flags().StringP("tail", "n", "100", "Number of lines to show from the end")
	return cmd
}

func runLogs(cmd *cobra.Command, args []string) error {
	mgr := docker.NewManager(getContainerName(), getDockerImage(), getPort())

	status, err := mgr.Status(cmd.Context())
	if err != nil {
		return fmt.Errorf("check container status: %w", err)
	}

	if !status.Exists {
		return fmt.Errorf("contextify container not found")
	}

	follow, _ := cmd.Flags().GetBool("follow")
	tail, _ := cmd.Flags().GetString("tail")

	return mgr.Logs(cmd.Context(), follow, tail)
}
