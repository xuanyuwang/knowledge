# Domain-Centered Repository Reorganization Plan

**Created:** 2026-07-14
**Status:** Initial migration complete; all four domain foundations migrated on 2026-07-15
**Canonical model:** `workflow/domain-centered-knowledge-model.md`

## Target Outcome

Use four product domains as the stable knowledge navigation layer:

1. `analytics`
2. `scorecard-workflows`
3. `scorecard-data-sync`
4. `notifications`

Tickets become work items inside a primary domain. Standalone initiative projects remain only for multi-workstream or independently coordinated outcomes. Career, blog, weekly, general learning, oncall, and repository workflow surfaces remain separate system/promotion destinations.

This plan does not authorize a bulk move. After the initial domain foundations, migrate legacy projects gradually when they are reopened or needed by active domain work. Migration is synthesis-first and must preserve existing user changes and historical evidence.

## Migration Progress

- **October 25, 2026, follow-up:** 36 additional projects consolidated using a September 25 substantive-content cutoff. [Selection, canonical destinations, exceptions and verification](../work-items/inactive-legacy-project-consolidation.md). Historical evidence remains in each domain’s dated sessions; old top-level folders are retired after checks.

- **October 25, 2026:** Five inactive Analytics projects (CONVI-6192, 6242, 6260, 6494, 6753) consolidated into canonical work items, existing subdomain references, and preserved dated session evidence. The user approved retiring their old top-level folders after preservation and link checks. See the [migration record](../../analytics/sessions/2026-10-25/codex-five-legacy-project-consolidation.md).

- **Scorecard Data Sync:** Canonical architecture, invariants, failure taxonomy, diagnosis, repair/backfill, legacy index, and CONVI-7186 work item created. Source projects retain evidence with migration pointers.
- **Notifications:** Canonical routing architecture, catalog, recipient semantics, operations playbook, and legacy index created. `oncall` and scorecard permission projects remain canonical for their own concerns and link to the domain synthesis.
- **Boundary correction:** CONVI-6841 moved to Scorecard Workflows; Hilton coaching discrepancy retained as coaching analytics evidence rather than being forced into Scorecard Data Sync.
- **Analytics:** Eight subdomains now cover the shared platform, product surfaces, Active Days, Quintiles, QA Score, user filtering, and conversation volume. Legacy projects retain evidence and point to the canonical references.
- **Scorecard Workflows:** Seven subdomains now cover authoring/versioning, evaluation/scoring, lifecycle, permissions/visibility, appeals, group calibration, and process-scorecard generation. Legacy projects retain evidence and point to the canonical references.
- **Initial foundation:** All four planned domains now have useful canonical navigation. Semantic expansion and selective legacy closure continue incrementally as tickets validate behavior.

## Target Topology

```text
analytics/
scorecard-workflows/
scorecard-data-sync/
notifications/
  project.yaml
  README.md
  subdomains/            # optional for broad domain families
  work-items/
  log/
  sessions/
  decisions/
  deliverables/

train-for-staff/       # career/performance synthesis
weekly-summary/        # weekly promotion
general-learnings/     # reusable non-domain learning
blog/                  # publication candidates
oncall/                # cross-domain incident operating model
workspace/             # repository/workspace system project
workflow/              # canonical model, templates, shared skills
templates/             # artifact templates
```

## Disposition Vocabulary

- **Synthesize**: extract durable knowledge into the destination domain, create work-item/case links, then replace the legacy README with a thin pointer after review.
- **Retain initiative**: keep a standalone project because it has an independent outcome or falls outside the initial domains; add explicit domain links.
- **Retain system/promotion**: keep as a shared workflow, career, publication, or cross-domain surface.
- **Review**: inspect before deciding; do not move automatically.
- **Remove after verification**: delete only after confirming the folder is empty/unreferenced and the human approves.

## Whole-Repo Inventory

### Analytics

| Existing folder | Disposition | Target/notes |
|---|---|---|
| [med-mutual-active-days-and-behavior-guides](../../analytics/work-items/med-mutual-active-days-and-behavior-guides.md) | Consolidated 2026-10-25 | Metric semantics and operational case under `analytics` |
| [agent-quintiles-and-directionality](../../analytics/work-items/agent-quintiles-and-directionality.md) | Consolidated 2026-10-25 | Cross-page ranking semantics; likely a work item plus metric catalog updates |
| [active-days-join-and-stale-label-diagnostics](../../analytics/work-items/active-days-join-and-stale-label-diagnostics.md) | Consolidated 2026-10-25 | Active Days case and API/query semantics |
| [agent-stats-and-conversation-count-contracts](../../analytics/work-items/agent-stats-and-conversation-count-contracts.md) | Consolidated 2026-10-25 | Strongest initial seed for API and conversation-count behavior |
| [bswift-duplicate-user-team-attribution](../../analytics/work-items/bswift-duplicate-user-team-attribution.md) | Consolidated 2026-10-25 | Identity/team mapping case; security-sensitive SQL evidence should be reviewed |
| [CONVI-6192](../../analytics/work-items/CONVI-6192.md) | Consolidated 2026-10-25 | Conversation-source filter semantics |
| [CONVI-6242](../../analytics/work-items/CONVI-6242.md) | Consolidated 2026-10-25 | Label freshness and Active Days semantics/operations |
| [CONVI-6247](../../analytics/work-items/CONVI-6247.md) | Consolidated 2026-10-25 | FE/BE filter contract across analytics pages |
| [CONVI-6260](../../analytics/work-items/CONVI-6260.md) | Consolidated 2026-10-25 | Team grouping and hierarchy semantics |
| [CONVI-6494](../../analytics/work-items/CONVI-6494.md) | Consolidated 2026-10-25 | Leaderboard metric edge case |
| [CONVI-6753](../../analytics/work-items/CONVI-6753.md) | Consolidated 2026-10-25 | PI display/calculation edge case |
| [CONVI-6808](../../analytics/work-items/CONVI-6808.md) | Consolidated 2026-10-25 | PI score investigation/case |
| [CONVI-6842](../../analytics/work-items/CONVI-6842.md) | Consolidated 2026-10-25 | Cross-surface conversation-count semantics |
| [CONVI-6968](../../analytics/work-items/CONVI-6968.md) | Consolidated 2026-10-25 | Leaderboard API strategy and rollout case |
| [clo-filter-query-optimization-and-rollout](../../analytics/work-items/clo-filter-query-optimization-and-rollout.md) | Consolidated 2026-10-25 | Filter semantics; confirm exact CLO meaning during migration |
| [CONVI-7162](../../analytics/work-items/CONVI-7162.md) | Consolidated 2026-10-25 | PI manager-scorecard metric semantics |
| [CONVI-7230](../../analytics/work-items/CONVI-7230.md) | Consolidated 2026-10-25 | PI outcome-filter FE/BE contract |
| [insights-user-filter-historical-contracts](../../analytics/work-items/insights-user-filter-historical-contracts.md) | Consolidated 2026-10-25 | User-filter architecture/reference; retain source details until canonical matrix exists |
| [large-user-filter-external-tables](../../analytics/work-items/large-user-filter-external-tables.md) | Consolidated 2026-10-25 | Infrastructure improvement used by analytics; decide after extracting behavioral contract |
| [CONVI-5805](../../analytics/work-items/CONVI-5805.md) | Consolidated 2026-10-25 | FE chart/popover semantics and regression case |
| `user-filter-consolidation` | Retain initiative, link to Analytics | Independent migration/strategy project with durable analytics impact |
| `virtual-group-filter` | Security review, then synthesize | Do not read/copy credential-shaped files; migrate only safe investigation knowledge |

### Scorecard Workflows

| Existing folder | Disposition | Target/notes |
|---|---|---|
| `alo` | Retain initiative, link | ALOs in Coaching design; extract scorecard/template workflow implications |
| [opera-dependent-scorecard-generation](../../scorecard-workflows/work-items/opera-dependent-scorecard-generation.md) | Consolidated 2026-10-25 | Primary question is scorecard generation; link repair/projection details to data sync |
| [achieve-excluded-criteria-na-investigation](../../scorecard-workflows/work-items/achieve-excluded-criteria-na-investigation.md) | Consolidated 2026-10-25 | Score/behavior N/A semantics |
| [nrg-option-value-display-mismatch](../../scorecard-workflows/work-items/nrg-option-value-display-mismatch.md) | Consolidated 2026-10-25 | Reversal lifecycle and business rules |
| [process-scorecard-create-autosave-race](../../scorecard-workflows/work-items/process-scorecard-create-autosave-race.md) | Consolidated 2026-10-25 | PostgreSQL read-after-create/replica consistency is workflow behavior, not a CH projection failure |
| [submitted-scorecard-permission-history](../../scorecard-workflows/work-items/submitted-scorecard-permission-history.md) | Consolidated 2026-10-25 | Submission/permission work item and decision history |
| [scorecard-access-advanced-layout](../../scorecard-workflows/work-items/scorecard-access-advanced-layout.md) | Consolidated 2026-10-25 | Scorecard access UI/permission presentation case |
| [cross-usecase-template-duplication](../../scorecard-workflows/work-items/cross-usecase-template-duplication.md) | Consolidated 2026-10-25 | Template duplication workflow and compatibility behavior |
| [appeal-resolution-comments-export](../../scorecard-workflows/work-items/appeal-resolution-comments-export.md) | Consolidated 2026-10-25 | Appeal/export semantics |
| [group-calibration-csv-comment-parity](../../scorecard-workflows/work-items/group-calibration-csv-comment-parity.md) | Consolidated 2026-10-25 | Review-workflow architecture, behavior, and cases |
| [na-score-representation-and-fallback](../../scorecard-workflows/work-items/na-score-representation-and-fallback.md) | Consolidated 2026-10-25 | Independent feature effort; promote stable scoring/N/A semantics |
| [opera-email-autoqa-outcome-semantics](../../scorecard-workflows/work-items/opera-email-autoqa-outcome-semantics.md) | Consolidated 2026-10-25 | Opera/AutoQM annotation-to-scorecard behavior |
| [scorecard-permission-evaluator-design-history](../../scorecard-workflows/work-items/scorecard-permission-evaluator-design-history.md) | Consolidated 2026-10-25 | Canonical permission/visibility design material |
| [scorecard-template-historical-reference](../../scorecard-workflows/work-items/scorecard-template-historical-reference.md) | Consolidated 2026-10-25 | Strongest lifecycle/concept-map source |
| [template-schema-updater-proposal](../../scorecard-workflows/work-items/template-schema-updater-proposal.md) | Consolidated 2026-10-25 | Independent architecture proposal; promote compatibility model |
| [Vivint orphan criterion](../../scorecard-workflows/work-items/vivint-orphan-criterion-2026-06.md) | Consolidated 2026-10-25 | Session preserved at the Scorecard Workflows parent domain |

### Scorecard Data Sync

| Existing folder | Disposition | Target/notes |
|---|---|---|
| [CONVI-6209](../../scorecard-data-sync/work-items/CONVI-6209.md) | Consolidated 2026-10-25 | Repair/backfill playbook and run evidence |
| [CONVI-5565](../../scorecard-data-sync/work-items/CONVI-5565.md) | Consolidated 2026-10-25 | Core race-condition, fix-attempt, and validation history |
| [CONVI-6298](../../scorecard-data-sync/work-items/CONVI-6298.md) | Consolidated 2026-10-25 | Reindex architecture and operational workflow |
| `historic-scorecard-missing` | Safety review, then synthesize | Product Go code does not belong in `knowledge`; preserve investigation, relocate code appropriately |
| [ui-pg-ch-ordering](../../scorecard-data-sync/work-items/ui-pg-ch-ordering.md) | Consolidated 2026-10-25 | Highest-level theory, monitor, query, and reusable diagnosis material |

### Notifications

No current top-level folder is a complete Notifications domain seed. Build it by extraction rather than wholesale folder moves:

| Existing source | Disposition | Target/notes |
|---|---|---|
| `oncall` notification research | Extract and link | Preserve `oncall` as cross-domain operations; move stable notification architecture to `notifications` |
| [scorecard-permission-evaluator-design-history](../../scorecard-workflows/work-items/scorecard-permission-evaluator-design-history.md) | Consolidated 2026-10-25 | Extract recipient/visibility implications without duplicating permission truth |
| scorecard/group-calibration/appeal ticket notes | Extract as discovered | Add triggers and delivery paths to the notification catalog |

### Retain Outside the Four Product Domains

| Existing folder | Disposition | Reason/next action |
|---|---|---|
| `2025-annual-review` | Retain system/promotion | Historical performance-review evidence; consider a future archive under `train-for-staff` |
| `blog` | Retain system/promotion | Publication destination |
| `coaching-session-generation` | Review with `multi-agent-coaching-assistant` | Prototype/hackathon material outside the initial product domains |
| `convi-6665-deactivated-coaching-plan-visibility` | Retain legacy/other coaching | Coaching-plan behavior is not automatically scorecard workflow; link only where scorecard semantics apply |
| [hilton-coaching-report-leaderboard-parity](../../analytics/work-items/hilton-coaching-report-leaderboard-parity.md) | Consolidated 2026-10-25 | Coaching session/efficiency and filtering behavior, not scorecard PG/CH projection |
| `dev-environment-tips` | Retain or merge into `general-learnings` | Utility knowledge, not a product domain |
| `general-learnings` | Retain system/promotion | Cross-domain learning destination |
| `multi-agent-coaching-assistant` | Retain initiative | Independent prototype/outcome |
| `oncall` | Retain system/cross-domain | Incident intake and runbook surface; domain lessons should be promoted outward |
| `productivity-with-ai` | Review, likely retain career analysis | Contains analysis code/data outside product domains |
| [team-feature-flag-cleanup-plugin](../../ai-skills/work-items/team-feature-flag-cleanup-plugin.md) | Consolidated 2026-10-25 | Independent AI enablement outcome |
| `templates` | Retain system | Canonical artifact templates |
| `train-for-staff` | Retain system/promotion | Annual evidence, Staff growth, resume synthesis |
| `weekly-summary` | Retain system/promotion | Weekly evidence aggregation |
| `workflow` | Retain system | Canonical operating model and shared skills |
| `workspace` | Retain system | Repo/worktree registry and workflow-change history |

### Cleanup and Review Queue

| Existing folder/file | Disposition | Safety requirement |
|---|---|---|
| `clean-deprecated-user-fetcher` | Remove after verification | Appears empty; confirm no ignored/untracked content or references |
| `qa-metadata-check-bug` | Remove after verification | Appears empty; confirm no ignored/untracked content or references |
| `sessions` (root orphan folder) | Eliminate as an orphan convention after migration | Move each session to its primary domain first |
| `historic-scorecard-missing/*.go`, `go.mod`, `go.sum` | Relocate/review | Product/tool code belongs in a source repo or an explicitly approved archive, not the knowledge layer |
| `virtual-group-filter/bearer-token.txt` | Credential review | Do not copy or display; determine whether it contains a real credential, remove from Git history if needed, and rotate externally if exposed |
| `virtual-group-filter/db-connections.txt` | Secret/config review | Do not copy or display; sanitize or replace with non-secret references |
| cache/temp directories | Leave outside migration | Existing ignored tool caches are not knowledge projects |

## Migration Phases

### Phase 0: Baseline and Safety

1. Freeze taxonomy changes while the migration matrix is reviewed.
2. Preserve the current dirty worktree; do not mix or overwrite unrelated user changes.
3. Review credential-shaped tracked files without printing their contents. Remove/rotate only with explicit authorization.
4. Identify product code currently stored in `knowledge` and decide its correct source-repo destination.
5. Validate inbound links to every folder before changing paths.

**Exit gate:** safety decisions recorded; no secret or product-code issue is hidden inside a content move.

### Phase 1: Domain Foundations

For each domain, create the first useful artifacts rather than empty scaffolding:

- architecture/surface map;
- semantics or invariant catalog;
- source repo/component map;
- operational gaps and open questions;
- legacy-source index;
- initial active work items.

**Exit gate:** each domain README can orient a new investigation and names its canonical seed artifacts.

### Phase 2: Analytics Pilot

**Foundation status:** Complete on 2026-07-15. Eight subdomains and a legacy-source index were created; retained sources now have navigation pointers. Exact matrices remain incremental follow-up work.

1. Use [agent-stats-and-conversation-count-contracts](../../analytics/work-items/agent-stats-and-conversation-count-contracts.md) as the first seed.
2. Build the Performance Insights and Leaderboard surface/API/filter inventory.
3. Migrate two representative cases: one shared API/metric case and one FE-specific interpretation case.
4. Test the work-item → daily log → weekly summary promotion chain.
5. Review whether the domain structure can express exact chart/table semantics without ambiguity.

**Exit gate:** a displayed value can be traced from FE semantics to request, BE calculation, and source data; two legacy folders have reviewed pointers.

### Phase 3: Scorecard Workflows

**Foundation status:** Complete on 2026-07-15. Seven workflow subdomains and a legacy-source index were created; retained sources now have navigation pointers.

1. Synthesize [scorecard-template-historical-reference](../../scorecard-workflows/work-items/scorecard-template-historical-reference.md), [scorecard-permission-evaluator-design-history](../../scorecard-workflows/work-items/scorecard-permission-evaluator-design-history.md), and [group-calibration-csv-comment-parity](../../scorecard-workflows/work-items/group-calibration-csv-comment-parity.md) as seeds.
2. Define lifecycle/state, type, permission, scoring, appeal, and calibration maps.
3. Migrate active ticket state into work items before historical cases.
4. Cross-link notification triggers and data-sync dependencies without duplicating their canonical details.

**Exit gate:** workflow behavior and boundaries are understandable without reading ticket folders.

### Phase 4: Scorecard Data Sync

1. Synthesize [ui-pg-ch-ordering](../../scorecard-data-sync/work-items/ui-pg-ch-ordering.md) and CONVI-5565 first.
2. Establish architecture, invariants, failure taxonomy, monitor/query catalog, and repair playbook.
3. Incorporate reindex/backfill and incident evidence.
4. Resolve misplaced product-code artifacts.

**Exit gate:** an engineer can diagnose and classify a PG/CH mismatch using only canonical domain artifacts and linked evidence.

### Phase 5: Notifications

1. Inventory all triggers, recipients, channels, jobs, templates, and config.
2. Extract stable material from oncall and workflow projects.
3. Define visibility/recipient boundaries with scorecard permissions.
4. Build the first operational playbook and gap list.

**Exit gate:** every known notification has an owner, trigger, recipient rule, delivery path, and diagnostic starting point.

### Phase 6: Legacy Closure

For each synthesized folder:

1. Verify every durable claim has a canonical destination.
2. Preserve ticket/PR/query evidence links.
3. Replace the legacy README with a thin pointer containing its disposition and canonical links.
4. Keep historical files until a separate deletion review; avoid destructive bulk cleanup.
5. Update root navigation and `project.yaml` active work items.

**Exit gate:** no conflicting canonical truth and no broken inbound link.

### Phase 7: Operating Cadence

1. Run `daily-capture` automatically at substantial task completion.
2. Run `weekly-summary` at the chosen weekly cadence.
3. Promote significant weekly candidates into `train-for-staff/deliverables/performance-evidence-2026.md` monthly or quarterly.
4. Audit domain gaps and stale work items monthly.

## Per-Folder Migration Checklist

- [ ] Read the full legacy README/project state and identify all artifacts.
- [ ] Choose one primary domain and optional secondary links.
- [ ] Decide whether the current ticket/task needs a work item.
- [ ] Extract concepts, semantics, architecture, operations, decisions, and reusable cases.
- [ ] Link source evidence rather than copying large raw transcripts.
- [ ] Reconcile conflicting claims and record uncertainty.
- [ ] Validate links with `rg` before changing paths.
- [ ] Update the destination README/log and legacy pointer.
- [ ] Run `git diff --check` and review the scoped diff.
- [ ] Mark the migration matrix row complete only after human review.

## Success Measures

- New tickets default to one of the four domains or a justified initiative.
- Long-running tickets have one canonical work item and recoverable next action.
- Performance/Leaderboard values are traceable from FE semantics to BE/source data.
- Scorecard workflow and PG/CH synchronization knowledge have a clear boundary.
- Weekly summaries can be generated from daily evidence without reconstructing work from memory.
- Annual-review candidates retain role, impact, collaborators, and evidence.
- Root-level ticket folders stop growing.
- Legacy folders have reviewed pointers before any deletion is considered.
