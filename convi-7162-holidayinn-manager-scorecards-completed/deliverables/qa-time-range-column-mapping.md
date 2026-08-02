# QA Time Range Filter → ClickHouse Column Mapping

**Date:** 2026-07-30
**Context:** CONVI-7162 Manager Leaderboard submit-time fix
**Code:** `go-servers` worktree `go-servers-convi-7162` (`retrieve_qa_score_stats_clickhouse.go`, `common_clickhouse.go`)

Explains how `filter_by_time_range` is translated into physical ClickHouse columns **before** and **after** the `time_range_filter_target` change.

---

## Shared pipeline (unchanged shape)

The pipeline is a **two-step rename**, then a `WHERE` / `DATE_TRUNC` on the resulting physical column.

```mermaid
flowchart LR
  A["filter_by_time_range<br/>+ request flags"] --> B["Step 1: logical column<br/>getConversationTimeRangeColumn()"]
  B --> C["Step 2: table remap<br/>tableSpecificColumnName()"]
  C --> D["Physical CH column<br/>used in WHERE / DATE_TRUNC"]
```

### Step 1 — logical column (`getConversationTimeRangeColumn`)

| `conversation_time_range_field` | Logical column |
|---|---|
| unspecified / started-at (default) | `conversation_start_time` |
| ended-at | `conversation_end_time` |

### Step 2 — table remap (`tableColumnNameMapping` for score/scorecard tables)

| Logical | On `score_d` / `scorecard_d` | On `conversation_d` |
|---|---|---|
| `conversation_start_time` | **`scorecard_time`** | `conversation_start_time` |
| `conversation_end_time` | column-not-exist (needs conversation join) | `conversation_end_time` |

So before our change, “default QA time range” meant:

`filter_by_time_range` → `conversation_start_time` → **`scorecard_time`** (interaction / convo start).

---

## Before the change

Only knobs: `filter_by_time_range` + `conversation_time_range_field`.

```mermaid
flowchart TD
  REQ["RetrieveQAScoreStats / Conversations<br/>filter_by_time_range"]

  REQ --> STEP1{"conversation_time_range_field?"}
  STEP1 -->|default / STARTED_AT| L1["logical: conversation_start_time"]
  STEP1 -->|ENDED_AT| L2["logical: conversation_end_time"]

  L1 --> MAP1{"Which table?"}
  MAP1 -->|score_d / scorecard_d| P1["physical: scorecard_time"]
  MAP1 -->|conversation_d if joined| P2["physical: conversation_start_time"]
  MAP1 -->|moment filters| P3["physical: conversation_start_time<br/>on moment tables"]

  L2 --> MAP2{"Which table?"}
  MAP2 -->|score_d / scorecard_d| X["no end column → join conversation_d"]
  MAP2 -->|conversation_d| P4["physical: conversation_end_time"]

  P1 --> SQL1["WHERE scorecard_time >= start AND < end<br/>DATE_TRUNC(..., scorecard_time)"]
```

**Manager Leaderboard (pre-fix):** never set ended-at → always landed on **`scorecard_time`**.

---

## After the change

Same two steps, plus an optional **override** from `time_range_filter_target`:

```mermaid
flowchart TD
  REQ["filter_by_time_range<br/>+ conversation_time_range_field<br/>+ time_range_filter_target NEW"]

  REQ --> STEP1{"time_range_filter_target?"}
  STEP1 -->|UNSPECIFIED / INTERACTION_TIME| OLD["Same as before:<br/>started→conversation_start_time<br/>ended→conversation_end_time"]
  STEP1 -->|SUBMIT_TIME| OVR["override logical/physical to<br/>scorecard_submit_time<br/>skip started/ended mapping"]

  OLD --> MAP["tableSpecificColumnName()"]
  MAP -->|score/scorecard| SC["scorecard_time"]
  MAP -->|conversation_d| CT["conversation_start/end_time"]

  OVR --> SUB["scorecard_submit_time<br/>no remap needed"]

  SC --> SQLA["WHERE / DATE_TRUNC on scorecard_time"]
  SUB --> SQLB["WHERE / DATE_TRUNC on scorecard_submit_time"]

  STEP1 -->|SUBMIT_TIME also| SKIP["conversation_d date range cleared<br/>do not re-filter by convo time"]
  SKIP --> MOM["moment filters still use<br/>conversation start/end — unchanged"]
```

### What the new field does in code

1. `qaTimeRangeFilterTargetOptions(...)`
   - default: `WithTimeRangeType(conversation_time_range_field)` only
   - `SUBMIT_TIME`: also `WithTimeRangeColumn("scorecard_submit_time")`
2. That override makes Step 1 return `scorecard_submit_time` directly, so Step 2 does **not** turn it into `scorecard_time`.
3. For `conversation_d`, submit-time mode passes `nil` time range so the selected window is **not** applied again as conversation start/end.
4. Moment filters still use conversation start/end (intentionally not submit time).
5. No runtime rejection if `SUBMIT_TIME` is combined with `CONVERSATION_ENDED_AT`; Manager (and other callers) simply do not set both. If both were set, the submit-time override wins on score/scorecard tables.

---

## Side-by-side: same Manager request

Assume: date range Jun 15–21, default started-at, scorecard resource.

| Stage | Before | After (`SUBMIT_TIME`) |
|---|---|---|
| Request flags | no target field | `time_range_filter_target=SUBMIT_TIME` |
| Step 1 logical | `conversation_start_time` | `scorecard_submit_time` (override) |
| Step 2 on `scorecard_d` | → `scorecard_time` | stays `scorecard_submit_time` |
| Filter SQL | `scorecard_time >= … AND < …` | `scorecard_submit_time >= … AND < …` |
| Daily group | `DATE_TRUNC(..., scorecard_time)` | `DATE_TRUNC(..., scorecard_submit_time)` |
| `conversation_d` time filter | may apply same range as start time if join needed | **not** applied for the date range |
| Moment time filter | conversation start | conversation start (unchanged) |

---

## Mental model

- **Before:** “time range” on QA scorecard tables always meant **interaction time**, because logical “conversation start” is aliased to `scorecard_time`.
- **After:** same translation **unless** Manager (or anyone) sets `SUBMIT_TIME`, which short-circuits the alias and points filter/group at **`scorecard_submit_time`**. Unspecified callers keep the old path.
