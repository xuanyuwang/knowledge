# Codex Session: Empty Scorecards Workflow/API Analysis

**Date:** 2026-06-25  
**Project:** scorecard-template  
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`  
**Related source repo:** `/Users/xuanyu.wang/repos/director`  
**Source commits:** `go-servers` `eb72d7bb87a43052fffed65d3a9f518c889d8f7d`; `director` `301610c1edef5708c0e1c71d4f3bd81e8aed6888`

## Prompt

Investigate empty scorecards across behavior frames: evaluation, appeal, group calibration, analytics, and the newly recognized AutoQA workflow. Produce a document explaining what an empty scorecard is, how each workflow/API uses it, and collect code/documentation evidence with clickable links.

## Actions

- Read `workflow/ai-operating-model.md`, `scorecard-template/project.yaml`, `scorecard-template/README.md`, and existing workflow/lifecycle deliverables.
- Inspected `go-servers` scorecard creation, update, submit, AutoQA, backfill, ClickHouse projection, QA stats, QA conversations, appeal stats, and group calibration stats code.
- Inspected `director` scorecard save path and analytics API wrappers.
- Added `scorecard-template/deliverables/empty-scorecards-workflow-and-api-analysis.md`.
- Updated project log, README, and project metadata.

## Key Findings

- Empty scorecard means a `director.scorecards` row with no usable `director.scores` rows.
- `CreateScorecardAndScoresInDB` does not require non-empty score input.
- `ComputeScores` intentionally accepts empty input and leaves the overall score unset.
- `RemoveEmptyScorecardScores` turns blank placeholder score rows into zero-score input before create/update.
- AutoQA live trigger and backfill bypass the coaching `CreateScorecard` API and write through shared scoring persistence.
- QA analytics are score-driven: empty scorecards are normally filtered out by projection or by the `scorecard_score JOIN filtered_scorecard` query shape.
- Appeal and group calibration stats are also score-driven; empty artifacts generally do not become criterion/evaluated-scorecard facts.

## Follow-Ups

- Decide whether empty scorecards are valid drafts or invalid persisted records.
- If invalid, enforce the invariant after `ComputeScores` for all creation paths, including AutoQA.
- If valid, make the state visible in operational APIs/UI and explicitly document that analytics APIs count score-bearing scorecards only.
