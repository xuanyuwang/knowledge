# Session: how to enable `useConversationOutcomeMomentAnnotationMaterializedView`

**Started:** 2026-08-07
**Tool:** Cursor
**Project:** `convi-7049-clo-filter`
**Goal:** Document how to enable the Insights CLO MV config flag on a customer, including prerequisites and current rollout status.

## Source Context

- **Primary repo:** `config` (enablement) + `go-servers` (consumer)
- **Repo paths:** `/Users/xuanyu.wang/repos/config`, `/Users/xuanyu.wang/repos/go-servers`
- **Worktree path:** `/Users/xuanyu.wang/repos/go-servers-convi-7383` (consumer reference)
- **Branch:** investigation against `config` `origin/master`, `go-servers` `origin/main`
- **Tickets:** [CONVI-7383](https://linear.app/cresta/issue/CONVI-7383), [CONVI-7424](https://linear.app/cresta/issue/CONVI-7424)

## Findings

### What the flag does

Per-profile Insights bool. When true, QA ClickHouse CLO filters (`CONVERSATION_OUTCOME` / moment type 14) read `moment_annotation_by_conversation_outcome_d` with typed `outcome_*` columns instead of `moment_annotation_d` + `JSONExtract*`.

Consumed in insights-server (merged [go-servers#30588](https://github.com/cresta/go-servers/pull/30588) on 2026-08-07):

- `retrieve_qa_score_stats_clickhouse.go`
- `retrieve_qa_conversations_clickhouse.go`

via:

```go
profileConfig.GetFeatures().GetInsights().GetUseConversationOutcomeMomentAnnotationMaterializedView()
```

Fail-open: `GetProfileConfig` errors leave the flag false. Conversation-end-time filters force raw-table fallback even when the flag is on (CLO/metadata MVs lack `conversation_end_time`).

### Proto / schema

- Proto field 30: `use_conversation_outcome_moment_annotation_materialized_view` ([cresta-proto#9400](https://github.com/cresta/cresta-proto/pull/9400), merged 2026-07-29)
- JSON / configv3 key: `useConversationOutcomeMomentAnnotationMaterializedView`
- Present on `config` `origin/master` under `json-schema/configv3/Insights.json` (and nested in Profile/Customer/Features schemas). Schema sync commit `f1179bdad7` from cresta-proto `386f8c1a6f`.
- Description: "When true, query conversation-outcome moment annotations from the conversation-outcome materialized view instead of moment_annotation_d. Unset/false keeps the existing moment_annotation_d path."
- Proto3 / unset default is `false`. No customer YAML currently sets it (`git grep` on `origin/master` `customer_config_history/**` → zero hits).

### How to enable on a customer

YAML path (profile scope; same key under usecase `features.insights` if applying at usecase level):

```yaml
features:
  insights:
    useConversationOutcomeMomentAnnotationMaterializedView: true
```

**Preferred path (GitOps via Cresta Admin):**

1. Open Tenant Admin / Cresta Admin **Feature Config** flags (not Director Feature Flags). Route pattern: `/:region/config/feature-flags/...` (`FeatureConfigRoute` / `FeatureConfigFlags`).
2. Filter/search for Insights → `insights.useConversationOutcomeMomentAnnotationMaterializedView`.
3. Select the target customer profile(s). Use application level **all** / Auto Inheritance (or profile-only) as appropriate.
4. Enable → ConfigService creates a `customer_config_history` PR for review.
5. Review that only this Insights key changed for the intended customer/profile/usecases; merge to `master` so multi-region sync applies.

**Manual path:** edit `config/customer_config_history/{staging|prod}/customers/<customer>.yaml` under the relevant profile (and usecases if needed), open a config PR, wait for YAML/schema + diff-only ConfigService checks, merge.

This is a **Tier-2 per-customer config flag** (see `insights-server/docs/feature-flags.md`), not a cluster envflag and not a Director UI flag.

### Prerequisites before enabling (do not skip)

Per CONVI-7383 rollout plan / [CONVI-7424](https://linear.app/cresta/issue/CONVI-7424):

1. Phase A DDL exists on that customer's ClickHouse DB: storage `moment_annotation_by_conversation_outcome`, trigger MV, distributed `moment_annotation_by_conversation_outcome_d`.
2. Phase B 180-day backfill completed and validated (row-count / spot-check vs `moment_annotation_d` where `moment_type=14`).
3. insights-server build including #30588 is deployed to the customer's cluster.
4. Flag remains **off** until those pass; enabling early routes queries to an empty/incomplete MV and undercounts CLO matches.

Staging pilot enablement is explicitly [CONVI-7424](https://linear.app/cresta/issue/CONVI-7424) (Backlog), blocked on Phase B ([CONVI-7423](https://linear.app/cresta/issue/CONVI-7423), In Progress).

### Current status (2026-08-07)

| Layer | Status |
|---|---|
| Proto flag | Merged |
| Config schema | On `origin/master` |
| go-servers consumer | Merged to `origin/main` (#30588) |
| ClickHouse Phase A | Partial (see work-item CONVI-7383) |
| ClickHouse Phase B backfill | Not ready for flag enable |
| Customer enablement | **None** — keep off |

## Follow-ups

1. Finish Phase A remaining targets + Phase B staging backfill before any config PR.
2. Pilot: enable for one staging customer via Feature Config bulk/single edit → validate QA latency/correctness (CONVI-7424).
3. Then selected large prod customers only after per-DB validation.

## Links

- [CONVI-7383](https://linear.app/cresta/issue/CONVI-7383/improve-clo-conversation-outcome-filter-query-performance-via)
- [CONVI-7424](https://linear.app/cresta/issue/CONVI-7424/rollout-validate-staging-clo-mv-enable-insights-flag-pilot)
- [cresta-proto#9400](https://github.com/cresta/cresta-proto/pull/9400)
- [go-servers#30588](https://github.com/cresta/go-servers/pull/30588)
- Prior: `sessions/2026-07-28/codex-clo-mv-config-flag.md`, `sessions/2026-07-29/cursor-clo-mv-flag-go-servers.md`
