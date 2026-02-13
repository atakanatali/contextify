package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/atakanatali/contextify/internal/docker"
	"github.com/atakanatali/contextify/internal/toolconfig"
	"github.com/spf13/cobra"
)

func newUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove Contextify configurations and optionally stop the container",
		RunE:  runUninstall,
	}
	cmd.Flags().Bool("remove-container", false, "Also stop and remove the Docker container")
	cmd.Flags().Bool("remove-data", false, "Also remove the data volume (DESTRUCTIVE)")
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompts")
	return cmd
}

func runUninstall(cmd *cobra.Command, args []string) error {
	removeContainer, _ := cmd.Flags().GetBool("remove-container")
	removeData, _ := cmd.Flags().GetBool("remove-data")
	force, _ := cmd.Flags().GetBool("force")

	printHeader("Contextify Uninstall")

	if !force {
		msg := "This will remove all AI tool configurations for Contextify."
		if removeData {
			msg += " ALL MEMORY DATA WILL BE PERMANENTLY DELETED."
		}
		fmt.Printf("  %s\n", msg)
		fmt.Print("  Are you sure? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("  Cancelled.")
			return nil
		}
	}

	// Remove tool configurations
	printStep("Removing Claude Code configuration...")
	_ = toolconfig.UninstallClaudeCode()
	printOK("Claude Code configuration removed.")

	printStep("Removing Claude Desktop / Cowork configuration...")
	_ = toolconfig.UninstallClaudeDesktop()
	printOK("Claude Desktop configuration removed.")

	printStep("Removing Claude Chat configuration...")
	_ = toolconfig.UninstallClaudeChat()
	printOK("Claude Chat configuration removed.")

	printStep("Removing Codex configuration...")
	_ = toolconfig.UninstallCodex()
	printOK("Codex configuration removed.")

	printStep("Removing Cursor configuration...")
	_ = toolconfig.UninstallCursor()
	printOK("Cursor configuration removed.")

	printStep("Removing Windsurf configuration...")
	_ = toolconfig.UninstallWindsurf()
	printOK("Windsurf configuration removed.")

	printStep("Removing Gemini configuration...")
	_ = toolconfig.UninstallGemini()
	printOK("Gemini configuration removed.")

	// Remove hooks directory
	printStep("Removing hooks...")
	home, _ := os.UserHomeDir()
	_ = os.RemoveAll(home + "/.contextify/hooks")
	printOK("Hooks removed.")

	if removeContainer {
		mgr := docker.NewManager(getContainerName(), getDockerImage(), getPort())
		status, _ := mgr.Status(cmd.Context())
		if status.Exists {
			if status.Running {
				printStep("Stopping container...")
				_ = mgr.Stop(cmd.Context())
			}
			printStep("Removing container...")
			_ = mgr.Remove(cmd.Context())
			printOK("Container removed.")
		}
	}

	if removeData {
		printStep("Removing data volume...")
		_ = removeDockerVolume("contextify-data")
		printOK("Data volume removed.")
	}

	printHeader("Uninstall Complete")
	if !removeContainer {
		printInfo("Container was not removed. Use --remove-container to also remove it.")
	}
	if !removeData {
		printInfo("Data volume was preserved. Use --remove-data to permanently delete all memories.")
	}
	fmt.Println()

	return nil
}

func removeDockerVolume(name string) error {
	mgr := docker.NewManager("", "", "")
	return mgr.VolumeRemove(name)
}
