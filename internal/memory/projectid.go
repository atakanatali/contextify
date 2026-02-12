package memory

import (
	"bufio"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// ProjectNormalizer resolves raw project_id values (usually filesystem paths)
// into stable canonical identifiers. It uses file-based detection (no external binaries)
// and caches results in a sync.Map for performance.
//
// Normalization priority:
//  1. Already canonical (github.com/...) → passthrough
//  2. .contextify.yml name field → explicit user config
//  3. VCS remote detection (.git/config, .hg/hgrc) → file parse, no git binary
//  4. Worktree suffix strip → regex fallback
//  5. Raw path fallback → unchanged
type ProjectNormalizer struct {
	cache sync.Map // path → canonical string
}

func NewProjectNormalizer() *ProjectNormalizer {
	return &ProjectNormalizer{}
}

// Normalize resolves a raw project_id into a canonical identifier.
func (n *ProjectNormalizer) Normalize(rawID string) string {
	if rawID == "" {
		return ""
	}

	// Already canonical? (hook or previous normalization)
	if isAlreadyCanonical(rawID) {
		return rawID
	}

	// Not a filesystem path? Return as-is
	if !strings.HasPrefix(rawID, "/") {
		return rawID
	}

	// Check cache
	if cached, ok := n.cache.Load(rawID); ok {
		return cached.(string)
	}

	canonical := n.resolve(rawID)
	n.cache.Store(rawID, canonical)
	return canonical
}

// resolve performs the actual normalization logic.
func (n *ProjectNormalizer) resolve(path string) string {
	// Find project root by walking up
	root := findProjectRoot(path)

	if root != "" {
		// Priority 1: .contextify.yml
		if name := readContextifyConfig(root); name != "" {
			return name
		}

		// Priority 2: VCS remote detection (file-based)
		if remote := detectVCSRemote(root); remote != "" {
			canonical := canonicalizeRemoteURL(remote)
			if canonical != "" {
				return canonical
			}
		}
	}

	// Priority 3: Strip worktree suffix
	stripped := stripWorktreeSuffix(path)
	if stripped != path {
		// Try to resolve the stripped path too (it might be the real repo root)
		strippedRoot := findProjectRoot(stripped)
		if strippedRoot != "" {
			if name := readContextifyConfig(strippedRoot); name != "" {
				return name
			}
			if remote := detectVCSRemote(strippedRoot); remote != "" {
				canonical := canonicalizeRemoteURL(remote)
				if canonical != "" {
					return canonical
				}
			}
		}
		return stripped
	}

	// Priority 4: Raw path fallback
	return path
}

// knownHosts are well-known VCS hosting services used for canonical detection.
var knownHosts = []string{
	"github.com/",
	"gitlab.com/",
	"bitbucket.org/",
	"codeberg.org/",
	"sr.ht/",
	"dev.azure.com/",
}

// isAlreadyCanonical checks if the ID is already in canonical format.
func isAlreadyCanonical(id string) bool {
	for _, host := range knownHosts {
		if strings.HasPrefix(id, host) {
			return true
		}
	}
	return false
}

// findProjectRoot walks up from path to find a directory containing
// .contextify.yml or a VCS directory (.git, .hg, .svn).
func findProjectRoot(path string) string {
	current := filepath.Clean(path)

	for {
		// Check for .contextify.yml
		if fileExists(filepath.Join(current, ".contextify.yml")) {
			return current
		}
		// Check for VCS directories
		if pathExistsAny(current, ".git", ".hg", ".svn") {
			return current
		}

		parent := filepath.Dir(current)
		if parent == current {
			break // reached filesystem root
		}
		current = parent
	}

	return ""
}

// readContextifyConfig reads the name field from .contextify.yml.
// Returns empty string if file doesn't exist or has no name field.
func readContextifyConfig(root string) string {
	path := filepath.Join(root, ".contextify.yml")
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Simple YAML parsing: look for "name: value"
		if strings.HasPrefix(line, "name:") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			// Strip quotes
			value = strings.Trim(value, `"'`)
			if value != "" {
				return value
			}
		}
	}

	return ""
}

// detectVCSRemote detects the VCS remote URL by reading config files directly.
// No external binary (git, hg) is invoked.
func detectVCSRemote(root string) string {
	dotGit := filepath.Join(root, ".git")
	info, err := os.Lstat(dotGit)
	if err == nil {
		if info.IsDir() {
			// Normal git repo — parse .git/config
			return parseGitConfigRemote(filepath.Join(dotGit, "config"))
		}
		// Git worktree — .git is a file containing "gitdir: <path>"
		return resolveGitWorktreeRemote(dotGit)
	}

	// Mercurial
	hgrc := filepath.Join(root, ".hg", "hgrc")
	if fileExists(hgrc) {
		return parseHgConfig(hgrc)
	}

	return ""
}

// parseGitConfigRemote parses a git config file to extract [remote "origin"] url.
func parseGitConfigRemote(configPath string) string {
	f, err := os.Open(configPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inRemoteOrigin := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Track section headers
		if strings.HasPrefix(line, "[") {
			inRemoteOrigin = line == `[remote "origin"]`
			continue
		}

		// Look for url = ... within [remote "origin"]
		if inRemoteOrigin && strings.HasPrefix(line, "url") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return ""
}

// resolveGitWorktreeRemote reads a .git file (worktree pointer), traverses
// to the main repo's config, and extracts the remote URL.
//
// Worktree .git file format: "gitdir: /path/to/.git/worktrees/<name>"
// The main repo config is at: <gitdir>/../../config
func resolveGitWorktreeRemote(dotGitFile string) string {
	data, err := os.ReadFile(dotGitFile)
	if err != nil {
		return ""
	}

	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, "gitdir:") {
		return ""
	}

	gitdir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))

	// Resolve relative path
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(filepath.Dir(dotGitFile), gitdir)
	}
	gitdir = filepath.Clean(gitdir)

	// Worktree gitdir typically points to .git/worktrees/<name>
	// The main .git directory is two levels up: .git/worktrees/<name> → .git
	mainGitDir := filepath.Dir(filepath.Dir(gitdir))

	// Verify this looks right by checking for config file
	configPath := filepath.Join(mainGitDir, "config")
	if fileExists(configPath) {
		return parseGitConfigRemote(configPath)
	}

	// Fallback: try gitdir/../config (if worktree structure is different)
	altConfig := filepath.Join(filepath.Dir(gitdir), "config")
	if fileExists(altConfig) {
		return parseGitConfigRemote(altConfig)
	}

	return ""
}

// parseHgConfig parses a Mercurial .hg/hgrc file to extract [paths] default.
func parseHgConfig(hgrcPath string) string {
	f, err := os.Open(hgrcPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inPaths := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "[") {
			inPaths = line == "[paths]"
			continue
		}

		if inPaths && strings.HasPrefix(line, "default") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return ""
}

var worktreeRegex = regexp.MustCompile(`/\.claude/worktrees/[^/]+$`)

// stripWorktreeSuffix removes /.claude/worktrees/<name> suffix from a path.
func stripWorktreeSuffix(path string) string {
	return worktreeRegex.ReplaceAllString(path, "")
}

// canonicalizeRemoteURL converts a VCS remote URL into a canonical identifier.
//
// Examples:
//
//	https://github.com/user/repo.git       → github.com/user/repo
//	git@github.com:user/repo.git           → github.com/user/repo
//	ssh://git@github.com/user/repo.git     → github.com/user/repo
//	ssh://hg@bitbucket.org/user/repo       → bitbucket.org/user/repo
//	https://gitlab.com/group/sub/repo.git  → gitlab.com/group/sub/repo
func canonicalizeRemoteURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	rawURL = strings.TrimSpace(rawURL)

	// Handle SCP-style URLs: git@github.com:user/repo.git
	if scp := parseSCPURL(rawURL); scp != "" {
		return scp
	}

	// Handle standard URLs: https://, ssh://, git://
	parsed, err := url.Parse(rawURL)
	if err != nil {
		slog.Debug("failed to parse remote URL", "url", rawURL, "error", err)
		return ""
	}

	host := parsed.Hostname()
	if host == "" {
		return ""
	}

	path := strings.TrimPrefix(parsed.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimSuffix(path, "/")

	if path == "" {
		return ""
	}

	return fmt.Sprintf("%s/%s", host, path)
}

var scpRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+@([^:]+):(.+)$`)

// parseSCPURL parses SCP-style git URLs like git@github.com:user/repo.git
func parseSCPURL(rawURL string) string {
	matches := scpRegex.FindStringSubmatch(rawURL)
	if len(matches) != 3 {
		return ""
	}

	host := matches[1]
	path := matches[2]
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimSuffix(path, "/")

	if path == "" {
		return ""
	}

	return fmt.Sprintf("%s/%s", host, path)
}

// --- helpers ---

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func pathExistsAny(dir string, names ...string) bool {
	for _, name := range names {
		if _, err := os.Lstat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}
