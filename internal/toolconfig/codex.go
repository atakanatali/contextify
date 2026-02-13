package toolconfig

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const codexInstructions = `# Contextify Memory System — Codex

You have access to Contextify MCP tools in Codex.

## Session Protocol
1. First action in each session: call ` + "`get_context`" + ` with current workspace path as ` + "`project_id`" + `.
2. Before new research or deep code search: call ` + "`recall_memories`" + `.
3. After bug fixes, decisions, and commits: call ` + "`store_memory`" + ` immediately.

## Required Memory Fields
- ` + "`title`" + `: specific and searchable
- ` + "`content`" + `: what changed and why
- ` + "`type`" + `: solution | problem | code_pattern | fix | error | workflow | decision
- ` + "`importance`" + `: 0.0-1.0
- ` + "`agent_source`" + `: "codex"
- ` + "`project_id`" + `: current workspace path
- ` + "`scope`" + `: "project" or "global"
- ` + "`tags`" + `: project, tech, feature keywords

## Recall-First Behavior
- Call ` + "`recall_memories`" + ` before architecture changes or dependency migrations.
- Prefer updating existing memory relationships over creating isolated notes.
`

// ConfigureCodex configures Codex CLI with the Contextify MCP server and instructions.
func ConfigureCodex(mcpURL string) error {
	return configureCodex(mcpURL, false)
}

// UpdateCodex refreshes Codex MCP configuration and force-overwrites instructions.
func UpdateCodex(mcpURL string) error {
	return configureCodex(mcpURL, true)
}

func configureCodex(mcpURL string, force bool) error {
	if !codexInstalled() {
		return fmt.Errorf("codex CLI not found in PATH")
	}

	if force {
		_ = codexRemoveContextifyMCP()
	}

	if !codexHasContextifyMCP() {
		if err := codexAddContextifyMCP(mcpURL); err != nil {
			return err
		}
	}

	instrPath := expandPath("~/.contextify/codex-instructions.md")
	if err := os.MkdirAll(filepath.Dir(instrPath), 0755); err != nil {
		return err
	}
	if force || !fileExists(instrPath) {
		if err := os.WriteFile(instrPath, []byte(codexInstructions), 0644); err != nil {
			return err
		}
	}

	return nil
}

// UninstallCodex removes Contextify configuration from Codex.
func UninstallCodex() error {
	if codexInstalled() {
		_ = codexRemoveContextifyMCP()
	}
	_ = os.Remove(expandPath("~/.contextify/codex-instructions.md"))
	return nil
}

func codexInstalled() bool {
	_, err := exec.LookPath("codex")
	return err == nil
}

func codexHasContextifyMCP() bool {
	if !codexInstalled() {
		return false
	}
	cmd := exec.Command("codex", "mcp", "get", "contextify", "--json")
	return cmd.Run() == nil
}

func codexAddContextifyMCP(mcpURL string) error {
	cmd := exec.Command("codex", "mcp", "add", "contextify", "--url", mcpURL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("configure codex MCP server: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func codexRemoveContextifyMCP() error {
	cmd := exec.Command("codex", "mcp", "remove", "contextify")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	msg := strings.ToLower(string(out))
	if strings.Contains(msg, "no mcp server") || strings.Contains(msg, "not found") || strings.Contains(msg, "does not exist") {
		return nil
	}

	return fmt.Errorf("remove codex MCP server: %w (%s)", err, strings.TrimSpace(string(out)))
}
