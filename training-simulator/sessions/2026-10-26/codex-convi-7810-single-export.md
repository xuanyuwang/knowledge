# CONVI-7810 single export API and CSV schema

Source repo: `/Users/xuanyu.wang/repos/cresta-proto`; branch/worktree: `xw/convi-7810-training-simulator-export-protos` at `/Users/xuanyu.wang/repos/cresta-proto-training-simulator-csv`.

The user approved consolidating the three export RPCs into `ExportTrainingSimulatorResults` and requested comments describing the CSV schema. Use a required `oneof selection` with session, lesson, and module selections. Each accepts multiple nonempty target names. Lesson/module selections retain assignment-window filtering; module selection also supports optional lesson narrowing. Preserve the manager/admin session roles and admin-only lesson/module role requirements in the contract; the future handler must enforce the selection-specific requirements.

Document the previously discussed fixed identity, timestamp, score, and result headers; dynamic historical criterion and quiz question columns; unique attempt rows including retakes; explicit N/A and Pending; and blank cells for definitions absent from an attempt's revision. No source implementation or generated files are part of this PR.

Pushed `69f4a5fa4c016d93a9f97647d41976215db398e1` to [PR #10413](https://github.com/cresta/cresta-proto/pull/10413). Repository-wide proto lint and API lint, focused Buf lint/build, compatibility against main with imports excluded, Gazelle, the Training Simulator Bazel build, formatting, and diff checks pass. No generated files changed. New CI pending. Git/gh used existing authentication; no explicit credential selection.

## Approved flat-filter follow-up

The user approved replacing the selection oneof with flat `director_task_names`, `training_lesson_names`, and `training_module_names` filters plus an optional top-level `time_range`. Names within a list use OR; populated lists intersect with AND; an empty list adds no restriction. At least one name list must be populated, even when a date range is provided. This cross-field validation is a handler requirement documented on the request; individual name strings have generated nonempty validation. Managers must supply session names in their reporting scope, and lesson/module filters can narrow those sessions. Requests without session names require the admin reporting roles. The CSV response/schema is retained. Validation and push pending.

Flat-filter update pushed as `fd2e479c76c4782abe5e2ef17d3a820e331fd328`. Full proto/API lint, focused Buf build/lint/compatibility, Gazelle, package Bazel build, formatting, and diff checks pass. Only the service proto changed in this commit. New CI pending. Git/gh used existing authentication; no credential was explicitly selected.
