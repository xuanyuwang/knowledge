# Test Plan: Duplicate Scorecard Template to Another Use Case

**Linear Ticket:** [CONVI-6116](https://linear.app/cresta/issue/CONVI-6116)
**Feature:** Duplicate Performance Config template across use cases
**PR:** [#16470](https://github.com/cresta/director/pull/16470)

---

## Test Environment Setup

### Prerequisites
- Access to a customer profile with multiple use cases (e.g., walter-dev)
- At least 2 use cases: source UC and target UC
- Outcome metadata configured in:
  - Both use cases (shared outcome)
  - Only source use case (UC-specific outcome)

### Test Data Setup
Create test templates with various configurations:
1. Template with no outcome criteria
2. Template with outcome criteria using shared metadata
3. Template with outcome criteria using UC-specific metadata
4. Template with mixed outcome criteria (some shared, some UC-specific)

---

## Test Scenarios

### 1. Access and UI Flow

| ID | Scenario | Steps | Expected Result | Tested |
|----|----------|-------|-----------------|--------|
| 1.1 | Menu item visibility - Active template | Open 3-dot menu on an active template | "Duplicate to another use case" option is visible | √ |
| 1.2 | Menu item visibility - Inactive template | Open 3-dot menu on an inactive template | "Duplicate to another use case" option is visible | √ |
| 1.3 | Menu item visibility - Archived template | Open 3-dot menu on an archived template | "Duplicate to another use case" option is NOT visible | √ |
| 1.4 | Warning modal display | Click "Duplicate to another use case" | Warning modal appears with correct message about Opera unlinking and audience reset | √ |
| 1.5 | Use case selector display | Click "Understood & Continue" | Use case selector modal appears | √ |
| 1.6 | Use case filtering | Check use case dropdown options | Current use case is excluded; only same-profile UCs shown; CARE_EFFICIENCY UCs excluded | √ |
| 1.7 | Channel icons | Check use case dropdown | Correct channel icons shown (Voice/Chat/Email) | √ |
| 1.8 | Cancel flow | Click "Cancel" on selector modal | Modal closes, no navigation occurs | √ |
| 1.9 | Create disabled state | Open selector without selecting UC | "Create" button is disabled | √ |
| 1.10 | Create enabled state | Select a target use case | "Create" button becomes enabled | √ |

### 2. Template Duplication - Basic

| ID | Scenario | Steps | Expected Result | Tested |
|----|----------|-------|-----------------|--------|
| 2.1 | Navigation after create | Select UC and click "Create" | Navigates to template builder with copied template | √ |
| 2.2 | Title suffix | Check template title | Title shows " Copy" suffix | √ |
| 2.3 | Use case assignment | Check template use case | Template is assigned to target use case | √ |
| 2.4 | Audience reset | Check audience configuration | Audience is cleared (defaults to all agents) | √ |
| 2.5 | Template structure preserved | Check criteria and chapters | All criteria and chapters are preserved | √ |
| 2.6 | Permissions preserved | Check permissions tab | Role-based permissions are copied | √ |
| 2.7 | Scoring config preserved | Check scoring configuration | Score values and ranges are preserved | √ |

### 3. Template Types

| ID | Scenario | Steps | Expected Result | Tested |
|----|----------|-------|-----------------|--------|
| 3.1 | Conversation template | Duplicate a Conversation type template | Type remains Conversation in target | √ |
| 3.2 | Process template | Duplicate a Process type template | Type remains Process in target; process-specific criteria preserved | √ |
| 3.3 | Process template criteria | Check non-removable criteria in Process template | Process-specific criteria (e.g., greeting) are preserved | √ |

### 4. Outcome Criteria - Smart Reset

| ID | Scenario | Steps | Expected Result | Tested |
|----|----------|-------|-----------------|--------|
| 4.1 | Shared outcome - preserved | Duplicate template with outcome using metadata available in BOTH UCs | Outcome configuration is preserved (metadata field, display name intact) | ☐ |
| 4.2 | UC-specific outcome - reset | Duplicate template with outcome using metadata ONLY in source UC | Outcome is reset to "New Conversation Outcome" with empty configuration | ☐ |
| 4.3 | Mixed outcomes | Duplicate template with both shared and UC-specific outcomes | Shared outcomes preserved; UC-specific outcomes reset | ☐ |
| 4.4 | No outcomes | Duplicate template with no outcome criteria | Template duplicates normally | ☐ |
| 4.5 | Reset outcome validation | Try to save template with reset outcome without configuring | Validation error shows for "New Conversation Outcome" (not old metadata name) | ☐ |
| 4.6 | Reconfigure reset outcome | Select new metadata for reset outcome and save | Template saves successfully with new outcome configuration | ☐ |

### 5. Automated Criteria (Auto-QA)

| ID | Scenario | Steps | Expected Result | Tested |
|----|----------|-------|-----------------|--------|
| 5.1 | Auto-QA triggers cleared | Duplicate template with Auto-QA (Opera-linked) criteria | Auto-QA triggers are cleared; criteria becomes manual | ☐ |
| 5.2 | Manual criteria | Duplicate template with manual criteria | Manual criteria preserved as-is | ☐ |
| 5.3 | Mixed criteria | Duplicate template with both Auto-QA and manual | Auto-QA cleared; manual preserved | ☐ |

### 6. Edge Cases

| ID | Scenario | Steps | Expected Result | Tested |
|----|----------|-------|-----------------|--------|
| 6.1 | Empty template | Duplicate template with no criteria | Empty template structure is copied | ☐ |
| 6.2 | Nested chapters | Duplicate template with criteria inside chapters | Chapter structure and nested criteria preserved | ☐ |
| 6.3 | Branching criteria | Duplicate template with branching criteria | Branching structure preserved | ☐ |
| 6.4 | Large template | Duplicate template with many criteria (20+) | All criteria copied correctly | ☐ |
| 6.5 | Special characters in title | Duplicate template with special chars in title | Title copied correctly with " Copy" suffix | ☐ |
| 6.6 | Multiple duplications | Duplicate same template twice to same UC | Second copy shows " Copy Copy" or similar | ☐ |

### 7. Superset Mode (Cross-UC View)

| ID | Scenario | Steps | Expected Result | Tested |
|----|----------|-------|-----------------|--------|
| 7.1 | Duplicate from superset view | In superset mode, duplicate template to another UC | Works correctly; navigates to target UC context | ☐ |
| 7.2 | Use case options in superset | Check UC selector options in superset mode | Shows all eligible UCs except source template's UC | ☐ |

### 8. Save and Persistence

| ID | Scenario | Steps | Expected Result | Tested |
|----|----------|-------|-----------------|--------|
| 8.1 | Save duplicated template | Complete duplication flow and save | New template created in target UC | ☐ |
| 8.2 | Verify in target UC | Switch to target UC and check templates list | New template appears in list | ☐ |
| 8.3 | Original unchanged | Check original template in source UC | Original template is unchanged | ☐ |
| 8.4 | Backfill prompt | Save template with outcome criteria | Backfill prompt modal appears | ☐ |

---

## Regression Tests

| ID | Scenario | Steps | Expected Result | Tested |
|----|----------|-------|-----------------|--------|
| R.1 | Existing duplicate (same UC) | Use "Duplicate this template" option | Works as before (no changes to existing flow) | ☐ |
| R.2 | Template editing | Edit an existing template | Works normally | ☐ |
| R.3 | Template creation | Create new template from scratch | Works normally | ☐ |
| R.4 | Outcome configuration | Configure outcome in new template | Works normally | ☐ |

---

## Test Matrix

### Template Configurations to Test

| Config | Criteria Types | Outcome Types | Expected Behavior | Tested |
|--------|---------------|---------------|-------------------|--------|
| A | Manual only | None | All preserved | ☐ |
| B | Auto-QA only | None | Auto-QA cleared | ☐ |
| C | Manual + Auto-QA | None | Manual preserved, Auto-QA cleared | ☐ |
| D | Manual | Shared outcomes | All preserved | ☐ |
| E | Manual | UC-specific outcomes | Manual preserved, outcomes reset | ☐ |
| F | Manual | Mixed outcomes | Manual preserved, UC-specific reset | ☐ |
| G | Auto-QA | Shared outcomes | Auto-QA cleared, outcomes preserved | ☐ |
| H | Auto-QA | UC-specific outcomes | Auto-QA cleared, outcomes reset | ☐ |
| I | Mixed | Mixed | Manual preserved, Auto-QA cleared, shared outcomes preserved, UC-specific reset | ☐ |

### Use Case Combinations

| Source UC | Target UC | Notes | Tested |
|-----------|-----------|-------|--------|
| Chat | Chat | Same channel | ☐ |
| Chat | Voice | Different channel | ☐ |
| Chat | Email | Different channel | ☐ |
| Voice | Chat | Different channel | ☐ |

---

## Bug Verification

Verify these issues from code review are fixed:

| ID | Issue | Verification | Tested |
|----|-------|--------------|--------|
| B.1 | Old metadata name in validation error | Reset outcome shows "New Conversation Outcome" in error, not old name | ☐ |
| B.2 | TablerIcon import | No console errors about missing TablerIcon | ☐ |
| B.3 | Unused imports | No lint warnings about BLUE_COLORS or FeatherIcon | ☐ |

---

## Performance Considerations

| ID | Test | Acceptance Criteria | Tested |
|----|------|---------------------|--------|
| P.1 | Large template duplication | Completes within 3 seconds | ☐ |
| P.2 | Outcome metadata fetch | No visible delay in configuration step | ☐ |
| P.3 | Multiple outcomes check | Smart reset completes without UI freeze | ☐ |

---

## Sign-off Checklist

- [ ] All access/UI flow tests pass
- [ ] Basic duplication works correctly
- [ ] Both template types work
- [ ] Smart outcome reset works as expected
- [ ] Auto-QA triggers are cleared
- [ ] Edge cases handled
- [ ] Superset mode works
- [ ] Save and persistence verified
- [ ] Regression tests pass
- [ ] No console errors
- [ ] Performance acceptable

---

## Test Execution Summary

| Category | Total | Passed | Failed | Blocked | Not Run |
|----------|-------|--------|--------|---------|---------|
| 1. Access and UI Flow | 10 | | | | 10 |
| 2. Basic Duplication | 7 | | | | 7 |
| 3. Template Types | 3 | | | | 3 |
| 4. Outcome Smart Reset | 6 | | | | 6 |
| 5. Auto-QA Criteria | 3 | | | | 3 |
| 6. Edge Cases | 6 | | | | 6 |
| 7. Superset Mode | 2 | | | | 2 |
| 8. Save/Persistence | 4 | | | | 4 |
| Regression | 4 | | | | 4 |
| Test Matrix - Config | 9 | | | | 9 |
| Test Matrix - UC Combos | 4 | | | | 4 |
| Bug Verification | 3 | | | | 3 |
| Performance | 3 | | | | 3 |
| **Total** | **64** | | | | **64** |
