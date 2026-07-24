# Manager Leaderboard: How the Date Filter Works Today

## Purpose

This report explains how the date range filter on the Manager Leaderboard page affects each metric. Today, different metrics on the same page interpret the date range differently — this document makes that behavior explicit so we can decide whether and how to unify it.

## Background: What Does "Last 30 Days" Mean?

When a user selects a date range on the Manager Leaderboard (e.g., "Jun 18 – Jul 18"), they expect every number on the page to reflect manager activity within that window. But behind the scenes, each metric answers a slightly different question about *when* something happened.

There are four different timestamps in play, each attached to a different real-world event:

### The Four Timestamps

**1. Conversation time** — "When did the agent's conversation happen?"

This is when the customer-agent conversation started. It represents the moment the work being evaluated actually occurred.

*Example: An agent had a call with a customer on July 1st.*

**2. Scorecard submission time** — "When did the manager submit the evaluation?"

This is when the manager finished filling out and submitted the scorecard. It represents the moment the manager did the evaluation work.

*Example: A manager evaluates the July 1st call and submits the scorecard on July 10th.*

**3. Manager page visit time** — "When did the manager visit the page?"

This is when the manager opened a page in the Cresta app (e.g., viewed a live conversation, opened a closed conversation for review). It represents the manager's engagement with the platform.

*Example: A manager browses closed conversations on July 5th.*

**4. Coaching session submission time** — "When did the manager submit the coaching session?"

This is when the manager finished and submitted a coaching session for an agent. It represents when the coaching activity was completed.

*Example: A manager conducts a coaching session with an agent and submits it on July 12th.*

---

## How Each Metric Uses the Date Filter

The table below shows every metric on the Manager Leaderboard page. The **"What the date filter actually means"** column explains what must fall within the selected date range for a data point to be counted.

| Metric | What the date filter actually means | Timestamp used |
|--------|-------------------------------------|----------------|
| **All Page Views** | The manager visited a page *during* the selected dates | Manager page visit time |
| **Live Convo Page Views** | The manager viewed a live conversation page *during* the selected dates | Manager page visit time |
| **Closed Convo Page Views** | The manager viewed a closed conversation page *during* the selected dates | Manager page visit time |
| **Scorecards Evaluated** | The *conversation being scored* happened during the selected dates | Conversation time |
| **Comments Placed** | The *scorecard was submitted* during the selected dates | Scorecard submission time |
| **Coaching Sessions Submitted** | The manager *submitted the coaching session* during the selected dates | Coaching session submission time |
| **Live Assist (gave)** | The *conversation where live assist was given* started during the selected dates | Conversation time |

---

## Why This Matters: A Concrete Example

Consider a manager named Sarah. She selects the date range **July 1–7** on the Manager Leaderboard. Here is what she did that week:

| What Sarah did | When it happened | Details |
|----------------|-----------------|---------|
| Viewed 15 closed conversations | July 2 | Reviewing agent work |
| Submitted 3 scorecards | July 3 | Scoring conversations from **June 25** |
| Left comments on 2 scorecards | July 5 | Commenting on scorecards submitted in **June** |
| Submitted 1 coaching session | July 4 | For an agent she reviewed |
| Gave live assist to an agent | July 6 | During a conversation that started **July 6** |

**What Sarah sees on the Manager Leaderboard for July 1–7:**

| Metric | Value shown | Why |
|--------|-------------|-----|
| All Page Views | **15** | She visited 15 pages during July 1–7 |
| Scorecards Evaluated | **0** | The conversations she scored were from June 25 — outside the July 1–7 range |
| Comments Placed | **0** | The scorecards she commented on were submitted in June — outside the range |
| Coaching Sessions | **1** | She submitted the coaching session on July 4 — within the range |
| Live Assist (gave) | **1** | The conversation started on July 6 — within the range |

**The problem**: Sarah submitted 3 scorecards and left 2 comments during this week, but the leaderboard shows **0** for both. Her actual evaluation work that week is invisible because the date filter looks at *when the original conversation happened* (for scorecards) and *when the scorecard was submitted* (for comments), not *when Sarah did the work*.

A different manager who scored recent conversations (from the same week) would show a non-zero count, even if they did the same amount of work as Sarah.

---

## Summary of Inconsistencies

### 1. "Scorecards Evaluated" looks at conversation time, not evaluation time

The count of scorecards a manager evaluated is filtered by when the *conversation* happened, not when the *evaluation* happened. This means:

- Managers who evaluate older conversations appear less productive
- A backlog-clearing effort (evaluating last month's conversations) won't show up on this month's leaderboard
- The metric doesn't answer "how many scorecards did this manager complete this week?"

### 2. "Comments Placed" looks at scorecard submission time

Comments are filtered by when the *scorecard was submitted*, not when the *comment was written*. This creates a different mismatch from scorecards:

- A comment left today on a scorecard submitted last month won't appear in this month's view
- "Comments Placed" and "Scorecards Evaluated" use different timestamps, so their date ranges don't align with each other or with the manager's actual activity

### 3. Page views, coaching sessions, and live assist each use their own event time

These three metrics correctly reflect "what happened during the selected period" from the manager's perspective. But they each reference a different underlying event, which is appropriate since they measure different activities.

### 4. No metric uses a single consistent definition

Across the six metrics on the page, four different timestamps are used. No two of the "coaching activity" metrics (scorecards, comments, coaching sessions) share the same timestamp.

---

## Options for Unification

The core question is: **should the Manager Leaderboard measure "what managers did during this period" or "what conversations were covered during this period"?**

### Option A: Unify on manager activity time

Filter every metric by *when the manager performed the action*:

- Scorecards Evaluated → filter by scorecard submission time
- Comments Placed → filter by comment creation time
- Coaching Sessions → filter by coaching session submission time (already correct)
- Page Views → filter by page visit time (already correct)
- Live Assist → filter by when live assist was given (close to current behavior)

**Pros**: The leaderboard answers "how active was this manager during this period?" — intuitive for tracking manager engagement and workload.

**Cons**: A manager could score an old conversation and it would count toward this period, even though the agent interaction was months ago.

### Option B: Unify on conversation time

Filter every metric by *when the underlying conversation happened*:

- Scorecards Evaluated → filter by conversation time (current behavior)
- Comments Placed → filter by conversation time of the scored conversation
- Coaching Sessions → filter by the conversation(s) discussed in the session
- Page Views → harder to map — not all page views relate to a specific conversation
- Live Assist → filter by conversation start time (current behavior)

**Pros**: The leaderboard answers "how much coverage did managers provide for conversations in this period?"

**Cons**: Page views don't have a natural conversation association. Manager work done weeks after a conversation happened would appear under the old date range, making it hard to track current manager activity.

### Option C: Hybrid — use manager activity time for manager-centric metrics

Use the timestamp that best matches each metric's purpose:

- **Manager activity metrics** (page views, coaching sessions): keep manager activity time
- **Evaluation metrics** (scorecards evaluated, comments placed): unify on scorecard submission time
- **Agent-facing metrics** (live assist): keep conversation time

**Pros**: Each metric uses the timestamp most relevant to its meaning. Evaluation metrics become consistent with each other.

**Cons**: Still uses multiple timestamps on one page, though the reasoning is clearer.

---

## Recommendation

We recommend discussing which question the Manager Leaderboard is primarily meant to answer — "how active are my managers?" vs. "how well are conversations being covered?" — and then aligning the timestamps accordingly. The current state, where each metric silently uses a different definition of "during this period," leads to confusing results and undermines trust in the numbers.
