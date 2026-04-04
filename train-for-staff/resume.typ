// Typst resume — update linkedin-url / handles if your public profile differs.
// Social icons: Font Awesome 7 desktop fonts (Free + Brands .otf from fontawesome.com/download).
#import "@preview/acorn-resume:0.1.0": *
#import "@preview/fontawesome:0.6.0": fa-icon

#let name = "Xuanyu Wang"
#let email = "wang.xuanyu@icloud.com"
#let linkedin-url = "https://www.linkedin.com/in/xuanyu-wang"
#let website-url = "https://www.xuanyuwang.com"

#let linkedin-handle = "xuanyu-wang"

// Inner content only — #header wraps each entry with link(url)[…].
#let fa-contact-label(icon-name, handle) = [
  #box(baseline: 0.2em, fa-icon(icon-name, size: 0.95em))
  #h(0.22em)
  #handle
]

#show: resume.with(
  author: name,
  margin: (
    x: 1.5cm,
    y: 1.5cm,
  ),
  // Arial ships on macOS; avoids missing Calibri substitution warnings.
  font: "Arial",
  font-size: 11pt,
  link-style: (
    underline: false,
    color: black,
  ),
)

#header(
  name: name,
  contacts: (
    ("mailto:" + email, email),
    (website-url, "xuanyuwang.com"),
    (linkedin-url, fa-contact-label("linkedin", linkedin-handle)),
  ),
)

== Experience
#exp(
  role: "Software Engineer",
  date: "Dec. 2023 — Present",
  organization: "Cresta — Conversation Intelligence (Coaching, QM, Analytics)",
  location: "Toronto, Canada",
  details: [
    - Consolidated user-filter semantics across 30+ analytics APIs and three implementations; authored a behavioral standard, surfaced five silent divergences (including union-vs-intersection), and completed migration to a canonical path (12/29 → 29/29) with feature flags and shadow-mode validation (10,000+ queries, 0 mismatches).
    - Introduced ClickHouse external tables for passing reference data into analytics queries without embedding huge `IN` lists; compared 10 approaches, benchmarked performance (e.g. ~4.8× improvement at large user lists vs embedding), and rolled out across 19+ call sites with phased deployment.
    - Investigated PostgreSQL↔ClickHouse scorecard inconsistencies (async races, ORM full-struct overwrites); after a reverted timestamp-based fix, used custom load tests to show true failure modes and shipped a multi-layer fix (atomic transactions, async re-read from DB, partial updates) with production verification on large submitted-scorecard samples.
    - Led backend delivery for Group Calibration: Director task CRUD, consistency scoring, `RetrieveGroupCalibrationStats`, notifications, and multi-select criteria—supporting QM workflows end-to-end.
    - Shipped Agent-on-Call features (conversation assign/unassign, manager whisper types, live-assist action annotations) and ongoing coaching/analytics fixes impacting major enterprise customers.
  ],
)

== Skills
#pad(
  top: 0.15em,
  [
    *Languages:* Go, Python, TypeScript, SQL \
    *Data & infra:* PostgreSQL, ClickHouse, Redis, gRPC, Protobuf, Docker, Kubernetes, AWS \
    *Product surface:* REST/gRPC APIs, React (director app), feature-flagged rollouts \
  ],
)

== Selected impact
#project(
  name: "User filter consolidation",
  technologies: ("Go", "ClickHouse", "PostgreSQL"),
  details: [
    - Single semantic standard for Insights APIs; incremental migration and operational safety (shadow mode, dashboards).
  ],
)

#project(
  name: "ClickHouse external tables",
  technologies: ("Go", "ClickHouse"),
  details: [
    - Reusable helpers (`buildExtTable`, context attachment) for reference-data filtering beyond user IDs.
  ],
)

#project(
  name: "Scorecard PG↔CH sync",
  technologies: ("Go", "PostgreSQL", "ClickHouse"),
  details: [
    - Alignment pattern for async PG→CH pipelines: transactions, re-read after commit, targeted GORM updates, verification tooling.
  ],
)

// Add == Education and #edu(...) when you want degrees listed on this version.
