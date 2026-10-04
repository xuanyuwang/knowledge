# From AI-Generated Code to Forward Engineering

AI coding agents have become good at producing software from surprisingly little direction. That is useful, but it changes the nature of the work. The more of the construction we delegate, the less directly we see how the result came to be—and we still own the consequences.

I ran into this while trying to understand code an AI had generated. Reading it carefully took too long. Asking the AI to explain it line by line gave me accurate descriptions, but not understanding. Plan mode helped me see the reasons and alternatives, yet the code still felt like a finished object whose construction I had missed.

That experience led me to a larger question: **how do we delegate substantial engineering work without losing control of its direction or giving up the chance to learn from it?**

## More capability, less visibility

At first, AI was mostly a tool for completing a task I already understood. I could picture the output, describe its shape, and ask the model to save me the typing. As the tools became more capable, I started delegating work whose solution I could not specify in detail.

That is a different relationship. The agent is no longer just filling in a known design; it is choosing steps along the way. Its speed can increase while my visibility into those steps decreases. I remain accountable for whether the result is correct, safe, maintainable, and appropriate—even when I did not make every decision that produced it.

The driving analogy only goes so far. A faster car is usually designed to remain controllable at speed. Delegating to an AI coding agent can feel more like getting into a car whose driver is moving quickly while you remain responsible for the trip. If I prescribe every turn, I have barely delegated. If I provide only a destination, the agent may take a route I did not expect—or misunderstand where I meant to go.

The missing capability is not simply more detail. I do not want to carry the agent's entire workload in my head or inspect every keystroke. I need enough transparency to tell whether the work is moving in the right direction, and enough understanding to judge the decisions that matter.

## The destination is not the journey

There are two things I want from AI assistance: useful work and a chance to become better at the work myself.

An agent can meet the first goal while missing the second. I can spend tokens, receive a working implementation, and learn almost nothing about why it is structured that way. The next time, I am no better at framing the problem, spotting a bad assumption, or deciding what to ask.

That matters professionally, too. Engineering skill is part of how we contribute and support ourselves and our families. If the agent's capability grows while our judgment stagnates, we may become less able to explain the value we bring or to recognize when its output is wrong. The point is not to reject delegation. It is to use it in a way that grows human judgment alongside machine capability.

And learning from the agent can be valuable in its own right. It may find a simpler abstraction, uncover an overlooked constraint, or suggest an approach I would not have considered. But those surprises only become useful knowledge if I can see the reasoning in an engineering form and connect it to the code that followed.

## Why explanations of the finished code did not work

When I asked for a line-by-line explanation, I got a map of the final artifact: what each function does, how the components fit together, and where the data flows. That can be accurate and still fail to answer the question I care about: **why did the implementation take this shape?**

Plan mode had a related limitation. It described intended files and choices before implementation, but the plan was still a list of destinations. It did not show the causal path from an initial requirement, through constraints discovered along the way, to the decisions that made the final design necessary.

What I was missing was the sequence of meaningful engineering states:

```text
Problem
  → constraint discovered
  → design decision
  → code change
  → new capability or invariant
  → next problem becomes visible
  → next decision
```

A diff tells me what changed. A plan tells me what the agent intends to change. An explanation tells me what the finished code does. None necessarily shows why one state led to the next.

## The missing artifact: a construction history

I want the agent to preserve an engineering decision chain as it works. Not a transcript of every tool call and not a dump of hidden internal reasoning. A concise record of the decisions that explain how the software came into being.

For each meaningful milestone, the record could answer:

1. What problem are we solving at this point?
2. What is true about the codebase before the change?
3. What constraint or new information matters?
4. What decision follows, and why is it the next useful step?
5. What changed in the code?
6. What can the system do now, or what invariant does it maintain?
7. What remains unresolved?

The unit is a semantic milestone, not a file edit. A change might introduce a domain concept, add a persistence boundary, move behavior out of a handler, or make an invariant verifiable. The sequence would show how each milestone creates the conditions for the next.

This is different from asking the agent to write a retrospective after it has finished. By then, the implementation has already happened, and its author—human or model—can tell a neat story that may not reflect the actual decisions. The history needs to be captured as the work proceeds: propose a milestone, make the change, inspect the result, verify it, and record what was learned before moving on.

## Teach through the steps, not by preserving bad code

There is a tradeoff here. Agents often jump straight to a polished architecture with interfaces, factories, dependency injection, and other abstractions. Those may be appropriate, but if they arrive all at once, it is hard to see what need each one serves.

A learning-oriented workflow might begin with a simpler implementation and introduce an abstraction only when a concrete pressure makes it useful. For example, first two callers need the same behavior; then the shared boundary becomes valuable. The intermediate code is not a lesson in writing bad software. It is a way to make the motivation for a good design visible.

The agent should still keep the overall destination in view. It should not manufacture problems or take needless detours just to produce a tutorial. The useful constraint is smaller: make one conceptually coherent change at a time, and explain the pressure that makes that change worthwhile.

## A forward-engineering mode

The prompt I wish I had started with is something like this:

> Help me build this feature in meaningful engineering milestones. Before each change, state the problem at that step, the relevant current state and constraints, and the decision you propose. Explain why this is the next useful change. Then implement only that milestone, show the diff, and verify it. Summarize the capability or invariant we gained and what problem remains. Keep intermediate designs simple, and add abstractions when a concrete need appears. Preserve this construction history as we go; do not reconstruct it only after the implementation is complete.

This would turn AI from a code generator into a pair programmer conducting a construction session. The agent still does substantial work. The human gets checkpoints at the level where judgment matters, along with a record that can be revisited later.

Git already gives us part of the structure. A sequence of focused commits can represent meaningful states, and the code at each point is inspectable. A tool could connect each milestone to its commit, rationale, and verification result, creating a version-controlled tutorial that grows alongside the implementation. That is a product direction, not a requirement for the workflow: even a small written decision record attached to each step would be an improvement.

## Keep the human in the learning loop

The central opportunity in AI-assisted engineering is not only to produce more code per hour. It is to delegate execution while retaining the visibility needed to steer, judge, and learn.

I do not need to know every detail to stay in control. I need to understand the significant decisions, see what evidence supports them, and notice when the work is drifting from the goal. And when the agent finds a better approach, I want to understand enough of it to carry that insight forward.

AI coding has optimized quickly for producing software. We should also make it good at preserving the construction process that lets humans understand software. The missing artifact is a persistent, causal narrative connecting requirements, decisions, intermediate code states, and the final implementation.

That is what I mean by **forward engineering**: not merely asking what code should exist, but seeing how it should come into existence—one meaningful engineering decision at a time.
