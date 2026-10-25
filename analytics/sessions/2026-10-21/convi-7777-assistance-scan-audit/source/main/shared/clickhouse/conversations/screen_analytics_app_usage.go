package conversations

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

// AppUsageConversationScope selects classified spans by their relationship to a
// conversation.
type AppUsageConversationScope int

const (
	// AppUsageConversationScopeAll includes every span.
	AppUsageConversationScopeAll AppUsageConversationScope = iota
	// AppUsageConversationScopeConversation includes only in-call spans (spans
	// with at least one conversation id).
	AppUsageConversationScopeConversation
	// AppUsageConversationScopeNonConversation includes only off-call spans (spans
	// with no conversation id).
	AppUsageConversationScopeNonConversation
)

// AppUsageBucketWidth is the width of one time bucket in an over-time query.
type AppUsageBucketWidth int

const (
	AppUsageBucketWidthHour AppUsageBucketWidth = iota
	AppUsageBucketWidthDay
	// AppUsageBucketWidthWeek buckets by calendar week starting on Monday.
	AppUsageBucketWidthWeek
	// AppUsageBucketWidthMonth buckets by calendar month starting on the 1st.
	AppUsageBucketWidthMonth
)

func (w AppUsageBucketWidth) clickhouseInterval() string {
	switch w {
	case AppUsageBucketWidthDay:
		return "toIntervalDay(1)"
	case AppUsageBucketWidthWeek:
		return "toIntervalWeek(1)"
	case AppUsageBucketWidthMonth:
		return "toIntervalMonth(1)"
	default:
		return "toIntervalHour(1)"
	}
}

// AppUsageBucketing says how an over-time query groups spans into buckets: how
// wide a bucket is, and whose calendar its boundaries follow. A day is a real
// local day, so it is 23 or 25 hours long when the zone moves its clocks.
type AppUsageBucketing struct {
	Width AppUsageBucketWidth
	// Location is the time zone the boundaries follow. Nil means UTC.
	Location *time.Location
}

// zoneName is the time zone name to send to ClickHouse. A missing location and
// the machine-local zone, whose name ClickHouse cannot resolve, both give UTC.
func (b AppUsageBucketing) zoneName() string {
	if b.Location == nil {
		return "UTC"
	}
	name := b.Location.String()
	if name == "" || name == "Local" {
		return "UTC"
	}
	return name
}

// appUsageCoverage says whether a read skips APP spans that a URL span already
// reports.
type appUsageCoverage int

const (
	// coverageCountEveryEntry counts every span. Per-entry reads use this so a
	// browser keeps its full focus time.
	coverageCountEveryEntry appUsageCoverage = iota
	// coverageURLWins skips APP spans marked url_covered, so each second counts
	// once. Totals use this.
	coverageURLWins
)

// condition returns the WHERE fragment for this coverage, or an empty string
// when it needs none.
func (c appUsageCoverage) condition() string {
	if c == coverageURLWins {
		return "url_covered = false"
	}
	return ""
}

// ScreenAnalyticsAppUsageQuery bounds an app-usage aggregation over classified
// spans.
//
// Window attribution: a span is selected when it overlaps [Start, End), and only
// the overlapping part of it is counted. A span that begins before the window or
// runs past its end contributes just the seconds that fall inside. Bucketed
// readers go further and give each bucket only the part of the span that falls
// in that bucket, so a boundary inside a span does not shift time between
// buckets. A span that began more than maxSpanLength before Start is not
// selected, so the query can prune partitions.
type ScreenAnalyticsAppUsageQuery struct {
	Start time.Time
	End   time.Time
	Scope AppUsageConversationScope
	// UserIDs restricts to these agents. Empty means all agents in the profile.
	UserIDs []string
	// RuleID restricts to spans classified by this rule. Empty means any rule.
	RuleID string
	// RuleActions restricts to spans with one of these classification outcomes,
	// as RuleAction enum numbers. Empty means any outcome. Uncategorized is a
	// stored value like any other, so it needs no special case.
	RuleActions []int32
	// EntryIdentifier restricts to spans for this app/URL entry. Empty means any
	// entry.
	EntryIdentifier string
	// EntryType restricts to spans of this kind, as an EntryType enum number.
	// Zero means any kind.
	//
	// An app is keyed by its normalized name, so one identifier can name both an
	// app and a URL host: the app "zoom.us" and the site "zoom.us". Pairing this
	// with EntryIdentifier is what keeps a drill-down on one of them from summing
	// both.
	EntryType int32
}

// maxSpanLength bounds how long one stored span can be. The rollup cuts every
// span on the hour, so this is far above the real maximum.
//
// It exists so the reader can keep a lower bound on span_start. Without that
// bound, every query would read the whole table.
const maxSpanLength = 24 * time.Hour

// maxConversationLength bounds how long one conversation can run. It exists so
// the conversation metadata join can keep a lower bound on
// conversation_start_time, which is the first column of that table's sort key.
// Without the bound the join reads every conversation the profile ever had:
// conversation_id is not a key prefix there, so looking a conversation up by id
// cannot use the index.
//
// A call that outruns this cap still gets a row, just without its start time and
// duration. So the cap has to clear the longest real call, not the longest one
// the screen spans happen to touch: a conversation with no known end time is
// never clipped, so it keeps collecting spans for as long as it stays open. On
// walter-dev 97.97% of conversations finish inside 24h and 99.94% inside 30 days.
//
// The extra days are close to free, because they prune at granule level. Over a
// 30-day query window the join reads 1,168,869 rows at this cap against
// 1,142,629 at a 24h cap. Dropping the cap and looking the ids up instead reads
// 3,435,372.
const maxConversationLength = 30 * 24 * time.Hour

// startTimeCorrectionSlack extends the conversation_d reads in
// ReadScreenAnalyticsAppUsageConversationRows and
// ReadScreenAnalyticsConversationEndTimes past the end of the query window.
// Ingest can correct conversation_start_time, and the read bounds on that column
// before argMax picks the newest version. Without the slack, a version corrected to
// start at or after the window end is dropped and the answer comes from the older
// version. Like the days maxConversationLength adds, this one prunes at granule
// level.
const startTimeCorrectionSlack = 24 * time.Hour

// Rule action numbers the per-action split compares against. They mirror the
// RuleAction enum, which this package keeps out of its imports and pins by
// value the same way entry_type does.
const (
	ruleActionAllowlist = 1
	ruleActionBlocklist = 2
)

// whereClause builds the shared WHERE fragment and its positional args. It
// selects every span that overlaps [Start, End).
func (q ScreenAnalyticsAppUsageQuery) whereClause(coverage appUsageCoverage) (string, []any) {
	conditions := []string{"span_start >= ? AND span_start < ? AND span_end > ?"}
	args := []any{q.Start.Add(-maxSpanLength), q.End, q.Start}
	if condition := coverage.condition(); condition != "" {
		conditions = append(conditions, condition)
	}
	switch q.Scope {
	case AppUsageConversationScopeConversation:
		conditions = append(conditions, "notEmpty(conversation_ids)")
	case AppUsageConversationScopeNonConversation:
		conditions = append(conditions, "empty(conversation_ids)")
	}
	if len(q.UserIDs) > 0 {
		conditions = append(conditions, "user_id IN (?)")
		args = append(args, q.UserIDs)
	}
	if q.RuleID != "" {
		conditions = append(conditions, "rule_id = ?")
		args = append(args, q.RuleID)
	}
	if len(q.RuleActions) > 0 {
		conditions = append(conditions, "rule_action IN (?)")
		args = append(args, q.RuleActions)
	}
	if q.EntryIdentifier != "" {
		conditions = append(conditions, "entry_identifier = ?")
		args = append(args, q.EntryIdentifier)
	}
	if q.EntryType != 0 {
		conditions = append(conditions, "entry_type = ?")
		args = append(args, q.EntryType)
	}
	return strings.Join(conditions, " AND "), args
}

// drillDownCoverage is coverageCountEveryEntry when the query names a rule or
// entry, so a drill-down matches the apps-over-time row it was opened from.
// Otherwise it is coverageURLWins.
func (q ScreenAnalyticsAppUsageQuery) drillDownCoverage() appUsageCoverage {
	if q.RuleID != "" || q.EntryIdentifier != "" {
		return coverageCountEveryEntry
	}
	return coverageURLWins
}

// clippedDurationExpr builds the SELECT fragment that sums how much of each span
// falls inside [Start, End), and its positional args.
func (q ScreenAnalyticsAppUsageQuery) clippedDurationExpr() (string, []any) {
	return "sum(dateDiff('millisecond', greatest(span_start, ?), least(span_end, ?))) AS total_duration_ms",
		[]any{q.Start, q.End}
}

// bucketedSpansFrom builds the FROM fragment every bucketed read runs on, and
// its positional args. columns are the span columns the outer query still needs.
//
// The fragment clips each span to [Start, End) and then emits one row per bucket
// the clipped span touches, carrying clipped_start, clipped_end and bucket_start.
// A bucket boundary can fall inside a span: spans are cut on the UTC hour, but a
// zone whose offset is not a whole hour, such as Asia/Kolkata, puts its hour and
// day boundaries at half past. The outer query pairs this with
// bucketedDurationExpr, which counts only the part of the span in each bucket.
//
// A clipped span is never longer than one bucket, so its first and last moment
// give every bucket it touches. Bucket boundaries follow the calendar of
// bucketing.Location.
//
// toDateTime is needed because a week interval returns a Date, which has no time
// zone. Reading a Date back would give midnight UTC, not local midnight.
func (q ScreenAnalyticsAppUsageQuery) bucketedSpansFrom(bucketing AppUsageBucketing, coverage appUsageCoverage, columns string) (string, []any) {
	zone := bucketing.zoneName()
	interval := bucketing.Width.clickhouseInterval()
	bucketOf := func(instant string) string {
		return fmt.Sprintf("toDateTime(toStartOfInterval(%s, %s, ?), ?)", instant, interval)
	}
	where, whereArgs := q.whereClause(coverage)

	clipped := "(SELECT " + columns + ", " +
		"greatest(span_start, ?) AS clipped_start, " +
		"least(span_end, ?) AS clipped_end, " +
		// clipped_last is the last moment inside the span. A span ending exactly on
		// a boundary belongs to the bucket before it, not the one it touches.
		"greatest(clipped_start, clipped_end - toIntervalMillisecond(1)) AS clipped_last " +
		"FROM screen_analytics_classified_spans_d FINAL " +
		"WHERE " + where + ")"

	from := "(SELECT " + columns + ", clipped_start, clipped_end, " +
		"arrayJoin(arrayDistinct([" + bucketOf("clipped_start") + ", " + bucketOf("clipped_last") + "])) AS bucket_start " +
		"FROM " + clipped + ")"

	args := slices.Concat([]any{zone, zone, zone, zone, q.Start, q.End}, whereArgs)
	return from, args
}

// bucketedDurationExpr sums how much of each clipped span falls in its own
// bucket. It runs on the rows bucketedSpansFrom emits.
func bucketedDurationExpr(bucketing AppUsageBucketing) string {
	return "sum(dateDiff('millisecond', greatest(clipped_start, bucket_start), " +
		"least(clipped_end, bucket_start + " + bucketing.Width.clickhouseInterval() + "))) AS total_duration_ms"
}

// ScreenAnalyticsActionDuration is the total focus duration for one rule action
// over a window, with the latest span end observed for that action.
type ScreenAnalyticsActionDuration struct {
	RuleAction    int32
	TotalDuration time.Duration
	LatestSpanEnd time.Time
}

// ScreenAnalyticsActionBucket is the focus duration for one rule action within
// one time bucket.
type ScreenAnalyticsActionBucket struct {
	BucketStart   time.Time
	RuleAction    int32
	TotalDuration time.Duration
}

// ScreenAnalyticsAgentActionDuration is the total focus duration for one agent
// and rule action over a window.
type ScreenAnalyticsAgentActionDuration struct {
	UserID        string
	RuleAction    int32
	TotalDuration time.Duration
}

// ScreenAnalyticsAgentBucket is the focus duration for one agent within one time
// bucket.
type ScreenAnalyticsAgentBucket struct {
	UserID        string
	BucketStart   time.Time
	TotalDuration time.Duration
}

// ScreenAnalyticsConversationUsageRow is one conversation's focus time over a
// window, split by rule action, alongside the conversation's own start time and
// duration.
//
// HasMetadata is false when the conversation is not in conversation_d. StartTime
// and ConversationDuration are then both zero, and the caller must leave the
// matching proto fields unset rather than send a zero value.
type ScreenAnalyticsConversationUsageRow struct {
	ConversationID       string
	Allowlisted          time.Duration
	Blocklisted          time.Duration
	Uncategorized        time.Duration
	HasMetadata          bool
	StartTime            time.Time
	ConversationDuration time.Duration
}

// AppUsageConversationOrderColumn is the column the conversation rows are sorted
// on. The sort runs in ClickHouse, so it has to be named before the read rather
// than applied to the result.
type AppUsageConversationOrderColumn int

const (
	// AppUsageConversationOrderStartTime sorts on when the conversation started.
	AppUsageConversationOrderStartTime AppUsageConversationOrderColumn = iota
	// AppUsageConversationOrderConversationDuration sorts on how long the whole
	// conversation ran.
	AppUsageConversationOrderConversationDuration
	// AppUsageConversationOrderTotalDuration sorts on focus time inside the
	// conversation, across every action.
	AppUsageConversationOrderTotalDuration
	AppUsageConversationOrderAllowlistedDuration
	AppUsageConversationOrderBlocklistedDuration
	AppUsageConversationOrderUncategorizedDuration
)

// sqlColumn returns the SELECT alias this column sorts on.
func (c AppUsageConversationOrderColumn) sqlColumn() string {
	switch c {
	case AppUsageConversationOrderConversationDuration:
		return "conversation_duration_secs"
	case AppUsageConversationOrderTotalDuration:
		return "total_ms"
	case AppUsageConversationOrderAllowlistedDuration:
		return "allowlisted_ms"
	case AppUsageConversationOrderBlocklistedDuration:
		return "blocklisted_ms"
	case AppUsageConversationOrderUncategorizedDuration:
		return "uncategorized_ms"
	default:
		return "conversation_start_time"
	}
}

// fromConversationMetadata reports whether the column comes from conversation_d
// rather than from the spans. A conversation missing there has no value for it.
func (c AppUsageConversationOrderColumn) fromConversationMetadata() bool {
	return c == AppUsageConversationOrderStartTime || c == AppUsageConversationOrderConversationDuration
}

// AppUsageConversationOrder is a resolved row order for the conversations read.
type AppUsageConversationOrder struct {
	Column AppUsageConversationOrderColumn
	Desc   bool
}

// orderByClause builds the ORDER BY that pages the conversation rows.
//
// Rows that tie on the sorted column fall back to conversation_id ascending, so
// two pages of the same result never overlap or skip. When the column comes from
// conversation_d, rows missing from there sort last whichever way the sort runs:
// ordering them on a value they do not have would float them to the top of an
// ascending sort, which reads as a bug.
func (o AppUsageConversationOrder) orderByClause() string {
	direction := "ASC"
	if o.Desc {
		direction = "DESC"
	}
	clause := o.Column.sqlColumn() + " " + direction + ", conversation_id ASC"
	if o.Column.fromConversationMetadata() {
		return "has_metadata DESC, " + clause
	}
	return clause
}

// ScreenAnalyticsEntryBucket is the focus duration for one (rule, entry) pair
// within one time bucket. It carries the rule action, entry type, latest entry
// display name, and latest rule revision, so the caller can build the apps
// over-time table and resolve rule names. LatestSpanEnd is the newest span end
// in the bucket. The caller uses it to pick each rule's latest revision across
// buckets.
type ScreenAnalyticsEntryBucket struct {
	RuleID           string
	RuleAction       int32
	RuleRevision     string
	EntryType        int32
	EntryIdentifier  string
	EntryDisplayName string
	BucketStart      time.Time
	TotalDuration    time.Duration
	LatestSpanEnd    time.Time
}

// ReadScreenAnalyticsAppUsageActionDurations returns the total focus duration and
// latest span end per rule action over the query window.
//
// The query uses FINAL so re-classified ReplacingMergeTree versions are not
// double counted.
func ReadScreenAnalyticsAppUsageActionDurations(
	ctx context.Context,
	conn driver.Conn,
	q ScreenAnalyticsAppUsageQuery,
) ([]*ScreenAnalyticsActionDuration, error) {
	where, whereArgs := q.whereClause(coverageURLWins)
	duration, durationArgs := q.clippedDurationExpr()
	query := "SELECT rule_action, " + duration + ", " +
		"max(span_end) AS latest_span_end " +
		"FROM screen_analytics_classified_spans_d FINAL " +
		"WHERE " + where + " " +
		"GROUP BY rule_action"
	args := slices.Concat(durationArgs, whereArgs)
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query screen_analytics_classified_spans_d for action durations: %w", err)
	}
	defer rows.Close()

	var result []*ScreenAnalyticsActionDuration
	for rows.Next() {
		var (
			ruleAction      int32
			totalDurationMs int64
			latestSpanEnd   time.Time
		)
		if err := rows.Scan(&ruleAction, &totalDurationMs, &latestSpanEnd); err != nil {
			return nil, fmt.Errorf("failed to scan action duration row: %w", err)
		}
		result = append(result, &ScreenAnalyticsActionDuration{
			RuleAction:    ruleAction,
			TotalDuration: time.Duration(totalDurationMs) * time.Millisecond,
			LatestSpanEnd: latestSpanEnd,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating action duration rows: %w", err)
	}
	return result, nil
}

// ReadScreenAnalyticsAppUsageAgentActionDurations returns the total focus
// duration per agent and rule action over the query window.
//
// The query uses FINAL so re-classified ReplacingMergeTree versions are not
// double counted.
func ReadScreenAnalyticsAppUsageAgentActionDurations(
	ctx context.Context,
	conn driver.Conn,
	q ScreenAnalyticsAppUsageQuery,
) ([]*ScreenAnalyticsAgentActionDuration, error) {
	where, whereArgs := q.whereClause(coverageURLWins)
	duration, durationArgs := q.clippedDurationExpr()
	query := "SELECT user_id, rule_action, " + duration + " " +
		"FROM screen_analytics_classified_spans_d FINAL " +
		"WHERE " + where + " " +
		"GROUP BY user_id, rule_action"
	args := slices.Concat(durationArgs, whereArgs)
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query screen_analytics_classified_spans_d for agent action durations: %w", err)
	}
	defer rows.Close()

	var result []*ScreenAnalyticsAgentActionDuration
	for rows.Next() {
		var (
			userID          string
			ruleAction      int32
			totalDurationMs int64
		)
		if err := rows.Scan(&userID, &ruleAction, &totalDurationMs); err != nil {
			return nil, fmt.Errorf("failed to scan agent action duration row: %w", err)
		}
		result = append(result, &ScreenAnalyticsAgentActionDuration{
			UserID:        userID,
			RuleAction:    ruleAction,
			TotalDuration: time.Duration(totalDurationMs) * time.Millisecond,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating agent action duration rows: %w", err)
	}
	return result, nil
}

// ReadScreenAnalyticsAppUsageConversationRows returns one ordered page of
// conversations that had classified in-call activity over the query window, each
// with its focus time split by rule action and labelled with the conversation's
// own start time and duration. Set the query's UserIDs, RuleID, and
// EntryIdentifier to scope to an agent set and one app or rule. Coverage follows
// drillDownCoverage.
//
// The whole page is built in ClickHouse: the action split, the conversation
// label, the sort, and the window. Pass limit = pageSize + 1 to learn whether a
// next page exists without a second query.
//
// A span that covers two calls at once is counted in full against each of them.
// That is what a per-conversation drill-down should show, so the per-conversation
// numbers can add up to more than the agent total. Only this read splits a span
// by conversation; every other read counts the span once.
//
// The conversation label comes from a LEFT JOIN, so a conversation missing from
// conversation_d still gets a row, with HasMetadata false. The joined side is
// bounded on conversation_start_time rather than looked up by conversation_id:
// id is not a prefix of that table's sort key, so an id lookup reads every
// conversation the profile ever had. See maxConversationLength and
// startTimeCorrectionSlack.
//
// The span side uses FINAL so re-classified ReplacingMergeTree versions are not
// double counted. The conversation side groups by conversation_id instead,
// because FINAL cannot collapse re-ingested versions there.
func ReadScreenAnalyticsAppUsageConversationRows(
	ctx context.Context,
	conn driver.Conn,
	q ScreenAnalyticsAppUsageQuery,
	order AppUsageConversationOrder,
	limit, offset int,
) ([]*ScreenAnalyticsConversationUsageRow, error) {
	where, whereArgs := q.whereClause(q.drillDownCoverage())

	// clipped_ms is computed once per (span, conversation) pair and then folded
	// three ways, so the window bounds are bound once instead of per action.
	clipped := "(SELECT arrayJoin(conversation_ids) AS conversation_id, rule_action, " +
		"dateDiff('millisecond', greatest(span_start, ?), least(span_end, ?)) AS clipped_ms " +
		"FROM screen_analytics_classified_spans_d FINAL " +
		"WHERE " + where + " AND notEmpty(conversation_ids))"

	// Any action that is not allowlist or blocklist counts as uncategorized,
	// matching how the API layer folds the same numbers.
	durations := fmt.Sprintf(
		"(SELECT conversation_id, "+
			"sumIf(clipped_ms, rule_action = %d) AS allowlisted_ms, "+
			"sumIf(clipped_ms, rule_action = %d) AS blocklisted_ms, "+
			"sumIf(clipped_ms, rule_action NOT IN (%d, %d)) AS uncategorized_ms, "+ //nolint:sqlinjection // both %d take package constants, never caller input
			"sum(clipped_ms) AS total_ms "+
			"FROM %s GROUP BY conversation_id)",
		ruleActionAllowlist, ruleActionBlocklist, ruleActionAllowlist, ruleActionBlocklist, clipped,
	)

	// FINAL does not promise one row per conversation here, so the join key has
	// to be made unique before the join. conversation_d sorts on
	// (toStartOfHour(conversation_start_time), agent_user_id, conversation_id, ...)
	// and shards on toUnixTimestamp(conversation_start_time). When ingest corrects
	// a call's start time or its agent, the two versions stop sharing a sort key
	// and stop sharing a shard, and FINAL runs per shard. Both then survive:
	// walter-dev holds 43 such calls out of 238,668. Joining on that would print
	// the call twice and eat a slot in the page. argMax takes the newest version,
	// which is what the ReplacingMergeTree would have kept, so FINAL would only
	// add a merge on read. One argMax over a tuple keeps the start time and the
	// duration from the same version, even if two versions tie on update_time.
	// The aliases sit one scope above the window filter. Sharing a scope would
	// let ClickHouse read the WHERE's conversation_start_time as the alias and
	// reject the query for holding an aggregate in a WHERE.
	metadata := "(SELECT conversation_id, " +
		"latest.1 AS conversation_start_time, " +
		"latest.2 AS conversation_duration_secs, " +
		"toUInt8(1) AS has_metadata " +
		"FROM (SELECT conversation_id, " +
		"argMax(tuple(conversation_start_time, conversation_duration_secs), update_time) AS latest " +
		"FROM conversation_d " +
		"WHERE conversation_start_time >= ? AND conversation_start_time < ? " +
		"GROUP BY conversation_id))"

	query := "SELECT conversation_id, allowlisted_ms, blocklisted_ms, uncategorized_ms, " +
		"has_metadata, conversation_start_time, conversation_duration_secs " +
		"FROM " + durations + " AS durations " +
		"LEFT JOIN " + metadata + " AS metadata USING (conversation_id) " +
		"ORDER BY " + order.orderByClause() + " " +
		"LIMIT ? OFFSET ?"

	args := slices.Concat(
		[]any{q.Start, q.End},
		whereArgs,
		[]any{q.Start.Add(-maxConversationLength), q.End.Add(startTimeCorrectionSlack), limit, offset},
	)
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query screen_analytics_classified_spans_d for conversation rows: %w", err)
	}
	defer rows.Close()

	var result []*ScreenAnalyticsConversationUsageRow
	for rows.Next() {
		var (
			conversationID   string
			allowlistedMs    int64
			blocklistedMs    int64
			uncategorizedMs  int64
			hasMetadata      uint8
			startTime        time.Time
			conversationSecs int32
		)
		if err := rows.Scan(
			&conversationID, &allowlistedMs, &blocklistedMs, &uncategorizedMs,
			&hasMetadata, &startTime, &conversationSecs,
		); err != nil {
			return nil, fmt.Errorf("failed to scan conversation usage row: %w", err)
		}
		row := &ScreenAnalyticsConversationUsageRow{
			ConversationID: conversationID,
			Allowlisted:    time.Duration(allowlistedMs) * time.Millisecond,
			Blocklisted:    time.Duration(blocklistedMs) * time.Millisecond,
			Uncategorized:  time.Duration(uncategorizedMs) * time.Millisecond,
			HasMetadata:    hasMetadata != 0,
		}
		// An unmatched LEFT JOIN row carries zero values, not the conversation's.
		if row.HasMetadata {
			row.StartTime = startTime
			row.ConversationDuration = time.Duration(conversationSecs) * time.Second
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating conversation usage rows: %w", err)
	}
	return result, nil
}

// ReadScreenAnalyticsConversationEndTimes returns the authoritative end time of
// each conversation whose newest version ended at or after endAtOrAfter and
// started before startBefore. Unended calls are skipped: the writer stores
// conversation_end_time = conversation_start_time as a placeholder until a real
// end is known, so the query keeps only rows whose end is strictly after the
// start. This prevents the classifier from treating that placeholder as a real
// end and dropping still-in-call focus to off-call. A reopened chat is unended
// again by its newest version, so it is skipped too. Calls that started more
// than maxConversationLength before endAtOrAfter are not read.
func ReadScreenAnalyticsConversationEndTimes(
	ctx context.Context,
	conn driver.Conn,
	endAtOrAfter, startBefore time.Time,
) (map[string]time.Time, error) {
	// argMax rather than FINAL, for the reason spelled out in
	// ReadScreenAnalyticsAppUsageConversationRows: a corrected start time or agent
	// splits a call's two versions across sort keys and across shards, and FINAL
	// runs per shard, so both survive. The caller folds rows into a map keyed by
	// conversation_id, so today that silently keeps whichever version arrives last.
	//
	// The start and end filters apply to the newest version, after argMax. Both
	// columns change between versions: conversation_start_time can be corrected, and
	// conversation_end_time goes back to the placeholder when a closed chat is
	// reopened. Filtering first would let an older version win, such as the old
	// closed end of a reopened chat, or a version whose start later moved to or past
	// startBefore. One argMax over a tuple keeps the start and the end from the same
	// version.
	//
	// The inner scope still bounds the scan. The start bounds prune on the sort key:
	// maxConversationLength before endAtOrAfter, as in the drill-down read, and
	// startTimeCorrectionSlack past startBefore. The end-or-update filter keeps the
	// GROUP BY to conversations that ended or were rewritten since endAtOrAfter
	// without dropping a newest version: update_time is the wall-clock time of the
	// write, which is never before a real end, so if any version passes, the newest
	// one does too.
	const query = "SELECT conversation_id, latest.2 AS conversation_end_time " +
		"FROM (SELECT conversation_id, " +
		"argMax(tuple(conversation_start_time, conversation_end_time), update_time) AS latest " +
		"FROM conversation_d " +
		"WHERE conversation_start_time >= ? AND conversation_start_time < ? " +
		"AND (conversation_end_time >= ? OR update_time >= ?) " +
		"GROUP BY conversation_id) " +
		"WHERE latest.1 < ? AND latest.2 >= ? AND latest.2 > latest.1"
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, query,
		endAtOrAfter.Add(-maxConversationLength), startBefore.Add(startTimeCorrectionSlack),
		endAtOrAfter, endAtOrAfter,
		startBefore, endAtOrAfter)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversation_d for conversation end times: %w", err)
	}
	defer rows.Close()

	endTimes := make(map[string]time.Time)
	for rows.Next() {
		var (
			conversationID string
			endTime        time.Time
		)
		if err := rows.Scan(&conversationID, &endTime); err != nil {
			return nil, fmt.Errorf("failed to scan conversation end time row: %w", err)
		}
		endTimes[conversationID] = endTime
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating conversation end time rows: %w", err)
	}
	return endTimes, nil
}

// ReadScreenAnalyticsAppUsageAgentBuckets returns the focus duration per agent
// per time bucket over the query window. Combine with the query's RuleID and
// EntryIdentifier to scope to one app or rule. bucketing sets the bucket width
// and the time zone whose calendar the boundaries follow. Coverage follows
// drillDownCoverage.
//
// The query uses FINAL so re-classified ReplacingMergeTree versions are not
// double counted.
func ReadScreenAnalyticsAppUsageAgentBuckets(
	ctx context.Context,
	conn driver.Conn,
	q ScreenAnalyticsAppUsageQuery,
	bucketing AppUsageBucketing,
) ([]*ScreenAnalyticsAgentBucket, error) {
	from, queryArgs := q.bucketedSpansFrom(bucketing, q.drillDownCoverage(), "user_id")
	query := "SELECT user_id, bucket_start, " + bucketedDurationExpr(bucketing) + " " +
		"FROM " + from + " " +
		"GROUP BY user_id, bucket_start " +
		"ORDER BY user_id, bucket_start"
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query screen_analytics_classified_spans_d for agent buckets: %w", err)
	}
	defer rows.Close()

	var result []*ScreenAnalyticsAgentBucket
	for rows.Next() {
		var (
			userID          string
			bucketStart     time.Time
			totalDurationMs int64
		)
		if err := rows.Scan(&userID, &bucketStart, &totalDurationMs); err != nil {
			return nil, fmt.Errorf("failed to scan agent bucket row: %w", err)
		}
		result = append(result, &ScreenAnalyticsAgentBucket{
			UserID:        userID,
			BucketStart:   bucketStart,
			TotalDuration: time.Duration(totalDurationMs) * time.Millisecond,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating agent bucket rows: %w", err)
	}
	return result, nil
}

// ReadScreenAnalyticsAppUsageActionBuckets returns the focus duration per rule
// action per time bucket over the query window. bucketing sets the bucket width
// and the time zone whose calendar the boundaries follow.
//
// The query uses FINAL so re-classified ReplacingMergeTree versions are not
// double counted.
func ReadScreenAnalyticsAppUsageActionBuckets(
	ctx context.Context,
	conn driver.Conn,
	q ScreenAnalyticsAppUsageQuery,
	bucketing AppUsageBucketing,
) ([]*ScreenAnalyticsActionBucket, error) {
	from, queryArgs := q.bucketedSpansFrom(bucketing, coverageURLWins, "rule_action")
	query := "SELECT bucket_start, rule_action, " + bucketedDurationExpr(bucketing) + " " +
		"FROM " + from + " " +
		"GROUP BY bucket_start, rule_action " +
		"ORDER BY bucket_start"
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query screen_analytics_classified_spans_d for action buckets: %w", err)
	}
	defer rows.Close()

	var result []*ScreenAnalyticsActionBucket
	for rows.Next() {
		var (
			bucketStart     time.Time
			ruleAction      int32
			totalDurationMs int64
		)
		if err := rows.Scan(&bucketStart, &ruleAction, &totalDurationMs); err != nil {
			return nil, fmt.Errorf("failed to scan action bucket row: %w", err)
		}
		result = append(result, &ScreenAnalyticsActionBucket{
			BucketStart:   bucketStart,
			RuleAction:    ruleAction,
			TotalDuration: time.Duration(totalDurationMs) * time.Millisecond,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating action bucket rows: %w", err)
	}
	return result, nil
}

// ScreenAnalyticsRuleBucket is one rule's focus duration in one time bucket,
// summed over every entry the rule matched. It is the rule-level rollup of
// ScreenAnalyticsEntryBucket, for callers that render rule rows and never open
// the entries underneath.
type ScreenAnalyticsRuleBucket struct {
	RuleID        string
	RuleAction    int32
	RuleRevision  string
	BucketStart   time.Time
	TotalDuration time.Duration
	LatestSpanEnd time.Time
}

// ReadScreenAnalyticsAppUsageRuleBuckets returns focus duration per rule per time
// bucket over the query window, summed over the rule's entries. Each row carries
// the latest rule revision, picked by (span_end, written_at) so two spans sharing
// a span_end do not flip the result, and the newest span end in the bucket.
// bucketing sets the width and the timezone whose calendar the boundaries follow.
//
// This aggregates the same spans as ReadScreenAnalyticsAppUsageEntryBuckets under
// the same coverage, but groups without the entry columns. A rule with many
// entries returns one row per bucket instead of one per entry per bucket, which
// is why it is a separate query rather than a Go-side sum. The query uses FINAL
// so re-classified ReplacingMergeTree versions are not double counted.
func ReadScreenAnalyticsAppUsageRuleBuckets(
	ctx context.Context,
	conn driver.Conn,
	q ScreenAnalyticsAppUsageQuery,
	bucketing AppUsageBucketing,
) ([]*ScreenAnalyticsRuleBucket, error) {
	from, queryArgs := q.bucketedSpansFrom(
		bucketing,
		coverageCountEveryEntry,
		"rule_id, rule_action, rule_revision, span_end, written_at",
	)
	query := "SELECT rule_id, rule_action, " +
		"argMax(rule_revision, (span_end, written_at)) AS rule_revision, " +
		"bucket_start, " + bucketedDurationExpr(bucketing) + ", " +
		"max(span_end) AS latest_span_end " +
		"FROM " + from + " " +
		"GROUP BY rule_id, rule_action, bucket_start " +
		"ORDER BY rule_id, bucket_start"
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query screen_analytics_classified_spans_d for rule buckets: %w", err)
	}
	defer rows.Close()

	var result []*ScreenAnalyticsRuleBucket
	for rows.Next() {
		var (
			ruleID          string
			ruleAction      int32
			ruleRevision    string
			bucketStart     time.Time
			totalDurationMs int64
			latestSpanEnd   time.Time
		)
		if err := rows.Scan(&ruleID, &ruleAction, &ruleRevision, &bucketStart, &totalDurationMs, &latestSpanEnd); err != nil {
			return nil, fmt.Errorf("failed to scan rule bucket row: %w", err)
		}
		result = append(result, &ScreenAnalyticsRuleBucket{
			RuleID:        ruleID,
			RuleAction:    ruleAction,
			RuleRevision:  ruleRevision,
			BucketStart:   bucketStart,
			TotalDuration: time.Duration(totalDurationMs) * time.Millisecond,
			LatestSpanEnd: latestSpanEnd,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rule bucket rows: %w", err)
	}
	return result, nil
}

// ReadScreenAnalyticsAppUsageEntryBuckets returns the focus duration per (rule,
// entry) pair per time bucket over the query window. Each row carries the latest
// entry display name and rule revision observed for the pair. The pick uses
// (span_end, written_at) as the ordering, so two spans that share a span_end do
// not flip the result; the more recently written span wins. Each row also
// carries the newest span end in the bucket, so the caller can pick the rule's
// latest revision across buckets. bucketing sets the bucket width and the time
// zone whose calendar the boundaries follow.
//
// The query uses FINAL so re-classified ReplacingMergeTree versions are not
// double counted.
func ReadScreenAnalyticsAppUsageEntryBuckets(
	ctx context.Context,
	conn driver.Conn,
	q ScreenAnalyticsAppUsageQuery,
	bucketing AppUsageBucketing,
) ([]*ScreenAnalyticsEntryBucket, error) {
	from, queryArgs := q.bucketedSpansFrom(
		bucketing,
		coverageCountEveryEntry,
		"rule_id, rule_action, entry_type, entry_identifier, entry_display_name, rule_revision, span_end, written_at",
	)
	query := "SELECT rule_id, rule_action, entry_type, entry_identifier, " +
		"argMax(entry_display_name, (span_end, written_at)) AS entry_display_name, " +
		"argMax(rule_revision, (span_end, written_at)) AS rule_revision, " +
		"bucket_start, " + bucketedDurationExpr(bucketing) + ", " +
		"max(span_end) AS latest_span_end " +
		"FROM " + from + " " +
		"GROUP BY rule_id, rule_action, entry_type, entry_identifier, bucket_start " +
		"ORDER BY rule_id, entry_identifier, bucket_start"
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query screen_analytics_classified_spans_d for entry buckets: %w", err)
	}
	defer rows.Close()

	var result []*ScreenAnalyticsEntryBucket
	for rows.Next() {
		var (
			ruleID           string
			ruleAction       int32
			entryType        int32
			entryIdentifier  string
			entryDisplayName string
			ruleRevision     string
			bucketStart      time.Time
			totalDurationMs  int64
			latestSpanEnd    time.Time
		)
		if err := rows.Scan(&ruleID, &ruleAction, &entryType, &entryIdentifier, &entryDisplayName, &ruleRevision, &bucketStart, &totalDurationMs, &latestSpanEnd); err != nil {
			return nil, fmt.Errorf("failed to scan entry bucket row: %w", err)
		}
		result = append(result, &ScreenAnalyticsEntryBucket{
			RuleID:           ruleID,
			RuleAction:       ruleAction,
			RuleRevision:     ruleRevision,
			EntryType:        entryType,
			EntryIdentifier:  entryIdentifier,
			EntryDisplayName: entryDisplayName,
			BucketStart:      bucketStart,
			TotalDuration:    time.Duration(totalDurationMs) * time.Millisecond,
			LatestSpanEnd:    latestSpanEnd,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating entry bucket rows: %w", err)
	}
	return result, nil
}
