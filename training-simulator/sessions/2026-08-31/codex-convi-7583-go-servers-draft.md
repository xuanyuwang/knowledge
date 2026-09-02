# CONVI-7583 go-servers backend implementation

**Date:** 2026-08-31
**Source repo:** `go-servers`
**Worktree:** `/Users/xuanyu.wang/repos/go-servers-convi-7583`
**Branch:** `xw/convi-7583-session-reporting-backend`
**Base:** `origin/main` at `95068db42b`

## Intent

Prepare the backend query and aggregation changes for review without committing them.

## Drafted shape

- Keep existing request user/group expansion and task filtering. Match `ListDirectorTasks` semantics: a non-empty `audienceUserNames` list selects tasks containing any requested user but does not project the matched task's audience; an empty list means no audience filter. `direct_team_only`, row-level ACL, manager manageable-user filtering, and agent self-scope remain outside this draft.
- Treat each filtered task audience as the assignment facts.
- Keep assignments and assignees as the aggregation root so tasks and users with zero runs are returned.
- Extract the public task-run List implementation into a shared internal method accepting `pageSize`. Keep the public 1,000-row path, while stats requests 100,001 rows as an overflow probe for its 100,000-row safety bound.
- Reuse the existing conversation and quiz enrichment without changing those loaders.
- Count all assigned-agent task runs within the session before official-attempt reduction.
- Select the official attempt by greatest `(created_at, task-run resource name)` and let a newer incomplete retry supersede an older settled result.
- Preserve existing settled-result detection: conversation score presence and quiz submission presence.
- Exclude stale modules from completion, score, and pass calculations, but retain their latest runs in `task_runs` and include all their runs in the session attempt count.
- Return numeric zero with `not_applicable = true` instead of exposing the internal negative score sentinel.

## Validation state

- Upgraded `cresta-proto` from v2.21.12 to v2.21.27 and synchronized `go.mod`, `go.sum`, `deps.bzl`, and Gazelle output.
- Added zero-run, latest-incomplete, deterministic tie-break, full-task-audience, mixed conversation/quiz attempt-count, stale-module, and 1,001-attempt coverage.
- Passed `mage cleanBuild`, `gofmt`, `git diff --check`, `go vet ./apiserver/internal/trainingsimulator`, the full package test suite, `bazel run //:gazelle`, and `bazel test //apiserver/internal/trainingsimulator:trainingsimulator_test`.
- `mage Lint apiserver/internal/trainingsimulator` could not start because the installed golangci-lint binary was built with Go 1.24 while the repository targets Go 1.25.

## Existing-behavior constraint audit

- Clearly requested changes: assignment-rooted zero-run rows over each matched task's full audience, all-session attempt counting, configurable task-run List page size, and zero-plus-`not_applicable` unavailable aggregates.
- User-approved additional constraints: a 100,000-run hard ceiling and deterministic resource-name tie-breaking for equal timestamps.
- Restored existing behavior for settled-result detection, malformed DirectorTask names, task-run ordering, and run-backed lesson selection. Zero-run tasks use their configured lesson as the necessary fallback.
- The public task-run List behavior is otherwise unchanged.

## PR review finding

- `ListDirectorTasks.AudienceUserNames` is an existential task filter: it retains a task when the expanded task audience contains any requested user (`HasAny`).
- It does not project the returned task audience down to the requested users. A shared task assigned to A and B is returned when filtering for A, still with both A and B in its audience.
- The initial PR intersected `AgentPerformance` to A while retaining B's runs in task metrics, creating a population mismatch. The follow-up restores full-task-audience semantics so both task metrics and agent rows consistently include A and B.

## Performance review finding

- The old 1,000-row cap cannot produce complete attempt counts.
- Raising one internal List call to 100,001 still materializes all DB models, score-ID slices, score/result maps, and converted task-run protos inside the RPC process before aggregation. Being internal does not bound heap usage.
- The enrichment helpers issue single `IN` queries over all score IDs; near the 100,000-row ceiling they can also exceed PostgreSQL's extended-query parameter limit before the explicit overflow check is useful.
- The scalable shape is to page or query minimal task-run fields, fold attempt counts and latest-per-key state, then enrich only the retained latest runs. A lower measured ceiling is a smaller interim option, but 100,000 is not demonstrated safe by the current tests.

## Credentials

No credentials were read or used.

## Next steps

- Review [go-servers#31780](https://github.com/cresta/go-servers/pull/31780).
- Decide row-level authorization and agent self-scope separately; current Director callers do not require `direct_team_only`.
- Rerun repository lint with a Go 1.25-compatible golangci-lint binary before merge.
