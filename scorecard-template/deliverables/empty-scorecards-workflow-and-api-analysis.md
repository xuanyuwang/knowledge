# Empty Scorecards: Workflow and API Analysis

**Created:** 2026-06-25  
**Status:** Investigation pass  
**Source repos:** `go-servers` at `eb72d7bb87a43052fffed65d3a9f518c889d8f7d`, `director` at `301610c1edef5708c0e1c71d4f3bd81e8aed6888`

## Summary

An **empty scorecard** is a `director.scorecards` row with no usable `director.scores` rows.

There are two practical variants:

- **No-score empty**: the scorecard has zero rows in `director.scores`.
- **Analytics-empty**: the scorecard has score rows in Postgres, but none survive score validation/projection into ClickHouse, so analytics behaves as if the scorecard is absent.

The current system mostly treats empty scorecards as persistence artifacts, not analytics facts. Postgres can store them. ClickHouse projection and QA analytics generally require at least one score row, so empty scorecards usually disappear from analytics APIs.

The most important workflow finding is that **AutoQA creates scorecards outside the coaching `CreateScorecard` API**. Live AutoQA and AutoQA backfill both call shared scoring persistence directly, so any invariant like "do not create empty scorecards" cannot be enforced only in the user-facing create API.

## Core Definition

In code terms, the persistence layer allows this shape:

- `CreateScorecardAndScoresInDB(...)` creates the scorecard row, then calls `CreateInBatches(scores, 50)` without checking `len(scores) > 0`.
- Tests explicitly call `CreateScorecardAndScoresInDB(..., make([]*dbmodel.Scores, 0), ...)` and expect success in several duplicate/scorecard-creation scenarios.
- `ComputeScores(...)` accepts an empty `[]*coaching.Score`, returns no score rows, leaves `scorecard.Score` unset, and sets `AutoFailed=false`.

Evidence:

- [CreateScorecardAndScoresInDB creates scorecard plus provided score batch](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/shared/scoring/scorecard_scores_dao.go#L29-L51)
- [DAO tests create scorecards with empty score slices](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/shared/scoring/scorecard_scores_dao_test.go#L208-L213)
- [ComputeScores skips missing criteria and computes no overall score for empty input](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/shared/scoring/scorecard_calculator.go#L20-L43)
- [TestComputeScoresWithoutScores documents empty-score behavior](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/shared/scoring/scorecard_calculator_test.go#L146-L160)

## Creation and Mutation APIs

### Coaching `CreateScorecard`

`CreateScorecard` dispatches by scorecard type:

- default evaluation
- appeal request
- appeal resolve
- group calibration answer key
- group calibration response

Each path converts API scores, flattens computed DB scores, and persists through `CreateScorecardAndScoresInDB`. I did not find a generic create-time guard requiring at least one score.

Evidence:

- [CreateScorecard type dispatch](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/apiserver/internal/coaching/action_create_scorecard.go#L43-L81)
- [Default scorecard creation computes and persists flattened scores](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/apiserver/internal/coaching/action_create_scorecard.go#L573-L648)
- [Group calibration answer key persists flattened scores](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/apiserver/internal/coaching/action_create_scorecard.go#L156-L201)
- [Group calibration response requires a task, not a non-empty score list](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/apiserver/internal/coaching/action_create_scorecard.go#L204-L341)
- [Appeal request creates a replica, then persists appeal scores](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/apiserver/internal/coaching/action_create_scorecard.go#L344-L459)
- [Appeal resolve persists resolve scores](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/apiserver/internal/coaching/action_create_scorecard.go#L462-L570)

Important converter behavior:

- `ConvertAPIScoresToDB` calls `RemoveEmptyScorecardScores` before `ComputeScores`.
- `RemoveEmptyScorecardScores` drops score entries where `not_applicable=false`, `numeric_value=nil`, `ai_value=nil`, empty text, and empty comment.
- Therefore, a UI/request payload containing placeholder blank score rows can become a true no-score empty scorecard before persistence.

Evidence:

- [ConvertAPIScoresToDB removes empty score rows before computing](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/apiserver/internal/coaching/scorecards/scorecards.go#L213-L254)
- [RemoveEmptyScorecardScores definition](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/apiserver/internal/coaching/scorecards/scorecards.go#L291-L303)

### Coaching `UpdateScorecard`

`UpdateScorecard` also removes empty score rows and computes scores from the request. However, the merge behavior is not a simple "delete all scores if the request is empty":

- if an existing score's criterion still exists and there is no new score for that criterion, `mergeScoresData` keeps the existing score;
- if the scorecard was created empty and still has no existing scores, it remains empty.

Evidence:

- [UpdateScorecard removes empty score rows before computing](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/apiserver/internal/coaching/action_update_scorecard.go#L371-L379)
- [mergeScoresData keeps existing valid criterion scores when no replacement exists](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/shared/scoring/scorecard_scores_dao.go#L176-L214)

### Coaching `SubmitScorecard`

`SubmitScorecard` loads the scorecard and template, checks permissions, and marks the scorecard submitted. I did not find a non-empty-score check in the submit path. For appeal resolve, submit can propagate resolve scores back to the original scorecard.

Evidence:

- [SubmitScorecard loads scorecard/template and checks permission](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/apiserver/internal/coaching/action_submit_scorecard.go#L98-L146)
- [Appeal resolve submit updates original scorecard scores](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/apiserver/internal/coaching/action_submit_scorecard.go#L197-L220)

### Director Save Path

Director sends `CreateScorecard` for a new scorecard and `UpdateScorecard` for autosave. It builds `scores` from form state and does not itself impose a universal non-empty-score invariant before calling the API. Appeal submission has a UI guard that blocks appeal submit when the appeal scorecard has no scores.

Evidence:

- [Director creates a new scorecard through `CrestaAPI.coaching.createScorecard`](https://github.com/cresta/director/blob/301610c1edef5708c0e1c71d4f3bd81e8aed6888/packages/director-app/src/components/scoring/hooks/useSaveScorecardMutation.ts#L146-L176)
- [Director updates existing scorecards through `updateScorecard`](https://github.com/cresta/director/blob/301610c1edef5708c0e1c71d4f3bd81e8aed6888/packages/director-app/src/components/scoring/hooks/useSaveScorecardMutation.ts#L192-L200)
- [Appeal submit is blocked when the appeal scorecard has no scores](https://github.com/cresta/director/blob/301610c1edef5708c0e1c71d4f3bd81e8aed6888/packages/director-app/src/components/scoring/scorecard-form/ScorecardForm.tsx#L815-L827)

## AutoQA Workflow

AutoQA is a distinct scorecard creation workflow, not just a variant of manual evaluation.

### Live AutoQA

Live AutoQA:

1. skips templates with zero AutoQA scorable items;
2. maps AutoQA output into a DB scorecard and API score list;
3. computes scores;
4. persists through `CreateScorecardAndScoresInDB`.

There is no explicit check after `MapToScores(...)` or `ComputeScores(...)` that `computedScores` is non-empty.

Evidence:

- [Live AutoQA skips templates with no configured scorable items](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/apiserver/internal/autoqa/action_trigger_conversation_autoscoring.go#L180-L189)
- [Live AutoQA maps, computes, flattens, and persists scorecards directly](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/apiserver/internal/autoqa/action_trigger_conversation_autoscoring.go#L222-L244)

### AutoQA Backfill

Backfill uses the same core shape:

1. process conversation/template;
2. compute scores;
3. create or update scorecard through shared scoring persistence.

There is no explicit non-empty-score guard in `processScorecard` before create/update.

Evidence:

- [Backfill processScorecard computes and persists create/update](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/temporal/ingestion/backfillscorecards/template_processor.go#L410-L439)

## Analytics and Projection APIs

### ClickHouse Projection

The analytics pipeline mostly requires score rows.

For conversation scorecards:

- `BuildScoreRowsFromDirectorScores` returns no rows when `directorScores` is empty.
- `BuildScorecardRows` returns no scorecard rows when there are no score rows and `isProcessScorecard=false`.
- In conversation indexing, scorecard rows are built from all score rows; empty conversation scorecards do not get scorecard rows.

For process scorecards:

- `BuildScorecardRows` now allows process scorecard metadata rows even without score rows.
- However, QA score stats still join scorecard rows to `scorecard_score`, so a scorecard-only CH row without score rows does not contribute to score averages/counts that are score-driven.

Evidence:

- [BuildScoreRowsFromDirectorScores returns nil for empty director scores](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/shared/clickhouse/conversations/scorecard_score.go#L244-L265)
- [Conversation indexing only builds score rows when director scores exist](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/shared/clickhouse/conversations/conversation.go#L797-L818)
- [Conversation indexing writes scorecard rows from generated score rows](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/shared/clickhouse/conversations/conversation.go#L860-L872)
- [BuildScorecardRows skips no-score conversation scorecards but allows process metadata rows](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/shared/clickhouse/conversations/conversation.go#L2947-L2995)

Internal documentation in the reindex workflow also treats unscored scorecards as a major PG-vs-CH count divergence source. That README says empty scorecards are skipped in the documented pipeline; current code has a process-scorecard metadata exception in `BuildScorecardRows`, so process scorecards need separate handling from conversation scorecards.

Evidence:

- [Reindex pipeline documents no-score scorecards as skipped](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/temporal/ingestion/reindexscorecards/README.md#L49-L88)
- [Reindex README explains CH count lower than PG count because unscored scorecards are skipped](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/temporal/ingestion/reindexscorecards/README.md#L109-L116)

### `RetrieveQAScoreStats`

`RetrieveQAScoreStats` is score-driven. The ClickHouse query builds:

- a `scorecard` CTE from `scorecard_d`;
- a latest-version `filtered_scorecard` CTE;
- a `scorecard_score` CTE from either score-level or scorecard-level table depending on resource;
- the final aggregation from `scorecard_score JOIN filtered_scorecard`.

This means a scorecard absent from ClickHouse, or present only as scorecard metadata without matching score rows, does not contribute rows to the final aggregation.

Evidence:

- [Stats query builds scorecard and scorecard_score CTEs](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go#L43-L155)
- [Stats query final aggregation joins scorecard_score to filtered_scorecard](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go#L187-L244)
- [Stats response totals are accumulated from returned ClickHouse rows](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go#L587-L640)
- [Director calls `AnalyticsService.RetrieveQAScoreStats`](https://github.com/cresta/director/blob/301610c1edef5708c0e1c71d4f3bd81e8aed6888/packages/director-api/src/services/cresta-api/insights/insightsApi.ts#L414-L422)

### `RetrieveQAConversations`

`RetrieveQAConversations` is also score-driven. Its result rows are read from the scorecard-score query output. Empty scorecards with no score rows do not produce QA conversation rows.

Evidence:

- [QA conversations query selects score fields from scorecard_score data](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/insights-server/internal/analyticsimpl/retrieve_qa_conversations_clickhouse.go#L139-L150)
- [RetrieveQAConversations reads ClickHouse score rows into response rows](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/insights-server/internal/analyticsimpl/retrieve_qa_conversations_clickhouse.go#L327-L359)
- [Director calls `AnalyticsService.RetrieveQAConversations`](https://github.com/cresta/director/blob/301610c1edef5708c0e1c71d4f3bd81e8aed6888/packages/director-api/src/services/cresta-api/insights/insightsApi.ts#L425-L434)

### Appeal Stats

Appeal stats are built from score maps grouped by scorecard and criterion. The main analysis loop iterates over appeal request scores. If an appeal request has no score rows, it contributes no criterion-level appeal facts. The helper treats missing original scores as changed when there is an appeal score to compare, but an entirely empty appeal request has no iteration entry.

Evidence:

- [Appeal stats build score maps by scorecard/criterion](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/insights-server/internal/analyticsimpl/retrieve_appeal_stats.go#L655-L681)
- [Appeal status treats missing original scores as changed only when comparison is reached](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/insights-server/internal/analyticsimpl/retrieve_appeal_stats.go#L811-L835)
- [Director calls `AnalyticsService.RetrieveAppealStats`](https://github.com/cresta/director/blob/301610c1edef5708c0e1c71d4f3bd81e8aed6888/packages/director-api/src/services/cresta-api/insights/insightsApi.ts#L670-L675)

### Group Calibration Stats

Group calibration stats are score-driven for completed response scorecards:

- response scores are grouped by response scorecard ID;
- response scorecards are added to tasks only from scorecard IDs found in response score rows;
- tasks are initialized with empty response slices so task-level assigned/pending counts can still exist without responses.

An empty group calibration response scorecard will not appear as an evaluated response scorecard because it never enters `responseScorecardIDToScores`.

Evidence:

- [Group calibration groups response scores by response scorecard ID](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/insights-server/internal/analyticsimpl/retrieve_group_calibration_stats.go#L760-L783)
- [Tasks are initialized even with no response scorecards](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/insights-server/internal/analyticsimpl/retrieve_group_calibration_stats.go#L796-L825)
- [Aggregation iterates response scorecards to count evaluated scorecards](https://github.com/cresta/go-servers/blob/eb72d7bb87a43052fffed65d3a9f518c889d8f7d/insights-server/internal/analyticsimpl/retrieve_group_calibration_stats.go#L903-L949)

## Workflow Interpretation

| Workflow | Empty scorecard role | Current behavior from evidence | Risk |
|---|---|---|---|
| Evaluation | Draft or attempted evaluation with no usable criterion scores | Can be persisted by create paths; usually absent from QA analytics if no score rows project | Users may see a scorecard in operational UI while analytics does not count it |
| AutoQA | AutoQA run created a scorecard but produced no mapped/computed score rows | Live/backfill paths can persist directly through shared DAO; no post-compute non-empty guard found | AutoQA can create evidence of "scored" work that analytics later ignores |
| Appeal | Appeal request/resolve with no criterion deltas/scores | FE blocks submitting appeal with no scores; BE create path itself does not show generic non-empty guard; stats iterate appeal request scores | API-only or edge paths may create appeal artifacts that stats ignore |
| Group calibration | Answer key or response without comparable scores | Create response requires a task, not a non-empty score list; stats only count response scorecards discovered through response scores | Assigned/pending task counts can exist, but empty submitted responses may not count as evaluated scorecards |
| Analytics | Scorecard source row without score facts | QA score stats and QA conversations require score rows; projection skips no-score conversation scorecards; process metadata rows may exist but do not contribute to score aggregations without score rows | PG vs CH counts diverge; API totals undercount persisted empty scorecards by design |

## API-Level Answer

| API or path | Empty scorecard handling |
|---|---|
| `CreateScorecard` default evaluation | Allows empty score input after `RemoveEmptyScorecardScores`; persists through DAO unless a template-specific validation fails |
| `CreateScorecard` appeal request | FE blocks empty appeal submit, but BE create path persists flattened appeal scores and has no generic non-empty guard found |
| `CreateScorecard` appeal resolve | Persists flattened resolve scores; no generic non-empty guard found |
| `CreateScorecard` group calibration answer key | Persists flattened scores; no generic non-empty guard found |
| `CreateScorecard` group calibration response | Requires a director task and permission/audience checks; no generic non-empty-score guard found |
| `UpdateScorecard` | Can keep an already-empty scorecard empty; does not necessarily delete existing valid scores when request scores are empty |
| `SubmitScorecard` | Submits existing scorecard; no generic non-empty-score guard found in inspected path |
| Live AutoQA trigger | Skips templates with no AutoQA scorable items, but does not check that computed scores are non-empty before DAO create |
| AutoQA backfill | Computes and directly creates/updates through shared DAO; no non-empty-score guard found in `processScorecard` |
| ClickHouse conversation projection | Empty conversation scorecards produce no score rows and no scorecard rows |
| ClickHouse process projection | Process scorecard metadata rows may be written without score rows, but score analytics still require score rows |
| `RetrieveQAScoreStats` | Effectively filters out empty scorecards by aggregating from `scorecard_score JOIN filtered_scorecard` |
| `RetrieveQAConversations` | Effectively filters out empty scorecards because rows come from score-level query output |
| `RetrieveAppealStats` | Empty appeal requests produce no criterion-level stats because the loop starts from appeal request scores |
| Group calibration stats | Empty responses do not become evaluated response scorecards; task assigned/pending counts can still exist |

## Open Questions

- Should "empty scorecard" mean no `director.scores` rows only, or should it include score rows that are all N/A / all invalid for analytics?
- Should the invariant be "never persist empty scorecards" or "persist them but mark them as non-analytic/non-complete"?
- If the invariant is non-empty, should enforcement live in shared scoring persistence, in workflow-specific create paths, or both?
- Do process scorecards intentionally support metadata-only CH scorecard rows, or is that only a repair/backfill compromise?
- Should AutoQA record a skipped-run artifact instead of a scorecard when no score rows are produced?

## Recommendation

Treat empty scorecards as a named state, not an accidental absence:

1. Add a domain invariant decision: either empty scorecards are allowed drafts, or they are invalid persisted scorecards.
2. If invalid, enforce after `ComputeScores` and before `CreateScorecardAndScoresInDB` in every creation workflow, especially live AutoQA and backfill.
3. If allowed, expose their state explicitly in operational APIs/UI and document that analytics APIs count only score-bearing scorecards.
4. Add tests for empty-scorecard behavior in AutoQA live trigger, AutoQA backfill, default create, appeal create, group calibration response, and QA stats projection.
