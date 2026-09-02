# Simulation Runtime

## Purpose

How an agent starts a module attempt and converses with the simulated customer: the Customer AI virtual agent, voice-agent/LiveKit pipeline, GoWalter role/channel mapping, and conversation creation with training metadata.

## Scope and Boundaries

**In scope**

- Scenario-backed Customer AI virtual agents (context, visitor objective, initial message, model, voice, and turn-taking)
- Shared Virtual Agent/LiveKit voice execution; the current local checkout does not include the `python-ai-services` runtime implementation, so its internals remain a dependency rather than revalidated fact here
- Conversation creation in `app.chat` with `conversation_source = TRAINING_SIMULATOR` + call/lesson/module/scenario/trainee metadata
- GoWalter speaker role mapping and the [AGENT, VISITOR] channel swap for training calls
- Marking a call as training through the four required training metadata keys and persisting the resulting conversation source

**Shared parent truth**

- Content/VA config semantics: `training-content`
- What a task run represents and how conversation is linked: `assignment-and-session`
- How the produced conversation becomes scored: `evaluation`

**Out of scope**

- LiveKit/media vendor operations, TTS/STT vendor capabilities themselves
- General voice-agent/VA platform behavior outside the training path (documented where needed only)

## Semantics and Invariants

- In training, **AI plays the customer**, human plays the agent (opposite of normal voice-agent calls).
- FE requests a module attempt, randomly selects one scenario from the module, and launches the scenario's umbrella VA pinned to `virtual_agent_revision_id`.
- Each scenario currently compiles into an `UMBRELLA_VA` plus a `SINGLE_PROMPT_SUB_VA`; both use `AI_AGENT_APP_TRAINING_SIMULATOR`. The sub-VA prompt combines context/objective/shared guidelines and uses the initial visitor message as its welcome message.
- Director passes a fresh platform call ID plus lesson/module/scenario IDs and the full trainee user name. `training_session_id` currently contains that call ID, not the DirectorTask name.
- GoWalter classifies the conversation as training only when all four session/lesson/module/scenario metadata keys are present. It separately uses `training_agent_user_name` to assign the conversation to the human trainee.
- GoWalter swaps speaker roles so channel 0 / AI customer is `VISITOR` and channel 1 / human trainee is `AGENT`, including diarized processing.
- Director does not pre-create or close the conversation. It resolves GoWalter's conversation by platform call ID, subscribes to messages, and disconnects the call; GoWalter finalizes and closes the conversation server-side.

## Architecture and Source Map

- **Frontend:** `director/.../features/training-simulator/simulation`, `training-conversation`; LiveKit launch in FE, conversation polling by platform ID, then live message subscription
- **Backend/services:**
  - Scenario materialization: `go-servers/apiserver/internal/trainingsimulator/action_batch_create_training_scenarios.go`, `action_batch_update_training_scenarios.go`, and `constants.go`
  - GoWalter: `go-servers/voice-integration/gowalter/internal/voicesession/streamingvoicesession.go` and `utils.go` for source, ownership, role mapping, transcript finalization, and close
  - Virtual Agent service: revisioned umbrella/sub-VA execution; the scenario persists the umbrella ID and revision ID
- **APIs:** (runtime uses LiveKit/VA platform; no dedicated training RPC for starting a call)
- **Configuration/flags:** VA `app: AI_AGENT_APP_TRAINING_SIMULATOR`; `TRAINING_SIMULATOR_MAIN_MODEL`; training VAD flags; conversation source `TRAINING_SIMULATOR = 13`

## Operational Knowledge

- Training classification requires all four metadata keys; trainee ownership is a separate, fail-open metadata path.
- `custom.call.simulated=true` currently causes GoWalter to skip PII redaction, so trainees must not use real customer data and the retention/data-handling policy must be explicit.
- The pinned VA revision lives on the conversation, while the task run stores stable content names. Historical reconstruction requires joining both artifacts.
- Audio/transcripts flow through STT/TTS/LLM infrastructure; runtime changes can alter evaluation inputs and per-attempt cost.
- Hang-up handling and VAD settings for simulated customers were fixed during development (Flux VAD settings, hang up tool #29079).

## Legacy Sources and Cases

- Training Simulator Design — "Call Initiation (voice-agent / LiveKit)" and "Pipeline construction" sections with run_bot/VA-load/pipeline details
- CONVI-6923 (new conversation source TRAINING_SIMULATOR), voice setup PR #30269, role swap fixes (Jack Jee update)

## Open Questions

- Chat (non-voice) simulation flow parity with voice path.
- Repair/reconciliation for a valid training conversation whose task-run creation fails.
- Explicit SLOs and cross-service correlation for launch, conversation resolution, close, and transcript finalization.
- Whether VAD settings placed on the initial sub-VA are consumed consistently by the active voice runtime; the proto warns that initial-sub-VA settings may not be used.
