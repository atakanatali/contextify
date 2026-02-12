package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsAlreadyCanonical(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"github.com/user/repo", true},
		{"github.com/atakanatali/contextify", true},
		{"gitlab.com/group/repo", true},
		{"bitbucket.org/user/repo", true},
		{"codeberg.org/user/repo", true},
		{"sr.ht/~user/repo", true},
		{"dev.azure.com/org/project", true},
		{"/Users/atakan/Desktop/project", false},
		{"/home/user/code/repo", false},
		{"", false},
		{"my-project", false},
		{"github.com", false}, // no path after host
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := isAlreadyCanonical(tt.id)
			if got != tt.want {
				t.Errorf("isAlreadyCanonical(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestCanonicalizeRemoteURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		// HTTPS
		{"https with .git", "https://github.com/user/repo.git", "github.com/user/repo"},
		{"https without .git", "https://github.com/user/repo", "github.com/user/repo"},
		{"https gitlab", "https://gitlab.com/group/sub/repo.git", "gitlab.com/group/sub/repo"},
		{"https bitbucket", "https://bitbucket.org/user/repo.git", "bitbucket.org/user/repo"},
		{"https with trailing slash", "https://github.com/user/repo/", "github.com/user/repo"},

		// SSH (SCP-style)
		{"ssh scp with .git", "git@github.com:user/repo.git", "github.com/user/repo"},
		{"ssh scp without .git", "git@github.com:user/repo", "github.com/user/repo"},
		{"ssh scp gitlab", "git@gitlab.com:group/sub/repo.git", "gitlab.com/group/sub/repo"},
		{"ssh scp custom user", "deploy@github.com:user/repo.git", "github.com/user/repo"},

		// SSH (URL-style)
		{"ssh url github", "ssh://git@github.com/user/repo.git", "github.com/user/repo"},
		{"ssh url bitbucket hg", "ssh://hg@bitbucket.org/user/repo", "bitbucket.org/user/repo"},

		// Git protocol
		{"git protocol", "git://github.com/user/repo.git", "github.com/user/repo"},

		// Edge cases
		{"empty", "", ""},
		{"whitespace", "  https://github.com/user/repo.git  ", "github.com/user/repo"},
		{"just host", "https://github.com/", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canonicalizeRemoteURL(tt.url)
			if got != tt.want {
				t.Errorf("canonicalizeRemoteURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestStripWorktreeSuffix(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/Users/a/project/.claude/worktrees/nice-jemison", "/Users/a/project"},
		{"/Users/a/project/.claude/worktrees/sad-margulis", "/Users/a/project"},
		{"/Users/a/project", "/Users/a/project"},
		{"/Users/a/project/.claude", "/Users/a/project/.claude"},
		{"/a/b/.claude/worktrees/x", "/a/b"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := stripWorktreeSuffix(tt.path)
			if got != tt.want {
				t.Errorf("stripWorktreeSuffix(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestParseGitConfigRemote(t *testing.T) {
	// Create temp git config
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")

	t.Run("normal config with origin", func(t *testing.T) {
		content := `[core]
	repositoryformatversion = 0
	filemode = true
[remote "origin"]
	url = https://github.com/user/repo.git
	fetch = +refs/heads/*:refs/remotes/origin/*
[branch "main"]
	remote = origin
	merge = refs/heads/main
`
		os.WriteFile(configPath, []byte(content), 0644)

		got := parseGitConfigRemote(configPath)
		if got != "https://github.com/user/repo.git" {
			t.Errorf("got %q, want %q", got, "https://github.com/user/repo.git")
		}
	})

	t.Run("ssh remote", func(t *testing.T) {
		content := `[remote "origin"]
	url = git@github.com:user/repo.git
`
		os.WriteFile(configPath, []byte(content), 0644)

		got := parseGitConfigRemote(configPath)
		if got != "git@github.com:user/repo.git" {
			t.Errorf("got %q, want %q", got, "git@github.com:user/repo.git")
		}
	})

	t.Run("no origin remote", func(t *testing.T) {
		content := `[core]
	repositoryformatversion = 0
[remote "upstream"]
	url = https://github.com/other/repo.git
`
		os.WriteFile(configPath, []byte(content), 0644)

		got := parseGitConfigRemote(configPath)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		got := parseGitConfigRemote("/nonexistent/config")
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestParseHgConfig(t *testing.T) {
	dir := t.TempDir()
	hgrcPath := filepath.Join(dir, "hgrc")

	t.Run("mercurial config with default path", func(t *testing.T) {
		content := `[paths]
default = https://bitbucket.org/user/repo
[ui]
username = Test User <test@example.com>
`
		os.WriteFile(hgrcPath, []byte(content), 0644)

		got := parseHgConfig(hgrcPath)
		if got != "https://bitbucket.org/user/repo" {
			t.Errorf("got %q, want %q", got, "https://bitbucket.org/user/repo")
		}
	})

	t.Run("no paths section", func(t *testing.T) {
		content := `[ui]
username = Test User <test@example.com>
`
		os.WriteFile(hgrcPath, []byte(content), 0644)

		got := parseHgConfig(hgrcPath)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestReadContextifyConfig(t *testing.T) {
	dir := t.TempDir()

	t.Run("config with name", func(t *testing.T) {
		content := `# Project config
name: "my-awesome-project"
`
		os.WriteFile(filepath.Join(dir, ".contextify.yml"), []byte(content), 0644)

		got := readContextifyConfig(dir)
		if got != "my-awesome-project" {
			t.Errorf("got %q, want %q", got, "my-awesome-project")
		}
	})

	t.Run("config with unquoted name", func(t *testing.T) {
		content := `name: my-project
`
		os.WriteFile(filepath.Join(dir, ".contextify.yml"), []byte(content), 0644)

		got := readContextifyConfig(dir)
		if got != "my-project" {
			t.Errorf("got %q, want %q", got, "my-project")
		}
	})

	t.Run("config with single quotes", func(t *testing.T) {
		content := `name: 'single-quoted'
`
		os.WriteFile(filepath.Join(dir, ".contextify.yml"), []byte(content), 0644)

		got := readContextifyConfig(dir)
		if got != "single-quoted" {
			t.Errorf("got %q, want %q", got, "single-quoted")
		}
	})

	t.Run("no name field", func(t *testing.T) {
		content := `version: 1
tags:
  - golang
`
		os.WriteFile(filepath.Join(dir, ".contextify.yml"), []byte(content), 0644)

		got := readContextifyConfig(dir)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("no config file", func(t *testing.T) {
		emptyDir := t.TempDir()
		got := readContextifyConfig(emptyDir)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestFindProjectRoot(t *testing.T) {
	t.Run("directory with .git", func(t *testing.T) {
		dir := t.TempDir()
		os.Mkdir(filepath.Join(dir, ".git"), 0755)

		got := findProjectRoot(dir)
		if got != dir {
			t.Errorf("got %q, want %q", got, dir)
		}
	})

	t.Run("subdirectory of git repo", func(t *testing.T) {
		dir := t.TempDir()
		os.Mkdir(filepath.Join(dir, ".git"), 0755)
		subdir := filepath.Join(dir, "src", "internal")
		os.MkdirAll(subdir, 0755)

		got := findProjectRoot(subdir)
		if got != dir {
			t.Errorf("got %q, want %q", got, dir)
		}
	})

	t.Run("contextify.yml takes priority", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, ".contextify.yml"), []byte("name: test"), 0644)

		got := findProjectRoot(dir)
		if got != dir {
			t.Errorf("got %q, want %q", got, dir)
		}
	})

	t.Run("no project root found", func(t *testing.T) {
		dir := t.TempDir()
		subdir := filepath.Join(dir, "empty", "deep")
		os.MkdirAll(subdir, 0755)

		got := findProjectRoot(subdir)
		// Should eventually return "" since /tmp dirs typically don't have .git
		// (unless the temp dir itself is inside a git repo on the test machine)
		_ = got // Can't assert exact value due to host filesystem
	})
}

func TestResolveGitWorktreeRemote(t *testing.T) {
	// Set up a fake worktree structure:
	// mainRepo/.git/config        (has remote origin)
	// mainRepo/.git/worktrees/wt1 (directory exists)
	// worktree/.git               (file with gitdir: pointer)
	dir := t.TempDir()

	mainRepo := filepath.Join(dir, "main-repo")
	mainGitDir := filepath.Join(mainRepo, ".git")
	worktreesDir := filepath.Join(mainGitDir, "worktrees", "wt1")
	worktreeDir := filepath.Join(dir, "worktree")

	os.MkdirAll(worktreesDir, 0755)
	os.MkdirAll(worktreeDir, 0755)

	// Write main repo config
	os.WriteFile(filepath.Join(mainGitDir, "config"), []byte(`[remote "origin"]
	url = https://github.com/test/worktree-repo.git
`), 0644)

	// Write worktree .git file pointing to worktrees dir
	os.WriteFile(filepath.Join(worktreeDir, ".git"), []byte("gitdir: "+worktreesDir+"\n"), 0644)

	got := resolveGitWorktreeRemote(filepath.Join(worktreeDir, ".git"))
	if got != "https://github.com/test/worktree-repo.git" {
		t.Errorf("got %q, want %q", got, "https://github.com/test/worktree-repo.git")
	}
}

func TestNormalize_Integration(t *testing.T) {
	normalizer := NewProjectNormalizer()

	t.Run("empty string", func(t *testing.T) {
		got := normalizer.Normalize("")
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("already canonical passthrough", func(t *testing.T) {
		got := normalizer.Normalize("github.com/user/repo")
		if got != "github.com/user/repo" {
			t.Errorf("got %q, want %q", got, "github.com/user/repo")
		}
	})

	t.Run("non-path string passthrough", func(t *testing.T) {
		got := normalizer.Normalize("my-custom-project")
		if got != "my-custom-project" {
			t.Errorf("got %q, want %q", got, "my-custom-project")
		}
	})

	t.Run("path with .contextify.yml", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, ".contextify.yml"), []byte(`name: "test-project"`), 0644)

		got := normalizer.Normalize(dir)
		if got != "test-project" {
			t.Errorf("got %q, want %q", got, "test-project")
		}
	})

	t.Run("path with git remote", func(t *testing.T) {
		dir := t.TempDir()
		gitDir := filepath.Join(dir, ".git")
		os.Mkdir(gitDir, 0755)
		os.WriteFile(filepath.Join(gitDir, "config"), []byte(`[remote "origin"]
	url = https://github.com/testuser/testrepo.git
`), 0644)

		got := normalizer.Normalize(dir)
		if got != "github.com/testuser/testrepo" {
			t.Errorf("got %q, want %q", got, "github.com/testuser/testrepo")
		}
	})

	t.Run("worktree path resolves to main repo", func(t *testing.T) {
		dir := t.TempDir()

		// Set up main repo
		mainRepo := filepath.Join(dir, "main-repo")
		mainGitDir := filepath.Join(mainRepo, ".git")
		worktreesDir := filepath.Join(mainGitDir, "worktrees", "wt1")
		os.MkdirAll(worktreesDir, 0755)
		os.WriteFile(filepath.Join(mainGitDir, "config"), []byte(`[remote "origin"]
	url = https://github.com/test/wt-repo.git
`), 0644)

		// Set up worktree with .claude path
		worktreeDir := filepath.Join(mainRepo, ".claude", "worktrees", "test-worktree")
		os.MkdirAll(worktreeDir, 0755)
		os.WriteFile(filepath.Join(worktreeDir, ".git"), []byte("gitdir: "+worktreesDir+"\n"), 0644)

		got := normalizer.Normalize(worktreeDir)
		if got != "github.com/test/wt-repo" {
			t.Errorf("got %q, want %q", got, "github.com/test/wt-repo")
		}
	})

	t.Run("contextify.yml has higher priority than git", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, ".contextify.yml"), []byte(`name: "explicit-name"`), 0644)
		gitDir := filepath.Join(dir, ".git")
		os.Mkdir(gitDir, 0755)
		os.WriteFile(filepath.Join(gitDir, "config"), []byte(`[remote "origin"]
	url = https://github.com/other/repo.git
`), 0644)

		got := normalizer.Normalize(dir)
		if got != "explicit-name" {
			t.Errorf("got %q, want %q — .contextify.yml should take priority over git", got, "explicit-name")
		}
	})

	t.Run("non-vcs path returns raw", func(t *testing.T) {
		dir := t.TempDir()
		subdir := filepath.Join(dir, "isolated", "project")
		os.MkdirAll(subdir, 0755)

		got := normalizer.Normalize(subdir)
		// Should return the subdir path since no VCS or config found
		// (unless the temp dir happens to be inside a git repo)
		if got == "" {
			t.Error("got empty, want non-empty path")
		}
	})
}

func TestNormalize_Cache(t *testing.T) {
	normalizer := NewProjectNormalizer()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".contextify.yml"), []byte(`name: "cached-project"`), 0644)

	// First call
	got1 := normalizer.Normalize(dir)
	if got1 != "cached-project" {
		t.Fatalf("first call got %q, want %q", got1, "cached-project")
	}

	// Remove the file — cached result should still work
	os.Remove(filepath.Join(dir, ".contextify.yml"))

	got2 := normalizer.Normalize(dir)
	if got2 != "cached-project" {
		t.Errorf("cached call got %q, want %q (should use cache)", got2, "cached-project")
	}
}

func TestParseSCPURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"git@github.com:user/repo.git", "github.com/user/repo"},
		{"git@github.com:user/repo", "github.com/user/repo"},
		{"deploy@gitlab.com:group/project.git", "gitlab.com/group/project"},
		{"https://github.com/user/repo.git", ""}, // not SCP format
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := parseSCPURL(tt.url)
			if got != tt.want {
				t.Errorf("parseSCPURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
