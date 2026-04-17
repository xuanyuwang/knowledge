# Meaningful Diff from Line 167 (Component Rendering)

## Summary
Most changes are **indentation only** due to wrapping the map callback in `{ }` to add the N/A skip logic.

---

## MEANINGFUL Changes (lines 167-220)

### 1. **Skip N/A option in normal options loop** (lines 167-169)
```diff
  {optionsField.fields.map((field, index) => {
+   // Skip isNA option — it's rendered separately as the N/A row
+   if (watchedOptionsField?.[index]?.isNA) return null;
    return (
      <Flex key={field.id} align="center" gap="xs">
```

**Why:** N/A option is now rendered separately (see #2), so skip it in the main loop.

---

### 2. **Add N/A score row** (lines 221-244)
```diff
  </Flex>
+ {showNAScoreRow && (
+   <Flex align="center" gap="xs">
+     <TextInput className={styles.optionsRow__label} value="N/A" disabled />
+     <Flex gap="xs" className={styles.optionsRow__value}>
+       {showNumericOptions && (
+         <Controller
+           name={`${fieldName}.settings.scores.${isNAIndex}.score`}
+           render={({ field }) => (
+             <NumberInput
+               {...field}
+               allowDecimal={false}
+               clampBehavior="none"
+               placeholder={t('criteria-labeled-options.na-score-placeholder', 'no score')}
+               hideControls
+             />
+           )}
+         />
+       )}
+       {/* Spacer to align with delete button column */}
+       <ActionIcon variant="outline" radius="md" size="lg" style={{ visibility: 'hidden' }}>
+         <IconTrash size={16} />
+       </ActionIcon>
+     </Flex>
+   </Flex>
+ )}
  <div className={styles.footer}>
```

**Why:** NEW - renders N/A score row separately with:
- Fixed "N/A" label (disabled input)
- Score input (can be null)
- Hidden trash icon (spacer for alignment)

---

### 3. **Add checkbox onChange handler** (lines 250-260)
```diff
  <Checkbox
    label={t('criteria-labeled-options.allow-na', 'Allow N/A')}
    {...checkedControllerFieldToMantine(showNAField)}
    data-testid="criteria-labeled-allow-na-checkbox"
+   onChange={(event) => {
+     const checked = event.currentTarget.checked;
+     showNAField.onChange(checked);
+     if (checked && enableNAScore && !isNAOption) {
+       onAddLabel(true);
+     }
+     if (!checked && isNAIndex >= 0) {
+       handleRemoveOption(isNAIndex);
+     }
+   }}
  />
```

**Why:** NEW - handles checking/unchecking "Allow N/A":
- Checked → create N/A option
- Unchecked → remove N/A option

---

### 4. **Change onClick to arrow function** (line 266)
```diff
  <Button
    className={styles.footer__addOption}
-   onClick={onAddLabel}
+   onClick={() => onAddLabel()}
    onPointerDown={stopPropagationOfEvent}
```

**Why:** `onAddLabel` now takes an optional parameter `isNA?: boolean`. Calling with `()` passes `undefined` (adds normal option).

---

## Everything Else is Indentation

Lines 170-220 look like big changes but they're just **+2 spaces** of indentation because we changed:

**Before:**
```tsx
{optionsField.fields.map((field, index) => (
  <Flex>...</Flex>
))}
```

**After:**
```tsx
{optionsField.fields.map((field, index) => {
  if (watchedOptionsField?.[index]?.isNA) return null;
  return (
    <Flex>...</Flex>
  );
})}
```

The `(` → `{` change requires:
1. Adding explicit `return` statement
2. Indenting the JSX by 2 more spaces
3. Adding closing `);` instead of `)}`

**No actual logic changes in those 50 lines** - just indentation!

---

## Visual Summary

```
Lines 167-169:  ✅ MEANINGFUL - Skip N/A in loop
Lines 170-220:  ⚪ INDENTATION ONLY
Lines 221-244:  ✅ MEANINGFUL - N/A score row (NEW)
Lines 245-249:  ⚪ NO CHANGE
Lines 250-260:  ✅ MEANINGFUL - Checkbox onChange (NEW)
Lines 261-265:  ⚪ NO CHANGE
Line 266:       ✅ MEANINGFUL - onClick={() => onAddLabel()}
```

**Net meaningful additions:** ~35 lines of actual new code.
