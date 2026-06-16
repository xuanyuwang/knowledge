# From Scorecard APIs to Business Workflows

I used to think the scorecard system was complicated because the code was complicated.

There were large handlers, overloaded APIs, branching behavior, old data models, new data models, permissions, submission rules, calibration rules, appeal rules, and analytics projection rules. Every time I looked at the code, the shape of the problem seemed to be in the implementation:

- too many fields on scorecard
- too many if/else branches
- too many scorecard types
- too many places where `updateScorecard`, `submitScorecard`, or `createScorecard` meant slightly different things

So my first instinct was to solve the complexity at the code level.

Looking back, that instinct was only half right. The code was complex, but the deeper issue was that the business model had outgrown the names in the code.

## When One Workflow Owned the Domain

Originally, the mental model was simple:

- a template defines what can be evaluated
- a scorecard records the evaluation
- users create, update, and submit scorecards

In that world, APIs like these were natural:

- `createScorecard`
- `updateScorecard`
- `submitScorecard`

The names matched the product. A scorecard was mostly an evaluation artifact. Updating a scorecard meant editing an evaluation. Submitting a scorecard meant finalizing an evaluation. The workflow was so obvious that it did not need a name.

That is the trap.

When there is only one workflow, the workflow disappears into the artifact. "Scorecard" silently means "evaluation scorecard." "Submit" silently means "submit an evaluation." "Update" silently means "update the evaluation result."

The API looks clean because the business context is implicit.

## Then the Product Added More Workflows

Over time, scorecards started appearing in more places:

- performance evaluation
- one-on-one calibration
- group calibration
- appeal
- analytics and historical review
- repair and backfill

The same scorecard-shaped data was still useful, but it no longer played one role.

In evaluation, a scorecard is the evaluation result.

In calibration, a scorecard may be the benchmark answer set, or it may be a participant response being compared with the benchmark.

In appeal, a scorecard may represent the original evaluation, the requested correction, or the resolved final decision for an appeal round.

In analytics, a scorecard is historical evidence projected into a reporting shape.

In repair, a scorecard is the authoritative source used to reconstruct downstream state.

Structurally, these things look similar. Semantically, they are not the same.

But the API names did not change at the same pace as the product model. We kept stretching generic verbs:

- update scorecard for evaluation
- update scorecard for calibration
- update scorecard for appeal
- submit scorecard for evaluation
- submit scorecard for calibration response
- create scorecard for benchmark, response, appeal request, or evaluation

The verbs stayed artifact-centric while the business had become workflow-centric.

## My First Attempt: More Scorecard Types

The natural engineering response was object-oriented.

If scorecards behave differently in different situations, create different scorecard types. Keep a base scorecard abstraction, then layer specific implementations on top:

- evaluation scorecard
- calibration scorecard
- group calibration scorecard
- appeal scorecard

Each type can implement familiar operations:

- create
- update
- submit
- validate
- compute permissions

Then an API like `updateScorecard` can load the scorecard, inspect its type, dispatch to the corresponding implementation, and let each implementation handle its own rules.

That approach is not wrong. It can be a reasonable transitional design. It creates places to put behavior. It may reduce some branching in a large handler. It gives engineers a way to stop adding every new case to the same function.

But it has several flaws.

### The Abstraction Boundary Is Still the Artifact

The design still starts from the question:

> What type of scorecard is this?

But the real business question is often:

> What workflow is this action part of?

Those are not always equivalent.

An appeal may involve the original scorecard, a proposed correction, reviewer decisions, audit history, and analytics impact. Calling the whole thing an "appeal scorecard" pushes workflow semantics into one artifact-shaped box.

Calibration has a similar issue. The benchmark and participant response may both be scorecard-shaped, but their roles are different. Treating them as different scorecard subtypes can work, but it hides the more important concept: calibration is a workflow with actors, roles, comparison rules, visibility rules, and state transitions.

### Generic Verbs Stay Ambiguous

Even with separate scorecard types, `updateScorecard` still carries too much meaning.

Does update mean:

- edit a draft evaluation?
- change a calibration benchmark?
- save a participant response?
- request an appeal correction?
- resolve an appeal?
- repair historical projection state?

Those operations may touch similar fields, but they are not the same business action. When a single API name covers all of them, the caller has to know hidden context.

The implementation may become cleaner internally, but the product semantics remain undernamed.

### Type Dispatch Replaces Domain Modeling

A type switch can make code easier to organize, but it can also become a disguised version of the original problem.

Instead of one large `updateScorecard` function with many branches, we get:

- a dispatcher
- several scorecard implementations
- duplicated lifecycle assumptions
- duplicated permission checks
- hidden cross-workflow coupling

The code looks more object-oriented, but the domain model is still incomplete if workflow is not explicit.

The result is often a system that is cleaner locally but still hard to reason about globally.

### Workflow Rules Leak Into Artifact Helpers

Once workflow is not named, basic artifact helpers start accumulating workflow rules:

- appeal-specific mutability
- calibration-specific access
- evaluation-specific submission semantics
- analytics-specific projection behavior
- historical-revision exceptions

The helper that was supposed to "just update a scorecard" becomes a policy engine. Over time, nobody knows whether a change is safe because the function name sounds generic but the behavior is not.

## The Missing Concept: Behavioral Frames

The shift for me was separating the domain into two categories.

**Domain artifacts** are the things that exist as product and system objects:

- template
- scorecard

**Behavioral frames** are the lenses that explain how those artifacts are used:

- lifecycle
- workflow

The short version:

- artifacts are what exists
- frames are how to reason about what exists

A template is still a template. A scorecard is still a scorecard. But their meaning changes depending on the lifecycle stage and workflow.

Lifecycle explains how an artifact changes over time:

- created
- edited
- submitted
- persisted
- projected
- viewed historically
- repaired

Workflow explains why the artifact exists and what role it plays:

- evaluation result
- calibration benchmark
- calibration response
- appeal request
- appeal resolution
- analytics input
- repair source

Once I saw that split, the complexity started to move to the right place.

## A Better Design Direction

The better design is not necessarily "replace all generic APIs with workflow APIs." It is to make each layer honest about what it owns.

### Layer 1: Artifact Primitives

At the bottom, keep small artifact-level helpers.

These should be boring and narrow:

- create a scorecard row
- update score rows
- submit a scorecard state
- load a template revision
- compute score semantics
- persist audit metadata
- project scorecard data

These helpers should not decide what an appeal means or when a calibration participant can edit a response. They should enforce low-level invariants and perform well-defined state changes.

In this layer, names like `updateScorecard` can still exist, but they should mean exactly that: update the scorecard artifact, not execute a business workflow.

### Layer 2: Workflow Commands

Above the primitives, define workflow-level commands with business names:

- `submitEvaluation`
- `saveCalibrationBenchmark`
- `submitCalibrationResponse`
- `requestScorecardAppeal`
- `resolveScorecardAppeal`
- `reindexScorecardProjection`

These names are longer, but they carry meaning. They tell the caller which business action is happening. They give the implementation a clear home for actor checks, state transitions, validation, audit behavior, and side effects.

This layer can call the artifact primitives, but it should own workflow policy.

### Layer 3: Workflow State and Audit

Some workflows deserve explicit state models.

Appeal is the clearest example. An appeal is not just an update to a scorecard. It has:

- requester
- reviewer
- original state
- proposed correction
- decision
- resolution status
- audit history
- possible analytics impact

Trying to express all of that through `updateScorecard` is how a simple API becomes a confusing API.

For workflows with real state, model the workflow state directly. The scorecard remains an artifact inside the workflow, not the workflow itself.

## The Design Options

There are a few reasonable options, depending on how mature the workflow is.

### Option 1: Keep Generic APIs, Add Workflow Parameters

Example:

- `updateScorecard(scorecard_id, workflow_context, payload)`

This is the smallest change, but it is also the weakest model. It can help with logging, permissions, and routing, but the API name still hides business intent. Over time, `workflow_context` risks becoming a bag of flags.

This option is useful as a migration step, not as the end state.

### Option 2: Scorecard Type Dispatch

Example:

- `EvaluationScorecard.Update`
- `CalibrationScorecard.Update`
- `AppealScorecard.Update`

This organizes code better than a single handler. It works when each scorecard type maps cleanly to one business meaning.

The flaw is that it still treats workflow as a property of the artifact. That breaks down when one workflow contains multiple scorecard roles, or when the same scorecard participates in multiple workflows.

This option is useful for local cleanup, but it does not fully solve the domain-modeling problem.

### Option 3: Workflow-Specific Commands Over Shared Artifact Primitives

Example:

- `submitEvaluation`
- `submitCalibrationResponse`
- `resolveAppeal`

Each command uses shared scorecard/template helpers underneath.

This is the best general direction. It keeps reusable code reusable while making business actions explicit. It also gives permissions, validation, lifecycle transitions, and audit behavior a natural home.

The cost is more API surface area and more naming work. But that cost is often worth paying once the product has multiple workflows.

### Option 4: Full Workflow Engine or State Machine

For mature workflows with many states and transitions, a state machine may be appropriate.

This can make transitions explicit:

- draft -> submitted
- submitted -> appealed
- appealed -> resolved
- resolved -> projected

But a workflow engine is not automatically better. If introduced too early, it can become ceremony around unclear concepts. The domain language has to come first.

This option is strongest when transitions, actors, and audit requirements are stable enough to model directly.

## What AI Can and Cannot Help With

AI can help refactor the code.

It can read handlers, identify duplication, propose abstractions, generate type hierarchies, and draft migration plans. Those are useful capabilities.

But AI will usually follow the concepts you give it.

If you ask it to clean up scorecard code, it will clean up scorecard code. If your mental model is artifact-centric, the output will probably be artifact-centric too:

- base scorecard type
- specialized scorecard types
- shared methods
- dispatch by type

That may look reasonable because it is reasonable inside the frame you supplied.

The harder part is noticing that the frame itself is wrong.

That usually requires hands-on product context:

- seeing the same API name confuse multiple workflows
- watching permissions become harder to explain
- noticing that "submit" means different things to different actors
- debugging why a clean scorecard abstraction still feels awkward
- realizing that the missing concept is not another struct, but another business frame

AI can help once you name the frame. It is much less likely to discover the frame for you.

## The General Lesson

When a domain grows, complexity often appears first in the code. The temptation is to answer with code structure:

- more types
- more interfaces
- more helpers
- more dispatch

Sometimes that is necessary. But before choosing the abstraction, ask whether the business language has changed.

In this case, the product moved from one implicit workflow to multiple explicit workflows. The code kept using artifact-centric names because those names had been correct in the old world.

The real shift was not from bad code to good code. It was from artifact thinking to behavior-frame thinking.

Once workflows are named, the design becomes clearer:

- keep scorecard helpers small and artifact-level
- put workflow policy in workflow commands
- model workflow state directly when the workflow deserves it
- let API names say the business action, not just the database object being touched

That is the kind of clarity that does not come from staring harder at the code. It comes from understanding what the business is now asking the code to represent.
