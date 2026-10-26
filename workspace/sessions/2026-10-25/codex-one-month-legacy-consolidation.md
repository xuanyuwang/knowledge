# Consolidate legacy projects inactive for one month

- Date: October 25, 2026, America/Toronto.
- Primary project: workspace; affected domains: Analytics, Scorecard Workflows, Scorecard Data Sync; system home: Shared AI Skills.
- Source repo: /Users/xuanyu.wang/repos/knowledge.
- Branch/worktree: main, existing checkout.
- Canonical task: [inactive legacy consolidation](../../work-items/inactive-legacy-project-consolidation.md).
- Authorization: the user requested removing more inactive legacy projects into their domains, defining inactive as no meaningful content for one month. Their prior instruction to remove old folders after verifying links still applies.

## Selection and interpretation

The cutoff is September 25, 2026. Changes only to links, navigation, or file layout do not make a project active. Git history is checked against the content's explicit dates and dated sessions/logs; October 4 bulk commits contain September 19/21/22 substantive updates for three selected roots. The September 29 parser design keeps user-filter-consolidation active. Current canonical domains, system/promotion destinations, and independent coaching prototypes without a suitable domain are retained.

The selection is [machine-readable](one-month-legacy-consolidation/selection.json); [baseline hashes](one-month-legacy-consolidation/baseline.json) preserve the pre-migration files, including earlier uncommitted navigation edits. Legacy metadata is added from the project template where missing before evidence is moved. Current official tickets, PRs, deployments, and customer state are not rechecked during this document migration.

## Work and verification

Domain syntheses, preservation manifest, reference rewrites, sanitization, ignored-runtime relocation, and final check results are recorded alongside this session. Original diagnostic source and scripts are archived as historical evidence and never executed. Six ignored nascore draft notes are preserved as supplemental notes rather than discarded. Python environments/cache directories are preserved outside the repository; they are not knowledge artifacts.

Four generic credential-URI warnings were found in two documents. Their connection values were removed from the archived copies and the exception to exact-byte preservation recorded; no value is printed or used to connect. Known credential-bearing virtual-group files are not read or moved.

Credentials used: none. Local file/Git/scanner operations only. No stage, commit, push, or external mutation.

## Final outcome

Removed all 36 selected roots after preservation and reference checks. The root-level Vivint session now has its own Scorecard Workflows work item and parent-domain session home, leaving the former sessions directory empty and removable. The existing first five-project migration remains uncommitted and preserved.

The domain synthesis reports and work items retain unresolved historical questions rather than presenting them as current deployed behavior. Domain READMEs, project metadata, legacy indexes, subdomain navigation, workspace migration plan and daily logs now point to their canonical homes. Subdomain identifiers remain unchanged. Historic mis-relative links were repaired against existing files, including two Director source links whose line anchors retain historical meaning.

[Final verification](one-month-legacy-consolidation/verification.json) aggregates the fresh preservation, link, YAML, whitespace, secret-scan, directory-removal and index checks. All 648 preserved files match their recorded final hashes and modes. Reversing the declared link changes reproduces every unsanitized original hash; the two sanitized documents differ only at declared credential substitutions and link changes. The final secret scan uses static detection only. No live credential validation or network authentication was performed.

No stage, commit or push. Retained exceptions and their reasons are in selection.json. This migration is complete; live source/customer follow-up remains the responsibility of the individual work items if reopened.
