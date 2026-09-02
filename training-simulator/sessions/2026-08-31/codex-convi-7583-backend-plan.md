# Codex Session: CONVI-7583 Backend Plan

## Context

- **Date:** 2026-08-31
- **Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`
- **Source context:** `go-servers`, `cresta-proto`, and `director` main checkouts inspected read-only; no implementation branch/worktree created
- **Objective:** Identify the precise backend data delta, decide whether protobuf changes are needed, and create the CONVI-7583 implementation plan.

## Findings

- The only new public response datum is per-agent `attempt_count`, counted across all matching conversation and quiz runs for current required modules before latest-attempt selection.
- A proto change is required only for `AgentPerformanceEntry.attempt_count = 8`. A follow-up consumer trace found that agent-facing Assigned Training Sessions calls this RPC, so `AGENT` must remain; backend authorization must enforce self-only rows for agent-only callers.
- No new result enum/status/warning field is needed after the accepted zero/false collapse. Existing completion status, passed, score, and not-applicable fields are sufficient.
- No schema change is needed. The backend can read task-run identity plus linked conversation/quiz outcomes from existing tables.
- The current handler has several computation/query defects to correct: it returns no task for an all-zero-run result set, calls a one-page task-run RPC capped at 1,000, ignores `direct_team_only`, returns full task audiences after task matching, has no deterministic resource-ID tie break, includes stale runs in returned agent task runs, uses a negative score sentinel, and has fixtures that mix 0–100 conversation scores with 0–1 quiz scores despite the public 0–1 contract.
- Authorization must be injected as a collection filter before audience expansion and result loading. Calling `ListDirectorTasks` is not a substitute for row-level manageable-user scope.

## Output

- Created `deliverables/convi-7583-backend-implementation-plan.md` with the exact contract, source data, classification matrix, proto decision, loading design, implementation sequence, validation plan, and PR split.
- Updated `work-items/CONVI-7583.md` and `project.yaml` to mark technical planning active.

## Credential Use

No credentials were read or used. All inspection was local and read-only.
