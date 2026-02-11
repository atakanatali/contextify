package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/atakanatali/contextify/internal/client"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete ID",
		Short: "Delete a memory",
		Args:  cobra.ExactArgs(1),
		RunE:  runDelete,
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

func runDelete(cmd *cobra.Command, args []string) error {
	id := args[0]
	force, _ := cmd.Flags().GetBool("force")

	c := client.New(getServerURL())

	if !force {
		// Show memory info before confirming
		mem, err := c.GetMemory(cmd.Context(), id)
		if err != nil {
			return fmt.Errorf("get memory: %w", err)
		}
		fmt.Printf("  Delete memory: %s (%s)\n", mem.Title, mem.ID)
		fmt.Print("  Are you sure? [y/N] ")

		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("  Cancelled.")
			return nil
		}
	}

	if err := c.DeleteMemory(cmd.Context(), id); err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}

	printOK("Memory deleted.")
	return nil
}
