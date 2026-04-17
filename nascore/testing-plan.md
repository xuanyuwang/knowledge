# N/A Score Feature — Testing Plan

**Created:** 2026-04-16
**Feature flag:** `enableNAScore`

## Prerequisites

- Enable `enableNAScore` feature flag in GrowthBook (or local override)
- Have access to Template Builder (Admin > QA > Scorecard Templates)
- Test in a use case that supports Auto QA (Behavior DND and # of Occurrences)

---

## 1. Labeled Options (Behavior DND criterion)

### 1.1 Allow N/A ON, no score (legacy-identical)

1. Create or edit a criterion with type "Labeled" (Behavior DND)
2. Add two options: "Yes" (score 1), "No" (score 0)
3. Check "Allow N/A"
4. **Do NOT enter a score** in the N/A row
5. Save the template
6. **Verify:** The saved JSON has:
   - `settings.options`: `[{label:"Yes",value:0}, {label:"No",value:1}]` — no N/A entry
   - `settings.scores`: `[{value:0,score:1}, {value:1,score:0}]` — no N/A entry
   - `settings.showNA`: `true`
   - `auto_qa.not_applicable`: `null`
   - This should be identical to legacy "Allow N/A" behavior

### 1.2 Allow N/A ON, score assigned

1. Same setup as 1.1
2. Type `5` in the N/A score input
3. **Verify immediately:** N/A option appears in arrays
4. Save the template
5. **Verify:** The saved JSON has:
   - `settings.options`: `[..., {label:"N/A",value:2,isNA:true}]`
   - `settings.scores`: `[..., {value:2,score:5}]`
   - `auto_qa.not_applicable`: `2`

### 1.3 Clear N/A score (revert to no-score)

1. After 1.2, clear the N/A score input (delete the number)
2. Click away (blur the input)
3. **Verify:** N/A option is removed from arrays, `not_applicable` resets to `null`
4. Save and verify JSON matches 1.1

### 1.4 Change N/A score

1. After 1.2 (score = 5), change the score to `10`
2. **Verify:** Score updates in-place (no duplicate N/A entries)
3. Save and verify `scores` has `score: 10` for the N/A entry

### 1.5 Uncheck Allow N/A (with scored N/A)

1. After 1.2 (N/A has score 5)
2. Uncheck "Allow N/A"
3. **Verify:** N/A row disappears, N/A is removed from options/scores arrays, `not_applicable` is `null`

### 1.6 Uncheck Allow N/A (without scored N/A)

1. After 1.1 (Allow N/A checked, no score)
2. Uncheck "Allow N/A"
3. **Verify:** N/A row disappears, no array changes needed, `showNA` is `false`

### 1.7 Add option after N/A is scored

1. After 1.2 (N/A has score, is in arrays)
2. Click "Add Option"
3. **Verify:** New option is inserted BEFORE N/A in the arrays (N/A stays last)
4. N/A index is renormalized correctly

### 1.8 Delete an option when N/A is scored

1. After 1.2, delete the "Yes" option
2. **Verify:** Options and scores are renormalized, N/A remains in arrays with correct value
3. Auto QA detected/not_detected/not_applicable values are remapped correctly

---

## 2. Numeric Bins (# of Occurrences criterion)

### 2.1 Allow N/A ON, no score

1. Create or edit a criterion with type "# of Occurrences"
2. Add a range bin (e.g., 0-5)
3. Check "Allow N/A"
4. Leave N/A score empty
5. Save
6. **Verify:** No N/A in `settings.options`/`settings.scores`, `not_applicable`: `null`

### 2.2 Allow N/A ON, score assigned

1. Same as 2.1, type `3` in N/A score
2. **Verify:** N/A appears in `settings.options`/`settings.scores` (appended at end)
3. `auto_qa.not_applicable` set to the N/A value

### 2.3 Clear N/A score

1. After 2.2, clear the score and blur
2. **Verify:** N/A removed from arrays, `not_applicable`: `null`

### 2.4 Uncheck Allow N/A (with scored N/A)

1. After 2.2, uncheck "Allow N/A"
2. **Verify:** N/A removed from arrays, `not_applicable`: `null`, score input resets

---

## 3. Auto QA Dropdowns (Behavior DND)

### 3.1 Synthetic "N/A (no score)" option

1. Create a Behavior DND criterion, check "Allow N/A", leave N/A score empty
2. Open the "If behavior is not applicable" dropdown
3. **Verify:** "N/A (no score)" appears as an option
4. Select it
5. **Verify:** `not_applicable` is set to `null` (the synthetic value maps to null)

### 3.2 Scored N/A in dropdown

1. Same criterion, enter score `5` for N/A
2. Open the "If behavior is not applicable" dropdown
3. **Verify:** N/A appears with its actual score label (e.g., "N/A (5)") instead of the synthetic option
4. Select it
5. **Verify:** `not_applicable` is set to the N/A option's value (numeric)

### 3.3 Done/Not Done dropdowns exclude synthetic N/A

1. With "Allow N/A" checked and no score
2. Open "If behavior is done" and "If behavior is not done" dropdowns
3. **Verify:** "N/A (no score)" does NOT appear in these dropdowns (only in the not_applicable dropdown)
4. Wait — per current implementation, all three dropdowns share `behaviorScoreSelectionOptions`. Confirm whether "N/A (no score)" appears in done/not_done dropdowns too. (It does in current code — the reviewer flagged this and it was marked "won't fix" since users may intentionally map outcomes to N/A)

---

## 4. Cross-field Validation (Inline)

### 4.1 detected == not_detected

1. Set "If behavior is done" and "If behavior is not done" to the same option
2. **Verify:** Error message appears on both dropdowns: "This value must be different from the other outcome values."
3. **Verify:** Cannot proceed to next step

### 4.2 detected == not_applicable

1. Set "If behavior is done" to option A
2. Set "If behavior is not applicable" to same option A
3. **Verify:** Error message on both conflicting fields
4. Change one to a different value
5. **Verify:** Error clears on both

### 4.3 not_detected == not_applicable

1. Set "If behavior is not done" and "If behavior is not applicable" to the same option
2. **Verify:** Error message appears

### 4.4 not_applicable is null (no conflict possible)

1. Leave "If behavior is not applicable" unset (or set to "N/A (no score)")
2. Set detected and not_detected to different values
3. **Verify:** No validation errors (null is inherently different from any number)

### 4.5 Cross-field re-validation

1. Set detected = A, not_detected = A (error shows)
2. Change not_detected to B
3. **Verify:** Error clears on BOTH detected and not_detected (not just the one changed)

---

## 5. Save-time Validation

### 5.1 not_applicable conflicts with detected

1. Set detected = 0, not_applicable = 0
2. Try to save
3. **Verify:** Validation error: "The not applicable value must be different from the done and not done values."

### 5.2 not_applicable conflicts with not_detected

1. Set not_detected = 1, not_applicable = 1
2. Try to save
3. **Verify:** Same validation error

### 5.3 All different (happy path)

1. Set detected = 0, not_detected = 1, not_applicable = 2
2. Save
3. **Verify:** Saves successfully

### 5.4 not_applicable is null (always valid)

1. Set detected = 0, not_detected = 1, not_applicable = null
2. Save
3. **Verify:** Saves successfully regardless of other values

---

## 6. Legacy Template Compatibility

### 6.1 Old template with showNA=true (no scores array)

1. Open an existing template created before the enableNAScore flag
2. It should have `showNA: true` but no `scores` array
3. **Verify:** Legacy migration in `useOnMount` initializes the scores array from option values
4. N/A row appears but score is empty (no N/A in arrays)

### 6.2 Old template with options but no N/A

1. Open a template that has options like `[{label:"Yes",value:1}, {label:"No",value:0}]`
2. **Verify:** Migration normalizes to sequential values `[{value:0}, {value:1}]` and creates scores using original values

### 6.3 Template with enableNAScore OFF

1. Disable the `enableNAScore` feature flag
2. Open a template with "Allow N/A" checked
3. **Verify:** No N/A score row appears, behavior is identical to pre-feature behavior

---

## 7. Default Criterion Copy

1. Set up a default criterion with "Allow N/A" and a scored N/A option
2. Add a new criterion (which copies from default)
3. **Verify:** N/A option is filtered out during copy (new criterion starts without N/A in arrays)
4. `showNA` setting IS copied from default

---

## 8. Edge Cases

### 8.1 Rapid score changes

1. Type a score, immediately change it, blur
2. **Verify:** No duplicate N/A entries, final state is correct

### 8.2 Multiple criteria with N/A

1. Create multiple criteria, each with "Allow N/A" and different scores
2. **Verify:** Each criterion's N/A is independent, no cross-contamination

### 8.3 Score = 0

1. Enter `0` as the N/A score
2. **Verify:** N/A is added to arrays with `score: 0` (zero is a valid score, not treated as empty)

### 8.4 Negative score

1. Enter `-1` as the N/A score
2. **Verify:** N/A is added with `score: -1` (negative is allowed per `clampBehavior="none"`)

---

## 9. Regression Checks

- [ ] Create a template without N/A — works identically to before
- [ ] Edit an existing template without touching N/A — no changes to saved JSON
- [ ] Scoring UI (CriterionInputDisplay) still shows N/A button when `showNA=true`
- [ ] Auto QA scoring works correctly for templates with and without N/A scores
- [ ] Template validation passes for all pre-existing templates (no false positives from new checks)
