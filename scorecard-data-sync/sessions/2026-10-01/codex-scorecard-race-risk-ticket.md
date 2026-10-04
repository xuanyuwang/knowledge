# Scorecard race-risk backlog ticket

- Date: 2026-10-01
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Source branch/worktree context: no source checkout used; ticket planning from knowledge records and Linear, with no code edits
- Primary domain: `scorecard-data-sync`

## Inputs and assessment

Reviewed the [CEL Retreat incident](../2026-09-28/codex-rcg-cel-retreat-score-mismatch.md), its [work item](../../work-items/rcg-cel-retreat-score-mismatch.md), the earlier [RCG Casino mismatch](../../work-items/rcg-casino-dtq-score-mismatch.md), and current Linear [CONVI-6076](https://linear.app/cresta/issue/CONVI-6076). The CEL Retreat PostgreSQL parent/criterion inconsistency is verified, and ClickHouse mirrored it. The exact concurrent request order is not verified. PR #32447 postdates the incident and addresses one likely submit/autosave path, leaving a broader evaluation appropriate.

## Action and handoff

Created [CONVI-7784](https://linear.app/cresta/issue/CONVI-7784/systematically-evaluate-scorecard-write-race-conditions-and) for Convo Intelligence in Backlog. It requests a complete mutation-path inventory, writer/field map, concurrency matrix across UI and backend callers, deterministic tests for the highest-risk interleavings, invariants for parent/child consistency and projection, and a ranked mitigation plan. It links CONVI-6076 as related completed work. The chart still showing 50% after the user edit remains with the incident work item because its query path is unverified.

No credentials were used. No source code or production data was changed.
