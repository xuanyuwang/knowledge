-- Read-only; run using the same user/connection as the application when possible.
SELECT hostName() AS host, currentUser() AS connection_user,
       currentDatabase() AS database, version() AS version,
       name, value, changed
FROM system.settings
WHERE name = 'join_use_nulls';

-- Synthetic data only. Expected survivors: B and C, totaling 4 searches.
SELECT
    hostName() AS host,
    currentUser() AS connection_user,
    (SELECT value FROM system.settings WHERE name = 'join_use_nulls') AS join_use_nulls,
    groupArray(tuple(c.conversation_id, e.excluded_id, isNull(e.excluded_id))) AS joined_rows,
    countIf(isNull(e.excluded_id)) AS legacy_kept_conversations,
    sumIf(c.searches, isNull(e.excluded_id)) AS legacy_kept_searches
FROM values('conversation_id String, searches UInt64', ('A', 2), ('B', 3), ('C', 1)) AS c
LEFT JOIN (SELECT 'A' AS excluded_id) AS e
    ON c.conversation_id = e.excluded_id;
