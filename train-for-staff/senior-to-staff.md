# Senior → Staff: Gap Framework (Working Doc)

This is the explicit rubric I’m using to identify gaps between **Senior Engineer** and **Staff Engineer**, and to turn those gaps into concrete weekly actions.

## Staff engineer (working definition)

A Staff Engineer increases org output by:
- Owning outcomes across a **broader scope** than a single team/service
- Making durable **technical strategy** decisions under ambiguity
- Creating **leverage** (standards, platforms, paved roads, tooling) that scales beyond their own code contributions
- Managing **risk** (correctness, security, reliability, performance) through design + rollout discipline
- Aligning stakeholders through **clear communication** and principled decision-making

## The gaps: Senior vs Staff

### 1) Scope & ownership
- Senior: owns a component/project; success is local delivery.
- Staff: owns an end-to-end problem space; success is cross-team outcomes and sustained health.

**Gap to close**
- Move from “I shipped X” to “X is now the default path / reduced incidents / reduced time-to-ship for N teams”.

### 2) Problem framing & strategy
- Senior: executes on a defined problem with clear requirements.
- Staff: defines the problem, clarifies constraints, and proposes a strategy that balances tradeoffs.

**Gap to close**
- Produce artifacts: problem statement, option set, tradeoff analysis, and a recommended path with rationale.

### 3) Architecture & correctness over time
- Senior: designs within a service boundary; focuses on current behavior.
- Staff: designs for evolution, compatibility, and long-term correctness across boundaries.

**Gap to close**
- Write compatibility plans, semantic standards, deprecation paths, and “how we’ll know it’s correct” test strategy.

### 4) Execution via leverage
- Senior: high output via personal execution.
- Staff: high output via enabling others (libraries, patterns, tooling, docs, migrations).

**Gap to close**
- Build paved roads and make adoption easy: default integrations, templates, migration guides, and guardrails.

### 5) Influence without authority
- Senior: influences within the team via reviews and pairing.
- Staff: aligns multiple teams with different incentives via crisp comms and shared success metrics.

**Gap to close**
- Run structured alignment: stakeholder map, decision log, RFC review loop, and clear “why now” narrative.

### 6) Operational excellence & risk management
- Senior: fixes issues and improves tests/monitoring locally.
- Staff: reduces risk systematically (rollout plans, feature flags, SLO/SLI alignment, incident learning).

**Gap to close**
- Make rollouts boring: staged deployment, observability, backout plan, and ownership model.

## “Move to staff” checklist (what I should do differently)

- Write an **exec summary** first (problem, impact, proposal, timeline, ask).
- Define a **behavioral standard** (semantics, invariants, ownership) before writing code.
- Prefer “make the right thing easy” over “tell people what to do” (tooling/templates/defaults).
- Measure outcomes (latency, correctness, adoption, incident rate, engineer time saved).
- Delegate by creating crisp tasks others can execute safely.
- Close the loop: post-launch evaluation + documented lessons learned.

## How I’ll track this repo toward Staff

For every “staff track” project, I want these artifacts:
- Problem statement + success metrics
- Constraints + stakeholders
- Options + decision record (why this approach)
- Rollout plan + backout plan
- Validation plan (tests, monitoring, invariants)
- Post-launch results (what changed, what we learned)

Primary project for 2026: `train-for-staff/staff-project.md`.
