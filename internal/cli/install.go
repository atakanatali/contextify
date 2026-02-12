package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/atakanatali/contextify/internal/client"
	"github.com/atakanatali/contextify/internal/docker"
	"github.com/atakanatali/contextify/internal/toolconfig"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Contextify: pull image, start container, configure AI tools",
		Long: `Install and configure Contextify in one step.

This command will:
  1. Pull the Docker image
  2. Start the Contextify container
  3. Wait for the system to be ready
  4. Configure your AI tools (Claude Code, Claude Desktop/Cowork, Claude Chat, Cursor, Windsurf, Gemini)
  5. Run a self-test to verify everything works`,
		RunE: runInstall,
	}
	cmd.Flags().StringSlice("tools", nil, "Tools to configure (claude-code,claude-desktop,claude-chat,cursor,windsurf,gemini)")
	cmd.Flags().Bool("all", false, "Configure all detected tools")
	cmd.Flags().Bool("no-test", false, "Skip self-test")
	return cmd
}

func runInstall(cmd *cobra.Command, args []string) error {
	printHeader("Contextify Install")

	mgr := docker.NewManager(getContainerName(), getDockerImage(), getPort())

	// 1. Check prerequisites
	if !mgr.IsDockerAvailable() {
		printFail("Docker is not installed.")
		fmt.Println("  Install Docker: https://docs.docker.com/get-docker/")
		return fmt.Errorf("docker not found")
	}
	if !mgr.IsDockerRunning() {
		printFail("Docker daemon is not running.")
		fmt.Println("  Start Docker Desktop or run: sudo systemctl start docker")
		return fmt.Errorf("docker not running")
	}
	printOK("Docker is available.")

	// 2. Pull and start container
	status, _ := mgr.Status(cmd.Context())
	if status.Running {
		printOK("Contextify container is already running.")
	} else {
		if !status.Exists {
			printStep(fmt.Sprintf("Pulling %s...", mgr.Image))
			if err := mgr.Pull(cmd.Context()); err != nil {
				return fmt.Errorf("pull image: %w", err)
			}
			printOK("Image pulled.")

			printStep("Starting container...")
			if err := mgr.Run(cmd.Context()); err != nil {
				return fmt.Errorf("start container: %w", err)
			}
		} else {
			printStep("Starting existing container...")
			if err := mgr.Start(cmd.Context()); err != nil {
				return fmt.Errorf("start container: %w", err)
			}
		}
		printOK("Container started.")
	}

	// 3. Wait for health
	printStep("Waiting for Contextify to be ready...")
	c := client.New(getServerURL())
	if err := waitForHealthWithProgress(c, 60*time.Second); err != nil {
		printFail("Contextify did not become healthy within 60 seconds.")
		fmt.Println("  Check logs: contextify logs")
		return err
	}
	printOK("Contextify is ready.")

	// 4. Tool selection
	selectedTools, err := selectTools(cmd)
	if err != nil {
		return err
	}

	// 5. Configure tools
	if len(selectedTools) > 0 {
		mcpURL := getServerURL() + "/mcp"
		printHeader("Configuring AI Tools")
		for _, tool := range selectedTools {
			printStep(fmt.Sprintf("Configuring %s...", tool))
			if err := configureTool(tool, mcpURL); err != nil {
				printFail(fmt.Sprintf("Failed to configure %s: %v", tool, err))
			} else {
				printOK(fmt.Sprintf("%s configured.", tool))
			}
		}
	}

	// 6. Self-test
	noTest, _ := cmd.Flags().GetBool("no-test")
	if !noTest {
		printHeader("Self-Test")
		if err := runSelfTest(cmd.Context(), c); err != nil {
			printFail(fmt.Sprintf("Self-test failed: %v", err))
		} else {
			printOK("Self-test passed.")
		}
	}

	// 7. Restart tools that need it
	for _, tool := range selectedTools {
		tn := toolconfig.ToolByName(string(tool))
		if tn != nil && toolconfig.IsToolRunning(tn.Name) {
			printStep(fmt.Sprintf("Restarting %s...", tn.Label))
			_ = toolconfig.RestartTool(tn.Name)
		}
	}

	// 8. Summary
	printHeader("Setup Complete")
	fmt.Println()
	fmt.Printf("  Server:  %s\n", getServerURL())
	fmt.Printf("  MCP:     %s/mcp\n", getServerURL())
	fmt.Printf("  Web UI:  %s\n", getServerURL())
	fmt.Println()
	if len(selectedTools) > 0 {
		fmt.Println("  Configured tools:")
		for _, t := range selectedTools {
			fmt.Printf("    %s %s\n", colorize(colorGreen, "✓"), t)
		}
	}
	fmt.Println()
	printInfo("Start a new AI session to use Contextify memory.")
	fmt.Println()

	return nil
}

func selectTools(cmd *cobra.Command) ([]toolconfig.ToolName, error) {
	toolsFlag, _ := cmd.Flags().GetStringSlice("tools")
	allFlag, _ := cmd.Flags().GetBool("all")

	if allFlag {
		detected := toolconfig.DetectInstalledTools()
		return detected, nil
	}

	if len(toolsFlag) > 0 {
		var selected []toolconfig.ToolName
		for _, name := range toolsFlag {
			t := toolconfig.ToolByName(name)
			if t == nil {
				return nil, fmt.Errorf("unknown tool: %s. Available: %s", name, strings.Join(toolconfig.ValidToolNames(), ", "))
			}
			selected = append(selected, t.Name)
		}
		return selected, nil
	}

	// Interactive selection
	return interactiveToolSelection()
}

func interactiveToolSelection() ([]toolconfig.ToolName, error) {
	detected := toolconfig.DetectInstalledTools()
	statuses := toolconfig.CheckAllStatuses()

	fmt.Println()
	fmt.Println("  Select AI tools to configure:")
	fmt.Println()

	for i, tool := range detected {
		status := statuses[tool]
		statusIcon := colorize(colorDim, "○")
		if status == toolconfig.StatusConfigured {
			statusIcon = colorize(colorGreen, "✓")
		} else if status == toolconfig.StatusPartial {
			statusIcon = colorize(colorYellow, "◐")
		}

		t := toolconfig.ToolByName(string(tool))
		label := string(tool)
		if t != nil {
			label = t.Label
		}

		fmt.Printf("  %s %d) %s\n", statusIcon, i+1, label)
	}
	fmt.Println()
	fmt.Printf("  Enter numbers separated by spaces (e.g., 1 2 3), or 'all': ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return nil, nil
	}

	if strings.ToLower(input) == "all" {
		return detected, nil
	}

	var selected []toolconfig.ToolName
	for _, part := range strings.Fields(input) {
		var idx int
		if _, err := fmt.Sscanf(part, "%d", &idx); err != nil || idx < 1 || idx > len(detected) {
			continue
		}
		selected = append(selected, detected[idx-1])
	}

	return selected, nil
}

func configureTool(tool toolconfig.ToolName, mcpURL string) error {
	switch tool {
	case toolconfig.ToolClaudeCode:
		return toolconfig.ConfigureClaudeCode(mcpURL)
	case toolconfig.ToolClaudeDesktop:
		return toolconfig.ConfigureClaudeDesktop(mcpURL)
	case toolconfig.ToolClaudeChat:
		if err := toolconfig.ConfigureClaudeChat(mcpURL); err != nil {
			return err
		}
		// Claude Chat requires manual setup via claude.ai UI
		printInfo("Claude Chat requires manual setup:")
		fmt.Println("    1. Go to https://claude.ai → Settings → Connectors")
		fmt.Println("    2. Click 'Add custom connector'")
		fmt.Printf("    3. Enter MCP URL: %s\n", mcpURL)
		fmt.Println("    4. Enable the connector in each conversation via '+' → 'Connectors'")
		return nil
	case toolconfig.ToolCursor:
		return toolconfig.ConfigureCursor(mcpURL)
	case toolconfig.ToolWindsurf:
		return toolconfig.ConfigureWindsurf(mcpURL)
	case toolconfig.ToolGemini:
		return toolconfig.ConfigureGemini()
	}
	return fmt.Errorf("unknown tool: %s", tool)
}

func runSelfTest(ctx context.Context, c *client.Client) error {
	agent := "cli-selftest"
	// Store
	printStep("Storing test memory...")
	mem, err := c.StoreMemory(ctx, client.StoreRequest{
		Title:       "Self-test memory",
		Content:     "This is a test memory created by contextify install",
		Type:        "general",
		Scope:       "global",
		Importance:  0.3,
		Tags:        []string{"test"},
		AgentSource: &agent,
	})
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}

	// Get
	printStep("Retrieving test memory...")
	_, err = c.GetMemory(ctx, mem.ID)
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}

	// Recall
	printStep("Searching test memory...")
	results, err := c.Recall(ctx, client.SearchRequest{
		Query: "self-test memory",
		Limit: 5,
	})
	if err != nil {
		return fmt.Errorf("recall: %w", err)
	}
	_ = results

	// Delete
	printStep("Cleaning up test memory...")
	if err := c.DeleteMemory(ctx, mem.ID); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	return nil
}

func waitForHealthWithProgress(c *client.Client, maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	ctx := context.Background()
	for time.Now().Before(deadline) {
		if err := c.Health(ctx); err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("health check timed out after %s", maxWait)
}
