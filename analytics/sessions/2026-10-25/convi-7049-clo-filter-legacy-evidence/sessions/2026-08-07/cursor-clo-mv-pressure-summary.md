## Pressure test — CLO moment combinations × time windows (2026-08-07)

**Target:** voice-staging / `cresta_walter_dev`
**End exclusive:** `2026-08-01 03:59:59` UTC (Toronto day boundary)
**Windows:** 7d / 30d / 90d / 180d
**Usecase:** `walter-dev`; timezone `America/Toronto` (`toIntervalHour(-4)`)

### Moments under test

| Key | Template ID | Value predicates |
|---|---|---|
| `csat_a` | `01964a99-d3ce-7b0d-823f-b7dbb50195c2` | numeric bins `[-1,-1] ∪ [0,6] ∪ (6,10]` |
| `csat_b` | `0199e72b-551b-74ca-b5de-5c1d9491a983` | same numeric bins |
| `resolved` | `7252ec08-0620-4c83-93aa-6ccc84b6750b` | string `resolved` OR `unresolved` OR `not_care` |
| `conversion` | `921efbb7-7797-4dcb-874e-16103e1b5102` | bool `true` OR `false` |

Multiple moment groups are AND’d (one CLO CTE + INNER JOIN per group), matching insights-server routing.

### A) Full QA score-stats correctness + latency (28 cases)

Combos: each single; `csat_a+resolved`; `resolved+conversion`; all-4. × all windows.

**Result: 28/28 MATCH** (daily aggregates identical: rows, convs, scorecards, weights, weighted sum within float epsilon).

| Case | raw_s | mv_s | speedup | sum_convs |
|---|---:|---:|---:|---:|
| `7d__csat_a` | 1.19 | 0.86 | 1.37× | 88 |
| `30d__csat_a` | 2.73 | 2.73 | 1.00× | 720 |
| `90d__csat_a` | 6.57 | 4.47 | 1.47× | 5735 |
| `180d__csat_a` | 21.25 | 9.75 | 2.18× | 19106 |
| `7d__csat_b` | 1.26 | 0.97 | 1.30× | 88 |
| `30d__csat_b` | 3.34 | 2.11 | 1.58× | 720 |
| `90d__csat_b` | 7.23 | 6.39 | 1.13× | 5735 |
| `180d__csat_b` | 18.00 | 12.38 | 1.45× | 19106 |
| `7d__resolved` | 1.09 | 1.10 | 0.99× | 88 |
| `30d__resolved` | 2.56 | 2.33 | 1.10× | 720 |
| `90d__resolved` | 6.52 | 5.17 | 1.26× | 5734 |
| `180d__resolved` | 16.60 | 11.74 | 1.41× | 24044 |
| `7d__conversion` | 1.29 | 1.06 | 1.21× | 88 |
| `30d__conversion` | 3.23 | 2.28 | 1.41× | 720 |
| `90d__conversion` | 7.17 | 5.18 | 1.38× | 3011 |
| `180d__conversion` | 15.83 | 12.53 | 1.26× | 3013 |
| `7d__csat_a+resolved` | 1.58 | 0.99 | 1.60× | 88 |
| `30d__csat_a+resolved` | 4.87 | 2.47 | 1.97× | 720 |
| `90d__csat_a+resolved` | 9.48 | 5.17 | 1.84× | 5734 |
| `180d__csat_a+resolved` | 20.77 | 12.39 | 1.68× | 19083 |
| `7d__resolved+conversion` | 1.30 | 0.98 | 1.32× | 88 |
| `30d__resolved+conversion` | 3.98 | 2.19 | 1.81× | 720 |
| `90d__resolved+conversion` | 8.79 | 4.95 | 1.78× | 3011 |
| `180d__resolved+conversion` | 18.94 | 10.03 | 1.89× | 3013 |
| `7d__csat_a+csat_b+resolved+conversion` | 2.11 | 1.03 | 2.05× | 88 |
| `30d__csat_a+csat_b+resolved+conversion` | 5.04 | 2.14 | 2.35× | 720 |
| `90d__csat_a+csat_b+resolved+conversion` | 17.32 | 4.96 | 3.49× | 3011 |
| `180d__csat_a+csat_b+resolved+conversion` | 29.46 | 11.10 | 2.66× | 3012 |

All-4 highlight: 7d **2.05×**, 30d **2.35×**, 90d **3.49×**, 180d **2.66×** (raw vs MV wall time).

### B) Filter conversation-set correctness (reduced matrix)

- All 15 non-empty combos × **7d + 30d**
- Singles + all-4 × **90d**
- Singles × **180d** (all-4 × 180d filter-diff hit CH 300s `TIMEOUT_EXCEEDED`; covered instead by full score-stats match above)

**Result: 39/40 OK with raw_only=mv_only=0; mismatches=0; errors=1**

Errors:
- `180d__csat_a+csat_b+resolved+conversion`: CH timeout 300s on dual raw+mv filter-diff CTE (not a correctness mismatch; full QA path for same combo/window matched).

Example filter-diff counts (selected):

| Case | raw | mv | raw_only | mv_only |
|---|---:|---:|---:|---:|
| `180d__csat_a` | 38609 | 38609 | 0 | 0 |
| `180d__resolved` | 43518 | 43518 | 0 | 0 |
| `30d__csat_a+csat_b+resolved+conversion` | 10915 | 10915 | 0 | 0 |
| `30d__csat_a+resolved` | 11138 | 11138 | 0 | 0 |
| `7d__csat_a+csat_b+resolved+conversion` | 2275 | 2275 | 0 | 0 |
| `7d__resolved` | 2349 | 2349 | 0 | 0 |
| `90d__csat_a+csat_b+resolved+conversion` | 16594 | 16594 | 0 | 0 |

### Path differences (unchanged)

| | Flag off | Flag on |
|---|---|---|
| Table | `moment_annotation_d` | `moment_annotation_by_conversation_outcome_d` |
| Latest | `create_time` | `update_time` |
| Type | `moment_type = 14` | omitted |
| Number | `JSONHas` + `JSONExtractFloat` | `outcome_value_type='number'` + `outcome_number_value` |
| String | `JSONHas` + `JSONExtractString` | `outcome_value_type='string'` + `outcome_string_value` |
| Bool | `JSONHas` + `JSONExtractBool` | `outcome_value_type='boolean'` + `outcome_bool_value` |

### Verdict

Across combination and window pressure (through 180d), MV path matches raw correctness on QA score-stats; MV is faster especially for multi-CLO joins at 90–180d. Staging pilot flag enablement remains consistent with these results.

Exact example all-4 7d raw/MV SQL in follow-up comment.
