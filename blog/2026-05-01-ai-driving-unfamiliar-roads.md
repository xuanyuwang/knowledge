# Driving a Performance Car on an Unfamiliar Road

I've been thinking about how my relationship with AI coding tools has changed over the past few months, using a recent project as a case study. The short version: I went from "this is useless" to "I'm 300% productive" to "I've completely lost control" — and I think the arc reveals something important about how to use AI well.

## The Three Phases

**Phase 1: Useless.** Early on, AI felt like more overhead than it was worth. I spent more time explaining context and fixing wrong suggestions than I would have spent just writing the code myself.

**Phase 2: Powerful.** Once I got comfortable, I could parallelize work I used to do sequentially. Write the investigation doc while the CI runs. Generate the test scenarios while reviewing the PR. Explore three approaches simultaneously and pick the best one. It genuinely felt like a multiplier.

**Phase 3: Lost control.** Then I hit a wall. The output kept coming, but I couldn't tell which parts were right. Fixes introduced new bugs. My understanding documents had to be corrected. I was producing more and understanding less.

The driving analogy feels right: newbie nervous in an old car → confident driver in normal conditions → formula car driver who can't feel the road anymore.

## What Actually Happened: The N/A Score Project

I was implementing N/A score support for scorecard templates. New to the frontend codebase — I'm primarily a backend Go engineer. Complex React Hook Form state management, a dual data model (options array + scores array), AutoQA sync mechanisms. Fragile code where behavior depends on execution order and subtle state transitions.

Here's the timeline:

**April 12:** Enormous output — 4 fixes, 9+ analysis documents, a critical issue identified. Felt incredibly productive.

**April 13:** Implemented the backend changes. Good flow. Tests passed.

**April 16:** PR review feedback came back. Address comments, fix CI failures from being 114 commits behind main. Friction.

**April 21:** Another validation bug surfaces — `# of Occurrences` with scored N/A. The root cause was a length mismatch between `auto_qa.options` and `settings.options` that only makes sense if you deeply understand that N/A is tracked separately via `auto_qa.not_applicable`. A nuance invisible from reading the code.

**April 25:** Stepped back, wrote a system-level lifecycle analysis (`options-scores-lifecycle.md`). Identified 8 pain points.

**April 29:** Corrected that lifecycle doc. Decoupled scoring mode wasn't a pain point — it was an intentional performance optimization. My earlier analysis was wrong because I had the wrong mental model.

The pattern: **high output → bugs from the output → stepping back → correcting earlier understanding.** Each fix was locally reasonable but missed systemic interactions.

## Why I Lost Control

It's tempting to say "I was new to the codebase" and leave it there. But that's not quite right. The specific failure mode was that **this code has hidden temporal invariants**.

React Hook Form's `useWatch`, `useOnMount`, `useFieldArray` creating `[{},{}]` empty placeholders — these aren't things you can understand from reading. They're things that only make sense when you have a mental model of the runtime execution order. When do these effects fire? What state has been set by then? What happens if initialization order changes?

AI can read code and produce accurate *descriptions*. It cannot tell you which of the 50 things that *could* break actually *will* break in a system with hidden temporal coupling. I got a document that correctly described "useWatch + useMemo provides automatic sync" — but that description didn't include "and if you change initialization order, the sync will observe stale state." That's the gap.

The deeper issue: **I outsourced exploration to AI before I had enough context to judge the output.** I asked AI to investigate, got thorough-looking analysis documents, and treated them as ground truth. But analysis documents are only as good as the questions you know to ask. I didn't yet know the right questions.

## How to Use AI Better in Unfamiliar Territory

**1. Use AI to build mental models, not to generate fixes.**

Instead of "fix this bug," ask "trace what happens step-by-step when a user unchecks AutoQA on a new criterion — what state changes occur, in what order, and what assumptions does each step make?" Then *you* identify the problem. The AI is your rubber duck with encyclopedic knowledge of the codebase, not your engineer.

**2. Demand blast radius analysis before any change.**

Before accepting a fix: "What other code paths depend on the behavior we're changing? What assumptions will this change invalidate? What's the worst-case failure mode?" Force the AI to enumerate what could go wrong. A good AI response to a proposed fix should always be longer about risks than about the fix itself.

**3. One thing at a time, tested before the next.**

My April 12 log had 4 fixes. Each one changed assumptions the others depended on. Fix 2 affected state that Fix 3 then had to compensate for. The correct discipline: ship one fix, test it, confirm your mental model was right, *then* proceed. Speed through the individual steps, not through the verification.

**4. Use AI for adversarial review, not just generation.**

After you have a fix, explicitly shift to skeptic mode: "Here's my change. What edge cases does this miss? What invariants could this violate? What would a careful reviewer push back on?" The April 29 PR review session — where I had AI look for problems rather than generate solutions — found real issues (misleading error messages, wrong dependency direction, semantically questionable N/A dropdown behavior).

**5. Write the lifecycle doc first.**

My `options-scores-lifecycle.md` — the full data lifecycle analysis — was the most valuable artifact in the project. I wrote it on April 25, two weeks in, after the bugs. If I'd asked AI to trace the full lifecycle *before* writing any fix, I'd have caught the decoupled-mode design intent upfront and avoided at least one bug.

The generalized principle: **spend the first 20% of the project building a model of how the system works at runtime, not just at read-time.** Have AI help you build that model with questions, traces, and examples — not with implementation.

## The Reformulated Analogy

The performance car analogy is right, but the problem isn't the car speed. It's the road.

If you know the road — the tight corners, the blind spots, the places where the pavement is unpredictable — you can use the car's full capability safely. If you don't know the road, going fast just means you reach the crash sooner and with more momentum.

AI is the car. Your domain understanding is knowing the road. The solution isn't to drive slower (use AI less). It's to first drive the road once at normal speed — trace the data flow, understand the execution order, identify the fragile invariants — and *then* use the full capability of the car.

High output on low understanding is a liability, not an asset. The multiplication is on both signal and noise.

## What This Means for Unfamiliar Codebases

Every codebase has a layer of tacit knowledge that isn't in the code: why the code was structured this way, what bugs were fixed by the current implementation, what invariants are maintained by convention rather than enforcement. For familiar codebases, you carry this knowledge implicitly. For unfamiliar ones, you have to build it deliberately.

AI can accelerate the building of that knowledge — but only if you direct it toward understanding rather than output. The failure mode is using AI as an output machine before you've used it as a learning tool.

Use AI to expand and deepen understanding. Let understanding gate output. The compound returns on the right sequencing are far better than the short-term gains of skipping it.
