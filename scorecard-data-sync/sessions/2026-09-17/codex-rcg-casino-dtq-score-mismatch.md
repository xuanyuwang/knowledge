# RCG Casino DTQ score mismatch investigation

- Date: 2026-09-17
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Source context: read-only production investigation; current `main` checkout used for schema and scoring-path tracing
- Customer/profile: `rcg/us-east-1`
- Conversation ID: `01a08bec-b88b-7768-809b-e7b2757f710c`
- Template name: `Consumer Outreach - Casino DTQ`

## Requested investigation ladder

1. Find the active template ID, disambiguating any same-name inactive revisions/templates.
2. Find the exact PostgreSQL scorecard using the conversation, template, and usage.
3. Manually calculate every criterion and the total from PostgreSQL source rows.
4. Query ClickHouse only if PostgreSQL is internally correct.
5. Trigger a targeted backfill only if PostgreSQL is correct and ClickHouse is wrong.

## Slack evidence

Glean search and `read_document` confirmed the root message and replies indexed through 19:34 UTC at the supplied Slack URL. The report says Closed Conversations shows 50% but the selected criteria imply 80%; only `Policy and Procedure` was answered No. A second conversation on the same active template produced 80%. John Pak reports that toggling an answer No -> Yes -> No corrects the percentage. Glean had not yet indexed the later timing-race hypothesis, so that later message was cross-checked in the authenticated Slack app but remains a hypothesis rather than primary evidence.

## Production evidence

The latest config resolves `rcg/us-east-1` to:

- config: `origin/master:configv3/prod/rcg/us-east-1/config.yaml`
- provider/account: AWS `us-east-1-prod`
- database cluster/database: `us-east-1-prod` / `rcg-us-east-1`

The first active-template query was initially blocked because AWS SSO for the normal `us-east-1-prod_dev` profile had expired. After the user refreshed SSO, read-only PostgreSQL queries completed. The GitHub SSH key used to refresh config was inspected and was not marked restricted; the separate break-glass SSH credential was identified from its config/README and was not read or used.

### Template and scorecard identity

The title is not unique: it returned 97 template revision rows across many RCG use cases/resource IDs. The conversation row resolved the exact use case to `rci-casino-fit-Groups`.

- active template resource ID: `019d521b-68c7-727e-bead-301e50d92c7c`
- scorecard-pinned revision: `c1af7a35`
- template status: `1` (`ACTIVE`)
- template deleted/deactivated: neither
- scorecard ID: `01a09154-585f-71cf-8809-c95bf13e165e`
- persisted PostgreSQL total: `50`
- scorecard created: `2026-09-11 16:36:59.196238+00`
- scorecard submitted: `2026-09-11 16:37:19.793038+00`

A newer duplicate resource for the same case-insensitive use-case spelling, `019f1df5-09d5-7f18-bd8a-f2c5e99fcb32@dd6bc2fd`, is deleted and was not selected. The scorecard foreign key directly confirms the active resource/revision above.

### Manual PostgreSQL calculation

All five scorable criteria have weight 20. The persisted leaf rows and template mappings are:

| Criterion | Answer | Mapped % | Weight | Weighted points |
|---|---:|---:|---:|---:|
| Relationship & Professionalism | Yes | 100% | 20 | 20 |
| Policy and Procedure | No | 0% | 20 | 0 |
| Efficiency | Yes | 100% | 20 | 20 |
| Resolution | Yes | 100% | 20 | 20 |
| Revenue Generation | Yes | 100% | 20 | 20 |

The Automatic Zero criterion is No, has weight 0, and did not auto-fail. The selected Policy and Procedure follow-up (`Appropriate use of policies and procedures`) also has weight 0.

Manual total: `(20 + 0 + 20 + 20 + 20) / (20 + 20 + 20 + 20 + 20) = 80%`.

The stored chapter aggregates independently agree with the leaf calculation: `100, 0, 100, 100, 100`. PostgreSQL is therefore internally inconsistent: its score rows imply 80%, while `director.scorecards.score` is 50%.

Per the requested ladder, ClickHouse was not queried and no reindex/backfill was triggered. A reindex would only copy/recompute from the incorrect PostgreSQL source and is not the correct repair boundary.

## Code-path evidence

Current `origin/main` has no unpulled changes to the three relevant scoring/update files versus the local inspected checkout.

- `UpdateScorecard` loads the scorecard before entering the write transaction, computes `scorecard.score` from the request's submitted score list, then performs a full-row GORM `Save` (omitting only submission fields).
- `ComputeScores` calculates the total as the weighted mean of only scorable criteria present in that request, rounded to one decimal percent.
- `mergeScoresData` deliberately keeps an existing criterion row when the incoming request has no new row for that criterion (documented as preserving branch-invalid scores).

This combination can produce exactly the reported PostgreSQL inconsistency under concurrent/out-of-order partial autosaves: a late partial request can overwrite `scorecards.score` using only its partial criterion set while `mergeScoresData` retains criterion rows written by an earlier complete request. The persisted criterion rows can therefore imply 80% while the persisted parent scorecard says 50%. This is stronger and more precise than a generic PG/CH ordering theory, but production rows are still required to confirm it happened in this instance.

Production rows now confirm the parent/child mismatch. The backend mechanism is confirmed capable of producing this exact state. The exact triggering request order is not recoverable from these tables because `director.scores` has no update timestamps or request/version identifiers. Current Director code debounces form changes and serializes mutations within one React Query scope, so a generic claim that ordinary single-form requests simply finish out of order is not yet proven; multiple editor instances/tabs, retries, submit/autosave interaction, or another caller remain possible triggers.

## Conclusion and actions

Confirmed PostgreSQL source-of-truth corruption for this scorecard: expected 80%, stored 50%. Do not run a ClickHouse backfill.

The durable defect is that the parent score is computed from the incoming request snapshot while child rows are merged with prior persisted state. The fix should make the parent total and chapter aggregates derive from the same final persisted criterion set under a serialization/versioning boundary. Candidate designs should be validated against branch-inactive score preservation, concurrent updates, autosave followed by submit, multiple editors, and idempotent retries.

## Existing repair mechanisms

There is no Temporal workflow that repairs this PostgreSQL inconsistency. `JOB_TYPE_REINDEX_SCORECARDS` reads the existing parent and child rows from PostgreSQL and rebuilds only ClickHouse, so it would preserve/propagate the incorrect parent score.

An existing one-shot cron implementation does repair this class of PostgreSQL inconsistency: `cron-manual-scorecard-scores-backfill` / `ManualScorecardScoresTask`. It selects scorecards by customer/profile, `updated_at` range, optional conversation IDs, and optional `manually_scored = TRUE`; for every selected scorecard it calls `GetScorecard` and then `UpdateScorecard` with the complete returned score set. That recomputes the parent and chapter values using the persisted leaf answers and then runs the normal asynchronous ClickHouse update.

Current deployment state prevents it from running automatically:

- the common production HelmRelease has `suspendCron: true`;
- `ENABLE_SCORECARD_SCORE=false` even though the outer typo-preserved flag `ENABLE_CRON_SCORECARD_SCORE_BACKILL=true`;
- RCG's current `scorecardScoreBackfill` config is enabled but pinned to the obsolete `2025-08-14` through `2025-08-29` window and has no conversation filter.

For this incident, the cron could be run as a controlled one-shot after setting:

- exact conversation ID `01a08bec-b88b-7768-809b-e7b2757f710c`;
- a narrow range containing `2026-09-11 16:37:19.793554+00`;
- manual-scores-only true;
- the inner scorecard-score task enabled, with the suspended CronJob invoked once.

The conversation has two scorecards in PostgreSQL, but only the affected row is manually scored; the manual-only filter would exclude the other automatic scorecard. A direct manual `GetScorecard` -> `UpdateScorecard` RPC is functionally equivalent for one row, but direct SQL is not recommended because it would bypass chapter recalculation and the normal ClickHouse projection path.

## PR #32447 assessment

[go-servers #32447](https://github.com/cresta/go-servers/pull/32447) fixes one concrete writer race: `SubmitScorecard` previously loaded a scorecard snapshot, then performed a full-row save. An `UpdateScorecard` autosave that committed between those operations could write a newer calculated `score` and then have submission overwrite only the parent row with the stale score. The PR moves the submit read to the primary, omits `score` and `auto_failed` from the submission save, and reloads the committed row before building the response and async work. Its new tests deterministically inject newer scoring fields between submit's read and write and verify that numeric, auto-failed, and NULL/N/A states survive.

This is a strong match for the observed incident. The production scorecard's `submitted_at` is `2026-09-11 16:37:19.793038+00` and `updated_at` is `2026-09-11 16:37:19.793554+00`, only 516 microseconds apart. Because submission updates the parent timestamp while leaving criterion rows alone, this timing plus the exact parent/criterion mismatch makes submit overwriting a just-saved 80% score the leading explanation. The tables do not retain enough request history to prove the exact interleaving.

The PR is nevertheless a mitigation, not a complete parent/criterion consistency fix. `UpdateScorecard` still:

- reads the parent and existing criterion rows before its write transaction;
- computes the parent score from only the request's score list;
- saves the full parent row; and
- merges child rows so criteria absent from that request may remain persisted.

Director's process-scorecard autosave uses a 300 ms debounce and invokes `void handleSubmitForm(values)`. The `useMutation` has no mutation scope or explicit client-side queue. A later debounce can therefore start another `UpdateScorecard` while an earlier request is still in flight, and independent tabs/users/clients are necessarily outside any one component's ordering. If requests commit in a different order from the user's edits, the backend has no revision, precondition, row lock spanning the initial read, or idempotency sequence with which to reject the stale update. A late request can therefore overwrite parent fields and selected child rows with an older snapshot. Partial/branch-specific payloads retain the stronger failure mode in which the parent is computed from one subset while other persisted children survive the merge.

The PR's regression tests cover submit-versus-scoring-field-update only. They do not exercise concurrent `UpdateScorecard` calls, stale full snapshots, or partial snapshots. Recommended follow-up coverage is a deterministic two-update test where a newer complete request commits first and an older/partial request commits last, asserting both edit ordering and parent/child consistency.

A complete fix needs a backend consistency boundary. Viable approaches include optimistic concurrency using a scorecard revision/update token, server-recognized per-editor request sequencing, or locking and recomputing parent/chapter aggregates from the final persisted active criterion set inside one transaction. The last option must preserve the intentional branch-inactive-score behavior without allowing inactive retained rows to affect the total.
