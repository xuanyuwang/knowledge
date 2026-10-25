# CONVI-7777 requirements and investigation

## Context
- Date: 2026-10-05
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`; branch/worktree: `main`, main checkout (read-only investigation).
- Related references: Director and cresta-proto main checkouts.
- Ticket: https://linear.app/cresta/issue/CONVI-7777
- No product code changes authorized at this stage; investigate and draft implementation.

## Confirmed requirements
- Conversation-level percentage, not pooled followed/sent rate.
- New column in `data-testid="assistance-used-leaderboard"`, standard filters.
- Hint merely sent does not qualify; sent and followed does. Other feature-specific used definitions require investigation.
- Backend API preferred (new or extended).
- Investigate ALL feature categories using code and Glean; ticket inventory is provisional. Smart Compose/Suggestions included only if conversation-linked; voice and chat both count. Summary/Notes meanings not yet known.
- Explain HintStats versus AssistanceStats.

## Provisional/open
- Resolved in follow-up: user confirmed all filtered conversations, including those with no assistance offered, and both Agent/Team tabs.
- Need per-feature use predicates, conversation linkage, agent/team attribution and dedup semantics.

## Evidence
- Linear description and no comments fetched in initial exchange.
- User clarification in current conversation supersedes pooled-rate example.

## Investigation outcome
- [Detailed evidence and provisional implementation draft](../../deliverables/convi-7777-conversation-assistance-usage-draft.md).
- Traced target table, split API composition, existing overall-engagement/powered-by-AA columns, per-feature ClickHouse predicates, conversation event proto/ingestion/projection and client-sdk Smart Compose producer via Glean.
- Glean confirms Summary copy, GenAI information conveyed, workflow open semantics; Notes public docs conflict with current copy-only query/migration doc.
- Suggestions can be conversation-linked. Smart Compose producer also links conversations but emits zero-used-length records; projection currently drops payload and old stats approximates agent/hour.
- Found omitted page surfaces: Silence/Hold, Customer Snapshot, email suggestions; scope not assumed.
- Historical knowledge warning on hint overcount is outdated for inspected main checkout: distinct action IDs are now used. No deployed-version claim.
- Recommended distinct conversation union in backend, preferably new API, common filtered/authorized denominator; do not combine aggregate percentages or repurpose online-session AA labels.
- Open: denominator confirmation, feature/channel scope, agent/team attribution, history/coverage, payload/enum validation. No production data queries, test runs, or product code changes.
- Source checkpoints and exact paths/URLs are recorded in the draft.

## Follow-up clarification
- User confirmed assistance (not hint-only) scope, all-filtered denominator and both tabs. Inventory agreed in principle; Notes/Smart Compose docs requested before final predicates. `usedTextLength > 0` remains pending. Customer Snapshot/email out of v1.
- Glean retrieved Atomic Voice Application Automatic note-taking docs: captured entities, click to copy, double-click to edit. Smart Compose doc explains inline autocomplete and Right Arrow/Tab acceptance. Links recorded in draft.
- Silence/Hold has tracked behavior adherence, unlike Reminder. Verified FE adapter uses policy-level hint configuration and generic adherence counts; actual per-occurrence sent/followed proof remains unresolved. Hold doc example: hint at 90 seconds ahead of 2-minute missed-behavior threshold.
- Email partial/full Suggestions have conversation-scoped events and current conversation-event analytics queries; technically includable later, not in v1. No data coverage validation or product changes.

## Latest scope revision
User superseded the email-v1 exclusion: email assistance should be considered if current stats APIs include it already. Partial/full email Suggestions are confirmed in SuggestionStats and enter the proposed inventory; other email assistance (including email Smart Compose) needs explicit API coverage verification. Conversation linkage and feature-specific qualifying use remain mandatory.
