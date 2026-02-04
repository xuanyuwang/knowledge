# Duplicate Scorecard Template Across Use Cases - Investigation

**Linear Ticket:** [CONVI-6116](https://linear.app/cresta/issue/CONVI-6116)
**Status:** In Progress
**Assignee:** Xuanyu Wang

## Summary

Add support for "duplicate to another use case" for Performance Config (Scorecard) templates, similar to the existing functionality for Opera Rules/Policies.

---

## 1. Current Duplicate Behavior

### 1.1 How Scorecard Duplication Works Today

**There is NO explicit "Duplicate Template" API endpoint.** Duplication is implemented client-side using the standard `CreateScorecardTemplate` API.

**Current Flow:**
1. User clicks 3-dot menu on a template row
2. Selects "Duplicate this template"
3. Frontend navigates to template builder with query params: `?copyFrom=scorecardTemplates/{id}@{revision}`
4. Template builder fetches source template via `GetScorecardTemplate`
5. Frontend transforms the template:
   - Clears `name`, `resourceId`, `revision` (auto-generated on save)
   - Clears `qaTaskConfig`
   - Appends " Copy" to title
   - Regenerates all criterion identifiers with new UUIDs
6. User edits and saves as new template
7. `CreateScorecardTemplate` API creates the new template

**Key Files:**
- Frontend: `/director/packages/director-app/src/features/admin/coaching/scorecard-templates/ScorecardTemplateThreeDotMenu.tsx`
- Frontend: `/director/packages/director-app/src/features/admin/coaching/template-builder/ScorecardTemplateBuilder.tsx`
- Backend: `/go-servers/apiserver/internal/coaching/action_create_scorecard_template.go`

### 1.2 Policy/Block Duplication (Existing Reference Implementation)

Policies and Blocks have a **dedicated duplicate API** that already supports cross-use-case duplication via a modal. This is **existing production code** we can reference.

**Backend Implementation:**
- `/go-servers/apiserver/internal/policy/action_duplicate_policy.go` - Server-side duplication logic
- `/go-servers/apiserver/internal/moment/action_duplicate_moment_test.go` - Moment duplication tests

**API Endpoints:**
- `DuplicatePolicy` RPC - Takes `{ name, usecases[] }`, returns created policies
- `DuplicateMoment` RPC - Same pattern for blocks/moments

**Server-side Logic (from `action_duplicate_policy.go`):**
- Validates source policy has exactly one use case
- For each target use case:
  - Builds new policy with `copyPrefix + originalDisplayName`
  - Updates moment fields based on use case compatibility
  - Handles GenAI intents specially (creates as draft if cross-use-case)
  - Clears use-case-specific identifiers (name, taxonomy, timestamps)
- Creates policy or upserts as draft depending on GenAI content

**Frontend UI Flow:**
1. User clicks action menu -> "Duplicate to another use case"
2. Modal opens with use case selector (ListSelect)
3. Shows channel compatibility warnings (email vs non-email)
4. Shows GenAI intent warnings (duplicated as draft)
5. User selects target use case and clicks "Create"

**Key Frontend Files:**
- Modal: `/director/packages/director-app/src/features/coach-builder/rules-overview/DuplicatePolicyModal.tsx`
- Hook: `/director/packages/director-app/src/features/coach-builder/backend-apis/useDuplicatePolicy.ts`

---

## 2. Data Model - Scorecard Template

### 2.1 Database Schema

**Table:** `director.scorecard_templates`

| Column | Type | Sensitivity | Notes |
|--------|------|-------------|-------|
| customer | VARCHAR | N/A | Primary key part |
| profile | VARCHAR | N/A | Primary key part |
| resource_id | VARCHAR | N/A | Auto-generated UUID |
| revision | VARCHAR | N/A | Auto-generated (8-char UUID suffix) |
| title | VARCHAR | LOW | Can copy with " Copy" suffix |
| template | JSONB | LOW | Scorecard structure - safe to copy |
| type | INT4 | LOW | CONVERSATION(1) or PROCESS(2) |
| **audience** | JSONB | **HIGH** | Users/teams/groups - use-case specific |
| **usecase_ids** | _VARCHAR | **HIGH** | Explicit use case scope |
| **qa_task_config** | JSONB | **HIGH** | Contains use-case specific configs |
| **autoqa_triggers** | _VARCHAR | **MEDIUM** | Resource names may not exist in target |
| permissions | JSONB | LOW | Role-based, usually safe to copy |
| qa_score_config | JSONB | LOW | Usually safe to copy |
| creator_user_id | VARCHAR | N/A | Auto-set to current user |
| created_at | TIMESTAMPTZ | N/A | Auto-set |
| status | INT2 | LOW | ACTIVE(1), INACTIVE(2), ARCHIVED(3) |

### 2.2 Proto Definition

```protobuf
message ScorecardTemplate {
  string name = 1;                                    // Auto-generated
  string title = 2;                                   // REQUIRED
  google.protobuf.Struct template = 3;               // REQUIRED - scorecard structure
  Audience audience = 4;                             // SENSITIVE
  ResolvedTemplateAudience resolved_audience = 5;    // OUTPUT_ONLY
  google.protobuf.Timestamp create_time = 6;         // OUTPUT_ONLY
  repeated string usecase_names = 9;                 // SENSITIVE
  cresta.v1.qa.QATaskConfig qa_task_config = 10;     // SENSITIVE
  Permissions permissions = 12;                       // LOW sensitivity
  ScorecardTemplateType type = 13;
  ScorecardTemplateStatus status = 14;
  cresta.v1.qa.QAScoreConfig qa_score_config = 15;   // LOW sensitivity
}
```

---

## 3. Sensitive Fields Analysis

### 3.1 HIGH Sensitivity - Must Reset/Clear

#### `audience` (users, teams, groups)
**Problem:** Contains specific user IDs, team IDs, group IDs that may not exist or be appropriate in the target use case.

**Structure:**
```json
{
  "users": ["customers/{customer}/users/{user_id}"],
  "teams": ["customers/{customer}/teams/{team_id}"],
  "groups": ["customers/{customer}/groups/{group_id}"]
}
```

**Action:**
- **CLEAR** audience when duplicating to another use case
- User must explicitly configure audience in the new use case

#### `usecase_names` (usecase_ids in DB)
**Problem:** Explicitly ties template to specific use cases.

**Action:**
- **REPLACE** with target use case name(s)
- This is the core of the feature - template should be scoped to new use case

#### `qa_task_config`
**Problem:** Contains use-case specific settings.

**Structure:**
```protobuf
message QATaskConfig {
  TaskCompleteConfig task_complete_config = 1;      // Contains user/group references
  EvaluationPeriodConfig evaluation_period_config = 2;  // Safe to copy
  QAAnalystQuotaConfig qa_analyst_quota_config = 3;     // Contains user IDs
  ConversationSuggestionConfig conversation_suggestion_config = 4;  // May have use-case specific filters
}
```

**Sensitive sub-fields:**
- `task_complete_config.audience` - user/team/group references
- `task_complete_config.overridden_agents` - specific user/group overrides
- `qa_analyst_quota_config.qa_analyst_quotas` - specific QA analyst user IDs

**Action:**
- **CLEAR** entire `qa_task_config` when duplicating across use cases
- User must reconfigure QA task settings in new use case
- Same approach as current same-use-case duplication (already clears this)

### 3.2 MEDIUM Sensitivity - Validate or Clear

#### `autoqa_triggers`
**Problem:** Contains resource names that may not exist in target use case.

**Example:** `["customers/{customer}/profiles/{profile}/usecases/{usecase}/triggers/{trigger_id}"]`

**Action:**
- **CLEAR** when duplicating to another use case
- Triggers are use-case specific and won't exist in target

#### `template.items[].auto_qa.triggers` (Outcomes)
**Problem:** Outcomes within the template structure reference external Moment objects via `auto_qa.triggers[0].resource_name`. These Moments are use-case specific metadata fields.

**Structure (in criterion):**
```typescript
{
  identifier: string,
  auto_qa: {
    triggers: [{
      resource_name: "customers/{customer}/profiles/{profile}/usecases/{usecase}/moments/{moment_id}",
      type: string
    }]
  },
  // ... other criterion fields
}
```

**Action:**
- **CLEAR** outcome references (`auto_qa.triggers`) when duplicating across use cases
- Outcome criteria become "unconfigured" - user must re-select metadata fields in target use case
- Alternative: Remove outcome criteria entirely and let user re-add them

### 3.3 LOW Sensitivity - Safe to Copy

| Field | Action |
|-------|--------|
| `title` | **COPY** with " Copy" suffix |
| `template` | **COPY** - regenerate criterion UUIDs |
| `type` | **COPY** - CONVERSATION or PROCESS |
| `permissions` | **COPY** - role-based, not user-specific |
| `qa_score_config` | **COPY** - scoring rules usually apply across use cases |
| `status` | **SET** to ACTIVE for new template |

---

## 4. Comparison: Scorecard vs Policy Duplication

| Aspect | Policy Duplicate | Scorecard (Current) | Scorecard (Proposed) |
|--------|------------------|---------------------|----------------------|
| API | Dedicated `DuplicatePolicy` RPC | `CreateScorecardTemplate` | Reuse `CreateScorecardTemplate` |
| Cross-UC Support | Yes | No | Yes |
| UI | Modal with UC selector | No modal, query params | Modal with UC selector |
| Server-side logic | Yes | No (client-side) | No (client-side) |
| Validation | Channel compatibility | None | Show warnings |

---

## 5. Implementation Plan

### Option B: Frontend-Only Approach (Selected for MVP)

**Pros:**
- Simpler, no backend changes required
- Faster to implement
- Reuses existing `CreateScorecardTemplate` API

**Cons:**
- Logic lives in frontend
- Less consistent with Policy/Block pattern (they use server-side)

**Changes Required:**

1. **Frontend** (`director`):
   - New component: `DuplicateScorecardTemplateModal.tsx`
     - Two-step modal flow (see UI Flow below)
   - Update `ScorecardTemplateThreeDotMenu.tsx`:
     - Add "Duplicate to another use case" menu item
     - Hidden for archived templates (same as existing duplicate)
   - Update `ScorecardTemplateBuilder.tsx`:
     - Accept `targetUsecase` query param
     - When present, clear sensitive fields and set `usecase_names` to target

2. **UI Flow (Two-Step Modal):**

   **Step 1 - Warning Confirmation:**
   ```
   ┌─────────────────────────────────────────────────────┐
   │ ⓘ Copy template to another use case           [X]  │
   ├─────────────────────────────────────────────────────┤
   │                                                     │
   │ When duplicating a performance template to another  │
   │ use case, any automated criteria integrating with   │
   │ Opera will be unlinked and require manual scoring.  │
   │ Scorecard access will default to all agents.        │
   │                                                     │
   │                    [Understood & Continue]          │
   └─────────────────────────────────────────────────────┘
   ```

   **Step 2 - Use Case Selector** (after clicking "Understood & Continue"):
   ```
   ┌─────────────────────────────────────────────────────┐
   │ Select use case                               [X]   │
   ├─────────────────────────────────────────────────────┤
   │ Select a use case:                                  │
   │ ┌─────────────────────────────────────────────────┐ │
   │ │ [Icon] Use Case Name                      ▼     │ │
   │ └─────────────────────────────────────────────────┘ │
   │                                                     │
   │                      [Cancel]  [Create]             │
   └─────────────────────────────────────────────────────┘
   ```

   **Notes:**
   - "Automated criteria integrating with Opera" = Auto-QA triggers (linked to moments/blocks)
   - "Scorecard access will default to all agents" = Audience is reset
   - Reuse `ListSelect` component from Policy modal
   - Filter use cases: same profile, exclude current UC, exclude CARE_EFFICIENCY

### Option A: Backend API Approach (Future Enhancement)

**Pros:**
- Consistent with Policy/Block duplication pattern
- Server-side validation and transformation
- Single source of truth for duplication logic
- Better audit trail

**Changes Required:**

1. **Proto** (`cresta-proto`):
   - Add `DuplicateScorecardTemplate` RPC to `coaching_service.proto`
   - Request: `{ source_template_name, target_usecase_names[], options }`
   - Response: `{ created_template }`

2. **Backend** (`go-servers/apiserver/internal/coaching`):
   - New file: `action_duplicate_scorecard_template.go`
   - Logic:
     - Fetch source template
     - Clear sensitive fields (audience, qa_task_config, autoqa_triggers)
     - Clear outcome references in template structure
     - Set new usecase_names to target
     - Generate new resource_id/revision
     - Append " Copy" to title
     - Regenerate criterion identifiers
     - Create new template

3. **Frontend** (`director`):
   - New hook: `useDuplicateScorecardTemplate.ts`
   - Update modal to call new API instead of navigating to builder

---

## 6. Decisions Made

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Implementation approach | Frontend-only (Option B) | Faster, simpler for MVP |
| Profile restriction | Same profile only | Consistent with Policy duplication |
| Type conversion | No | Keep same type (CONVERSATION/PROCESS) |
| Multi-UC duplication | No (single UC) | MVP simplicity |
| Archived templates | Not supported | Same as existing same-UC duplication |
| Modal flow | Two-step (warning → selector) | UX design + reuse Policy pattern |

---

## 7. Open Questions

1. **How should we handle outcomes?**
   - Option A: Clear outcome `auto_qa.triggers` (outcome becomes unconfigured)
   - Option B: Remove outcome criteria entirely
   - Recommend: Option A (preserve structure, user reconfigures)

---

## 8. References

### Key Files

**Backend (Reference - Policy Duplication):**
- `/go-servers/apiserver/internal/policy/action_duplicate_policy.go` - **Existing cross-UC duplication logic**
- `/go-servers/apiserver/internal/coaching/action_create_scorecard_template.go`
- `/go-servers/apiserver/internal/coaching/transformers.go`
- `/go-servers/apiserver/sql-schema/gen/model/scorecard_templates.go`

**Frontend:**
- `/director/packages/director-app/src/features/admin/coaching/scorecard-templates/ScorecardTemplates.tsx`
- `/director/packages/director-app/src/features/admin/coaching/scorecard-templates/ScorecardTemplateThreeDotMenu.tsx`
- `/director/packages/director-app/src/features/admin/coaching/template-builder/ScorecardTemplateBuilder.tsx`
- `/director/packages/director-app/src/features/admin/coaching/template-builder/TemplateBuilderOutcomes.tsx` - Outcomes UI
- `/director/packages/director-app/src/features/coach-builder/rules-overview/DuplicatePolicyModal.tsx` - **Reference modal**
- `/director/packages/director-app/src/features/coach-builder/backend-apis/useDuplicatePolicy.ts` - **Reference hook**

**Proto:**
- `/cresta-proto/cresta/v1/coaching/coaching_service.proto`
- `/cresta-proto/cresta/v1/coaching/scorecard_template.proto`
- `/cresta-proto/cresta/v1/qa/qa.proto`

### UX Design Reference
- Modal mockup: `/Users/xuanyu.wang/Downloads/image.png` (from Linear ticket)

### Related PRs
- (To be added after implementation)
