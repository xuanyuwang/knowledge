# INSI-4262 applicability review for go-servers PR #32443

**Date:** 2026-09-17
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree context:** review-only comparison of PR head `c3a99f49670d093767bb89861ed955b0efcf6d9c` against `main`
**Ticket:** [INSI-4262](https://linear.app/cresta/issue/INSI-4262/leaderboard-not-capturing-active-users)
**PR:** [go-servers #32443](https://github.com/cresta/go-servers/pull/32443)

## Conclusion

PR #32443 fixes the confirmed label-generation defect behind the Janalie Shene Labeste and Kyle Batobato examples in INSI-4262. It changes the online-event candidate predicate from session-start containment to true interval overlap:

```sql
endTime >= batchStart
AND startTime <= batchEnd
```

For the confirmed Bill West examples, each qualifying application-type-0 session started before the lower bound but remained alive across the agent messages and conversation window. The new predicate includes those sessions. The shared downstream `labelMessages` path will then match the messages to the session and write active `conversation_with_labels_d` rows.

## Validation

- PR head: `c3a99f49670d093767bb89861ed955b0efcf6d9c`.
- Review verdict: no actionable correctness findings. The interval predicate, valid-session assumptions, downstream matching behavior, and regression path are sound for the stated defect.
- CI's required `cron-sync-users` Bazel build/test and Go lint jobs passed at review completion; the PR was approved, mergeable, and passed the code-owner check.
- The new integration regression seeds a session that begins 10 hours before the batch window and remains alive across a conversation inside the window; it fails under the old containment predicate and passes under the overlap predicate.
- Although the regression test exercises the no-message conversation-anchor path, candidate retrieval is shared with the per-message path used by Janalie and Kyle. Their persisted messages are inside their qualifying session intervals, so the predicate change applies directly.
- A message-level regression case would strengthen coverage of the customer-specific path, but its absence is not a correctness blocker because the modified candidate query is shared.

## Boundaries

- The PR is open and not yet deployed at review completion; only the `donotmerge` context remains pending.
- It fixes future label generation only. Existing missing Bill West labels remain behind the watermark and require a bounded re-label/backfill after deployment.
- It does not change the separate Mervel Molina interpretation: `DIRECTOR` application type 5 remains excluded from qualifying Agent Assist types.
- It fixes this ticket's confirmed same-day long-session fingerprint. The retained ±3-hour event window can still be insufficient for a multi-day conversation whose relevant message and completed online session both substantially predate the conversation-end batch; that broader case is not required to explain the Bill examples.

## Actions

- No code, PR, production, or Linear changes were made.
- Update INSI-4262 after merge/deploy with the backfill range and post-backfill verification for Janalie 2026-08-11/13 and Kyle 2026-07-23.
