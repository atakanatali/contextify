# Contextify Issue Guidlines (CDX Series)

This document defines how to write engineering issues for Contextify improvements.
It is optimized for AI-agent execution with strict scope, measurable outcomes, and low ambiguity.

## Global Mandatory Rules

1. Language must be English only.
2. Every issue title must start with `CDXNN - ...` where `NN` is zero-padded (01, 02, ...).
3. Add label `feature` to every issue in this series.
4. Each issue must be small enough for at most ~20 meaningful code changes in one PR.
5. Each issue must include a short "Agent Directive" at the top that configures the assigned agent mindset.
6. Each issue must contain these sections in order:
   - Agent Directive
   - Why
   - Goal
   - Scope
   - Out of Scope
   - Expected Result
   - Implementation Notes
   - Done Checklist
   - Risks and Mitigations
   - Validation
7. Done checklist items must be testable and binary (`done/not done`).
8. Risks must include at least one operational risk and one integration risk.
9. Validation must include commands, not only prose.
10. If work touches install/update flows, rollback behavior must be explicitly documented.

## Issue Granularity Rules

1. Separate concerns by responsibility boundary:
   - Tool detection/status
   - Tool configure/update/uninstall
   - Installer UX and selection flow
   - Documentation and migration notes
2. Do not combine code changes and large docs refactors in the same issue.
3. Prefer vertical slices that can ship independently.
4. If an issue grows beyond one focused behavior, split it.

## Agent Directive Format

Use a short block at the top:

```
You are the implementation agent for this issue.
Primary objective: <single sentence>.
Constraints: keep changes minimal, preserve backward compatibility, and fail safely.
Decision policy: prefer deterministic and idempotent behavior.
```

## Done Checklist Baseline

Every issue should include, at minimum:

- [ ] Code change implemented with backward-compatible behavior
- [ ] Update path handled (if applicable)
- [ ] Uninstall/rollback path handled (if applicable)
- [ ] Documentation updated
- [ ] Validation commands executed and captured

## Example Skeleton

```md
# CDXNN - Short actionable title

## Agent Directive
You are the implementation agent for this issue.
Primary objective: ...
Constraints: ...
Decision policy: ...

## Why
...

## Goal
...

## Scope
- ...

## Out of Scope
- ...

## Expected Result
...

## Implementation Notes
- ...

## Done Checklist
- [ ] ...

## Risks and Mitigations
- Risk: ...
  Mitigation: ...

## Validation
- `go test ./...`
- `<other command>`
```
