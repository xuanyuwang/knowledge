# CONVI-7810: replace CSV RPC proposal with task-run List extensions

Date: 2026-10-27 (America/Toronto).
Primary source repo/worktree: /Users/xuanyu.wang/repos/cresta-proto-training-simulator-csv.
Branch: xw/convi-7810-training-simulator-export-protos.
PR: https://github.com/cresta/cresta-proto/pull/10413.

## Approved design and execution

The user approved using task-run record create_time for date filtering, including question definitions inside historical quiz modules, and replacing the branch history with one fresh commit extending ListTrainingSimulatorTaskRuns. No backend implementation is requested in this proto branch.

Plan:
1. Preserve previous head fd2e479c76 on a local backup branch; rebuild the existing branch from current origin/main (590a10a1a3 at start).
2. Extend the existing List request with optional time_range and include_training_module_revisions; extend response with page-local, distinct historical TrainingModule definitions, including exact quiz-template revisions/questions without answer keys.
3. Document time bounds as the shared TimestampRange convention: inclusive start and exclusive end, filtering task-run create_time. Preserve existing name filters, pagination, access annotations, and default responses. Resource lists remain OR within and AND across; FE leaves attempt-owner filters unset to export whole session audiences.
4. Validate lint/API lint, focused Buf build/compatibility, formatting and Bazel package; inspect complete diff. Commit once, force-push with the exact old remote head as lease, update PR title/body, and present for user review.

No generated source or unrelated LLM enum changes may be included. Historical module data is keyed by module name and revision_id matching each page's runs. Quiz question definitions use the linked historical template. Backend/FE implementation and runtime tests remain follow-up work.

## Result and verification

Pushed one fresh commit `464bedb4a86321c12c7eb76e79f685c0620de616` to the existing PR branch, using a lease pinned to the previous remote head `fd2e479c76c4782abe5e2ef17d3a820e331fd328`. Exactly one commit ahead of base `590a10a1a3`; complete diff is one service proto (44 additions, one comment-line replacement). No CSV RPC, generated files, BUILD changes, or unrelated LLM enum changes remain. Source worktree is clean.

Validation passed:
- Full `mage -v lint` and `mage -v apiLint`.
- Focused Buf lint/build and breaking check against origin/main excluding imports.
- `bazel build //cresta/v1/trainingsimulator:all`: success, 1062 actions.
- Google clang-format, proto import lint, HTTP/resource-path lint, and git diff --check.

The Docker-backed commands initially could not reach the stopped daemon; started the existing Colima instance and reran them successfully. Bazel emitted Xcode-license/SDK warnings but completed successfully using available toolchain paths; no license acceptance or system changes were made to resolve those warnings. No imports changed, so Gazelle regeneration was not needed. No runtime behavior is implemented or claimed by this proto-only commit.

PR title/description updated to describe the List extension; review panel requested. GitHub CI and human review remain pending. Git/gh used their existing authentication; no credential or named AWS/SSH profile was explicitly selected. No credentials were read or copied.

## Review discussion: response-level versus per-attempt module definitions

Read live GitHub discussion r4391901212 and user reply r4392196041; PR head remains 464bedb4a8. Reviewer asks whether historical definitions should be nested inside TrainingSimulatorTaskRun. User points out repeated revisions across attempts and possible response-size limits.

Both shapes are valid. Nesting simplifies per-run consumption and can make an enriched run self-contained, but duplicates serialized module definitions and quiz questions for every run referencing the same revision. Sharing in-memory pointers or fetching the revision once does not deduplicate serialized repeated messages. The current response-level collection is distinct per (module name, revision_id), and task runs already contain both references, so FE can map them without another field. With N runs and U distinct revisions, metadata size roughly scales with N versus U. Backend List currently defaults/caps pages at 1000 runs; pagination bounds row count, not definition bytes.

Recommendation: retain the response-level field for this batch CSV use. The user's duplication argument is sound; exceeding an actual response-size limit is only a possibility, not demonstrated by source inspection. No relevant production limit or payload distribution was measured. Smaller pages could mitigate nested payload size too. Also avoid calling the existing List API export-only: the new optional enrichment serves export while existing general callers remain.

No product code changes, GitHub replies, or thread resolution were made. Read-only gh used existing authentication. Human may clarify the reply to emphasize deduplication, with size-limit risk qualified.

## Approved review follow-up: separate definition reads

Live discussion now includes reviewer reply r4392282518 proposing a separate ListTrainingModules call, user reply r4392938381 identifying GetQuizTemplate plus FE joins, and reviewer reply r4393114076 agreeing the join is lightweight. The user explicitly requested removing include_training_module_revisions and training_module_revisions.

Removed both proposed fields and their comments/lint exception. Kept time_range = 10 on task-run create_time, resource-filter semantics, and pagination comments. Response now exactly matches base. No field reservations are needed for unmerged/unreleased proposed additions. Amended the single branch commit to d6be44b611 (13 additions, one deletion in one proto). PR title/body now describe creation-time filtering and separate existing definition reads, with a link to the review discussion. No review-thread reply or resolution requested or performed.

Fresh checks after removal: full mage lint and API lint, focused Buf descriptor build and compatibility against origin/main excluding imports, Google clang-format, git diff --check, and Bazel Training Simulator package build all pass. New CI remains pending. Git/gh use existing authentication; no credentials explicitly selected/read. Runtime backend filtering and FE joins remain follow-up implementation.
