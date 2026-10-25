# CONVI-7874: Dynamic group filter in Closed Conversations

Primary domain: analytics. Primary subdomain: insights-user-filter.
Primary source repo: /Users/xuanyu.wang/repos/go-servers.
Technical state (2026-10-23): Compared open PR #34439 at `9ed3337b252fe698dcba4d311aaeb6804e3db191` with merged Oportun fix #33026. Same member-user ACL principle, separate backend path; additional handling for root `dynamic_groups`, empty audiences and mixed team/dynamic selection. No implementation or customer data changes.
Official Linear status: not checked.

- [PR #34439](https://github.com/cresta/go-servers/pull/34439)
- [Earlier fix #33026](https://github.com/cresta/go-servers/pull/33026)
- [Detailed comparison](../sessions/2026-10-23/codex-convi-7874-fix-comparison.md)
- [Interactive comparison](../deliverables/convi-7874-fix-comparison.html)

Next: staging validation and PR review remain with the author; no deployment outcome verified here.
