# PR 31265 Appeal Workflow Review

## Context

- Source repo: `/Users/xuanyu.wang/repos/go-servers`
- Review worktree: `/Users/xuanyu.wang/repos/go-servers-pr-31265-review`
- PR: [cresta/go-servers#31265](https://github.com/cresta/go-servers/pull/31265)
- Head: `96129b6f4ac1c54e332085159238d6db2d947371`
- Base: `origin/main`
- Primary subdomain: `appeals`

## Change Reviewed

- Appeal resolution finalizes an unsubmitted original by copying resolve submission metadata and setting `manually_scored = true`.
- Appeal reporting now requires only the appeal request to be submitted; the original and replica may be unsubmitted.

## Review Findings

1. **High — concurrent submission metadata can be lost.** The resolve path reads the original without a row lock, decides `finalizedOriginal`, and later performs a full save with submission fields. A normal `SubmitScorecard` that commits between those operations can have its `submitted_at` and `submitter_user_id` overwritten by the resolver. Re-read under lock or use a conditional update that stamps submission fields only while `submitted_at IS NULL`.
2. **High, rollout-gated — publish-enabled customers lose agent visibility.** Finalization sets `manually_scored = true` but deliberately leaves `published_at` null. With `enableScorecardPublish`, agent get/list access requires either publication or an unmodified auto score, so the finalized original becomes hidden.
3. **Medium — no regression coverage for either new branch.** Existing appeal-resolve tests use the submitted `Scorecard1` fixture. Add tests for finalization metadata/backfill protection and analytics eligibility, plus the concurrent-submit case.

## Semantics Confirmed

- Appeal creation already permits an unsubmitted original; it checks appeal-request edit permission but not original submission state.
- Relationship chain: original <- replica <- appeal request <- optional resolve.
- The replica remains the appeal-time comparison snapshot.
- `manually_scored = true` protects resolved values from auto-score backfill.
- Already-submitted originals retain their original submission metadata.
- Analytics uses submitted appeal-request time as the workflow boundary; a draft resolve is ignored.
- Original reviewer filters and QA-analyst grouping still depend on the original submitter, which is absent before an auto-scored original is finalized.

## Validation

- Inspected the full two-file PR diff and relevant create, submit, access, analytics, DAO, backfill, and test code.
- CodeRabbit CLI `0.7.3` completed against `origin/main` with zero findings.
- GitHub CI was green except the coaching coverage job and codeowner approval were still pending during review.
- No source files were modified.
