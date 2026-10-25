# CONVI-7777 hidden metric sticky edge

- Source repo: `/Users/xuanyu.wang/repos/director`.
- Worktree: `/Users/xuanyu.wang/repos/director-convi-7777-assistance-usage`.
- Branch: `xw/convi-7777-assistance-usage`, PR #24020.
- Request: validate and apply Cursor's prepared fix for review 5619927777 if sound, then push.
- Reviewed exact single-file patch at `2c57eb138aa3595df9b37bfcfa42f038370132a5`; applied the patch rather than merging the side branch.
- Finding is valid: width adjustment retains hidden metric definitions; shared `isLastSticky` previously included them when choosing next/last columns. Header, body and footer all consume that result. `shouldCellHaveDivider` already filters visibility.
- Patch makes next/last-column checks visibility-aware and checks first visible child in the next group, preserving current divider mode semantics. No unrelated group traversal redesign.
- Added six real TanStack table cases covering hidden trailing metric with groups/none, visible metric restored, hidden root sticky column, last visible root column, and hidden first column in next group before sticky continuation.
- Baseline: four cases fail, two pass before the production patch.
- All 16 tests across the new helper cases and existing DirectorTable suite pass; components TypeScript build passes. All commit hooks pass. Pushed `5d3abfdee9` to #24020; worktree clean. Remote CI pending. No review replies posted or threads resolved.

- Authentication: self-authenticating git/gh only, no explicit credential selection.
