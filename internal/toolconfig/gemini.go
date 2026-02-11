package toolconfig

import (
	"os"
	"path/filepath"
)

const geminiPrompt = `# Contextify Memory System

You have access to Contextify, a shared AI memory system at http://localhost:8420.

## REST API Endpoints

### Load Context (do this at session start)
` + "```" + `
POST http://localhost:8420/api/v1/context/{project_path}
` + "```" + `

### Store Memory
` + "```" + `
POST http://localhost:8420/api/v1/memories
Content-Type: application/json

{
  "title": "Specific, searchable title",
  "content": "Detailed description with context",
  "type": "solution|problem|code_pattern|fix|error|workflow|decision|general",
  "scope": "project|global",
  "project_id": "/path/to/project",
  "importance": 0.7,
  "tags": ["project-name", "technology", "category"],
  "agent_source": "gemini"
}
` + "```" + `

### Semantic Search
` + "```" + `
POST http://localhost:8420/api/v1/memories/recall
Content-Type: application/json

{"query": "natural language description of what you need", "limit": 20}
` + "```" + `

### Advanced Search with Filters
` + "```" + `
POST http://localhost:8420/api/v1/memories/search
Content-Type: application/json

{"query": "search terms", "tags": ["filter-tag"], "type": "solution", "min_importance": 0.5}
` + "```" + `

### Create Relationship
` + "```" + `
POST http://localhost:8420/api/v1/relationships
Content-Type: application/json

{"from_memory_id": "uuid", "to_memory_id": "uuid", "relationship_type": "SOLVES", "strength": 0.8}
` + "```" + `

## When to Use
- **Session start**: Load context for current project
- **Bug fixes**: Store the problem and solution
- **Decisions**: Store architecture choices with rationale
- **Errors**: Store error patterns and resolutions
- **Before starting work**: Recall relevant memories

## Importance Guide
- 0.8+ = permanent (critical knowledge)
- 0.5-0.7 = standard (24h TTL, extended on access)
- 0.3-0.4 = temporary (expires if unused)

Always set ` + "`agent_source`" + ` to "gemini".
`

func ConfigureGemini() error {
	instrPath := expandPath("~/.contextify/gemini-instructions.md")

	if err := os.MkdirAll(filepath.Dir(instrPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(instrPath, []byte(geminiPrompt), 0644)
}

func UninstallGemini() error {
	instrPath := expandPath("~/.contextify/gemini-instructions.md")
	_ = os.Remove(instrPath)
	return nil
}
