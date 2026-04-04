// Typst resume
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

#show: resume.with(
  author: name,
  margin: (x: 1.5cm, y: 1.5cm),
  font: "Arial",
  font-size: 11pt,
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
    Senior Software Engineer with 7+ years of experience across backend systems, data platforms, and frontend applications. \
    Specialized in distributed data systems, data consistency, and high-scale analytics infrastructure. \
    Strong track record of diagnosing complex production issues, driving architectural fixes, and improving system reliability across teams.
  ],
)

== Experience
#exp(
  role: "Software Engineer",
  date: "Dec. 2023 — Present",
  organization: "Cresta — Conversation Intelligence (Coaching, QM, Analytics)",
  location: "Toronto, Canada",
  details: [
    - Standardized user-filter semantics across 30+ analytics APIs and three independent implementations; identified five silent logic divergences (including union vs. intersection), and led full migration to a canonical path (12/29 → 29/29) using feature flags and shadow-mode validation (10,000+ queries, 0 mismatches).
    - Designed and rolled out ClickHouse external table strategy for large-scale filtering, eliminating massive `IN` clause expansion; evaluated 10 approaches, benchmarked performance (~4.8× improvement at scale), and deployed across 19+ production call sites.
    - Diagnosed and resolved cross-system data inconsistencies between PostgreSQL and ClickHouse caused by async execution races and ORM full-struct overwrites; designed and shipped a multi-layer solution (atomic transactions, async re-read after commit, partial updates) validated via load testing and production verification.
    - Owned backend architecture and delivery for Group Calibration, including task CRUD, consistency scoring, analytics APIs, and notification workflows, enabling end-to-end QM calibration processes.
    - Delivered multiple high-impact backend features for enterprise customers, including Agent-on-Call (assignment flows, manager whisper, live assist annotations) and critical fixes across coaching and analytics systems.
  ],
)

#exp(
  role: "Software Engineer (Data)",
  date: "2021 — 2023",
  organization: "Boosted.ai — AI-driven investment platform",
  location: "Toronto, Canada",
  details: [
    - Designed and maintained ETL pipelines ingesting global financial market data across multiple exchanges, enabling daily analytics and model-driven investment workflows.
    - Built a high-performance data access service (FastAPI + PostgreSQL + Solr) with multiple semantic layers, providing a unified and flexible interface for internal teams to query large-scale financial datasets.
    - Developed backend microservices (gRPC) for user data management and investment signal generation, supporting scalable personalization workflows.
    - Identified bottlenecks in data orchestration workflows; prototyped and advocated adoption of Prefect-based orchestration, improving pipeline reliability and operational visibility.
    - Reduced local development setup time from days to minutes by standardizing Docker-based environments across teams, improving onboarding efficiency and developer productivity.
  ],
)

#exp(
  role: "Software Developer (Frontend), Scrum Master",
  date: "2018 — 2021",
  organization: "IBM — Cognos Analytics",
  location: "Ottawa, Canada",
  details: [
    - Developed and shipped frontend features in a large-scale analytics platform (React, TypeScript)
    - Improved Crosstab rendering performance by simplifying DOM structure (>50% reduction), significantly improving responsiveness for large datasets.
  ],
)

== Skills
#pad(
  top: 0.15em,
  [
    *Backend:* Go, Python \
    *Data Systems:* PostgreSQL, ClickHouse \
    *Frontend:* React \
  ],
)
