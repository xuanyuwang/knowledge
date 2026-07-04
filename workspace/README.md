# Knowledge Workspace Operating Protocol

This project tracks the shared workspace rules that let AI tools and humans coordinate across repos under `/Users/xuanyu.wang/repos`.

## Scope

- Maintain `workspace/repos.yaml` as the source of truth for repo paths and named worktrees.
- Keep the `knowledge` operating model explicit enough for Codex, Claude Code, and other tools to save durable reasoning in project folders.
- Capture workflow-level fixes that do not belong to a product-ticket project.

## Current State

The workspace now has parent-level `/Users/xuanyu.wang/repos/AGENTS.md` and `/Users/xuanyu.wang/repos/CLAUDE.md` files that tell Codex and Claude Code sessions started from the repo parent to use `/Users/xuanyu.wang/repos/knowledge` as the durable context layer.

`/Users/xuanyu.wang/repos` is now also a lightweight `workspace-meta` Git repository. It tracks shared workspace metadata such as `AGENTS.md`, `CLAUDE.md`, `README.md`, and `workspace.code-workspace`, while ignoring child source repositories so each child repo keeps its own Git history and status.

Substantial investigations, designs, reviews, and multi-step execution should be project-first: open or create a project under `knowledge`, create or update a session note, then update the daily project log before handoff.

## Key Artifacts

- `/Users/xuanyu.wang/repos/AGENTS.md`
- `/Users/xuanyu.wang/repos/CLAUDE.md`
- `/Users/xuanyu.wang/repos/README.md`
- `/Users/xuanyu.wang/repos/workspace.code-workspace`
- `workflow/ai-operating-model.md`
- `workspace/repos.yaml`
- `AGENTS.md`
- `CLAUDE.md`
