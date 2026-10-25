package analyticsimpl

import (
	"os"
	"strconv"

	trendandanomalypb "github.com/cresta/cresta-proto/v2/gen/go/cresta/ai_service/trend_and_anomaly"
	internaljobpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/job"
	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	behaviorpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/behavior"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/metadataframework"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/go-servers/shared/featureflag"
	"github.com/cresta/shared-go/container/cache"
	grpcRegistry "github.com/cresta/shared-go/framework/client/grpc/registry"
	logger "github.com/cresta/shared-go/framework/log"
	"github.com/elastic/go-elasticsearch/v8"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/auth"
	clickhouseeventsclient "github.com/cresta/go-servers/shared/clickhouse/events/client"
	"github.com/cresta/go-servers/shared/clickhouse/registry"
	"github.com/cresta/go-servers/shared/conversation/converter"
	dbRegistry "github.com/cresta/go-servers/shared/db/registry"
	"github.com/cresta/go-servers/shared/prometheus"
)

type AnalyticsServiceImpl struct {
	analyticspb.UnimplementedAnalyticsServiceServer
	appsDB                                                     *dbRegistry.AppsDB
	esClient                                                   *elasticsearch.Client
	momentClientRegistry                                       grpcRegistry.Registry[momentpb.MomentServiceClient]
	behaviorClientRegistry                                     grpcRegistry.Registry[behaviorpb.BehaviorServiceClient]
	coachingClientRegistry                                     grpcRegistry.Registry[coachingpb.CoachingServiceClient]
	userClientRegistry                                         grpcRegistry.Registry[userpb.UserServiceClient]
	authUserServiceClient                                      userpb.UserServiceClient
	trendAndAnomalyClientRegistry                              grpcRegistry.Registry[trendandanomalypb.TrendAndAnomalyServiceClient]
	internalJobServiceClientRegistry                           grpcRegistry.Registry[internaljobpb.InternalJobServiceClient]
	internalUserServiceClientRegistry                          grpcRegistry.Registry[internaluserpb.InternalUserServiceClient]
	metadataFrameworkClientRegistry                            grpcRegistry.Registry[metadatapb.MetadataServiceClient]
	configClient                                               config.Client
	environment                                                shared.Environment
	clickhouseConvoRegistry                                    registry.ConversationsClickhouseRegistry
	clickhouseAuthEventRegistry                                registry.AuthEventsClickhouseRegistry
	clickhouseEventRegistry                                    registry.EventsClickhouseRegistry
	clickhouseEventsClient                                     *clickhouseeventsclient.ClickhouseEventsClient
	stage                                                      shared.Stage
	appEnv                                                     string
	esUseCase                                                  string
	esIndexVersion                                             string
	maxConcurrentQaScoreStatsQueries                           int
	logger                                                     logger.Logger
	enableAgentCountFromCrestaConversation                     bool
	enableListUsersCache                                       bool
	enableUsageOfExplicitMappingMomentAnnotationPayload        bool
	enableACLFilteringInEvaluateAnalyticsChart                 bool
	enableSimplifiedSubcategoryGroupByInEvaluateAnalyticsChart bool
	enableExtTableForUserFilter                                bool
	enableDirectionality                                       bool
	converter                                                  *converter.Converter
	resourceACLHelperProvider                                  auth.ResourceACLHelperProvider
	listUsersCache                                             *shared.ListUsersCache
	suggestedConvoCache                                        *shared.SuggestedConvoCache
	authEntityProvider                                         auth.AuthEntityProvider
	featureFlagClient                                          featureflag.Client
	prometheusClient                                           prometheus.Client
}

const (
	TrueString = "true"
	RootUser   = "root"
)

func Init(
	appsDB *dbRegistry.AppsDB,
	esClient *elasticsearch.Client,
	momentClientRegistry grpcRegistry.Registry[momentpb.MomentServiceClient],
	behaviorClientRegistry grpcRegistry.Registry[behaviorpb.BehaviorServiceClient],
	coachingClientRegistry grpcRegistry.Registry[coachingpb.CoachingServiceClient],
	userClientRegistry grpcRegistry.Registry[userpb.UserServiceClient],
	trendAndAnomalyClientRegistry grpcRegistry.Registry[trendandanomalypb.TrendAndAnomalyServiceClient],
	internalJobServiceClientRegistry grpcRegistry.Registry[internaljobpb.InternalJobServiceClient],
	internalUserServiceClientRegistry grpcRegistry.Registry[internaluserpb.InternalUserServiceClient],
	authUserServiceClient userpb.UserServiceClient,
	metadataFrameworkClientRegistry grpcRegistry.Registry[metadatapb.MetadataServiceClient],
	configClient config.Client,
	logger logger.Logger,
	converter *converter.Converter,
	clickhouseConvoRegistry registry.ConversationsClickhouseRegistry,
	clickhouseAuthEventRegistry registry.AuthEventsClickhouseRegistry,
	clickhouseEventRegistry registry.EventsClickhouseRegistry,
	clickhouseEventsClient *clickhouseeventsclient.ClickhouseEventsClient,
	aclHelperProvider auth.ResourceACLHelperProvider,
	authEntityProvider auth.AuthEntityProvider,
	featureFlagClient featureflag.Client,
	prometheusClient prometheus.Client,
) (*AnalyticsServiceImpl, error) {
	appEnv := os.Getenv("APP_ENV")

	esUseCase := os.Getenv("ES_USE_CASE")
	if len(esUseCase) == 0 {
		esUseCase = "conversations"
	}
	esIndexVersion := os.Getenv("ES_INDEX_VERSION")
	if len(esIndexVersion) == 0 {
		esIndexVersion = "2"
	}

	environmentString := os.Getenv("ENVIRONMENT_ID")
	var environment shared.Environment
	var stage shared.Stage
	switch environmentString {
	case "voice-prod":
		environment = shared.Voice
		stage = shared.Prod
	case "voice-staging":
		environment = shared.Voice
		stage = shared.Staging
	case "chat-prod":
		environment = shared.Chat
		stage = shared.Prod
	default:
		environment = shared.UnspecifiedEnvironment
		stage = shared.UnspecifiedStage
	}
	enableAgentCountFromCrestaConversation := false
	enableAgentCountFromCrestaConversationString := os.Getenv("ENABLE_AGENT_COUNT_FROM_CRESTA_CONVERSATION")
	if enableAgentCountFromCrestaConversationString == TrueString {
		enableAgentCountFromCrestaConversation = true
	}
	enableListUsersCache := false
	enableListUsersCacheString := os.Getenv("ENABLE_LIST_USERS_MAPPED_TO_GROUPS_CACHE")
	if enableListUsersCacheString == TrueString {
		enableListUsersCache = true
	}
	enableUsageOfExplicitMappingMomentAnnotationPayload := false
	enableUsageOfExplicitMappingMomentAnnotationPayloadString := os.Getenv("ENABLE_USAGE_OF_EXPLICIT_MAPPING_MOMENT_ANNOTATION_PAYLOAD")
	if enableUsageOfExplicitMappingMomentAnnotationPayloadString == TrueString {
		enableUsageOfExplicitMappingMomentAnnotationPayload = true
	}
	enableACLFilteringInEvaluateAnalyticsChart := false
	enableACLFilteringInEvaluateAnalyticsChartString := os.Getenv("ENABLE_ACL_FILTERING_IN_EVALUATE_ANALYTICS_CHART")
	if enableACLFilteringInEvaluateAnalyticsChartString == TrueString {
		enableACLFilteringInEvaluateAnalyticsChart = true
	}
	enableSimplifiedSubcategoryGroupByInEvaluateAnalyticsChart := false
	enableSimplifiedSubcategoryGroupByInEvaluateAnalyticsChartString := os.Getenv("ENABLE_SIMPLIFIED_SUBCATEGORY_GROUP_BY_IN_EVALUATE_ANALYTICS_CHART")
	if enableSimplifiedSubcategoryGroupByInEvaluateAnalyticsChartString == TrueString {
		enableSimplifiedSubcategoryGroupByInEvaluateAnalyticsChart = true
	}
	enableExtTableForUserFilter := false
	enableExtTableForUserFilterString := os.Getenv("ENABLE_EXT_TABLE_FOR_USER_FILTER")
	if enableExtTableForUserFilterString == TrueString {
		enableExtTableForUserFilter = true
	}
	enableDirectionality := false
	enableDirectionalityString := os.Getenv("ENABLE_DIRECTIONALITY")
	if enableDirectionalityString == TrueString {
		enableDirectionality = true
	}
	maxConcurrentQaScoreStatsQueries := defaultMaxConcurrentQaScoreStatsQueries
	if v := os.Getenv("MAX_CONCURRENT_QA_SCORE_STATS_QUERIES"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			maxConcurrentQaScoreStatsQueries = parsed
		}
	}
	return &AnalyticsServiceImpl{
		appsDB:                 appsDB,
		esClient:               esClient,
		momentClientRegistry:   momentClientRegistry,
		behaviorClientRegistry: behaviorClientRegistry,
		coachingClientRegistry: coachingClientRegistry,
		userClientRegistry:     userClientRegistry,
		// TODO(zhiyang.lu): replace userClientRegistry with userServiceClient
		authUserServiceClient:                  authUserServiceClient,
		trendAndAnomalyClientRegistry:          trendAndAnomalyClientRegistry,
		internalJobServiceClientRegistry:       internalJobServiceClientRegistry,
		internalUserServiceClientRegistry:      internalUserServiceClientRegistry,
		metadataFrameworkClientRegistry:        metadataFrameworkClientRegistry,
		configClient:                           configClient,
		clickhouseConvoRegistry:                clickhouseConvoRegistry,
		clickhouseAuthEventRegistry:            clickhouseAuthEventRegistry,
		clickhouseEventRegistry:                clickhouseEventRegistry,
		clickhouseEventsClient:                 clickhouseEventsClient,
		environment:                            environment,
		stage:                                  stage,
		esUseCase:                              esUseCase,
		esIndexVersion:                         esIndexVersion,
		maxConcurrentQaScoreStatsQueries:       maxConcurrentQaScoreStatsQueries,
		appEnv:                                 appEnv,
		logger:                                 logger,
		enableAgentCountFromCrestaConversation: enableAgentCountFromCrestaConversation,
		enableListUsersCache:                   enableListUsersCache,
		enableUsageOfExplicitMappingMomentAnnotationPayload:        enableUsageOfExplicitMappingMomentAnnotationPayload,
		enableACLFilteringInEvaluateAnalyticsChart:                 enableACLFilteringInEvaluateAnalyticsChart,
		enableSimplifiedSubcategoryGroupByInEvaluateAnalyticsChart: enableSimplifiedSubcategoryGroupByInEvaluateAnalyticsChart,
		enableExtTableForUserFilter:                                enableExtTableForUserFilter,
		enableDirectionality:                                       enableDirectionality,
		converter:                                                  converter,
		resourceACLHelperProvider:                                  aclHelperProvider,
		listUsersCache: &shared.ListUsersCache{
			UsersToGroupsCache: cache.New(
				cache.WithDefaultExpirationTimeout[string, map[string][]string](*shared.ListUsersCacheTimeout),
				cache.WithCleanupInterval[string, map[string][]string](*shared.ListUsersCacheCleanupTimeout),
			),
			UsersCache: cache.New(
				cache.WithDefaultExpirationTimeout[string, []*userpb.User](*shared.ListUsersCacheTimeout),
				cache.WithCleanupInterval[string, []*userpb.User](*shared.ListUsersCacheCleanupTimeout),
			),
			GroupsCache: cache.New(
				cache.WithDefaultExpirationTimeout[string, []*userpb.Group](*shared.ListUsersCacheTimeout),
				cache.WithCleanupInterval[string, []*userpb.Group](*shared.ListUsersCacheCleanupTimeout),
			),
		},
		suggestedConvoCache: &shared.SuggestedConvoCache{
			ConversationSuggested: cache.New(
				cache.WithDefaultExpirationTimeout[string, string](*shared.SuggestedConvoCacheTimeout),
				cache.WithCleanupInterval[string, string](*shared.SuggestedConvoCacheCleanupTimeout),
			),
		},
		authEntityProvider: authEntityProvider,
		featureFlagClient:  featureFlagClient,
		prometheusClient:   prometheusClient,
	}, nil
}

var _ analyticspb.AnalyticsServiceServer = &AnalyticsServiceImpl{}
