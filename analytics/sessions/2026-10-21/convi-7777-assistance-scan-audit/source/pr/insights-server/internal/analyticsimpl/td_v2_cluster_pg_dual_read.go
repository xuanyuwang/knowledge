package analyticsimpl

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	conversationpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/featureflag"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/framework/stats"
)

const (
	tdV2ClusterPGReadModeFlag = "td_v2_cluster_postgres_read_mode"
	pgCompareTimeout          = 30 * time.Second

	// tdV2PGMembershipMaxRows bounds a single Postgres membership read.
	//
	// In pg_only mode the result becomes an Elasticsearch `terms` filter, and Elasticsearch
	// rejects a terms list longer than `index.max_terms_count` (65536 by default). In dual_read
	// mode a silently truncated set would surface as a large false `es_only` count. Either way,
	// overflow means "Postgres cannot answer this request" and the caller falls back to
	// Elasticsearch.
	tdV2PGMembershipMaxRows = 50000
)

// Reasons reported on tdV2PGCompareSkipped. The dual read is observability only, so every path
// that declines to compare records why instead of emitting a misleading mismatch count.
const (
	tdV2SkipReasonPaginated        = "paginated"
	tdV2SkipReasonNoTimeRange      = "no_time_range"
	tdV2SkipReasonNoActiveClusters = "no_active_clusters"
	tdV2SkipReasonPGResultTooLarge = "pg_result_too_large"
	tdV2SkipReasonOnlyPartialChat  = "only_return_partial_chat"
)

type tdV2ReadMode string

const (
	tdV2ReadModeESOnly   tdV2ReadMode = "es_only"
	tdV2ReadModeDualRead tdV2ReadMode = "dual_read"
	tdV2ReadModePGOnly   tdV2ReadMode = "pg_only"
)

var (
	tdV2ESPGMismatch     = stats.NewMetric("cresta.insights.td_v2_cluster_es_pg_mismatch")
	tdV2PGReadError      = stats.NewMetric("cresta.insights.td_v2_cluster_pg_read_error")
	tdV2PGCompareSkipped = stats.NewMetric("cresta.insights.td_v2_cluster_pg_compare_skipped")
)

// errTDV2PGMembershipTooLarge signals that the Postgres membership read exceeded
// tdV2PGMembershipMaxRows. Callers treat it as "serve Elasticsearch", not as a failure.
var errTDV2PGMembershipTooLarge = errors.New("TDv2 cluster membership exceeds the Postgres read limit")

// tdV2ClusterDetailedTypes are the moment detailed types written by V2 Topic Discovery
// reclustering: conversation-reason clusters (31) and extracted-question clusters (29).
var tdV2ClusterDetailedTypes = []int32{
	int32(momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V2_TOPIC),
	int32(momentpb.Moment_DETAILED_TYPE_EXTRACTED_QUESTION_TOPIC),
}

func (a AnalyticsServiceImpl) evaluateTDV2ReadMode(ctx context.Context, customerID, profileID string) tdV2ReadMode {
	if a.featureFlagClient == nil {
		return tdV2ReadModeESOnly
	}
	flagOptions := []featureflag.Option{
		featureflag.WithEvaluationContext(featureflag.EvaluationContext{
			CustomerID: customerID,
			ProfileID:  profileID,
		}),
	}
	mode, err := a.featureFlagClient.StringValue(ctx, tdV2ClusterPGReadModeFlag, string(tdV2ReadModeESOnly), flagOptions...)
	if err != nil {
		a.logger.Warnf(ctx, "failed to evaluate %s flag: %v", tdV2ClusterPGReadModeFlag, err)
		return tdV2ReadModeESOnly
	}
	switch tdV2ReadMode(mode) {
	case tdV2ReadModeDualRead, tdV2ReadModePGOnly:
		return tdV2ReadMode(mode)
	default:
		return tdV2ReadModeESOnly
	}
}

// extractTDV2ClusterCandidates returns the conversation-level TOPIC moments that could be V2 Topic
// Discovery clusters. It is a cheap, allocation-only prefilter: V3 categories are excluded because
// they carry an unambiguous detailed type, but legacy topics and pre-topics are indistinguishable
// here and are narrowed later by resolveTDV2Clusters.
func extractTDV2ClusterCandidates(req *analyticspb.RetrieveConversationMessagesRequest) []*momentpb.Moment {
	if req.GetFilterConversationByAttribute() == nil {
		return nil
	}
	var candidates []*momentpb.Moment
	for _, mg := range req.GetFilterConversationByAttribute().GetMomentGroups() {
		for _, m := range mg.GetMoments() {
			if m.GetType() == momentpb.Moment_TOPIC && !elasticsearch.IsTopicDiscoveryV3Moment(m) {
				candidates = append(candidates, m)
			}
		}
	}
	return candidates
}

//nolint:gocritic // hand-written GORM model outside a .../gen*/... path
type tdV2ClusterTemplateRow struct {
	MomentTemplateID string `gorm:"column:moment_template_id"`
}

// resolveTDV2Clusters narrows candidate TOPIC moments down to the ones that really are V2 Topic
// Discovery clusters, by intersecting them with the active TDv2 template set in Postgres. It
// returns the matching template ids (sorted, for a stable query) and the matching moment names.
//
// The request cannot be trusted to carry detailed_type. detailed_type is not stored in
// Elasticsearch, so several Director call sites send topic moments as `{name, type}` only — see
// director's ProcessDetails outcome/topic filter. Reading GetDetailedType() off those moments
// yields DETAILED_TYPE_UNSPECIFIED, which filters every Postgres row out and would make pg_only
// return an empty response. app.moment_templates holds the authoritative detailed_type.
func (a AnalyticsServiceImpl) resolveTDV2Clusters(
	ctx context.Context,
	customerID, profileID string,
	candidates []*momentpb.Moment,
) (templateIDs []string, clusterNames set.Set[string], err error) {
	templateIDsByMomentName := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		momentName, parseErr := momentpb.ParseMomentName(candidate.GetName())
		if parseErr != nil {
			a.logger.Warnf(ctx, "failed to parse moment name %s: %v", candidate.GetName(), parseErr)
			continue
		}
		templateIDsByMomentName[candidate.GetName()] = momentName.MomentID
	}
	if len(templateIDsByMomentName) == 0 {
		return nil, set.New[string](), nil
	}

	activeTemplateIDs, err := a.prefetchTDV2ClusterTemplateIDs(ctx, customerID, profileID)
	if err != nil {
		return nil, nil, err
	}

	clusterNames = set.New[string]()
	templateIDs = make([]string, 0, len(templateIDsByMomentName))
	for momentName, templateID := range templateIDsByMomentName {
		if !activeTemplateIDs.Has(templateID) {
			continue
		}
		clusterNames.Add(momentName)
		templateIDs = append(templateIDs, templateID)
	}
	slices.Sort(templateIDs)
	return templateIDs, clusterNames, nil
}

func (a AnalyticsServiceImpl) prefetchTDV2ClusterTemplateIDs(
	ctx context.Context,
	customerID, profileID string,
) (set.Set[string], error) {
	pgCtx, cancel := context.WithTimeout(ctx, pgCompareTimeout)
	defer cancel()

	var rows []tdV2ClusterTemplateRow
	result := a.appsDB.DB(pgCtx).
		Table("app.moment_templates").
		Select("moment_template_id").
		Where("customer_id = ? AND profile_id = ? AND taxonomy_state = ? AND type = ? AND detailed_type IN ?",
			customerID, profileID,
			int32(momentpb.Moment_STATE_ACTIVE), int32(momentpb.Moment_TOPIC), tdV2ClusterDetailedTypes).
		Find(&rows)
	if result.Error != nil {
		return nil, fmt.Errorf("prefetching active TDv2 cluster templates for %s/%s: %w", customerID, profileID, result.Error)
	}

	templateIDs := set.New[string]()
	for _, row := range rows {
		templateIDs.Add(row.MomentTemplateID)
	}
	return templateIDs, nil
}

func (a AnalyticsServiceImpl) compareTDV2ClusterMembership(
	ctx context.Context,
	parent *shared.CustomerProfile,
	req *analyticspb.RetrieveConversationMessagesRequest,
	resp *analyticspb.RetrieveConversationMessagesResponse,
	candidates []*momentpb.Moment,
) {
	// resp holds a single search_after page while the Postgres read covers the whole time range,
	// so on a paginated request every off-page Postgres member would be miscounted as pg_only.
	// Compare only when the request and the response are both unpaginated.
	if req.GetPageToken() != "" || resp.GetNextPageToken() != "" {
		a.skipTDV2Compare(ctx, parent.CustomerID, tdV2SkipReasonPaginated)
		return
	}

	timeRange := req.GetFilterByTimeRange()
	if timeRange.GetStartTimestamp() == nil || timeRange.GetEndTimestamp() == nil {
		a.skipTDV2Compare(ctx, parent.CustomerID, tdV2SkipReasonNoTimeRange)
		return
	}

	clusterTemplateIDs, _, err := a.resolveTDV2Clusters(ctx, parent.CustomerID, parent.ProfileID, candidates)
	if err != nil {
		// fail-open: the dual read is observability only, so a Postgres error never affects serving.
		a.logger.Warnf(ctx, "TDv2 cluster template prefetch failed: %v", err)
		tdV2PGReadError.Incr(ctx, "customer_id:"+parent.CustomerID)
		return
	}
	if len(clusterTemplateIDs) == 0 {
		a.skipTDV2Compare(ctx, parent.CustomerID, tdV2SkipReasonNoActiveClusters)
		return
	}

	isMessageMode := req.GetFilterConversationDocumentByConversationByAttribute() == nil
	esSet := extractESMembershipSet(resp, isMessageMode)

	pgSet, err := a.queryPGClusterMembership(
		ctx, parent.CustomerID, parent.ProfileID, clusterTemplateIDs,
		timeRange.GetStartTimestamp().AsTime(), timeRange.GetEndTimestamp().AsTime(),
		isMessageMode,
	)
	if errors.Is(err, errTDV2PGMembershipTooLarge) {
		a.skipTDV2Compare(ctx, parent.CustomerID, tdV2SkipReasonPGResultTooLarge)
		return
	}
	if err != nil {
		// fail-open: the dual read is observability only, so a Postgres error never affects serving.
		a.logger.Warnf(ctx, "TDv2 cluster PG dual-read failed: %v", err)
		tdV2PGReadError.Incr(ctx, "customer_id:"+parent.CustomerID)
		return
	}

	// These totals span every cluster in the request and are not partitioned by detailed type, so
	// they carry no detailed_type tag: emitting one copy per type would multiply each count by the
	// number of types present.
	esOnly, pgOnly, both := compareSets(esSet, pgSet)
	tdV2ESPGMismatch.Count(ctx, int64(esOnly), "customer_id:"+parent.CustomerID, "side:es_only")
	tdV2ESPGMismatch.Count(ctx, int64(pgOnly), "customer_id:"+parent.CustomerID, "side:pg_only")
	tdV2ESPGMismatch.Count(ctx, int64(both), "customer_id:"+parent.CustomerID, "side:both")
}

func (a AnalyticsServiceImpl) skipTDV2Compare(ctx context.Context, customerID, reason string) {
	tdV2PGCompareSkipped.Incr(ctx, "customer_id:"+customerID, "reason:"+reason)
}

func extractESMembershipSet(resp *analyticspb.RetrieveConversationMessagesResponse, isMessageMode bool) set.Set[string] {
	esSet := set.New[string]()
	for _, cmr := range resp.GetConversationMessageResults() {
		conv := cmr.GetConversationResult().GetConversation()
		if conv == nil {
			continue
		}
		convName, err := conv.ParseResourceName()
		if err != nil {
			continue
		}
		convID := convName.GetConversationID()
		if !isMessageMode {
			esSet.Add(convID)
			continue
		}
		addESMessageKeys(esSet, convID, cmr.GetMessageResults())
	}
	return esSet
}

func addESMessageKeys(esSet set.Set[string], convID string, messageResults []*analyticspb.ConversationMessageResult_MessageResult) {
	for _, mr := range messageResults {
		msg := mr.GetConversationMessage()
		if msg == nil {
			continue
		}
		msgName, err := conversationpb.ParseConversationMessageName(msg.GetName())
		if err != nil {
			continue
		}
		esSet.Add(convID + ":" + msgName.MessageID)
	}
}

type pgClusterAnnotationRow struct {
	ConversationID string
	MessageID      string
}

func (a AnalyticsServiceImpl) queryPGClusterMembership(
	ctx context.Context,
	customerID, profileID string,
	clusterTemplateIDs []string,
	startTime, endTime time.Time,
	isMessageMode bool,
) (set.Set[string], error) {
	pgCtx, cancel := context.WithTimeout(ctx, pgCompareTimeout)
	defer cancel()

	selectClause := "DISTINCT a.conversation_id"
	whereClause := "a.customer_id = ? AND a.profile_id = ? AND a.type = ? AND a.detailed_type IN ? " +
		"AND a.moment_template_id IN ? AND c.started_at >= ? AND c.started_at < ?"
	if isMessageMode {
		selectClause = "DISTINCT a.conversation_id, a.message_id"
		// Only message mode needs a message id in the key. Conversation mode must not filter on
		// it: a conversation-level annotation has message_id NULL, and dropping it there would
		// show up as a false es_only rather than protecting anything.
		whereClause += " AND a.message_id IS NOT NULL"
	}

	var rows []pgClusterAnnotationRow
	result := a.appsDB.DB(pgCtx).
		Table("app.moment_annotations AS a").
		Select(selectClause).
		Joins("JOIN app.chats AS c ON c.customer_id = a.customer_id AND c.profile_id = a.profile_id AND c.conversation_id = a.conversation_id").
		Where(whereClause,
			customerID, profileID, int32(momentpb.Moment_TOPIC), tdV2ClusterDetailedTypes,
			clusterTemplateIDs, startTime, endTime).
		Limit(tdV2PGMembershipMaxRows + 1).
		Find(&rows)
	if result.Error != nil {
		return nil, fmt.Errorf("querying TDv2 cluster membership for %s/%s: %w", customerID, profileID, result.Error)
	}
	if len(rows) > tdV2PGMembershipMaxRows {
		return nil, fmt.Errorf("%w: over %d rows in [%s, %s)",
			errTDV2PGMembershipTooLarge, tdV2PGMembershipMaxRows, startTime, endTime)
	}

	pgSet := set.New[string]()
	for _, row := range rows {
		if isMessageMode {
			pgSet.Add(row.ConversationID + ":" + row.MessageID)
			continue
		}
		pgSet.Add(row.ConversationID)
	}
	return pgSet, nil
}

// applyPGOnlyClusterFilter rewrites the request in place so that TDv2 cluster membership comes from
// Postgres instead of the Elasticsearch has_child moment filter, and returns how many conversations
// Postgres matched. Any error means the request was left untouched and the caller should serve
// Elasticsearch.
func (a AnalyticsServiceImpl) applyPGOnlyClusterFilter(
	ctx context.Context,
	parent *shared.CustomerProfile,
	req *analyticspb.RetrieveConversationMessagesRequest,
	candidates []*momentpb.Moment,
) (int, error) {
	timeRange := req.GetFilterByTimeRange()
	if timeRange.GetStartTimestamp() == nil || timeRange.GetEndTimestamp() == nil {
		return 0, errors.New("nil time range: cannot query PG cluster membership")
	}
	if len(req.GetFilterConversationByAttribute().GetConversations()) > 0 {
		return 0, fmt.Errorf("pre-existing conversation filter (%d entries) incompatible with pg_only cluster mode",
			len(req.GetFilterConversationByAttribute().GetConversations()))
	}

	clusterTemplateIDs, clusterNames, err := a.resolveTDV2Clusters(ctx, parent.CustomerID, parent.ProfileID, candidates)
	if err != nil {
		return 0, err
	}
	if len(clusterTemplateIDs) == 0 {
		return 0, errors.New("no active TDv2 cluster templates matched the request moments")
	}
	if err := validateTDV2ClusterGroupsIsolated(req.GetFilterConversationByAttribute(), clusterNames); err != nil {
		return 0, err
	}

	pgConvIDs, err := a.queryPGClusterMembership(
		ctx, parent.CustomerID, parent.ProfileID, clusterTemplateIDs,
		timeRange.GetStartTimestamp().AsTime(), timeRange.GetEndTimestamp().AsTime(),
		false, /* isMessageMode */
	)
	if err != nil {
		return 0, err
	}

	attr := req.FilterConversationByAttribute
	stripTDV2ClustersFromMomentGroups(attr, clusterNames)
	for convID := range pgConvIDs.All() {
		attr.Conversations = append(attr.Conversations, &conversationpb.Conversation{
			Name: conversationpb.ConversationName{
				CustomerID:     parent.CustomerID,
				ProfileID:      parent.ProfileID,
				ConversationID: convID,
			}.String(),
		})
	}

	return pgConvIDs.Len(), nil
}

// validateTDV2ClusterGroupsIsolated rejects a request whose TDv2 clusters share a MomentGroup with
// anything else.
//
// Moments inside a group are OR-ed and groups are AND-ed together, but pg_only strips the clusters
// and replaces them with an AND-ed conversation-id terms filter. Rewriting a group of
// `(clusterX OR momentY)` would therefore silently turn it into `momentY AND conv IN (...)`.
// Director normally puts a cluster in a group of its own; when it does not, serve Elasticsearch.
func validateTDV2ClusterGroupsIsolated(attr *analyticspb.Attribute, clusterNames set.Set[string]) error {
	for i, mg := range attr.GetMomentGroups() {
		clusterCount := countTDV2Clusters(mg.GetMoments(), clusterNames)
		if clusterCount == 0 {
			continue
		}
		if clusterCount != len(mg.GetMoments()) {
			return fmt.Errorf("moment group %d mixes TDv2 clusters with other moments: incompatible with pg_only cluster mode", i)
		}
		if len(mg.GetExcludedMoments()) > 0 || len(mg.GetBehaviors()) > 0 ||
			len(mg.GetMetadataValueAttributes()) > 0 || len(mg.GetOutcomeValueAttributes()) > 0 {
			return fmt.Errorf("moment group %d combines TDv2 clusters with additional filters: incompatible with pg_only cluster mode", i)
		}
		// The Postgres read ORs the cluster template ids together, so an AND-ed group of several
		// clusters would be widened into an OR.
		if clusterCount > 1 && mg.GetBooleanLogicOperator() == analyticspb.MomentGroup_BOOLEAN_LOGIC_OPERATOR_AND {
			return fmt.Errorf("moment group %d AND-s %d TDv2 clusters: incompatible with pg_only cluster mode", i, clusterCount)
		}
	}
	return nil
}

func countTDV2Clusters(moments []*momentpb.Moment, clusterNames set.Set[string]) int {
	count := 0
	for _, m := range moments {
		if clusterNames.Has(m.GetName()) {
			count++
		}
	}
	return count
}

func stripTDV2ClustersFromMomentGroups(attr *analyticspb.Attribute, clusterNames set.Set[string]) {
	if attr == nil {
		return
	}
	filteredGroups := make([]*analyticspb.MomentGroup, 0, len(attr.GetMomentGroups()))
	for _, mg := range attr.GetMomentGroups() {
		kept := keepNonTDV2Clusters(mg.GetMoments(), clusterNames)
		if len(kept) == 0 {
			continue
		}
		mg.Moments = kept
		filteredGroups = append(filteredGroups, mg)
	}
	attr.MomentGroups = filteredGroups
}

func keepNonTDV2Clusters(moments []*momentpb.Moment, clusterNames set.Set[string]) []*momentpb.Moment {
	kept := make([]*momentpb.Moment, 0, len(moments))
	for _, m := range moments {
		if clusterNames.Has(m.GetName()) {
			continue
		}
		kept = append(kept, m)
	}
	return kept
}

func compareSets(esSet, pgSet set.Set[string]) (esOnly, pgOnly, both int) {
	for item := range esSet.All() {
		if pgSet.Has(item) {
			both++
			continue
		}
		esOnly++
	}
	for item := range pgSet.All() {
		if !esSet.Has(item) {
			pgOnly++
		}
	}
	return esOnly, pgOnly, both
}
