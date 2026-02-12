package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/atakanatali/contextify/internal/client"
	"github.com/spf13/cobra"
)

func newStoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "store TITLE",
		Short: "Store a new memory",
		Long: `Store a new memory in Contextify.

Content can be provided via --content flag or piped from stdin:
  contextify store "Bug fix" --content "Fixed the timeout issue"
  cat error.log | contextify store "Error log" --type error`,
		Args: cobra.ExactArgs(1),
		RunE: runStore,
	}
	cmd.Flags().StringP("type", "t", "general", "Memory type (solution|problem|code_pattern|fix|error|workflow|decision|general)")
	cmd.Flags().Float32P("importance", "i", 0.5, "Importance score (0.0-1.0)")
	cmd.Flags().StringSliceP("tags", "T", nil, "Tags (comma-separated)")
	cmd.Flags().StringP("content", "c", "", "Memory content")
	cmd.Flags().StringP("scope", "s", "project", "Scope (global|project)")
	cmd.Flags().StringP("project", "p", "", "Project ID (auto-detected from git)")
	cmd.Flags().StringP("agent", "a", "cli", "Agent source identifier")
	return cmd
}

func runStore(cmd *cobra.Command, args []string) error {
	title := args[0]
	content, _ := cmd.Flags().GetString("content")
	memType, _ := cmd.Flags().GetString("type")
	importance, _ := cmd.Flags().GetFloat32("importance")
	tags, _ := cmd.Flags().GetStringSlice("tags")
	scope, _ := cmd.Flags().GetString("scope")
	project, _ := cmd.Flags().GetString("project")
	agent, _ := cmd.Flags().GetString("agent")

	// Read content from stdin if not provided
	if content == "" {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			scanner := bufio.NewScanner(os.Stdin)
			var lines []string
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}
			content = strings.Join(lines, "\n")
		}
	}

	if content == "" {
		content = title
	}

	// Auto-detect project
	if project == "" && scope == "project" {
		project = detectProjectID()
	}

	req := client.StoreRequest{
		Title:      title,
		Content:    content,
		Type:       memType,
		Scope:      scope,
		Importance: importance,
		Tags:       tags,
	}
	if project != "" {
		req.ProjectID = &project
	}
	if agent != "" {
		req.AgentSource = &agent
	}

	c := client.New(getServerURL())
	mem, err := c.StoreMemory(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("store memory: %w", err)
	}

	printOK(fmt.Sprintf("Memory stored: %s", mem.ID))
	return nil
}
