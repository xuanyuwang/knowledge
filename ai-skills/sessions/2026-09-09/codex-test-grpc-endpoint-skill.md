# Test gRPC endpoint skill

Date: 2026-09-09  
Source repo: `/Users/xuanyu.wang/repos/coaching-qm-skills`  
Branch: `xw/test-grpc-endpoint-skill`

## Goal

Turn the CONVI-7656 RPC-testing workflow into a reusable repository skill without hardcoding one ticket, endpoint, or API contract.

## Design

- Skill name: `test-grpc-endpoint`.
- Use direct TLS gRPC by default; allow plaintext only when explicitly selected for a trusted local endpoint.
- Obtain customer bearer tokens at call time with `cresta-cli`; never accept a caller-provided authorization header or persist the token.
- Treat reflection as schema discovery only. A valid method invocation is required to establish deployment.
- Derive expected results independently from an approved read-only data source and state aggregation/revision semantics before comparing.
- Distinguish business-logic mismatches from auth failures, unavailable data, and undeployed handlers.

## Files

- `test-grpc-endpoint/SKILL.md`
- `test-grpc-endpoint/agents/openai.yaml`
- `test-grpc-endpoint/scripts/call-grpc-endpoint.zsh`
- Root `README.md` skill inventory

## Validation

- `zsh -n` passed.
- The helper rejects caller-supplied authorization metadata and mutually exclusive request-data options.
- Skill Creator `quick_validate.py` passed using an isolated PyYAML runtime.
- A live read-only call to `RetrieveTrainingSimulatorLessonStats` on voice-staging returned the expected lesson aggregate.

## Delivery

- Commit: `085fa85` (`Add gRPC endpoint testing skill`), rebased onto current `origin/main`.
- Linear issue: [CONVI-7659](https://linear.app/cresta/issue/CONVI-7659/add-reusable-grpc-endpoint-testing-skill).
- Pull request: [coaching-qm-skills#2](https://github.com/cresta/coaching-qm-skills/pull/2), titled `CONVI-7659 Add gRPC endpoint testing skill`.

## Review follow-up

- Commit `cef6f53` enforces loopback-only plaintext transport by resolving the target before auth and replacing the grpcurl target with the resolved loopback address.
- Added `references/read-only-methods.txt`; the helper now rejects every unlisted RPC before token acquisition. The initial allowlist contains only the reviewed module- and lesson-statistics retrieval methods.
- Verified remote plaintext rejection, mutating-method rejection, local plaintext acceptance through the transport guard, a live allowed TLS request, shell syntax, and skill package validation.
- Replied to and resolved both review threads. CodeRabbit re-review was still pending at handoff; codeowner, PR-format, and Linear-ticket checks passed.
