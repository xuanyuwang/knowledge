# Session Note - 2026-08-16 - Codex - Lesson/module statistics plan publication

**Started:** 2026-08-16
**Tool:** Codex
**Project:** `training-simulator`
**Goal:** Reconcile the backend/frontend lesson-module statistics plans against the rewritten domain README and publish the combined engineering design for review.

## Source Context

- **Primary durable repo:** `/Users/xuanyu.wang/repos/knowledge` (main checkout; no knowledge worktree)
- **Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`
- **Related repos:** `/Users/xuanyu.wang/repos/cresta-proto`, `/Users/xuanyu.wang/repos/director`
- **Validated refs:** `go-servers/main` `01a2c4ecbfbb7f90145898fa8c200687266eeaed`; `cresta-proto/main` `df2b436d03772894dbb467bd9eb02cb3e43b4099`; `director/main` `4a0962688d73a75631dfd35483c437c1c0c0eafe`
- **Product code changes:** none
- **Credentials/session used:** existing authenticated Superhuman Docs/Coda connector session; no credential files were inspected.

## Inputs Reviewed

- `training-simulator/README.md`
- `training-simulator/project.yaml`
- `training-simulator/deliverables/lesson-module-statistics-eng-design.md`
- `training-simulator/deliverables/lesson-module-statistics-fe-design.md`
- `training-simulator/work-items/lesson-module-statistics-reporting.md`
- Superhuman Docs engineering-design template at `https://docs.superhuman.com/d/_dE0dCcz8Bub/Untitled-page_subF07cy`

## Plan Reconciliation

The rewritten domain README materially changed the correctness bar for reporting. Both plans were updated so implementation cannot treat current-content lookup or score/pass fields alone as historical truth.

Backend corrections:

- Assignment-time lesson/module revision snapshots are immutable inputs to run creation and aggregation.
- Evaluation resolves the exact module revision recorded on the run.
- Persisted outcome state distinguishes pending/partial, applicable pass, applicable fail, and complete N/A.
- Genuine N/A is never inferred from all criterion results being N/A because timeout snapshots share that encoding.
- ACTIVE and ARCHIVED assignments remain reportable; DRAFT and DELETED are excluded, pending product confirmation.
- Conversation outcomes ship first. Quiz attempts expose attempt/started facts only until subtype completion/pass/N/A semantics are approved.
- Ambiguous legacy outcomes remain outside score/pass/N/A denominators and raise `outcome_status_missing`.

Frontend corrections:

- Surface missing-snapshot, mixed-revision, and missing-outcome-status warnings.
- Never render ambiguous legacy/partial outcomes as 0%, failed, or N/A.
- Render quiz outcomes as unavailable rather than deriving pass in Director.
- Wait for snapshot and explicit outcome-state contracts before implementing authoritative rollups.

## Publication

- Decoded the supplied Superhuman Docs URL with the authenticated Coda/Superhuman connector.
- Inspected the existing engineering-design template and content-write guidance.
- Retitled the page to **Training Simulator Lesson and Module Statistics — Engineering Design**.
- Replaced boilerplate with the combined plan while preserving the template section model: Goal, Non-goals, Background, Overview, User Experience, Detailed Design, API, Storage, Security & Privacy, Monitoring, SLO, Testing, Technical Debt, Cost, Release, and Review Notes.
- The connector rejected the first oversized atomic body operation without changing canvas content. The design was then written successfully in bounded blocks within the connector's limits.
- Read-back verified the title, required sections, and final product/backend/security/FE-QA review gates.

Published artifact: [Training Simulator Lesson and Module Statistics — Engineering Design](https://docs.superhuman.com/d/_dE0dCcz8Bub/_subF07cy)

## Outcome and Impact

The local plans and external review document now agree on the domain's assignment grain, latest-attempt key, score scales, revision requirements, partial/N/A ambiguity, archived-history behavior, quiz limitations, and authorization boundary. This removes a major risk that the new UI would display internally consistent but historically or semantically false metrics.

## Follow-up

1. Product confirms task lifecycle/date scope, first reporting route, metric set, quiz semantics, and all-N/A display.
2. Backend/data review assignment snapshots, outcome-state storage/migration, exact-revision evaluation, and the representative query plan.
3. Security approves initial roles and the future manageable-user scope.
4. FE/BE freeze score scale, optional/empty/warning behavior, response ordering, and batch size.
5. Size and create implementation/QA tickets after the gates close.
