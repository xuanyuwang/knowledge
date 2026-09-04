# Workstream 4: Career Hypotheses

**Status:** First synthesis pass
**Date:** 2026-09-03

## Purpose

Turn the recurring-strength and seniority assessments into distinct, testable descriptions of the engineer I could choose to emphasize next.

These are positioning hypotheses, not permanent identities. Each should be tested against work I want to do, roles that actually exist, and feedback from hiring managers, recruiters, interviews, and future projects.

## Stable evidence underneath every hypothesis

The strongest repeated evidence is not a programming language or one project. It is the combination of:

- reconstructing ambiguous failures across system boundaries;
- making identity, time, state, eligibility, revision, and lifecycle semantics explicit;
- reframing local symptoms into the right system problem;
- making proportionate architecture and rollout decisions;
- carrying work through realistic validation and production read-back.

The current seniority calibration is an established, high-performing Senior Engineer with repeated Staff-like behavior inside bounded technical domains. None of the hypotheses should imply organization-wide Staff impact that has not been demonstrated.

## Candidate 1: Cross-system data correctness and reliability engineer

### Positioning statement

> Senior backend systems engineer who diagnoses and prevents correctness failures across APIs, asynchronous workflows, PostgreSQL, ClickHouse, and analytics systems, then carries fixes through safe rollout and production verification.

### Value proposition

I make distributed product data trustworthy when multiple systems can each look locally correct but disagree as a whole.

### Supporting evidence

- The scorecard PostgreSQL-to-ClickHouse work traced stale asynchronous state and ORM lost updates as independent causes, redirected a failed first solution with load evidence, and verified the final design in production.
- The scorecard monitor, auto-heal design, repair work, and operational playbooks move the problem from reactive customer repair toward detection, classification, bounded recovery, and convergence verification.
- Analytics discrepancy work repeatedly distinguishes storage drift from revision, eligibility, attribution, time, and aggregation semantics.
- External-table rollout and customer data repair show strong feature-flag, validation, and backout discipline.

### Contradictory or limiting evidence

- The strongest examples cluster around scorecards and analytics; breadth across availability, performance, security, or lower-level infrastructure is limited.
- “Reliability engineer” may be misread as SRE, infrastructure operations, or availability engineering, which the evidence does not primarily show.
- Business impact and long-term incident reduction are not consistently measured.

### Natural targets

- Senior Backend Engineer, data-intensive product systems
- Senior Distributed Systems Engineer, where correctness matters more than low-level consensus specialization
- Senior Data Reliability or Data Platform Engineer
- Backend roles involving transactional systems, event pipelines, analytics projection, reconciliation, or migrations

### What this deemphasizes

- Product-semantic and cross-functional translation work
- Reusable product-infrastructure design outside failure diagnosis
- Three years of frontend experience
- Emerging domain stewardship and team-leverage ambitions

### Main positioning risk

This can become a “production debugger” or “database consistency specialist” box. It should emphasize prevention, architecture, and ownership—not only difficult incident response.

### Day-to-day fit hypothesis

The repository shows strong capability and sustained investment in this work, but it does not prove that a role dominated by incidents and reconciliation is desirable. Test whether the appealing part is deep diagnosis and system design or whether repeated reactive correctness work would become too narrow.

## Candidate 2: Product infrastructure and data-platform builder

### Positioning statement

> Senior product-infrastructure engineer who turns recurring backend and data constraints into reusable mechanisms, explicit contracts, and safe migration paths for complex product teams.

### Value proposition

I build the backend and data foundations that let product engineers ship complex workflows without repeatedly rediscovering scale, semantics, or correctness traps.

### Supporting evidence

- External tables reframed a query-size failure into a general reference-data transport mechanism, with option analysis, generic helpers, broad caller adoption, feature flags, and staged rollout.
- User-filter work created implementation-independent behavioral rules and tests for historically divergent semantics.
- Provider boundaries, canonical filter direction, verification tools, repair playbooks, and domain references all aim to make future changes safer and cheaper.
- The stated growth goal is to convert investigation judgment into assets other engineers can use without making me the critical path.

### Contradictory or limiting evidence

- Reusable mechanisms are well documented, but use by other engineers and measured development-time savings are sparse.
- The portfolio does not yet show ownership of a broad internal platform, platform roadmap, service-level product, or adoption program across multiple teams.
- “Platform” could overstate current organizational scope if it is not qualified as product infrastructure or data-heavy backend systems.

### Natural targets

- Senior Product Infrastructure Engineer
- Senior Backend Platform Engineer
- Senior Data Platform Engineer for application-facing data systems
- Backend roles building shared services, query infrastructure, authorization/filtering primitives, workflow foundations, or migration tooling

### What this deemphasizes

- The exceptional depth of the scorecard consistency investigation
- Customer-specific diagnosis and repair
- Product-domain expertise as a primary differentiator
- Frontend delivery, except as useful cross-layer context

### Main positioning risk

The hypothesis can sound more proven than it is. The safe claim is that I have built reusable mechanisms and paved-road candidates; broad platform adoption and organization-level leverage remain outcomes to demonstrate.

### Day-to-day fit hypothesis

This aligns most directly with the documented desire to turn recurring investigation knowledge into maintained tools, standards, and workflows used by others. Preference confidence is **medium**, not high, because the repository does not yet say whether sustained platform maintenance, internal-customer support, and adoption work are enjoyable.

## Candidate 3: Domain-oriented product systems engineer

### Positioning statement

> Senior product systems engineer who turns ambiguous business rules in data-heavy SaaS products into explicit cross-layer contracts and owns the result from product definition through backend design and production validation.

### Value proposition

I make technically complex product domains understandable and correct by connecting user-visible meaning to APIs, workflows, persistence, analytics, and operational behavior.

### Supporting evidence

- Leaderboard work distinguished agent versus submitter attribution, aggregation versus drill-down grain, and conversation versus submission time before selecting an API direction.
- Scorecard workflow and analytics stewardship organize scattered lifecycle, permission, revision, scoring, and metric rules into durable domain models.
- Training Simulator reporting design defined defensible denominators and latest-attempt rules while refusing to expose ambiguous completion or N/A semantics.
- The self-review expresses a desire to enter roadmap and customer-impact discussions earlier and connect technical strategy to measurable product outcomes.

### Contradictory or limiting evidence

- Cross-functional alignment is inferred more often than directly documented; the record rarely names the PM or stakeholder decision that changed.
- Several of the strongest domain artifacts prove reasoning quality, not shipped adoption or a multi-quarter product outcome.
- The evidence supports senior technical partnership, but not yet a broad organizational technical-lead claim.

### Natural targets

- Senior Product Backend Engineer in analytics, workflow, fintech, enterprise SaaS, or other semantics-heavy products
- Senior Product Systems Engineer
- Technical lead for a bounded backend-heavy product domain
- Backend roles requiring close PM, design, data, and frontend collaboration

### What this deemphasizes

- Distributed-data correctness as a specialized differentiator
- General-purpose platform and infrastructure building
- Low-level systems depth
- Technology-specific credibility that may help initial recruiter matching

### Main positioning risk

“Product systems engineer” is less standardized in the market and can become vague. It needs concrete language about backend ownership, data semantics, and the types of products involved.

### Day-to-day fit hypothesis

The growth goals support earlier customer and roadmap involvement, but the repository does not establish a preference for the meeting, negotiation, prioritization, and stakeholder load that comes with domain technical leadership. Preference confidence is **medium-low** until tested directly.

## Comparison

| Question | Candidate 1: Correctness and reliability | Candidate 2: Product infrastructure | Candidate 3: Product systems |
|---|---|---|---|
| Strongest proof | Scorecard consistency, monitoring, repair, verification | External tables, user-filter contracts, reusable tools and paths | Leaderboard semantics, domain stewardship, reporting contracts |
| Primary value | Makes cross-system data trustworthy | Makes future product engineering safer and easier | Makes complex business behavior explicit and correct |
| Typical work | Diagnosis, prevention, reconciliation, migration, validation | Shared backend/data mechanisms, standards, migrations, adoption | Product framing, cross-layer design, domain ownership, outcome validation |
| Most credible current level | Strong Senior with bounded Staff-like stories | Strong Senior with emerging Staff-like leverage | Strong Senior technical partner with bounded Staff-like framing |
| Biggest evidence gap | Breadth and long-term reliability outcomes | Adoption and measured team leverage | Cross-functional influence and shipped business outcomes |
| Biggest fit question | Would too much reactive work be draining? | Is internal-platform ownership and adoption work desirable? | Is sustained stakeholder-heavy domain leadership desirable? |

## Working hypothesis to test first

Candidate 2 is the best **forward-looking umbrella**, anchored by Candidate 1's strongest proof:

> Senior backend and product-infrastructure engineer who owns ambiguous data-correctness and business-semantic problems across system boundaries, turns them into explicit reusable contracts, and carries solutions through safe production validation.

Why lead with this version:

- It preserves the strongest evidence in backend systems, data correctness, and production rigor.
- It describes value beyond debugging by including reusable mechanisms and contracts.
- It leaves room for customer-facing product context without claiming broad product leadership.
- It targets a wider and likely better-fitting role set than “scorecard/analytics specialist” or generic “distributed systems engineer.”
- It expresses the documented growth direction—make judgment reusable—without claiming that organization-wide leverage is already proven.

This is a testable working hypothesis, not a final selection. Candidate 1 should be used for reliability-heavy opportunities; Candidate 3 should be used for product-domain roles where semantic complexity and PM partnership are central.

## Thirty-second memory

A hiring manager should remember:

> He makes complex product and data systems trustworthy. He is strongest when the problem is ambiguous, the semantics cross several layers, and success requires both a precise technical model and production proof.

This is a memory aid, not resume copy. Specific claims still need project evidence and role-appropriate language.

## Market and preference tests

### Test the role market

For each candidate, collect five plausible job descriptions and record:

- whether the actual responsibilities match the hypothesis rather than only the title;
- which required experiences are already proven;
- which terms repeatedly produce false-positive roles;
- whether the role offers ownership through rollout and follow-up;
- whether it provides a credible path from strong Senior to broader Staff scope.

### Test the narrative

Use recruiter and hiring-manager conversations to learn:

- which phrase makes the experience immediately legible;
- whether “product infrastructure” needs more concrete explanation;
- whether the scorecard-sync story is heard as architecture and ownership or only debugging;
- which missing proof blocks Senior versus Staff consideration;
- which two additional stories make the profile feel broad rather than repetitive.

### Test personal preference

After representative work or interviews, record energy and interest separately for:

- deep diagnosis and incident-like ambiguity;
- building shared mechanisms and supporting adoption;
- product semantics and cross-functional decision-making;
- long-term operational ownership;
- mentoring, delegation, and execution through others.

Do not infer preference solely from demonstrated competence.

## Implications for current artifacts

- The current resume summary is closest to Candidate 1. Workstream 6 should decide whether to widen it toward the working Candidate 2 hypothesis and should validate claims such as “across teams” against direct evidence.
- Interview stories should pair the scorecard consistency narrative with one infrastructure-building story and one product-semantic story.
- Workstream 5 should translate the working hypothesis into role, team, mandate, and company-fit criteria from both sides of the hiring decision.
- Evidence recovery should prioritize actual use of the external-table/filtering paths, team adoption of investigation assets, named stakeholder decisions, and measurable customer or engineering outcomes.

## Sources

- [Workstream 2 recurring-strength synthesis](workstream-2-recurring-strengths.md)
- [Workstream 3 seniority calibration](workstream-3-seniority-calibration.md)
- [FY27 mid-year self-review draft](fy27-mid-year-self-review-draft.md)
- [Staff project portfolio](../staff-project.md)
- [Current resume](../resume.typ)
- Canonical product-domain evidence linked from those artifacts

## Maintenance rule

Keep at least two meaningfully different hypotheses until role and preference feedback consistently favors one. Revise a hypothesis when new evidence changes its proof, target market, or desirability—not merely to mirror the wording of one job description.
