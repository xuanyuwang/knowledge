import type {
  DataTag,
  InfiniteData,
  QueryFunction,
  QueryFunctionContext,
  UnusedSkipTokenInfiniteOptions,
  UnusedSkipTokenOptions,
  UseInfiniteQueryOptions,
  UseQueryOptions,
} from '@tanstack/react-query';
import {
  infiniteQueryOptions as tanstackInfiniteQueryOptions,
  queryOptions as tanstackQueryOptions,
  useInfiniteQuery,
  useQuery,
} from '@tanstack/react-query';

import type { ErrorHandlerType } from '../services';
import type { CancellableFetch } from '../utils';

import { useMemoObject } from './useMemoObject';

export type QueryMeta = {
  errorHandlerType?: ErrorHandlerType;
  errorMessage?: string;
  // If this is set, the error will be ignored if the function returns true
  ignoreError?: (error: unknown) => boolean;
};

export type QueryKey = [DirectorQueryKey, ...unknown[]];

/** Query keys for `transformCancellableQueryProvider` outside director-app. */
export type CancellableQueryKey = readonly [string, ...unknown[]];

type CancellableQueryFn<TQueryFnData, TQueryKey extends CancellableQueryKey, TPageParam = never> = (
  context: QueryFunctionContext<TQueryKey, TPageParam>
) => CancellableFetch<TQueryFnData>;

export type CancellableQueryOptions<
  TQueryFnData,
  TError = Error,
  TData = TQueryFnData | undefined,
  TQueryKey extends CancellableQueryKey = CancellableQueryKey,
> = UnusedSkipTokenOptions<TQueryFnData | undefined, TError, TData, TQueryKey> & {
  queryKey: DataTag<TQueryKey, TQueryFnData | undefined, TError>;
};

type CancellableQueryOptionsInput<TQueryFnData, TError, TData, TQueryKey extends CancellableQueryKey> = Omit<
  UnusedSkipTokenOptions<TQueryFnData | undefined, TError, TData, TQueryKey>,
  'queryFn'
> & {
  queryFn: CancellableQueryFn<TQueryFnData, TQueryKey>;
};

export type CancellableQueryOptionsFactory<TQueryKeyBase extends CancellableQueryKey, TDefaultError = Error> = <
  TQueryFnData = unknown,
  TError = TDefaultError,
  TData = TQueryFnData | undefined,
  TQueryKey extends TQueryKeyBase = TQueryKeyBase,
>(
  options: CancellableQueryOptionsInput<TQueryFnData, TError, TData, TQueryKey>
) => CancellableQueryOptions<TQueryFnData, TError, TData, TQueryKey>;

/**
 * Creates a TanStack query-options wrapper constrained to an application's query-key type.
 * Use `enabled: false` to skip a query; when invoked, `queryFn` must return a `CancellableFetch`.
 */
export function createCancellableQueryOptions<
  TQueryKeyBase extends CancellableQueryKey,
  TDefaultError = Error,
>(): CancellableQueryOptionsFactory<TQueryKeyBase, TDefaultError> {
  return function cancellableQueryOptions<
    TQueryFnData = unknown,
    TSelectData = TQueryFnData | undefined,
    TError = TDefaultError,
    TQueryKey extends TQueryKeyBase = TQueryKeyBase,
  >(
    options: CancellableQueryOptionsInput<TQueryFnData, TError, TSelectData, TQueryKey>
  ): CancellableQueryOptions<TQueryFnData, TError, TSelectData, TQueryKey> {
    const { queryFn, ...optionsWithoutQueryFn } = options;
    return tanstackQueryOptions<TQueryFnData | undefined, TError, TSelectData, TQueryKey>({
      ...optionsWithoutQueryFn,
      queryFn: transformCancellableQueryProvider<TQueryFnData, TQueryKey>(queryFn),
    });
  };
}

export type CancellableInfiniteQueryOptions<
  TQueryFnData,
  TError = Error,
  TData = InfiniteData<TQueryFnData | undefined>,
  TQueryKey extends CancellableQueryKey = CancellableQueryKey,
  TPageParam = unknown,
> = UnusedSkipTokenInfiniteOptions<TQueryFnData | undefined, TError, TData, TQueryKey, TPageParam> & {
  queryKey: DataTag<TQueryKey, InfiniteData<TQueryFnData | undefined>, TError>;
};

type CancellableInfiniteQueryOptionsInput<
  TQueryFnData,
  TError,
  TData,
  TQueryKey extends CancellableQueryKey,
  TPageParam,
> = Omit<UnusedSkipTokenInfiniteOptions<TQueryFnData | undefined, TError, TData, TQueryKey, TPageParam>, 'queryFn'> & {
  queryFn: CancellableQueryFn<TQueryFnData, TQueryKey, TPageParam>;
};

export type CancellableInfiniteQueryOptionsFactory<TQueryKeyBase extends CancellableQueryKey, TDefaultError = Error> = <
  TQueryFnData,
  TError = TDefaultError,
  TData = InfiniteData<TQueryFnData | undefined>,
  TQueryKey extends TQueryKeyBase = TQueryKeyBase,
  TPageParam = unknown,
>(
  options: CancellableInfiniteQueryOptionsInput<TQueryFnData, TError, TData, TQueryKey, TPageParam>
) => CancellableInfiniteQueryOptions<TQueryFnData, TError, TData, TQueryKey, TPageParam>;

/**
 * Creates a TanStack infinite-query-options wrapper constrained to an application's query-key type.
 * Use `enabled: false` to skip a query; when invoked, `queryFn` must return a `CancellableFetch`.
 */
export function createCancellableInfiniteQueryOptions<
  TQueryKeyBase extends CancellableQueryKey,
  TDefaultError = Error,
>(): CancellableInfiniteQueryOptionsFactory<TQueryKeyBase, TDefaultError> {
  return function cancellableInfiniteQueryOptions<
    TQueryFnData,
    TData = InfiniteData<TQueryFnData | undefined>,
    TError = TDefaultError,
    TPageParam = unknown,
    TQueryKey extends TQueryKeyBase = TQueryKeyBase,
  >(
    options: CancellableInfiniteQueryOptionsInput<TQueryFnData, TError, TData, TQueryKey, TPageParam>
  ): CancellableInfiniteQueryOptions<TQueryFnData, TError, TData, TQueryKey, TPageParam> {
    const { queryFn, ...optionsWithoutQueryFn } = options;
    return tanstackInfiniteQueryOptions<TQueryFnData | undefined, TError, TData, TQueryKey, TPageParam>({
      ...optionsWithoutQueryFn,
      queryFn: transformCancellableQueryProvider<TQueryFnData, TQueryKey, TPageParam>(queryFn),
    });
  };
}

/**
 * @deprecated New Director queries should define an app-level query-options factory with
 * `cancellableQueryOptions` and consume it with TanStack `useQuery`.
 */
export function useCancellableQuery<
  TQueryFnData = unknown,
  TError = Error,
  TData = TQueryFnData,
  TQueryKey extends QueryKey = QueryKey,
>(
  queryKey: TQueryKey,
  queryFn: (context: QueryFunctionContext<TQueryKey>) => CancellableFetch<TQueryFnData>,
  options?: Omit<
    UseQueryOptions<TQueryFnData | undefined, TError, TData | undefined, TQueryKey>,
    'queryKey' | 'queryFn'
  > & {
    meta?: QueryMeta;
  }
): ReturnType<typeof useQuery<TQueryFnData | undefined, TError, TData | undefined, TQueryKey>>;
export function useCancellableQuery<
  TQueryFnData = unknown,
  TError = Error,
  TData = TQueryFnData,
  TQueryKey extends QueryKey = QueryKey,
>(
  queryKey: TQueryKey,
  queryFn: (context: QueryFunctionContext<TQueryKey>) => CancellableFetch<TQueryFnData> | undefined,
  options?: Omit<
    UseQueryOptions<TQueryFnData | undefined | null, TError, TData | undefined | null, TQueryKey>,
    'queryKey' | 'queryFn'
  > & {
    meta?: QueryMeta;
  }
): ReturnType<typeof useQuery<TQueryFnData | undefined | null, TError, TData | null | undefined, TQueryKey>>;
export function useCancellableQuery<
  TQueryFnData = unknown,
  TError = Error,
  TData = TQueryFnData,
  TQueryKey extends QueryKey = QueryKey,
>(
  queryKey: TQueryKey,
  queryFn: (context: QueryFunctionContext<TQueryKey>) => CancellableFetch<TQueryFnData> | undefined,
  options?: Omit<
    UseQueryOptions<TQueryFnData | undefined | null, TError, TData | undefined | null, TQueryKey>,
    'queryKey' | 'queryFn'
  > & {
    meta?: QueryMeta;
  }
): ReturnType<typeof useQuery<TQueryFnData | undefined | null, TError, TData | null | undefined, TQueryKey>> {
  const transformedFn = transformCancellableQueryProvider<TQueryFnData, TQueryKey>(queryFn);
  return useMemoObject(
    useQuery<TQueryFnData | null | undefined, TError, TData | undefined | null, TQueryKey>({
      ...options,
      queryKey,
      queryFn: transformedFn,
    })
  );
}

/**
 * @deprecated New Director infinite queries should define an app-level query-options factory with
 * `cancellableInfiniteQueryOptions` and consume it with TanStack `useInfiniteQuery`.
 */
export function useCancellableInfiniteQuery<
  TQueryFnData = unknown,
  TError = Error,
  TData = InfiniteData<TQueryFnData | undefined>,
  TQueryKey extends QueryKey = QueryKey,
  TPageParam = unknown,
>(
  queryKey: TQueryKey,
  queryFn: (context: QueryFunctionContext<TQueryKey, TPageParam>) => CancellableFetch<TQueryFnData>,
  options: Omit<
    UseInfiniteQueryOptions<TQueryFnData | undefined, TError, TData, TQueryKey, TPageParam>,
    'queryKey' | 'queryFn'
  > & {
    meta?: QueryMeta;
  }
): ReturnType<typeof useInfiniteQuery<TQueryFnData | undefined, TError, TData, TQueryKey, TPageParam>>;
export function useCancellableInfiniteQuery<
  TQueryFnData = unknown,
  TError = Error,
  TData = InfiniteData<TQueryFnData | undefined | null>,
  TQueryKey extends QueryKey = QueryKey,
  TPageParam = unknown,
>(
  queryKey: TQueryKey,
  queryFn: (context: QueryFunctionContext<TQueryKey, TPageParam>) => CancellableFetch<TQueryFnData> | undefined,
  options: Omit<
    UseInfiniteQueryOptions<TQueryFnData | undefined | null, TError, TData, TQueryKey, TPageParam>,
    'queryKey' | 'queryFn'
  > & {
    meta?: QueryMeta;
  }
): ReturnType<typeof useInfiniteQuery<TQueryFnData | undefined | null, TError, TData, TQueryKey, TPageParam>>;
export function useCancellableInfiniteQuery<
  TQueryFnData = unknown,
  TError = Error,
  TData = InfiniteData<TQueryFnData | undefined | null>,
  TQueryKey extends QueryKey = QueryKey,
  TPageParam = unknown,
>(
  queryKey: TQueryKey,
  queryFn: (context: QueryFunctionContext<TQueryKey, TPageParam>) => CancellableFetch<TQueryFnData> | undefined,
  options: Omit<
    UseInfiniteQueryOptions<TQueryFnData | undefined | null, TError, TData, TQueryKey, TPageParam>,
    'queryKey' | 'queryFn'
  > & {
    meta?: QueryMeta;
  }
): ReturnType<typeof useInfiniteQuery<TQueryFnData | undefined | null, TError, TData, TQueryKey, TPageParam>> {
  const transformedFn = transformCancellableQueryProvider<TQueryFnData | undefined | null, TQueryKey, TPageParam>(
    queryFn
  );
  const result = useInfiniteQuery<TQueryFnData | undefined | null, TError, TData, TQueryKey, TPageParam>({
    ...options,
    queryKey,
    queryFn: transformedFn,
  });

  return result;
}

export function transformCancellableQueryProvider<
  TQueryFnData,
  TQueryKey extends CancellableQueryKey,
  TPageParam = never,
>(
  provider: (context: QueryFunctionContext<TQueryKey, TPageParam>) => CancellableFetch<TQueryFnData>
): QueryFunction<TQueryFnData | undefined, TQueryKey, TPageParam>;
export function transformCancellableQueryProvider<
  TQueryFnData,
  TQueryKey extends CancellableQueryKey,
  TPageParam = never,
>(
  provider: (context: QueryFunctionContext<TQueryKey, TPageParam>) => CancellableFetch<TQueryFnData> | undefined
): QueryFunction<TQueryFnData | undefined | null, TQueryKey, TPageParam>;
export function transformCancellableQueryProvider<
  TQueryFnData,
  TQueryKey extends CancellableQueryKey,
  TPageParam = never,
>(
  provider: (context: QueryFunctionContext<TQueryKey, TPageParam>) => CancellableFetch<TQueryFnData> | undefined
): QueryFunction<TQueryFnData | undefined | null, TQueryKey, TPageParam> {
  return (context) => {
    const cancellableFetch = provider(context);
    if (!cancellableFetch) {
      return Promise.resolve(null);
    }
    context.signal.addEventListener('abort', () => {
      cancellableFetch.cancel();
    });
    return cancellableFetch.promise;
  };
}

export enum DirectorQueryKey {
  USE_USER = 'useActiveUser',
  USE_ACTIVE_PROMPTS = 'useActivePrompts',
  USE_ALL_ALERTS_CACHED = 'useAllAlertsCached',
  USE_AVAILABLE_VERSIONS = 'useAvailableVersions',
  USE_GET_ANALYTICS_CHART = 'useGetAnalyticsChart',
  USE_LIST_ANALYTICS_CHARTS = 'useListAnalyticsCharts',
  USE_GET_ANALYTICS_DASHBOARD = 'useGetAnalyticsDashboard',
  USE_GET_DASHBOARD_GROUP = 'useGetDashboardGroup',
  USE_LIST_ANALYTICS_DASHBOARDS = 'useListAnalyticsDashboards',
  USE_LIST_DASHBOARD_GROUPS = 'useListDashboardGroups',
  USE_ALL_BEHAVIORS = 'useAllBehaviors',
  USE_ALL_CHECKLISTS_CACHED = 'useAllChecklistsCached',
  USE_ALL_DISCOVERED_BEHAVIOR_MESSAGES = 'useAllDiscoveredBehaviorMessages',
  USE_ALL_EMOTIONS_CACHED = 'useAllEmotionsCached',
  USE_ALL_GUIDED_WORKLFOW_CACHED = 'useAllGuidedWorkflowsCached',
  USE_ALL_MOMENTS = 'useAllMoments',
  USE_LIST_GLOSSARY_ENTRIES = 'useListGlossaryEntries',
  USE_GET_BUSINESS_CONTEXT = 'useGetBusinessContext',
  USE_LIST_MOMENTS = 'useListMoments',
  USE_ALL_OOB_INTENTS = 'useAllOobIntents',
  USE_ALL_POLICIES = 'useAllPolicies',
  USE_ALL_SENTIMENT_CACHED = 'useAllSentimentsCached',
  USE_ALL_GROUPS = 'useAllGroups',
  USE_BATCH_GET_CONVERSATION_MESSAGES = 'useBatchGetConversationMessages',
  USE_BATCH_TRANSLATE_CONVERSATION_MESSAGES = 'useBatchTranslateConversationMessages',
  USE_GET_GROUP = 'useGetGroup',
  USE_CACHED_HINT_ACTIONS = 'useCachedHintActions',
  USE_CACHED_INTEGRATION_CONFIG = 'useCachedIntegrationConfig',
  USE_CACHED_JOB = 'useCachedJob',
  USE_CACHED_JOB_SCHEDULE = 'useCachedJobSchedule',
  USE_CACHED_KB_ARTICLES = 'useCachedKBArticles',
  USE_CALCULATE_MODEL_PREVIEW_SCORE = 'useCalculateModelPreviewScore',
  USE_CONVERSATION = 'useConversation',
  USE_CONVERSATION_BY_PLATFORM_ID = 'useConversationByPlatformId',
  USE_COUNT_CONVERSATIONS = 'useCountConversations',
  USE_LIST_CONVERSATIONS = 'useListConversations',
  USE_LIST_COMMENTS = 'useListComments',
  USE_LIST_FEEDBACK_STATUSES = 'useListFeedbackStatuses',
  USE_ALL_CONVERSATION_ACTION_ANNOTATIONS = 'useAllConversationActionAnnotations',
  USE_CONVERSATION_ACTION_ANNOTATIONS = 'useConversationActionAnnotations',
  USE_CONVERSATION_APP_ACTIVITY = 'useConversationAppActivity',
  USE_CONVERSATION_COACHING_DETAILS = 'useConversationCoachingDetails',
  USE_CONVERSATION_COMMENTS = 'useConversationComments',
  USE_CONVERSATION_ENTITIES = 'useConversationEntities',
  USE_CONVERSATION_EVENTS = 'useConversationEvents',
  USE_CONVERSATION_INBOX_FROM_INSIGHTS = 'retrieveClosedConversations',
  USE_CONVERSATION_INSIGHTS_ANNOTATIONS = 'useConversationInsightsAnnotations',
  USE_CONVERSATION_MESSAGES = 'useConversationMessages',
  USE_DOCUMENTS = 'useDocuments',
  USE_GET_DOCUMENT = 'useGetDocument',
  USE_LIST_DOCUMENT_CHUNKS = 'useListDocumentChunks',
  USE_LIST_CONVERSATION_MESSAGES = 'useListConversationMessages',
  USE_FETCH_ALL_CONVERSATION_MESSAGES = 'useFetchAllConversationMessages',
  USE_LIST_CLIP_MESSAGES = 'useListClipMessages',
  USE_CONVERSATION_MOMENT_ANNOTATIONS = 'useConversationMomentAnnotations',
  USE_CONVERSATION_TAGS = 'useConversationTags',
  USE_COUNT_USERS = 'useCountUsers',
  USE_COUNT_GROUPS = 'useCountGroups',
  USE_CUSTOMER_CONFIG = 'useCustomerConfig',
  USE_EVALUATE_ANALYTICS_CHART = 'useEvaluateAnalyticsChart',
  USE_FETCH_JOB_SCHEDULES = 'useFetchJobSchedules',
  USE_FILTERED_CHANGE_LOG = 'useFilteredChangeLog',
  USE_FILTERED_JOBS = 'useFilteredJobs',
  USE_GET_BEHAVIOR_STATS = 'useGetBehaviorStats',
  USE_GET_CHANGE_LOG_ENTRY = 'useGetChangeLogEntry',
  USE_GET_COMMENT = 'useGetComment',
  USE_GET_EDITOR_DOCUMENT = 'useGetEditorDocument',
  USE_GET_LIBRARY_CLIP = 'useGetLibraryClip',
  USE_GET_LIST_POLICIES = 'useGetListPolicies',
  USE_GET_METADATA_MAPPINGS = 'useGetMetadataMappings',
  USE_GET_NOTIFICATION = 'useGetNotification',
  USE_GET_POLICY = 'useGetPolicy',
  USE_GET_POLICY_HISTORY = 'useGetPolicyHistory',
  USE_GET_POLICY_DRAFT = 'useGetPolicyDraft',
  USE_GET_SESSION_LOG = 'useGetSessionLog',
  USE_GET_SET = 'useGetSet',
  USE_GET_TEST_CASE_SET = 'useGetTestCaseSet',
  USE_GET_USER = 'useGetUser',
  USE_GET_VIRTUAL_AGENT = 'useGetVirtualAgent',
  USE_GET_INQUIRY = 'useGetInquiry',
  USE_HINT_STATS = 'useHintStats',
  USE_INBOX_CONVERSATIONS_API = 'useInboxConversationsAPI',
  USE_INSIGHTS_ADHERENCE_STATS = 'useInsightsAdherenceStats',
  USE_INFRA_METRICS = 'useInfraMetrics',
  USE_LIST_ACTIVE_PERSONA_GENERATION_JOBS = 'useListActivePersonaGenerationJobs',
  USE_LIST_ACTIVE_TEST_GENERATION_JOBS = 'useListActiveTestGenerationJobs',
  USE_LIST_INSIGHTS_TOPICS = 'useListInsightsTopics',
  USE_LIST_DIAGNOSTIC_RECORDS = 'useListDiagnosticRecords',
  USE_LIST_DISCOVERED_BEHAVIOR_MESSAGES = 'useListDiscoveredBehaviorMessages',
  USE_LIST_DOCUMENT_INGESTIONS_RELEASES = 'useListDocumentIngestionsReleases',
  USE_LIST_GROUP_MEMBERSHIPS = 'useListGroupMemberships',
  USE_LIST_INGESTIONS = 'useListIngestions',
  USE_LIST_INTEGRATION_CONFIGS = 'useListIntegrationConfigs',
  USE_LIST_INTEGRATION_CONFIG_VERSIONS = 'useListIntegrationConfigVersions',
  USE_GET_INTEGRATION_CONFIG_VERSION = 'useGetIntegrationConfigVersion',
  USE_DIFF_INTEGRATION_CONFIG_VERSIONS = 'useDiffIntegrationConfigVersions',
  USE_LIST_CONNECTOR_INGESTION_PLATFORM_CONFIGS = 'useListConnectorIngestionPlatformConfigs',
  USE_GET_CONNECTOR_INGESTION_CONNECTOR = 'useGetConnectorIngestionConnector',
  USE_LIST_CREDENTIAL_SOURCES = 'useListCredentialSources',
  USE_GET_CREDENTIAL_SOURCE = 'useGetCredentialSource',
  USE_NOTION_DATABASE = 'useNotionDatabase',
  USE_NOTION_DATABASE_PROPERTIES = 'useNotionDatabaseProperties',
  USE_SEISMIC_CONTENT_PROPERTIES = 'useSeismicContentProperties',
  USE_SEISMIC_CONTENT_PROFILES = 'useSeismicContentProfiles',
  USE_SEISMIC_USER_GROUPS = 'useSeismicUserGroups',
  USE_CONFLUENCE_SPACES = 'useConfluenceSpaces',
  USE_CONFLUENCE_SPACE_ITEMS = 'useConfluenceSpaceItems',
  USE_SHAREPOINT_SITES = 'useSharepointSites',
  USE_SHAREPOINT_DRIVES = 'useSharepointDrives',
  USE_SHAREPOINT_LISTS = 'useSharepointLists',
  USE_SHAREPOINT_LIST_COLUMNS = 'useSharepointListColumns',
  USE_SHAREPOINT_DRIVE_COLUMNS = 'useSharepointDriveColumns',
  USE_SALESFORCE_INSTANCE_DETAILS = 'useSalesforceInstanceDetails',
  USE_SALESFORCE_KNOWLEDGE_OBJECTS = 'useSalesforceKnowledgeObjects',
  USE_SALESFORCE_RESOLVED_KNOWLEDGE_OBJECT = 'useSalesforceResolvedKnowledgeObject',
  USE_SALESFORCE_DATA_CATEGORY_GROUPS = 'useSalesforceDataCategoryGroups',
  USE_SALESFORCE_PERMISSION_SETS = 'useSalesforcePermissionSets',
  USE_SALESFORCE_PERMISSION_SET_GROUPS = 'useSalesforcePermissionSetGroups',
  USE_SALESFORCE_PROFILES = 'useSalesforceProfiles',
  USE_GITBOOK_ORGANIZATIONS = 'useGitbookOrganizations',
  USE_GITBOOK_SITES = 'useGitbookSites',
  USE_CONTENTFUL_SPACES = 'useContentfulSpaces',
  USE_CONTENTFUL_ENVIRONMENTS = 'useContentfulEnvironments',
  USE_CONTENTFUL_CONTENT_TYPES = 'useContentfulContentTypes',
  USE_CONTENTFUL_CONTENT_TYPE = 'useContentfulContentType',
  USE_ZENDESK_ARTICLE_LABELS = 'useZendeskArticleLabels',
  USE_ZENDESK_USER_SEGMENTS = 'useZendeskUserSegments',
  USE_LIST_LIBRARY_CLIPS = 'useListLibraryClips',
  USE_LIST_METADATA_MOMENT = 'useListMetadataMoment',
  USE_LIST_NOTIFICATIONS = 'useListNotifications',
  USE_LIST_GLOBAL_NOTIFICATION_PREFERENCES = 'useListGlobalNotificationPreferences',
  USE_LIST_SYSTEM_MOMENTS = 'useListSystemMoments',
  USE_LIST_POLICY_DRAFTS = 'useListPolicyDrafts',
  USE_FETCH_SCREEN_RECORDINGS = 'useFetchRecordings',
  USE_LIST_SCREEN_RECORDING_CONFIGS = 'useListScreenRecordingConfigs',
  USE_LIST_USERS = 'useListUsers',
  USE_LIST_USERS_LOGIN_STATUS = 'useListUsersLoginStatus',
  USE_EVALUATED_POLICIES = 'useEvaluatedPolicies',
  USE_NOTIFICATION_INTEGRATION = 'useNotificationIntegrations',
  USE_NOTIFICATION_TEMPLATES = 'useNotificationTemplates',
  USE_OAUTH_CLIENTS = 'useOAuthClients',
  USE_PROMPTS_BY_GROUP_ID = 'usePromptsByGroupId',
  USE_RETRIEVE_SCORECARD_TEMPLATE_EVALUATION_PERIODS = 'useRetrieveScorecardTemplateEvaluationPeriods',
  USE_RUN_ORG_MGMT_DIAGNOSTICS = 'useRunOrgManagementDiagnostics',
  USE_TEST_CASES = 'useTestCases',
  USE_VIRTUAL_AGENTS = 'useVirtualAgents',
  USE_VIRTUAL_AGENT_OPTIONS = 'useVirtualAgentOptions',
  USE_ONE_MOMENT = 'useOneMoment',
  USE_LIST_POLICY_REVIEW_OVERVIEW = 'useListPolicyReviewOverview',
  USE_RUN_TEACHER_MODEL_ON_DATASET = 'useRunTeachModelOnDataset',
  USE_VOICEMAIL_CACHED = 'useVoicemailMomentCached',
  USE_FETCH_OPERA_JOB = 'useFetchOperaJob',
  USE_FETCH_PREVIEW = 'useFetchPreviewStatus',
  USE_RETRIEVE_MANUAL_QA_STATS = 'useRetrieveManualQAStats',
  USE_RETRIEVE_DIRECTOR_TASK_STATS = 'useRetrieveDirectorTaskStats',
  USE_ALL_KNOWLEDGE_BASES = 'useAllKnowledgeBases',
  USE_ALL_WEBHOOKS = 'useAllWebhooks',
  USE_ALL_EXTERNAL_API_ENDPOINTS = 'useAllExternalApiEndpoints',
  USE_EXTERNAL_API_ENDPOINT = 'useExternalApiEndpoint',
  USE_LIST_EXTERNAL_API_INVOCATIONS = 'useListExternalApiInvocations',
  USE_EXTERNAL_API_INVOCATION = 'useExternalApiInvocation',
  USE_LIST_MOCK_RULES = 'useListMockRules',
  USE_ALL_MOCK_RULES = 'useAllMockRules',
  USE_MOCK_RULE = 'useMockRule',
  USE_LIST_MOCK_SCENARIOS = 'useListMockScenarios',
  USE_ALL_MOCK_SCENARIOS = 'useAllMockScenarios',
  USE_MOCK_SCENARIO = 'useMockScenario',
  USE_CURRENT_SCORECARD_TEMPLATES = 'useCurrentScorecardTemplates',
  USE_PRISMATIC_TOKEN = 'usePrismaticToken',
  USE_MANUAL_QA_PROGRESS = 'useManualQAProgress',
  USE_CUSTOM_SUMMARIZATION_LIST = 'useCustomSummarizationList',
  USE_CUSTOM_SUMMARIZATION_MODEL = 'useCustomSummarizationModel',
  USE_LIST_PREBUILT_TOPICS = 'useListPrebuiltTopics',
  USE_LIST_VALID_CASE_STATUSES = 'useListValidCaseStatuses',
  USE_CANNED_SUGGESTIONS_LIST = 'useCannedSuggestionsList',
  USE_CANNED_SUGGESTIONS_PAST_UPLOADS = 'useCannedSuggestionsPastUploads',
  USE_CANNED_SUGGESTIONS_AUTO_COUNT = 'useCannedSuggestionsAutoCount',
  USE_KNOWLEDGE_BASE = 'useKnowledgeBase',
  USE_KNOWLEDGE_BASE_ID = 'useKnowledgeBaseId',
  USE_KNOWLEDGE_BASE_LIST = 'useKnowledgeBaseList',
  USE_KNOWLEDGE_BASE_EXTERNAL_SOURCES = 'useKnowledgeBaseExternalSources',
  USE_KNOWLEDGE_BASE_ARTICLE = 'useKnowledgeBaseArticle',
  USE_INBOX_SCORECARD_DETAILS = 'useInboxScorecardDetails',
  USE_CHECK_DATA_AVAILABILITY = 'useCheckDataAvailability',
  USE_RETRIEVE_METADATA = 'useRetrieveMetadata',
  USE_RETRIEVE_LLM_ANNOTATION_VALUES = 'useRetrieveLlmAnnotationValues',
  USE_CONVERSATION_MESSAGE_STATS = 'useConversationMessageStats',
  USE_SCREEN_RECORDING_SESSION = 'useScreenRecordingSession',
  USE_SCREEN_RECORDING_MAPPING = 'useScreenRecordingMapping',
  USE_SCREEN_RECORDING_MAPPINGS = 'useScreenRecordingMappings',
  USE_SUGGEST_MANUAL_QA_CONVERSATIONS = 'useSuggestManualQAConversations',
  USE_QA_TASK_CONFIG_USERS = 'useQATaskConfigUsers',
  USE_LIST_POLICY_REVIEWS = 'useListPolicyReviews',
  USE_LIST_CALIBRATIONS = 'useListCalibrations',
  USE_LIST_REQUIREMENT_CALIBRATIONS = 'useListRequirementCalibrations',
  USE_GET_CALIBRATION = 'useGetCalibration',
  USE_RETRIEVE_TEAM_USECASES = 'useRetrieveTeamUsecases',
  USE_SCORECARD_TEMPLATE_AUTO_SCORE_RESULT = 'useScorecardTemplateAutoScoreResult',
  USE_LIST_OBJECTS_S3 = 'useListObjectsS3',
  USE_RETRIEVE_BEHAVIOR_DISCOVERY = 'useRetrieveBehaviorDiscovery',
  USE_RETRIEVE_CONVERSATION_MESSAGES = 'useRetrieveConversationMessages',
  USE_LIST_TAGS = 'useListTags',
  USE_LIST_FILTER_SETS = 'useListFilterSets',
  USE_GET_OPERA_ANALYZER_INFO = 'useGetOperaAnalyzerInfo',
  USE_LIST_TARGETS = 'useListTargets',
  USE_LIST_WEBHOOK_INVOCATIONS = 'useListWebhookInvocations',
  USE_GET_RECENT_COACHING_ACTIVITIES = 'useGetRecentCoachingActivities',
  USE_RETRIEVE_CONVERSATION_STATS = 'useRetrieveConversationStats',
  USE_RETRIEVE_CONVERSATION_ASSISTANCE_USAGE_STATS = 'useRetrieveConversationAssistanceUsageStats',
  USE_QA_SCORE_STATS = 'useQAScoreStats',
  USE_USER_OUTCOME_STATS = 'useUserOutcomeStats',
  USE_COACHING_COMMENTS = 'useCoachingComments',
  USE_COACHING_OVERVIEW_DATA = 'useCoachingOverviewData',
  USE_COACHING_PLANS = 'useCoachingPlans',
  USE_COACHING_PLAN_DETAILS = 'useCoachingPlanDetails',
  USE_RETRIEVE_AGENT_CRITERIA_CONVERSATIONS = 'useRetrieveAgentCriteriaConversations',
  USE_RETRIEVE_SUGGESTED_COACHING_ACTIVITIES = 'useRetrieveSuggestedCoachingActivities',
  USE_RETRIEVE_COACHING_PROGRESSES = 'useRetrieveCoachingProgresses',
  USE_RETRIEVE_CONVERSATION_OUTCOME_STATS = 'useRetrieveConversationOutcomeStats',
  USE_RETRIEVE_QA_CONVERSATIONS = 'useRetrieveQAConversations',
  USE_ALL_SCORECARDS = 'useAllScorecards',
  USE_SCORECARD = 'useScorecard',
  USE_EVALUATE_SCORECARD_PERMISSIONS = 'useEvaluateScorecardPermissions',
  USE_KNOWLEDGE_ASSIST_STATS = 'useKnowledgeAssistStats',
  USE_CUSTOMER_SNAPSHOT_STATS = 'useCustomerSnapshotStats',
  USE_KNOWLEDGE_ASSIST_GENERATION = 'useKnowledgeAssistGeneration',
  USE_KNOWLEDGE_ASSIST_AUTOCOMPLETE = 'useKnowledgeAssistAutocomplete',
  USE_SCORECARDS = 'useScorecards',
  USE_LIST_SCORECARDS_FOR_QA_CONVO_EXAMPLES = 'useListScorecardsForQAConvoExamples',
  USE_GET_MOMENT_HISTORY = 'useGetMomentHistory',
  USE_LIST_MOMENT_HISTORIES = 'useListMomentHistories',
  USE_RETRIEVE_COACHING_EFFICIENCY_STATS = 'useRetrieveCoachingEfficiencyStats',
  USE_SUGGESTION_STATS = 'useSuggestionStats',
  USE_SMART_COMPOSE_STATS = 'useSmartComposeStats',
  USE_KNOWLEDGE_BASE_STATS = 'useKnowledgeBaseStats',
  USE_NOTE_TAKING_STATS = 'useNoteTakingStats',
  USE_GUIDED_WORKFLOW_STATS = 'useGuidedWorkflowStats',
  USE_SUMMARIZATION_STATS = 'useSummarizationStats',
  USE_AGENT_STATS = 'useAgentStats',
  USE_SCORECARD_STATS = 'useScorecardStats',
  USE_COACHING_SESSION_STATS = 'useCoachingSessionStats',
  USE_MANAGER_STATS = 'useManagerStats',
  USE_COMMENT_STATS = 'useCommentStats',
  USE_LIVE_ASSIST_STATS = 'useLiveAssistStats',
  USE_LLM_ANALYST = 'useLLMAnalyst',
  USE_EXTEND_LLM_ANALYST = 'useExtendLLMAnalyst',
  USE_LIST_LLM_MODELS = 'useListLlmModels',
  USE_LIST_LLM_PROVIDERS = 'useListLlmProviders',
  USE_LIST_LLM_SECRETS = 'useListLlmSecrets',
  USE_EXTEND_ANALYST_V2 = 'useExtendAnalystV2',
  USE_DISCOVER_FLOW = 'useDiscoverFlow',
  USE_MAP_TO_FLOW = 'useMapToFlow',
  USE_RETRIEVE_FLOWS = 'useRetrieveFlows',
  USE_EXPORT_FORMATTED_FLOW = 'useExportFormattedFlow',
  USE_EXPORT_CONVERSATION_TOOL_MESSAGES = 'useExportConversationToolMessages',
  USE_USER_ONLINE_STATUS_REPORTER = 'useUserOnlineStatusReporter',
  USE_RETRIEVE_QA_TASK_USER_QUOTA = 'useRetrieveQATaskUserQuota',
  USE_RETRIEVE_ANALYTICS_CHART_EXAMPLES = 'useRetrieveAnalyticsChartExamples',
  USE_LIST_TEST_CASES = 'useListTestCases',
  USE_LIST_TEST_RESULTS = 'useListTestResults',
  USE_LIST_TEST_RUNS = 'useListTestRuns',
  USE_GET_TEST_RUN = 'useGetTestRun',
  USE_RETRIEVE_APPEAL_STATS = 'useRetrieveAppealStats',
  USE_GET_TEST_CASE = 'useGetTestCase',
  USE_GET_TEST_RESULT = 'useGetTestResult',
  USE_GET_REQUIREMENT_GROUP = 'useGetRequirementGroup',
  USE_LIST_REQUIREMENT_GROUPS = 'useListRequirementGroups',
  USE_LIST_REQUIREMENTS = 'useListRequirements',
  USE_GET_CONVERSATION_HANDOFF = 'useGetConversationHandoff',
  USE_FILTERED_ROLES = 'useFilteredRoles',
  USE_PERMISSION_FEATURE_DEFINITION_MAP = 'usePermissionFeatureDefinitionMap',
  USE_CACHED_ROLE = 'useCachedRole',
  USE_LIST_TEST_SETS = 'useListTestSets',
  USE_GET_TEST_SET = 'useGetTestSet',
  USE_LIST_GROUPS = 'useListGroups',
  USE_CONVERSATION_SUMMARY = 'useConversationSummary',
  USE_LIST_POLICY_HISTORIES = 'useListPolicyHistories',
  USE_LIST_POLICY_GROUPINGS = 'useListPolicyGroupings',
  USE_APPLY_POLICY_GROUPINGS = 'useApplyPolicyGroupings',
  USE_LIST_DIRECTOR_TASKS = 'useListDirectorTasks',
  USE_LIST_AI_ANALYST_CHATS = 'useListAiAnalystChats',
  USE_LIST_AI_ANALYST_TURNS = 'useListAiAnalystTurns',
  USE_GET_AI_ANALYST_CHAT = 'useGetAiAnalysisChat',
  USE_LIST_EXPERIMENTS = 'useListExperiments',
  USE_RETRIEVE_EXPERIMENT_TRAFFIC_GROUP_ANALYSIS = 'useRetrieveExperimentTrafficGroupAnalysis',
  USE_LIST_VIRTUAL_AGENTS = 'useListVirtualAgents',
  USE_LIST_ALL_VIRTUAL_AGENTS = 'useListAllVirtualAgents',
  USE_LIST_VIRTUAL_AGENTS_INFINITE = 'useListVirtualAgentsInfinite',
  USE_LIST_VIRTUAL_AGENT_REVISIONS = 'useListVirtualAgentRevisions',
  USE_GET_VIRTUAL_AGENT_REVISION = 'useGetVirtualAgentRevision',
  USE_LIST_VIRTUAL_AGENT_ALIASES = 'useListVirtualAgentAliases',
  USE_GET_QUERY_BUILTIN_FUNCTIONS = 'useGetQueryBuiltinFunctions',
  USE_GET_QUERY_BUILTIN_SCHEMAS = 'useGetQueryBuiltinSchemas',
  USE_LIST_REMOTE_FUNCTION_BUNDLES = 'useListRemoteFunctionBundles',
  USE_LIST_REMOTE_FUNCTION_BUNDLES_INFINITE = 'useListRemoteFunctionBundlesInfinite',
  USE_GET_REMOTE_FUNCTION_BUNDLE = 'useGetRemoteFunctionBundle',
  USE_LIST_REMOTE_FUNCTION_SECRETS = 'useListRemoteFunctionSecrets',
  USE_RETRIEVE_ASSISTANCE_STATS = 'useRetrieveAssistanceStats',
  USE_BATCH_SET_FEATURE_FLAGS = 'useBatchSetFeatureFlags',
  GET_ROLLOUT_FLAG = 'getRolloutFlag',
  USE_LIST_API_CHANGE_LOG = 'useListApiChangeLog',
  USE_GET_API_CHANGE_LOG_ENTRY = 'useGetApiChangeLogEntry',
  USE_RETRIEVE_POLICY_INSIGHT = 'useRetrievePolicyInsight',
  USE_WEBHOOK = 'useWebhook',
  USE_CLIENT_CERT_OPTIONS = 'useClientCertOptions',
  USE_LIST_COMMON_PHRASES = 'useListCommonPhrases',
  USE_LIST_FLOWS = 'useListFlows',
  USE_LIST_TAXONOMIES = 'useListTaxonomies',
  USE_SAVE_FLOW = 'useSaveFlow',
  USE_DELETE_FLOW = 'useDeleteFlow',
  USE_CREATE_FLOW = 'useCreateFlow',
  USE_LIST_AOC_AUDIENCE = 'useListAOCAudience',
  USE_LIST_AOC_SETTINGS = 'useListAOCSettings',
  USE_LIST_RESOURCE_TYPES = 'useListResourceTypes',
  USE_GET_RESOURCE_TYPE = 'useGetResourceType',
  USE_LIST_FIELD_DEFINITIONS = 'useListFieldDefinitions',
  USE_GET_FIELD_DEFINITION = 'useGetFieldDefinition',
  USE_LIST_RESOURCE_RECORDS = 'useListResourceRecords',
  USE_USER_METADATA = 'useUserMetadata',
  USE_AI_AGENT_TRACE = 'useAiAgentTrace',
  USE_GET_REAL_TIME_TREND = 'useGetRealTimeTrend',
  USE_GET_REAL_TIME_TREND_DEBUG = 'useGetRealTimeTrendDebug',
  USE_LIST_REAL_TIME_TRENDS = 'useListRealTimeTrends',
  USE_LIST_REAL_TIME_TRENDS_DEBUG = 'useListRealTimeTrendsDebug',
  USE_CHECK_AI_ANALYSIS_QUOTA = 'useCheckAiAnalysisQuota',
  USE_GET_ANONYMOUS_AUTH_CONFIG = 'useGetAnonymousAuthConfig',
  USE_LIST_SYNTHETIC_CUSTOMER_SETS = 'useListSyntheticCustomerSets',
  USE_GET_SYNTHETIC_CUSTOMER_SET = 'useGetSyntheticCustomerSet',
  USE_LIST_SYNTHETIC_CUSTOMERS = 'useListSyntheticCustomers',
  USE_LIST_END_CUSTOMERS = 'useListEndCustomers',
  USE_LIST_TRAINING_MODULES = 'useListTrainingModules',
  USE_LIST_TRAINING_LESSONS = 'useListTrainingLessons',
  USE_LIST_TRAINING_SCENARIOS = 'useListTrainingScenarios',
  USE_LIST_TRAINING_SIMULATOR_TASK_RUNS = 'useListTrainingSimulatorTaskRuns',
  USE_EVALUATE_TRAINING_CONVERSATION = 'useEvaluateTrainingConversation',
  USE_TRAINING_SIMULATOR_OVERRIDE_PREVIEW = 'useTrainingSimulatorOverridePreview',
  USE_TRAINING_SIMULATOR_LESSON_STATS = 'useTrainingSimulatorLessonStats',
  USE_TRAINING_SIMULATOR_MODULE_STATS = 'useTrainingSimulatorModuleStats',
  USE_TRAINING_SIMULATOR_TASK_STATS = 'useTrainingSimulatorTaskStats',
  USE_LIST_QUIZ_TEMPLATES = 'useListQuizTemplates',
  USE_GET_QUIZ_TEMPLATE = 'useGetQuizTemplate',
  USE_CRESTA_STATUS_SUMMARY = 'useCrestaStatusSummary',
  USE_GET_END_CUSTOMER_JOURNEY = 'useGetEndCustomerJourney',
  USE_GET_JOURNEY_SUMMARY = 'useGetJourneySummary',
  USE_GET_JOURNEY_TOPIC_ANALYSIS = 'useGetJourneyTopicAnalysis',
  USE_GET_JOURNEY_TOPIC_TRANSITIONS = 'useGetJourneyTopicTransitions',
  USE_LIST_ENVIRONMENTS = 'useListEnvironments',
  USE_GET_ENVIRONMENT = 'useGetEnvironment',
  USE_LIST_ENVIRONMENT_VARIABLES = 'useListEnvironmentVariables',
  USE_GET_ENVIRONMENT_VARIABLE = 'useGetEnvironmentVariable',
  USE_LIST_ENVIRONMENT_SECRETS = 'useListEnvironmentSecrets',
  USE_GET_ENVIRONMENT_SECRET = 'useGetEnvironmentSecret',
  USE_LIST_SANDBOX_WORKSPACES = 'useListSandboxWorkspaces',
  USE_LIST_AI_AGENT_DEPLOYMENTS = 'useListAIAgentDeployments',
  USE_LIST_AI_AGENT_DEPLOYMENT_HISTORY = 'useListAIAgentDeploymentHistory',
  USE_GET_AI_AGENT_DEPLOYMENT = 'useGetAIAgentDeployment',
  USE_RESOLVE_AI_AGENT_DEPLOYMENT = 'useResolveAIAgentDeployment',
  USE_LIST_AI_AGENTS = 'useListAIAgents',
  USE_GET_AI_AGENT = 'useGetAIAgent',
  USE_LIST_AI_AGENT_REVISIONS = 'useListAIAgentRevisions',
  USE_FETCH_SUMMARIZATION_TEMPLATE = 'useFetchSummarizationTemplate',
  USE_LIST_SUMMARIZATION_TEMPLATES = 'useListSummarizationTemplates',
  USE_LIST_PREBUILT_TOPICS_OPERA = 'useListPrebuiltTopicsOpera',
  USE_CHECK_SUPPORTED_TRIGGER_TIMINGS = 'useCheckSupportedTriggerTimings',
  USE_SCREEN_ANALYTICS_APP_USAGE_SUMMARY = 'useScreenAnalyticsAppUsageSummary',
  USE_SCREEN_ANALYTICS_AGENTS_APP_USAGE = 'useScreenAnalyticsAgentsAppUsage',
  USE_SCREEN_ANALYTICS_AGENTS_USAGE_FOR_APP = 'useScreenAnalyticsAgentsUsageForApp',
  USE_SCREEN_ANALYTICS_AGENT_CONVERSATIONS = 'useScreenAnalyticsAgentConversations',
  USE_SCREEN_ANALYTICS_APPS_USAGE_OVER_TIME = 'useScreenAnalyticsAppsUsageOverTime',
  USE_SCREEN_ANALYTICS_APPS_USAGE_OPTIONS = 'useScreenAnalyticsAppsUsageOptions',
  USE_SCREEN_ANALYTICS_BACKFILL_PREVIEW = 'useScreenAnalyticsBackfillPreview',
  USE_SCREEN_ANALYTICS_ACTIVE_BACKFILL_JOB = 'useScreenAnalyticsActiveBackfillJob',
  USE_SCREEN_ANALYTICS_BACKFILL_JOB = 'useScreenAnalyticsBackfillJob',
  USE_SCREEN_ANALYTICS_RULES = 'useScreenAnalyticsRules',
  USE_SCREEN_ANALYTICS_CONFIGS = 'useScreenAnalyticsConfigs',
  USE_LIST_WORKFLOW_AUTOMATION_DEFINITIONS = 'useListWorkflowAutomationDefinitions',
  USE_LIST_SUMMARIZATION_CALIBRATIONS = 'useListSummarizationCalibrations',
  USE_LIST_SUMMARIZATION_CALIBRATION_ROWS = 'useListSummarizationCalibrationRows',
  USE_GET_SUMMARIZATION_CALIBRATION = 'useGetSummarizationCalibration',
  USE_GENERATE_CONDUCTOR_DOWNLOAD_URL = 'useGenerateConductorDownloadUrl',
}

// The lists below will be used to invalidate queries
export const MOMENT_QUERY_KEYS = [
  DirectorQueryKey.USE_ALL_BEHAVIORS,
  DirectorQueryKey.USE_LIST_INSIGHTS_TOPICS,
  DirectorQueryKey.USE_ALL_MOMENTS,
  DirectorQueryKey.USE_ALL_OOB_INTENTS,
  DirectorQueryKey.USE_LIST_METADATA_MOMENT,
  DirectorQueryKey.USE_LIST_MOMENT_HISTORIES, // Once a moment is created/updated, it should reflect in the moment history table.
  DirectorQueryKey.USE_LIST_SYSTEM_MOMENTS, // Once a moment is created/updated, it should be able to be used in a new policy (the moment chooser can see the new moment in the list).
  DirectorQueryKey.USE_LIST_MOMENTS,
];

// Cached chart-evaluation responses become stale when the category tree changes
// (adding/editing/moving a subcategory, moving a folder), so these query keys are
// invalidated by those operations to force a re-evaluation.
export const CHART_EVALUATION_QUERY_KEYS = [DirectorQueryKey.USE_EVALUATE_ANALYTICS_CHART];

export const SCORECARD_SUBMIT_QUERY_KEYS = [
  DirectorQueryKey.USE_MANUAL_QA_PROGRESS,
  DirectorQueryKey.USE_RETRIEVE_MANUAL_QA_STATS,
  DirectorQueryKey.USE_RETRIEVE_DIRECTOR_TASK_STATS,
  DirectorQueryKey.USE_SCORECARDS,
];

export const SCORECARD_QUERY_KEYS = [
  DirectorQueryKey.USE_MANUAL_QA_PROGRESS,
  DirectorQueryKey.USE_RETRIEVE_MANUAL_QA_STATS,
  DirectorQueryKey.USE_RETRIEVE_DIRECTOR_TASK_STATS,
  DirectorQueryKey.USE_ALL_SCORECARDS,
  DirectorQueryKey.USE_SCORECARDS,
  DirectorQueryKey.USE_RETRIEVE_APPEAL_STATS,
  // If you decide to add DirectorQueryKey.USE_SCORECARD here, make sure that updating an existing process scorecard's
  // general notes section works properly, doesn't jump around.
];

export const SCORECARD_TEMPLATE_QUERY_KEYS = [
  DirectorQueryKey.USE_MANUAL_QA_PROGRESS,
  DirectorQueryKey.USE_RETRIEVE_MANUAL_QA_STATS,
  DirectorQueryKey.USE_RETRIEVE_DIRECTOR_TASK_STATS,
  DirectorQueryKey.USE_CURRENT_SCORECARD_TEMPLATES,
  DirectorQueryKey.USE_RETRIEVE_SCORECARD_TEMPLATE_EVALUATION_PERIODS,
  DirectorQueryKey.USE_SCORECARDS,
  // Do not add DirectorQueryKey.USE_INBOX_SCORECARD_DETAILS, since it is updated in place
];

export const COACHING_PLAN_QUERY_KEYS = [
  DirectorQueryKey.USE_GET_RECENT_COACHING_ACTIVITIES,
  DirectorQueryKey.USE_QA_SCORE_STATS,
  DirectorQueryKey.USE_RETRIEVE_SUGGESTED_COACHING_ACTIVITIES,
  DirectorQueryKey.USE_LIST_TARGETS,
  DirectorQueryKey.USE_COACHING_PLANS,
  DirectorQueryKey.USE_COACHING_PLAN_DETAILS,
];

export const ANALYTICS_DASHBOARD_QUERY_KEYS = [
  DirectorQueryKey.USE_LIST_ANALYTICS_CHARTS,
  DirectorQueryKey.USE_GET_ANALYTICS_CHART,
  DirectorQueryKey.USE_LIST_ANALYTICS_DASHBOARDS,
  DirectorQueryKey.USE_GET_ANALYTICS_DASHBOARD,
];

export const DASHBOARD_GROUP_QUERY_KEYS = [
  DirectorQueryKey.USE_LIST_DASHBOARD_GROUPS,
  DirectorQueryKey.USE_GET_DASHBOARD_GROUP,
];

export const DASHBOARD_GROUP_AND_DASHBOARD_QUERY_KEYS = [
  ...ANALYTICS_DASHBOARD_QUERY_KEYS,
  ...DASHBOARD_GROUP_QUERY_KEYS,
];

export const COMMENT_QUERY_KEYS = [
  DirectorQueryKey.USE_GET_COMMENT,
  DirectorQueryKey.USE_LIST_COMMENTS,
  DirectorQueryKey.USE_CONVERSATION_COMMENTS,
];

export const USER_QUERY_KEYS = [DirectorQueryKey.USE_GET_USER, DirectorQueryKey.USE_LIST_USERS];

export const SCREEN_RECORDING_QUERY_KEYS = [
  DirectorQueryKey.USE_FETCH_SCREEN_RECORDINGS,
  DirectorQueryKey.USE_SCREEN_RECORDING_SESSION,
];

export const LIBRARY_CLIP_QUERY_KEYS = [
  DirectorQueryKey.USE_LIST_LIBRARY_CLIPS,
  DirectorQueryKey.USE_GET_LIBRARY_CLIP,
  DirectorQueryKey.USE_LIST_CLIP_MESSAGES,
];

export interface MutateHookOptions<TData, TVariables> {
  onSuccess?: (data: TData, variables: TVariables) => void;
  onError?: (err: unknown) => void;
  errorMessage?: string;
}
