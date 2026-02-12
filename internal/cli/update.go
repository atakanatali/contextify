package cli

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/atakanatali/contextify/internal/client"
	"github.com/atakanatali/contextify/internal/docker"
	"github.com/atakanatali/contextify/internal/toolconfig"
	"github.com/spf13/cobra"
)

const (
	cliReleaseBaseURL = "https://github.com/atakanatali/contextify/releases"
)

func newUpdateCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update Contextify server and CLI",
		Long: `Update Contextify by pulling a new Docker image, recreating the container,
and updating the CLI binary to the latest release. Data is preserved via the Docker volume.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd, args, version)
		},
	}
	cmd.Flags().StringP("version", "v", "", "Target version (e.g. 0.6.0). Defaults to latest")
	cmd.Flags().Bool("skip-cli", false, "Skip CLI binary update")
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func runUpdate(cmd *cobra.Command, args []string, currentVersion string) error {
	skipConfirm, _ := cmd.Flags().GetBool("yes")

	if !skipConfirm {
		fmt.Println()
		printWarn("Updating will restart the Contextify server.")
		printWarn("Active AI agent sessions (Claude Code, Cursor, etc.) will lose their MCP connection.")
		printWarn("You will need to start a new session after the update.")
		fmt.Println()
		fmt.Print("  Continue? [y/N] ")

		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			printInfo("Update cancelled.")
			return nil
		}
		fmt.Println()
	}

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

	// --- Server update ---
	printHeader("Updating Server")

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

	printOK("Server updated.")

	// --- CLI update ---
	skipCLI, _ := cmd.Flags().GetBool("skip-cli")
	if !skipCLI {
		printHeader("Updating CLI")
		if err := updateCLI(targetVersion, currentVersion); err != nil {
			printWarn(fmt.Sprintf("CLI update failed: %v", err))
			printInfo("You can update manually: curl -fsSL https://raw.githubusercontent.com/atakanatali/contextify/main/scripts/install-cli.sh | sh")
		}
	}

	// --- Tool config update ---
	printHeader("Updating Tool Configurations")
	mcpURL := getServerURL() + "/mcp"
	updatedTools, err := toolconfig.UpdateConfiguredTools(mcpURL)
	if err != nil {
		printWarn(fmt.Sprintf("Some tool configs failed to update: %v", err))
	}
	if len(updatedTools) > 0 {
		for _, t := range updatedTools {
			tool := toolconfig.ToolByName(string(t))
			label := string(t)
			if tool != nil {
				label = tool.Label
			}
			printOK(fmt.Sprintf("%s configs updated.", label))
		}
	} else {
		printInfo("No configured tools found to update.")
	}

	printOK("Contextify updated successfully!")
	return nil
}

func updateCLI(targetVersion, currentVersion string) error {
	// Determine download URL
	asset := fmt.Sprintf("contextify-%s-%s", runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	if targetVersion != "" {
		downloadURL = fmt.Sprintf("%s/download/v%s/%s", cliReleaseBaseURL, targetVersion, asset)
	} else {
		downloadURL = fmt.Sprintf("%s/latest/download/%s", cliReleaseBaseURL, asset)
	}

	printStep(fmt.Sprintf("Downloading %s...", asset))

	// Download to temp file
	tmpFile, err := os.CreateTemp("", "contextify-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	resp, err := http.Get(downloadURL)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("download failed: HTTP %d (check if release exists)", resp.StatusCode)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write binary: %w", err)
	}
	tmpFile.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	// Verify the downloaded binary works
	out, err := exec.Command(tmpPath, "version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("downloaded binary is invalid: %w", err)
	}
	newVersion := strings.TrimSpace(string(out))
	printStep(fmt.Sprintf("Downloaded: %s", newVersion))

	// Find current binary path
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find current binary: %w", err)
	}

	// Try direct replace first, fall back to sudo
	err = replaceFile(tmpPath, currentBinary)
	if err != nil {
		// Try with sudo
		printStep("Need elevated permissions, trying sudo...")
		sudoCmd := exec.Command("sudo", "mv", tmpPath, currentBinary)
		sudoCmd.Stdin = os.Stdin
		sudoCmd.Stdout = os.Stdout
		sudoCmd.Stderr = os.Stderr
		if err := sudoCmd.Run(); err != nil {
			return fmt.Errorf("install binary (try: sudo mv %s %s): %w", tmpPath, currentBinary, err)
		}
	}

	printOK(fmt.Sprintf("CLI updated: %s → %s", currentVersion, newVersion))
	return nil
}

// replaceFile attempts to replace dst with src via rename.
func replaceFile(src, dst string) error {
	// Try rename (works if same filesystem and have write permission)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Try copy if rename fails (cross-device)
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
