# Skill: Issue Guidline Skill (Codex)

Use this skill when creating or refining implementation issues for the CDX series.

## Purpose

Generate high-quality, execution-ready issues that are:
- small and independent,
- architecturally clear,
- safe to implement in production repositories,
- optimized for AI agent handoff.

## Required Inputs

1. Problem statement
2. Target outcome
3. Constraints (timeline, compatibility, stack limits)
4. Existing architecture touchpoints

If any input is missing, infer minimally and state assumptions.

## Workflow

1. Parse requested initiative into bounded deliverables.
2. Split deliverables into issues with <=20 meaningful changes each.
3. Write each issue in English, in the required section order.
4. Start each issue with an "Agent Directive" tailored to the specific task.
5. Add concrete done checklist and command-level validation.
6. Add integration and operational risks with mitigation.
7. Ensure each issue is independently mergeable.

## Quality Gates

Before finalizing an issue, verify:

1. Title format: `CDXNN - <actionable title>`
2. Label requirement (`feature`) is explicitly stated or applied.
3. Scope is narrow and unambiguous.
4. Out-of-scope prevents accidental expansion.
5. Validation includes runnable commands.
6. No section is missing.

## Anti-Patterns

Do not:

1. Create broad "do everything" issues.
2. Hide constraints in long paragraphs.
3. Use vague completion criteria ("improve", "optimize") without metrics.
4. Merge unrelated tooling changes into one issue.
5. Omit rollback considerations for installer/update paths.
