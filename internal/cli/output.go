package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/atakanatali/contextify/internal/client"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorDim    = "\033[2m"
	colorBold   = "\033[1m"
)

func isColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return true
}

func colorize(color, text string) string {
	if !isColorEnabled() {
		return text
	}
	return color + text + colorReset
}

func printOK(msg string) {
	fmt.Println(colorize(colorGreen, "  ✓ ") + msg)
}

func printWarn(msg string) {
	fmt.Println(colorize(colorYellow, "  ! ") + msg)
}

func printFail(msg string) {
	fmt.Println(colorize(colorRed, "  ✗ ") + msg)
}

func printInfo(msg string) {
	fmt.Println(colorize(colorBlue, "  → ") + msg)
}

func printStep(msg string) {
	fmt.Println(colorize(colorCyan, "  ▸ ") + msg)
}

func printHeader(msg string) {
	fmt.Println()
	fmt.Println(colorize(colorBold, "  "+msg))
	fmt.Println(colorize(colorDim, "  "+strings.Repeat("─", len(msg)+2)))
}

func printMemory(m *client.Memory) {
	fmt.Println()
	fmt.Printf("  %s %s\n", colorize(colorBold, m.Title), colorize(colorDim, m.ID))
	fmt.Printf("  Type: %s  Scope: %s  Importance: %.1f\n", m.Type, m.Scope, m.Importance)
	if len(m.Tags) > 0 {
		fmt.Printf("  Tags: %s\n", strings.Join(m.Tags, ", "))
	}
	if m.ProjectID != nil {
		fmt.Printf("  Project: %s\n", *m.ProjectID)
	}
	if m.AgentSource != nil {
		fmt.Printf("  Agent: %s\n", *m.AgentSource)
	}
	fmt.Printf("  Access count: %d  Created: %s\n", m.AccessCount, formatTime(m.CreatedAt))
	if m.ExpiresAt != nil {
		fmt.Printf("  Expires: %s\n", formatTime(*m.ExpiresAt))
	}
	fmt.Println()
	fmt.Println(colorize(colorDim, "  ─────"))
	// Print content with indentation
	for _, line := range strings.Split(m.Content, "\n") {
		fmt.Printf("  %s\n", line)
	}
	fmt.Println(colorize(colorDim, "  ─────"))
}

func printMemoryList(memories []client.Memory) {
	if len(memories) == 0 {
		printWarn("No memories found.")
		return
	}
	for i, m := range memories {
		typeColor := colorBlue
		switch m.Type {
		case "fix", "solution":
			typeColor = colorGreen
		case "error", "problem":
			typeColor = colorRed
		case "decision":
			typeColor = colorYellow
		}
		fmt.Printf("  %s%-12s%s %s %s\n",
			typeColor, m.Type, colorReset,
			colorize(colorBold, m.Title),
			colorize(colorDim, m.ID),
		)
		if i < len(memories)-1 {
			// no separator needed, spacing is sufficient
		}
	}
	fmt.Printf("\n  %s %d memories\n", colorize(colorDim, "Total:"), len(memories))
}

func printSearchResults(results []client.SearchResult) {
	if len(results) == 0 {
		printWarn("No results found.")
		return
	}
	for _, r := range results {
		score := fmt.Sprintf("%.2f", r.Score)
		typeColor := colorBlue
		switch r.Memory.Type {
		case "fix", "solution":
			typeColor = colorGreen
		case "error", "problem":
			typeColor = colorRed
		}
		fmt.Printf("  %s%-12s%s %s  %s  %s\n",
			typeColor, r.Memory.Type, colorReset,
			colorize(colorBold, r.Memory.Title),
			colorize(colorDim, "score:"+score),
			colorize(colorDim, r.Memory.ID),
		)
	}
	fmt.Printf("\n  %s %d results\n", colorize(colorDim, "Total:"), len(results))
}

func printStats(stats *client.Stats) {
	printHeader("Contextify Stats")
	fmt.Printf("  Total memories:    %d\n", stats.TotalMemories)
	fmt.Printf("  Long-term:         %s\n", colorize(colorGreen, fmt.Sprintf("%d", stats.LongTermCount)))
	fmt.Printf("  Short-term:        %s\n", colorize(colorYellow, fmt.Sprintf("%d", stats.ShortTermCount)))
	fmt.Printf("  Expiring soon:     %s\n", colorize(colorRed, fmt.Sprintf("%d", stats.ExpiringCount)))

	if len(stats.ByType) > 0 {
		fmt.Println()
		fmt.Println(colorize(colorDim, "  By type:"))
		for t, count := range stats.ByType {
			fmt.Printf("    %-16s %d\n", t, count)
		}
	}
	if len(stats.ByAgent) > 0 {
		fmt.Println()
		fmt.Println(colorize(colorDim, "  By agent:"))
		for a, count := range stats.ByAgent {
			fmt.Printf("    %-16s %d\n", a, count)
		}
	}
	fmt.Println()
}

func formatTime(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04")
}
