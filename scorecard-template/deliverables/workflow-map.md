# Scorecard / Template Workflow Map

**Created:** 2026-06-11
**Status:** Draft starting point
**Purpose:** Name the main business workflows that give scorecard and template artifacts different roles

## Why This Document Exists

The core domain artifacts are:

- **Template**: reusable definition/configuration
- **Scorecard**: concrete runtime record created from template semantics

Those definitions are necessary but not sufficient. The same scorecard-shaped artifact can mean different things depending on the business workflow.

Workflow is the missing lens for details such as:

- who initiates the action
- who owns the artifact
- who can view, edit, submit, appeal, or resolve it
- which state transitions matter
- whether the artifact is evidence, response, benchmark, request, decision, or projection input

Without naming the workflow, rule capture gets noisy. Permission, mutability, submission, and historical-consistency rules start to look like global scorecard rules even when they only apply to one business process.

## Core Mental Model

A workflow describes the business process in which a template or scorecard is used.

It answers:

- why does this artifact exist here?
- what role is it playing?
- who are the actors?
- which lifecycle transitions are meaningful?
- which rules are workflow-specific?

The artifact definition should stay stable. The workflow supplies role-specific semantics.

## Workflow vs Lifecycle

Workflow and lifecycle are related but not the same.

- **Lifecycle** describes how an artifact changes over time.
- **Workflow** describes why the artifact is being used and what role it plays in a business process.

Example:

- A scorecard may move through draft, submitted, projected, and historically viewed lifecycle states.
- In an evaluation workflow, that scorecard is the evaluation result.
- In a calibration workflow, a scorecard may be the benchmark or a participant response.
- In an appeal workflow, a scorecard may be the original record, the requested correction, or the resolved final decision.

The same artifact class can therefore share lifecycle mechanics while carrying different workflow meaning.

## Workflow Inventory

### Performance Evaluation Workflow

Primary purpose:

- evaluate a conversation, agent behavior, process, or coaching outcome

Template role:

- defines what can be evaluated and how scores are interpreted
- supplies criterion structure, option wiring, score mapping, N/A behavior, and operational scope

Scorecard role:

- records the concrete evaluation outcome
- becomes historical evidence after persistence and often after submission

Main actors:

- evaluator or grader
- evaluated agent or team member
- manager, QA lead, or admin depending on visibility rules

Main rule areas:

- score capture
- submission and post-submission mutability
- template revision interpretation
- visibility and access
- analytics projection

Open questions:

- Which evaluation scorecards are mutable after submission?
- Which permission rules come from the latest template state versus the historical template revision?
- Which evaluation variants share the same scorecard lifecycle?

### Calibration Workflow

Primary purpose:

- compare scoring behavior across people against a benchmark or shared reference

Template role:

- provides the shared scoring definition for all calibration participants
- defines the criteria and score semantics used for comparison

Scorecard roles:

- benchmark answer set created or selected by the calibration initiator
- participant response created by the person being calibrated
- comparison artifact used to identify agreement, disagreement, or coaching gaps

Main actors:

- calibration initiator
- calibration participant
- reviewer or facilitator

Main rule areas:

- benchmark ownership
- participant access
- response mutability
- comparison semantics
- result visibility

Open questions:

- Is the benchmark scorecard a normal scorecard with a workflow role, or a separate artifact represented with scorecard-shaped data?
- When should participant responses become immutable?
- How does group calibration differ from one-to-one calibration?

### Appeal Workflow

Primary purpose:

- challenge, correct, or resolve a completed evaluation

Template role:

- provides the historical semantics needed to interpret the original evaluation
- may constrain what can be appealed or corrected

Scorecard roles:

- original evaluated record
- appeal request describing what should be corrected
- resolved final decision for the appeal round

Main actors:

- appeal requester
- appeal reviewer or resolver
- original evaluator
- affected agent or manager

Main rule areas:

- preservation of original state
- request editability
- resolver authority
- audit history
- final decision semantics
- analytics impact after resolution

Open questions:

- Should an appeal create a new scorecard, a linked scorecard variant, or a workflow-specific wrapper around scorecard data?
- Which original fields must remain immutable for auditability?
- When does an appeal resolution change analytics versus only annotate historical context?

### Analytics and Reporting Workflow

Primary purpose:

- expose historical scorecard outcomes for reporting, aggregation, and business review

Template role:

- provides semantic context for grouping, labels, score interpretation, and historical display

Scorecard role:

- source record projected into analytics-friendly shape
- historical evidence used for aggregate reporting

Main actors:

- managers and leaders
- analysts
- backend projection and reporting systems

Main rule areas:

- Postgres versus ClickHouse source-of-truth boundaries
- revision-aware interpretation
- projection correctness
- aggregation semantics
- historical label display

Open questions:

- Which analytics views require historical template semantics versus latest operational metadata?
- How should reporting identify projection rows that exist but are semantically stale?

### Repair and Backfill Workflow

Primary purpose:

- reconstruct, correct, or validate historical scorecard-derived state

Template role:

- supplies the semantics needed to recompute or validate scorecard outcomes

Scorecard role:

- authoritative record to use as the recovery source
- input to downstream projection repair

Main actors:

- engineers
- support or customer-facing operators
- background repair jobs

Main rule areas:

- authoritative Postgres recovery
- template revision lookup
- process versus conversation scorecard coverage
- semantic validation beyond row counts
- idempotent projection repair

Open questions:

- Which repair paths are safe to run broadly?
- Which old scorecards lack enough data for exact reconstruction?
- How should repair workflows report semantic drift rather than only missing rows?

## How To Use This Map

When a new scorecard/template detail appears, first classify it:

1. Is it about what a template or scorecard is? Put it in the artifact model.
2. Is it about how an artifact changes over time? Put it in the lifecycle docs.
3. Is it about why the artifact exists in a business process, who can act on it, or what role it plays? Put it in this workflow map.
4. Is it a repeated concrete rule? Promote it into the business-rules catalog with the relevant lifecycle stage and workflow.

The goal is not to make every rule workflow-specific. The goal is to avoid pretending that every detail is globally true for every scorecard or template.
