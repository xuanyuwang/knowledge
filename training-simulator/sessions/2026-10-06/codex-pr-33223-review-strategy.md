# PR #33223 as a code-review skill case study

Date: 2026-10-06 (America/Toronto). Primary source: `/Users/xuanyu.wang/repos/go-servers`, read-only review of GitHub PR #33223 at `68f22ba24fa3d9ed8a43dec8c2d90934e05cadc5`, compared with merged part-1 base `24706da3f3e9c2b97f7d3e0e59701c39b63696ad`. Primary subdomain: evaluation.

## Review question

Develop a review skill that starts with the PR's purpose, tests whether the approach achieves it, and explores side effects. Exclude test and minor build/config files from the primary narrative. Use a construction-step account to make the final state intelligible.

## Evidence and scope

- PR #33222 is merged part 1: storage, response fields, and protection against automatic saves overwriting overrides. PR #33223 is part 2: read-only `EvaluateTrainingConversation` with criterion overrides. PR #33224 is open part 3: persistence through `UpdateTrainingSimulatorTaskRun`.
- The part-2 production diff changes `action_evaluate_training_conversation.go`, adds `conversation_scoring.go` and `grade_override.go`, and adds a historical module revision loader in `training_module_helper.go`. The actual PR commit history shows an initial override path, a pending-evaluation safeguard, and a later switch from `ListMomentAnnotations` to a direct annotation read.
- Inspected surrounding production paths: preexisting automatic save, task-run and conversation-score lookup helpers, `ListMomentAnnotations` archived/deleted-policy filtering, and the existing Training Simulator evaluation reference. Tests and BUILD files were excluded from the reasoning pass; no test run or runtime validation was performed. The local cresta-proto checkout was older than the PR contract, so it was not used as proof of the new fields.

## Purpose and reconstructed transition

The user goal, as stated by PR #33223, is to let an authorized manager preview sparse changes to a saved conversation attempt's criterion verdicts, preserving original results and metadata, recomputing the outcome under the attempt's historical rules, and performing no write. Its dependent save flow belongs to part 3.

1. Start from part 1's saved task run, conversation score, and override-safe automatic writer. The normal evaluator still reads current module rules and calls the moment service.
2. Add a distinct override request branch. Resolve the named run and score, enforce manager/customer/trainee access, verify conversation identity, and load the module revision captured on the run.
3. Merge supplied verdicts by criterion ID into copies of the saved results. Keep unmentioned results, originals, metadata, and evidence. Recompute weights, N/A exclusion, auto-fail, and pass threshold with a scorer shared by normal evaluation.
4. Detect a hidden prerequisite: the frontend may save a polling-timeout snapshot whose pending criteria look exactly like real N/A in storage. Re-evaluate available annotations under historical rules and reject unfinished results or a saved N/A that is now applicable.
5. Correct the annotation source: `ListMomentAnnotations` filters annotations for behaviors on archived or deleted policies. Read the stored annotations directly, scoped to customer, profile, conversation, and historical criterion behavior IDs, so retired policies do not make an old completed attempt appear unfinished.

These are explanatory transitions, corroborated by commits and code. A skill should identify reconstructed steps as a model, not claim that every proposed transition was an actual commit or a proven author intention.

## Review method learned

- Build a purpose contract before reading implementation details: actor, input, successful output, forbidden outcomes, and non-goals. Confirm the stack and compare against the correct base; a naive main-to-head history can include merged upstream work.
- Explain the transition from old state to new in small semantic steps. At each step state the invariant introduced, why it is needed, and what would fail without it. After the narrative, verify the final combined state rather than assuming individually sensible steps compose correctly.
- For approach and side effects, leave the diff along specific dependency paths: API/proto and caller, persisted data shape, shared helpers and callers, upstream annotation filtering, and downstream save behavior. Use bounded scenario matrices for N/A/pending, historical revisions, retired policies, auth, and concurrency.
- Separate direct code evidence, PR claims, inferred behavior, and unverified risk. The code shows a read-only override path and shared scoring; runtime compatibility and broader customer impact were not validated here. In particular, the completion check depends on currently stored annotations and a lossy saved N/A representation. That is a semantic boundary to investigate, not an established defect from this pass.
- “Best approach” cannot be proven from a diff. Compare plausible alternatives only when they change a material tradeoff. Here a durable completion marker would avoid reconstructing completion from annotations, but would require data-contract and migration work beyond part 2. Record the tradeoff and evidence rather than declaring a winner without product and rollout constraints.

## Follow-up

When drafting the skill, make the output include: purpose contract, construction timeline, goal-achievement verdict with evidence, external-path impact map, scenario matrix, findings with confidence, and open questions. Keep test/config review as an optional later validation pass; production behavior remains the primary narrative.

## Interactive construction review

Created [PR #33223 construction review](../../deliverables/pr-33223-construction-review.html), a standalone HTML page with a workflow map and six selectable teaching versions: baseline evaluator; new override branch; merge and shared scoring; the insufficient saved-field completion check; annotation-based completion; and direct stored-annotation reads. The change view computes additions and deletions between successive simulated versions. The page labels these as explanatory, not compile-ready or exact historical commits, and links the actual source and commit landmarks.

The page also records a scenario matrix and a bounded source-level review judgment. One confirmed documentation mismatch remains: the final `evaluateCriterionOverrides` comment says it fetches no annotations, although `finishedAIEvaluation` now loads stored annotations. This is a review clarity issue rather than proof of a runtime defect. The unresolved semantic limit is that the saved representation cannot distinguish a pending N/A from a genuine N/A if currently stored annotations also indicate N/A. Part 3's save boundary remains separate.

Validation: extracted page JavaScript passed `node --check`; HTML parsed with Python's `HTMLParser`; a separate exhaustive comparison of the old and new score formulas matched 43,275 bounded combinations (up to three criteria; N/A, verdict, weights 0/1/2, auto-fail, thresholds 0/50/100). No Go tests or runtime service validation. Local browser preview was unavailable because the browser's security policy blocked `file:` URLs; no alternate browser route was attempted.

## Reader feedback and revision

The user reviewed the page in their browser and flagged four clarity gaps: “New idea” and “What forces the next revision” did not state their meaning; “Version” should be “Code snapshot”; “External paths” lacked an explanation; and each step needed the problem and cause before the code change. The page now gives every step four explicit fields: problem this step addresses, why it exists, what changes, and what remains unresolved. The workflow's adjacent connections are written as complete sentences about stored annotations, the normal evaluation scorer, and the later manual save. All visible “Version” references became “Code snapshot.” A JavaScript smoke check traversed all six steps, checked all four fields, and exercised both snapshot and change views; syntax, HTML parse, and diff checks passed.

The user then asked for a high-level PR summary and proposed that the PR generates a preview of a run on the fly with overrides. Verified against the pinned handler: this is substantially correct when phrased as a preview of the saved attempt's **grade**. The override branch loads the named run's saved criterion results and historical module revision, applies sparse verdict changes, computes an effective grade, and returns it without writing. It also re-evaluates stored moment annotations to confirm completion, but does not rerun AI or use that re-evaluation to replace saved verdicts. The separate module-preview mode is rejected when criterion overrides are present. Added this interpretation and qualification prominently near the top of the HTML report.

## Merge-readiness check

On 2026-10-06, PR #33223 remained open and mergeable at the same head `68f22ba24fa3d9ed8a43dec8c2d90934e05cadc5`; GitHub's merge state was BLOCKED because codeowner approval was pending. `gh pr checks --required` showed `required` and Custom lints passing, with Codeowner check pending. A source pass through the final override branch, shared scorer, stored-annotation query, moment-service list filtering, and downstream save boundary found no demonstrated merge-blocking correctness defect. The direct query is scoped by customer, profile, conversation, and behavior IDs; the schema has a conversation-ID index. The comment claiming no annotations are fetched is stale and should be corrected, but it does not affect runtime behavior. The saved pending/N/A encoding remains a data-model limit; this review did not establish a wrong final grade from the residual ambiguity. Recommendation: near ready on code, await codeowner approval and address the comment when convenient. No code, PR comments, or merge actions were performed.

## Auth and permission terminology

In `go-servers`, `auth` names an umbrella package covering principal extraction, role checks, custom feature/action permissions, and resource ACLs. Authentication establishes the caller represented by the incoming token/context (`GetAuthEntity` -> `PrincipalEntity`); authorization decides whether that caller may perform this operation on this attempt. `authorizeManualOverride` is predominantly authorization: it requires a signed-in user, checks the claim customer against the run customer, checks manager-plus roles, denies self-grading, and requires either root ACL or managed-user access to the trainee. Passing `customerID` into `GetAuthEntity` is not itself the tenant permission check; the handler compares `GetClaimCustomerID()` explicitly. The RPC's proto role gate includes AGENT because normal automatic evaluation uses the same method; the handler narrows the manual-override branch. The separate `shared/auth/roleverifier` framework supports feature/action permissions for methods declaring them, but this branch uses legacy proto roles plus resource ACL rather than a dedicated grade-override permission. These are naming and policy-layer distinctions, not a new PR finding.

## Non-blocking maintainability review

After a request to include nits, inspected the four production files in the PR and their function boundaries. Added a dedicated maintainability section to the HTML report with three specific suggestions: correct the stale “no annotations fetched” comment; consider moving the ~110-line `evaluateCriterionOverrides` orchestration out of the ~492-line normal evaluator file into an action-specific override file; and rename `requireSavedAIVerdictsFinished` so its name reflects that it compares saved AI N/A against the current annotation-based evaluation rather than proving every saved verdict was historically complete. An optional extraction of saved-result/criterion-ID validation would be warranted only if the handler remains dense after the file move. These are nits and do not change the merge recommendation. No source edits or PR comments were made.
