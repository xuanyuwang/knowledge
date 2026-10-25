import type { ConversationAssistanceUsageStatsResponse, InsightsRequestParams } from '@cresta/director-api';
import { CrestaAPI, ErrorHandlerType } from '@cresta/director-api';
import { getI18n } from '@cresta/director-utils';

import type { QueryOptions } from '@/shared/api';
import { cancellableQueryOptions, DirectorQueryKey, directorQueryKey } from '@/shared/api';

export function conversationAssistanceUsageStatsQuery(
  requestParams: InsightsRequestParams
): QueryOptions<ConversationAssistanceUsageStatsResponse> {
  const i18n = getI18n();
  const t = i18n.getFixedT(null, 'director-app-insights', 'assistance.conversation-assistance-usage');

  return cancellableQueryOptions({
    queryKey: directorQueryKey(DirectorQueryKey.USE_RETRIEVE_CONVERSATION_ASSISTANCE_USAGE_STATS, requestParams),
    queryFn: () => CrestaAPI.insights.retrieveConversationAssistanceUsageStats(requestParams),
    meta: {
      errorMessage: t('load-error', 'Failed to load conversation assistance usage.'),
      errorHandlerType: ErrorHandlerType.Toast,
    },
  });
}
