# Collapse overall zero/false conversation results for reporting

Date: 2026-08-27
Status: Accepted
Decision owner consulted: Jack Jee
Related ticket: [CONVI-7582](https://linear.app/cresta/issue/CONVI-7582/persist-evaluation-status-and-overall-na-on-training-simulator)

## Decision

Reporting does not need to distinguish these persisted overall outcomes when each is represented as `score = 0` and `passed = false`:

- an incomplete evaluation that timed out;
- a completed evaluation where every criterion is N/A; and
- a completed applicable evaluation that failed.

They are treated as the same failure-equivalent reporting result. Consequently, reporting does not require separate persisted overall `evaluation_status` or `evaluation_not_applicable` fields, and the original premise of CONVI-7582 is superseded.

## Criterion-level N/A remains meaningful

This decision applies only to the overall all-criteria-N/A edge case. It does not erase criterion-level N/A:

- an N/A criterion is excluded from the score calculation;
- if any other criteria are applicable, those criteria determine the score and passed value; and
- one N/A criterion does not make the overall evaluation fail or become N/A.

## Validation against the source messages

Jack's messages directly establish that a completed all-criteria-N/A conversation may be treated like a complete failure, while individual N/A criteria remain excluded from scoring and the applicable criteria still determine pass/fail. They also directly support not adding a separate field for the completed all-criteria-N/A edge case.

The quoted messages do not explicitly discuss incomplete timeouts. Therefore, the excerpt fully validates the all-N/A-versus-failure collapse and the criterion-level nuance, but only partially validates the full three-way collapse. The timeout treatment is recorded as part of the stated outcome of the broader discussion, not as a claim derived from this excerpt alone.

The decision also does not define how to classify a timed-out snapshot containing partial applicable results or a nonzero score. That case requires separate confirmation if it can be persisted and reported.

## Source messages

> For the edge case you mentioned above where all criteria of a conversation are N/A, it could be treated the same as complete failure
>
> Having said that, a criterion could be N/A and that will be excluded from score calculation but it doesn't mean that will make the evaluation fail
>
> It will still calculate score based on other criteria and decide the passed status
>
> Given that, I dont believe we require a separate field to indicate the edge case where completed but all criteria is N/A

## Consequences and follow-up

- Do not add overall result fields solely to distinguish the three listed zero/false cases for reporting.
- Preserve per-criterion N/A data and exclude those criteria from score denominators.
- Revisit CONVI-7582 because its requirement has been superseded. The proto, backend, and Director PRs were closed with decision context on 2026-08-27.
- Rebaseline dependent reporting work so it no longer treats CONVI-7582 as a prerequisite.
- Linear ticket cleanup remains pending; this record does not change its external status.
