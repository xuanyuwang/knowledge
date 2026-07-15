# CONVI-7230 Performance Insights Outcome Filters

> Migrated navigation: [Analytics / Performance Insights](../analytics/subdomains/performance-insights/README.md). This folder remains detailed historical evidence.

## Status

Fix implemented, locally validated, committed, and opened as a draft PR on 2026-07-09.

## Problem

Holiday Inn Transfer reports that the customer-provided outcome `Package Paid` is no longer available as a Performance Insights filter, even though it remains available in Closed Conversations and is configured as active for `Filters`.

## Current Direction

Performance Insights now keeps customer-provided outcome filters available regardless of `enableCLOFilters`. Cresta-modeled conversation outcome filters remain gated behind that flag.

## Key Finding

Customer-provided outcomes such as `Package Paid` are metadata moments with `DETAILED_TYPE_METADATA_OUTCOME`, not the Cresta-modeled conversation outcome moments intended for the CLO feature flag. The regression came from gating both categories behind `enableCLOFilters` in the Performance Insights filter hook.

## Source Context

- Source repo: `/Users/xuanyu.wang/repos/director`
- Worktree: `/Users/xuanyu.wang/repos/director-convi-7230`
- Branch: `convi-7230-holiday-inn-transfer-performance-insights-regression-outcome`
- Ticket: `CONVI-7230`
- Draft PR: https://github.com/cresta/director/pull/20606