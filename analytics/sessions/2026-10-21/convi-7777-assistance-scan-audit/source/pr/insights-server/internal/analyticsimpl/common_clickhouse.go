package analyticsimpl

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/ext"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	behaviorpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/behavior"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	policypb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/policy"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"
	logger "github.com/cresta/shared-go/framework/log"
	"github.com/cresta/shared-go/util/envflag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/postgres"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/insights-server/internal/shared/descriptors"
)

const (
	weekFrequency                    = "week"
	timestampFormat                  = "2006-01-02 15:04:05"
	agentUserIDColumn                = "agent_user_id"
	actionIDColumn                   = "action_id"
	actionTypeColumn                 = "action_type"
	actionAdherenceTypeColumn        = "action_adherence_type"
	behaviorIDColumn                 = "behavior_id"
	conversationDurationSecsColumn   = "conversation_duration_secs"
	conversationDurationBinsIDColumn = "conversation_duration_bins_id"
	conversationIDColumn             = "conversation_id"
	conversationStartTimeColumn      = "conversation_start_time"
	conversationEndTimeColumn        = "conversation_end_time"
	createTimeColumn                 = "create_time"
	updateTimeColumn                 = "update_time"
	momentTemplateIDColumn           = "moment_template_id"
	policyIDColumn                   = "policy_id"
	truncatedTimeColumn              = "truncated_time"
	usecaseIDColumn                  = "usecase_id"
	languageCodeColumn               = "language_code"
	momentTypeColumn                 = "moment_type"
	metadataStringValueColumn        = "metadata_string_value"
	metadataNumberValueColumn        = "metadata_number_value"
	metadataBoolValueColumn          = "metadata_bool_value"
	momentAnnotationPayloadColumn    = "moment_annotation_payload"
	conversationOutcomePayloadKey    = "conversation_outcome_payload"
	conversationOutcomeStringKey     = "string_value"
	conversationOutcomeNumberKey     = "number_value"
	conversationOutcomeBoolKey       = "boolean_value"
	isVoicemailColumn                = "is_voice_mail"
	guidedWorkflowNameColumn         = "guided_workflow_name"
	// For scorecard_score (QA) table
	percentageValueColumn           = "percentage_value"
	notApplicableColumn             = "not_applicable"
	manuallyScoredColumn            = "manually_scored"
	aiScoredColumn                  = "ai_scored"
	aiScoreTimeColumn               = "ai_score_time"
	scorecardTemplateIDColumn       = "scorecard_template_id"
	criterionIDColumn               = "criterion_id"
	conversationFormattedTimeColumn = "conversation_formatted_time"
	scorecardTimeColumn             = "scorecard_time"
	scorecardSubmitTimeColumn       = "scorecard_submit_time"
	scoreColumn                     = "score"
	creatorUserIDColumn             = "creator_user_id"
	submitterUserIDColumn           = "submitter_user_id"
	scorecardPublishTimeColumn      = "scorecard_publish_time"
	extTableAgentFilterColumn       = "user_id"
	extTableSubmitterFilterColumn   = "user_id"
	// For the `events` cluster tables.
	eventTimeColumn = "event_time"
	// Table name
	scorecardScoreTableName                          = "scorecard_score_d"
	scoreTableName                                   = "score_d"
	scorecardTableName                               = "scorecard_d"
	momentAnnotationDTableName                       = "moment_annotation_d"
	momentAnnotationMetadataViewTableName            = "moment_annotation_mv_by_metadata_d"
	momentAnnotationConversationOutcomeViewTableName = "moment_annotation_by_conversation_outcome_d"
	outcomeStringValueColumn                         = "outcome_string_value"
	outcomeNumberValueColumn                         = "outcome_number_value"
	outcomeBoolValueColumn                           = "outcome_bool_value"
	outcomeValueTypeColumn                           = "outcome_value_type"
	outcomeValueTypeString                           = "string"
	outcomeValueTypeNumber                           = "number"
	outcomeValueTypeBoolean                          = "boolean"
	extTableAgentFilter                              = "agent_filter"
	extTableSubmitterFilter                          = "submitter_filter"
	// External table for lower-is-better criterion IDs (directionality ranking).
	extTableLowerIsBetterCriterion       = "lower_is_better_criterion"
	extTableLowerIsBetterCriterionColumn = "criterion_id"
)

type ClickhouseTable int32

const (
	UnknownTable ClickhouseTable = iota
	conversationTable
	messageTable
	momentAnnotationTable
	actionAnnotationTable
	conversationEventTable
	scorecardScoreTable
	scoreTable
	scorecardTable
	eventUserEventTable
	eventConversationEventTable
	eventAnalyticsEventTable
	conversationLabelTable
)

var enableExcludedMoments = envflag.Bool("CLICKHOUSE_ENABLE_EXCLUDED_MOMENTS", true, "Whether to enable excluded moments from moment groups in clickhouse query.")

var useActionAnnotationForHintStats = envflag.Bool("USE_ACTION_ANNOTATION_FOR_HINT_STATS", false, "Whether to use action_annotation_d with GLOBAL IN subquery instead of moment_annotation_d for behavioral hint stats.")

// momentAnnotationFilterSource selects which ClickHouse table backs a parsed moment-group filter.
type momentAnnotationFilterSource int

const (
	momentAnnotationFilterSourceRaw momentAnnotationFilterSource = iota
	momentAnnotationFilterSourceMetadataMV
	momentAnnotationFilterSourceConversationOutcomeMV
)

// momentAnnotationFilter is one include/exclude moment group with its table source.
type momentAnnotationFilter struct {
	conditions []conditionAndArg
	source     momentAnnotationFilterSource
}

// In product semantics this means: (conversation matches one of the selected values) OR
// (conversation has no annotation for that metadata moment template id).
type qaMomentValueOrMissingFilter struct {
	selectedValueFilter momentAnnotationFilter
	existenceFilter     momentAnnotationFilter // negated after LEFT JOIN
}

func (s momentAnnotationFilterSource) tableName() string {
	switch s {
	case momentAnnotationFilterSourceMetadataMV:
		return momentAnnotationMetadataViewTableName
	case momentAnnotationFilterSourceConversationOutcomeMV:
		return momentAnnotationConversationOutcomeViewTableName
	default:
		return momentAnnotationDTableName
	}
}

func (s momentAnnotationFilterSource) latestTimeColumn() string {
	// Raw moment_annotation_d uses create_time (source-DB creation). MV paths use update_time
	// because moment_annotation_mv_by_metadata_d only projects update_time. CLO storage also
	// projects create_time so the MV path can switch to create_time in a follow-up (CONVI-7383).
	switch s {
	case momentAnnotationFilterSourceMetadataMV, momentAnnotationFilterSourceConversationOutcomeMV:
		return updateTimeColumn
	default:
		return createTimeColumn
	}
}

func (s momentAnnotationFilterSource) stripMomentType() bool {
	return s == momentAnnotationFilterSourceMetadataMV || s == momentAnnotationFilterSourceConversationOutcomeMV
}

// buildQAMomentFilterQuery builds the latest-value CTE and join for one QA moment filter.
func buildQAMomentFilterQuery(
	filter momentAnnotationFilter,
	index int,
	joinType string,
	outerConversationID string,
	resultConversationID string,
) (string, string, []any) {
	tableName := filter.source.tableName()
	latestTimeColumn := filter.source.latestTimeColumn()
	stripMomentType := filter.source.stripMomentType()
	conditions, args := concatConditionsAndArgsWithPredicate(filter.conditions, func(caa conditionAndArg) bool {
		return stripMomentType && strings.Contains(caa.condition, momentTypeColumn)
	}, true /* concatWithAnd */)
	conditionsNoValue, argsNoValue := concatConditionsAndArgsWithPredicate(filter.conditions, func(caa conditionAndArg) bool {
		if stripMomentType && strings.Contains(caa.condition, momentTypeColumn) {
			return true
		}
		for _, keyword := range momentAnnotationValueConditionKeywords() {
			if strings.Contains(caa.condition, keyword) {
				return true
			}
		}
		return false
	}, true /* concatWithAnd */)

	selectedConversationID := "t1.conversation_id"
	if resultConversationID != conversationIDColumn {
		selectedConversationID += " AS " + resultConversationID
	}
	subQuery := fmt.Sprintf(`
		t1_moment_annotation_filter_%d AS (
			SELECT
				-- Rename the column to avoid ambiguous column issue.
				conversation_id,
				MAX(%s) AS latest_update_time
			FROM
				%s
			WHERE
				%s
			GROUP BY conversation_id
		),
		t2_moment_annotation_filter_%d AS (
			SELECT
				conversation_id,
				%s
			FROM
				%s
			WHERE
				%s
		),
		moment_annotation_filter_%d AS (
			SELECT DISTINCT
				%s
			FROM
				t1_moment_annotation_filter_%d AS t1
			JOIN t2_moment_annotation_filter_%d AS t2 ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.%s
		),
	`, index, latestTimeColumn, tableName, conditionsNoValue, index, latestTimeColumn, tableName, conditions, index, selectedConversationID, index, index, latestTimeColumn)
	joinQuery := fmt.Sprintf(`
		%s moment_annotation_filter_%d ON
			%s = moment_annotation_filter_%d.%s
	`, joinType, index, outerConversationID, index, resultConversationID)
	return subQuery, joinQuery, append(argsNoValue, args...)
}

// momentAnnotationMVSupportsTimeColumn reports whether metadata/CLO MV tables
// can filter on timeColumn. Both only project conversation_start_time.
func momentAnnotationMVSupportsTimeColumn(timeColumn string) bool {
	return timeColumn != conversationEndTimeColumn
}

// conversationOutcomeMVEnabled reports whether CLO filters should use the
// conversation-outcome storage table. End-time filters always use the raw
// moment_annotation path because CLO storage lacks conversation_end_time.
func conversationOutcomeMVEnabled(flagEnabled bool, timeColumn string) bool {
	return flagEnabled && momentAnnotationMVSupportsTimeColumn(timeColumn)
}

func momentAnnotationValueConditionKeywords() []string {
	return []string{
		"metadata_string_value",
		"metadata_number_value",
		"metadata_bool_value",
		momentAnnotationPayloadColumn,
		outcomeStringValueColumn,
		outcomeNumberValueColumn,
		outcomeBoolValueColumn,
		outcomeValueTypeColumn,
	}
}

type conversationQueryOptions struct {
	timeRangeType                    analyticspb.RetrieveClosedConversationsRequest_TargetFieldForTimeRange
	timeRangeColumnOverride          string
	failClosedOnUnsupportedTimeRange bool
}

type ConversationQueryOption func(*conversationQueryOptions)

// WithFailClosedOnUnsupportedTimeRange rejects a time range the target table cannot express rather
// than approximating it. Callers pass the qaStatsFailClosedOnUnsupportedTimeRange rollout flag here;
// evaluating it needs a request context, which the parse helpers do not have.
func WithFailClosedOnUnsupportedTimeRange(failClosed bool) ConversationQueryOption {
	return func(o *conversationQueryOptions) {
		o.failClosedOnUnsupportedTimeRange = failClosed
	}
}

func WithTimeRangeType(timeRangeType analyticspb.RetrieveClosedConversationsRequest_TargetFieldForTimeRange) ConversationQueryOption {
	return func(o *conversationQueryOptions) {
		o.timeRangeType = timeRangeType
	}
}

func WithTimeRangeColumn(column string) ConversationQueryOption {
	return func(o *conversationQueryOptions) {
		o.timeRangeColumnOverride = column
	}
}

func qaTimeRangeFilterTargetOptions(
	basis analyticspb.TimeRangeFilterTarget,
	timeRangeField analyticspb.RetrieveClosedConversationsRequest_TargetFieldForTimeRange,
) ([]ConversationQueryOption, error) {
	if basis == analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_SUBMIT_TIME &&
		timeRangeField == analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT {
		return nil, status.Errorf(codes.InvalidArgument,
			"time_range_filter_target=SUBMIT_TIME is incompatible with conversation_time_range_field=CONVERSATION_ENDED_AT")
	}
	opts := []ConversationQueryOption{WithTimeRangeType(timeRangeField)}
	if basis == analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_SUBMIT_TIME {
		opts = append(opts, WithTimeRangeColumn(scorecardSubmitTimeColumn))
	}
	return opts, nil
}

type conditionAndArg struct {
	condition string
	arg       any
	args      []any
}

func buildTimeRangeConditionAndArgs(
	timeRange *analyticspb.TimeRange,
	targetTables []ClickhouseTable,
	timeColumn string,
) (map[ClickhouseTable][]conditionAndArg, error) {
	conditionsAndArgs := map[ClickhouseTable][]conditionAndArg{}

	// Time range filter.
	if timeRange == nil {
		return nil, status.Errorf(codes.InvalidArgument, "error when parse time range into sql conditions: must set time_range for attribute")
	}
	startTime, endTime, err := shared.ParseFilterByTimeRange(timeRange)
	if err != nil {
		return nil, err
	}
	for _, t := range targetTables {
		colName := tableSpecificColumnName(t, timeColumn)
		conStart, argStart := columnGTE(colName, startTime.Format(timestampFormat))
		conEnd, argEnd := columnLT(colName, endTime.Format(timestampFormat))
		conditionsAndArgs[t] = append(conditionsAndArgs[t],
			conditionAndArg{condition: conStart, arg: argStart},
			conditionAndArg{condition: conEnd, arg: argEnd},
		)
	}
	return conditionsAndArgs, nil
}

// buildUsersExtTable creates a ClickHouse external data table from a list of user IDs.
// The table can be attached to a context via attachExtTablesToContext and referenced
// in SQL as: WHERE agent_user_id IN (SELECT user_id FROM agent_filter)
// The column is named "user_id" (not "agent_user_id") to avoid collision with
// blanket column renames in scorecard (agent_user_id→creator_user_id) and
// live assist (agent_user_id→manager_user_id) query builders.
func buildUsersExtTable(userIDs []string, tableName, columnName string) (*ext.Table, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	table, err := ext.NewTable(tableName,
		ext.Column(columnName, "String"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating ext table for user filter: %w", err)
	}
	for _, id := range userIDs {
		if err := table.Append(id); err != nil {
			return nil, fmt.Errorf("appending user ID to ext table: %w", err)
		}
	}
	return table, nil
}

// buildLowerIsBetterCriteriaExtTable creates a ClickHouse external data table from
// a set of lower-is-better criterion IDs. Referenced in SQL as:
// CASE WHEN criterion_id IN (SELECT criterion_id FROM lower_is_better_criterion) THEN ...
func buildLowerIsBetterCriteriaExtTable(criterionIDs []string) (*ext.Table, error) {
	if len(criterionIDs) == 0 {
		return nil, nil
	}
	table, err := ext.NewTable(extTableLowerIsBetterCriterion,
		ext.Column(extTableLowerIsBetterCriterionColumn, "String"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating ext table for lower-is-better criteria: %w", err)
	}
	for _, id := range criterionIDs {
		if err := table.Append(id); err != nil {
			return nil, fmt.Errorf("appending criterion ID to ext table: %w", err)
		}
	}
	return table, nil
}

// attachExtTablesToContext attaches ClickHouse external data tables to a context.
// The tables are preserved through clickhouse.Context calls (e.g., in QueryWithRetry).
// Returns the original context unchanged if no non-nil tables are provided.
func attachExtTablesToContext(ctx context.Context, tables ...*ext.Table) context.Context {
	nonNil := make([]*ext.Table, 0, len(tables))
	for _, t := range tables {
		if t != nil {
			nonNil = append(nonNil, t)
		}
	}
	if len(nonNil) == 0 {
		return ctx
	}
	return clickhouse.Context(ctx, clickhouse.WithExternalTable(nonNil...))
}

func buildUsersConditionAndArgs(
	users []*userpb.User,
	targetTables []ClickhouseTable,
	useExtTable bool,
) (map[ClickhouseTable][]conditionAndArg, *ext.Table, error) {
	// Agent user ID filter.
	agentUserIDs := []string{}
	for _, user := range users {
		userName, err := userpb.ParseUserName(user.Name)
		if err != nil {
			return nil, nil, status.Error(codes.InvalidArgument, err.Error())
		}
		agentUserIDs = append(agentUserIDs, userName.UserID)
	}
	conditionsAndArgs := map[ClickhouseTable][]conditionAndArg{}
	if len(agentUserIDs) > 0 {
		if useExtTable {
			extTable, err := buildUsersExtTable(agentUserIDs, extTableAgentFilter, extTableAgentFilterColumn)
			if err != nil {
				return nil, nil, err
			}
			for _, t := range targetTables {
				colName := tableSpecificColumnName(t, agentUserIDColumn)
				condition := fmt.Sprintf("%s IN (SELECT %s FROM %s)", colName, extTableAgentFilterColumn, extTableAgentFilter)
				conditionsAndArgs[t] = append(conditionsAndArgs[t], conditionAndArg{condition: condition})
			}
			return conditionsAndArgs, extTable, nil
		}
		for _, t := range targetTables {
			colName := tableSpecificColumnName(t, agentUserIDColumn)
			con, arg := columnIn(colName, agentUserIDs)
			conditionsAndArgs[t] = append(conditionsAndArgs[t], conditionAndArg{condition: con, arg: arg})
		}
	}
	return conditionsAndArgs, nil, nil
}

func parseClickhouseFilter(
	ctx context.Context,
	timeRange *analyticspb.TimeRange,
	attribute *analyticspb.Attribute,
	targetTables []ClickhouseTable,
	useExtTable bool,
	opts ...ConversationQueryOption,
) (map[ClickhouseTable][]conditionAndArg, [][]conditionAndArg, [][]conditionAndArg, *ext.Table, error) {
	conditionsAndArgs := map[ClickhouseTable][]conditionAndArg{}
	options := getConversationQueryOptions(opts...)
	timeColumn := getConversationTimeRangeColumn(options)

	// Time range filter.
	if timeRange == nil {
		return nil, nil, nil, nil, status.Errorf(codes.InvalidArgument, "error when parse time range into sql conditions: must set time_range for attribute")
	}
	startTime, endTime, err := shared.ParseFilterByTimeRange(timeRange)
	if err != nil {
		return nil, nil, nil, nil, status.Errorf(codes.InvalidArgument, "error when parse time range: %v", err)
	}
	timeRangeCondAndArgs, err := buildTimeRangeConditionAndArgs(
		timeRange, targetTables, timeColumn,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	conditionsAndArgs = mergeTableConditionsAndArgs(conditionsAndArgs, timeRangeCondAndArgs)

	if attribute == nil {
		return conditionsAndArgs, nil, nil, nil, nil
	}

	// Agent user ID filter.
	usersCondAndArgs, usersExtTable, err := buildUsersConditionAndArgs(attribute.Users, targetTables, useExtTable)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	conditionsAndArgs = mergeTableConditionsAndArgs(conditionsAndArgs, usersCondAndArgs)

	// Behavior ID filter.
	behaviorIDs := []string{}
	for _, b := range attribute.Behaviors {
		behaviorName, err := behaviorpb.ParseBehaviorName(b.Name)
		if err != nil {
			return nil, nil, nil, nil, status.Error(codes.InvalidArgument, err.Error())
		}
		behaviorIDs = append(behaviorIDs, behaviorName.BehaviorID)
	}
	if len(behaviorIDs) > 0 {
		for _, t := range targetTables {
			colName := tableSpecificColumnName(t, behaviorIDColumn)
			if colName == ColumnNotExist {
				conditionsAndArgs[t] = append(conditionsAndArgs[t], conditionAndArg{condition: "false", arg: nil})
			} else {
				con, arg := columnIn(colName, behaviorIDs)
				conditionsAndArgs[t] = append(conditionsAndArgs[t], conditionAndArg{condition: con, arg: arg})
			}
		}
	}

	// Policy ID filter.
	policyIDs := []string{}
	if len(attribute.AttributePolicies) > 0 {
		for _, p := range attribute.AttributePolicies {
			policyName, err := policypb.ParsePolicyName(p.Name)
			if err != nil {
				return nil, nil, nil, nil, status.Error(codes.InvalidArgument, err.Error())
			}
			policyIDs = append(policyIDs, policyName.PolicyID)
		}
	}
	if len(policyIDs) > 0 {
		for _, t := range targetTables {
			colName := tableSpecificColumnName(t, policyIDColumn)
			if colName == ColumnNotExist {
				conditionsAndArgs[t] = append(conditionsAndArgs[t], conditionAndArg{condition: "false", arg: nil})
			} else {
				con, arg := columnIn(colName, policyIDs)
				conditionsAndArgs[t] = append(conditionsAndArgs[t], conditionAndArg{condition: con, arg: arg})
			}
		}
	}

	// Moment filter.
	momentTemplateIDs := []string{}
	for _, m := range attribute.Moments {
		momentName, err := momentpb.ParseMomentName(m.Name)
		if err != nil {
			return nil, nil, nil, nil, status.Error(codes.InvalidArgument, err.Error())
		}
		momentTemplateIDs = append(momentTemplateIDs, momentName.MomentID)
	}
	if len(momentTemplateIDs) > 0 {
		for _, t := range targetTables {
			colName := tableSpecificColumnName(t, momentTemplateIDColumn)
			if colName == ColumnNotExist {
				conditionsAndArgs[t] = append(conditionsAndArgs[t], conditionAndArg{condition: "false", arg: nil})
			} else {
				con, arg := columnIn(colName, momentTemplateIDs)
				conditionsAndArgs[t] = append(conditionsAndArgs[t], conditionAndArg{condition: con, arg: arg})
			}
		}
	}

	// Usecase ID filter.
	usecaseIDs, err := parseUsercaseNames(attribute.UsecaseNames)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if len(usecaseIDs) > 0 {
		for _, t := range targetTables {
			colName := tableSpecificColumnName(t, usecaseIDColumn)
			if colName == ColumnNotExist {
				conditionsAndArgs[t] = append(conditionsAndArgs[t], conditionAndArg{condition: "true", arg: nil})
			} else {
				con, arg := columnIn(colName, usecaseIDs)
				conditionsAndArgs[t] = append(conditionsAndArgs[t], conditionAndArg{condition: con, arg: arg})
			}
		}
	}

	// Language code filter.
	if len(attribute.LanguageCodes) > 0 {
		for _, t := range targetTables {
			colName := tableSpecificColumnName(t, languageCodeColumn)
			con, arg := columnIn(colName, attribute.LanguageCodes)
			conditionsAndArgs[t] = append(conditionsAndArgs[t], conditionAndArg{condition: con, arg: arg})
		}
	}

	// Conversation duration filter
	var durationFilters []*elasticsearch.FloatRange
	durationbuckets, durationRange := attribute.ConversationDurationBuckets, attribute.ConversationDurationRange
	if len(durationbuckets) > 0 && (durationRange != nil && (durationRange.MinDuration != nil || durationRange.MaxDuration != nil)) {
		return nil, nil, nil, nil, status.Errorf(codes.InvalidArgument, "cannot set both `conversation_duration_buckets` and `conversation_duration_range`.")
	}
	if len(durationbuckets) > 0 {
		referenceDurationBuckets := hardcodedConversationDurationBuckets
		durationFilters = parseDurationBuckets(durationbuckets, referenceDurationBuckets)
	}
	if durationRange != nil && (durationRange.MinDuration != nil || durationRange.MaxDuration != nil) {
		durationFilter := &elasticsearch.FloatRange{}
		if durationRange.MinDuration != nil {
			durationFilter.GTE = proto.Float64(float64(durationRange.MinDuration.Seconds))
		}
		if durationRange.MaxDuration != nil {
			durationFilter.LT = proto.Float64(float64(durationRange.MaxDuration.Seconds))
		}
		durationFilters = []*elasticsearch.FloatRange{durationFilter}
	}
	if len(durationFilters) > 0 {
		for _, t := range targetTables {
			colName := tableSpecificColumnName(t, conversationDurationSecsColumn)
			if colName == ColumnNotExist {
				conditionsAndArgs[t] = append(conditionsAndArgs[t], conditionAndArg{condition: "true", arg: nil})
			} else {
				conditionsAndArgs[t] = appendClickhouseFloatRangeConditions(conditionsAndArgs[t], colName, durationFilters)
			}
		}
	}

	// Note: only RetrieveHintStats API provides this filter.
	if hintType := attribute.HintType; hintType != analyticspb.Attribute_HINT_TYPE_UNSPECIFIED {
		actionType, err := postgres.HintTypeToActionType(hintType)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		for _, t := range targetTables {
			con, arg := columnEq(tableSpecificColumnName(t, actionTypeColumn), actionType)
			conditionsAndArgs[t] = append(conditionsAndArgs[t], conditionAndArg{condition: con, arg: arg})
			// Extra condition for remainder hint and regular hint.
			switch hintType {
			case analyticspb.Attribute_REGULAR_HINT:
				// Looking for remainder hint.
				con, arg := columnEq(tableSpecificColumnName(t, actionAdherenceTypeColumn), 0)
				conditionsAndArgs[t] = append(conditionsAndArgs[t], conditionAndArg{condition: con, arg: arg})
			case analyticspb.Attribute_BEHAVIOR_HINT:
				// Looking for sdx hint.
				con, arg := columnEq(tableSpecificColumnName(t, actionAdherenceTypeColumn), 1)
				conditionsAndArgs[t] = append(conditionsAndArgs[t], conditionAndArg{condition: con, arg: arg})
			}
		}
	}
	if excludeVoiceMail(attribute.MomentGroups, attribute.ConversationCriterions) {
		for _, t := range targetTables {
			colName := tableSpecificColumnName(t, isVoicemailColumn)
			con, arg := columnNotEq(colName, 1)
			conditionsAndArgs[t] = append(conditionsAndArgs[t], conditionAndArg{condition: con, arg: arg})
		}
	}

	// Moment group filter
	momentConditionsAndArgsList := [][]conditionAndArg{}
	excludedMomentConditionsAndArgsList := [][]conditionAndArg{}
	if len(attribute.MomentGroups) > 0 {
		for _, momentGroup := range attribute.MomentGroups {
			// As of 06/20/2024, we only support metadata filter (both included and excluded).
			if *enableExcludedMoments {
				if err := validateNoMetadataMomentOverlap(momentGroup); err != nil {
					return nil, nil, nil, nil, err
				}
			}
			if len(momentGroup.Moments) == 1 &&
				momentGroup.Moments[0].Type == momentpb.Moment_CONVERSATION_METADATA &&
				len(momentGroup.MetadataValueAttributes) > 0 {
				conditionsAndArgsList, err := convertMetadataMomentGroupFilter(momentGroup, startTime, endTime, false /*fetchFromMetadataView=*/, timeColumn)
				if err != nil {
					return nil, nil, nil, nil, err
				}
				momentConditionsAndArgsList = append(momentConditionsAndArgsList, conditionsAndArgsList)
			}
			if *enableExcludedMoments &&
				len(momentGroup.ExcludedMoments) == 1 &&
				momentGroup.ExcludedMoments[0].Type == momentpb.Moment_CONVERSATION_METADATA {
				excludedConditionsAndArgsList, err := convertExcludedMetadataMomentGroupFilter(momentGroup, startTime, endTime, false /*fetchFromMetadataView=*/, timeColumn)
				if err != nil {
					return nil, nil, nil, nil, err
				}
				excludedMomentConditionsAndArgsList = append(excludedMomentConditionsAndArgsList, excludedConditionsAndArgsList)
			}
		}
	}

	return conditionsAndArgs, momentConditionsAndArgsList, excludedMomentConditionsAndArgsList, usersExtTable, nil
}

// validateNoMetadataMomentOverlap rejects OR groups that the shared parser's
// separate include/exclude lists cannot represent, before either side is lowered.
func validateNoMetadataMomentOverlap(group *analyticspb.MomentGroup) error {
	if len(group.GetMoments()) == 0 || len(group.GetExcludedMoments()) == 0 ||
		!elasticsearch.IsConversationMetadataMomentGroup(group) {
		return nil
	}
	// Like Elasticsearch, classify the whole group before comparing identities:
	// an overlapping entry may omit its type on either side.
	includedIDs := make(map[string]struct{})
	for _, moment := range group.Moments {
		name, err := momentpb.ParseMomentName(moment.Name)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "unparsable included moment name %q: %v", moment.Name, err)
		}
		includedIDs[name.MomentID] = struct{}{}
	}
	for _, moment := range group.ExcludedMoments {
		name, err := momentpb.ParseMomentName(moment.Name)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "unparsable excluded moment name %q: %v", moment.Name, err)
		}
		if _, overlaps := includedIDs[name.MomentID]; overlaps {
			return status.Error(codes.InvalidArgument, "value-or-missing metadata filters are not supported for this ClickHouse endpoint")
		}
	}
	return nil
}

// The conditions used when querying the latest scorecards or scores.
// The conditions filter those unchangeable properties of the scorecards,
// like the usercase_id, create_time, scorecard_template_id, etc.
func parseCommonConditionsForQAAttribute(
	timeRange *analyticspb.TimeRange,
	qaAttribute *analyticspb.QAAttribute,
	table ClickhouseTable,
	useExtTable bool,
	opts ...ConversationQueryOption,
) ([]conditionAndArg, []*ext.Table, error) {
	// Process options with defaults
	options := getConversationQueryOptions(opts...)

	conditionsAndArgs := []conditionAndArg{}

	// Time range filter using the new column mapping approach
	if timeRange != nil {
		startTime, endTime, err := shared.ParseFilterByTimeRange(timeRange)
		if err != nil {
			return nil, nil, err
		}

		// Use the table-specific column name with options
		timeColumn := getConversationTimeRangeColumn(options)
		conversationTimeColumn := tableSpecificColumnName(table, timeColumn)
		if conversationTimeColumn == ColumnNotExist {
			if options.failClosedOnUnsupportedTimeRange {
				return nil, nil, status.Errorf(codes.InvalidArgument,
					"conversation_time_range_field %q cannot be applied to the requested score_resource: the table has no such column, "+
						"and bounding its start-time column instead drops conversations that start before the window and end inside it",
					timeColumn)
			}
			// fail-open: only conversation_d has conversation_end_time, so fall back to the
			// table's own start-time column rather than dropping the predicate. Dropping it
			// leaves score_d and scorecard_d unbounded, and toStartOfHour(scorecard_time) leads
			// their primary key, so an unbounded scan reads every granule of full history. The
			// fallback excludes conversations that start before the window and end inside it:
			// 0.54% of scorecards on marriott over a 30-day window, under 0.04% on rcg,
			// united_east and rentokil.
			conversationTimeColumn = tableSpecificColumnName(table, conversationStartTimeColumn)
		}

		if conversationTimeColumn == ColumnNotExist {
			// No time column at all. Log rather than return an error, so an unfilterable table
			// degrades to today's behaviour instead of failing the request outright.
			logger.Errorf(context.Background(),
				"no column on table %d can express time range %q; query will be unbounded", table, timeColumn)
		} else {
			conStart, argStart := columnGTE(conversationTimeColumn, startTime.Format(timestampFormat))
			conEnd, argEnd := columnLT(conversationTimeColumn, endTime.Format(timestampFormat))
			conditionsAndArgs = append(conditionsAndArgs,
				conditionAndArg{condition: conStart, arg: argStart},
				conditionAndArg{condition: conEnd, arg: argEnd},
			)
		}
	}

	if qaAttribute == nil {
		return conditionsAndArgs, nil, nil
	}

	// Agent user ID filter.
	usersExtTables := []*ext.Table{}
	agentUserIDs := []string{}
	for _, user := range qaAttribute.Users {
		userName, err := userpb.ParseUserName(user.Name)
		if err != nil {
			return nil, nil, status.Error(codes.InvalidArgument, err.Error())
		}
		agentUserIDs = append(agentUserIDs, userName.UserID)
	}
	if len(agentUserIDs) > 0 {
		colName := tableSpecificColumnName(table, agentUserIDColumn)
		if useExtTable {
			usersExtTable, err := buildUsersExtTable(agentUserIDs, extTableAgentFilter, extTableAgentFilterColumn)
			if err != nil {
				return nil, nil, err
			}
			usersExtTables = append(usersExtTables, usersExtTable)
			condition := fmt.Sprintf("%s IN (SELECT %s FROM %s)", colName, extTableAgentFilterColumn, extTableAgentFilter)
			conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: condition})
		} else {
			con, arg := columnIn(colName, agentUserIDs)
			conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: con, arg: arg})
		}
	}

	submitterUserIDs := []string{}
	if qaAttribute.GetScorecardReviewerAudience() != nil {
		for _, userNameStr := range qaAttribute.GetScorecardReviewerAudience().GetUsers() {
			userName, err := userpb.ParseUserName(userNameStr)
			if err != nil {
				return nil, nil, status.Error(codes.InvalidArgument, err.Error())
			}
			submitterUserIDs = append(submitterUserIDs, userName.UserID)
		}
	}
	if len(submitterUserIDs) > 0 {
		colName := tableSpecificColumnName(table, submitterUserIDColumn)
		if colName != ColumnNotExist {
			if useExtTable {
				submitterExtTable, err := buildUsersExtTable(submitterUserIDs, extTableSubmitterFilter, extTableSubmitterFilterColumn)
				if err != nil {
					return nil, nil, err
				}
				usersExtTables = append(usersExtTables, submitterExtTable)
				condition := fmt.Sprintf("%s IN (SELECT %s FROM %s)", colName, extTableSubmitterFilterColumn, extTableSubmitterFilter)
				conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: condition})
			} else {
				con, arg := columnIn(colName, submitterUserIDs)
				conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: con, arg: arg})
			}
		}
	}

	// Usecase ID filter.
	usecaseIDs, err := parseUsercaseNames(qaAttribute.UsecaseNames)
	if err != nil {
		return nil, nil, err
	}
	if len(usecaseIDs) > 0 {
		con, arg := columnIn(usecaseIDColumn, usecaseIDs)
		conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: con, arg: arg})
	}

	// Scorecard template filter
	if len(qaAttribute.ScorecardTemplates) > 0 && (table == scorecardTable || table == scoreTable) {
		templateResourceIds := make([]string, len(qaAttribute.ScorecardTemplates))
		for i, scorecardTemplateName := range qaAttribute.ScorecardTemplates {
			// TODO Director-Squad: decide on which format we expect for this endpoint (with or without revision, possibly with wildcard)
			templateNameWithRevision, err := coachingpb.ParseScorecardTemplateName(scorecardTemplateName)
			if err == nil {
				templateResourceIds[i] = templateNameWithRevision.ScorecardTemplateID
				continue
			}
			templateName, err := descriptors.ParseScorecardTemplateName(scorecardTemplateName)
			if err == nil {
				templateResourceIds[i] = templateName.TemplateID
				continue
			}
			return nil, nil, status.Error(codes.InvalidArgument, err.Error())
		}
		con, arg := columnIn(scorecardTemplateIDColumn, templateResourceIds)
		conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: con, arg: arg})
	}

	return conditionsAndArgs, usersExtTables, nil
}

// The conditions filter those unchangeable properties of the scorecards,
// such as the scorecard status, the scorecard can be changed from status
// DRAFT to status SUBMIT.
func parseScorecardConditionsForQAAttribute(
	qaAttribute *analyticspb.QAAttribute,
) ([]conditionAndArg, error) {
	conditionsAndArgs := []conditionAndArg{}

	// Scorecard status filter
	if len(qaAttribute.ScorecardStatuses) > 0 {
		tempConditionsAndArgs := []conditionAndArg{}
		for _, scorecardStatus := range qaAttribute.ScorecardStatuses {
			switch scorecardStatus {
			case analyticspb.QAAttribute_MANUALLY_SUBMITTED:
				// scorecard_submit_time not null
				conSubmitTime, argSubmitTime := columnNotEq(scorecardSubmitTimeColumn, 0)
				tempConditionsAndArgs = append(tempConditionsAndArgs, conditionAndArg{condition: conSubmitTime, arg: argSubmitTime})
			case analyticspb.QAAttribute_DRAFT:
				// scorecard_submit_time is null AND manually_scored is true
				conSubmitTime, argSubmitTime := columnEq(scorecardSubmitTimeColumn, 0)
				conManuallyScored, argManuallyScored := columnEq(manuallyScoredColumn, true)

				tempConditionsAndArgs = append(tempConditionsAndArgs, conditionAndArg{
					condition: fmt.Sprintf("%s AND %s", conSubmitTime, conManuallyScored),
					args:      []any{argSubmitTime, argManuallyScored},
				})
			case analyticspb.QAAttribute_AUTO:
				// scorecard_submit_time is null AND manually_scored is false
				conSubmitTime, argSubmitTime := columnEq(scorecardSubmitTimeColumn, 0)
				conManuallyScored, argManuallyScored := columnEq(manuallyScoredColumn, false)

				tempConditionsAndArgs = append(tempConditionsAndArgs, conditionAndArg{
					condition: fmt.Sprintf("%s AND %s", conSubmitTime, conManuallyScored),
					args:      []any{argSubmitTime, argManuallyScored},
				})
			case analyticspb.QAAttribute_PUBLISHED:
				conPublishTime, argPublishTime := columnNotEq(scorecardPublishTimeColumn, 0)
				tempConditionsAndArgs = append(tempConditionsAndArgs, conditionAndArg{condition: conPublishTime, arg: argPublishTime})
			case analyticspb.QAAttribute_NOT_PUBLISHED:
				conPublishTime, argPublishTime := columnEq(scorecardPublishTimeColumn, 0)
				conSubmitTime, argSubmitTime := columnNotEq(scorecardSubmitTimeColumn, 0)
				tempConditionsAndArgs = append(tempConditionsAndArgs, conditionAndArg{
					condition: fmt.Sprintf("(%s AND %s)", conPublishTime, conSubmitTime),
					args:      []any{argPublishTime, argSubmitTime},
				})
			default:
				return nil, status.Errorf(codes.InvalidArgument, "invalid qa status filter: %v", scorecardStatus)
			}
		}

		condition, args := concatConditionsAndArgsWithOR(tempConditionsAndArgs)
		conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{
			condition: condition,
			args:      args,
		})
	}
	return conditionsAndArgs, nil
}

func parseScoreConditionsForQAAttribute(
	timeRange *analyticspb.TimeRange,
	qaAttribute *analyticspb.QAAttribute,
	table ClickhouseTable,
	useExtTable bool,
	opts ...ConversationQueryOption,
) ([]conditionAndArg, []*ext.Table, error) {
	conditionsAndArgs, usersExtTables, err := parseCommonConditionsForQAAttribute(timeRange, qaAttribute, table, useExtTable, opts...)
	if err != nil {
		return nil, nil, err
	}

	// Include N/A scored filter
	if !qaAttribute.GetIncludeNaScored() {
		if table == scorecardTable {
			conPer, argPer := columnGTE(scoreColumn, 0)
			conditionsAndArgs = append(conditionsAndArgs,
				conditionAndArg{condition: conPer, arg: argPer},
			)
		} else {
			// We are using `-1` to represent `NULL` in ClickHouse table.
			conPer, argPer := columnGTE(percentageValueColumn, 0)
			conApp, argApp := columnNotEq(notApplicableColumn, true)
			conditionsAndArgs = append(conditionsAndArgs,
				conditionAndArg{condition: conPer, arg: argPer},
				conditionAndArg{condition: conApp, arg: argApp},
			)
		}
	}

	// Score type filter
	switch qaAttribute.ScoreType {
	case analyticspb.QAAttribute_SCORE_TYPE_MANUAL:
		con, arg := columnEq(manuallyScoredColumn, true)
		conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: con, arg: arg})
	case analyticspb.QAAttribute_SCORE_TYPE_AUTO:
		if table == scorecardTable {
			con, arg := columnNotEq(aiScoreTimeColumn, 0)
			conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: con, arg: arg})
		} else {
			con, arg := columnEq(aiScoredColumn, true)
			conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: con, arg: arg})
		}
	case analyticspb.QAAttribute_SCORE_TYPE_OVERRIDDEN:
		conMan, argMan := columnEq(manuallyScoredColumn, true)
		conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: conMan, arg: argMan})
		if table == scorecardTable {
			conAi, argAi := columnNotEq(aiScoreTimeColumn, 0)
			conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: conAi, arg: argAi})
		} else {
			conAi, argAi := columnEq(aiScoredColumn, true)
			conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: conAi, arg: argAi})
		}
	}

	// Score range filter
	if len(qaAttribute.ScoreRanges) > 0 {
		for _, scoreRange := range qaAttribute.ScoreRanges {
			scoreColumn := tableSpecificColumnName(table, scoreColumn)
			minScore := scoreRange.MinScore
			// The scale of the `score` column in scorecard table is [0, 100].
			if table == scorecardTable {
				minScore *= 100.
			}
			maxScore := scoreRange.MaxScore
			if table == scorecardTable {
				maxScore *= 100.
			}
			conGte, argGte := columnGTE(scoreColumn, minScore)
			conLte, argLte := columnLTE(scoreColumn, maxScore)
			conditionsAndArgs = append(conditionsAndArgs,
				conditionAndArg{condition: conGte, arg: argGte},
				conditionAndArg{condition: conLte, arg: argLte},
			)
		}
	}

	// Criterion filter, it only applied when the score resource is criteria score rather than scorecard score.
	if table == scoreTable {
		if criterionsFilter := qaAttribute.CriterionIdentifiers; len(criterionsFilter) > 0 {
			con, arg := columnIn(criterionIDColumn, criterionsFilter)
			conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: con, arg: arg})
		}
	}

	return conditionsAndArgs, usersExtTables, nil
}

func parseConversationConditionsForQAAttribute(
	ctx context.Context,
	timeRange *analyticspb.TimeRange,
	qaAttribute *analyticspb.QAAttribute,
	useExtTable bool,
	opts ...ConversationQueryOption,
) ([]conditionAndArg, []*ext.Table, error) {
	// Process options with defaults
	options := getConversationQueryOptions(opts...)
	conditionsAndArgs := []conditionAndArg{}

	if len(qaAttribute.ConversationDurationBuckets) > 0 {
		referenceDurationBuckets := hardcodedConversationDurationBuckets
		durationFilters := parseDurationBuckets(qaAttribute.ConversationDurationBuckets, referenceDurationBuckets)
		conditionsAndArgs = appendClickhouseFloatRangeConditions(conditionsAndArgs, conversationDurationSecsColumn, durationFilters)
	}

	// Voice mail filter
	if excludeVoiceMail(qaAttribute.MomentGroups, nil) {
		con, arg := columnNotEq(isVoicemailColumn, 1)
		conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: con, arg: arg})
	}

	if len(conditionsAndArgs) == 0 && options.timeRangeType != analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT {
		return conditionsAndArgs, nil, nil
	}

	// Adds some default conversation conditions if we need filter on conversation_d table.
	commonConditionsAndArgs, usersExtTables, err := parseCommonConditionsForQAAttribute(
		timeRange,
		qaAttribute,
		conversationTable,
		useExtTable,
		opts...,
	)
	if err != nil {
		return nil, nil, err
	}
	conditionsAndArgs = append(conditionsAndArgs, commonConditionsAndArgs...)

	return conditionsAndArgs, usersExtTables, nil
}

func parseMomentConditionsForQAAttribute(
	timeRange *analyticspb.TimeRange,
	qaAttribute *analyticspb.QAAttribute,
	useConversationOutcomeMV bool,
	timeColumn string,
) ([]momentAnnotationFilter, []momentAnnotationFilter, []qaMomentValueOrMissingFilter, error) {
	// Time range filter.
	var startTime, endTime time.Time
	if timeRange != nil {
		var err error
		startTime, endTime, err = shared.ParseFilterByTimeRange(timeRange)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	// Moment group filter
	momentFilters := []momentAnnotationFilter{}
	excludedMomentFilters := []momentAnnotationFilter{}
	valueOrMissingFilters := []qaMomentValueOrMissingFilter{}
	if len(qaAttribute.MomentGroups) > 0 {
		for _, momentGroup := range qaAttribute.MomentGroups {
			if *enableExcludedMoments && len(momentGroup.Moments) > 0 && len(momentGroup.ExcludedMoments) > 0 {
				valueOrMissingFilter, err := parseValueOrMissingMetadataMomentGroup(
					momentGroup,
					startTime,
					endTime,
					momentAnnotationMVSupportsTimeColumn(timeColumn),
					timeColumn,
				)
				if err != nil {
					return nil, nil, nil, err
				}
				if valueOrMissingFilter != nil {
					valueOrMissingFilters = append(valueOrMissingFilters, *valueOrMissingFilter)
					continue
				}

				return nil, nil, nil, status.Error(codes.InvalidArgument, "mixed included/excluded QA moment groups require one CONVERSATION_METADATA moment with the same moment ID and at least one metadata value")
			}
			if len(momentGroup.Moments) == 1 &&
				momentGroup.Moments[0].Type == momentpb.Moment_CONVERSATION_METADATA &&
				len(momentGroup.MetadataValueAttributes) > 0 {
				// Metadata MV lacks conversation_end_time; fall back to raw for end-time filters.
				fetchFromMetadataView := momentAnnotationMVSupportsTimeColumn(timeColumn)
				conditionsAndArgsList, err := convertMetadataMomentGroupFilter(momentGroup, startTime, endTime, fetchFromMetadataView, timeColumn)
				if err != nil {
					return nil, nil, nil, err
				}
				source := momentAnnotationFilterSourceRaw
				if fetchFromMetadataView {
					source = momentAnnotationFilterSourceMetadataMV
				}
				momentFilters = append(momentFilters, momentAnnotationFilter{
					conditions: conditionsAndArgsList,
					source:     source,
				})
			}
			if len(momentGroup.Moments) == 1 &&
				momentGroup.Moments[0].Type == momentpb.Moment_CONVERSATION_OUTCOME &&
				len(momentGroup.OutcomeValueAttributes) > 0 {
				conditionsAndArgsList, err := convertConversationOutcomeMomentGroupFilter(momentGroup, startTime, endTime, timeColumn, useConversationOutcomeMV)
				if err != nil {
					return nil, nil, nil, err
				}
				source := momentAnnotationFilterSourceRaw
				if useConversationOutcomeMV {
					source = momentAnnotationFilterSourceConversationOutcomeMV
				}
				momentFilters = append(momentFilters, momentAnnotationFilter{
					conditions: conditionsAndArgsList,
					source:     source,
				})
			}
			if *enableExcludedMoments &&
				len(momentGroup.ExcludedMoments) == 1 &&
				momentGroup.ExcludedMoments[0].Type == momentpb.Moment_CONVERSATION_METADATA {
				fetchFromMetadataView := momentAnnotationMVSupportsTimeColumn(timeColumn)
				excludedConditionsAndArgsList, err := convertExcludedMetadataMomentGroupFilter(momentGroup, startTime, endTime, fetchFromMetadataView, timeColumn)
				if err != nil {
					return nil, nil, nil, err
				}
				source := momentAnnotationFilterSourceRaw
				if fetchFromMetadataView {
					source = momentAnnotationFilterSourceMetadataMV
				}
				excludedMomentFilters = append(excludedMomentFilters, momentAnnotationFilter{
					conditions: excludedConditionsAndArgsList,
					source:     source,
				})
			}
		}
	}

	return momentFilters, excludedMomentFilters, valueOrMissingFilters, nil
}

// parseValueOrMissingMetadataMomentGroup parses the value-or-missing metadata shape emitted by the metadata filter UI.
func parseValueOrMissingMetadataMomentGroup(
	momentGroup *analyticspb.MomentGroup,
	startTime, endTime time.Time,
	fetchFromMetadataView bool,
	timeColumn string,
) (*qaMomentValueOrMissingFilter, error) {
	if len(momentGroup.Moments) != 1 ||
		len(momentGroup.ExcludedMoments) != 1 ||
		momentGroup.Moments[0].Type != momentpb.Moment_CONVERSATION_METADATA ||
		momentGroup.ExcludedMoments[0].Type != momentpb.Moment_CONVERSATION_METADATA ||
		len(momentGroup.MetadataValueAttributes) == 0 {
		return nil, nil
	}

	includedMomentName, err := momentpb.ParseMomentName(momentGroup.Moments[0].Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unparsable included moment name %q: %v", momentGroup.Moments[0].Name, err)
	}
	excludedMomentName, err := momentpb.ParseMomentName(momentGroup.ExcludedMoments[0].Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unparsable excluded moment name %q: %v", momentGroup.ExcludedMoments[0].Name, err)
	}
	if includedMomentName.MomentID != excludedMomentName.MomentID {
		return nil, nil
	}

	valueConditionsAndArgsList, err := convertMetadataMomentGroupFilter(
		momentGroup,
		startTime,
		endTime,
		fetchFromMetadataView,
		timeColumn,
	)
	if err != nil {
		return nil, err
	}
	missingConditionsAndArgsList, err := convertExcludedMetadataMomentGroupFilter(
		momentGroup,
		startTime,
		endTime,
		fetchFromMetadataView,
		timeColumn,
	)
	if err != nil {
		return nil, err
	}

	source := momentAnnotationFilterSourceRaw
	if fetchFromMetadataView {
		source = momentAnnotationFilterSourceMetadataMV
	}
	return &qaMomentValueOrMissingFilter{
		selectedValueFilter: momentAnnotationFilter{
			conditions: valueConditionsAndArgsList,
			source:     source,
		},
		existenceFilter: momentAnnotationFilter{
			conditions: missingConditionsAndArgsList,
			source:     source,
		},
	}, nil
}

func excludeVoiceMail(momentGroups []*analyticspb.MomentGroup, conversationCriterons []analyticspb.ConversationCriterion) bool {
	for _, mg := range momentGroups {
		for _, m := range mg.ExcludedMoments {
			if m.Type == momentpb.Moment_VOICE_MAIL {
				return true
			}
		}
	}
	for _, val := range conversationCriterons {
		if val == analyticspb.ConversationCriterion_EXCLUDE_VOICE_MAIL {
			return true
		}
	}
	return false
}

// Parse the group by clause and and group by columns and subqueries.
func parseClickhouseGroupBy(
	frequency *analyticspb.Frequency,
	groupByAttributes []analyticspb.AttributeType,
	metadata *analyticspb.Metadata,
	targetTables []ClickhouseTable,
	conversationQueryOptions ...ConversationQueryOption,
) ([]string, map[ClickhouseTable][]string, error) {
	groupByKeys, groupByQueries := []string{}, map[ClickhouseTable][]string{}
	options := getConversationQueryOptions(conversationQueryOptions...)
	if *frequency != analyticspb.Frequency_FREQUENCY_UNSPECIFIED {
		var (
			freq, timeZoneID   string
			firstDayOfWeekBias int
			err                error
		)
		freq, err = shared.ParseFrequency(frequency)
		if err != nil {
			return nil, nil, err
		}
		firstDayOfWeekBias = shared.ParseStartDayOfWeek(metadata, freq == weekFrequency)
		timeZoneID = shared.DefaultTimeZone
		if metadata != nil && metadata.TimeZoneId != "" {
			timeZoneID = metadata.TimeZoneId
		}
		offsetSecs, err := shared.TimezoneOffsetSecs(timeZoneID)
		if err != nil {
			return nil, nil, err
		}

		timeColumn := getConversationTimeRangeColumn(options)
		groupByKeys = append(groupByKeys, truncatedTimeColumn)
		for _, t := range targetTables {
			query := truncatedTimestamp(tableSpecificColumnName(t, timeColumn), truncatedTimeColumn, freq, offsetSecs/3600, firstDayOfWeekBias)
			groupByQueries[t] = append(groupByQueries[t], query)
		}
	}

	for _, groupByAttribute := range groupByAttributes {
		groupByKey := ""
		switch groupByAttribute {
		case analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT:
			groupByKey = agentUserIDColumn
		case analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR:
			groupByKey = behaviorIDColumn
		case analyticspb.AttributeType_ATTRIBUTE_TYPE_POLICY:
			groupByKey = policyIDColumn
		case analyticspb.AttributeType_ATTRIBUTE_TYPE_MOMENT_POLICY:
			groupByKey = momentTemplateIDColumn
		case analyticspb.AttributeType_ATTRIBUTE_TYPE_INTENT:
			groupByKey = momentTemplateIDColumn
		case analyticspb.AttributeType_ATTRIBUTE_TYPE_GUIDED_WORKFLOW:
			groupByKey = guidedWorkflowNameColumn
		case analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE:
			// Skipping here, should pass the frequency field for group by time.
			continue
		default:
			return nil, nil, status.Errorf(codes.InvalidArgument, "group by attribute %v is not supported in clickhouse query", groupByAttribute)
		}
		if groupByKey != "" {
			groupByKeys = append(groupByKeys, groupByKey)
			for _, t := range targetTables {
				query := groupByKey
				colName := tableSpecificColumnName(t, groupByKey)
				if colName == ColumnNotExist {
					query = fmt.Sprintf("'non-exist' AS %s", groupByKey)
				} else if groupByKey != colName {
					query = fmt.Sprintf("%s AS %s", colName, groupByKey)
				}
				groupByQueries[t] = append(groupByQueries[t], query)
			}
		}
	}
	return groupByKeys, groupByQueries, nil
}

// Parse the group by clause and and group by columns and subqueries.
func parseClickhouseGroupByForQAAttribute(
	frequency *analyticspb.Frequency,
	groupByQaAttributes []analyticspb.QAAttributeType,
	metadata *analyticspb.Metadata,
	table ClickhouseTable,
	conversationQueryOptions ...ConversationQueryOption,
) ([]string, []string, error) {
	options := getConversationQueryOptions(conversationQueryOptions...)

	groupByKeys, groupByQueries := []string{}, []string{}
	if slices.Contains(groupByQaAttributes, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE) &&
		*frequency != analyticspb.Frequency_FREQUENCY_UNSPECIFIED {

		groupByKeys = append(groupByKeys, truncatedTimeColumn)
		groupByColumn := getConversationTimeRangeColumn(options)
		conversationTimeColumn := tableSpecificColumnName(table, groupByColumn)
		if conversationTimeColumn == ColumnNotExist {
			// ColumnNotExist happens when we want to group by the end time of the conversation, but that only exists on the conversation table
			// In this case, we just use the end time from the conversation table.
			conversationTimeColumn = groupByColumn
		}

		query, err := parseClickhouseGroupByForTimeColumn(frequency, metadata, conversationTimeColumn, truncatedTimeColumn)
		if err != nil {
			return nil, nil, err
		}
		groupByQueries = append(groupByQueries, query)
	}

	// Grouping by QA attributes.
	for _, groupByQaAttribute := range groupByQaAttributes {
		var keysToAdd []string
		switch groupByQaAttribute {
		case analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT:
			keysToAdd = []string{agentUserIDColumn}
		case analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT_TIER:
			keysToAdd = []string{agentUserIDColumn}
		case analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER:
			keysToAdd = []string{submitterUserIDColumn}
		case analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE:
			// Skipping here, should pass the frequency field for groupp by time.
			continue
		case analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION:
			// Group by (scorecard_template_id, criterion_id): criterion ids aren't unique
			// across templates (CONVI-7014), so a shared id must not collapse into one row.
			keysToAdd = []string{criterionIDColumn, scorecardTemplateIDColumn}
		default:
			return nil, nil, status.Errorf(codes.InvalidArgument, "group by QA attribute %v is not supported in clickhouse query", groupByQaAttribute)
		}
		for _, groupByKey := range keysToAdd {
			groupByKeys = append(groupByKeys, groupByKey)

			query := groupByKey
			colName := tableSpecificColumnName(table, groupByKey)
			if colName == ColumnNotExist {
				query = fmt.Sprintf("NULL AS %s", groupByKey)
			} else if colName != groupByKey {
				query = fmt.Sprintf("%s AS %s", colName, groupByKey)
			}
			groupByQueries = append(groupByQueries, query)
		}
	}

	return groupByKeys, groupByQueries, nil
}

func parseClickhouseGroupByForTimeColumn(frequency *analyticspb.Frequency, metadata *analyticspb.Metadata, timeColumn string, aliasColumnName string) (string, error) {
	var (
		freq, timeZoneID   string
		firstDayOfWeekBias int
		err                error
	)

	freq, err = shared.ParseFrequency(frequency)
	if err != nil {
		return "", err
	}

	firstDayOfWeekBias = shared.ParseStartDayOfWeek(metadata, freq == weekFrequency)

	timeZoneID = shared.DefaultTimeZone
	if metadata != nil && metadata.TimeZoneId != "" {
		timeZoneID = metadata.TimeZoneId
	}
	offsetSecs, err := shared.TimezoneOffsetSecs(timeZoneID)
	if err != nil {
		return "", err
	}

	query := truncatedTimestamp(timeColumn, aliasColumnName, freq, offsetSecs/3600, firstDayOfWeekBias)

	return query, nil
}

func attributesToIgnore(db *gorm.DB, attributes []*analyticspb.Attribute) (map[*analyticspb.Attribute]bool, error) {
	ret := map[*analyticspb.Attribute]bool{}
	if len(attributes) == 0 {
		return ret, nil
	}

	policyIDToAttribute := map[string]*analyticspb.Attribute{}
	policyIDs := []string{}
	for _, a := range attributes {
		if p := a.AttributePolicies; len(p) > 0 {
			name, err := policypb.ParsePolicyName(p[0].Name)
			if err != nil {
				return nil, err
			}
			policyIDToAttribute[name.PolicyID] = a
			policyIDs = append(policyIDs, name.PolicyID)
		}
	}
	policies := postgres.ListPolicies(db, policyIDs)

	momentToPolicy := map[string]string{}
	momentIDs := []string{}
	for _, p := range policies {
		// adherence moments of a policy.
		for _, m := range p.AdherenceMoments {
			momentToPolicy[m] = p.PolicyID
			momentIDs = append(momentIDs, m)
		}
		// TODO(zhucaiyi): handle moments from trigger moments of policies.
	}

	moments := postgres.ListMoments(db, momentIDs)
	hiddenMoments := []string{}
	for _, m := range moments {
		if m.Labels.Message != nil {
			if m.Labels.Message.Labels["hiding_in_performance_insights"] == TrueString || (m.TaxonomyState.Int32 == 3) {
				hiddenMoments = append(hiddenMoments, m.MomentTemplateID)
			}
		}
	}

	// Hide a policy when at least one of its moment has label hiding_in_performance_insights or taxonomy_state is `deprecated`.
	for _, momentID := range hiddenMoments {
		attr := policyIDToAttribute[momentToPolicy[momentID]]
		if attr != nil {
			ret[attr] = true
		}
	}
	return ret, nil
}

type groupByAttribute struct {
	policyID, agentUserID, behaviorID, criterionID, momentTemplateID, guidedWorkflowName string
}

type clickhouseKeyTimeRow struct {
	policyID, agentUserID, submitterUserID, behaviorID, criterionID, scorecardTemplateID, momentTemplateID, guidedWorkflowName string
	truncatedTime                                                                                                              time.Time
}

func (r clickhouseKeyTimeRow) GetKeyTimeString() string {
	return fmt.Sprintf("%s_%s_%s_%s_%s_%s@%d",
		r.policyID, r.agentUserID, r.submitterUserID, r.behaviorID, r.criterionID, r.momentTemplateID,
		r.truncatedTime.Unix())
}

func (r clickhouseKeyTimeRow) GetGroupByAttribute() groupByAttribute {
	return groupByAttribute{
		policyID:           r.policyID,
		agentUserID:        r.agentUserID,
		criterionID:        r.criterionID,
		behaviorID:         r.behaviorID,
		momentTemplateID:   r.momentTemplateID,
		guidedWorkflowName: r.guidedWorkflowName,
	}
}

func (r groupByAttribute) GetAttribute(customerID, profileID string) *analyticspb.Attribute {
	ret := &analyticspb.Attribute{}
	if r.policyID != "" {
		ret.AttributePolicies = append(ret.AttributePolicies, &analyticspb.Attribute_AttributePolicy{
			Name: policypb.PolicyName{
				CustomerID: customerID,
				ProfileID:  profileID,
				PolicyID:   r.policyID,
			}.String(),
		})
	}
	if r.agentUserID != "" {
		ret.Users = append(ret.Users, &userpb.User{
			Name: userpb.UserName{
				CustomerID: customerID,
				UserID:     r.agentUserID,
			}.String(),
		})
	}
	if r.behaviorID != "" {
		ret.Behaviors = append(ret.Behaviors, &behaviorpb.Behavior{
			Name: behaviorpb.BehaviorName{
				CustomerID: customerID,
				ProfileID:  profileID,
				BehaviorID: r.behaviorID,
			}.String(),
		})
	}
	if r.momentTemplateID != "" {
		ret.Moments = append(ret.Moments, &momentpb.Moment{
			Name: momentpb.MomentName{
				CustomerID: customerID,
				ProfileID:  profileID,
				MomentID:   r.momentTemplateID,
			}.String(),
		})
	}
	if r.guidedWorkflowName != "" {
		ret.GuidedWorkflowNames = append(ret.GuidedWorkflowNames, r.guidedWorkflowName)
	}
	return ret
}

func (r groupByAttribute) GetQAAttribute(customerID string) *analyticspb.QAAttribute {
	ret := &analyticspb.QAAttribute{}
	if r.criterionID != "" {
		ret.CriterionIdentifiers = append(ret.CriterionIdentifiers, r.criterionID)
	}
	if r.agentUserID != "" {
		ret.Users = append(ret.Users, &userpb.User{
			Name: userpb.UserName{
				CustomerID: customerID,
				UserID:     r.agentUserID,
			}.String(),
		})
	}
	return ret
}

func convertAttributeToGroupByAttribute(attr *analyticspb.Attribute) (groupByAttribute, error) {
	ret := groupByAttribute{}
	if attr != nil {
		// Postgres may set resource name to * when only group by time.
		if val := attr.AttributePolicies; len(val) > 0 {
			if n := val[0].Name; n != "*" {
				name, err := policypb.ParsePolicyName(n)
				if err != nil {
					return groupByAttribute{}, err
				}
				ret.policyID = name.PolicyID
			}
		}
		if val := attr.Users; len(val) > 0 {
			if n := val[0].Name; n != "*" {
				name, err := userpb.ParseUserName(n)
				if err != nil {
					return groupByAttribute{}, err
				}
				ret.agentUserID = name.UserID
			}
		}
		if val := attr.Behaviors; len(val) > 0 {
			if n := val[0].Name; n != "*" {
				name, err := behaviorpb.ParseBehaviorName(n)
				if err != nil {
					return groupByAttribute{}, err
				}
				ret.behaviorID = name.BehaviorID
			}
		}
		if val := attr.Moments; len(val) > 0 {
			if n := val[0].Name; n != "*" {
				name, err := momentpb.ParseMomentName(n)
				if err != nil {
					return groupByAttribute{}, err
				}
				ret.behaviorID = name.MomentID
			}
		}
		if val := attr.GuidedWorkflowNames; len(val) > 0 {
			if n := val[0]; n != "*" {
				ret.guidedWorkflowName = n
			}
		}
	}

	return ret, nil
}

func convertMetadataMomentGroupFilter(
	momentGroup *analyticspb.MomentGroup,
	startTime, endTime time.Time,
	fetchFromMetadataView bool,
	timeColumn string,
) ([]conditionAndArg, error) {
	momentConditionsAndArgs := []conditionAndArg{}
	if !startTime.IsZero() && !endTime.IsZero() {
		momentConStart, momentArgStart := columnGTE(timeColumn, startTime.Format(timestampFormat))
		momentConEnd, momentArgEnd := columnLT(timeColumn, endTime.Format(timestampFormat))
		momentConditionsAndArgs = append(momentConditionsAndArgs,
			conditionAndArg{condition: momentConStart, arg: momentArgStart},
			conditionAndArg{condition: momentConEnd, arg: momentArgEnd},
		)
	}

	if !fetchFromMetadataView {
		// moment_type == 19 -- Moment_CONVERSATION_METADATA
		metadataTypeCon, metadataTypeArg := columnEq(momentTypeColumn, int32(momentpb.Moment_CONVERSATION_METADATA))
		momentConditionsAndArgs = append(momentConditionsAndArgs, conditionAndArg{condition: metadataTypeCon, arg: metadataTypeArg})
	}

	// moment_template_id
	momentName, err := momentpb.ParseMomentName(momentGroup.Moments[0].Name)
	if err != nil {
		return nil, err
	}
	if momentName.MomentID != "*" {
		momentNameCon, momentNameArg := columnEq(momentTemplateIDColumn, momentName.MomentID)
		momentConditionsAndArgs = append(momentConditionsAndArgs, conditionAndArg{condition: momentNameCon, arg: momentNameArg})
	}

	// metadata value attributes
	metadataValueConditions := []string{}
	metadataValueArgs := []any{}
	for _, metadataValueAttribute := range momentGroup.MetadataValueAttributes {
		switch {
		// metadata_number_value on numeric bins.
		case metadataValueAttribute.GetNumericBin() != nil:
			numberBin := metadataValueAttribute.GetNumericBin()
			fromNumberCon, fromNumberArg := columnGTE(metadataNumberValueColumn, float64(numberBin.FromValue))
			if numberBin.FromValueIsExclusive {
				fromNumberCon, fromNumberArg = columnGT(metadataNumberValueColumn, float64(numberBin.FromValue))
			}
			toNumberCon, toNumberArg := columnLT(metadataNumberValueColumn, float64(numberBin.ToValue))
			if numberBin.ToValueIsInclusive {
				toNumberCon, toNumberArg = columnLTE(metadataNumberValueColumn, float64(numberBin.ToValue))
			}
			metadataValueConditions = append(metadataValueConditions, fmt.Sprintf("(%s AND %s)", fromNumberCon, toNumberCon))
			metadataValueArgs = append(metadataValueArgs, fromNumberArg, toNumberArg)
		case metadataValueAttribute.GetMetadataValue() != nil:
			var metadataValueCon string
			var metadataValueArg any
			switch v := metadataValueAttribute.GetMetadataValue().GetValue().(type) {
			case *metadatapb.MetadataValue_BooleanValue:
				metadataValueCon, metadataValueArg = columnEq(metadataBoolValueColumn, v.BooleanValue)
			case *metadatapb.MetadataValue_NumberValue:
				metadataValueCon, metadataValueArg = columnEq(metadataNumberValueColumn, v.NumberValue)
			case *metadatapb.MetadataValue_StringValue:
				metadataValueCon, metadataValueArg = columnEq(metadataStringValueColumn, v.StringValue)
			default:
				return nil, status.Error(codes.InvalidArgument, "metadata_value must set a boolean, number, or string value")
			}
			metadataValueConditions = append(metadataValueConditions, metadataValueCon)
			metadataValueArgs = append(metadataValueArgs, metadataValueArg)
		default:
			return nil, status.Error(codes.InvalidArgument, "both numeric_bin and metadata_value are missing in the metadata attribute.")
		}
	}
	jointMetadataValueConditions := strings.Join(metadataValueConditions, " OR ")
	momentConditionsAndArgs = append(momentConditionsAndArgs, conditionAndArg{condition: jointMetadataValueConditions, args: metadataValueArgs})
	return momentConditionsAndArgs, nil
}

func convertConversationOutcomeMomentGroupFilter(
	momentGroup *analyticspb.MomentGroup,
	startTime, endTime time.Time,
	timeColumn string,
	useConversationOutcomeMV bool,
) ([]conditionAndArg, error) {
	momentConditionsAndArgs := []conditionAndArg{}
	if !startTime.IsZero() && !endTime.IsZero() {
		momentConStart, momentArgStart := columnGTE(timeColumn, startTime.Format(timestampFormat))
		momentConEnd, momentArgEnd := columnLT(timeColumn, endTime.Format(timestampFormat))
		momentConditionsAndArgs = append(momentConditionsAndArgs,
			conditionAndArg{condition: momentConStart, arg: momentArgStart},
			conditionAndArg{condition: momentConEnd, arg: momentArgEnd},
		)
	}

	if !useConversationOutcomeMV {
		outcomeTypeCon, outcomeTypeArg := columnEq(momentTypeColumn, int32(momentpb.Moment_CONVERSATION_OUTCOME))
		momentConditionsAndArgs = append(momentConditionsAndArgs, conditionAndArg{condition: outcomeTypeCon, arg: outcomeTypeArg})
	}

	momentName, err := momentpb.ParseMomentName(momentGroup.Moments[0].Name)
	if err != nil {
		return nil, err
	}
	if momentName.MomentID != "*" {
		momentNameCon, momentNameArg := columnEq(momentTemplateIDColumn, momentName.MomentID)
		momentConditionsAndArgs = append(momentConditionsAndArgs, conditionAndArg{condition: momentNameCon, arg: momentNameArg})
	}

	valueConditions := []string{}
	valueArgs := []any{}
	for _, outcomeValueAttribute := range momentGroup.OutcomeValueAttributes {
		valueConditionAndArg, err := convertConversationOutcomeValueAttribute(outcomeValueAttribute, useConversationOutcomeMV)
		if err != nil {
			return nil, err
		}
		valueConditions = append(valueConditions, valueConditionAndArg.condition)
		valueArgs = append(valueArgs, valueConditionAndArg.args...)
	}
	jointValueConditions := strings.Join(valueConditions, " OR ")
	momentConditionsAndArgs = append(momentConditionsAndArgs, conditionAndArg{condition: jointValueConditions, args: valueArgs})
	return momentConditionsAndArgs, nil
}

func convertConversationOutcomeValueAttribute(outcomeValueAttribute *analyticspb.OutcomeValueAttribute, useConversationOutcomeMV bool) (conditionAndArg, error) {
	switch {
	case outcomeValueAttribute.NumericBin != nil:
		return convertConversationOutcomeNumericBin(outcomeValueAttribute.NumericBin, useConversationOutcomeMV), nil
	case outcomeValueAttribute.GetOutcomeValue() != nil && outcomeValueAttribute.GetOutcomeValue().GetValue() != nil:
		return convertConversationOutcomeValue(outcomeValueAttribute.GetOutcomeValue().GetValue(), useConversationOutcomeMV)
	default:
		return conditionAndArg{}, status.Error(codes.InvalidArgument, "both numeric_bin and outcome_value are missing in the outcome attribute.")
	}
}

func convertConversationOutcomeNumericBin(numberBin *metadatapb.NumericBin, useConversationOutcomeMV bool) conditionAndArg {
	numberValueExpression := conversationOutcomeValueExpression("JSONExtractFloat", conversationOutcomeNumberKey)
	if useConversationOutcomeMV {
		numberValueExpression = outcomeNumberValueColumn
	}
	fromNumberCon, fromNumberArg := numericBinLowerBoundCondition(numberValueExpression, float64(numberBin.FromValue), numberBin.FromValueIsExclusive)
	toNumberCon, toNumberArg := numericBinUpperBoundCondition(numberValueExpression, float64(numberBin.ToValue), numberBin.ToValueIsInclusive)
	if useConversationOutcomeMV {
		typeCon, typeArg := columnEq(outcomeValueTypeColumn, outcomeValueTypeNumber)
		return conditionAndArg{
			condition: fmt.Sprintf("(%s AND %s AND %s)", typeCon, fromNumberCon, toNumberCon),
			args:      []any{typeArg, fromNumberArg, toNumberArg},
		}
	}
	keyExistsCon, keyExistsArg := columnEq(conversationOutcomeValueExistsExpression(conversationOutcomeNumberKey), true)
	return conditionAndArg{
		condition: fmt.Sprintf("(%s AND %s AND %s)", keyExistsCon, fromNumberCon, toNumberCon),
		args:      []any{keyExistsArg, fromNumberArg, toNumberArg},
	}
}

func convertConversationOutcomeValue(value *momentpb.ConversationOutcomeValue, useConversationOutcomeMV bool) (conditionAndArg, error) {
	switch v := value.Value.(type) {
	case *momentpb.ConversationOutcomeValue_BooleanValue:
		if useConversationOutcomeMV {
			typeCon, typeArg := columnEq(outcomeValueTypeColumn, outcomeValueTypeBoolean)
			valueCon, valueArg := columnEq(outcomeBoolValueColumn, v.BooleanValue)
			return conditionAndArg{
				condition: fmt.Sprintf("(%s AND %s)", typeCon, valueCon),
				args:      []any{typeArg, valueArg},
			}, nil
		}
		valueCon, valueArg := columnEq(conversationOutcomeValueExpression("JSONExtractBool", conversationOutcomeBoolKey), v.BooleanValue)
		keyExistsCon, keyExistsArg := columnEq(conversationOutcomeValueExistsExpression(conversationOutcomeBoolKey), true)
		return conditionAndArg{
			condition: fmt.Sprintf("(%s AND %s)", keyExistsCon, valueCon),
			args:      []any{keyExistsArg, valueArg},
		}, nil
	case *momentpb.ConversationOutcomeValue_NumberValue:
		if useConversationOutcomeMV {
			typeCon, typeArg := columnEq(outcomeValueTypeColumn, outcomeValueTypeNumber)
			valueCon, valueArg := columnEq(outcomeNumberValueColumn, float64(v.NumberValue))
			return conditionAndArg{
				condition: fmt.Sprintf("(%s AND %s)", typeCon, valueCon),
				args:      []any{typeArg, valueArg},
			}, nil
		}
		valueCon, valueArg := columnEq(conversationOutcomeValueExpression("JSONExtractFloat", conversationOutcomeNumberKey), float64(v.NumberValue))
		keyExistsCon, keyExistsArg := columnEq(conversationOutcomeValueExistsExpression(conversationOutcomeNumberKey), true)
		return conditionAndArg{
			condition: fmt.Sprintf("(%s AND %s)", keyExistsCon, valueCon),
			args:      []any{keyExistsArg, valueArg},
		}, nil
	case *momentpb.ConversationOutcomeValue_StringValue:
		if useConversationOutcomeMV {
			typeCon, typeArg := columnEq(outcomeValueTypeColumn, outcomeValueTypeString)
			valueCon, valueArg := columnEq(outcomeStringValueColumn, v.StringValue)
			return conditionAndArg{
				condition: fmt.Sprintf("(%s AND %s)", typeCon, valueCon),
				args:      []any{typeArg, valueArg},
			}, nil
		}
		valueCon, valueArg := columnEq(conversationOutcomeValueExpression("JSONExtractString", conversationOutcomeStringKey), v.StringValue)
		keyExistsCon, keyExistsArg := columnEq(conversationOutcomeValueExistsExpression(conversationOutcomeStringKey), true)
		return conditionAndArg{
			condition: fmt.Sprintf("(%s AND %s)", keyExistsCon, valueCon),
			args:      []any{keyExistsArg, valueArg},
		}, nil
	default:
		return conditionAndArg{}, status.Errorf(codes.InvalidArgument, "unsupported conversation outcome value %v", v)
	}
}

func numericBinLowerBoundCondition(columnName string, lowerbound float64, isExclusive bool) (string, any) {
	if isExclusive {
		return columnGT(columnName, lowerbound)
	}
	return columnGTE(columnName, lowerbound)
}

func numericBinUpperBoundCondition(columnName string, upperbound float64, isInclusive bool) (string, any) {
	if isInclusive {
		return columnLTE(columnName, upperbound)
	}
	return columnLT(columnName, upperbound)
}

func conversationOutcomeValueExpression(extractFunction, valueKey string) string {
	return fmt.Sprintf("%s(%s, '%s', '%s')", extractFunction, momentAnnotationPayloadColumn, conversationOutcomePayloadKey, valueKey)
}

func conversationOutcomeValueExistsExpression(valueKey string) string {
	return fmt.Sprintf("JSONHas(%s, '%s', '%s')", momentAnnotationPayloadColumn, conversationOutcomePayloadKey, valueKey)
}

func convertExcludedMetadataMomentGroupFilter(
	momentGroup *analyticspb.MomentGroup,
	startTime, endTime time.Time,
	fetchFromMetadataView bool,
	timeColumn string,
) ([]conditionAndArg, error) {
	conditionsAndArgs := []conditionAndArg{}
	if !startTime.IsZero() && !endTime.IsZero() {
		momentConStart, momentArgStart := columnGTE(timeColumn, startTime.Format(timestampFormat))
		momentConEnd, momentArgEnd := columnLT(timeColumn, endTime.Format(timestampFormat))
		conditionsAndArgs = append(conditionsAndArgs,
			conditionAndArg{condition: momentConStart, arg: momentArgStart},
			conditionAndArg{condition: momentConEnd, arg: momentArgEnd},
		)
	}

	if !fetchFromMetadataView {
		// moment_type == 19 -- Moment_CONVERSATION_METADATA
		metadataTypeCon, metadataTypeArg := columnEq(momentTypeColumn, int32(momentpb.Moment_CONVERSATION_METADATA))
		conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: metadataTypeCon, arg: metadataTypeArg})
	}

	// moment_template_id to exclude
	momentName, err := momentpb.ParseMomentName(momentGroup.ExcludedMoments[0].Name)
	if err != nil {
		return nil, err
	}
	if momentName.MomentID != "*" {
		momentNameCon, momentNameArg := columnEq(momentTemplateIDColumn, momentName.MomentID)
		conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: momentNameCon, arg: momentNameArg})
	}

	return conditionsAndArgs, nil
}

// eg: DATE_TRUNC('hour', toDateTime(create_time + toIntervalDay(1), 'America/Toronto')) - toIntervalDay(1)  AS truncated_time
func truncatedTimestamp(columnName, aliasColumnName string, frequency string, offsetHours int, firstDayOfWeek int) string {
	return fmt.Sprintf("DATE_TRUNC('%v', %v + toIntervalDay(%v) + toIntervalHour(%v)) - toIntervalDay(%v) - toIntervalHour(%v) AS %v", frequency, columnName, firstDayOfWeek, offsetHours, firstDayOfWeek, offsetHours, aliasColumnName)
}

func columnIn(columnName string, values []string) (string, any) {
	sort.Strings(values)
	return fmt.Sprintf("%s IN (?)", columnName), values
}

func columnGTE[T any](columnName string, lowerbound T) (string, any) {
	return fmt.Sprintf("%s >= ?", columnName), lowerbound
}

func columnGT[T any](columnName string, lowerbound T) (string, any) {
	return fmt.Sprintf("%s > ?", columnName), lowerbound
}

func columnLT[T any](columnName string, upperbound T) (string, any) {
	return fmt.Sprintf("%s < ?", columnName), upperbound
}

func columnLTE[T any](columnName string, upperbound T) (string, any) {
	return fmt.Sprintf("%s <= ?", columnName), upperbound
}

func columnEq[T any](columnName string, val T) (string, any) {
	return fmt.Sprintf("%s = ?", columnName), val
}

func qaAttributeHasConversationOutcomeMomentGroup(qaAttribute *analyticspb.QAAttribute) bool {
	if qaAttribute == nil {
		return false
	}
	for _, momentGroup := range qaAttribute.MomentGroups {
		if len(momentGroup.Moments) == 1 && momentGroup.Moments[0].Type == momentpb.Moment_CONVERSATION_OUTCOME {
			return true
		}
	}
	return false
}

func columnNotEq[T any](columnName string, val T) (string, any) {
	return fmt.Sprintf("%s <> ?", columnName), val
}

const ColumnNotExist = "column-not-exist"

var tableColumnNameMapping = map[ClickhouseTable]map[string]string{
	// The tables in `conversations` cluster.
	conversationTable: {
		submitterUserIDColumn: ColumnNotExist,
	},
	momentAnnotationTable: {
		actionTypeColumn:          "adherence_action_type",
		actionAdherenceTypeColumn: "adherence_action_adherence_type",
	},
	actionAnnotationTable: {
		actionAdherenceTypeColumn: "adherence_type",
		behaviorIDColumn:          ColumnNotExist,
		momentTemplateIDColumn:    ColumnNotExist,
	},
	conversationEventTable: {
		policyIDColumn:         "action_policy_id",
		behaviorIDColumn:       ColumnNotExist,
		momentTemplateIDColumn: ColumnNotExist,
	},
	scorecardScoreTable: {
		conversationStartTimeColumn: conversationFormattedTimeColumn,
		conversationEndTimeColumn:   ColumnNotExist,
		scoreColumn:                 percentageValueColumn,
	},
	scoreTable: {
		conversationStartTimeColumn: scorecardTimeColumn,
		conversationEndTimeColumn:   ColumnNotExist,
		scoreColumn:                 percentageValueColumn,
	},
	scorecardTable: {
		conversationStartTimeColumn: scorecardTimeColumn,
		conversationEndTimeColumn:   ColumnNotExist,
		scoreColumn:                 scoreColumn,
	},
	// The tables in `events` cluster.
	eventConversationEventTable: {
		conversationDurationSecsColumn: ColumnNotExist,
	},
	eventUserEventTable: {
		conversationStartTimeColumn:    eventTimeColumn,
		conversationEndTimeColumn:      ColumnNotExist,
		conversationDurationSecsColumn: ColumnNotExist,
		usecaseIDColumn:                ColumnNotExist,
	},
	eventAnalyticsEventTable: {
		conversationStartTimeColumn:    eventTimeColumn,
		conversationEndTimeColumn:      ColumnNotExist,
		conversationDurationSecsColumn: ColumnNotExist,
		usecaseIDColumn:                ColumnNotExist,
	},
	conversationLabelTable: {
		conversationDurationSecsColumn: ColumnNotExist,
	},
}

// column "policy_id" may have a different name in different table. eg it is "action_policy_id" in conversation_event table.
func tableSpecificColumnName(t ClickhouseTable, columnName string) string {
	colName := columnName
	if tableColumnNameMapping[t] != nil && tableColumnNameMapping[t][columnName] != "" {
		colName = tableColumnNameMapping[t][columnName]
	}
	return colName
}

// concat keys used in GROUP BY and JOIN.
func concatKeys(vals []string) string {
	if len(vals) == 0 {
		return ""
	}
	return strings.Join(vals, ", ")
}

// concatnate queries used in SELECT.
func concatQueries(vals []string) string {
	if len(vals) == 0 {
		return ""
	}
	return strings.Join(vals, ",\n") + ","
}

func concatConditionsAndArgsWithPredicate(
	conditionsAndArgs []conditionAndArg,
	predicateForSkipping func(conditionAndArg) bool,
	concatWithAnd bool,
) (string, []any) {
	if len(conditionsAndArgs) == 0 {
		return "", nil
	}

	// Extract conditions and args from input.
	conditions := []string{}
	args := []any{}
	for _, conditionAndArg := range conditionsAndArgs {
		if predicateForSkipping != nil && predicateForSkipping(conditionAndArg) {
			continue
		}
		conditions = append(conditions, conditionAndArg.condition)
		if conditionAndArg.arg != nil {
			args = append(args, conditionAndArg.arg)
		}
		if len(conditionAndArg.args) > 0 {
			args = append(args, conditionAndArg.args...)
		}
	}

	// Enclose each condition in parentheses.
	bracedConditions := make([]string, len(conditions))
	for i, condition := range conditions {
		bracedConditions[i] = fmt.Sprintf("(%s)", condition)
	}

	// Join the braced conditions with "AND"/"OR" and enclose the whole expression:
	conditionsStr := ""
	if concatWithAnd {
		conditionsStr = fmt.Sprintf("(%s)", strings.Join(bracedConditions, " AND "))
	} else {
		conditionsStr = fmt.Sprintf("(%s)", strings.Join(bracedConditions, " OR "))
	}

	return conditionsStr, args
}

func concatConditionsAndArgs(conditionsAndArgs []conditionAndArg) (string, []any) {
	return concatConditionsAndArgsWithPredicate(conditionsAndArgs, nil, true)
}

func concatConditionsAndArgsWithOR(conditionsAndArgs []conditionAndArg) (string, []any) {
	return concatConditionsAndArgsWithPredicate(conditionsAndArgs, nil, false)
}

func unsupportedGroupByKeys(groupByKeys, supportedGroupByKeys []string) []string {
	m := map[string]any{}
	for _, s := range supportedGroupByKeys {
		m[s] = nil
	}

	unsupported := []string{}
	for _, s := range groupByKeys {
		if _, exists := m[s]; !exists {
			unsupported = append(unsupported, s)
		}
	}
	return unsupported
}

// Merge conditions and args from two maps with keeping order
// i.e. conditons from mapA is in front of conditions from mapB
func mergeTableConditionsAndArgs(
	mapA map[ClickhouseTable][]conditionAndArg,
	mapB map[ClickhouseTable][]conditionAndArg,
) map[ClickhouseTable][]conditionAndArg {
	keys := set.NewFromSlice(fn.MapKeyToSlice(mapA))
	keys.AddAll(fn.MapKeyToSlice(mapB)...)
	merged := map[ClickhouseTable][]conditionAndArg{}
	for key := range keys.All() {
		condAndArgsFromA := mapA[key]
		condAndArgsFromB := mapB[key]
		merged[key] = append(merged[key], condAndArgsFromA...)
		merged[key] = append(merged[key], condAndArgsFromB...)
	}
	return merged
}

func getConversationQueryOptions(opts ...ConversationQueryOption) conversationQueryOptions {
	options := conversationQueryOptions{
		timeRangeType: analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_UNSPECIFIED,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

func getConversationTimeRangeColumn(options conversationQueryOptions) string {
	if options.timeRangeColumnOverride != "" {
		return options.timeRangeColumnOverride
	}
	switch options.timeRangeType {
	case analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT:
		return conversationEndTimeColumn
	default:
		return conversationStartTimeColumn
	}
}

// MetadataFilterResult contains the generated metadata filtering queries and arguments
type MetadataFilterResult struct {
	MomentSubQuery             string
	MomentJoinQueries          map[string]string
	ExcludedMomentSubQuery     string
	ExcludedMomentJoinQueries  map[string]string
	ExcludedMomentWhereQueries string
	AllArgs                    []any
}

// Generates metadata moment annotation filters for include criteria
func generateMomentSubQuery(momentConditionsAndArgsList [][]conditionAndArg, aliasDict map[string]string, fetchFromMetadataView bool) (string, map[string]string, []any) {
	momentSubQueryList := []string{}
	allArgs := []any{}
	joinQueries := make(map[string]string)

	// When querying moment_annotation_mv_by_metadata_d the view is pre-filtered to
	// CONVERSATION_METADATA moments and does not expose a moment_type column, so any
	// moment_type condition must be stripped before building the WHERE clause.
	skipCondition := func(caa conditionAndArg) bool {
		if fetchFromMetadataView {
			return strings.Contains(caa.condition, "moment_type")
		}
		return false
	}

	tableName := momentAnnotationDTableName
	timeCol := createTimeColumn
	latestTimeAlias := "latest_create_time"
	if fetchFromMetadataView {
		tableName = momentAnnotationMetadataViewTableName
		timeCol = updateTimeColumn
		latestTimeAlias = "latest_time"
	}

	for i, momentConditionsAndArgs := range momentConditionsAndArgsList {
		conditions, args := concatConditionsAndArgsWithPredicate(momentConditionsAndArgs, skipCondition, true)
		conditionsNoMetadataValue, argsNoMetadataValue := concatConditionsAndArgsWithPredicate(momentConditionsAndArgs, func(caa conditionAndArg) bool {
			if skipCondition(caa) {
				return true
			}
			keywords := []string{"metadata_string_value", "metadata_number_value", "metadata_bool_value"}
			for _, k := range keywords {
				if strings.Contains(caa.condition, k) {
					return true
				}
			}
			return false
		}, true /* concatWithAnd */)

		momentSubQueryList = append(momentSubQueryList, fmt.Sprintf(`
			-- Find the latest moment annotation for each conversation that matches non-metadata conditions
			t1_moment_annotation_filter_%d AS (
				SELECT
					conversation_id,
					MAX(%s) AS %s
				FROM
					%s
				WHERE
					%s
				GROUP BY conversation_id
			),
			-- Get all moment annotations that match the complete conditions (including metadata values)
			t2_moment_annotation_filter_%d AS (
				SELECT 
					conversation_id,
					%s
				FROM 
					%s
				WHERE 
					%s
			),
			-- Final filter: Only include conversations where the LATEST moment annotation matches metadata criteria
			moment_annotation_filter_%d AS (
				SELECT DISTINCT
					t1.conversation_id AS conversation_id_%d
				FROM 
					t1_moment_annotation_filter_%d AS t1
				JOIN t2_moment_annotation_filter_%d AS t2 ON t1.conversation_id = t2.conversation_id AND t1.%s = t2.%s
			)
		`, i, timeCol, latestTimeAlias, tableName, conditionsNoMetadataValue, i, timeCol, tableName, conditions, i, i, i, i, latestTimeAlias, timeCol))
		allArgs = append(allArgs, argsNoMetadataValue...)
		allArgs = append(allArgs, args...)

		// Create INNER JOIN to identify conversations that matched the include criteria
		joinQueryList := map[string][]string{}
		for key, alias := range aliasDict {
			joinQueryList[key] = append(joinQueryList[key], fmt.Sprintf(`
			GLOBAL INNER JOIN moment_annotation_filter_%d ON
				%s.conversation_id = moment_annotation_filter_%d.conversation_id_%d
		`, i, alias, i, i))
		}

		for key, joinQueryList := range joinQueryList {
			joinQueries[key] = strings.Join(joinQueryList, "\n")
		}
	}
	return strings.Join(momentSubQueryList, ",\n"), joinQueries, allArgs
}

// Generates metadata moment annotation filters for exclude criteria
func generateExcludeMomentSubQuery(excludedMomentConditionsAndArgs [][]conditionAndArg, aliasDict map[string]string, fetchFromMetadataView bool) (string, map[string]string, string, []any) {
	excludedMomentSubQueryList := []string{}
	excludedMomentWhereQueryList := []string{}
	allArgs := []any{}
	joinQueries := make(map[string]string)

	tableName := momentAnnotationDTableName
	if fetchFromMetadataView {
		tableName = momentAnnotationMetadataViewTableName
	}

	for i, excludedMomentConditionsAndArgs := range excludedMomentConditionsAndArgs {
		// moment_annotation_mv_by_metadata_d does not expose moment_type — strip it when using the view.
		conditions, args := concatConditionsAndArgsWithPredicate(excludedMomentConditionsAndArgs, func(caa conditionAndArg) bool {
			return fetchFromMetadataView && strings.Contains(caa.condition, "moment_type")
		}, true)

		excludedMomentSubQueryList = append(excludedMomentSubQueryList, fmt.Sprintf(`
			moment_annotation_exclude_filter_%d AS (
				SELECT DISTINCT
					conversation_id AS conversation_id_exclude_%d
				FROM
					%s
				WHERE
					%s
			)
		`, i, i, tableName, conditions))
		allArgs = append(allArgs, args...)

		// Create LEFT JOIN to identify conversations that should be excluded
		leftJoinQueries := make(map[string][]string)
		for key, alias := range aliasDict {
			leftJoinQueries[key] = append(leftJoinQueries[key], fmt.Sprintf(`
				GLOBAL LEFT JOIN moment_annotation_exclude_filter_%d ON
					%s.conversation_id = moment_annotation_exclude_filter_%d.conversation_id_exclude_%d
			`, i, alias, i, i))
		}

		for key, joinQueryList := range leftJoinQueries {
			joinQueries[key] = strings.Join(joinQueryList, "\n")
		}

		// Create WHERE condition to exclude conversations that matched the exclusion criteria
		excludedMomentWhereQueryList = append(excludedMomentWhereQueryList, fmt.Sprintf(`
			AND moment_annotation_exclude_filter_%d.conversation_id_exclude_%d IS NULL
		`, i, i))
	}

	excludedMomentWhereQueries := ""
	if len(excludedMomentWhereQueryList) > 0 {
		excludedMomentWhereQueries = strings.Join(excludedMomentWhereQueryList, "\n")
	}

	return strings.Join(excludedMomentSubQueryList, ",\n"), joinQueries, excludedMomentWhereQueries, allArgs
}

// generateMetadataFilters generates all metadata filtering queries and returns a MetadataFilterResult
func generateMetadataFilters(momentConditionsAndArgsList [][]conditionAndArg, excludedMomentConditionsAndArgs [][]conditionAndArg, aliasDict map[string]string, isLastCTE bool, fetchFromMetadataView bool) MetadataFilterResult {
	result := MetadataFilterResult{}

	// Check if we need metadata filtering
	hasMetadataFiltering := len(momentConditionsAndArgsList) > 0 || len(excludedMomentConditionsAndArgs) > 0

	if hasMetadataFiltering {
		// Generate include criteria filters
		momentSubQueryArgs := []any{}
		if len(momentConditionsAndArgsList) > 0 {
			result.MomentSubQuery, result.MomentJoinQueries, momentSubQueryArgs = generateMomentSubQuery(momentConditionsAndArgsList, aliasDict, fetchFromMetadataView)
		}

		// Generate exclude criteria filters
		excludedMomentSubQuery := ""
		excludedMomentSubQueryArgs := []any{}
		if len(excludedMomentConditionsAndArgs) > 0 {
			result.ExcludedMomentSubQuery = ""
			// If there are both include and exclude criteria, add a comma between them
			if len(momentConditionsAndArgsList) > 0 {
				result.ExcludedMomentSubQuery = ",\n"
			}
			excludedMomentSubQuery, result.ExcludedMomentJoinQueries, result.ExcludedMomentWhereQueries, excludedMomentSubQueryArgs = generateExcludeMomentSubQuery(excludedMomentConditionsAndArgs, aliasDict, fetchFromMetadataView)
			result.ExcludedMomentSubQuery += excludedMomentSubQuery
			// Add trailing comma if this is not the last CTE in the query
			if !isLastCTE {
				result.ExcludedMomentSubQuery += ","
			}
		} else if !isLastCTE {
			// If only include criteria exist and this is not the last CTE, add trailing comma
			result.MomentSubQuery += ","
		}
		result.AllArgs = append(result.AllArgs, momentSubQueryArgs...)
		result.AllArgs = append(result.AllArgs, excludedMomentSubQueryArgs...)
	}

	return result
}
