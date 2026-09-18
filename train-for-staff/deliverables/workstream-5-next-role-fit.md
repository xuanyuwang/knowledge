# Workstream 5: Next-Role Fit

**Status:** Revised for current work–life priorities
**Date:** 2026-09-18

## Purpose

Translate the working career hypothesis into criteria for deciding both:

> Why should this company hire me?

and:

> Why should I take this job?

The working hypothesis is a Senior backend and product-infrastructure engineer who owns ambiguous data-correctness and business-semantic problems across system boundaries, turns them into explicit reusable contracts, and carries solutions through safe production validation.

## Why a company should hire me

The most defensible answer is:

> I am strongest in data-intensive product systems where a visible issue crosses APIs, workflows, persistence, analytics, and user-facing semantics. I reconstruct the real failure or contract, make the tradeoffs explicit, and own the result through rollout and production verification. I also look for the reusable mechanism or standard that prevents the next team from solving the same class of problem again.

Evidence for this answer includes:

- diagnosing independent asynchronous and ORM races behind scorecard inconsistency;
- reframing oversized user filters into a general ClickHouse reference-data mechanism;
- defining user-filter and metric semantics before implementation;
- building validation, monitoring, and repair paths rather than stopping at merge;
- converting repeated scorecard and analytics work into durable system references and playbooks.

The documented technical claim remains at strong Senior scope. Organization-wide adoption and execution through others are not required next-job goals. The user reports prior organizational influence at IBM; specific outcomes need recovery before external claims, not another job undertaken to prove capability.

## Why I should take a job

A next role should improve financial security and opportunities for my wife and William, help make employment optional over time, and leave time, health, and attention for present family life and intellectual interests. Evaluate **financial value and interesting technical work per unit of life and attention surrendered**. This is a decision framework, not a literal formula that prices health or family time.

Higher compensation is a major objective. It is evaluated together with normal and peak hours, on-call, stress, organizational dysfunction, autonomy, engineering quality, recurring maintenance, funded root-cause work, learning, technical interest, stability, flexibility, and psychological detachment after work. Neither title nor broader influence substitutes for a good exchange.

See the [governing career position](career-position.md) for the agency-ceiling pattern and bounded response to organizational problems. A role need not cure every structural issue. It must not require me to personally compensate for missing organizational support indefinitely.

## Target role families

### Primary

1. **Senior Product Infrastructure Engineer**
   - Best match when “infrastructure” means application-facing backend and data foundations used by product teams.
   - Look for shared services, query infrastructure, workflow foundations, migration systems, authorization/filtering primitives, or correctness platforms.

2. **Senior Backend Engineer, data-intensive product systems**
   - Best match when the role owns complex workflows across transactional and analytical stores.
   - Look for event-driven state, reconciliation, data lifecycle, reporting, or high-scale enterprise features.

3. **Senior Data Platform Engineer, application-facing**
   - Best match when the platform serves product and engineering use cases rather than only warehouse ingestion or analytics engineering.
   - Look for APIs, serving systems, data contracts, lineage, correctness, and operational ownership.

### Conditional

4. **Senior Distributed Systems Engineer**
   - Fit only when the role values application-level consistency, state transitions, asynchronous workflows, and production reasoning.
   - Question roles centered on consensus protocols, storage-engine internals, kernel work, or networking depth not supported by the current portfolio.

5. **Senior Product Backend Engineer / bounded technical lead**
   - Fit when the product is semantics-heavy and the role includes product framing, cross-layer contracts, and backend ownership.
   - Validate that “product” does not mean feature throughput without authority over architecture or correctness.

6. **Staff Engineer**
   - Consider when the company defines a bounded technical mandate that matches demonstrated capability, pays well, and protects boundaries; broader influence is optional, not the purpose.
   - Avoid relying on title equivalence; ask what decisions, teams, time horizon, and outcomes the role actually owns.

## Decision gates and positive signals

Before committing, establish that:

- The compensation package offers a credible, meaningful improvement in family financial value, assessed against current compensation and costs. The exact threshold is not yet specified.
- Expected hours, peak periods, on-call, recovery time, and availability expectations leave acceptable family time, health, and mental attention. Exact personal limits remain to be set.
- Substantial work is technically interesting and offers learning, preferably backend/software infrastructure, deep systems, or computational problems. No arbitrary percentage is a settled requirement.
- Responsibility has matching authority, time, staffing, and manager support. Engineers can address worthwhile root causes within normal hours and can accept documented residual risk when leadership declines investment.
- Success does not depend on becoming a perpetual escalation point, organizational rescuer, or political owner. Strong technical contribution is valued without mandatory management or ever-broader influence.

Evidence of good fit includes bounded production ownership and shared support; safe rollout and verification practices; protected engineering time; a concrete example of funded prevention; respectful decisions when a proposal is declined; useful technical depth within a well-defined scope; and employees who can actually disconnect.

A failed personal boundary is not offset by an aggregate score. An unknown is a question to resolve, not a pass or automatic rejection. At posting stage, pursue promising roles with specific unknowns; before accepting, resolve consequential uncertainty.

## Warning signs and probes

- Chronic firefighting, routine nights/weekends, or an unstaffed support queue described as ownership.
- “Transform engineering,” “wear every hat,” or “drive change without authority” without funded time, sponsorship, and explicit boundaries.
- Attractive salary that depends mainly on uncertain equity or repeated exceptional hours.
- Root-cause proposals repeatedly deferred while the individual remains indefinitely accountable for symptoms.
- Promotion or respect available only through increasing political/organizational ownership.
- Technically shallow repetition, despite a prestigious title or company name.

Do not automatically reject a narrow component role, well-scoped tickets, maintenance, or a company with organizational problems. Ask whether the actual work is interesting, maintenance is proportionate, support is shared, and the overall exchange is worthwhile. Infrastructure and data-platform labels can also hide mostly operations or ETL work; assess the actual work rather than the label. Low-level or mathematical specialties may be interesting but need a separate skills-fit assessment; do not claim expertise from interest alone.

## Team and company hypotheses

The repository does not establish a preferred company stage. Treat these as tests:

| Environment | Potential fit | Primary risk | What to verify |
|---|---|---|---|
| Growth-stage product company | Broad backend/data ownership and visible customer impact | Chronic urgency may crowd out durable leverage | Roadmap stability, on-call load, engineering authority, manager support |
| Mature product company | Complex systems, operational support, potentially predictable boundaries | Work may be uninteresting or coordination-heavy | Technical depth, bounded authority, actual hours, support and internal mobility |
| Infrastructure/platform company | Deep technical systems and reusable product mechanisms | Work may be lower-level than the demonstrated or desired profile | Application-facing context, customer interaction, expected systems depth |
| Early startup | Maximum ambiguity and product connection | Weak mentorship, unstable priorities, hero culture | Runway, engineering leadership, sustainability, quality expectations |

## Opportunity scorecard

Use these **provisional weights**, proposed from the stated priorities rather than user-approved numerical preferences. Rate known categories 0–4 and record evidence, confidence, and open questions. Unknowns remain unscored; show known-weight coverage and a possible total range rather than inventing a midpoint. No automatic 70/100 cutoff remains. Compare options with the current job using the same assumptions, and check whether reasonable weight changes reverse the ranking.

| Category | Weight | 0 | 4 |
|---|---:|---|---|
| Financial value | 25 | Little credible improvement after costs/risks | Meaningful, durable improvement in family security and savings capacity |
| Time, health, and mental boundaries | 25 | Chronic stress, interruption, or inability to disconnect | Predictable hours, bounded on-call, recovery, and protected attention |
| Technical interest and learning | 20 | Repetitive or uninteresting work | Deep reasoning, meaningful engineering, sustained learning |
| Autonomy and funded root-cause work | 15 | Responsibility without support; endless symptom handling | Authority and normal-hours capacity for proportionate durable fixes |
| Engineering and manager quality | 10 | Hero dependence, dysfunction, unclear decisions | Sound practices, shared ownership, supportive and realistic decisions |
| Stability and flexibility | 5 | High unwanted risk or rigid life constraints | Credible stability and flexibility compatible with family life |

For fully evidenced categories, contribution is `(rating / 4) × weight`. Title, prestige, promotion speed, and organizational scope receive no independent points. Record risks separately so a high total cannot conceal unacceptable boundaries.

## Compensation versus workload

For each serious opportunity, use the same currency and time horizon and record:

- Base, realistically expected bonus, equity vesting/liquidity and downside, benefits, retirement contributions, leave, and one-time versus recurring compensation. Do not treat speculative equity as guaranteed cash.
- Incremental commuting, relocation, childcare, travel, and other work-related costs; tax effects only when inputs and a suitable calculation are available.
- Normal and peak weekly hours, frequency of peaks, actual on-call interruptions, standby restrictions, commute/travel time, meeting load, and recovery days.
- Expected reliable annual financial gain versus the current job, with conservative/base/upside scenarios and assumptions. Any savings or financial-independence projection needs actual household inputs; no date is assumed here.
- A rough reliable-compensation-per-annual-work-hour comparison as a diagnostic, with commute and active on-call made explicit. Show standby constraints, stress, sleep disruption, and attention spillover separately; they are not captured by hourly pay.

A large pay increase with similar or better boundaries is attractive. A higher headline package with much greater life cost requires explicit consideration and may be declined. There is no predetermined salary premium that makes unacceptable boundaries worthwhile.

## Interview questions to ask

| Dimension | Questions and evidence to request |
|---|---|
| Hours and detachment | What did a typical week and the busiest recent week look like for someone in this role? How often do evenings/weekends occur? What response is expected after hours or on vacation? |
| On-call | How large is the rotation, how often does it page overnight, and how many incidents required active work recently? Who handles escalation, and what recovery time follows? |
| Root-cause capacity | Describe a recurring problem the team fixed at its root. Who allocated time and authority? What happens when leadership declines such a proposal? |
| Maintenance and stress | How much of the last quarter was roadmap work, maintenance, incidents, and recurring support? Is the load improving, and who covers it when the expert is away? |
| Autonomy and organizational burden | Which decisions could I make directly? Who owns cross-team alignment? Is changing the organization an explicit success criterion for this role? |
| Technical depth | What difficult engineering problem would I work on in the first six months? What could I learn in year two? How much time goes to coding/design versus meetings and support? |
| Engineering quality | Tell me about a rollout that disproved an assumption. What changed in testing, observability, architecture, or process afterward? |
| Stability and flexibility | How have priorities and staffing changed? What are location, core-hours, leave, travel, and family-scheduling expectations in practice? |
| Technical career fit | Can an excellent engineer remain primarily technical at this scope? How are they valued without continually expanding organizational ownership? |
| Financial package | What is recurring and guaranteed versus discretionary, one-time, or illiquid? What conditions affect bonus, equity, benefits, and future pay? |

Ask the manager and future peers for concrete recent examples. Record disagreements and confidence rather than resolving them in favor of the most reassuring answer.

## Practical pursuit and decision workflow

1. **Posting screen:** Record actual technical work, compensation range if given, location/flexibility, on-call language, and transformation expectations. Classify as pursue, investigate, or decline with reasons; missing information is not proof of fit.
2. **Company/team evaluation:** Test team behavior, manager support, staffing, engineering quality, and stability. Company prestige and stage do not establish team fit.
3. **Interview diligence:** Use the questions above to verify the highest-impact unknowns. Check for the agency ceiling: is responsibility wider than supported decision power?
4. **Offer comparison:** Compare the current job and alternatives using financial scenarios, life costs, decision gates, and the scorecard. State which tradeoffs are acceptable and which remain unresolved.
5. **Decision:** Pursue/accept when the overall exchange is worthwhile; investigate consequential unknowns; decline when the work or life cost fails personal criteria. No offer has been evaluated or accepted by this document.
6. **Revisit:** After interviews or actual work, update preference confidence. If organizational problems emerge, identify → understand → propose → observe response → maintain reasonable boundaries → reassess. A bounded attempt is sufficient; leaving is not automatically failure.

For each opportunity, save a short record: role/team/date; technical work and fit; compensation scenarios; normal/peak hours and on-call; attention cost; authority/resources; stability/flexibility; evidence sources and confidence; unresolved questions; comparison to current role; decision and rationale. Use the existing career work item and linked sessions, not a duplicate career tracker.

## Sources

- [Workstream 3 seniority calibration](workstream-3-seniority-calibration.md)
- [Workstream 4 career hypotheses](workstream-4-career-hypotheses.md)
- [FY27 mid-year self-review draft](fy27-mid-year-self-review-draft.md)
- [Career positioning operating brief](career-position.md)

## Maintenance rule

Update these criteria from real opportunities. Record which filters predicted fit, which produced false positives, and which preferences changed after interviews or representative work.
