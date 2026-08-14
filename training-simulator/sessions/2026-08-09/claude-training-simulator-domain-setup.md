# Claude Session: Training Simulator Domain Setup

**Date:** 2026-08-09
**Tool:** Claude Code
**Source repos inspected:** `cresta-proto` (main), `go-servers` (main), `director` (main)
**Sources fetched via Glean MCP:** Training Simulator Design (gdrive Jack Jee; coda Kevin Kiernicki), Coaching/Training Simulator PRD, Coaching Simulator Assignment Flow Design Doc (search result), launch/enablement/rollout docs, related GitHub/Linear items.

## Objective

Create the `training-simulator` domain under `knowledge` and fill it with project documents fetched from Glean MCP.

## Inputs reviewed

- Glean search "training simulator" and "training simulator backend go-servers implementation" → 30+ docs (product doc, design docs, PRD, rollout/gTM, Gong calls, GitHub PRs, Linear items, Slack thread).
- Full text of the two Training Simulator Design documents (gdrive 60KB, coda 15KB).
- Local proto surface: `cresta-proto/cresta/v1/trainingsimulator/` — service, lesson, module, scenario, task_run, stats, evaluation, quiz protos.
- Local implementation map: `go-servers/apiserver/internal/trainingsimulator/` (service impl + action handlers), `director/packages/director-app/src/features/training-simulator/` + `hooks/training-simulator/`.

## Key findings

- **Domain shape:** content config (lesson/module/scenario) + assignment as DirectorTask + task runs + Customer AI simulation runtime (voice-agent/LiveKit/GoWalter) + Opera/LLM evaluation + reporting. Split into 5 subdomains.
- **Launched Jul 9, 2026** as paid add-on to QM & Coach; beta to Mutual of Omaha & Snap Finance; expanded beta early Q3, GA late Q3.
- **Stat is a durable engineering-accessible surface** despite documents being draft; merged protos differ from design docs (batch scenario CRUD, `EvaluateTrainingConversation` w/ polling + statuses, `RetrieveTrainingSimulatorTaskStats`, `TrainingSimulatorTaskRun`, `evaluation_score` 0–1).
- **Role swap** (voice-agent plays customer, local trainee plays agent) happens in GoWalter channel mapping, not voice-agent.
- **CONVI ticket map** recorded (6747, 6774, 6923, 6926, 7010, 7020, 7023, 7047, 7110, 7145, 7146, 7241).

## Decisions

- Primary source repo = `go-servers`; related = `director`, `cresta-proto`, `python-ai-services`.
- Subdomains: `training-content`, `assignment-and-session`, `simulation-runtime`, `evaluation`, `reporting`.
- Treat local protos + go-servers implementation as authoritative over draft design docs (noted in project.yaml `notes`).
- `enableTrainingSimulator` is the entry feature flag; role/permission gating is interim (CONVI-7145) — note for future permission work.

## Actions / output

- Created `training-simulator/project.yaml`.
- Created `training-simulator/README.md` (domain overview, scope, semantics, architecture, ops, source context).
- Created 5 subdomain READMEs.
- Created `log/2026-08-09.md`.
- Updated `knowledge/README.md` domain catalog.

## Concept verification pass (code-grounding)

After domain creation, verified 8 user-supplied concepts against merged code:

- Hierarchy Lesson→Module→Scenario ✅; Scenario AI-as-customer ✅; separate tables ✅; `TrainingSimulatorTaskStats` = `RetrieveTrainingSimulatorTaskStats` ✅.
- FE picks scenario: confirmed `pickRandomScenario` in `director/.../simulation/lessonUtils.ts:14` (`Math.floor(Math.random() * len)`).
- Passing rule confirmed: 1 attempt = 1 conversation = 1 scenario (`TrainingSimulatorTaskRun.training_scenario_name`, evaluated against module `EvaluationConfig`); stats score the latest attempt per module, and pass = all required modules passed (`calculateTaskMetrics`/`buildAgentPerformanceEntries`).
- Claim 8 was wrong: it's the **session/assignment** that is a `DIRECTOR_TASK_TYPE_TRAINING_SIMULATOR` DirectorTask; the **lesson** is content referenced via `training_lesson_names`.
- Coaching Plan hosts assigned sessions: `coaching-workflow/agent-coaching/assigned-training-sessions/` FE surface.

Applied refinements to domain README, `subdomains/training-content` (#4 passing rule + FE picker), and `subdomains/assignment-and-session` (#5 Coaching Plan surface, #8 lesson-vs-task).

## Next steps

- Add CONVI work items as they progress (permission access controls CONVI-7145 is the visible active thread).
- Refresh subdomains when quiz P1 (quizzes + media upload) and Synthetic Customers ship (early Q3 beta enhancements).
- Verify FE/backend line refs (gowalter messagehandler ~L350, async evaluator 60s) against current main when doing code work.
