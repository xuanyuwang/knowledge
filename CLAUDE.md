# CLAUDE.md

This repository uses a shared workflow spec.

Read `workflow/ai-operating-model.md` and `workflow/domain-centered-knowledge-model.md` before doing substantial work in this repo.

## Claude-specific adapter

- Respect `.claude/settings.json`. Worktrees are intentionally disabled for the `knowledge` repo itself; work directly in the main repo checkout.
- For substantial investigation, design, review, or multi-step execution, identify the primary domain before starting. Create a standalone initiative only when the operating-model threshold is met.
- Use the domain and subdomain catalog in `README.md` and `workflow/domain-centered-knowledge-model.md` when choosing that home.
- Use `workspace/repos.yaml` to resolve the target source repo or named worktree instead of guessing from prose.
- When creating a new worktree for any other repo, place it under `/Users/xuanyu.wang/repos`.
- Use `<project>/project.yaml` as the primary machine-readable handoff surface.
- If an existing project lacks `project.yaml`, add it from `templates/project.yaml` before substantial work.
- Put rich Claude session context in `sessions/YYYY-MM-DD/claude-<topic>.md`.
- Track tickets and continuing tasks in one canonical `<primary-domain>/work-items/<id>.md`.
- Record an optional primary subdomain, but keep work items, sessions, logs, and decisions at the parent domain; use `subdomains/` for durable reference knowledge only.
- Before final handoff, automatically update the work item and concise daily log; do not wait for an explicit logging request.
- Use `workflow/skills/daily-capture/SKILL.md` and `workflow/skills/weekly-summary/SKILL.md` as the canonical capture and synthesis workflows. Existing `.claude/skills/wrap-day` and `.claude/skills/wrap-week` are compatibility aliases.
- Treat the work as incomplete if durable investigation findings remain only in chat history or terminal output.
- Migrate legacy projects gradually when they are reopened or their evidence is needed; synthesize and point to the canonical domain before any separately reviewed removal.

## Allowed write targets

Claude may update only:

- project `README.md`
- `project.yaml`
- `subdomains/`
- `work-items/`
- `log/`
- `sessions/`
- `decisions/`
- `deliverables/`
- `templates/`
- `workflow/`
- `workspace/`
- `workflow/skills/`
- `.claude/skills/`

Do not invent new top-level structures without first updating `workflow/ai-operating-model.md`.

## Separation of concerns

- Product code belongs in the target repo or repo worktree.
- This repo exists for investigations, designs, decisions, reviews, execution records, and synthesis.
