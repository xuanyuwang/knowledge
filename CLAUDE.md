# CLAUDE.md

This repository uses a shared workflow spec.

Read `workflow/ai-operating-model.md` before doing substantial work in this repo.

## Claude-specific adapter

- Respect `.claude/settings.json`. Worktrees are intentionally disabled for the `knowledge` repo itself.
- Use `workspace/repos.yaml` to resolve the target source repo or named worktree instead of guessing from prose.
- Use `<project>/project.yaml` as the primary machine-readable handoff surface.
- If an existing project lacks `project.yaml`, add it from `templates/project.yaml` before substantial work.
- Put rich Claude session context in `sessions/YYYY-MM-DD/claude-<topic>.md`.
- Keep daily logs concise; keep session notes detailed.

## Allowed write targets

Claude may update only:

- project `README.md`
- `project.yaml`
- `log/`
- `sessions/`
- `decisions/`
- `deliverables/`
- `templates/`
- `workflow/`
- `workspace/`

Do not invent new top-level structures without first updating `workflow/ai-operating-model.md`.

## Separation of concerns

- Product code belongs in the target repo or repo worktree.
- This repo exists for investigations, designs, decisions, reviews, execution records, and synthesis.
