package cli

import (
	"fmt"

	"github.com/atakanatali/contextify/internal/docker"
	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the Contextify container",
		RunE:  runStart,
	}
}

func runStart(cmd *cobra.Command, args []string) error {
	mgr := docker.NewManager(getContainerName(), getDockerImage(), getPort())

	if !mgr.IsDockerAvailable() {
		return fmt.Errorf("docker is not installed. Please install Docker first")
	}
	if !mgr.IsDockerRunning() {
		return fmt.Errorf("docker daemon is not running. Please start Docker first")
	}

	status, err := mgr.Status(cmd.Context())
	if err != nil {
		return fmt.Errorf("check container status: %w", err)
	}

	if status.Running {
		printOK("Contextify is already running.")
		return nil
	}

	if status.Exists {
		printStep("Starting existing container...")
		if err := mgr.Start(cmd.Context()); err != nil {
			return fmt.Errorf("start container: %w", err)
		}
		printOK("Contextify started.")
		return nil
	}

	printStep("Container not found. Pulling image...")
	if err := mgr.Pull(cmd.Context()); err != nil {
		return fmt.Errorf("pull image: %w", err)
	}

	printStep("Creating and starting container...")
	if err := mgr.Run(cmd.Context()); err != nil {
		return fmt.Errorf("run container: %w", err)
	}

	printOK("Contextify started.")
	return nil
}
