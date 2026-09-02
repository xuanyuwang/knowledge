# Codex Session: CONVI-7582 PR Context

## Outcome

Updated all three draft PR descriptions with shared project-owner context:

- Reporting must distinguish unfinished evaluation, completed overall N/A, passed, and failed results.
- The backend evaluator computes `status` and `not_applicable`; Director transports rather than derives them.
- The workflow polls `EvaluateTrainingConversation`, places the returned fields on `TrainingSimulatorTaskRun`, calls `UpdateTrainingSimulatorTaskRun`, and persists them to `training_simulator_conversation_scores`.
- `TrainingSimulatorTaskRun` is the existing persisted-attempt write/read contract, whereas the evaluator response is transient.
- The PR sequence is contract ([cresta-proto#9656](https://github.com/cresta/cresta-proto/pull/9656)) → persistence ([go-servers#31521](https://github.com/cresta/go-servers/pull/31521)) → caller ([director#22107](https://github.com/cresta/director/pull/22107)).
