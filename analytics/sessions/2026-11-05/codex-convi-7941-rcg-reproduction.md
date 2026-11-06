# CONVI-7941: RCG reproduction

Date: 2026-11-05
Primary source repo: /Users/xuanyu.wang/repos/director
Context: existing xw/convi-7941-export-errors worktree; live RCG Chrome testing, no source changes yet.
Work item: ../../work-items/CONVI-7941.md

User supplied the RCG Performance Insights Chrome tab and authorized database discovery of overwritten templates and scorecards before browser reproduction. Use read-only PostgreSQL and ClickHouse through the connection skills. Cleared the us-east-1-prod_dev entry and cresta SSO session in ~/.aws/config before explicit profile use. Config resolves ClickHouse to rcg_us_east_1 on us-east-1-prod. No customer writes or external comments authorized.

## Database discovery

Current config: `origin/master:configv3/prod/rcg/us-east-1/config.yaml`. PostgreSQL read-only connection: account/cluster `us-east-1-prod`, DB `rcg-us-east-1`; ClickHouse: `clickhouse-conversations.us-east-1-prod.internal.cresta.ai:9440`, DB `rcg_us_east_1`. AWS profile used: `us-east-1-prod_dev` (DevLimitedModify_Canada), own entry and associated SSO entry read before use. No credentials printed or saved. Both sessions closed.

Discovery query:

```sql
SELECT scorecard_template_id, uniqExact(score_id), uniqExact(scorecard_id),
       max(scorecard_time), groupUniqArray(usecase_id)
FROM score_d_read
WHERE manually_scored AND ai_scored AND NOT is_voice_mail
  AND scorecard_time >= '2026-10-05'
GROUP BY scorecard_template_id;
```

One candidate template: `01a15fd5-59ed-75bf-b6b5-c4d9a2f5def5`, use case `rci-service` (RCI CO Inbound Service), title **TEST The Royal Way - Sales & Service [AQM]**. Raw historical rows contain two overwritten criterion IDs; FINAL/current-version verification reduces that to one. `Ask for the Sale`, ID `01a15fef-6ea5-77cd-ad9d-8722460a5bad`, remains `(manually_scored=true, ai_scored=true, ai_value=0, numeric_value=1, not_applicable=false)`. `Avoid Using Jargon` is currently manually_scored=false and must not be counted from stale physical rows.

Scorecard: `01a182d2-bd0c-7065-b711-4435c317bb95`; conversation resource ID `01a182c3-f0b3-790a-879f-fb96c5a1d9ad`; template revision `9c316f3c`; scorecard time October 28, 2026 13:47:21 UTC; agent ID `40d3c6079d6d378c`. Revision options map value 0 to Yes, 1 to No. Latest selected template revision in downloaded URL is `36de00be`; export still correctly maps historical scores. No customer writes.

A direct PostgreSQL scorecard-ID SELECT stalled and was canceled; no result is claimed from that query. Template mapping was verified through PostgreSQL, criterion versions through ClickHouse, and matching resource links in the exported CSV.

## Browser reproduction

Used the supplied Chrome tab `1073952890` at RCG Performance Insights. Changed use case from CEL Retreat to RCI CO Inbound Service and selected the discovered template. Retained Last 30 days / Weekly, all audiences, Exclude voicemail Yes, Criteria adherence. Changed all three tables to Number of overwritten. UI shows one overwrite in Ask for the Sale, October 25–31, for Mikaela Almodovar.

| Test | Result |
| --- | --- |
| Detailed cell → Download scorecard → Export, comments checked | CSV downloaded, one matching conversation, Original Auto Score Yes, Overwritten Score No, comment test |
| Progression table ZIP, first attempt | **Defect reproduced: only performance_progression_by_agent.csv is present; performance_progression.csv is missing** |
| Progression table ZIP, second attempt, unchanged data/filters | Both expected files present |
| Leaderboard by criteria CSV | One row, total overwritten 1, Ask for the Sale 1 |
| Leaderboard by criterion/time CSV, Ask for the Sale | One row, total 1, October 25 bucket 1 |

The detailed download observer timed out after 20 seconds while the actual file arrived. Disk/CSV evidence is authoritative for that result; observer timeout is not a product failure. Browser logs also showed a separate scorecard-template-permissions response oversize error (81,580,372 vs 52,428,800 bytes); downloads succeeded despite that error, so it is not established as this export's cause.

## Root-cause evidence and limit

`PerformanceProgression.tsx:392-428` in the Director checkout starts the summary and by-agent analytics requests independently. It adds the summary entry only in the first request continuation, but calls `zip.generateAsync` immediately after adding the by-agent entry in the second continuation. It never awaits the summary before generating the ZIP. This is a concrete ordering race consistent with the incomplete first archive and complete repeat. Both exported by-agent data and table-level CSVs contain the correct overwrite count; this defect is ZIP packaging, not a proven wrong analytics count or missing detailed CSV.

The intended repair is to wait for both requests/CSV entries before ZIP generation and handle request failures visibly. The deployed minified bundle was not compared line-by-line to the local source, and request completion ordering was not independently captured. Therefore distinguish source-level explanation from direct timing-trace proof. This is an actual reproduced defect on the reported surface; the one-sentence original ticket still does not establish whether its reporter meant this incomplete ZIP or a different download symptom.

## Artifacts and state

- `convi-7941-rcg-downloads/`: original downloaded files, incomplete and complete ZIPs, plus a SHA-256/entry manifest.
- `convi-7941-rcg-overwritten-tables.png`: screenshot of the selected use case/template and progression overwritten setting; the page uses an internal scroll area, so the screenshot does not show all three tables.
- Existing silent-error worktree remains uncommitted and unchanged by this investigation. No further source edits, PRs, external messages, or database mutations.

## Follow-up: deterministic root-cause investigation

User asked to investigate root cause after browser reproduction. Applied systematic debugging; no product source edits. Read the callback, mutation/API wrapper, request groupings, JSZip implementation, global query-client configuration, and introduction history.

### Source identity and data flow

The browser log's build path `main-6d5b707` resolves to local Git commit `6d5b707a21ab607378e4ed34d2a917d61f6f0efb` (November 2, 2026). The callback in that source commit matches the investigated callback; this improves source provenance but is not a byte-level comparison to the deployed compiled bundle.

1. Export starts two `RetrieveQAScoreStats` mutations using the same filters. Summary grouping is criterion/time; detail grouping adds agent.
2. Summary continuation adds `performance_progression.csv`.
3. By-agent continuation adds `performance_progression_by_agent.csv` and immediately calls `generateAsync`.
4. JSZip `generateAsync` calls `generateInternalStream` synchronously; `generateWorker` enumerates current ZIP entries immediately. A later file addition cannot join that archive, even if it occurs before `saveAs`.
5. The `.catch(handleError)` belongs only to the inner ZIP-generation chain. It cannot catch rejection of either outer data-request promise. The mutation defines no onError, and inspected app QueryClient defaults have no global mutation error handler.

Overwritten mode contributes only `scoreType=OVERRIDDEN, includeNaScored=true`; the same ZIP callback serves other progression metrics. This defect is not overwrite-specific. The CSV formatter always adds a filename, even if CSV text is empty, so a completely absent archive entry points to packaging order/failure rather than filtering out its rows.

### Controlled experiment

Saved `convi-7941-export-race.cjs`, which uses TypeScript AST to extract the actual callback from commit `6d5b707`, executes it with controlled request promises and real JSZip, and captures the saved Blob. CSV formatting is stubbed to two literal CSVs to isolate packaging; neither React rendering, live requests, nor analytics calculations are exercised. All original callback statements and its catch placement run unchanged. ZIPs are reopened and their entries/content asserted, with request/file/generation/save events recorded. It is an investigation harness, not a shipped regression test.

Command: `node knowledge/analytics/sessions/2026-11-05/convi-7941-export-race.cjs` (absolute path used in execution). Exit 0; all five scenario assertions pass:

| Controlled completion | Actual result |
| --- | --- |
| Summary then by-agent | Both entries saved |
| By-agent then summary after save | Summary omitted |
| By-agent then summary before save, but after generation begins | Summary still omitted |
| Summary request rejects, by-agent succeeds | Partial ZIP saved; rejection unhandled by export |
| By-agent request rejects, summary succeeds | No download; rejection unhandled by export |

Results: `convi-7941-export-race-results.json`; generated `convi-7941-controlled-*.zip` archives use synthetic data only. This deterministically confirms the packaging race and the separate missing error catch. The original live request ordering/error status was not captured, so do not claim which specific timing/failure produced the first RCG archive. Both are covered by the proposed repair.

### History and repair

Git history shows commit `5cae1a6584a6852738904b228a9a57dce51bf2ba`, April 15, 2024, Director PR #7414, split the previous single-response export into two independent requests but left ZIP generation tied to only the by-agent continuation. This is a longstanding race, not evidence of a new overwrite-specific backend regression.

Recommended repair: fetch both datasets concurrently with `Promise.all`, build/add both CSV entries only after both complete, then generate/save the ZIP. Put the entire request/format/generate/save operation under one error handler; do not download a partial archive if a request fails. Permanent regression tests should control both completion orders and either request rejection. Existing detail-download silent-error fix is a different callback and cannot repair this ZIP path.

Worktree status is unchanged: only the previous two silent-error fix files are modified/untracked. No database or credential access was needed for this follow-up. No commit, push, PR, or external message.

## Implementation: replace the separate silent-error change

User requested reverting the earlier silent-error fix and applying the await-both repair. Source repo: Director. Worktree `/Users/xuanyu.wang/repos/director-convi-7941-export-errors`, branch `xw/convi-7941-export-errors`, HEAD `344235d59316f03440436e05335e56c3c86abf8a`.

- Restored `useExportPerformanceInsightCriteria.tsx` to HEAD and removed its previous assistant-created test. No detail-export changes remain.
- In `PerformanceProgression.tsx`, replaced independent continuations with concurrent `Promise.all`, followed by formatting/adding both entries, ZIP generation and save. Moved the existing error handler to the outer chain so a request failure cannot save a partial ZIP or escape uncaught. Filters, grouping, formatting and filenames unchanged.
- Added `PerformanceProgression.export.test.tsx`. Uses the real component, real request builders, real React Query, real JSZip and app test providers; mocks only the exact analytics API and file saving. Both request groupings are asserted. Controlled promises cover both response orders and either rejection. Empty data isolates packaging; these tests do not validate analytics values or CSV calculations. The saved real Blob is reopened and both archive filenames checked.
- Red run: summary-first passed; by-agent-first failed because `generateAsync` was already called with the summary pending. After the repair, all four cases passed. Existing skipped component suite preserved unchanged.
- Validation: four export tests pass; director-app TypeScript build, changed-file Biome and ESLint, and `git diff --check` pass. Two test type errors (required voicemail field and potentially missing save call) and require-await lint errors were corrected before final verification. The original detail hook and existing component test file are byte-identical to HEAD. No database or AWS credential access, commit, push, PR, deployment or external message in this implementation.

## Draft PR

User requested a draft PR. Re-ran four export regressions (all pass), reviewed final diff and repository PR/commit skills, and used the template with only its description filled. Commit `0d6a4637368fbd112a001844d569140613a8d272`; normal commit hooks (i18n extraction, lint, i18n lint, formatting, ticket prefix) all passed. Pushed branch and opened [draft Director PR #24681](https://github.com/cresta/director/pull/24681). Readback confirms draft, matching commit, and exactly the progression handler and new export regression file. Attached PR to the chat. Worktree clean. No explicit AWS/Okta/Azure/SSH credentials selected; Git and gh used their existing authentication. No deployment.
