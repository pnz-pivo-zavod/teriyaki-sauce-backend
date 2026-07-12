---
name: maintain-project-context
description: Load and maintain the canonical Teriyaki Sauce backend context, requirements, source links, roadmap, architecture decisions, and project change history. Use for every planning, implementation, review, debugging, architecture, dependency, configuration, deployment, Jira-stage, or documentation task in this repository, and whenever an accepted decision or project change must be recorded.
---

# Maintain Project Context

## Start every project task

1. Read [references/PROJECT_CONTEXT.md](references/PROJECT_CONTEXT.md) completely.
2. Read [references/ADR_LOG.md](references/ADR_LOG.md) completely.
3. Inspect the relevant repository files and Git state. Treat repository code as the source of truth for current implementation and the references as the canonical intent/history.
4. Reconcile discrepancies explicitly before planning or changing code.

## Maintain the context

Update the references in the same change whenever the task alters project knowledge:

- Update `PROJECT_CONTEXT.md` when requirements, stack, configuration, public interfaces, security rules, roadmap status, deployment assumptions, or current implementation change.
- Append an ADR to `ADR_LOG.md` when an architectural choice is accepted or an earlier choice is superseded.
- Append a project-change entry to `ADR_LOG.md` when an implementation stage or cross-cutting project setup changes.
- Never rewrite accepted ADR history. Add a new ADR with `Supersedes`/`Superseded by` links.
- Record only decisions actually accepted by the user or already established in the repository. Mark proposals as `Proposed`, not `Accepted`.
- Keep links direct and prefer official documentation, Jira issues, and repository files.
- Never store tokens, passwords, cookies, Telegram `initData`, private URLs, or other secrets.

## ADR format

Use the next sequential identifier and include:

```markdown
## ADR-NNN — Title

- Status: Proposed | Accepted | Superseded | Rejected
- Date: YYYY-MM-DD
- Related: links
- Supersedes: ADR-NNN, when applicable

### Context
Why a decision was required.

### Decision
The exact choice that was accepted.

### Consequences
Important benefits, constraints, and follow-up work.
```

## Quality rules

- Keep the current snapshot concise enough to load on every task.
- Put historical reasoning in the ADR log rather than duplicating it in the snapshot.
- Verify unstable library/API facts against current official documentation before updating links or versions.
- Preserve user-authored changes and do not mark a stage complete until its acceptance checks pass.
