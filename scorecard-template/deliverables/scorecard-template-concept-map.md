# Scorecard / Template Concept Map

**Created:** 2026-05-17  
**Status:** First populated pass  
**Purpose:** A domain-level map of the main concepts and relationships in the coaching scorecard/template system

## How To Read This

This document sits between the domain skeleton and the full system reference.

- The domain skeleton gives the framework.
- This concept map populates that framework with the current best model.
- The system reference goes deeper into implementation details and invariants.

This is intentionally not exhaustive. Its purpose is to make the domain discussable.

## 1. The Core Distinction

The most important distinction in this domain is:

- **Template** = the reusable definition of what can be evaluated and how it should be interpreted
- **Scorecard** = the runtime evaluation artifact produced when that definition is used in a real grading or coaching context

If this distinction gets blurred, almost every downstream discussion becomes harder:

- product conversations mix authoring rules with scoring behavior
- debugging mixes persisted configuration with runtime results
- historical questions become confusing because template evolution and scorecard history are different things

## 2. Core Concepts

### Template

What it is:

- the configuration backbone of the coaching and QA system

What it contains:

- structural definition of chapters and criteria
- scoring semantics
- AutoQA mapping semantics
- operational metadata such as audience, permissions, use case scope, type, and status

What matters most:

- it is versioned by revision
- it is authoritative in Postgres
- it drives multiple system layers, not just the builder UI

### Scorecard

What it is:

- the runtime scoring artifact that records evaluation outcomes for a conversation, process, or coaching context

What it depends on:

- a template definition
- score inputs from a human grader, AutoQA, or both
- scoring rules applied by backend logic

What matters most:

- it is the artifact that becomes historical data
- it is stored authoritatively in Postgres
- it is later projected into ClickHouse for analytics

### Criterion

What it is:

- the evaluatable unit inside a template

Why it matters:

- it is where business meaning becomes scoreable behavior
- criterion type determines which settings, scoring logic, and AutoQA wiring are valid

Important distinction:

- some criteria are scorable
- some criteria are metadata-only and do not participate in QA scoring

### Chapter

What it is:

- a structural grouping node that organizes criteria

Why it matters:

- chapters create the hierarchical shape of the template
- scores eventually roll up across criteria and chapters into larger summaries

### Option

What it is:

- a selectable outcome for a criterion, usually carrying a label and an identity key

Why it matters:

- option identity is the wiring anchor for score mapping and AutoQA mapping
- the meaning of `option.value` changes across some representations, which is a recurring source of confusion

### Score Mapping

What it is:

- the mapping from a selected criterion value to the numeric score that participates in aggregation

Why it matters:

- raw selected value and actual score are not always the same
- many bugs come from assuming “selected option value” and “score meaning” are identical

### AutoQA Mapping

What it is:

- the logic that converts automated detection outcomes into criterion selections

Why it matters:

- it reuses the same wiring keys as options and score mappings
- it makes template semantics operational in automated scoring paths, not only in manual grading

### Revision

What it is:

- the immutable version boundary for a template definition

Why it matters:

- historical interpretation depends on the effective revision, not only on the template identity
- replay, rescoring, and debugging need revision awareness

### Audience / Permissions / Use Case Scope

What they are:

- operational concepts that determine who can use a template, where it applies, and under what access rules

Why they matter:

- templates are not only scoring definitions; they are also operational business objects
- many “template bugs” are actually scope or applicability bugs

### Analytics Projection

What it is:

- the ClickHouse representation of scorecard-derived data for reporting and aggregation

Why it matters:

- analytics is downstream from the authoritative Postgres model
- mismatches between authoritative state and projection create debugging and trust issues

## 3. Relationship Map

## Template -> Chapter -> Criterion

- A template defines a hierarchy of chapters and criteria.
- Chapters are structural containers.
- Criteria are the actual evaluatable units.

This means the template is not a flat list of rules. It is a structured evaluation model.

## Criterion -> Option / Range / Score Mapping

- A criterion’s type determines whether it uses labeled options, numeric ranges, dropdown values, or non-scorable input.
- For scorable criteria, the criterion settings define how a selection becomes a score.

This is one of the main places where product behavior becomes code behavior.

## Option -> Score Mapping -> AutoQA Mapping

- `options[].value` identifies a logical option
- `scores[].value` points to that option identity
- AutoQA `detected`, `not_detected`, and sometimes `not_applicable` point to that same logical option identity

This is the central coupling in the system. If these drift apart, the structure can still look valid while the meaning is wrong.

## Template -> Revision

- A template has an identity and multiple revisions.
- Revision captures the exact shape and semantics used at a point in time.

This is the boundary that prevents “current template understanding” from silently overriding historical reality.

## Template -> Scorecard

- A scorecard is created or interpreted using template semantics.
- A scorecard is not the template itself; it is the runtime result of using the template in a concrete context.

This relationship is where many lifecycle questions live:

- when is the scorecard instantiated?
- what is copied versus referenced?
- what changes after template edits?

## Scorecard -> Score Rows / Aggregation

- Individual criterion outcomes become score records.
- Backend scoring logic converts them into percentage scores, weights, and rollups.

This is where template configuration becomes actual business output.

## Scorecard -> Analytics Projection

- Authoritative scorecard data lives in Postgres.
- Analytics/reporting data is projected into ClickHouse.

This means there is a source-of-truth boundary:

- Postgres answers “what the system currently knows as truth”
- ClickHouse answers “how that truth is exposed for analytics”

## Template / Scorecard -> Coaching Context

- The same domain concepts appear in conversation QA, process scorecards, AutoQA flows, and coaching/reporting surfaces.
- These contexts share a core model but may differ in lifecycle details and edge cases.

That is why domain understanding matters more than endpoint-level understanding.

## 4. Domain Layers

Thinking in layers helps organize knowledge without flattening everything into one list.

### Layer 1: Authoring

Primary concept:

- template as an editable business definition

Main concerns:

- builder behavior
- criterion configuration
- option/score editing
- N/A behavior
- access and audience setup

### Layer 2: Persistence

Primary concept:

- template and scorecard as authoritative data in Postgres

Main concerns:

- resource identity
- revision
- stored JSON structure
- score records
- operational metadata

### Layer 3: Runtime Scoring

Primary concept:

- turning selected values into business outcomes

Main concerns:

- percentage calculation
- weighting
- auto-fail
- N/A semantics
- multi-select and per-message behavior
- AutoQA mapping

### Layer 4: Historical Interpretation

Primary concept:

- understanding a scorecard correctly after templates and systems evolve

Main concerns:

- revision-aware interpretation
- replay / rescoring
- backward compatibility
- legacy template behavior

### Layer 5: Analytics Consumption

Primary concept:

- using projected scorecard data for reporting and insight generation

Main concerns:

- ClickHouse projection correctness
- aggregation semantics
- filtering behavior
- consistency with authoritative Postgres data

## 5. Business-Rule Hotspots

These are the places where scattered rules are most likely to accumulate.

### Criterion scoring semantics

Questions:

- Is the selected value the score, or only a lookup key?
- Is the criterion excluded dynamically, statically, or neither?
- How should N/A affect numerator and denominator?

### Template revision semantics

Questions:

- What exactly changes when a template is edited?
- Which runtime objects should remain pinned to older semantics?

### Representation boundaries

Questions:

- What does a field mean in persisted template JSON?
- What does the same-looking field mean in builder form state?
- What does the grader submit?
- What is stored in Postgres score rows?
- What is projected into ClickHouse?

### Applicability and scope

Questions:

- Who can use a template?
- In which use cases does it apply?
- How do permissions and audience affect runtime behavior?

### Historical and analytics correctness

Questions:

- When should historical scorecards stay stable?
- How should analytics behave when template semantics evolve?

## 6. Current Best Mental Model

If I had to compress the domain into one paragraph:

Scorecard/template is a configuration-driven evaluation system. Templates define a structured, versioned business model for what is evaluated and how it is interpreted; scorecards are the runtime and historical artifacts produced from that model; scoring and AutoQA logic operationalize the model; Postgres is authoritative for templates, scorecards, and scores; ClickHouse is a downstream projection for analytics; and many recurring bugs come from losing track of which layer, representation, or revision is currently being discussed.

## 7. What This Map Still Does Not Resolve

This first pass does not yet fully answer:

- the exact runtime boundary between template reference and copied scorecard state
- the full set of assignment and audience semantics
- the complete list of criterion types and edge-case behaviors in practice
- the exact lifecycle differences between conversation scorecards and process scorecards
- the complete historical-decision trail that explains today’s legacy behavior

Those gaps should drive the next rounds of refinement rather than block the map now.
