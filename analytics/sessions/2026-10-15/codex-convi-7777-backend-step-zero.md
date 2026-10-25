# CONVI-7777 — Backend Phase I Step 0

**Primary domain:** analytics; **subdomain:** assistance-insights.
**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`.
**Branch/worktree:** `xw/convi-7777-assistance-usage` at `/Users/xuanyu.wang/repos/go-servers-convi-7777-assistance-usage`, HEAD `8d6a2b0b3666bde1a6bcb44a1fbd3848f1dfb498`.

## User contract

Backend-only Phase I. User corrected “step 1” to “step 0” before implementation. Complete baseline inspection/reporting only. Every implementation step must explain current state, change, reason, resulting state and validation; update the report and stop for review before proceeding. Do not treat acceptance of one step as acceptance of later steps.

## Outcome and evidence

[Baseline contract](../../deliverables/convi-7777-backend-phase-one-baseline.md) records pinned source, the two modified files and untracked combined-query draft, hashes proving preservation, dependencies and planned cohort acceptance cases. Live `gh pr view` confirmed PR #10283 merged at `f3ed4fabb8ec019a04ab6740df9bd7948a859092`; the final feature enum supersedes earlier wrapper messages. Cached v2.31.32 has neither dedicated RPC nor draft fields, though Go/Bazel pins agree. No published compatible version was selected. Inspected both ordinary and metadata cohort paths at backend HEAD, plus user filter and start-time behavior. The shared selection is reusable but keys are lost at aggregation. Metadata and Agent Assist ON/OFF paths must be preserved in the extraction.

Attempted `go test ./insights-server/internal/analyticsimpl -run '^TestRetrieveConversationStatsValueOrMissingMetadataFilter$' -count=1` in the existing ticket worktree. Compilation failed before tests ran because the superseded draft references absent proto fields/methods: `AssistanceUsageIncluded`, `GetIncludeAssistanceUsage`, and `ConversationsWithAssistanceUsedCount`. This is the existing baseline blocker, not a test failure caused by Step 0. No code was changed to repair it. Backend diff check passed, and the tracked draft diff plus untracked query file retained their recorded SHA256 hashes.

## Artifact changes

Updated the current interactive report, preserving the original protobuf report as a separate HTML file; added baseline contract and updated construction record, roadmap context, canonical work item, domain summary and daily log. No source/dependency/Director files were edited. No Git staging, commit, push, reset or stash occurred. No customer DB or live endpoint was queried. GitHub and dependency reads used configured tool authentication; no AWS, Okta, Azure or SSH credential/profile was explicitly selected or read.

## Next checkpoint

Await user acceptance of Step 0 and proposed Step 1. Step 1 should preserve/recover the superseded draft before replacing ticket-specific old changes, extract/share filtered conversation/agent/start-time keys from both cohort paths, establish parity, update report and stop. Feature queries, numerator, grouping, buckets and RPC implementation remain later checkpoints. Details=true rejection is proposed, not approved.

## Report verification

HTML parsed successfully, element IDs are unique, local source/history links resolve, and embedded JavaScript passed `node --check`. The localhost report was opened in the in-app browser; all four topics and both snapshot/change views were exercised, including Previous/Next controls and disabled endpoints. The visible report layout was inspected at the current browser viewport. A separate narrow mobile viewport was not tested. The prior protobuf report was preserved before rewriting the current report. Scoped Markdown diff checks passed. Backend draft diff and untracked query file SHA256 values match the initial inspection.
