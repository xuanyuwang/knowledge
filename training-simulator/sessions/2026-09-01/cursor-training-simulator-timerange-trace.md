# Training Simulator date/time range: frontend → RetrieveTrainingSimulatorTaskStats → ListDirectorTasks

Date: 2026-09-01
Repos: go-servers (`/Users/xuanyu.wang/repos/go-servers`), cresta-proto, director (frontend evidence only)

## Verdict

UI `TimestampRange` (`startTimestamp`/`endTimestamp`) is forwarded unchanged into `ListDirectorTasks.time_range`. For training-simulator tasks, SQL keeps rows whose schedule interval `[created_at, dueTime]` **overlaps** the filter window using **inclusive** operators (`created_at <= end`, `dueTime >= start`). That conflicts with `TimestampRange`'s documented `[start, end)` half-open semantics. Task-run / conversation / evaluation timestamps are **not** used as range predicates; runs are loaded for matching tasks only, and `create_time` is used later only to pick latest attempts.

## Key mismatch

- Documented: `[start_timestamp, end_timestamp)` in `cresta/v1/common/time/time.proto`
- Implemented: inclusive `[start, end]` overlap / created_at window in `action_list_director_task.go`
- Retrieve proto comment says "filtering task runs" but implementation filters **director tasks** by schedule overlap, then loads all runs for those tasks with no time filter
