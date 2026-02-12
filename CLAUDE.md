# Contextify - Project Rules

## Changelog Rule

When making a git commit, **always** update `CHANGELOG.md` before committing:

1. Add a line under `## [Unreleased]` in the appropriate section:
   - `### Added` — new features
   - `### Changed` — changes to existing functionality
   - `### Fixed` — bug fixes
   - `### Removed` — removed features
   - `### Security` — security-related changes

2. The entry should be a concise, user-facing description (not a commit message copy).
   - Good: `- CLI tool for managing Contextify (install, update, status, memory operations)`
   - Bad: `- feat: add CLI`

3. Stage `CHANGELOG.md` together with the rest of the commit.

### Release Preparation

When the user requests a release (e.g. "release v0.5.0"), before triggering the workflow:

1. Run `./scripts/update-changelog.sh <VERSION>` to move `[Unreleased]` entries to the new version section.
2. Commit the updated `CHANGELOG.md` as part of the release PR.
3. This must be merged to `main` **before** triggering the release workflow.

## Memory Protocol (MANDATORY)

After EVERY `git commit`, you MUST immediately call `store_memory` before doing anything else.
This is not optional. Do NOT proceed to the next task, PR creation, or any other action until `store_memory` is called.

- **type**: fix | decision | code_pattern | workflow
- **tags**: include project name, technology, category
- **importance**: 0.7+ for commits, 0.8+ for architecture decisions
- **agent_source**: "claude-code"
- **project_id**: current working directory

Violation: If you run any tool after `git commit` that is not `store_memory`, you are in violation.

## Commit Convention

Use conventional commits: `feat:`, `fix:`, `ci:`, `refactor:`, `docs:`, `chore:`, `test:`
