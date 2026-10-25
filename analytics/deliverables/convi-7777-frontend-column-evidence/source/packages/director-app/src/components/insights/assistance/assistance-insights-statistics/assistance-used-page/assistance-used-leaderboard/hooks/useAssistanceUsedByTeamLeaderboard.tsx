import { useCallback, useMemo } from 'react';
import { useComputedColorScheme } from '@mantine/core';
import type { CellContext, ColumnDef } from '@tanstack/react-table';
import { createColumnHelper } from '@tanstack/react-table';
import { saveAs } from 'file-saver';
import { sum } from 'lodash-es';
import { Link } from 'react-router';

import type {
  AgentStatsGroupResult,
  AssistanceStatsGroupResult,
  ConversationAssistanceUsageStatsGroupResult,
  ConversationAssistanceUsageStatsResponse,
  ConversationStatsGroupResult,
  DirectorEnvironment,
  HintStatsGroupResult,
  KnowledgeAssistStatsGroupResult,
} from '@cresta/director-api';
import {
  createBackgroundColorFn,
  defaultNumberColumnDef,
  SharedTooltip,
  withDefaultValues,
} from '@cresta/director-components';
import { formatNumberToFixed } from '@cresta/director-utils';
import { AttributeHintType } from '@cresta/web-client';

import { getHeatmapColors } from '@/components/gridtable';
import { usersTeamsGroupsFiltersQueryRecord } from '@/hooks/filters';
import type { DirectorTableSortingStateProps } from '@/hooks/useCustomSorting';
import { useCustomSorting } from '@/hooks/useCustomSorting';
import { useFeatureFlag } from '@/hooks/useFeatureFlag';
import { useTranslation } from '@/hooks/useTranslation';
import { ABS_ROUTES } from '@/routes';
import { generatePathWithParams, getCSV } from '@/utils';

import type {
  AssistanceStatsBaseResult,
  ConversationStatsQueryResult,
  ManagedAgentStats,
  ManagedKnowledgeAssistStats,
} from '../../../../../hooks';
import type { ManagedHintStats } from '../../../../../types';
import type { TeamAssistanceUsedLeaderboardRow } from '../types';
import {
  getAHTColumns,
  getAssistanceColumns,
  getAssistanceUsedLeaderboardRowCSVHeader,
  getAutoSummarizationColumns,
  getGenAIColumns,
  getHintColumns,
  getOverallEngagementColumns,
} from '../utils';

import { useVisibleColumnsForAssistanceUsedLeaderboards } from './useVisibleColumnsForAssistanceUsedLeaderboards';

// define fixed size for sticky column
const TEAM_COL_WIDTH = 150;
const STICKY_STATS_COL_WIDTH = 80;

function createTeamAssistanceUsedLeaderboardRow(
  resourceName: string,
  teamResourceName: string,
  displayName: string
): TeamAssistanceUsedLeaderboardRow {
  return {
    resourceName: resourceName,
    teamResourceName: teamResourceName,
    team: displayName,
  };
}

function getCorrectRowFromLeaderboard(
  groupResult:
    | AssistanceStatsGroupResult
    | HintStatsGroupResult
    | KnowledgeAssistStatsGroupResult
    | ConversationAssistanceUsageStatsGroupResult
    | ConversationStatsGroupResult
    | AgentStatsGroupResult,
  teamIDtoData: Map<string, TeamAssistanceUsedLeaderboardRow>,
  isManagerStats?: boolean
): TeamAssistanceUsedLeaderboardRow | undefined {
  const groupedByAttributes = groupResult.groupedByAttributes;
  const displayName = isManagerStats ? groupedByAttributes.user?.fullName : groupedByAttributes.group?.displayName;
  const managerResourceName = isManagerStats ? groupedByAttributes.user?.name : groupedByAttributes.group?.name;
  if (!displayName || !managerResourceName) {
    return;
  }

  const teamResourceName = groupedByAttributes.team?.name || groupedByAttributes.group?.name || '';
  return teamIDtoData.has(managerResourceName)
    ? teamIDtoData.get(managerResourceName)
    : createTeamAssistanceUsedLeaderboardRow(managerResourceName, teamResourceName, displayName);
}

export function useAssistanceUsedByTeamLeaderboard(
  appType: DirectorEnvironment,
  assistanceStats: AssistanceStatsBaseResult,
  hintStatsByHintType: Record<AttributeHintType, ManagedHintStats>,
  allHintTypeLoading: boolean,
  knowledgeAssistStats: ManagedKnowledgeAssistStats,
  conversationStats: ConversationStatsQueryResult,
  conversationStatsWithAA: ConversationStatsQueryResult,
  agentStats: ManagedAgentStats,
  displayAsAgent: boolean,
  conversationAssistanceUsageStats: ConversationAssistanceUsageStatsResponse | undefined
): {
  directorTableProps: DirectorTableSortingStateProps;
  rows: TeamAssistanceUsedLeaderboardRow[] | undefined;
  columns: ColumnDef<TeamAssistanceUsedLeaderboardRow>[];
  exportCSV: (hiddenColumnIds: string[]) => void;
} {
  const { t } = useTranslation('director-app-insights', {
    keyPrefix: 'assistance.assistance-used-by-team-leaderboard',
  });
  const enableATT = useFeatureFlag('enableATT');
  const isDark = useComputedColorScheme() === 'dark';
  const { directorTableProps, columnHelperAddition } = useCustomSorting([{ desc: true, id: 'overallEngagement' }]);
  const columnHelper = useMemo(() => {
    return withDefaultValues<TeamAssistanceUsedLeaderboardRow>(createColumnHelper<TeamAssistanceUsedLeaderboardRow>(), {
      ...defaultNumberColumnDef,
      ...columnHelperAddition,
    });
  }, [columnHelperAddition]);

  const visibleColumns = useVisibleColumnsForAssistanceUsedLeaderboards(
    appType,
    assistanceStats,
    knowledgeAssistStats,
    hintStatsByHintType,
    displayAsAgent
  );

  const rows: TeamAssistanceUsedLeaderboardRow[] | undefined = useMemo(():
    | TeamAssistanceUsedLeaderboardRow[]
    | undefined => {
    if (
      assistanceStats.isPending ||
      allHintTypeLoading ||
      knowledgeAssistStats.isLoading ||
      agentStats.isLoading ||
      conversationStats.isPending ||
      conversationStatsWithAA.isPending
    ) {
      return;
    }

    if (
      assistanceStats.error ||
      knowledgeAssistStats.error ||
      hintStatsByHintType[AttributeHintType.BEHAVIOR_HINT].error ||
      hintStatsByHintType[AttributeHintType.CHECKLIST_HINT].error ||
      hintStatsByHintType[AttributeHintType.GW_HINT].error ||
      hintStatsByHintType[AttributeHintType.KB_HINT].error ||
      agentStats.error ||
      conversationStats.error ||
      conversationStatsWithAA.error
    ) {
      // pass
    }

    const teamIdToRow = new Map<string, TeamAssistanceUsedLeaderboardRow>();

    for (const groupResult of assistanceStats.data?.resultsGroupedByAttributeTypes || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, teamIdToRow);
      if (!row) {
        continue;
      }
      row.team = groupResult.groupedByAttributes.group?.displayName || '';

      row.assistanceUsed = groupResult.averageAssistanceCountPerFrequencyPerUser;

      row.numOfSummarySent = groupResult.totalConversationWithAutoSummarizationCount;
      row.averageConvoWithSummary = groupResult.averageConversationWithAutoSummarizationCountPerFrequencyPerUser;
      row.numOfSummaryUsed = groupResult.totalAutoSummarizationUsedCount;
      row.summaryUsageRate = groupResult.totalConversationWithAutoSummarizationCount
        ? Math.min(
            // TODO (kurt.choi) temp fix for auto summ metric bug - revisit when BE issue is resolved
            groupResult.totalConversationWithAutoSummarizationCount,
            groupResult.totalAutoSummarizationUsedCount
          ) / groupResult.totalConversationWithAutoSummarizationCount
        : undefined;
      row.suggestionsPerFrequency = groupResult.averageSuggestionCountPerFrequencyPerUser;
      row.smartComposePerFrequency = groupResult.averageSmartComposeCountPerFrequencyPerUser;
      row.searchesPerFrequency = groupResult.averageKbSearchCountPerFrequencyPerUser;
      row.gwPerFrequency = groupResult.averageGwCountPerFrequencyPerUser;
      row.notesPerFrequency = groupResult.averageNoteCountPerFrequencyPerUser;
      row.hintsFollowed = groupResult.averageHintsFollowedPerFrequencyPerUser;
      row.summaryPerFrequency = groupResult.averageAutoSummarizationUsedCountPerFrequencyPerUser;

      teamIdToRow.set(row.resourceName, row);
    }

    for (const groupResult of conversationAssistanceUsageStats?.resultGroups || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, teamIdToRow);
      if (!row) {
        continue;
      }
      row.conversationsWithAssistanceUsedPercent = groupResult.conversationsWithAssistanceUsedPercent;
      teamIdToRow.set(row.resourceName, row);
    }

    for (const groupResult of agentStats.data?.resultsGroupedByAttributeTypes || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, teamIdToRow);
      if (!row) {
        continue;
      }
      row.activeDays = groupResult.averageActiveDayCountPerUser;
    }

    for (const groupResult of conversationStats.data?.resultGroups || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, teamIdToRow);
      if (!row) {
        continue;
      }
      row.numOfConvos = groupResult.totalConversationCount;
      row.avgConvosPerDay = groupResult.averageConversationCountPerFrequencyPerUser;
      row.averageHandleTime = groupResult.averageHandleTime;
    }

    for (const groupResult of conversationStatsWithAA.data?.resultGroups || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, teamIdToRow);
      if (!row) {
        continue;
      }
      row.numOfConvosPoweredByAA = groupResult.totalConversationCount;
    }

    for (const groupResult of hintStatsByHintType[AttributeHintType.BEHAVIOR_HINT].data?.resultGroups || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, teamIdToRow);
      if (!row || groupResult.totalHintSentCount === 0) {
        continue;
      }

      row['behavioralHintFollowed'] = (row['behavioralHintFollowed'] || 0) + groupResult.totalHintFollowedCount;
      row['behavioralHintSent'] = (row['behavioralHintSent'] || 0) + groupResult.totalHintSentCount;
      row['behavioralHintFollowRate'] = row['behavioralHintSent']
        ? row['behavioralHintFollowed'] / row['behavioralHintSent']
        : undefined;
    }

    for (const groupResult of hintStatsByHintType[AttributeHintType.CHECKLIST_HINT].data?.resultGroups || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, teamIdToRow);
      if (!row || groupResult.totalHintSentCount === 0) {
        continue;
      }
      row['checklistHintFollowed'] = (row['checklistHintFollowed'] || 0) + groupResult.totalHintFollowedCount;
      row['checklistHintSent'] = (row['checklistHintSent'] || 0) + groupResult.totalHintSentCount;
      row['checklistHintFollowRate'] = row['checklistHintSent']
        ? row['checklistHintFollowed'] / row['checklistHintSent']
        : undefined;
    }
    for (const groupResult of hintStatsByHintType[AttributeHintType.GW_HINT].data?.resultGroups || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, teamIdToRow);
      if (!row || groupResult.totalHintSentCount === 0) {
        continue;
      }
      row['gwfHintFollowed'] = (row['gwfHintFollowed'] || 0) + groupResult.totalHintFollowedCount;
      row['gwfHintSent'] = (row['gwfHintSent'] || 0) + groupResult.totalHintSentCount;
      row['gwfHintFollowRate'] = row['gwfHintSent'] ? row['gwfHintFollowed'] / row['gwfHintSent'] : undefined;
    }
    for (const groupResult of hintStatsByHintType[AttributeHintType.KB_HINT].data?.resultGroups || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, teamIdToRow);
      if (!row || groupResult.totalHintSentCount === 0) {
        continue;
      }
      row['kbHintFollowed'] = (row['kbHintFollowed'] || 0) + groupResult.totalHintFollowedCount;
      row['kbHintSent'] = (row['kbHintSent'] || 0) + groupResult.totalHintSentCount;
      row['kbHintFollowRate'] = row['kbHintSent'] ? row['kbHintFollowed'] / row['kbHintSent'] : undefined;
    }

    for (const groupResult of knowledgeAssistStats.data?.resultsGroupedByAttributeTypes || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, teamIdToRow);
      if (!row) {
        continue;
      }
      row.genAiAnswersUsed = groupResult.totalGenAIAnswersUsedCount;
      row.genAiAnswersViewed = groupResult.totalGenAIAnswersViewedCount;
      row.knowledgeMomentDetectedCount = groupResult.totalKnowledgeMomentDetectedCount;
      row.genAiAnswersUsageRate = groupResult.totalGenAIAnswersViewedCount
        ? groupResult.totalGenAIAnswersUsedCount / groupResult.totalGenAIAnswersViewedCount
        : -1;

      row.genAiAnswersUsedPerFrequency = groupResult.averageGenAIAnswersUsedCountPerUser;
    }

    for (const row of teamIdToRow.values()) {
      const totalAssistanceUsed = sum(
        [
          row.behavioralHintFollowed,
          row.checklistHintFollowed,
          row.gwfHintFollowed,
          row.kbHintFollowed,
          Math.min(row.numOfSummaryUsed || 0, row.numOfSummarySent || 0), // TODO (kurt.choi) temp fix for auto summ metric bug - revisit when BE issue is resolved
          row.genAiAnswersUsed,
        ].map((num) => num || 0)
      );

      const totalAssistanceSent = sum(
        [
          row.behavioralHintSent,
          row.checklistHintSent,
          row.gwfHintSent,
          row.kbHintSent,
          row.numOfSummarySent,
          row.knowledgeMomentDetectedCount,
        ].map((num) => num || 0)
      );

      const totalHintsFollowed = sum(
        [row.behavioralHintFollowed, row.checklistHintFollowed, row.gwfHintFollowed, row.kbHintFollowed].map(
          (num) => num || 0
        )
      );

      const totalHintsSent = sum(
        [row.behavioralHintSent, row.checklistHintSent, row.gwfHintSent, row.kbHintSent].map((num) => num || 0)
      );

      row.overallEngagement = totalAssistanceUsed / totalAssistanceSent || undefined;
      row.convosPoweredByAARate =
        Math.min(row.numOfConvosPoweredByAA || 0, row.numOfConvos || 0) / (row.numOfConvos || 0);
      row.totalHintsFollowed = totalHintsFollowed;
      row.totalHintsSent = totalHintsSent;
      row.totalHintsFollowedRate = totalHintsFollowed / totalHintsSent || undefined;
    }

    return Array.from(teamIdToRow.values());
  }, [
    assistanceStats.isPending,
    assistanceStats.error,
    assistanceStats.data?.resultsGroupedByAttributeTypes,
    allHintTypeLoading,
    hintStatsByHintType,
    knowledgeAssistStats.isLoading,
    knowledgeAssistStats.error,
    knowledgeAssistStats.data?.resultsGroupedByAttributeTypes,
    agentStats.isLoading,
    agentStats.error,
    agentStats.data?.resultsGroupedByAttributeTypes,
    conversationStats.isPending,
    conversationStats.error,
    conversationStats.data?.resultGroups,
    conversationAssistanceUsageStats,
    conversationStatsWithAA.isPending,
    conversationStatsWithAA.error,
    conversationStatsWithAA.data?.resultGroups,
  ]);

  const columns = useMemo<ColumnDef<TeamAssistanceUsedLeaderboardRow>[]>(() => {
    let stickyHeadersWidth = 0;
    const stickyColumns = [];

    if (visibleColumns.has('team')) {
      stickyColumns.push(
        columnHelper.accessor('team', {
          header: t('team-name', 'Team name'),
          cell: (cell: CellContext<TeamAssistanceUsedLeaderboardRow, string>) => {
            const team = cell.getValue();
            return (
              <SharedTooltip label={`Show insights for team ${team}`}>
                <Link
                  to={`${generatePathWithParams(ABS_ROUTES.INSIGHTS.PERFORMANCE.CONVERSATIONS, {
                    query: usersTeamsGroupsFiltersQueryRecord(cell.row.original.teamResourceName),
                  })}`}
                >
                  {team}
                </Link>
              </SharedTooltip>
            );
          },
          meta: { sticky: true, fixedWidth: TEAM_COL_WIDTH },
        })
      );
      stickyHeadersWidth += TEAM_COL_WIDTH;
    }

    if (visibleColumns.has('avgConvosPerDay')) {
      stickyColumns.push(
        columnHelper.accessor('avgConvosPerDay', {
          header: t('convos-per-day-per-agent', '# of convos (per day per agent)'),
          meta: { sticky: true, fixedWidth: STICKY_STATS_COL_WIDTH },
          cell: (cell) => {
            if (cell.getValue() === undefined) {
              return 'N/A';
            }
            return formatNumberToFixed(cell.getValue(), 1);
          },
        })
      );
      stickyHeadersWidth += STICKY_STATS_COL_WIDTH;
    }

    if (visibleColumns.has('activeDays')) {
      stickyColumns.push(
        columnHelper.accessor('activeDays', {
          header: t('average-active-days', 'Average active days'),
          meta: { sticky: true, fixedWidth: STICKY_STATS_COL_WIDTH },
          cell: (cell) => {
            if (cell.getValue() === undefined) {
              return 'N/A';
            }
            return formatNumberToFixed(cell.getValue(), 0);
          },
        })
      );
      stickyHeadersWidth += STICKY_STATS_COL_WIDTH;
    }

    if (visibleColumns.has('convosPoweredByAARate')) {
      stickyColumns.push(
        columnHelper.accessor('convosPoweredByAARate', {
          header: t('convos-powered-by-aa-percent', 'Convos powered by AA %'),
          meta: { sticky: true, fixedWidth: STICKY_STATS_COL_WIDTH },
          cell: (cell) => {
            if (cell.getValue() === undefined) {
              return 'N/A';
            }
            return formatNumberToFixed((cell.getValue() as number) * 100, 1);
          },
        })
      );
      stickyHeadersWidth += STICKY_STATS_COL_WIDTH;
    }

    if (visibleColumns.has('conversationsWithAssistanceUsedPercent')) {
      stickyColumns.push(
        columnHelper.accessor('conversationsWithAssistanceUsedPercent', {
          header: t('conversations-with-assistance-used-percent', 'Conversations with assistance used %'),
          meta: { sticky: true, fixedWidth: STICKY_STATS_COL_WIDTH },
          cell: (cell) => {
            const rate = cell.getValue();
            return rate === undefined ? 'N/A' : formatNumberToFixed(rate, 1);
          },
        })
      );
      stickyHeadersWidth += STICKY_STATS_COL_WIDTH;
    }

    const teamLeaderboardColumns: ColumnDef<TeamAssistanceUsedLeaderboardRow, unknown>[] = [
      columnHelper.group({
        header: ' ',
        columns: stickyColumns,
        meta: { sticky: true, fixedWidth: stickyHeadersWidth },
      }),
    ];

    const assistanceUsedColumnGroup = columnHelper.group({
      id: 'assistanceUsed',
      columns: [
        columnHelper.accessor('assistanceUsed', {
          header: t('assistance-used-per-day', 'Assistance used (per day)'),
          meta: { cellBackgroundColor: createBackgroundColorFn(getHeatmapColors('blue', isDark)) },
          cell: (cell) => {
            if (cell.getValue() === undefined) {
              return 'N/A';
            }
            return formatNumberToFixed(cell.getValue() as number, 1);
          },
        }),
      ],
    });
    teamLeaderboardColumns.push(assistanceUsedColumnGroup);

    const overallEngagementColumnGroup = getOverallEngagementColumns(columnHelper, isDark);
    teamLeaderboardColumns.push(overallEngagementColumnGroup);

    const ahtColumnGroup = getAHTColumns(columnHelper, visibleColumns, enableATT);
    if (ahtColumnGroup) {
      teamLeaderboardColumns.push(ahtColumnGroup);
    }

    const hintColumnGroup = getHintColumns(columnHelper, visibleColumns);
    if (hintColumnGroup) {
      teamLeaderboardColumns.push(hintColumnGroup);
    }

    const autoSummarizationColumnGroup = getAutoSummarizationColumns(columnHelper, visibleColumns);
    if (autoSummarizationColumnGroup) {
      teamLeaderboardColumns.push(autoSummarizationColumnGroup);
    }

    const genAIColumnGroup = getGenAIColumns(columnHelper, visibleColumns);
    if (genAIColumnGroup) {
      teamLeaderboardColumns.push(genAIColumnGroup);
    }

    const assistanceColumnGroup = getAssistanceColumns(columnHelper, visibleColumns);
    if (assistanceColumnGroup) {
      teamLeaderboardColumns.push(assistanceColumnGroup);
    }

    return teamLeaderboardColumns;
  }, [columnHelper, visibleColumns, t, enableATT, isDark]);

  const exportCSV = useCallback(
    (hiddenColumnIds: string[]) => {
      const headers = new Map<keyof TeamAssistanceUsedLeaderboardRow, string>([
        ...getAssistanceUsedLeaderboardRowCSVHeader(enableATT),
      ]);
      const hiddenKeys = new Set(hiddenColumnIds) as Set<keyof TeamAssistanceUsedLeaderboardRow>;

      const file = new File(
        [getCSV(rows || [], headers, hiddenKeys, visibleColumns)],
        'assistance_used_team_leaderboard.csv'
      );
      saveAs(file);
    },
    [rows, visibleColumns, enableATT]
  );

  return {
    directorTableProps: directorTableProps,
    rows: rows,
    columns: columns,
    exportCSV: exportCSV,
  };
}
