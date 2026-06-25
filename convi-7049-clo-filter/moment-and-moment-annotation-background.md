# Moment and Moment Annotation Background

**Date:** 2026-06-23
**Context:** CONVI-7049 CLO filter in Performance Insights

## Short Version

A **Moment** is the configured thing that can happen in or be assigned to a conversation. A **MomentAnnotation** is the actual occurrence or value of that moment on a specific conversation or message.

The closest analogy is:

```text
Scorecard template  -> Scorecard / score result
Moment              -> MomentAnnotation
```

For CLO, the moment is the configured Cresta Modeled Outcome. The moment annotation is the predicted outcome value for one conversation.

## Product Meaning

Internal docs describe moments as events that may happen in a conversation based on metadata, detection, generation, or policy definition. When that event actually happens in a conversation, the system creates a moment annotation.

Examples:

- A keyword moment defines what keyword or phrase to detect.
- A keyword moment annotation records that the keyword was detected in a specific message.
- A conversation metadata moment defines a metadata field.
- A conversation metadata moment annotation records the value of that field for a conversation.
- A conversation outcome moment defines an outcome such as issue resolution, conversion, or CSAT.
- A conversation outcome moment annotation records the outcome value for a conversation.

Source context:

- Glean result: **Moments, Policies, and Actions Master Doc**
- Code: `apiserver/sql-schema/protos/moment/moment.proto`
- Code: `apiserver/sql-schema/protos/moment/moment_annotation.proto`

## CLO Mapping

For CONVI-7049, CLO means **Cresta Modeled Outcome**.

In code, CLO is stored as:

```text
moment type: MOMENT_TYPE_CONVERSATION_OUTCOME
DB enum value: 14
annotation payload field: conversation_outcome
value type: oneof number_value | string_value | boolean_value
```

Relevant code references:

- `/Users/xuanyu.wang/repos/go-servers-convi-7049/apiserver/sql-schema/protos/moment/moment.proto:41`
- `/Users/xuanyu.wang/repos/go-servers-convi-7049/apiserver/sql-schema/protos/moment/moment_annotation.proto:29`
- `/Users/xuanyu.wang/repos/go-servers-convi-7049/apiserver/sql-schema/protos/moment/moment_annotation.proto:569`

## How Moment Annotations Are Generated

There are multiple producers, but they converge on the MomentService APIs and end up in `app.moment_annotations`.

The external/service API path is:

```text
BatchCreateMomentAnnotations
  -> assign/fill moment annotation resource name and create_time
  -> ConvertServiceMomentAnnotationToDBMomentAnnotation
  -> insert into app.moment_annotations
```

Code references:

- `/Users/xuanyu.wang/repos/go-servers-convi-7049/apiserver/internal/moment/action_batch_create_moment_annotations.go:22`
- `/Users/xuanyu.wang/repos/go-servers-convi-7049/shared/converters/momentconverter/moment_annotation_converters.go:239`

The internal AI/orchestrator path is:

```text
BatchCreateInternalMomentAnnotations
  -> fill missing annotation ID and conversation metadata
  -> AIMomentAnnotationsToDB
  -> insert into app.moment_annotations
```

Code references:

- `/Users/xuanyu.wang/repos/go-servers-convi-7049/apiserver/internal/moment/action_batch_create_internal_moment_annotations.go:25`
- `/Users/xuanyu.wang/repos/go-servers-convi-7049/shared/converters/momentconverter/ai_moment_annotation_converter.go:26`

For CLO specifically, the orchestrator path is:

```text
PredictConversationOutcome
  -> find CONVERSATION_OUTCOME moment templates in conversation config
  -> skip already-annotated templates unless recompute is enabled
  -> build AI service request
  -> call conversation outcome service
  -> return outcome moment annotations
  -> persistence path writes them to app.moment_annotations
```

Code reference:

- `/Users/xuanyu.wang/repos/go-servers-convi-7049/orchestrator/internal/nodes/conversationoutcome/predict_conversation_outcome_node.go:45`

Internal product context from Glean:

- **Outcome Management v2 PRD** says Cresta-modeled outcomes are expected to be deployed/routed through `CONVERSATION_OUTCOME` and written back as system-sourced moments.
- **Cresta modeled outcomes inventory** lists modeled outcome examples such as conversion, issue resolved, Cresta AHT, CSAT, and conversation-level sentiment.

## Postgres Storage

The source-of-truth table is:

```text
app.moment_annotations
```

Schema reference:

- `/Users/xuanyu.wang/repos/go-servers-convi-7049/apiserver/sql-schema/app/app-schema.sql:1828`
- `/Users/xuanyu.wang/repos/go-servers-convi-7049/apiserver/sql-schema/gen/model/moment_annotations.go:57`

Important columns:

```text
customer_id
profile_id
conversation_id
message_id
moment_annotation_id
moment_template_id
policy_id
behavior_id
type
detailed_type
adherence_type
taxonomy
taxonomy_reference_id
payload jsonb
labels jsonb
parent_action_annotation_id
adherence_moment_annotation_id
adherence_action_annotation_id
created_at
db_created_at
usecase_id
language_code
```

Primary key:

```text
(customer_id, profile_id, conversation_id, moment_annotation_id)
```

For CLO, the important Postgres fields are:

```text
type = 14
moment_template_id = outcome template ID
taxonomy = outcome taxonomy
payload = JSONB MomentAnnotationPayload with conversation_outcome value
```

Payload examples:

```json
{"conversation_outcome":{"boolean_value":true}}
{"conversation_outcome":{"string_value":"resolved"}}
{"conversation_outcome":{"number_value":42}}
```

Some API-facing JSON may use camelCase field names, but the DB proto shape uses `conversation_outcome` and snake_case when marshaled with proto names.

## ClickHouse Storage

ClickHouse stores a denormalized analytics row in:

```text
moment_annotation
moment_annotation_d
```

`moment_annotation_d` is the distributed table over the local `moment_annotation` table.

Schema/code references:

- `/Users/xuanyu.wang/repos/go-servers-convi-7049/shared/clickhouse/testing/schemas/ch_conv_schema.sql:166`
- `/Users/xuanyu.wang/repos/go-servers-convi-7049/shared/clickhouse/testing/schemas/ch_conv_schema.sql:222`
- `/Users/xuanyu.wang/repos/go-servers-convi-7049/shared/clickhouse/conversations/conversation.go:114`

Important ClickHouse columns:

```text
conversation_id
message_id
moment_annotation_id
moment_template_id
moment_type
moment_detailed_type
adherence_type
policy_id
behavior_id
taxonomy
conversation_start_time
conversation_end_time
create_time
update_time
agent_user_id
usecase_id
metadata_bool_value
metadata_number_value
metadata_string_value
metadata_type
metadata_source
moment_annotation_payload
```

ClickHouse rows are not a direct PG copy. The writer enriches the annotation with conversation, message, policy, user, and derived analytics fields.

For `CONVERSATION_METADATA`, the ClickHouse writer extracts:

```text
metadata_bool_value
metadata_number_value
metadata_string_value
metadata_type
metadata_source
```

For CLO, PR `cresta/go-servers#29175` makes sure `moment_annotation_payload` is populated for `CONVERSATION_OUTCOME` rows so analytics can query the payload JSON.

Code references:

- `/Users/xuanyu.wang/repos/go-servers-convi-7049/shared/clickhouse/conversations/conversation.go:2350`
- `/Users/xuanyu.wang/repos/go-servers-convi-7049/shared/clickhouse/conversations/conversation.go:2387`
- `/Users/xuanyu.wang/repos/go-servers-convi-7049/shared/clickhouse/conversations/conversation.go:2415`

## `moment_annotation_d` vs `moment_annotation_mv_by_metadata_d`

`moment_annotation_d` is the broad table. It contains all moment annotation types, including:

- keyword
- intent
- conversation metadata
- conversation outcome
- sentiment
- emotion
- knowledge
- policy/adherence moments

`moment_annotation_mv_by_metadata_d` is a distributed table over a materialized view pre-filtered to conversation metadata only.

View definition:

```sql
SELECT
  conversation_id,
  conversation_start_time,
  moment_template_id,
  metadata_string_value,
  metadata_number_value,
  metadata_bool_value,
  update_time
FROM moment_annotation
WHERE moment_type = 19;
```

Schema reference:

- `/Users/xuanyu.wang/repos/go-servers-convi-7049/shared/clickhouse/testing/schemas/ch_conv_schema.sql:574`

The metadata view is fast for metadata filters because it avoids scanning all moment annotation types and only projects metadata value columns.

It cannot serve CLO filters because:

- CLO is `moment_type = 14`, not `moment_type = 19`.
- The view does not contain CLO rows.
- The view does not project `moment_annotation_payload`.
- CLO values live in the `conversation_outcome` payload, not in `metadata_*_value` columns.

That is why the CONVI-7049 backend path uses `moment_annotation_d` when a request contains a CLO filter.

## Possible Future Optimization

A CLO-specific materialized view could be useful if CLO filters become hot or `moment_annotation_d` scans are expensive.

Potential shape:

```text
moment_annotation_mv_by_conversation_outcome_d
  conversation_id
  conversation_start_time
  conversation_end_time
  moment_template_id
  outcome_string_value
  outcome_number_value
  outcome_bool_value
  outcome_value_type
  create_time
  update_time
```

This should be a separate ClickHouse schema ticket because it requires migration/deployment/backfill decisions and query changes. It should not just copy the metadata view shape because CLO values are a typed `oneof`, and `false` / `0` are valid values that must not be confused with missing values.

## Practical Takeaways For CONVI-7049

- Treat `Moment` as the outcome definition/template.
- Treat `MomentAnnotation` as the per-conversation outcome value.
- For CLO filters, look for `CONVERSATION_OUTCOME` moment groups in the request.
- In Postgres, CLO is stored in `app.moment_annotations.payload`.
- In ClickHouse, CLO filtering depends on `moment_annotation_payload`.
- `moment_annotation_mv_by_metadata_d` is metadata-only and cannot return CLO rows.
- A CLO-specific ClickHouse materialized view is a future performance optimization, not required for correctness.

## Questions Raised During Review

### What is a moment annotation?

A moment annotation is the recorded occurrence or value of a moment on a specific conversation or message.

For this work, the useful mental model is:

```text
Moment              = definition/configuration
MomentAnnotation    = actual detected/generated value

Scorecard template  = definition/configuration
Scorecard result    = actual scored instance
```

For CLO specifically:

```text
CONVERSATION_OUTCOME moment template
  -> defines the modeled outcome

CONVERSATION_OUTCOME moment annotation
  -> stores the predicted outcome value for one conversation
```

### How are moment annotations generated?

They can be produced by several systems, but the persisted shape converges on `app.moment_annotations`.

For CLO, the relevant flow is:

```text
PredictConversationOutcome
  -> read CONVERSATION_OUTCOME moment templates from conversation config
  -> call the conversation outcome prediction service
  -> produce outcome moment annotations
  -> persist them through the moment annotation creation path
  -> ClickHouse ingestion denormalizes them into moment_annotation_d
```

### What is the schema for moment annotations in Postgres and ClickHouse?

Postgres source of truth:

```text
app.moment_annotations
  customer_id
  profile_id
  conversation_id
  moment_annotation_id
  moment_template_id
  type
  taxonomy
  payload jsonb
  created_at
  ...
```

For CLO:

```text
type = MOMENT_TYPE_CONVERSATION_OUTCOME = 14
moment_template_id = configured outcome moment
taxonomy = outcome taxonomy
payload.conversation_outcome = oneof boolean_value | string_value | number_value
```

ClickHouse analytics table:

```text
moment_annotation_d
  conversation_id
  moment_annotation_id
  moment_template_id
  moment_type
  taxonomy
  conversation_start_time
  conversation_end_time
  create_time
  update_time
  metadata_bool_value
  metadata_number_value
  metadata_string_value
  moment_annotation_payload
  ...
```

For CLO filters, the backend reads `moment_annotation_payload` because CLO values are not stored in the `metadata_*_value` columns.

### What is the difference between `moment_annotation_mv_by_metadata_d` and `moment_annotation_d`?

`moment_annotation_d` is the broad distributed ClickHouse table for all moment annotations.

`moment_annotation_mv_by_metadata_d` is a metadata-only materialized view. Its source filter is:

```sql
WHERE moment_type = 19
```

It only projects columns needed for conversation metadata filters, such as `metadata_string_value`, `metadata_number_value`, `metadata_bool_value`, and `update_time`.

It cannot serve CLO filters because CLO is `moment_type = 14` and CLO values live inside `moment_annotation_payload`.

### Should we create a similar materialized view for `moment_type = 14`?

Not for the correctness fix in CONVI-7049.

A CLO-specific materialized view may be a good follow-up optimization if the broad `moment_annotation_d` scan becomes expensive. It should be a separate ticket because it needs a schema design, migration, rollout, and possibly backfill. It should also preserve type presence explicitly because `false` and `0` are valid CLO values.

### Why does PR 29175 use `moment_annotation_d` for CLO instead of silently using the metadata view?

The metadata view is pre-filtered to conversation metadata rows and does not project `moment_annotation_payload`. Even if `clickHouseEnableMomentAnnotationPayload` is enabled, the metadata view still cannot return CLO rows.

The backend therefore checks the requested moment type and routes:

```text
CONVERSATION_METADATA -> moment_annotation_mv_by_metadata_d
CONVERSATION_OUTCOME  -> moment_annotation_d
```

### How do we know `create_time` is the latest-time column for CLO rows?

The metadata view exposes `update_time`, so metadata filtering uses `update_time` when querying `moment_annotation_mv_by_metadata_d`.

For CLO, the query uses `moment_annotation_d`. In that table, the existing "latest annotation per conversation" CTE pattern used `create_time` before the metadata-view optimization. The metadata view branch is the special case that switches to `update_time`; falling back to `moment_annotation_d` preserves the broad-table behavior with `create_time`.

### Can a `CONVERSATION_OUTCOME` value be either `NumericBin` or `OutcomeValue`?

In FE request shape, yes: `analyticspb.MomentGroup.OutcomeValueAttributes` may contain either:

```text
outcome_value = exact boolean/string/number value
numeric_bin   = range bucket for numeric outcomes
```

The protobuf relationship is:

```text
MomentGroup
  moments[]                -> which outcome definition/template
  outcome_value_attributes -> which outcome values or numeric bins are selected
```

The concrete exact CLO value is represented by `ConversationOutcomeValue`, whose value is a protobuf `oneof`:

```text
boolean_value
string_value
number_value
```

Numeric bins are not a separate stored outcome type. They are a FE/API filter representation for ranges over stored numeric outcome values.

### Why do FE/test setups need both `Moments` and `OutcomeValueAttributes`?

`Moments` identifies the outcome definition to filter on:

```text
moment_template_id = outcome_moment_id
moment_type = CONVERSATION_OUTCOME
```

`OutcomeValueAttributes` identifies the selected values for that outcome:

```text
string value = "resolved"
number bin = [10, 20)
boolean value = false
```

Using only `Moments` would mean "has this outcome moment." Using only `OutcomeValueAttributes` would not say which outcome definition the values belong to. CLO filtering needs both.
