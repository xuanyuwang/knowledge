# CONVI-7671 go-servers conflict resolution

**Date:** 2026-09-16
**Source repo:** `/Users/xuanyu.wang/repos/go-servers-convi-7671`
**Branch:** `xw/convi-7671-list-comments-size`
**PR:** [go-servers #32304](https://github.com/cresta/go-servers/pull/32304)

## Resolution

- Fetched current `main` over authenticated HTTPS and rebased the feature branch.
- The implementation and request-inlining commits replayed cleanly.
- The only conflict was the feature branch's cresta-proto v2.24.3 dependency commit versus current main's newer v2.24.7.
- Dropped the obsolete dependency-only commit, retaining main's v2.24.7 rather than downgrading.
- Verified the v2.24.7 generated binding contains `StripInlineImages` and `GetStripInlineImages()`.

## Validation

- `git diff --check origin/main..HEAD` passed.
- Focused collaboration and coaching Bazel targets passed with filters covering `TestListComments`, `TestStripInlineImages`, and `TestExportScorecards`.
- Force-pushed with an exact lease; PR #32304 is mergeable at `a274bc548a`.
- Updated the PR body from v2.24.3 to inherited v2.24.7 while preserving the existing CodeRabbit release-notes block.

## Credential use

- Used the GitHub CLI keyring token for HTTPS fetch and push.
- Did not use SSH credentials.
