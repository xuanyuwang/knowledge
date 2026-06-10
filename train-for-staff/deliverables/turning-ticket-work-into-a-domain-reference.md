# Turning Ticket Work into a Domain Reference

Most engineering teams have a few parts of the system where every ticket feels harder than it should. The code is not necessarily bad. The problem is that the business rules are real, the history matters, and the behavior is spread across UI state, APIs, persistence, reporting, old decisions, and tribal knowledge.

Scorecards and templates are a good example of this kind of domain. A template defines what can be evaluated. A scorecard captures one concrete evaluation. That sounds simple until you account for versions, options, scoring rules, permissions, historical display, automated evaluation, and analytics.

At first, the work can look like a stream of unrelated tickets: one display issue, one scoring edge case, one reporting inconsistency, one migration concern. Each ticket has a local fix. But after enough of them, the repeated pattern becomes clear: the team is paying the same investigation cost because the domain model is not explicit enough.

The useful move is to stop treating every ticket as only a ticket.

## The Repeated Problem

Business-rule-heavy systems accumulate hidden concepts:

- a reusable definition is different from a concrete instance created from it
- historical behavior may depend on the version that existed when the instance was created
- a value can mean different things at different layers: display label, stored identity, numeric score, or reporting dimension
- UI behavior, backend validation, persistence, and analytics may each encode part of the same rule
- derived systems can drift if they re-implement business semantics instead of relying on a canonical path

None of these ideas is unusual. The issue is that they are often discovered one ticket at a time.

That creates a predictable failure mode. A bug looks local, so the fix stays local. Then the next ticket hits the same concept in another layer.

A display issue may really expose a representation mismatch. A scoring edge case may affect authoring, storage, runtime calculation, and reporting. A reporting bug may reveal that a derived system is making assumptions the source-of-truth path does not make. These are domain patterns, not isolated bugs.

## The Intervention: Build the Map

The better response is to create a working reference.

The first version does not need to be complete. It only needs to answer four questions:

1. What are the core concepts?
2. How do they relate?
3. What are the main lifecycle transitions?
4. Where do the important rules show up?

That gives new knowledge somewhere to land. From there, the reference can grow into a concept map, lifecycle notes, a rule catalog, a pattern log, and a list of small improvement opportunities.

The important part is not perfect documentation. The important part is that each new ticket leaves behind one durable piece of understanding.

For each ticket, capture:

- the local issue
- the lifecycle stage involved
- the concepts involved
- what looked local at first
- the repeated pattern underneath
- the rule clarified
- the kind of weakness exposed: doc gap, model gap, API gap, test gap, observability gap, or ownership gap
- a candidate small improvement

That structure turns scattered debugging into accumulated domain knowledge.

## What Changes

The biggest change is the quality of follow-up questions.

Instead of only asking "where is this bug?" the reference pushes the investigation toward better questions:

- Which lifecycle stage is this in?
- Is this an authoring problem, runtime behavior problem, display problem, or reporting problem?
- Is this value an identity, an index, a score, or a label at this layer?
- Is a derived path re-implementing a rule that already exists elsewhere?
- Is this another example of a known representation mismatch?

That changes the work. You still fix the bug, but you also classify it. Over time, the classifications reveal where the system needs small structural improvements.

One common example is schema evolution. Product features keep changing the shape of stored configuration. If compatibility logic is scattered, every future change has to rediscover the same migration concerns. A domain reference makes that pattern easier to name: this is a versioning and backward-compatibility problem. That can lead to a more deliberate design, such as explicit schema versions and sequential updaters that normalize old shapes at a clear boundary.

That is a better output than "remember to handle old data."

## What Makes This Staff-Level Work

The staff-level part is not writing documentation. Documentation is cheap if it does not change decisions.

The staff-level part is turning repeated ambiguity into a shared operating model.

When a domain has implicit rules, every engineer has to rediscover them. When the rules are organized by concept and lifecycle, the team can reason from the same map. Product discussions get more concrete. Code reviews can ask whether a change preserves the right invariant at the right boundary. Future tickets start from the current model instead of a blank investigation.

It also creates a different kind of leverage. A single bug fix helps one workflow. A good domain reference makes the next ten changes cheaper to understand and safer to make.

## The General Pattern

If you keep seeing unrelated tickets in the same area, do not only ask whether the code needs cleanup. Ask whether the domain needs a map.

A useful reference does not need to be exhaustive. It needs to be structured enough that new facts have somewhere to land.

Start with:

1. core concepts
2. relationships
3. lifecycle stages
4. rule buckets
5. recurring ticket patterns

Then let real work grow it. Every ticket should leave behind more than a merged PR. It should leave behind a clearer model of the system.

That is how local execution becomes domain stewardship.
