package cli

import (
	"fmt"

	"github.com/atakanatali/contextify/internal/client"
	"github.com/spf13/cobra"
)

func newPromoteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "promote ID",
		Short: "Promote a memory to permanent storage",
		Args:  cobra.ExactArgs(1),
		RunE:  runPromote,
	}
}

func runPromote(cmd *cobra.Command, args []string) error {
	c := client.New(getServerURL())
	mem, getErr := c.GetMemory(cmd.Context(), args[0])
	resp, err := c.PromoteMemory(cmd.Context(), args[0])
	if err != nil {
		return fmt.Errorf("promote memory: %w", err)
	}

	if getErr == nil && mem != nil {
		printOK(fmt.Sprintf("Memory promoted to permanent: %s (%s)", mem.Title, resp.ID))
		return nil
	}
	printOK(fmt.Sprintf("Memory promoted to permanent: %s", resp.ID))
	return nil
}
