# Session Note - 2026-08-16 - Codex - Staff domain walkthrough TOC

**Started:** 2026-08-16
**Tool:** Codex
**Project:** `training-simulator`
**Goal:** Define a staff-level learning path for the Training Simulator domain, beginning with a high-level table of contents

## Source Context

- **Primary durable repo:** `/Users/xuanyu.wang/repos/knowledge` (main checkout; no knowledge worktree)
- **Primary source repo:** `/Users/xuanyu.wang/repos/go-servers` (identified through the workspace registry; no source inspection required for this outline)
- **Related repos:** `/Users/xuanyu.wang/repos/director`, `/Users/xuanyu.wang/repos/cresta-proto`, `/Users/xuanyu.wang/repos/python-ai-services`
- **Product code changes:** none
- **Credentials used:** none

## Inputs Reviewed

- `workflow/ai-operating-model.md`
- `workspace/repos.yaml`
- `training-simulator/project.yaml`
- `training-simulator/README.md`
- all five Training Simulator subdomain READMEs

## Curriculum Shape

The walkthrough should proceed in four parts:

1. Frame the product, ownership boundaries, vocabulary, and end-to-end lifecycle.
2. Traverse the five subdomains: training content, assignment/session, simulation runtime, evaluation, and reporting.
3. Examine cross-cutting staff concerns: invariants, versioning/data correctness, security/privacy, reliability/operations, and scale/cost.
4. Close with codebase navigation, current gaps/roadmap, and the staff-level decision framework for evolving the domain.

The walkthrough should distinguish shipped behavior from designed or exploratory behavior, and treat merged proto/backend implementation as authoritative where older design documents disagree.

## Outcome

- Produced a high-level table of contents only; detailed instruction is intentionally deferred to subsequent turns.
- Rewrote the domain `README.md` as a progressive staff-level learning guide, expanding reviewed Topics 1 and 2 while retaining the implementation reference material for later topics.

## Topic 1 - Product Context, Customer Value, and Business Stage

- Training Simulator replaces difficult-to-scale manual role-play with repeatable scenario-based practice against an AI-simulated customer.
- Its differentiated product loop is practice plus automated evaluation using the same Opera/AutoQA quality language used for production interactions.
- Primary users are agents practicing assigned lessons and supervisors/training leaders authoring content, assigning sessions, and reviewing results.
- Customer value has three layers: scalable practice, consistent/evidence-based assessment, and diagnostics that can inform coaching and content improvement.
- It is a paid QM & Coach add-on that launched as Limited Availability on 2026-07-09, initially with Mutual of Omaha and Snap Finance; the recorded plan was expanded beta in early Q3 and GA in late Q3 2026.
- The shipped minimum vertical slice covers authoring through session-level reporting. Lesson/module diagnostics, richer analytics, quizzes, Synthetic Customers, and permission hardening represent product maturation toward GA; exploratory artifacts must not be mistaken for committed scope.
- Staff-level framing: the product succeeds only if the whole learning loop is trustworthy. Conversation realism, evaluation correctness, historical metric semantics, safe separation from production analytics, and actionable reporting are product concerns as much as implementation concerns.

### Sources

- `training-simulator/README.md`
- `sessions/2026-08-09/claude-training-simulator-domain-setup.md`
- `deliverables/lesson-module-statistics-reporting.md`
- Coaching Training Simulator PRD: https://coda.io/d/_ddKtmYQWQVC/Coaching-Training-Simulator-PRD_suAmSoxf
- Training Simulator engineering design: https://docs.google.com/document/d/1GCeE9XCAVcgetOhYWPvqZJ3hp3qeVhK5YMk4rBkWmd4/edit
- 2026-07-07 SE enablement deck: https://docs.google.com/presentation/d/1lY6UOohDrUn7zxcW_DgfNpq0rvz9z_zz0ZfNJSVktgg/edit?slide=id.g3f0842e9369_0_277#slide=id.g3f0842e9369_0_277

## Topic 2 - Domain Scope, Ownership, and System Boundaries

- Training Simulator owns the end-to-end product outcome and the training-specific semantics connecting content, assignment, simulation, evaluation, and reporting.
- It directly owns training content entities/configuration, training task-run identity, simulator-specific orchestration, evaluation entry points/results, and training reporting contracts.
- It reuses shared platforms rather than owning their generic internals: DirectorTask and Coaching Plan, virtual-agent/LiveKit/media infrastructure, Opera moment detection, AutoQA/scorecard semantics, analytics infrastructure, and notification delivery.
- The primary implementation repo is `go-servers`; public contracts are in `cresta-proto`, product UI is in `director`, and the Python voice runtime participates through `python-ai-services`.
- The boundary is semantic rather than repository-shaped: one product workflow crosses multiple repos and services, while each repo also contains capabilities outside the domain.
- The domain must preserve explicit contracts at shared boundaries, especially conversation source, role/channel identity, content revision references, evaluation readiness/results, authorization scope, and reporting inclusion/exclusion rules.
- Staff ownership means coordinating cross-system changes and defending end-to-end invariants without absorbing ownership of every reused platform.

### Sources

- `training-simulator/README.md`
- `training-simulator/project.yaml`
- the five `training-simulator/subdomains/*/README.md` files

## Topic 3 - Core Vocabulary and Invariants

- Defined the content hierarchy and execution hierarchy separately: lesson → module → scenario/quiz content versus DirectorTask session → task-run attempt → conversation/quiz outcome.
- Distinguished lesson/session, module/scenario, scenario pool/sequence, session/conversation, attempt/official score, completion/pass, N/A/failure, and archive/history.
- Documented the independent content, task, progress, completion, pass, and evaluation state machines.
- Captured twelve compatibility invariants covering ordered composition, one-scenario attempts, subtype outcomes, revision safety, latest-attempt reporting, completion/pass, N/A, score scales, training isolation, participant roles, and resource scope.
- Revalidated vocabulary against current `cresta-proto` contracts and the current task-stats implementation; no product code was changed.

## Topic 4 - End-to-End Lifecycle

- Traced the workflow from content authoring through assignment, Coaching Plan discovery, scenario selection, voice runtime, GoWalter conversation creation, task-run association, asynchronous evaluation, verdict persistence, agent progression, and session reporting.
- Verified that Director does not pre-create the conversation; GoWalter creates it after call connection, after which Director creates the linked task run.
- Documented evaluation and verdict persistence as separate commits with distinct retry paths and failure modes.
- Documented the shorter quiz branch and the current gap between configured `allowed_number_attempts` and enforcement in the conversation pipeline.
- Framed the workflow as a distributed saga with partial-success, idempotency, correlation, revision, and reporting-correctness requirements.

## Topic 5 - Training Content

- Corrected the older “scenario pool plus optional quiz” model: current backend and Director enforce exactly one module content type—one-or-more scenarios or one quiz.
- Documented lesson composition/focus criteria, module content and pass configuration, scenario-to-VA materialization, Opera behavior criteria, quiz questions/answer keys, active/archive lifecycle, and authoring validation.
- Traced append-only revisions for lessons/modules/scenarios/quizzes and mapped stable-child-ID versus pinned-revision relationships.
- Identified authoring consistency risks: remote VA or quiz side effects before parent persistence, assignment snapshot gaps, behavior-reference drift, randomized scenario comparability, and non-persisted quiz weight/auto-fail editor fields.
- Revalidated the content model against tracked upstream refs (`go-servers/origin/main` dated 2026-08-13, `cresta-proto/origin/main` dated 2026-08-10, and `director/origin/main` dated 2026-08-14); no product code was changed.

## Topic 6 - Assignment and Session Model

- Corrected the old per-agent-DirectorTask model: one training DirectorTask can target multiple explicit users; an agent session is the `(task, agent)` projection joined with that agent's attempts.
- Documented audience/content/schedule config, one-lesson product semantics despite the repeated proto field, task creation, best-effort notifications, full-replace updates, unassignment, final-agent archive, and mutable cohort risks.
- Traced agent discovery, Start/Resume/Review actions, ordered module frontier, retry/overdue behavior, latest-attempt identity, and completion/pass status semantics.
- Identified the all-zero-run stats gap: the task list retains the assignment, but `RetrieveTrainingSimulatorTaskStats` returns empty when no task runs exist.
- Updated Topic 3 vocabulary, Topic 4 assignment language, the domain key findings, and the assignment-and-session subdomain to the implementation-accurate grain.

## Topic 7 - Simulation Runtime and Customer AI

- Corrected older design-era language: the current contract has no `CUSTOMER_AI_SIMULATOR` VA kind. Each scenario materializes a revisioned umbrella VA and a `SINGLE_PROMPT_SUB_VA`, both stamped for the Training Simulator app.
- Traced the active Director path: random scenario selection, pinned umbrella VA revision, new platform call ID, training resource metadata, simulated LiveKit launch, GoWalter-owned conversation creation, platform-ID resolution, live transcript subscription, disconnect, and server-owned close.
- Distinguished three independent runtime invariants: `TRAINING_SIMULATOR` source classification requires four metadata keys; conversation agent ownership requires a valid full trainee user name; transcript correctness requires the GoWalter speaker-role inversion.
- Documented that `training_session_id` currently means platform call ID rather than DirectorTask identity, and that the durable assignment link is added only when the task run is created after conversation resolution.
- Captured the distributed failure model, including orphaned VA revisions, delayed or misclassified conversations, fail-open trainee attribution, close/finalization races, and valid conversations without task runs.
- Captured historical reconstruction across task run plus conversation/VA revision, and the runtime quality, privacy, retention, latency, and per-attempt cost implications.
- The workspace registry references `python-ai-services`, but that checkout is absent locally; no claims about its current internal pipeline were treated as revalidated implementation truth.

## Topic 8 - Evaluation, Scoring, Pass/Fail, and N/A Semantics

- Corrected older design-era descriptions: the current evaluator does not directly run an LLM, invoke AutoQA scorecard scoring, poll internally, or persist results. It projects already-produced Opera moment annotations into a response snapshot.
- Documented criterion semantics: `SHOULD_DO_X` establishes applicability; pass requires at least one `DID_DO_X` and no `DID_NOT_DO_X`; annotations without an opportunity marker are N/A; no annotations yet are pending.
- Documented weighted scoring over applicable positive-weight criteria, auto-fail gating, positive-threshold versus zero-threshold behavior, and the 0–100 evaluation to 0–1 task-run conversion.
- Traced Director-owned two-second polling, the tracked 60-second guard, COMPLETE-triggered task-run update, and separate retry paths for evaluation/attempt failure versus verdict-save failure.
- Identified the overloaded criterion encoding where pending and genuine N/A both use `not_applicable=true`; response status disambiguates only during live evaluation.
- Identified that the timeout can persist an incomplete snapshot, including a partial pass, and permanently turn pending criteria into apparent N/A.
- Identified that overall `not_applicable` is not persisted; all-N/A becomes score zero/passed false and is treated by current reporting as a scored completion.
- Identified current-module revision drift and the missing runtime enforcement of the proto's Training Simulator conversation-source precondition.

## Artifacts Updated

- `training-simulator/README.md` — added the reviewed-topic curriculum, Topic 1 product framing, Topic 2 ownership/boundary map, source links, progress statuses, and a preserved domain-reference section
- `training-simulator/log/2026-08-16.md`

## Next Step

- Continue with Topic 9: reporting and metric correctness.
