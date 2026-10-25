# CONVI-7777 — Dedicated feature enum

**Primary source repo:** `/Users/xuanyu.wang/repos/cresta-proto`.
**Branch/worktree:** `xw/convi-7777-assistance-usage` at `/Users/xuanyu.wang/repos/cresta-proto-convi-7777-assistance-usage`.

## User feedback and decision

The reused enums admit values outside this API, such as regular hints. The user requested an API-specific enum instead. This supersedes the previously approved hint/event/moment oneof design.

## Construction

1. Replaced the `AssistanceUsageFeature` wrapper with a dedicated enum in [PR #10283](https://github.com/cresta/cresta-proto/pull/10283), commit `f3ed4fabb8ec019a04ab6740df9bd7948a859092`. `used_features` remains field 2 and now contains deduplicated enum values. The enum exposes only ten scoped categories: behavioral/checklist/KB/workflow hints, GenAI answers, Summary, Notes, KB search, workflow document use, and Chat Suggestions. The three chat suggestion origins map to one category. Regular hints, Smart Compose, Silence/Hold, and email have no enum entries. The zero value is unspecified and must never be returned as qualifying use.
2. Removed unused conversation-event and moment imports. Kept request opt-in, flat conversation/agent records, no-use entries, completeness policy, and logging annotations.
3. Buf build/lint, compatibility against `origin/main`, Bazel generated-Go compilation, clang-format 22.1.8, and diff checks passed. Replacing message values with enum values intentionally changes the wire/JSON representation from the earlier unreleased PR draft. Backend feature mapping remains pending.
4. Updated the existing PR, plan, construction record, and report.

## Verification boundary

HTML syntax and all five construction-step controls passed under a simulated DOM. Browser visual verification remains unavailable under the previously recorded restriction. No backend/runtime, customer-row, or performance claim is made. Git/gh and package tools used configured authentication; no AWS, Okta, Azure, or SSH credential/profile was explicitly selected or read.

Final verification: remote PR head is `f3ed4fabb8ec019a04ab6740df9bd7948a859092`; source worktree is clean. The PR body also carries an automated downstream go-servers `go generate` failure notice; backend/client integration remains follow-up work, and its merge-readiness checkbox was left unchanged.
