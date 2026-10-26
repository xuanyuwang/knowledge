# resetAutoQADetectedFields History

## Timeline

### April 15, 2025: Originally Added (CONVI-4491)
**Commit:** `1d7bfe7569` by Kurt Choi  
**Purpose:** "Reset behavior score if done/if not done mapping if score value is updated in template builder"

**What was added:**
```tsx
const resetAutoQADetectedFields = useCallback(() => {
  autoQADetectedField.onChange(null);
  autoQANotDetectedField.onChange(null);
}, [autoQADetectedField, autoQANotDetectedField]);
```

**Where it was used:**
```tsx
// In LEGACY MODE only (settings.options[index].value)
<NumberInput
  {...field}
  onChange={(value) => {
    field.onChange(value);
    resetAutoQADetectedFields();  // ← Reset when value changes
  }}
/>
```

**Why it was needed:**
- In **legacy mode**, `option.value` is both:
  1. The score value (what gets summed up)
  2. The autoQA reference (what `auto_qa.detected`/`not_detected` store)
- When user changes `option.value` from 5 → 10:
  - AutoQA still references value=5 (now invalid)
  - Must clear autoQA to prevent stale references

---

### April 9, 2026: Removed in Feature Flag Cleanup (CONVI-6471)
**Commit:** `c4e5df3298` by Kurt Choi  
**Purpose:** "Clean up enableDuplicateScoreForCriteria feature flag"

**What was removed:**
```diff
- const resetAutoQADetectedFields = useCallback(() => {
-   autoQADetectedField.onChange(null);
-   autoQANotDetectedField.onChange(null);
- }, [autoQADetectedField, autoQANotDetectedField]);
```

**Why it was removed:**
- Legacy mode was completely removed (assumed decoupled everywhere)
- In **decoupled mode**, `option.value` is just an ID (never changes)
- AutoQA stores **indices** (position in array), not score values
- Changing `score.score` doesn't change indices → no need to reset

**Decoupled mode (no onChange):**
```tsx
<NumberInput
  {...field}  // ← No onChange handler needed
  allowDecimal={false}
  min={0}
  // ...
/>
```

---

### My PR: Accidentally Re-added, Then Removed

**Commit 4 (April 12, 2026):** "Make N/A score onChange consistent with normal scores"
- ❌ **Mistake:** Added `resetAutoQADetectedFields` back, thinking it was needed
- Added it to N/A score onChange handler

**Commit 7 (April 12, 2026):** "Remove enableDuplicateScoreForCriteria feature flag"
- ✅ **Fixed:** Removed `resetAutoQADetectedFields` entirely
- Brought branch in sync with main

---

## Summary

### Before This PR

**In Legacy Mode** (before April 2026 cleanup):
- `option.value` is the score
- AutoQA stores `option.value` as reference
- Changing `option.value` → must reset autoQA (stale reference)
- **Solution:** `onChange` handler calls `resetAutoQADetectedFields()`

**In Decoupled Mode** (always):
- `option.value` is just an ID (unchanging)
- `score.score` is the actual score
- AutoQA stores **indices** (not scores)
- Changing `score.score` → autoQA still valid (index unchanged)
- **Solution:** No onChange handler needed

### After April 2026 Cleanup

- Only decoupled mode exists
- No `resetAutoQADetectedFields` needed
- No onChange handler for score inputs

### How AutoQA Fields Were Updated

**Answer:** They were NOT updated when scores changed in decoupled mode, because they don't need to be!

- AutoQA mappings (`auto_qa.detected`, `auto_qa.not_detected`) store **indices**
- Changing a score value doesn't invalidate these indices
- Only **deleting an option** requires remapping (handled by `handleRemoveOption`)

The only time autoQA needs to be reset is when:
1. **Deleting an option** → indices shift (handled in `handleRemoveOption`)
2. **Legacy mode: changing option.value** → reference becomes invalid (removed in April 2026)

In decoupled mode, changing scores is a no-op for autoQA! 🎉
