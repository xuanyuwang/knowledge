# Daily Engineering Notes – 2025-11-25

## 1. Fixes (Bugs / Issues Resolved)
### Problem:
When scorecards are created, updated, or submitted via the coaching API, the system performs synchronous writes to the primary Postgres schema, followed by asynchronous updates to multiple other places, including
- **Postgres (director schema)**: Source of truth for scorecards and scores
- **Postgres (historic schema)**: Intermediate format used for ClickHouse transformation
- **ClickHouse**: Analytics database for reporting and querying

The problem is, the execution order of async work function, let's say `asyncWork`, are not in the same order as those APIs are called. Therefore, it's possible that an earlier call of asyncWork overwrite CH with stale data.


### Symptoms
The data of scorecard is not consistent across director schema and clickhouse. Some are submitted in director schema but not in CH.
### Root Cause:
Unpredictable execution order of async work of each API
### How I Diagnosed It:
It's hard to find the cause, so I have to rule out other possibilities, and locally hijack the asyncWork call to execute them in different order
### Final Fix:
First try: always use `update_at` from director schema as `update_time` column, which is the version column, in CH. Since the table is using ReplaceMergeTree engine, then the execution order does not matter.
Result: failed because there are other cron job or Temporal job that have assumptions that the `udpate_time` is using `time.Now()`, therefore new data is treated at old data and then delted

Second try: each asyncWork execution always read the latest data from director schema. I refactored the code, by writing data in the API sync logic, so that the director schema and historic schema always have the latest data. Then each asyncWork always update CH to the latest data. Therefore, the execution order does not matter. Since CH data can have a short latency because data in CH is used in data analysis.
### Preventative Ideas:
Data quality monitoring.

## 2. Learnings (New Knowledge)
- What I learned:
- Context:
- Why it's important:
- Example:
- When to apply:

## 3. Surprises (Unexpected Behavior)
- What surprised me:
- Expected vs actual behavior:
- Why it happened:
- Takeaway:

## 4. Explanations I Gave
- Who I explained to (team / code review / slack):
- Topic:
- Summary of explanation:
- Key concepts clarified:
- Possible blog angle:

## 5. Confusing Things (First Confusion → Later Clarity)
- What was confusing:
- Why it was confusing:
- How I figured it out:
- Clean explanation (my future-self will thank me):
- Mental model:

## 6. Things I Googled Multiple Times
- Search topic:
- Why I kept forgetting:
- Clean “final answer”:
- Snippet / Command / Example:

## 7. Code Patterns I Used Today
- Pattern name:
- Situation:
- Code example:
- When this pattern works best:
- Pitfalls:

## 8. Design Decisions / Tradeoffs
- Problem being solved:
- Options considered:
- Decision made:
- Tradeoffs:
- Why this matters at a system level:
- Future considerations:

---

## Screenshots
(Drag & paste images here)

## Raw Snippets / Logs
\`\`\`
Paste raw logs, stack traces, or snippets here
\`\`\`

## Blog Potential
- Short post ideas:
- Deep-dive post ideas: today's bug worth a deep-dive post
