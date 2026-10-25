# Direct template readers and core queryability boundary

- Date: 2026-10-07
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only)
- Question: Whether complex analytics SQL must migrate to a template query layer for criterion applicability.

## Group calibration path

`insights-server/internal/analyticsimpl/retrieve_group_calibration_stats.go:findTasks` joins `director.scorecard_templates`, selects `st.template`, and parses that JSON with `scoring.ParseScorecardTemplateStructure` after the query. It iterates the parsed criterion tree in Go in `aggregateGroupCalibrationCriteriaStats` and `calculateGroupCaliScorecardConsistencyScores`. `findScores` joins scorecards, scores, and templates in SQL but reads criterion score rows rather than applying a criterion JSON predicate there.

Under the agreed model, inapplicable criteria create no score rows. The current aggregation still iterates all criteria and treats missing score rows as empty responses, so it must filter by the task/scorecard's concrete use case. A normalized parser of historical templates needs the same stored revision's template `usecase_ids`, which `findTasks` currently does not select. These changes appear possible without rewriting the complex SQL joins into an application query layer. This is a source-level conclusion for this path, not proof for all analytics readers.

## Draft boundary and correction

Keep the authoritative structure in template JSON and expose a normalized Go model. If a consumer joins a template row then parses its JSON in Go, it can adopt normalization at that boundary. Other readers may need different local changes. This source trace answers part of Xuanyu's earlier blast-radius concern, but it was premature to turn SQL-side lookup or an extra derived table/view into the next core design question. Keep this as a high-level impact note and defer detailed query design.
