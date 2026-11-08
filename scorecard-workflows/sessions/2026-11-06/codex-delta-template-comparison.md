# Delta template configuration and criterion-ID comparison

- Date: 2026-11-06, America/Toronto
- Primary source repo: /Users/xuanyu.wang/repos/go-servers
- Context: existing main checkout; no product/source edits. This follow-up examines production director.scorecard_templates through a read-only connection, building on source snapshot b013c5ae3b from the earlier investigation.
- Work item: [Delta organization-target failure](../../work-items/CONVI-7981.md)
- Request: inspect other templates for empty use-case lists and non-UUID criterion IDs; compare with the POC Template.
- Scope: all templates/revisions for customer delta-air-lines in its application DB, including deleted/deactivated rows. This is not a cross-customer survey.

## Method and evidence

Used connect-customer-app-db skill and cresta-cli self-authentication. Config: origin/master:configv3/prod/delta-air-lines/us-west-2/config.yaml. Provider AWS, account us-west-2-prod, cluster us-west-2-prod-2, database delta-air-lines. No explicit AWS credential/profile or SSH key selected; no credentials printed/persisted. Broad config fetch again warned about case-colliding refs; read-only queries succeeded. No customer-data writes or external posts.

Inventory found one profile (us-west-2), four distinct template resource IDs, and 17 revisions. Five revisions have empty use-case arrays. Latest means the single newest stored revision per profile/resource ID by created_at, without excluding deleted rows. All four latest rows have type=1/status=1 and NULL deletion/deactivation timestamps.

[Saved SQL](delta-template-configuration-comparison.sql) recursively traverses JSON objects/arrays exactly once, collects objects with identifier and type (criteria, rather than chapters), and classifies IDs using the canonical hyphenated UUID pattern, case-insensitively. This avoids lax recursive JSONPath duplication seen in the previous metadata query. [Structured results](delta-template-configuration-results.json) preserve latest counts, per-template history, empty-scope revisions, pairwise ID overlap, and POC overlap across every other template's historical revisions.

## Current templates

| Template | Resource ID | Latest revision | Use cases | Criteria | UUID IDs | Non-UUID IDs |
| --- | --- | --- | --- | --- | --- | --- |
| Delta Care Template | 01a18389-c8f6-75e4-b05a-12ace3705645 | 6cc2062e | voice-care | 12 | 12 | 0 |
| Delta Care Template | 01a1838a-5ebf-70f0-8abe-e192b0cfc7d1 | 8c969b78 | synthetic-care | 20 | 20 | 0 |
| POC Template (original title Agent Adherence) | 01a1a9fc-4b07-70c7-ae85-ae2ded1a923a | 7508e489 | empty | 13 | 0 | 13 |
| Customer Experience | 01a1a9fc-4dbe-72b4-9e5e-8bfea0f3e902 | 7883b020 | empty | 5 | 0 | 5 |

Customer Experience has descriptive IDs: visitor-demand-escalation-criterion, visitor-mentions-airspace-outage-criterion, visitor-mentions-competitors-criterion, visitor-threaten-dot-complaint-criterion, visitor-threaten-litigation-criterion. POC Template retains the 13 agent-* criterion IDs captured in the saved results.

## Historical pattern and timing

- Both Delta Care templates were first stored October 28, 2026 (17:24:01.316055 and 17:24:23.376742 UTC). Each has six revisions. Every stored revision has nonempty use-case scope and only UUID criterion IDs. All their revisions have creator_user_id 1722a4291580a19f.
- Agent Adherence/POC Template was first stored November 5, 2026 at 04:34:06.216123 UTC, creator_user_id e2f03fc79f3250aa. Its next revision followed at 04:34:06.536737 UTC. All three revisions have empty scope and 13 non-UUID criteria; the later POC rename/edit at 20:34:22.611306 UTC is attributed to creator 1722a4291580a19f.
- Customer Experience was first stored at 04:34:06.911033 UTC, 694.910 milliseconds after Agent Adherence, with the same initial creator e2f03fc79f3250aa. Its second revision followed at 04:34:07.191588 UTC. Both revisions have empty scope and five non-UUID criteria.
- The two initial creation times correspond to November 4 at 11:34 p.m. in America/Toronto. Four early revision writes for the pair occurred within about 975 ms.

## Copying versus common authoring

None of the latest template pairs share criterion IDs, and no pair has identical template JSON fingerprints. POC Template's criterion IDs also match zero IDs in every historical revision of the other three templates. Therefore this survey does not identify another Delta template whose criterion IDs were copied unchanged into POC Template. Regenerating IDs or transforming a copied template could conceal reuse; this query does not exclude that.

The combination of same initial creator, subsecond creation timing, empty scope from the oldest stored revisions, and descriptive IDs in both new templates supports a common setup/import/API operation as a hypothesis. It does not establish the tool, prove automation, identify the original HTTP caller, or prove either template was copied. Current data do not indicate that later UI editing removed a previously populated scope in either new template's stored history.

## Follow-up

Trace the initial creator's requests around 2026-11-05 04:34:06-04:34:08 UTC for both resource IDs. If remediation is pursued, assess Customer Experience alongside POC Template because both fail the target API's nonempty use-case overlap lookup. No target-creation replay or remediation was performed in this follow-up.
