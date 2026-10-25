package analyticsimpl

import (
	"sort"

	"github.com/cresta/go-servers/insights-server/internal/shared"

	useroutcomes "github.com/cresta/go-servers/shared/clickhouse/conversations/useroutcomes"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/metadataframework"
)

type internalUserOutcomeStats struct {
	GroupBy          *analyticspb.UserOutcomeStatsGroupBy
	RecordCount      int32
	ValueSum         float64
	WeightSum        float64
	WeightedValueSum float64
}

func transformCHRowsToInternalUserOutcomeStats(
	rows []CHOutcomeStatsRow,
	spec *UserOutcomeQuerySpec,
) []*internalUserOutcomeStats {
	internalStats := make([]*internalUserOutcomeStats, 0, len(rows))
	for _, row := range rows {
		internalStats = append(internalStats, &internalUserOutcomeStats{
			GroupBy:          transformCHRowToUserInternalOutcomeStatsGroupBy(row, spec),
			RecordCount:      int32(row.RecordCount),
			ValueSum:         row.ValueSum,
			WeightSum:        row.WeightSum,
			WeightedValueSum: row.WeightedValueSum,
		})
	}
	return internalStats
}

func transformCHRowToUserInternalOutcomeStatsGroupBy(
	row CHOutcomeStatsRow,
	spec *UserOutcomeQuerySpec,
) *analyticspb.UserOutcomeStatsGroupBy {
	groupBy := &analyticspb.UserOutcomeStatsGroupBy{}
	for _, attribute := range spec.GroupByAttributeTypes {
		switch attribute {
		case analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER,
			analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_AGENT_TIER:
			groupBy.User = buildUserResourceName(spec.Parent.CustomerID, row.UserID)
		case analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_TIME_RANGE:
			groupBy.TimeRange = &analyticspb.TimeRange{
				StartTimestamp: shared.ConvertTimeToAPITimestamp(row.BucketTime),
			}
		case analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_OUTCOME_FIELD:
			groupBy.OutcomeFieldDefinitionName = metadatapb.FieldDefinitionName{
				CustomerID:        spec.Parent.CustomerID,
				ProfileID:         spec.Parent.ProfileID,
				ResourceTypeID:    useroutcomes.UserOutcomeResourceTypeID,
				FieldDefinitionID: row.FieldDefinitionID,
			}.String()
		}
	}
	return groupBy
}

func transformInternalUserOutcomeStatsToResponse(
	internalStats []*internalUserOutcomeStats,
	aggregationMode analyticspb.AggregationMode,
) *analyticspb.RetrieveUserOutcomeStatsResponse {
	userOutcomeStatsResponse := &analyticspb.RetrieveUserOutcomeStatsResponse{
		UserOutcomeStatsSummary: &analyticspb.UserOutcomeStatsSummary{},
		UserOutcomeStats:        []*analyticspb.UserOutcomeStats{},
	}

	totalRecordCount := 0

	for _, stat := range internalStats {
		totalRecordCount += int(stat.RecordCount)
		userOutcomeStatsResponse.UserOutcomeStats = append(userOutcomeStatsResponse.UserOutcomeStats,
			&analyticspb.UserOutcomeStats{
				GroupBy:     stat.GroupBy,
				Value:       computeOutcomeStatValue(stat, aggregationMode),
				RecordCount: stat.RecordCount,
			},
		)
	}

	sort.Slice(userOutcomeStatsResponse.UserOutcomeStats, func(i, j int) bool {
		timeRangeStat1 := userOutcomeStatsResponse.UserOutcomeStats[i].GroupBy.TimeRange
		timeRangeStat2 := userOutcomeStatsResponse.UserOutcomeStats[j].GroupBy.TimeRange

		if timeRangeStat1 == nil || timeRangeStat2 == nil {
			return false
		}
		return timeRangeStat1.StartTimestamp.AsTime().Before(timeRangeStat2.StartTimestamp.AsTime())
	})

	userOutcomeStatsResponse.UserOutcomeStatsSummary.RecordCount = int32(totalRecordCount)

	return userOutcomeStatsResponse
}

func computeOutcomeStatValue(stat *internalUserOutcomeStats, aggregationMode analyticspb.AggregationMode) float64 {
	switch aggregationMode {
	case analyticspb.AggregationMode_AGGREGATION_MODE_AVERAGE:
		return safeDivideFloat64(stat.ValueSum, float64(stat.RecordCount))
	case analyticspb.AggregationMode_AGGREGATION_MODE_WEIGHTED_AVERAGE:
		return safeDivideFloat64(stat.WeightedValueSum, stat.WeightSum)
	case analyticspb.AggregationMode_AGGREGATION_MODE_SUM:
		return stat.ValueSum
	default:
		return 0
	}
}
