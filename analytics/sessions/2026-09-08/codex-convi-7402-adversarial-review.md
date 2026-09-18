# CONVI-7402 adversarial PR review

- Date: 2026-09-08
- Source repo: `/Users/xuanyu.wang/repos/go-servers`
- Worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7402`
- Branch: `convi-7402-bswift-users-getting-qa-score-stats-errors`
- PR: https://github.com/cresta/go-servers/pull/31335
- Remote head: `1b2a95bd9a21c2453cd70609ebd20c36f0365ed7`.
- Fetched main: `98583180ee54f812a19390fbd356afa87cc24fce`.
- Local rebased head: `12117463df3318998e944551b63913c6110ba6b6`; no conflicts, all ten patches equivalent in range-diff; no push.
- Source worktree clean before review and after removing temporary reproduction tests.

## Findings and evidence

[Full review](../../deliverables/convi-7402-pr-31335-adversarial-review.md) contains exact locations, trigger shapes, expected/actual results, confidence, tests, fix directions, semantic coverage matrix, shared-helper caller audit, operational risks, and credential inventory.

Four high-confidence P2 findings remain: submit-time QA conversations still filter annotations by conversation start time; shared rejection skips multi-ID overlap shapes; newly accepted numeric overlap uses a converter that ignores endpoint flags; empty metadata-value oneofs produce invalid SQL. Findings 2–4 explicitly distinguish inherited helper limitations from this PR's incomplete guard/newly accepted shapes.

ClickHouse 26.8.2.7 executed both committed SQL goldens and generated synthetic fixtures. Ordinary string/missing, latest-update, duplicate, independent include/exclude cases pass. Full submit-time queries disagree: conversations includes an out-of-window disallowed value, stats correctly excludes it. Numeric singleton and open/closed boundary fixtures fail; empty oneof raises Code 43. Existing targeted parser/overlap tests and both QA suites pass (the latter use mocked ClickHouse); broader focused tests expose a whitespace assertion broken by CTE extraction. Live ES differential and production-scale distributed tests were not run.

[Evidence files](convi-7402-review-evidence/) preserve temporary Go tests, full SQL, fixture scripts, and outputs. Production code was not fixed. No external writes were made. Credentials used were the individually inspected GitHub SSH key, GitHub CLI keychain account and repository-local embedded PostgreSQL test credential, as detailed in the review.

## Next actions

Address the four findings and whitespace assertion in a separately authorized implementation pass. Add runtime submit-time+overlap and boundary/group-cardinality regressions. Verify production scan cost and MV freshness. Retain local rebased history; remote head remains unchanged.
