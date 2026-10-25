# CONVI-7777 issue register and production join verification

**Date:** 2026-10-19 (America/Toronto)
**Primary domain/subdomain:** analytics / assistance-insights
**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree context:** main checkout; read-only source inspection. Feature evidence is pinned to the revisions linked in the register, rather than assumed to match main.
**Canonical work item:** [CONVI-7777](../../work-items/CONVI-7777.md).

## Outcome

Created a [PM-facing register](../../deliverables/convi-7777-assistance-analytics-issue-register.md) with 15 consolidated entries. It covers the five feature reports' 19 preserved-quirk observations, deduplicated into 13 entries, plus two population/metric-contract entries. Each entry distinguishes its trigger, observed behavior, consequence, evidence, and proposed decision/follow-up. The user requested placement under the existing CONVI-7777 project; moved the register and [read-only reproduction SQL](../../deliverables/convi-7777-production-join-check.sql) into `analytics/deliverables/`, preserving file contents and removing the original artifact copies.

## Production evidence

User selected `brinks-sbx` on `us-west-2-prod`. Latest config resolved `configv3/prod/brinks-sbx/us-west-2/config.yaml`, config revision `7d17e6a77e559db02794c0ea0d9520b0b9f03367`, database `brinks_sbx_us_west_2`, endpoint `clickhouse-conversations.us-west-2-prod.internal.cresta.ai`. Individually reviewed AWS SSO profile `us-west-2-prod_dev`; SSO login succeeded. The connection skill used the configured ClickHouse admin credential without printing or persisting its password.

Live settings returned `join_use_nulls=0`, `changed=0`, user `admin`, version `26.3.12.3`, on replica `chi-conversations-conversations-1-0-0`. A separate synthetic-data query on `chi-conversations-conversations-0-2-0` also saw 0: joined rows were `(A,A,0)`, `(B,'',0)`, `(C,'',0)` for `(conversation, excluded key, isNull)`. The legacy predicate kept 0 conversations/0 searches; the intended unmatched rows B/C represented 2 conversations/4 searches. No customer conversation rows were read by this reproduction.

The coordinator query log had one completed KB-shaped SELECT in the preceding 24 hours as admin without a join-setting override. Empty client name prevents conclusive insights-server attribution. Inspected deployment configuration uses the admin-password secret without a username override, and the source connection builder supplies no join setting. This verifies the failure on observed production connections, not exhaustive replica coverage, end-to-end RPC behavior, deployment prevalence, or customer impact.

## Interpretation and next actions

The exclusion bug predates extraction and is intentionally preserved in the refactor. Feature evidence and the new conversation population are separate contracts: membership can remove otherwise qualifying events. Agent Assist ON has conversation-row source/developer exceptions and label-only pairs while still requiring matching eligible message rows.

Review the register with PM, confirm unresolved contracts, create scoped follow-up tickets/owners, and measure affected customer requests before prioritizing rollout. No PM message, official ticket mutation, product-code change, commit, or push was performed.

## Additional report relocation

Moved 120 files from the feature-review visualization folder into the canonical Analytics project: six HTML reports/index pages and their sibling JS/JSON assets under `deliverables/`, and the complete `implementation/` evidence under `sessions/2026-10-19/convi-7777-feature-refactor-evidence/`. Updated HTML navigation and old artifact paths in documents/report JS while preserving historical raw logs. [Report index](../../deliverables/convi-7777-feature-refactors.html). The [relocation manifest](convi-7777-report-relocation.json) records old/new paths and SHA256 values. This moves existing evidence without rerunning tests or asserting current PR status.
