Weekly Summary: Mar 2–8, 2026

Progress

- **Multi-agent coaching assistant (hackathon):** Built end-to-end from codebase exploration to deployed app in 3 days (Mar 2–4). Explored `cresta-assistant` codebase, designed team coaching architecture (identifier → scorer → data fetcher), implemented prototype with 15 passing tests, then rewrote as vibe-coded Flask app (`coaching-ai-summary`) in `chat-ai` repo. Deployed to voice-staging with 5 endpoints: `/api/identify` (agent priority ranking), `/api/recommend` (LLM coaching recommendations via gpt-4o structured output), `/api/recommend-direct` (combined), `/api/create-session` (OpenAI-generated meeting notes → gRPC `CreateCoachingSession`), `/api/users`. Added demo preset (synthetic data, all 4 priority levels), snapfinance preset (live prod data), context switcher UI, CORS support, header-based auth for Director integration, agent display name resolution, criterion display name resolution (V1+V2 template formats), and mock data for 8 hackathon agents. Registered in tenant-admin (PR #2764). Deployed 23 revisions to voice-staging, targeted to `cresta-sandbox-2`/`voice-sandbox-2` for demo.
- **Process scorecard backfill (CONVI-6298):** Created `backfill_process_scorecards.py` for one-time PG→CH backfill of process scorecards (template type=2). Spirit completed: 2,120 scorecards + 30,464 scores. Validated Python scoring logic against `historic.scorecard_scores` — 1,601/1,784 match (100% after accounting for N/A and exclude_from_qa edge cases, 0 unexplained mismatches). Refactored script to compute scores on-the-fly from `director.scores` + template revision, eliminating `historic.scorecard_scores` dependency. Oportun completed: 43,953 scorecards + 501,456 scores in 44 batches.
- **Reindex process scorecards Go workflow (PR #25916):** Rewrote `activity.go` incorporating all optimizations discovered during Python backfill. Key changes: `reindexConfig` struct for centralized config parsing, batch-fetch director scores (1 query per batch instead of N+1), CH deletion strategy reduced from 2N mutations per batch to 2 total mutations using `DELETE ON CLUSTER conversation`, template revision caching, customer/profile added to all queries to use indexes. Updated cresta-proto v2.0.632→v2.0.634 for `ScorecardTemplateNames` field. Documented consistency limitations.

Problems

- **Criterion display names showing UUIDs:** `RetrieveCoachingProgresses` returns empty `criterion_display_name`. Fixed by fetching `ListCurrentScorecardTemplates` and parsing template JSON (both V1 `criteria` array and V2 nested `items` with chapters).
- **Mock data issues for hackathon demo:** Focus criteria were hardcoded separately from assessments (inconsistent), agent detection didn't support full resource names, missing `recognize_improvement` action type. All fixed by Mar 5.
- **CH verification limits on large datasets:** Oportun's 43K UUIDs in `IN` clause exceeded CH max query size (1MB), and batching with `FINAL` hit 10GB memory limit. Fixed by filtering on `customer_id`/`profile_id`/`conversation_id=''` instead.
- **Deployment friction:** Multiple issues resolved during vibe-code deployment: `gsed` not found, `python` shim missing (pyenv), Docker buildx missing, colima not running, `coaching_assistant_pb2` not in container (pinned crestaproto too old), port 5000/5001 conflict.
- **N+1 and full table scan in Go workflow:** Backfill experience revealed `scorecardQuery` missing customer/profile (causes full table scan), JOIN on `scorecard_template_revisions` causing duplicates (composite PK includes revision), and per-scorecard template fetch (N+1). All addressed in activity.go rewrite.

Plan

- **PR #25916:** Get code review, address feedback, merge reindex process scorecards workflow.
- **Hackathon follow-up:** Demo coaching-ai-summary at hackathon presentation. Evaluate if any features should move to production (python-ai-services proper gRPC service).
- **Backfill remaining customers:** If PR #25916 merges, use the Temporal workflow for remaining clusters beyond Spirit and Oportun.

