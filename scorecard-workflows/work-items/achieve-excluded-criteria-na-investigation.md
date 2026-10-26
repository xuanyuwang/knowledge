# Achieve behavior values excluded from QA aggregation

**Primary domain:** scorecard-workflows
**Primary subdomain:** evaluation-and-scoring
**State:** historical synthesis; live ticket/PR/deployment state not refreshed
**Last meaningful source content:** 2026-04-26
**Consolidated:** 2026-10-25

## Meaning and recorded outcome

The recorded ClickHouse evidence distinguishes detected behavior from a computed QA percentage: affected rows had valid `numeric_value` 0/1 and `not_applicable=false`, but `percentage_value=-1` and near-zero `float_weight=1e-13`. The examined scoring code returns no percentage when `excludeFromQAScores` is true. Analytics filters nonnegative percentages or weights, so a valid Yes/No can still appear as N/A.

The customer-template attribution remains a hypothesis. Template `019c203e-1bba-71be-9ae7-e093ef0c80b7` matched the symptom pattern among three Welcome Call templates, but the notes explicitly require PostgreSQL verification of its title and criterion JSON. The working criterion might also follow an outcome scoring path that omitted the exclusion check. Do not upgrade the README's root-cause wording into confirmed customer configuration.

## Source context and limits

go-servers shared/scoring/scorecard_scores_dao.go and shared/clickhouse/conversations/scorecard_score.go; historical Achieve Welcome Call ClickHouse sample.

Inactivity evidence: Dated README and log agree. All substantive evidence reviewed predates the 2026-09-25 cutoff. Filesystem timestamps and later bulk Git capture do not establish later engineering activity. Statements about tests, implementation, rollout, and PRs above describe the retained record, not checks rerun during this documentation move.

## Continuing work

Verify the template/title and exclusion flags in authoritative template revisions before changing configuration. If incorrect, fix configuration and separately plan rescoring/projection repair; analytics presentation belongs to Analytics / QA Score.

## Preserved evidence

The original folder is retained intact at [convi-6672-achieve-behavior-na legacy evidence](../sessions/2026-10-25/convi-6672-achieve-behavior-na-legacy-evidence/README.md); its internal chronology remains authoritative for historical detail.

- [README.md](../sessions/2026-10-25/convi-6672-achieve-behavior-na-legacy-evidence/README.md)
- [log/2026-04-26.md](../sessions/2026-10-25/convi-6672-achieve-behavior-na-legacy-evidence/log/2026-04-26.md)
