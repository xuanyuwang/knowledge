# RetrieveHintStats

## Purpose

`RetrieveHintStats` is the canonical analytics API for counts and rates associated with hints sent to agents. Director uses it in Assistance Insights and Leaderboard for Behavioral, Reminder, Knowledge Base, Guided Workflow, and Checklist hint metrics.

It does **not** return QA scorecard results. A behavior criterion in Performance Insights normally comes from `RetrieveQAScoreStats`; a similarly named Behavioral Hint metric comes from `RetrieveHintStats`. Those surfaces can use different populations and should not be compared without first aligning their definitions and filters.

## Contract

**RPC**

- Proto: `cresta-proto/cresta/v1/analytics/analytics_service.proto`
- Request: `RetrieveHintStatsRequest`
- Response: `RetrieveHintStatsResponse`
- Metric messages: `cresta-proto/cresta/v1/analytics/hint_stats.proto`

**Request fields**

- `parent`: customer/profile resource.
- `filter_by_time_range`: half-open interval `[start, end)`.
- `filter_by_attribute`: users, groups, policies, behaviors, duration, moment metadata, hint type, and other shared analytics filters.
- `frequency`: optional time bucketing. Director commonly uses daily or weekly; it deliberately omits frequency for aggregate agent/policy leaderboards.
- `group_by_attribute_types`: supported ClickHouse keys are agent, behavior, policy, and time.
- `include_peer_user_stats`: used by Agent Leaderboard.
- `filter_to_agents_only`: restricts results to users with only the Agent role.

**Response levels**

1. `RetrieveHintStatsResponse` contains response-wide totals.
2. `hint_stats_results[]` contains one result per requested grouping key.
3. `hint_stats[]` contains time buckets when frequency is set.

Each level can expose:

- `total_hint_sent_count`
- `total_hint_followed_count`
- `total_conversation_count`
- sent/followed averages per conversation
- sent/followed averages per frequency per user

Response totals are sums of retained result groups. Grouping changes the breakdown shape but does not remove response-level aggregates.

The `average_*_per_frequency_per_user` fields are only meaningful when the request groups by agent; the backend derives their denominator from per-group user/time rows.

## Director Callers

### Shared hooks

- `director/packages/director-app/src/components/insights/hooks/useHintStats.ts`
  - Calls `CrestaAPI.insights.retrieveHintStats`.
- `director/packages/director-app/src/components/insights/hooks/useGetHintStatsByHintType.tsx`
  - Builds separate requests for Behavioral, Reminder, KB, Guided Workflow, and Checklist hints.
  - Adds `filterByAttribute.hintType`.
- `director/packages/director-app/src/components/insights/hooks/util-hooks/useInsightsRequestParams.ts`
  - Builds the common time range, filters, grouping, frequency, metadata, and agent-only fields.

### Assistance Insights request shapes

| Surface | Grouping | Frequency | Displayed meaning |
|---|---|---|---|
| Behavioral Hints card and chart | none | selected daily/weekly frequency | response-wide followed/sent rate and trend |
| Hint breakdown table | policy | selected frequency | per-policy sent, followed, and followed/sent |
| Agent hint leaderboard | agent + policy | unset | per-agent/per-policy and weighted agent rate |
| Team hint leaderboard | group + policy | unset | per-group/per-policy and weighted team rate |
| Policy detail heatmap | agent or group, filtered to one policy | selected frequency | per-user/team time series |

Important frontend formulas:

- Hint engagement is normally `totalHintFollowedCount / totalHintSentCount`.
- Reminder hints intentionally surface sent count rather than a followed rate.
- Most views assume the API ratio is in `[0, 1]`.
- Some breakdown utilities clamp the display to `1`; the carousel, chart, and agent/policy leaderboard do not consistently clamp.
- Clamping is only defensive presentation. It cannot repair an incorrect numerator or denominator.

### Leaderboard

Agent and Team Leaderboards also call `RetrieveHintStats`, usually grouped by agent/team and time, to populate engagement and hint-type columns. These paths may transform the ratio into a `0–100` value before rendering, while Assistance Insights generally retains a `0–1` ratio until formatting.

## Backend Flow

Primary implementation:

- Handler: `go-servers/insights-server/internal/analyticsimpl/retrieve_hint_stats.go`
- Query builder and response conversion: `go-servers/insights-server/internal/analyticsimpl/retrieve_hint_stats_clickhouse.go`
- Shared filters/grouping: `go-servers/insights-server/internal/analyticsimpl/common_clickhouse.go`

Execution flow:

1. Initialize an empty attribute filter when absent and validate grouping/frequency.
2. Resolve the customer/profile.
3. Resolve user and group filters through `ParseUserFilterForAnalytics`.
4. Apply cache lookup using a normalized request key.
5. For group-based requests, query per-agent rows and aggregate them into groups in Go.
6. Otherwise, query ClickHouse directly.
7. Resolve policy/action metadata from PostgreSQL, discard archived or unresolved policies when a hint type is specified, and return sorted results.

### Group aggregation

ClickHouse does not directly group by current application groups. For group requests, the service:

1. converts group grouping to agent grouping;
2. queries per-agent data;
3. attaches current group membership or sums per-agent rows into group rows.

A user belonging to multiple requested groups contributes to each group. Group totals are therefore not mutually exclusive unless the selected groups are mutually exclusive.

## ClickHouse Data Sources

| Table | Role |
|---|---|
| `moment_annotation_d` | Behavioral/checklist hint linkage and positive/negative adherence moments |
| `action_annotation_d` | Hint actions sent, including non-behavioral hint types |
| `conversation_event_d` | KB/GW followed events |

Time filtering uses `conversation_start_time`, not scorecard submission time or `scorecard_time`.

The query is generated as three stages:

1. `hint_sent`: union behavioral and non-behavioral sent counts.
2. `combined_hint_sent`: collapse the sent branches with `MAX` per grouping key.
3. `hint_followed`: union behavioral/checklist followed moments with KB/GW followed action events, then join to sent rows and `SUM` followed counts.

Golden SQL fixtures live under:

`go-servers/insights-server/internal/analyticsimpl/testdata/clickhouse_RetrieveHintStats_*.sql`

The most useful aggregate agent/policy fixture is:

`clickhouse_RetrieveHintStats_GroupByAgentGroupPolicy_NotGroupByTime_request.sql`

## Hint-Type Semantics

The `hintType` filter is translated to table-specific action and adherence columns in `common_clickhouse.go`.

| Hint type | Sent source | Followed source | Typical UI |
|---|---|---|---|
| Behavioral | `moment_annotation_d` by default; optional action-annotation path | `moment_annotation_d` positive adherence | Behavioral Hints engagement |
| Reminder/regular | `action_annotation_d` | not normally presented as engagement | Reminder hints sent |
| KB | `action_annotation_d` | `conversation_event_d` event type 9 | KB Hint engagement |
| Guided Workflow | `action_annotation_d` | `conversation_event_d` event type 9 | Guided Workflow Hint engagement |
| Checklist | action/moment linkage depending on data path | behavioral followed branch includes checklist moments | Checklist engagement |

Two controls select the sent-query implementation:

- process flag `USE_ACTION_ANNOTATION_FOR_HINT_STATS`
- profile config `allowHintStatsCalculationActionAnnotationBasedQuery`

The alternate path changes how behavioral sent hints are read. It does not, by itself, change the behavioral followed count grain.

## Counting Semantics and Invariants

### Current behavioral query

- Sent: `COUNT(DISTINCT adherence_action_annotation_id)`
- Followed: `COUNT(DISTINCT moment_annotation_id)`

These are different entities:

- an action annotation represents a hint fire;
- a moment annotation represents an observed adherence moment;
- one hint action can link to several positive moment annotations.

Therefore the current API can return `total_hint_followed_count > total_hint_sent_count`, and Director can display engagement above 100%.

### Required metric decision

The product label and tooltip describe a percentage of sent hints that were followed. Under that definition:

- denominator: distinct sent hint actions;
- numerator: distinct sent hint actions with at least one qualifying positive adherence;
- invariant: `0 <= followed <= sent`;
- rate invariant: `0 <= followed / sent <= 1`.

The aligned behavioral numerator would count distinct `adherence_action_annotation_id` under the positive-adherence predicate.

If product instead intends to count all positive moments, the value is a **moments-per-hint rate**, not a percentage. The API field and Director labels would need to change, and values above one would be legitimate.

## Known Defect: Mixed Behavioral Grains

The Heartland investigation demonstrated the current mismatch for the week `[2026-07-20, 2026-07-27)`:

- sent hint actions: 134;
- positive moment annotations returned as followed: 215;
- current Assistance rate: `215 / 134 = 160.4%`;
- distinct actions with at least one positive moment: 106;
- action-deduplicated hint rate: `106 / 134 = 79.1%`.

The source rows are valid. The invalid percentage is created by dividing valid counts of different entities.

Canonical investigation:

- `analytics/work-items/heartland-behavior-hints-adherence-mismatch.md`
- `analytics/sessions/2026-07-28/codex-heartland-behavior-hints-mismatch.md`

## Operational Validation

For an agent, policy, and half-open date range, compare the current API grains:

```sql
SELECT
  uniqExactIf(
    adherence_action_annotation_id,
    adherence_type IN (2, 3)
      AND adherence_action_annotation_id != ''
  ) AS sent_actions,
  uniqExactIf(
    moment_annotation_id,
    adherence_type = 2
      AND adherence_action_id != ''
  ) AS followed_moments,
  uniqExactIf(
    adherence_action_annotation_id,
    adherence_type = 2
      AND adherence_action_id != ''
  ) AS followed_actions,
  followed_moments / sent_actions AS current_api_rate,
  followed_actions / sent_actions AS action_deduplicated_rate
FROM moment_annotation_d
WHERE agent_user_id = '<user_id>'
  AND policy_id = '<policy_id>'
  AND adherence_action_policy_source_type = 1
  AND adherence_action_type = 1
  AND adherence_action_adherence_type = 1
  AND is_dev_user = 0
  AND conversation_source IN (0, 8)
  AND conversation_start_time >= '<start>'
  AND conversation_start_time < '<end>';
```

When `followed_moments > sent_actions`, inspect the fan-out:

```sql
SELECT
  adherence_action_annotation_id,
  uniqExactIf(
    moment_annotation_id,
    adherence_type = 2
      AND adherence_action_id != ''
  ) AS positive_moments
FROM moment_annotation_d
WHERE agent_user_id = '<user_id>'
  AND policy_id = '<policy_id>'
  AND adherence_action_annotation_id != ''
  AND conversation_start_time >= '<start>'
  AND conversation_start_time < '<end>'
GROUP BY adherence_action_annotation_id
ORDER BY positive_moments DESC;
```

## Testing Gaps

Existing tests validate generated SQL shapes and response conversion but do not establish the percentage invariant.

Required regression coverage:

1. one behavioral hint action linked to multiple positive moment annotations;
2. followed behavioral count deduplicated at the intended hint-action grain;
3. `followed <= sent` for percentage-producing hint types;
4. agent + policy aggregation without frequency;
5. group + policy aggregation from per-agent rows;
6. both default and action-annotation sent-query paths;
7. archived/unresolved policy removal recomputes response totals correctly.

## Troubleshooting Checklist

1. Confirm the exact Director surface and hint type.
2. Capture date range, frequency, user/group filters, policy/behavior filters, and agent-only settings.
3. Distinguish response totals from grouped result totals.
4. Reproduce the generated SQL grouping, not just a broad table count.
5. Compare sent-action, followed-moment, and followed-action grains.
6. Check the action-annotation feature flag path.
7. For group results, inspect current group membership and overlap.
8. Do not compare directly with `RetrieveQAScoreStats` without aligning metric definition and population.
