import type React from 'react';
import { memo } from 'react';
import { useQuery } from '@tanstack/react-query';

import type { DateRange, DirectorEnvironment } from '@cresta/director-api';
import type { AttributeStructure } from '@cresta/web-client';
import { AttributeType, Frequency } from '@cresta/web-client';

import { conversationAssistanceUsageStatsQuery } from '@/features/insights/assistance/api/conversationAssistanceUsageStatsQuery';
import type { FilterByAttributesOptions } from '@/hooks/insights';
import { useAuthenticatedUser } from '@/hooks/user';
import type { InsightsAttribute } from '@/types';

import {
  useAgentStats,
  useAssistanceStatsWithSplitAPIs,
  useConversationStats,
  useGetHintStatsByHintType,
  useInsightsRequestParams,
  useKnowledgeAssistStats,
} from '../../../hooks';
import { ATTRIBUTE_STRUCTURE_AGENT_TEAM } from '../../../utils';

import { AssistanceUsedLeaderboard } from './assistance-used-leaderboard';

const INSIGHTS_REQUEST_PARAMS_OPTIONS_FOR_CONVOS_WITH_AA = { agentAssistUsedOnly: true };
const ATTRIBUTE_STRUCTURE_BY_AGENT: AttributeStructure[] = [{ attributeType: AttributeType.ATTRIBUTE_TYPE_AGENT }];
const ATTRIBUTE_STRUCTURE_BY_GROUP: AttributeStructure[] = [{ attributeType: AttributeType.ATTRIBUTE_TYPE_GROUP }];
const GET_HINT_STATS_BY_HINT_TYPE_OPTIONAL_PARAMS = {};

interface AssistanceUsedLeaderboardContainerProps {
  dateDayRange: DateRange;
  filterValues: InsightsAttribute;
  appType: DirectorEnvironment;
  filterOptions?: FilterByAttributesOptions;
}

export const AssistanceUsedLeaderboardContainer = memo(function AssistanceUsedLeaderboardContainer(
  props: AssistanceUsedLeaderboardContainerProps
): React.ReactElement {
  const { dateDayRange, filterValues, appType, filterOptions } = props;
  const { isAgentOnly } = useAuthenticatedUser();

  const insightsRequestParams = useInsightsRequestParams(
    dateDayRange,
    ATTRIBUTE_STRUCTURE_AGENT_TEAM,
    'daily',
    filterValues,
    filterOptions
  );

  const insightsRequestParamsForTeam = useInsightsRequestParams(
    dateDayRange,
    ATTRIBUTE_STRUCTURE_BY_GROUP,
    'daily',
    filterValues,
    filterOptions
  );

  const insightsRequestParamsWithAA = useInsightsRequestParams(
    dateDayRange,
    ATTRIBUTE_STRUCTURE_AGENT_TEAM,
    'daily',
    filterValues,
    { ...INSIGHTS_REQUEST_PARAMS_OPTIONS_FOR_CONVOS_WITH_AA, ...filterOptions, agentAssistUsedOnly: true }
  );

  const insightsRequestParamsWithAAForTeam = useInsightsRequestParams(
    dateDayRange,
    ATTRIBUTE_STRUCTURE_BY_GROUP,
    'daily',
    filterValues,
    { ...INSIGHTS_REQUEST_PARAMS_OPTIONS_FOR_CONVOS_WITH_AA, ...filterOptions, agentAssistUsedOnly: true }
  );

  const { hintStatsByHintType, allHintTypeLoading } = useGetHintStatsByHintType(filterValues, dateDayRange, 'DAILY', {
    ...GET_HINT_STATS_BY_HINT_TYPE_OPTIONAL_PARAMS,
    filterOptions,
  });

  const { hintStatsByHintType: hintStatsByHintTypeForTeam, allHintTypeLoading: allHintTypeLoadingForTeam } =
    useGetHintStatsByHintType(filterValues, dateDayRange, 'DAILY', {
      attributeStructure: ATTRIBUTE_STRUCTURE_BY_GROUP,
      filterOptions,
    });

  const assistanceStats = useAssistanceStatsWithSplitAPIs(insightsRequestParams);

  const assistanceStatsForTeam = useAssistanceStatsWithSplitAPIs(insightsRequestParamsForTeam);

  const knowledgeAssistStats = useKnowledgeAssistStats(insightsRequestParams);
  const knowledgeAssistStatsForTeam = useKnowledgeAssistStats(insightsRequestParamsForTeam);
  const conversationStats = useConversationStats(insightsRequestParams);
  const conversationStatsForTeam = useConversationStats(insightsRequestParamsForTeam);
  const conversationAssistanceUsageStats = useQuery(
    conversationAssistanceUsageStatsQuery({
      ...insightsRequestParams,
      frequency: Frequency.FREQUENCY_UNSPECIFIED,
      groupByAttributeTypes: ATTRIBUTE_STRUCTURE_BY_AGENT,
    })
  );
  const conversationAssistanceUsageStatsForTeam = useQuery(
    conversationAssistanceUsageStatsQuery({
      ...insightsRequestParamsForTeam,
      frequency: Frequency.FREQUENCY_UNSPECIFIED,
    })
  );
  const conversationStatsWithAA = useConversationStats(insightsRequestParamsWithAA);
  const conversationStatsWithAAForTeam = useConversationStats(insightsRequestParamsWithAAForTeam);
  const agentStats = useAgentStats(insightsRequestParams);
  const agentStatsForTeam = useAgentStats(insightsRequestParamsForTeam);

  return (
    <AssistanceUsedLeaderboard
      appType={appType}
      assistanceStats={assistanceStats}
      hintStatsByHintType={hintStatsByHintType}
      allHintTypeLoading={allHintTypeLoading}
      hintStatsByHintTypeForTeam={hintStatsByHintTypeForTeam}
      allHintTypeLoadingForTeam={allHintTypeLoadingForTeam}
      knowledgeAssistStats={knowledgeAssistStats}
      displayAsAgent={isAgentOnly}
      conversationAssistanceUsageStats={
        conversationAssistanceUsageStats.isError ? undefined : conversationAssistanceUsageStats.data
      }
      conversationAssistanceUsageStatsForTeam={
        conversationAssistanceUsageStatsForTeam.isError ? undefined : conversationAssistanceUsageStatsForTeam.data
      }
      conversationStats={conversationStats}
      conversationStatsWithAA={conversationStatsWithAA}
      agentStats={agentStats}
      assistanceStatsForTeam={assistanceStatsForTeam}
      knowledgeAssistStatsForTeam={knowledgeAssistStatsForTeam}
      conversationStatsForTeam={conversationStatsForTeam}
      conversationStatsWithAAForTeam={conversationStatsWithAAForTeam}
      agentStatsForTeam={agentStatsForTeam}
    />
  );
});
