# Notes and Guided Workflow suite consolidation

1. Preserve the existing characterization method bodies while moving them to the two original RPC suites.
2. Replace mocked ClickHouse fixtures and SQL snapshots with real seeded full-RPC request, response, weekly, group and document cases.
3. Run the merged suites against unchanged baseline production; commit tests and freeze all helper/test bytes.
4. Restack the extraction, preserve old production bytes, and run the identical merged suites.
5. Record removed-case mapping, hashes, commits and JSON test evidence in the shared reports.
