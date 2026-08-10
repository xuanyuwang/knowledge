# Simulation Runtime

## Purpose

How an agent starts a module attempt and converses with the simulated customer: the Customer AI virtual agent, voice-agent/LiveKit pipeline, GoWalter role/channel mapping, and conversation creation with training metadata.

## Scope and Boundaries

**In scope**

- Customer AI simulator virtual agents (persona: context, visitor objective, initial message, model) vs normal AI-agent-as-agent
- Voice-agent (python) `run_bot()`: VA load via `VirtualAgentService.GetVirtualAgentRevision`, Pipecat pipeline construction (LiveKit transport, VAD, STT, `CrestaVALLMService`, TTS, GoWalter)
- Conversation creation in `app.chat` with `conversation_source = TRAINING_SIMULATOR` + training metadata (session/lesson/module/scenario/attempt)
- GoWalter speaker role mapping and the [AGENT, VISITOR] channel swap for training calls
- Marking a call as training: `platform_params` and/or VA `kind`/conversation source branching

**Shared parent truth**

- Content/VA config semantics: `training-content`
- What a task run represents and how conversation is linked: `assignment-and-session`
- How the produced conversation becomes scored: `evaluation`

**Out of scope**

- LiveKit/media vendor operations, TTS/STT vendor capabilities themselves
- General voice-agent/VA platform behavior outside the training path (documented where needed only)

## Semantics and Invariants

- In training, **AI plays the customer**, human plays the agent (opposite of normal voice-agent calls).
- FE requests a module attempt → picks a scenario from the module's pool → `{vaDomain}/livekit/join/room/{room}/{vaPath}`; `vaPath` encodes which Customer AI simulator to load.
- The VA proto returned for a training scenario is effectively a `SINGLE_PROMPT_SUB_VA` (system prompt combining context/objective/behavior) plus model; optionally `CUSTOMER_AI_SIMULATOR` kind with `CustomerAIConfig`, `emotional_states`, function tools, pre/postprocessors.
- Voice-agent builds the Pipecat pipeline from VA config and uses it for every turn (CrestaVALLMService), but **voice-agent does not assign speaker roles** — GoWalter does.
- GoWalter maps participants [Agent(channel0/TTS), Visitor(channel1/STT)] to roles; for training, a flag in `customer_ai_config`/conversation metadata swaps channels so the "customer"/bot is visitor role and the trainee is agent role.

## Architecture and Source Map

- **Frontend:** `director/.../features/training-simulator/simulation`, `training-conversation`; LiveKit join handled in FE; `useConversationDetails()` live polling
- **Backend/services:**
  - Voice-agent (python): `python-ai-services/voice-agent/src/processors/gowalter.py` (hardcodes participants [Agent, Visitor], sends text at L438-444)
  - GoWalter (Go): `go-servers/voice-integration/gowalter/internal/voicesession/messagehandler.go` (channel→role mapping L350-357), conversation creation with training source/metadata
  - Virtual-agent service: `VirtualAgentService.GetVirtualAgentRevision` for Customer AI VA load; batch VA revision creation from scenarios
  - CrestaVALLMService for turn-by-turn LLM (model per scenario)
- **APIs:** (runtime uses LiveKit/VA platform; no dedicated training RPC for starting a call)
- **Configuration/flags:** VA `labels`/`app: TRAINING_SIMULATOR`, `purpose: training_simulator`, `conversation_source` enum `TRAINING_SIMULATOR = 13`

## Operational Knowledge

- Need to mark training calls distinctly (platform_params or branch on VA kind / conversation source) so they are not treated as live or agent-progression traffic.
- Audio/transcripts flow to external providers (STT/TTS, LLM) — compliance/PII review per customer.
- Hang-up handling and VAD settings for simulated customers were fixed during development (Flux VAD settings, hang up tool #29079).

## Legacy Sources and Cases

- Training Simulator Design — "Call Initiation (voice-agent / LiveKit)" and "Pipeline construction" sections with run_bot/VA-load/pipeline details
- CONVI-6923 (new conversation source TRAINING_SIMULATOR), voice setup PR #30269, role swap fixes (Jack Jee update)

## Open Questions

- Chat (non-voice) simulation flow parity with voice path.
- Whether `CUSTOMER_AI_SIMULATOR` kind is used in production or only `SINGLE_PROMPT_SUB_VA`; design docs show both.
