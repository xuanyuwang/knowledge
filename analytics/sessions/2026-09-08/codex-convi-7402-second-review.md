# CONVI-7402 second correctness review

## Finding: [P2, high confidence] Omitted moment types bypass shared overlap rejection

Location: [common_clickhouse.go:678](/Users/xuanyu.wang/repos/go-servers-convi-7402/insights-server/internal/analyticsimpl/common_clickhouse.go:678), also the symmetric skip at line 668 in `validateNoMetadataMomentOverlap`.

Trigger: a metadata value-or-missing group whose two sides name the same template A, but only one side sets `type=CONVERSATION_METADATA`; the other omits type (protobuf default `TYPE_UNSPECIFIED`). For example:

```json
{
  "moments": [{"name": "customers/c/profiles/p/moments/A", "type": "CONVERSATION_METADATA"}],
  "excluded_moments": [{"name": "customers/c/profiles/p/moments/A"}],
  "metadata_value_attributes": [{"metadata_value": {"string_value": "X"}}]
}
```

Expected: preserve `(A=X OR missing(A))`, or explicitly reject the unsupported shared ClickHouse shape before lowering. QA's dedicated parser already rejects it; the shared parser should not silently change it.

Actual: the new validation helper skips every entry not explicitly typed as metadata before comparing identities. With an untyped exclusion, `parseClickhouseFilter` accepts the request and returns one include filter and zero exclusions (value-only). With an untyped inclusion and typed exclusion, it returns zero includes and one exclusion (missing-only). Either drops part of the OR and returns incorrect results on consumers such as Conversation Stats and Agent Stats.

The ES reference identifies a metadata group if **either** side contains a metadata moment (`elasticsearch/common.go:138`), then pairs moments by ID without requiring both entries to carry a type. Reproductions using the exact same two request shapes produced two ES `should` branches with `minimum_should_match=1`, versus the partial shared ClickHouse filters above. This is a remaining gap in the existing shared-parser repair; the partial-object behavior predates the five fixes, rather than being a newly introduced regression. The normal Director helper supplies types on both sides; occurrence through other clients or saved partial filters was not measured.

Why CI can pass: all overlap-rejection cases in the committed table build every moment with explicit metadata type. The expanded ID cardinality coverage therefore does not exercise type omission. Pure QA-parser tests use a different stricter mixed-group guard.

Concrete regression: parameterize `[included metadata, excluded unspecified]` and `[included unspecified, excluded metadata]`, keeping identical resource names and a selected string value. Require `InvalidArgument` from the shared parser (or a representation retaining both OR branches). Both expected-rejection tests failed in this review; equivalent ES parser tests passed.

Fix direction: classify the whole group before validating overlap. Once it is a metadata group, compare all included/excluded resource IDs independently of each entry's redundant type, or explicitly reject omitted/inconsistent types. Keep existing legitimate non-overlapping groups and the exclusions-disabled behavior intact. No fix was implemented in this review.

## Reviewed state and scope

- Source repo: `/Users/xuanyu.wang/repos/go-servers`
- Worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7402`
- Branch: `convi-7402-bswift-users-getting-qa-score-stats-errors`
- Local and live PR head both verified: `fe0109acd5e6176c356142cfe51f7357b142d7d6`.
- Fetched main: `782e72ba518f2ff817a5bca121aa5619f0b035e2`.
- Four commits landed on main since the earlier base `98583180ee`: unrelated AI-receptionist/ASR changes and a proto dependency update v2.22.37 → v2.22.38. No changed analytics or shared ClickHouse files. No new rebase was needed for this focused review; tests ran at the exact PR head with its v2.22.37 dependency.
- Reviewed all five fix commits against `12117463df`, plus their interaction with the complete PR and shared/ES semantics.
- Temporary test files were removed after saving their sources. Source worktree is clean; no product edits, commits, pushes, external comments or review-thread changes.

## Coverage and execution

| Case | Result this round |
|---|---|
| Shared same-ID overlap with one type omitted | Two expected-rejection cases fail; one side of OR is discarded |
| Same request shapes in ES | Both produce value-or-missing OR; parser tests pass |
| Explicitly typed shared ID sets | All 49 combinations of nonempty include/exclude subsets of three IDs match expected overlap rejection |
| Two independent QA overlap groups | All nine present-selected/present-other/absent combinations executed; exactly four correct combinations pass |
| Two overlaps with `join_use_nulls=0` and `1` | Both runtime cases pass |
| Numeric `[5,10)`, `[5,10]`, `(5,10)`, `(5,10]` plus missing | All four generated ClickHouse predicates match independently constructed ES range expectations on 5, 6, 10 and missing fixtures |
| Submit-time endpoint wiring, nil/empty value validation, literal zero preservation | Code/test audit of previous fixes; no additional defect found; prior passing suites/runtime evidence retained |
| Shared caller paths and actual outcome handling | Rechecked group classification/identity loss and converter usage; no other new finding |

Commands executed:

- `gh pr view 31335 --repo cresta/go-servers --json headRefOid,baseRefOid,statusCheckRollup` and a later status refresh, with environment token overrides unset: live head matches local; CI/review checks pending or running when observed.
- `git fetch origin main convi-7402-bswift-users-getting-qa-score-stats-errors`, explicitly selecting the cleared SSH key: success. `git log`/diff of new main commits confirmed no relevant source overlap.
- `GOPROXY=off GONOPROXY=none GOTOOLCHAIN=local go test ./insights-server/internal/analyticsimpl ./insights-server/internal/analyticsimpl/elasticsearch -run '^TestSecondReview' -count=1 -v`: two shared expected-rejection failures; 49-set invariant and query export pass; ES equivalent-shape tests pass.
- Additional ES numeric reference run: pass, 0.972 s. This evaluates the ES production converter's range objects on fixtures; it is not a live ES query.
- Final SQL export run: pass, 1.476 s. The temporary exporter supplies the normal score predicates from `parseScoreConditionsForQAAttribute`; an initial harness omission of those defaults was corrected before runtime assertions, with no product change.
- `python3 /tmp/convi7402-review2/runtime.py`: six real ClickHouse 26.8.2.7 executions pass (two multi-overlap/null-setting cases, four numeric-range comparisons). Each file uses in-process Memory tables and synthetic rows.
- Final `git status --short` clean and `git diff --check` passes.

Full source and outputs: [second-review evidence](convi-7402-second-review-evidence/). Existing suites were not rerun unchanged simply to repeat the prior passing results. No whole-package/Bazel build, live ES server, or production distributed/load test was performed.

## Residual risks and credentials

Previously documented production risks remain: unbounded submit-time annotation scans, MV freshness/retention and raw-versus-MV latest ordering, actual production schema/version/distributed settings. The new null-setting runtime checks cover overlap-only queries; independent exclusion-only joins retain the prior empty-string/default-setting assumption.

Credentials used after their individual inspection earlier in this same session: GitHub CLI keychain service `gh:github.com`, account `xuanyuwang`; SSH `/Users/xuanyu.wang/.ssh/id_ed25519` for explicit fetch. No PostgreSQL or ClickHouse credential was used in this round; local ClickHouse is in-process. No restricted credential was used.

## Next action

One small shared-validation fix and regression pair remain before considering the reviewed contracts complete. The other four fix commits have no newly identified correctness issue. CI and code-owner review are still required independently of this local review.
