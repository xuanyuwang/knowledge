# NCLH browser check with CLO MV enabled

**Date:** 2026-08-11
**Environment:** NCLH production, `us-east-1`
**Page:** `https://nclh-us-east-1.cresta.com/director/insights/performance/conversations`
**Purpose:** Directional latency check after the CLO MV flag was enabled; not a controlled benchmark.

## Test state

The user confirmed the CLO MV flag was on. The existing Performance Insights page state was:

- Template: `Recipe Questions`
- Criteria: `All`
- Target audience: unrestricted (`Teams / Groups / Users` placeholder)
- Time period and frequency: `Last 180 days | Daily`
- Duration: `All minutes`
- Exclude voicemail: `Yes`
- Calculation: `Criteria adherence`
- Cresta-modeled outcome filters: `Conversions All = Converted` and `CSAT = low CSAT`

To make the sample closer to the historical conversion-only request, the test deselected `low CSAT`, leaving `Conversions All = Converted` as the CLO condition. This caused four visible page regions to enter a `Loading` state.

## Result

Polling the visible loading regions produced these completion bounds:

- 34.477 s: 4 loading regions remained
- 37.498 s: 3 loading regions remained
- 40.534 s: 0 loading regions remained

Therefore the browser-visible refresh completed in **37.5-40.5 s**. This is safely below the frontend's 120 s timeout, with at least 79.5 s of margin using the conservative upper bound.

The closest historical raw-table baseline is the 2026-07-28 representative NCLH CLO query at **85.830 s**. Relative to that baseline, the new browser observation is directionally:

- **52.8-56.3% lower latency**
- **2.12-2.29x faster**

The original `low CSAT` filter was restored after the run. The restored page showed the original 25% Performance Score, 3m 19s Average Handle Time, and 19,217 filtered conversations. A warm restoration completed within 13.7 s, but the loading interval was not captured precisely, so it is excluded from the improvement estimate.

## Limitations

- This was not an identical request pair: the browser state used the live `Recipe Questions` template and current production data, while the historical request used the supplied six-month payload and an earlier execution window.
- The observed value is browser-visible page completion, not ClickHouse `query_duration_ms`.
- The Chrome automation surface did not expose Resource Timing entries for `qaScoreStats:retrieve`, so the exact longest request could not be read directly. The test used disappearance of all visible loading regions as a conservative page-level proxy.
- Cache state and concurrent production load were not controlled.
- One measured cold-ish transition is enough for a rough estimate, not p50/p95 evidence.

## Interpretation

The result is strong directional evidence that the CLO MV materially improves the slow path and keeps this sample well below the frontend timeout. It should not replace a controlled same-payload flag-off/flag-on comparison or ClickHouse query-log analysis if a release-grade performance claim is required.

## Credentials

No credentials were read or used. The test used the user's existing signed-in Chrome session.
