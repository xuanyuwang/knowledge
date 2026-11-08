# Omni-use-case AutoQM, filtered completion, and ALL reset

**Date:** 2026-11-04\
**Status:** Confirmed product decisions; implementation pending\
**Primary domain:** Scorecard Workflows\
**Design:** [full engineering plan](../deliverables/omni-usecase-scorecard-eng-plan-draft.md)

Xuanyu answered five focused questions while the full engineering draft was prepared. These answers supplement, rather than replace, the existing criterion ownership, scope encoding, revision pinning, no-inapplicable-score-row, full-Commit precedence, and QM template-inheritance decisions.

## Decisions

1. **Uncovered AutoQM use cases remain manually scorable.** A criterion applying to A+B with an automatic rule only for A remains available for manual scoring in B. Display the coverage gap. Select rule eligibility before interpreting missing evidence; an ineligible rule must not generate an automatic N/A row. Missing evidence from an eligible rule retains existing scoring semantics.
2. **Filtered task completion is unavailable.** For a task with one quota across A+B, a B-only report shows B's evaluated count and completion rate N/A, with the whole-task quota labeled separately. This does not add per-use-case quotas. Preserve authorization and existing partial-period behavior.
3. **Finite-to-ALL is an explicit reset.** The earlier rule requires finite template scope before narrowing criteria. Changing a finite template to ALL therefore resets every criterion, including NONE, to ALL. Allow this only after an explicit editor confirmation. Cancellation leaves the edit unchanged. ALL means all profile use cases, including future ones.
4. **New AutoQM mappings reject overlap.** Reject linked rules with overlapping coverage on the same criterion, including a global rule plus an A-only rule. Distinct A-only and B-only rules on one shared criterion are valid. This avoids implicit precedence or accidental evidence/occurrence-count combination. Legacy intentionally combined triggers require separate migration review.
5. **New automatic evaluations use current Opera scope.** Resolve the current rule scope for each new AutoQM run. Template revision pins criterion/configuration; it does not snapshot an entire Opera policy. Stored historical scores remain unchanged. A later scope edit that creates overlap must be detected at runtime; proposed response is to skip the conflicting automatic criterion with diagnostics while retaining manual scoring and prior scores.

## Evidence and remaining work

The five questions and selected answers are preserved in this chat and summarized in the [session](../sessions/2026-11-04/codex-omni-usecase-full-engineering-plan.md). The [October QM task decision](2026-10-08-qm-task-inherits-template-usecase-scope.md) remains unchanged.

V3 format, derived task representation, grouped analytics response, lock mechanism, exact cache freshness, template lifecycle handling, migration details, effort estimates, and release readiness remain engineering proposals/review work. These product answers do not establish implemented or deployed behavior.
