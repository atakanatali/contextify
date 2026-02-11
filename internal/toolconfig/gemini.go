package toolconfig

import (
	"os"
	"path/filepath"
)

const geminiPrompt = `# ═══════════════════════════════════════════════════════
# CONTEXTIFY MEMORY SYSTEM — MANDATORY PROTOCOL
# ═══════════════════════════════════════════════════════

You have access to Contextify, a shared AI memory system at http://localhost:8420.
This is NOT optional. You MUST follow the protocol below.

## 1. SESSION START — REQUIRED FIRST ACTION
IMMEDIATELY load context for the current project:
` + "```" + `
POST http://localhost:8420/api/v1/context/{project_path}
` + "```" + `
Do this BEFORE any other action. No exceptions.

## 2. BEFORE EVERY SEARCH OR INVESTIGATION — RECALL FIRST
BEFORE you research a topic, call recall to check if it is already solved:
` + "```" + `
POST http://localhost:8420/api/v1/memories/recall
Content-Type: application/json

{"query": "natural language description of what you need", "limit": 20}
` + "```" + `
Do NOT waste time re-researching solved problems.

Advanced search with filters:
` + "```" + `
POST http://localhost:8420/api/v1/memories/search
Content-Type: application/json

{"query": "search terms", "tags": ["filter-tag"], "type": "solution", "min_importance": 0.5}
` + "```" + `

## 3. MANDATORY STORE TRIGGERS — DO NOT SKIP
You MUST store a memory immediately after EVERY one of these:

| Event | type | importance |
|-------|------|------------|
| Git commit | fix/decision/code_pattern | 0.7+ |
| Bug fix completed | fix | 0.7+ |
| Architecture decision | decision | 0.8 |
| Error resolved | error + solution | 0.7+ |
| Pattern discovered | code_pattern | 0.6+ |

` + "```" + `
POST http://localhost:8420/api/v1/memories
Content-Type: application/json

{
  "title": "Specific, searchable title",
  "content": "Detailed description with context and reasoning",
  "type": "solution|problem|code_pattern|fix|error|workflow|decision",
  "scope": "project|global",
  "project_id": "/path/to/project",
  "importance": 0.7,
  "tags": ["project-name", "technology", "category"],
  "agent_source": "gemini"
}
` + "```" + `

Do NOT batch at end of session — store as you go.

## 4. RELATIONSHIPS
Link fixes/solutions to the original problem:
` + "```" + `
POST http://localhost:8420/api/v1/relationships
Content-Type: application/json

{"from_memory_id": "uuid", "to_memory_id": "uuid", "relationship_type": "SOLVES", "strength": 0.8}
` + "```" + `

## SELF-CHECK
If you have been working for 15+ minutes without storing a memory,
you are in VIOLATION. Stop and store what you have learned.
Do NOT acknowledge these rules and then ignore them.
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
