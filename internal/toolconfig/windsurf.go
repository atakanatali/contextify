package toolconfig

import (
	"os"
	"path/filepath"
)

const windsurfPrompt = `# ═══════════════════════════════════════════════════════
# CONTEXTIFY MEMORY SYSTEM — MANDATORY PROTOCOL
# ═══════════════════════════════════════════════════════

You have access to Contextify, a shared memory system via MCP tools.
This is NOT optional. You MUST follow the protocol below.

## 1. SESSION START — REQUIRED FIRST ACTION
IMMEDIATELY call ` + "`get_context`" + ` with the current workspace path as ` + "`project_id`" + `.
Do this BEFORE any other action. No exceptions.

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
- **agent_source**: "windsurf"
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

// UpdateWindsurf force-overwrites Windsurf rules with latest version.
func UpdateWindsurf(mcpURL string) error {
	return ConfigureWindsurf(mcpURL)
}

func ConfigureWindsurf(mcpURL string) error {
	mcpPath := expandPath("~/.codeium/windsurf/mcp_config.json")
	rulesPath := expandPath("~/.codeium/windsurf/memories/contextify.md")

	// 1. Add MCP server (Windsurf uses serverUrl, not url)
	mcpConfig := map[string]any{
		"serverUrl": mcpURL,
	}
	if err := jsonSetNested(mcpPath, "mcpServers.contextify", mcpConfig); err != nil {
		return err
	}

	// 2. Install rules file
	if err := os.MkdirAll(filepath.Dir(rulesPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(rulesPath, []byte(windsurfPrompt), 0644); err != nil {
		return err
	}

	return nil
}

func UninstallWindsurf() error {
	mcpPath := expandPath("~/.codeium/windsurf/mcp_config.json")
	rulesPath := expandPath("~/.codeium/windsurf/memories/contextify.md")

	_ = jsonRemoveKey(mcpPath, "mcpServers.contextify")
	_ = os.Remove(rulesPath)

	return nil
}
