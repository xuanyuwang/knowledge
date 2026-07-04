# Analytics API Attribute Map

**Created:** 2026-06-27  
**Updated:** 2026-06-27  
**Status:** Working analytics API reference  
**Source repo:** `go-servers` at `eb72d7bb87a4`

## Purpose

This document is the scorecard/template analytics API reference.

The working question is:

> For each scorecard/template attribute, how is it used by analytics APIs?

Analytics looks simple at the surface: read from ClickHouse or Postgres and build a response. The complexity sits in the large input filter. A single `FilterByAttribute` can imply conditions across scorecard rows, score rows, conversation rows, moment annotation rows, template rows, task rows, and user/audience state.

It combines the low-level storage/query surface map with the response-impact matrix for scorecard, score, template, and request-only analytics attributes.

## Core Read Surfaces

| Surface | Backing store | Main row meaning | Scorecard/template relevance |
|---|---|---|---|
| `scorecard_d` | ClickHouse | One projected scorecard metadata row per scorecard version | Scorecard-level stats, latest-version dedup, status, submit/publish time, scorecard-level score resource |
| `score_d` | ClickHouse | One projected score row per criterion or score item | QA score stats and QA conversations when using criterion/score resource |
| `scorecard_score_d` | ClickHouse | Legacy/alternate scorecard score table | Still appears in table constants and read routing, but current QA score stats path can read `score_d` or `scorecard_d` depending on score resource |
| `director.scorecards` | Postgres | Source-of-truth persisted scorecard | Manual QA progress, QM task stats, group calibration, appeal paths, and projection source |
| `director.scores` | Postgres | Source-of-truth persisted score rows | Group calibration, appeal stats, comments, and projection source |
| `director.scorecard_templates` | Postgres | Template JSON and revision | Template structure parsing for criteria, task config linkage, and current-template resolution |

## Projection Columns

ClickHouse scorecard rows carry scorecard-level metadata:

| Attribute | CH column | Source meaning | Notes |
|---|---|---|---|
| scorecard id | `scorecard_id` | `director.scorecards.resource_id` | Join key between `scorecard_d` and score rows |
| template id | `scorecard_template_id` | `scorecards.template_id` | Used for template filtering and criterion grouping context |
| template revision | `scorecard_template_revision` | `scorecards.template_revision` | Returned by QA conversations; not always used by template filters |
| agent | `agent_user_id` | Evaluated user | Common filter/group-by |
| submitter/reviewer | `submitter_user_id` | QA analyst / reviewer | Reviewer audience filter and submitter group-by |
| manual vs auto | `manually_scored`, `ai_score_time` | Scorecard source/type | Status and score type semantics depend on both |
| status times | `scorecard_submit_time`, `publish_time` | draft/submitted/published state | Status filters are time-column predicates |
| score | `score` | scorecard aggregate score | Uses `-1` when missing in projection |
| special flags | `auto_failed`, `is_voice_mail`, `is_dev_user` | operational/projection metadata | Some APIs expose or filter these indirectly |

ClickHouse score rows carry score-level facts:

| Attribute | CH column | Source meaning | Notes |
|---|---|---|---|
| score id | `score_id` | `director.scores.resource_id` | Used for comments and row dedup in QA conversations |
| criterion id | `criterion_id` | `scores.criterion_identifier` | Criterion filter and criterion group-by |
| values | `numeric_value`, `ai_value`, `text_value` | Manual/AI/text score payload | Missing numeric/AI values are represented as `-1` |
| percentage | `percentage_value` | Derived criterion percentage | Main metric input for score-resource QA stats |
| weights | `weight`, `float_weight` | Derived from template criterion | Metric weighting depends on template parsing |
| N/A flag | `not_applicable` | Score row N/A state | Default QA filters exclude N/A rows |
| score type | `manually_scored`, `ai_scored` | Manual/AI source for this row | Score type filter differs between score and scorecard resource |
| scorecard context | `scorecard_id`, `scorecard_template_id`, `scorecard_template_revision`, submit/publish times | Copied from scorecard | Lets score-driven queries avoid joining PG |

## QA Score Stats

`RetrieveQAScoreStats` builds three main ClickHouse CTEs:

1. optional `conversation` CTE for conversation-level filters;
2. `scorecard` / `filtered_scorecard` CTEs from `scorecard_d`, deduping to the latest scorecard version;
3. `scorecard_score` CTE from either `score_d` or `scorecard_d`, depending on `ScoreResource`.

The final aggregation joins score rows to the latest scorecard rows by:

- `scorecard_id`
- `scorecard_last_update_time`

This means the API is score-driven even when scorecard metadata participates in filtering. A scorecard version with no matching score rows contributes nothing to score-resource stats.

Important behavior:

- Latest-version dedup intentionally comes from `scorecard_d`, not the N/A-filtered score CTE. This avoids falling back to an old scored version when the latest overwrite is all N/A.
- When grouping by criterion, the query groups by both `criterion_id` and `scorecard_template_id`, because criterion IDs are not globally unique across templates.
- For criterion/score resource, the metric is `SUM(percentage_value * float_weight) / SUM(float_weight)` after filtering to usable rows.
- For scorecard resource, the metric is `SUM(score/100) / SUM(1)` after filtering to scorecard rows with usable `score`.

## QA Conversations

`RetrieveQAConversations` uses the same condition split and latest-scorecard join pattern as QA score stats, but returns row-level facts:

- scorecard id
- template id and revision
- score id
- criterion id
- manual/AI fields
- N/A flag
- numeric/percentage/max/weight values

For score-resource reads, each returned row corresponds to a score row. For scorecard-resource reads, the query synthesizes a score-like row from `scorecard_d` with blank `score_id` and `criterion_id`.

Comments are fetched from Postgres only for score-resource rows, using returned `score_id`.

## Filter Split

The same visible filter object lands in different query layers:

| Filter concept | Query layer | Columns / behavior |
|---|---|---|
| time range | common score/scorecard/conversation conditions | `scorecard_time` for score/scorecard tables; optional `conversation_end_time` through conversation join |
| agent users | common conditions | `agent_user_id`; may use external ClickHouse tables |
| reviewer audience | common conditions | `submitter_user_id`; skipped if target table has no such column |
| usecase | common conditions | `usecase_id` |
| scorecard template | common conditions | parsed template name to template id, then `scorecard_template_id IN (...)`; revision is parsed but not used in this condition |
| scorecard status | scorecard conditions | submitted: `scorecard_submit_time <> 0`; draft: `scorecard_submit_time = 0 AND manually_scored = true`; auto: `scorecard_submit_time = 0 AND manually_scored = false`; published/not-published use `publish_time` |
| include N/A | score conditions | default excludes `percentage_value < 0` and `not_applicable = true`; scorecard resource uses `score >= 0` |
| score type | score conditions | manual uses `manually_scored`; auto uses `ai_scored` on score rows or `ai_score_time <> 0` on scorecard rows; overridden combines manual and AI |
| score ranges | score conditions | score rows compare normalized score; scorecard rows multiply request range by 100 |
| criterion identifiers | score conditions | only applies when the score resource is criterion/score rows, not scorecard rows |
| metadata moments | moment annotation CTEs | filters conversations, then joins back to QA score rows |

## Postgres-Backed Analytics

Not every analytics API is ClickHouse-first.

`RetrieveManualQAProgress` reads `director.scorecards` directly. It requires exactly one scorecard template in the request, resolves task/evaluation-period context from template/task config, and queries manually scored, non-calibration, default scorecards by `template_id`, submit time, task ids, usecase, and filtered agents.

Group calibration stats read tasks, templates, scorecards, and scores from Postgres. Template id and revision define the task/template key, while score rows supply criterion-level comparison facts. This makes empty response scorecards invisible to evaluated-response metrics unless another task-level path counts the assignment separately.

Appeal stats are also Postgres score-row driven for criterion-level facts. They parse template criteria to decide which criteria are in scope, then compare original, appeal request, and resolve scores by criterion.

QM task stats mix task config and scorecard rows. Scorecard template names and "template with no task" filters are parsed into template ids before querying task configs and scorecards.

## Starting Attribute Inventory

| Artifact | Attribute | Analytics role |
|---|---|---|
| Template | template id | Primary template filter in CH and PG. Revision is usually not part of the CH template filter. |
| Template | revision | Needed to parse the right template structure during projection and group calibration; returned by QA conversations. |
| Template | criterion id | Score filter/group-by. Must be interpreted with template id when grouped. |
| Template | criterion weight | Projected into `weight` / `float_weight`, affects weighted QA score. |
| Template | criterion max value / scoring options | Used during projection to derive `percentage_value`. |
| Template | lower-is-better criterion direction | Used by QA score stats ranking/quintile logic through an external table of criterion ids. |
| Scorecard | resource id | Join and dedup key. |
| Scorecard | last update time | Latest-version selection and join key between score and scorecard CTEs. |
| Scorecard | submitted/published timestamps | Status filters and scorecard stats time semantics. |
| Scorecard | manually scored / AI scored | Manual/auto/overridden filtering. |
| Scorecard | aggregate score | Scorecard-resource metric input. Missing score is projected as `-1`. |
| Scorecard | agent / submitter | Agent and reviewer filters/group-bys. |
| Scorecard | task ids | PG task/progress filtering. |
| Score | criterion id | Score-level filtering/grouping and template-criterion interpretation. |
| Score | numeric/AI/text values | Response fields and projection inputs. |
| Score | percentage value | Primary score-resource metric input. Missing/invalid values use negative sentinel behavior. |
| Score | not applicable | Default-excluded unless `include_na_scored` is true. |
| Score | auto failed | Returned by QA conversations and carried in projection; deeper API semantics still need mapping. |

## Response Impact Matrix

This section lists how scorecard, score, and template attributes affect the response of scorecard-related analytics APIs.

Covered APIs:

| API | Main backing store |
|---|---|
| `RetrieveQAScoreStats` | ClickHouse |
| `RetrieveQAConversations` | ClickHouse + PG comments/conversations |
| `RetrieveScorecardStats` | ClickHouse |
| `RetrieveManualQAStats` | Postgres |
| `RetrieveManualQAProgress` | Postgres |
| `RetrieveQMTaskStats` / `RetrieveDirectorTaskStats` for QM | Postgres |
| `RetrieveScorecardCriteriaStats` | Postgres |
| `RetrieveAppealStats` | Postgres |
| `RetrieveGroupCalibrationStats` / Director task stats for group calibration | Postgres |
| `RetrieveConversationOutcomeStats` | Template + conversation/moment analytics |

Impact language:

- **filters**: changes which rows are included.
- **groups**: changes response grouping or response `Attribute`.
- **math**: changes counts, averages, percentages, consistency, or overwrite calculations.
- **returns**: appears directly in response detail rows.
- **interprets**: changes how rows are parsed or compared, usually through template structure.
- **excludes**: removes rows by default or by status/type.

### API Response Shape Summary

| API | Response impact model |
|---|---|
| `RetrieveQAScoreStats` | Aggregates scorecard/score rows into `AverageQaScore`, total conversation/scorecard counts, optional grouped `QAScore` rows, and optional ranking/quintile data. |
| `RetrieveQAConversations` | Returns row-level QA conversation facts: scorecard/template/criterion ids, score values, N/A, AI/manual flags, conversation info, and optional score comments. |
| `RetrieveScorecardStats` | Counts completed scorecards and distinct submitters/users from `scorecard_d`, grouped by time or submitter-like user dimension. |
| `RetrieveManualQAStats` | Returns manual QA counts and conversation detail lists: evaluated, commented, acknowledged, published, calibrated, overwritten criteria, average performance, completion. |
| `RetrieveManualQAProgress` | Returns agent progress against a template/task/evaluation period: assigned/evaluated/completion-style progress and optional handled conversation count. |
| `RetrieveQMTaskStats` / `RetrieveDirectorTaskStats` for QM | Returns task/time-period scorecard counts: evaluated, assigned, acknowledged, published, average performance, completion, and export-only overwrite/comment/calibration stats. |
| `RetrieveScorecardCriteriaStats` | Returns AutoQA criterion overwrite stats: total auto-scored criteria and manual overwrite percentage/counts, optionally grouped by behavior. |
| `RetrieveAppealStats` | Returns appealed/resolved/adjusted criteria and conversation counts, plus appealed conversation detail rows. |
| `RetrieveGroupCalibrationStats` / Director task stats for group calibration | Returns group calibration task stats: assigned/pending/evaluated scorecards, consistency score, evaluated scorecard details, optional QA analyst/task/time grouping. |
| `RetrieveConversationOutcomeStats` | Uses scorecard template audience and template behavior/outcome mapping to constrain and interpret conversation outcome/adherence analytics. |

### Template Attributes

| Attribute | RetrieveQAScoreStats | RetrieveQAConversations | RetrieveScorecardStats | RetrieveManualQAStats | RetrieveManualQAProgress | RetrieveQMTaskStats | RetrieveScorecardCriteriaStats | RetrieveAppealStats | RetrieveGroupCalibrationStats | RetrieveConversationOutcomeStats |
|---|---|---|---|---|---|---|---|---|---|---|
| Template id | Filters `scorecard_template_id`; participates in criterion group-by context. | Filters rows and returns `scorecard_template_id`. | Can filter through generic CH filters; changes scorecard counts. | Required single template; filters scorecards, resolves task config/audience, shapes all counts/details. | Required single template; filters scorecards and resolves period/task/audience. | Filters tasks/templates/scorecards; partitions task stats by template. | Filters submitted scorecards and template set used to map behaviors to criteria. | Optional filter on appeal request scorecards. | Filters group calibration tasks by `scorecard_template_id`. | Required single template; resolves audience and behavior/outcome mapping. |
| Template revision | Not used in common CH template filter; affects projection correctness before query. | Returned as `scorecard_template_revision`; not generally a filter. | Not a visible response dimension. | Used when calibration scorecards are reconstructed for consistency; bad revisions can break consistency calculation. | Usually indirect through template/task config, not response field. | Current template lookup drives task/template set; revision less visible except task config. | Uses current templates for behavior-to-criterion mapping, not returned. | Reads `(template_id, revision)` from appeal scorecards to parse criteria in scope. | Task content config has template revision; joins exact template revision and parses criteria. | Required scorecard template name includes revision; fetches that revision for audience/mapping. |
| Template title | No response effect in inspected CH path. | No response effect in inspected CH path. | No response effect. | Not a metric input; may appear only through surrounding template lookup errors/logs. | No direct response effect. | No direct response effect. | No direct response effect. | No direct response effect. | No direct response effect. | No direct response effect. |
| Template audience | No direct CH effect unless translated into user filters before request. | Same as `RetrieveQAScoreStats`. | Same as `RetrieveQAScoreStats`. | Resolves assigned/audience users; affects assigned count, completion rate, and empty rows for audience agents. | Core input to progress baseline. | Indirect through tasks and filtered users. | No direct response effect except ACL/user filters. | No direct response effect. | Task audience defines assigned QA analysts and pending counts. | Fills missing user/group filter with template audience, changing included conversations and grouped response rows. |
| Template usecase | Filters `usecase_id` when request includes usecase names. | Filters rows and returned result set. | Filters scorecard counts through generic CH conditions. | Filters templates/task configs/scorecards and assignment baseline. | Filters templates/evaluation periods/scorecards. | Filters templates/tasks/scorecards. | Filters templates used for behavior mapping and scorecards counted. | Required one usecase in request validation/conditions; affects appeal scorecard set. | Required task/usecase condition; filters group calibration tasks. | Indirect via template and conversation filters. |
| Criterion id | Filters and groups score rows. Criterion group-by returns `criterion_id` plus template id. | Filters score-resource rows; returned as `criterion_id`. Blank for scorecard resource. | No direct effect. | Used only in overwrite export logic via score rows and scorable criteria set. | No direct effect in progress counts. | Export-only overwrite stats count distinct overwritten criteria. | Primary response key for criterion overwrite counts; optionally mapped to behavior. | Primary unit of appealed/resolved/adjusted criteria; used in response counts. | Primary comparable-score key; can group stats by criterion. | Template behavior/outcome mapping may derive behavior moments from criteria/triggers. |
| Criterion display name | No direct response field in inspected path. | No direct response field. | No direct effect. | No direct effect. | No direct effect. | No direct effect. | No direct response field. | No direct effect. | Parsed and stored as criterion display name for evaluated criterion output. | No direct effect. |
| Criterion weight | Math: `float_weight` changes weighted QA score and ranking score. | Returned as `weight`; affects row facts only, not row inclusion. | No direct effect. | No direct effect on counts; scorecard aggregate already persisted. | No direct effect. | No direct effect on task counts; aggregate score already persisted. | No direct effect. | No direct effect; appeal compares raw score values/N/A/text. | No direct effect unless consistency helper uses criterion scoring semantics. | No direct effect. |
| Criterion max value / scoring options | Projection derives `percentage_value`; therefore changes QA score math. | Returned as `max_value`, `percentage_value`, numeric fields. | No direct effect. | Affects persisted scorecard score before analytics; not recomputed here. | Same as `RetrieveManualQAStats`. | Same as `RetrieveManualQAStats`; average perf uses persisted `score`. | Numeric/AI equality drives overwrite count, not percentage math. | Raw score equality and N/A/text/numeric comparison drive appeal status. | Determines whether score is comparable and how consistency can be computed. | Outcome/adherence mapping uses behavior/outcome config, not score percentage. |
| Criterion lower-is-better direction | Optional ranking/quintile math negates selected criterion weighted values. | No direct effect. | No direct effect. | No direct effect. | No direct effect. | No direct effect. | No direct effect. | No direct effect. | No direct effect. | No direct effect. |
| Outcome criterion marker | Usually only matters before projection/derivation. | Returned like other criteria if projected as score row. | No direct effect. | Overwrite count excludes outcome criteria. | No direct effect. | Export overwrite count excludes outcome criteria through shared detail logic. | Behavior mapping only includes AutoQA behavior triggers; outcome criteria are not central here. | Criteria list includes template criteria; appeal comparison can include any criterion not skipped as chapter. | Comparable-score filtering depends on scorable-for-consistency rules. | Central: outcome moments/behavior mappings shape outcome stats. |
| AutoQA behavior trigger | No direct CH query effect unless criterion ids are filtered. | No direct effect. | No direct effect. | Overwrite counts depend on `ai_scored`, not trigger config. | No direct effect. | Export overwrite count depends on AI/manual deltas, not trigger config. | Maps behaviors to criterion ids; changes behavior-grouped overwrite response. | No direct effect. | No direct effect. | Provides behavior/outcome mapping for conversation outcome/adherence analytics. |

### Scorecard Attributes

| Attribute | RetrieveQAScoreStats | RetrieveQAConversations | RetrieveScorecardStats | RetrieveManualQAStats | RetrieveManualQAProgress | RetrieveQMTaskStats | RetrieveScorecardCriteriaStats | RetrieveAppealStats | RetrieveGroupCalibrationStats | RetrieveConversationOutcomeStats |
|---|---|---|---|---|---|---|---|---|---|---|
| Scorecard id | Dedup/join key; counted distinct in totals. | Returned as `scorecard_id`; row dedup key with score id. | Counted distinct. | Returned as `ScorecardName`; keys detail lists and overwrite/calibration maps. | Returned/used as progress detail identity where applicable. | Counted; export stats key scorecard details. | Joins scorecards to scores for criterion overwrite counts. | Original scorecard id appears in appealed conversation detail; workflow key. | Response scorecard id appears in evaluated scorecard detail and consistency maps. | Not central unless conversation filters include scorecard constraints upstream. |
| Scorecard last update time | Latest-version join/dedup with score rows; changes whether old rows count. | Same latest-version join; changes returned latest row facts. | No explicit latest-version CTE in simple count path beyond CH table contents. | No direct use. | No direct use. | No direct use. | No direct use. | No direct use. | No direct use. | No direct use. |
| Scorecard create/conversation time | Time range can use `scorecard_time`/conversation time; affects included rows and time grouping. | Returned conversation start/create time; affects ordering/page result. | Time grouping uses scorecard submit time after column rewrite. | Conversation create time returned in details; submitted time usually drives periods. | Evaluation period/time range affects scorecards and assigned baseline. | Submitted time drives periods/time groups. | Conversation started time used when `FilterByTimeRange` joins chats. | Appeal request submit time drives inclusion and frequency grouping; conversation create time returned. | Task overlap and submitted time/frequency drive task/score inclusion. | Conversation time filters/grouping drive outcome/adherence response. |
| Submitted at / submit time | Status filter and latest scorecard response counts. | Status filter; returned indirectly through selected rows. | Core completed count time and grouping source. | Core inclusion: submitted manual scorecards only; drives time grouping and detail lists. | Core inclusion and period progress. | Core inclusion, time grouping, evaluated count. | Requires submitted scorecards. | Requires submitted appeal request/original/replica; drives appeal frequency. | Requires submitted answer/response scorecards; drives frequency grouping. | Indirect only. |
| Published at | Published/not-published status filters. | Status filter; not directly returned. | No direct response field except filter effect. | Adds scorecard to `PublishedConversations`; affects published count/list. | No direct effect in progress baseline unless surfaced in detail. | Affects published scorecard count. | No direct effect. | No direct effect. | No direct effect. | No direct effect. |
| Acknowledged at | Available as CH column but not central to QA score response. | Not returned by inspected QA conversation response. | No direct effect. | Adds scorecard to acknowledged list/count. | No direct effect in inspected progress query. | Affects acknowledged scorecard count. | No direct effect. | No direct effect. | No direct effect. | No direct effect. |
| Aggregate score | Scorecard-resource math uses `score/100`; negative score excluded by default. | Scorecard-resource rows return it as normalized numeric/percentage value. | No score math; counts scorecards. | Returned as `PerfScore`; average perf excludes negative scores. | May be part of progress only indirectly. | Average performance score sums non-negative scores. | No direct effect. | No direct effect; appeal compares score rows. | Consistency is separate from aggregate perf score. | No direct effect. |
| Manual vs auto scorecard | Status and score type filters: manual/draft/submitted/auto depend on `manually_scored` and submit time. | Returned as `ManuallyScored`; filters result rows. | No direct unless generic filter applies. | Requires `manually_scored = TRUE`. | Requires/evaluates manual scorecards. | Time-range aggregate path requires manual; task period fetch does not always add manual condition in every helper. | Submitted scorecards are counted; score rows must be `ai_scored` for overwrite. | Appeal scorecard types, not manual flag, drive inclusion. | Group calibration `scorecard_type` drives inclusion. | No direct effect. |
| AI score time / AI scored | Score type auto/overridden filters; ranking/score rows can depend on score-level AI flags. | Returned as `AiScored`; filters result rows. | No direct effect. | Overwrite stats compare AI/manual score values. | No direct effect. | Export overwrite count compares AI/manual score values. | Requires `s.ai_scored = TRUE`; overwrite percent based on AI vs manual values. | No direct effect. | No direct effect. | No direct effect. |
| Agent user id | Filters and groups QA scores; contributes user attributes. | Returned as user in conversation info. | Count distinct users and group user/time. | Filters/group-by agent; returned in detail rows. | Filters agent progress and audience rows. | Filters task scorecards; may group indirectly through task/audience. | Filters scorecards. | Filters appeal scorecards and group-by agent. | Not task assignee; response scorecard analyst is submitter. | Filters conversation/outcome user set. |
| Submitter / reviewer user id | Reviewer audience filter; group-by scorecard submitter. | Returned only indirectly through filters; no main row field in response. | Scorecard stats rewrites agent column to submitter for user count/grouping. | Filters QA analyst/reviewer; returned as QA analyst in details and group-by. | Filters QA analyst/reviewer where applicable. | Filters submitter; response task stats affected by evaluator subset. | No direct effect. | Appeal submitter/resolver/QA analyst group keys. | QA analyst/assignee id determines assigned/pending/evaluated groups. | No direct effect. |
| Task ids | No direct CH response effect in inspected path. | No direct effect. | No direct effect. | Filters scorecards by task or no-task; changes assigned/completion detail. | Filters task/evaluation period and scorecards. | Primary task grouping and assigned/evaluated mapping. | No direct effect. | No direct effect. | Primary task membership for answer/response scorecards. | No direct effect. |
| Usecase id | Filters rows through common conditions. | Filters rows. | Filters counts through generic CH filters. | Filters templates, task configs, scorecards, assigned baseline. | Filters evaluation periods and scorecards. | Filters templates/tasks/scorecards. | Filters scorecards and template behavior mapping. | Required/filters appeal stats conditions. | Filters group calibration tasks. | Filters conversations if present in request. |
| Scorecard type | Not directly exposed in CH QA path. | Not directly returned. | Usually default scorecard table rows only; no explicit response field. | Excludes calibration/appeal: `calibrated_scorecard_id IS NULL` and default type. | Same default scorecard filter. | Default QM stats exclude non-default scorecards; group calibration path selects GC types. | Excludes non-default scorecards. | Selects appeal request scorecards and workflow scorecard roles. | Selects group calibration response vs answer key scorecards. | No direct effect. |
| Calibration linkage | Not direct. | Not direct. | Not direct. | Left joins calibration scorecards; affects calibrated list and consistency score. | No direct effect. | Export stats count calibrated scorecards and average consistency. | Excluded from base stats. | Appeal workflow uses separate linkage logic, not calibration linkage. | Core group calibration response/answer linkage through task ids and scorecard type. | No direct effect. |
| Process identifiers | Process scorecard projection can make scorecard metadata visible without score rows, but score-driven stats still need score rows. | Process details can be returned when projected/queried. | Can count process scorecard rows if present and filters match. | Returned as process id/interaction time in detail rows for process templates. | Progress can use process template context. | Process template flag affects export/session logic. | No direct effect. | No direct effect. | No direct effect. | No direct effect. |
| Comment | No direct CH response effect. | Score comments optionally fetched from PG by score id, not scorecard comment. | No direct effect. | Scorecard comment adds to commented conversation list/count. | No direct effect. | Export stats count commented scorecards. | No direct effect. | No direct effect. | No direct effect. | No direct effect. |
| Submission source | Not in inspected CH QA matrix. | Not returned. | No direct effect. | Filters scorecards and changes counts/details. | No direct effect in inspected progress snippet. | Filters scorecards for evaluated/published/acknowledged/average stats. | No direct effect. | No direct effect. | No direct effect. | No direct effect. |

### Score Attributes

| Attribute | RetrieveQAScoreStats | RetrieveQAConversations | RetrieveScorecardStats | RetrieveManualQAStats | RetrieveManualQAProgress | RetrieveQMTaskStats | RetrieveScorecardCriteriaStats | RetrieveAppealStats | RetrieveGroupCalibrationStats | RetrieveConversationOutcomeStats |
|---|---|---|---|---|---|---|---|---|---|---|
| Score id | No response field, but score rows determine aggregation. | Returned as `ScoreId`; used to fetch comments and dedup rows. | No effect. | Not returned; scorecard detail keyed by scorecard. | No effect. | Export overwrite query groups by scorecard/criterion, not score id. | No direct response field. | No direct response field; scores grouped by criterion/message. | No direct response field. | No effect. |
| Criterion identifier on score row | Filter/group/math unit. | Returned and filterable. | No effect. | Export overwrite count uses distinct criterion ids. | No effect. | Export overwrite count uses distinct criterion ids. | Primary grouping key. | Primary appeal status key. | Primary answer/response comparison key. | Indirect only through template mapping. |
| Numeric value | Projection/math input via percentage; score range filters use derived percentage/score. | Returned as `NumericValue`. | No effect. | Only aggregate score is used except overwrite comparisons. | No direct effect. | Export overwrite and consistency paths use score values. | Compared with `ai_value` to count manual overwrite. | Compared original vs appeal vs resolve. | Compared to answer key for consistency. | No direct effect. |
| AI value | Projection/math input and AI/manual comparison source. | Returned as `AiValue`. | No effect. | Overwrite count compares AI vs numeric value. | No effect. | Export overwrite count compares AI vs numeric. | Compared with numeric value. | No direct effect unless appeal scores contain AI/manual deltas in values. | No direct effect. | No direct effect. |
| Text value | Not part of numeric QA score math. | Available only if projected/read path exposes it; current response focuses numeric/AI/percent. | No effect. | Not central. | No effect. | No direct effect. | No direct effect. | Text equality participates in appeal changed/not-changed comparison. | Usually not comparable for consistency unless criterion rules allow. | No direct effect. |
| Percentage value | Primary score-resource numerator and score-range filter. Negative values are excluded by default. | Returned as `PercentageValue`. | No effect. | Not recomputed; aggregate score already persisted. | No effect. | Not recomputed for task averages. | No direct effect. | No direct effect. | No direct effect. | No direct effect. |
| Weight / float weight | Primary score-resource weighted average denominator and numerator. | Returned as `Weight`; affects row interpretation. | No effect. | No direct effect. | No effect. | No direct effect. | No direct effect. | No direct effect. | No direct effect. | No direct effect. |
| Not applicable | Default `RetrieveQAScoreStats` excludes `not_applicable = true` and negative percentage unless `include_na_scored`. | Returned as `NotApplicable`; default filters can remove rows. | No effect. | No direct count effect unless aggregate score/overwrites changed upstream. | No direct effect. | Export consistency may treat N/A according to score comparison rules. | No direct filter in overwrite stats unless values imply equality/delta. | N/A equality participates in appeal changed/not-changed comparison. | Response/answer N/A affects comparable score and consistency behavior. | No direct effect. |
| AI scored flag | Score type filter and AI/overridden filtering. | Returned as `AiScored`; filters rows. | No effect. | Overwrite details count `s.ai_scored = TRUE` rows. | No effect. | Export overwrite count requires `s.ai_scored = TRUE`. | Requires `s.ai_scored = TRUE`. | No direct effect. | No direct effect. | No direct effect. |
| Auto failed flag | Carried in CH rows; can be returned by QA conversations. | Returned as `AutoFailed`. | No direct effect. | No direct effect in inspected path. | No direct effect. | No direct effect. | No direct effect. | No direct effect. | No direct effect. | No direct effect. |
| Message id | No direct grouped field in default stats. | May be returned indirectly through conversation/message info depending on source path. | No effect. | Returned as message names in manual QA detail rows from scorecard message ids. | No direct effect. | No direct effect. | No direct effect. | Per-message appeal mode uses message id as part of appeal status key. | No direct effect. | No direct effect. |
| Score comment | No direct effect. | Optional PG fetch returns score comment when `include_comments` and score resource is score rows. | No effect. | Scorecard comment, not score comment, drives commented scorecards. | No effect. | Scorecard comment drives export commented count. | No effect. | No direct effect. | No direct effect. | No effect. |

### Request-Only Analytics Attributes That Bind To Scorecard State

| Request/filter attribute | RetrieveQAScoreStats | RetrieveQAConversations | RetrieveScorecardStats | RetrieveManualQAStats | RetrieveManualQAProgress | RetrieveQMTaskStats | RetrieveScorecardCriteriaStats | RetrieveAppealStats | RetrieveGroupCalibrationStats | RetrieveConversationOutcomeStats |
|---|---|---|---|---|---|---|---|---|---|---|
| `ScoreResource` | Switches from score-row weighted math to scorecard-row aggregate score math. | Switches from score rows to synthetic scorecard rows. | N/A. | N/A. | N/A. | N/A. | N/A. | N/A. | N/A. | N/A. |
| `IncludeNaScored` | Keeps N/A/negative rows that are otherwise excluded. | Keeps N/A rows in returned details. | N/A. | N/A. | N/A. | N/A. | N/A. | N/A. | N/A. | N/A. |
| `ScoreType` | Filters manual/auto/overridden using manual and AI fields. | Same as `RetrieveQAScoreStats`; changes returned row set. | N/A. | N/A. | N/A. | N/A. | N/A. | N/A. | N/A. | N/A. |
| `ScoreRanges` | Filters by score or percentage; scorecard resource uses 0-100 scale. | Same filter changes returned row set. | N/A. | N/A. | N/A. | N/A. | N/A. | N/A. | N/A. | N/A. |
| Scorecard status filters | Filters latest scorecards before aggregation. | Filters returned row set. | May filter through generic scorecard conditions if wired by caller. | Manual stats have implicit submitted/default/manual constraints instead. | Same implicit constraints. | Same implicit constraints. | Requires submitted/default scorecards. | Appeal has workflow-specific submitted/type constraints. | GC has workflow-specific task/type/submitted constraints. | N/A. |
| Group by criterion | Groups by `(scorecard_template_id, criterion_id)`. | N/A; row detail already criterion-level. | N/A. | N/A. | N/A. | N/A. | Optional behavior grouping uses criterion mapping, not plain criterion group-by. | Criteria counts are always criterion-derived; group-by can be agent/time/etc depending implementation. | Explicit criterion group-by supported. | N/A. |
| Group by QA analyst / submitter | Uses `submitter_user_id` when requested. | N/A in response rows. | User count/grouping uses submitter due scorecard stats column rewrite. | QA analyst group-by changes response buckets. | QA analyst filter affects progress. | Reviewer/submitter filter changes task stats; group-by task/time only in QM path. | N/A. | QA analyst / appeal submitter / resolver can be group keys. | QA analyst group-by supported. | N/A. |
| Frequency / time group | Uses scorecard/conversation time bucket in grouped `QAScore`. | N/A except paging/order. | Groups completed scorecards by time. | Groups manual QA stats by submitted time or evaluation period. | Evaluation periods/time range define progress. | Groups task stats by period/time. | Time range filters scorecards; no main time-group response in inspected path. | Groups by appeal submit frequency. | Groups by task/score submitted frequency. | Groups by conversation time when requested. |

### Cross-API Edge Rules

| Rule | Response impact |
|---|---|
| Empty scorecard with no score rows | Usually invisible to score-row APIs (`RetrieveQAScoreStats` score resource, `RetrieveQAConversations` score resource, `RetrieveAppealStats` criterion counts, `RetrieveGroupCalibrationStats` evaluated responses). It can still count in scorecard-row APIs if projected as `scorecard_d` and the API reads scorecard rows. |
| All-N/A latest version | Default `RetrieveQAScoreStats` / `RetrieveQAConversations` score-resource filters remove N/A rows, but latest-version dedup still comes from `scorecard_d`; this prevents stale old scored versions from reappearing. |
| Missing aggregate score | Projected as negative sentinel; average score calculations usually exclude negative scores while counts may still include the scorecard. |
| Template revision mismatch | APIs that parse template JSON by exact revision (`RetrieveAppealStats`, `RetrieveGroupCalibrationStats`, consistency calculations) can fail or misinterpret criteria. CH template filters usually do not protect against revision-level ambiguity. |
| Criterion id collision across templates | `RetrieveQAScoreStats` criterion group-by carries template id. Any response or client logic that groups only by criterion id can merge unrelated criteria. |
| Process scorecards | Projection may emit scorecard metadata without score rows. Scorecard-row APIs can see them, but score-driven APIs still need score rows. |

## Edge Cases to Expand Next

- **Empty scorecard:** persisted scorecard with no usable score rows. Existing analysis: `empty-scorecards-workflow-and-api-analysis.md`.
- **All-N/A scorecard:** latest scorecard version may have score rows, but default score filters remove all rows. Latest-version dedup must still use `scorecard_d` so old scored versions do not leak back in.
- **Template revision drift:** CH template filters currently collapse to template id in common QA conditions; projection and PG workflow paths still care about revision for structure parsing.
- **Criterion ID collision:** criterion group-by must carry template id; any consumer that treats criterion id alone as globally unique is suspect.
- **Scorecard resource vs score resource:** the same API can switch from criterion-row semantics to scorecard-row semantics. Filters such as score type and score range change columns/scales.
- **Process scorecards:** projection can write scorecard metadata rows without score rows, unlike normal conversation scorecards; score-driven QA stats still need matching score facts.
- **RetrieveClosedConversations:** scorecard filters also affect the conversation search/list response, but that should be modeled separately because the primary response entity is a conversation rather than a scorecard analytics aggregate.

## Working Method for the Knowledge Base

For the next passes, use two complementary directions:

1. Attribute-first: pick one scorecard/template attribute and map every analytics API that reads, filters, groups, projects, or returns it.
2. API-first: pick one analytics API, read its query builder, and list which scorecard/template attributes participate in each CTE or Postgres query.

The target output should eventually be a matrix of:

- API
- backing store
- table/CTE
- scorecard attributes used
- template attributes used
- default exclusions
- edge-case behavior
- tests that lock the behavior

Additional refinement targets:

- Split `RetrieveDirectorTaskStats` into QM and group-calibration mode in separate matrices, because the API name is shared but the business semantics are different.
- Add source-line evidence links once the matrix stabilizes.
- Add tests/fixtures column showing which behavior is covered and which is only inferred from query shape.
