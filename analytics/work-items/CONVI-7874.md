# CONVI-7874: Dynamic group filter in Closed Conversations

## Current review — 2026-10-26

Request changes at PR head `db800e7d2f64fe2c0763bb4557dc120c60a5db50`: two P2 findings reproduced with compiled helper probes. New root dynamic-member merge reintroduces inactive users after exclusion (base fixture passes, head fails). Early empty checks still discard valid split-field dynamic Groups beside an empty/inaccessible Team; that second issue is incomplete support rather than a new regression. Three positive controls and existing shared tests pass; four targeted cases fail across the two findings. CI green, Codeowner pending; no customer/deployment verification or product changes.

- [Interactive semantic review](../deliverables/convi-7874-semantic-review.html)
- [Session and validation details](../sessions/2026-10-26/codex-convi-7874-semantic-review.md)

Next: combine all selected audiences before active filtering and empty decisions, add the four regressions, and complete staging validation. No GitHub comments posted.

Primary domain: analytics. Primary subdomain: insights-user-filter.
Primary source repo: /Users/xuanyu.wang/repos/go-servers.
Prior comparison (2026-10-23): Compared open PR #34439 at `9ed3337b252fe698dcba4d311aaeb6804e3db191` with merged Oportun fix #33026. Same member-user ACL principle, separate backend path; additional handling for root `dynamic_groups`, empty audiences and mixed team/dynamic selection. No implementation or customer data changes.
Official Linear status: not checked.

- [PR #34439](https://github.com/cresta/go-servers/pull/34439)
- [Earlier fix #33026](https://github.com/cresta/go-servers/pull/33026)
- [Detailed comparison](../sessions/2026-10-23/codex-convi-7874-fix-comparison.md)
- [Interactive comparison](../deliverables/convi-7874-fix-comparison.html)

Next: staging validation and PR review remain with the author; no deployment outcome verified here.
