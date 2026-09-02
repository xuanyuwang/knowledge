# Codex Session: CONVI-7583 Proto Contract

## Context

- **Date:** 2026-08-31
- **Primary source repo:** `/Users/xuanyu.wang/repos/cresta-proto`
- **Worktree:** `/Users/xuanyu.wang/repos/cresta-proto-convi-7583`
- **Branch:** `xw/convi-7583-session-stats-contract`
- **Base:** local `origin/main` at `83f6d6ec4de7ff6fce456c78e192eed5fc9d3cea`

## Authorization Decision

Keep `AGENT` on `RetrieveTrainingSimulatorTaskStats`. Director has two consumers:

- manager/admin Training Sessions reporting; and
- agent-facing Assigned Training Sessions, which requests one agent and uses the result for module progress, attempt presence, session status, and score.

Removing `AGENT` would break the second consumer. CONVI-7583 must instead enforce self-only rows for agent-only callers and manageable-user scope for managers in go-servers.

## Change

Added `AgentPerformanceEntry.attempt_count = 8` to `cresta/v1/trainingsimulator/stats.proto`. It counts conversation and quiz task runs across required modules before latest-attempt selection. No service annotation or generated file changed.

The main cresta-proto checkout already contained unrelated uncommitted Training Simulator proto work. It was preserved untouched; the new worktree was created from `origin/main` rather than from the dirty checkout.

## Validation

- `buf lint --path cresta/v1/trainingsimulator/stats.proto` — passed with only the repository's existing deprecated `DEFAULT` category warning.
- `buf build --path cresta/v1/trainingsimulator/stats.proto -o /tmp/convi-7583-stats.binpb` — passed.
- `bazel build //cresta/v1/trainingsimulator:all` — passed, 3 targets / 581 actions.
- `git diff --check` — passed.
- `clang-format` was not available locally; the five-line change follows the surrounding Google-style formatting. `buf format --diff` proposes unrelated whole-file formatting changes and was not applied.

## Pull Request

- Commit: `7d425251e50ad19faccd8e024245773789878651`
- PR: [cresta-proto#9716](https://github.com/cresta/cresta-proto/pull/9716)
- Title: `[CONVI-7583] Add session attempt count`
- State at creation: open, non-draft; initial CI queued/in progress

## Credential Use

Local validation used no credentials. The push used the individually cleared `~/.ssh/id_ed25519` GitHub host entry; PR creation used the individually cleared `gh` keyring entry for `github.com` user `xuanyuwang`. The restricted emergency SSH certificate was not used.
