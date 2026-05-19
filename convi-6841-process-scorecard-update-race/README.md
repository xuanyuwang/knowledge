# CONVI-6841: Lending Club process scorecard failed to update on creation

**Status**: Fix in progress  
**Ticket**: https://linear.app/cresta/issue/CONVI-6841  
**Customer**: Lending Club  
**Worktree**: `/Users/xuanyu.wang/repos/go-servers`  
**Branch**: `convi-6841-update-scorecard-write-db-read`  
**PR**: https://github.com/cresta/go-servers/pull/27934

## Problem

Lending Club users reported a submission error while working on coaching. After tracing the flow, the actual failure was not a coaching session submission issue. The failing user-facing error was:

- `Failed to update scorecard.`

The impacted flow is creation of a brand-new process scorecard in Director. Users can hit an update failure immediately after creation.

## Current Working Model

For a new process scorecard, Director does:

1. `CreateScorecard`
2. stores the returned scorecard name in client state
3. follows up with autosave/debounced `UpdateScorecard` calls

The backend `UpdateScorecard` path begins by reading the scorecard row from Postgres. If that read goes to a lagging replica, the freshly created scorecard may not be visible yet and the RPC returns `NOT_FOUND`.

## Key Findings

- The frontend toast `Failed to update scorecard.` maps to `UpdateScorecard`, not `SubmitScorecard`.
- API logs showed gRPC status code `5 NOT_FOUND` on `UpdateScorecard`.
- In the backend, `UpdateScorecard` returns `NOT_FOUND` when `dao.GetScorecards(...)` cannot find the scorecard by `customer/profile/resource_id`.
- `s.appsDB.DB(ctx)` is backed by GORM `dbresolver` with read replicas enabled.
- `provider.Get(ctx)` does not force primary reads by default.
- The `shared-go` provider tests confirm that outside a transaction, an immediate read after a write can miss the new row because the read may go to a replica.
- This is a classic read-after-write inconsistency caused by replication lag.

## Root Cause

`UpdateScorecard` was doing its initial scorecard existence lookup on a plain DB handle:

- read could route to a replica
- replica could lag behind the preceding `CreateScorecard`
- the just-created scorecard row was not yet visible
- `UpdateScorecard` returned `NOT_FOUND`
- Director surfaced that as `Failed to update scorecard.`

## Fix

Narrow backend fix:

- pin only the initial `dao.GetScorecards(...)` lookup in `UpdateScorecard` to the write DB using `db.Clauses(dbresolver.Write)`

This keeps the fix scoped to the read-after-create race without changing the rest of the function's DB behavior.

## Validation Notes

- Local focused test execution was limited because the coaching test suite depends on Docker-backed ClickHouse fixtures.
- Initial PR attempt was too broad: forcing the whole function to the write DB caused a CI failure in `TestScorecardAsyncOrder`.
- The fix was narrowed so only the scorecard existence lookup uses the write DB.

## Current Assets

- Linear issue updated with investigation summary
- GitHub PR opened with backend fix
- Knowledge entry created to track investigation, root cause, and fix history
