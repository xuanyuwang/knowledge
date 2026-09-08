# Explore: RetrieveTrainingSimulatorLessonStats in go-servers-milestone-2

Date: 2026-09-02
Tool: Cursor (read-only)
Checkout: `/Users/xuanyu.wang/repos/go-servers-milestone-2` on `codex/milestone-2-lesson-reporting`

## Verdict

Dedicated RPC exists in released cresta-proto `>= v2.22.7` (flat 6-field `TrainingSimulatorLessonStats`). go-servers-milestone-2 is still on `v2.20.25` and has obsolete List `IncludeStats` WIP (`lesson_stats.go` untracked) that does not match the released contract and does not compile against the pin. No `RetrieveTrainingSimulatorLessonStats` handler yet. No ModuleStats backend exists to copy; reuse TaskStats RPC pattern + draft direct-SQL pipeline as reference.

## Dependency order

1. Rebase/reset worktree toward `origin/main` (already `v2.22.11`) or bump cresta-proto/`deps.bzl` to `>= v2.22.7`; `mage cleanBuild` / gazelle; drop List IncludeStats WIP.
2. Implement RPC in one new production file; do not wait on ModuleStats.

## Two-step vertical split (aligned with existing plan)

See also `cursor-convi-7600-two-step-plan.md`.

- Step 1: parse/validate + query + prepare aggregation inputs (+ tests)
- Step 2: aggregate + post-process/return RPC wiring (+ tests)
- Single production file: `action_retrieve_training_simulator_lesson_stats.go`
