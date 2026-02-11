package cli

import (
	"fmt"

	"github.com/atakanatali/contextify/internal/docker"
	"github.com/spf13/cobra"
)

func newRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the Contextify container",
		RunE:  runRestart,
	}
}

func runRestart(cmd *cobra.Command, args []string) error {
	mgr := docker.NewManager(getContainerName(), getDockerImage(), getPort())

	status, err := mgr.Status(cmd.Context())
	if err != nil {
		return fmt.Errorf("check container status: %w", err)
	}

	if !status.Exists {
		printWarn("Contextify container not found. Use 'contextify start' to create one.")
		return nil
	}

	printStep("Restarting Contextify...")
	if err := mgr.Restart(cmd.Context()); err != nil {
		return fmt.Errorf("restart container: %w", err)
	}

	printOK("Contextify restarted.")
	return nil
}
