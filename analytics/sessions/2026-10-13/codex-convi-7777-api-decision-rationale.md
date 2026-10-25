# CONVI-7777 team decision rationale

**Date:** 2026-10-13  
**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`  
**Worktree/branch context:** `/Users/xuanyu.wang/repos/go-servers-convi-7777-assistance-usage`, `xw/convi-7777-assistance-usage`; related proto worktree `/Users/xuanyu.wang/repos/cresta-proto-convi-7777-assistance-usage`.

## Work done

Prepared [team-facing API decision rationale](../../deliverables/convi-7777-assistance-usage-api-decision-rationale.md) from the current semantics audit, split-query discussion, dedicated implementation plan, work item, and prepared proto contract. It explains why existing aggregate responses cannot yield a distinct-conversation union, compares backend-RPC composition, frontend composition, and a dedicated API, and records the population, attribution, filter, source, metadata, and aggregation gaps. It identifies the proto/HTTP route, intended internal flow, performance measurements, and three proposed cleanup tickets. No external tickets or product code were changed in this session.

Added a compact V1/V2 scope table on request: ten V1 categories, chat Smart Compose and all email assistance as later-phase P2/V2, Silence/Hold deferred pending a delivered-hint link, and Reminder/Customer Snapshot excluded without an automatic V2 commitment. The table distinguishes proposed feature wording from the current predicates used for parity.

## Evidence boundary

The dedicated proto has passed its separate local checkpoint, while backend/frontend implementation remains planned. This document reports source-level semantics and design decisions. It does not establish customer-row mismatch frequency or query performance. Existing feature use predicates remain the current-count reference, with Summary and metadata path defects called out explicitly.
