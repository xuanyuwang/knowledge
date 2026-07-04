# Session Note - 2026-07-03 - Codex - Workspace Layout Redesign

**Started:** 2026-07-03 20:39 EDT  
**Tool:** Codex  
**Project:** `dev-environment-tips`  
**Goal:** Evaluate whether working repositories should move under `knowledge` to improve AI tool behavior and cross-repo collaboration.

## Source Context

- **Primary repo:** `knowledge`
- **Repo path:** `/Users/xuanyu.wang/repos/knowledge`
- **Worktree path:** `/Users/xuanyu.wang/repos/knowledge`
- **Branch:** `main`
- **Ticket / PR:** none

## Inputs Reviewed

- `/Users/xuanyu.wang/repos/knowledge/workflow/ai-operating-model.md`
- `/Users/xuanyu.wang/repos/knowledge/workspace/repos.yaml`
- Existing `/Users/xuanyu.wang/repos/knowledge/dev-environment-tips/README.md`

## Actions Summary

- Reviewed the current knowledge operating model and repo registry.
- Confirmed the current model intentionally keeps `knowledge` as the durable context repo and source repos as siblings under `/Users/xuanyu.wang/repos`.
- Added missing protocol files for this project so future environment-layout decisions have a durable home.
- Follow-up implementation created `/Users/xuanyu.wang/repos` as a lightweight workspace metadata Git repo with child repositories ignored.

## Findings

- Moving source repos physically under `knowledge` can be made lean with `.gitignore`, but it creates a nested-repository layout that is awkward for Git and does not guarantee better AI tooling behavior.
- AI tool feature gating usually depends on the opened workspace root and Git discovery behavior. A nested ignored repo may still be treated as an independent Git repo by tools that discover the inner `.git`, while tools opened at `knowledge` may ignore or poorly model the nested repos because they are gitignored.
- The current documented model already recommends using `/Users/xuanyu.wang/repos` as the broad workspace root and keeping source repos/worktrees under that parent.
- Mature alternatives are a thin top-level workspace Git repo, editor/workspace files, symlinks or manifest files, and Git submodules only when pinning exact external repo commits is desired.

## Decisions Made

- No physical repository move was performed.
- Adopted `/Users/xuanyu.wang/repos` as a lightweight `workspace-meta` Git repo while preserving sibling source repos.

## Follow-ups

- Keep `/Users/xuanyu.wang/repos/workspace.code-workspace` aligned with the local checkout set.

## Links

- none
