package analyticsimpl

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/cresta/shared-go/util/errgroup"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	behaviorpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/behavior"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	enumspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/enums"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/fn"
	"github.com/cresta/shared-go/middleware/rpcauth"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	sharedes "github.com/cresta/go-servers/shared/conversation/elastic"
	"github.com/cresta/go-servers/shared/qa"
	"github.com/cresta/go-servers/shared/scoring"
	"github.com/cresta/go-servers/shared/utils"
)

const (
	// The max number of outstanding ES queries in parallel.
	maxConcurrentESQueries = 10
	// The max number of agents to query in ES.
	maxAgentCount = 1500
	// The percentage thresholds to split the agents into tiers.
	topAgentPercentage    = 0.25
	AvgAgentPercentage    = 0.5
	bottomAgentPercentage = 0.25
	// The default time key for the agent adherence map.
	agentAdherenceDefaultTime = "default"
)

// ES Request
type ReqExtendedStats struct {
	Field string `json:"field,omitempty" mapstructure:"field,omitempty"`
}

type ReqStats struct {
	ExtendedStats *ReqExtendedStats `json:"extended_stats,omitempty" mapstructure:"extended_stats,omitempty"`
}

type ReqAvg struct {
	Field string `json:"field,omitempty" mapstructure:"field,omitempty"`
}

type ReqAvgOutcomes struct {
	Avg *ReqAvg `json:"avg,omitempty" mapstructure:"avg,omitempty"`
}

type ReqBool struct {
	Filter             []*ReqFilter                  `json:"filter,omitempty" mapstructure:"filter,omitempty"`
	Should             *[]elasticsearch.NestedFilter `json:"should,omitempty" mapstructure:"should,omitempty"`
	MinimumShouldMatch int32                         `json:"minimum_should_match,omitempty" mapstructure:"minimum_should_match,omitempty"`
}

type ReqFilter struct {
	Terms    *elasticsearch.NestedTermsFilter `json:"terms,omitempty" mapstructure:"terms,omitempty"`
	HasChild *elasticsearch.HasChild          `json:"has_child,omitempty" mapstructure:"has_child,omitempty"`
	Range    *elasticsearch.RangeFilter       `json:"range,omitempty" mapstructure:"range,omitempty"`
	Bool     *ReqBool                         `json:"bool,omitempty" mapstructure:"bool,omitempty"`
}

type ReqAnnotationFilters struct {
	Filters []ReqAnnotationFilters `json:"filters,omitempty" mapstructure:"filters,omitempty"`
	Bool    *ReqBool               `json:"bool,omitempty" mapstructure:"bool,omitempty"`
}

type ReqFilters struct {
	Filters  []ReqFilters             `json:"filters,omitempty" mapstructure:"filters,omitempty"`
	HasChild *elasticsearch.HasChild  `json:"has_child,omitempty" mapstructure:"has_child,omitempty"`
	Bool     *elasticsearch.ConvoBool `json:"bool,omitempty" mapstructure:"bool,omitempty"`
}

type ReqTerms struct {
	Field string `json:"field,omitempty" mapstructure:"field,omitempty"`
	Size  int32  `json:"size,omitempty" mapstructure:"size,omitempty"`
}

type ReqMoments struct {
	Aggs    *ReqAggs    `json:"aggs,omitempty" mapstructure:"aggs,omitempty"`
	Filters *ReqFilters `json:"filters,omitempty" mapstructure:"filters,omitempty"`
}

type ReqOutcomes struct {
	Nested  *elasticsearch.Nested `json:"nested,omitempty" mapstructure:"nested,omitempty"`
	Aggs    *ReqAggs              `json:"aggs,omitempty" mapstructure:"aggs,omitempty"`
	Filter  *ReqFilter            `json:"filter,omitempty" mapstructure:"filter,omitempty"`
	Filters *ReqAnnotationFilters `json:"filters,omitempty" mapstructure:"filters,omitempty"`
}

type ReqAgents struct {
	Terms *ReqTerms `json:"terms,omitempty" mapstructure:"terms,omitempty"`
	Aggs  *ReqAggs  `json:"aggs,omitempty" mapstructure:"aggs,omitempty"`
}

type ReqSDX struct {
	Aggs   *ReqAggs   `json:"aggs,omitempty" mapstructure:"aggs,omitempty"`
	Filter *ReqFilter `json:"filter,omitempty" mapstructure:"filter,omitempty"`
}

type ReqDDX struct {
	Aggs   *ReqAggs   `json:"aggs,omitempty" mapstructure:"aggs,omitempty"`
	Filter *ReqFilter `json:"filter,omitempty" mapstructure:"filter,omitempty"`
}

type ReqTimeRanges struct {
	DateHistogram *elasticsearch.ReqDateHistogram `json:"date_histogram,omitempty" mapstructure:"date_histogram,omitempty"`
	Aggs          *ReqAggs                        `json:"aggs,omitempty" mapstructure:"aggs,omitempty"`
}

type ReqAggs struct {
	Moments     *ReqMoments     `json:"moments,omitempty" mapstructure:"moments,omitempty"`
	Outcomes    *ReqOutcomes    `json:"outcomes,omitempty" mapstructure:"outcomes,omitempty"`
	Stats       *ReqStats       `json:"stats,omitempty" mapstructure:"stats,omitempty"`
	Agents      *ReqAgents      `json:"agents,omitempty" mapstructure:"agents,omitempty"`
	AvgOutcomes *ReqAvgOutcomes `json:"avg_outcomes,omitempty" mapstructure:"avg_outcomes,omitempty"`
	SDX         *ReqSDX         `json:"sdx,omitempty" mapstructure:"sdx,omitempty"`
	DDX         *ReqDDX         `json:"ddx,omitempty" mapstructure:"ddx,omitempty"`
	TimeRanges  *ReqTimeRanges  `json:"time_ranges,omitempty" mapstructure:"time_ranges,omitempty"`
}

type Request struct {
	Query           *elasticsearch.OuterQuery      `json:"query,omitempty" mapstructure:"query,omitempty"`
	Aggs            *ReqAggs                       `json:"aggs,omitempty" mapstructure:"aggs,omitempty"`
	Size            *int32                         `json:"size,omitempty" mapstructure:"size,omitempty"`
	RuntimeMappings *elasticsearch.RuntimeMappings `json:"runtime_mappings,omitempty" mapstructure:"runtime_mappings,omitempty"`
}

// ES Response
type ResStats struct {
	Count int32   `mapstructure:"count,omitempty"`
	Avg   float32 `mapstructure:"avg,omitempty"`
	Std   float32 `mapstructure:"std_deviation,omitempty"`
}

type ResAvgOutcomes struct {
	Value float32 `mapstructure:"value,omitempty"`
}

type ResSDX struct {
	DocCount int32 `mapstructure:"doc_count,omitempty"`
}

type ResDDX struct {
	DocCount int32 `mapstructure:"doc_count,omitempty"`
}

type ResBuckets struct {
	Outcomes *ResOutcomes `mapstructure:"outcomes,omitempty"`
	SDX      *ResSDX      `mapstructure:"sdx,omitempty"`
	DDX      *ResDDX      `mapstructure:"ddx,omitempty"`
	Key      string       `mapstructure:"key,omitempty"`
	DocCount int32        `mapstructure:"doc_count,omitempty"`
}

type ResOutcomes struct {
	Outcomes    *ResOutcomes    `mapstructure:"outcomes,omitempty"`
	Stats       *ResStats       `mapstructure:"stats,omitempty"`
	AvgOutcomes *ResAvgOutcomes `mapstructure:"avg_outcomes,omitempty"`
	DocCount    int32           `mapstructure:"doc_count,omitempty"`
	Buckets     []ResBuckets    `mapstructure:"buckets,omitempty"`
}

type ResMoments struct {
	Buckets []ResBuckets `mapstructure:"buckets,omitempty"`
}

type ResAgents struct {
	Buckets []ResBuckets `mapstructure:"buckets,omitempty"`
}

type ResTimeRanges struct {
	Buckets []ResTimeRangeBucket `mapstructure:"buckets,omitempty"`
}

type ResTimeRangeBucket struct {
	Agents      *ResAgents `json:"agents,omitempty" mapstructure:"agents,omitempty"`
	KeyAsString string     `mapstructure:"key_as_string,omitempty"`
	DocCount    int32      `mapstructure:"doc_count,omitempty"`
	DDX         *ResDDX    `mapstructure:"ddx,omitempty"`
}

type ResAggregations struct {
	Moments    *ResMoments    `json:"moments,omitempty" mapstructure:"moments,omitempty"`
	Agents     *ResAgents     `json:"agents,omitempty" mapstructure:"agents,omitempty"`
	TimeRanges *ResTimeRanges `json:"time_ranges,omitempty" mapstructure:"time_ranges,omitempty"`
}

type Response struct {
	Aggregations *ResAggregations `mapstructure:"aggregations,omitempty"`
}

func (a AnalyticsServiceImpl) getMomentBehaviorMapping(
	ctx context.Context,
	parent string,
	scorecardTemplateNames []string,
) (map[*momentpb.Moment]*behaviorpb.Behavior, error) {
	behaviorNames := make(map[string]bool)
	for _, name := range scorecardTemplateNames {
		scorecardTemplateResp, err := a.coachingClientRegistry.Client(ctx).GetScorecardTemplate(ctx, &coachingpb.GetScorecardTemplateRequest{
			Name: name,
		})
		if err != nil {
			return nil, err
		}
		templateJSON, err := scorecardTemplateResp.ScorecardTemplate.Template.MarshalJSON()
		if err != nil {
			a.logger.Errorf(ctx, "unparsable JSON scorecard template given: %v", err)
			return nil, status.Errorf(codes.InvalidArgument, "not able to parse the given template, please review the structure.")
		}
		template, err := scoring.ParseScorecardTemplateStructure(string(templateJSON))
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "could not validate the given template: %v", err)
		}
		// Only filters the criteria with auto_qa enabled.
		for _, criteria := range template.GetAutoScorableItems() {
			if criteria.ShouldExcludeOutcomeInsights() {
				continue
			}
			for _, trigger := range *criteria.GetAutoQA().Triggers {
				if trigger.Type == scoring.TriggerTypeBehavior {
					behaviorNames[trigger.ResourceName] = true
				}
			}
		}
	}

	momentBehaviorMapping := make(map[*momentpb.Moment]*behaviorpb.Behavior)
	retrieveBehaviorResp, err := a.behaviorClientRegistry.Client(ctx).RetrieveBehaviors(ctx, &behaviorpb.RetrieveBehaviorsRequest{
		Parent: parent,
	})
	if err != nil {
		return nil, status.Errorf(status.Code(err), "calling RetrieveBehaviors() for behavior with error %v", err)
	}
	for _, behavior := range retrieveBehaviorResp.Behaviors {
		if _, exist := behaviorNames[behavior.Name]; exist {
			if behavior.Moment != nil {
				_, err = behaviorpb.ParseBehaviorName(behavior.Name)
				if err != nil {
					return nil, status.Errorf(status.Code(err), "failed to parse the behavior name (%s) with error %v", behavior.Name, err)
				}
				behavior.Moment.DisplayName = behavior.DisplayName
				momentBehaviorMapping[behavior.Moment] = behavior
			}
		}
	}
	return momentBehaviorMapping, nil
}

func getMomentIDBehaviorMapping(
	momentBehaviorMapping map[*momentpb.Moment]*behaviorpb.Behavior,
) (map[string]*behaviorpb.Behavior, error) {
	momentIDBehaviorMapping := make(map[string]*behaviorpb.Behavior)
	for moment, behavior := range momentBehaviorMapping {
		momentName, err := momentpb.ParseMomentName(moment.Name)
		if err != nil {
			return nil, status.Errorf(status.Code(err), "failed to parse the moment name (%s) with error %v", moment.Name, err)
		}
		momentIDBehaviorMapping[momentName.MomentID] = behavior
		// Also index by behavior ID to support drill-down requests. When the frontend
		// echoes back the behavior-scoped synthetic moment name (see behaviorScopedMomentName),
		// the moment ID in that name is the behavior UUID, so we need it resolvable here.
		behaviorName, err := behaviorpb.ParseBehaviorName(behavior.Name)
		if err != nil {
			return nil, status.Errorf(status.Code(err), "failed to parse the behavior name (%s) with error %v", behavior.Name, err)
		}
		momentIDBehaviorMapping[behaviorName.BehaviorID] = behavior
	}
	return momentIDBehaviorMapping, nil
}

// behaviorScopedMomentName builds a moment resource name with the behavior UUID substituted for
// the moment UUID. When multiple behaviors share the same underlying moment, this creates a
// unique per-behavior name in the response so the frontend can distinguish them.
// The resulting name is syntactically valid as a moment resource name. On drill-down requests
// the frontend echoes this name back unchanged; the backend resolves it via behavior ID lookup
// in getMomentIDBehaviorMapping.
func behaviorScopedMomentName(customerID, profileID, behaviorID string) string {
	return momentpb.MomentName{
		CustomerID: customerID,
		ProfileID:  profileID,
		MomentID:   behaviorID,
	}.String()
}

func getOutcomeMomentInRequest(req *analyticspb.RetrieveConversationOutcomeStatsRequest) (*momentpb.Moment, error) {
	if len(req.OutcomeMoments) != 1 {
		return nil, status.Error(codes.InvalidArgument, "must provide exactly one outcome moment")
	}
	if req.OutcomeMoments[0].Type != momentpb.Moment_CONVERSATION_OUTCOME &&
		(req.OutcomeMoments[0].Type != momentpb.Moment_CONVERSATION_METADATA || req.OutcomeMoments[0].DetailedType != momentpb.Moment_DETAILED_TYPE_METADATA_OUTCOME) {
		return nil, status.Error(codes.InvalidArgument, "the outcome moment must be CONVERSATION_OUTCOME or CONVERSATION_METADATA")
	}
	if req.OutcomeMoments[0].Type == momentpb.Moment_CONVERSATION_METADATA && req.OutcomeMoments[0].MomentData.GetConversationMetadataMomentData() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "there is no ConversationMetadataMomentData in the outcome metadata moment payload")
	}
	if req.OutcomeMoments[0].Type == momentpb.Moment_CONVERSATION_OUTCOME && req.OutcomeMoments[0].MomentData.GetConversationOutcomeMomentData() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "there is no ConversationOutcomeMomentData in the conversation outcome moment payload")
	}
	return req.OutcomeMoments[0], nil
}

type numberValueBounds struct {
	from            float64
	fromIsExclusive bool
	to              float64
	toIsInclusive   bool
}

const (
	conversationOutcomeNumberValueField                = "moment_annotations.payload.conversationOutcome.numberValue"
	conversationOutcomeNumberValueExplicitMappingField = "moment_annotations.payload_with_explicit_mapping.outcome_number_value"
	metadataNumberValueField                           = "moment_annotations.payload.conversationMetadataPayload.values.numberValue"
	metadataNumberValueExplicitMappingField            = "moment_annotations.payload_with_explicit_mapping.metadata_number_value"
)

// getMomentAnnotationNumberValueField picks the ES numberValue field matching the outcome
// moment type: outcome annotations carry conversationOutcome.numberValue, metadata outcome
// annotations carry conversationMetadataPayload.values.numberValue.
func getMomentAnnotationNumberValueField(outcomeMoment *momentpb.Moment, explicitMapping bool) string {
	if outcomeMoment.Type == momentpb.Moment_CONVERSATION_OUTCOME {
		if explicitMapping {
			return conversationOutcomeNumberValueExplicitMappingField
		}
		return conversationOutcomeNumberValueField
	}
	if explicitMapping {
		return metadataNumberValueExplicitMappingField
	}
	return metadataNumberValueField
}

// getOutcomeNumberValueBounds returns the range of numberValues the outcome stats should
// aggregate over, or nil when no range is declared. Values outside the range (e.g. the -1
// the CSAT predictor writes when it cannot score) are unscored sentinels, not scores.
// CSAT-subtype outcomes without declared bins fall back to the predictor's own [0, 10]
// clamp (csat_prediction.py: max(0, min(10, score))). INSI-4673.
func getOutcomeNumberValueBounds(outcomeMoment *momentpb.Moment) *numberValueBounds {
	var numericBins []*metadatapb.NumericBin
	if outcomeMoment.Type == momentpb.Moment_CONVERSATION_OUTCOME {
		numericBins = outcomeMoment.MomentData.GetConversationOutcomeMomentData().GetNumericBins()
	} else {
		numericBins = outcomeMoment.MomentData.GetConversationMetadataMomentData().GetNumericBins()
	}
	if len(numericBins) == 0 {
		if outcomeMoment.Type != momentpb.Moment_CONVERSATION_OUTCOME {
			return nil
		}
		outcomeMomentData := outcomeMoment.MomentData.GetConversationOutcomeMomentData()
		if outcomeMomentData.GetOutcomeSubtype() != enumspb.OutcomeSubtype_OUTCOME_SUBTYPE_CSAT ||
			outcomeMomentData.GetOutcomeValueType() != momentpb.ConversationOutcomeMomentData_OUTCOME_VALUE_TYPE_NUMBER {
			return nil
		}
		return &numberValueBounds{from: 0, to: 10, toIsInclusive: true}
	}
	bounds := &numberValueBounds{
		from:            float64(numericBins[0].GetFromValue()),
		fromIsExclusive: numericBins[0].GetFromValueIsExclusive(),
		to:              float64(numericBins[0].GetToValue()),
		toIsInclusive:   numericBins[0].GetToValueIsInclusive(),
	}
	for _, numericBin := range numericBins[1:] {
		// The envelope is a union: a shared endpoint stays reachable whenever any bin
		// at that endpoint includes it, so inclusivity wins over exclusivity on ties.
		if from := float64(numericBin.GetFromValue()); from < bounds.from {
			bounds.from, bounds.fromIsExclusive = from, numericBin.GetFromValueIsExclusive()
		} else if from == bounds.from && !numericBin.GetFromValueIsExclusive() {
			bounds.fromIsExclusive = false
		}
		if to := float64(numericBin.GetToValue()); to > bounds.to {
			bounds.to, bounds.toIsInclusive = to, numericBin.GetToValueIsInclusive()
		} else if to == bounds.to && numericBin.GetToValueIsInclusive() {
			bounds.toIsInclusive = true
		}
	}
	return bounds
}

// getOutcomeNumberValueRangeFilter maps the outcome moment's value bounds onto the ES
// range filter for the numberValue field the stats aggregate on. Returns nil when the
// field is unrecognized or no bounds are declared.
func getOutcomeNumberValueRangeFilter(outcomeMoment *momentpb.Moment, numberValueField string) *elasticsearch.RangeFilter {
	bounds := getOutcomeNumberValueBounds(outcomeMoment)
	if bounds == nil {
		return nil
	}
	fieldRange := &elasticsearch.FloatRange{}
	if bounds.fromIsExclusive {
		fieldRange.GT = &bounds.from
	} else {
		fieldRange.GTE = &bounds.from
	}
	if bounds.toIsInclusive {
		fieldRange.LTE = &bounds.to
	} else {
		fieldRange.LT = &bounds.to
	}
	switch numberValueField {
	case conversationOutcomeNumberValueField:
		return &elasticsearch.RangeFilter{OutcomeNumberValue: fieldRange}
	case conversationOutcomeNumberValueExplicitMappingField:
		return &elasticsearch.RangeFilter{OutcomeNumberValueExplicitMapping: fieldRange}
	case metadataNumberValueField:
		return &elasticsearch.RangeFilter{MetadataNumberValue: fieldRange}
	case metadataNumberValueExplicitMappingField:
		return &elasticsearch.RangeFilter{MetadataNumberValueExplicitMapping: fieldRange}
	default:
		return nil
	}
}

// getOutcomeTemplateFilter filters the outcome stats aggs to annotations of the outcome
// moment template, restricting numberValue to the declared bounds when available so
// unscored sentinel values stay out of the averages.
func getOutcomeTemplateFilter(outcomeMomentID string, numberValueRange *elasticsearch.RangeFilter) *ReqFilter {
	templateTerms := &ReqFilter{
		Terms: &elasticsearch.NestedTermsFilter{
			MomentTemplateIDs: &[]string{outcomeMomentID},
		},
	}
	if numberValueRange == nil {
		return templateTerms
	}
	return &ReqFilter{
		Bool: &ReqBool{
			Filter: []*ReqFilter{
				templateTerms,
				{Range: numberValueRange},
			},
		},
	}
}

func shouldGroupByTime(req *analyticspb.RetrieveConversationOutcomeStatsRequest) bool {
	return len(req.GroupByAttributes) > 0 &&
		req.GroupByAttributes[0].GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE
}

func shouldGroupByAgent(req *analyticspb.RetrieveConversationOutcomeStatsRequest) bool {
	for _, groupBy := range req.GroupByAttributes {
		if groupBy.GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT_TIER || groupBy.GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP {
			return true
		}
	}
	return false
}

// nolint:gosec
func (a AnalyticsServiceImpl) RetrieveConversationOutcomeStats(
	ctx context.Context, req *analyticspb.RetrieveConversationOutcomeStatsRequest,
) (*analyticspb.RetrieveConversationOutcomeStatsResponse, error) {
	a.logger.Infof(ctx, "RetrieveConversationOutcomeStats RPC starts, req = %v", req)
	if req.FilterConversationByAttribute == nil {
		req.FilterConversationByAttribute = &analyticspb.Attribute{}
	}

	// Validates the parent.
	parent, err := customerpb.ParseCustomerOrProfileName(req.Parent)
	if err != nil {
		return nil, err
	}

	// Reading feature config only once.
	profileConfig, err := a.configClient.GetProfileConfig(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	// Read-recency instrumentation for ES hot-window sizing (PII-free; see read_recency_metrics.go).
	recordConversationReadRecency(ctx, req.GetFilterByTimeRange().GetStartTimestamp(), parent.CustomerID, parent.ProfileID, "retrieve_conversation_outcome_stats")

	// Parses the scorecard template audiences.
	if len(req.ScorecardTemplateNames) != 1 {
		return nil, status.Errorf(codes.InvalidArgument, "ScorecardTemplateNames is required")
	}

	// Get resource ACL based on the caller identity.
	req.FilterConversationByAttribute.Users, req.FilterConversationByAttribute.Groups, err = shared.ApplyResourceACL(ctx, a.configClient, parent.CustomerID, a.resourceACLHelperProvider.Get(), req.FilterConversationByAttribute.Users, req.FilterConversationByAttribute.Groups, false)
	if err != nil {
		return nil, err
	}

	if req.FilterConversationByAttribute == nil || (len(req.FilterConversationByAttribute.Users) == 0 && len(req.FilterConversationByAttribute.Groups) == 0) {
		scorecardTemplateName, err := coachingpb.ParseScorecardTemplateName(req.ScorecardTemplateNames[0])
		if err != nil {
			return nil, err
		}
		scorecardTemplate, err := qa.GetScorecardTemplateFromDB(
			a.appsDB.DB(ctx), scorecardTemplateName.CustomerID, scorecardTemplateName.ProfileID, scorecardTemplateName.ScorecardTemplateID, scorecardTemplateName.ScorecardTemplateRevisionID)
		if err != nil {
			return nil, err
		}
		audience, err := qa.GetTemplateAudience(scorecardTemplate)
		if err != nil {
			return nil, err
		}
		if audience != nil && (len(audience.Groups) > 0 || len(audience.Users) > 0) {
			userClient, err := a.userClientRegistry.CreateClient(parent.CustomerID, parent.ProfileID)
			if err != nil {
				return nil, err
			}
			templateAudienceUsers, err := qa.ListTemplateAudienceUsers(ctx, userClient, parent, scorecardTemplate)
			if err != nil {
				return nil, err
			}
			if req.FilterConversationByAttribute == nil {
				req.FilterConversationByAttribute = &analyticspb.Attribute{}
			}
			req.FilterConversationByAttribute.Users = append(req.FilterConversationByAttribute.Users, fn.MapValueToSlice[string, *userpb.User](templateAudienceUsers)...)
		}
	}

	if req.FilterConversationByAttribute != nil && (len(req.FilterConversationByAttribute.Groups) > 0 || req.FilterConversationByAttribute.ExcludeDeactivatedUsers) {
		requestGroups := req.FilterConversationByAttribute.Groups
		filterByAttribute, err := shared.MoveFiltersToUserFilter(
			ctx,
			a.userClientRegistry,
			a.internalUserServiceClientRegistry,
			parent.CustomerID,
			parent.ProfileID,
			req.FilterConversationByAttribute,
			false, // includeDirectGroupMembershipsOnly
			false, // listAgentOnly
		)
		if err != nil {
			return nil, err
		}
		req.FilterConversationByAttribute = filterByAttribute
		// Need keep the original group filter so that the response would only include these group stats.
		req.FilterConversationByAttribute.Groups = requestGroups
	}

	momentBehaviorMapping, err := a.getMomentBehaviorMapping(ctx, req.Parent, req.ScorecardTemplateNames)
	if err != nil {
		return nil, err
	}
	momentIDBehaviorMapping, err := getMomentIDBehaviorMapping(momentBehaviorMapping)
	if err != nil {
		return nil, err
	}

	switch {
	case len(req.GroupByAttributes) == 2 &&
		req.GroupByAttributes[0].GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT &&
		req.GroupByAttributes[1].GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT_ADHERENCE_TYPE:
		return a.computeBehaviorOutcomeStats(ctx, profileConfig, req, parent, momentBehaviorMapping)

	case len(req.GroupByAttributes) == 1 &&
		req.GroupByAttributes[0].GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT_TIER:
		return a.computeAgentTierAdherenceStats(ctx, profileConfig, req, parent, momentIDBehaviorMapping)

	case len(req.GroupByAttributes) == 1 &&
		req.GroupByAttributes[0].GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP:
		if len(req.BehaviorMoments) > 0 {
			return a.computeGroupAdherenceStats(ctx, profileConfig, req, parent, momentIDBehaviorMapping)
		}
		if len(req.OutcomeMoments) > 0 {
			return a.computeGroupOutcomeStats(ctx, profileConfig, req, parent)
		}
		return nil, status.Error(codes.InvalidArgument, "missing the outcome_moments or behavior_moments in the request.")

	case len(req.GroupByAttributes) == 2 &&
		req.GroupByAttributes[0].GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE &&
		req.GroupByAttributes[1].GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP:
		if len(req.BehaviorMoments) == 0 {
			return nil, status.Error(codes.InvalidArgument, "missing the behavior moment in the request.")
		}
		return a.computeGroupAdherenceStats(ctx, profileConfig, req, parent, momentIDBehaviorMapping)

	case len(req.GroupByAttributes) == 1 &&
		req.GroupByAttributes[0].GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE:
		if len(req.BehaviorMoments) == 0 {
			return nil, status.Error(codes.InvalidArgument, "missing the behavior moment in the request.")
		}
		return a.computeTimeRangeAdherenceStats(ctx, profileConfig, req, parent, momentIDBehaviorMapping)

	default:
		return nil, status.Error(codes.InvalidArgument, "the group_by_attributes pattern is not supported")
	}
}

func (a AnalyticsServiceImpl) computeBehaviorOutcomeStats(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	req *analyticspb.RetrieveConversationOutcomeStatsRequest,
	parent *customerpb.CustomerOrProfileName,
	momentBehaviorMapping map[*momentpb.Moment]*behaviorpb.Behavior,
) (*analyticspb.RetrieveConversationOutcomeStatsResponse, error) {
	outcomeMoment, err := getOutcomeMomentInRequest(req)
	if err != nil {
		return nil, err
	}

	// For each behavior, sends ES queries to aggregate the outcome stats.
	// Each behavior gets its own per-behavior mapping so that behaviors sharing the same
	// underlying moment are not collapsed; see getMomentIDBehaviorMapping for the dedup concern.
	numBehaviors := len(momentBehaviorMapping)
	outcomeStatsChan := make(chan *analyticspb.AttributeOutcomeStats, 3*numBehaviors)
	eg := errgroup.New(errgroup.WithLimit(maxConcurrentESQueries))
	for behaviorMoment, behavior := range momentBehaviorMapping {
		behaviorMoment, behavior := behaviorMoment, behavior
		eg.Go(func() error {
			momentName, err := momentpb.ParseMomentName(behaviorMoment.Name)
			if err != nil {
				return status.Errorf(status.Code(err), "failed to parse moment name (%s): %v", behaviorMoment.Name, err)
			}
			behaviorName, err := behaviorpb.ParseBehaviorName(behavior.Name)
			if err != nil {
				return status.Errorf(status.Code(err), "failed to parse behavior name (%s): %v", behavior.Name, err)
			}
			// Build a mapping scoped to this behavior. We index by both the real moment ID
			// (used for ES query construction) and the behavior ID (used when resolving
			// drill-down requests that echo back the synthetic moment name).
			perBehaviorMapping := map[string]*behaviorpb.Behavior{
				momentName.MomentID:     behavior,
				behaviorName.BehaviorID: behavior,
			}
			outcomeStats, err := a.queryBehaviorStats(ctx, profileConfig, req, parent, outcomeMoment, behaviorMoment, behaviorName.BehaviorID, perBehaviorMapping)
			if err != nil {
				return err
			}
			if len(outcomeStats) != 3 {
				return status.Errorf(codes.Internal, "should have the sdx/ddx/dnx stats for each behavior.")
			}
			if outcomeStats[0].ConversationCount > 0 {
				for _, outcomeStat := range outcomeStats {
					select {
					case outcomeStatsChan <- outcomeStat:
					default:
						return fmt.Errorf("outcomeStatsChan is full or blocked")
					}
				}
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	close(outcomeStatsChan)
	response := analyticspb.RetrieveConversationOutcomeStatsResponse{}
	for outcomeStats := range outcomeStatsChan {
		response.AttributeOutcomeStats = append(response.AttributeOutcomeStats, outcomeStats)
	}
	return &response, nil
}

func (a AnalyticsServiceImpl) computeAgentTierAdherenceStats(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	req *analyticspb.RetrieveConversationOutcomeStatsRequest,
	parent *customerpb.CustomerOrProfileName,
	momentIDBehaviorMapping map[string]*behaviorpb.Behavior,
) (*analyticspb.RetrieveConversationOutcomeStatsResponse, error) {
	if len(req.BehaviorMoments) != 1 {
		return nil, status.Error(codes.InvalidArgument, "must provide exactly one behavior moment when group by agents")
	}

	// Sends ES query for the agent stats.
	outcomeStatsMap, err := a.queryAdherenceStats(ctx, profileConfig, req, parent, req.BehaviorMoments[0], momentIDBehaviorMapping)
	if err != nil {
		return nil, err
	}

	// Aggregates the stats for each tier of agents.
	outcomeStats := outcomeStatsMap[agentAdherenceDefaultTime]
	agentTierStats, err := a.computeAgentTierStats(req.BehaviorMoments[0], outcomeStats)
	if err != nil {
		a.logger.Warnf(ctx, "Error in computing the agent tier stats: %s", err.Error())
		agentTierStats = []*analyticspb.AgentTierOutcomeStats{}
	}

	response := analyticspb.RetrieveConversationOutcomeStatsResponse{
		AgentTierOutcomeStats: agentTierStats,
	}
	return &response, nil
}

func (a AnalyticsServiceImpl) computeTimeRangeAdherenceStats(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	req *analyticspb.RetrieveConversationOutcomeStatsRequest,
	parent *customerpb.CustomerOrProfileName,
	momentIDBehaviorMapping map[string]*behaviorpb.Behavior,
) (*analyticspb.RetrieveConversationOutcomeStatsResponse, error) {
	if len(req.BehaviorMoments) != 1 {
		return nil, status.Error(codes.InvalidArgument, "must provide exactly one behavior moment when group by time")
	}

	// Sends ES query for the agent stats.
	outcomeStatsMap, err := a.queryAdherenceStats(ctx, profileConfig, req, parent, req.BehaviorMoments[0], momentIDBehaviorMapping)
	if err != nil {
		return nil, err
	}

	// Aggregates the stats for each tier of agents.
	attributeOutcomeStats := []*analyticspb.AttributeOutcomeStats{}
	orderedTimes := []string{}
	for timeStr := range outcomeStatsMap {
		orderedTimes = append(orderedTimes, timeStr)
	}
	sort.Slice(orderedTimes, func(i, j int) bool {
		return orderedTimes[i] < orderedTimes[j]
	})

	for _, timeStr := range orderedTimes {
		outcomeStatsArray := outcomeStatsMap[timeStr]
		apiTime, err := shared.ConvertStringToAPITimestamp(timeStr)
		if err != nil {
			return nil, err
		}
		attribute := &analyticspb.Attribute{
			TimeRanges: []*analyticspb.TimeRange{
				{
					StartTimestamp: apiTime,
				},
			},
		}
		outcomeStats := outcomeStatsArray[0]
		outcomeStats.Attribute = attribute
		attributeOutcomeStats = append(attributeOutcomeStats, outcomeStats)
	}

	response := analyticspb.RetrieveConversationOutcomeStatsResponse{
		AttributeOutcomeStats: attributeOutcomeStats,
	}
	return &response, nil
}

func (a AnalyticsServiceImpl) computeGroupAdherenceStats(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	req *analyticspb.RetrieveConversationOutcomeStatsRequest,
	parent *customerpb.CustomerOrProfileName,
	momentIDBehaviorMapping map[string]*behaviorpb.Behavior,
) (*analyticspb.RetrieveConversationOutcomeStatsResponse, error) {
	// Computes the agent adherence stats for each behaviors.
	numBehaviors := len(req.BehaviorMoments)
	type adherenceResult struct {
		moment         *momentpb.Moment
		adherenceStats map[string][]*analyticspb.AttributeOutcomeStats
	}
	resultChan := make(chan adherenceResult, numBehaviors)
	eg := errgroup.New(errgroup.WithLimit(maxConcurrentESQueries))
	for _, behaviorMoment := range req.BehaviorMoments {
		eg.Go(func() error {
			adherenceStats, err := a.queryAdherenceStats(ctx, profileConfig, req, parent, behaviorMoment, momentIDBehaviorMapping)
			if err != nil {
				return err
			}
			select {
			case resultChan <- adherenceResult{moment: behaviorMoment, adherenceStats: adherenceStats}:
			default:
				return fmt.Errorf("resultChan is full or blocked")
			}
			return nil
		})
	}

	// Lists the users for the groups.
	groupsToUsersMap, err := a.listUsersInGroups(ctx, parent, req.FilterConversationByAttribute.GetGroups())
	if err != nil {
		return nil, err
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	close(resultChan)

	// Collects the ES query results.
	behaviorMomentToUserAdherencesMap := make(map[*momentpb.Moment]map[string][]*analyticspb.AttributeOutcomeStats)
	for r := range resultChan {
		behaviorMomentToUserAdherencesMap[r.moment] = r.adherenceStats
	}

	// Aggregates the group stats.
	groupStats, err := aggregateGroupAdherenceStats(behaviorMomentToUserAdherencesMap, groupsToUsersMap)
	if err != nil {
		return nil, err
	}
	return &analyticspb.RetrieveConversationOutcomeStatsResponse{AttributeOutcomeStats: groupStats}, nil
}

func (a AnalyticsServiceImpl) computeGroupOutcomeStats(
	ctx context.Context, profileConfig *config.ProfileConfig, req *analyticspb.RetrieveConversationOutcomeStatsRequest, parent *customerpb.CustomerOrProfileName,
) (*analyticspb.RetrieveConversationOutcomeStatsResponse, error) {
	outcomeMoment, err := getOutcomeMomentInRequest(req)
	if err != nil {
		return nil, err
	}

	// Computes the outcome stats for each agent.
	outcomeErrorChan := make(chan error, 1)
	userIDToOutcomeStatsMap := make(map[string]*analyticspb.AttributeOutcomeStats)
	go utils.AvoidPanic(ctx, func() {
		outcomeStats, err := a.queryAgentOutcomeStats(ctx, profileConfig, req, parent, outcomeMoment)
		if err != nil {
			outcomeErrorChan <- err
			return
		}
		for _, stats := range outcomeStats {
			userIDToOutcomeStatsMap[stats.Attribute.Users[0].Name] = stats
		}
		outcomeErrorChan <- nil
	})()

	// Lists the users for the groups.
	groupsToUsersMap, err := a.listUsersInGroups(ctx, parent, req.FilterConversationByAttribute.GetGroups())
	if err != nil {
		return nil, err
	}

	err = <-outcomeErrorChan
	if err != nil {
		return nil, err
	}

	// Aggregates the group stats.
	groupStats := aggregateGroupOutcomeStats(userIDToOutcomeStatsMap, groupsToUsersMap)
	return &analyticspb.RetrieveConversationOutcomeStatsResponse{AttributeOutcomeStats: groupStats}, nil
}

func (a AnalyticsServiceImpl) listUsersInGroups(
	ctx context.Context, parent *customerpb.CustomerOrProfileName, requestGroups []*userpb.Group,
) (map[*userpb.Group][]string, error) {
	// Use the user propagated ctx to list groups.
	groups, err := shared.ListGroups(rpcauth.PropagateFromContext(ctx), a.userClientRegistry, a.internalUserServiceClientRegistry, parent.CustomerID, parent.ProfileID, requestGroups, true /* onlyFilteredGroups= */)
	if err != nil {
		return nil, err
	}
	groupsToUsersMap := make(map[*userpb.Group][]string)
	// TODO: make it more efficient
	for _, group := range groups {
		users, err := shared.ParseFiltersToUsers(ctx, a.userClientRegistry, a.internalUserServiceClientRegistry,
			parent.CustomerID,
			parent.ProfileID,
			[]*userpb.Group{group},
			false, // activeUserFilter
			false, // includeDirectGroupMembershipsOnly
			false, // listAgentOnly
		)
		if err != nil {
			return nil, err
		}
		userIDs := []string{}
		for _, user := range users {
			userName, err := userpb.ParseUserName(user.Name)
			if err != nil {
				return nil, err
			}
			userIDs = append(userIDs, userName.UserID)
		}
		if len(userIDs) > 0 {
			groupsToUsersMap[group] = userIDs
		}
	}
	return groupsToUsersMap, nil
}

func getHasMomentAnnotationChildQuery(momentID string, adherenceType int, momentIDBehaviorMapping map[string]*behaviorpb.Behavior) *elasticsearch.HasChild {
	if behavior, exist := momentIDBehaviorMapping[momentID]; exist {
		// Have verified the behavior name at first.
		behaviorName, _ := behaviorpb.ParseBehaviorName(behavior.Name)
		return &elasticsearch.HasChild{
			Type: "message",
			Query: elasticsearch.InnerMsgQuery{
				Bool: &elasticsearch.InnerMsgBool{
					Filter: &[]elasticsearch.MsgFilter{
						{
							Nested: &elasticsearch.Nested{
								Path: "moment_annotations",
								Query: &elasticsearch.NestedFilter{
									Bool: &elasticsearch.NestedFilterBool{
										Filter: &[]elasticsearch.NestedFilter{
											{
												Terms: &elasticsearch.NestedTermsFilter{
													BehaviorIDs: &[]string{behaviorName.BehaviorID},
												},
											},
											{
												Terms: &elasticsearch.NestedTermsFilter{
													MomentAdherenceTypes: &[]int{adherenceType},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
	}
	return &elasticsearch.HasChild{
		Type: "message",
		Query: elasticsearch.InnerMsgQuery{
			Bool: &elasticsearch.InnerMsgBool{
				Filter: &[]elasticsearch.MsgFilter{
					{
						Nested: &elasticsearch.Nested{
							Path: "moment_annotations",
							Query: &elasticsearch.NestedFilter{
								Bool: &elasticsearch.NestedFilterBool{
									Filter: &[]elasticsearch.NestedFilter{
										{
											Terms: &elasticsearch.NestedTermsFilter{
												MomentTemplateIDs: &[]string{momentID},
											},
										},
										{
											Terms: &elasticsearch.NestedTermsFilter{
												MomentAdherenceTypes: &[]int{adherenceType},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func getBehaviorDDXAndDNXFilters(behaviorMomentID string, momentIDBehaviorMapping map[string]*behaviorpb.Behavior) *ReqFilters {
	return &ReqFilters{
		Filters: []ReqFilters{
			{
				HasChild: getHasMomentAnnotationChildQuery(
					behaviorMomentID, int(enumspb.AdherenceType_ADHERENCE_TYPE_DID_DO_X), momentIDBehaviorMapping),
			},
			{
				Bool: &elasticsearch.ConvoBool{
					MustNot: &[]elasticsearch.ConvoFilter{
						{
							HasChild: getHasMomentAnnotationChildQuery(
								behaviorMomentID, int(enumspb.AdherenceType_ADHERENCE_TYPE_DID_DO_X), momentIDBehaviorMapping),
						},
					},
				},
			},
		},
	}
}

func getMetadataStringValuedBinaryOutcomeAggs(positiveValues []string, negativeValues []string, outcomeMomentID string) *ReqOutcomes {
	return &ReqOutcomes{
		Nested: &elasticsearch.Nested{
			Path: "moment_annotations",
		},
		Aggs: &ReqAggs{
			Outcomes: &ReqOutcomes{
				Filters: &ReqAnnotationFilters{
					Filters: []ReqAnnotationFilters{
						{
							Bool: &ReqBool{
								Filter: []*ReqFilter{
									{
										Terms: &elasticsearch.NestedTermsFilter{
											MetadataStringPayloadValues: &positiveValues,
										},
									},
									{
										Terms: &elasticsearch.NestedTermsFilter{
											MomentTemplateIDs: &[]string{outcomeMomentID},
										},
									},
								},
							},
						},
						{
							Bool: &ReqBool{
								Filter: []*ReqFilter{
									{
										Terms: &elasticsearch.NestedTermsFilter{
											MetadataStringPayloadValues: &negativeValues,
										},
									},
									{
										Terms: &elasticsearch.NestedTermsFilter{
											MomentTemplateIDs: &[]string{outcomeMomentID},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func getMetadataBooleanValuedBinaryOutcomeAggs(positiveBoolValue bool, outcomeMomentID string) *ReqOutcomes {
	return &ReqOutcomes{
		Nested: &elasticsearch.Nested{
			Path: "moment_annotations",
		},
		Aggs: &ReqAggs{
			Outcomes: &ReqOutcomes{
				Filters: &ReqAnnotationFilters{
					Filters: []ReqAnnotationFilters{
						{
							Bool: &ReqBool{
								Filter: []*ReqFilter{
									{
										Terms: &elasticsearch.NestedTermsFilter{
											MetadataBooleanPayloadValues: &[]bool{positiveBoolValue},
										},
									},
									{
										Terms: &elasticsearch.NestedTermsFilter{
											MomentTemplateIDs: &[]string{outcomeMomentID},
										},
									},
								},
							},
						},
						{
							Bool: &ReqBool{
								Filter: []*ReqFilter{
									{
										Terms: &elasticsearch.NestedTermsFilter{
											MetadataBooleanPayloadValues: &[]bool{!positiveBoolValue},
										},
									},
									{
										Terms: &elasticsearch.NestedTermsFilter{
											MomentTemplateIDs: &[]string{outcomeMomentID},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func getConversationStringValuedBinaryOutcomeAggs(positiveValues []string, negativeValues []string, outcomeMomentID string) *ReqOutcomes {
	return &ReqOutcomes{
		Nested: &elasticsearch.Nested{
			Path: "moment_annotations",
		},
		Aggs: &ReqAggs{
			Outcomes: &ReqOutcomes{
				Filters: &ReqAnnotationFilters{
					Filters: []ReqAnnotationFilters{
						{
							Bool: &ReqBool{
								Filter: []*ReqFilter{
									{
										Terms: &elasticsearch.NestedTermsFilter{
											OutcomeStringPayloadValues: &positiveValues,
										},
									},
									{
										Terms: &elasticsearch.NestedTermsFilter{
											MomentTemplateIDs: &[]string{outcomeMomentID},
										},
									},
								},
							},
						},
						{
							Bool: &ReqBool{
								Filter: []*ReqFilter{
									{
										Terms: &elasticsearch.NestedTermsFilter{
											OutcomeStringPayloadValues: &negativeValues,
										},
									},
									{
										Terms: &elasticsearch.NestedTermsFilter{
											MomentTemplateIDs: &[]string{outcomeMomentID},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func getMetadataNumericValuedBinaryOutcomeAggs(positiveNumericBins []*metadatapb.NumericBin, negativeNumericBins []*metadatapb.NumericBin, outcomeMomentID string) *ReqOutcomes {
	positiveShould := []elasticsearch.NestedFilter{}
	for _, numericBin := range positiveNumericBins {
		positiveShould = append(positiveShould, elasticsearch.ConvertNumericBinToESRange(numericBin, false))
	}
	negativeShould := []elasticsearch.NestedFilter{}
	for _, numericBin := range negativeNumericBins {
		negativeShould = append(negativeShould, elasticsearch.ConvertNumericBinToESRange(numericBin, false))
	}
	return &ReqOutcomes{
		Nested: &elasticsearch.Nested{
			Path: "moment_annotations",
		},
		Aggs: &ReqAggs{
			Outcomes: &ReqOutcomes{
				Filters: &ReqAnnotationFilters{
					Filters: []ReqAnnotationFilters{
						{
							Bool: &ReqBool{
								Filter: []*ReqFilter{
									{
										Terms: &elasticsearch.NestedTermsFilter{
											MomentTemplateIDs: &[]string{outcomeMomentID},
										},
									},
								},
								Should:             &positiveShould,
								MinimumShouldMatch: 1,
							},
						},
						{
							Bool: &ReqBool{
								Filter: []*ReqFilter{
									{
										Terms: &elasticsearch.NestedTermsFilter{
											MomentTemplateIDs: &[]string{outcomeMomentID},
										},
									},
								},
								Should:             &negativeShould,
								MinimumShouldMatch: 1,
							},
						},
					},
				},
			},
		},
	}
}

func getBinaryOutcomeAggs(outcomeMoment *momentpb.Moment) (*ReqOutcomes, error) {
	outcomeMomentName, err := momentpb.ParseMomentName(outcomeMoment.Name)
	if err != nil {
		return nil, err
	}

	var outcomeAggs *ReqOutcomes
	if outcomeMoment.Type == momentpb.Moment_CONVERSATION_OUTCOME {
		for _, outcomeValue := range outcomeMoment.MomentData.GetConversationOutcomeMomentData().GetOutcomeValues() {
			switch v := outcomeValue.GetValue().GetValue().(type) {
			case *momentpb.ConversationOutcomeValue_StringValue:
				positiveStringValues, negativeStringValues, err := getPositiveNegativeConversationOutcomeStringValues(outcomeMoment)
				if err != nil {
					return nil, err
				}
				outcomeAggs = getConversationStringValuedBinaryOutcomeAggs(positiveStringValues, negativeStringValues, outcomeMomentName.MomentID)
			case *momentpb.ConversationOutcomeValue_BooleanValue:
				positiveBoolValue, err := getPositiveConversationOutcomeBooleanValue(outcomeMoment)
				if err != nil {
					return nil, err
				}
				outcomeAggs = getConversationOutcomeBoolValuedBinaryOutcomeAggs(outcomeMomentName.MomentID, positiveBoolValue)
			default:
				return nil, status.Errorf(codes.InvalidArgument, "unsupported conversation outcome value type: %v", v)
			}
			break
		}
	} else {
		switch outcomeMoment.MomentData.GetConversationMetadataMomentData().Type {
		case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING:
			// Gets the annotation string values for both positive/negative outcomes.
			positiveStringValues, negativeStringValues, err := getPositiveNegativeMetadataOutcomeStringValues(outcomeMoment)
			if err != nil {
				return nil, err
			}
			outcomeAggs = getMetadataStringValuedBinaryOutcomeAggs(positiveStringValues, negativeStringValues, outcomeMomentName.MomentID)
		case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER:
			positiveNumericBins, negativeNumericBins, err := getPositiveNegativeOutcomeNumericValues(outcomeMoment)
			if err != nil {
				return nil, err
			}
			outcomeAggs = getMetadataNumericValuedBinaryOutcomeAggs(positiveNumericBins, negativeNumericBins, outcomeMomentName.MomentID)
		case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_BOOLEAN:
			positiveBoolValue, err := getPositiveNegativeMetadataOutcomeBooleanValues(outcomeMoment)
			if err != nil {
				return nil, err
			}
			outcomeAggs = getMetadataBooleanValuedBinaryOutcomeAggs(positiveBoolValue, outcomeMomentName.MomentID)
		}
	}
	return outcomeAggs, nil
}

func getConversationOutcomeBoolValuedBinaryOutcomeAggs(outcomeMomentID string, positiveBoolValue bool) *ReqOutcomes {
	return &ReqOutcomes{
		Nested: &elasticsearch.Nested{
			Path: "moment_annotations",
		},
		Aggs: &ReqAggs{
			Outcomes: &ReqOutcomes{
				Filters: &ReqAnnotationFilters{
					Filters: []ReqAnnotationFilters{
						{
							Bool: &ReqBool{
								Filter: []*ReqFilter{
									{
										Terms: &elasticsearch.NestedTermsFilter{
											OutcomeBooleanPayloadValues: &[]bool{positiveBoolValue},
										},
									},
									{
										Terms: &elasticsearch.NestedTermsFilter{
											MomentTemplateIDs: &[]string{outcomeMomentID},
										},
									},
								},
							},
						},
						{
							Bool: &ReqBool{
								Filter: []*ReqFilter{
									{
										Terms: &elasticsearch.NestedTermsFilter{
											OutcomeBooleanPayloadValues: &[]bool{!positiveBoolValue},
										},
									},
									{
										Terms: &elasticsearch.NestedTermsFilter{
											MomentTemplateIDs: &[]string{outcomeMomentID},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (a AnalyticsServiceImpl) getOutcomeStatsESQuery(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	request *analyticspb.RetrieveConversationOutcomeStatsRequest,
	parent *customerpb.CustomerOrProfileName,
	behaviorMoment *momentpb.Moment,
	momentIDBehaviorMapping map[string]*behaviorpb.Behavior,
) (*elasticsearch.OuterQuery, error) {
	req := proto.Clone(request).(*analyticspb.RetrieveConversationOutcomeStatsRequest)
	if behaviorMoment != nil {
		momentName, err := momentpb.ParseMomentName(behaviorMoment.Name)
		if err != nil {
			return nil, status.Errorf(status.Code(err), "failed to parse the moment name (%s) with error %v", momentName, err)
		}
		var behaviorMomentGroup *analyticspb.MomentGroup
		// Filters the conversations that have the SDX of the behavior.
		if behavior, exist := momentIDBehaviorMapping[momentName.MomentID]; exist {
			behaviorMomentGroup = &analyticspb.MomentGroup{
				Behaviors: []*behaviorpb.Behavior{behavior},
				AdherenceTypesForMoments: []*analyticspb.MomentGroup_MomentAdherenceTypeConfig{
					{
						AdherenceTypes: []enumspb.AdherenceType{enumspb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X},
					},
				},
			}
		} else {
			behaviorMomentGroup = &analyticspb.MomentGroup{
				Moments:   []*momentpb.Moment{behaviorMoment},
				Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
			}
		}
		if req.FilterConversationByAttribute == nil {
			req.FilterConversationByAttribute = &analyticspb.Attribute{}
		}
		req.FilterConversationByAttribute.MomentGroups = append(req.FilterConversationByAttribute.MomentGroups, behaviorMomentGroup)
	}

	// Prepares the data to assemble the ES Query phrase.
	durationBuckets := hardcodedConversationDurationBuckets
	momentNames, hasCompositeMoment := getInsightsCompositeMoments(req.FilterConversationByAttribute, nil, nil)
	var topicDetails map[string]elasticsearch.TopicDetails
	var err error
	if hasCompositeMoment {
		profile := shared.CustomerProfile{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}
		aggregateToLeaves, _, err := a.resolveAggregateMoments(ctx, profile, momentNames)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if len(aggregateToLeaves) > 0 {
			expandAggregateMomentsInAttribute(req.FilterConversationByAttribute, aggregateToLeaves)
			momentNames, _ = getInsightsCompositeMoments(req.FilterConversationByAttribute, nil, nil)
		}
		topicDetails, err = a.fetchTopicDetails(ctx, parent.String(), momentNames)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	esQueryConfig, err := a.getElasticsearchQueryConfig(ctx, profileConfig, nil, &analyticspb.Attribute{})
	if err != nil {
		return nil, err
	}
	query, err := getESQuery(
		req.FilterByTimeRange,
		req.Metadata,
		durationBuckets,
		req.FilterConversationByAttribute,
		nil,
		&analyticspb.Attribute{},
		nil, /*groupByAttributes*/
		topicDetails,
		a.appsDB.DB(ctx), esQueryConfig,
		false /*allowFilteringByConvoFieldsInMsg*/)
	if err != nil {
		return nil, err
	}
	return query, nil
}

func (a AnalyticsServiceImpl) queryBehaviorStats(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	req *analyticspb.RetrieveConversationOutcomeStatsRequest,
	parent *customerpb.CustomerOrProfileName,
	outcomeMoment *momentpb.Moment,
	behaviorMoment *momentpb.Moment,
	behaviorID string,
	momentIDBehaviorMapping map[string]*behaviorpb.Behavior,
) ([]*analyticspb.AttributeOutcomeStats, error) {
	binaryOutcome, err := isBinaryOutcome(outcomeMoment)
	if err != nil {
		return nil, err
	}
	if binaryOutcome {
		return a.queryBehaviorStatsWithBinaryOutcome(ctx, profileConfig, req, parent, outcomeMoment, behaviorMoment, behaviorID, momentIDBehaviorMapping)
	}
	return a.queryBehaviorStatsWithNumericOutcome(ctx, profileConfig, req, parent, outcomeMoment, behaviorMoment, behaviorID, momentIDBehaviorMapping)
}

func (a AnalyticsServiceImpl) queryBehaviorStatsWithNumericOutcome(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	req *analyticspb.RetrieveConversationOutcomeStatsRequest,
	parent *customerpb.CustomerOrProfileName,
	outcomeMoment *momentpb.Moment,
	behaviorMoment *momentpb.Moment,
	behaviorID string,
	momentIDBehaviorMapping map[string]*behaviorpb.Behavior,
) ([]*analyticspb.AttributeOutcomeStats, error) {
	// Assembles the ES query.
	momentName, err := momentpb.ParseMomentName(behaviorMoment.Name)
	if err != nil {
		return nil, err
	}
	// Encode the behavior UUID into the response moment name so the frontend can distinguish
	// multiple behaviors that share the same underlying moment. The resulting name is valid as
	// a moment resource name; on drill-down the frontend echoes it back and the backend
	// resolves it via behavior ID lookup (see getMomentIDBehaviorMapping).
	responseMoment := proto.Clone(behaviorMoment).(*momentpb.Moment)
	responseMoment.Name = behaviorScopedMomentName(parent.CustomerID, parent.ProfileID, behaviorID)
	outcomeMomentName, err := momentpb.ParseMomentName(outcomeMoment.Name)
	if err != nil {
		return nil, err
	}

	query, err := a.getOutcomeStatsESQuery(ctx, profileConfig, req, parent, behaviorMoment, momentIDBehaviorMapping)
	if err != nil {
		return nil, err
	}
	numberValueField := getMomentAnnotationNumberValueField(outcomeMoment, a.enableUsageOfExplicitMappingMomentAnnotationPayload)

	aggs := ReqAggs{
		Moments: &ReqMoments{
			Filters: getBehaviorDDXAndDNXFilters(momentName.MomentID, momentIDBehaviorMapping),
			Aggs: &ReqAggs{
				Outcomes: &ReqOutcomes{
					Nested: &elasticsearch.Nested{
						Path: "moment_annotations",
					},
					Aggs: &ReqAggs{
						Outcomes: &ReqOutcomes{
							Aggs: &ReqAggs{
								Stats: &ReqStats{
									ExtendedStats: &ReqExtendedStats{
										Field: numberValueField,
									},
								},
							},
							Filter: getOutcomeTemplateFilter(
								outcomeMomentName.MomentID,
								getOutcomeNumberValueRangeFilter(outcomeMoment, numberValueField)),
						},
					},
				},
			},
		},
	}
	size := int32(0)
	esRequest := Request{Query: query, Aggs: &aggs, Size: &size}

	// Sends the ES query.
	esResponse := Response{}
	err = elasticsearch.Query(
		ctx, a.esClient, a.logger,
		sharedes.GetIndexName(profileConfig.GetFeatures(), &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}),
		elasticsearch.EsModeCount, 0 /*=pageSize*/, false, esRequest, &esResponse, "RetrieveConversationOutcomeStats")
	if err != nil {
		return nil, err
	}

	// Parses the ES response.
	if len(esResponse.Aggregations.Moments.Buckets) != 2 {
		return nil, status.Errorf(
			codes.Internal,
			"the returned buckets have unexpected length of %v", len(esResponse.Aggregations.Moments.Buckets))
	}
	ddxCount := esResponse.Aggregations.Moments.Buckets[0].DocCount
	dnxCount := esResponse.Aggregations.Moments.Buckets[1].DocCount
	sdxCount := ddxCount + dnxCount
	ddxOutcomeCount := esResponse.Aggregations.Moments.Buckets[0].Outcomes.Outcomes.Stats.Count
	ddxOutcomeMean := esResponse.Aggregations.Moments.Buckets[0].Outcomes.Outcomes.Stats.Avg
	ddxOutcomeStd := esResponse.Aggregations.Moments.Buckets[0].Outcomes.Outcomes.Stats.Std
	ddxOutcomeMeanConfidenceIntervalStart, ddxOutcomeMeanConfidenceIntervalEnd, err := shared.ComputeTConfidenceInterval(
		ddxOutcomeCount, ddxOutcomeMean, ddxOutcomeStd)
	if err != nil {
		a.logger.Warnf(ctx, "Error in computing the confidence interval: %s", err.Error())
	}
	dnxOutcomeCount := esResponse.Aggregations.Moments.Buckets[1].Outcomes.Outcomes.Stats.Count
	dnxOutcomeMean := esResponse.Aggregations.Moments.Buckets[1].Outcomes.Outcomes.Stats.Avg
	dnxOutcomeStd := esResponse.Aggregations.Moments.Buckets[1].Outcomes.Outcomes.Stats.Std
	dnxOutcomeMeanConfidenceIntervalStart, dnxOutcomeMeanConfidenceIntervalEnd, err := shared.ComputeTConfidenceInterval(
		dnxOutcomeCount, dnxOutcomeMean, dnxOutcomeStd)
	if err != nil {
		a.logger.Warnf(ctx, "Error in computing the confidence interval: %s", err.Error())
	}
	sdxStats := analyticspb.AttributeOutcomeStats{
		Attribute: &analyticspb.Attribute{
			Moments: []*momentpb.Moment{
				responseMoment,
			},
			Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
		},
		ConversationCount: sdxCount,
		BehaviorAdherence: safeDivideFloat(ddxCount, sdxCount),
	}
	ddxStats := analyticspb.AttributeOutcomeStats{
		Attribute: &analyticspb.Attribute{
			Moments: []*momentpb.Moment{
				responseMoment,
			},
			Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_DO_X,
		},
		ConversationCount: ddxCount,
		OutcomeStats: &analyticspb.OutcomeStats{
			Count:                   ddxOutcomeCount,
			Mean:                    ddxOutcomeMean,
			ConfidenceIntervalStart: ddxOutcomeMeanConfidenceIntervalStart,
			ConfidenceIntervalEnd:   ddxOutcomeMeanConfidenceIntervalEnd,
		},
	}
	dnxStats := analyticspb.AttributeOutcomeStats{
		Attribute: &analyticspb.Attribute{
			Moments: []*momentpb.Moment{
				responseMoment,
			},
			Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_NOT_DO_X,
		},
		ConversationCount: dnxCount,
		OutcomeStats: &analyticspb.OutcomeStats{
			Count:                   dnxOutcomeCount,
			Mean:                    dnxOutcomeMean,
			ConfidenceIntervalStart: dnxOutcomeMeanConfidenceIntervalStart,
			ConfidenceIntervalEnd:   dnxOutcomeMeanConfidenceIntervalEnd,
		},
	}
	return []*analyticspb.AttributeOutcomeStats{&sdxStats, &ddxStats, &dnxStats}, nil
}

func (a AnalyticsServiceImpl) queryBehaviorStatsWithBinaryOutcome(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	req *analyticspb.RetrieveConversationOutcomeStatsRequest,
	parent *customerpb.CustomerOrProfileName,
	outcomeMoment *momentpb.Moment,
	behaviorMoment *momentpb.Moment,
	behaviorID string,
	momentIDBehaviorMapping map[string]*behaviorpb.Behavior,
) ([]*analyticspb.AttributeOutcomeStats, error) {
	momentName, err := momentpb.ParseMomentName(behaviorMoment.Name)
	if err != nil {
		return nil, err
	}
	// Encode the behavior UUID into the response moment name so the frontend can distinguish
	// multiple behaviors that share the same underlying moment. The resulting name is valid as
	// a moment resource name; on drill-down the frontend echoes it back and the backend
	// resolves it via behavior ID lookup (see getMomentIDBehaviorMapping).
	responseMoment := proto.Clone(behaviorMoment).(*momentpb.Moment)
	responseMoment.Name = behaviorScopedMomentName(parent.CustomerID, parent.ProfileID, behaviorID)

	outcomeAggs, err := getBinaryOutcomeAggs(outcomeMoment)
	if err != nil {
		return nil, err
	}

	// Assembles the ES query.
	query, err := a.getOutcomeStatsESQuery(ctx, profileConfig, req, parent, behaviorMoment, momentIDBehaviorMapping)
	if err != nil {
		return nil, err
	}
	aggs := ReqAggs{
		Moments: &ReqMoments{
			Filters: getBehaviorDDXAndDNXFilters(momentName.MomentID, momentIDBehaviorMapping),
			Aggs: &ReqAggs{
				Outcomes: outcomeAggs,
			},
		},
	}
	size := int32(0)
	esRequest := Request{Query: query, Aggs: &aggs, Size: &size}

	// Sends the ES query.
	esResponse := Response{}
	err = elasticsearch.Query(
		ctx, a.esClient, a.logger,
		sharedes.GetIndexName(profileConfig.GetFeatures(), &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}),
		elasticsearch.EsModeCount, 0 /*=pageSize*/, false, esRequest, &esResponse, "RetrieveConversationOutcomeStats")
	if err != nil {
		return nil, err
	}

	// Parses the ES response.
	if len(esResponse.Aggregations.Moments.Buckets) != 2 {
		return nil, status.Errorf(
			codes.Internal,
			"the returned buckets have unexpected length of %v", len(esResponse.Aggregations.Moments.Buckets))
	}
	ddxCount := esResponse.Aggregations.Moments.Buckets[0].DocCount
	dnxCount := esResponse.Aggregations.Moments.Buckets[1].DocCount
	sdxCount := ddxCount + dnxCount
	if len(esResponse.Aggregations.Moments.Buckets[0].Outcomes.Outcomes.Buckets) != 2 {
		return nil, status.Errorf(
			codes.Internal,
			"the returned outcome buckets for ddx group have unexpected length of %v",
			len(esResponse.Aggregations.Moments.Buckets[0].Outcomes.Outcomes.Buckets))
	}
	if len(esResponse.Aggregations.Moments.Buckets[1].Outcomes.Outcomes.Buckets) != 2 {
		return nil, status.Errorf(
			codes.Internal,
			"the returned outcome buckets for dnx group have unexpected length of %v",
			len(esResponse.Aggregations.Moments.Buckets[1].Outcomes.Outcomes.Buckets))
	}
	ddxPositiveOutcomeCount := esResponse.Aggregations.Moments.Buckets[0].Outcomes.Outcomes.Buckets[0].DocCount
	ddxNegativeOutcomeCount := esResponse.Aggregations.Moments.Buckets[0].Outcomes.Outcomes.Buckets[1].DocCount
	dnxPositiveOutcomeCount := esResponse.Aggregations.Moments.Buckets[1].Outcomes.Outcomes.Buckets[0].DocCount
	dnxNegativeOutcomeCount := esResponse.Aggregations.Moments.Buckets[1].Outcomes.Outcomes.Buckets[1].DocCount
	ddxOutcomeMeanConfidenceIntervalStart, ddxOutcomeMeanConfidenceIntervalEnd := shared.ComputeProportionConfidenceInterval(
		ddxPositiveOutcomeCount+ddxNegativeOutcomeCount, ddxPositiveOutcomeCount)
	dnxOutcomeMeanConfidenceIntervalStart, dnxOutcomeMeanConfidenceIntervalEnd := shared.ComputeProportionConfidenceInterval(
		dnxPositiveOutcomeCount+dnxNegativeOutcomeCount, dnxPositiveOutcomeCount)
	sdxStats := analyticspb.AttributeOutcomeStats{
		Attribute: &analyticspb.Attribute{
			Moments: []*momentpb.Moment{
				responseMoment,
			},
			Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
		},
		ConversationCount: sdxCount,
		BehaviorAdherence: safeDivideFloat(ddxCount, sdxCount),
	}
	ddxStats := analyticspb.AttributeOutcomeStats{
		Attribute: &analyticspb.Attribute{
			Moments: []*momentpb.Moment{
				responseMoment,
			},
			Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_DO_X,
		},
		ConversationCount: ddxCount,
		OutcomeStats: &analyticspb.OutcomeStats{
			Count:                   ddxPositiveOutcomeCount + ddxNegativeOutcomeCount,
			Mean:                    safeDivideFloat(ddxPositiveOutcomeCount, ddxPositiveOutcomeCount+ddxNegativeOutcomeCount),
			ConfidenceIntervalStart: ddxOutcomeMeanConfidenceIntervalStart,
			ConfidenceIntervalEnd:   ddxOutcomeMeanConfidenceIntervalEnd,
		},
	}
	dnxStats := analyticspb.AttributeOutcomeStats{
		Attribute: &analyticspb.Attribute{
			Moments: []*momentpb.Moment{
				responseMoment,
			},
			Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_NOT_DO_X,
		},
		ConversationCount: dnxCount,
		OutcomeStats: &analyticspb.OutcomeStats{
			Count:                   dnxPositiveOutcomeCount + dnxNegativeOutcomeCount,
			Mean:                    safeDivideFloat(dnxPositiveOutcomeCount, dnxPositiveOutcomeCount+dnxNegativeOutcomeCount),
			ConfidenceIntervalStart: dnxOutcomeMeanConfidenceIntervalStart,
			ConfidenceIntervalEnd:   dnxOutcomeMeanConfidenceIntervalEnd,
		},
	}
	return []*analyticspb.AttributeOutcomeStats{&sdxStats, &ddxStats, &dnxStats}, nil
}

func parseAgentAdherenceBuckets(agents *ResAgents, behaviorMoment *momentpb.Moment) []*analyticspb.AttributeOutcomeStats {
	agentStats := []*analyticspb.AttributeOutcomeStats{}
	for _, agent := range agents.Buckets {
		sdxCount := agent.DocCount
		ddxCount := agent.DDX.DocCount
		agentStats = append(agentStats, &analyticspb.AttributeOutcomeStats{
			Attribute: &analyticspb.Attribute{
				Users: []*userpb.User{
					{
						Name: agent.Key,
					},
				},
				MomentGroups: []*analyticspb.MomentGroup{
					{
						Moments: []*momentpb.Moment{
							behaviorMoment,
						},
					},
				},
			},
			ConversationCount: sdxCount,
			BehaviorAdherence: safeDivideFloat(ddxCount, sdxCount),
		})
	}
	return agentStats
}

func (a AnalyticsServiceImpl) queryAdherenceStats(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	req *analyticspb.RetrieveConversationOutcomeStatsRequest,
	parent *customerpb.CustomerOrProfileName,
	behaviorMoment *momentpb.Moment,
	momentIDBehaviorMapping map[string]*behaviorpb.Behavior,
) (map[string][]*analyticspb.AttributeOutcomeStats, error) {
	behaviorMomentName, err := momentpb.ParseMomentName(behaviorMoment.Name)
	if err != nil {
		return nil, err
	}

	// Assembles the ES request.
	query, err := a.getOutcomeStatsESQuery(ctx, profileConfig, req, parent, behaviorMoment, momentIDBehaviorMapping)
	if err != nil {
		return nil, err
	}
	aggs := &ReqAggs{
		DDX: &ReqDDX{
			Filter: &ReqFilter{
				HasChild: getHasMomentAnnotationChildQuery(
					behaviorMomentName.MomentID, int(enumspb.AdherenceType_ADHERENCE_TYPE_DID_DO_X), momentIDBehaviorMapping),
			},
		},
	}
	if !shouldGroupByAgent(req) && !shouldGroupByTime(req) {
		return nil, status.Error(codes.Internal, "must provide the groupBy type.")
	}
	if shouldGroupByAgent(req) {
		agentAggs := &ReqAgents{
			Terms: &ReqTerms{
				Field: "agent_user_id.keyword",
				Size:  maxAgentCount,
			},
			Aggs: aggs,
		}
		aggs = &ReqAggs{
			Agents: agentAggs,
		}
	}
	if shouldGroupByTime(req) {
		timeZone, err := shared.ParseTimeZone(req.Metadata)
		if err != nil {
			return nil, err
		}
		frequency, err := shared.ParseFrequency(&req.Frequency)
		if err != nil {
			return nil, err
		}
		aggs = &ReqAggs{
			TimeRanges: &ReqTimeRanges{
				DateHistogram: &elasticsearch.ReqDateHistogram{
					Field:            "conversation_started_at",
					TimeZone:         timeZone,
					CalendarInterval: frequency,
				},
				Aggs: aggs,
			},
		}
	}

	size := int32(0)
	esRequest := Request{Query: query, Aggs: aggs, Size: &size}

	// Sends the ES query.
	esResponse := Response{}
	err = elasticsearch.Query(
		ctx, a.esClient, a.logger,
		sharedes.GetIndexName(profileConfig.GetFeatures(), &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}),
		elasticsearch.EsModeCount, 0 /*=pageSize*/, false, esRequest, &esResponse, "RetrieveConversationOutcomeStats")
	if err != nil {
		return nil, err
	}

	// Parses the ES response.
	result := make(map[string][]*analyticspb.AttributeOutcomeStats)
	if shouldGroupByTime(req) {
		for _, timeRangeBucket := range esResponse.Aggregations.TimeRanges.Buckets {
			if shouldGroupByAgent(req) {
				result[timeRangeBucket.KeyAsString] = parseAgentAdherenceBuckets(timeRangeBucket.Agents, behaviorMoment)
			} else {
				result[timeRangeBucket.KeyAsString] = []*analyticspb.AttributeOutcomeStats{
					{
						ConversationCount: timeRangeBucket.DocCount,
						BehaviorAdherence: safeDivideFloat(timeRangeBucket.DDX.DocCount, timeRangeBucket.DocCount),
					},
				}
			}
		}
	} else {
		result[agentAdherenceDefaultTime] = parseAgentAdherenceBuckets(esResponse.Aggregations.Agents, behaviorMoment)
	}
	return result, nil
}

func (a AnalyticsServiceImpl) queryAgentOutcomeStats(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	req *analyticspb.RetrieveConversationOutcomeStatsRequest,
	parent *customerpb.CustomerOrProfileName,
	outcomeMoment *momentpb.Moment,
) ([]*analyticspb.AttributeOutcomeStats, error) {
	binaryOutcome, err := isBinaryOutcome(outcomeMoment)
	if err != nil {
		return nil, err
	}
	if binaryOutcome {
		return a.queryAgentStatsWithBinaryOutcome(ctx, profileConfig, req, parent, outcomeMoment)
	}
	return a.queryAgentStatsWithNumericOutcome(ctx, profileConfig, req, parent, outcomeMoment)
}

func (a AnalyticsServiceImpl) queryAgentStatsWithNumericOutcome(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	req *analyticspb.RetrieveConversationOutcomeStatsRequest,
	parent *customerpb.CustomerOrProfileName,
	outcomeMoment *momentpb.Moment,
) ([]*analyticspb.AttributeOutcomeStats, error) {
	outcomeMomentName, err := momentpb.ParseMomentName(outcomeMoment.Name)
	if err != nil {
		return nil, err
	}

	// Assembles the ES request.
	query, err := a.getOutcomeStatsESQuery(ctx, profileConfig, req, parent, nil, nil)
	if err != nil {
		return nil, err
	}
	momentAnnotationPayloadField := getMomentAnnotationNumberValueField(outcomeMoment, a.enableUsageOfExplicitMappingMomentAnnotationPayload)
	aggs := ReqAggs{
		Agents: &ReqAgents{
			Terms: &ReqTerms{
				Field: "agent_user_id.keyword",
				Size:  maxAgentCount,
			},
			Aggs: &ReqAggs{
				Outcomes: &ReqOutcomes{
					Nested: &elasticsearch.Nested{
						Path: "moment_annotations",
					},
					Aggs: &ReqAggs{
						Outcomes: &ReqOutcomes{
							Aggs: &ReqAggs{
								AvgOutcomes: &ReqAvgOutcomes{
									Avg: &ReqAvg{
										Field: momentAnnotationPayloadField,
									},
								},
							},
							Filter: getOutcomeTemplateFilter(
								outcomeMomentName.MomentID,
								getOutcomeNumberValueRangeFilter(outcomeMoment, momentAnnotationPayloadField)),
						},
					},
				},
			},
		},
	}
	size := int32(0)
	esRequest := Request{Query: query, Aggs: &aggs, Size: &size}

	// Sends the ES query.
	esResponse := Response{}
	err = elasticsearch.Query(
		ctx, a.esClient, a.logger,
		sharedes.GetIndexName(profileConfig.GetFeatures(), &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}),
		elasticsearch.EsModeCount, 0 /*=pageSize*/, false, esRequest, &esResponse, "RetrieveConversationOutcomeStats")
	if err != nil {
		return nil, err
	}

	// Parses the ES response.
	agentStats := []*analyticspb.AttributeOutcomeStats{}
	for _, agent := range esResponse.Aggregations.Agents.Buckets {
		agentStats = append(agentStats, &analyticspb.AttributeOutcomeStats{
			Attribute: &analyticspb.Attribute{
				Users: []*userpb.User{
					{
						Name: agent.Key,
					},
				},
			},
			OutcomeStats: &analyticspb.OutcomeStats{
				Count: agent.Outcomes.Outcomes.DocCount,
				Mean:  agent.Outcomes.Outcomes.AvgOutcomes.Value,
			},
			ConversationCount: agent.DocCount,
		})
	}
	return agentStats, nil
}

func (a AnalyticsServiceImpl) queryAgentStatsWithBinaryOutcome(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	req *analyticspb.RetrieveConversationOutcomeStatsRequest,
	parent *customerpb.CustomerOrProfileName,
	outcomeMoment *momentpb.Moment,
) ([]*analyticspb.AttributeOutcomeStats, error) {
	outcomeAggs, err := getBinaryOutcomeAggs(outcomeMoment)
	if err != nil {
		return nil, err
	}

	// Assembles the ES request.
	// Gets the annotation string values for both positive/negative outcomes.
	query, err := a.getOutcomeStatsESQuery(ctx, profileConfig, req, parent, nil, nil)
	if err != nil {
		return nil, err
	}
	aggs := ReqAggs{
		Agents: &ReqAgents{
			Terms: &ReqTerms{
				Field: "agent_user_id.keyword",
				Size:  maxAgentCount,
			},
			Aggs: &ReqAggs{
				Outcomes: outcomeAggs,
			},
		},
	}
	size := int32(0)
	esRequest := Request{Query: query, Aggs: &aggs, Size: &size}

	// Sends the ES query.
	esResponse := Response{}
	err = elasticsearch.Query(
		ctx, a.esClient, a.logger,
		sharedes.GetIndexName(profileConfig.GetFeatures(), &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}),
		elasticsearch.EsModeCount, 0 /*=pageSize*/, false, esRequest, &esResponse, "RetrieveConversationOutcomeStats")
	if err != nil {
		return nil, err
	}

	// Parses the ES response.
	agentStats := []*analyticspb.AttributeOutcomeStats{}
	for _, agent := range esResponse.Aggregations.Agents.Buckets {
		if len(agent.Outcomes.Outcomes.Buckets) != 2 {
			return nil, status.Errorf(
				codes.Internal,
				"the returned outcome buckets have unexpected length of %v",
				len(agent.Outcomes.Outcomes.Buckets))
		}
		positiveOutcomeCount := agent.Outcomes.Outcomes.Buckets[0].DocCount
		negativeOutcomeCount := agent.Outcomes.Outcomes.Buckets[1].DocCount
		agentStats = append(agentStats, &analyticspb.AttributeOutcomeStats{
			Attribute: &analyticspb.Attribute{
				Users: []*userpb.User{
					{
						Name: agent.Key,
					},
				},
			},
			OutcomeStats: &analyticspb.OutcomeStats{
				Count: positiveOutcomeCount + negativeOutcomeCount,
				Mean:  safeDivideFloat(positiveOutcomeCount, positiveOutcomeCount+negativeOutcomeCount),
			},
			ConversationCount: agent.DocCount,
		})
	}
	return agentStats, nil
}

func (a AnalyticsServiceImpl) computeAgentTierStats(
	behaviorMoment *momentpb.Moment,
	agentStats []*analyticspb.AttributeOutcomeStats,
) ([]*analyticspb.AgentTierOutcomeStats, error) {
	numAgents := int32(len(agentStats))
	if numAgents == 0 {
		return nil, status.Errorf(codes.Internal,
			"there are zero agents, cannot split them into three tiers.")
	}

	// Sorts the agents by the adherence in descending order.
	sort.Slice(agentStats, func(i, j int) bool {
		return agentStats[i].BehaviorAdherence > agentStats[j].BehaviorAdherence
	})

	// Splits the agents into tiers. The top agent tier takes `topAgentPercentage` of all the conversations, etc.
	accumulatedConvoCount := []int32{}
	for i, agent := range agentStats {
		if i == 0 {
			accumulatedConvoCount = append(accumulatedConvoCount, agent.ConversationCount)
		} else {
			accumulatedConvoCount = append(accumulatedConvoCount, agent.ConversationCount+accumulatedConvoCount[i-1])
		}
	}
	totalConvoCount := accumulatedConvoCount[numAgents-1]
	idxTopAgents := sort.Search(int(numAgents), func(i int) bool {
		return accumulatedConvoCount[i] >= int32(math.Floor(float64(totalConvoCount)*topAgentPercentage))
	})
	idxAvgAgents := sort.Search(int(numAgents), func(i int) bool {
		return accumulatedConvoCount[i] >= int32(math.Floor(float64(totalConvoCount)*(topAgentPercentage+AvgAgentPercentage)))
	})
	return []*analyticspb.AgentTierOutcomeStats{
		assignTierAndAggregateAdherenceStats(behaviorMoment, agentStats, analyticspb.AgentTier_TOP_AGENTS, 0, int32(idxTopAgents+1)),
		assignTierAndAggregateAdherenceStats(behaviorMoment, agentStats, analyticspb.AgentTier_AVERAGE_AGENTS, int32(idxTopAgents+1), int32(idxAvgAgents+1)),
		assignTierAndAggregateAdherenceStats(behaviorMoment, agentStats, analyticspb.AgentTier_BOTTOM_AGENTS, int32(idxAvgAgents+1), numAgents),
	}, nil
}

func assignTierAndAggregateAdherenceStats(
	behaviorMoment *momentpb.Moment,
	agentStats []*analyticspb.AttributeOutcomeStats,
	agentTier analyticspb.AgentTier,
	from int32, to int32,
) *analyticspb.AgentTierOutcomeStats {
	numAgents := to - from
	var totalConvoCount int32
	var ddxConvoCount int32
	for i := from; i < to; i++ {
		agentStats[i].AgentTier = agentTier
		totalConvoCount += agentStats[i].ConversationCount
		ddxConvoCount += int32(float32(agentStats[i].ConversationCount) * agentStats[i].BehaviorAdherence)
	}
	return &analyticspb.AgentTierOutcomeStats{
		AgentTier:         agentTier,
		AgentCount:        numAgents,
		ConversationCount: totalConvoCount,
		BehaviorAdherence: safeDivideFloat(ddxConvoCount, totalConvoCount),
		BehaviorMoment:    behaviorMoment,
	}
}

func aggregateGroupAdherenceStats(
	behaviorMomentToUserAdherencesMap map[*momentpb.Moment]map[string][]*analyticspb.AttributeOutcomeStats,
	groupsToUsersMap map[*userpb.Group][]string,
) ([]*analyticspb.AttributeOutcomeStats, error) {
	groupStats := []*analyticspb.AttributeOutcomeStats{}
	for behaviorMoment, userAdherences := range behaviorMomentToUserAdherencesMap {
		orderedTimes := []string{}
		for timeStr := range userAdherences {
			orderedTimes = append(orderedTimes, timeStr)
		}
		sort.Slice(orderedTimes, func(i, j int) bool {
			return orderedTimes[i] < orderedTimes[j]
		})

		for _, timeStr := range orderedTimes {
			adherences := userAdherences[timeStr]
			var timeRange *analyticspb.TimeRange
			if timeStr != agentAdherenceDefaultTime {
				apiTime, err := shared.ConvertStringToAPITimestamp(timeStr)
				if err != nil {
					return nil, err
				}
				timeRange = &analyticspb.TimeRange{
					StartTimestamp: apiTime,
				}
			}

			// Computes the stats per time range per group
			userIDToAdherenceMap := make(map[string]*analyticspb.AttributeOutcomeStats)
			for _, adherence := range adherences {
				userIDToAdherenceMap[adherence.Attribute.Users[0].Name] = adherence
			}
			groups := getOrderedGroups(groupsToUsersMap)
			for _, group := range groups {
				userIDs := groupsToUsersMap[group]
				var groupUserCount int32
				var groupSdxConvoCount int32
				var groupDdxConvoCount float32
				for _, userID := range userIDs {
					stats, exist := userIDToAdherenceMap[userID]
					if !exist {
						continue
					}
					groupUserCount++
					groupSdxConvoCount += stats.ConversationCount
					groupDdxConvoCount += float32(stats.ConversationCount) * stats.BehaviorAdherence
				}
				if groupUserCount > 0 {
					attribute := &analyticspb.Attribute{
						Groups: []*userpb.Group{group},
						MomentGroups: []*analyticspb.MomentGroup{
							{
								Moments: []*momentpb.Moment{behaviorMoment},
							},
						},
					}
					if timeRange != nil {
						attribute.TimeRanges = []*analyticspb.TimeRange{timeRange}
					}
					groupStats = append(groupStats, &analyticspb.AttributeOutcomeStats{
						Attribute:         attribute,
						AgentCount:        groupUserCount,
						ConversationCount: groupSdxConvoCount,
						BehaviorAdherence: safeDivideFloat(groupDdxConvoCount, float32(groupSdxConvoCount)),
					})
				}
			}
		}

	}
	return groupStats, nil
}

func aggregateGroupOutcomeStats(
	userIDToOutcomeStatsMap map[string]*analyticspb.AttributeOutcomeStats,
	groupsToUsersMap map[*userpb.Group][]string,
) []*analyticspb.AttributeOutcomeStats {
	groupStats := []*analyticspb.AttributeOutcomeStats{}

	groups := getOrderedGroups(groupsToUsersMap)
	for _, group := range groups {
		userIDs := groupsToUsersMap[group]
		var userCount int32
		var convoCount int32
		var outcomeConvoCount int32
		var outcomeSum float32
		for _, userID := range userIDs {
			stats, exist := userIDToOutcomeStatsMap[userID]
			if !exist {
				continue
			}
			userCount++
			convoCount += stats.ConversationCount
			outcomeConvoCount += stats.OutcomeStats.Count
			outcomeSum += stats.OutcomeStats.Mean * float32(stats.OutcomeStats.Count)
		}
		if userCount > 0 {
			groupStats = append(groupStats, &analyticspb.AttributeOutcomeStats{
				Attribute: &analyticspb.Attribute{
					Groups: []*userpb.Group{group},
				},
				OutcomeStats: &analyticspb.OutcomeStats{
					Count: outcomeConvoCount,
					Mean:  safeDivideFloat(outcomeSum, float32(outcomeConvoCount)),
				},
				AgentCount:        userCount,
				ConversationCount: convoCount,
			})
		}
	}
	return groupStats
}

func getOrderedGroups(groupsMap map[*userpb.Group][]string) []*userpb.Group {
	groups := []*userpb.Group{}
	for group := range groupsMap {
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].GetDisplayName() < groups[j].GetDisplayName()
	})
	return groups
}

func getPositiveConversationOutcomeBooleanValue(outcomeMoment *momentpb.Moment) (bool, error) {
	outcomeMomentPayload := outcomeMoment.MomentData.GetConversationOutcomeMomentData()
	if outcomeMomentPayload.GetOutcomeValues() == nil || len(outcomeMomentPayload.GetOutcomeValues()) == 0 {
		return false, status.Errorf(codes.InvalidArgument, "missing outcome_values for the string typed conversation outcome moment.")
	}
	positiveBoolValue := true
	positiveBoolValueFound := false
	for _, outcomeValue := range outcomeMomentPayload.GetOutcomeValues() {
		if outcomeValue.GetBinaryOutcomeValueEnum() == enumspb.BinaryOutcomeValueEnum_POSITIVE {
			positiveBoolValue = outcomeValue.GetValue().GetBooleanValue()
			positiveBoolValueFound = true
			break
		}
	}
	if !positiveBoolValueFound {
		return false, status.Error(codes.InvalidArgument, "failed to find the positive binary value.")
	}
	return positiveBoolValue, nil
}

func getPositiveNegativeConversationOutcomeStringValues(outcomeMoment *momentpb.Moment) ([]string, []string, error) {
	outcomeMomentPayload := outcomeMoment.MomentData.GetConversationOutcomeMomentData()
	if outcomeMomentPayload.GetOutcomeValues() == nil || len(outcomeMomentPayload.GetOutcomeValues()) == 0 {
		return nil, nil, status.Errorf(codes.InvalidArgument, "missing outcome_values for the string typed conversation outcome moment.")
	}
	positiveStringValues := []string{}
	negativeStringValues := []string{}
	for _, outcomeValue := range outcomeMomentPayload.GetOutcomeValues() {
		stringValue := outcomeValue.GetValue().GetStringValue()
		if stringValue == "" {
			return nil, nil, status.Errorf(codes.InvalidArgument, "empty string value in the string typed outcome metadata moment")
		}
		switch outcomeValue.GetBinaryOutcomeValueEnum() {
		case enumspb.BinaryOutcomeValueEnum_POSITIVE:
			positiveStringValues = append(positiveStringValues, stringValue)
		case enumspb.BinaryOutcomeValueEnum_NEGATIVE:
			negativeStringValues = append(negativeStringValues, stringValue)
		default:
			continue
		}
	}
	if len(positiveStringValues) == 0 {
		return nil, nil, status.Errorf(codes.InvalidArgument, "no positive outcome values are defined in the conversation outcome payload.")
	}
	if len(negativeStringValues) == 0 {
		return nil, nil, status.Errorf(codes.InvalidArgument, "no negative outcome values are defined in the conversation outcome payload.")
	}
	return positiveStringValues, negativeStringValues, nil
}

func getPositiveNegativeMetadataOutcomeStringValues(outcomeMoment *momentpb.Moment) ([]string, []string, error) {
	outcomeMomentPayload := outcomeMoment.MomentData.GetConversationMetadataMomentData()
	if len(outcomeMomentPayload.AllowedValues) == 0 {
		return nil, nil, status.Errorf(codes.InvalidArgument, "missing allowed_values for the string typed outcome metadata moment.")
	}
	positiveStringValues := []string{}
	negativeStringValues := []string{}
	for _, allowedValue := range outcomeMoment.MomentData.GetConversationMetadataMomentData().AllowedValues {
		stringValue := allowedValue.GetValue().GetStringValue()
		if stringValue == "" {
			return nil, nil, status.Errorf(codes.InvalidArgument, "empty string value in the string typed outcome metadata moment")
		}
		switch allowedValue.GetBinaryOutcomeValueEnum() {
		case enumspb.BinaryOutcomeValueEnum_POSITIVE:
			positiveStringValues = append(positiveStringValues, stringValue)
		case enumspb.BinaryOutcomeValueEnum_NEGATIVE:
			negativeStringValues = append(negativeStringValues, stringValue)
		default:
			continue
		}
	}
	if len(positiveStringValues) == 0 {
		return nil, nil, status.Errorf(codes.InvalidArgument, "no positive outcome values are defined in the allowed_values.")
	}
	if len(negativeStringValues) == 0 {
		return nil, nil, status.Errorf(codes.InvalidArgument, "no negative outcome values are defined in the allowed_values.")
	}
	return positiveStringValues, negativeStringValues, nil
}

func getPositiveNegativeMetadataOutcomeBooleanValues(outcomeMoment *momentpb.Moment) (bool, error) {
	outcomeMomentPayload := outcomeMoment.MomentData.GetConversationMetadataMomentData()
	if len(outcomeMomentPayload.AllowedValues) == 0 {
		return false, status.Errorf(codes.InvalidArgument, "missing allowed_values for the boolean typed outcome metadata moment.")
	}
	hasPositive := false
	hasNegative := false
	var positiveBoolValue bool
	for _, allowedValue := range outcomeMoment.MomentData.GetConversationMetadataMomentData().AllowedValues {
		boolValue := allowedValue.GetValue().GetBooleanValue()
		switch allowedValue.GetBinaryOutcomeValueEnum() {
		case enumspb.BinaryOutcomeValueEnum_POSITIVE:
			if hasPositive {
				return false, status.Errorf(codes.InvalidArgument, "multiple positive outcome values defined in the allowed_values.")
			}
			positiveBoolValue = boolValue
			hasPositive = true
		case enumspb.BinaryOutcomeValueEnum_NEGATIVE:
			hasNegative = true
		default:
			continue
		}
	}
	if !hasPositive {
		return false, status.Errorf(codes.InvalidArgument, "no positive outcome value is defined in the allowed_values.")
	}
	if !hasNegative {
		return false, status.Errorf(codes.InvalidArgument, "no negative outcome value is defined in the allowed_values.")
	}
	return positiveBoolValue, nil
}

func getPositiveNegativeOutcomeNumericValues(outcomeMoment *momentpb.Moment) ([]*metadatapb.NumericBin, []*metadatapb.NumericBin, error) {
	outcomeMomentPayload := outcomeMoment.MomentData.GetConversationMetadataMomentData()
	if len(outcomeMomentPayload.NumericBins) == 0 {
		return nil, nil, status.Errorf(codes.InvalidArgument, "missing numeric_bin for the numeric outcome metadata moment.")
	}
	positiveNumericBins := []*metadatapb.NumericBin{}
	negativeNumericBins := []*metadatapb.NumericBin{}
	for _, numericBin := range outcomeMoment.MomentData.GetConversationMetadataMomentData().NumericBins {
		switch numericBin.GetBinaryOutcomeValueEnum() {
		case enumspb.BinaryOutcomeValueEnum_POSITIVE:
			positiveNumericBins = append(positiveNumericBins, numericBin)
		case enumspb.BinaryOutcomeValueEnum_NEGATIVE:
			negativeNumericBins = append(negativeNumericBins, numericBin)
		default:
			continue
		}
	}
	if len(positiveNumericBins) == 0 {
		return nil, nil, status.Errorf(codes.InvalidArgument, "no positive outcome values are defined in the numeric_bins.")
	}
	if len(negativeNumericBins) == 0 {
		return nil, nil, status.Errorf(codes.InvalidArgument, "no negative outcome values are defined in the numeric_bins.")
	}
	return positiveNumericBins, negativeNumericBins, nil
}

func isBinaryOutcome(outcomeMoment *momentpb.Moment) (bool, error) {
	if outcomeMoment.Type == momentpb.Moment_CONVERSATION_OUTCOME {
		outcomeMomentPayload := outcomeMoment.MomentData.GetConversationOutcomeMomentData()
		switch outcomeMomentPayload.OutcomeValueType {
		case momentpb.ConversationOutcomeMomentData_OUTCOME_VALUE_TYPE_STRING:
			return true, nil
		case momentpb.ConversationOutcomeMomentData_OUTCOME_VALUE_TYPE_BOOLEAN:
			return true, nil
		case momentpb.ConversationOutcomeMomentData_OUTCOME_VALUE_TYPE_NUMBER:
			return false, nil
		default:
			return false, status.Errorf(codes.InvalidArgument, "unsupported type of the metadata moment: %s.", outcomeMomentPayload.OutcomeValueType)
		}
	}

	// When it's metadata outcome:
	// 1. respect the outcome_stats_type field
	// 1. string typed   --> binary outcome
	// 2. numeric typed  --> numeric outcome
	outcomeMomentPayload := outcomeMoment.MomentData.GetConversationMetadataMomentData()
	if outcomeMomentPayload.GetOutcomeStatsTypes() != nil && len(outcomeMomentPayload.GetOutcomeStatsTypes()) > 0 {
		if len(outcomeMomentPayload.GetOutcomeStatsTypes()) != 1 {
			return false, status.Errorf(codes.InvalidArgument, "only supports the single outcome_stats_type now: %s.", outcomeMomentPayload.GetOutcomeStatsTypes())
		}
		switch outcomeMomentPayload.GetOutcomeStatsTypes()[0] {
		case enumspb.OutcomeStatsTypeEnum_CONVERSION_RATE:
			return true, nil
		case enumspb.OutcomeStatsTypeEnum_AVERAGE:
			return false, nil
		default:
			return false, status.Errorf(codes.InvalidArgument, "unsupported outcome_stats_type: %s.", outcomeMomentPayload.GetOutcomeStatsTypes()[0])
		}
	}
	switch outcomeMomentPayload.Type {
	case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING:
		return true, nil
	case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_BOOLEAN:
		return true, nil
	case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER:
		return false, nil
	default:
		return false, status.Errorf(codes.InvalidArgument, "unsupported type of the metadata moment: %s.", outcomeMomentPayload.Type)
	}
}
