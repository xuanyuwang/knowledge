# Session Note - 2026-08-17 - Codex - Statistics API design review

**Started:** 2026-08-17
**Tool:** Codex
**Project:** `training-simulator`
**Goal:** Read engineering-design comments, examine analytics API reuse patterns, and propose updates to the Training Simulator statistics API decision.

## Source Context

- **Primary durable repo:** `/Users/xuanyu.wang/repos/knowledge` (main checkout; no knowledge worktree)
- **Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`
- **Related repos:** `/Users/xuanyu.wang/repos/cresta-proto`, `/Users/xuanyu.wang/repos/director`
- **Tracked refs inspected:** `go-servers/origin/main` `9099ace9140a472d617f03da955f7a7fdebec137`; `cresta-proto/origin/main` `d175ba9c8a1388b8d4c2b86fbc7625018f49fff7`; `director/origin/main` `18370ed8fd7da72d706ab22f0f37e966bb9ba4ca`
- **Product code changes:** none
- **Credentials/session used:** existing signed-in Superhuman Docs desktop session for read-only comment inspection; no credential files were inspected and no Git network operation was run.

## Review Comments Read

Jack Jee left four comments on the published engineering design:

1. The task-run legacy fields should now be removed rather than treated as a migration fallback.
2. Consider extending the existing stats API so reporting tabs do not need separate stats calls, avoiding tab-loading delay and duplicate fetching.
3. Consider returning stats from `ListTrainingModules` and `ListTrainingLessons`.
4. Add proto examples for the proposed API changes.

The API comment was anchored to “Add two batch read RPCs.” The list-API alternative was anchored to the first-surface discussion. No comments were replied to, resolved, or modified during this session.

## Analytics Reuse Evidence

The analytics service demonstrates both the leverage and the cost of broad API reuse.

Contract growth:

- `analytics_service.proto` is 4,856 lines and `common.proto` is 1,636 lines at the inspected ref.
- Shared `Attribute` filter/grouping semantics have field IDs through 83 and include deprecated/reserved fields.
- `AttributeType` supports 24 values plus reserved legacy IDs.
- `RetrieveConversationStatsRequest` carries general filter/group/frequency fields plus page-specific or migration fields such as `include_peer_user_stats`, selectable conversation time field, stale-metadata matching, and agent-only filtering.
- `RetrieveAgentStatsRequest` retains a customer parent plus separate `profile_id` and an explicit TODO to merge them.
- The older combined `RetrieveAssistanceStats` is documented as being replaced by separate stats APIs; Director contains an `enableSplitAssistanceStats` migration path.

Backend complexity:

- `RetrieveConversationStats` spans a 276-line orchestration/aggregation file plus a 611-line ClickHouse path.
- `RetrieveAgentStats` is 330 lines with an 809-line test file.
- Both branch on agent/group combinations, rewrite group-by requests into per-agent requests, and reconstruct group aggregates in Go.
- `EvaluateAnalyticsChart` is 1,682 lines and routes many chart shapes/data paths behind one endpoint.

Frontend request behavior:

- `useInsightsRequestParams` feeds at least ten analytics APIs, which is useful reuse but spreads shared request semantics widely.
- The same `RetrieveConversationStats` endpoint is called across Coaching Report, QA Insights, agent/team leaderboards, Assistance Insights, and metadata tooling.
- Reuse does not imply one request: Assistance views call `RetrieveConversationStats` for user/team and with/without Agent Assist variants; some components issue delta and all-cohort calls as well.
- `useEvaluateAnalyticsChart` can issue the original, ungrouped-total, primary-grouped, and groups-as-filters variants for one logical chart.
- The analytics domain map records Leaderboard as combining 8+ APIs because the metrics have different source/time/denominator semantics.

Lesson: share stable filter/value objects and internal query machinery where semantics match. Do not use endpoint consolidation as the primary loading optimization; consumers still need multiple requests when cohort or grain differs, while the broad endpoint accumulates flags and branching.

## Training Simulator Options

### Option 1: Reuse `ListTrainingSimulatorTaskRuns`

Possible additive shape:

```protobuf
message ListTrainingSimulatorTaskRunsRequest {
  // Existing attempt filters...
  TrainingSimulatorStatsFilter stats_filter = 7;
  repeated TrainingSimulatorStatsGrain include_stats = 8;
}

message ListTrainingSimulatorTaskRunsResponse {
  repeated TrainingSimulatorTaskRun training_simulator_task_runs = 1;
  repeated TrainingSimulatorLessonStats lesson_stats = 2;
  repeated TrainingSimulatorModuleStats module_stats = 3;
}
```

Advantages:

- one reporting request could return session, lesson, and module projections;
- existing attempt filters and subtype loading can be reused internally.

Problems:

- the resource/action says “list attempts,” while the new purpose is assignment-rooted aggregation;
- raw runs omit never-started assignments, so the handler must also load DirectorTasks and snapshots;
- the endpoint has a hard 1,000-run cap and no page-token response;
- adding date/task-lifecycle/cohort/group/direct-team/aggregation fields creates the same generic-filter growth seen in analytics;
- returning raw runs plus multiple aggregates increases payload and failure coupling;
- existing authorization includes `AGENT`, which is inappropriate for cohort-level content statistics;
- a shared endpoint does not eliminate content-list calls and may still need multiple requests when filters differ.

Assessment: reject. If an existing endpoint must be extended, `RetrieveTrainingSimulatorTaskStats` is semantically closer than `ListTrainingSimulatorTaskRuns`, but broadening task-grain output into all content grains still creates the same boundary problem.

### Option 2: Enrich lesson/module list APIs

Possible additive shape:

```protobuf
message TrainingSimulatorStatsFilter {
  TimestampRange time_range = 1;
  repeated string user_names = 2;
  repeated string group_names = 3;
  bool direct_team_only = 4;
}

message ListTrainingLessonsRequest {
  // Existing catalog fields...
  optional TrainingSimulatorStatsFilter stats_filter = 8;
}

message ListTrainingLessonsResponse {
  repeated TrainingLesson training_lessons = 1;
  string next_page_token = 2;
  repeated TrainingSimulatorLessonStats training_lesson_stats = 3;
}
```

The module list would mirror this shape.

Advantages:

- one request per active tab returns the current content page and matching stats;
- response naturally correlates table content with table metrics;
- fewer frontend hooks and loading states for the initial Lesson Configuration surface.

Problems:

- current catalog pagination/filter/cache semantics become coupled to historical cohort reporting;
- stats failures or heavy queries can block authoring/catalog use;
- both list APIs are used outside reporting, including agent-accessible paths, and currently authorize `AGENT`;
- per-field authorization does not exist, so stats require a separate permission boundary or an awkward role-dependent response;
- `ListTrainingLessons` can call `ListTrainingModules` for hydration, making stats inclusion and nested-loading behavior ambiguous;
- adding cohort/date fields to content lists begins the same gradual generalization seen in analytics;
- archived assignment history and current catalog lifecycle are different grains and may not align page-for-page;
- later manager reporting may not list content the same way as admin authoring.

Assessment: viable only if the product commits to stats as intrinsic catalog metadata with identical authorization, lifecycle, pagination, cache, and failure semantics. Current requirements do not meet that test.

### Option 3: Keep two dedicated stats RPCs

Recommended refinement:

```protobuf
message TrainingSimulatorStatsFilter {
  cresta.v1.common.time.TimestampRange time_range = 1;
  repeated string user_names = 2;
  repeated string group_names = 3;
  bool direct_team_only = 4;
}

message RetrieveTrainingSimulatorLessonStatsRequest {
  string parent = 1;
  repeated string training_lesson_names = 2;
  TrainingSimulatorStatsFilter filter = 3;
}

message RetrieveTrainingSimulatorModuleStatsRequest {
  string parent = 1;
  repeated string training_module_names = 2;
  TrainingSimulatorStatsFilter filter = 3;
}
```

Both responses reuse the shared outcome/warning messages already proposed but preserve explicit lesson and module result shapes.

Advantages:

- API name, request identity, output grain, authorization, SLO, caching, and errors remain readable;
- catalog and reporting can evolve independently;
- zero-run assignment and archived-history semantics are natural rather than exceptions to list behavior;
- backend can still share one loader, normalized attempt model, filter parser, and aggregation library;
- a manager-visible reporting surface can use report-specific authorization without exposing authoring/list APIs.

Costs:

- one additional batch stats request per active tab;
- inactive-tab transition can show loading if data was not prefetched;
- duplicated request/filter wrapper code if shared proto/FE builders are not extracted.

Latency mitigation:

1. Keep one stats request per visible page, never per row.
2. Start list and stats requests in parallel when content names are already cached.
3. Background-prefetch the inactive tab after its content-name list resolves, or prefetch on tab intent/hover.
4. Use stable React Query keys so switching tabs reuses cached results.
5. Share the normalized cohort builder and stats filter message across both RPCs.
6. Measure cold and cached tab-switch latency before adding transport-level consolidation.

Assessment: recommend. It addresses the reviewer's latency concern operationally while preserving semantic boundaries and avoiding the analytics complexity trajectory.

## Proposed Engineering-Design Updates

Do not edit the published design until the owner confirms the recommendation. Proposed patch:

1. Replace the unconditional “Add two batch read RPCs” statement with an **API alternatives and decision** section containing the three options above.
2. Add a short **Lesson from analytics APIs** subsection: reuse improved shared behavior but accumulated broad filters, page-specific flags, deprecated fields, handler branching, and still did not remove multi-request UI patterns.
3. Mark option 3 recommended and state the boundary rule: share narrow value objects and backend machinery, not a universal endpoint.
4. Add the concrete option-3 proto request/response examples plus shared filter and outcome messages.
5. Add a **Loading and prefetch contract** to the frontend section and a latency test for cold/cached tab transitions.
6. Correct the storage/read-model text to remove pre-migration task-run fallback fields, after validating against the tracked `go-servers/origin/main` schema and handler.
7. Update review notes with the API-boundary decision and measurable latency acceptance criteria.

## Follow-up

1. Review the three-option analysis with Jack and confirm the selected API boundary.
2. If option 3 is accepted, update the local backend/frontend plans and the Superhuman design, then reply to rather than prematurely resolve the comments.
3. Define a cold/cached tab-switch target and whether inactive-tab background prefetch is acceptable.
4. Revalidate source refs before implementation because local checkouts are behind their tracking refs.

## Publication Checkpoint

The API recommendation was accepted. The canonical backend and frontend plans now use option 3 and include the shared filter, complete lesson/module request and response messages, RPC declarations, HTTP mappings, cache/prefetch behavior, and cold/cached latency checks.

The published Superhuman engineering design was updated with:

- the three API alternatives and the selected dedicated-RPC boundary;
- the analytics-service lesson about accumulated flags and handler branching;
- the batch/cache/prefetch latency mitigation;
- compact proto examples for the shared filter, lesson/module requests and responses, and both RPC declarations.

The final published section was re-read through the desktop app. The original design sections and final review-gate bullet remain intact, and no malformed draft fragments remain.
