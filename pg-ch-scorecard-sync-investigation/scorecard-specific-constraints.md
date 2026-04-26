# Scorecard-Specific Constraints

## Purpose

Translate the general sync model into the real scorecard problem space.

## Constraints To Add

- PostgreSQL is the authoritative store for scorecards and scores
- ClickHouse is queried for analytics and reporting
- Scorecards are mutable and may be updated multiple times before or after submission
- Some writes are asynchronous
- Some recovery flows are time-range based
- Some scorecards do not map cleanly to conversation-based reindex flows
- Ordering matters when multiple async writes race
- Merge semantics in ClickHouse may not match application intent unless versioning is explicit

## Questions To Answer

- What is the authoritative version field for a scorecard projection?
- Which scorecard attributes are monotonic and which are mutable?
- Which write paths exist: create, update, submit, backfill, repair?
- Which fields depend on external context beyond the scorecard row itself?
- Which scorecards are invisible to conversation-based recovery?
- Which mismatches are count-only versus content-level mismatches?

## Failure Classes To Test Against

- Missing scorecard row in ClickHouse
- Missing score rows in ClickHouse
- Scorecard exists but submit state is stale
- Scorecard exists but criterion scores are stale
- Scorecard cannot be recovered by time-range conversation reindex
- Repair workflow writes incomplete projection
