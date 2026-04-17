# Export Appeal Comments

**Created:** 2026-04-14
**Updated:** 2026-04-14

## Overview

When exporting a scorecard to CSV, criterion comments are added as columns next to each criterion. However, when a criterion has an approved appeal with an approval reason, the export still shows the original comment — which doesn't reflect the final criterion value.

### Expected Behavior

- Fetch appeal approvals for a scorecard (if any), build a map: `scorecard_id + criterion_id -> appeal approval reason`
- When building CSV comment columns, check if an appeal approval reason exists and use it to replace the original comment

## Research Questions

1. How is the comment column of a criterion built in the CSV export?
2. How is scorecard data queried for export?
3. What's the best way to query appeals of scorecards?

## Log History

| Date       | Summary                                      |
|------------|----------------------------------------------|
| 2026-04-14 | Project created, initial research             |
