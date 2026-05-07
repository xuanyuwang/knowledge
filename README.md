# Knowledge

This repo is the operating system for staff-level engineering work.

It sits alongside source repos under `~/repos` and captures the context that should outlive a single branch, prompt, terminal session, or AI tool run.

## Core model

- Source code truth lives in the target repo or repo worktree.
- Reasoning truth lives here: investigations, design tradeoffs, execution notes, reviews, decisions, and synthesis.
- Every durable artifact should point back to a concrete repo, worktree, branch, commit, ticket, or PR.

## Why this exists

This repo exists to make growth toward **Staff Engineer** explicit, measurable, and repeatable.

It is used to:

- track where work is operating at senior level versus staff level
- preserve staff-level artifacts instead of losing them in chat logs
- turn day-to-day delivery into reusable judgment, design records, rollout plans, and retrospectives
- give Codex, Claude Code, and other AI tools a shared operating protocol

## Canonical interfaces

The workflow treats these files as the main interfaces:

- `workflow/ai-operating-model.md` - shared operating spec for humans and AI tools
- `workspace/repos.yaml` - repo and named worktree registry
- `<project>/project.yaml` - machine-readable project state
- `<project>/README.md` - human-readable project summary
- `<project>/log/YYYY-MM-DD.md` - daily progress record
- `<project>/sessions/YYYY-MM-DD/*.md` - raw session notes
- `CLAUDE.md` and `AGENTS.md` - tool-specific entrypoints into the shared spec

## Project-first structure

The canonical working unit is a **project folder** at the repo root.

Each active engineering project should eventually contain:

- `project.yaml`
- `README.md`
- `log/`
- `sessions/`
- `decisions/`
- `deliverables/`

Legacy folders are being backfilled incrementally. When reopening an existing project that does not yet have `project.yaml`, add or refresh it before doing substantial work.

## Promotion path

The intended flow is:

1. A session note captures investigation or execution details.
2. The daily log records the meaningful movement for that day.
3. The project `README.md` is updated when project state changes.
4. Important decisions are promoted into `decisions/`.
5. Reusable lessons are promoted into `general-learnings/`, `blog/`, or `weekly-summary/`.

## Staff track anchors

- Senior vs Staff gap framework: `train-for-staff/senior-to-staff.md`
- Staff project framing: `train-for-staff/staff-project.md`
- Resume bullets and synthesis: `train-for-staff/resume-snippets.md`
