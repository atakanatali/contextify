package cli

import (
	"fmt"

	"github.com/atakanatali/contextify/internal/client"
	"github.com/atakanatali/contextify/internal/docker"
	"github.com/atakanatali/contextify/internal/toolconfig"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Contextify status and configuration",
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	printHeader("Contextify Status")

	mgr := docker.NewManager(getContainerName(), getDockerImage(), getPort())

	// Container status
	status, _ := mgr.Status(cmd.Context())
	if !status.Exists {
		printFail("Container not found. Run 'contextify install' to set up.")
	} else if !status.Running {
		printWarn(fmt.Sprintf("Container exists but stopped (%s)", status.Status))
	} else {
		printOK(fmt.Sprintf("Container running (%s)", status.Image))
	}

	// Health check
	if status.Running {
		c := client.New(getServerURL())
		if err := c.Health(cmd.Context()); err != nil {
			printWarn("Server is not healthy yet")
		} else {
			printOK(fmt.Sprintf("Server healthy at %s", getServerURL()))

			// Show stats if healthy
			stats, err := c.GetStats(cmd.Context())
			if err == nil {
				fmt.Printf("    Memories: %d total (%d long-term, %d short-term)\n",
					stats.TotalMemories, stats.LongTermCount, stats.ShortTermCount)
			}
		}
	}

	// Tool configurations
	fmt.Println()
	fmt.Println(colorize(colorBold, "  Tool Configurations"))
	fmt.Println(colorize(colorDim, "  ─────────────────"))

	statuses := toolconfig.CheckAllStatuses()
	for _, tool := range toolconfig.AllTools {
		s := statuses[tool.Name]
		icon := colorize(colorDim, "○")
		label := "not configured"
		switch s {
		case toolconfig.StatusConfigured:
			icon = colorize(colorGreen, "✓")
			label = "configured"
		case toolconfig.StatusPartial:
			icon = colorize(colorYellow, "◐")
			label = "partial"
		}
		fmt.Printf("  %s %-14s %s\n", icon, tool.Label, colorize(colorDim, label))
	}

	fmt.Println()
	return nil
}
