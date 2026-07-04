# Session Note - 2026-06-28 - Codex - Knowledge Project Autosave

**Started:** 2026-06-28 09:55 America/Toronto  
**Tool:** Codex  
**Project:** `workspace`  
**Goal:** Make the work-and-log approach automatically save durable investigations under a `knowledge` project.

## Source Context

- **Primary repo:** `knowledge`
- **Repo path:** `/Users/xuanyu.wang/repos/knowledge`
- **Worktree path:** `/Users/xuanyu.wang/repos/knowledge`
- **Branch:** `main`
- **Ticket / PR:** none

## Inputs Reviewed

- `/Users/xuanyu.wang/repos/knowledge/README.md`
- `/Users/xuanyu.wang/repos/knowledge/workflow/ai-operating-model.md`
- `/Users/xuanyu.wang/repos/knowledge/AGENTS.md`
- `/Users/xuanyu.wang/repos/knowledge/CLAUDE.md`
- `/Users/xuanyu.wang/repos/knowledge/templates/session-note.md`
- `/Users/xuanyu.wang/repos/knowledge/templates/project.yaml`
- Existing source repo `AGENTS.md` / `CLAUDE.md` files for `director` and `go-servers`

## Actions Summary

- Added `/Users/xuanyu.wang/repos/AGENTS.md` and `/Users/xuanyu.wang/repos/CLAUDE.md` as parent workspace instruction files so future Codex and Claude Code sessions started from `/Users/xuanyu.wang/repos` see the knowledge logging protocol before entering a specific source repo.
- Updated `workflow/ai-operating-model.md` so substantial work is explicitly project-first, then work-and-log.
- Updated `knowledge/AGENTS.md` and `knowledge/CLAUDE.md` so tool-specific adapters treat unsaved durable investigation findings as incomplete work.
- Added `workspace/project.yaml` and `workspace/README.md` because the workspace protocol itself is now an active knowledge project.

## Findings

- The `knowledge` repo already had the right project model, templates, and lifecycle guidance.
- The missing piece was an instruction surface above individual source repos. Without parent-level agent adapters, a session opened at the workspace root did not automatically know to create or update a `knowledge` project before doing investigation work.
- Individual source repos have their own `AGENTS.md` / `CLAUDE.md`; the parent-level adapter reduces the need to duplicate knowledge logging instructions across every repo.

## Decisions Made

- Use the parent workspace `AGENTS.md` as the primary Codex entrypoint for cross-repo knowledge logging.
- Use the parent workspace `CLAUDE.md` as the matching Claude Code entrypoint.
- Keep the canonical workflow in `knowledge/workflow/ai-operating-model.md`.
- Record workflow-level changes under the `knowledge/workspace` project.

## Follow-ups

- Consider adding a small helper script to create a project skeleton from `templates/` when a ticket or project slug is provided.

## Links

- `/Users/xuanyu.wang/repos/AGENTS.md`
- `/Users/xuanyu.wang/repos/CLAUDE.md`
- `/Users/xuanyu.wang/repos/knowledge/workflow/ai-operating-model.md`
