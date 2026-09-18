# CONVI-7711 Sunbit Performance Insights no data

- Date: 2026-09-18
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Branch/worktree context: existing `main` checkout, read-only investigation; related Director source will be inspected without checkout changes.
- Domain/subdomain: analytics/performance-insights
- Request: investigate CONVI-7711 and compare with CONVI-7674; verify PostgreSQL and ClickHouse data first.
- Ticket: https://linear.app/cresta/issue/CONVI-7711/sunbit-performance-insights-returns-no-data-for-one-user-despite#comment-306c5ae9
- Initial hypothesis: process scorecards may be removed by conversation-ended-time filtering, as in CONVI-7674. Not yet established for Sunbit.

## Investigation

Evidence gathering in progress. No product changes or external messages authorized.
