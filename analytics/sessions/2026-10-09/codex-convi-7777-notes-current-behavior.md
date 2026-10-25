# CONVI-7777 Notes consistency answer

**Question:** EA asks what the current Notes number on Assistance Insights counts, and directs the new metric to stay consistent.

**Source context:** `/Users/xuanyu.wang/repos/go-servers` `origin/main` `48d308822e90e17d2ca190987220601601ee9d97`; related `/Users/xuanyu.wang/repos/director` `origin/main` `da2e210926b60faefcdef8ede940ff40575d0e50`. Read-only source inspection; no production query.

**Trace:** Director's `NotesPage.tsx:38-41,69-82` displays `totalNotesCount`; `useAssistanceStatsWithSplitAPIs.ts:421` maps it directly from `RetrieveNoteTakingStats.totalNoteTakingUsedCount`. In the ordinary events-cluster path, `retrieve_note_taking_stats_clickhouse.go:157-163` counts distinct event IDs only when the event is `USED_NOTE_TAKING` and named `VoiceAgentAssist.NoteTaking.Click.CopyEntity`. The metadata-filter path checks copy payload data type 1 but uses event type 5 against the conversations cluster, where the DB type is 17, so it is a separate source-level correctness concern. The public `UsedNoteTaking` payload has distinct `copy_entity` and `update_entity` cases. The UI tooltip says “copied, clicked, or edited notes” (`NotesPage.tsx:74`), but the current backend predicate does not include editing.

**Conclusion for reply:** Current analytics counts copies of automatically extracted entities, not edits. Under EA's direction to match the existing number, propose CopyEntity only for the new conversation metric: one or more copy events makes a conversation qualify; an edit alone does not. Do not use the existing metadata-filtered Notes count as a parity oracle until the enum mismatch is resolved. Source-level only; customer coverage and deployed impact unverified.
