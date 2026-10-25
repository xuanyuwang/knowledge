# Notes and Guided Workflow consolidated suite verification

Tests commit: `4481c7fe36944ed28d89b9c2845b8dfd34264142`. Refactor commit: `b721a5086e6bbe09b41bd842645bf10d0d7c7bb7`.

Draft PRs: [tests #34040](https://github.com/cresta/go-servers/pull/34040) and [refactor #34041](https://github.com/cresta/go-servers/pull/34041). Publishing remains with the parent agent.

Both original named suites use real seeded ClickHouse and one shared non-runnable fixture. The same **66 test nodes passed** against the original production and extracted production. All three complete test/helper files have identical hashes across both runs.

| Stage | Package elapsed | Result |
| --- | ---: | --- |
| before | 150.708s | 66 nodes passed |
| after | 148.660s | 66 nodes passed |

The 24 characterization methods/helpers are byte-identical after receiver renames. All three original expected-response and assertion blocks are byte-identical; seeded rows replace mocked ClickHouse rows. All 11 SQL-snapshot scenarios now run through the full RPC and compare complete responses, retaining selected-user, duration, group, weekly, metadata, and workflow-name coverage. The three separate integration files, their two runnable targets, and duplicate Guided Workflow runner were removed.

All four production extraction files are byte-identical to `0546e0ba5e31f445f201ef50f6269f2c1caf62fd`; the tests branch contains no production change. Existing DISTINCT grain, conversation-only joins, metadata argument order, usage gates, denominators and conversions remain intact.

| Frozen file | SHA-256 |
| --- | --- |
| `common_notes_workflow_usage_test.go` | `c34ae44465de43c2ecde84f0a24bf6dd4d155620949549bcc9e48bc943fb9620` |
| `retrieve_note_taking_stats_test.go` | `3b59601489966cee0a796851ef6e67533354e9d98b0405d24a1de3dfdd242d85` |
| `retrieve_guided_workflow_stats_test.go` | `b9956ca12ccabda8ff5bbbd41b5f12ad8aedeafc45cec4de471e695c73aabb2a` |

Calibration failures were fixture-only: legacy event tables have no duration column, and the group RPC issues ListGroups requests containing selected IDs plus paired direct/indirect membership flags. Fixture corrections passed a focused two-RPC group probe before the final baseline; characterization expectations were unchanged. Their failure logs remain alongside the final evidence.

Preserved behavior includes zero aggregates under the local default metadata-exclusion joins, countable empty Guided Workflow sessions, and year-1 timestamps for ungrouped rows. No production deployment or performance claim is made. Generated Bazel targets were not executed by this agent.

Evidence files:

- [notes-workflow-merged-before.jsonl](notes-workflow-merged-before.jsonl)
- [notes-workflow-merged-after.jsonl](notes-workflow-merged-after.jsonl)
- [notes-workflow-merged-group-probe.jsonl](notes-workflow-merged-group-probe.jsonl)
- [notes-workflow-merged-frozen-tests.json](notes-workflow-merged-frozen-tests.json)
- [notes-workflow-merged-body-equivalence.json](notes-workflow-merged-body-equivalence.json)
- [notes-workflow-merged-legacy-response-equivalence.json](notes-workflow-merged-legacy-response-equivalence.json)
- [notes-workflow-merged-case-mapping.json](notes-workflow-merged-case-mapping.json)
- [notes-workflow-original-refactor-production.json](notes-workflow-original-refactor-production.json)

Credentials: no credential profiles or keys read or selected.
