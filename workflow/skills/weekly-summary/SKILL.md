---
name: weekly-summary
description: Synthesize a week's domain logs, work items, sessions, and current ticket or PR state into a compact weekly wrap-up. Use when the user asks for a weekly summary, progress report, manager update, or next-week plan.
---

# Weekly Summary

Promote weekly evidence into a short reusable summary without rewriting the week from memory.

## Workflow

1. Read `workflow/ai-operating-model.md`, `workflow/domain-centered-knowledge-model.md`, and `templates/weekly-summary.md`.
2. Resolve the requested date range. Default to Monday through Sunday in the user's timezone, using the previous completed week when the user asks for “this week” before the week has meaningfully concluded.
3. Gather evidence for the range:
   - every project `log/YYYY-MM-DD.md` in range;
   - linked work items and their current status;
   - relevant session notes, decisions, deliverables, tickets, PRs, commits, and validation evidence;
   - an existing weekly summary for the range, if present.
4. Reconcile duplicates that refer to the same outcome. Prefer the canonical work item or domain and avoid double-counting legacy project notes.
5. Refresh current status when the summary wording depends on it. If a bullet says something is merged, open, validating, or blocked, verify that from the current canonical work item or the authoritative PR or ticket state.
6. Write or update `weekly-summary/weekly-summary-YYYY-MM-DD-to-YYYY-MM-DD.md`.
7. Run `git diff --check` on the summary and report evidence gaps explicitly.

## Default output format

Unless the user asks for another shape, use exactly three sections:

- **Progress**
- **Problem**
- **Plan**

## Section rules

### Progress

- Use a flat bullet list.
- One bullet should represent one ticket or one clearly bounded work atom.
- Lead with a descriptive ticket label when a ticket exists, for example `CONVI-7281: Training-simulator-only Opera rules`, not just the raw ID.
- Describe the current outcome or shipped status, not the implementation process used to get there.
- Keep process-only details, temporary debugging steps, and unrelated side issues out of the bullet unless they materially changed the final status.

### Problem

- If there is no meaningful problem summary for the week, write `- None.`
- Do not invent a problem section just to fill the template.

### Plan

- Use one action atom per line.
- Prefer concrete next actions tied to the same descriptive ticket labels used in Progress.
- Remove plan items that are already completed by the time the summary is written.

## Quality rules

- State outcomes before implementation details.
- Prefer human-readable ticket labels over bare identifiers.
- Keep ticket or PR state current; do not leave “in review” or “ready to merge” after it has merged.
- Omit inline evidence by default from the final weekly summary unless the user explicitly asks for it.
- Preserve contribution accuracy; do not claim sole ownership when the work was collaborative.
- Use ticket counts, PR counts, or hours only as supporting context, not the main value statement.

## Safety and idempotence

- Preserve unrelated and pre-existing user changes.
- Update an existing summary rather than creating duplicates.
- Do not include secrets or sensitive raw output.
- Do not stage, commit, push, or publish the summary unless explicitly requested.
