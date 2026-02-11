# Changelog

All notable changes to Contextify are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Post-tool-use hook state machine — enforces `store_memory` after every `git commit` with violation nagging
- `contextify update` and `install.sh --update` now force-overwrite tool configs (hooks, prompts, rules) with latest versions
- `UpdateConfiguredTools()` function for centralized tool config updates

### Changed
- Post-tool-use hook output upgraded from soft reminder to aggressive required/violation messages

## [0.5.3] - 2026-02-12

### Added
- Interactive setup wizard — `install.sh` now asks which AI tools to configure
- Windsurf support (MCP via `~/.codeium/windsurf/mcp_config.json`)
- `--tools`, `--all`, `--status`, `--help` flags for non-interactive usage
- Re-runnability — wizard skips already-completed steps on subsequent runs
- Per-tool status indicators (`✓ configured`, `◐ partial`, `○ not configured`)
- Auto-restart for Cursor and Windsurf after configuration to load MCP servers
- `prompts/windsurf.md` template
- Go CLI tool (`contextify`) with Cobra framework for managing Contextify from the terminal
- CLI management commands: `install`, `uninstall`, `start`, `stop`, `restart`, `update`, `status`, `logs`, `version`
- CLI memory commands: `store`, `recall`, `search`, `get`, `delete`, `promote`, `stats`, `context`
- HTTP client for REST API (`internal/client/`) — standalone, no server-side dependencies
- Docker container manager (`internal/docker/`) via `os/exec` shell-out
- AI tool configuration in pure Go (`internal/toolconfig/`) — replaces jq/python3 dependency
- `Makefile` with `build-cli`, `build-server`, `build-all`, `release-cli` targets
- `scripts/install-cli.sh` — one-line CLI installer via `curl | sh`
- CLI binary cross-compilation in release workflow (darwin/linux, amd64/arm64)
- Automatic CHANGELOG.md updates on release (`scripts/update-changelog.sh`)
- Auto-generated release notes from CHANGELOG + git log (`scripts/release-notes.sh`)
- CLI build verification step in backend CI
- Expanded PostToolUse hooks: recall triggers on Grep, Glob, WebSearch, WebFetch, Task, Read; store triggers on git commit, push, PR, errors
- Aggressive mandatory protocol prompts for Cursor, Windsurf, and Gemini rules files
- Claude Code CLAUDE.md prompt block upgraded to mandatory protocol (matching hook enforcement level)

### Changed
- `install.sh` no longer auto-detects and silently configures all tools
- Gemini configuration is now an explicit selection (not always copied)
- Release workflow generates notes from CHANGELOG.md instead of hardcoded body
- CHANGELOG.md updates are now developer-driven (via CLAUDE.md rule) instead of automated in CI

### Removed
- Auto-detect behavior (`detect_tools()`) replaced by interactive selection

## [0.2.0] - 2026-02-11

### Added
- All-in-one Docker image with PostgreSQL, Ollama, and server bundled
- Production-quality Web UI with redesigned design system
- Auto-context system with `install.sh` for one-command setup
- Claude Code hooks (SessionStart, PostToolUse) for automatic memory loading
- Cursor MCP configuration and rules file support
- Gemini REST API prompt template
- `install.sh --uninstall` for clean removal
- JSON merge utilities (`scripts/lib/json-merge.sh`) with jq/python3 fallback
- Backend, Web UI, and .github restriction CI workflows

### Changed
- Merged Web UI into single Docker image (was separate service)

## [0.1.0] - 2026-01-20

### Added
- Unified AI agent memory system with PostgreSQL + pgvector
- MCP server (Streamable HTTP transport) for Claude Code and Cursor
- REST API for Gemini, Antigravity, and other tools
- Semantic search via Ollama embeddings (nomic-embed-text, 768d)
- Memory model with types, scopes, importance, and TTL
- Auto-promotion (importance >= 0.8 or access count >= 5)
- Background TTL cleanup scheduler
- Relationship system for linking memories
- Docker image publishing and CI/CD pipeline
- Architecture documentation

[Unreleased]: https://github.com/atakanatali/contextify/compare/v0.5.3...HEAD
[0.5.3]: https://github.com/atakanatali/contextify/compare/v0.2.0...v0.5.3
[0.2.0]: https://github.com/atakanatali/contextify/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/atakanatali/contextify/releases/tag/v0.1.0
