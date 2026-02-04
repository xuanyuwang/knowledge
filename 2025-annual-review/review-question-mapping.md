# Review Questions - Evidence Mapping
## How to Use This Document

This document maps your work evidence to each self-review question. Use this as reference material when writing your actual responses.

---

## 1. Key Accomplishments
*Summarize your key accomplishments over the past 12 months. Describe the impact on team, customers, or business.*

### Major Accomplishments to Highlight:

#### A. Group Calibration Feature (High Impact - New Product Capability)
- **What**: Built end-to-end backend for Group Calibration, enabling QA teams to create calibration exercises and measure evaluator consistency
- **Impact**:
  - New product capability for enterprise customers
  - Enables QA managers to identify and train evaluators who score inconsistently
  - Improved scoring accuracy across organizations
- **Evidence**: 30+ Linear tickets (CONVI-4274 through CONVI-4765), 50+ PRs

**My Contributions (Senior IC+):**

| Contribution | Senior IC Scope | Beyond Senior IC |
|--------------|-----------------|------------------|
| **Data Model Design**: Designed the `director.tasks` schema including task types, statuses, assignees, usecases, and scorecard relationships | Owned data model boundaries for entire feature | These schema decisions now constrain all future QM task features |
| **API Contract Design**: Defined proto contracts for `CreateDirectorTask`, `UpdateDirectorTask`, `ListDirectorTasks`, `RetrieveGroupCalibrationStats` across cresta-proto | Full ownership of API surface area | API contracts are consumed by frontend team; changes require cross-team coordination |
| **Algorithm Design**: Designed and implemented consistency score calculation algorithm (comparing evaluator responses against answer keys) | Complex business logic implementation | Algorithm became the standard for measuring evaluator accuracy company-wide |
| **End-to-End Technical Plan**: Created technical plan spanning 4 repos (sql-schema, cresta-proto, go-servers, director) over 2 quarters | Led multi-quarter technical initiative | Coordinated dependencies across backend, frontend, and infra teams |
| **Notification System**: Designed notification triggers for task lifecycle (assignment, due dates, responses) | Integrated with existing notification infra | Extended notification framework with new notification kinds |
| **Long-term Accountability**: Remained point-of-contact for all GC issues months after delivery (CONVI-5918, customer escalations) | Ongoing ownership | Absorbed complexity of maintaining feature in production |

---

#### B. Analytics API Refactoring (High Impact - Customer Satisfaction)
- **What**: Unified user filtering logic across 12+ analytics APIs via `ParseUserFilterForAnalytics`
- **Impact**:
  - Fixed long-standing "N/A" and "unknown" agent issues affecting Hilton, Greenix, Guitar Center, Mutual of Omaha
  - Improved data accuracy in Performance Insights dashboards
  - Reduced customer support escalations
- **Evidence**: CONVI-6005 through CONVI-6020, go-servers#25094-25155

**My Contributions (Senior IC+):**

| Contribution | Senior IC Scope | Beyond Senior IC |
|--------------|-----------------|------------------|
| **Identified Systemic Issue**: Traced customer complaints across multiple tickets to root cause in inconsistent user filtering logic | Deep debugging across systems | Recognized pattern that others missed; proposed unified solution |
| **Designed Shared Utility**: Created `ParseUserFilterForAnalytics` as reusable component for all Insights APIs | Abstraction design | Established pattern now used by all analytics endpoints |
| **Executed Large-Scale Refactoring**: Systematically updated 12 APIs with consistent approach over 2 weeks | High-volume execution (12 PRs in rapid succession) | Managed rollout with feature flags to minimize risk |
| **Cross-API Consistency**: Ensured `filterToAgentsOnly` parameter worked identically across all endpoints | API contract consistency | Created proto-level changes (cresta-proto#7515) affecting multiple consumers |
| **Customer Communication**: Worked directly with CS team on customer-specific rollouts (Hilton, Greenix) | Customer-facing technical support | Bridged gap between engineering and customer success |

---

#### C. Data Sync Infrastructure (High Impact - System Reliability)
- **What**: Fixed ClickHouse-PostgreSQL synchronization issues causing scorecard data discrepancies
- **Impact**:
  - Resolved data loss issues for multiple customers
  - Created BatchReindexConversations for systematic data backfill
  - Improved system reliability
- **Evidence**: CONVI-5565, CONVI-5757, CONVI-6076

**My Contributions (Senior IC+):**

| Contribution | Senior IC Scope | Beyond Senior IC |
|--------------|-----------------|------------------|
| **Root Cause Analysis**: Identified race condition between async ClickHouse writes and PostgreSQL transactions | Complex distributed systems debugging | Issue had been undetected for months; required deep understanding of both systems |
| **Architectural Decision**: Moved scorecard writes from async to synchronous execution path | Trade-off decision (latency vs consistency) | Changed system architecture pattern; set precedent for future data sync |
| **Version Column Strategy**: Implemented `updated_at` as ClickHouse version column for deduplication | Data infrastructure design | Solution now used for all scorecard-related CH tables |
| **Backfill Infrastructure**: Built `BatchReindexConversations` cron task for systematic data repair | Operability tooling | Created reusable infrastructure for future data issues |
| **Production Rollout**: Managed staged rollout across staging → prod environments via flux-deployments | Deployment orchestration | Coordinated with infra team on cron job scheduling |
| **Race Condition Fix (CONVI-6076)**: Discovered and fixed concurrent `UpdateScorecard`/`SubmitScorecard` race | Production incident response | Prevented data loss for active customers |

---

#### D. AOC Features (Medium Impact - New Capability)
- **What**: Implemented conversation assignment and supervisor collaboration features
- **Impact**: Enables real-time supervisor intervention during live calls
- **Evidence**: CONVI-5485, 5486, 5503, 5530, 5607

**My Contributions (Senior IC+):**

| Contribution | Senior IC Scope | Beyond Senior IC |
|--------------|-----------------|------------------|
| **Proto Design**: Designed `WhisperType` enum distinguishing Guidance vs DirectMessage (cresta-proto#7028) | API contract design | Proto changes consumed by multiple teams (backend, frontend, real-time services) |
| **Cross-Service Integration**: Updated `CreateCollaboration` API to integrate with real-time collaboration service | Service integration | Coordinated with AOC team on API contracts |
| **Action Annotation Design**: Created new action annotation type for live assist actions (cresta-proto#7164) | Data model extension | Extended core annotation framework used across products |
| **Virtual Agent Support**: Distinguished virtual agent in collaboration APIs (CONVI-5530) | Business logic for new entity type | Enabled new product capability (virtual agent monitoring) |
| **Multi-Conversation Support**: Extended `ListMomentAnnotations` to support batch queries (cresta-proto#7213) | Performance optimization | Reduced API calls for AOC dashboard |

---

### Summary: Work Beyond Typical Senior IC Scope

| Area | Evidence |
|------|----------|
| **Architectural Decisions** | Data models, API contracts, and sync strategies that constrain future features |
| **Cross-Team Influence** | Proto changes consumed by frontend, infra, and AOC teams |
| **Long-Term Accountability** | Remained owner of GC, coaching, and analytics issues months after initial delivery |
| **System-Wide Patterns** | `ParseUserFilterForAnalytics` became standard; sync strategy became precedent |
| **Production Reliability** | Identified and fixed issues that had gone undetected across the organization |

---

## 2. Strengths
*What are 2-3 key strengths? How have these shown up? How have you grown?*

### Suggested Strengths:

#### Strength 1: Technical Depth & API Design
- **Evidence**: Designed data models and APIs for Group Calibration from scratch
- **Growth**: Expanded from single-service ownership to cross-team API design (AOC collaboration APIs)
- **Example**: The consistency score calculation required understanding scoring algorithms, database design, and API contracts - delivered a complete solution

#### Strength 2: Customer-Focused Problem Solving
- **Evidence**: Rapidly resolved 15+ customer-reported issues across Hilton, Guitar Center, Spirit, etc.
- **Growth**: Developed systematic debugging approach for data discrepancy issues
- **Example**: CONVI-5173 (Hilton agent issues) required tracing through multiple systems to find root cause in user filtering logic

#### Strength 3: System Reliability & Data Integrity
- **Evidence**: Identified and fixed race conditions in scorecard submission, built data backfill infrastructure
- **Growth**: Expanded from feature development to infrastructure improvements
- **Example**: CONVI-6076 - Discovered and fixed a race condition where concurrent API calls caused data loss

---

## 3. Development Areas
*What are 2-3 areas of development? What will you focus on next year?*

### Suggested Areas (aligned with career-path-self-evaluation.md):

#### Area 1: Cross-Team Technical Influence
- **Current state**: Technical authority within CONVI team, but limited cross-boundary influence
- **Goal**: Become a cross-team service provider or own a business domain end-to-end
- **Action**: Propose and lead initiatives that span multiple teams (e.g., unified analytics framework)

#### Area 2: Strategic Product Thinking
- **Current state**: Provide technical consultation to PM, but don't reshape roadmap
- **Goal**: Validate features from technology perspective earlier in planning
- **Action**: Participate in roadmap discussions, propose technically-informed feature variations

#### Area 3: Mentorship & Knowledge Sharing
- **Current state**: Primary owner of coaching/QM APIs - creates continuity but not leverage
- **Goal**: Share knowledge to reduce bus factor, mentor others on system designs
- **Action**: Document architecture decisions, conduct knowledge sharing sessions

---

## 4. Operating Principles
*Which Cresta operating principle(s) most influenced your work?*

### Suggested Principles to Highlight:

#### "Move Fast" / "Bias for Action"
- **Evidence**:
  - 213 PRs in 12 months (~4 PRs/week)
  - Rapid customer issue resolution (often same-day fixes)
  - Q4 2025 alone: 78 PRs
- **Example**: CONVI-5921 - Customer reported coaching session discrepancy, delivered fix within 2 days

#### "Customer Obsession" / "Customer First"
- **Evidence**:
  - 15+ customer-specific issues resolved
  - Built features directly addressing customer needs (Group Calibration for enterprise QA)
- **Example**: Analytics refactoring initiative was driven by customer complaints about data accuracy

#### "Own It" / "Accountability"
- **Evidence**:
  - Long-term ownership of coaching service - accountable for issues months after feature delivery
  - Proactive feature flag cleanup and technical debt reduction
- **Example**: From career-path-self-evaluation: "Ever since the features have a question from customer success team or other team, they'll ping me"

---

## 5. Empowerment
*How can your manager support your success? What should they start, stop, or continue?*

### Suggested Points (aligned with career goals):

#### Start:
- **Provide opportunities to lead cross-team initiatives** - Currently limited to within-team technical authority
- **Include me in earlier product planning discussions** - To validate features from technology perspective before commitment

#### Continue:
- **Giving me ownership of complex features** - Group Calibration success demonstrates capability
- **Supporting rapid customer issue resolution** - Flexibility to address urgent issues has built customer trust

#### Stop:
- **N/A or consider**: Reducing context-switching between feature work and support issues
- Alternative framing: "Continue delegating design authority while helping me expand organizational influence"

---

## Quick Reference: Key Numbers

| Metric | Value |
|--------|-------|
| Total PRs | ~213 |
| PRs Merged | ~201 |
| Linear Tickets Completed | 70+ |
| Major Features Delivered | 4 |
| Customers Directly Impacted | 8+ |
| APIs Designed/Refactored | 15+ |
| Repositories Contributed To | 7 |

---

## Repositories Worked In

1. **go-servers** - Main backend monorepo (~120 PRs)
2. **cresta-proto** - API definitions (~20 PRs)
3. **director** - Frontend (~15 PRs)
4. **flux-deployments** - Infrastructure/deployments (~30 PRs)
5. **sql-schema** - Database schemas (~8 PRs)
6. **config** - Feature flags (~10 PRs)
7. **python-ai-services** - AI services (~3 PRs)
8. **historic-analytics** - Analytics pipeline (~2 PRs)

---

*Generated on: 2026-01-26*
