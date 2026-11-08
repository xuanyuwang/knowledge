# Performance Insights + QM P0: engineering plan

**Historical P0 sketch for Xuanyu · 2026-10-08.** Covers the P0 requirements in the [Omni Use Case QM / Coaching PRD](https://docs.google.com/document/d/1iEYDUg7NrYo5NjeTcr9nHyNqEbv_sWHrDHJeAkFJS9c/edit?tab=t.0#heading=h.i4n1p4ads1o). The SQL below shows query shape, not copy-ready production SQL.

> **November 4 update:** the [full company-format plan](omni-usecase-scorecard-eng-plan-draft.md) supersedes this sketch's implementation proposals. The [confirmed decisions](../decisions/2026-11-04-omni-usecase-autoqm-and-reporting.md) allow manual-only uncovered AutoQM, reject overlapping new rule mappings, use current Opera scope on new automatic runs, show filtered completion as N/A, and require confirmation for a finite-to-ALL reset. V3 and derived task scope are proposed engineering choices; the examples below retain their October context.

## Core change: one template, scoped criteria

Template scope already lives in each revision's PostgreSQL `usecase_ids` column. Add criterion scope inside the versioned `template` JSONB and its typed Go model, with every criterion's effective scope contained by its **own revision's** template scope. Criteria remain owned by that template; no criterion table is needed for this phase.

**PostgreSQL — existing table, relevant columns only.** `director.scorecard_template_revisions` also stores `usecase_ids` and `template` JSONB.

~~~sql
CREATE TABLE director.scorecard_templates (
    customer     VARCHAR NOT NULL,
    profile      VARCHAR NOT NULL,
    resource_id  VARCHAR NOT NULL,
    revision     VARCHAR NOT NULL,
    usecase_ids  VARCHAR[],       -- e.g. ARRAY['A', 'B']; NULL or '{}' means ALL
    template     JSONB NOT NULL,  -- versioned criterion structure
    PRIMARY KEY (customer, profile, resource_id, revision)
    -- Other existing columns omitted.
);
~~~

**JSONB — proposed criterion field.** For a revision whose table row has `usecase_ids = ARRAY['A', 'B']`:

~~~json
{
  "version": 2,
  "items": [
    {
      "identifier": "quality",
      "displayName": "Quality",
      "type": "numeric-radios",
      "weight": 1,
      "settings": {"range": {"min": 0, "max": 5}},
      "usecase_ids": ["A", "B"]
    },
    {
      "identifier": "chat-specific",
      "displayName": "Chat-specific",
      "type": "numeric-radios",
      "weight": 1,
      "settings": {"range": {"min": 0, "max": 5}},
      "usecase_ids": ["B"]
    }
  ]
}
~~~

**Go — illustrative additions to the existing parser and typed criterion.** The current structs do not yet have either field; the excerpt shows the intended shape, not implemented code.

~~~go
type scorecardTemplateItem struct { // parser input; existing fields omitted
    UsecaseIDs json.RawMessage `json:"usecase_ids"`
}

type BaseCriterion struct { // typed model; existing fields omitted
    UsecaseIDs *[]string `json:"usecase_ids"`
}
~~~

The parser must check whether `UsecaseIDs` is absent **before** converting it to the typed field: absent means a legacy criterion inherits that revision's template scope; JSON `null` becomes a nil pointer (NONE); `[]` becomes a pointer to an empty slice (ALL profile use cases; valid only under an ALL template); explicit IDs become a pointer to that subset. A plain `*[]string` parser field would collapse absent and `null`. New serialization must retain the explicit field. Whether to increment the JSON version is still open.

In this example, a scorecard for a conversation of use case A writes only the quality score; a scorecard for a conversation of use case B writes both scores. An inapplicable criterion writes **no score row** and contributes no denominator weight. Existing scorecards keep their pinned template revision and scores. IDs A/B are illustrative.

With a finite template scope, an applicable criterion writes its explicit IDs. On template scope edits, add/remove the use case on every criterion in the new revision. Canonical template reads return the full revision; evaluation derives the per-use-case view. Fix latest-template selection before use-case filtering: the current DISTINCT ON includes usecase_ids and can surface multiple revisions of one template after a scope edit.

## 1. Template config

| Requirement | Change |
| --- | --- |
| Author once for A+B; criterion Y only in B | Builder edits template and criterion scopes together. Frontend and backend validate subset, branch containment, and at least one applicable criterion per template use case. Commit one coherent revision. |
| AutoQM across use cases | `enableGlobalOperaRule` gates multi-use-case Opera rule authoring. Plan for a shared criterion to reference separate use-case-specific Opera rules, such as one A rule and one B rule, when that flag is off. A single rule spanning A+B remains an option where the flag is on. The existing JSON supports multiple `auto_qa.triggers`, but the builder currently edits only `triggers[0]`; extend its picker/save path to attach and display several rules on one criterion. |
| Criterion/rule mismatch | Show each linked rule's use-case coverage and warn about criterion use cases with no eligible rule. For a concrete conversation, score only the criterion's eligible rule(s); skip ineligible rules before interpreting absent annotations, which otherwise may become N/A. Detect overlapping global and use-case-specific rules and reject ambiguity or define precedence. Decide whether an uncovered use case remains manually scorable or requires full AutoQM coverage. |

## 2. Performance Insights

**Why it fits:** ClickHouse score_d and scorecard_d already have usecase_id; score_d also has criterion_id. RetrieveQAScoreStats accepts repeated use-case and criterion filters, while RetrieveConversationStats accepts repeated use cases for conversation metrics. The Director request currently supplies at most one use case. A read-only staging EXPLAIN ESTIMATE found identical local-table marks for representative one- and two-use-case filters; it had no shared template and did not measure latency.

| P0 display | Query / UI change |
| --- | --- |
| Multi-select and cross-use-case template picker | Use the Dashboard Builder filter pattern: top-left selection initializes the page, then a local selection overrides it. Pass authorized selected IDs to score stats, conversation stats, and example retrieval. Keep one template identity across use cases. |
| First table: template and criterion scope | Read the normalized **current** template revision and display its applicability. |
| Second/third tables: combined result plus use-case breakdown | Add use case to QAAttributeType grouping (or bounded scoped requests). Carry the latest scorecard's usecase_id through the scorecard → filtered_scorecard CTE to GROUP BY usecase_id with criterion/time/agent as requested. Update both ordinary and metadata-filter query paths, row scanner, API response, and rendering. Show N/A where a criterion does not apply. |

**Existing-query sketch:** keep latest-scorecard-version selection and DISTINCT score-row handling; add the use-case projection/grouping:

~~~sql
SELECT filtered_scorecard.usecase_id, scorecard_score.criterion_id,
       sumIf(percentage_value * float_weight, percentage_value >= 0) AS weighted_sum,
       sumIf(float_weight, percentage_value >= 0) AS weight_sum
FROM scorecard_score JOIN filtered_scorecard
  USING (scorecard_id, scorecard_last_update_time)
GROUP BY filtered_scorecard.usecase_id, scorecard_score.criterion_id;
~~~

**Scope changes inside the date range:** filter historical facts by each scorecard's stored use case and pinned revision, not today's criterion scope. A template removed from use case B must still allow its old B scorecards to be found in historical reporting; current template discovery and historical reporting discovery therefore need different scope rules. Check added/removed criteria, all-N/A scorecards, and weighted numerator/denominator parity between combined and breakdown views. Distinct conversation counts are not always additive across criterion groups.

## 3. QM task

**Current gap:** Director copies the template's first use case into singular tasks.usecase_id; backend create/update do not enforce the relationship. Task list, evaluation-period lookup, and QM stats filter that copied column. An A+B template's task can disappear under a B-only filter.

**P0 choice:** a template-based QM task's effective scope equals its latest linked template's scope for future evaluations. Enforce a valid same-profile template link on new writes; audit missing/stale legacy links. Resolve the latest template **before** applying selected-use-case overlap, using the template ID in qmTaskContentConfig. Replace QM task predicates on tasks.usecase_id with that derived scope in list, period/progress, suggestion, and report paths. Multiple tasks may share one template but keep their own audience, schedule, quota, and conversation filters. Existing scorecards retain their historical task link. Benchmark the join before deciding whether an indexed task-scope projection is needed. Group-calibration tasks keep their separate scope contract.

## 4. QM Task Home

~~~mermaid
flowchart LR
    F["Selected + entitled use cases"] --> I["Intersect with current template scope<br/>and task conversation filters"]
    Q["Assigned QM task"] --> I
    I --> V["Fetch candidate conversations<br/>from all matching use cases"]
    V --> E["Evaluate each conversation<br/>in its concrete use case"]
~~~

List assigned tasks across entitled use cases, with a filter to narrow them. The evaluation interface can adjust the multi-select. SuggestManualQAConversations currently accepts one usecase_name and passes it to template lookup, period lookup, and progress. Extend it to a selected set, intersect that set with task/template/permission scope, and apply the same set to conversation retrieval. Its existing fallback can fill use cases from the template when the task filter is empty, but a populated task filter does not guarantee this intersection. Review QM task-config, scorecard-task-ID, and AutoQM backfills before enabling shared templates. P0 assignment and quotas remain task-wide.

## 5. QM Report

Use one local multi-select across template/task lists, stats, drilldowns, and exports; enforce entitlement server-side and include the effective set in frontend cache keys.

| Tab | Query change |
| --- | --- |
| Evaluations | Keep the combined grain **template × task × cycle**. Derive shared-task scope from its template. Add scorecards.usecase_id IN (selected IDs) to the overall-chart and task-cycle scorecard queries **before** aggregation; the no-task branch already has it. Stop the RetrieveManualQAStats frontend hook from forcing one use case. Add a per-use-case evaluated-conversation breakdown in task drilldown and use the same filter for CSV. |
| Appeals | Remove backend validation requiring exactly one use case; apply the selected set to the cross-use-case template picker and all cards/table queries. |
| Group Calibrations | Pass selected use cases through template, task, session, stats, and scorecard reads. Preserve each calibration task's own use-case identity. |

**PostgreSQL proof:** director.scorecards stores usecase_id, template_id, template_revision, and task_ids; director.scores joins by scorecard ID. RetrieveManualQAStats count/detail/overwritten-score queries already filter scorecards.usecase_id IN (...). The two main RetrieveQMTaskStats scorecard branches omit it. The existing schema and list-aware manual-QA queries establish source-level feasibility for cross-use-case reads, but no PostgreSQL benchmark or shared-task fixture has been run.

~~~sql
-- One task cycle; omit sc.usecase_id from GROUP BY for the combined row.
SELECT sc.template_id, sc.usecase_id, count(*) AS evaluated
FROM director.scorecards sc
WHERE sc.template_id = <template> AND <task> = ANY(sc.task_ids)
  AND sc.submitted_at BETWEEN <cycle_start> AND <cycle_end>
  AND sc.usecase_id IN (<selected use cases>)
GROUP BY sc.template_id, sc.usecase_id;
~~~

**Open metric:** quota is task-wide. A subset's evaluated count divided by the whole-task quota is misleading; define whether filtered completion rate is N/A or gains a per-use-case denominator.

## Implementation checks and future query design

| Gate | Evidence needed |
| --- | --- |
| Compatibility | Single-use-case behavior, template-scope edits, pinned historical scorecards, entitled-use-case enforcement, and rule coverage. |
| Data paths | Revision-safe latest-template lookup; direct SQL readers; QM task/config, scorecard-task-ID, and AutoQM backfills. |
| Performance | Actual ClickHouse plans/latency with a shared template and PostgreSQL EXPLAIN/timings for derived task lookup and report grouping. The earlier ClickHouse check was feasibility only. |

For future Performance Insights tracing, record the normalized filters, named query shape, SQL fingerprint/ClickHouse query ID when available, and per-use-case weighted numerator/denominator. Keep sanitized query fixtures and reconciliation checks at each metric's true grain. The ClickHouse builder currently composes filter, CTE, metadata, and group-by strings; split **internal builders by supported page/query shape** while sharing latest-scorecard and dedup logic. A public API split by every combination would multiply contracts and callers; consider it only if measured semantics or performance require it. Implement against the newer backend head, which has additional conversation-version handling in the same builder.

**Superseded open questions:** AutoQM uncovered-rule behavior, overlap, and filtered completion were settled November 4; see the decision above. The full plan recommends grouped responses, with the final API contract and performance review still pending.

Source detail: [core design](omni-usecase-scorecard-eng-plan-draft.md), [Performance Insights query study](../sessions/2026-10-08/codex-performance-insights-clickhouse-feasibility.md), [QM Evaluations trace](../sessions/2026-10-08/codex-qm-report-evaluations-query-fit.md), [QM task model](../sessions/2026-10-08/codex-qm-task-template-usecase-relationship.md), [AutoQM mapping](../sessions/2026-10-08/codex-multi-usecase-autoqm-rule-mapping.md).
