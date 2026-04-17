# TemplateBuilderFormConfigurationStep.tsx Review

**Created:** 2026-04-12  
**File:** `packages/director-app/src/features/admin/coaching/template-builder/steps/configuration/TemplateBuilderFormConfigurationStep.tsx`

---

## Purpose

Main orchestrator component for template configuration UI. Manages:
- Adding/deleting chapters, criteria, and outcomes
- Drag-and-drop reordering
- Configuration panel (right side) for selected item
- Default criterion copying (when creating new criterion, copy settings from previously configured one)
- Cross-use-case duplication (reset unavailable outcomes)

---

## Key Workflows

### Workflow 1: Adding New Performance Criterion

**Trigger:** User clicks "Add Criterion" button in Performance section

**Handler:** `handleAddCriterion` (lines 279-343)

**Steps:**

1. **Generate new identifier** (line 281)
   ```tsx
   const newCriterionIdentifier = uuidv7();
   ```

2. **Create default AutoQA settings** (lines 288-295)
   ```tsx
   const newAutoQA = {
     auto_qa: {
       triggers: [],
       detected: 1,        // ← Default: "detected" maps to option 1 (No)
       not_detected: 0,    // ← Default: "not_detected" maps to option 0 (Yes)
       not_applicable: null,
     } as ScorecardTemplateAutoQA,
   };
   ```
   
   **Why these defaults?**
   - `detected: 1` = "If behavior is done" → maps to "No" (index 1)
   - `not_detected: 0` = "If behavior is not done" → maps to "Yes" (index 0)
   - `not_applicable: null` = "If behavior is N/A" → not set
   
   **NOTE:** These seem backwards! Usually you'd expect "detected" → "Yes". But this is configurable by user.

3. **Create base criterion from DEFAULT_CRITERION** (lines 297-300)
   ```tsx
   const newCriterion = {
     ...cloneDeep(DEFAULT_CRITERION),
     identifier: newCriterionIdentifier,
   };
   ```

4. **Copy settings from default criterion if exists** (lines 302-320)
   
   **What is "default criterion"?**
   - When user configures a criterion, it becomes the "default"
   - Next time user adds criterion, settings are copied from this default
   - Allows users to quickly create multiple similar criteria
   
   **What gets copied:**
   - AutoQA mappings (detected/not_detected/not_applicable)
   - Criterion type (Dropdown, Button, etc.)
   - `showNA` setting
   - Options (Yes, No, Maybe, etc.)
   - Range (for numeric inputs)

   ```tsx
   if (defaultCriterion) {
     // Copy AutoQA mappings
     if ('auto_qa' in defaultCriterion && defaultCriterion.auto_qa) {
       newAutoQA.auto_qa.detected = defaultCriterion.auto_qa.detected;
       newAutoQA.auto_qa.not_detected = defaultCriterion.auto_qa.not_detected;
       newAutoQA.auto_qa.not_applicable = defaultCriterion.auto_qa.not_applicable ?? null;
     }

     // Copy criterion type
     newCriterion.type = defaultCriterion.type;

     // Copy settings
     if ('settings' in defaultCriterion && defaultCriterion.settings) {
       newCriterion.settings.showNA = defaultCriterion.settings.showNA ?? true; // ← Default showNA to true!
       
       if ('options' in defaultCriterion.settings && defaultCriterion.settings.options) {
         newCriterion.settings.options = defaultCriterion.settings.options;
       }
       if ('range' in defaultCriterion.settings && defaultCriterion.settings.range) {
         newCriterion.settings.range = defaultCriterion.settings.range;
       }
     }
   }
   ```

5. **Add to form** (lines 321-334)
   ```tsx
   form.setValue(
     itemsListFieldName,
     [
       ...itemsList,
       {
         ...newCriterion,
         ...newAutoQA,
         itemType: 'performance',
       },
     ],
     { shouldDirty: true }
   );
   ```

6. **Update parent map and focus** (lines 335-340)
   ```tsx
   setParentMap((prevMap) => {
     const cloneMap = cloneDeep(prevMap);
     cloneMap[newCriterionIdentifier] = chapterFieldName ? chapter.identifier : '';
     return cloneMap;
   });
   setConfiguringId(newCriterionIdentifier); // ← Focus on new criterion
   ```

**Result:**
- New criterion appears in template
- Configuration panel opens on right (focused on new criterion)
- Settings copied from previous criterion (if exists)
- AutoQA enabled with default mappings

---

### Workflow 2: Adding New Outcome Criterion

**Trigger:** User clicks "Add Outcome" button in Outcomes section

**Handler:** `handleAddOutcome` (lines 345-393)

**Similar to adding performance criterion, but:**
- No chapter support (outcomes are always top-level)
- Default type is `LabeledRadios`
- Default display name is "New Conversation Outcome"
- Item type is `'metadata'` instead of `'performance'`

**Steps are identical otherwise:** create AutoQA, copy from default, add to form

---

### Workflow 3: Setting Default Criterion

**Trigger:** User configures a criterion (called from TemplateBuilderCriterionConfiguration)

**Handler:** `handleSetDefaultCriterion` (lines 268-277)

```tsx
const handleSetDefaultCriterion = useCallback(
  (criterion?: ScorecardCriterionTemplate) => {
    // We don't want to set the default criterion to user because user criterion is not allowed to be added.
    if (!criterion || criterion.type === CriterionTypes.User) {
      return;
    }
    setDefaultCriterion(cloneDeep(criterion));
  },
  [setDefaultCriterion]
);
```

**What this does:**
- When user finishes configuring a criterion, it becomes the "template" for next criterion
- User criterion type is excluded (not allowed to be added)
- Deep clone to prevent mutations

**Use case:**
1. User configures first criterion: "Was greeting polite?" with Yes/No options
2. This becomes default criterion
3. User clicks "Add Criterion" again
4. New criterion automatically has Yes/No options (copied from default)
5. User only needs to change display name, not recreate options

---

### Workflow 4: Cross-Use-Case Duplication

**Trigger:** User duplicates template from Use Case A to Use Case B (targetUsecase prop is set)

**Effect:** Lines 455-475

**Problem:**
- Outcome criteria use metadata (e.g., "Resolution Type", "Escalation Reason")
- Different use cases have different metadata available
- Template from Use Case A might reference metadata that doesn't exist in Use Case B

**Solution:**
```tsx
useEffect(() => {
  if (!isCrossUseCaseDuplicateFlag || hasAppliedCrossUseCaseReset || !outcomeMetadataByName) {
    return;
  }

  const defaultDisplayName = t('template-builder.configuration.new-conversation-outcome', 'New Conversation Outcome');
  const itemsCopy = cloneDeep(templateItems);
  
  // Check each item, reset if metadata doesn't exist in target use case
  const hasChanges = itemsCopy
    .map((item) => resetUnavailableOutcome(item, outcomeMetadataByName, defaultDisplayName))
    .some(Boolean);

  if (hasChanges) {
    form.setValue('template.items', itemsCopy, { shouldDirty: true });
  }

  setHasAppliedCrossUseCaseReset(true);
}, [isCrossUseCaseDuplicateFlag, hasAppliedCrossUseCaseReset, outcomeMetadataByName, form, t, templateItems]);
```

**`resetUnavailableOutcome` function** (lines 157-194):
```tsx
function resetUnavailableOutcome(
  item: ScorecardChapterTemplate | ScorecardCriterionTemplate,
  availableMetadata: Record<string, unknown>,
  defaultDisplayName: string
): boolean {
  // Recursively check chapters
  if (isScorecardChapter(item)) {
    return item.items
      .map((child) => resetUnavailableOutcome(child, availableMetadata, defaultDisplayName))
      .some(Boolean);
  }

  // Skip non-scorable criteria
  if (!isScorableCriterion(item)) {
    return false;
  }

  // Check if this is an outcome criterion (has metadata trigger)
  const metadataTrigger = Array.isArray(item.auto_qa?.triggers)
    ? item.auto_qa.triggers.find((trigger) => trigger.type === 'metadata')
    : undefined;
  const isMetadataItem = (item as ScorecardCriterionTemplate & { itemType?: string }).itemType === 'metadata';

  if (!metadataTrigger && !isMetadataItem) {
    return false; // Not an outcome criterion
  }

  // Check if metadata exists in target use case
  const resourceName = metadataTrigger?.resource_name;
  const isAvailable = resourceName && availableMetadata[resourceName];

  if (isAvailable) {
    return false; // Metadata exists, no reset needed
  }

  // Reset to default outcome criterion
  Object.assign(item, {
    ...cloneDeep(DEFAULT_OUTCOME_CRITERION),
    identifier: item.identifier, // Keep same identifier
    displayName: defaultDisplayName,
  });
  return true; // Signal that item was reset
}
```

**Example:**
1. Template in Use Case A has outcome "Was issue escalated?" using metadata "escalation_reason"
2. User duplicates to Use Case B
3. Use Case B doesn't have "escalation_reason" metadata
4. Criterion is reset to blank outcome: "New Conversation Outcome" with no metadata selected
5. User must reconfigure it to use available metadata in Use Case B

---

### Workflow 5: Drag and Drop Reordering

**Handlers:**
- `handleDragStart` (lines 225-235) - Track what's being dragged
- `handleDragEnd` (lines 395-441) - Reorder within same container
- `handleDragOver` (lines 478-556) - Move between containers (chapter ↔ template)
- `handleDragCancel` (lines 558-567) - Cancel and restore original state

**Drag Start** (lines 225-235):
```tsx
const handleDragStart = useCallback(
  (event: DragStartEvent): void => {
    const formValues = form.getValues();
    unstable_batchedUpdates(() => {
      setDraggingId(String(event.active.id));
      setConfiguringId(String(event.active.id)); // Auto-focus dragged item
      setClonedStructure(formValues.template.items); // Backup for cancel
    });
  },
  [form]
);
```

**Drag End - Reorder within same container** (lines 395-441):
```tsx
const handleDragEnd = useCallback(
  (event: DragEndEvent): void => {
    const { active, over } = event;

    unstable_batchedUpdates(() => {
      setDraggingId(undefined);
      if (over && active.id !== over.id) {
        const containerId = parentMap[over.id]!;
        
        // Check if moved to different container (shouldn't happen in drag end)
        if (containerId !== parentMap[active.id]) {
          return; // Handled by handleDragOver
        }

        // Get container (chapter or template root)
        const prevFormValues = form.getValues();
        const containerPath = getPath(containerId, parentMap, prevFormValues.template);
        const container = containerPath === ''
          ? prevFormValues.template
          : (get(prevFormValues.template, containerPath) as TemplateBuilderFormChapter);

        // Find old and new positions
        const oldIndex = container.items.findIndex((item) => item.identifier === active.id);
        const newIndex = container.items.findIndex((item) => item.identifier === over.id);

        if (oldIndex !== -1 && newIndex !== -1) {
          const itemsPath = containerPath ? `${containerPath}.items` : 'items';
          const reorderedItems = arrayMove(container.items, oldIndex, newIndex);

          // Update form
          form.setValue(`template.${itemsPath}` as 'template.items', reorderedItems, {
            shouldDirty: true,
            shouldTouch: true,
          });

          // Recalculate parent map
          const updatedFormValues = form.getValues();
          const newParentMap = calculateParentMap({}, updatedFormValues.template);

          unstable_batchedUpdates(() => {
            setParentMap(newParentMap);
          });
        }
      }
    });
  },
  [parentMap, form, setParentMap]
);
```

**Drag Over - Move between containers** (lines 478-556):
- More complex: removes from old container, inserts into new container
- Calculates insertion position based on cursor position
- Updates parent map
- Uses `allowContainerSwitching` ref to prevent multiple rapid switches

**Drag Cancel** (lines 558-567):
```tsx
const handleDragCancel = useCallback((): void => {
  if (clonedStructure) {
    // Restore original state
    form.setValue('template.items', clonedStructure);
  }
  unstable_batchedUpdates(() => {
    setDraggingId(undefined);
    setClonedStructure(undefined);
  });
}, [clonedStructure, form]);
```

---

## Data Flow

### Parent Map

**What is it?**
- Record mapping `item.identifier → parent.identifier`
- Used for drag and drop to know where items live
- Recalculated after every drag operation

**Example:**
```tsx
{
  'criterion-1': '',           // Top-level criterion (parent is template root)
  'chapter-1': '',             // Top-level chapter
  'criterion-2': 'chapter-1',  // Criterion inside chapter-1
  'criterion-3': 'chapter-1',  // Another criterion inside chapter-1
}
```

**Why needed?**
- Drag and drop library (dnd-kit) provides item IDs, not paths
- Need to find item's location in template structure: `template.items[0].items[2]`
- `getPath()` function (lines 51-151) uses parent map to construct path

### Path Construction

**`getPath(itemId, parentMap, template)` function** (lines 51-151):

**Purpose:** Convert item identifier → form field path

**Example:**
```tsx
getPath('criterion-2', parentMap, template)
// Returns: "items.0.items.1"
// Meaning: template.items[0].items[1]
```

**Algorithm:**
1. Walk up parent chain to get ancestors: `['criterion-2', 'chapter-1', '']`
2. Walk down template structure matching identifiers
3. Build path segments: `items.0` → `items.0.items.1`

**Why so complex?**
- Comment on line 50: "could be a lot easier if I didn't take into account the future case of chapters in chapters"
- Supports nested chapters (not currently used, but future-proofed)
- Handles branch logic (conditional questions)

### Form Value Updates

**Pattern:**
```tsx
const formValues = form.getValues();           // Read current state
const newValue = /* calculate new state */;    // Transform
form.setValue('template.items', newValue, {    // Write back
  shouldDirty: true,
  shouldTouch: true,
});
```

**Why `unstable_batchedUpdates`?**
- Prevents multiple re-renders when updating state + parent map
- React batches updates into single render
- Important for performance during drag and drop

---

## Connection to Other Components

### Data flows TO this component:

1. **From parent (TemplateBuilderForm):**
   - `initiallyFocusedCriterionId` - which criterion to focus on mount
   - `setParentMap`, `parentMap` - drag and drop state
   - `onDeleteItem` - callback when item deleted
   - `isProcessTemplate` - hide outcomes section for process templates
   - `targetUsecase` - for cross-use-case duplication

2. **From form context:**
   - `template.items` - all chapters and criteria
   - `scoringSubject` - per-conversation vs per-message scoring

### Data flows FROM this component:

1. **To child components (left panel):**
   - `TemplateBuilderFormConfigurationStepOutcomesSection` - outcomes list
   - `TemplateBuilderFormConfigurationStepPerformanceSection` - performance criteria list
   - Both receive: `itemsField`, event handlers, `configuringId`

2. **To child components (right panel):**
   - `TemplateBuilderEmptyConfiguration` - "Select an item to configure"
   - `TemplateBuilderChapterConfiguration` - chapter config panel
   - `TemplateBuilderOutcomeConfiguration` - outcome config panel
   - `TemplateBuilderCriterionConfiguration` - criterion config panel (this is where our bug fixes are!)
   - Receives: `configuringItemFieldPath`, `outcomeMetadataByName`, `setDefaultCriterion`

### Key callback: `setDefaultCriterion`

**Called from:** `TemplateBuilderCriterionConfiguration` (passed as prop on line 667)

**Effect:** Next time user adds criterion, settings are copied from this one

**Flow:**
```
User configures criterion
  ↓
TemplateBuilderCriterionConfiguration calls setDefaultCriterion(currentCriterion)
  ↓
TemplateBuilderFormConfigurationStep stores in state
  ↓
User clicks "Add Criterion"
  ↓
handleAddCriterion copies settings from defaultCriterion
  ↓
New criterion has same type, options, AutoQA mappings
```

---

## Scenarios Related to N/A Score Feature

### Scenario 1: Creating First Criterion (No Default)

**Steps:**
1. User opens new template
2. Clicks "Add Criterion"
3. `defaultCriterion` is undefined

**Result:**
```tsx
// Lines 288-300
const newAutoQA = {
  auto_qa: {
    triggers: [],
    detected: 1,
    not_detected: 0,
    not_applicable: null,
  }
};

const newCriterion = {
  ...cloneDeep(DEFAULT_CRITERION),
  identifier: newCriterionIdentifier,
};

// DEFAULT_CRITERION (from consts.ts) has:
// - type: LabeledRadios
// - settings: { showNA: true, options: [], scores: [] }
```

**Q: Why is AutoQA enabled by default?**
- Line 328: `itemType: 'performance'`
- All performance criteria have AutoQA object
- AutoQA checkbox in UI reads `auto_qa.triggers.length > 0` to determine checked state
- New criterion has `triggers: []` (empty array)
- So AutoQA checkbox should be UNCHECKED initially

**Q: Wait, but user said AutoQA is checked by default?**
- Need to check `DEFAULT_CRITERION` in consts.ts
- Or check TemplateBuilderCriterionConfiguration to see how it determines AutoQA checkbox state

---

### Scenario 2: Creating Second Criterion (With Default)

**Steps:**
1. User configured first criterion: type=Dropdown, options=[Yes, No], showNA=false
2. User clicks "Add Criterion" again
3. `defaultCriterion` exists

**Result:**
```tsx
// Lines 303-320
newAutoQA.auto_qa.detected = defaultCriterion.auto_qa.detected;        // Copied
newAutoQA.auto_qa.not_detected = defaultCriterion.auto_qa.not_detected; // Copied
newAutoQA.auto_qa.not_applicable = defaultCriterion.auto_qa.not_applicable ?? null;

newCriterion.type = defaultCriterion.type; // Dropdown

newCriterion.settings.showNA = defaultCriterion.settings.showNA ?? true; // false
newCriterion.settings.options = defaultCriterion.settings.options;       // [Yes, No]
```

**Important:** Line 312 - `showNA ?? true`
- If default criterion has `showNA: false`, use false
- If default criterion has `showNA: undefined`, use true
- **This means showNA defaults to TRUE if not set!**

---

### Scenario 3: Cross-Use-Case Outcome Reset

**Setup:** Template from Use Case A duplicated to Use Case B

**Effect:**
- Outcome criteria with unavailable metadata are reset
- Reset to `DEFAULT_OUTCOME_CRITERION` (need to check what this is)
- User must reconfigure metadata selection

**Q: Does this affect N/A score feature?**
- No, outcomes use different UI (TemplateBuilderOutcomeConfiguration)
- N/A score is only for performance criteria

---

## Questions to Investigate

### Q1: What is DEFAULT_CRITERION?

Need to read `consts.ts` to see default settings.

**Related to:** Understanding AutoQA checkbox default state

---

### Q2: Why is AutoQA "checked initially" according to user?

User said: "when you create a new criterion (Dropdown/Button), 'Automated scoring' checked initially"

But code shows `triggers: []` (empty array) which should mean unchecked?

Need to check: How does TemplateBuilderCriterionConfiguration determine AutoQA checkbox state?

**Possible answers:**
- Checkbox reads `auto_qa !== undefined` (not `triggers.length > 0`)
- Or checkbox reads a different field
- Or DEFAULT_CRITERION has non-empty triggers array

---

### Q3: showNA defaults to true?

Line 312: `newCriterion.settings.showNA = defaultCriterion.settings.showNA ?? true;`

This only applies when copying from default criterion. What about first criterion?

Need to check DEFAULT_CRITERION.settings.showNA.

---

### Q4: How are scores array initialized?

Code only copies `options` and `range` from default criterion. No mention of `scores`.

**Where are scores initialized?**
- Probably in CriteriaLabeledOptions component (our bug fix location)
- Lines 313-318 only copy options, not scores
- This makes sense - scores are derived from options

---

## Simplification Opportunities

### 1. Extract Default AutoQA Creation

**Current:** Duplicated in `handleAddCriterion` and `handleAddOutcome`

Lines 288-295 and lines 351-357 are identical.

**Suggestion:**
```tsx
function createDefaultAutoQA(defaultCriterion?: ScorecardCriterionTemplate): { auto_qa: ScorecardTemplateAutoQA } {
  const newAutoQA = {
    auto_qa: {
      triggers: [],
      detected: 1,
      not_detected: 0,
      not_applicable: null,
    } as ScorecardTemplateAutoQA,
  };

  if (defaultCriterion?.auto_qa) {
    newAutoQA.auto_qa.detected = defaultCriterion.auto_qa.detected;
    newAutoQA.auto_qa.not_detected = defaultCriterion.auto_qa.not_detected;
    newAutoQA.auto_qa.not_applicable = defaultCriterion.auto_qa.not_applicable ?? null;
  }

  return newAutoQA;
}
```

**Benefit:** Single source of truth for default AutoQA settings

---

### 2. Extract Default Criterion Copying

**Current:** Duplicated logic in `handleAddCriterion` (lines 302-320) and `handleAddOutcome` (lines 369-377)

**Suggestion:**
```tsx
function applyDefaultCriterionSettings(
  newCriterion: ScorecardCriterionTemplate,
  defaultCriterion?: ScorecardCriterionTemplate
): void {
  if (!defaultCriterion) return;

  newCriterion.type = defaultCriterion.type;

  if ('settings' in defaultCriterion && defaultCriterion.settings) {
    newCriterion.settings.showNA = defaultCriterion.settings.showNA ?? true;
    
    if ('options' in defaultCriterion.settings && defaultCriterion.settings.options) {
      newCriterion.settings.options = defaultCriterion.settings.options;
    }
    if ('range' in defaultCriterion.settings && defaultCriterion.settings.range) {
      newCriterion.settings.range = defaultCriterion.settings.range;
    }
  }
}
```

**Benefit:** Consistent copying logic, easier to maintain

---

### 3. Simplify getPath Function

**Current:** 100 lines (lines 51-151) with nested loops and complex logic

**Issue:** Comment says "could be a lot easier if I didn't take into account the future case of chapters in chapters"

**Suggestion:** If nested chapters aren't needed yet, simplify to current use case:
- Template has items (chapters or criteria)
- Chapters have items (criteria only)
- Criteria have branches (conditional questions)
- That's it - no chapters in chapters

**Benefit:** Easier to understand, faster execution

**Risk:** Breaking change if nested chapters are added later

**Recommendation:** Leave as-is unless performance becomes issue. Future-proofing is valuable here.

---

## Summary

### What this component does:
- Manages template builder UI (left panel = list, right panel = config)
- Handles add/delete/reorder of chapters and criteria
- Copies settings from "default criterion" when creating new ones
- Resets unavailable outcomes during cross-use-case duplication

### How it relates to N/A score feature:
- Creates new criteria with default AutoQA settings (detected: 1, not_detected: 0, not_applicable: null)
- Copies showNA setting from default criterion (defaults to true if not set)
- Copies options array from default criterion (but NOT scores array)
- Passes `setDefaultCriterion` callback to TemplateBuilderCriterionConfiguration

### Key insights:
- **AutoQA defaults:** New criteria get AutoQA object with empty triggers array
- **showNA defaults:** Line 312 shows `showNA ?? true` (defaults to true)
- **Default criterion copying:** Allows users to quickly create similar criteria
- **Parent map:** Used to convert item IDs to form field paths for drag and drop

### Questions to answer next:
1. Check DEFAULT_CRITERION in consts.ts - what are the default settings?
2. Check TemplateBuilderCriterionConfiguration - how does AutoQA checkbox work?
3. Verify: Is AutoQA really "checked initially" as user said?
