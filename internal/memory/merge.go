package memory

import (
	"fmt"
	"time"
)

type MergeStrategy string

const (
	MergeLatestWins MergeStrategy = "latest_wins"
	MergeAppend     MergeStrategy = "append"
	MergeSmartMerge MergeStrategy = "smart_merge"
)

// mergeContent combines existing memory content with incoming content based on strategy.
func mergeContent(existing *Memory, incoming StoreRequest, strategy MergeStrategy) (title string, content string, tags []string) {
	switch strategy {
	case MergeLatestWins:
		title = incoming.Title
		content = incoming.Content
		tags = mergeTags(existing.Tags, incoming.Tags)

	case MergeAppend:
		separator := fmt.Sprintf("\n\n---\n[Updated %s]\n\n", time.Now().Format("2006-01-02"))
		title = existing.Title
		if len(incoming.Title) > len(existing.Title) {
			title = incoming.Title
		}
		content = existing.Content + separator + incoming.Content
		tags = mergeTags(existing.Tags, incoming.Tags)

	case MergeSmartMerge:
		// If incoming content is longer, it's likely a superset — use it directly.
		// Otherwise append to avoid losing information.
		if len(incoming.Content) >= len(existing.Content) {
			title = incoming.Title
			content = incoming.Content
		} else {
			title = existing.Title
			if len(incoming.Title) > len(existing.Title) {
				title = incoming.Title
			}
			content = existing.Content + "\n\n" + incoming.Content
		}
		tags = mergeTags(existing.Tags, incoming.Tags)

	default:
		title = incoming.Title
		content = incoming.Content
		tags = mergeTags(existing.Tags, incoming.Tags)
	}

	return title, content, tags
}

// mergeMultipleContents combines content from multiple memories into a target.
func mergeMultipleContents(target *Memory, sources []*Memory, strategy MergeStrategy) (title string, content string, tags []string) {
	title = target.Title
	content = target.Content
	tags = append([]string{}, target.Tags...)

	for _, src := range sources {
		switch strategy {
		case MergeLatestWins:
			if src.UpdatedAt.After(target.UpdatedAt) {
				title = src.Title
				content = src.Content
			}

		case MergeAppend:
			separator := fmt.Sprintf("\n\n---\n[Merged from %s]\n\n", src.ID)
			content += separator + src.Content

		case MergeSmartMerge:
			// If source is longer and more recent, replace. Otherwise append.
			if len(src.Content) > len(content) && src.UpdatedAt.After(target.UpdatedAt) {
				title = src.Title
				content = src.Content
			} else if src.Content != content {
				content += "\n\n" + src.Content
			}

		default:
			content += "\n\n" + src.Content
		}

		tags = mergeTags(tags, src.Tags)
	}

	return title, content, tags
}

// mergeTags returns the union of two tag slices, preserving order.
func mergeTags(a, b []string) []string {
	seen := make(map[string]bool, len(a))
	result := make([]string, 0, len(a)+len(b))

	for _, tag := range a {
		if !seen[tag] {
			seen[tag] = true
			result = append(result, tag)
		}
	}
	for _, tag := range b {
		if !seen[tag] {
			seen[tag] = true
			result = append(result, tag)
		}
	}

	return result
}

// maxImportance returns the highest importance from a target and sources.
func maxImportance(target float32, sources []*Memory) float32 {
	max := target
	for _, src := range sources {
		if src.Importance > max {
			max = src.Importance
		}
	}
	return max
}
