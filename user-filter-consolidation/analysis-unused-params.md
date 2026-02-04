# Unused Parameters Analysis

## Summary

Three parameters in `ParseUserFilterForAnalytics` are **declared but never used**:

| Parameter | Line | Status |
|-----------|------|--------|
| `enableListUsersCache` | 111 | **UNUSED** - declared, not referenced |
| `listUsersCache` | 112 | **UNUSED** - declared, not referenced |
| `shouldMoveFiltersToUserFilter` | 115 | **UNUSED** - declared, not referenced |

## Verification

Searched for each parameter name in `common_user_filter.go`:
- `enableListUsersCache` - only appears at line 111 (declaration)
- `listUsersCache` - only appears at line 112 (declaration)
- `shouldMoveFiltersToUserFilter` - only appears at line 115 (declaration)

## Why These Exist

These parameters were likely added for:
1. **Cache params**: Compatibility with old implementation (`ListUsersMappedToGroups`)
2. **shouldMoveFiltersToUserFilter**: Future use or copy-paste from old code

## Derivation Pattern for shouldMoveFiltersToUserFilter

All callers derive it the same way:
```go
shouldMoveFiltersToUserFilter := req.FilterByAttribute != nil &&
    (len(req.FilterByAttribute.Groups) > 0 || req.FilterByAttribute.ExcludeDeactivatedUsers) ||
    shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP)
```

This is just:
- `len(reqGroups) > 0`
- `excludeDeactivatedUsers`
- A flag for "grouping by group attribute"

## Recommendation

**Remove all 3 parameters** when consolidating to shared package:
- Reduces parameter count from 17 to 14
- Combined with other optimizations, final count: **~4 required + options**

## Updated Parameter Count

| Category | Before | After Removing Unused |
|----------|--------|----------------------|
| Required | 17 | 14 |
| With options pattern | - | ~4 required |
