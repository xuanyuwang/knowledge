package analyticsimpl

import (
	"github.com/cresta/shared-go/util/envflag"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
)

// usePlatformTimestampForMessageCreateTime controls whether ConversationMessage CreateTime prefers
// Elasticsearch `platform_timestamp` over `created_at` (ingestion time). Set to false to revert
// to legacy behavior (red-button kill switch).
var usePlatformTimestampForMessageCreateTime = envflag.Bool(
	"INSIGHTS_USE_PLATFORM_TIMESTAMP_FOR_MESSAGE_CREATE_TIME",
	true,
	"When true, message CreateTime uses ES platform_timestamp when set, else created_at. When false, always uses created_at.",
)

func messageCreateTimeFromESSource(hit elasticsearch.ResHit) (*timestamppb.Timestamp, error) {
	source := hit.Source
	if !*usePlatformTimestampForMessageCreateTime {
		return timestampFromNonEmptyString(source.CreatedAt)
	}
	if val := source.PlatformTimestamp; val != "" {
		return shared.ConvertStringToAPITimestamp(val)
	}
	return timestampFromNonEmptyString(source.CreatedAt)
}

func timestampFromNonEmptyString(timeStr string) (*timestamppb.Timestamp, error) {
	if timeStr == "" {
		return nil, nil
	}
	return shared.ConvertStringToAPITimestamp(timeStr)
}
