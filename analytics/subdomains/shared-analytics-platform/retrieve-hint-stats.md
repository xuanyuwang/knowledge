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

### Annotation concepts

The configured objects and their runtime annotations are different:

- An `Action` is a configured definition such as a Hint. Its stable template identifier becomes `action_id`.
- An `ActionAnnotation` is one runtime prediction in a conversation/message that results in an action. For Behavioral Hints, one `action_annotation_id` identifies one concrete hint firing or delivery.
- A `Moment` is a configured detector or conversation-state definition. Its stable template identifier becomes `moment_template_id`.
- A `MomentAnnotation` is one runtime observation of a moment in a conversation/message. Its occurrence identifier is `moment_annotation_id`.

An action annotation is not necessarily something the agent performed. In the Behavioral Hint flow, it is normally the system-side assistance action shown to the agent. A moment annotation is evidence about what happened in the conversation, such as detecting the encouraged behavior.

### Independence and linkage

`action_annotation_d` and `moment_annotation_d` are independent ClickHouse tables. A moment annotation does not contain all action annotations, and neither table is a child table embedded inside the other.

The general relationship is optional:

- many moment annotations have no action relationship;
- an action annotation can have no related adherence moments;
- an adherence outcome moment has one singular `adherence_action_annotation_id`;
- multiple positive or negative outcome moments can reference the same action annotation.

For adherence moments, `moment_annotation_d` denormalizes selected information about the linked action:

- `adherence_action_annotation_id`: runtime action occurrence; logically references `action_annotation_d.action_annotation_id`;
- `adherence_action_id`: configured action/template identifier; corresponds to `action_annotation_d.action_id`;
- `adherence_action_policy_id`, `adherence_action_policy_source_type`, `adherence_action_type`, `adherence_action_detailed_type`, and `adherence_action_adherence_type`: copied action dimensions used by analytics;
- `adherence_moment_annotation_id` and related `adherence_moment_*` fields: the SDX moment associated with a DDX/DNX adherence result.

The service proto defines `MomentAnnotationMetadata.adherence_action_annotation` as the SDX action of a DDX and requires it to be in the same conversation. Conversion code parses that resource name and stores only its action annotation ID in PostgreSQL. During ClickHouse projection, `buildMomentAnnotationRows` looks up that action annotation and copies its configured action ID, type, adherence type, policy, and other dimensions into the `adherence_action_*` columns. ClickHouse does not declare a foreign-key constraint, so this is an application-level relationship rather than a database-enforced one.

The identifier hierarchy is therefore:

```text
configured Action (action_id)
  -> runtime ActionAnnotation (action_annotation_id)
       <- adherence_action_annotation_id on zero or more MomentAnnotation rows

configured Moment (moment_template_id)
  -> runtime MomentAnnotation (moment_annotation_id)
```

For Behavioral Hint adherence:

```text
hint action annotation A1
  <- positive DDX moment M1
  <- positive DDX moment M2
  <- positive DDX moment M3
```

This means one hint was sent and one hint was followed, even though three positive moments were observed.

### Physical ClickHouse schema

The local `action_annotation` table is a `ReplicatedReplacingMergeTree` keyed for analytics by conversation hour, agent, and policy. `action_annotation_d` is the distributed table used by queries. Relevant columns include:

- occurrence and template identity: `action_annotation_id`, `action_id`;
- classification: `action_type`, `action_detailed_type`, `adherence_type`;
- dimensions: `agent_user_id`, `conversation_id`, `message_id`, `policy_id`, `policy_source_type`;
- timestamps and lifecycle: `conversation_start_time`, `create_time`, `update_time`;
- action-specific payload columns.

The local `moment_annotation` table is a separate `ReplicatedReplacingMergeTree`; `moment_annotation_d` is its distributed query surface. Relevant columns include:

- occurrence and template identity: `moment_annotation_id`, `moment_template_id`;
- classification: `moment_type`, `moment_detailed_type`, `adherence_type`;
- dimensions: `agent_user_id`, `behavior_id`, `conversation_id`, `message_id`, `policy_id`, `policy_source_type`;
- adherence linkage: all `adherence_action_*` and `adherence_moment_*` columns;
- timestamps, labels, metadata values, and moment payload.

The schemas define ordering and replacement behavior but no foreign key from `moment_annotation_d.adherence_action_annotation_id` to `action_annotation_d.action_annotation_id`.

### Query structure

`hintStatsClickhouseQuery` creates the SQL in these stages:

1. Parse the request into table-specific filters and group expressions for `moment_annotation_d`, `action_annotation_d`, and `conversation_event_d`. Supported final keys are agent, behavior, policy, and truncated time.
2. Build `hint_sent` as a `UNION ALL`:
   - default Behavioral path: read DDX/DNX rows from `moment_annotation_d` and count distinct non-empty `adherence_action_annotation_id`;
   - action-annotation Behavioral path: read `action_annotation_d`, restricted to action IDs referenced by DDX/DNX moment rows, and count distinct `action_annotation_id`;
   - non-Behavioral path: read action annotations for regular hints, manager alerts, GW hints, and KB hints with unspecified adherence.
3. Build `combined_hint_sent` per grouping key. The implementation uses `MAX`, not `SUM`, across the two sent branches. This assumes at most one branch contributes a meaningful count for each request/group. If Behavioral and non-Behavioral branches both contribute to the same group, `MAX` undercounts their union.
4. Build `hint_followed` as a `UNION ALL`:
   - Behavioral/checklist branch: read positive DDX rows (`adherence_type = 2`) from `moment_annotation_d`;
   - KB/GW branch: read followed events (`event_type = 9`) from `conversation_event_d`.
5. Left-join followed groups to sent groups using the requested grouping keys. The final select keeps sent/conversation/user counts with `MAX` and combines followed branches with `SUM`. Consequently, an unfiltered mixed-category request can sum Behavioral and KB/GW followed counts while retaining only the larger sent branch.

### CONVI-7387 behavior change

Before the fix, the Behavioral followed branch used:

```sql
COUNT(DISTINCT moment_annotation_id) AS hint_followed_count
```

That answered “how many positive adherence observations occurred?” It did not answer “how many sent hints were followed?”

PR #30782 changes it to:

```sql
COUNT(DISTINCT adherence_action_annotation_id) AS hint_followed_count
```

with `adherence_action_annotation_id <> ''`. All positive moments linked to the same hint action now contribute one followed hint.

For the default sent path, the intended subset relationship is explicit:

- sent IDs: distinct action annotation IDs appearing in DDX or DNX rows;
- followed IDs: distinct action annotation IDs appearing in DDX rows;
- therefore followed IDs are a subset of sent IDs under the same filters and grouping dimensions.

For the action-annotation sent path, the same relationship is weaker:

- sent applies `maCondition` inside the moment subquery and `aaCondition` to the outer action row;
- followed applies `maCondition` only because it always reads `moment_annotation_d`;
- each followed link must also resolve to an `action_annotation_d` row whose filter/group fields agree.

ClickHouse has no foreign key enforcing the link, and the two paths do not apply identical table predicates. The fix resolves the observed multi-moment fan-out, but action-filter asymmetry, orphaned links, or inconsistent denormalized dimensions could still violate the percentage invariant.

For a request explicitly filtered to Behavioral Hints, the non-Behavioral sent branch is made empty by the conflicting adherence filter, so the `MAX` mixed-branch risk does not explain CONVI-7387. It remains a separate risk for unfiltered or mixed-category requests.

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

### Deployed behavioral query before CONVI-7387

- Sent: `COUNT(DISTINCT adherence_action_annotation_id)`
- Followed: `COUNT(DISTINCT moment_annotation_id)`

These are different entities:

- an action annotation represents a hint fire;
- a moment annotation represents an observed adherence moment;
- one hint action can link to several positive moment annotations.

Therefore the deployed API can return `total_hint_followed_count > total_hint_sent_count`, and Director can display engagement above 100%.

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

Fix status:

- [go-servers #30782](https://github.com/cresta/go-servers/pull/30782) changes the behavioral numerator to `COUNT(DISTINCT adherence_action_annotation_id)`.
- The query now requires a non-empty action annotation ID and covers both behavioral sent-query implementations.
- The targeted `RetrieveHintStats` Bazel test passes. Production verification remains pending deployment.

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

Before CONVI-7387, tests validated generated SQL shapes and response conversion but did not establish the behavioral counting grain. PR #30782 adds a focused query test for both sent-query implementations and updates all golden SQL fixtures.

Required regression coverage:

1. end-to-end fixture data with one behavioral hint action linked to multiple positive moment annotations;
2. `followed <= sent` for percentage-producing hint types;
3. agent + policy aggregation without frequency;
4. group + policy aggregation from per-agent rows;
5. archived/unresolved policy removal recomputes response totals correctly.
6. a mixed Behavioral + non-Behavioral request where both sent branches contribute to one group;
7. the action-annotation sent path with action-table predicates that do not match denormalized moment predicates.

## Source References

- ClickHouse tables: `clickhouse-schema/conversations/migrations/20230824160348_init_db.up.sql`
- Service concepts: `cresta-proto/cresta/v1/action/action.proto`, `cresta-proto/cresta/v1/action/action_annotation.proto`, `cresta-proto/cresta/v1/moment/moment.proto`, `cresta-proto/cresta/v1/moment/moment_annotation.proto`
- PostgreSQL source tables: `go-servers/apiserver/sql-schema/app/app-schema.sql`
- ClickHouse projection and adherence denormalization: `go-servers/shared/clickhouse/conversations/conversation.go`
- API query: `go-servers/insights-server/internal/analyticsimpl/retrieve_hint_stats_clickhouse.go`
- Filter and grouping mappings: `go-servers/insights-server/internal/analyticsimpl/common_clickhouse.go`

## Troubleshooting Checklist

1. Confirm the exact Director surface and hint type.
2. Capture date range, frequency, user/group filters, policy/behavior filters, and agent-only settings.
3. Distinguish response totals from grouped result totals.
4. Reproduce the generated SQL grouping, not just a broad table count.
5. Compare sent-action, followed-moment, and followed-action grains.
6. Check the action-annotation feature flag path.
7. For group results, inspect current group membership and overlap.
8. Do not compare directly with `RetrieveQAScoreStats` without aligning metric definition and population.
