# Backward compatibility for three template scope states

> Superseded exploration: Xuanyu clarified later on 2026-10-05 that templates need only all/some. The three-state requirement belongs to criteria in versioned template JSON. See [the corrected scope model](codex-criterion-three-state-versioned-json.md). The investigation below remains historical evidence, not the current design direction.

- Date: 2026-10-05
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only)
- Related source repos: `/Users/xuanyu.wang/repos/cresta-proto` and `/Users/xuanyu.wang/repos/director` (`main`, read-only)
- Input: Xuanyu's proposal to encode all / none / finite subsets as SQL NULL / empty array / non-empty array.
- Data evidence: [template array state inventory](codex-template-usecase-array-states.md); all 1,513 rows in each table of the selected `voice-sandbox-2` database have one use case. Other databases are unverified.

## Compatibility answer

The DB encoding is feasible without changing the PostgreSQL column type. It is not a backward-compatible reinterpretation of `{}` by itself. Current list/QA filters include both `IS NULL` and `= '{}'` as all, `ScorableScorecardTemplate.IsApplicable` treats any zero-length array as all, and `ScorecardTemplateProtoFromDB` maps both SQL states to an empty repeated `usecase_names`. Existing callers can neither express nor read the new distinction.

Add an explicit presence-bearing API scope mode to `ScorecardTemplate` (for example `ALL`, `NONE`, `INCLUDED`, with absent/unspecified meaning the legacy request contract), while retaining `usecase_names` for the finite IDs. On new writes: `ALL` → SQL NULL, `NONE` → SQL `{}`, `INCLUDED` + a non-empty name list → that array. A legacy request with no mode and no names continues to mean all and is written as SQL NULL; a legacy request with names writes the finite array. Reject contradictory mode/list combinations. Populate the mode on reads. A new caller must use the mode for all versus none; it cannot infer this from the repeated list.

Old clients reading a new `NONE` template would see an empty `usecase_names` and drop the unknown scope mode. A legacy full-template Commit could then inadvertently broaden the template back to all. The server must reject a scope-ambiguous legacy Commit against an existing NONE revision, or preserve NONE when the request clearly changes only other fields. A FieldMask for scope-only writes is another way to make intent explicit, but does not solve old clients editing a new NONE revision by itself. Old clients cannot fully understand or intentionally select the new NONE state; avoid silent broadening.

## Safe rollout order

1. Inventory NULL/empty/non-empty in both template tables across the intended customer population, including historical revisions. Any legacy empty rows mean all today and must be canonicalized to NULL before switching semantics. The sampled `voice-sandbox-2` database needs no such data rewrite.
2. Deploy API and writer changes that preserve legacy empty-request = all and write SQL NULL for it. Add the new scope mode, read representation, and safe legacy-Commit handling. Keep empty DB arrays interpreted as all during this phase.
3. Update and verify every reader, including direct SQL queries and background workers, to use `usecase_ids IS NULL` for all, cardinality zero for none, and membership for finite arrays. Wait until old binaries that treat `{}` as all or can write legacy `{}` are retired. Normalize any remaining legacy `{}` rows to NULL in both tables.
4. Switch interpretation of `{}` to none and only then permit new NONE writes. Until the switch, do not persist a NONE row: old running readers would treat it as all.

This is a compatibility plan, not an agreed implementation. A template with no applicable use case may need a product rule for whether it can be active; that is separate from its storage encoding. The criterion-level zero-applicability state is also separate.

## Alternative: `{}` = all and SQL NULL = none

Xuanyu proposed reversing the two zero-length states. This preserves the **normal legacy write** more naturally: `newDBScorecardTemplateFromProto` already makes a non-nil empty slice for an empty `usecase_names` request, and `lib/pq` writes it as `{}`. Director normally writes a non-empty list. Existing empty DB arrays also retain their current all meaning.

It does not remove the reader/API migration. Existing list/QA SQL explicitly includes `IS NULL` as all, and `ScorableScorecardTemplate.IsApplicable` treats nil and empty alike through `len(...) == 0`; both must change before a NULL/none row is written. The DB-to-proto converter and repeated `usecase_names` also collapse NULL and `{}`, so a presence-bearing API scope mode is still needed. An old binary would treat a newly written NULL/none template as all. An old client editing one could drop the mode and turn it back to `{}`/all, so ambiguous legacy Commit must be rejected or safely preserve the prior scope.

The nullable columns currently have **no SQL default**; an INSERT omitting `usecase_ids` becomes NULL. Under this reversed mapping, omission would silently mean none. Set an explicit `DEFAULT '{}'::varchar[]` or require deliberate scope assignment on every insert before enabling NULL/none. Existing legacy NULL rows, if any in other customer databases, currently mean all and must be migrated to `{}` in both current and revision tables before the switch, or distinguished by a separate version marker. The checked `voice-sandbox-2` database has zero NULL/empty rows and cannot settle the global migration need.

Assessment: this mapping reduces write-path migration for existing callers, but it is not compatible with old readers once NONE exists. It also makes implicit SQL NULL a dangerous default. Prefer it over the opposite mapping only if the old-writer continuity is more valuable and the explicit default, population audit, and reader rollout are accepted.

## Working rollout direction from Xuanyu

Xuanyu proposes assuming no existing NULL rows because a template historically belongs to a use case, then (1) deploy readers that interpret SQL NULL as none and `{}` as all, and (2) deploy writers that can persist SQL NULL for none. This order is sound **if** the reader rollout finishes everywhere before the first none write. The checked `voice-sandbox-2` data supports the no-NULL premise for that database only; a pre-enable count guard should verify the premise for every database in rollout, in both template tables.

Reader rollout means more than list filtering: remove `IS NULL` from all-applicability SQL branches (`action_list_current_scorecard_templates.go`, `action_export_scorecards.go`, `shared/qa/scorecard_template.go`), make `shared/scoring/scorecard_template_dao.go:IsApplicable` distinguish a nil slice from a non-nil empty slice, and check other direct SQL/background readers. The public read representation must expose the scope mode because `ScorecardTemplateProtoFromDB` currently maps both zero-length states to an empty repeated `usecase_names` list. Empty `{}` remains all; NULL must be excluded from use-case-filtered discovery and evaluation.

Writer enablement needs an explicit none signal: absent/legacy mode plus empty `usecase_names` remains `{}`/all, and explicit NONE writes nil `pq.StringArray`/SQL NULL. Add `DEFAULT '{}'::varchar[]` or an equally explicit insert invariant before enabling NONE. A legacy full-template Commit against a NONE revision must not silently widen it to all. Also change Commit's prior-revision lookup to use template identity/revision, rather than incoming `usecase_names`; otherwise changing a NONE revision to a finite set can fail to find the old row. Keep NONE writes gated until every old reader binary is retired.

This is the current working rollout direction, not an implementation. The premise of zero existing NULL rows is an explicit assumption to check at rollout, not a globally verified data fact.
