package cli

import (
	"fmt"

	"github.com/atakanatali/contextify/internal/client"
	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [QUERY]",
		Short: "Search memories with filters",
		Long:  `Search memories with advanced filters. Query is optional when using filters.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runSearch,
	}
	cmd.Flags().StringP("type", "t", "", "Filter by memory type")
	cmd.Flags().StringP("scope", "s", "", "Filter by scope (global|project)")
	cmd.Flags().StringSliceP("tags", "T", nil, "Filter by tags")
	cmd.Flags().Float32("min-importance", 0, "Minimum importance threshold")
	cmd.Flags().StringP("project", "p", "", "Filter by project ID")
	cmd.Flags().StringP("agent", "a", "", "Filter by agent source")
	cmd.Flags().IntP("limit", "l", 20, "Maximum number of results")
	return cmd
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	memType, _ := cmd.Flags().GetString("type")
	scope, _ := cmd.Flags().GetString("scope")
	tags, _ := cmd.Flags().GetStringSlice("tags")
	minImp, _ := cmd.Flags().GetFloat32("min-importance")
	project, _ := cmd.Flags().GetString("project")
	agent, _ := cmd.Flags().GetString("agent")
	limit, _ := cmd.Flags().GetInt("limit")

	req := client.SearchRequest{
		Query: query,
		Tags:  tags,
		Limit: limit,
	}
	if memType != "" {
		req.Type = &memType
	}
	if scope != "" {
		req.Scope = &scope
	}
	if project != "" {
		req.ProjectID = &project
	}
	if agent != "" {
		req.AgentSource = &agent
	}
	if minImp > 0 {
		req.MinImportance = &minImp
	}

	c := client.New(getServerURL())
	results, err := c.Search(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	printSearchResults(results)
	return nil
}
