# Dedicated statistics API decision capture

Date: 2026-08-28
Source repo: `/Users/xuanyu.wang/repos/cresta-proto`
Branch/worktree context: Milestone 3 in `/Users/xuanyu.wang/repos/cresta-proto-milestone-3-module-reporting`; stacked Milestone 2 in `/Users/xuanyu.wang/repos/cresta-proto-milestone-2`

## Inputs reviewed

- [Slack decision thread](https://crestalabs.slack.com/archives/C05L7FBAGRF/p1787928318264329), retrieved through the permission-aware company search index.
- Current #9676 module contract and #9677 lesson contract in their local worktrees.
- Existing Option 2/Option 3 comparison, frontend loading contract, milestone session records, work item, and domain summary.

## Outcome

Recorded the accepted switch from enriching `ListTrainingLessons` / `ListTrainingModules` to dedicated `RetrieveTrainingSimulatorLessonStats` / `RetrieveTrainingSimulatorModuleStats` RPCs. The decisive change is that the extra batched frontend request and cold-tab concern are not considered large enough to justify mixing reporting into content-list contracts. Cleaner responsibility, convention, discoverability, and independent reporting behavior take priority.

Created separate implementation prompts for updating the existing module and lesson proto PRs. No source PR, branch, credential, or external system was modified in this session.

## Credentials used

None.

