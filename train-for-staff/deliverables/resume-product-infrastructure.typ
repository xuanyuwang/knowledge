// Targeted resume: Senior Product Infrastructure / data-intensive backend roles
#import "@preview/acorn-resume:0.1.0": *
#import "@preview/fontawesome:0.6.0": fa-icon

#let name = "Xuanyu Wang"
#let email = "wang.xuanyu@icloud.com"
#let linkedin-url = "https://www.linkedin.com/in/xuanyu-wang"
#let website-url = "https://www.xuanyuwang.com/blog"
#let linkedin-handle = "xuanyu-wang"

#let fa-contact-label(icon-name, handle) = [
  #box(baseline: 0.2em, fa-icon(icon-name, size: 0.95em))
  #h(0.22em)
  #handle
]

#set par(justify: false)

#show: resume.with(
  author: name,
  margin: (x: 1.3cm, y: 1.1cm),
  font: "Arial",
  font-size: 10pt,
  link-style: (underline: false, color: black),
)

#header(
  name: name,
  contacts: (
    ("mailto:" + email, email),
    (website-url, "xuanyuwang.com"),
    (linkedin-url, fa-contact-label("linkedin", linkedin-handle)),
  ),
)

== Summary
#pad(
  top: 0.15em,
  [
    Senior backend engineer with 7+ years building data-intensive product systems. \
    Diagnoses cross-system correctness failures, turns ambiguous semantics into explicit contracts, and owns solutions through architecture, safe rollout, and production verification.
  ],
)

== Experience
#exp(
  role: "Software Engineer",
  date: "Dec. 2023 - Present",
  organization: "Cresta - Conversation Intelligence (Coaching, QM, Analytics)",
  location: "Toronto, Canada",
  details: [
    - Designed and globally rolled out a general ClickHouse external-table path for large analytics filters; evaluated 10 approaches, integrated generic helpers across 17 caller files, and benchmarked 3.3× faster queries at 10,000 users.
    - Diagnosed PostgreSQL-to-ClickHouse scorecard inconsistencies caused by asynchronous stale writes and ORM lost updates; shipped atomic, post-commit re-read and partial-update fixes with zero score or submitter mismatches across 2,996 comparable production records over 39 days.
    - Built fleet-wide scorecard sync monitoring and safety-gated repair across seven production clusters, classifying missing and stale records and dispatching targeted ID-based reindex jobs with allowlists, thresholds, and workflow chunking.
    - Defined user-filter semantics across three divergent implementations; created an implementation-independent behavioral standard and 62-test suite, identified silent ACL and selection differences, and fixed a production union-versus-intersection defect.
    - Led backend architecture and delivery for Group Calibration, including task APIs, response workflows, consistency scoring, analytics, and notifications for end-to-end quality-management calibration.
  ],
)

#exp(
  role: "Software Engineer (Data)",
  date: "2021 - 2023",
  organization: "Boosted.ai - AI-driven investment platform",
  location: "Toronto, Canada",
  details: [
    - Designed and maintained ETL pipelines ingesting global financial market data across multiple exchanges, enabling daily analytics and model-driven investment workflows.
    - Built a high-performance data access service with FastAPI, PostgreSQL, and Solr, providing internal teams a unified interface for large financial datasets.
    - Developed gRPC microservices for user-data management and investment-signal generation, supporting personalization workflows.
    - Prototyped and advocated Prefect-based orchestration to improve pipeline reliability and operational visibility.
    - Standardized Docker-based development environments, reducing local setup from days to minutes.
  ],
)

#exp(
  role: "Software Developer (Frontend), Scrum Master",
  date: "2018 - 2021",
  organization: "IBM - Cognos Analytics",
  location: "Ottawa, Canada",
  details: [
    - Developed and shipped React and TypeScript features for a large-scale analytics platform.
    - Improved Crosstab rendering performance by simplifying DOM structure by more than 50%, improving responsiveness for large datasets.
  ],
)

== Skills
#pad(
  top: 0.15em,
  [
    *Backend:* Go, Python \
    *Data Systems:* PostgreSQL, ClickHouse \
    *Frontend:* React, TypeScript \
  ],
)
