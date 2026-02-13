# Rule: CDX Issue Authoring Rule

When generating issues for Contextify improvements, enforce all rules below:

1. English only.
2. Prefix titles with `CDXNN -`.
3. Use label `feature`.
4. Keep each issue scoped to roughly <=20 meaningful code changes.
5. Start with a compact "Agent Directive" section.
6. Include mandatory sections:
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
7. Add at least one explicit rollback/compatibility note for install/update/uninstall tasks.
8. Done checklist must be objective and testable.
9. Validation must reference executable commands.
10. Reject issue drafts that violate any rule above.
