# CONVI-6968: Submitted scorecards and submitter-attributed Leaderboards

**Record state:** Historical synthesis; live product, ticket, PR and deployment state not checked.
**Primary domain:** analytics
**Primary subdomain:** leaderboard
**Official ticket:** [CONVI-6968](https://linear.app/cresta/issue/CONVI-6968)
**Substantive source dates:** 2026-06-01 to 2026-06-08
**Consolidated:** 2026-10-25

## Objective and source context

Preserve the findings and continuation path from [CONVI-6968](CONVI-6968.md) in the Analytics domain. Source repositories: `/Users/xuanyu.wang/repos/director`; `/Users/xuanyu.wang/repos/go-servers`; `/Users/xuanyu.wang/repos/cresta-proto`. These are source locations resolved through `workspace/repos.yaml`, not claims about an active branch or worktree. Historical branch/worktree details remain in the preserved evidence.

## Findings and decisions

Agent submitted-scorecard counts need a dedicated MANUALLY_SUBMITTED query; the performance query with an empty status filter covers a wider population. RetrieveQAScoreStats provides aggregate counts but no template dimension; RetrieveQAConversations carries scorecard IDs and template IDs for client grouping.

Backend PRs #28525/#28526/#28530/#28527 and proto #8803 added a distinct submitter axis. QAAttribute.users/groups filter the conversation agent, scorecard_reviewer_audience filters the submitter, and SCORECARD_SUBMITTER groups submitters. These axes are not interchangeable. The June decision accepted QA conversation-time semantics for Manager migration, but [CONVI-7162](CONVI-7162.md) records the later July decision restoring submit-time semantics; do not reuse June's time-parity waiver as current guidance.

## Recorded outcome and limits

Backend historically merged; FE integration pending/active in June notes. Later behavior is traced through CONVI-7162. The consolidation reads retained documents only; historical scripts and queries were not executed and services were not authenticated.

## Next actions

When extending drawers or counts, preserve submitted status, correct identity axis, time basis and aggregate/detail filters. Read the later submit-time decision before changing Manager reporting.

## Evidence and timeline

- 2026-06-01 to 2026-06-08 — Investigation, implementation or reference updates recorded in the original material.
- 2026-10-25 — Synthesized into Analytics; original tree preserved under the [historical overview](../sessions/2026-10-25/convi-6968-schwab-leaderboard-launch-legacy-evidence/README.md).
- Reviewed evidence: [README.md](../sessions/2026-10-25/convi-6968-schwab-leaderboard-launch-legacy-evidence/README.md); [log/2026-06-08.md](../sessions/2026-10-25/convi-6968-schwab-leaderboard-launch-legacy-evidence/log/2026-06-08.md).
- Durable reference home: [leaderboard](../subdomains/leaderboard/README.md).
