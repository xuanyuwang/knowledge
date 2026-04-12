# Weekly Summary - Week of 2026-04-06

**Created:** 2026-04-11

## Progresses

### mismatch-scorecard-count

- **PG/CH sync gap investigation**: Identified 116 missing scorecards from ClickHouse for brinks/care-voice (88% missing before March 30, 0% after). Root cause: sync mechanism fixed around March 29-30.
- **Cluster-wide monitoring**: Ran `cron-scorecard-sync-monitor` on all 7 clusters for March 2026, built cluster-wide summary feature with single CLUSTER SUMMARY log and Slack batched messages (go-servers#26838).
- **Key finding**: us-east-1-prod has widespread sync gaps (1,601/57,800 missing, 2.77%) across 22 customers; voice-prod has 450/17,147 missing (2.62%), mostly brinks/care-voice.
- **Backfill execution**: Ran test backfill for Brinks Mar 3-5 (84 scorecards, 100% recovery). Created per-day parallel backfill plan; executed 19 jobs recovering 401/436 (92%), 35 remaining on Mar 13, 23, 27.
- **Root cause of remaining 35**: 34 have empty `conversation_id` (standalone scorecards from external QA system). The `reindexconversations` orphan batch skipped Mar 13 and Mar 27 for unknown reasons. Requires ID-based backfill.
- **Auto-heal design doc**: Created formal design doc for automated monitor+backfill system. Key decisions: new Temporal workflow `backfill-scorecards-by-id`, cron task triggers directly via `temporalClient.ExecuteWorkflow()`, count-first optimization, triage conversation-linked vs standalone, batch at 500 IDs, 3 phases (~4 SWE-weeks).
- **API mismatch investigation**: Documented why Performance page and QM Report show different scorecard counts — different timestamp columns (`scorecard_time` vs `submitted_at`) and data sources (ClickHouse vs PostgreSQL).

### scorecard-template

- **Scored N/A design**: Expanded NumericRadios scored N/A exploration in design doc with three approaches (NAScore field, synthesize options array, skip) — Approach B preferred, deferred for now.
- **FE PR code review (director#17763)**: Fixed multiple bugs in scored N/A implementation:
  - Fixed duplicate N/A button in criterion preview (CriterionInputDisplay.tsx) by filtering isNA options before rendering
  - Simplified isNA option lookup in utils.ts using TypeScript type narrowing
  - Refactored CriteriaLabeledOptions.tsx: unified onAddLabel with isNA param, replaced useEffect with useOnMount + onChange for StrictMode idempotency, used form.getValues() instead of useWatch
  - Fixed StrictMode double-fire creating 2 N/A options in AutoQA dropdowns
  - Documented stale AutoQA dropdown values on criterion recreate as pre-existing bug (newAutoQA object created but never included in form.setValue)
  - Addressed all PR review comments, resolved rebase conflicts with origin/main, fixed Biome lint errors

### agent-quintiles-support

- **PR review responses (go-servers#26616)**: Refactored `extractLowerIsBetterFieldDefs` as standalone function per reviewer suggestion. Declined making `FieldDefinitionNames` a `set.Set[string]` — slice semantics fit better for iteration in SQL building, set is appropriate for membership checks.

## Problems

### Technical Issues

- **Missing scorecards with empty conversation_id**: 34 of 35 remaining missing scorecards after backfill have empty `conversation_id` (external QA system integration). The `reindexconversations` workflow's orphan batch step skipped Mar 13 and Mar 27 for unknown reasons. **Resolution**: Requires new ID-based backfill workflow bypassing conversation-date-range approach.

- **API timestamp column mismatch**: Performance page uses `scorecard_time` (conversation start), QM Report uses `submitted_at` (scorecard submission). Scorecards submitted days after conversation appear in different time windows. **Status**: Documented, no immediate fix planned (design decision).

- **React StrictMode idempotency**: useWatch returns stale snapshot when called in useEffect/useOnMount, causing double-fire bugs in StrictMode. **Resolution**: Use form.getValues() (synchronous) instead of useWatch for reads in mount hooks.

### Learnings from Failures

- **Rebase conflict resolution**: When main has refactored a file you modified (e.g., CriteriaLabeledOptions.tsx with icon imports, renormalized indices, new deletion logic), manually merging requires carefully preserving both main's refactoring AND your feature additions. Using `git checkout --ours` for subsequent conflicts on the same file is safe after the first manual merge.

- **Linter line length limits**: Long if conditions (feature flags + multiple predicates) exceed Biome's line length limit. Breaking into multi-line format is required even if semantically a single check.

## Plan

### Next Week Priorities

1. **Auto-heal implementation**: Start Phase 1 (Temporal workflow `backfill-scorecards-by-id` + direct cron trigger). Target completion: workflow skeleton + PG query for ID fetch.
2. **Scored N/A PR merge**: Address any final review comments on director#17763, get approval, merge to main.
3. **Quintiles PR merge**: Monitor go-servers#26616 for final approvals, merge to main.

### Follow-ups Required

- **Investigate orphan batch skipping**: Why did `reindexconversations` orphan batch skip Mar 13 and Mar 27 in the backfill? Check Temporal workflow logs for those dates.
- **NumericRadios scored N/A**: Decide whether to implement Approach B (synthesize options array from NAScore field) or skip for now based on PM priority.
- **Cluster-wide sync gaps**: Follow up with team on whether to backfill all 1,601 missing scorecards in us-east-1-prod or investigate root cause first.

### Pending Reviews/Decisions

- **director#17763** (scored N/A FE): Waiting for final approval after addressing all review comments and lint fixes.
- **go-servers#26616** (quintiles directionality): Waiting for sebastiancoteanu's approval after refactor.
- **go-servers#26838** (cluster summary): Waiting for CodeRabbit review completion and approval.
