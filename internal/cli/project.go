package cli

import (
	"os"
	"os/exec"
	"strings"
)

func detectProjectID() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}
