package analyticsimpl

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

func metadataValuesClickhouseQuery(
	metadataType metadatapb.ConversationMetadataEnum_Type,
	momentTemplateID, metadataStringContains string,
	usecaseConditionAndArgs []conditionAndArg,
	numericMinMaxOnly bool,
) (string, []any, error) {
	// Add usecase filter.
	usecaseCondition, usecaseArgs := concatConditionsAndArgs(usecaseConditionAndArgs)
	optionalUsecaseFilter := ""
	if len(usecaseCondition) > 0 {
		optionalUsecaseFilter = fmt.Sprintf("AND %s -- USECASE_FILTER", usecaseCondition)
	}

	// Handle numeric min-max
	if metadataType == metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER && numericMinMaxOnly {
		// Parameterized (SECH1-49, defense-in-depth): momentTemplateID is
		// server-validated via BatchGetMoments and not directly caller-controlled,
		// but bind it as an arg anyway so no quoted-value interpolation remains.
		return fmt.Sprintf(`
                SELECT
                    min(metadata_number_value) AS min,
                    max(metadata_number_value) AS max
                FROM
                    metadata_moment_value_count_mv_d
                WHERE
                    moment_template_id = ?
                    %s`,
			optionalUsecaseFilter), append([]any{momentTemplateID}, usecaseArgs...), nil

	}

	optionalMetadataStringContains := ""
	metadataColumn := ""
	switch metadataType {
	case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
		metadatapb.ConversationMetadataEnum_TYPE_VISITOR_GEO_COUNTRY,
		metadatapb.ConversationMetadataEnum_TYPE_VISITOR_GEO_STATE,
		metadatapb.ConversationMetadataEnum_TYPE_VISITOR_DEVICE_TYPE,
		metadatapb.ConversationMetadataEnum_TYPE_VISITOR_DEVICE_OS,
		metadatapb.ConversationMetadataEnum_TYPE_CHAT_SKILL,
		metadatapb.ConversationMetadataEnum_TYPE_AGENT_SURVEY:
		metadataColumn = "metadata_string_value"
		// Add metadata string contains filter.
		// Parameterized (SECH1-48): bind the value as an arg instead of
		// interpolating it into the query text, so a caller-supplied string
		// can never break out of the LIKE literal.
		if len(metadataStringContains) > 0 {
			optionalMetadataStringContains = "AND metadata_string_value LIKE ?"
			usecaseArgs = append(usecaseArgs, "%"+metadataStringContains+"%")
		}
	case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER:
		metadataColumn = "metadata_number_value"
	case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_BOOLEAN:
		metadataColumn = "metadata_bool_value"
	default:
		return "", nil, status.Errorf(codes.InvalidArgument, "unsupported metadata type %v", metadataType)
	}

	return fmt.Sprintf(`
			SELECT
				%s, uniqMerge(metadata_value_count)
			FROM
				metadata_moment_value_count_mv_d
			WHERE
				moment_template_id = ?
				%s
				%s
			GROUP BY 1
			ORDER BY 2 DESC
			LIMIT %d`,
		metadataColumn,
		optionalUsecaseFilter,
		optionalMetadataStringContains,
		maxMetadataValueCount), append([]any{momentTemplateID}, usecaseArgs...), nil

}

func (a AnalyticsServiceImpl) readMetadataValuesFromClickhouse(
	ctx context.Context,
	parent *shared.CustomerProfile,
	filterOnConversation *analyticspb.RetrieveMetadataValuesRequest_FilterOnConversation,
	metadataType metadatapb.ConversationMetadataEnum_Type,
	momentTemplateID, metadataStringContains string,
	numericMinMaxOnly bool,
) (*analyticspb.RetrieveMetadataValuesResponse, error) {
	// Add usecase filter.
	usecaseIDs := []string{}
	var err error
	if filterOnConversation != nil {
		usecaseIDs, err = parseUsercaseNames(filterOnConversation.UsecaseNames)
		if err != nil {
			return nil, err
		}
	}
	usecaseConditionAndArgs := []conditionAndArg{}
	if len(usecaseIDs) > 0 {
		con, arg := columnIn(usecaseIDColumn, usecaseIDs)
		usecaseConditionAndArgs = append(usecaseConditionAndArgs, conditionAndArg{
			condition: con,
			arg:       arg,
		})
	}
	// Get Clickhouse query.
	chQuery, chArgs, err := metadataValuesClickhouseQuery(metadataType, momentTemplateID, metadataStringContains, usecaseConditionAndArgs, numericMinMaxOnly)
	if err != nil {
		return nil, err
	}
	a.logger.Infof(ctx, "clickhouse query: %v", chQuery)

	conn, err := a.getConversationsClickhouseConn(ctx, parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %w", err)
	}
	defer rows.Close()

	// Handle numeric min-max case
	if metadataType == metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER && numericMinMaxOnly {
		var minVal, maxVal float64
		if rows.Next() {
			if err := rows.Scan(&minVal, &maxVal); err != nil {
				return nil, fmt.Errorf("scanning min-max values: %w", err)
			}
			if minVal != 0 || maxVal != 0 {
				return &analyticspb.RetrieveMetadataValuesResponse{
					MetadataValues: []*metadatapb.MetadataValue{
						{Value: &metadatapb.MetadataValue_NumberValue{NumberValue: float32(minVal)}},
						{Value: &metadatapb.MetadataValue_NumberValue{NumberValue: float32(maxVal)}},
					},
				}, nil
			}
		}
		return &analyticspb.RetrieveMetadataValuesResponse{
			MetadataValues: []*metadatapb.MetadataValue{},
		}, nil
	}

	// Read metadata values.
	var metadataValues []*metadatapb.MetadataValue
	for rows.Next() {
		// Read metadata value based on metadata type.
		val := metadatapb.MetadataValue{}
		count := uint64(0)
		switch metadataType {
		case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
			metadatapb.ConversationMetadataEnum_TYPE_VISITOR_GEO_COUNTRY,
			metadatapb.ConversationMetadataEnum_TYPE_VISITOR_GEO_STATE,
			metadatapb.ConversationMetadataEnum_TYPE_VISITOR_DEVICE_TYPE,
			metadatapb.ConversationMetadataEnum_TYPE_VISITOR_DEVICE_OS,
			metadatapb.ConversationMetadataEnum_TYPE_CHAT_SKILL,
			metadatapb.ConversationMetadataEnum_TYPE_AGENT_SURVEY:
			stringValue := ""
			if err := rows.Scan(&stringValue, &count); err != nil {
				return nil, fmt.Errorf("[metadataValues] scanning clickhouse rows: %w", err)
			}
			val.Value = &metadatapb.MetadataValue_StringValue{
				StringValue: stringValue,
			}
		case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER:
			numericValue := float64(0)
			if err := rows.Scan(&numericValue, &count); err != nil {
				return nil, fmt.Errorf("[metadataValues] scanning clickhouse rows: %w", err)
			}
			// This is not an ideal solution. In theory we should have defined better
			// interface to filter on numberic value based metadata.
			if len(metadataStringContains) > 0 && !strings.Contains(strconv.FormatFloat(numericValue, 'f', 2, 64), metadataStringContains) {
				continue
			}
			val.Value = &metadatapb.MetadataValue_NumberValue{
				NumberValue: float32(numericValue),
			}
		case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_BOOLEAN:
			booleanValue := false
			if err := rows.Scan(&booleanValue, &count); err != nil {
				return nil, fmt.Errorf("[metadataValues] scanning clickhouse rows: %w", err)
			}
			val.Value = &metadatapb.MetadataValue_BooleanValue{
				BooleanValue: booleanValue,
			}
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unsupported metadata type %v", metadataType)
		}

		metadataValues = append(metadataValues, &val)
	}

	return &analyticspb.RetrieveMetadataValuesResponse{
		MetadataValues: metadataValues,
	}, nil
}
