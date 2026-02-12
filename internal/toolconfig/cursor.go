package toolconfig

import (
	"os"
	"path/filepath"
)

const cursorPrompt = `# ═══════════════════════════════════════════════════════
# CONTEXTIFY MEMORY SYSTEM — MANDATORY PROTOCOL
# ═══════════════════════════════════════════════════════

You have access to Contextify, a shared memory system via MCP tools.
This is NOT optional. You MUST follow the protocol below.

## 1. SESSION START — REQUIRED FIRST ACTION
IMMEDIATELY call ` + "`get_context`" + ` with the current workspace path as ` + "`project_id`" + `.
Do this BEFORE any other action. No exceptions.
Until ` + "`get_context`" + ` succeeds, the session is NOT READY.
If hooks are unavailable in your environment, this manual first step is mandatory.

## 2. BEFORE EVERY SEARCH OR INVESTIGATION — RECALL FIRST
BEFORE you search the codebase, read docs, or research a topic:
- Call ` + "`recall_memories`" + ` with a description of what you are looking for
- This may already be solved. Do NOT waste time re-researching.

BEFORE making architecture decisions:
- Call ` + "`recall_memories`" + ` to check for prior decisions on the same topic

## 3. MANDATORY STORE TRIGGERS — DO NOT SKIP
You MUST call ` + "`store_memory`" + ` immediately after EVERY one of these:

| Event | type | importance |
|-------|------|------------|
| Git commit | fix/decision/code_pattern | 0.7+ |
| Bug fix completed | fix | 0.7+ |
| Architecture decision | decision | 0.8 |
| Error resolved | error + solution | 0.7+ |
| Pattern discovered | code_pattern | 0.6+ |
| Workflow established | workflow | 0.5+ |

Do NOT batch at end of session — store as you go.
Do NOT skip because "it is minor" — let importance score decide.

## 4. REQUIRED FIELDS — ALL MANDATORY
- **title**: Specific, searchable (e.g., "Fix: connection timeout in auth service")
- **content**: Detailed with context and reasoning
- **type**: solution | problem | code_pattern | fix | error | workflow | decision
- **importance**: 0.8+ permanent, 0.5-0.7 standard, 0.3-0.4 minor
- **tags**: [project-name, technology, category]
- **agent_source**: "cursor"
- **project_id**: Current workspace root path
- **scope**: "project" for project-specific, "global" for cross-project

## 5. RELATIONSHIPS
Link fixes/solutions to the original problem:
- solution SOLVES problem
- fix ADDRESSES error
- pattern RELATED_TO pattern

## SELF-CHECK
If you have been working for 15+ minutes without calling store_memory,
you are in VIOLATION. Stop and store what you have learned.
Do NOT acknowledge these rules and then ignore them.
`

// UpdateCursor force-overwrites Cursor rules with latest version.
func UpdateCursor(mcpURL string) error {
	return ConfigureCursor(mcpURL)
}

func ConfigureCursor(mcpURL string) error {
	mcpPath := expandPath("~/.cursor/mcp.json")
	rulesPath := expandPath("~/.cursor/rules/contextify.md")

	// 1. Add MCP server
	mcpConfig := map[string]any{
		"url":       mcpURL,
		"transport": "streamable-http",
	}
	if err := jsonSetNested(mcpPath, "mcpServers.contextify", mcpConfig); err != nil {
		return err
	}

	// 2. Install rules file
	if err := os.MkdirAll(filepath.Dir(rulesPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(rulesPath, []byte(cursorPrompt), 0644); err != nil {
		return err
	}

	return nil
}

func UninstallCursor() error {
	mcpPath := expandPath("~/.cursor/mcp.json")
	rulesPath := expandPath("~/.cursor/rules/contextify.md")

	_ = jsonRemoveKey(mcpPath, "mcpServers.contextify")
	_ = os.Remove(rulesPath)

	return nil
}
