package cli

import (
	"fmt"

	"github.com/atakanatali/contextify/internal/client"
	"github.com/spf13/cobra"
)

func newRecallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recall QUERY",
		Short: "Semantic search for memories",
		Long:  `Search memories using natural language. Uses hybrid vector + keyword matching.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runRecall,
	}
	cmd.Flags().IntP("limit", "l", 10, "Maximum number of results")
	cmd.Flags().StringP("project", "p", "", "Filter by project ID")
	cmd.Flags().StringSliceP("tags", "T", nil, "Filter by tags")
	cmd.Flags().StringP("type", "t", "", "Filter by memory type")
	return cmd
}

func runRecall(cmd *cobra.Command, args []string) error {
	query := args[0]
	limit, _ := cmd.Flags().GetInt("limit")
	project, _ := cmd.Flags().GetString("project")
	tags, _ := cmd.Flags().GetStringSlice("tags")
	memType, _ := cmd.Flags().GetString("type")

	req := client.SearchRequest{
		Query: query,
		Limit: limit,
		Tags:  tags,
	}
	if project != "" {
		req.ProjectID = &project
	}
	if memType != "" {
		req.Type = &memType
	}

	c := client.New(getServerURL())
	results, err := c.Recall(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("recall: %w", err)
	}

	printSearchResults(results)
	return nil
}
