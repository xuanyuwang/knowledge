---
name: daily-capture
description: Capture sufficient durable evidence after substantial work in the knowledge repository. Use when closing an investigation, design, review, implementation, or multi-step session; when the user asks to wrap up, log, or summarize today's work; or when repository instructions require work-item, session, and daily-log updates before handoff.
---

# Daily Capture

Close substantial work without making the human reconstruct it later. Keep current state in one work item, rich evidence in sessions, and concise movement in the daily log.

## Workflow

1. Read `workflow/ai-operating-model.md`, `workflow/domain-centered-knowledge-model.md`, and the primary project's `project.yaml` and `README.md`.
2. Determine today's date in the user's timezone.
3. Inspect task evidence:
   - current conversation and actions;
   - scoped `git status` and diffs in `knowledge`;
   - the active source repo/worktree, branch, ticket, PR, queries, or external artifacts used;
   - existing work item, session, and daily log for this task/date.
4. Choose exactly one primary domain. If the work is a ticket or continuing task, create or update `work-items/<id>.md` from `templates/work-item.md`.
5. Create or update `sessions/YYYY-MM-DD/<tool>-<topic>.md` when the work produced investigation, design, review, or execution reasoning that should survive the session.
6. Create or merge into `log/YYYY-MM-DD.md` using `templates/daily-log.md`.
7. Update the domain `README.md` only when human-facing domain state or durable understanding changed.
8. Run `git diff --check` on edited Markdown/YAML files and report the paths written.

## Sufficiency standard

The daily log must make these facts recoverable:

- **Outcome:** what changed, shipped, was decided, learned, or unblocked.
- **Impact:** why it matters to customers, correctness, reliability, delivery, or team effectiveness.
- **Role:** led, designed, diagnosed, implemented, reviewed, coordinated, or supported.
- **Evidence:** work item, ticket, PR, commit, query/dashboard, decision, or feedback.
- **Follow-up:** validation, metric, decision, blocker, or next action.

Do not write vague entries such as "worked on ticket" or infer impact unsupported by evidence. Mark unknown metrics or outcomes as pending.

## Work-item rules

- Make the current understanding and next action readable without replaying the timeline.
- Add only meaningful dated checkpoints to the timeline.
- For cross-domain work, keep one canonical work item in the primary domain and link to it from secondary-domain logs.
- Keep official ticket workflow/status in Linear or the relevant issue tracker.
- On completion, promote durable semantics, architecture, operations, or decisions into domain artifacts.

## Safety and idempotence

- Preserve unrelated and pre-existing user changes.
- Merge with existing entries instead of replacing or duplicating them.
- Do not create empty scaffold directories or logs for trivial work.
- Do not expose secrets or copy sensitive terminal output into notes.
- Do not stage, commit, push, or modify external systems unless explicitly requested.
