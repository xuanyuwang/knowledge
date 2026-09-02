# CONVI-7533 directly readable CSV backup

## Context

- Primary source repo context: `/Users/xuanyu.wang/repos/go-servers`, branch `main`
- Source artifact: captured PostgreSQL row-level export supplied by the user
- Destination: existing Linear document `f5dc494b-eafa-4d7e-a265-f02f2d9cc571`

## Execution

Parsed each `row_json` object from the captured export without database access. Generated UTF-8 RFC 4180-compatible CSV with CRLF record terminators, retaining SQL NULL as an unquoted empty field and database empty strings as quoted empty fields. JSON arrays remain compact JSON strings escaped as CSV fields.

Published five indexed document comments:

- all 4 `director.scorecards` rows;
- 21 scores for `019c2e30-ba72-7ed4-a202-b3ea651eaeec`;
- 56 scores for `019e7502-3fd5-7528-94c6-e45c0b7e7448`;
- 57 scores for `019e7503-2411-7e04-883f-6bea8930d41c`;
- 56 scores for `019e7504-dc03-78a7-b70b-d0e751b4e725`.

## Verification

Reread the updated Linear document and the five latest comments. Extracted each stored CSV code block, verified exact byte equality and SHA-256 against generated payloads, confirmed CRLF record counts, parsed every record, and checked expected row and column counts. Result: 4 scorecard rows, 190 score rows split `21 / 56 / 57 / 56`, with all 33 scorecard and 16 score columns represented.

No production database query or mutation occurred.
