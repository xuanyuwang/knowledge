# Superhuman engineering-design sync

Date: 2026-08-24
Source repo: `/Users/xuanyu.wang/repos/knowledge`
Branch/worktree context: main checkout; no knowledge worktree

## Outcome

Replaced the existing Superhuman Docs engineering-design page body with `deliverables/superhuman-api-design-update-draft.md`.

The SuperhumanDocs OAuth refresh endpoint repeatedly rejected refresh requests, so the update was completed in section-sized operations within fresh access-token windows. The final page contains the Figma-strict lesson/module statistics contract, the narrowed session additions, the `RetrieveTrainingSimulatorTaskStats` changes, and all three API options with updated protobuf examples.

## Comment preservation

Captured the 12 active thread URIs before the update and compared them after the final write. All 12 thread IDs remained present and active. No comment resolve or delete operation was called.

## Verification

- Read the full remote page after the update: 287 blocks, no pagination remainder.
- Confirmed the new `attempt_count` and `TrainingSimulatorModulePassRate` contracts.
- Confirmed all three API-option sections remain present.
- Confirmed stale `TrainingSimulatorResultSummary` and `retried_agent_count` text is absent.
- Converted Markdown fourth-level headings to bold labels because Superhuman Docs supports only three heading levels.

Remote page: https://docs.superhuman.com/d/_dE0dCcz8Bub/Untitled-page_subF07cy
