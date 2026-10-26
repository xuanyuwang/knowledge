# Export Appeal Comments - Research

**Created:** 2026-04-14

## 1. How the Comment Column of a Criterion is Built

**File:** `go-servers/apiserver/internal/coaching/action_export_scorecards.go`

### Header Construction (lines 661-687)

`buildCriteriaHeaders()` creates pairs of columns for each criterion:
- `[Criterion Display Name]` — the score value
- `[Criterion Display Name] + " comment"` — the comment

The mapping `criterionIdentifierToHeaderIndex` maps criterion identifier to the index of the score column. The comment column is always at `index + 1`.

### Comment Population (lines 612-637)

In `convertScorecardsToCSVBytes()`:

```go
// Line 632: Gets comment from the first score's Comment field
comment := criterionScores[0].Comment.String

// Lines 633-635: Appends collaboration comments (conversation-level comments)
if convComments := exportData.conversationToCommentMap[scorecard.ConversationID][criterion.GetIdentifier()]; len(convComments) > 0 {
    comment += "\n" + strings.Join(convComments, "\n")
}

// Line 636: Sets the comment at index+1
scoreValues[criterionIdentifierToHeaderIndex[criterion.GetIdentifier()]+1] = comment
```

**Key insight:** The comment comes from `criterionScores[0].Comment.String` — the first score entry for that criterion on the scorecard. This is the original grader's comment. It does NOT check for appeal approval reasons.

---

## 2. How Scorecard Data is Queried for Export

**File:** `go-servers/apiserver/internal/coaching/action_export_scorecards.go`

### Flow

1. **`processScorecardExports()`** (line 261) or **`processScorecardExportsForTimeRange()`** (line 247) — entry points
2. **`createScorecardQuerySupplier()`** (line 379) — builds the base query with filters (time range, agent IDs, team IDs, template IDs, usecase IDs)
3. Scorecards are queried via `scorecardQuerySupplier().Find(&dbScorecards)` (line 272)
4. **`extractLinkedScorecardsDataForExport()`** (line 791) — fetches all linked data:
   - Templates (line 806)
   - Template structures (line 819)
   - Criteria map (line 824)
   - **Scores** (line 826) — via `getLinkedScoresForExport()`
   - Users (line 831)
   - Conversations (lines 839-847)
   - Conversation comments (line 844)
   - Moments taxonomy (line 851)

### Data Structure

```go
type linkedScorecardData struct {
    usersMap                     map[string]*userpb.User
    conversationsMap             map[string]*dbmodel.Chats
    templatesMap                 map[string]*dbmodel.ScorecardTemplates
    templateStructuresMap        map[string]scoring.ScorecardTemplateStructure
    criteriaMap                  map[string]scoring.ScorecardTemplateCriterion
    scoresMap                    map[string][]*dbmodel.Scores        // scorecardID -> scores
    conversationToCommentMap     map[string]map[string][]string
    momentsTaxonomyToDisplayName map[string]string
    hasProcessTemplate           bool
}
```

**Key insight:** `scoresMap` is keyed by scorecard resource ID -> list of scores. Each score has a `Comment` field and a `CriterionIdentifier` field.

---

## 3. How to Query Appeals of Scorecards

### Appeal Data Model

**File:** `go-servers/shared/scoring/appeal_workflow.go`

An appeal workflow consists of 4 scorecard types linked via `reference_scorecard_id`:

```go
type AppealWorkflow struct {
    Original        *dbmodel.Scorecards  // scorecard_type = NULL (the original graded scorecard)
    OriginalReplica *dbmodel.Scorecards  // scorecard_type = APPEAL_ORIGINAL_REPLICA
    AppealRequest   *dbmodel.Scorecards  // scorecard_type = APPEAL_REQUEST
    AppealResolve   *dbmodel.Scorecards  // scorecard_type = APPEAL_RESOLVE
}
```

### Query Function

**`NewGroupedAppealWorkflowsFromScorecardIDs()`** (line 74):
- Takes a list of scorecard IDs (can be any type — original, request, resolve, etc.)
- Uses a recursive CTE to traverse the `reference_scorecard_id` chain in both directions
- Returns `GroupedAppealWorkflows` — a two-level map: `original_scorecard_id -> original_replica_id -> AppealWorkflow`

### Appeal Approval Reason

**Proto definition** (`cresta-proto/cresta/v1/coaching/scorecard.proto`, line 177-179):
```proto
// Comment left for the criterion.
// The purpose of the comment depends on the type of its associated scorecard.
// It could be used to appeal reason or resolve reason
optional string comment = 5;
```

The `Score.comment` field is **overloaded** — its meaning depends on the parent scorecard type:
- On a regular scorecard: the grader's comment
- On an appeal request scorecard: the appeal reason
- On an appeal resolve scorecard: the **approval/rejection reason**

### How to Fetch Appeal Approval Reasons

To get approval reasons for a batch of scorecards:

1. Collect original scorecard IDs from the export query
2. Call `scoring.NewGroupedAppealWorkflowsFromScorecardIDs(db, profileName, scorecardIDs)`
3. For each `AppealWorkflow` where `AppealResolve != nil`:
   - Get the resolve scorecard ID
4. Call `scoring.FindExistingScores(db, profileName, resolveScorecardIDs, false)` to get scores for resolve scorecards
5. Build map: `originalScorecardID + criterionIdentifier -> resolveScore.Comment.String`

### Existing Usage Reference

**File:** `go-servers/insights-server/internal/analyticsimpl/retrieve_appeal_stats.go`
- Lines 673-675: Builds separate score maps for each appeal phase
- Shows the pattern of fetching scores for appeal resolve scorecards to get approval reasons

---

## Implementation Plan (Updated)

### Approach: Simple 3-JOIN Forward Traversal

Instead of using `NewGroupedAppealWorkflowsFromScorecardIDs` (which uses expensive dual recursive CTEs for bidirectional traversal), we use a dedicated `findAppealResolveScorecardIDs` function with 3 INNER JOINs:

```
Original → Replica (type=1) → Request (type=2) → Resolve (type=3)
```

This mirrors the existing `FindOriginalScorecardForAppealResolve()` pattern in `action_submit_scorecard.go:293-326` but in the reverse direction.

### Changes to `action_export_scorecards.go`

1. **New field in `linkedScorecardData`:**
   ```go
   appealResolveCommentMap map[string]string // key: scorecardID + "_" + criterionIdentifier → resolve comment
   ```

2. **New function `findAppealResolveScorecardIDs()`:**
   - 3 INNER JOINs following the `reference_scorecard_id` chain forward
   - Filters by `scorecard_type` at each join and `submitted_at IS NOT NULL` on resolve
   - Returns `map[string]string` — `originalScorecardID → resolveScorecardID` (latest resolve if multiple rounds)

3. **In `extractLinkedScorecardsDataForExport()`:**
   - Call `findAppealResolveScorecardIDs()` with original scorecard IDs
   - Call `FindExistingScores()` for the resolve scorecard IDs
   - Build `appealResolveCommentMap` from resolve scores' comments

4. **In `convertScorecardsToCSVBytes()`:**
   - Check `appealResolveCommentMap` before using original comment
   - Appeal resolve comment fully replaces original comment (conversation comments still appended)

5. **Updated `mergeLinkedScorecardData()` and `newLinkedScorecardData()`** to include the new field

### Design Decisions

- **Non-fatal error handling:** If appeal queries fail, the export proceeds without appeal comments (logged as error)
- **Latest resolve wins:** If multiple appeal rounds exist, the most recently created submitted resolve is used
- **Column header unchanged:** The "comment" column stays the same, only the content changes
