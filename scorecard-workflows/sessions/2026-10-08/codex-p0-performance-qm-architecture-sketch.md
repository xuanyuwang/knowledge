# P0 Performance Insights and QM architecture sketch

- Date: 2026-10-08
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`, `main` checkout, read-only; related repos: `/Users/xuanyu.wang/repos/director` and `/Users/xuanyu.wang/repos/cresta-proto`.
- Product source: [PRD: Omni Use Case QM & Coaching](https://docs.google.com/document/d/1iEYDUg7NrYo5NjeTcr9nHyNqEbv_sWHrDHJeAkFJS9c/edit?tab=t.0#heading=h.i4n1p4ads1o), refreshed via Glean on 2026-10-08. The document labels these requirements P0; Xuanyu's “B0” was interpreted as P0.
- Output: [concise private architecture sketch](../../deliverables/phase-1-p0-performance-insights-qm-sketch.md).
- Evidence reused: current core engineering draft, Performance Insights ClickHouse feasibility, QM Evaluations PostgreSQL trace, cross-page QM fit, AutoQM rule mapping, and the QM task scope decision. No new database access, benchmark, or source mutation was performed.
- Review point: the sketch covers common template/evaluation/rule/task/filter foundations plus Performance Insights, QM Task Home, and all three QM Report tabs. It marks the AutoQM gap, filtered completion-rate denominator, and breakdown API shape as unresolved rather than inventing decisions.

## Expanded assumption check (Xuanyu follow-up)

- Verified template `usecase_ids` on both revision-bearing tables and typed Go parsing of `template JSONB`. Criterion `usecase_ids` is the proposed JSON field, not a present column. NONE/ALL/SOME encoding and subset rules come from earlier user decisions; no criterion-definition table is required for this phase.
- Verified that Opera policies/behaviors can cover multiple use cases, subject to an authoring flag. The AutoQM scorer currently does not check that scope and can treat no annotations as N/A, so a warning must be paired with runtime eligibility handling. Global/specific overlap and no-rule fallback remain decisions.
- Verified ClickHouse score/scorecard use-case columns, existing repeated-filter parsing, and the missing use-case group-by. The updated plan sketches a use-case projection from the filtered latest scorecard into both score-query branches. The local `go-servers` main checkout is `435cb9047a`; `origin/main` is `975adf2eb3` and changes `retrieve_qa_score_stats_clickhouse.go` conversation-version collapse but not the proposed use-case grouping boundary. Implement on the current head, preserving that newer behavior. Director local `f47845f4f7` is behind `origin/main` `a0952b6862`; the changed relevant frontend files are outside the score/filter request seams reviewed here.
- Verified `tasks.usecase_id` and task proto are singular; QM UI copies the first template use case, while backend create/update do not enforce the template relationship. Proposed no new task-scope column: resolve current scope from latest linked template and retire singular-column filtering for QM. This is a design choice pending legacy-row audit and PostgreSQL query benchmark, not a deployed implementation.
- Verified QM Evaluations has concrete `scorecards.usecase_id` and score rows joinable to scorecards. Manual-QA queries already filter lists; two main QM task stats branches do not. The plan includes the missing predicates and use-case group-by shape. This proves source-level query feasibility, not actual performance or a shared-template staging population.
- Added a future PI trace contract and internal query-shape decomposition recommendation. A public QA API split is deferred because combinations/callers and current dedup semantics require evidence first.

No live database connection or credential was used for this follow-up. No product-source code was changed.

## Readability revision

Xuanyu requested the plan to begin with the core data model, then explain template config, Performance Insights, QM task, QM Task Home, and QM Report in that order. Reorganized the same deliverable accordingly. Added a model-to-score-to-report Mermaid diagram, a Task Home selection-flow diagram, and tables for model changes, page requirements, QM Report tabs, and rollout gates. Kept the two illustrative query shapes beside the pages they support. No new factual investigation or database access was needed.

Xuanyu then requested concrete storage fragments at the core model. Checked the schema and current V2 parser/test JSON; added an excerpt of the existing template table and a proposed V2-shaped JSON example with one A+B and one B-only criterion. Marked the new criterion field and JSON-version choice as proposed rather than implemented.

Xuanyu clarified that the opening diagram hid the actual model change. Replaced it with a short explanation followed immediately by the existing PostgreSQL column excerpt, proposed criterion JSON, and illustrative Go parser/typed-model fields. The Go parser example uses `json.RawMessage` so legacy omission stays distinguishable from explicit JSON `null`; the typed `*[]string` field represents NONE, ALL, and SOME after parsing. Kept the score-row consequence next to the examples and left downstream page sections in their established order. No source code or database was changed.
