# CONVI-7874 semantic review — October 26, 2026

Primary domain: analytics. Subdomain: insights-user-filter.
Primary source repo: /Users/xuanyu.wang/repos/go-servers.
Checkout: main at aca4c1377468f4c439c8e2428ea5640bc318f6bc; unchanged during review.
Reviewed PR: https://github.com/cresta/go-servers/pull/34439
Head: db800e7d2f64fe2c0763bb4557dc120c60a5db50.
Base and merge base: 9e6d8110a6b1ad6ba67b35d7746cf4410c984adc.
Director request-construction source: f47845f4f7f17d4da582d8d480dfec7b2ac1c1b6.

## Verdict

Request changes. The legacy Sunbit-shaped case works in a targeted compiled probe, but two P2 findings remain. The previous October 23 artifact only compared the two fixes and did not provide merge approval.

1. New regression at common.go:1293-1294: for root callers, merging raw dynamicGroupMemberUsers after MoveFiltersToUserFilter reintroduces inactive users. Explicit active A + dynamic inactive B + ExcludeDeactivatedUsers=true yields A/B at head; identical base fixture yields A. UserService ListUsers includes active/inactive by default, and ListUsersFromDynamicGroups sets no state restriction, so the supplemental B is reachable. The normal Director conversation request supports this combination. No cross-ACL leak is demonstrated; it violates the selected active-user filter.
2. Incomplete handling at common.go:1285-1286 and 1239-1240: early empty checks operate on groups before dynamic_groups members are combined. Root + empty Team + dynamic Group with A returns empty; expected A. A non-root inaccessible Team + allowed dynamic Group also returns empty in split format, while legacy mixed format works at head. The inaccessible Team may require stale/direct input, but the empty-Team root scenario does not. These mixed cases were already broken before the PR; they are gaps in the new root/split/mixed-selection support, not newly introduced regressions.

Suggested repair: assemble the complete selected audience before activity filtering and empty decisions; apply active-user and ACL intersections to that complete audience once. Never append raw dynamic members again afterward. No fix was implemented.

## Verification

- Read exact six-file diff and pinned helper/caller code, shared message and chart callers, Director request path, UserService state filtering and test additions.
- Go overlay maps the existing checkout's common.go to the exact PR source, plus one virtual test file. No product files were written.
- Seven targeted subtests: three positive controls pass; four reproductions fail across the two findings. See saved evidence.
- Base overlay for explicit-active plus inactive-dynamic fixture passes, establishing the inactive-user regression.
- Base dynamic-only/activity fixture selects A/C instead of A; head adds B to produce A/B/C. C widening is pre-existing, B reintroduction is new.
- `go test -overlay=/tmp/convi-7874-review/overlay.json -skip '^TestReview34439$' -count=1 ./insights-server/internal/shared` passes existing shared tests. Overlay source is exact PR helper; dependency/package context is local main, not a full PR checkout. Full analytics suite not run locally.
- Current GitHub checks: Insights build/lint and all five Bazel shards successful; PR open, reviewDecision APPROVED, Codeowner pending and mergeStateStatus BLOCKED. This does not resolve the review findings.
- HTML/JS controls checked programmatically and in IAB across all four steps and both views; Previous/Next checked programmatically. Desktop screenshot and narrow 390px screenshot inspected; viewport reset. Source links are pinned; local evidence paths resolve. No customer or deployment validation performed.
- Read automated review comments; the flag cleanup issue is already fixed with t.Cleanup at this head. Did not repeat it as a finding.
- GitHub reads used gh/git with existing authentication. No AWS/Okta/Azure/SSH credential was selected or inspected.

## Artifacts

- [Interactive review](../../deliverables/convi-7874-semantic-review.html)
- [Probe source](../../deliverables/convi-7874-review-evidence/probe_test.go)
- [Head probe output](../../deliverables/convi-7874-review-evidence/head-probes.txt)
- [Base comparison](../../deliverables/convi-7874-review-evidence/base-probe.txt)
- [Reviewed diff](../../deliverables/convi-7874-review-evidence/pr.diff)

No product edits, commits, pushes or GitHub comments. Findings and fixes remain for the author to address; staging validation remains necessary.
