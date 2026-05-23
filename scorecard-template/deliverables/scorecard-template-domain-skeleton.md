# Scorecard / Template Domain Skeleton

**Created:** 2026-05-17  
**Status:** Draft starting point  
**Purpose:** Minimal framework for organizing the scorecard/template domain before attempting exhaustive rule capture

## How To Use This Document

This is not the full reference. It is the starting scaffold.

When new details appear in ticket work, place them into one of these sections instead of collecting isolated notes. Unknowns are part of the structure and should be recorded explicitly.

## 1. Core Concepts

### Scorecard

Working definition:

- the runtime evaluation artifact used to assess a conversation, agent behavior, or coaching outcome

Open questions:

- When exactly is a scorecard instantiated?
- Which fields are copied from the template versus referenced indirectly?
- Which parts of scorecard state are mutable after creation or submission?

### Template

Working definition:

- the reusable blueprint that defines what can be evaluated and how evaluation semantics work

Known characteristics:

- stored authoritatively in Postgres
- versioned by revision
- used by builder UI, scoring logic, and analytics projection flows

Open questions:

- Which template changes are safe for future scorecards but must not affect historical scorecards?
- Which template metadata is operational versus scoring-related?

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

Open questions:

- Which contexts share the same core semantics and which introduce special cases?

## 2. Relationship Map

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

## 3. Main Lifecycle

This section is the first place to attach business rules. Rules become easier to reason about when tied to a transition.

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

## 4. Rule Buckets

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

### Historical consistency rules

Examples to capture:

- what must remain interpretable for old scorecards
- how analytics should reflect historical scorecards after template changes

### Migration and backward-compatibility rules

Examples to capture:

- legacy builder behavior that still affects saved templates
- assumptions that old data may violate

## 5. Known Sharp Edges

These are already visible enough to deserve a slot in the skeleton.

- Persisted/API representation and builder form-state representation are not the same.
- `option.value` may mean identity in one stage and score in another stage.
- Template semantics are coupled to scoring, AutoQA, storage, and analytics, so local-looking changes can have system-wide effects.
- Historical correctness likely depends on revision-aware interpretation rather than “latest template wins.”

## 6. Surrounding Systems

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
