package cli

import (
	"fmt"

	"github.com/atakanatali/contextify/internal/client"
	"github.com/spf13/cobra"
)

func newStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show memory system statistics",
		RunE:  runStats,
	}
}

func runStats(cmd *cobra.Command, args []string) error {
	c := client.New(getServerURL())
	stats, err := c.GetStats(cmd.Context())
	if err != nil {
		return fmt.Errorf("get stats: %w", err)
	}

	printStats(stats)
	return nil
}
