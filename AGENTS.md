# AGENTS.md

This repository uses a shared workflow spec for Codex and other AI tools.

Read `workflow/ai-operating-model.md` and `workflow/domain-centered-knowledge-model.md` before making substantial changes.

## Tool-specific adapter

- Do not create a repo-specific worktree for `knowledge`. Work directly in the main repo checkout.
- For substantial investigation, design, review, or multi-step execution, identify the primary domain before starting. Create a standalone initiative only when the operating-model threshold is met.
- Use the domain and subdomain catalog in `README.md` and `workflow/domain-centered-knowledge-model.md` when choosing that home.
- Use `workspace/repos.yaml` to resolve the source repo and any named worktree.
- When creating a new worktree for any other repo, place it under `/Users/xuanyu.wang/repos`.
- Prefer existing `<project>/project.yaml` as the machine-readable handoff surface.
- If an existing project folder lacks `project.yaml`, add it from `templates/project.yaml` before doing substantial work.
- Record rich session context in `sessions/YYYY-MM-DD/<tool>-<topic>.md`.
- Track tickets and continuing tasks in one canonical `<primary-domain>/work-items/<id>.md`.
- Record an optional primary subdomain, but keep work items, sessions, logs, and decisions at the parent domain; use `subdomains/` for durable reference knowledge only.
- Before final handoff, automatically record concise daily movement in `log/YYYY-MM-DD.md`; do not wait for an explicit logging request.
- Use `workflow/skills/daily-capture/SKILL.md` for daily/session closure and `workflow/skills/weekly-summary/SKILL.md` for weekly synthesis.
- Update the project `README.md` only when the human-facing project state changes.
- Treat the work as incomplete if durable investigation findings remain only in chat history or terminal output.
- Migrate legacy projects gradually when they are reopened or their evidence is needed; synthesize and point to the canonical domain before any separately reviewed removal.

## Write boundaries

In this repo, agents may update only:

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

Do not invent new top-level directories or sidecar conventions without first updating `workflow/ai-operating-model.md`.

## Separation of concerns

- Do not treat this repo as the place for product code changes.
- Code changes belong in the target source repo or its worktree.
- This repo holds context, plans, execution records, and durable synthesis.
