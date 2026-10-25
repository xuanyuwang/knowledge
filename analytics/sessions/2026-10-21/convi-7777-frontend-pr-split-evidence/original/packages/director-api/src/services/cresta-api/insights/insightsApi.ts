import type { Dayjs } from 'dayjs';

import type {
  Attribute,
  AttributeStructure,
  BatchDeleteInsightsTopicsRequest,
  Metadata as CrestaV1Metadata,
  RetrieveMetadataValuesRequest as CrestaV1RetrieveMetadataValuesRequest,
  EvaluateAnalyticsChartRequest,
  Frequency,
  GenerateInsightsTopicRequest,
  ModifyInsightsTopicRequest,
  RetrieveAdherencesRequest,
  RetrieveAgentStatsRequest,
  RetrieveAnalyticsChartExamplesRequest,
  RetrieveAnnotatedTimeSeriesRequest,
  RetrieveAppealStatsRequest,
  RetrieveAppealStatsResponse,
  RetrieveAssistanceStatsRequest,
  RetrieveChatBotConversationStatsRequest,
  RetrieveClosedConversationsRequest,
  RetrieveCoachingEfficiencyStatsRequest,
  RetrieveCommentStatsRequest,
  RetrieveConversationMessageStatsRequest,
  RetrieveConversationMessagesRequest,
  RetrieveConversationOutcomeStatsRequest,
  RetrieveConversationStatsRequest,
  RetrieveDirectorTaskStatsRequest,
  RetrieveDirectorTaskStatsResponse,
  RetrieveGuidedWorkflowStatsRequest,
  RetrieveHintStatsRequest,
  RetrieveInsightsTopicsRequest,
  RetrieveKnowledgeAssistStatsRequest,
  RetrieveKnowledgeBaseStatsRequest,
  RetrieveLiveAssistStatsRequest,
  RetrieveManualQAProgressRequest,
  RetrieveManualQAStatsRequest,
  RetrieveNoteTakingStatsRequest,
  RetrievePartialConversationsRequest,
  RetrievePartialConversationsResponse,
  RetrieveQAConversationsRequest,
  RetrieveQAScoreStatsRequest,
  RetrieveReceptivitiesRequest,
  RetrieveScorecardStatsRequest,
  RetrieveSmartComposeStatsRequest,
  RetrieveSuggestionStatsRequest,
  RetrieveSummarizationStatsRequest,
  RetrieveTeamUsecaseConversationMappingsRequest,
  SearchKeywordsMessagesResponse,
  SuggestManualQAConversationsRequest,
} from '@cresta/web-client';
import { AnalyticsService, AttributeType, fetchReq, KeywordsService, replacer } from '@cresta/web-client';
import type {
  RetrieveLlmAnnotationValuesRequest as CrestaV1RetrieveLlmAnnotationValuesRequest,
  RetrieveCustomerSnapshotStatsRequest,
  RetrieveUserOutcomeStatsRequest,
  RetrieveUserOutcomeStatsResponse,
} from '@cresta/web-client/v1/analytics';
import { UserOutcomeService } from '@cresta/web-client/v1/analytics';
import type {
  AppUsageSummary,
  CreateBackfillJobRequest,
  GetAppUsageSummaryRequest,
  GetConversationAppActivityRequest,
  ListAgentConversationsForAppRequest,
  ListAgentConversationsForAppResponse,
  ListAgentsAppUsageRequest,
  ListAgentsAppUsageResponse,
  ListAgentsUsageForAppRequest,
  ListAgentsUsageForAppResponse,
  ListAppsUsageOverTimeRequest,
  ListAppsUsageOverTimeResponse,
  PreviewBackfillRequest,
} from '@cresta/web-client/v1/screenanalytics';
import { ScreenAnalyticsInsightsService } from '@cresta/web-client/v1/screenanalytics';

import type { CancellableFetch } from '../../../utils';
import type { BackfillPreview, ConversationAppActivity, ScreenAnalyticsBackfillJob } from '../screenanalytics';
import { transformBackfillJob, transformConversationAppActivity, transformPreviewBackfill } from '../screenanalytics';
import { CrestaApiBaseService, CrestaApiServiceId, getStreamingApiEndpoint } from '../shared';

import type {
  AdherenceStatsResponse,
  AgentStatsResponse,
  AnalyticsChartEvaluation,
  AnalyticsChartExamplesResponse,
  AnnotatedTimeSeriesResponse,
  AssistanceStatsResponse,
  ChatbotConversationStatsResponse,
  ChatbotSessionStatsResponse,
  ClosedConversationsResponse,
  CoachingSessionStatsResponse,
  CommentStatsResponse,
  ConversationAssistanceUsageStatsApiResponse,
  ConversationAssistanceUsageStatsResponse,
  ConversationDurationBucketsResponse,
  ConversationMessageStatsResponse,
  ConversationMessagesResponse,
  ConversationOutcomeStats,
  ConversationStatsResponse,
  CustomerSnapshotStatsResponse,
  GuidedWorkflowStatsResponse,
  HintEngagementResponse,
  HintStatsResponse,
  InsightsTopic,
  KeywordsStatsResponse,
  KnowledgeAssistStatsResponse,
  KnowledgeBaseStatsResponse,
  LiveAssistStatsResponse,
  LlmAnnotationValuesResponse,
  ManagerStatsResponse,
  ManualQAProgressResponse,
  ManualQAStatsResponse,
  MetadataValuesResponse,
  NoteTakingStatsResponse,
  QAConversationsResponse,
  QAScoreStatsResponse,
  RetrieveCoachingEfficiencyStatsResponse,
  ScorecardStatsResponse,
  SmartComposeStatsResponse,
  SuggestionStatsResponse,
  SuggestManualQAConversationsResponse,
  SummarizationStatsResponse,
  TeamUsecaseConversationMappingResponse,
} from './apiTypes';
import {
  transformAdherenceStatsResponse,
  transformAgentStatsResponse,
  transformAnnotatedTimeSeriesResponse,
  transformAssistanceStatsResponse,
  transformChatbotConversationStats,
  transformChatbotSessionStats,
  transformClosedConversations,
  transformCoachingEfficiencyStatsResponse,
  transformCoachingSessionStats,
  transformCommentStats,
  transformConversationAssistanceUsageStatsResponse,
  transformConversationBucketListResponse,
  transformConversationMessageStats,
  transformConversationMessages,
  transformConversationOutcomeStatsResponse,
  transformConversationStatsResponse,
  transformCustomerSnapshotStatsResponse,
  transformGuidedWorkflowStatsResponse,
  transformHintStats,
  transformInsightsTopic,
  transformInsightsTopics,
  transformKeywordsStats,
  transformKnowledgeAssistStatsResponse,
  transformKnowledgeBaseStatsResponse,
  transformLiveAssistStatsResponse,
  transformLlmAnnotationValues,
  transformManagerStatsResponse,
  transformManualQAProgressResponse,
  transformManualQAStatsResponse,
  transformMetadataValues,
  transformNoteTakingStatsResponse,
  transformReceptivities,
  transformScorecardStats,
  transformSmartComposeStatsResponse,
  transformSuggestionStatsResponse,
  transformSuggestManualQAConversationResponse,
  transformSummarizationStatsResponse,
  transformTeamUsecaseConversationMappingResponse,
} from './transformers';
import { transformQAConversationsResponse, transformQAScoreStatsResponse } from './transformersQAI';
import type {
  ChatbotRequestParams,
  CustomerInsightsRequestParams,
  InsightsAPICommonPayload,
  InsightsRequestParams,
  QAConversationsRequest,
  QAScoreStatsRequestParams,
} from './types';
import { toTimeRange } from './utils';

export class InsightsApi extends CrestaApiBaseService {
  public readonly identifier = CrestaApiServiceId.InsightsApi;

  protected get port(): string {
    return process.env.REACT_APP_INSIGHTSSERVER_PORT ?? '9091';
  }

  private getRequestPayload(
    requestParams: InsightsRequestParams
  ):
    | RetrieveConversationStatsRequest
    | RetrieveReceptivitiesRequest
    | RetrieveAssistanceStatsRequest
    | RetrieveHintStatsRequest
    | RetrieveAdherencesRequest
    | RetrieveAgentStatsRequest
    | RetrieveGuidedWorkflowStatsRequest
    | RetrieveScorecardStatsRequest
    | RetrieveCommentStatsRequest
    | RetrieveKnowledgeAssistStatsRequest
    | RetrieveCustomerSnapshotStatsRequest
    | RetrieveLiveAssistStatsRequest
    | RetrieveSuggestionStatsRequest
    | RetrieveSmartComposeStatsRequest
    | RetrieveKnowledgeBaseStatsRequest
    | RetrieveNoteTakingStatsRequest
    | RetrieveSummarizationStatsRequest {
    const attributeTypes: AttributeType[] = requestParams.groupByAttributeTypes.map(
      (attributeStructure: AttributeStructure): AttributeType => {
        return attributeStructure.attributeType || AttributeType.ATTRIBUTE_TYPE_UNSPECIFIED;
      }
    );
    const commonPayload = this.getCommonPayload(requestParams);
    return {
      ...commonPayload,
      frequency: requestParams.frequency,
      groupByAttributeTypes: attributeTypes,
      filterByAttribute: requestParams.filterByAttribute,
      dashboard: requestParams.dashboard,
      includePeerUserStats: requestParams.includePeerUserStats,
      filterToAgentsOnly: requestParams.filterToAgentsOnly,
    };
  }

  private getCommonPayload(
    requestParams: InsightsRequestParams | CustomerInsightsRequestParams
  ): InsightsAPICommonPayload {
    return {
      parent: this.buildResourceName(),
      profileId: this.profileId,
      filterByTimeRange: toTimeRange(requestParams.dateRange[0], requestParams.dateRange[1]),
      metadata: {
        ...this.getTimezoneMetadata(),
        ...requestParams.metadata,
      },
    };
  }

  private getTimezoneMetadata(): CrestaV1Metadata {
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    return {
      timeZoneId: this.timezone || tz,
    };
  }

  private getRequestPayloadForConversationMessageStats(
    requestParams: CustomerInsightsRequestParams
  ): RetrieveConversationMessageStatsRequest {
    const commonPayload = this.getCommonPayload(requestParams);
    return {
      ...commonPayload,
      frequency: requestParams.frequency,
      groupByAttributes: requestParams.groupByAttributeTypes,
      filterConversationByAttribute: requestParams.filterConversationByAttribute,
      filterMessageByAttribute: requestParams.filterMessageByAttribute,
      filterConversationDocumentByConversationByAttribute:
        requestParams.filterConversationDocumentByConversationByAttribute,
      elasticsearchQueryConfig: requestParams.elasticsearchQueryConfig,
    };
  }

  private getRequestPayloadForConversationMessages(
    requestParams: CustomerInsightsRequestParams
  ): RetrieveConversationMessagesRequest {
    const commonPayload = this.getCommonPayload(requestParams);
    const payload: RetrieveConversationMessagesRequest & Pick<CustomerInsightsRequestParams, 'skip'> = {
      ...commonPayload,
      filterConversationByAttribute: requestParams.filterConversationByAttribute,
      filterMessageByAttribute: requestParams.filterMessageByAttribute,
      filterConversationDocumentByConversationByAttribute:
        requestParams.filterConversationDocumentByConversationByAttribute,
      populateMomentMatchOnTheFly: requestParams.populateMomentMatchOnTheFly,
      skip: requestParams.skip,
      pageSize: requestParams.pageSize,
      pageToken: requestParams.pageToken,
      elasticsearchQueryConfig: requestParams.elasticsearchQueryConfig,
      sortOption: requestParams.sortOption,
      seedForRandomSort: requestParams.seedForRandomSort,
    };
    return payload;
  }

  private getRequestPayloadForRetrieveQAScoreStats(
    requestParams: QAScoreStatsRequestParams
  ): RetrieveQAScoreStatsRequest {
    const { includePeerUserStats, metadata, ...rest } = requestParams;

    return {
      includePeerUserStats,
      parent: this.buildResourceName(),
      ...rest,
      metadata: {
        ...this.getTimezoneMetadata(),
        ...metadata,
      },
    };
  }

  private getRequestPayloadForRetrieveQAConversations(
    requestParams: QAConversationsRequest
  ): RetrieveQAConversationsRequest {
    const { metadata, ...rest } = requestParams;

    return {
      parent: this.buildResourceName(),
      ...rest,
      metadata: {
        ...this.getTimezoneMetadata(),
        ...metadata,
      },
    };
  }

  public retrieveConversationStats(requestParams: InsightsRequestParams): CancellableFetch<ConversationStatsResponse> {
    const payload: RetrieveConversationStatsRequest = {
      ...this.getRequestPayload(requestParams),
      conversationTimeRangeField: requestParams.conversationTimeRangeField,
    };
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveConversationStats(payload, init),
      (v) => transformConversationStatsResponse(v)
    );
  }

  public retrieveConversationAssistanceUsageStats(
    requestParams: InsightsRequestParams
  ): CancellableFetch<ConversationAssistanceUsageStatsResponse> {
    const { parent, filterByTimeRange, metadata } = this.getCommonPayload(requestParams);
    const payload = {
      parent,
      filterByTimeRange,
      metadata,
      filterByAttribute: requestParams.filterByAttribute,
      frequency: requestParams.frequency,
      groupByAttributeTypes: requestParams.groupByAttributeTypes.map(({ attributeType }) => attributeType),
      includePeerUserStats: requestParams.includePeerUserStats,
      filterToAgentsOnly: requestParams.filterToAgentsOnly,
    };
    return this.wrapRequest(
      (init) =>
        fetchReq<typeof payload, ConversationAssistanceUsageStatsApiResponse>(
          `/v1/${parent}/conversationAssistanceUsageStats:retrieve`,
          { ...init, method: 'POST', body: JSON.stringify(payload, replacer) }
        ),
      transformConversationAssistanceUsageStatsResponse
    );
  }

  public retrieveAgentStats(payloadParams: InsightsRequestParams): CancellableFetch<AgentStatsResponse> {
    const payload = this.getRequestPayload(payloadParams);
    payload.parent = this.buildCustomerResourceName();
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveAgentStats(payload, init),
      (v) => transformAgentStatsResponse(v)
    );
  }

  public retrieveManagerStats(payloadParams: InsightsRequestParams): CancellableFetch<ManagerStatsResponse> {
    const payload = this.getRequestPayload(payloadParams);
    payload.parent = this.buildCustomerResourceName();
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveManagerStats(payload, init),
      (v) => transformManagerStatsResponse(v)
    );
  }

  public retrieveAssistanceStats(req: InsightsRequestParams): CancellableFetch<AssistanceStatsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveAssistanceStats(this.getRequestPayload(req), init),
      (v) => transformAssistanceStatsResponse(v)
    );
  }

  public retrieveSuggestionStats(req: InsightsRequestParams): CancellableFetch<SuggestionStatsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveSuggestionStats(this.getRequestPayload(req), init),
      (v) => transformSuggestionStatsResponse(v)
    );
  }

  public retrieveSmartComposeStats(req: InsightsRequestParams): CancellableFetch<SmartComposeStatsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveSmartComposeStats(this.getRequestPayload(req), init),
      (v) => transformSmartComposeStatsResponse(v)
    );
  }

  public retrieveKnowledgeBaseStats(req: InsightsRequestParams): CancellableFetch<KnowledgeBaseStatsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveKnowledgeBaseStats(this.getRequestPayload(req), init),
      (v) => transformKnowledgeBaseStatsResponse(v)
    );
  }

  public retrieveNoteTakingStats(req: InsightsRequestParams): CancellableFetch<NoteTakingStatsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveNoteTakingStats(this.getRequestPayload(req), init),
      (v) => transformNoteTakingStatsResponse(v)
    );
  }

  public retrieveSummarizationStats(req: InsightsRequestParams): CancellableFetch<SummarizationStatsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveSummarizationStats(this.getRequestPayload(req), init),
      (v) => transformSummarizationStatsResponse(v)
    );
  }

  public listConversationDurationBuckets(): CancellableFetch<ConversationDurationBucketsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.ListConversationDurationBuckets({ parent: this.buildResourceName() }, init),
      transformConversationBucketListResponse
    );
  }

  public retrieveAdherenceStats(req: InsightsRequestParams): CancellableFetch<AdherenceStatsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveAdherences(this.getRequestPayload(req), init),
      (v) => transformAdherenceStatsResponse(v)
    );
  }

  public retrieveHintStats(req: InsightsRequestParams): CancellableFetch<HintStatsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveHintStats(this.getRequestPayload(req), init),
      (v) => transformHintStats(v)
    );
  }

  public retrieveHintEngagement(req: InsightsRequestParams): CancellableFetch<HintEngagementResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveReceptivities(this.getRequestPayload(req), init),
      (v) => transformReceptivities(v)
    );
  }

  public retrieveConversationMessageStats(
    req: CustomerInsightsRequestParams
  ): CancellableFetch<ConversationMessageStatsResponse> {
    return this.wrapRequest(
      (init) =>
        AnalyticsService.RetrieveConversationMessageStats(this.getRequestPayloadForConversationMessageStats(req), init),
      transformConversationMessageStats
    );
  }

  public retrieveConversationMessages(
    req: CustomerInsightsRequestParams
  ): CancellableFetch<ConversationMessagesResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveConversationMessages(this.getRequestPayloadForConversationMessages(req), init),
      (v) => transformConversationMessages(v)
    );
  }

  public retrieveQAScoreStats(
    req: QAScoreStatsRequestParams,
    chapterId?: string
  ): CancellableFetch<QAScoreStatsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveQAScoreStats(this.getRequestPayloadForRetrieveQAScoreStats(req), init),
      (response) => ({ ...transformQAScoreStatsResponse(response), chapterId: chapterId })
    );
  }

  /**
   * This might also be used for process scorecards. Then it doesn't return conversations.
   */
  public retrieveQAConversations(req: QAConversationsRequest): CancellableFetch<QAConversationsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveQAConversations(this.getRequestPayloadForRetrieveQAConversations(req), init),
      (v) => transformQAConversationsResponse(v)
    );
  }

  public retrieveGuidedWorkflowStats(req: InsightsRequestParams): CancellableFetch<GuidedWorkflowStatsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveGuidedWorkflowStats(this.getRequestPayload(req), init),
      (v) => transformGuidedWorkflowStatsResponse(v)
    );
  }

  public retrieveMetadataValues(
    requestParams: CrestaV1RetrieveMetadataValuesRequest
  ): CancellableFetch<MetadataValuesResponse> {
    const req: CrestaV1RetrieveMetadataValuesRequest = {
      parent: this.buildResourceName(),
      ...requestParams,
    };

    return this.wrapRequest((init) => AnalyticsService.RetrieveMetadataValues(req, init), transformMetadataValues);
  }

  public retrieveLlmAnnotationValues(
    requestParams: CrestaV1RetrieveLlmAnnotationValuesRequest
  ): CancellableFetch<LlmAnnotationValuesResponse> {
    const req: CrestaV1RetrieveLlmAnnotationValuesRequest = {
      parent: this.buildResourceName(),
      ...requestParams,
    };

    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveLlmAnnotationValues(req, init),
      transformLlmAnnotationValues
    );
  }

  public retrieveKeywordsStats(
    filterByAttribute: Attribute,
    start: Dayjs,
    end: Dayjs,
    keywordsExpression: string,
    frequency: string
  ): CancellableFetch<KeywordsStatsResponse> {
    return this.wrapRequest(
      (init) =>
        KeywordsService.RetrieveKeywordsStats(
          {
            parent: this.buildResourceName(),
            filterByTimeRange: toTimeRange(start, end),
            filterByAttribute: filterByAttribute,
            frequency: frequency.toUpperCase() as Frequency,
            keywordsExpression: keywordsExpression,
          },
          init
        ),
      (v) => transformKeywordsStats(v)
    );
  }

  public searchKeywordsMessages(
    filterByAttribute: Attribute,
    start: Dayjs,
    end: Dayjs,
    keywordsExpression: string,
    pageSize: number
  ): CancellableFetch<SearchKeywordsMessagesResponse> {
    return this.wrapRequest((init) =>
      KeywordsService.SearchKeywordsMessages(
        {
          parent: this.buildResourceName(),
          filterByTimeRange: toTimeRange(start, end),
          filterByAttribute: filterByAttribute,
          keywordsExpression: keywordsExpression,
          pageSize: pageSize,
        },
        init
      )
    );
  }

  public retrieveChatbotConversationStats(
    req: ChatbotRequestParams
  ): CancellableFetch<ChatbotConversationStatsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveChatBotConversationStats(this.getChatbotAPIRequestParams(req), init),
      transformChatbotConversationStats
    );
  }

  public retrieveChatBotSessionStats(req: ChatbotRequestParams): CancellableFetch<ChatbotSessionStatsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveChatBotSessionStats(this.getChatbotAPIRequestParams(req), init),
      transformChatbotSessionStats
    );
  }

  private getChatbotAPIRequestParams(requestParams: ChatbotRequestParams): RetrieveChatBotConversationStatsRequest {
    return {
      parent: this.buildResourceName(),
      filterByTimeRange: toTimeRange(requestParams.dateRange[0], requestParams.dateRange[1]),
      filterByChatBotAttribute: requestParams.filterByAttribute,
      groupByChatBotAttributeTypes: requestParams.groupByAttributeTypes,
      mode: requestParams.mode,
      metadata: this.getTimezoneMetadata(),
    };
  }

  public retrieveCommentsStats(req: InsightsRequestParams): CancellableFetch<CommentStatsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveCommentStats(this.getRequestPayload(req), init),
      (v) => transformCommentStats(v)
    );
  }

  public retrieveLiveAssistStats(payloadParams: InsightsRequestParams): CancellableFetch<LiveAssistStatsResponse> {
    const payload = this.getRequestPayload(payloadParams);
    payload.parent = this.buildResourceName();
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveLiveAssistStats(payload, init),
      (v) => transformLiveAssistStatsResponse(v)
    );
  }

  public retrieveScorecardStats(req: InsightsRequestParams): CancellableFetch<ScorecardStatsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveScorecardStats(this.getRequestPayload(req), init),
      (v) => transformScorecardStats(v)
    );
  }

  public retrieveCoachingSessionStats(
    requestParams: InsightsRequestParams
  ): CancellableFetch<CoachingSessionStatsResponse> {
    const payload = {
      ...this.getRequestPayload(requestParams),
      coachingSessionFilter: requestParams.coachingSessionFilter,
    };
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveCoachingSessionStats(payload, init),
      (v) => transformCoachingSessionStats(v)
    );
  }

  public retrieveKnowledgeAssistStats(req: InsightsRequestParams): CancellableFetch<KnowledgeAssistStatsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveKnowledgeAssistStats(this.getRequestPayload(req), init),
      (v) => transformKnowledgeAssistStatsResponse(v)
    );
  }

  public retrieveCustomerSnapshotStats(req: InsightsRequestParams): CancellableFetch<CustomerSnapshotStatsResponse> {
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveCustomerSnapshotStats(this.getRequestPayload(req), init),
      (v) => transformCustomerSnapshotStatsResponse(v)
    );
  }

  public retrieveClosedConversations(
    requestParams?: RetrieveClosedConversationsRequest
  ): CancellableFetch<ClosedConversationsResponse> {
    return this.wrapRequest(
      (init) =>
        AnalyticsService.RetrieveClosedConversations(
          {
            parent: this.buildResourceName(),
            ...requestParams,
            metadata: this.getTimezoneMetadata(),
          },
          init
        ),
      (v) => transformClosedConversations(v)
    );
  }

  public generateInsightsTopic(requestParams: GenerateInsightsTopicRequest): CancellableFetch<InsightsTopic> {
    return this.wrapRequest(
      (init) =>
        AnalyticsService.GenerateInsightsTopic(
          {
            ...requestParams,
            parent: this.buildResourceName(),
          },
          init
        ),
      transformInsightsTopic
    );
  }

  public retrieveInsightsTopic(requestParams: RetrieveInsightsTopicsRequest): CancellableFetch<InsightsTopic[]> {
    return this.wrapRequest(
      (init) =>
        AnalyticsService.RetrieveInsightsTopics(
          {
            ...requestParams,
            parent: this.buildResourceName(),
          },
          init
        ),
      (v) => transformInsightsTopics(v)
    );
  }

  public modifyInsightsTopic(requestParams: ModifyInsightsTopicRequest): CancellableFetch<InsightsTopic> {
    return this.wrapRequest(
      (init) =>
        AnalyticsService.ModifyInsightsTopic(
          {
            ...requestParams,
            parent: this.buildResourceName(),
          },
          init
        ),
      transformInsightsTopic
    );
  }

  public batchDeleteInsightsTopic(requestParams: BatchDeleteInsightsTopicsRequest): CancellableFetch<InsightsTopic[]> {
    return this.wrapRequest(
      (init) =>
        AnalyticsService.BatchDeleteInsightsTopics(
          {
            ...requestParams,
            parent: this.buildResourceName(),
          },
          init
        ),
      (v) => transformInsightsTopics(v)
    );
  }

  public retrieveAnnotatedTimeSeries(
    requestParams: RetrieveAnnotatedTimeSeriesRequest
  ): CancellableFetch<AnnotatedTimeSeriesResponse> {
    return this.wrapRequest(
      (init) =>
        AnalyticsService.RetrieveAnnotatedTimeSeries(
          {
            ...requestParams,
            parent: this.buildResourceName(),
          },
          init
        ),
      (v) => transformAnnotatedTimeSeriesResponse(v)
    );
  }

  public retrieveConversationOutcomeStats(
    requestParams: RetrieveConversationOutcomeStatsRequest
  ): CancellableFetch<ConversationOutcomeStats> {
    return this.wrapRequest(
      (init) =>
        AnalyticsService.RetrieveConversationOutcomeStats(
          {
            ...requestParams,
            parent: this.buildResourceName(),
          },
          init
        ),
      (v) => transformConversationOutcomeStatsResponse(v)
    );
  }

  public retrieveManualQAStats(requestParams: RetrieveManualQAStatsRequest): CancellableFetch<ManualQAStatsResponse> {
    return this.wrapRequest(
      (init) =>
        AnalyticsService.RetrieveManualQAStats(
          {
            ...requestParams,
            metadata: this.getTimezoneMetadata(),
            parent: this.buildResourceName(),
          },
          init
        ),
      (v) => transformManualQAStatsResponse(v)
    );
  }

  public retrieveAppealStats(requestParams: RetrieveAppealStatsRequest): CancellableFetch<RetrieveAppealStatsResponse> {
    return this.wrapRequest((init) =>
      AnalyticsService.RetrieveAppealStats(
        {
          ...requestParams,
          metadata: this.getTimezoneMetadata(),
          parent: this.buildResourceName(),
        },
        init
      )
    );
  }

  public retrieveDirectorTaskStats(
    requestParams: RetrieveDirectorTaskStatsRequest
  ): CancellableFetch<RetrieveDirectorTaskStatsResponse> {
    return this.wrapRequest((init) =>
      AnalyticsService.RetrieveDirectorTaskStats(
        {
          parent: this.buildResourceName(),
          ...requestParams,
          metadata: this.getTimezoneMetadata(),
        },
        init
      )
    );
  }

  public retrieveManualQAProgress(
    requestParams: RetrieveManualQAProgressRequest
  ): CancellableFetch<ManualQAProgressResponse> {
    return this.wrapRequest(
      (init) =>
        AnalyticsService.RetrieveManualQAProgress(
          {
            ...requestParams,
            parent: this.buildResourceName(),
          },
          init
        ),
      (v) => transformManualQAProgressResponse(v)
    );
  }

  public suggestManualQAConversations(
    requestParams: SuggestManualQAConversationsRequest
  ): CancellableFetch<SuggestManualQAConversationsResponse> {
    return this.wrapRequest(
      (init) =>
        AnalyticsService.SuggestManualQAConversations(
          {
            ...requestParams,
            parent: this.buildResourceName(),
          },
          init
        ),
      (v) => transformSuggestManualQAConversationResponse(v)
    );
  }

  public retrieveTeamUsecaseConversationMapping(
    requestParams: RetrieveTeamUsecaseConversationMappingsRequest
  ): CancellableFetch<TeamUsecaseConversationMappingResponse> {
    const req = {
      ...requestParams,
      metadata: this.getTimezoneMetadata(),
    };
    return this.wrapRequest(
      (init) => AnalyticsService.RetrieveTeamUsecaseConversationMappings(req, init),
      transformTeamUsecaseConversationMappingResponse
    );
  }

  public retrieveCoachingEfficiencyStats(
    request: RetrieveCoachingEfficiencyStatsRequest
  ): CancellableFetch<RetrieveCoachingEfficiencyStatsResponse> {
    return this.wrapRequest(
      (init) =>
        AnalyticsService.RetrieveCoachingEfficiencyStats(
          {
            ...request,
            metadata: this.getTimezoneMetadata(),
            parent: this.buildResourceName(),
          },
          init
        ),
      (v) => transformCoachingEfficiencyStatsResponse(v)
    );
  }

  public evaluateAnalyticsChart(request: EvaluateAnalyticsChartRequest): CancellableFetch<AnalyticsChartEvaluation> {
    const defaultMetadata = this.getTimezoneMetadata();
    return this.wrapRequest(
      (init) => {
        if (this.streamingApiEnabled && this.cluster && this.cluster !== 'local') {
          init.pathPrefix = getStreamingApiEndpoint(this.cluster);
        }
        return AnalyticsService.EvaluateAnalyticsChart(
          {
            parent: this.buildResourceName(),
            ...request,
            metadata: {
              ...defaultMetadata,
              ...request.metadata,
              timeZoneId: request.metadata?.timeZoneId ?? defaultMetadata.timeZoneId,
            },
          },
          init
        );
      },
      (result) => ({
        analyticsResults: result.resultForChart?.analyticsResults,
        updateTimestamp: result.updateTimestamp,
      })
    );
  }

  public retrieveAnalyticsChartExamples(
    request: RetrieveAnalyticsChartExamplesRequest
  ): CancellableFetch<AnalyticsChartExamplesResponse> {
    const defaultMetadata = this.getTimezoneMetadata();
    return this.wrapRequest((init) =>
      AnalyticsService.RetrieveAnalyticsChartExamples(
        {
          parent: this.buildResourceName(),
          ...request,
          metadata: {
            ...defaultMetadata,
            ...request.metadata,
            timeZoneId: request.metadata?.timeZoneId ?? defaultMetadata.timeZoneId,
          },
        },
        init
      )
    );
  }

  public retrievePartialConversations(
    request: RetrievePartialConversationsRequest
  ): CancellableFetch<RetrievePartialConversationsResponse> {
    return this.wrapRequest((init) =>
      AnalyticsService.RetrievePartialConversations(
        {
          parent: this.buildResourceName(),
          ...request,
        },
        init
      )
    );
  }

  public retrieveUserOutcomeStats(
    requestParams: RetrieveUserOutcomeStatsRequest
  ): CancellableFetch<RetrieveUserOutcomeStatsResponse> {
    return this.wrapRequest((init) =>
      UserOutcomeService.RetrieveUserOutcomeStats(
        {
          parent: this.buildResourceName(),
          metadata: this.getTimezoneMetadata(),
          ...requestParams,
        },
        init
      )
    );
  }

  // --- Screen analytics insights ---

  // Per-agent/per-team focus-time table. `parent` resolved here; callers never build it.
  public listAgentsAppUsage(
    request: Omit<ListAgentsAppUsageRequest, 'parent'>
  ): CancellableFetch<ListAgentsAppUsageResponse> {
    const fullRequest: ListAgentsAppUsageRequest = { parent: this.buildResourceName(), ...request };
    return this.wrapRequest((init) => ScreenAnalyticsInsightsService.ListAgentsAppUsage(fullRequest, init));
  }

  // Usage Insights card: KPI totals + stacked area series. `parent` resolved here; callers
  // never build it. The response maps to the UI model in the app's overview hook.
  public getAppUsageSummary(request: Omit<GetAppUsageSummaryRequest, 'parent'>): CancellableFetch<AppUsageSummary> {
    const fullRequest: GetAppUsageSummaryRequest = { parent: this.buildResourceName(), ...request };
    return this.wrapRequest((init) => ScreenAnalyticsInsightsService.GetAppUsageSummary(fullRequest, init));
  }

  // App Usage conversations drawer. `parent` resolved here. A drawer agent (bare `userId`)
  // scopes the filter to that one agent; the server unions filter.users with filter.groups, so
  // both are overwritten. Omit `userId` for the all-agents drawer, which keeps the page scope.
  public listAgentConversationsForApp(
    request: Omit<ListAgentConversationsForAppRequest, 'parent'> & { userId?: string }
  ): CancellableFetch<ListAgentConversationsForAppResponse> {
    const { userId, ...rest } = request;
    const fullRequest: ListAgentConversationsForAppRequest = {
      parent: this.buildResourceName(),
      ...rest,
    };
    if (userId) {
      fullRequest.filter = {
        ...fullRequest.filter,
        users: [this.buildCustomerResourceName('users', userId)],
        groups: [],
      };
    }
    return this.wrapRequest((init) => ScreenAnalyticsInsightsService.ListAgentConversationsForApp(fullRequest, init));
  }

  // Closed conversation app-activity: focus spans grouped by classifying rule, bounds rebased
  // to ms relative to conversation start.
  public getConversationAppActivity(conversationResourceName: string): CancellableFetch<ConversationAppActivity> {
    const request: GetConversationAppActivityRequest = {
      conversation: conversationResourceName,
    };
    return this.wrapRequest(
      (init) => ScreenAnalyticsInsightsService.GetConversationAppActivity(request, init),
      transformConversationAppActivity
    );
  }

  // Backfill impact preview (CONVI-7331). `parent` is resolved here, so callers pass only the range.
  public previewBackfill(request: Omit<PreviewBackfillRequest, 'parent'>): CancellableFetch<BackfillPreview> {
    const fullRequest: PreviewBackfillRequest = { parent: this.buildResourceName(), ...request };
    return this.wrapRequest(
      (init) => ScreenAnalyticsInsightsService.PreviewBackfill(fullRequest, init),
      transformPreviewBackfill
    );
  }

  // Starts a backfill (CONVI-7331), re-classifying historical spans against the profile's current
  // rules. One per profile. A second create returns FAILED_PRECONDITION.
  public createBackfillJob(
    request: Omit<CreateBackfillJobRequest, 'parent'>
  ): CancellableFetch<ScreenAnalyticsBackfillJob> {
    const fullRequest: CreateBackfillJobRequest = { parent: this.buildResourceName(), ...request };
    return this.wrapRequest(
      (init) => ScreenAnalyticsInsightsService.CreateBackfillJob(fullRequest, init),
      transformBackfillJob
    );
  }

  public listAgentsUsageForApp(
    request: Omit<ListAgentsUsageForAppRequest, 'parent'>
  ): CancellableFetch<ListAgentsUsageForAppResponse> {
    const fullRequest: ListAgentsUsageForAppRequest = { ...request, parent: this.buildResourceName() };
    return this.wrapRequest((init) => ScreenAnalyticsInsightsService.ListAgentsUsageForApp(fullRequest, init));
  }

  // Single-page `ListAppsUsageOverTime`. Trend table backend-paginates; app selector aggregates
  // every page via an infinite query. Mirrors `listAgentsUsageForApp`.
  public listAppsUsageOverTime(
    request: Omit<ListAppsUsageOverTimeRequest, 'parent'>
  ): CancellableFetch<ListAppsUsageOverTimeResponse> {
    const fullRequest: ListAppsUsageOverTimeRequest = { ...request, parent: this.buildResourceName() };
    return this.wrapRequest((init) => ScreenAnalyticsInsightsService.ListAppsUsageOverTime(fullRequest, init));
  }
}
