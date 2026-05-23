# Decision Record - Separate Permission for Post-Submit Editing

**Date:** 2026-05-19  
**Status:** Proposed

## Context

CONVI-6862 needs an exception path for users who may still edit a scorecard after it has been submitted.

The current template permission model already has separate buckets for:

- `template_editors`
- `scorecard_viewers`
- `scorecard_graders`
- `scorecard_appealers`

## Decision

Do not reuse `scorecardGraders` for post-submit editing.

Instead, introduce a separate permission concept for post-submit editing, for example:

- `submittedScorecardEditors`
- `scorecardPostSubmitEditors`

## Reasoning

- Reusing `scorecardGraders` would silently expand authority for every existing template.
- Pre-submit grading and post-submit override are different product semantics.
- A dedicated field gives a safer migration path and clearer audit meaning.
- It keeps future UI and backend logic readable.

## Consequences

- Proto, backend, and frontend permission models will likely need schema changes.
- Template builder access UI will need a new field if the feature is made configurable per template.
- Existing templates can default to empty post-submit editors and preserve current authority boundaries.
