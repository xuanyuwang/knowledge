import { useMemo } from 'react';

import { DirectorEnvironment } from '@cresta/director-api';
import { AttributeHintType } from '@cresta/web-client';

import { useFeatureFlag } from '@/hooks/useFeatureFlag';

import type { AssistanceStatsBaseResult, ManagedKnowledgeAssistStats } from '../../../../../hooks';
import type { ManagedHintStats } from '../../../../../types';
import type { AssistanceUsedLeaderboardRow } from '../types';

const alwaysVisible: (keyof AssistanceUsedLeaderboardRow)[] = [
  'team',
  'assistanceUsed',
  'overallEngagement',
  'avgConvosPerDay',
  'conversationsWithAssistanceUsedPercent',
  'averageHandleTime',
];

export function useVisibleColumnsForAssistanceUsedLeaderboards(
  appType: DirectorEnvironment,
  assistanceStats: AssistanceStatsBaseResult,
  knowledgeAssistStats: ManagedKnowledgeAssistStats,
  hintStatsByHintType: Record<AttributeHintType, ManagedHintStats>,
  displayAsAgent: boolean
): Set<keyof AssistanceUsedLeaderboardRow> {
  const enableConversationWithAgentAssistStats = useFeatureFlag('enableConversationWithAgentAssistStats');

  const totalHintSentCount = useMemo(() => {
    let count = 0;
    const hintTypes = [
      AttributeHintType.BEHAVIOR_HINT,
      AttributeHintType.KB_HINT,
      AttributeHintType.GW_HINT,
      AttributeHintType.CHECKLIST_HINT,
    ];

    for (const hintType of hintTypes) {
      const hintStats = hintStatsByHintType[hintType];

      if (hintStats.isLoading || !hintStats.data) {
        continue;
      }
      count += hintStats.data.totalHintSentCount;
    }

    return count;
  }, [hintStatsByHintType]);

  return useMemo((): Set<keyof AssistanceUsedLeaderboardRow> => {
    const visibleColumns: Set<keyof AssistanceUsedLeaderboardRow> = new Set([...alwaysVisible]);
    !displayAsAgent ? visibleColumns.add('activeDays') : undefined;
    enableConversationWithAgentAssistStats && !displayAsAgent ? visibleColumns.add('convosPoweredByAARate') : undefined;
    assistanceStats.data?.averageConversationWithAutoSummarizationCountPerFrequencyPerUser
      ? visibleColumns.add('averageConvoWithSummary')
      : undefined;
    assistanceStats.data?.totalAutoSummarizationUsedCount ? visibleColumns.add('summaryUsageRate') : undefined;
    assistanceStats.data?.totalAutoSummarizationUsedCount ? visibleColumns.add('numOfSummaryUsed') : undefined;
    assistanceStats.data?.averageKbSearchCountPerFrequencyPerUser
      ? visibleColumns.add('searchesPerFrequency')
      : undefined;
    assistanceStats.data?.averageGwCountPerFrequencyPerUser ? visibleColumns.add('gwPerFrequency') : undefined;
    assistanceStats.data?.averageSuggestionUsedCountPerFrequencyPerUser
      ? visibleColumns.add('suggestionsPerFrequency')
      : undefined;
    assistanceStats.data?.averageSmartComposeUsedCountPerFrequencyPerUser
      ? visibleColumns.add('smartComposePerFrequency')
      : undefined;
    appType === DirectorEnvironment.VOICE && assistanceStats.data?.averageNoteCountPerFrequencyPerUser
      ? visibleColumns.add('notesPerFrequency')
      : undefined;
    assistanceStats.data?.averageAutoSummarizationUsedCountPerFrequencyPerUser
      ? visibleColumns.add('summaryPerFrequency')
      : undefined;
    totalHintSentCount ? visibleColumns.add('hintsFollowed') : undefined;
    totalHintSentCount ? visibleColumns.add('totalHintsFollowedRate') : undefined;

    knowledgeAssistStats.data?.totalKnowledgeMomentDetectedCount
      ? visibleColumns.add('genAiAnswersUsageRate')
      : undefined;
    knowledgeAssistStats.data?.averageGenAIAnswersUsedCountPerUser
      ? visibleColumns.add('genAiAnswersUsedPerFrequency')
      : undefined;

    hintStatsByHintType[AttributeHintType.BEHAVIOR_HINT].data?.resultGroups.length
      ? visibleColumns.add('behavioralHintFollowRate')
      : undefined;
    hintStatsByHintType[AttributeHintType.KB_HINT].data?.resultGroups.length
      ? visibleColumns.add('kbHintFollowRate')
      : undefined;
    hintStatsByHintType[AttributeHintType.GW_HINT].data?.resultGroups.length
      ? visibleColumns.add('gwfHintFollowRate')
      : undefined;
    appType === DirectorEnvironment.VOICE &&
    hintStatsByHintType[AttributeHintType.CHECKLIST_HINT].data?.resultGroups.length
      ? visibleColumns.add('checklistHintFollowRate')
      : undefined;

    return visibleColumns;
  }, [
    appType,
    assistanceStats.data,
    knowledgeAssistStats.data,
    hintStatsByHintType,
    enableConversationWithAgentAssistStats,
    displayAsAgent,
    totalHintSentCount,
  ]);
}
