# Scorecard Template System Reference

**Created:** 2026-05-07  
**Updated:** 2026-05-07  
**Status:** Canonical distilled reference  
**Last validated:** 2026-05-07

## Purpose

This document captures the distilled understanding of scorecard templates as a **system**, not just as a single table or UI screen.

It is intended to serve three audiences:

- future me, when I need the scorecard mental model without replaying weeks of investigation
- teammates and AI tools that need a reliable foundation for scorecard-related work
- staff-level artifact consumers who want to see the depth of system understanding behind coaching work

## Executive Summary

Scorecard templates are the configuration backbone of the coaching and QA system. They define:

- what is scored
- how it is scored
- how AutoQA outcomes map into human-readable criterion selections
- how scores are normalized and aggregated
- how scored data becomes analytics in ClickHouse

The critical architectural point is:

- **Postgres is authoritative for scorecard templates, scorecards, and scores**
- **ClickHouse is a projection used for analytics and reporting**

The scorecard template system is not only about authoring criteria. It spans:

1. Director template builder authoring
2. template persistence and revisioning
3. runtime scoring and AutoQA mapping
4. score storage and historical projection
5. analytics consumption and display

## What a Scorecard Template Actually Is

A scorecard template is a reusable blueprint for evaluating agent performance. In practice, a template combines:

- hierarchical structure: chapters, criteria, branches
- score semantics: option values, score mappings, weights, auto-fail rules
- automation semantics: AutoQA triggers and mappings
- operational semantics: audience, permissions, status, use-case scope, template type

The authoritative template record lives in `director.scorecard_templates`, with the actual structure stored as JSON in the `template` field. The key persistent identifiers are:

- `resource_id`: template identity
- `revision`: immutable version of the template structure

This revision boundary matters because downstream score computation and replay logic depend on the exact template revision used when a scorecard was created or rescored.

## Core Mental Model

### 1. Templates are configuration, not just UI state

The same template structure drives:

- the builder UI in Director
- grader input semantics
- AutoQA mapping behavior
- backend score calculation
- analytics aggregation

This is why scorecard-template bugs often appear far away from the place where the template was edited.

### 2. Option wiring is the most important invariant

For scorable criteria, three structures must stay logically aligned:

- `settings.options[]`
- `settings.scores[]`
- `auto_qa.*`

The important concept is the **wiring key**:

- `options[].value` identifies an option
- `scores[].value` points at that option and gives it a score
- `auto_qa.detected`, `not_detected`, and sometimes `not_applicable` point at that same logical option

When this wiring drifts, the system can still appear valid structurally while producing wrong labels, wrong scores, or wrong AutoQA behavior.

### 3. Template semantics differ from presentation semantics

The same criterion can be represented differently across stages:

- persisted template JSON
- form state in the builder
- grader UI options
- score rows stored in Postgres
- analytics rows in ClickHouse

Some bugs come from assuming these stages use the same representation. They do not.

## Technical Core: The Three Coupled Arrays

For scorable labeled criteria, the deepest invariant is the relationship between:

- `settings.options[]`
- `settings.scores[]`
- `auto_qa`

### Persisted/API shape

In persisted template form, these arrays are intended to line up by **option identity**, not by display order semantics:

```text
settings.options[i].value   = option identity key
settings.scores[i].value    = same identity key
settings.scores[i].score    = actual numeric score
auto_qa.detected            = option identity key or option index, depending on stage
auto_qa.not_detected        = option identity key or option index, depending on stage
auto_qa.not_applicable      = option index or null in newer flows
```

For a standard persisted labeled criterion, the common healthy shape is:

```text
options = [
  { label: "Yes", value: 0 },
  { label: "No",  value: 1 },
]

scores = [
  { value: 0, score: 1 },
  { value: 1, score: 0 },
]

auto_qa = {
  detected: 0,
  not_detected: 1,
}
```

The important distinction is:

- `options[].value` is the lookup key
- `scores[].score` is the actual score meaning

When people casually say “option value”, they often accidentally mean score. That confusion is the root of several real bugs.

## Technical Core: What Decoupled Mode Actually Means

“Decoupled mode” is tricky because it changes the meaning of `option.value` **in form state**, but not in the persisted canonical model.

### Persisted/API meaning

In persisted/API form:

- `option.value` is the option identity key, usually sequential after save
- `scores[].score` is the real numeric score

### Form-state meaning after decoupled load

In the template builder, after the decoupled load transform:

- `option.value` is rewritten to the **score itself**
- `scores[].value` still keeps the original identity key

That means this relation, which is usually true in the DB, becomes false in form state:

```text
options[i].value === scores[i].value
```

This is not accidental. It is an intentional edit-time optimization.

### Exact decoupled load transform

Conceptually:

```text
API:
options = [{ label: "Yes", value: 0 }, { label: "No", value: 1 }]
scores  = [{ value: 0, score: 1 },    { value: 1, score: 0 }]

Form after decoupled load:
options = [{ label: "Yes", value: 1 }, { label: "No", value: 0 }]
scores  = [{ value: 0, score: 1 },     { value: 1, score: 0 }]
```

So in form state:

- `option.value` is now the displayed/editable score
- `scores[].score` is still the authoritative score field
- `scores[].value` still carries the original option identity

This design exists to avoid recomputing mappings on every keystroke while editing options and scores in the builder.

### Save transform restores canonical shape

On save, the builder reindexes options back to sequential identity keys and preserves the actual score in `scores[i].score`.

Conceptually:

```text
for each option at index i:
  newOption.value = i
  newScore.value = i
  newScore.score = scores[i]?.score ?? option.value
```

This is the inverse of the load-time decoupling.

### Practical rule

When debugging:

- in persisted/API data, treat `option.value` as identity
- in decoupled builder form state, treat `option.value` as edit-time score

If you fail to distinguish those two stages, you will misread the bug.

## System Boundaries

### Authoring Layer

The authoring surface lives in Director’s template builder. The builder lets admins define:

- criterion type
- labels and scores
- N/A behavior
- AutoQA mappings
- audience and access

This layer is where several sharp edges emerge:

- legacy vs decoupled score handling
- `showNA` versus real N/A option semantics
- type-specific transforms on save
- AutoQA defaults and synchronization behavior

### Persistence Layer

Templates are stored in PostgreSQL. The important records are:

- `director.scorecard_templates`
- template JSON revision
- associated audience, permissions, use cases, and QA config

The template JSON is the source material for backend parsing and scoring.

### Scoring Layer

The scoring layer turns template semantics plus grader or AutoQA input into criterion percentages and aggregated scores. It depends on:

- criterion type
- score mapping presence
- weights
- N/A handling
- auto-fail rules
- branch conditions

This is where “template meaning” becomes “actual score math”.

### Projection and Analytics Layer

Scorecards and scores are projected into ClickHouse for analytics. This is where scorecard-template semantics influence:

- QA score stats
- scorecard-level aggregations
- criterion-level analytics
- leaderboard and coaching insights

This layer is downstream and lossy relative to Postgres. It is optimized for analytics, not authoritative reconstruction.

## Canonical Data Model

### Template hierarchy

The durable conceptual hierarchy is:

- template
- chapters
- criteria
- branches
- child criteria

V2 templates are hierarchical and are the practical norm.

### Scorable criterion types

The scoring-relevant criterion types are:

- `numeric-radios`
- `labeled-radios`
- `dropdown-numeric-values`

The non-scorable types such as `sentence`, `date`, and `user` matter for the UI and workflow, but not for QA score computation.

### Template-level fields that matter operationally

Beyond the structure JSON, these fields shape behavior:

- `type`: conversation vs process template
- `status`: active/inactive/archived
- `usecase_ids`
- `audience`
- `permissions`
- `qa_task_config`
- `qa_score_config`

These fields are especially important when duplicating or migrating templates across use cases.

## End-to-End Flow

### 1. Builder authoring

An admin edits a template in Director. The builder mutates form state, not the final API shape. During this phase:

- options and scores may be decoupled in form state
- AutoQA dropdowns derive from current options
- N/A may be represented as `showNA`, as an actual N/A option, or both depending on the code path and feature state

### 2. Save transform

The builder transforms form state into API/persisted shape. This step is not a trivial pass-through. It performs type-specific normalization such as:

- reindexing options
- persisting score arrays
- preserving or stripping settings based on criterion type

This is a major source of hidden semantics. A form model that “looks right” may save into a different canonical structure.

### 2a. Legacy migration during load

The builder also has a legacy migration path for templates that have options but no proper scores array. In that path:

- options are renormalized to index-like values
- scores are synthesized from original option values
- `auto_qa.detected` and `auto_qa.not_detected` are remapped through `valueToIndexMap`

One nuance that matters: a previously identified latent bug is that `auto_qa.not_applicable` is not remapped in the same path.

### 3. Runtime scoring

When a scorecard is created or updated, the system combines:

- template revision
- criterion inputs
- AutoQA outcome mapping
- score mapping rules

This produces normalized criterion percentages and weighted aggregate scores.

### 4. Score storage

The system stores authoritative scorecard and score records in Postgres. Historically, `historic.scorecard_scores` also matters as an intermediate or validation surface in some flows.

### 5. ClickHouse projection

Analytics and reporting consume projected scorecard/score rows in ClickHouse. This projection is useful but not authoritative; it can be stale, incomplete, or structurally limited relative to Postgres.

## Technical Scoring Model

### Primary entry point

The important backend entry point is `ComputeScores()` in `scorecard_calculator.go`.

At a high level:

1. parse/extract scoreable criteria
2. group raw scores by criterion
3. skip excluded criteria
4. compute criterion percentage scores
5. roll percentages upward through parent chapters
6. compute weighted final percentages

### Criterion percentage scoring

The core percentage logic lives in `ComputeCriterionPercentageScore()`.

The major branches are:

- standard single-select scoring
- multi-select scoring
- per-message scoring
- outcome-specific scoring

### Standard single-select scoring

Conceptually:

1. resolve the raw selected value
2. if there is a value→score mapping, map the raw value to a configured score
3. determine the max score
4. compute:

```text
percentage = scoreValue / maxScore
```

Example:

```text
scores = [
  { value: 0, score: 0 },
  { value: 1, score: 10 },
]

selected value = 1
scoreValue = 10
maxScore = 10
percentage = 10 / 10 = 1.0
```

### Weight aggregation

At aggregation time, the system uses weighted accumulation:

```text
summary.Total  += percentage * weight
summary.Weight += weight
finalPercentage = summary.Total / summary.Weight
```

The implementation rounds the final percentage for output, but the conceptual rule is weighted average over criterion percentages.

### Chapter aggregation

A chapter aggregates **all descendant criterion scores**, not only direct children.

That means a nested criterion affects:

- itself
- its direct parent chapter
- every ancestor chapter above it
- the overall template score

### Multi-select scoring

In multi-select mode, the system uses a different formula. The selected options do not simply average themselves. Instead, the selected score is normalized against the sum of all possible scores, and the criterion weight is distributed across the selected entries.

Conceptually:

```text
sumAllScores = Σ(all configured option scores)
perSelectedPercentage = (numSelections * selectedScore) / sumAllScores
perSelectedWeight = criterionWeight / numSelections
```

This matters because multi-select behavior is not equivalent to “treat each selected option as an independent normal criterion”.

### Exclusion rules

Two distinct exclusion concepts matter:

- `excludeFromQAScores`: static template-level exclusion
- `NotApplicable`: dynamic per-score exclusion

These are not the same thing.

- `excludeFromQAScores` means the criterion is structurally excluded from QA score stats
- `NotApplicable` means this specific scoring event contributes no score and no weight

## Technical N/A Model

N/A is one of the most subtle parts of the system because it spans modeling, UI, scoring, and analytics.

### Legacy N/A semantics

Historically:

- `showNA: true` exposes an N/A path to the grader
- when N/A is selected, the resulting score has `NotApplicable = true`
- the criterion is skipped entirely in scoring

In the legacy path, this effectively means:

```text
N/A => no numeric score => no percentage => no weight contribution
```

### Scored N/A semantics

The later N/A work explores a richer model where N/A can exist as an actual option carrying score semantics:

- a real option with `isNA: true`
- a matching entry in `scores[]`
- possibly an AutoQA `not_applicable` mapping

In that model, the system can distinguish:

- legacy N/A: `not_applicable = true`, no numeric value
- scored N/A: `not_applicable = true`, but also a numeric value associated with an explicit N/A option

That is why N/A is not just a UX checkbox. It changes what score data exists and how it is interpreted.

### Important N/A invariant

For the richer model to work safely:

- the N/A option must be identifiable as N/A, not just by its label
- label-based detection is not sufficient because labels are not stable enough for semantics

This is why the `isNA` idea matters.

## Technical AutoQA Model

AutoQA has two materially different mapping modes:

- behavior done/not done
- number-of-occurrences / metadata-bucket mapping

### Behavior done/not done

In this mode:

- `auto_qa.detected` points to the option to assign when behavior is detected
- `auto_qa.not_detected` points to the option to assign when it is not detected

Conceptually:

```text
DETECTED     -> option identity -> score lookup -> percentage
NOT_DETECTED -> option identity -> score lookup -> percentage
```

### Number-of-occurrences mode

In this mode, `auto_qa.options[]` provides bins or exact mappings from evidence counts / metadata values to criterion option values.

This is important because:

- outcome mode and DND mode do not share the same save semantics
- some save paths renormalize and some preserve literal values

That difference is part of why outcome-related rendering bugs can survive persistence.

## Technical Rendering and the Reversed-Scorecard Bug

The `convi-6709` investigation is one of the highest-value technical lessons in this area because it proves that scorecard correctness depends on display-stage semantics too.

### The core mismatch

In the problematic path:

- auto-scoring stored `option.value` as the persisted `numeric_value`
- read-only `CriterionInputDisplay` in decoupled mode built display options using **array index**

That means the field value carried one meaning, while the renderer interpreted it with another.

### Simplified example

Persisted semantics:

```text
settings.options = [
  { label: "Not Resolved", value: 1 },
  { label: "Resolved",     value: 0 },
]

stored numeric_value = 0
```

Meaning of stored `0`:

```text
option.value = 0 => "Resolved"
```

But the read-only renderer, in decoupled mode, effectively rebuilt options as:

```text
index 0 => first option => "Not Resolved"
index 1 => second option => "Resolved"
```

So the same `"0"` became:

- persisted meaning: option value 0 => "Resolved"
- rendered meaning: array index 0 => "Not Resolved"

That is why the UI showed reversed labels even though the stored data and filtering logic were correct.

### Lesson

Any scorecard-template change that touches:

- option ordering
- outcome transforms
- decoupled rendering
- AutoQA value persistence

must be validated in both edit mode and read-only consumption paths.

## The Most Important Semantic Rules

### Option values are identifiers, not necessarily the displayed score

This is the single most important conceptual rule.

- An option’s `value` is primarily the wiring key.
- A score is the numeric meaning attached to that option.
- In some flows, the UI temporarily treats option values as score-like.
- In other flows, read-only rendering or AutoQA still assumes persisted option identity semantics.

Conflating these meanings causes subtle bugs.

### N/A is a full-system concern, not a checkbox

N/A behavior spans:

- template modeling
- builder UX
- AutoQA mapping
- scoring semantics
- analytics inclusion/exclusion
- read-only display

Historically `showNA` looked like a small presentation toggle, but the later investigations show that N/A becomes a system-wide modeling problem once it can carry score semantics or AutoQA mappings.

### AutoQA references template semantics, not just labels

AutoQA does not care about human-readable labels. It maps detected outcomes to option identity. If options are reindexed, reordered, or represented differently between builder, API, and read-only display, AutoQA-linked behavior can drift even when the template still “looks right” in the UI.

### ClickHouse is downstream and imperfect

Any scorecard-template conclusion that depends only on ClickHouse should be treated as provisional. Projection gaps, replay limits, and stale rows can make ClickHouse disagree with Postgres even when the template logic itself is correct.

## Critical System Cases

### Conversation vs process scorecards

This distinction is not cosmetic.

- conversation scorecards fit the common conversation-based reindex and analytics flows
- process scorecards do not always map cleanly to those flows

The main implication is operational:

- the write layer can support process scorecards
- the standard recovery and backfill workflows do not fully support them

This means a template can be valid while its downstream analytics visibility is still operationally fragile.

### Appeal scorecards

Appeal scorecards are another case where template or scorecard existence does not mean analytics inclusion. Reindex and cleanup logic may explicitly exclude them from ClickHouse projections.

### Template duplication across use cases

Scorecard template duplication is not a pure structural copy. Some fields are safe to copy, but these are use-case-sensitive:

- audience
- use case IDs
- QA task config
- AutoQA trigger references

This means “duplicate template” is partly a product feature and partly a semantic migration problem.

## Known Sharp Edges and Failure Modes

### 1. Read-only display can disagree with scoring semantics

The reversed-scorecard investigation showed a concrete example: read-only display interpreted decoupled scoring by array index while auto-scored values still reflected persisted option identity. The result was a wrong label even though the underlying score data was correct.

This is a high-value lesson: **display code is part of scorecard correctness**.

### 2. N/A can be modeled inconsistently across paths

The N/A work exposed multiple overlapping models:

- legacy `showNA`
- explicit `isNA` option
- `not_applicable` score flag
- nullable N/A score
- AutoQA `not_applicable` mapping

If a feature touches N/A, it must be checked across authoring, save, scoring, and display paths together.

### 3. Save/load transforms hide semantics

The builder’s load and save transforms deliberately change the shape of options and scores. Any investigation that looks only at stored template JSON or only at React form state is incomplete.

### 3a. Decoupled mode is intentional, not accidental

The fact that `option.value` becomes “score-like” in builder form state is an intentional design for edit-time performance, not an incidental corruption. Refactors must preserve that distinction unless they also replace the performance strategy.

### 4. Process scorecards expose projection gaps

The process-scorecard backfill work showed that operational support for process templates is incomplete in standard recovery paths. That is a system-level limitation, not just an implementation bug in one script.

### 5. Projection repair can be logically incomplete

The PG→CH sync investigation highlighted the broader class of failures:

- missing scorecard rows
- missing score rows
- stale projection state
- write-order races
- scorecards invisible to conversation-based recovery

This matters because template understanding must include how template-driven data is projected and repaired.

### 6. Array order and option identity are different axes

A healthy system can preserve logical option identity while changing array order. A renderer that assumes `array index === option identity` will eventually break.

## Practical Source of Truth Hierarchy

When trying to understand a scorecard-template behavior, trust sources in this order:

1. persisted Postgres template revision and score data
2. backend scoring and mapping logic
3. builder save/load transforms
4. ClickHouse projection
5. read-only UI rendering

The lower levels are still useful, but they are more likely to contain adapted or lossy representations.

## Recommended Reading Order

If someone new needs to understand this domain quickly, read in this order:

1. this document
2. `template-structure.md`
3. `criterion-options.md`
4. `fe-template-builder.md`
5. `nascore/options-scores-lifecycle.md`
6. `convi-6709-reversed-scorecard/README.md`
7. `backfill-scorecards/README.md`
8. `pg-ch-scorecard-sync-investigation/scorecard-specific-constraints.md`

## Technical Reading Order

If the goal is implementation or debugging, read in this order instead:

1. this document
2. `scorecard-template/template-structure.md`
3. `scorecard-template/criterion-options.md`
4. `nascore/options-scores-lifecycle.md`
5. `scorecard-template/fe-template-builder.md`
6. `convi-6709-reversed-scorecard/exact-flow.md`
7. `scorecard-template/na-score-design.md`

## Cross-Project Supporting Artifacts

These documents materially informed this distilled reference:

- `scorecard-template/template-structure.md`
- `scorecard-template/criterion-options.md`
- `scorecard-template/fe-template-builder.md`
- `scorecard-template/na-score-design.md`
- `nascore/README.md`
- `nascore/options-scores-lifecycle.md`
- `convi-6709-reversed-scorecard/README.md`
- `convi-6709-reversed-scorecard/exact-flow.md`
- `backfill-scorecards/README.md`
- `pg-ch-scorecard-sync-investigation/scorecard-specific-constraints.md`
- `duplicate-template-across-usecase/investigation.md`

## What This Distillation Is Good For

This document is intended to support:

- scorecard-template feature work
- debugging score correctness issues
- AutoQA/template-builder changes
- analytics and projection investigations
- onboarding teammates or AI tools to the coaching foundation
- demonstrating system-level contribution and architectural understanding

## Open Questions

- What should be treated as the canonical version field for scorecard projection repair across all write paths?
- Which scorecard attributes are truly monotonic versus mutable after initial scorecard creation?
- What is the clean long-term model for N/A now that legacy `showNA` and richer N/A semantics coexist?
- How much of the builder’s save/load normalization should be made explicit in shared utility layers rather than hidden in component-local transforms?

## Suggested Follow-Up Deliverables

The next high-value docs to create under `deliverables/` are:

- `scorecard-template-source-map.md` - exact code paths, tables, protos, and flows
- `scorecard-template-failure-modes.md` - a concentrated debugging guide
- `scorecard-template-change-checklist.md` - what to validate before touching templates, AutoQA, or score rendering
