package analyticsimpl

import (
	"fmt"

	"github.com/cresta/shared-go/container/set"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
)

func groupInternalUserOutcomeStats(
	perUserStats []*internalUserOutcomeStats,
	groups []*userpb.Group,
	userNameToGroupNamesMap map[string][]string,
) ([]*internalUserOutcomeStats, error) {
	allowedGroupNames := make([]string, 0, len(groups))
	for _, group := range groups {
		allowedGroupNames = append(allowedGroupNames, group.GetName())
	}
	allowedGroupNamesSet := set.NewFromSlice(allowedGroupNames)

	perGroupStatsByKey := make(map[string]*internalUserOutcomeStats)

	for _, stat := range perUserStats {
		userName := stat.GroupBy.GetUser()
		if userName == "" {
			return nil, status.Errorf(codes.FailedPrecondition, "empty user attribute")
		}

		groupNames, exists := userNameToGroupNamesMap[userName]
		if !exists {
			continue
		}

		for _, groupName := range groupNames {
			if !allowedGroupNamesSet.Has(groupName) {
				continue
			}

			key := outcomeStatsKey(groupName, stat.GroupBy.GetTimeRange(), stat.GroupBy.GetOutcomeFieldDefinitionName())

			if _, exists := perGroupStatsByKey[key]; !exists {
				groupBy := proto.Clone(stat.GroupBy).(*analyticspb.UserOutcomeStatsGroupBy)
				groupBy.User = ""
				groupBy.Group = groupName
				perGroupStatsByKey[key] = &internalUserOutcomeStats{
					GroupBy: groupBy,
				}
			}
			aggregateInternalOutcomeStat(perGroupStatsByKey[key], stat)
		}
	}

	perGroupStats := make([]*internalUserOutcomeStats, 0, len(perGroupStatsByKey))
	for _, stat := range perGroupStatsByKey {
		perGroupStats = append(perGroupStats, stat)
	}
	return perGroupStats, nil
}

func aggregateInternalOutcomeStat(target *internalUserOutcomeStats, source *internalUserOutcomeStats) {
	target.RecordCount += source.RecordCount
	target.ValueSum += source.ValueSum
	target.WeightSum += source.WeightSum
	target.WeightedValueSum += source.WeightedValueSum
}

func outcomeStatsKey(groupName string, timeRange *analyticspb.TimeRange, outcomeFieldDefinitionName string) string {
	timeRangeString := ""

	if timeRange != nil {
		timeRangeString = timeRange.String()
	}

	return fmt.Sprintf(
		"%d:%s|%d:%s|%d:%s",
		len(groupName), groupName,
		len(timeRangeString), timeRangeString,
		len(outcomeFieldDefinitionName), outcomeFieldDefinitionName,
	)
}
