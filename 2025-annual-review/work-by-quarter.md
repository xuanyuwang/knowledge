# Work Timeline by Quarter
## Period: February 2025 - January 2026

---

## Q1 2025 (Feb - Apr)

### Major Focus: Group Calibration Foundation
This quarter was heavily focused on building the foundation for the Group Calibration feature.

#### Key Deliverables:
1. **Database Schema Design**
   - Created director.tasks table schema (sql-schema#1676, #1681, #1723)
   - Added task-scorecard relationships
   - Supported usecase conversion for director tasks

2. **Proto Definitions**
   - CONVI-4275: New scorecard types for group calibration (cresta-proto#5807)
   - CONVI-4299: CRUD APIs for group calibration tasks (cresta-proto#5823)
   - CONVI-4366: TaskType field for task API (cresta-proto#5942)
   - CONVI-4430: Director task names filter (cresta-proto#5995)
   - CONVI-4467: New filters for ListDirectorTask (cresta-proto#6021)

3. **Backend APIs**
   - CONVI-4318: CreateDirectorTask implementation (go-servers#19529)
   - CONVI-4319: ListDirectorTasks implementation (go-servers#19740)
   - CONVI-4320: UpdateDirectorTask implementation (go-servers#19681)
   - CONVI-4275: Create answer key support (go-servers#19333)
   - CONVI-4276: Update answer key support (go-servers#19464)

4. **Infrastructure & Cleanup**
   - CONVI-4380: Removed legacy scorecard submission code (6 PRs: go-servers#19634-19651)
   - CONVI-4322: Enabled ENABLE_MEASURE_ACTIVE_DAYS_WITH_AA_CONV
   - CONVI-4373: Fixed auth DB connection in cron-label-conversations
   - CONVI-4511: Removed auth DB usage from cron-label-conversations

5. **Bug Fixes**
   - CONVI-4132: Added ACL check for managed users (go-servers#19248)
   - CONVI-3490: Enabled virtual group as template audience (config#127495, director#10716)
   - CONVI-4412: Strip session notes (go-servers#19756)

#### PRs This Quarter: ~50

---

## Q2 2025 (May - Jul)

### Major Focus: Group Calibration Completion & Analytics

#### Key Deliverables:
1. **Group Calibration Stats & Notifications**
   - CONVI-4387: RetrieveGroupCalibrationStats proto (cresta-proto#6106)
   - CONVI-4559: Parse filter and groupby for stats (go-servers#20327, #20378)
   - CONVI-4596: Count assigned & pending scorecards (go-servers#20546)
   - CONVI-4598: Add consistency score to scorecards (go-servers#20669, #20672, #20677)
   - CONVI-4600: Notification system for GC (go-servers#20783, #20836, cresta-proto#6250)

2. **Group Calibration Response Flow**
   - CONVI-4431: Create/update/submit GC response (go-servers#20037)
   - CONVI-4506: Migrate consistency score calculation to BE (go-servers#20157)
   - CONVI-4727: Exclude unfinished tasks from stats (go-servers#20900)
   - CONVI-4728: Task home stats investigation (go-servers#20904)

3. **Multi-Select Criteria**
   - CONVI-4812: Enable multi-select for criteria (go-servers#21210)
   - CONVI-4813: Drop unique constraint on scores index (go-servers#21220)
   - CONVI-4814: Update Scorecard APIs for multi-select (go-servers#21309)

4. **Bug Fixes**
   - CONVI-4761: Fixed GC notification format (go-servers#20960)
   - CONVI-4764: Fixed GC notification navigation (go-servers#21027, #21029)
   - CONVI-4765: Return negative consistency score when no valid response (go-servers#20968)
   - CONVI-4809: Fixed coaching plans list error (go-servers#21469)
   - CONVI-4868: Cleaned user filter for GC tab (director#12257)
   - CONVI-4879: Filter scorecards by task assignees (go-servers#21394)
   - CONVI-4899: Added timezone to notification (go-servers#21410)

#### PRs This Quarter: ~45

---

## Q3 2025 (Aug - Oct)

### Major Focus: AOC Features & Data Infrastructure

#### Key Deliverables:
1. **AOC (Agent on Call) Features**
   - CONVI-5485: ManagerWhisper proto for Guidance vs DirectMessage (cresta-proto#7028, #7030)
   - CONVI-5486: Update CreateCollaboration for whisper types (go-servers#23345)
   - CONVI-5503: List action annotations on multiple conversations (go-servers#23365, cresta-proto#7043)
   - CONVI-5530: Distinguish virtual agent in CreateCollaboration (go-servers#23401)
   - CONVI-5538: Use GetUser rather than ListUsers (go-servers#23429)
   - CONVI-5604: Live assist action type proto (cresta-proto#7164)
   - CONVI-5607: Assign/unassign conversation to supervisor (go-servers#23697)
   - CONVI-5629: Support multiple conversations in ListMomentAnnotations (go-servers#23732, cresta-proto#7213)

2. **Analytics Improvements**
   - CONVI-5173: ListAgentOnly for Insights APIs (go-servers#23473, #24607, #24638)
   - CONVI-5507: Filter outdated targets with outdated criteria (go-servers#23393)
   - CONVI-5647: AHT (Average Handle Time) support (python-ai-services#9808, #9943, go-servers#23848)

3. **Feature Flag Cleanup**
   - CONVI-5355: Removed enableVirtualGroupAsTemplateAudience (director#13863, config#134520)
   - CONVI-5638: Removed enableEditAppealOption

4. **Customer Issues Resolved**
   - CONVI-5578: Holiday Inn leaderboard missing scorecards
   - CONVI-5609: SnapFinance missing evaluations
   - CONVI-5662: Spirit conversation volume undercounting
   - CONVI-5674: Manager unable to see Coaching Reports
   - CONVI-5685: Investigated async work order of coaching APIs

#### PRs This Quarter: ~40

---

## Q4 2025 (Nov - Jan 2026)

### Major Focus: Data Sync & Analytics API Refactoring

#### Key Deliverables:
1. **ClickHouse-PostgreSQL Data Sync**
   - CONVI-5565: Fix historic schema race condition (go-servers#24086, #24103, #23999)
   - CONVI-5757: Batch reindex conversations task (go-servers#24209, flux-deployments#240241)
   - CONVI-5672: Backfill Guitar Center CH <> Postgres
   - CONVI-5730: Backfill Cox business-voice
   - CONVI-5760: Backfill guitar centre and voice sandbox 2

2. **Analytics API Refactoring (Major Initiative)**
   - CONVI-6005: Refactor RetrieveConversationStats (go-servers#25094)
   - CONVI-6007: Refactor RetrieveHintStats (go-servers#25095)
   - CONVI-6008: Refactor RetrieveKnowledgeBaseStats (go-servers#25130)
   - CONVI-6009: Refactor RetrieveLiveAssistStats (go-servers#25155)
   - CONVI-6010: Refactor RetrieveQAScoreStats (go-servers#25143)
   - CONVI-6015-6020: Refactor 6 more stats APIs (go-servers#25125-25129, #25100)
   - CONVI-6069: Use FinalUsers instead of UsersFromGroups (go-servers#25218)
   - CONVI-5173: Complete ParseUserFilterForAnalytics rollout (go-servers#25316, #25324)

3. **Race Condition & Bug Fixes**
   - CONVI-6076: Fix UpdateScorecard overwriting SubmitScorecard (go-servers#25260)
   - CONVI-5838: Remove UpdateScorecard before SubmitScorecard (go-servers#25330, director#16251)
   - CONVI-5921: Fix coaching session count discrepancy (go-servers#25032)
   - CONVI-5968: Include all sessions in coaching efficiency stats (go-servers#25066, cresta-proto#7681)
   - CONVI-6087: Fix Overview widget double-counting sessions (director#16145)

4. **Feature Flag Cleanup**
   - CONVI-5846: Removed ENABLE_MEASURE_ACTIVE_DAYS_WITH_AA_CONV (go-servers#24546)
   - CONVI-5935: Removed GROUP_CALIBRATION_ASSIGNED_NOTIFICATION flag (go-servers#24927)

5. **Notifications**
   - CONVI-5866: Virtual agent raise hand notifications (go-servers#24861)
   - CONVI-5816: Fixed AOC/hand raise notification issue

6. **Customer Issues Resolved**
   - CONVI-5723/5724: Fixed "unknown" agents in Greenix/Propel
   - CONVI-5733: Fixed missing "Top sessions" in Greenix Coaching Report
   - CONVI-5743: Investigated zero agents in Team Leaderboard
   - CONVI-5748: Home Care Delivered scorecard data investigation
   - CONVI-5752: Spirit "No Data" issue
   - CONVI-5774: Holiday Inn manager leaderboard undercounts
   - CONVI-5805: QA score popover bug fix (director#15637)

#### PRs This Quarter: ~78

---

## Summary by Quarter

| Quarter | Focus Areas | PRs | Key Features |
|---------|-------------|-----|--------------|
| Q1 2025 | Group Calibration foundation | ~50 | Task APIs, DB schema, proto definitions |
| Q2 2025 | GC completion, multi-select | ~45 | Stats APIs, notifications, consistency score |
| Q3 2025 | AOC features, AHT | ~40 | Whisper types, conversation assignment |
| Q4 2025 | Data sync, API refactoring | ~78 | ParseUserFilterForAnalytics, CH fixes |

**Total PRs: ~213**

---

## Growth Trajectory

### Early Year (Q1-Q2):
- Focused on single large feature (Group Calibration)
- Heavy database and proto design work
- Building foundational APIs

### Mid Year (Q3):
- Expanded to cross-team features (AOC)
- Started addressing customer-reported issues
- Feature flag cleanup

### Late Year (Q4):
- Major technical initiative (Analytics refactoring)
- High volume of customer issue resolution
- Data infrastructure improvements
- Highest productivity period (78 PRs)

---

*Generated on: 2026-01-26*
