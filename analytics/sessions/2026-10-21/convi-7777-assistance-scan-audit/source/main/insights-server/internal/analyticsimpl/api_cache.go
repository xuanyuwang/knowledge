package analyticsimpl

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"google.golang.org/protobuf/proto"

	"github.com/cresta/go-servers/shared/requestcache"
	"github.com/cresta/shared-go/framework/stats"
	"github.com/cresta/shared-go/util/envflag"
)

var (
	// NOTE(dhoske): This is an envflag for sake of expediency to quickly
	// mitigate customer pain. Ideally this would be in the profile configs.
	insightsStatsAPICachingCustomerIDs = envflag.Custom(
		"INSIGHTS_STATS_API_CACHING_CUSTOMER_IDS",
		&map[string]struct{}{},
		parseCommaSeparatedSet,
		"Comma-separated customer IDs for which insights Retrieve* stats APIs may use the request cache.",
	)
	insightsAnnotatedTimeSeriesCachingCustomerIDs = envflag.Custom(
		"INSIGHTS_ANNOTATED_TIME_SERIES_CACHING_CUSTOMER_IDS",
		&map[string]struct{}{},
		parseCommaSeparatedSet,
		"Comma-separated customer IDs for which RetrieveAnnotatedTimeSeries may use the request cache.",
	)
	insightsQAScoreStatsCachingCustomerIDs = envflag.Custom(
		"INSIGHTS_QA_SCORE_STATS_CACHING_CUSTOMER_IDS",
		&map[string]struct{}{},
		parseCommaSeparatedSet,
		"Comma-separated customer IDs for which RetrieveQAScoreStats may use the request cache.",
	)
	insightsConversationMessageStatsCachingCustomerIDs = envflag.Custom(
		"INSIGHTS_CONVERSATION_MESSAGE_STATS_CACHING_CUSTOMER_IDS",
		&map[string]struct{}{},
		parseCommaSeparatedSet,
		"Comma-separated customer IDs for which RetrieveConversationMessageStats may use the request cache.",
	)
	insightsConversationMessageStatsCachingAllCustomers = envflag.Bool(
		"INSIGHTS_CONVERSATION_MESSAGE_STATS_CACHING_ALL_CUSTOMERS",
		false,
		"When true, all customers may use the RetrieveConversationMessageStats request cache.",
	)
	insightsConversationMessageStatsCachingRequireTopicMoments = envflag.Bool(
		"INSIGHTS_CONVERSATION_MESSAGE_STATS_CACHING_REQUIRE_TOPIC_MOMENTS",
		true,
		"When true, RetrieveConversationMessageStats caching is only allowed if all moments in groupBy moment groups are of type TOPIC. (Only expected from topic discovery.)",
	)
	insightsStatsAPICachingDuration = envflag.Duration(
		"INSIGHTS_STATS_API_CACHING_DURATION",
		4*time.Hour,
		"Maximum age of cached responses for insights stats APIs when caching is enabled for the customer.",
	)
	cacheMetric = stats.NewMetric("insights.api_cache")
)

func parseCommaSeparatedSet(s string) (*map[string]struct{}, error) {
	m := make(map[string]struct{})
	for _, id := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			m[trimmed] = struct{}{}
		}
	}
	return &m, nil
}

func sortAttributeUsersAndGroupsForStatsCache(attr *analyticspb.Attribute) {
	if attr == nil {
		return
	}
	sort.SliceStable(attr.Users, func(i, j int) bool {
		return attr.Users[i].GetName() < attr.Users[j].GetName()
	})
	sort.SliceStable(attr.Groups, func(i, j int) bool {
		return attr.Groups[i].GetName() < attr.Groups[j].GetName()
	})
}

func sortQAAttributeUsersAndGroupsForStatsCache(attr *analyticspb.QAAttribute) {
	if attr == nil {
		return
	}
	sort.SliceStable(attr.Users, func(i, j int) bool {
		return attr.Users[i].GetName() < attr.Users[j].GetName()
	})
	sort.SliceStable(attr.Groups, func(i, j int) bool {
		return attr.Groups[i].GetName() < attr.Groups[j].GetName()
	})
}

func sortRetrieveMetadataValuesRequestForStatsCache(req *analyticspb.RetrieveMetadataValuesRequest) {
	if req == nil || req.FilterOnConversation == nil {
		return
	}
	sort.Strings(req.FilterOnConversation.UsecaseNames)
}

func sortRetrieveConversationMessageStatsRequestForCache(req *analyticspb.RetrieveConversationMessageStatsRequest) {
	if req == nil {
		return
	}
	sortAttributeUsersAndGroupsForStatsCache(req.FilterConversationByAttribute)
	sortAttributeUsersAndGroupsForStatsCache(req.FilterMessageByAttribute)
	sortMomentsInAttributeForStatsCache(req.FilterConversationByAttribute)
	sortMomentsInAttributeForStatsCache(req.FilterMessageByAttribute)
	sortMomentsInGroupByAttributesForStatsCache(req.GroupByAttributes)
}

func sortMomentsInGroupByAttributesForStatsCache(attrs []*analyticspb.AttributeStructure) {
	for _, attr := range attrs {
		mg := attr.GetMomentGroup()
		if mg == nil {
			continue
		}
		sortMomentsByNameStable(mg.Moments)
		sortMomentsByNameStable(mg.ExcludedMoments)
	}
}

func sortMomentsInAttributeForStatsCache(attr *analyticspb.Attribute) {
	if attr == nil {
		return
	}
	for _, mg := range attr.MomentGroups {
		sortMomentsByNameStable(mg.Moments)
		sortMomentsByNameStable(mg.ExcludedMoments)
	}
}

func sortMomentsByNameStable(moments []*momentpb.Moment) {
	sort.SliceStable(moments, func(i, j int) bool {
		return moments[i].GetName() < moments[j].GetName()
	})
}

func conversationMessageStatsCachingEligible(profileName *customerpb.ProfileName) bool {
	if !*insightsConversationMessageStatsCachingAllCustomers {
		if _, ok := (*insightsConversationMessageStatsCachingCustomerIDs)[profileName.GetCustomerID()]; !ok {
			return false
		}
	}
	return true
}

// groupByMomentsAllTopicForCache returns true when the TOPIC-only caching guard is disabled,
// or when: there is at least one groupBy attribute, every groupBy attribute is a MomentGroup,
// and every moment in every MomentGroup is of type TOPIC.
//
// Given how big of a product difference caching makes compared to realtime, as of 2026/05/08
// we decided to only cache for Cresta-customers (demos) for topic discovery:
// we decided to only cache for Cresta-customers (demos) for topic discovery:
func groupByMomentsAllTopicForCache(groupByAttributes []*analyticspb.AttributeStructure) bool {
	if !*insightsConversationMessageStatsCachingRequireTopicMoments {
		return true
	}
	if len(groupByAttributes) == 0 {
		return false
	}
	for _, attr := range groupByAttributes {
		mg := attr.GetMomentGroup()
		if mg == nil {
			return false
		}
		if len(mg.GetMoments()) == 0 {
			return false
		}
		for _, m := range mg.GetMoments() {
			if m.GetType() != momentpb.Moment_TOPIC {
				return false
			}
		}
	}
	return true
}

// getInsightsStatsFromCache gets cached results of insights stats APIs from the DB cache.
//
// It returns whether a cached result was found, a store function to cache the response later, and any error.
// The store function is safe to call regardless of whether caching is enabled; it is a no-op when disabled.
func (a AnalyticsServiceImpl) getInsightsStatsFromCache(
	ctx context.Context,
	method string,
	customerAllowlist *map[string]struct{},
	allowAllCustomers bool,
	profileName *customerpb.ProfileName,
	cacheKey proto.Message,
	out proto.Message,
) (bool, func(proto.Message, error), error) {
	noop := func(proto.Message, error) {}
	if !allowAllCustomers {
		if _, ok := (*customerAllowlist)[profileName.GetCustomerID()]; !ok {
			return false, noop, nil
		}
	}
	a.logger.Infof(ctx, "%s: insights api cache enabled for profile %s", method, profileName)
	// WARNING: Do not use DB(ctx) directly here. Some insights APIs don't have
	// profile ID in the context.
	provider, err := a.appsDB.GetProvider(profileName.GetCustomerID(), profileName.GetProfileID())
	if err != nil {
		return false, noop, err
	}
	db := provider.Get(ctx)
	store := func(response proto.Message, err error) {
		if response == nil || err != nil {
			return
		}
		if err := requestcache.StoreRequestCache(db, profileName, cacheKey, response, *insightsStatsAPICachingDuration); err != nil {
			a.logger.Warnf(ctx, "%s: failed to store insights api cache: %v", method, err)
		} else {
			a.logger.Infof(ctx, "%s: insights api cache stored for profile %s", method, profileName)
		}
	}
	found, err := requestcache.GetRequestCache(
		db,
		profileName,
		cacheKey,
		out,
	)
	switch {
	case err != nil:
		cacheMetric.Incr(ctx, fmt.Sprintf("method:%s", method), "status:error")
		a.logger.Warnf(ctx, "%s: insights api cache lookup failed for profile %s: %v", method, profileName, err)
		return found, noop, err
	case found:
		cacheMetric.Incr(ctx, fmt.Sprintf("method:%s", method), "status:hit")
		a.logger.Infof(ctx, "%s: insights api cache hit for profile %s", method, profileName)
		// Do not store if found in TTL in order to avoid refreshing possibly stale data.
		return found, noop, nil
	default:
		cacheMetric.Incr(ctx, fmt.Sprintf("method:%s", method), "status:miss")
		a.logger.Infof(ctx, "%s: insights api cache miss for profile %s", method, profileName)
		return found, store, nil
	}
}
