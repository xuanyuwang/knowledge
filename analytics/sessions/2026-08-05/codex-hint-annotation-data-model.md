# Hint annotation data model and query correctness

**Date:** 2026-08-05
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Worktree:** `/Users/xuanyu.wang/repos/go-servers-convi-7387`
**Branch:** `convi-7387-discrepancy-in-user-adherence-data-between-insights-tools`
**Issue:** https://linear.app/cresta/issue/CONVI-7387/discrepancy-in-user-adherence-data-between-insights-tools
**PR:** https://github.com/cresta/go-servers/pull/30782

## Questions

- What do Action, ActionAnnotation, Moment, and MomentAnnotation mean?
- Are action and moment annotations embedded or independent?
- What are their ClickHouse schemas and linkage fields?
- How does `RetrieveHintStats` aggregate them, and why is the action-grain fix correct?

## Verified data model

- `Action` is a configured definition. `ActionAnnotation` is a runtime prediction in a conversation/message that results in an action.
- For Behavioral Hints, an action annotation represents the concrete assistance action shown or fired. It is not the agent's performance of the encouraged behavior.
- `Moment` is a configured conversation-state detector. `MomentAnnotation` is one runtime observation.
- The tables are independent. A moment does not contain the action table.
- `MomentAnnotationMetadata.adherence_action_annotation` is a singular same-conversation reference to the SDX action annotation associated with a DDX.
- Conversion code extracts the action annotation ID from that resource name into `MomentAnnotations.AdherenceActionAnnotationID`.
- ClickHouse projection looks up the referenced action annotation, writes the link as `moment_annotation_d.adherence_action_annotation_id`, and copies selected action fields into `adherence_action_*`.
- There is no ClickHouse foreign key. Referential integrity is application-managed.
- The relation used by hint analytics is many adherence outcome moments to zero or one linked action annotation. Multiple DDX rows can reference one hint action.

## Identifier map

- `action_id`: configured Action identifier.
- `action_annotation_id`: runtime ActionAnnotation occurrence.
- `moment_template_id`: configured Moment identifier.
- `moment_annotation_id`: runtime MomentAnnotation occurrence.
- `adherence_action_annotation_id`: linked runtime SDX action annotation on a DDX/DNX moment.
- `adherence_action_id`: linked configured action identifier.
- `adherence_moment_annotation_id`: linked SDX moment annotation associated with a DDX/DNX result.

## Schema sources

- `clickhouse-schema/conversations/migrations/20230824160348_init_db.up.sql`
  - `action_annotation` and distributed `action_annotation_d`.
  - `moment_annotation` and distributed `moment_annotation_d`.
- `cresta-proto/cresta/v1/action/action.proto`
- `cresta-proto/cresta/v1/action/action_annotation.proto`
- `cresta-proto/cresta/v1/moment/moment.proto`
- `cresta-proto/cresta/v1/moment/moment_annotation.proto`
- `go-servers/shared/converters/momentconverter/moment_annotation_converters.go`
- `go-servers/shared/clickhouse/conversations/conversation.go`

## RetrieveHintStats query

1. Generate table-specific filters and grouping expressions.
2. Build `hint_sent` from a Behavioral branch and a non-Behavioral action-annotation branch.
3. Collapse sent branches with `MAX` per group.
4. Build `hint_followed` from Behavioral/checklist DDX moments and KB/GW followed events.
5. Left-join followed groups to sent groups and combine followed branches with `SUM`.

The default Behavioral sent branch counts distinct non-empty `adherence_action_annotation_id` from DDX/DNX moment rows. The alternate branch counts `action_annotation_d.action_annotation_id` restricted to IDs referenced by DDX/DNX moments.

Before CONVI-7387, positive Behavioral moments were counted by distinct `moment_annotation_id`. That allowed several DDX observations linked to one hint to count several times. PR #30782 instead counts distinct non-empty `adherence_action_annotation_id`, so each sent hint contributes at most one followed hint per query group.

## Correctness assessment

- For the default moment-based sent path, DDX followed IDs are a set subset of DDX/DNX sent IDs under the same filters and groups.
- For the alternate action-annotation sent path, sent applies both moment-table and action-table predicates while followed applies only moment-table predicates. Subset correctness therefore assumes the predicates agree, linked action rows exist, and their denormalized dimensions match.
- ClickHouse does not enforce that assumption with a foreign key. Orphaned links or mismatched policy/group fields could still violate the invariant.
- `combined_hint_sent` uses `MAX` across Behavioral and non-Behavioral branches, while final followed uses `SUM` across Behavioral and KB/GW branches. Mixed-category requests can therefore undercount sent hints and distort engagement. This does not explain CONVI-7387 because its Behavioral Hint filter empties the non-Behavioral sent branch.
- The PR's generated-SQL regression test covers both sent-query modes, but an end-to-end data fixture and integrity monitoring remain useful follow-ups.
