# References

## Design and schema

| Resource | URL / path |
|----------|------------|
| Unified moments & actions doc (adherence types section) | https://docs.google.com/document/d/12Y6JsOaVHkgo7lM-h8B10dW9xHDW-0b1CtP1EsfEzrQ/edit#heading=h.75a34mg3eey3 |
| `AdherenceType` enum | `apiserver/sql-schema/protos/moment/moment_annotation.proto` |
| `BehaviorConfig` (negative adherence) | `apiserver/sql-schema/protos/behavior/behavior.proto` |
| Studio schema comment linking doc | `studio-server/sql-schema/v2-schema.sql` (moment_adherence_type) |

## go-servers code

| Area | Path |
|------|------|
| Window filtering | `shared/scoring/autoqa_dao.go` |
| Filter tests | `shared/scoring/autoqa_dao_test.go` |
| Evidence → outcome | `shared/scoring/autoqa_scoring.go` |
| Coach builder dedup | `orchestrator/internal/nodes/policyengine/post_process_coach_builder_node.go` |
| Hint finished-policy logic | `shared/stateconverter/hint_state_processor.go` |
| Email integration tests | `apiserver/internal/autoqa/action_calculate_conversation_autoscoring_evidence_test.go` |

## Pull requests

| PR | Title |
|----|-------|
| [#27040](https://github.com/cresta/go-servers/pull/27040) | Include all behavior annotations when any annotation matches agent message |
| [#27494](https://github.com/cresta/go-servers/pull/27494) | Score all agents between SDX and DDX/DNX for email |
| [#29694](https://github.com/cresta/go-servers/pull/29694) | Support SNX behaviors in email AutoQA window filtering |

## Tickets

| ID | Title |
|----|-------|
| [COA-2566](https://linear.app/cresta/issue/COA-2566/marriott-cec-email-rules-stopped-triggering-after-0330) | Marriott CEC email rules stopped triggering after 03/30 |

## Internal discussion

| Source | Topic |
|--------|-------|
| [#convo-intelligence, 2026-06-09](https://crestalabs.slack.com/archives/C04NB5AMV0F/p1781022524212219) | Ural: done vs not-done relevance for email filtering |
| [#engineer-ing, 2025-01-29](https://crestalabs.slack.com/archives/CCKNEJGF5/p1738171524175319) | Unified moments doc access (`Eng@cresta.ai`) |

## Glossary

| Term | Definition |
|------|------------|
| SDX | Should do X — window opens |
| DDX | Did do X — positive outcome |
| DNX | Did not do X — negative outcome |
| SNX | Should not do X — negative-adherence trigger |
| Synthetic SDX | `SHOULD_DO_X` anchor for SNX window machinery |
| NOX | No opportunity to do X |
| AutoQM / AutoQA | Automated quality management scoring |
