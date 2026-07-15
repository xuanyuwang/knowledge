# Domain-Centered Repository Reorganization Plan

**Created:** 2026-07-14
**Status:** Proposed; domain scaffolding created, legacy migration not started
**Canonical model:** `workflow/domain-centered-knowledge-model.md`

## Target Outcome

Use four product domains as the stable knowledge navigation layer:

1. `analytics`
2. `scorecard-workflows`
3. `scorecard-data-sync`
4. `notifications`

Tickets become work items inside a primary domain. Standalone initiative projects remain only for multi-workstream or independently coordinated outcomes. Career, blog, weekly, general learning, oncall, and repository workflow surfaces remain separate system/promotion destinations.

This plan does not authorize a bulk move. Migration is synthesis-first and must preserve existing user changes and historical evidence.

## Target Topology

```text
analytics/
scorecard-workflows/
scorecard-data-sync/
notifications/
  project.yaml
  README.md
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
| `active-days` | Synthesize | Metric semantics and operational case under `analytics` |
| `agent-quintiles-support` | Synthesize | Cross-page ranking semantics; likely a work item plus metric catalog updates |
| `agent-stats-active-days-fix` | Synthesize | Active Days case and API/query semantics |
| `agent-stats-analytics-behaviors` | Synthesize first; seed | Strongest initial seed for API and conversation-count behavior |
| `bswift-wrong-team-mapping` | Synthesize | Identity/team mapping case; security-sensitive SQL evidence should be reviewed |
| `convi-6192-conversation-source-config` | Synthesize | Conversation-source filter semantics |
| `convi-6242-cron-label-conversations` | Synthesize | Label freshness and Active Days semantics/operations |
| `convi-6247-agent-only-filter` | Synthesize | FE/BE filter contract across analytics pages |
| `convi-6260-team-leaderboard` | Synthesize | Team grouping and hierarchy semantics |
| `convi-6494-raises-answered-zero` | Synthesize | Leaderboard metric edge case |
| `convi-6753-weight-zero-pi-na` | Synthesize | PI display/calculation edge case |
| `convi-6808-greenix-pi-scores` | Synthesize | PI score investigation/case |
| `convi-6842-holiday-inn-pi-vs-closed-conversations` | Synthesize | Cross-surface conversation-count semantics |
| `convi-6968-schwab-leaderboard-launch` | Synthesize | Leaderboard API strategy and rollout case |
| `convi-7049-clo-filter` | Synthesize | Filter semantics; confirm exact CLO meaning during migration |
| `convi-7162-holidayinn-manager-scorecards-completed` | Synthesize | PI manager-scorecard metric semantics |
| `convi-7230-performance-insights-outcome-filters` | Synthesize | PI outcome-filter FE/BE contract |
| `insights-user-filter` | Synthesize | User-filter architecture/reference; retain source details until canonical matrix exists |
| `large-user-id-clickhouse` | Synthesize or retain initiative | Infrastructure improvement used by analytics; decide after extracting behavioral contract |
| `qa-score-popover-fix` | Synthesize | FE chart/popover semantics and regression case |
| `user-filter-consolidation` | Retain initiative, link to Analytics | Independent migration/strategy project with durable analytics impact |
| `virtual-group-filter` | Security review, then synthesize | Do not read/copy credential-shaped files; migrate only safe investigation knowledge |

### Scorecard Workflows

| Existing folder | Disposition | Target/notes |
|---|---|---|
| `alo` | Retain initiative, link | ALOs in Coaching design; extract scorecard/template workflow implications |
| `auto-backfill-missing-scorecards` | Retain initiative, link | Primary question is scorecard generation; link repair/projection details to data sync |
| `convi-6672-achieve-behavior-na` | Synthesize | Score/behavior N/A semantics |
| `convi-6709-reversed-scorecard` | Synthesize | Reversal lifecycle and business rules |
| `convi-6862-disable-editing-on-submitted-scorecard` | Synthesize | Submission/permission work item and decision history |
| `convi-7237` | Synthesize | Scorecard access UI/permission presentation case |
| `duplicate-template-across-usecase` | Synthesize | Template duplication workflow and compatibility behavior |
| `export-appeal-comments` | Synthesize | Appeal/export semantics |
| `group-calibration` | Synthesize first; seed | Review-workflow architecture, behavior, and cases |
| `nascore` | Retain initiative, link | Independent feature effort; promote stable scoring/N/A semantics |
| `outcome` | Synthesize | Opera/AutoQM annotation-to-scorecard behavior |
| `scorecard-permission-policy` | Synthesize first; seed | Canonical permission/visibility design material |
| `scorecard-template` | Synthesize first; seed | Strongest lifecycle/concept-map source |
| `template-schema-version-updater` | Retain initiative, link | Independent architecture proposal; promote compatibility model |
| root `sessions/2026-06-19/claude-vivint-orphan-criterion.md` | Review, then move | Likely scorecard-workflow evidence; inspect references and choose a work item |

### Scorecard Data Sync

| Existing folder | Disposition | Target/notes |
|---|---|---|
| `backfill-scorecards` | Synthesize | Repair/backfill playbook and run evidence |
| `convi-5565-scorecard-ch-pg-sync` | Synthesize first; seed | Core race-condition, fix-attempt, and validation history |
| `convi-6298-reindex-process-scorecards` | Synthesize | Reindex architecture and operational workflow |
| `convi-6841-process-scorecard-update-race` | Synthesize | Read-after-write/replica-lag case; distinguish PG workflow from CH projection |
| `hilton-coaching-discrepancy` | Synthesize | Production mismatch investigation/case |
| `historic-scorecard-missing` | Safety review, then synthesize | Product Go code does not belong in `knowledge`; preserve investigation, relocate code appropriately |
| `pg-ch-scorecard-sync-investigation` | Synthesize first; seed | Highest-level theory, monitor, query, and reusable diagnosis material |

### Notifications

No current top-level folder is a complete Notifications domain seed. Build it by extraction rather than wholesale folder moves:

| Existing source | Disposition | Target/notes |
|---|---|---|
| `oncall` notification research | Extract and link | Preserve `oncall` as cross-domain operations; move stable notification architecture to `notifications` |
| `scorecard-permission-policy` | Cross-link | Extract recipient/visibility implications without duplicating permission truth |
| scorecard/group-calibration/appeal ticket notes | Extract as discovered | Add triggers and delivery paths to the notification catalog |

### Retain Outside the Four Product Domains

| Existing folder | Disposition | Reason/next action |
|---|---|---|
| `2025-annual-review` | Retain system/promotion | Historical performance-review evidence; consider a future archive under `train-for-staff` |
| `blog` | Retain system/promotion | Publication destination |
| `coaching-session-generation` | Review with `multi-agent-coaching-assistant` | Prototype/hackathon material outside the initial product domains |
| `convi-6665-deactivated-coaching-plan-visibility` | Retain legacy/other coaching | Coaching-plan behavior is not automatically scorecard workflow; link only where scorecard semantics apply |
| `dev-environment-tips` | Retain or merge into `general-learnings` | Utility knowledge, not a product domain |
| `general-learnings` | Retain system/promotion | Cross-domain learning destination |
| `multi-agent-coaching-assistant` | Retain initiative | Independent prototype/outcome |
| `oncall` | Retain system/cross-domain | Incident intake and runbook surface; domain lessons should be promoted outward |
| `productivity-with-ai` | Review, likely retain career analysis | Contains analysis code/data outside product domains |
| `team-enable` | Retain initiative | Independent AI enablement outcome |
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

1. Use `agent-stats-analytics-behaviors` as the first seed.
2. Build the Performance Insights and Leaderboard surface/API/filter inventory.
3. Migrate two representative cases: one shared API/metric case and one FE-specific interpretation case.
4. Test the work-item → daily log → weekly summary promotion chain.
5. Review whether the domain structure can express exact chart/table semantics without ambiguity.

**Exit gate:** a displayed value can be traced from FE semantics to request, BE calculation, and source data; two legacy folders have reviewed pointers.

### Phase 3: Scorecard Workflows

1. Synthesize `scorecard-template`, `scorecard-permission-policy`, and `group-calibration` as seeds.
2. Define lifecycle/state, type, permission, scoring, appeal, and calibration maps.
3. Migrate active ticket state into work items before historical cases.
4. Cross-link notification triggers and data-sync dependencies without duplicating their canonical details.

**Exit gate:** workflow behavior and boundaries are understandable without reading ticket folders.

### Phase 4: Scorecard Data Sync

1. Synthesize `pg-ch-scorecard-sync-investigation` and CONVI-5565 first.
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
