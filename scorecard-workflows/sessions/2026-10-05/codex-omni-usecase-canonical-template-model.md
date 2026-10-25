# Omni-use-case canonical template model

- Date: 2026-10-05
- Source repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only)
- Design input: Xuanyu's answer in the current engineering-plan discussion.

## Decision

Keep the current authoritative model: the Go template-structure structs and the versioned `template` JSONB on `director.scorecard_templates` and `director.scorecard_template_revisions`. Criteria remain owned by their template and its revision. Extend the existing criterion model for use-case applicability; do not make relational criterion rows authoritative for this project.

Direct criterion lookup is still an engineering requirement. A database view or revision-keyed index/projection may be assessed as a derived read model after query patterns are inventoried. It must not compete with the versioned template definition as a second source of truth.

## Criterion scope representation

Xuanyu chose included-use-case IDs on each criterion because they are straightforward to read. Adding a use case to a template requires traversing all criteria and adding it; removing one requires traversing all criteria and removing it where present. These transformations are committed as part of the new template revision, preserving `criterion use cases ⊆ template use cases`.

Xuanyu clarified that an empty list means all use cases for both templates and criteria; finite subsets use explicit included IDs. With the containment invariant, a finite template cannot contain a criterion whose empty list means global all. It must repeat the finite template IDs if the criterion applies everywhere within that template. Legacy criteria lack the field entirely, so omitted and explicit-empty representations need deliberate defaulting.

Xuanyu agreed to requiring an explicit finite template use-case list before any criterion is narrowed. Therefore `template=[]` (all) keeps `criterion=[]` (all); a finite template must carry finite criterion lists, including on criteria that apply to its full scope. A universal-to-finite template edit materializes those criterion lists in the new revision. This avoids the implicit new-profile-use-case event for finite criterion lists under a universal template.

Xuanyu confirmed scope belongs only on criteria, including criteria nested under branches. Chapters have no independent use-case scope.

## Canonical read contract

Xuanyu confirmed that `GetScorecardTemplate` and canonical list reads should return the full shared template revision. Evaluation and other consumers derive a use-case-specific effective view when needed. `apiserver/internal/coaching/action_get_scorecard_template.go` currently loads a revision and calls `ScorecardTemplateProtoFromDB` without a use-case filter. `apiserver/internal/coaching/transformers.go` converts the parsed structure into the response's `Template` field. A new criterion-scope field must round-trip through the parser and converter; a filtered response must not become the canonical editing representation.

Xuanyu confirmed the compatibility rule: an omitted criterion-scope field in historical JSON inherits that revision's template scope on read. Historical revisions remain unchanged. A subsequent commit of a finite-scope template materializes explicit criterion lists. The parser and serializer must distinguish omission from explicit empty because an empty criterion list means all use cases, while omission on an older finite-scope revision means its template scope.

Xuanyu confirmed that canonical Get/list responses should normalize historical stored revisions into the current full criterion model, materializing inherited scope from the same stored revision's `usecase_ids`. Historical JSON stays unchanged. This direction does not decide whether the persisted JSON structure version must increment; the earlier V3 proposal remains an implementation exploration.

`apiserver/internal/coaching/transformers.go:newDBScorecardTemplateFromProto` currently stores the original incoming `templateJSON` after typed parsing and validation. Materialization therefore requires a new normalization step before persistence. An incoming Commit that omits a scope after the prior revision contains narrower criteria is a separate unresolved compatibility issue: treating that omission as full template scope could accidentally broaden access to criteria. The next discussion should decide whether to reject that write or recover prior scope by stable criterion identifiers.

Xuanyu confirmed that an inapplicable criterion has no score row, not an N/A row. `shared/scoring/scorecard_calculator.go:ComputeScores` currently skips template criteria with no score rows before updating the weighted summary, so this choice excludes them from the aggregate denominator. `mapScoresByCriterion` and `GetValidScorecardScores` currently validate against criterion existence in the full template without a use-case applicability check; a new server-side check must prevent clients from supplying scores for excluded criteria.

The next validity question is whether a template may apply to a use case with zero applicable scoreable criteria. If yes, the scorecard can have no QA aggregate; if no, template writes need to reject that configuration. Define whether informational/non-scored criteria count.
