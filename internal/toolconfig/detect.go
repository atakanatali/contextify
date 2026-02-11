package toolconfig

import (
	"os"
	"path/filepath"
	"strings"
)

func home() string {
	h, _ := os.UserHomeDir()
	return h
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		return filepath.Join(home(), path[1:])
	}
	return path
}

// DetectInstalledTools returns which AI tools are installed on the system.
func DetectInstalledTools() []ToolName {
	var installed []ToolName

	// Claude Code: check if ~/.claude/ exists
	if dirExists(expandPath("~/.claude")) {
		installed = append(installed, ToolClaudeCode)
	}

	// Cursor: check if ~/.cursor/ exists
	if dirExists(expandPath("~/.cursor")) {
		installed = append(installed, ToolCursor)
	}

	// Windsurf: check if ~/.codeium/windsurf/ exists
	if dirExists(expandPath("~/.codeium/windsurf")) {
		installed = append(installed, ToolWindsurf)
	}

	// Gemini: always available (REST API based)
	installed = append(installed, ToolGemini)

	return installed
}

// CheckStatus checks the configuration status of a specific tool.
func CheckStatus(tool ToolName) ToolStatus {
	switch tool {
	case ToolClaudeCode:
		return checkClaudeCodeStatus()
	case ToolCursor:
		return checkCursorStatus()
	case ToolWindsurf:
		return checkWindsurfStatus()
	case ToolGemini:
		return checkGeminiStatus()
	}
	return StatusNotConfigured
}

// CheckAllStatuses returns the status of all tools.
func CheckAllStatuses() map[ToolName]ToolStatus {
	statuses := make(map[ToolName]ToolStatus)
	for _, t := range AllTools {
		statuses[t.Name] = CheckStatus(t.Name)
	}
	return statuses
}

func checkClaudeCodeStatus() ToolStatus {
	settingsPath := expandPath("~/.claude/settings.json")
	claudeMDPath := expandPath("~/.claude/CLAUDE.md")
	hooksDir := expandPath("~/.contextify/hooks")

	hasMCP := jsonHasKey(settingsPath, "mcpServers.contextify")
	hasClaudeMD := fileContains(claudeMDPath, "<!-- contextify-memory-system -->")
	hasHooks := fileExists(filepath.Join(hooksDir, "session-start.sh"))

	if hasMCP && hasClaudeMD && hasHooks {
		return StatusConfigured
	}
	if hasMCP || hasClaudeMD || hasHooks {
		return StatusPartial
	}
	return StatusNotConfigured
}

func checkCursorStatus() ToolStatus {
	mcpPath := expandPath("~/.cursor/mcp.json")
	rulesPath := expandPath("~/.cursor/rules/contextify.md")

	hasMCP := jsonHasKey(mcpPath, "mcpServers.contextify")
	hasRules := fileExists(rulesPath)

	if hasMCP && hasRules {
		return StatusConfigured
	}
	if hasMCP || hasRules {
		return StatusPartial
	}
	return StatusNotConfigured
}

func checkWindsurfStatus() ToolStatus {
	mcpPath := expandPath("~/.codeium/windsurf/mcp_config.json")
	rulesPath := expandPath("~/.codeium/windsurf/memories/contextify.md")

	hasMCP := jsonHasKey(mcpPath, "mcpServers.contextify")
	hasRules := fileExists(rulesPath)

	if hasMCP && hasRules {
		return StatusConfigured
	}
	if hasMCP || hasRules {
		return StatusPartial
	}
	return StatusNotConfigured
}

func checkGeminiStatus() ToolStatus {
	instrPath := expandPath("~/.contextify/gemini-instructions.md")
	if fileExists(instrPath) {
		return StatusConfigured
	}
	return StatusNotConfigured
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileContains(path, substring string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), substring)
}
