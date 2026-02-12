package cli

import (
	"fmt"

	"github.com/atakanatali/contextify/internal/docker"
	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the Contextify container",
		RunE:  runStop,
	}
}

func runStop(cmd *cobra.Command, args []string) error {
	mgr := docker.NewManager(getContainerName(), getDockerImage(), getPort())

	status, err := mgr.Status(cmd.Context())
	if err != nil {
		return fmt.Errorf("check container status: %w", err)
	}

	if !status.Exists {
		printWarn("Contextify container not found.")
		return nil
	}

	if !status.Running {
		printWarn("Contextify is already stopped.")
		return nil
	}

	printStep("Stopping Contextify...")
	if err := mgr.Stop(cmd.Context()); err != nil {
		return fmt.Errorf("stop container: %w", err)
	}

	printOK("Contextify stopped.")
	return nil
}
