package toolconfig

import (
	"os"
	"path/filepath"
	"runtime"
)

const claudeDesktopPrompt = `# ═══════════════════════════════════════════════════════
# CONTEXTIFY MEMORY SYSTEM — MANDATORY PROTOCOL
# ═══════════════════════════════════════════════════════

You have access to Contextify, a shared memory system via MCP tools.
This is NOT optional. You MUST follow the protocol below.

## 1. SESSION START — REQUIRED FIRST ACTION
IMMEDIATELY call ` + "`get_context`" + ` with the current workspace/folder path as ` + "`project_id`" + `.
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
| Task completed | fix/decision/code_pattern | 0.7+ |
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
- **agent_source**: "claude-desktop"
- **project_id**: Current workspace/folder path
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

// claudeDesktopConfigPath returns the path to Claude Desktop's config file.
func claudeDesktopConfigPath() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home(), "Library", "Application Support", "Claude", "claude_desktop_config.json")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home(), "AppData", "Roaming")
		}
		return filepath.Join(appData, "Claude", "claude_desktop_config.json")
	default: // linux
		return filepath.Join(home(), ".config", "Claude", "claude_desktop_config.json")
	}
}

// ConfigureClaudeDesktop sets up Claude Desktop / Cowork for Contextify.
// Uses npx mcp-remote to bridge HTTP MCP to stdio for Claude Desktop.
func ConfigureClaudeDesktop(mcpURL string) error {
	return configureClaudeDesktop(mcpURL, false)
}

// UpdateClaudeDesktop force-overwrites Claude Desktop config with latest version.
func UpdateClaudeDesktop(mcpURL string) error {
	return configureClaudeDesktop(mcpURL, true)
}

func configureClaudeDesktop(mcpURL string, _ bool) error {
	configPath := claudeDesktopConfigPath()

	// Claude Desktop only supports stdio transport via claude_desktop_config.json.
	// Use npx mcp-remote to bridge our HTTP MCP endpoint to stdio.
	mcpConfig := map[string]any{
		"command": "npx",
		"args":    []any{"mcp-remote", mcpURL},
	}
	if err := jsonSetNested(configPath, "mcpServers.contextify", mcpConfig); err != nil {
		return err
	}

	// Install instructions file for Claude Desktop/Cowork
	instrPath := expandPath("~/.contextify/claude-desktop-instructions.md")
	if err := os.MkdirAll(filepath.Dir(instrPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(instrPath, []byte(claudeDesktopPrompt), 0644); err != nil {
		return err
	}

	return nil
}

// UninstallClaudeDesktop removes Contextify configuration from Claude Desktop.
func UninstallClaudeDesktop() error {
	configPath := claudeDesktopConfigPath()
	_ = jsonRemoveKey(configPath, "mcpServers.contextify")
	_ = os.Remove(expandPath("~/.contextify/claude-desktop-instructions.md"))
	return nil
}

func checkClaudeDesktopStatus() ToolStatus {
	configPath := claudeDesktopConfigPath()
	instrPath := expandPath("~/.contextify/claude-desktop-instructions.md")

	hasMCP := jsonHasKey(configPath, "mcpServers.contextify")
	hasInstr := fileExists(instrPath)

	if hasMCP && hasInstr {
		return StatusConfigured
	}
	if hasMCP || hasInstr {
		return StatusPartial
	}
	return StatusNotConfigured
}
