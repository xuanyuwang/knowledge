package analyticsimpl

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/shared-go/util/envflag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/consts"
	logger "github.com/cresta/shared-go/framework/log"
)

var maxKeywordPhraseCount = envflag.Int("MAX_KEYWORD_PHRASE_COUNT", 1000, "Maximum number of keyword phrases allowed per keyword moment. Disabled if <= 0.")

func validateMoment(ctx context.Context, moment *momentpb.Moment) error {
	if moment.Type == momentpb.Moment_TYPE_UNSPECIFIED || moment.Taxonomy == "" || moment.DisplayName == "" {
		return status.Errorf(codes.InvalidArgument, "field type, taxonomy, display_name  can't be empty, but found %v", moment)
	}
	if moment.TaxonomyState != momentpb.Moment_STATE_ACTIVE {
		return status.Errorf(codes.InvalidArgument, "initial state of moment %q has to be ACTIVE, found %v", moment.Name, moment.TaxonomyState)
	}
	if moment.MomentSource != momentpb.MomentSource_MOMENT_SOURCE_CUSTOM {
		return status.Errorf(codes.InvalidArgument, "source of moment %q must be 'CUSTOM', but found %v", moment.Name, moment.MomentSource)
	}

	hasInsightsAppType := false
	for _, t := range moment.AppTypes {
		if t == momentpb.Moment_APP_TYPE_INSIGHTS {
			hasInsightsAppType = true
			break
		}
	}
	if !hasInsightsAppType {
		return status.Error(codes.InvalidArgument, "field app_types requires 'INSIGHTS'")
	}

	switch moment.Type {
	case momentpb.Moment_COMPOSITE:
		// TODO: Validates parent moment exists.
		// TODO: Reenable the check after the parent_moment_template_id are all backfilled.
		// if moment.ParentMomentTemplateId == "" {
		// 	return status.Error(codes.InvalidArgument, "parent_moment_template_id is not set for composite moment")
		// }
		return nil
	case momentpb.Moment_KEYWORD:
		if moment.MomentData == nil || moment.MomentData.GetKeywordMomentData() == nil {
			return status.Error(codes.InvalidArgument, "keyword_moment_data is not set for keyword moment")
		}
		if err := validateKeywordPhraseCount(moment); err != nil {
			return err
		}
	default:
		return status.Errorf(codes.InvalidArgument, "moment.type %v is not supported", moment.Type)
	}
	return nil
}

func validateKeywordPhraseCount(moment *momentpb.Moment) error {
	if *maxKeywordPhraseCount <= 0 {
		return nil
	}
	phraseCount := len(moment.GetMomentData().GetKeywordMomentData().GetKeywordExpression().GetOperands())
	if phraseCount > *maxKeywordPhraseCount {
		return status.Errorf(codes.InvalidArgument, "keyword moment has too many phrases (%d, max %d)", phraseCount, *maxKeywordPhraseCount)
	}
	return nil
}

func constructCompositeMomentData(prerequirementMoments []*momentpb.Moment, momentIncluded, momentExcluded *momentpb.Moment, clusters []*momentpb.Moment) *momentpb.MomentData {
	momentData := &momentpb.MomentData{
		Moment: &momentpb.MomentData_CompositeMomentData{
			CompositeMomentData: &momentpb.CompositeMomentData{
				Operator: momentpb.CompositeMomentData_AND,
			},
		},
	}

	if momentExcluded != nil {
		momentData.GetCompositeMomentData().Operands = append(momentData.GetCompositeMomentData().Operands, &momentpb.CompositeMomentData_CompositeMomentOperand{
			Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
				CompositeMoment: &momentpb.CompositeMomentData{
					Operator: momentpb.CompositeMomentData_NOT,
					Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
						{
							Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
								BaseMoment: momentExcluded.Name,
							},
						},
					},
				},
			},
		})
	}

	for _, pm := range prerequirementMoments {
		momentData.GetCompositeMomentData().Operands = append(momentData.GetCompositeMomentData().Operands, &momentpb.CompositeMomentData_CompositeMomentOperand{
			Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
				BaseMoment: pm.Name,
			},
		})
	}

	clustersOrIncluded := []string{}
	for _, c := range clusters {
		clustersOrIncluded = append(clustersOrIncluded, c.Name)
	}
	if momentIncluded != nil {
		clustersOrIncluded = append(clustersOrIncluded, momentIncluded.Name)
	}
	if len(clustersOrIncluded) > 0 {
		innerComp := &momentpb.CompositeMomentData{
			Operator: momentpb.CompositeMomentData_OR,
		}
		for _, name := range clustersOrIncluded {
			innerComp.Operands = append(innerComp.Operands, &momentpb.CompositeMomentData_CompositeMomentOperand{
				Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
					BaseMoment: name,
				},
			})
		}
		momentData.GetCompositeMomentData().Operands = append(momentData.GetCompositeMomentData().Operands, &momentpb.CompositeMomentData_CompositeMomentOperand{
			Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
				CompositeMoment: innerComp,
			},
		})
	}
	return momentData
}

// GetIncludedAndExcludedMomentsFromParentMoment gets the included and excluded moments from the moment data of the parent moment.
func GetIncludedAndExcludedMomentsFromParentMoment(ctx context.Context, parentMoment *momentpb.Moment, momentClient momentpb.MomentServiceClient, logger logger.Logger) ([]*momentpb.Moment, *momentpb.Moment, *momentpb.Moment) {
	if parentMoment == nil {
		return nil, nil, nil
	}
	if parentMoment.GetMomentData() == nil || parentMoment.GetMomentData().GetCompositeMomentData() == nil {
		return nil, nil, nil
	}

	// Get the included and excluded moment names from the parent moment. Included moment names include both included keywords and included clusters.
	var includedMomentNames, excludedMomentNames []string
	for _, op := range parentMoment.GetMomentData().GetCompositeMomentData().GetOperands() {
		if op.GetCompositeMoment() != nil && op.GetCompositeMoment().GetOperator() == momentpb.CompositeMomentData_NOT {
			for _, operand := range op.GetCompositeMoment().GetOperands() {
				excludedKeywordsMomentName := operand.GetBaseMoment()
				excludedMomentNames = append(excludedMomentNames, excludedKeywordsMomentName)
			}
		}
		if op.GetCompositeMoment() != nil && op.GetCompositeMoment().GetOperator() == momentpb.CompositeMomentData_OR {
			for _, operand := range op.GetCompositeMoment().GetOperands() {
				includedKeywordsMomentName := operand.GetBaseMoment()
				includedMomentNames = append(includedMomentNames, includedKeywordsMomentName)
			}
		}
	}

	var includedKeywordsMoment, excludedKeywordsMoment *momentpb.Moment
	var includedClusterMoments []*momentpb.Moment

	// Get the included and excluded moments from the moment names.
	if len(includedMomentNames) > 0 {
		resp, err := momentClient.BatchGetMoments(ctx, &momentpb.BatchGetMomentsRequest{
			Names: includedMomentNames,
		})
		if err != nil {
			logger.Errorf(ctx, "getting included moments %v: %v", includedMomentNames, err)
			return nil, nil, nil
		}
		for _, m := range resp.GetMoments() {
			switch m.Type {
			case momentpb.Moment_KEYWORD:
				includedKeywordsMoment = m
			case momentpb.Moment_TOPIC:
				includedClusterMoments = append(includedClusterMoments, m)
			}
		}
	}
	if len(excludedMomentNames) > 0 {
		resp, err := momentClient.BatchGetMoments(ctx, &momentpb.BatchGetMomentsRequest{
			Names: excludedMomentNames,
		})
		if err != nil {
			logger.Errorf(ctx, "getting excluded moments %v: %v", excludedMomentNames, err)
			return nil, nil, nil
		}
		for _, m := range resp.GetMoments() {
			if m.Type == momentpb.Moment_KEYWORD {
				excludedKeywordsMoment = m
			}
		}
	}

	return includedClusterMoments, includedKeywordsMoment, excludedKeywordsMoment
}

func mergeClusterMoments(clusterMoments1, clusterMoments2 []*momentpb.Moment) []*momentpb.Moment {
	if clusterMoments1 == nil {
		return clusterMoments2
	}
	if len(clusterMoments2) == 0 {
		return clusterMoments1
	}

	return append(clusterMoments1, clusterMoments2...)
}

func mergeKeywordMomentData(childMoment, parentMoment *momentpb.Moment, momentNamesToDeprecate *[]string) *momentpb.Moment {
	if childMoment == nil || childMoment.MomentData == nil || childMoment.MomentData.GetKeywordMomentData() == nil {
		return parentMoment
	}
	if parentMoment == nil || parentMoment.MomentData == nil || parentMoment.MomentData.GetKeywordMomentData() == nil {
		return childMoment
	}

	merged := proto.Clone(childMoment).(*momentpb.Moment)

	// Get the keyword expressions from both moments
	childExpr := childMoment.MomentData.GetKeywordMomentData().GetKeywordExpression()
	parentExpr := parentMoment.MomentData.GetKeywordMomentData().GetKeywordExpression()

	// Collect all operands from both expressions
	var allOperands []*momentpb.KeywordMomentData_Operand
	if childExpr != nil {
		allOperands = append(allOperands, childExpr.Operands...)
	}
	if parentExpr != nil {
		allOperands = append(allOperands, parentExpr.Operands...)
	}

	// Remove duplicates by using a map to track unique operands
	uniqueOperands := make([]*momentpb.KeywordMomentData_Operand, 0, len(allOperands))
	seen := make(map[string]bool)

	for _, operand := range allOperands {
		// Use proto.Marshal to get a unique string representation for comparison
		operandBytes, err := proto.Marshal(operand)
		if err != nil {
			continue
		}
		if !seen[string(operandBytes)] {
			seen[string(operandBytes)] = true
			uniqueOperands = append(uniqueOperands, operand)
		}
	}

	// Create a new expression that combines all unique operands. The operator of the 2 moments is the same at this point because:
	// 1. The operator for exclude keyword moments is always OR.
	// 2. The operator for included keyword moments is the same as the parent moment because we check for this at a previous step.
	mergedExpression := &momentpb.KeywordMomentData_KeywordExpression{
		Operator: childMoment.MomentData.GetKeywordMomentData().GetKeywordExpression().GetOperator(),
		Operands: uniqueOperands,
	}

	// Set the merged expression in the merged moment
	merged.MomentData.GetKeywordMomentData().KeywordExpression = mergedExpression

	// Add the parent moment name to the deprecation list
	if momentNamesToDeprecate != nil && parentMoment.Name != "" {
		*momentNamesToDeprecate = append(*momentNamesToDeprecate, parentMoment.Name)
	}

	return merged
}

func IsParentAndChildMomentDataForInheritanceCompatible(ctx context.Context, parentMoment, childMoment *momentpb.Moment, logger logger.Logger) bool {
	if parentMoment == nil || parentMoment.MomentData == nil || parentMoment.MomentData.GetKeywordMomentData() == nil || parentMoment.MomentData.GetKeywordMomentData().GetKeywordExpression() == nil ||
		childMoment == nil || childMoment.MomentData == nil || childMoment.MomentData.GetKeywordMomentData() == nil || childMoment.MomentData.GetKeywordMomentData().GetKeywordExpression() == nil {
		return false
	}
	if parentMoment.MomentData.GetKeywordMomentData().GetKeywordExpression().GetOperator() != childMoment.MomentData.GetKeywordMomentData().GetKeywordExpression().GetOperator() {
		logger.Errorf(ctx, "parent and child included keyword moment data are not compatible for inheritance due to different operators: parent %v, child %v", parentMoment, childMoment)
		return false
	}
	return true
}

type topicInfo struct {
	compositeMoment  *momentpb.Moment
	parentMomentName string
	// ML clusters and included keyword moment.
	innerBaseMomentNames []string
	// prerequirement moments.
	outerBaseMomentNames []string
}

type insightsTopic struct {
	apiInsightsTopic *analyticspb.InsightsTopic
	parentMoment     *momentpb.Moment
}

func (a AnalyticsServiceImpl) fetchInsightsTopics(ctx context.Context, parentName string, topicMomentNames []string, fetchParentMoment bool) ([]insightsTopic, error) {
	a.logger.Infof(ctx, "fetching insights topics %q", topicMomentNames)
	if len(topicMomentNames) == 0 {
		return nil, nil
	}
	req := &momentpb.BatchGetMomentsRequest{Parent: parentName, Names: topicMomentNames}

	resp, err := a.momentClientRegistry.Client(ctx).BatchGetMoments(ctx, req)
	if err != nil {
		return nil, status.Errorf(status.Code(err), "calling BatchGetMoments() for composite moments with request %v: %v", req, err)
	}
	// The order of resp.GetMoments() does not match topicMomentNames
	// Update so that the returned moments order matches the order of topicMomentNames.
	// Create a map for O(1) lookups
	originalMomentMap := make(map[string]*momentpb.Moment)
	for _, m := range resp.GetMoments() {
		originalMomentMap[m.Name] = m
	}

	// Process moments in the original order
	topicInfos := []topicInfo{}
	for _, name := range topicMomentNames {
		m, exists := originalMomentMap[name]
		if !exists || m.Type != momentpb.Moment_COMPOSITE {
			continue
		}

		ti := topicInfo{compositeMoment: m}
		if id := m.ParentMomentTemplateId; id != "" && fetchParentMoment {
			momentName, err := resolveMomentNameFromTemplateID(req.Parent, id)
			if err != nil {
				return nil, err
			}
			ti.parentMomentName = momentName
		}

		for _, op := range m.GetMomentData().GetCompositeMomentData().GetOperands() {
			// The outer base moments which can be keyword moment for excluded keywords
			// and topic moments for prev pattern.
			if bm := op.GetBaseMoment(); bm != "" {
				ti.outerBaseMomentNames = append(ti.outerBaseMomentNames, bm)
			}
			// The inner composite moment which contains the clusters and moment for included keywords.
			if innerComp := op.GetCompositeMoment(); innerComp != nil {
				for _, op := range innerComp.GetOperands() {
					if op.GetCompositeMoment() != nil {
						return nil, fmt.Errorf("extra nested composite moment is found in moment data %v", m.GetMomentData())
					}
					// Either ML clusters or included keyword moments, or exclude keyword moments when Operator is NOT.
					if bm := op.GetBaseMoment(); bm != "" {
						if innerComp.Operator == momentpb.CompositeMomentData_NOT {
							ti.outerBaseMomentNames = append(ti.outerBaseMomentNames, bm)
						} else {
							ti.innerBaseMomentNames = append(ti.innerBaseMomentNames, bm)
						}
					} else {
						return nil, fmt.Errorf("base_moment is not set in operands of inner composite moment: %v", op)
					}
				}
			}
		}
		topicInfos = append(topicInfos, ti)
	}

	childMoments := []string{}
	parentMomentNames := map[string]string{}
	for _, ti := range topicInfos {
		if len(ti.outerBaseMomentNames) > 0 {
			childMoments = append(childMoments, ti.outerBaseMomentNames...)
		}
		if len(ti.innerBaseMomentNames) > 0 {
			childMoments = append(childMoments, ti.innerBaseMomentNames...)
		}
		if name := ti.parentMomentName; name != "" {
			parentMomentNames[ti.compositeMoment.Name] = name
		}
	}

	momentNames := childMoments
	for _, v := range parentMomentNames {
		momentNames = append(momentNames, v)
	}

	momentMap := map[string]*momentpb.Moment{}
	if len(momentNames) > 0 {
		batchGetReq := &momentpb.BatchGetMomentsRequest{
			Parent: parentName,
			Names:  momentNames,
		}
		resp, err = a.momentClientRegistry.Client(ctx).BatchGetMoments(ctx, batchGetReq)
		if err != nil {
			return nil, status.Errorf(status.Code(err), "calling BatchGetMoments() for child moments with request %v: %v", batchGetReq, err)
		}

		notFoundMoment := map[string]bool{}
		for _, name := range momentNames {
			notFoundMoment[name] = true
		}
		for _, m := range resp.Moments {
			momentMap[m.Name] = m
			delete(notFoundMoment, m.Name)
		}

		if len(notFoundMoment) > 0 {
			return nil, fmt.Errorf("child moments %v are not found ", notFoundMoment)
		}
	}

	// Walk up the ParentMomentTemplateId chain so each topic inherits the
	// speaker-role filter from its nearest labeled ancestor, not just the
	// immediate parent. CMv2 folders are intermediate nodes without filter
	// labels, so the filter lives on a further ancestor (typically the root).
	// Batched per level; the message-window labels co-locate with it.
	for depth := 0; depth < maxInheritedFilterAncestorDepth; depth++ {
		advanced := false
		toFetch := []string{}
		queued := map[string]bool{}
		for i := range topicInfos {
			ti := &topicInfos[i] // mutate the slice element, not a copy
			if ti.parentMomentName == "" {
				continue
			}
			pm := momentMap[ti.parentMomentName]
			if momentHasSpeakerRoleFilterLabel(pm) {
				continue
			}
			if pm == nil || pm.ParentMomentTemplateId == "" {
				continue
			}
			ancestorName, err := resolveMomentNameFromTemplateID(parentName, pm.ParentMomentTemplateId)
			if err != nil {
				return nil, err
			}
			ti.parentMomentName = ancestorName
			advanced = true
			if _, ok := momentMap[ancestorName]; !ok && !queued[ancestorName] {
				queued[ancestorName] = true
				toFetch = append(toFetch, ancestorName)
			}
		}
		if !advanced {
			break
		}
		if len(toFetch) > 0 {
			ancestorResp, err := a.momentClientRegistry.Client(ctx).BatchGetMoments(ctx, &momentpb.BatchGetMomentsRequest{
				Parent: parentName,
				Names:  toFetch,
			})
			if err != nil {
				return nil, status.Errorf(status.Code(err), "calling BatchGetMoments() for ancestor moments %v: %v", toFetch, err)
			}
			returned := map[string]bool{}
			for _, m := range ancestorResp.GetMoments() {
				momentMap[m.Name] = m
				returned[m.Name] = true
			}
			// A missing ancestor would otherwise silently degrade to "no filter"
			// (the walk treats nil as exhausted), masking a broken tree. Surface it.
			missing := []string{}
			for _, name := range toFetch {
				if !returned[name] {
					missing = append(missing, name)
				}
			}
			if len(missing) > 0 {
				return nil, fmt.Errorf("ancestor moments %v not found while resolving inherited filter labels", missing)
			}
		}
	}

	ret := []insightsTopic{}
	for _, ti := range topicInfos {
		var (
			included              *momentpb.Moment
			excluded              *momentpb.Moment
			clusters              []*momentpb.Moment
			prerequirementMoments []*momentpb.Moment
		)
		for _, name := range ti.innerBaseMomentNames {
			if momentMap[name].Type == momentpb.Moment_KEYWORD {
				included = momentMap[name]
			} else {
				clusters = append(clusters, momentMap[name])
			}
		}
		for _, name := range ti.outerBaseMomentNames {
			if momentMap[name].Type == momentpb.Moment_KEYWORD {
				excluded = momentMap[name]
			}
			if momentMap[name].Type == momentpb.Moment_TOPIC {
				prerequirementMoments = append(prerequirementMoments, momentMap[name])
			}
		}
		insightsTopic := insightsTopic{
			apiInsightsTopic: &analyticspb.InsightsTopic{
				TopicMoment:           ti.compositeMoment,
				ExcludedKeywordMoment: excluded,
				IncludedKeywordMoment: included,
				ClusterMoments:        clusters,
				PrerequirementMoments: prerequirementMoments,
			},
		}
		if name := ti.parentMomentName; name != "" {
			insightsTopic.parentMoment = momentMap[name]
		}
		ret = append(ret, insightsTopic)
	}

	return ret, nil
}

func toAPIInsightsTopic(l []insightsTopic) []*analyticspb.InsightsTopic {
	ret := []*analyticspb.InsightsTopic{}
	for _, val := range l {
		ret = append(ret, val.apiInsightsTopic)
	}
	return ret
}

// Calls moment service to fetch the information of child moment for topics.
func (a AnalyticsServiceImpl) fetchTopicDetails(ctx context.Context, parentName string, topicMomentNames []string) (map[string]elasticsearch.TopicDetails, error) {
	insightsTopics, err := a.fetchInsightsTopics(ctx, parentName, topicMomentNames, true)
	if err != nil {
		a.logger.Errorf(
			ctx,
			"failed to fetch insights topics for topic details (parent=%q, topic_moment_count=%d, topic_moment_names=%v): %v",
			parentName,
			len(topicMomentNames),
			truncateMomentNamesForErrorLog(topicMomentNames),
			err,
		)
		return nil, err
	}

	ret := map[string]elasticsearch.TopicDetails{}
	for _, t := range insightsTopics {
		td, err := a.convertInsightsTopicToTopicDetails(ctx, t)
		if err != nil {
			topicMomentName := ""
			if t.apiInsightsTopic != nil && t.apiInsightsTopic.TopicMoment != nil {
				topicMomentName = t.apiInsightsTopic.TopicMoment.Name
			}
			a.logger.Errorf(
				ctx,
				"failed to convert insights topic to topic details (parent=%q, topic_moment_name=%q): %v",
				parentName,
				topicMomentName,
				err,
			)
			return nil, err
		}
		if td != nil {
			ret[t.apiInsightsTopic.TopicMoment.Name] = *td
		}
	}
	return ret, nil
}

func (a AnalyticsServiceImpl) convertInsightsTopicToTopicDetails(ctx context.Context, topic insightsTopic) (*elasticsearch.TopicDetails, error) {
	ret := elasticsearch.TopicDetails{}
	apiInsightsTopic, parentMoment := topic.apiInsightsTopic, topic.parentMoment
	// Filters inherited from Parent moment.
	if parentMoment != nil {
		speakerRoles := getSpeakerRoleFilterFromLabel(parentMoment)
		ret.SpeakerRoles = speakerRoles
		msgWindowAsCountWindow, err := getMessageWindowAsMessageCountFromLabel(parentMoment)
		if err != nil {
			return nil, err
		}
		if msgWindowAsCountWindow != nil {
			ret.MessageWindowAsMessageCount = msgWindowAsCountWindow
		}

		msgWindowAsDuration, err := getMessageWindowAsDurationFromLabel(parentMoment)
		if err != nil {
			return nil, err
		}
		if msgWindowAsDuration != nil {
			ret.MessageWindownAsDurationInMin = msgWindowAsDuration
		}
	}
	if len(apiInsightsTopic.SpeakerRoles) > 0 {
		if len(ret.SpeakerRoles) > 0 {
			return nil, status.Errorf(codes.InvalidArgument, "speaker_roles filter already inherited from parent moment")
		}
		ret.SpeakerRoles = apiInsightsTopic.SpeakerRoles
	}

	if apiInsightsTopic.IncludedKeywordMoment != nil {
		regexExpr, excludeRegexExpr := elasticsearch.ExtractRegexFromKeywordMoment(apiInsightsTopic.IncludedKeywordMoment)
		if regexExpr != "" || excludeRegexExpr != "" {
			ret.RegexExpression = regexExpr
			ret.ExcludeRegexExpression = excludeRegexExpr
		} else {
			ret.IncludedPhrases, ret.IncludedPhrasesMatchType, ret.KeywordMomentConstraint = elasticsearch.ConvertKeywordMomentToPhrases(ctx, a.logger, apiInsightsTopic.IncludedKeywordMoment)
		}
	}
	if apiInsightsTopic.ExcludedKeywordMoment != nil {
		// Ignore possible labels from excluded keyword moment.
		ret.ExcludedPhrases, ret.ExcludedPhrasesMatchType, _ = elasticsearch.ConvertKeywordMomentToPhrases(ctx, a.logger, apiInsightsTopic.ExcludedKeywordMoment)
	}
	for _, c := range apiInsightsTopic.ClusterMoments {
		ret.Clusters = append(ret.Clusters, c.Name)
	}
	for _, m := range apiInsightsTopic.PrerequirementMoments {
		ret.PrerequrementMoments = append(ret.PrerequrementMoments, m.Name)
	}
	if elasticsearch.IsEmptyTopicDetails(ret) {
		return nil, nil
	}
	if m := apiInsightsTopic.GetTopicMoment(); m != nil {
		ret.LanguageCode = m.GetLanguageCode()
	}
	if ret.LanguageCode == "" {
		ret.LanguageCode = apiInsightsTopic.GetIncludedKeywordMoment().GetLanguageCode()
	}
	if ret.LanguageCode == "" {
		ret.LanguageCode = consts.AllLanguages
	}
	return &ret, nil
}

func getSpeakerRoleFilterFromLabel(m *momentpb.Moment) []string {
	if m != nil && m.Labels != nil && m.Labels.Labels != nil {
		// Speaker roles is a comma separated string in DB.
		if val := strings.TrimSpace(m.Labels.Labels[speakerRoleFilterLabelKey]); val != "" {
			return strings.Split(val, ",")
		}
	}
	return nil
}

func getMessageWindowAsMessageCountFromLabel(m *momentpb.Moment) (*int32, error) {
	if m != nil && m.Labels != nil && m.Labels.Labels != nil {
		if val := strings.TrimSpace(m.Labels.Labels[messageWindowAsMessageCountLabelKey]); val != "" {
			parsedVal, err := strconv.ParseInt(strings.TrimSpace(val), 10, 32)
			if err != nil {
				return nil, err
			}
			return proto.Int32(int32(parsedVal)), nil
		}
	}
	return nil, nil
}
func getMessageWindowAsDurationFromLabel(m *momentpb.Moment) (*int32, error) {
	if m != nil && m.Labels != nil && m.Labels.Labels != nil {
		if val := strings.TrimSpace(m.Labels.Labels[messageWindowAsDurationLabelKey]); val != "" {
			parsedVal, err := strconv.ParseInt(strings.TrimSpace(val), 10, 32)
			if err != nil {
				return nil, err
			}
			return proto.Int32(int32(parsedVal)), nil
		}
	}
	return nil, nil
}

// nolint: gosec

const (
	preTopicLabelKey                    = "topic_types"
	speakerRoleFilterLabelKey           = "message_filters"
	messageWindowAsMessageCountLabelKey = "window_as_message_count_at_conversation_start"
	messageWindowAsDurationLabelKey     = "window_as_duration_at_conversation_start"
)

// maxInheritedFilterAncestorDepth bounds the ancestor walk. Real CMv2 category trees are
// at most 3 levels deep; 5 adds headroom as a backstop against malformed/cyclic trees.
const maxInheritedFilterAncestorDepth = 5

// resolveMomentNameFromTemplateID qualifies a ParentMomentTemplateId (bare id or full
// resource name) into a moment name within the given profile parent.
func resolveMomentNameFromTemplateID(profileParentName, templateID string) (string, error) {
	if strings.HasPrefix(templateID, "customers/") {
		return templateID, nil
	}
	parent, err := shared.ParseCustomerProfile(profileParentName)
	if err != nil {
		return "", err
	}
	return momentpb.MomentName{
		CustomerID: parent.CustomerID,
		ProfileID:  parent.ProfileID,
		MomentID:   templateID,
	}.String(), nil
}

// momentHasSpeakerRoleFilterLabel reports whether m carries the speaker-role filter label.
// CMv2 folders do not carry it, so the walk skips them.
func momentHasSpeakerRoleFilterLabel(m *momentpb.Moment) bool {
	if m == nil || m.Labels == nil || m.Labels.Labels == nil {
		return false
	}
	return strings.TrimSpace(m.Labels.Labels[speakerRoleFilterLabelKey]) != ""
}

var momentDetailedTypeMapping = map[momentpb.Moment_DetailedType]momentpb.Moment_DetailedType{
	momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASONS_TOPIC:   momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASONS_PRE_TOPIC,
	momentpb.Moment_DETAILED_TYPE_VISITOR_QUESTIONS_TOPIC:      momentpb.Moment_DETAILED_TYPE_VISITOR_QUESTIONS_PRE_TOPIC,
	momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V2_TOPIC: momentpb.Moment_DETAILED_TYPE_UNSPECIFIED, // No Pre Topic
	momentpb.Moment_DETAILED_TYPE_EXTRACTED_QUESTION_TOPIC:     momentpb.Moment_DETAILED_TYPE_UNSPECIFIED, // No Pre Topic
}

// nolint:gosec
// This methods fetches the corresponding prerequirement moments as indicated in topic_types label.
// for a composite moment.
func (a AnalyticsServiceImpl) fetchPrerequirementMomentsFromLabel(ctx context.Context, parentName string, m *momentpb.Moment, momentClient momentpb.MomentServiceClient) ([]*momentpb.Moment, error) {
	detailedTypes := []int32{}
	if m != nil && m.Labels != nil && m.Labels.Labels != nil {
		if val, exists := m.Labels.Labels[preTopicLabelKey]; exists && strings.TrimSpace(val) != "" {
			subStrs := strings.Split(val, ",")
			for _, ss := range subStrs {
				dt, err := strconv.ParseInt(strings.TrimSpace(ss), 10, 32)
				if err != nil {
					return nil, err
				}
				detailedTypes = append(detailedTypes, int32(dt))
			}
		}
	}
	if len(detailedTypes) == 0 {
		return nil, nil
	}

	dts := []momentpb.Moment_DetailedType{}
	for _, dt := range detailedTypes {
		detailedType, exists := momentDetailedTypeMapping[momentpb.Moment_DetailedType(dt)]
		if !exists {
			return nil, fmt.Errorf("invalid detailed type int value found topic_types label %v", dt)
		}
		if detailedType != momentpb.Moment_DETAILED_TYPE_UNSPECIFIED {
			dts = append(dts, detailedType)
		}
	}
	if len(dts) == 0 {
		return nil, nil
	}
	listResp, err := momentClient.ListMoments(ctx, &momentpb.ListMomentsRequest{
		Parent: parentName,
		Filter: &momentpb.ListMomentsRequest_ListMomentsFilter{
			MomentTypes:   []momentpb.Moment_Type{momentpb.Moment_TOPIC},
			AppTypes:      []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
			DetailedTypes: dts,
		},
	})
	if err != nil {
		return nil, err
	}
	return listResp.Moments, nil
}

// convertAPIInsightsTopics convert insightsTopics field to momentGroup field and es.TopicDetails.
func (a AnalyticsServiceImpl) convertAPIInsightsTopics(ctx context.Context, att *analyticspb.Attribute, topicDetails map[string]elasticsearch.TopicDetails, parent shared.CustomerProfile, idSalt string, momentClient momentpb.MomentServiceClient) error {
	if att == nil {
		return nil
	}
	newMomentGroup := analyticspb.MomentGroup{}
	for i, t := range att.InsightsTopics {
		prerequirementMoments, err := a.fetchPrerequirementMomentsFromLabel(ctx, parent.String(), t.TopicMoment, momentClient)
		if err != nil {
			return err
		}

		t.PrerequirementMoments = append(t.PrerequirementMoments, prerequirementMoments...)
		td, err := a.convertInsightsTopicToTopicDetails(ctx, insightsTopic{apiInsightsTopic: t})
		if err != nil {
			return err
		}
		if td != nil {
			// Simply create a unique map key to store new TopicDetails in the map so that
			// it is fetchable in ES module.
			momentName := fmt.Sprintf("customers/%s/profiles/%s/moments/temp-moment-resource-id-%s-%d", parent.CustomerID, parent.ProfileID, idSalt, i)
			topicDetails[momentName] = *td
			newMomentGroup.Moments = append(newMomentGroup.Moments, &momentpb.Moment{Name: momentName, Type: momentpb.Moment_COMPOSITE})
		}
	}
	if len(newMomentGroup.Moments) > 0 {
		att.MomentGroups = append(att.MomentGroups, &newMomentGroup)
	}
	return nil
}
