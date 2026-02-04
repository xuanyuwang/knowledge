# Duplicate Performance Template to Another Use Case

**Linear Ticket:** [CONVI-6116](https://linear.app/cresta/issue/CONVI-6116)

## Overview

Add the ability to duplicate a Performance Config (Scorecard) template from one use case to another. This allows teams to quickly reuse template structures across use cases without manually recreating them.

---

## User Flow

### Step 1: Access the feature
- Navigate to **Admin > Performance Config**
- In the templates table, click the **3-dot menu** on a template row
- Select **"Duplicate to another use case"**

> Note: This option is hidden for archived templates.

### Step 2: Warning confirmation
A modal appears explaining what will and won't be copied:

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

### Step 3: Select target use case
After clicking "Understood & Continue", a use case selector appears:

```
┌─────────────────────────────────────────────────────┐
│ Select use case                               [X]   │
├─────────────────────────────────────────────────────┤
│ Select a use case:                                  │
│ ┌─────────────────────────────────────────────────┐ │
│ │ 🎤 Sales Voice                            ▼     │ │
│ └─────────────────────────────────────────────────┘ │
│                                                     │
│                      [Cancel]  [Create]             │
└─────────────────────────────────────────────────────┘
```

- Shows available use cases within the same profile
- Excludes the current use case
- Shows channel icon (Voice/Chat/Email) for each option

### Step 4: Edit and save
- User is taken to the template builder with the copied template
- Title shows " Copy" suffix
- User can edit and save as a new template

---

## What Gets Copied

| Item | Copied? | Notes |
|------|---------|-------|
| Template structure (criteria, chapters) | ✅ Yes | All criteria and chapters preserved |
| Template title | ✅ Yes | " Copy" suffix added |
| Template type | ✅ Yes | Conversation or Process |
| Permissions (who can edit/view/grade) | ✅ Yes | Role-based permissions preserved |
| Scoring configuration | ✅ Yes | Score values and ranges |

---

## What Does NOT Get Copied

| Item | Copied? | Why |
|------|---------|-----|
| Audience (target agents) | ❌ No | Defaults to all agents; user must reconfigure |
| QA task configuration | ❌ No | Task quotas and schedules are use-case specific |
| Auto-QA triggers (Opera integration) | ❌ No | Linked blocks/moments don't exist in target use case |
| Outcome metadata references | ❌ No | Metadata fields are use-case specific |

---

## Constraints

| Constraint | Detail |
|------------|--------|
| Same profile only | Can only duplicate to use cases within the same profile |
| Single use case | Can only duplicate to one use case at a time |
| No archived templates | Cannot duplicate archived templates |
| Same template type | Template type (Conversation/Process) is preserved |

---

## Similar Existing Feature

This feature mirrors the existing **"Duplicate to another use case"** functionality for Opera Rules, which is already in production. The UI pattern and user experience will be consistent.

---

## Open Questions

1. **Outcome criteria handling:** When a template has outcomes linked to metadata fields, should we:
   - Keep the outcome criteria but mark them as unconfigured (user re-selects metadata), or
   - Remove outcome criteria entirely (user re-adds them)

   **Recommendation:** Keep criteria structure, mark as unconfigured.
