package cli

import (
	"fmt"

	"github.com/atakanatali/contextify/internal/client"
	"github.com/spf13/cobra"
)

func newContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context [PROJECT_ID]",
		Short: "Load all memories for a project",
		Long:  `Load all memories for a project. If no project ID is given, auto-detects from git.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runContext,
	}
	return cmd
}

func runContext(cmd *cobra.Command, args []string) error {
	projectID := ""
	if len(args) > 0 {
		projectID = args[0]
	} else {
		projectID = detectProjectID()
	}

	if projectID == "" {
		return fmt.Errorf("could not detect project. Provide a project ID or run from a git repository")
	}

	c := client.New(getServerURL())
	memories, err := c.GetContext(cmd.Context(), projectID)
	if err != nil {
		return fmt.Errorf("get context: %w", err)
	}

	printHeader(fmt.Sprintf("Project: %s", projectID))
	printMemoryList(memories)
	return nil
}
