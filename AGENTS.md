# AGENTS.md

This repository uses a shared workflow spec for Codex and other AI tools.

Read `workflow/ai-operating-model.md` before making substantial changes.

## Tool-specific adapter

- Use `workspace/repos.yaml` to resolve the source repo and any named worktree.
- Prefer existing `<project>/project.yaml` as the machine-readable handoff surface.
- If an existing project folder lacks `project.yaml`, add it from `templates/project.yaml` before doing substantial work.
- Record rich session context in `sessions/YYYY-MM-DD/<tool>-<topic>.md`.
- Record concise daily movement in `log/YYYY-MM-DD.md`.
- Update the project `README.md` only when the human-facing project state changes.

## Write boundaries

In this repo, agents may update only:

- project `README.md`
- `project.yaml`
- `log/`
- `sessions/`
- `decisions/`
- `deliverables/`
- `templates/`
- `workflow/`
- `workspace/`

Do not invent new top-level directories or sidecar conventions without first updating `workflow/ai-operating-model.md`.

## Separation of concerns

- Do not treat this repo as the place for product code changes.
- Code changes belong in the target source repo or its worktree.
- This repo holds context, plans, execution records, and durable synthesis.
