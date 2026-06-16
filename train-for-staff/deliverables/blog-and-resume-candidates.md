# Blog and Resume Candidates

**Created:** 2026-06-05

This pass picks up newer staff-level material after the first three polished examples in `staff-project.md`.

## Blog candidates

### 1. Turning Ticket Work into a Domain Reference

Draft:

- `train-for-staff/deliverables/turning-ticket-work-into-a-domain-reference.md`

Source projects:

- `scorecard-template/README.md`
- `scorecard-template/deliverables/scorecard-template-system-reference.md`
- `scorecard-template/deliverables/business-rules-catalog.md`
- `scorecard-template/deliverables/ticket-pattern-log.md`
- `template-schema-version-updater/README.md`

Core argument:

Staff leverage in a business-rule-heavy domain often comes from making repeated ambiguity visible. The scorecard/template work is a good example: each ticket exposed a local bug, but the reusable value came from building a domain map, lifecycle model, rule catalog, and improvement backlog.

Possible outline:

1. The symptom: every scorecard/template ticket required rediscovering the same concepts.
2. The real problem: business rules were split across UI behavior, proto, DB shape, analytics projection, and historical tickets.
3. The intervention: create a living working reference rather than another one-off investigation doc.
4. The staff move: turn repeated local pain into shared vocabulary, lifecycle stages, rule buckets, and concrete improvement proposals.
5. The follow-up: explicit template schema versioning as an example of a small systemic improvement born from the reference.

Resume angle:

- Domain stewardship, cross-functional clarity, reducing repeated investigation, making future changes safer.

### 2. When a Metric Changes Its Subject: Agent vs Submitter Semantics

Source projects:

- `convi-6968-schwab-leaderboard-launch/README.md`
- `convi-6968-schwab-leaderboard-launch/deliverables/api-decision-table.md`
- `convi-6968-schwab-leaderboard-launch/deliverables/BE plan.md`

Core argument:

A leaderboard feature looked like a frontend drawer and metric addition, but the real decision was semantic: agent-scoped filters, submitter-scoped filters, reviewer audiences, and time basis all answer different business questions. The useful staff artifact was the API decision table that separated current behavior, MVP behavior, and the future backend contract.

Possible outline:

1. The request: add submitted-scorecard counts and template breakdowns to leaderboard.
2. The trap: existing APIs have similar names but different entity grain, time basis, and attribution semantics.
3. The analysis: compare `RetrieveQAScoreStats`, `RetrieveQAConversations`, `RetrieveScorecardStats`, and `ListScorecards`.
4. The decision: use an MVP provider abstraction while planning the backend path toward explicit agent and submitter axes.
5. The lesson: metric work needs a semantic contract before code reuse.

Resume angle:

- API strategy, analytics correctness, cross-layer product semantics, future-proofing with provider abstraction.

### 3. Fail-and-Freeze as a Safer UX Pattern for Permission Drift

Source projects:

- `convi-6862-disable-editing-on-submitted-scorecard/README.md`
- `convi-6862-disable-editing-on-submitted-scorecard/deliverables/submitted-scorecard-edit-permission-fail-freeze-plan.md`
- `convi-6862-disable-editing-on-submitted-scorecard/decisions/2026-05-29-final-fe-submitted-editor-behavior.md`

Core argument:

Submitted scorecard edit permissions can change while a user is already editing. The safer frontend pattern is not a fake preflight write or optimistic continuation; it is reactive rollback plus freeze when the authoritative write fails with permission denied.

Possible outline:

1. The product problem: submitted scorecards can remain visible but become no longer editable.
2. The tempting approach: preflight "sniffing" writes.
3. The chosen approach: fail-and-freeze on real permission denial.
4. The UX contract: restore persisted state, disable further edits, show inline warning, suppress noisy handled toasts.
5. The system lesson: permission drift should be handled as a runtime consistency problem.

Resume angle:

- Risk management, UX correctness, permission semantics, rollback behavior, avoiding extra write traffic.

### 4. From Scorecard APIs to Business Workflows

Draft:

- `train-for-staff/deliverables/from-scorecard-apis-to-business-workflows.md`

Published:

- `blog/2026-06-15-from-scorecard-apis-to-business-workflows.md`

Source projects:

- `scorecard-template/README.md`
- `scorecard-template/deliverables/scorecard-template-domain-skeleton.md`
- `scorecard-template/deliverables/workflow-map.md`

Core argument:

Generic artifact APIs work while a product has one dominant workflow. Once calibration, group calibration, appeal, analytics, and repair workflows emerged, names like `updateScorecard`, `submitScorecard`, and `createScorecard` hid too much business meaning. The useful shift was separating domain artifacts from behavioral frames, then designing workflow-specific commands over small artifact primitives.

Resume angle:

- Domain modeling, API semantics, business workflow design, moving from code-level abstraction to product-level architecture.

## Resume candidates to polish next

These are not final bullets yet. They are high-signal raw material to refine once metrics, PRs, and adoption outcomes are known.

### Scorecard/template domain stewardship

- Built a living scorecard/template domain reference covering lifecycle, business rules, concept map, ticket-pattern log, and system sharp edges across Director, coaching service, Postgres, ClickHouse, and proto boundaries. Reframed repeated scorecard/template bugs from isolated fixes into a domain stewardship problem, creating reusable artifacts that reduce repeated investigation and surface small systemic improvements such as explicit template schema versioning.

### Leaderboard metric semantics and API strategy

- Led API and product-semantics analysis for adding submitted-scorecard leaderboard metrics and template drill-downs. Compared four candidate APIs across entity grain, filter parity, template support, attribution basis, and time semantics; identified agent-vs-submitter gaps in existing QA APIs; and proposed an MVP provider abstraction plus a backend path using explicit submitter filtering and `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER`.

### Submitted scorecard permission drift

- Designed a fail-and-freeze UX contract for submitted scorecards when edit permissions change during an active session. Rejected preflight write checks in favor of authoritative write failure handling: rollback to persisted state, freeze further edits, show an inline warning, and preserve existing behavior for unrelated failures.
