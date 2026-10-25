import type { Dayjs } from 'dayjs';

import type {
  AdherenceType,
  AgentTier,
  AnalyticsResult,
  Attribute as ApiAttribute,
  ChatBotAttribute,
  CoachingEfficiencyStats as CrestaV1RetrieveCoachingEfficiencyStatsResponse,
  EvaluationPeriodProgressStatus,
  QuintileRank,
  RetrieveAnalyticsChartExamplesResponse,
} from '@cresta/web-client';

import type { Group, GroupMembership, Intent, Team, User } from '../../../types';
import type { Action, ActionAnnotation } from '../action';
import type { Behavior } from '../behavior';
import type { Conversation, ConversationMessage } from '../conversation';
import type {
  ConversationMetadataNumericBin,
  ConversationMetadataOrOutcomeValue,
  ConversationOutcomeOutcomeValue,
  MatchedMoment,
  Moment,
  MomentAnnotation,
} from '../moment';
import type { AttributePolicy, Policy } from '../policy';

export interface TimeRange {
  startTimestamp?: string;
  endTimestamp?: string;
  startTime?: Dayjs;
  endTime?: Dayjs;
}

export interface Attribute {
  user?: User;
  team?: Team;
  group?: Group;
  criterionIdentifiers?: string[];
  criterionIdentifier?: string;
  intents?: Intent[];
  moments?: Moment[];
  actions?: Action[];
  policies?: Policy[];
  momentGroups?: MomentGroup[];
  timeRanges?: TimeRange[];
  conversationDurationBuckets?: ConversationDurationBucket[];
  adherence?: AdherenceType;
  groups?: Group[];
  behaviors?: Behavior[];
  languageCodes?: string[];
  coachingSession?: string;
  coachingSessionNames?: string[];
  guidedWorkflowNames?: string[];
  attributePolicies?: AttributePolicy[];
}

// A group of moments whose taxonomy have the same prefix.
export interface MomentGroup {
  moments: Moment[];
  metadataValueAttributes: MetadataValueAttribute[];
  outcomeValueAttributes?: ConversationOutcomeOutcomeValue[];
}

export interface MetadataValueAttribute {
  metadataValue?: ConversationMetadataOrOutcomeValue;
  numericBin?: ConversationMetadataNumericBin;
}

export interface MetadataValuesResponse {
  metadataValues: ConversationMetadataOrOutcomeValue[];
}

// A single LLM annotation value (scalar string/number/boolean). Structurally
// identical to ConversationMetadataOrOutcomeValue, but kept distinct so the LLM
// annotation surface is not coupled to metadata/outcome naming (mirrors how the
// backend response reuses the neutral common metadata value type).
export interface LlmAnnotationValue {
  numberValue?: number;
  stringValue?: string;
  booleanValue?: boolean;
}

export interface LlmAnnotationValuesResponse {
  values: LlmAnnotationValue[];
}

export interface ConversationAssistanceUsageStatsGroupResult {
  groupedByAttributes: Attribute;
  totalConversationCount: number;
  conversationsWithAssistanceUsedCount: number;
  conversationsWithAssistanceUsedPercent?: number;
}

export interface ConversationAssistanceUsageStatsResponse {
  resultGroups: ConversationAssistanceUsageStatsGroupResult[];
}

// The dedicated RPC is not yet included in the published web client.
export interface ConversationAssistanceUsageStatsApiResponse {
  conversationAssistanceUsageStatsResults?: {
    attribute?: ApiAttribute;
    totalConversationCount?: number;
    conversationsWithAssistanceUsedCount?: number;
  }[];
}

export interface ConversationStatsEntry {
  averageHandleTime: number;
  totalConversationCount: number;
  intervalStart: Dayjs;
  averageConversationCountPerFrequencyPerUser: number;
}

export interface ConversationStatsGroupResult {
  averageConversationCountPerFrequencyPerUser: number;
  averageHandleTime: number;
  totalConversationCount: number;
  entries: ConversationStatsEntry[];
  groupedByAttributes: Attribute;
}

export interface ConversationStatsResponse {
  totalConversationCount: number;
  averageConversationCountPerFrequency: number;
  averageConversationCountPerFrequencyPerUser: number;
  averageHandleTime: number;
  resultGroups: ConversationStatsGroupResult[];
}

export interface AssistanceStatsEntry {
  totalAssistanceUsedCount: number;
  totalAssistanceSentCount: number;
  averagePerConversation: number;
  averagePerUser: number;
  intervalStart: Dayjs;
  totalConversationCount?: number;
}

export interface AssistanceStatsGroupResult {
  groupedByAttributes: Attribute;
  totalAssistanceCount: number;
  totalHintsFollowedCount: number;
  totalKbSearchCount: number;
  totalAutoSummarizationUsedCount: number;
  totalConversationWithAutoSummarizationCount: number;
  totalSuggestionCount: number;
  totalSmartComposeCount: number;
  totalNoteCount: number;
  totalGwCount: number;
  totalEmailPartialSuggestionCount: number;
  totalEmailFullSuggestionCount: number;
  averageAssistanceCountPerConv: number;
  averageHintsFollowedPerConv: number;
  averageKbSearchCountPerConv: number;
  averageAutoSummarizationUsedCountPerConv: number;
  averageSuggestionCountPerConv: number;
  averageSmartComposeCountPerConv: number;
  averageNoteCountPerConv: number;
  averageGwCountPerConv: number;
  averageEmailFullSuggestionCountPerConv: number;
  averageEmailPartialSuggestionCountPerConv: number;
  averageAssistanceCountPerFrequencyPerUser: number;
  averageHintsFollowedPerFrequencyPerUser: number;
  averageKbSearchCountPerFrequencyPerUser: number;
  averageAutoSummarizationUsedCountPerFrequencyPerUser: number;
  averageAutoSummarizationUsedRatioPerFrequencyPerUser: number;
  averageConversationWithAutoSummarizationCountPerFrequencyPerUser: number;
  averageSuggestionCountPerFrequencyPerUser: number;
  averageSmartComposeCountPerFrequencyPerUser: number;
  averageNoteCountPerFrequencyPerUser: number;
  averageGwCountPerFrequencyPerUser: number;
  averageEmailFullSuggestionCountPerFrequencyPerUser: number;
  averageEmailPartialSuggestionCountPerFrequencyPerUser: number;
  hintFollowedStats: AssistanceStatsEntry[];
  kbSearchStats: AssistanceStatsEntry[];
  autoSummarizationStats: AssistanceStatsEntry[];
  autoSummarizationRatioStats: AssistanceStatsEntry[];
  autoSummarizationConversationStats: AssistanceStatsEntry[];
  suggestionsStats: AssistanceStatsEntry[];
  smartComposeStats: AssistanceStatsEntry[];
  totalAssistanceStats: AssistanceStatsEntry[];
  noteStats: AssistanceStatsEntry[];
  guidedWorkflowStats: AssistanceStatsEntry[];
  emailPartialSuggestionsStats: AssistanceStatsEntry[];
  emailFullSuggestionsStats: AssistanceStatsEntry[];
}

export interface AssistanceStatsResponse {
  resultsGroupedByAttributeTypes: AssistanceStatsGroupResult[];
  totalAssistanceCount: number;
  totalHintsFollowedCount: number;
  totalKbSearchCount: number;
  totalAutoSummarizationUsedCount: number;
  totalConversationWithAutoSummarizationCount: number;
  totalSuggestionCount: number;
  totalSmartComposeCount: number;
  totalNotesCount: number;
  totalGwCount: number;
  totalEmailPartialSuggestionCount: number;
  totalEmailFullSuggestionCount: number;
  averageAssistanceUsedCountPerFrequencyPerUser: number;
  averageAutoSummarizationUsedCountPerFrequencyPerUser: number;
  averageAutoSummarizationUsedRatioPerFrequencyPerUser: number;
  averageConversationWithAutoSummarizationCountPerFrequencyPerUser: number;
  averageHintFollowedCountPerFrequencyPerUser: number;
  averageKbSearchCountPerFrequencyPerUser: number;
  averageNoteCountPerFrequencyPerUser: number;
  averageSmartComposeUsedCountPerFrequencyPerUser: number;
  averageSuggestionUsedCountPerFrequencyPerUser: number;
  averageGwCountPerFrequencyPerUser: number;
  averageEmailPartialSuggestionUsedCountPerFrequencyPerUser: number;
  averageEmailFullSuggestionUsedCountPerFrequencyPerUser: number;
}

interface BaseSuggestionStats {
  totalSuggestionUsedCount: number;
  averageSuggestionUsedCountPerConversation: number;
  averageSuggestionUsedCountPerFrequencyPerUser: number;
  totalEmailPartialSuggestionUsedCount: number;
  averageEmailPartialSuggestionUsedCountPerConversation: number;
  averageEmailPartialSuggestionUsedCountPerFrequencyPerUser: number;
  totalEmailFullSuggestionUsedCount: number;
  averageEmailFullSuggestionUsedCountPerConversation: number;
  averageEmailFullSuggestionUsedCountPerFrequencyPerUser: number;
  totalConversationCount: number;
}
export interface SuggestionStatsEntry extends BaseSuggestionStats {
  intervalStart: Dayjs;
}

export interface SuggestionStatsGroupResult extends BaseSuggestionStats {
  groupedByAttributes: Attribute;
  suggestionStats: SuggestionStatsEntry[];
}

export interface SuggestionStatsResponse extends BaseSuggestionStats {
  resultsGroupedByAttributeTypes: SuggestionStatsGroupResult[];
}

interface BaseSmartComposeStats {
  totalSmartComposeUsedCount: number;
  averageSmartComposeUsedCountPerConversation: number;
  averageSmartComposeUsedCountPerFrequencyPerUser: number;
  totalConversationCount: number;
}

export interface SmartComposeStatsEntry extends BaseSmartComposeStats {
  intervalStart: Dayjs;
}

export interface SmartComposeStatsGroupResult extends BaseSmartComposeStats {
  groupedByAttributes: Attribute;
  smartComposeStats: SmartComposeStatsEntry[];
}

export interface SmartComposeStatsResponse extends BaseSmartComposeStats {
  resultsGroupedByAttributeTypes: SmartComposeStatsGroupResult[];
}

interface BaseKnowledgeBaseStats {
  totalKbSearchCount: number;
  averageKbSearchCountPerConversation: number;
  averageKbSearchCountPerFrequencyPerUser: number;
  totalConversationCount: number;
}

export interface KnowledgeBaseStatsEntry extends BaseKnowledgeBaseStats {
  intervalStart: Dayjs;
}

export interface KnowledgeBaseStatsGroupResult extends BaseKnowledgeBaseStats {
  groupedByAttributes: Attribute;
  knowledgeBaseStats: KnowledgeBaseStatsEntry[];
}

export interface KnowledgeBaseStatsResponse extends BaseKnowledgeBaseStats {
  resultsGroupedByAttributeTypes: KnowledgeBaseStatsGroupResult[];
}

interface BaseNoteTakingStats {
  totalNoteTakingUsedCount: number;
  averageNoteTakingUsedCountPerConversation: number;
  averageNoteTakingUsedCountPerFrequencyPerUser: number;
  totalConversationCount: number;
}

export interface NoteTakingStatsEntry extends BaseNoteTakingStats {
  intervalStart: Dayjs;
}

export interface NoteTakingStatsGroupResult extends BaseNoteTakingStats {
  groupedByAttributes: Attribute;
  noteTakingStats: NoteTakingStatsEntry[];
}

export interface NoteTakingStatsResponse extends BaseNoteTakingStats {
  resultsGroupedByAttributeTypes: NoteTakingStatsGroupResult[];
}

interface BaseGuidedWorkflowStats {
  totalGuidedWorkflowClickCount: number;
  averageGuidedWorkflowClickCountPerConversation: number;
  averageGuidedWorkflowClickCountPerFrequencyPerUser: number;
  totalConversationCount: number;
}
export interface GuidedWorkflowStatsEntry extends BaseGuidedWorkflowStats {
  intervalStart: Dayjs;
}

export interface GuidedWorkflowStatsGroupResult extends BaseGuidedWorkflowStats {
  groupedByAttributes: Attribute;
  guidedWorkflowStats: GuidedWorkflowStatsEntry[];
}

export interface GuidedWorkflowStatsResponse extends BaseGuidedWorkflowStats {
  resultsGroupedByAttributeTypes: GuidedWorkflowStatsGroupResult[];
}

interface BaseSummarizationStats {
  totalSummarizationUsedCount: number;
  totalSummarizationGeneratedCount: number;
  averageSummarizationUsedRatio: number;
  averageSummarizationUsedCountPerConversation: number;
  averageSummarizationUsedCountPerFrequencyPerUser: number;
  averageSummarizationGeneratedCountPerFrequencyPerUser: number;
  totalConversationCount: number;
}

export interface SummarizationStatsEntry extends BaseSummarizationStats {
  intervalStart: Dayjs;
}

export interface SummarizationStatsGroupResult extends BaseSummarizationStats {
  groupedByAttributes: Attribute;
  summarizationStats: SummarizationStatsEntry[];
}

export interface SummarizationStatsResponse extends BaseSummarizationStats {
  resultsGroupedByAttributeTypes: SummarizationStatsGroupResult[];
}

export interface AdherenceStatsResponse {
  averageAdherence: number;
  resultsGroupedByAttributeTypes: AdherenceStatsGroupResult[];
  totalSdxCount: number;
}

export interface AdherenceStatsGroupResult {
  averageAdherence: number;
  entries: AdherenceEntry[];
  groupedByAttributes: Attribute;
  totalSdxCount: number;
}

export interface AdherenceEntry {
  adherence: number;
  intervalStart: Dayjs;
  totalSdxCount: number;
}

interface CommonHintStats {
  totalHintSentCount: number;
  totalHintFollowedCount: number;
  totalConversationCount: number;
  averageHintSentCountPerConversation: number;
  averageHintSentCountPerFrequencyPerUser: number;
  averageHintFollowedCountPerConversation: number;
  averageHintFollowedCountPerFrequencyPerUser: number;
}

export interface HintStatsResponse extends CommonHintStats {
  resultGroups: HintStatsGroupResult[];
}

export interface HintStatsGroupResult extends CommonHintStats {
  entries: HintStatsEntry[];
  groupedByAttributes: Attribute;
}

export interface HintStatsEntry {
  totalHintSentCount: number;
  totalHintFollowedCount: number;
  totalConversationCount: number;
  averageHintFollowedCountPerFrequencyPerUser: number;
  intervalStart: Dayjs;
}

export interface HintEngagementResponse {
  resultsGroupedByAttributeTypes: HintEngagementGroupResult[];
  averageHintEngagement: number;
}

export interface HintEngagementGroupResult {
  averageHintEngagement: number;
  entries: HintEngagementEntry[];
  groupedByAttributes: Attribute;
}

export interface HintEngagementEntry {
  hintEngagement: number;
  intervalStart: Dayjs;
}

export interface AgentStatsEntry {
  intervalStart: Dayjs;
  activeAgentCount: number;
  averageActiveAgentCountPerFrequency: number;
  averageActiveDayCountPerUser: number;
}

export interface AgentStatsGroupResult {
  activeAgentCount: number;
  averageActiveAgentCountPerFrequency: number;
  averageActiveDayCountPerUser: number;
  entries: AgentStatsEntry[];
  groupedByAttributes: Attribute;
}

export interface AgentStatsResponse {
  resultsGroupedByAttributeTypes: AgentStatsGroupResult[];
  activeAgentCount: number;
  averageActiveAgentCountPerFrequency: number;
  averageActiveDayCountPerUser: number;
}

export interface ConversationMessageStatsResponse {
  conversationMessageStats: ConversationMessageStats[];
  aggregatedConversationMessageStats: AggregatedConversationMessageStats;
}

export interface ConversationMessagesResponse {
  conversationMessageResults: ConversationMessageResult[];
  nextPageToken?: string;
}

export interface ConversationMessageResult {
  messageResults: ConversationMessageResultMessageResult[];
  conversationResult: ConversationMessageResultConversationResult;
}

export interface ConversationMessageResultConversationResult {
  conversation?: Conversation;
}

export interface ConversationMessageResultMessageResult {
  conversationMessage?: ConversationMessage;
  momentResults: MomentResult[];
  adherenceType?: AdherenceType;
}

export interface MomentResult {
  moment?: Moment;
  matchedMoment?: MatchedMoment;
  momentAnnotation?: MomentAnnotation;
}

export interface ConversationMessageStats {
  aggregatedConversationMessageStats: AggregatedConversationMessageStats;
  attribute: Attribute;
}

export interface AggregatedConversationMessageStats {
  totalConversationCount: number;
  totalConversationMessageCount: number;
  averageHandleTime: string;
}

export interface KeywordsStats {
  intervalStart: Dayjs;
  totalConversationCount: number;
  totalMessageCount: number;
  keywordsConversationCount: number;
  keywordsMessageCount: number;
}

export interface RetrieveKeywordsResult {
  keywordStats: KeywordsStats[];
}

export interface KeywordsStatsResponse {
  retrieveKeywordsResults: RetrieveKeywordsResult[];
  overallStats: KeywordsStats;
}

export interface ChatbotConversationStatsGroupedResult {
  totalConversationCount: number;
  averageConversationCountPerFrequency: number;
  totalMessageCount: number;
  averageMessageCountPerFrequency: number;
  attribute?: ChatBotAttribute;
}

export interface ChatbotConversationStatsResponse {
  totalConversationCount: number;
  averageConversationCountPerFrequency: number;
  totalMessageCount: number;
  averageMessageCountPerFrequency: number;
  groupedResult: ChatbotConversationStatsGroupedResult[];
}

export interface ChatbotSessionStatsResponse {
  totalWidgetSessionCount: number;
}

export interface CommentStatsResponse {
  totalCommentsPlacedCount: number;
  averageCommentsPlacedCountPerUser: number;
  resultsGroupedByAttributeTypes: CommentStatsGroupResult[];
}

export interface CommentStatsGroupResult {
  totalCommentsPlacedCount: number;
  averageCommentsPlacedCountPerUser: number;
  entries: CommentStatsEntry[];
  groupedByAttributes: Attribute;
}

export interface CommentStatsEntry {
  totalCommentsPlacedCount: number;
  averageCommentsPlacedCountPerUser: number;
  intervalStart: Dayjs;
}

export interface LiveAssistStatsResponse {
  totalRaisedHandCount: number;
  totalRaisedHandAnsweredCount: number;
  totalWhisperedToAgentCount: number;
  totalWhisperByManagerCount: number;
  resultsGroupedByAttributeTypes: LiveAssistStatsGroupResult[];
}

export interface LiveAssistStatsGroupResult {
  entries: LiveAssistStatsEntry[];
  groupedByAttributes: Attribute;
  totalRaisedHandCount: number;
  totalRaisedHandAnsweredCount: number;
  totalWhisperedToAgentCount: number;
  totalWhisperByManagerCount: number;
}

export interface LiveAssistStatsEntry {
  intervalStart: Dayjs;
  totalRaisedHandCount: number;
  totalRaisedHandAnsweredCount: number;
  totalWhisperedToAgentCount: number;
  totalWhisperByManagerCount: number;
}

export interface KnowledgeAssistStatsResponse {
  totalGenAIAnswersViewedCount: number;
  totalGenAIAnswersUsedCount: number;
  totalKnowledgeMomentDetectedCount: number;
  averageGenAIAnswersUsedCountPerUser: number;
  resultsGroupedByAttributeTypes: KnowledgeAssistStatsGroupResult[];
}

export interface KnowledgeAssistStatsGroupResult {
  entries: KnowledgeAssistStatsEntry[];
  groupedByAttributes: Attribute;
  totalGenAIAnswersViewedCount: number;
  totalGenAIAnswersUsedCount: number;
  totalKnowledgeMomentDetectedCount: number;
  averageGenAIAnswersUsedCountPerUser: number;
}

export interface KnowledgeAssistStatsEntry {
  intervalStart: Dayjs;
  totalGenAIAnswersViewedCount: number;
  totalGenAIAnswersUsedCount: number;
  totalKnowledgeMomentDetectedCount: number;
  averageGenAIAnswersUsedCountPerUser: number;
}

export interface CustomerSnapshotStatsResponse {
  totalDisplayTime: number;
  averageDisplayTime: number;
  averageDisplayTimePerFrequencyPerUser: number;
  totalConversationsWithSnapshot: number;
  resultsGroupedByAttributeTypes: CustomerSnapshotStatsGroupResult[];
}

export interface CustomerSnapshotStatsGroupResult {
  entries: CustomerSnapshotStatsEntry[];
  groupedByAttributes: Attribute;
  totalDisplayTime: number;
  averageDisplayTime: number;
  averageDisplayTimePerFrequencyPerUser: number;
  totalConversationsWithSnapshot: number;
}

export interface CustomerSnapshotStatsEntry {
  intervalStart: Dayjs;
  totalDisplayTime: number;
  averageDisplayTime: number;
  averageDisplayTimePerFrequencyPerUser: number;
  totalConversationsWithSnapshot: number;
}

export interface ManagerStatsEntry {
  intervalStart: Dayjs;
  totalDirectorPagesVisitCount: number;
  totalDirectorLiveChatPagesVisitCount: number;
  totalDirectorClosedChatPagesVisitCount: number;
}

export interface ManagerStatsGroupResult {
  totalDirectorPagesVisitCount: number;
  totalDirectorLiveChatPagesVisitCount: number;
  totalDirectorClosedChatPagesVisitCount: number;
  entries: ManagerStatsEntry[];
  groupedByAttributes: Attribute;
}

export interface ManagerStatsResponse {
  resultsGroupedByAttributeTypes: ManagerStatsGroupResult[];
  averageDirectorPagesVisitCountPerUser: number;
  averageDirectorLiveChatPagesVisitCountPerUser: number;
  averageDirectorClosedChatPagesVisitCountPerUser: number;
}

export interface CoachingSessionStatsEntry {
  coachingSessionsSubmitted: number;
  averageCoachingSessionsSubmittedPerUser: number;
  intervalStart: Dayjs;
}

export interface CoachingSessionStatsGroupResult {
  coachingSessionsSubmitted: number;
  averageCoachingSessionsSubmittedPerUser: number;
  entries: CoachingSessionStatsEntry[];
  groupedByAttributes: Attribute;
}

export interface CoachingSessionStatsResponse {
  coachingSessionsSubmitted: number;
  averageCoachingSessionsSubmittedPerUser: number;
  resultsGroupedByAttributeTypes: CoachingSessionStatsGroupResult[];
}

export interface ScorecardStatsResponse {
  scorecardCompleted: number;
  averageScorecardCompletedPerUser: number;
  resultsGroupedByAttributeTypes: ScorecardStatsGroupResult[];
}

export interface ScorecardStatsGroupResult {
  scorecardCompleted: number;
  averageScorecardCompletedPerUser: number;
  entries: ScorecardStatsEntry[];
  groupedByAttributes: Attribute;
}

export interface ScorecardStatsEntry {
  scorecardCompleted: number;
  averageScorecardCompletedPerUser: number;
  intervalStart: Dayjs;
}

export interface ConversationDurationBucketsResponse {
  conversationDurationBuckets: ConversationDurationBucket[];
}

export interface ConversationDurationBucket {
  id: string;
  lowerBound: number;
  upperBound: number;
}

export interface XdxOpportunity {
  id: number;
  /**
   * Deprecated: This field is added for migration purposes.
   */
  legacyMessageId?: number;
  messageName: string;
  messageTime: Dayjs;
}

export interface XdxInfo {
  intent: Intent;
  opportunities: XdxOpportunity[];
}

export interface RetrieveConversationsConversation {
  platformConversationId: string;
  legacyChatId: number;
  startTime: Dayjs;
  name: string;
}

export interface RetrieveConversationsResult {
  conversation: RetrieveConversationsConversation;
  /**
   * Deprecated and replaced with sdxMomentAnnotations (moments and actions migration)
   */
  sdxInfos: XdxInfo[];
  /**
   * Deprecated and replaced with ddxMomentAnnotations (moments and actions migration)
   */
  ddxInfos: XdxInfo[];
  sdxMomentAnnotations: MomentAnnotation[];
  ddxMomentAnnotations: MomentAnnotation[];
  sdxActionAnnotations: ActionAnnotation[];
}

export interface ClosedConversationsResponse {
  conversations: Conversation[];
  totalConversationCount: number;
  nextPageToken?: string;
}

/**
 * This is an Insights internal model to wrap different type of moments into a single moment
 * TopicMoment: General data about the Topic
 * includedKeywordMoments: (Optional) Keyword Moment where data of keyword moment data (if matched) will be included for CI Results
 * excludeKeywordMoments: (Optional) Moment of type keyword where the data of keyword moment data will be excluded for CI Results
 *  clusterMoments: List of moments of type topic cluster which get included in the CI Result
 */
export interface InsightsTopic {
  topicMoment: Moment;
  includedKeywordMoment?: Moment;
  excludedKeywordMoment?: Moment;
  clusterMoments: Moment[];
}

export interface QAScoreStatsResponse {
  qaScoreResult: QAScoreResult;
  /**
   * This is used to track the request for the score stats.
   */
  chapterId?: string;
}

export interface QAScoreResult {
  scores: QAScore[];
  averageQaScore?: number;
  totalConversationCount?: number;
  totalScorecardCount?: number;
}

export interface QAScore {
  score?: number;
  weightSum: number;
  totalConversationCount?: number;
  groupedBy?: QAScoreGroupBy;
  totalScorecardCount?: number;
}

export interface QAScoreGroupBy {
  intervalStart?: Dayjs;
  team?: Team;
  /**
   * Read from groupMemberships where isRoot === false && isInherited === false
   */
  directGroupMembership?: GroupMembership;
  group?: Group;
  user?: User;
  criterionId?: string;
  /**
   * Bare id of the scorecard template the criterion belongs to. Set when grouping by criterion.
   * Criterion ids are not unique across templates, so a criterion is identified by the
   * (scorecardTemplateId, criterionId) pair.
   */
  scorecardTemplateId?: string;
  agentTier?: AgentTier;
  quintileRank?: QuintileRank;
}

export interface QACriterionInfo {
  notApplicable: boolean;
  numericValue: number;
  percentageValue: number;
  maxValue: number;
  criterionId: string;
  scorecardTemplateId: string;
  scorecardTemplateRevision: string;
  aiValue: number;
  /**
   * Populated only when RetrieveQAConversationsRequest.includeComments=true.
   * Comment specific to this criterion.
   */
  comment?: string;
}

export interface QAConversationsResponse {
  conversationInfos: QAConversationInfo[];
  nextPageToken?: string;
}

export interface QAConversationInfo {
  conversation?: Pick<Conversation, 'platformInfo' | 'name' | 'startTime' | 'endTime' | 'legacyChatId'>;
  totalPercentage: number;
  criteriaInfo: QACriterionInfo[];
  agent: User;
  scorecardId: string;
  scorecardTemplateId?: string;
  scorecardTemplateRevision?: string;
}

export interface AnnotatedTimeSeriesDataPoint {
  intervalStart: Dayjs;

  intervalEnd: Dayjs;
  /**
   * The value during the time range.
   * value = value_numerator / value_denominator
   **/
  value: number;
  /**
   * The lower bound of the predicted value during the time range.
   **/
  predictedRangeLower: number;
  /**
   * The upper bound of the predicted value during the time range.
   **/
  predictedRangeUpper: number;
  /**
   * A single score to indicate how anomalous the data point is.
   * 0 means that it's within the predicted range hence not anomalous.
   * The higher the value the more significant the anomaly.
   **/
  anomalyScore: number;
  /**
   * If the value is a relative count, then this is the numerator of the value.
   * For example if quantity_type is QUANTITY_TYPE_RELATIVE_CONVERSATION_COUNT,
   * then value_numerator is the number of conversations in the group in the time range.
   * If the value is an absolute count then value_numerator equals value.
   **/
  valueNumerator: number;
  /**
   * If the value is a relative count, then this is the denominator of the value.
   * For example if quantity_type is QUANTITY_TYPE_RELATIVE_CONVERSATION_COUNT,
   * then value_denominator is the total number of conversations in the time range.
   * If the value is an absolute count then value_denominator equals 1.
   **/
  valueDenominator: number;
  /**
   * The y axis value of the trend line at this data point.
   **/
  trendLineValue: number;
  /**
   * The predicted value during the time range.
   **/
  predictedValue: number;
}

export interface TrendInfo {
  /**
   * The time range of the trend, always a multiple of weeks.
   **/
  trendTimeRange: {
    intervalStart: Dayjs;
    intervalEnd: Dayjs;
  };
  /**
   * Absolute change in trend line value between trend_time_range.start_timestamp
   * and trend_time_range.end_timestamp. This is the value used to prioritize trends.
   **/
  valueChange: number;
}

export interface AnnotatedTimeSeries {
  attribute: Attribute;
  /**
   * The maximum anomaly score found in the time range used to search for anomalies.
   **/
  maxAnomalyScore: number;
  /**
   * The slope of the trend in the time range used to compute the trend.
   * Assumes that the trend is linear after accounting for weekly variations and similar,
   * so value ~= slope_of_trend * x + constant, where x is the day number if frequency is DAILY,
   * week number if frequency is WEEKLY, etc.
   **/
  slopeOfTrend: number;

  dataPoints: AnnotatedTimeSeriesDataPoint[];

  /**
   * Information about the detected trend
   */
  trendInfo: TrendInfo;
}

export interface AnnotatedTimeSeriesResponse {
  annotatedTimeSeries: AnnotatedTimeSeries[];
}

export interface OutcomeStats {
  /**
   * The number of conversations that have outcome data.
   **/
  count: number;
  /**
   * The average of the outcome data, i.e. the average sale amount.
   **/
  mean: number;
  /**
   * The start of the confidence interval of the mean.
   **/
  confidenceIntervalStart: number;
  /**
   * The end of the confidence interval of the mean.
   **/
  confidenceIntervalEnd: number;
}

export interface AttributeOutcomeStats {
  attribute: Attribute;
  /**
   * The count of conversation with the attribute.
   **/
  conversationCount: number;
  /**
   * The outcome stats associated with the attribute.
   **/
  outcomeStats?: OutcomeStats;
  /**
   * When it's a behavior attribute, returns the adherence of the behavior.
   **/
  behaviorAdherence?: number;

  agentCount: number;
}

export interface AgentTierOutcomeStats {
  /**
   * The agent tier.
   **/
  agentTier: AgentTier;
  /**
   * The count of agents in the tier.
   **/
  agentCount: number;

  /**
   * The total conversation count in the tier.
   **/
  conversationCount: number;
  /**
   * The average adherence of the agent tier.
   **/
  behaviorAdherence: number;
  /**
   * The outcome stats of the agent tier.
   **/
  outcomeStats: OutcomeStats;
  /**
   * The behavior moment for the agent tier.
   **/
  behaviorMoment?: Moment;
}

export interface ConversationOutcomeStats {
  /**
   * The outcome data for each attribute.
   **/
  attributeOutcomeStats: AttributeOutcomeStats[];
  /**
   * When `group_by_attributes` is `ATTRIBUTE_TYPE_AGENT`, returns
   * the slope of the correlation line between the behavior adherence and outcome.
   **/
  adherenceOutcomeSlope?: number;
  /**
   * the intercept of the correlation line between the behavior adherence and outcome.
   **/
  adherenceOutcomeIntercept?: number;

  /**
   * When `group_by_attributes` is `ATTRIBUTE_TYPE_AGENT`, returns the
   * outcome stats for each tier of the agents.
   **/
  agentTierOutcomeStats: AgentTierOutcomeStats[];
}

/**
 * A wrapper message that contains the conversation information and other QA
 * info of the conversation.
 **/
export interface ManualQAConversationInfo {
  /**
   * A conversation object where the score happened. Only the once necessary for
   * the FE.
   **/
  conversation?: Conversation;
  /**
   * The agent of the conversation.
   **/
  agent?: User;
  /**
   * The QA analyst who evaluated the conversation.
   **/
  qaAnalyst?: User;

  /**
   * The submitted scorecard score for the specified scorecard template.
   **/
  perfScore?: number;
  /**
   * The user who calibrated the conversation.
   **/
  calibrator?: User;
  /**
   * The consistency score when the scorecard for the conversation is calibrated.
   **/
  consistencyScore?: number;
  /**
   * The scorecard name.
   */
  scorecardName: string;
  /**
   * Process id for process scorecards.
   */
  processId?: string;
  /**
   * Process interaction time for process scorecards.
   */
  processInteractionTime?: Dayjs;

  /**
   * Message names that are suggested to score. Should be used for email conversations for templates that
   * can be scored per message. Otherwise, agent name will be used.
   **/
  messageNames: string[];
}

/**
 * The stats for the manual QA workflows.
 **/
export interface ManualQAStats {
  /**
   * Attribute of the result.
   **/
  attribute: Attribute | null;
  /**
   * The list of the evaluated conversations.
   **/
  evaluatedConversations: ManualQAConversationInfo[];
  /**
   * The number of the evaluated agents.
   * Deprecated
   **/
  evaluatedAgentsCount?: number;
  /**
   * The average performance score given.
   **/
  avgPerfScore?: number;
  /**
   * The completion rate.
   **/
  completionRate?: number;
  /**
   * The number of criteria overwritten by the QA analyst.
   **/
  overwrittenCriteriaCount?: number;
  /**
   * The number of skipped suggested conversations.
   */
  skipSuggestedConversationCount?: number;
  /**
   * The list of the conversations that have criteria overwritten by the QA
   * analyst.
   **/
  criteriaOverwrittenConversations: ManualQAConversationInfo[];
  /**
   * The list of the conversations that auto failed.
   * Deprecated
   **/
  autoFailedConversations: ManualQAConversationInfo[];
  /**
   * The list of the conversations that have comments from the QA analyst.
   **/
  commentedConversations: ManualQAConversationInfo[];
  /**
   * The number of assigned conversations for the QA analyst.
   **/
  assignedConversationCount?: number;
  /**
   * The number of evaluated conversations for the QA analyst.
   **/
  evaluatedConversationCount?: number;
  /**
   * Shows the progress status of this evaluation period.
   * It's set only when the selected period is current.
   **/
  evaluationPeriodProgressStatus?: EvaluationPeriodProgressStatus;

  /**
   * The average consistency score calibrated.
   **/
  avgConsistencyScore?: number;
  /**
   * The list of the conversations that have been calibrated.
   **/
  calibratedConversation: ManualQAConversationInfo[];
  /**
   * The list of the conversations that have been calibrated.
   **/
  acknowledgedConversation: ManualQAConversationInfo[];
  /**
   * The list of the conversations that have at least one published scorecard.
   **/
  publishedConversations: ManualQAConversationInfo[];
}

/**
 * The response of the RetrieveManualQAStats RPC.
 **/
export type ManualQAStatsResponse = {
  /**
   * The attributed qa stats.
   **/
  manualQaStats: ManualQAStats[];
};

export type ManualQAProgress = {
  attribute: Attribute;
  lastEvaluatedTime?: Dayjs;
  lastQaAnalyst: User | undefined;
  assignedConversationCount: number;
  evaluatedConversationCount: number;
  evaluatedConversations: ManualQAConversationInfo[];
  agentHandledConversationCount?: number;
};

export type ManualQAProgressResponse = {
  manualQaProgresses: ManualQAProgress[];
};

export interface SuggestManualQAConversationsResponse {
  suggestedConversations: ManualQAConversationInfo[];
}

export interface TeamUsecaseConversationCount {
  team?: string;
  usecase?: string;
  count: number;
}

export type TeamUsecaseConversationMappingResponse = {
  teamUseCaseMappings: TeamUsecaseConversationCount[];
};

export type CoachingEfficiencyStats = Omit<CrestaV1RetrieveCoachingEfficiencyStatsResponse, 'attribute'> & {
  attribute?: Attribute;
  outcomeEfficiency?: number;
};

export type RetrieveCoachingEfficiencyStatsResponse = {
  coachingEfficiencyStats?: CoachingEfficiencyStats[];
  totalCoachingEfficiency?: CoachingEfficiencyStats;
};

export type AnalyticsChartEvaluation =
  | {
      analyticsResults: AnalyticsResult[] | undefined;
      updateTimestamp?: string;
    }
  | undefined;

export type AnalyticsChartExamplesResponse = RetrieveAnalyticsChartExamplesResponse | undefined;
