# Xuanyu Wang

Toronto, Canada · [wang.xuanyu@icloud.com](mailto:wang.xuanyu@icloud.com) · [xuanyuwang.com](https://www.xuanyuwang.com/blog) · [LinkedIn](https://www.linkedin.com/in/xuanyu-wang)

## Senior Software Engineer · Backend & Product Infrastructure

Senior software engineer with 7+ years of experience building backend, data-intensive, and analytics products. I take technical ownership of ambiguous product problems: clarifying what needs to be correct, investigating across system boundaries, weighing implementation and operational tradeoffs, and carrying solutions through production validation.

## Experience

### Cresta — Software Engineer
*Toronto, Canada · December 2023–Present*

- **Built and rolled out a reusable ClickHouse external-table path** after large analytics filters exceeded query-size limits. Evaluated 10 approaches, implemented generic helpers across 17 caller files, and measured 3.3× faster query performance than a large `IN` clause at 10,000 users.
- **Diagnosed scorecard consistency failures across PostgreSQL and ClickHouse.** Built concurrency and cross-store verification tools, drove atomic-write, post-commit read, and guarded partial-update changes through a feature-flagged rollout, and verified zero score or submitter mismatches among 2,996 comparable production records over 39 days.
- **Designed fleet-wide scorecard sync monitoring and bounded repair** across seven production clusters, distinguishing missing and stale records and using targeted reindex jobs with allowlists, thresholds, and chunking to control operational risk.
- **Owned backend architecture and delivery for Group Calibration**, building the task, answer-key and response workflows, consistency scoring, analytics, and notification paths that support end-to-end quality calibration sessions.
- **Made user-filter behavior explicit across divergent implementations** by defining a shared behavioral standard and 62 behavioral tests; identified semantic differences and fixed a production union-versus-intersection defect.

### Boosted.ai — Software Engineer, Data
*Toronto, Canada · 2021–2023*

- Designed and maintained ETL pipelines ingesting global financial-market data across exchanges for analytics and investment workflows.
- Built a FastAPI, PostgreSQL, and Solr data-access service that gave internal teams a unified interface to large financial datasets.
- Developed gRPC services for user-data management and investment-signal generation.
- Prototyped Prefect-based orchestration to improve pipeline reliability and operational visibility.
- Standardized Docker development environments, reducing local setup from days to minutes.

### IBM, Cognos Analytics — Software Developer, Frontend; Scrum Master
*Ottawa, Canada · 2018–2021*

- Developed React and TypeScript features for a large-scale analytics platform.
- Improved Crosstab rendering performance by simplifying the DOM structure by more than 50%, improving responsiveness for large datasets.

## Technical Skills

**Languages:** Go, Python, TypeScript, JavaScript

**Backend and data:** PostgreSQL, ClickHouse, FastAPI, gRPC, ETL

**Frontend:** React

**Infrastructure and delivery:** Docker, feature-flagged rollouts, data validation and production diagnosis

---

## Draft notes (remove before sending)

- This is an evidence-based first draft, not the final selected-project portfolio. It deliberately does not list every feature. The aim is to keep projects that show complementary strengths: reusable infrastructure, cross-system investigation, production reliability, product workflow ownership, and semantic correctness.
- The Group Calibration bullet reflects the existing resume and project records. The user's broader set of end-to-end workflow launches is not yet captured here; review those features before deciding which examples are strongest and whether they should replace or supplement current bullets.
- Older-company bullets and titles are carried forward from the existing resume and have not been revalidated in this pass. No education section was available in the reviewed resume files.
- The comparative data point is a benchmark at 10,000 users, not a production latency claim. The cross-store verification denominator is 2,996 comparable records, not all inspected PostgreSQL records.
