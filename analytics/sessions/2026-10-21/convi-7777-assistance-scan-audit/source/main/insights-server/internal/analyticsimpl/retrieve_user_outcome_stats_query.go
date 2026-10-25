package analyticsimpl

import (
	"context"
	"fmt"
	"strings"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/metadataframework"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	useroutcomes "github.com/cresta/go-servers/shared/clickhouse/conversations/useroutcomes"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/container/slice"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserOutcomeQuerySpec struct {
	GroupByAttributeTypes             []analyticspb.UserOutcomeAttributeType
	Filter                            *analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestFilter
	GroupBy                           *analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy
	Parent                            *shared.CustomerProfile
	Users                             []*userpb.User
	Groups                            []*userpb.Group
	UserNameToGroupNamesMap           map[string][]string
	IncludeDirectGroupMembershipsOnly bool
	FieldDefinitionNames              []string
	Metadata                          *analyticspb.Metadata
	LowerIsBetterFieldDefinitions     set.Set[string] // Field definition names where lower values indicate better performance (TIME, CHURN)
	UseExtTable                       bool
}

type CHOutcomeStatsRow struct {
	BucketTime        time.Time
	FieldDefinitionID string
	RecordCount       uint64
	UserID            string
	ValueSum          float64
	WeightSum         float64
	WeightedValueSum  float64
}

// buildOutcomeStatsClickhouseQuery builds the clickhouse query for the outcome stats.
func buildOutcomeStatsClickhouseQuery(spec *UserOutcomeQuerySpec) (string, []any, []string, error) {
	query, queryArgs, err := buildOutcomeStatsBaseQuery(spec)
	if err != nil {
		return "", nil, nil, err
	}

	query, groupByKeys, err := wrapOutcomeStatsQueryWithGroupBy(query, spec)
	if err != nil {
		return "", nil, nil, err
	}

	return query, queryArgs, groupByKeys, nil
}

// buildOutcomeStatsBaseQuery builds the base query for the outcome stats (SELECT + JOIN).
func buildOutcomeStatsBaseQuery(spec *UserOutcomeQuerySpec) (string, []any, error) {
	hasTimeRangeAsGroupKey := slice.Contains(spec.GroupByAttributeTypes, analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_TIME_RANGE)
	specFrequency := spec.GroupBy.GetFrequency()

	startTime, endTime, err := shared.ParseFilterByTimeRange(spec.Filter.TimeRange)
	if err != nil {
		return "", nil, err
	}

	outcomesMainWhereCondition, outcomesMainWhereArgs, err := buildOutcomeStatsMainWhereClause(
		spec,
		spec.FieldDefinitionNames,
		startTime,
		endTime,
	)
	if err != nil {
		return "", nil, err
	}
	outcomesWeightWhereCondition, outcomesWeightWhereArgs, err := buildOutcomeStatsWeightWhereClause(
		spec,
		startTime,
		endTime,
	)
	if err != nil {
		return "", nil, err
	}

	selectParts := []string{
		"outcomes.user_id",
		"outcomes.field_definition_id",
		"outcomes.value_numeric",
		"IF(outcomes.weight_field_definition_id = '', 1.0, outcomes_weight.value_numeric) AS weight_value",
	}
	if hasTimeRangeAsGroupKey && specFrequency != analyticspb.Frequency_FREQUENCY_UNSPECIFIED {
		timeExpr, err := parseClickhouseGroupByForTimeColumn(&specFrequency, spec.Metadata, "outcomes.outcome_time", BucketTimeColumn)
		if err != nil {
			return "", nil, err
		}
		selectParts = append([]string{timeExpr}, selectParts...)
	}

	queryString := fmt.Sprintf(`
		WITH outcomes_main AS (
			SELECT
				customer_id,
				profile_id,
				data_source,
				data_source_record_id,
				user_id,
				outcome_time,
				field_definition_id,
				value_numeric,
				weight_field_definition_id
			FROM
				user_outcome_field_value_d
			WHERE
				%s
		),
		outcomes_weight_filtered AS (
			SELECT
				customer_id,
				profile_id,
				data_source,
				data_source_record_id,
				user_id,
				outcome_time,
				field_definition_id,
				value_numeric
			FROM
				user_outcome_field_value_d
			WHERE
				%s
		)
		SELECT
			%s
		FROM
			outcomes_main outcomes
			LEFT JOIN outcomes_weight_filtered outcomes_weight ON
				outcomes.customer_id = outcomes_weight.customer_id
				AND outcomes.profile_id = outcomes_weight.profile_id
				AND outcomes.data_source = outcomes_weight.data_source
				AND outcomes.data_source_record_id = outcomes_weight.data_source_record_id
				AND outcomes.user_id = outcomes_weight.user_id
				AND outcomes.outcome_time = outcomes_weight.outcome_time
				AND outcomes.weight_field_definition_id = outcomes_weight.field_definition_id
	`, outcomesMainWhereCondition, outcomesWeightWhereCondition, strings.Join(selectParts, ", "))

	return queryString, append(outcomesMainWhereArgs, outcomesWeightWhereArgs...), nil
}

// buildOutcomeStatsMainWhereClause builds the where clause used by outcomes_main CTE.
func buildOutcomeStatsMainWhereClause(
	spec *UserOutcomeQuerySpec,
	fieldDefinitionNames []string,
	startTime time.Time,
	endTime time.Time,
) (string, []any, error) {
	conditionsAndArgs, err := buildOutcomeStatsSharedConditionsAndArgs(spec)
	if err != nil {
		return "", nil, err
	}

	fieldDefinitionIDs := []string{}
	for _, fieldDefinitionName := range fieldDefinitionNames {
		parsedFieldDefinitionResourceName, err := metadatapb.ParseFieldDefinitionName(fieldDefinitionName)
		if err != nil {
			return "", nil, status.Error(codes.InvalidArgument, err.Error())
		}
		fieldDefinitionIDs = append(fieldDefinitionIDs, parsedFieldDefinitionResourceName.FieldDefinitionID)
	}
	if len(fieldDefinitionIDs) > 0 {
		conFieldDefs, argFieldDefs := columnIn("field_definition_id", fieldDefinitionIDs)
		conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: conFieldDefs, arg: argFieldDefs})
	}

	conStart, argStart := columnGTE("outcome_time", startTime.Format(timestampFormat))
	conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: conStart, arg: argStart})
	conEnd, argEnd := columnLT("outcome_time", endTime.Format(timestampFormat))
	conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: conEnd, arg: argEnd})

	whereCondition, whereArgs := concatConditionsAndArgs(conditionsAndArgs)
	return whereCondition, whereArgs, nil
}

func buildOutcomeStatsWeightWhereClause(
	spec *UserOutcomeQuerySpec,
	startTime time.Time,
	endTime time.Time,
) (string, []any, error) {
	conditionsAndArgs, err := buildOutcomeStatsSharedConditionsAndArgs(spec)
	if err != nil {
		return "", nil, err
	}

	conStart, argStart := columnGTE("outcome_time", startTime.Format(timestampFormat))
	conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: conStart, arg: argStart})
	conEnd, argEnd := columnLT("outcome_time", endTime.Format(timestampFormat))
	conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: conEnd, arg: argEnd})

	whereCondition, whereArgs := concatConditionsAndArgs(conditionsAndArgs)
	return whereCondition, whereArgs, nil
}

func buildOutcomeStatsSharedConditionsAndArgs(
	spec *UserOutcomeQuerySpec,
) ([]conditionAndArg, error) {
	conditionsAndArgs := []conditionAndArg{}

	conCustomer, argCustomer := columnEq("customer_id", spec.Parent.CustomerID)
	conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: conCustomer, arg: argCustomer})
	conProfile, argProfile := columnEq("profile_id", spec.Parent.ProfileID)
	conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: conProfile, arg: argProfile})
	conDataSource, argDataSource := columnEq("data_source", useroutcomes.UserOutcomeResourceTypeID)
	conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: conDataSource, arg: argDataSource})

	userIDs, err := extractUserIDsFromUsers(spec.Users)
	if err != nil {
		return nil, err
	}
	if len(userIDs) > 0 {
		if spec.UseExtTable {
			condition := fmt.Sprintf("user_id IN (SELECT %s FROM %s)", extTableAgentFilterColumn, extTableAgentFilter)
			conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: condition})
		} else {
			conUsers, argUsers := columnIn("user_id", userIDs)
			conditionsAndArgs = append(conditionsAndArgs, conditionAndArg{condition: conUsers, arg: argUsers})
		}
	}

	return conditionsAndArgs, nil
}

func extractUserIDsFromUsers(users []*userpb.User) ([]string, error) {
	if len(users) == 0 {
		return nil, nil
	}

	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		parsedUserResourceName, err := userpb.ParseUserName(user.Name)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		userIDs = append(userIDs, parsedUserResourceName.UserID)
	}

	return userIDs, nil
}

// wrapOutcomeStatsQueryWithGroupBy wraps the inner query with SELECT (group-by cols + aggregates) and GROUP BY.
func wrapOutcomeStatsQueryWithGroupBy(query string, spec *UserOutcomeQuerySpec) (string, []string, error) {
	groupByKeys, groupByQueries, err := buildOutcomeStatsQueryGroupBy(spec)
	if err != nil {
		return "", nil, err
	}

	aggregationColumns := []string{
		"SUM(outcomes.value_numeric * outcomes.weight_value) AS weighted_value_sum",
		"SUM(outcomes.weight_value) AS weight_sum",
		"SUM(outcomes.value_numeric) AS value_sum",
		"COUNT(*) AS record_count",
	}

	selectColumns := append([]string{}, groupByQueries...)
	selectColumns = append(selectColumns, aggregationColumns...)

	query = fmt.Sprintf("SELECT %s FROM (%s) outcomes", strings.Join(selectColumns, ", "), query)

	if len(groupByKeys) > 0 {
		query += fmt.Sprintf(" GROUP BY %s", concatKeys(groupByKeys))
	}

	return query, groupByKeys, nil
}

// buildOutcomeStatsQueryGroupBy returns groupByKeys (for GROUP BY clause)
func buildOutcomeStatsQueryGroupBy(spec *UserOutcomeQuerySpec) (groupByKeys, groupByQueries []string, err error) {
	hasTimeRangeAsGroupKey := slice.Contains(spec.GroupByAttributeTypes, analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_TIME_RANGE)

	if hasTimeRangeAsGroupKey && spec.GroupBy.GetFrequency() != analyticspb.Frequency_FREQUENCY_UNSPECIFIED {
		groupByKeys = append(groupByKeys, BucketTimeColumn)
		groupByQueries = append(groupByQueries, "outcomes."+BucketTimeColumn)
	}

	for _, groupByAttribute := range spec.GroupByAttributeTypes {
		groupByKey := ""
		switch groupByAttribute {
		case analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER:
			groupByKey = useroutcomes.UserIDFieldID
		case analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_AGENT_TIER:
			groupByKey = useroutcomes.UserIDFieldID
		case analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_TIME_RANGE:
			continue
		case analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_OUTCOME_FIELD:
			groupByKey = FieldDefinitionIDColumn
		default:
			return nil, nil, status.Errorf(codes.InvalidArgument, "group by attribute %v is not supported in clickhouse query", groupByAttribute)
		}
		if groupByKey != "" {
			groupByKeys = append(groupByKeys, groupByKey)
			groupByQueries = append(groupByQueries, "outcomes."+groupByKey)
		}
	}

	return groupByKeys, groupByQueries, nil
}

func (a *AnalyticsServiceImpl) readOutcomeStatsFromClickhouse(ctx context.Context, parent *shared.CustomerProfile, query string, queryArgs []any, groupByKeys []string) ([]CHOutcomeStatsRow, error) {
	conn, err := a.getConversationsClickhouseConn(ctx, parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	startT := time.Now()
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("[ReadOutcomeStatsFromClickhouse] querying clickhouse: %w", err)
	}
	defer rows.Close()

	resultRows := []CHOutcomeStatsRow{}
	for rows.Next() {
		row := CHOutcomeStatsRow{}
		vals := []any{}

		for _, groupKey := range groupByKeys {
			switch groupKey {
			case FieldDefinitionIDColumn:
				vals = append(vals, &row.FieldDefinitionID)
			case useroutcomes.UserIDFieldID:
				vals = append(vals, &row.UserID)
			case BucketTimeColumn:
				vals = append(vals, &row.BucketTime)
			}
		}

		vals = append(vals, &row.WeightedValueSum, &row.WeightSum, &row.ValueSum, &row.RecordCount)
		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("[ReadOutcomeStatsFromClickhouse] scanning clickhouse row: %w", err)
		}
		resultRows = append(resultRows, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("[ReadOutcomeStatsFromClickhouse] error iterating clickhouse rows: %w", err)
	}

	a.logger.Infof(ctx, "ClickHouse query latency for ReadOutcomeStatsFromClickhouse in seconds %v", time.Since(startT).Seconds())
	return resultRows, nil
}
