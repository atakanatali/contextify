package cli

import (
	"fmt"

	"github.com/atakanatali/contextify/internal/client"
	"github.com/spf13/cobra"
)

func newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get ID",
		Short: "Get a memory by ID",
		Args:  cobra.ExactArgs(1),
		RunE:  runGet,
	}
}

func runGet(cmd *cobra.Command, args []string) error {
	c := client.New(getServerURL())
	mem, err := c.GetMemory(cmd.Context(), args[0])
	if err != nil {
		return fmt.Errorf("get memory: %w", err)
	}

	printMemory(mem)
	return nil
}
