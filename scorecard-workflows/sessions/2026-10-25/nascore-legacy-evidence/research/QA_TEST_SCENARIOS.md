# QA Test Scenarios - N/A Score Support

**Created:** 2026-04-12  
**Feature:** Enable N/A scoring for scorecard templates  
**Branch:** xuanyu/enable-na-score

---

## Scenarios Discovered

### 1. Legacy Template Migration (Old → New)

**Setup:** Open an old template created before April 2026 that has `showNA: true` in backend

**Expected Behavior:**
- ✅ When opening template, N/A option is automatically created
- ✅ N/A option appears at the end of options list: `{ label: 'N/A', value: 2, isNA: true }`
- ✅ N/A score is `{ value: 2, score: null }`
- ✅ "Allow N/A" checkbox is already checked
- ✅ N/A input shows placeholder "no score"

**What to Verify:**
- N/A option exists in options list
- "Allow N/A" checkbox is checked
- Template can be saved without errors
- No duplicate N/A options created on re-open

---

### 2. Creating New Criterion with AutoQA Enabled

**Steps:**
1. Create new scorecard template
2. Add new criterion (Dropdown/Button type)
3. "Automated scoring" is checked by default

**Expected Behavior:**
- ✅ Two options auto-created: "Yes" (value: 0), "No" (value: 1)
- ✅ Labels are empty ""
- ✅ Score inputs are empty (not showing 0 or 1)
- ✅ "Allow N/A" checkbox is unchecked
- ✅ No N/A option exists

**What to Verify:**
- Score inputs are visually empty (not displaying 0)
- Form can be saved only after filling in labels and scores
- Validation error "Value is required" shows when submitting without scores

---

### 3. Disabling AutoQA on New Criterion

**Steps:**
1. Create new criterion with AutoQA enabled
2. Uncheck "Automated scoring" checkbox

**Expected Behavior:**
- ✅ Score inputs remain empty (DO NOT auto-populate to 0, 1)
- ✅ AutoQA dropdown fields are removed
- ✅ Options and scores arrays remain unchanged

**What to Verify:**
- Score inputs stay empty after unchecking
- No auto-population of scores
- Can re-enable AutoQA without issues

---

### 4. Checking "Allow N/A" Checkbox

**Steps:**
1. Create new criterion or open existing one
2. Check "Allow N/A" checkbox

**Expected Behavior:**
- ✅ N/A option is appended: `{ label: 'N/A', value: nextValue, isNA: true }`
- ✅ N/A score is appended: `{ value: nextValue, score: null }`
- ✅ N/A appears at the end of options list
- ✅ N/A label input is disabled (can't edit "N/A")
- ✅ N/A score input shows placeholder "no score"
- ✅ No delete button for N/A row (hidden spacer)

**What to Verify:**
- N/A option appears at bottom
- Can't edit "N/A" label
- Can leave N/A score empty (validation passes)
- Can enter a score for N/A if desired

---

### 5. Unchecking "Allow N/A" Checkbox

**Steps:**
1. Have criterion with N/A option enabled
2. Uncheck "Allow N/A" checkbox

**Expected Behavior:**
- ✅ N/A option is removed from options array
- ✅ N/A score is removed from scores array
- ✅ Remaining options are renormalized: values become 0, 1, 2, ...
- ✅ Branch conditions are remapped (old indexes → new indexes)
- ✅ AutoQA mappings are remapped (detected/not_detected/not_applicable)

**What to Verify:**
- N/A row disappears
- Remaining options have sequential values (0, 1, 2, ...)
- Branch logic still works (conditions updated)
- AutoQA dropdowns still work (indexes updated)

---

### 6. Adding Normal Options

**Steps:**
1. Click "Add Option" button
2. Fill in label and score

**Expected Behavior:**
- ✅ New option inserted BEFORE N/A (if N/A exists)
- ✅ New option appended at END (if no N/A)
- ✅ New option gets next sequential value
- ✅ New score defaults to `score: 0` (not null, not undefined)
- ✅ Score input is empty but has value 0 underneath

**What to Verify:**
- N/A stays at the end
- New option appears before N/A
- Can add multiple options

---

### 7. Removing Normal Options

**Steps:**
1. Have multiple options (e.g., Yes, No, Maybe, N/A)
2. Click delete button on "Maybe"

**Expected Behavior:**
- ✅ "Maybe" option removed
- ✅ Remaining options renormalized: values become 0, 1, 2, ...
- ✅ Branch conditions remapped
- ✅ AutoQA mappings remapped
- ✅ If deleted option was mapped in AutoQA, mapping is cleared (set to null)

**What to Verify:**
- Options have sequential values after deletion
- AutoQA dropdowns still show correct labels
- Branch logic still works

---

### 8. N/A Option in AutoQA Dropdowns

**Steps:**
1. Enable AutoQA for criterion
2. Check "Allow N/A"
3. Look at AutoQA behavior mapping dropdowns

**Expected Behavior:**
- ✅ "If behavior is done" dropdown shows: "Yes (5)", "No (0)", "N/A (no score)"
- ✅ "If behavior is not done" dropdown shows: "Yes (5)", "No (0)", "N/A (no score)"
- ✅ "If behavior is N/A" dropdown shows: "Yes (5)", "No (0)", "N/A (no score)"
- ✅ N/A option shows "N/A (no score)" when `score = null`
- ✅ N/A option shows "N/A (3)" when user enters score 3

**What to Verify:**
- N/A label shows "(no score)" when score is null (NOT showing value like "N/A (2)")
- Dropdown labels update when scores change
- Can select N/A for any behavior outcome

---

### 9. Score Validation

**Setup:** Create criterion with normal options and N/A

**Expected Behavior:**
- ✅ Normal options (Yes, No): `required: 'Value is required'`
  - Cannot submit with empty score
  - `score: 0` is VALID (zero passes validation)
  - `score: null` or `undefined` FAILS validation
- ✅ N/A option: NO validation
  - Can submit with empty score (`score: null`)
  - Can submit with filled score (`score: 3`)

**What to Verify:**
- Error "Value is required" shows for empty normal scores
- No error for empty N/A score
- Zero (0) is accepted as valid score

---

### 10. AutoQA Dropdown Sync

**Steps:**
1. Enable AutoQA
2. Edit option label from "Yes" to "Correct"
3. Edit score from 5 to 10
4. Look at AutoQA dropdowns

**Expected Behavior:**
- ✅ Dropdown immediately updates to "Correct (10)"
- ✅ Sync is automatic via `useWatch` + `useMemo`
- ✅ No manual refresh needed

**What to Verify:**
- Dropdown labels update in real-time
- Scores in parentheses update when scores change
- Labels update when labels change

---

## Edge Cases

### Edge Case 1: N/A with Score

**Steps:**
1. Check "Allow N/A"
2. Enter score 5 for N/A

**Expected:** N/A option shows "N/A (5)" in AutoQA dropdowns

---

### Edge Case 2: Multiple Add/Remove Cycles

**Steps:**
1. Add option → Remove option → Add option → Remove option (repeat 5x)

**Expected:** Values stay sequential (0, 1, 2, ...) without gaps

---

### Edge Case 3: N/A Mapped to AutoQA Outcomes

**Steps:**
1. Enable AutoQA
2. Check "Allow N/A"
3. Map "If behavior is done" → "N/A"
4. Uncheck "Allow N/A"

**Expected:** AutoQA mapping is cleared (remapped to null/undefined)

---

### Edge Case 4: Template with showNA but No Feature Flag

**Setup:** Feature flag `enableNAScore` is OFF, template has `showNA: true`

**Expected:**
- Migration does NOT run
- No N/A option created
- "Allow N/A" checkbox may not appear (depends on feature flag)

---

## Test Data

### Sample Old Template (Pre-Migration)
```json
{
  "settings": {
    "options": [
      { "label": "Yes", "value": 0 },
      { "label": "No", "value": 1 }
    ],
    "scores": [
      { "value": 0, "score": 5 },
      { "value": 1, "score": 0 }
    ],
    "showNA": true
  }
}
```

**After Migration:**
```json
{
  "settings": {
    "options": [
      { "label": "Yes", "value": 0 },
      { "label": "No", "value": 1 },
      { "label": "N/A", "value": 2, "isNA": true }
    ],
    "scores": [
      { "value": 0, "score": 5 },
      { "value": 1, "score": 0 },
      { "value": 2, "score": null }
    ],
    "showNA": true
  }
}
```

---

## Known Issues (Fixed)

### ~~Issue 1: N/A shows wrong value in dropdowns~~
**Status:** ✅ FIXED (dd96747dc4)
- Before: "N/A (2)" when score is null
- After: "N/A (no score)" when score is null

### ~~Issue 2: Scores auto-populate when disabling AutoQA~~
**Status:** ✅ FIXED (99aaed9705)
- Before: Unchecking AutoQA fills scores with 0, 1
- After: Scores stay empty when unchecking AutoQA

---

## Regression Tests

### Regression 1: Existing Templates Without N/A
**Steps:** Open template that never had N/A enabled

**Expected:** No N/A option created, no migration runs

---

### Regression 2: AutoQA Toggle
**Steps:** Enable → Disable → Enable AutoQA multiple times

**Expected:** Scores remain unchanged, no data corruption

---

### Regression 3: Branch Conditions
**Steps:** 
1. Create branch: "If answer is Yes → skip to Question 5"
2. Remove "Yes" option

**Expected:** Branch condition is cleared or remapped correctly

---

## Performance Tests

### Perf 1: Many Options
**Steps:** Create criterion with 20 options

**Expected:** 
- Dropdowns populate without lag
- Add/remove operations stay fast
- Form saves without timeout

---

### Perf 2: Template with 50 Criteria
**Steps:** Open large template with 50 criteria, each with N/A enabled

**Expected:**
- Template loads without freeze
- Migration runs only once per criterion
- No duplicate N/A options created
