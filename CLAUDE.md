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

## Commit Convention

Use conventional commits: `feat:`, `fix:`, `ci:`, `refactor:`, `docs:`, `chore:`, `test:`
