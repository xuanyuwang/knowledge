# Codex Session: Workflow Taxonomy

**Date:** 2026-06-11
**Tool:** Codex
**Source repo:** /Users/xuanyu.wang/repos/go-servers
**Knowledge repo:** /Users/xuanyu.wang/repos/knowledge
**Project:** scorecard-template

## Prompt

Reorganize the scorecard/template working reference around four concepts:

- template
- scorecard
- lifecycle
- workflow

The key question was how to name the two higher-level categories when template and scorecard are concrete artifacts, while lifecycle and workflow are more abstract usage/reasoning concepts.

## Working Decision

Use two categories:

- **Domain Artifacts**: the things that exist as product and system objects.
  - Template
  - Scorecard
- **Behavioral Frames**: the lenses used to interpret those artifacts.
  - Lifecycle
  - Workflow

Short rule:

- artifacts = what exists
- frames = how to reason about what exists

## Changes Made

- Updated `deliverables/scorecard-template-domain-skeleton.md` to make the two-category model the top-level organizing model.
- Reframed scorecard and template as domain artifacts.
- Added lifecycle and workflow as behavioral frames.
- Added workflow-specific scorecard roles for performance evaluation, calibration, appeal, analytics/reporting, and repair/backfill.
- Added `deliverables/workflow-map.md` as the dedicated home for workflow role semantics.
- Updated `README.md` with the new organizing model, reading order, and log history.
- Updated `project.yaml` so the workflow map is part of the machine-readable key docs.
- Updated `deliverables/scorecard-template-concept-map.md` and `deliverables/business-rules-catalog.md` so workflow is visible in the populated concept map and future rule capture.

## Notes

The important modeling move is that workflow should not replace the base definition of scorecard or template. It should name the role-specific semantics that apply when those artifacts are used in a business process.

This prevents workflow-specific details such as appeal mutability, calibration benchmark ownership, or analytics projection semantics from being treated as globally true scorecard rules.

## Follow-Ups

- When future tickets clarify concrete permission or mutability behavior, record both the lifecycle stage and the workflow.
