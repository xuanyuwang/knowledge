# CONVI-7378 follow-up conversation investigation

**Ticket:** [CONVI-7378](https://linear.app/cresta/issue/CONVI-7378/auto-scored-scorecard-changes-not-updating)
**Comment:** [Jimmy Skelton follow-up](https://linear.app/cresta/issue/CONVI-7378/auto-scored-scorecard-changes-not-updating#comment-a7b8348c)
**Domain:** analytics (`qa-score`)
**Source repo / branch:** `/Users/xuanyu.wang/repos/go-servers` / `main`

## Question

Determine why the newer SCAN conversation `019ffc63-5239-7c1b-9f9d-ebaef898ec5d`, created after the previously discussed template correction, still scores below 100% even though all displayed answers are Yes.

## Production evidence

- Telesales QM scorecard: `019ffc6d-4515-7c21-a9df-c383019e2ee2`.
- Created 2026-08-13 18:40:47 UTC; last updated 2026-08-17 01:33:05 UTC.
- Pinned template revision: `7290d017`, published 2026-08-04 14:24:57 UTC.
- Stored overall score: 95.8; Positive Contact / Communication Skills section: 83.3.
- Scorecard is `manually_scored=true`. Consent to Call stores `numeric_value=0` (Yes) and `ai_value=1` (original auto No), confirming the override persisted.
- PostgreSQL and ClickHouse agree. ClickHouse has 24 leaf criteria; all have `numeric_value=0`, but Consent to Call has `percentage_value=0` while the other 23 have `percentage_value=1`. Thus `23 / 24 = 95.8%`.

## Revision comparison

Consent to Call identifier: `019d4176-27ef-76e1-9836-9d2cabe674b5`.

- `a88e3c94` (2026-07-22): Yes/value 0 -> 0 points; No/value 1 -> 1 point.
- `eb3e8862` (2026-07-26): Yes/value 0 -> 1 point; No/value 1 -> 0 points.
- `7290d017` (2026-08-04): Yes/value 0 -> 0 points; No/value 1 -> 1 point.
- `b3facf54` (2026-08-17): Yes/value 0 -> 0 points; No/value 1 -> 1 point.

The July correction did not remain in subsequent template revisions. Revision `7290d017` reintroduced the inverted score mapping, and the latest observed revision `b3facf54` still has it.

## Conclusion

Jimmy is correct that the prior explanation was incomplete for this example. This scorecard does not predate all template changes. It uses a later revision that again maps Consent to Call Yes to zero points. The manual override is saved and projected correctly, but the pinned revision turns that Yes into 0%, producing the 83.3% section and 95.8% overall scores.

This remains a template-revision scoring configuration issue rather than a PostgreSQL-to-ClickHouse propagation defect. Correcting the active template mapping would protect newly created scorecards, but this manually edited scorecard remains pinned to `7290d017` and is not eligible for the existing AutoQM-only backfill path.

## Draft response

> You are right to question the earlier explanation—this newer example exposed an additional issue. I checked the stored scorecard and template revisions.
>
> The manual change did save correctly: “Consent to Call” was auto-scored No and then changed to Yes. However, this scorecard was created on August 13 using template revision `7290d017`, published August 4. That revision maps “Yes” to 0 points and “No” to 1 point for Consent to Call. As a result, Consent contributes 0% even though the displayed answer is Yes. The other 23 scored questions contribute 100%, which produces the 95.8% overall score; that section is 5 out of 6, or 83.3%.
>
> So this conversation does not predate the template correction as I previously suggested. The corrected mapping from July was reintroduced incorrectly in the August 4 revision, and the latest revision I checked still has the inverted mapping. This is a template configuration/revision issue, not a failure to save the manual override or update Performance Insights.
>
> The active template needs “Consent to Call” corrected to Yes = 1 and No = 0 so new scorecards calculate as intended. Existing manually edited scorecards remain pinned to their original revision and cannot be corrected by the AutoQM-only backfill. I’m sorry my earlier response missed that the mapping had regressed in a later revision.
