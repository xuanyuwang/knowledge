---
name: weekly-summary
description: Synthesize a week's domain logs, work items, sessions, and evidence into an impact-oriented progress summary. Use when the user asks for a weekly wrap-up, progress report, manager update, performance-review evidence, next-week plan, or synthesis of work completed during a date range.
---

# Weekly Summary

Promote daily evidence into a concise account of outcomes, ownership, risk reduction, leverage, and next priorities without rewriting the week from memory.

## Workflow

1. Read `workflow/ai-operating-model.md`, `workflow/domain-centered-knowledge-model.md`, and `templates/weekly-summary.md`.
2. Resolve the requested date range. Default to Monday through Sunday in the user's timezone, using the current week when no range is given.
3. Gather evidence for the range:
   - every project `log/YYYY-MM-DD.md` in range;
   - linked work items and their current status;
   - relevant session notes, decisions, deliverables, tickets, PRs, commits, and validation evidence;
   - an existing weekly summary for the range, if present.
4. Reconcile duplicates that refer to the same outcome. Prefer the canonical work item/domain and link secondary-domain contributions.
5. Write or update `weekly-summary/weekly-summary-YYYY-MM-DD-to-YYYY-MM-DD.md`.
6. Organize by impact and ownership rather than by commit count or raw ticket chronology.
7. Run `git diff --check` on the summary and report evidence gaps explicitly.

## Required sections

- **Executive summary:** the most important outcomes and overall direction.
- **Outcomes delivered:** grouped by domain, including current status and evidence.
- **Domain stewardship:** semantics, architecture, operational knowledge, or ownership improved.
- **Reliability and risk:** incidents, correctness, rollout safety, prevention, or unresolved exposure.
- **Leverage and influence:** decisions, reviews, reusable artifacts, alignment, or engineers enabled.
- **Problems and lessons:** blockers, failures, changed hypotheses, and learning.
- **Next-week priorities:** concrete and realistically ordered.
- **Performance evidence candidates:** the strongest items worth promoting into annual review material, including role, impact, evidence, collaborators, and pending metrics.

Omit empty subsections rather than invent content.

## Quality rules

- State outcomes before implementation details.
- Distinguish completed, validating, blocked, and planned work.
- Preserve contribution accuracy: do not claim sole ownership when work was collaborative.
- Use ticket/PR counts and hours only as supporting evidence, not impact.
- Link claims to repository artifacts or external evidence.
- Keep unresolved metrics and follow-ups visible so later reviews can close the loop.

## Safety and idempotence

- Preserve unrelated and pre-existing user changes.
- Update an existing summary rather than creating duplicates.
- Do not include secrets or sensitive raw output.
- Do not stage, commit, push, or publish the summary unless explicitly requested.
