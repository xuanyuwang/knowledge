# Scorecard Template Structure

**Created:** 2026-03-26
**Updated:** 2026-03-26

## Overview

Deep dive into the scorecard template data model, scoring algorithm, and how templates flow through the system — from creation in Director UI → storage in Postgres → score calculation → analytics aggregation in ClickHouse.

## Key Source Locations

| Area | Path |
|------|------|
| Template Go types & parsing | `go-servers/shared/scoring/scorecard_templates.go` |
| Score calculation algorithm | `go-servers/shared/scoring/scorecard_calculator.go` |
| Criterion percentage scoring | `go-servers/shared/scoring/scorecard_scores_dao.go` |
| Auto-QA scoring | `go-servers/shared/scoring/autoqa_scoring.go` |
| Template DB operations | `go-servers/shared/qa/scorecard_template.go` |
| Analytics QA score stats | `go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go` |
| ClickHouse score indexing | `go-servers/shared/clickhouse/conversations/conversation.go` |
| Proto definition | `cresta-proto/cresta/v1/coaching/scorecard_template.proto` |

## Internal Docs

| Doc | URL |
|-----|-----|
| Scorecard Async Database Updates | [Notion](https://www.notion.so/2974a587b06180f595c3c14492a96104) |
| Duplicate Template Investigation | [Notion](https://www.notion.so/2fb4a587b0618033a7fcf6230f964cbd) |
| Cresta QA User Guide | [Coda](https://coda.io/d/_dB67ghP7yCZ/_suEaeJ6F) |
| Quintile Rank Behaviour Ref | [Coda](https://coda.io/d/_daskGrvEvmm/_sunGeWL3) |
| Data Platform Export Design | [GDrive](https://docs.google.com/document/d/12Q7Qui8SgtIy1NOOrHB4oHlA5FLO3iinPiqXoj2mJys) |

## Log History

| Date | Summary |
|------|---------|
| 2026-03-26 | Initial exploration: template structure, scoring algorithm, analytics pipeline |
