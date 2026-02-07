# QA Score Popover Bug Fix

**Created:** 2026-02-07
**Updated:** 2026-02-07

## Overview

Investigation and fix for the QA score popover bug where non-scorable criteria showed wrong data in the Performance page.

## Problem Summary

**Bug:** The popover showed data for ALL criteria instead of just the clicked criterion, but only for non-scorable criteria.

**Root Cause:** The filtering logic had a fallback that returned ALL scorable criteria when the intersection with requested criteria was empty (which happened for non-scorable criteria).

## Solution

Created a global criterion filter at the `PerformanceProgression` level:
- `RELATIVE` mode: Filter to scorable criteria only
- `EXCEPTIONS/OVERWRITTEN` modes: Include all criteria (scorable + non-scorable)

**Branch:** `convi-5805-discrepancy-between-the-number-of-overwritten-shown-in-perf`

## Key Learnings

1. Pattern recognition matters - finding that only non-scorable criteria were affected focused the investigation
2. Architecture over quick fixes - global filter fixed everything consistently
3. Type safety prevents bugs - filter functions instead of boolean flags

## Log History

| Date | Summary |
|------|---------|
| 2025-12-16 | Full development journey from investigation to fix |
