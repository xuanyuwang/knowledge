# 2025 Annual Review - Work Summary
## Period: February 2025 - January 2026

---

## Executive Summary

Over the past year, I have been a key contributor to the Conversation Intelligence (CONVI) team at Cresta, focusing on backend development for coaching, quality management (QM), and analytics services. My work has spanned major feature development, critical bug fixes, data infrastructure improvements, and cross-team API designs.

---

## Major Projects & Accomplishments

### 1. Group Calibration Feature (Q1-Q2 2025)
**Impact: New product capability for QA teams**

Led the backend implementation of the Group Calibration feature, enabling QA managers to create calibration tasks, assign evaluators, and measure scoring consistency across teams.

**Key contributions:**
- Designed and implemented CRUD APIs for Director Tasks (`CreateDirectorTask`, `UpdateDirectorTask`, `ListDirectorTasks`)
- Built consistency score calculation engine for comparing evaluator responses against answer keys
- Implemented notification system for task assignments, due dates, and responses
- Created `RetrieveGroupCalibrationStats` API for analytics dashboards
- Supported multi-select criteria functionality in scorecards

**PRs (selection):**
- [CONVI-4318] Create QM task - go-servers#19529
- [CONVI-4506] Migrate consistency score calculation to BE - go-servers#20157
- [CONVI-4600] Group calibration notifications - go-servers#20783, go-servers#20836
- [CONVI-4387] RetrieveGroupCalibrationStats proto - cresta-proto#6106
- [CONVI-4274] DB schema for tasks and scorecards - sql-schema#1676

**Linear tickets:** CONVI-4274 through CONVI-4765 (30+ tickets)

---

### 2. Analytics API Refactoring (Q4 2025 - Q1 2026)
**Impact: Fixed critical data accuracy issues affecting multiple customers**

Led a comprehensive refactoring effort to fix user filtering logic across all Insights APIs, resolving long-standing issues where agents appeared as "N/A" or "unknown" in performance dashboards.

**Key contributions:**
- Created `ParseUserFilterForAnalytics` - a unified utility for consistent user filtering
- Refactored 12+ analytics APIs to use the new filtering approach
- Fixed metadata enrichment to use `FinalUsers` instead of `UsersFromGroups`
- Implemented `filterToAgentsOnly` parameter across all analytics endpoints

**APIs updated:**
- `RetrieveConversationStats`
- `RetrieveHintStats`
- `RetrieveSuggestionStats`
- `RetrieveSummarizationStats`
- `RetrieveSmartComposeStats`
- `RetrieveNoteTakingStats`
- `RetrieveGuidedWorkflowStats`
- `RetrieveKnowledgeBaseStats`
- `RetrieveKnowledgeAssistStats`
- `RetrieveLiveAssistStats`
- `RetrieveQAScoreStats`

**PRs:** go-servers#25094, #25095, #25100, #25125-#25130, #25143, #25155, #24638, #24607

**Customers impacted:** Hilton (HGV), Greenix, Mutual of Omaha, Guitar Center

---

### 3. ClickHouse-PostgreSQL Data Sync (Q4 2025)
**Impact: Resolved data consistency issues and improved system reliability**

Addressed critical data synchronization issues between PostgreSQL (source of truth) and ClickHouse (analytics database) that caused scorecard data discrepancies.

**Key contributions:**
- Implemented synchronous write path for scorecard data to eliminate race conditions
- Added `updated_at` as version column for ClickHouse deduplication
- Created `BatchReindexConversations` cron task for data backfill
- Fixed race condition where `UpdateScorecard` overwrote `SubmitScorecard` changes

**PRs:**
- [CONVI-5565] Fix historic schema race condition - go-servers#24086, #24103
- [CONVI-6076] Fix UpdateScorecard race condition - go-servers#25260
- [CONVI-5757] Batch reindex conversations task - go-servers#24209
- [CONVI-5565] Use PostgreSQL updated_at as CH version - go-servers#23999

**Customers affected by fixes:** Guitar Center, Cox Business, Spirit Airlines, Holiday Inn

---

### 4. AOC (Agent on Call) Features (Q3-Q4 2025)
**Impact: Enabled real-time supervisor collaboration features**

Contributed to AOC features allowing supervisors to monitor and assist agents during live conversations.

**Key contributions:**
- Implemented conversation assignment/unassignment APIs
- Added support for different whisper types (Guidance vs DirectMessage)
- Created action annotation type for live assist actions
- Enabled notifications for virtual agent hand raises

**PRs:**
- [CONVI-5607] Assign/unassign conversation - go-servers#23697
- [CONVI-5486] ManagerWhisper types - go-servers#23345
- [CONVI-5604] Live assist action type - cresta-proto#7164
- [CONVI-5866] Virtual agent raise hand notifications - go-servers#24861

---

### 5. Coaching Service Improvements (Throughout Year)
**Impact: Improved coaching experience and fixed customer-reported issues**

Continuously improved the coaching service with bug fixes and enhancements.

**Key contributions:**
- Fixed coaching session count discrepancies between reports
- Implemented proper soft-delete handling for coaching sessions
- Added timezone info to notification text
- Fixed scorecard filtering by task assignees
- Improved coaching efficiency stats calculations

**PRs:**
- [CONVI-5921] Fix coaching session count discrepancy - go-servers#25032
- [CONVI-4879] Filter scorecards by task assignees - go-servers#21394
- [CONVI-4899] Add timezone to notification - go-servers#21410
- [CONVI-5968] Include all sessions in coaching report - go-servers#25066

---

### 6. Feature Flag Cleanup & Technical Debt (Throughout Year)
**Impact: Improved codebase maintainability**

Proactively cleaned up feature flags and technical debt.

**Flags removed:**
- `enableVirtualGroupAsTemplateAudience` (CONVI-5355)
- `ENABLE_MEASURE_ACTIVE_DAYS_WITH_AA_CONV` (CONVI-5846)
- `ENABLE_LIST_USERS_FOR_ANALYTICS` (CONVI-5846)
- `COACHING_SERVICE_ENABLE_GROUP_CALIBRATION_ASSIGNED_NOTIFICATION` (CONVI-5935)

---

## Quantitative Summary

### GitHub Activity (Feb 2025 - Jan 2026)
| Repository | PRs Created | PRs Merged |
|------------|-------------|------------|
| go-servers | ~120 | ~115 |
| cresta-proto | ~20 | ~18 |
| director (frontend) | ~15 | ~15 |
| flux-deployments | ~30 | ~30 |
| sql-schema | ~8 | ~7 |
| config | ~10 | ~8 |
| Other repos | ~10 | ~8 |
| **Total** | **~213** | **~201** |

### Linear Tickets
- Tickets worked on: 90+
- Done/Released: 70+
- In Progress: ~5
- Major features delivered: 4 (Group Calibration, Analytics Refactoring, Data Sync, AOC)

---

## Technical Skills Demonstrated

1. **API Design**: Designed and implemented RESTful/gRPC APIs for coaching, QM, and analytics services
2. **Database Design**: Created and modified PostgreSQL schemas, ClickHouse tables
3. **System Integration**: Built data synchronization between PostgreSQL and ClickHouse
4. **Performance Optimization**: Optimized analytics queries and data retrieval patterns
5. **Cross-functional Collaboration**: Worked with frontend (director), infra (flux-deployments), and proto teams
6. **Customer Support**: Quickly resolved production issues affecting major customers

---

## Customer Impact

Work directly improved product experience for:
- **Hilton (HGV)** - Fixed agent leaderboard discrepancies, coaching report accuracy
- **Guitar Center** - Resolved scorecard data sync issues
- **Greenix** - Fixed "unknown" agent issues in Performance Insights
- **Spirit Airlines** - Resolved conversation volume undercounting
- **Holiday Inn** - Fixed manager leaderboard counts
- **Mutual of Omaha** - Enabled appeals feature, fixed agent display issues
- **Brinks** - Fixed AOC notification issues
- **SnapFinance** - Resolved missing evaluations in Performance Insights

---

## Areas of Technical Ownership

1. **Coaching Service APIs** - scorecard submission, coaching plans, coaching sessions
2. **Director Task APIs** - QM task management for group calibration
3. **Analytics/Insights APIs** - performance metrics, leaderboards, stats retrieval
4. **Data Sync Pipeline** - ClickHouse indexing, conversation labeling crons

---

## Alignment with Career Development Goals

Based on my self-evaluation:
- **Decision Ownership**: I design technical plans and data models for my team's service
- **Blast Radius**: My API designs constrain future feature development; I own the data representation
- **PM Interaction**: Provide technical consultation on feature feasibility and prioritization
- **Time Allocation**: 40% coding, 40% design docs, 10% discussions, 10% debugging

**Path Forward (from self-evaluation):**
1. Become the canonical owner of a business domain (e.g., coaching service)
2. Work toward cross-team service provider role
3. Collaborate more with PM to validate features from technology perspective

---

*Generated on: 2026-01-26*
*Data sources: GitHub PRs, Linear tickets*
