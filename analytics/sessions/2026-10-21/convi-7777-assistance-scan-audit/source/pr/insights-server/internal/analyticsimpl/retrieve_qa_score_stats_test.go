package analyticsimpl

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	authpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/auth"
	commonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/insights-server/internal/shared/descriptors"
	sharedauth "github.com/cresta/go-servers/shared/auth"
	authmocks "github.com/cresta/go-servers/shared/auth/mocks"
	clickhousetesting "github.com/cresta/go-servers/shared/clickhouse/conversations/testing"
	"github.com/cresta/go-servers/shared/featureflag"
	logger "github.com/cresta/shared-go/framework/log"
	"github.com/cresta/shared-go/util/jwt"
)

type RetrieveQaScoreStatsTestSuite struct {
	AnalyticsTestSuite
	users               *UsersData
	dates               *TimePeriodData
	templateID          string
	conversationID      string
	reuseConversationID bool
}

func TestConvertQaGroupByForQaScoreStatsRow_WithScorecardSubmitter(t *testing.T) {
	groupBy := convertQaGroupByForQaScoreStatsRow(
		qaScoreStatsRow{clickhouseKeyTimeRow: clickhouseKeyTimeRow{submitterUserID: "submitter-id1"}},
		"test_customer",
		[]analyticspb.QAAttributeType{analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER},
		map[string]*userpb.User{
			"customers/test_customer/users/submitter-id1": {
				Name:        "customers/test_customer/users/submitter-id1",
				DisplayName: "Submitter One",
			},
		},
	)

	require.Equal(t, "customers/test_customer/users/submitter-id1", groupBy.GetUser().GetName())
	require.Equal(t, "Submitter One", groupBy.GetUser().GetDisplayName())
}

func TestConvertQaGroupByForQaScoreStatsRow_WithScorecardSubmitterWithoutUserMetadata(t *testing.T) {
	groupBy := convertQaGroupByForQaScoreStatsRow(
		qaScoreStatsRow{clickhouseKeyTimeRow: clickhouseKeyTimeRow{submitterUserID: "submitter-id1"}},
		"test_customer",
		[]analyticspb.QAAttributeType{analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER},
		nil,
	)

	require.Equal(t, "customers/test_customer/users/submitter-id1", groupBy.GetUser().GetName())
	require.Empty(t, groupBy.GetUser().GetDisplayName())
}

func TestConvertQaGroupByForQaScoreStatsRow_HydratesAgentFullName(t *testing.T) {
	groupBy := convertQaGroupByForQaScoreStatsRow(
		qaScoreStatsRow{clickhouseKeyTimeRow: clickhouseKeyTimeRow{agentUserID: "agent-id1"}},
		"test_customer",
		[]analyticspb.QAAttributeType{analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT},
		map[string]*userpb.User{
			"customers/test_customer/users/agent-id1": {
				Name:     "customers/test_customer/users/agent-id1",
				FullName: "Brenda Luedke",
				Username: "bluedke@xanterra.com",
			},
		},
	)

	// FullName is the only display field ListUsersForAnalytics populates, so
	// dropping it made clients fall back to the email in Username (CONVI-7413).
	require.Equal(t, "Brenda Luedke", groupBy.GetUser().GetFullName())
	require.Equal(t, "bluedke@xanterra.com", groupBy.GetUser().GetUsername())
}

func TestDeduplicateUsersInResponse_PreservesFullName(t *testing.T) {
	userName := "customers/test_customer/users/agent-id1"
	resp := &analyticspb.RetrieveQAScoreStatsResponse{
		QaScoreResult: &analyticspb.QAScoreResult{
			Scores: []*analyticspb.QAScore{
				{
					Score: 0.4,
					GroupedBy: &analyticspb.QAScoreGroupBy{
						User: &userpb.User{
							Name:     userName,
							FullName: "Brenda Luedke",
							Username: "bluedke@xanterra.com",
						},
					},
				},
			},
		},
	}

	deduped := deduplicateUsersInResponse(resp, nil, nil, nil)

	require.Equal(t, "Brenda Luedke", deduped.GetQaScoreResult().GetUsers()[userName].GetFullName())
	require.Equal(t, "bluedke@xanterra.com", deduped.GetQaScoreResult().GetUsers()[userName].GetUsername())
	// Per-row user is stripped to a lookup key.
	require.Equal(t, userName, deduped.GetQaScoreResult().GetScores()[0].GetGroupedBy().GetUser().GetName())
	require.Empty(t, deduped.GetQaScoreResult().GetScores()[0].GetGroupedBy().GetUser().GetFullName())
}

func TestRetrieveQAScoreStatsClickhouseQuery_WithAgentAndSubmitterFilters(t *testing.T) {
	qaAttribute := qaScoreStatsAttributeWithAgentAndSubmitterFilters()
	timeRange := qaScoreStatsSubmitterSQLTestTimeRange()

	commonConditionsAndArgs, _, err := parseCommonConditionsForQAAttribute(
		timeRange,
		qaAttribute,
		scoreTable,
		false, // useExtTable
	)
	require.NoError(t, err)
	scorecardConditionsAndArgs, err := parseScorecardConditionsForQAAttribute(qaAttribute)
	require.NoError(t, err)
	scoreConditionsAndArgs, _, err := parseScoreConditionsForQAAttribute(
		timeRange,
		qaAttribute,
		scoreTable,
		false, // useExtTable
	)
	require.NoError(t, err)
	groupByKeys, groupByQueries, err := parseClickhouseGroupByForQAAttribute(
		nil,
		[]analyticspb.QAAttributeType{analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER},
		qaScoreStatsSubmitterSQLTestMetadata(),
		scoreTable,
	)
	require.NoError(t, err)

	chQuery, chArgs := qaScoreStatsClickhouseQuery(
		commonConditionsAndArgs,
		scorecardConditionsAndArgs,
		scoreConditionsAndArgs,
		nil, // conversationConditionsAndArgs
		nil, // conversationScanBounds
		groupByKeys,
		groupByQueries,
		scoreTable,
		false, // needsConversationEndTime
		false, // hasLowerIsBetterCriteria
	)

	expectedCHQuery := getSQLTestFile(t, "clickhouse_RetrieveQAScoreStats_WithAgentAndSubmitterFilters_request.sql")
	require.Equal(t, formatSQL(expectedCHQuery), formatSQL(combineQueryAndArgs(chQuery, chArgs)))
}

func TestRetrieveQAScoreStatsClickhouseQuery_FilterByConversationOutcomeMomentGroup(t *testing.T) {
	qaAttribute := &analyticspb.QAAttribute{
		MomentGroups: []*analyticspb.MomentGroup{
			{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/outcome_moment_id",
						Type: momentpb.Moment_CONVERSATION_OUTCOME,
					},
				},
				OutcomeValueAttributes: []*analyticspb.OutcomeValueAttribute{
					{
						OutcomeValue: &momentpb.ConversationOutcomeMomentData_OutcomeValue{
							Value: &momentpb.ConversationOutcomeValue{
								Value: &momentpb.ConversationOutcomeValue_StringValue{
									StringValue: "resolved",
								},
							},
						},
					},
					{
						NumericBin: &commonpb.NumericBin{
							FromValue:            0,
							ToValue:              10,
							ToValueIsInclusive:   true,
							FromValueIsExclusive: false,
						},
					},
				},
			},
		},
	}
	timeRange := qaScoreStatsSubmitterSQLTestTimeRange()

	commonConditionsAndArgs, _, err := parseCommonConditionsForQAAttribute(
		timeRange,
		qaAttribute,
		scoreTable,
		false, // useExtTable
	)
	require.NoError(t, err)
	scorecardConditionsAndArgs, err := parseScorecardConditionsForQAAttribute(qaAttribute)
	require.NoError(t, err)
	scoreConditionsAndArgs, _, err := parseScoreConditionsForQAAttribute(
		timeRange,
		qaAttribute,
		scoreTable,
		false, // useExtTable
	)
	require.NoError(t, err)
	momentFilters, excludedMomentFilters, valueOrMissingFilters, err := parseMomentConditionsForQAAttribute(
		timeRange,
		qaAttribute,
		false, // useConversationOutcomeMV
		conversationStartTimeColumn,
	)
	require.NoError(t, err)
	require.Len(t, momentFilters, 1)
	require.Equal(t, momentAnnotationFilterSourceRaw, momentFilters[0].source)
	frequency := analyticspb.Frequency_DAILY
	groupByKeys, groupByQueries, err := parseClickhouseGroupByForQAAttribute(
		&frequency,
		[]analyticspb.QAAttributeType{
			analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
			analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE,
		},
		qaScoreStatsSubmitterSQLTestMetadata(),
		scoreTable,
	)
	require.NoError(t, err)

	chQuery, chArgs := qaScoreStatsClickhouseQueryWithMetadataView(
		commonConditionsAndArgs,
		scorecardConditionsAndArgs,
		scoreConditionsAndArgs,
		nil, // conversationConditionsAndArgs
		nil, // conversationScanBounds
		momentFilters,
		excludedMomentFilters,
		valueOrMissingFilters,
		groupByKeys,
		groupByQueries,
		scoreTable,
		false, // needsConversationEndTime
		false, // hasLowerIsBetterCriteria
	)

	gotCHQuery := combineQueryAndArgs(chQuery, chArgs)
	fmt.Printf("got ClickHouse Query: \n%v\n", gotCHQuery)
	require.Contains(t, formatSQL(gotCHQuery), formatSQL("FROM moment_annotation_d"))
	require.NotContains(t, gotCHQuery, "moment_annotation_mv_by_metadata_d")
	require.Contains(t, gotCHQuery, "moment_type = 14")
	require.Contains(t, gotCHQuery, "moment_template_id = 'outcome_moment_id'")
	require.Contains(t, gotCHQuery, "JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') = true")
	require.Contains(t, gotCHQuery, "JSONExtractString(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') = 'resolved'")
	require.Contains(t, gotCHQuery, "JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') = true")
	require.Contains(t, gotCHQuery, "JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') >= 0")
	require.Contains(t, gotCHQuery, "JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') <= 10")

	expectedCHQuery := getSQLTestFile(t, "clickhouse_RetrieveQAScoreStats_FilterByConversationOutcomeMomentGroup_request.sql")
	require.Equal(t, formatSQL(expectedCHQuery), formatSQL(gotCHQuery))
}

func TestRetrieveQAScoreStatsClickhouseQuery_FilterByMetadataMomentGroups_OverlapValuePlusNoValue(t *testing.T) {
	qaAttribute := &analyticspb.QAAttribute{
		MomentGroups: []*analyticspb.MomentGroup{
			{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_id_1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
					},
				},
				ExcludedMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_id_1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
					},
				},
				MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
					{
						MetadataValue: &commonpb.MetadataValue{
							Value: &commonpb.MetadataValue_StringValue{
								StringValue: "metadata-value-1",
							},
						},
					},
					{
						MetadataValue: &commonpb.MetadataValue{
							Value: &commonpb.MetadataValue_StringValue{
								StringValue: "metadata-value-2",
							},
						},
					},
				},
			},
			{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_id_2",
						Type: momentpb.Moment_CONVERSATION_METADATA,
					},
				},
				MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
					{
						NumericBin: &commonpb.NumericBin{
							FromValue: 0,
							ToValue:   100,
						},
					},
					{
						NumericBin: &commonpb.NumericBin{
							FromValue: 100,
							ToValue:   250,
						},
					},
				},
			},
			{
				ExcludedMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/moment_id_3",
						Type: momentpb.Moment_CONVERSATION_METADATA,
					},
				},
			},
		},
	}

	timeRange := qaScoreStatsSubmitterSQLTestTimeRange()

	commonConditionsAndArgs, _, err := parseCommonConditionsForQAAttribute(
		timeRange,
		qaAttribute,
		scoreTable,
		false, // useExtTable
	)
	require.NoError(t, err)
	scorecardConditionsAndArgs, err := parseScorecardConditionsForQAAttribute(qaAttribute)
	require.NoError(t, err)
	scoreConditionsAndArgs, _, err := parseScoreConditionsForQAAttribute(
		timeRange,
		qaAttribute,
		scoreTable,
		false, // useExtTable
	)
	require.NoError(t, err)

	momentFilters, excludedMomentFilters, valueOrMissingFilters, err := parseMomentConditionsForQAAttribute(
		timeRange,
		qaAttribute,
		false, // useConversationOutcomeMV
		conversationStartTimeColumn,
	)
	require.NoError(t, err)
	require.Len(t, valueOrMissingFilters, 1)
	require.Len(t, momentFilters, 1)
	require.Len(t, excludedMomentFilters, 1)

	frequency := analyticspb.Frequency_DAILY
	groupByKeys, groupByQueries, err := parseClickhouseGroupByForQAAttribute(
		&frequency,
		[]analyticspb.QAAttributeType{
			analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
			analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE,
		},
		qaScoreStatsSubmitterSQLTestMetadata(),
		scoreTable,
	)
	require.NoError(t, err)

	chQuery, chArgs := qaScoreStatsClickhouseQueryWithMetadataView(
		commonConditionsAndArgs,
		scorecardConditionsAndArgs,
		scoreConditionsAndArgs,
		nil, // conversationConditionsAndArgs
		nil, // conversationScanBounds
		momentFilters,
		excludedMomentFilters,
		valueOrMissingFilters,
		groupByKeys,
		groupByQueries,
		scoreTable,
		false, // needsConversationEndTime
		false, // hasLowerIsBetterCriteria
	)

	gotCHQuery := combineQueryAndArgs(chQuery, chArgs)
	fmt.Printf("got ClickHouse Query: \n%v\n", gotCHQuery)

	expectedCHQuery := getSQLTestFile(t, "clickhouse_RetrieveQAScoreStats_FilterByMetadataMomentGroups_OverlapValuePlusNoValue_request.sql")
	require.Equal(t, formatSQL(expectedCHQuery), formatSQL(gotCHQuery))
}

func TestRetrieveQAScoreStatsClickhouseQuery_FilterByConversationOutcomeMomentGroup_UsesOutcomeMV(t *testing.T) {
	qaAttribute := &analyticspb.QAAttribute{
		MomentGroups: []*analyticspb.MomentGroup{
			{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/outcome_moment_id",
						Type: momentpb.Moment_CONVERSATION_OUTCOME,
					},
				},
				OutcomeValueAttributes: []*analyticspb.OutcomeValueAttribute{
					{
						OutcomeValue: &momentpb.ConversationOutcomeMomentData_OutcomeValue{
							Value: &momentpb.ConversationOutcomeValue{
								Value: &momentpb.ConversationOutcomeValue_StringValue{
									StringValue: "resolved",
								},
							},
						},
					},
				},
			},
		},
	}
	timeRange := qaScoreStatsSubmitterSQLTestTimeRange()

	commonConditionsAndArgs, _, err := parseCommonConditionsForQAAttribute(
		timeRange,
		qaAttribute,
		scoreTable,
		false,
	)
	require.NoError(t, err)
	scorecardConditionsAndArgs, err := parseScorecardConditionsForQAAttribute(qaAttribute)
	require.NoError(t, err)
	scoreConditionsAndArgs, _, err := parseScoreConditionsForQAAttribute(
		timeRange,
		qaAttribute,
		scoreTable,
		false,
	)
	require.NoError(t, err)
	momentFilters, excludedMomentFilters, valueOrMissingFilters, err := parseMomentConditionsForQAAttribute(
		timeRange,
		qaAttribute,
		true, // useConversationOutcomeMV
		conversationStartTimeColumn,
	)
	require.NoError(t, err)
	require.Len(t, momentFilters, 1)
	require.Equal(t, momentAnnotationFilterSourceConversationOutcomeMV, momentFilters[0].source)

	frequency := analyticspb.Frequency_DAILY
	groupByKeys, groupByQueries, err := parseClickhouseGroupByForQAAttribute(
		&frequency,
		[]analyticspb.QAAttributeType{
			analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
		},
		qaScoreStatsSubmitterSQLTestMetadata(),
		scoreTable,
	)
	require.NoError(t, err)

	chQuery, chArgs := qaScoreStatsClickhouseQueryWithMetadataView(
		commonConditionsAndArgs,
		scorecardConditionsAndArgs,
		scoreConditionsAndArgs,
		nil, // conversationConditionsAndArgs
		nil, // conversationScanBounds
		momentFilters,
		excludedMomentFilters,
		valueOrMissingFilters,
		groupByKeys,
		groupByQueries,
		scoreTable,
		false,
		false,
	)
	gotCHQuery := combineQueryAndArgs(chQuery, chArgs)
	require.Contains(t, gotCHQuery, "moment_annotation_by_conversation_outcome_d")
	require.NotContains(t, gotCHQuery, "moment_annotation_d")
	require.NotContains(t, gotCHQuery, "moment_type = 14")
	require.NotContains(t, gotCHQuery, "JSONExtract")
	require.Contains(t, gotCHQuery, "outcome_value_type = 'string'")
	require.Contains(t, gotCHQuery, "outcome_string_value = 'resolved'")
	require.Contains(t, gotCHQuery, "MAX(update_time)")
}

func qaScoreStatsAttributeWithAgentAndSubmitterFilters() *analyticspb.QAAttribute {
	return &analyticspb.QAAttribute{
		Users: []*userpb.User{
			{Name: fmt.Sprintf("customers/%s/users/1", CustomerID)},
		},
		ScorecardReviewerAudience: &analyticspb.UserAudience{
			Users: []string{fmt.Sprintf("customers/%s/users/2", CustomerID)},
		},
		ScorecardTemplates: []string{
			fmt.Sprintf("customers/%s/profiles/%s/scorecardTemplates/template_id", CustomerID, ProfileID),
		},
	}
}

func qaScoreStatsSubmitterSQLTestTimeRange() *analyticspb.TimeRange {
	return &analyticspb.TimeRange{
		StartTimestamp: timestamppb.New(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
		EndTimestamp:   timestamppb.New(time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC)),
	}
}

func qaScoreStatsSubmitterSQLTestMetadata() *analyticspb.Metadata {
	return &analyticspb.Metadata{
		TimeZoneId:     "Pacific/Honolulu",
		StartDayOfWeek: analyticspb.Metadata_SUNDAY,
	}
}

func getSQLTestFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("testdata", path))
	require.NoError(t, err)
	return string(contents)
}

func (r *RetrieveQaScoreStatsTestSuite) SetupSuite() {
	r.AnalyticsTestSuite.SetupSuite()

	r.users = r.SetupUsersData()
	r.dates = r.SetupTimePeriodData()
	r.UseMockedClickhouseConnections()

	r.mockConfigClient.EXPECT().
		Get(gomock.Any()).AnyTimes().
		Return(&config.CustomerConfig{}, nil)
	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().
		Return(&config.ProfileConfig{}, nil)
}

func (r *RetrieveQaScoreStatsTestSuite) BeforeTest(suiteName, methodName string) {
	r.templateID = methodName
	r.conversationID = uuid.NewString()
	r.reuseConversationID = false
}

func (r *RetrieveQaScoreStatsTestSuite) TestRetrieveQaStatsByUserAndGroupAndTimeRangeThrows() {
	// Given
	groupByAgentAndGroupAndTimeRange := []analyticspb.QAAttributeType{analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_GROUP, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE}

	// When
	_, err := r.server.RetrieveQAScoreStats(
		r.ctx,
		&analyticspb.RetrieveQAScoreStatsRequest{
			Parent: r.profileResourceName,
			FilterByAttribute: &analyticspb.QAAttribute{
				ScorecardTemplates: []string{
					descriptors.ScorecardTemplateName{
						CustomerID: r.customerID,
						ProfileID:  r.profileID,
						TemplateID: r.templateID,
					}.String(),
				},
			},
			FilterByTimeRange:     &r.dates.janPeriod,
			Frequency:             analyticspb.Frequency_DAILY,
			GroupByAttributeTypes: groupByAgentAndGroupAndTimeRange,
			Metadata:              &r.dates.metadataLA,
		},
	)

	// Then
	r.assertError(
		err,
		codes.InvalidArgument,
		"when grouping by `group`, only `time_range` and `criterion` are supported")
}

func (r *RetrieveQaScoreStatsTestSuite) TestRetrieveQaScoreStats_MomentSourceScanScope() {
	originalEnabled := *enableExcludedMoments
	r.T().Cleanup(func() { *enableExcludedMoments = originalEnabled })
	*enableExcludedMoments = true
	moment := func(id string) *momentpb.Moment {
		return &momentpb.Moment{Name: r.profileResourceName + "/moments/" + id, Type: momentpb.Moment_CONVERSATION_METADATA}
	}
	value := []*analyticspb.MetadataValueAttribute{{MetadataValue: &commonpb.MetadataValue{
		Value: &commonpb.MetadataValue_StringValue{StringValue: "selected"},
	}}}
	for name, number := range analyticspb.TimeRangeFilterTarget_value {
		target := analyticspb.TimeRangeFilterTarget(number)
		r.Run(name, func() {
			defer r.convoMockConn.AssertExpectationsAndReset(r.T())
			var query string
			r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) { query = args.String(1) }).
				Return(clickhousetesting.NewMockRows(nil), nil)
			_, err := r.server.RetrieveQAScoreStats(r.ctx, &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:                r.profileResourceName,
				FilterByTimeRange:     &r.dates.janPeriod,
				TimeRangeFilterTarget: target,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT},
				FilterByAttribute: &analyticspb.QAAttribute{MomentGroups: []*analyticspb.MomentGroup{
					{Moments: []*momentpb.Moment{moment("include")}, MetadataValueAttributes: value},
					{ExcludedMoments: []*momentpb.Moment{moment("exclude")}},
					{Moments: []*momentpb.Moment{moment("overlap")}, ExcludedMoments: []*momentpb.Moment{moment("overlap")}, MetadataValueAttributes: value},
				}},
			})
			r.Require().NoError(err)
			// Check every source CTE, including latest-value scans and absence checks.
			ctes := regexp.MustCompile(`(?s)(t[12]_moment_annotation_filter_[0-9]+|moment_annotation_exclude_filter_[0-9]+) AS \((.*?)\),`).FindAllStringSubmatch(query, -1)
			r.Require().Len(ctes, 6)
			for _, cte := range ctes {
				body := strings.Join(strings.Fields(cte[2]), " ")
				if target == analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_SUBMIT_TIME {
					r.Contains(body, "conversation_id GLOBAL IN (SELECT DISTINCT conversation_id FROM scorecard_score_per_conversation)", cte[1])
					r.NotContains(body, "conversation_start_time", cte[1])
				} else {
					r.NotContains(body, "scorecard_score_per_conversation", cte[1])
					r.Contains(body, "conversation_start_time >=", cte[1])
					r.Contains(body, "conversation_start_time <", cte[1])
				}
			}
		})
	}
}

func (r *RetrieveQaScoreStatsTestSuite) TestRetrieveQaScoreStats_SubmitTimeWithEndedAtThrows() {
	_, err := r.server.RetrieveQAScoreStats(
		r.ctx,
		&analyticspb.RetrieveQAScoreStatsRequest{
			Parent:            r.profileResourceName,
			FilterByTimeRange: &r.dates.janPeriod,
			GroupByAttributeTypes: []analyticspb.QAAttributeType{
				analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
			},
			ConversationTimeRangeField: analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT,
			TimeRangeFilterTarget:      analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_SUBMIT_TIME,
			Metadata:                   &r.dates.metadataLA,
		},
	)

	r.assertError(
		err,
		codes.InvalidArgument,
		"time_range_filter_target=SUBMIT_TIME is incompatible with conversation_time_range_field=CONVERSATION_ENDED_AT")
}

func (r *RetrieveQaScoreStatsTestSuite) TestRetrieveQaStatsByUserAndGroupThrows() {
	// Given
	groupByAgentAndGroup := []analyticspb.QAAttributeType{analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_GROUP}

	// When
	_, err := r.server.RetrieveQAScoreStats(
		r.ctx,
		&analyticspb.RetrieveQAScoreStatsRequest{
			Parent: r.profileResourceName,
			FilterByAttribute: &analyticspb.QAAttribute{
				ScorecardTemplates: []string{
					descriptors.ScorecardTemplateName{
						CustomerID: r.customerID,
						ProfileID:  r.profileID,
						TemplateID: r.templateID,
					}.String(),
				},
			},
			FilterByTimeRange:     &r.dates.janPeriod,
			Frequency:             analyticspb.Frequency_DAILY,
			GroupByAttributeTypes: groupByAgentAndGroup,
			Metadata:              &r.dates.metadataLA,
		},
	)

	// Then
	r.assertError(
		err,
		codes.InvalidArgument,
		"when grouping by `group`, only `time_range` and `criterion` are supported")
}

func (r *RetrieveQaScoreStatsTestSuite) TestRetrieveQaStatsByAgentAndScorecardSubmitterThrows() {
	// Given
	groupByAgentAndScorecardSubmitter := []analyticspb.QAAttributeType{analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT, analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER}

	// When
	_, err := r.server.RetrieveQAScoreStats(
		r.ctx,
		&analyticspb.RetrieveQAScoreStatsRequest{
			Parent: r.profileResourceName,
			FilterByAttribute: &analyticspb.QAAttribute{
				ScorecardTemplates: []string{
					descriptors.ScorecardTemplateName{
						CustomerID: r.customerID,
						ProfileID:  r.profileID,
						TemplateID: r.templateID,
					}.String(),
				},
			},
			FilterByTimeRange:     &r.dates.janPeriod,
			Frequency:             analyticspb.Frequency_DAILY,
			GroupByAttributeTypes: groupByAgentAndScorecardSubmitter,
			Metadata:              &r.dates.metadataLA,
		},
	)

	// Then
	r.assertError(
		err,
		codes.InvalidArgument,
		"Not supporting `agent` with `scorecard submitter` group attribute")
}

func (r *RetrieveQaScoreStatsTestSuite) TestRetrieveQaScoreStats() {
	for _, tc := range []struct {
		desc      string
		request   *analyticspb.RetrieveQAScoreStatsRequest
		chReqPath string
	}{
		{
			desc: "GroupByAgent",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
				},
				Metadata: &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_GroupByAgent_request.sql",
		},
		{
			desc: "GroupByAgent_SourceScorecard",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
				},
				ScoreResource: analyticspb.RetrieveQAScoreStatsRequest_QA_SCORE_RESOURCE_SCORECARD,
				Metadata:      &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_GroupByAgent_SourceScorecard_request.sql",
		},
		{
			desc: "GroupByAgent_EnableLastScorecardFilter",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
				},
				FilterByAttribute: &analyticspb.QAAttribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Name: "voicemail-moment-template-name",
									Type: momentpb.Moment_VOICE_MAIL,
								},
							},
						},
					},
					ConversationDurationBuckets: []*analyticspb.ConversationDurationBucket{
						{Id: "1"},
						{Id: "2"},
					},
				},
				Metadata: &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_GroupByAgent_EnableLastScorecardFilter_request.sql",
		},
		{
			desc: "GroupByAgentTime",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				Frequency:         analyticspb.Frequency_DAILY,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Metadata: &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_GroupByAgentTime_request.sql",
		},
		{
			desc: "GroupByAgentCriterion",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION,
				},
				Metadata: &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_GroupByAgentCriterion_request.sql",
		},
		{
			desc: "GroupByGroupCriterion",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent: r.profileResourceName,
				FilterByAttribute: &analyticspb.QAAttribute{
					ScorecardTemplates: []string{
						descriptors.ScorecardTemplateName{
							CustomerID: r.customerID,
							ProfileID:  r.profileID,
							TemplateID: r.templateID,
						}.String(),
					},
					Groups: []*userpb.Group{
						r.agentsAndGroups.groupUser1,
						r.agentsAndGroups.groupUser2,
					},
				},
				FilterByTimeRange: &r.dates.janPeriod,
				Frequency:         analyticspb.Frequency_DAILY,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_GROUP,
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION,
				},
				Metadata: &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_GroupByGroupCriterion_request.sql",
		},
		{
			desc: "FilterByScoreRange",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				FilterByAttribute: &analyticspb.QAAttribute{
					ScoreRanges: []*analyticspb.QAAttribute_ScoreRange{
						{MinScore: 0, MaxScore: 0},
					},
				},
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
				},
				Metadata: &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_FilterByScoreRange_request.sql",
		},
		{
			desc: "FilterByMetadataMomentGroups",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				FilterByAttribute: &analyticspb.QAAttribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/moment_id_1",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									MetadataValue: &commonpb.MetadataValue{
										Value: &commonpb.MetadataValue_StringValue{
											StringValue: "metadata-value-1",
										},
									},
								},
								{
									MetadataValue: &commonpb.MetadataValue{
										Value: &commonpb.MetadataValue_StringValue{
											StringValue: "metadata-value-2",
										},
									},
								},
							},
						},
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/moment_id_2",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									NumericBin: &commonpb.NumericBin{
										FromValue: 0,
										ToValue:   100,
									},
								},
								{
									NumericBin: &commonpb.NumericBin{
										FromValue: 100,
										ToValue:   250,
									},
								},
							},
						},
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/moment_id_3",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
						},
					},
				},
				Frequency: analyticspb.Frequency_DAILY,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Metadata: &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_FilterByMetadataMomentGroups_request.sql",
		},
		{
			desc: "FilterByMetadataMomentGroups_OverlapValuePlusNoValue",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				FilterByAttribute: &analyticspb.QAAttribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/moment_id_1",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
							ExcludedMoments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/moment_id_1",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									MetadataValue: &commonpb.MetadataValue{
										Value: &commonpb.MetadataValue_StringValue{
											StringValue: "metadata-value-1",
										},
									},
								},
								{
									MetadataValue: &commonpb.MetadataValue{
										Value: &commonpb.MetadataValue_StringValue{
											StringValue: "metadata-value-2",
										},
									},
								},
							},
						},
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/moment_id_2",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									NumericBin: &commonpb.NumericBin{
										FromValue: 0,
										ToValue:   100,
									},
								},
								{
									NumericBin: &commonpb.NumericBin{
										FromValue: 100,
										ToValue:   250,
									},
								},
							},
						},
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/moment_id_3",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
						},
					},
				},
				Frequency: analyticspb.Frequency_DAILY,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Metadata: &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_FilterByMetadataMomentGroups_OverlapValuePlusNoValue_request.sql",
		},
		{
			desc: "FilterByMetadataMomentGroups_EnableLastScorecardFilter",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				FilterByAttribute: &analyticspb.QAAttribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/moment_id_1",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									MetadataValue: &commonpb.MetadataValue{
										Value: &commonpb.MetadataValue_StringValue{
											StringValue: "metadata-value-1",
										},
									},
								},
								{
									MetadataValue: &commonpb.MetadataValue{
										Value: &commonpb.MetadataValue_StringValue{
											StringValue: "metadata-value-2",
										},
									},
								},
							},
						},
						{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/moment_id_2",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								{
									NumericBin: &commonpb.NumericBin{
										FromValue: 0,
										ToValue:   100,
									},
								},
								{
									NumericBin: &commonpb.NumericBin{
										FromValue: 100,
										ToValue:   250,
									},
								},
							},
						},
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/moment_id_3",
									Type: momentpb.Moment_CONVERSATION_METADATA,
								},
							},
						},
					},
				},
				Frequency: analyticspb.Frequency_DAILY,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				Metadata: &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_FilterByMetadataMomentGroups_EnableLastScorecardFilter_request.sql",
		},
		{
			desc: "FilterByConversationEndedAt",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
				},
				ConversationTimeRangeField: analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT,
				Metadata:                   &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_FilterByConversationEndedAt_request.sql",
		},
		{
			desc: "FilterByConversationEndedAt_EnableLastScorecardFilter",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
				},
				FilterByAttribute: &analyticspb.QAAttribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							ExcludedMoments: []*momentpb.Moment{
								{
									Name: "voicemail-moment-template-name",
									Type: momentpb.Moment_VOICE_MAIL,
								},
							},
						},
					},
					ConversationDurationBuckets: []*analyticspb.ConversationDurationBucket{
						{Id: "1"},
						{Id: "2"},
					},
				},
				ConversationTimeRangeField: analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT,
				Metadata:                   &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_FilterByConversationEndedAt_EnableLastScorecardFilter_request.sql",
		},
		{
			desc: "GroupByAgentTime_FilterByConversationEndedAt",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				Frequency:         analyticspb.Frequency_DAILY,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				ConversationTimeRangeField: analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT,
				Metadata:                   &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_GroupByAgentTime_FilterByConversationEndedAt_request.sql",
		},
		{
			desc: "GroupByAgentTime_ScorecardSubmitTime",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				Frequency:         analyticspb.Frequency_DAILY,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				TimeRangeFilterTarget: analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_SUBMIT_TIME,
				Metadata:              &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_GroupByAgentTime_ScorecardSubmitTime_request.sql",
		},
		{
			desc: "GroupByAgent_SourceScorecard_ScorecardSubmitTime",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
				},
				ScoreResource:         analyticspb.RetrieveQAScoreStatsRequest_QA_SCORE_RESOURCE_SCORECARD,
				TimeRangeFilterTarget: analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_SUBMIT_TIME,
				Metadata:              &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_GroupByAgent_SourceScorecard_ScorecardSubmitTime_request.sql",
		},
		{
			desc: "GroupByAgentTime_SourceScorecard_ScorecardSubmitTime",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				Frequency:         analyticspb.Frequency_DAILY,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				ScoreResource:         analyticspb.RetrieveQAScoreStatsRequest_QA_SCORE_RESOURCE_SCORECARD,
				TimeRangeFilterTarget: analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_SUBMIT_TIME,
				Metadata:              &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_GroupByAgentTime_SourceScorecard_ScorecardSubmitTime_request.sql",
		},
		{
			desc: "GroupByAgentTime_ScorecardInteractionTime",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				Frequency:         analyticspb.Frequency_DAILY,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_TIME_RANGE,
				},
				TimeRangeFilterTarget: analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_INTERACTION_TIME,
				Metadata:              &r.dates.metadataLA,
			},
			chReqPath: "clickhouse_RetrieveQAScoreStats_GroupByAgentTime_request.sql",
		},
	} {
		r.Run(tc.desc, func() {
			// Given
			defer r.convoMockConn.AssertExpectationsAndReset(r.T())
			mockRows := clickhousetesting.NewMockRows(nil)
			r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

			// When
			_, err := r.server.RetrieveQAScoreStats(r.ctx, tc.request)

			// Then
			r.Require().NoError(err)
			r.Require().Len(r.convoMockConn.Calls, 1, "expected to exactly query once")
			r.Equal(clickhouseQueryMethod, r.convoMockConn.Calls[0].Method)
			gotQuery := r.convoMockConn.Calls[0].Arguments.Get(1).(string)
			gotArgs := r.convoMockConn.Calls[0].Arguments.Get(2).([]any)
			fmt.Printf("got ClickHouse Query: \n%v\n", combineQueryAndArgs(gotQuery, gotArgs))
			expectedCHQuery := r.getFile(tc.chReqPath)
			r.Equal(formatSQL(expectedCHQuery), formatSQL(combineQueryAndArgs(gotQuery, gotArgs)))
		})
	}
}

func TestRetrieveQaScoreStats(t *testing.T) {
	suite.Run(t, &RetrieveQaScoreStatsTestSuite{})
}

func TestSetQuintileRankForPerAgentScores(t *testing.T) {
	t.Run("PercentileBased_10Agents", func(t *testing.T) {
		// 10 agents: top 2 → Q1, next 2 → Q2, next 2 → Q3, next 2 → Q4, bottom 2 → Q5.
		agents := make([]*analyticspb.QAScore, 10)
		for i := 0; i < 10; i++ {
			agents[i] = &analyticspb.QAScore{
				GroupedBy: &analyticspb.QAScoreGroupBy{
					User: &userpb.User{Name: fmt.Sprintf("u/agent%d", i)},
				},
				Score: float32(100-i*10) / 100, // 1.0, 0.9, 0.8, ..., 0.1
			}
		}
		resp := &analyticspb.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analyticspb.QAScoreResult{Scores: agents},
		}

		setQuintileRankForPerAgentScores(resp, nil /* rankingScores */)

		// After sorting descending: agent0(1.0), agent1(0.9), ..., agent9(0.1)
		// Quintile assignment: 2 per group
		want := []analyticspb.QuintileRank{
			analyticspb.QuintileRank_QUINTILE_RANK_1, // 1.0
			analyticspb.QuintileRank_QUINTILE_RANK_1, // 0.9
			analyticspb.QuintileRank_QUINTILE_RANK_2, // 0.8
			analyticspb.QuintileRank_QUINTILE_RANK_2, // 0.7
			analyticspb.QuintileRank_QUINTILE_RANK_3, // 0.6
			analyticspb.QuintileRank_QUINTILE_RANK_3, // 0.5
			analyticspb.QuintileRank_QUINTILE_RANK_4, // 0.4
			analyticspb.QuintileRank_QUINTILE_RANK_4, // 0.3
			analyticspb.QuintileRank_QUINTILE_RANK_5, // 0.2
			analyticspb.QuintileRank_QUINTILE_RANK_5, // 0.1
		}
		// Sort is by score descending, so agents should be in original order.
		for i, a := range agents {
			if a.GroupedBy.QuintileRank != want[i] {
				t.Errorf("agent[%d] (score %.2f) quintile = %v, want %v",
					i, a.Score, a.GroupedBy.QuintileRank, want[i])
			}
		}
	})

	t.Run("SmallGroup_3Agents", func(t *testing.T) {
		// 3 agents: Q1 gets 1, Q2 gets 1, Q3 gets 1, Q4 and Q5 empty.
		// N=3, base=0, extra=3 → first 3 quintiles get 1 each.
		agents := []*analyticspb.QAScore{
			{GroupedBy: &analyticspb.QAScoreGroupBy{User: &userpb.User{Name: "u/top"}}, Score: 0.90},
			{GroupedBy: &analyticspb.QAScoreGroupBy{User: &userpb.User{Name: "u/mid"}}, Score: 0.70},
			{GroupedBy: &analyticspb.QAScoreGroupBy{User: &userpb.User{Name: "u/low"}}, Score: 0.50},
		}
		resp := &analyticspb.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analyticspb.QAScoreResult{Scores: agents},
		}

		setQuintileRankForPerAgentScores(resp, nil /* rankingScores */)

		want := []analyticspb.QuintileRank{
			analyticspb.QuintileRank_QUINTILE_RANK_1,
			analyticspb.QuintileRank_QUINTILE_RANK_2,
			analyticspb.QuintileRank_QUINTILE_RANK_3,
		}
		for i, a := range agents {
			if a.GroupedBy.QuintileRank != want[i] {
				t.Errorf("agent[%d] (score %.2f) quintile = %v, want %v",
					i, a.Score, a.GroupedBy.QuintileRank, want[i])
			}
		}
	})

	t.Run("UnevenDistribution_7Agents", func(t *testing.T) {
		// 7 agents: base=1, extra=2 → Q1 gets 2, Q2 gets 2, Q3–Q5 get 1 each.
		agents := make([]*analyticspb.QAScore, 7)
		for i := 0; i < 7; i++ {
			agents[i] = &analyticspb.QAScore{
				GroupedBy: &analyticspb.QAScoreGroupBy{
					User: &userpb.User{Name: fmt.Sprintf("u/agent%d", i)},
				},
				Score: float32(70-i*10) / 100, // 0.7, 0.6, 0.5, 0.4, 0.3, 0.2, 0.1
			}
		}
		resp := &analyticspb.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analyticspb.QAScoreResult{Scores: agents},
		}

		setQuintileRankForPerAgentScores(resp, nil /* rankingScores */)

		want := []analyticspb.QuintileRank{
			analyticspb.QuintileRank_QUINTILE_RANK_1, // 0.7
			analyticspb.QuintileRank_QUINTILE_RANK_1, // 0.6
			analyticspb.QuintileRank_QUINTILE_RANK_2, // 0.5
			analyticspb.QuintileRank_QUINTILE_RANK_2, // 0.4
			analyticspb.QuintileRank_QUINTILE_RANK_3, // 0.3
			analyticspb.QuintileRank_QUINTILE_RANK_4, // 0.2
			analyticspb.QuintileRank_QUINTILE_RANK_5, // 0.1
		}
		for i, a := range agents {
			if a.GroupedBy.QuintileRank != want[i] {
				t.Errorf("agent[%d] (score %.2f) quintile = %v, want %v",
					i, a.Score, a.GroupedBy.QuintileRank, want[i])
			}
		}
	})

	t.Run("NonAgentRowsUntouched", func(t *testing.T) {
		agentScore := &analyticspb.QAScore{
			GroupedBy: &analyticspb.QAScoreGroupBy{
				User: &userpb.User{Name: "u/agent1"},
			},
			Score: 0.85,
		}
		nonAgentScore := &analyticspb.QAScore{
			GroupedBy: &analyticspb.QAScoreGroupBy{
				CriterionId: "criterion-1",
			},
			Score: 0.45,
		}

		resp := &analyticspb.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analyticspb.QAScoreResult{
				Scores: []*analyticspb.QAScore{agentScore, nonAgentScore},
			},
		}

		setQuintileRankForPerAgentScores(resp, nil /* rankingScores */)

		// Single agent in peer group → Q1
		if agentScore.GroupedBy.QuintileRank != analyticspb.QuintileRank_QUINTILE_RANK_1 {
			t.Errorf("agent score quintile = %v, want QUINTILE_RANK_1", agentScore.GroupedBy.QuintileRank)
		}
		if nonAgentScore.GroupedBy.QuintileRank != analyticspb.QuintileRank_QUINTILE_RANK_UNSPECIFIED {
			t.Errorf("non-agent score quintile = %v, want QUINTILE_RANK_UNSPECIFIED", nonAgentScore.GroupedBy.QuintileRank)
		}
	})

	t.Run("AllAgentsSameScore", func(t *testing.T) {
		// 5 agents with identical scores — tie-aware ranking puts them all in Q1.
		agents := make([]*analyticspb.QAScore, 5)
		for i := 0; i < 5; i++ {
			agents[i] = &analyticspb.QAScore{
				GroupedBy: &analyticspb.QAScoreGroupBy{
					User: &userpb.User{Name: fmt.Sprintf("u/agent%d", i)},
				},
				Score: 0.75,
			}
		}
		resp := &analyticspb.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analyticspb.QAScoreResult{Scores: agents},
		}

		setQuintileRankForPerAgentScores(resp, nil /* rankingScores */)

		// With identical scores, all agents should be in Q1 (ties move to higher rank).
		for i, a := range agents {
			if a.GroupedBy.QuintileRank != analyticspb.QuintileRank_QUINTILE_RANK_1 {
				t.Errorf("agent[%d] (score %.2f) quintile = %v, want QUINTILE_RANK_1",
					i, a.Score, a.GroupedBy.QuintileRank)
			}
		}
	})

	t.Run("SingleAgent", func(t *testing.T) {
		// 1 agent → Q1 (top 20% = 1 of 1).
		agent := &analyticspb.QAScore{
			GroupedBy: &analyticspb.QAScoreGroupBy{
				User: &userpb.User{Name: "u/solo"},
			},
			Score: 0.50,
		}
		resp := &analyticspb.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analyticspb.QAScoreResult{Scores: []*analyticspb.QAScore{agent}},
		}

		setQuintileRankForPerAgentScores(resp, nil /* rankingScores */)

		if agent.GroupedBy.QuintileRank != analyticspb.QuintileRank_QUINTILE_RANK_1 {
			t.Errorf("single agent quintile = %v, want QUINTILE_RANK_1", agent.GroupedBy.QuintileRank)
		}
	})

	t.Run("NilSafety", func(t *testing.T) {
		// Should not panic on nil inputs.
		setQuintileRankForPerAgentScores(nil, nil /* rankingScores */)
		setQuintileRankForPerAgentScores(&analyticspb.RetrieveQAScoreStatsResponse{}, nil /* rankingScores */)
		setQuintileRankForPerAgentScores(&analyticspb.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analyticspb.QAScoreResult{},
		}, nil /* rankingScores */)
	})

	t.Run("TieAtBoundary_10Agents", func(t *testing.T) {
		// scores [90,80,80,70,60,50,40,30,20,10] (as float32 fractions)
		// Tie at index 1-2 (0.80): naive boundary at 2 → moved to 3.
		// Group 0=[0,1,2], group 1=[3], group 2=[4,5], group 3=[6,7], group 4=[8,9]
		scores := []float32{0.90, 0.80, 0.80, 0.70, 0.60, 0.50, 0.40, 0.30, 0.20, 0.10}
		agents := make([]*analyticspb.QAScore, len(scores))
		for i, s := range scores {
			agents[i] = &analyticspb.QAScore{
				GroupedBy: &analyticspb.QAScoreGroupBy{
					User: &userpb.User{Name: fmt.Sprintf("u/agent%d", i)},
				},
				Score: s,
			}
		}
		resp := &analyticspb.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analyticspb.QAScoreResult{Scores: agents},
		}

		setQuintileRankForPerAgentScores(resp, nil /* rankingScores */)

		want := []analyticspb.QuintileRank{
			analyticspb.QuintileRank_QUINTILE_RANK_1, // 0.90
			analyticspb.QuintileRank_QUINTILE_RANK_1, // 0.80
			analyticspb.QuintileRank_QUINTILE_RANK_1, // 0.80
			analyticspb.QuintileRank_QUINTILE_RANK_2, // 0.70
			analyticspb.QuintileRank_QUINTILE_RANK_3, // 0.60
			analyticspb.QuintileRank_QUINTILE_RANK_3, // 0.50
			analyticspb.QuintileRank_QUINTILE_RANK_4, // 0.40
			analyticspb.QuintileRank_QUINTILE_RANK_4, // 0.30
			analyticspb.QuintileRank_QUINTILE_RANK_5, // 0.20
			analyticspb.QuintileRank_QUINTILE_RANK_5, // 0.10
		}
		for i, a := range agents {
			if a.GroupedBy.QuintileRank != want[i] {
				t.Errorf("agent[%d] (score %.2f) quintile = %v, want %v",
					i, a.Score, a.GroupedBy.QuintileRank, want[i])
			}
		}
	})

	t.Run("TiedPairsAtBoundaries_10Agents", func(t *testing.T) {
		// scores [0.90,0.90,0.80,0.80,0.70,0.70,0.60,0.60,0.50,0.50]
		// Tied pairs fall exactly within groups, no ties span boundaries.
		scores := []float32{0.90, 0.90, 0.80, 0.80, 0.70, 0.70, 0.60, 0.60, 0.50, 0.50}
		agents := make([]*analyticspb.QAScore, len(scores))
		for i, s := range scores {
			agents[i] = &analyticspb.QAScore{
				GroupedBy: &analyticspb.QAScoreGroupBy{
					User: &userpb.User{Name: fmt.Sprintf("u/agent%d", i)},
				},
				Score: s,
			}
		}
		resp := &analyticspb.RetrieveQAScoreStatsResponse{
			QaScoreResult: &analyticspb.QAScoreResult{Scores: agents},
		}

		setQuintileRankForPerAgentScores(resp, nil /* rankingScores */)

		// No ties span boundaries, so groups are evenly [0,1],[2,3],[4,5],[6,7],[8,9].
		want := []analyticspb.QuintileRank{
			analyticspb.QuintileRank_QUINTILE_RANK_1,
			analyticspb.QuintileRank_QUINTILE_RANK_1,
			analyticspb.QuintileRank_QUINTILE_RANK_2,
			analyticspb.QuintileRank_QUINTILE_RANK_2,
			analyticspb.QuintileRank_QUINTILE_RANK_3,
			analyticspb.QuintileRank_QUINTILE_RANK_3,
			analyticspb.QuintileRank_QUINTILE_RANK_4,
			analyticspb.QuintileRank_QUINTILE_RANK_4,
			analyticspb.QuintileRank_QUINTILE_RANK_5,
			analyticspb.QuintileRank_QUINTILE_RANK_5,
		}
		for i, a := range agents {
			if a.GroupedBy.QuintileRank != want[i] {
				t.Errorf("agent[%d] (score %.2f) quintile = %v, want %v",
					i, a.Score, a.GroupedBy.QuintileRank, want[i])
			}
		}
	})
}

func (r *RetrieveQaScoreStatsTestSuite) TestConvertCHResponseDoesNotSetQuintileRank() {
	// Given: ClickHouse rows with 5 per-agent scores.
	// convertCHResponseToQaScoreStatsResponse should NOT set quintile ranks —
	// that's done by the caller (retrieveQAScoreStatsInternal) for AGENT-only queries.
	rows := []qaScoreStatsRow{
		{clickhouseKeyTimeRow: clickhouseKeyTimeRow{agentUserID: "agent1"}, weightedPercentageSum: 0.90, weightSum: 1, totalConversationCount: 5, totalScorecardCount: 5},
		{clickhouseKeyTimeRow: clickhouseKeyTimeRow{agentUserID: "agent2"}, weightedPercentageSum: 0.70, weightSum: 1, totalConversationCount: 5, totalScorecardCount: 5},
		{clickhouseKeyTimeRow: clickhouseKeyTimeRow{agentUserID: "agent3"}, weightedPercentageSum: 0.50, weightSum: 1, totalConversationCount: 5, totalScorecardCount: 5},
		{clickhouseKeyTimeRow: clickhouseKeyTimeRow{agentUserID: "agent4"}, weightedPercentageSum: 0.30, weightSum: 1, totalConversationCount: 5, totalScorecardCount: 5},
		{clickhouseKeyTimeRow: clickhouseKeyTimeRow{agentUserID: "agent5"}, weightedPercentageSum: 0.10, weightSum: 1, totalConversationCount: 5, totalScorecardCount: 5},
	}
	groupByAttributes := []analyticspb.QAAttributeType{analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT}

	// When
	chResult, err := r.server.convertCHResponseToQaScoreStatsResponse(r.customerID, rows, groupByAttributes, nil)

	// Then: all scores should have UNSPECIFIED quintile rank (not set at this layer).
	r.Require().NoError(err)
	resp := chResult.response
	r.Require().Len(resp.QaScoreResult.Scores, 5)
	for _, s := range resp.QaScoreResult.Scores {
		r.Equal(analyticspb.QuintileRank_QUINTILE_RANK_UNSPECIFIED, s.GroupedBy.QuintileRank,
			"convertCHResponseToQaScoreStatsResponse should not set quintile rank (score %.2f)", s.Score)
	}
}

func (r *RetrieveQaScoreStatsTestSuite) TestSetQuintileRankAfterConvertCHResponse() {
	// Given: ClickHouse rows with 5 per-agent scores.
	rows := []qaScoreStatsRow{
		{clickhouseKeyTimeRow: clickhouseKeyTimeRow{agentUserID: "agent1"}, weightedPercentageSum: 0.90, weightSum: 1, totalConversationCount: 5, totalScorecardCount: 5},
		{clickhouseKeyTimeRow: clickhouseKeyTimeRow{agentUserID: "agent2"}, weightedPercentageSum: 0.70, weightSum: 1, totalConversationCount: 5, totalScorecardCount: 5},
		{clickhouseKeyTimeRow: clickhouseKeyTimeRow{agentUserID: "agent3"}, weightedPercentageSum: 0.50, weightSum: 1, totalConversationCount: 5, totalScorecardCount: 5},
		{clickhouseKeyTimeRow: clickhouseKeyTimeRow{agentUserID: "agent4"}, weightedPercentageSum: 0.30, weightSum: 1, totalConversationCount: 5, totalScorecardCount: 5},
		{clickhouseKeyTimeRow: clickhouseKeyTimeRow{agentUserID: "agent5"}, weightedPercentageSum: 0.10, weightSum: 1, totalConversationCount: 5, totalScorecardCount: 5},
	}
	groupByAttributes := []analyticspb.QAAttributeType{analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT}

	// When: convert then explicitly call setQuintileRankForPerAgentScores (mimicking the AGENT path).
	chResult, err := r.server.convertCHResponseToQaScoreStatsResponse(r.customerID, rows, groupByAttributes, nil)
	r.Require().NoError(err)
	resp := chResult.response
	setQuintileRankForPerAgentScores(resp, chResult.rankingScores)

	// Then: percentile-based, 5 agents → 1 per quintile, ranked by score descending.
	r.Require().Len(resp.QaScoreResult.Scores, 5)
	scoreToRank := map[float32]analyticspb.QuintileRank{
		0.90: analyticspb.QuintileRank_QUINTILE_RANK_1,
		0.70: analyticspb.QuintileRank_QUINTILE_RANK_2,
		0.50: analyticspb.QuintileRank_QUINTILE_RANK_3,
		0.30: analyticspb.QuintileRank_QUINTILE_RANK_4,
		0.10: analyticspb.QuintileRank_QUINTILE_RANK_5,
	}
	for _, s := range resp.QaScoreResult.Scores {
		roundedScore := float32(int(s.Score*100+0.5)) / 100
		wantRank, ok := scoreToRank[roundedScore]
		r.True(ok, "unexpected score %.4f", s.Score)
		r.Equal(wantRank, s.GroupedBy.QuintileRank,
			"agent (score %.2f) should have quintile %v", s.Score, wantRank)
	}
}

func (r *RetrieveQaScoreStatsTestSuite) TestConvertCHResponseNoQuintileForNonAgentRows() {
	// Given: ClickHouse rows grouped by criterion (not agent) — should not get quintile rank.
	rows := []qaScoreStatsRow{
		{clickhouseKeyTimeRow: clickhouseKeyTimeRow{criterionID: "c1"}, weightedPercentageSum: 0.85, weightSum: 1, totalConversationCount: 10, totalScorecardCount: 10},
		{clickhouseKeyTimeRow: clickhouseKeyTimeRow{criterionID: "c2"}, weightedPercentageSum: 0.45, weightSum: 1, totalConversationCount: 10, totalScorecardCount: 10},
	}
	groupByAttributes := []analyticspb.QAAttributeType{analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_CRITERION}

	// When
	chResult, err := r.server.convertCHResponseToQaScoreStatsResponse(r.customerID, rows, groupByAttributes, nil)

	// Then
	r.Require().NoError(err)
	for _, s := range chResult.response.QaScoreResult.Scores {
		r.Equal(analyticspb.QuintileRank_QUINTILE_RANK_UNSPECIFIED, s.GroupedBy.QuintileRank,
			"criterion row should have QUINTILE_RANK_UNSPECIFIED")
	}
}

func (r *RetrieveQaScoreStatsTestSuite) TestAggregateTopAgentsResponse_NoQuintileRankLeakage() {
	// Given
	// Build per-agent scores that would get quintile ranks if setQuintileRankForPerAgentScores were called.
	// aggregateTopAgentsResponse should NOT set quintile rank because the call site is gated to AGENT-only.
	scores := make([]*analyticspb.QAScore, 0, 4)
	for i, s := range []float32{0.90, 0.70, 0.40, 0.15} {
		scores = append(scores, &analyticspb.QAScore{
			GroupedBy: &analyticspb.QAScoreGroupBy{
				User: &userpb.User{Name: userpb.UserName{CustomerID: r.customerID, UserID: fmt.Sprintf("agent%d", i)}.String()},
			},
			Score:               s,
			WeightSum:           1,
			TotalScorecardCount: 10,
		})
	}
	resp := &analyticspb.RetrieveQAScoreStatsResponse{
		QaScoreResult: &analyticspb.QAScoreResult{Scores: scores},
	}

	// When
	result, err := aggregateTopAgentsResponse(resp, nil)

	// Then
	r.Require().NoError(err)
	for _, s := range result.QaScoreResult.Scores {
		r.Equal(analyticspb.QuintileRank_QUINTILE_RANK_UNSPECIFIED, s.GroupedBy.QuintileRank,
			"tier %v should have QUINTILE_RANK_UNSPECIFIED", s.GroupedBy.AgentTier)
	}
}

func (r *RetrieveQaScoreStatsTestSuite) TestRetrieveQaScoreStatsResponseCorrectness() {
	for _, tc := range []struct {
		desc       string
		request    *analyticspb.RetrieveQAScoreStatsRequest
		mockValues [][]any
		wantScores map[string]struct {
			score                  float32
			weightSum              float32
			totalConversationCount int32
			totalScorecardCount    int32
		}
		wantAverageQaScore         float32
		wantTotalConversationCount int32
		wantTotalScorecardCount    int32
	}{
		{
			desc: "GroupByAgent_TwoAgents",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
				},
				Metadata: &r.dates.metadataLA,
			},
			// Scan order: agent_user_id, weighted_percentage_sum, weight_sum, total_conversation_count, total_scorecard_count
			mockValues: [][]any{
				{"1", 0.80, 1.0, uint64(10), uint64(5)},
				{"2", 1.50, 2.0, uint64(20), uint64(8)},
			},
			wantScores: map[string]struct {
				score                  float32
				weightSum              float32
				totalConversationCount int32
				totalScorecardCount    int32
			}{
				"customers/customer_id/users/1": {score: float32(0.80 / 1.0), weightSum: 1.0, totalConversationCount: 10, totalScorecardCount: 5},
				"customers/customer_id/users/2": {score: float32(1.50 / 2.0), weightSum: 2.0, totalConversationCount: 20, totalScorecardCount: 8},
			},
			wantAverageQaScore:         float32((0.80 + 1.50) / (1.0 + 2.0)),
			wantTotalConversationCount: 30,
			wantTotalScorecardCount:    13,
		},
		{
			desc: "GroupByAgent_EmptyResult",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
				},
				Metadata: &r.dates.metadataLA,
			},
			mockValues: nil,
			wantScores: map[string]struct {
				score, weightSum                            float32
				totalConversationCount, totalScorecardCount int32
			}{},
			wantAverageQaScore:         0,
			wantTotalConversationCount: 0,
			wantTotalScorecardCount:    0,
		},
		{
			desc: "GroupByAgent_SingleAgent",
			request: &analyticspb.RetrieveQAScoreStatsRequest{
				Parent:            r.profileResourceName,
				FilterByTimeRange: &r.dates.janPeriod,
				GroupByAttributeTypes: []analyticspb.QAAttributeType{
					analyticspb.QAAttributeType_QA_ATTRIBUTE_TYPE_AGENT,
				},
				Metadata: &r.dates.metadataLA,
			},
			mockValues: [][]any{
				{"1", 3.60, 4.0, uint64(100), uint64(40)},
			},
			wantScores: map[string]struct {
				score                  float32
				weightSum              float32
				totalConversationCount int32
				totalScorecardCount    int32
			}{
				"customers/customer_id/users/1": {score: float32(3.60 / 4.0), weightSum: 4.0, totalConversationCount: 100, totalScorecardCount: 40},
			},
			wantAverageQaScore:         float32(3.60 / 4.0),
			wantTotalConversationCount: 100,
			wantTotalScorecardCount:    40,
		},
	} {
		r.Run(tc.desc, func() {
			// Given
			defer r.convoMockConn.AssertExpectationsAndReset(r.T())
			mockRows := clickhousetesting.NewMockRows(tc.mockValues)
			r.convoMockConn.On(clickhouseQueryMethod, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

			// When
			resp, err := r.server.RetrieveQAScoreStats(r.ctx, tc.request)

			// Then
			r.Require().NoError(err)
			r.Require().NotNil(resp)
			r.Require().NotNil(resp.QaScoreResult)
			r.Require().Len(resp.QaScoreResult.Scores, len(tc.wantScores))

			for _, s := range resp.QaScoreResult.Scores {
				r.Require().NotNil(s.GroupedBy)
				r.Require().NotNil(s.GroupedBy.User)
				userName := s.GroupedBy.User.Name
				want, ok := tc.wantScores[userName]
				r.Require().True(ok, "unexpected user %s in response", userName)
				r.InDelta(want.score, s.Score, 0.0001, "score mismatch for %s", userName)
				r.InDelta(want.weightSum, s.WeightSum, 0.0001, "weight_sum mismatch for %s", userName)
				r.Equal(want.totalConversationCount, s.TotalConversationCount, "conversation count mismatch for %s", userName)
				r.Equal(want.totalScorecardCount, s.TotalScorecardCount, "scorecard count mismatch for %s", userName)
			}

			r.InDelta(tc.wantAverageQaScore, resp.QaScoreResult.AverageQaScore, 0.0001, "average QA score mismatch")
			r.Equal(tc.wantTotalConversationCount, resp.QaScoreResult.TotalConversationCount, "total conversation count mismatch")
			r.Equal(tc.wantTotalScorecardCount, resp.QaScoreResult.TotalScorecardCount, "total scorecard count mismatch")
		})
	}
}

func TestApplyScorecardStatusFilter(t *testing.T) {
	parent := &shared.CustomerProfile{CustomerID: CustomerID, ProfileID: ProfileID}

	agentOnlyCtx := func(t *testing.T) context.Context {
		claim := jwt.CrestaAccessTokenClaim{
			User: jwt.ClaimUser{UserRoles: []string{authpb.AuthProto_AGENT.String()}},
		}
		claimJSON, err := json.Marshal(claim)
		require.NoError(t, err)
		md := metadata.New(map[string]string{"cresta-access-token-claim": string(claimJSON)})
		return metadata.NewIncomingContext(context.Background(), md)
	}

	newServer := func(t *testing.T, isAgentOnly bool, enableScorecardPublish bool) AnalyticsServiceImpl {
		ctrl := gomock.NewController(t)
		mockAuth := authmocks.NewMockAuthEntityProvider(ctrl)
		if enableScorecardPublish {
			if isAgentOnly {
				mockAuth.EXPECT().GetAuthEntity(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, _ string) (*sharedauth.PrincipalEntity, error) {
						return sharedauth.GetPrincipalEntityFromAuthContext(ctx)
					})
			} else {
				mockAuth.EXPECT().GetAuthEntity(gomock.Any(), gomock.Any()).
					Return(&sharedauth.PrincipalEntity{}, nil)
			}
		}
		return AnalyticsServiceImpl{
			logger:             logger.GetLogger(),
			authEntityProvider: mockAuth,
			featureFlagClient: featureflag.NewStubClient(featureflag.StubClientConfig{
				Bools: map[string]bool{"enableScorecardPublish": enableScorecardPublish},
			}),
		}
	}

	t.Run("NonAgent_NoOp", func(t *testing.T) {
		// Given
		server := newServer(t, false, true)
		req := &analyticspb.RetrieveQAScoreStatsRequest{
			FilterByAttribute: &analyticspb.QAAttribute{},
		}

		// When
		err := server.applyScorecardStatusFilter(context.Background(), req, parent)

		// Then
		require.NoError(t, err)
		require.Empty(t, req.FilterByAttribute.ScorecardStatuses)
	})

	t.Run("AgentOnly_FeatureFlagOff_NoOp", func(t *testing.T) {
		// Given
		ctx := agentOnlyCtx(t)
		server := newServer(t, true, false)
		req := &analyticspb.RetrieveQAScoreStatsRequest{
			FilterByAttribute: &analyticspb.QAAttribute{},
		}

		// When
		err := server.applyScorecardStatusFilter(ctx, req, parent)

		// Then
		require.NoError(t, err)
		require.Empty(t, req.FilterByAttribute.ScorecardStatuses)
	})

	t.Run("AgentOnly_NoStatuses_DefaultsToAllowed", func(t *testing.T) {
		// Given
		ctx := agentOnlyCtx(t)
		server := newServer(t, true, true)
		req := &analyticspb.RetrieveQAScoreStatsRequest{
			FilterByAttribute: &analyticspb.QAAttribute{},
		}

		// When
		err := server.applyScorecardStatusFilter(ctx, req, parent)

		// Then
		require.NoError(t, err)
		require.Equal(t, allowedAgentOnlyScorecardStatuses, req.FilterByAttribute.ScorecardStatuses)
	})

	t.Run("AgentOnly_AllowedStatuses_PassThrough", func(t *testing.T) {
		// Given
		ctx := agentOnlyCtx(t)
		server := newServer(t, true, true)
		req := &analyticspb.RetrieveQAScoreStatsRequest{
			FilterByAttribute: &analyticspb.QAAttribute{
				ScorecardStatuses: []analyticspb.QAAttribute_ScorecardStatus{
					analyticspb.QAAttribute_AUTO,
				},
			},
		}

		// When
		err := server.applyScorecardStatusFilter(ctx, req, parent)

		// Then
		require.NoError(t, err)
		require.Equal(t,
			[]analyticspb.QAAttribute_ScorecardStatus{analyticspb.QAAttribute_AUTO},
			req.FilterByAttribute.ScorecardStatuses,
		)
	})

	t.Run("AgentOnly_DisallowedStatus_PermissionDenied", func(t *testing.T) {
		// Given
		ctx := agentOnlyCtx(t)
		server := newServer(t, true, true)
		req := &analyticspb.RetrieveQAScoreStatsRequest{
			FilterByAttribute: &analyticspb.QAAttribute{
				ScorecardStatuses: []analyticspb.QAAttribute_ScorecardStatus{
					analyticspb.QAAttribute_DRAFT,
				},
			},
		}

		// When
		err := server.applyScorecardStatusFilter(ctx, req, parent)

		// Then
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.PermissionDenied, st.Code())
	})
}

func TestGroupTemplatesByConversationDurationFilter(t *testing.T) {
	const (
		emptyConfig   = `{"conversationFilters": {}}`
		bucketAConfig = `{"conversationFilters": {"conversationDurationBuckets": [{"id": "bucket-a"}]}}`
		bucketBConfig = `{"conversationFilters": {"conversationDurationBuckets": [{"id": "bucket-b"}]}}`
	)
	template := func(qaScoreConfig sql.NullString) *dbmodel.ScorecardTemplates {
		return &dbmodel.ScorecardTemplates{QaScoreConfig: qaScoreConfig}
	}

	t.Run("EmptyAndNullConfigsCollapseIntoOneGroup", func(t *testing.T) {
		// Given — three templates that carry no duration filter: one with a NULL config and two with a
		// non-null but content-empty config. The mere presence of a QaScoreConfig must not force a
		// separate query.
		templates := map[string]*dbmodel.ScorecardTemplates{
			"tpl-null":   template(sql.NullString{}),
			"tpl-empty1": template(sql.NullString{String: emptyConfig, Valid: true}),
			"tpl-empty2": template(sql.NullString{String: "{}", Valid: true}),
		}

		// When
		groups, err := groupTemplatesByConversationDurationFilter(templates)

		// Then
		require.NoError(t, err)
		require.Len(t, groups, 1, "templates without a duration filter must collapse into a single query")
		require.Empty(t, groups[0].durationBuckets)
		require.Equal(t, []string{"tpl-empty1", "tpl-empty2", "tpl-null"}, groups[0].templateIDs)
	})

	t.Run("TemplatesSharingADurationFilterShareAGroup", func(t *testing.T) {
		// Given — two templates with the same duration bucket and one with a different bucket.
		templates := map[string]*dbmodel.ScorecardTemplates{
			"tpl-a1": template(sql.NullString{String: bucketAConfig, Valid: true}),
			"tpl-a2": template(sql.NullString{String: bucketAConfig, Valid: true}),
			"tpl-b":  template(sql.NullString{String: bucketBConfig, Valid: true}),
		}

		// When
		groups, err := groupTemplatesByConversationDurationFilter(templates)

		// Then
		require.NoError(t, err)
		require.Len(t, groups, 2, "templates are grouped by their duration filter, not one query per template")
		groupForTemplate := map[string][]string{}
		for _, group := range groups {
			for _, templateID := range group.templateIDs {
				groupForTemplate[templateID] = group.templateIDs
			}
		}
		require.Equal(t, []string{"tpl-a1", "tpl-a2"}, groupForTemplate["tpl-a1"])
		require.Equal(t, groupForTemplate["tpl-a1"], groupForTemplate["tpl-a2"], "tpl-a1 and tpl-a2 must share a group")
		require.Equal(t, []string{"tpl-b"}, groupForTemplate["tpl-b"])
	})

	t.Run("MixOfFilteredAndUnfilteredTemplates", func(t *testing.T) {
		// Given — empty-config templates collapse together, while a filtered template stays separate.
		templates := map[string]*dbmodel.ScorecardTemplates{
			"tpl-empty1": template(sql.NullString{String: "{}", Valid: true}),
			"tpl-empty2": template(sql.NullString{}),
			"tpl-bucket": template(sql.NullString{String: bucketAConfig, Valid: true}),
		}

		// When
		groups, err := groupTemplatesByConversationDurationFilter(templates)

		// Then
		require.NoError(t, err)
		require.Len(t, groups, 2)
		// Groups are sorted by signature; "" (no filter) sorts before "bucket-a".
		require.Equal(t, []string{"tpl-empty1", "tpl-empty2"}, groups[0].templateIDs)
		require.Empty(t, groups[0].durationBuckets)
		require.Equal(t, []string{"tpl-bucket"}, groups[1].templateIDs)
		require.Len(t, groups[1].durationBuckets, 1)
		require.Equal(t, "bucket-a", groups[1].durationBuckets[0].GetId())
	})
}
