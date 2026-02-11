package cli

import (
	"fmt"
	"time"

	"github.com/atakanatali/contextify/internal/client"
	"github.com/atakanatali/contextify/internal/docker"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update Contextify to a new version",
		Long: `Update Contextify by pulling a new Docker image and recreating the container.
Data is preserved via the Docker volume.`,
		RunE: runUpdate,
	}
	cmd.Flags().StringP("version", "v", "", "Target version (e.g. 0.4.0). Defaults to latest")
	return cmd
}

func runUpdate(cmd *cobra.Command, args []string) error {
	mgr := docker.NewManager(getContainerName(), getDockerImage(), getPort())

	if !mgr.IsDockerAvailable() {
		return fmt.Errorf("docker is not installed")
	}

	targetVersion, _ := cmd.Flags().GetString("version")
	targetImage := mgr.Image
	if targetVersion != "" {
		targetImage = mgr.ImageForVersion(targetVersion)
	}

	// Update the manager's image for pull
	mgr.Image = targetImage

	printHeader("Updating Contextify")

	// 1. Pull new image
	printStep(fmt.Sprintf("Pulling %s...", targetImage))
	if err := mgr.Pull(cmd.Context()); err != nil {
		return fmt.Errorf("pull image: %w", err)
	}
	printOK("Image pulled.")

	// 2. Stop existing container
	status, _ := mgr.Status(cmd.Context())
	if status.Exists {
		if status.Running {
			printStep("Stopping current container...")
			if err := mgr.Stop(cmd.Context()); err != nil {
				return fmt.Errorf("stop container: %w", err)
			}
		}

		// 3. Remove old container
		printStep("Removing old container...")
		if err := mgr.Remove(cmd.Context()); err != nil {
			return fmt.Errorf("remove container: %w", err)
		}
	}

	// 4. Run new container
	printStep("Starting new container...")
	if err := mgr.Run(cmd.Context()); err != nil {
		return fmt.Errorf("run container: %w", err)
	}

	// 5. Wait for health
	printStep("Waiting for Contextify to be ready...")
	c := client.New(getServerURL())
	if err := waitForHealthWithProgress(c, 60*time.Second); err != nil {
		printWarn("Container started but health check timed out. Check logs with 'contextify logs'.")
		return nil
	}

	printOK("Contextify updated successfully!")
	return nil
}
