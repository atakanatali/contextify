package toolconfig

import (
	"os/exec"
	"runtime"
	"time"
)

// RestartTool attempts to restart an AI tool after configuration changes.
func RestartTool(tool ToolName) error {
	switch tool {
	case ToolCursor:
		return restartApp("Cursor")
	case ToolWindsurf:
		return restartApp("Windsurf")
	case ToolClaudeCode:
		// Claude Code has no programmatic restart; user must start a new session
		return nil
	case ToolGemini:
		// API-based, no restart needed
		return nil
	}
	return nil
}

// IsToolRunning checks if an AI tool process is currently running.
func IsToolRunning(tool ToolName) bool {
	switch tool {
	case ToolCursor:
		return isProcessRunning("Cursor")
	case ToolWindsurf:
		return isProcessRunning("Windsurf")
	default:
		return false
	}
}

func restartApp(name string) error {
	if !isProcessRunning(name) {
		return nil
	}

	// Kill the process
	_ = exec.Command("pkill", "-f", name).Run()
	time.Sleep(2 * time.Second)

	// Reopen on macOS
	if runtime.GOOS == "darwin" {
		return exec.Command("open", "-a", name).Start()
	}

	return nil
}

func isProcessRunning(name string) bool {
	err := exec.Command("pgrep", "-f", name).Run()
	return err == nil
}
