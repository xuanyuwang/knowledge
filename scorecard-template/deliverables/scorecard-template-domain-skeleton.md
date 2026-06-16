# Scorecard / Template Domain Skeleton

**Created:** 2026-05-17  
**Status:** Draft starting point  
**Purpose:** Minimal framework for organizing the scorecard/template domain before attempting exhaustive rule capture

## How To Use This Document

This is not the full reference. It is the starting scaffold.

When new details appear in ticket work, place them into one of these sections instead of collecting isolated notes. Unknowns are part of the structure and should be recorded explicitly.

## 1. Organizing Model

The scorecard/template domain has two kinds of concepts.

### Domain Artifacts

Domain artifacts are the things that exist as product and system objects.

- **Template**: the reusable definition/configuration.
- **Scorecard**: the concrete runtime record created from template semantics.

These are the nouns of the domain. They have identity, storage, ownership, and system representations.

### Behavioral Frames

Behavioral frames are the lenses used to interpret the artifacts.

- **Lifecycle**: how an artifact changes over time.
- **Workflow**: why and how an artifact is used in a business process.

These are not separate persisted objects by default. They explain which rules apply to the artifacts in a given situation.

The short version:

- artifacts = what exists
- frames = how to reason about what exists

## 2. Core Concepts

### Domain Artifacts

#### Scorecard

Working definition:

A scorecard is the runtime evaluation artifact created from template semantics in a concrete workflow.

Its exact role depends on the workflow:

- Performance evaluation: records the evaluation outcome for a conversation, agent behavior, process, or coaching outcome.
- Calibration: may act as the benchmark answer set or as a participant response being compared against the benchmark.
- Appeal: may act as the original evaluated record, the requested correction, or the resolved final decision for an appeal round.

Open questions:

- When exactly is a scorecard instantiated?
- Which fields are copied from the template versus referenced indirectly?
- Which parts of scorecard state are mutable after creation or submission?

#### Template

Working definition:

A template is the reusable definition that describes what can be evaluated, how it should be evaluated, and the operational rules for where the resulting scorecards can be used.

Known characteristics:

- stored authoritatively in Postgres
- versioned by revision
  - a scorecard is always associated with a specific revision
  - scoring semantics should be interpreted against the effective revision
  - some operational metadata, such as permissions, may follow latest-template behavior rather than historical revision behavior
- used by builder UI, scoring logic, and analytics projection flows

Open questions:

- Which template changes are safe for future scorecards but must not affect historical scorecards?
- Which template metadata is operational versus scoring-related?

### Behavioral Frames

#### Lifecycle

Working definition:

- the ordered state and transition model for an artifact over time

Why it matters:

- lifecycle makes mutability, revisioning, submission, persistence, projection, and historical interpretation easier to place
- rules become easier to reason about when attached to transitions instead of floating as isolated exceptions

Open questions:

- Which transitions are customer-visible?
- Which transitions create immutable historical meaning?
- Which lifecycle changes affect only future artifacts versus existing artifacts?

#### Workflow

Working definition:

- the business process in which an artifact is used, giving that artifact its immediate role, actor model, and rule set

Why it matters:

- workflow explains why the same scorecard-shaped artifact can behave differently in evaluation, calibration, appeal, analytics, or repair contexts
- workflow is the right home for details such as who initiates, who responds, who can edit, who can view, and what decision is being made

Open questions:

- Which workflows are first-class product concepts versus implementation reuse of the same artifact?
- Which workflows share the same lifecycle and which introduce distinct transitions?
- Which workflow-specific permissions override or refine template-level permissions?

## 3. Supporting Concepts

### Criterion

Working definition:

- the evaluatable unit inside a template; may carry type-specific scoring and AutoQA behavior

Open questions:

- Which criterion types exist and which are semantically distinct versus just UI variants?
- Which criterion types support N/A, branching, or AutoQA mapping?

### Option

Working definition:

- the selectable outcome for a criterion, with identity and presentation semantics that may differ by system stage

Known sharp edge:

- option identity and option score can diverge across persisted data and builder form state

### Score

Working definition:

- the numeric meaning attached to a selected option or criterion outcome

Known sharp edge:

- “option value” and “score value” are not consistently the same concept across representations

### Assignment

Working definition:

- the mechanism that determines who can use, see, or apply a template or scorecard in a given business context

Open questions:

- What entities can assignments target: users, teams, queues, use cases, audiences?
- Which behaviors are enforced in UI only versus backend only?

### Version / Revision

Working definition:

- the boundary that separates one immutable template shape from another

Known importance:

- revision matters for replay, score computation, and historical interpretation

### Evaluation Context

Working definition:

- the surrounding business and system context in which a template or scorecard is used, such as coaching workflow, QA flow, AutoQA, process scorecards, or analytics consumption

Relationship to workflow:

- evaluation context is the concrete runtime setting
- workflow is the business process pattern that gives that setting meaning

Open questions:

- Which contexts share the same core semantics and which introduce special cases?

## 4. Relationship Map

Use this section to record the most important dependencies, not every edge in the system.

### Template -> Criterion

- A template contains criteria and related structure.
- Criteria inherit meaning from both local settings and template-level context.

### Criterion -> Option / Score

- Options define selectable outcomes.
- Scores define numeric interpretation.
- AutoQA behavior is coupled to this mapping.

### Template -> Revision

- A template identity can have multiple revisions.
- Runtime behavior often depends on the exact revision, not only on the template resource identity.

### Template -> Scorecard

- A scorecard depends on a template definition at creation or scoring time.
- Historical correctness likely depends on preserving the effective template semantics used at that time.

### Scorecard -> Analytics Projection

- Scorecard data is authored and stored authoritatively in Postgres, then projected into ClickHouse for analytics.

### Template / Scorecard -> Evaluation Context

- The same core concepts appear in multiple product contexts, but not always with identical semantics.

### Artifact -> Workflow

- A template or scorecard can participate in multiple workflows.
- Workflow determines the artifact role, actor model, permission details, and meaningful state transitions.
- A workflow should not silently redefine the artifact's base semantics; it should name the additional role-specific rules.

## 5. Main Lifecycle

### Template lifecycle

1. Template created
2. Template edited in builder
3. Template saved and normalized
4. Template revision persisted
5. Template assigned, exposed, or used in a business context

Questions to resolve:

- Is there a publish/activate state distinct from save?
- What is the lifecycle of duplication or cloning?
- Which edits create a new revision versus mutate metadata only?

### Scorecard lifecycle

1. Scorecard instantiated from a template or evaluation context
2. Scorecard populated or scored by human or automated flow
3. Scorecard updated, submitted, or finalized
4. Scorecard written to authoritative storage
5. Scorecard projected to analytics storage
6. Historical scorecard viewed, queried, or compared

Questions to resolve:

- Which actions are synchronous versus async?
- Which transitions are customer-visible?
- What happens when template definitions change after scorecards already exist?

## 6. Workflow Map

This section names the main workflows that should receive their own rule details as the domain reference grows.

### Performance evaluation workflow

- Primary scorecard role: evaluation result
- Primary template role: evaluation definition
- Main concerns: grading, scoring, submission, permissions, analytics relevance

### Calibration workflow

- Primary scorecard roles: benchmark answer set and participant response
- Primary template role: shared scoring definition used for comparison
- Main concerns: initiator ownership, participant access, comparison semantics, calibration result interpretation

### Appeal workflow

- Primary scorecard roles: original evaluated record, requested correction, resolved final decision
- Primary template role: historical definition used to interpret the appealed scorecard
- Main concerns: immutability of original state, editable appeal request state, final decision state, visibility and auditability

### Analytics and reporting workflow

- Primary scorecard role: historical record projected into reporting shape
- Primary template role: semantic reference for interpretation and grouping
- Main concerns: revision correctness, ClickHouse projection correctness, historical display, aggregation

### Repair and backfill workflow

- Primary scorecard role: authoritative record to reconstruct or correct downstream state
- Primary template role: semantic reference needed for recomputation
- Main concerns: source-of-truth boundaries, old revision semantics, projection repair, semantic validation

## 7. Rule Buckets

This is where rules should be placed as they are discovered.

### Creation rules

Examples to capture:

- what minimum structure a valid template requires
- what makes a valid scorecard instantiation

### Edit rules

Examples to capture:

- which builder transforms are canonical versus temporary edit-time behavior
- which fields are normalized on save

### Versioning rules

Examples to capture:

- what creates a new revision
- what historical behaviors must stay pinned to an earlier revision

### Assignment rules

Examples to capture:

- who can use or see a template
- how assignment interacts with use case or audience

### Scoring rules

Examples to capture:

- option-to-score mapping
- N/A behavior
- weighting and aggregation
- auto-fail behavior
- AutoQA mapping semantics

### Visibility and access rules

Examples to capture:

- admin-only versus grader-visible semantics
- runtime authorization versus builder-time configuration
- workflow-specific actor permissions, such as calibration initiator, calibration participant, appeal requester, and appeal resolver

### Historical consistency rules

Examples to capture:

- what must remain interpretable for old scorecards
- how analytics should reflect historical scorecards after template changes

### Migration and backward-compatibility rules

Examples to capture:

- legacy builder behavior that still affects saved templates
- assumptions that old data may violate

## 8. Known Sharp Edges

These are already visible enough to deserve a slot in the skeleton.

- Persisted/API representation and builder form-state representation are not the same.
- `option.value` may mean identity in one stage and score in another stage.
- Template semantics are coupled to scoring, AutoQA, storage, and analytics, so local-looking changes can have system-wide effects.
- Historical correctness likely depends on revision-aware interpretation rather than “latest template wins.”
- The same scorecard-shaped artifact may have different roles in different workflows, so workflow must be named before applying detailed permissions or mutability rules.

## 9. Surrounding Systems

Add only the systems that materially change meaning or flow.

- Director template builder
- Postgres template and scorecard persistence
- Backend scoring and AutoQA logic
- ClickHouse analytics projection
- Coaching and QA product surfaces that consume scorecards

## 7. Open Questions

These questions are intentionally first-class. They are the next investigation targets.

- What is the cleanest canonical distinction between scorecard and template at runtime?
- Which template fields are business-critical invariants versus UI/editor convenience?
- What is the exact lifecycle boundary between template revisioning and scorecard historical interpretation?
- Which scorecard/template rules are enforced centrally, and which are duplicated across layers?
- Which recurring bugs are actually symptoms of missing invariants or missing terminology?

## 8. Ticket Learning Template

Use this template for future additions:

- Local issue:
- Concepts involved:
- Lifecycle stage:
- Rule discovered or clarified:
- Edge case or historical constraint:
- Where the knowledge lived before:
- Category: doc gap / naming gap / model gap / API gap / test gap / observability gap / ownership gap
- Should this update the main reference? yes/no
