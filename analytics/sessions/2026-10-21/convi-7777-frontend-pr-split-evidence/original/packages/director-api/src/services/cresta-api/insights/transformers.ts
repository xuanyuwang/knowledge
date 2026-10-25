import { first, omit } from 'lodash-es';

import type {
  ChatBotConversationStatsResult,
  ConversationDurationBucket as ConversationDurationBucketFromApi,
  AgentTierOutcomeStats as CrestaV1AgentTierOutcomeStats,
  AnnotatedTimeSeries as CrestaV1AnnotatedTimeSeries,
  AnnotatedTimeSeriesDataPoint as CrestaV1AnnotatedTimeSeriesDataPoint,
  Attribute as CrestaV1Attribute,
  AttributeOutcomeStats as CrestaV1AttributeOutcomeStats,
  CoachingEfficiencyStats as CrestaV1CoachingEfficiencyStats,
  ConversationMessageResultMessageResult as CrestaV1ConversationMessageResultMessageResult,
  ConversationOutcomeValue as CrestaV1ConversationOutcomeValue,
  ConversationResultConversationInfoIntentInfo as CrestaV1ConversationResultConversationInfoIntentInfo,
  ConversationStats as CrestaV1ConversationStats,
  RetrieveScorecardStatsResponse as CrestaV1CrestaV1RetrieveReceptivitiesResponse,
  GenerateInsightsTopicResponse as CrestaV1GenerateInsightsTopicResponse,
  ListConversationDurationBucketsResponse as CrestaV1ListConversationDurationBucketsResponse,
  ManualQAConversationInfo as CrestaV1ManualQAConversationInfo,
  ManualQAProgress as CrestaV1ManualQAProgress,
  ManualQAStats as CrestaV1ManualQAStats,
  MetadataValue as CrestaV1MetadataValue,
  MetadataValueAttribute as CrestaV1MetadataValueAttribute,
  ModifyInsightsTopicResponse as CrestaV1ModifyInsightsTopicResponse,
  Moment as CrestaV1Moment,
  MomentGroup as CrestaV1MomentGroup,
  NumericBin as CrestaV1NumericBin,
  OutcomeStats as CrestaV1OutcomeStats,
  RetrieveAdherencesResponse as CrestaV1RetrieveAdherencesResponse,
  RetrieveAgentStatsResponse as CrestaV1RetrieveAgentStatsResponse,
  RetrieveAnnotatedTimeSeriesResponse as CrestaV1RetrieveAnnotatedTimeSeriesResponse,
  RetrieveAssistanceStatsResponse as CrestaV1RetrieveAssistanceStatsResponse,
  RetrieveChatBotConversationStatsResponse as CrestaV1RetrieveChatBotConversationStatsResponse,
  RetrieveChatBotSessionStatsResponse as CrestaV1RetrieveChatBotSessionStatsResponse,
  RetrieveClosedConversationsResponse as CrestaV1RetrieveClosedConversationsResponse,
  RetrieveCoachingEfficiencyStatsResponse as CrestaV1RetrieveCoachingEfficiencyStatsResponse,
  RetrieveCoachingSessionStatsResponse as CrestaV1RetrieveCoachingSessionStatsResponse,
  RetrieveCommentStatsResponse as CrestaV1RetrieveCommentStatsResponse,
  RetrieveConversationMessagesResponse as CrestaV1RetrieveConversationMessagesResponse,
  RetrieveConversationOutcomeStatsResponse as CrestaV1RetrieveConversationOutcomeStatsResponse,
  RetrieveConversationStatsResponse as CrestaV1RetrieveConversationStatsResponse,
  RetrieveGuidedWorkflowStatsResponse as CrestaV1RetrieveGuidedWorkflowStatsResponse,
  RetrieveHintStatsResponse as CrestaV1RetrieveHintStatsResponse,
  RetrieveInsightsTopicsResponse as CrestaV1RetrieveInsightsTopicsResponse,
  RetrieveKeywordsStatsResponse as CrestaV1RetrieveKeywordsStatsResponse,
  RetrieveKnowledgeBaseStatsResponse as CrestaV1RetrieveKnowledgeBaseStatsResponse,
  RetrieveLiveAssistStatsResponse as CrestaV1RetrieveLiveAssistStatsResponse,
  RetrieveManagerStatsResponse as CrestaV1RetrieveManagerStatsResponse,
  RetrieveMetadataValuesResponse as CrestaV1RetrieveMetadataValuesResponse,
  RetrieveNoteTakingStatsResponse as CrestaV1RetrieveNoteTakingStatsResponse,
  RetrieveReceptivitiesResponse as CrestaV1RetrieveReceptivitiesResponse,
  RetrieveSmartComposeStatsResponse as CrestaV1RetrieveSmartComposeStatsResponse,
  RetrieveSuggestionStatsResponse as CrestaV1RetrieveSuggestionStatsResponse,
  RetrieveSummarizationStatsResponse as CrestaV1RetrieveSummarizationStatsResponse,
  RetrieveTeamUsecaseConversationMappingsResponse as CrestaV1RetrieveTeamUsecaseConversationMappingsResponse,
  SuggestManualQAConversationsResponse as CrestaV1SuggestManualQAConversationsResponse,
  TeamUsecaseConversationMapping as CrestaV1TeamUsecaseConversationMapping,
  team_pb as CrestaV1UserTeam,
  user_pb as CrestaV1UserUser,
  OutcomeValueAttribute,
  RetrieveConversationMessageStatsResponse,
  RetrieveKnowledgeAssistStatsResponse,
  RetrieveManualQAProgressResponse,
  RetrieveManualQAStatsResponse,
} from '@cresta/web-client';
import { AgentTier } from '@cresta/web-client';
import type {
  RetrieveLlmAnnotationValuesResponse as CrestaV1RetrieveLlmAnnotationValuesResponse,
  RetrieveCustomerSnapshotStatsResponse,
} from '@cresta/web-client/v1/analytics';

import type { User } from '../../../types';
import { isNonNaScore } from '../../../utils';
import { dayjsIfTruthy } from '../../shared';
import { transformAction } from '../action';
import { transformApiBehaviorToModel } from '../behavior';
import { transformConversation, transformConversationMessage } from '../conversation';
import { transformApiIntentToModel } from '../intents';
import type {
  ConversationMetadataMoment,
  ConversationMetadataMomentPayload,
  ConversationMetadataNumericBin,
  ConversationMetadataOrOutcomeValue,
  ConversationOutcomeMoment,
  ConversationOutcomeMomentPayload,
  ConversationOutcomeOutcomeValue,
  Moment,
} from '../moment';
import {
  transformMatchedMoment,
  transformMetadataOrOutcomeValue,
  transformMoment,
  transformMomentAnnotation,
  transformNumericBin,
  transformOutcomeValues,
} from '../moment';
import { transformAttributePolicy } from '../policy';
import { transformApiGroupToModel, transformApiTeamToModel, transformApiUserToModel } from '../user';

import type {
  AdherenceEntry,
  AdherenceStatsGroupResult,
  AdherenceStatsResponse,
  AgentStatsEntry,
  AgentStatsGroupResult,
  AgentStatsResponse,
  AgentTierOutcomeStats,
  AnnotatedTimeSeries,
  AnnotatedTimeSeriesDataPoint,
  AnnotatedTimeSeriesResponse,
  AssistanceStatsEntry,
  AssistanceStatsGroupResult,
  AssistanceStatsResponse,
  Attribute,
  AttributeOutcomeStats,
  ChatbotConversationStatsGroupedResult,
  ChatbotConversationStatsResponse,
  ChatbotSessionStatsResponse,
  ClosedConversationsResponse,
  CoachingEfficiencyStats,
  CoachingSessionStatsResponse,
  CommentStatsEntry,
  CommentStatsGroupResult,
  CommentStatsResponse,
  ConversationAssistanceUsageStatsApiResponse,
  ConversationAssistanceUsageStatsResponse,
  ConversationDurationBucket,
  ConversationDurationBucketsResponse,
  ConversationMessageResult,
  ConversationMessageResultMessageResult,
  ConversationMessageStats,
  ConversationMessageStatsResponse,
  ConversationMessagesResponse,
  ConversationOutcomeStats,
  ConversationStatsEntry,
  ConversationStatsGroupResult,
  ConversationStatsResponse,
  CustomerSnapshotStatsEntry,
  CustomerSnapshotStatsGroupResult,
  CustomerSnapshotStatsResponse,
  GuidedWorkflowStatsEntry,
  GuidedWorkflowStatsGroupResult,
  GuidedWorkflowStatsResponse,
  HintEngagementEntry,
  HintEngagementGroupResult,
  HintEngagementResponse,
  HintStatsEntry,
  HintStatsGroupResult,
  HintStatsResponse,
  InsightsTopic,
  KeywordsStats,
  KeywordsStatsResponse,
  KnowledgeAssistStatsEntry,
  KnowledgeAssistStatsGroupResult,
  KnowledgeAssistStatsResponse,
  KnowledgeBaseStatsEntry,
  KnowledgeBaseStatsGroupResult,
  KnowledgeBaseStatsResponse,
  LiveAssistStatsEntry,
  LiveAssistStatsGroupResult,
  LiveAssistStatsResponse,
  LlmAnnotationValue,
  LlmAnnotationValuesResponse,
  ManagerStatsEntry,
  ManagerStatsGroupResult,
  ManagerStatsResponse,
  ManualQAConversationInfo,
  ManualQAProgress,
  ManualQAProgressResponse,
  ManualQAStats,
  ManualQAStatsResponse,
  MetadataValueAttribute,
  MetadataValuesResponse,
  MomentGroup,
  MomentResult,
  NoteTakingStatsEntry,
  NoteTakingStatsGroupResult,
  NoteTakingStatsResponse,
  OutcomeStats,
  RetrieveCoachingEfficiencyStatsResponse,
  RetrieveKeywordsResult,
  ScorecardStatsEntry,
  ScorecardStatsGroupResult,
  ScorecardStatsResponse,
  SmartComposeStatsEntry,
  SmartComposeStatsGroupResult,
  SmartComposeStatsResponse,
  SuggestionStatsEntry,
  SuggestionStatsGroupResult,
  SuggestionStatsResponse,
  SuggestManualQAConversationsResponse,
  SummarizationStatsEntry,
  SummarizationStatsGroupResult,
  SummarizationStatsResponse,
  TeamUsecaseConversationCount,
  TeamUsecaseConversationMappingResponse,
  XdxInfo,
} from './apiTypes';
import { transformEndTimeStamp, transformStartTimestamp, transformTimeRange, transformTimeStamp } from './utils';

function transformAverageHandleTime(averageHandleTime: string | undefined): number {
  return averageHandleTime?.endsWith('s') ? Number.parseInt(averageHandleTime.slice(0, -1)) : 0;
}

function transformConversationStatsEntry(entry: CrestaV1ConversationStats): ConversationStatsEntry {
  return {
    averageHandleTime: transformAverageHandleTime(entry.averageHandleTime),
    intervalStart: transformStartTimestamp(entry.timeRange),
    totalConversationCount: entry.totalConversationCount || 0,
    averageConversationCountPerFrequencyPerUser: entry.averageConversationCountPerFrequencyPerUser || 0,
  };
}

export function transformConversationAssistanceUsageStatsResponse(
  response: ConversationAssistanceUsageStatsApiResponse
): ConversationAssistanceUsageStatsResponse {
  return {
    resultGroups: (response.conversationAssistanceUsageStatsResults || []).map((result) => {
      const totalConversationCount = result.totalConversationCount || 0;
      const conversationsWithAssistanceUsedCount = result.conversationsWithAssistanceUsedCount || 0;
      return {
        groupedByAttributes: transformAttributes(result.attribute),
        totalConversationCount,
        conversationsWithAssistanceUsedCount,
        conversationsWithAssistanceUsedPercent: totalConversationCount
          ? (conversationsWithAssistanceUsedCount / totalConversationCount) * 100
          : undefined,
      };
    }),
  };
}

export function transformConversationStatsResponse(
  response: CrestaV1RetrieveConversationStatsResponse
): ConversationStatsResponse {
  return {
    totalConversationCount: response.totalConversationCount || 0,
    averageConversationCountPerFrequency: response.averageConversationCountPerFrequency || 0,
    averageConversationCountPerFrequencyPerUser: response.averageConversationCountPerFrequencyPerUser || 0,
    averageHandleTime: transformAverageHandleTime(response.averageHandleTime),
    resultGroups:
      response.conversationStatsResults?.map((groupResult): ConversationStatsGroupResult => {
        return {
          averageConversationCountPerFrequencyPerUser: groupResult.averageConversationCountPerFrequencyPerUser || 0,
          averageHandleTime: transformAverageHandleTime(groupResult.averageHandleTime),
          totalConversationCount: groupResult.totalConversationCount || 0,
          entries: groupResult.conversationStats?.map((v) => transformConversationStatsEntry(v)) || [],
          groupedByAttributes: transformAttributes(groupResult.attribute),
        };
      }) || [],
  };
}

export function transformAgentStatsResponse(response: CrestaV1RetrieveAgentStatsResponse): AgentStatsResponse {
  const resultsByAttributeType =
    response.agentStatsResults?.map((groupResult): AgentStatsGroupResult => {
      const entries =
        groupResult.agentStats?.map((agentStats): AgentStatsEntry => {
          return {
            activeAgentCount: agentStats.activeAgentCount || 0,
            averageActiveAgentCountPerFrequency: agentStats.averageActiveAgentCountPerFrequency || 0,
            averageActiveDayCountPerUser: agentStats.averageActiveDayCountPerUser || 0,
            intervalStart: transformStartTimestamp(agentStats.timeRange),
          };
        }) || [];

      return {
        activeAgentCount: groupResult.activeAgentCount || 0,
        averageActiveAgentCountPerFrequency: groupResult.averageActiveAgentCountPerFrequency || 0,
        averageActiveDayCountPerUser: groupResult.averageActiveDayCountPerUser || 0,
        entries: entries,
        groupedByAttributes: transformAttributes(groupResult.attribute),
      };
    }) || [];
  return {
    resultsGroupedByAttributeTypes: resultsByAttributeType,
    activeAgentCount: response.activeAgentCount || 0,
    averageActiveDayCountPerUser: response.averageActiveDayCountPerUser || 0,
    averageActiveAgentCountPerFrequency: response.averageActiveAgentCountPerFrequency || 0,
  };
}

export function transformManagerStatsResponse(response: CrestaV1RetrieveManagerStatsResponse): ManagerStatsResponse {
  const resultsByAttributeType =
    response.managerStatsResults?.map((groupResult): ManagerStatsGroupResult => {
      const entries =
        groupResult.managerStats?.map((managerStats): ManagerStatsEntry => {
          const aggregated = managerStats.aggregatedManagerStats;
          return {
            intervalStart: transformStartTimestamp(managerStats.timeRange),
            totalDirectorPagesVisitCount: aggregated?.totalDirectorPagesVisitCount || 0,
            totalDirectorLiveChatPagesVisitCount: aggregated?.totalDirectorLiveConversationPagesVisitCount || 0,
            totalDirectorClosedChatPagesVisitCount: aggregated?.totalDirectorClosedConversationPagesVisitCount || 0,
          };
        }) || [];

      const aggregated = groupResult.aggregatedManagerStats;
      return {
        totalDirectorPagesVisitCount: aggregated?.totalDirectorPagesVisitCount || 0,
        totalDirectorLiveChatPagesVisitCount: aggregated?.totalDirectorLiveConversationPagesVisitCount || 0,
        totalDirectorClosedChatPagesVisitCount: aggregated?.totalDirectorClosedConversationPagesVisitCount || 0,
        entries: entries,
        groupedByAttributes: transformAttributes(groupResult.attribute),
      };
    }) || [];
  const aggregated = response.aggregatedManagerStats;
  return {
    resultsGroupedByAttributeTypes: resultsByAttributeType,
    averageDirectorPagesVisitCountPerUser: aggregated?.averageDirectorPagesVisitCountPerUser || 0,
    averageDirectorLiveChatPagesVisitCountPerUser:
      aggregated?.averageDirectorLiveConversationPagesVisitCountPerUser || 0,
    averageDirectorClosedChatPagesVisitCountPerUser:
      aggregated?.averageDirectorClosedConversationPagesVisitCountPerUser || 0,
  };
}

export function transformAssistanceStatsResponse(
  response: CrestaV1RetrieveAssistanceStatsResponse
): AssistanceStatsResponse {
  const resultsByAttributeType =
    response.assistanceStatsResults?.map((groupResult): AssistanceStatsGroupResult => {
      const hintFollowedStats: AssistanceStatsEntry[] =
        groupResult.hintStats?.map((entry) => ({
          intervalStart: transformStartTimestamp(entry.timeRange),
          totalAssistanceUsedCount: entry.totalHintFollowedCount || 0,
          totalAssistanceSentCount: entry.totalHintSentCount || 0,
          averagePerConversation: entry.averageHintFollowedCountPerConversation || 0,
          averagePerUser: entry.averageHintFollowedCountPerFrequencyPerUser || 0,
          totalConversationCount: entry.totalConversationCount || 0,
        })) || [];

      const kbSearchStats: AssistanceStatsEntry[] =
        groupResult.kbSearchStats?.map((entry) => ({
          intervalStart: transformStartTimestamp(entry.timeRange),
          totalAssistanceUsedCount: entry.totalKbSearchCount || 0,
          totalAssistanceSentCount: entry.totalKbSearchCount || 0,
          averagePerConversation: entry.averageKbSearchCountPerConversation || 0,
          averagePerUser: entry.averageKbSearchCountPerFrequencyPerUser || 0,
          totalConversationCount: entry.totalConversationCount || 0,
        })) || [];

      const autoSummarizationStats: AssistanceStatsEntry[] =
        groupResult.autoSummarizationStats?.map((entry) => ({
          intervalStart: transformStartTimestamp(entry.timeRange),
          totalAssistanceUsedCount: entry.totalAutoSummarizationUsedCount || 0,
          totalAssistanceSentCount: entry.totalConversationWithAutoSummarizationCount || 0,
          averagePerConversation: entry.averageAutoSummarizationUsedCountPerConversation || 0,
          averagePerUser: entry.averageAutoSummarizationUsedCountPerFrequencyPerUser || 0,
          totalConversationCount: entry.totalConversationWithAutoSummarizationCount || 0,
        })) || [];

      // Note: averagePerConversation is not available but it is currently never used
      const autoSummarizationRatioStats: AssistanceStatsEntry[] =
        groupResult.autoSummarizationStats
          // Exclude columns where the denominator (totalConversationWithAutoSummarizationCount) is zero
          ?.filter((entry) => (entry.totalConversationWithAutoSummarizationCount || 0) > 0)
          .map((entry) => ({
            intervalStart: transformStartTimestamp(entry.timeRange),
            totalAssistanceUsedCount: entry.averageAutoSummarizationUsedRatioPerFrequencyPerUser || 0,
            totalAssistanceSentCount: entry.averageConversationWithAutoSummarizationCountPerFrequencyPerUser || 0,
            averagePerConversation: 0,
            averagePerUser: entry.averageAutoSummarizationUsedRatioPerFrequencyPerUser || 0,
            totalConversationCount: entry.totalConversationWithAutoSummarizationCount || 0,
          })) || [];

      const autoSummarizationConversationStats: AssistanceStatsEntry[] =
        groupResult.autoSummarizationStats?.map((entry) => ({
          intervalStart: transformStartTimestamp(entry.timeRange),
          totalAssistanceUsedCount: entry.totalConversationWithAutoSummarizationCount || 0,
          totalAssistanceSentCount: entry.totalConversationWithAutoSummarizationCount || 0,
          averagePerConversation: 0,
          averagePerUser: entry.averageConversationWithAutoSummarizationCountPerFrequencyPerUser || 0,
          totalConversationCount: entry.totalConversationWithAutoSummarizationCount || 0,
        })) || [];

      const suggestionsStats: AssistanceStatsEntry[] =
        groupResult.suggestionStats?.map((entry) => ({
          intervalStart: transformStartTimestamp(entry.timeRange),
          totalAssistanceUsedCount: entry.totalSuggestionUsedCount || 0,
          totalAssistanceSentCount: entry.totalConversationCount || 0,
          averagePerConversation: entry.averageSuggestionUsedCountPerConversation || 0,
          averagePerUser: entry.averageSuggestionUsedCountPerFrequencyPerUser || 0,
          totalConversationCount: entry.totalConversationCount || 0,
        })) || [];

      const smartComposeStats: AssistanceStatsEntry[] =
        groupResult.smartComposeStats?.map((entry) => ({
          intervalStart: transformStartTimestamp(entry.timeRange),
          totalAssistanceUsedCount: entry.totalSmartComposeUsedCount || 0,
          totalAssistanceSentCount: entry.totalSmartComposeUsedCount || 0,
          averagePerConversation: entry.averageSmartComposeUsedCountPerConversation || 0,
          averagePerUser: entry.averageSmartComposeUsedCountPerFrequencyPerUser || 0,
        })) || [];

      const noteStats: AssistanceStatsEntry[] =
        groupResult.automaticNoteStats?.map(
          (entry): AssistanceStatsEntry => ({
            intervalStart: transformStartTimestamp(entry.timeRange),
            totalAssistanceUsedCount: entry.totalNoteCount || 0,
            totalAssistanceSentCount: entry.totalNoteCount || 0,
            averagePerConversation: entry.averageNoteCountPerConversation || 0,
            averagePerUser: entry.averageNoteCountPerFrequencyPerUser || 0,
          })
        ) || [];

      const guidedWorkflowStats: AssistanceStatsEntry[] =
        groupResult.gwStats?.map((entry) => ({
          intervalStart: transformStartTimestamp(entry.timeRange),
          totalAssistanceUsedCount: entry.totalGuidedWorkflowClickCount || 0,
          totalAssistanceSentCount: entry.totalGuidedWorkflowClickCount || 0,
          averagePerConversation: entry.averageGuidedWorkflowClickCountPerConversation || 0,
          averagePerUser: entry.averageGuidedWorkflowClickCountPerFrequencyPerUser || 0,
          totalConversationCount: entry.totalConversationCount || 0,
        })) || [];

      const totalAssistanceStats: AssistanceStatsEntry[] =
        groupResult.assistanceStats?.map((entry) => ({
          intervalStart: transformStartTimestamp(entry.timeRange),
          totalAssistanceUsedCount: entry.totalAssistanceUsedCount || 0,
          totalAssistanceSentCount: entry.totalAssistanceUsedCount || 0,
          averagePerConversation: entry.averageAssistanceUsedCountPerConversation || 0,
          averagePerUser: entry.averageAssistanceUsedCountPerFrequencyPerUser || 0,
        })) || [];

      return {
        groupedByAttributes: transformAttributes(groupResult.attribute),
        totalAssistanceCount: groupResult.totalAssistanceUsedCount || 0,
        totalHintsFollowedCount: groupResult.totalHintFollowedCount || 0,
        totalKbSearchCount: groupResult.totalKbSearchCount || 0,
        totalAutoSummarizationUsedCount: groupResult.totalAutoSummarizationUsedCount || 0,
        totalConversationWithAutoSummarizationCount: groupResult.totalConversationWithAutoSummarizationCount || 0,
        totalSuggestionCount: groupResult.totalSuggestionUsedCount || 0,
        totalSmartComposeCount: groupResult.totalSmartComposeUsedCount || 0,
        totalNoteCount: groupResult.totalNoteCount || 0,
        totalGwCount: groupResult.totalGuidedWorkflowClickCount || 0,
        averageAssistanceCountPerConv: groupResult.averageAssistanceUsedCountPerConversation || 0,
        averageHintsFollowedPerConv: groupResult.averageHintFollowedCountPerConversation || 0,
        averageKbSearchCountPerConv: groupResult.averageKbSearchCountPerConversation || 0,
        averageAutoSummarizationUsedCountPerConv: groupResult.averageAutoSummarizationUsedCountPerConversation || 0,
        averageSuggestionCountPerConv: groupResult.averageSuggestionUsedCountPerConversation || 0,
        averageSmartComposeCountPerConv: groupResult.averageSmartComposeUsedCountPerConversation || 0,
        averageNoteCountPerConv: groupResult.averageNoteCountPerConversation || 0,
        averageGwCountPerConv: groupResult.averageGuidedWorkflowClickCountPerConversation || 0,
        averageAssistanceCountPerFrequencyPerUser: groupResult.averageAssistanceUsedCountPerFrequencyPerUser || 0,
        averageHintsFollowedPerFrequencyPerUser: groupResult.averageHintFollowedCountPerFrequencyPerUser || 0,
        averageKbSearchCountPerFrequencyPerUser: groupResult.averageKbSearchCountPerFrequencyPerUser || 0,
        averageAutoSummarizationUsedCountPerFrequencyPerUser:
          groupResult.averageAutoSummarizationUsedCountPerFrequencyPerUser || 0,
        averageAutoSummarizationUsedRatioPerFrequencyPerUser:
          groupResult.averageAutoSummarizationUsedRatioPerFrequencyPerUser || 0,
        averageConversationWithAutoSummarizationCountPerFrequencyPerUser:
          groupResult.averageConversationWithAutoSummarizationCountPerFrequencyPerUser || 0,
        averageSuggestionCountPerFrequencyPerUser: groupResult.averageSuggestionUsedCountPerFrequencyPerUser || 0,
        averageSmartComposeCountPerFrequencyPerUser: groupResult.averageSmartComposeUsedCountPerFrequencyPerUser || 0,
        averageNoteCountPerFrequencyPerUser: groupResult.averageNoteCountPerFrequencyPerUser || 0,
        averageGwCountPerFrequencyPerUser: groupResult.averageGuidedWorkflowClickCountPerFrequencyPerUser || 0,
        hintFollowedStats,
        kbSearchStats,
        autoSummarizationStats,
        autoSummarizationRatioStats,
        autoSummarizationConversationStats,
        suggestionsStats,
        smartComposeStats,
        totalAssistanceStats,
        noteStats,
        guidedWorkflowStats: guidedWorkflowStats,

        // Email suggestions are not supported by old api.
        totalEmailPartialSuggestionCount: 0,
        totalEmailFullSuggestionCount: 0,
        averageEmailPartialSuggestionCountPerConv: 0,
        averageEmailFullSuggestionCountPerConv: 0,
        averageEmailPartialSuggestionCountPerFrequencyPerUser: 0,
        averageEmailFullSuggestionCountPerFrequencyPerUser: 0,
        emailPartialSuggestionsStats: [],
        emailFullSuggestionsStats: [],
      };
    }) || [];

  return {
    resultsGroupedByAttributeTypes: resultsByAttributeType,
    totalAutoSummarizationUsedCount: response.totalAutoSummarizationUsedCount || 0,
    totalConversationWithAutoSummarizationCount: response.totalConversationWithAutoSummarizationCount || 0,
    totalSuggestionCount: response.totalSuggestionUsedCount || 0,
    totalSmartComposeCount: response.totalSmartComposeUsedCount || 0,
    totalKbSearchCount: response.totalKbSearchCount || 0,
    totalHintsFollowedCount: response.totalHintFollowedCount || 0,
    totalAssistanceCount: response.totalAssistanceUsedCount || 0,
    totalNotesCount: response.totalNoteCount || 0,
    totalGwCount: response.totalGuidedWorkflowClickCount || 0,
    averageAutoSummarizationUsedCountPerFrequencyPerUser:
      response.averageAutoSummarizationUsedCountPerFrequencyPerUser || 0,
    averageAutoSummarizationUsedRatioPerFrequencyPerUser:
      response.averageAutoSummarizationUsedRatioPerFrequencyPerUser || 0,
    averageConversationWithAutoSummarizationCountPerFrequencyPerUser:
      response.averageConversationWithAutoSummarizationCountPerFrequencyPerUser || 0,
    averageSuggestionUsedCountPerFrequencyPerUser: response.averageSuggestionUsedCountPerFrequencyPerUser || 0,
    averageSmartComposeUsedCountPerFrequencyPerUser: response.averageSmartComposeUsedCountPerFrequencyPerUser || 0,
    averageKbSearchCountPerFrequencyPerUser: response.averageKbSearchCountPerFrequencyPerUser || 0,
    averageHintFollowedCountPerFrequencyPerUser: response.averageHintFollowedCountPerFrequencyPerUser || 0,
    averageNoteCountPerFrequencyPerUser: response.averageNoteCountPerFrequencyPerUser || 0,
    averageGwCountPerFrequencyPerUser: response.averageGuidedWorkflowClickCountPerFrequencyPerUser || 0,
    averageAssistanceUsedCountPerFrequencyPerUser: response.averageAssistanceUsedCountPerFrequencyPerUser || 0,

    totalEmailPartialSuggestionCount: 0,
    totalEmailFullSuggestionCount: 0,
    averageEmailPartialSuggestionUsedCountPerFrequencyPerUser: 0,
    averageEmailFullSuggestionUsedCountPerFrequencyPerUser: 0,
  };
}

export function transformSuggestionStatsResponse(
  response: CrestaV1RetrieveSuggestionStatsResponse
): SuggestionStatsResponse {
  const resultsGroupedByAttributeTypes: SuggestionStatsGroupResult[] =
    response.suggestionStatsResults?.map((groupResult) => {
      const suggestionStats: SuggestionStatsEntry[] =
        groupResult.suggestionStats?.map((entry) => ({
          intervalStart: transformStartTimestamp(entry.timeRange),
          totalConversationCount: entry.totalConversationCount || 0,
          totalSuggestionUsedCount: entry.totalSuggestionUsedCount || 0,
          averageSuggestionUsedCountPerConversation: entry.averageSuggestionUsedCountPerConversation || 0,
          averageSuggestionUsedCountPerFrequencyPerUser: entry.averageSuggestionUsedCountPerFrequencyPerUser || 0,
          totalEmailPartialSuggestionUsedCount: entry.totalEmailPartialSuggestionUsedCount || 0,
          averageEmailPartialSuggestionUsedCountPerConversation:
            entry.averageEmailPartialSuggestionUsedCountPerConversation || 0,
          averageEmailPartialSuggestionUsedCountPerFrequencyPerUser:
            entry.averageEmailPartialSuggestionUsedCountPerFrequencyPerUser || 0,
          totalEmailFullSuggestionUsedCount: entry.totalEmailFullSuggestionUsedCount || 0,
          averageEmailFullSuggestionUsedCountPerConversation:
            entry.averageEmailFullSuggestionUsedCountPerConversation || 0,
          averageEmailFullSuggestionUsedCountPerFrequencyPerUser:
            entry.averageEmailFullSuggestionUsedCountPerFrequencyPerUser || 0,
        })) || [];

      return {
        groupedByAttributes: transformAttributes(groupResult.attribute),
        suggestionStats,
        totalConversationCount: groupResult.totalConversationCount || 0,
        totalSuggestionUsedCount: groupResult.totalSuggestionUsedCount || 0,
        averageSuggestionUsedCountPerConversation: groupResult.averageSuggestionUsedCountPerConversation || 0,
        averageSuggestionUsedCountPerFrequencyPerUser: groupResult.averageSuggestionUsedCountPerFrequencyPerUser || 0,
        totalEmailPartialSuggestionUsedCount: groupResult.totalEmailPartialSuggestionUsedCount || 0,
        averageEmailPartialSuggestionUsedCountPerConversation:
          groupResult.averageEmailPartialSuggestionUsedCountPerConversation || 0,
        averageEmailPartialSuggestionUsedCountPerFrequencyPerUser:
          groupResult.averageEmailPartialSuggestionUsedCountPerFrequencyPerUser || 0,
        totalEmailFullSuggestionUsedCount: groupResult.totalEmailFullSuggestionUsedCount || 0,
        averageEmailFullSuggestionUsedCountPerConversation:
          groupResult.averageEmailFullSuggestionUsedCountPerConversation || 0,
        averageEmailFullSuggestionUsedCountPerFrequencyPerUser:
          groupResult.averageEmailFullSuggestionUsedCountPerFrequencyPerUser || 0,
      };
    }) || [];

  return {
    resultsGroupedByAttributeTypes,
    totalSuggestionUsedCount: response.totalSuggestionUsedCount || 0,
    totalConversationCount: response.totalConversationCount || 0,
    averageSuggestionUsedCountPerConversation: response.averageSuggestionUsedCountPerConversation || 0,
    averageSuggestionUsedCountPerFrequencyPerUser: response.averageSuggestionUsedCountPerFrequencyPerUser || 0,
    totalEmailPartialSuggestionUsedCount: response.totalEmailPartialSuggestionUsedCount || 0,
    averageEmailPartialSuggestionUsedCountPerConversation:
      response.averageEmailPartialSuggestionUsedCountPerConversation || 0,
    averageEmailPartialSuggestionUsedCountPerFrequencyPerUser:
      response.averageEmailPartialSuggestionUsedCountPerFrequencyPerUser || 0,
    totalEmailFullSuggestionUsedCount: response.totalEmailFullSuggestionUsedCount || 0,
    averageEmailFullSuggestionUsedCountPerConversation:
      response.averageEmailFullSuggestionUsedCountPerConversation || 0,
    averageEmailFullSuggestionUsedCountPerFrequencyPerUser:
      response.averageEmailFullSuggestionUsedCountPerFrequencyPerUser || 0,
  };
}

export function transformSmartComposeStatsResponse(
  response: CrestaV1RetrieveSmartComposeStatsResponse
): SmartComposeStatsResponse {
  const resultsGroupedByAttributeTypes: SmartComposeStatsGroupResult[] =
    response.smartComposeStatsResults?.map((groupResult) => {
      const smartComposeStats: SmartComposeStatsEntry[] =
        groupResult.smartComposeStats?.map((entry) => ({
          intervalStart: transformStartTimestamp(entry.timeRange),
          totalConversationCount: entry.totalConversationCount || 0,
          totalSmartComposeUsedCount: entry.totalSmartComposeUsedCount || 0,
          averageSmartComposeUsedCountPerConversation: entry.averageSmartComposeUsedCountPerConversation || 0,
          averageSmartComposeUsedCountPerFrequencyPerUser: entry.averageSmartComposeUsedCountPerFrequencyPerUser || 0,
        })) || [];

      return {
        groupedByAttributes: transformAttributes(groupResult.attribute),
        smartComposeStats,
        totalConversationCount: groupResult.totalConversationCount || 0,
        totalSmartComposeUsedCount: groupResult.totalSmartComposeUsedCount || 0,
        averageSmartComposeUsedCountPerConversation: groupResult.averageSmartComposeUsedCountPerConversation || 0,
        averageSmartComposeUsedCountPerFrequencyPerUser:
          groupResult.averageSmartComposeUsedCountPerFrequencyPerUser || 0,
      };
    }) || [];

  return {
    resultsGroupedByAttributeTypes,
    totalSmartComposeUsedCount: response.totalSmartComposeUsedCount || 0,
    totalConversationCount: response.totalConversationCount || 0,
    averageSmartComposeUsedCountPerConversation: response.averageSmartComposeUsedCountPerConversation || 0,
    averageSmartComposeUsedCountPerFrequencyPerUser: response.averageSmartComposeUsedCountPerFrequencyPerUser || 0,
  };
}

export function transformKnowledgeBaseStatsResponse(
  response: CrestaV1RetrieveKnowledgeBaseStatsResponse
): KnowledgeBaseStatsResponse {
  const resultsGroupedByAttributeTypes: KnowledgeBaseStatsGroupResult[] =
    response.knowledgeBaseStatsResults?.map((groupResult) => {
      const knowledgeBaseStats: KnowledgeBaseStatsEntry[] =
        groupResult.kbSearchStats?.map((entry) => ({
          intervalStart: transformStartTimestamp(entry.timeRange),
          totalConversationCount: entry.totalConversationCount || 0,
          totalKbSearchCount: entry.totalKbSearchCount || 0,
          averageKbSearchCountPerConversation: entry.averageKbSearchCountPerConversation || 0,
          averageKbSearchCountPerFrequencyPerUser: entry.averageKbSearchCountPerFrequencyPerUser || 0,
        })) || [];

      return {
        groupedByAttributes: transformAttributes(groupResult.attribute),
        knowledgeBaseStats,
        totalConversationCount: groupResult.totalConversationCount || 0,
        totalKbSearchCount: groupResult.totalKbSearchCount || 0,
        averageKbSearchCountPerConversation: groupResult.averageKbSearchCountPerConversation || 0,
        averageKbSearchCountPerFrequencyPerUser: groupResult.averageKbSearchCountPerFrequencyPerUser || 0,
      };
    }) || [];

  return {
    resultsGroupedByAttributeTypes,
    totalKbSearchCount: response.totalKbSearchCount || 0,
    totalConversationCount: response.totalConversationCount || 0,
    averageKbSearchCountPerConversation: response.averageKbSearchCountPerConversation || 0,
    averageKbSearchCountPerFrequencyPerUser: response.averageKbSearchCountPerFrequencyPerUser || 0,
  };
}

export function transformNoteTakingStatsResponse(
  response: CrestaV1RetrieveNoteTakingStatsResponse
): NoteTakingStatsResponse {
  const resultsGroupedByAttributeTypes: NoteTakingStatsGroupResult[] =
    response.noteTakingStatsResults?.map((groupResult) => {
      const noteTakingStats: NoteTakingStatsEntry[] =
        groupResult.noteTakingStats?.map((entry) => ({
          intervalStart: transformStartTimestamp(entry.timeRange),
          totalConversationCount: entry.totalConversationCount || 0,
          totalNoteTakingUsedCount: entry.totalNoteTakingUsedCount || 0,
          averageNoteTakingUsedCountPerConversation: entry.averageNoteTakingUsedCountPerConversation || 0,
          averageNoteTakingUsedCountPerFrequencyPerUser: entry.averageNoteTakingUsedCountPerUser || 0,
        })) || [];

      return {
        groupedByAttributes: transformAttributes(groupResult.attribute),
        noteTakingStats,
        totalConversationCount: groupResult.totalConversationCount || 0,
        totalNoteTakingUsedCount: groupResult.totalNoteTakingUsedCount || 0,
        averageNoteTakingUsedCountPerConversation: groupResult.averageNoteTakingUsedCountPerConversation || 0,
        averageNoteTakingUsedCountPerFrequencyPerUser: groupResult.averageNoteTakingUsedCountPerFrequencyPerUser || 0,
      };
    }) || [];

  return {
    resultsGroupedByAttributeTypes,
    totalNoteTakingUsedCount: response.totalNoteTakingUsedCount || 0,
    totalConversationCount: response.totalConversationCount || 0,
    averageNoteTakingUsedCountPerConversation: response.averageNoteTakingUsedCountPerConversation || 0,
    averageNoteTakingUsedCountPerFrequencyPerUser: response.averageNoteTakingUsedCountPerFrequencyPerUser || 0,
  };
}

export function transformGuidedWorkflowStatsResponse(
  response: CrestaV1RetrieveGuidedWorkflowStatsResponse
): GuidedWorkflowStatsResponse {
  const resultsGroupedByAttributeTypes: GuidedWorkflowStatsGroupResult[] =
    response.guidedWorkflowStatsResults?.map((groupResult) => {
      const guidedWorkflowStats: GuidedWorkflowStatsEntry[] =
        groupResult.guidedWorkflowStats?.map((entry) => ({
          intervalStart: transformStartTimestamp(entry.timeRange),
          totalConversationCount: entry.totalConversationCount || 0,
          totalGuidedWorkflowClickCount: entry.totalGuidedWorkflowClickCount || 0,
          averageGuidedWorkflowClickCountPerConversation: entry.averageGuidedWorkflowClickCountPerConversation || 0,
          averageGuidedWorkflowClickCountPerFrequencyPerUser:
            entry.averageGuidedWorkflowClickCountPerFrequencyPerUser || 0,
        })) || [];

      return {
        groupedByAttributes: transformAttributes(groupResult.attribute),
        guidedWorkflowStats,
        totalConversationCount: groupResult.totalConversationCount || 0,
        totalGuidedWorkflowClickCount: groupResult.totalGuidedWorkflowClickCount || 0,
        averageGuidedWorkflowClickCountPerConversation: groupResult.averageGuidedWorkflowClickCountPerConversation || 0,
        averageGuidedWorkflowClickCountPerFrequencyPerUser:
          groupResult.averageGuidedWorkflowClickCountPerFrequencyPerUser || 0,
      };
    }) || [];

  return {
    resultsGroupedByAttributeTypes,
    totalGuidedWorkflowClickCount: response.totalGuidedWorkflowClickCount || 0,
    totalConversationCount: response.totalConversationCount || 0,
    averageGuidedWorkflowClickCountPerConversation: response.averageGuidedWorkflowClickCountPerConversation || 0,
    averageGuidedWorkflowClickCountPerFrequencyPerUser:
      response.averageGuidedWorkflowClickCountPerFrequencyPerUser || 0,
  };
}

export function transformSummarizationStatsResponse(
  response: CrestaV1RetrieveSummarizationStatsResponse
): SummarizationStatsResponse {
  const resultsGroupedByAttributeTypes: SummarizationStatsGroupResult[] =
    response.summarizationStatsResults?.map((groupResult) => {
      const summarizationStats: SummarizationStatsEntry[] =
        groupResult.summarizationStats?.map((entry) => ({
          intervalStart: transformStartTimestamp(entry.timeRange),
          totalConversationCount: entry.totalConversationCount || 0,
          totalSummarizationGeneratedCount: entry.totalSummarizationGeneratedCount || 0,
          totalSummarizationUsedCount: entry.totalSummarizationUsedCount || 0,
          averageSummarizationGeneratedCountPerFrequencyPerUser: entry.averageSummarizationGeneratedCountPerUser || 0,
          averageSummarizationUsedCountPerConversation: entry.averageSummarizationUsedCountPerConversation || 0,
          averageSummarizationUsedCountPerFrequencyPerUser: entry.averageSummarizationUsedCountPerUser || 0,
          averageSummarizationUsedRatio: entry.averageSummarizationUsedRatio || 0,
        })) || [];

      return {
        groupedByAttributes: transformAttributes(groupResult.attribute),
        summarizationStats,
        totalConversationCount: groupResult.totalConversationCount || 0,
        totalSummarizationGeneratedCount: groupResult.totalSummarizationGeneratedCount || 0,
        totalSummarizationUsedCount: groupResult.totalSummarizationUsedCount || 0,
        averageSummarizationGeneratedCountPerFrequencyPerUser:
          groupResult.averageSummarizationGeneratedCountPerFrequencyPerUser || 0,
        averageSummarizationUsedCountPerConversation: groupResult.averageSummarizationUsedCountPerConversation || 0,
        averageSummarizationUsedCountPerFrequencyPerUser:
          groupResult.averageSummarizationUsedCountPerFrequencyPerUser || 0,
        averageSummarizationUsedRatio: groupResult.averageSummarizationUsedRatio || 0,
      };
    }) || [];

  return {
    resultsGroupedByAttributeTypes,
    totalSummarizationGeneratedCount: response.totalSummarizationGeneratedCount || 0,
    totalSummarizationUsedCount: response.totalSummarizationUsedCount || 0,
    totalConversationCount: response.totalConversationCount || 0,
    averageSummarizationGeneratedCountPerFrequencyPerUser:
      response.averageSummarizationGeneratedCountPerFrequencyPerUser || 0,
    averageSummarizationUsedCountPerConversation: response.averageSummarizationUsedCountPerConversation || 0,
    averageSummarizationUsedCountPerFrequencyPerUser: response.averageSummarizationUsedCountPerFrequencyPerUser || 0,
    averageSummarizationUsedRatio: response.averageSummarizationUsedRatio || 0,
  };
}

function transformMetadataValueAttributes(
  metadataValueAttributes: CrestaV1MetadataValueAttribute[] | undefined
): MetadataValueAttribute[] {
  if (!metadataValueAttributes) {
    return [];
  }
  return metadataValueAttributes.map((attribute): MetadataValueAttribute => {
    return {
      metadataValue: attribute.metadataValue ? transformMetadataOrOutcomeValue(attribute.metadataValue) : undefined,
      numericBin: attribute.numericBin ? transformNumericBin(attribute.numericBin, undefined) : undefined,
    };
  });
}

function transformMomentGroup(group: CrestaV1MomentGroup): MomentGroup {
  return {
    moments: group.moments?.map((moment) => transformMoment(moment)) || [],
    metadataValueAttributes: transformMetadataValueAttributes(group.metadataValueAttributes),
    outcomeValueAttributes:
      group.outcomeValueAttributes?.map((attribute) => {
        return {
          ...transformOutcomeValues(attribute.outcomeValue ?? {}),
          numericBin: transformNumericBin(attribute.numericBin ?? {}, ''),
        };
      }) ?? [],
  };
}

export function transformMomentGroupToApi(group: MomentGroup): CrestaV1MomentGroup {
  return {
    moments: group.moments.map((moment) => transformMomentToApiForInsights(moment)),
  };
}

export function transformMomentGroupsToApi(momentGroups: MomentGroup[] | undefined): CrestaV1MomentGroup[] | undefined {
  return momentGroups?.map((group) => transformMomentGroupToApi(group));
}

function transformAttributes(attribute: CrestaV1Attribute | undefined): Attribute {
  if (!attribute) {
    return {};
  }
  const team: CrestaV1UserTeam.Team | undefined = first(attribute.teams);

  return {
    user: transformUsers(attribute.users),
    team: team ? transformApiTeamToModel(team) : undefined,
    criterionIdentifiers: attribute.criterionIdentifiers,
    criterionIdentifier: attribute.criterionIdentifiers?.[0],
    group: attribute.groups?.length ? transformApiGroupToModel(attribute.groups[0]) : undefined,
    intents: attribute.intents,
    moments: attribute.moments?.map((m) => transformMoment(m)),
    actions: attribute.actions?.map((a) => transformAction(a)).filter((a) => !!a),
    attributePolicies: attribute.attributePolicies?.map((p) => transformAttributePolicy(p)),
    momentGroups: attribute.momentGroups?.map((group) => transformMomentGroup(group)),
    timeRanges: attribute.timeRanges?.map(transformTimeRange),
    conversationDurationBuckets: attribute.conversationDurationBuckets?.map(transformConversationDurationBucket),
    adherence: attribute.adherence,
    groups: attribute.groups?.map((v) => transformApiGroupToModel(v)),
    behaviors: attribute.behaviors?.map(transformApiBehaviorToModel),
    languageCodes: attribute.languageCodes,
    coachingSession: attribute.coachingSessionNames?.[0],
    coachingSessionNames: attribute.coachingSessionNames,
    guidedWorkflowNames: attribute.guidedWorkflowNames,
  };
}

export function transformAdherenceStatsResponse(response: CrestaV1RetrieveAdherencesResponse): AdherenceStatsResponse {
  const resultsByAttributeType =
    response.adherenceResults?.map((groupResult): AdherenceStatsGroupResult => {
      const entries =
        groupResult.adherences?.map((adherence): AdherenceEntry => {
          return {
            adherence: adherence.adherence || 0,
            intervalStart: transformStartTimestamp(adherence.timeRange),
            totalSdxCount: adherence.totalSdxCount || 0,
          };
        }) || [];

      return {
        averageAdherence: groupResult.averageAdherence || 0,
        totalSdxCount: groupResult.totalSdxCount || 0,
        entries: entries,
        groupedByAttributes: transformAttributes(groupResult.attribute),
      };
    }) || [];

  return {
    resultsGroupedByAttributeTypes: resultsByAttributeType,
    averageAdherence: response.averageAdherence || 0,
    totalSdxCount: response.totalSdxCount || 0,
  };
}

export function transformHintStats(response: CrestaV1RetrieveHintStatsResponse): HintStatsResponse {
  return {
    totalHintSentCount: response.totalHintSentCount || 0,
    totalHintFollowedCount: response.totalHintFollowedCount || 0,
    totalConversationCount: response.totalConversationCount || 0,
    averageHintSentCountPerConversation: response.averageHintSentCountPerConversation || 0,
    averageHintSentCountPerFrequencyPerUser: response.averageHintSentCountPerFrequencyPerUser || 0,
    averageHintFollowedCountPerFrequencyPerUser: response.averageHintFollowedCountPerFrequencyPerUser || 0,
    averageHintFollowedCountPerConversation: response.averageHintFollowedCountPerConversation || 0,
    resultGroups:
      response.hintStatsResults?.map((groupResult): HintStatsGroupResult => {
        const entries =
          groupResult.hintStats?.map((hintStats): HintStatsEntry => {
            return {
              totalHintSentCount: hintStats.totalHintSentCount || 0,
              totalHintFollowedCount: hintStats.totalHintFollowedCount || 0,
              intervalStart: transformStartTimestamp(hintStats.timeRange),
              totalConversationCount: hintStats.totalConversationCount || 0,
              averageHintFollowedCountPerFrequencyPerUser: hintStats.averageHintFollowedCountPerFrequencyPerUser || 0,
            };
          }) || [];
        return {
          totalHintFollowedCount: groupResult.totalHintFollowedCount || 0,
          totalHintSentCount: groupResult.totalHintSentCount || 0,
          totalConversationCount: groupResult.totalConversationCount || 0,
          averageHintSentCountPerConversation: groupResult.averageHintSentCountPerConversation || 0,
          averageHintSentCountPerFrequencyPerUser: groupResult.averageHintSentCountPerFrequencyPerUser || 0,
          averageHintFollowedCountPerConversation: groupResult.averageHintFollowedCountPerConversation || 0,
          averageHintFollowedCountPerFrequencyPerUser: groupResult.averageHintFollowedCountPerFrequencyPerUser || 0,
          entries: entries,
          groupedByAttributes: transformAttributes(groupResult.attribute),
        };
      }) || [],
  };
}

export function transformReceptivities(response: CrestaV1RetrieveReceptivitiesResponse): HintEngagementResponse {
  const resultsByAttributeType =
    response.receptivityResults?.map((groupResult): HintEngagementGroupResult => {
      const entries =
        groupResult.receptivities?.map((receptivityStats): HintEngagementEntry => {
          return {
            hintEngagement: receptivityStats.receptivity || 0,
            intervalStart: transformStartTimestamp(receptivityStats.timeRange),
          };
        }) || [];

      return {
        averageHintEngagement: groupResult.averageReceptivity || 0,
        entries,
        groupedByAttributes: transformAttributes(groupResult.attribute),
      };
    }) || [];
  return {
    resultsGroupedByAttributeTypes: resultsByAttributeType,
    averageHintEngagement: response.averageReceptivity || 0,
  };
}

export function transformConversationMessageStats(
  response: RetrieveConversationMessageStatsResponse
): ConversationMessageStatsResponse {
  const conversationMessageStats: ConversationMessageStats[] =
    response.conversationMessageStats?.map((messageStats): ConversationMessageStats => {
      const aggregated = messageStats.aggregatedConversationMessageStats;
      return {
        aggregatedConversationMessageStats: {
          totalConversationCount: aggregated?.totalConversationCount || 0,
          totalConversationMessageCount: aggregated?.totalConversationMessageCount || 0,
          averageHandleTime: aggregated?.averageHandleTime ?? '-1',
        },
        attribute: transformAttributes(messageStats.attribute),
      };
    }) || [];
  return {
    conversationMessageStats: conversationMessageStats,
    aggregatedConversationMessageStats: {
      totalConversationCount: response.aggregatedConversationMessageStats?.totalConversationCount || 0,
      totalConversationMessageCount: response.aggregatedConversationMessageStats?.totalConversationMessageCount || 0,
      averageHandleTime: response.aggregatedConversationMessageStats?.averageHandleTime ?? '-1',
    },
  };
}

export function transformConversationMessages(
  response: CrestaV1RetrieveConversationMessagesResponse
): ConversationMessagesResponse {
  const conversationMessageResults: ConversationMessageResult[] =
    response.conversationMessageResults?.map((result): ConversationMessageResult => {
      return {
        messageResults:
          result.messageResults?.map(
            (messageResult): ConversationMessageResultMessageResult =>
              transformConversationMessageResultMessageResult(messageResult)
          ) || [],
        conversationResult: {
          conversation: result.conversationResult?.conversation
            ? transformConversation(result.conversationResult.conversation)
            : undefined,
        },
      };
    }) || [];

  return {
    conversationMessageResults: conversationMessageResults,
    nextPageToken: response.nextPageToken,
  };
}

function transformConversationMessageResultMessageResult(
  messageResult: CrestaV1ConversationMessageResultMessageResult
): ConversationMessageResultMessageResult {
  return {
    conversationMessage: messageResult.conversationMessage
      ? transformConversationMessage(messageResult.conversationMessage)
      : undefined,
    momentResults:
      messageResult.momentResults?.map((momentResult): MomentResult => {
        return {
          moment: momentResult.moment ? transformMoment(momentResult.moment) : undefined,
          matchedMoment: momentResult.matchedMoment ? transformMatchedMoment(momentResult.matchedMoment) : undefined,
          momentAnnotation: momentResult.momentAnnotation
            ? transformMomentAnnotation(momentResult.momentAnnotation)
            : undefined,
        };
      }) || [],
  };
}

export function transformXdxInfo(xdxInfos?: CrestaV1ConversationResultConversationInfoIntentInfo[]): XdxInfo[] {
  if (!xdxInfos) {
    return [];
  }
  return xdxInfos.map((xdxInfo) => ({
    intent: transformApiIntentToModel(xdxInfo.intent),
    opportunities:
      xdxInfo.opportunities?.map((o) => ({
        id: o.id || -1,
        legacyMessageId: o.message?.legacyMessageId,
        messageName: o.message?.name || '',
        messageTime: transformTimeStamp(o.message?.createTime || ''),
      })) || [],
  }));
}

function transformUsers(users: CrestaV1UserUser.User[] | undefined): User | undefined {
  if (!users) {
    return undefined;
  }
  if (users.length === 0) {
    return undefined;
  }
  return transformApiUserToModel(users[0]);
}

export function transformKeywordsStats(response: CrestaV1RetrieveKeywordsStatsResponse): KeywordsStatsResponse {
  return {
    retrieveKeywordsResults:
      response.retrieveKeywordsResults?.map((result): RetrieveKeywordsResult => {
        const stats =
          result.keywordStats?.map((KeywordsStats): KeywordsStats => {
            return {
              totalConversationCount: KeywordsStats.totalConversationCount || 0,
              totalMessageCount: KeywordsStats.totalMessageCount || 0,
              keywordsConversationCount: KeywordsStats.keywordsConversationCount || 0,
              keywordsMessageCount: KeywordsStats.keywordsMessageCount || 0,
              intervalStart: transformStartTimestamp(KeywordsStats.timeRange),
            };
          }) || [];

        return {
          keywordStats: stats,
        };
      }) || [],
    overallStats: {
      totalConversationCount: response.overallStats?.totalConversationCount || 0,
      totalMessageCount: response.overallStats?.totalMessageCount || 0,
      keywordsConversationCount: response.overallStats?.keywordsConversationCount || 0,
      keywordsMessageCount: response.overallStats?.keywordsMessageCount || 0,
      intervalStart: transformStartTimestamp(response.overallStats?.timeRange),
    },
  };
}

export function transformChatbotConversationStats(
  response: CrestaV1RetrieveChatBotConversationStatsResponse
): ChatbotConversationStatsResponse {
  const groupedResults: ChatbotConversationStatsGroupedResult[] =
    response.conversationStatsResults?.map(
      (groupResult: ChatBotConversationStatsResult): ChatbotConversationStatsGroupedResult => {
        return {
          totalConversationCount: groupResult.totalConversationCount || 0,
          averageMessageCountPerFrequency: groupResult.averageConversationCountPerFrequency || 0,
          totalMessageCount: groupResult.totalMessageCount || 0,
          averageConversationCountPerFrequency: groupResult.averageConversationCountPerFrequency || 0,
          attribute: groupResult.chatBotAttribute,
        };
      }
    ) || [];

  return {
    totalConversationCount: response.totalConversationCount || 0,
    averageConversationCountPerFrequency: response.averageConversationCountPerFrequency || 0,
    totalMessageCount: response.totalMessageCount || 0,
    averageMessageCountPerFrequency: response.averageMessageCountPerFrequency || 0,
    groupedResult: groupedResults,
  };
}

export function transformChatbotSessionStats(
  response: CrestaV1RetrieveChatBotSessionStatsResponse
): ChatbotSessionStatsResponse {
  return {
    totalWidgetSessionCount: response.totalWidgetSessionCount || 0,
  };
}

export function transformCommentStats(response: CrestaV1RetrieveCommentStatsResponse): CommentStatsResponse {
  return {
    totalCommentsPlacedCount: response.totalCommentsPlacedCount || 0,
    averageCommentsPlacedCountPerUser: response.averageCommentsPlacedCountPerUser || 0,
    resultsGroupedByAttributeTypes:
      response.commentStatsResults?.map((groupResult): CommentStatsGroupResult => {
        const entries: CommentStatsEntry[] =
          groupResult.commentStats?.map((commentStats): CommentStatsEntry => {
            return {
              totalCommentsPlacedCount: commentStats.totalCommentsPlacedCount || 0,
              averageCommentsPlacedCountPerUser: commentStats.averageCommentsPlacedCountPerUser || 0,
              intervalStart: transformStartTimestamp(commentStats.timeRange),
            };
          }) || [];
        return {
          totalCommentsPlacedCount: groupResult.totalCommentsPlacedCount || 0,
          averageCommentsPlacedCountPerUser: groupResult.averageCommentsPlacedCountPerUser || 0,
          entries: entries,
          groupedByAttributes: transformAttributes(groupResult.attribute),
        };
      }) || [],
  };
}

export function transformScorecardStats(
  response: CrestaV1CrestaV1RetrieveReceptivitiesResponse
): ScorecardStatsResponse {
  return {
    scorecardCompleted: response.scorecardCompleted || 0,
    averageScorecardCompletedPerUser: response.averageScorecardCompletedPerUser || 0,
    resultsGroupedByAttributeTypes:
      response.scorecardStatsResults?.map((groupResult): ScorecardStatsGroupResult => {
        const entries: ScorecardStatsEntry[] =
          groupResult.scorecardStats?.map((scorecardStats): ScorecardStatsEntry => {
            return {
              scorecardCompleted: scorecardStats.scorecardCompleted || 0,
              averageScorecardCompletedPerUser: scorecardStats.averageScorecardCompletedPerUser || 0,
              intervalStart: transformStartTimestamp(scorecardStats.timeRange),
            };
          }) || [];
        return {
          scorecardCompleted: groupResult.scorecardCompleted || 0,
          averageScorecardCompletedPerUser: groupResult.averageScorecardCompletedPerUser || 0,
          entries: entries,
          groupedByAttributes: transformAttributes(groupResult.attribute),
        };
      }) || [],
  };
}

export function transformCoachingSessionStats(
  response: CrestaV1RetrieveCoachingSessionStatsResponse
): CoachingSessionStatsResponse {
  return {
    coachingSessionsSubmitted: response.coachingSessionsSubmitted || 0,
    averageCoachingSessionsSubmittedPerUser: response.averageCoachingSessionsSubmittedPerUser || 0,
    resultsGroupedByAttributeTypes:
      response.coachingSessionStatsResults?.map((groupResult) => ({
        coachingSessionsSubmitted: groupResult.coachingSessionsSubmitted || 0,
        averageCoachingSessionsSubmittedPerUser: groupResult.averageCoachingSessionsSubmittedPerUser || 0,
        groupedByAttributes: transformAttributes(groupResult.attribute),
        entries:
          groupResult.coachingSessionStats?.map((stats) => ({
            coachingSessionsSubmitted: stats.coachingSessionsSubmitted || 0,
            averageCoachingSessionsSubmittedPerUser: stats.averageCoachingSessionsSubmittedPerUser || 0,
            intervalStart: transformStartTimestamp(stats.timeRange),
          })) || [],
      })) || [],
  };
}

export function transformConversationBucketListResponse(
  response: CrestaV1ListConversationDurationBucketsResponse
): ConversationDurationBucketsResponse {
  return {
    conversationDurationBuckets: response.conversationDurationBuckets?.map(transformConversationDurationBucket) || [],
  };
}

export function transformConversationDurationBucket(
  item: ConversationDurationBucketFromApi
): ConversationDurationBucket {
  return {
    id: item.id || '',
    lowerBound: item.lowerBound || 0,
    upperBound: item.upperBound || 0,
  };
}

export function transformClosedConversations(
  response: CrestaV1RetrieveClosedConversationsResponse
): ClosedConversationsResponse {
  return {
    conversations: response.conversations?.map((t) => transformConversation(t)) || [],
    totalConversationCount: response.totalConversationCount || 0,
    nextPageToken: response.nextPageToken || undefined,
  };
}

export function transformInsightsTopic(
  response: CrestaV1GenerateInsightsTopicResponse | CrestaV1ModifyInsightsTopicResponse
): InsightsTopic {
  return {
    topicMoment: transformMoment(response.insightsTopic?.topicMoment || {}),
    includedKeywordMoment: response.insightsTopic?.includedKeywordMoment
      ? transformMoment(response.insightsTopic.includedKeywordMoment)
      : undefined,
    excludedKeywordMoment: response.insightsTopic?.excludedKeywordMoment
      ? transformMoment(response.insightsTopic.excludedKeywordMoment)
      : undefined,
    clusterMoments: response.insightsTopic?.clusterMoments?.map(transformMoment) || [],
  };
}

export function transformInsightsTopics(response: CrestaV1RetrieveInsightsTopicsResponse): InsightsTopic[] {
  return response.insightsTopics?.map((topic) => transformInsightsTopic({ insightsTopic: topic })) || [];
}

export function transformLiveAssistStatsResponse(
  response: CrestaV1RetrieveLiveAssistStatsResponse
): LiveAssistStatsResponse {
  const resultsByAttributeType =
    response.liveAssistResults?.map((groupResult): LiveAssistStatsGroupResult => {
      const entries =
        groupResult.liveAssistStats?.map((stats): LiveAssistStatsEntry => {
          return {
            totalRaisedHandCount: stats.totalRaisedHandCount || 0,
            totalRaisedHandAnsweredCount: stats.totalRaisedHandAnsweredCount || 0,
            totalWhisperedToAgentCount: stats.totalWhisperedToAgentCount || 0,
            totalWhisperByManagerCount: stats.totalWhisperByManagerCount || 0,
            intervalStart: transformStartTimestamp(stats.timeRange),
          };
        }) || [];

      return {
        totalRaisedHandCount: groupResult.totalRaisedHandCount || 0,
        totalRaisedHandAnsweredCount: groupResult.totalRaisedHandAnsweredCount || 0,
        totalWhisperedToAgentCount: groupResult.totalWhisperedToAgentCount || 0,
        totalWhisperByManagerCount: groupResult.totalWhisperByManagerCount || 0,
        entries: entries,
        groupedByAttributes: transformAttributes(groupResult.attribute),
      };
    }) || [];
  return {
    resultsGroupedByAttributeTypes: resultsByAttributeType,
    totalRaisedHandCount: response.totalRaisedHandCount || 0,
    totalRaisedHandAnsweredCount: response.totalRaisedHandAnsweredCount || 0,
    totalWhisperedToAgentCount: response.totalWhisperedToAgentCount || 0,
    totalWhisperByManagerCount: response.totalWhisperByManagerCount || 0,
  };
}

export function transformKnowledgeAssistStatsResponse(
  response: RetrieveKnowledgeAssistStatsResponse
): KnowledgeAssistStatsResponse {
  const resultsByAttributeType =
    response.knowledgeAssistStatsResults?.map((groupResult): KnowledgeAssistStatsGroupResult => {
      const entries =
        groupResult.knowledgeAssistStats?.map((knowledgeAssistStats): KnowledgeAssistStatsEntry => {
          return {
            totalGenAIAnswersUsedCount: knowledgeAssistStats.genaiAnswersUsedCount || 0,
            totalGenAIAnswersViewedCount: knowledgeAssistStats.genaiAnswersViewedCount || 0,
            totalKnowledgeMomentDetectedCount: knowledgeAssistStats.knowledgeMomentDetectedCount || 0,
            averageGenAIAnswersUsedCountPerUser:
              knowledgeAssistStats.averageGenaiAnswerUsedCountPerFrequencyPerUser || 0,
            intervalStart: transformStartTimestamp(knowledgeAssistStats.timeRange),
          };
        }) || [];

      return {
        totalGenAIAnswersUsedCount: groupResult.genaiAnswersUsedCount || 0,
        totalGenAIAnswersViewedCount: groupResult.genaiAnswersViewedCount || 0,
        totalKnowledgeMomentDetectedCount: groupResult.knowledgeMomentDetectedCount || 0,
        averageGenAIAnswersUsedCountPerUser: groupResult.averageGenaiAnswerUsedCountPerFrequencyPerUser || 0,
        entries: entries,
        groupedByAttributes: transformAttributes(groupResult.attribute),
      };
    }) || [];
  return {
    resultsGroupedByAttributeTypes: resultsByAttributeType,
    totalGenAIAnswersUsedCount: response.genaiAnswersUsedCount || 0,
    totalGenAIAnswersViewedCount: response.genaiAnswersViewedCount || 0,
    totalKnowledgeMomentDetectedCount: response.knowledgeMomentDetectedCount || 0,
    averageGenAIAnswersUsedCountPerUser: response.averageGenaiAnswerUsedCountPerFrequencyPerUser || 0,
  };
}

export function transformCustomerSnapshotStatsResponse(
  response: RetrieveCustomerSnapshotStatsResponse
): CustomerSnapshotStatsResponse {
  const resultsByAttributeType =
    response.customerSnapshotStatsResults?.map((groupResult): CustomerSnapshotStatsGroupResult => {
      const entries =
        groupResult.customerSnapshotStats?.map((customerSnapshotStats): CustomerSnapshotStatsEntry => {
          return {
            totalDisplayTime: Number(customerSnapshotStats.totalDisplayTime) || 0,
            averageDisplayTime: customerSnapshotStats.averageDisplayTime || 0,
            averageDisplayTimePerFrequencyPerUser: customerSnapshotStats.averageDisplayTimePerFrequencyPerUser || 0,
            totalConversationsWithSnapshot: customerSnapshotStats.totalConversationsWithSnapshot || 0,
            intervalStart: transformStartTimestamp(customerSnapshotStats.timeRange),
          };
        }) || [];

      return {
        totalDisplayTime: Number(groupResult.totalDisplayTime) || 0,
        averageDisplayTime: groupResult.averageDisplayTime || 0,
        averageDisplayTimePerFrequencyPerUser: groupResult.averageDisplayTimePerFrequencyPerUser || 0,
        totalConversationsWithSnapshot: groupResult.totalConversationsWithSnapshot || 0,
        entries: entries,
        groupedByAttributes: transformAttributes(groupResult.attribute),
      };
    }) || [];
  return {
    resultsGroupedByAttributeTypes: resultsByAttributeType,
    totalDisplayTime: Number(response.totalDisplayTime) || 0,
    averageDisplayTime: response.averageDisplayTime || 0,
    averageDisplayTimePerFrequencyPerUser: response.averageDisplayTimePerFrequencyPerUser || 0,
    totalConversationsWithSnapshot: response.totalConversationsWithSnapshot || 0,
  };
}

function transformAnnotatedTimeSeriesDataPoint(
  annotatedTimeSeriesDataPoint: CrestaV1AnnotatedTimeSeriesDataPoint
): AnnotatedTimeSeriesDataPoint {
  return {
    intervalStart: transformStartTimestamp(annotatedTimeSeriesDataPoint.timeRange),
    intervalEnd: transformEndTimeStamp(annotatedTimeSeriesDataPoint.timeRange),
    value: annotatedTimeSeriesDataPoint.value || 0,
    predictedRangeLower: annotatedTimeSeriesDataPoint.predictedRangeLower || 0,
    predictedRangeUpper: annotatedTimeSeriesDataPoint.predictedRangeUpper || 0,
    anomalyScore: annotatedTimeSeriesDataPoint.anomalyScore || 0,
    valueNumerator: annotatedTimeSeriesDataPoint.valueNumerator || 0,
    valueDenominator: annotatedTimeSeriesDataPoint.valueDenominator || 0,
    trendLineValue: annotatedTimeSeriesDataPoint.trendLineValue || 0,
    predictedValue: annotatedTimeSeriesDataPoint.predictedValue || 0,
  };
}

function transformAnnotatedTimeSeries(annotatedTimeSeries: CrestaV1AnnotatedTimeSeries): AnnotatedTimeSeries {
  return {
    attribute: transformAttributes(annotatedTimeSeries.attribute),
    maxAnomalyScore: annotatedTimeSeries.maxAnomalyScore || 0,
    slopeOfTrend: annotatedTimeSeries.slopeOfTrend || 0,
    dataPoints: annotatedTimeSeries.dataPoints?.map((v) => transformAnnotatedTimeSeriesDataPoint(v)) || [],
    trendInfo: {
      valueChange: annotatedTimeSeries.trendInfo?.valueChange || 0,
      trendTimeRange: {
        intervalStart: transformStartTimestamp(annotatedTimeSeries.trendInfo?.trendTimeRange),
        intervalEnd: transformEndTimeStamp(annotatedTimeSeries.trendInfo?.trendTimeRange),
      },
    },
  };
}

export function transformAnnotatedTimeSeriesResponse(
  response: CrestaV1RetrieveAnnotatedTimeSeriesResponse
): AnnotatedTimeSeriesResponse {
  return {
    annotatedTimeSeries: response.annotatedTimeSeries?.map((v) => transformAnnotatedTimeSeries(v)) || [],
  };
}

/**
 * In some cases in Insights FE sends the entire moment object to the BE. Some properties of the FE Moment object
 * shouldn't be sent.
 *
 * E.g.: in Topic Discovery
 * 1. All relevant moments are fetched from API
 * 2. Moments are sent to insights to retrieve bubble size of different moments
 */
export function transformMomentToApiForInsights(
  moment: Moment | ConversationMetadataMoment | ConversationOutcomeMoment
): CrestaV1Moment {
  return omit(moment, ['_raw', 'payload']);
}

export function transformMomentsToApiForInsights(
  moments: Moment[] | ConversationMetadataMoment[] | ConversationOutcomeMoment[]
): CrestaV1Moment[] {
  return moments.map((moment) => transformMomentToApiForInsights(moment));
}

export function transformMetadataPayloadToMetadataValueAttributeApi(
  conversationMetadataMomentPayload: ConversationMetadataMomentPayload | undefined
): CrestaV1MetadataValueAttribute[] {
  if (!conversationMetadataMomentPayload) {
    return [];
  }

  if (conversationMetadataMomentPayload.numericBins.length > 0) {
    return conversationMetadataMomentPayload.numericBins.map(
      (bin): CrestaV1MetadataValueAttribute => ({
        numericBin: bin,
      })
    );
  }

  return conversationMetadataMomentPayload.allowedValues.map(
    (allowedValue): CrestaV1MetadataValueAttribute => ({
      metadataValue: transformToMetadataOrOutcomeValueApi(allowedValue.value),
    })
  );
}

export function transformOutcomePayloadToOutcomeValueAttributeApi(
  conversationOutcomeMomentPayload: ConversationOutcomeMomentPayload | undefined
): OutcomeValueAttribute[] {
  if (!conversationOutcomeMomentPayload) {
    return [];
  }

  const outcomeValues = conversationOutcomeMomentPayload.outcomeValues.map(
    transformConversationOutcomeValueToOutcomeValueApi
  );
  const numericBins = conversationOutcomeMomentPayload.numericBins.map((bin) => ({
    numericBin: transformNumericBinToApi(bin),
  }));

  return [...outcomeValues, ...numericBins];
}

export function transformConversationOutcomeValueToOutcomeValueApi(
  value: ConversationOutcomeOutcomeValue
): OutcomeValueAttribute {
  if (value.numericBin?.fromValue !== undefined) {
    return {
      numericBin: {
        fromValue: value.numericBin.fromValue,
        toValue: value.numericBin.toValue,
        fromValueIsExclusive: value.numericBin.fromValueIsExclusive,
        toValueIsInclusive: value.numericBin.toValueIsInclusive,
        binaryOutcomeValueEnum: value.binaryOutcomeValueEnum,
      },
    };
  }
  return {
    outcomeValue: {
      value: transformToMetadataOrOutcomeValueApi(value.value),
      description: value.description,
      binaryOutcomeValueEnum: value.binaryOutcomeValueEnum,
    },
  };
}

export function transformToMetadataOrOutcomeValueApi(
  value: ConversationMetadataOrOutcomeValue
): CrestaV1MetadataValue | CrestaV1ConversationOutcomeValue {
  if (value.numberValue !== undefined) {
    return {
      numberValue: value.numberValue,
    };
  }
  if (value.booleanValue !== undefined) {
    return {
      booleanValue: value.booleanValue,
    };
  }
  return {
    stringValue: value.stringValue || '',
  };
}

export function transformNumericBinToApi(numericBin: ConversationMetadataNumericBin): CrestaV1NumericBin | undefined {
  return {
    toValue: numericBin.toValue,
    toValueIsInclusive: numericBin.toValueIsInclusive,
    fromValue: numericBin.fromValue,
    fromValueIsExclusive: numericBin.fromValueIsExclusive,
    description: numericBin.description,
    binaryOutcomeValueEnum: numericBin.binaryOutcomeValueEnum,
  };
}

export function transformMetadataValues(response: CrestaV1RetrieveMetadataValuesResponse): MetadataValuesResponse {
  return {
    metadataValues: response.metadataValues?.map((value) => transformMetadataOrOutcomeValue(value)) || [],
  };
}

export function transformLlmAnnotationValue(value: CrestaV1MetadataValue): LlmAnnotationValue {
  return {
    numberValue: value.numberValue,
    stringValue: value.stringValue,
    booleanValue: value.booleanValue,
  };
}

export function transformLlmAnnotationValues(
  response: CrestaV1RetrieveLlmAnnotationValuesResponse
): LlmAnnotationValuesResponse {
  return {
    values: response.values?.map(transformLlmAnnotationValue) || [],
  };
}

function transformOutcomeStats(outcomeStats?: CrestaV1OutcomeStats): OutcomeStats {
  return {
    count: outcomeStats?.count || 0,
    mean: outcomeStats?.mean || 0,
    confidenceIntervalEnd: outcomeStats?.confidenceIntervalEnd || 0,
    confidenceIntervalStart: outcomeStats?.confidenceIntervalStart || 0,
  };
}

function transformConversationOutcomeStats(
  attributeOutcomeStats: CrestaV1AttributeOutcomeStats
): AttributeOutcomeStats {
  return {
    attribute: transformAttributes(attributeOutcomeStats.attribute),
    behaviorAdherence: attributeOutcomeStats.behaviorAdherence,
    conversationCount: attributeOutcomeStats.conversationCount || 0,
    outcomeStats: attributeOutcomeStats.outcomeStats
      ? transformOutcomeStats(attributeOutcomeStats.outcomeStats)
      : undefined,
    agentCount: attributeOutcomeStats.agentCount || 0,
  };
}

function transformAgentTierOutcomeStats(agentTierOutcomeStats: CrestaV1AgentTierOutcomeStats): AgentTierOutcomeStats {
  return {
    agentTier: agentTierOutcomeStats.agentTier || AgentTier.AGENT_TIER_UNSPECIFIED,
    outcomeStats: transformOutcomeStats(agentTierOutcomeStats.outcomeStats),
    conversationCount: agentTierOutcomeStats.conversationCount || 0,
    agentCount: agentTierOutcomeStats.agentCount || 0,
    behaviorAdherence: agentTierOutcomeStats.behaviorAdherence || 0,
    behaviorMoment: agentTierOutcomeStats.behaviorMoment
      ? transformMoment(agentTierOutcomeStats.behaviorMoment)
      : undefined,
  };
}

export function transformConversationOutcomeStatsResponse(
  response: CrestaV1RetrieveConversationOutcomeStatsResponse
): ConversationOutcomeStats {
  return {
    attributeOutcomeStats: response.attributeOutcomeStats?.map((v) => transformConversationOutcomeStats(v)) || [],
    adherenceOutcomeIntercept: response.adherenceOutcomeIntercept || 0,
    adherenceOutcomeSlope: response.adherenceOutcomeSlope || 0,
    agentTierOutcomeStats: response.agentTierOutcomeStats?.map(transformAgentTierOutcomeStats) || [],
  };
}

function transformManualQAConversationInfo(
  manualQAConversationInfo: CrestaV1ManualQAConversationInfo
): ManualQAConversationInfo {
  return {
    conversation: manualQAConversationInfo.conversation
      ? transformConversation(manualQAConversationInfo.conversation)
      : undefined,
    agent: manualQAConversationInfo.agent ? transformApiUserToModel(manualQAConversationInfo.agent) : undefined,
    qaAnalyst: manualQAConversationInfo.qaAnalyst
      ? transformApiUserToModel(manualQAConversationInfo.qaAnalyst)
      : undefined,
    perfScore: isNonNaScore(manualQAConversationInfo.perfScore) ? manualQAConversationInfo.perfScore : undefined,
    consistencyScore: isNonNaScore(manualQAConversationInfo.consistencyScore)
      ? manualQAConversationInfo.consistencyScore
      : undefined,
    calibrator: manualQAConversationInfo.calibrator
      ? transformApiUserToModel(manualQAConversationInfo.calibrator)
      : undefined,
    processId: manualQAConversationInfo.processId,
    scorecardName: manualQAConversationInfo.scorecardName ?? '',
    processInteractionTime: dayjsIfTruthy(manualQAConversationInfo.processInteractionTime),
    messageNames: manualQAConversationInfo.messageNames || [],
  };
}

function transformManualQAStats(manualQAStats: CrestaV1ManualQAStats): ManualQAStats {
  return {
    attribute: transformAttributes(manualQAStats.attribute),
    evaluatedConversations:
      manualQAStats.evaluatedConversations?.map((v) => transformManualQAConversationInfo(v)) ?? [],
    evaluatedAgentsCount: manualQAStats.evaluatedAgentsCount,
    avgPerfScore: isNonNaScore(manualQAStats.avgPerfScore) ? manualQAStats.avgPerfScore : undefined,
    completionRate: manualQAStats.completionRate,
    overwrittenCriteriaCount: manualQAStats.overwrittenCriteriaCount,
    criteriaOverwrittenConversations:
      manualQAStats.criteriaOverwrittenConversations?.map((v) => transformManualQAConversationInfo(v)) ?? [],
    autoFailedConversations:
      manualQAStats.autoFailedConversations?.map((v) => transformManualQAConversationInfo(v)) ?? [],
    commentedConversations:
      manualQAStats.commentedConversations?.map((v) => transformManualQAConversationInfo(v)) ?? [],
    assignedConversationCount: manualQAStats.assignedConversationCount,
    evaluatedConversationCount: manualQAStats.evaluatedConversationCount,
    evaluationPeriodProgressStatus: manualQAStats.evaluationPeriodProgressStatus,
    avgConsistencyScore: isNonNaScore(manualQAStats.avgConsistencyScore)
      ? manualQAStats.avgConsistencyScore
      : undefined,
    calibratedConversation:
      manualQAStats.calibratedConversations?.map((v) => transformManualQAConversationInfo(v)) ?? [],
    acknowledgedConversation:
      manualQAStats.acknowledgedConversations?.map((v) => transformManualQAConversationInfo(v)) ?? [],
    skipSuggestedConversationCount: manualQAStats.skipSuggestedConversationCount,
    publishedConversations:
      manualQAStats.publishedConversations?.map((v) => transformManualQAConversationInfo(v)) ?? [],
  };
}

export function transformManualQAStatsResponse(response: RetrieveManualQAStatsResponse): ManualQAStatsResponse {
  return {
    manualQaStats: response.manualQaStats?.map((v) => transformManualQAStats(v)) || [],
  };
}

export function transformManualQAProgressResponse(
  response: RetrieveManualQAProgressResponse
): ManualQAProgressResponse {
  return {
    manualQaProgresses: response.manualQaProgresses?.map((v) => transformManualQAProgress(v)) || [],
  };
}

export function transformManualQAProgress(manaualQAProgress: CrestaV1ManualQAProgress): ManualQAProgress {
  return {
    attribute: transformAttributes(manaualQAProgress.attribute),
    evaluatedConversationCount: manaualQAProgress.evaluatedConversationCount ?? 0,
    lastQaAnalyst: manaualQAProgress.lastQaAnalyst
      ? transformApiUserToModel(manaualQAProgress.lastQaAnalyst)
      : undefined,
    lastEvaluatedTime: dayjsIfTruthy(manaualQAProgress.lastEvaluatedTime),
    evaluatedConversations:
      manaualQAProgress.evaluatedConversations?.map((v) => transformManualQAConversationInfo(v)) ?? [],
    assignedConversationCount: manaualQAProgress.assignedConversationCount ?? 0,
    agentHandledConversationCount: manaualQAProgress.handledConversationsCount,
  };
}

export function transformSuggestManualQAConversationResponse(
  response: CrestaV1SuggestManualQAConversationsResponse
): SuggestManualQAConversationsResponse {
  return {
    suggestedConversations: response.suggestedConversations?.map((v) => transformManualQAConversationInfo(v)) ?? [],
  };
}

function transformTeamUsecaseConversationMapping(
  value: CrestaV1TeamUsecaseConversationMapping
): TeamUsecaseConversationCount {
  return {
    count: value.count || 0,
    team: value.team,
    usecase: value.usecase,
  };
}

export function transformTeamUsecaseConversationMappingResponse(
  response: CrestaV1RetrieveTeamUsecaseConversationMappingsResponse
): TeamUsecaseConversationMappingResponse {
  return {
    teamUseCaseMappings: response.mappings?.map(transformTeamUsecaseConversationMapping) || [],
  };
}

function transformCoachingEfficiencyStats(stats: CrestaV1CoachingEfficiencyStats): CoachingEfficiencyStats {
  return {
    ...stats,
    coachingEfficiency: stats.isEfficiencyScoreInvalid ? undefined : stats.coachingEfficiency,
    attribute: stats.attribute ? transformAttributes(stats.attribute) : undefined,
  };
}

export function transformCoachingEfficiencyStatsResponse(
  response: CrestaV1RetrieveCoachingEfficiencyStatsResponse
): RetrieveCoachingEfficiencyStatsResponse {
  return {
    coachingEfficiencyStats: response.coachingEfficiencyStats?.length
      ? response.coachingEfficiencyStats.map((v) => transformCoachingEfficiencyStats(v))
      : undefined,
    totalCoachingEfficiency: response.totalCoachingEfficiency
      ? transformCoachingEfficiencyStats(response.totalCoachingEfficiency)
      : undefined,
  };
}
