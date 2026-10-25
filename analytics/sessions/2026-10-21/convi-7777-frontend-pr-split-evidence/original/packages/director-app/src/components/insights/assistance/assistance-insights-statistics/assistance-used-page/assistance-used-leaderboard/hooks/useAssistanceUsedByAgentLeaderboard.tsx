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
import { parseUserResourceId } from '@cresta/director-api';
import {
  createBackgroundColorFn,
  defaultNumberColumnDef,
  SharedTooltip,
  withDefaultValues,
} from '@cresta/director-components';
import { formatNumberToFixed } from '@cresta/director-utils';
import { AttributeHintType } from '@cresta/web-client';

import { getHeatmapColors } from '@/components/gridtable';
import { ProfileAvatarWithLoginStatus } from '@/components/users';
import type { DirectorTableSortingStateProps } from '@/hooks/useCustomSorting';
import { useCustomSorting } from '@/hooks/useCustomSorting';
import { useFeatureFlag } from '@/hooks/useFeatureFlag';
import { useTranslation } from '@/hooks/useTranslation';
import { ABS_ROUTES } from '@/routes';
import { generateEncodedPath, getCSV } from '@/utils';

import type {
  AssistanceStatsBaseResult,
  ConversationStatsQueryResult,
  ManagedAgentStats,
  ManagedKnowledgeAssistStats,
} from '../../../../../hooks';
import type { ManagedHintStats } from '../../../../../types';
import type { AgentAssistanceUsedLeaderboardRow } from '../types';
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

// define fixed size for sticky columns
const ICON_COL_WIDTH = 60;
const NAME_COL_WIDTH = 150;
const TEAM_COL_WIDTH = 150;
const STICKY_STATS_COL_WIDTH = 80;

function createAgentAssistanceUsedLeaderboardRow(userResourceName: string): AgentAssistanceUsedLeaderboardRow {
  return {
    resourceName: userResourceName,
    name: '',
    username: '',
    team: '',
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
  agentIdToData: Map<string, AgentAssistanceUsedLeaderboardRow>
): AgentAssistanceUsedLeaderboardRow | undefined {
  const groupedByAttributes = groupResult.groupedByAttributes;
  const userResourceName = groupedByAttributes.user?.name;
  const fullName = groupedByAttributes.user?.fullName;

  if (!userResourceName || !fullName || groupedByAttributes.user?.isDevUser) {
    return;
  }

  return agentIdToData.has(userResourceName)
    ? agentIdToData.get(userResourceName)
    : createAgentAssistanceUsedLeaderboardRow(userResourceName);
}

export function useAssistanceUsedByAgentLeaderboard(
  appType: DirectorEnvironment,
  assistanceStats: AssistanceStatsBaseResult,
  hintStatsByHintType: Record<AttributeHintType, ManagedHintStats>,
  allHintTypeLoading: boolean,
  knowledgeAssistStats: ManagedKnowledgeAssistStats,
  displayAsAgent: boolean,
  conversationStats: ConversationStatsQueryResult,
  conversationStatsWithAA: ConversationStatsQueryResult,
  agentStats: ManagedAgentStats,
  conversationAssistanceUsageStats: ConversationAssistanceUsageStatsResponse | undefined
): {
  directorTableProps: DirectorTableSortingStateProps;
  rows: AgentAssistanceUsedLeaderboardRow[] | undefined;
  columns: ColumnDef<AgentAssistanceUsedLeaderboardRow>[];
  exportCSV: (hiddenColumnIds: string[]) => void;
} {
  const { t } = useTranslation('director-app-insights', {
    keyPrefix: 'assistance.assistance-used-by-agent-leaderboard',
  });
  const enableATT = useFeatureFlag('enableATT');
  const isDark = useComputedColorScheme() === 'dark';
  const { directorTableProps, columnHelperAddition } = useCustomSorting([{ desc: true, id: 'overallEngagement' }]);
  const columnHelper = useMemo(() => {
    return withDefaultValues<AgentAssistanceUsedLeaderboardRow>(
      createColumnHelper<AgentAssistanceUsedLeaderboardRow>(),
      {
        ...defaultNumberColumnDef,
        ...columnHelperAddition,
      }
    );
  }, [columnHelperAddition]);

  const visibleColumnsForAssistanceUsedLeaderboards = useVisibleColumnsForAssistanceUsedLeaderboards(
    appType,
    assistanceStats,
    knowledgeAssistStats,
    hintStatsByHintType,
    displayAsAgent
  );

  const visibleColumns: Set<keyof AgentAssistanceUsedLeaderboardRow> = useMemo((): Set<
    keyof AgentAssistanceUsedLeaderboardRow
  > => {
    const visibleColumns = new Set(visibleColumnsForAssistanceUsedLeaderboards.values()) as Set<
      keyof AgentAssistanceUsedLeaderboardRow
    >;
    visibleColumns.add('username');
    visibleColumns.add('name');
    return visibleColumns;
  }, [visibleColumnsForAssistanceUsedLeaderboards]);

  const rows: AgentAssistanceUsedLeaderboardRow[] | undefined = useMemo(():
    | AgentAssistanceUsedLeaderboardRow[]
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

    const agentIdToData = new Map<string, AgentAssistanceUsedLeaderboardRow>();
    for (const groupResult of assistanceStats.data?.resultsGroupedByAttributeTypes || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, agentIdToData);
      if (!row) {
        continue;
      }
      row.name = groupResult.groupedByAttributes.user?.fullName || '';
      row.username = groupResult.groupedByAttributes.user?.username || '';
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

      agentIdToData.set(row.resourceName, row);
    }

    for (const groupResult of conversationAssistanceUsageStats?.resultGroups || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, agentIdToData);
      if (!row) {
        continue;
      }
      row.name ||= groupResult.groupedByAttributes.user?.fullName || '';
      row.username ||= groupResult.groupedByAttributes.user?.username || '';
      row.conversationsWithAssistanceUsedPercent = groupResult.conversationsWithAssistanceUsedPercent;
      agentIdToData.set(row.resourceName, row);
    }

    for (const groupResult of agentStats.data?.resultsGroupedByAttributeTypes || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, agentIdToData);
      if (!row) {
        continue;
      }
      row.activeDays = groupResult.averageActiveDayCountPerUser;
    }

    for (const groupResult of conversationStats.data?.resultGroups || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, agentIdToData);
      if (!row) {
        continue;
      }
      row.numOfConvos = groupResult.totalConversationCount;
      row.avgConvosPerDay = groupResult.averageConversationCountPerFrequencyPerUser;
      row.averageHandleTime = groupResult.averageHandleTime;
    }

    for (const groupResult of conversationStatsWithAA.data?.resultGroups || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, agentIdToData);
      if (!row) {
        continue;
      }
      row.numOfConvosPoweredByAA = groupResult.totalConversationCount;
    }

    for (const groupResult of hintStatsByHintType[AttributeHintType.BEHAVIOR_HINT].data?.resultGroups || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, agentIdToData);
      if (!row || groupResult.totalHintSentCount === 0) {
        continue;
      }
      row['behavioralHintFollowRate'] = groupResult.totalHintSentCount
        ? groupResult.totalHintFollowedCount / groupResult.totalHintSentCount
        : undefined;
      row['behavioralHintFollowed'] = groupResult.totalHintFollowedCount;
      row['behavioralHintSent'] = groupResult.totalHintSentCount;
    }
    for (const groupResult of hintStatsByHintType[AttributeHintType.CHECKLIST_HINT].data?.resultGroups || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, agentIdToData);
      if (!row || groupResult.totalHintSentCount === 0) {
        continue;
      }
      row['checklistHintFollowRate'] = groupResult.totalHintSentCount
        ? groupResult.totalHintFollowedCount / groupResult.totalHintSentCount
        : undefined;
      row['checklistHintFollowed'] = groupResult.totalHintFollowedCount;
      row['checklistHintSent'] = groupResult.totalHintSentCount;
    }
    for (const groupResult of hintStatsByHintType[AttributeHintType.GW_HINT].data?.resultGroups || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, agentIdToData);
      if (!row || groupResult.totalHintSentCount === 0) {
        continue;
      }
      row['gwfHintFollowRate'] = groupResult.totalHintSentCount
        ? groupResult.totalHintFollowedCount / groupResult.totalHintSentCount
        : undefined;
      row['gwfHintFollowed'] = groupResult.totalHintFollowedCount;
      row['gwfHintSent'] = groupResult.totalHintSentCount;
    }
    for (const groupResult of hintStatsByHintType[AttributeHintType.KB_HINT].data?.resultGroups || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, agentIdToData);
      if (!row || groupResult.totalHintSentCount === 0) {
        continue;
      }
      row['kbHintFollowRate'] = groupResult.totalHintSentCount
        ? groupResult.totalHintFollowedCount / groupResult.totalHintSentCount
        : undefined;
      row['kbHintFollowed'] = groupResult.totalHintFollowedCount;
      row['kbHintSent'] = groupResult.totalHintSentCount;
    }

    for (const groupResult of knowledgeAssistStats.data?.resultsGroupedByAttributeTypes || []) {
      const row = getCorrectRowFromLeaderboard(groupResult, agentIdToData);
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

    for (const row of agentIdToData.values()) {
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
          row.genAiAnswersViewed,
        ].map((num) => num || 0)
      );

      const totalHintsSent = sum(
        [row.behavioralHintSent, row.checklistHintSent, row.gwfHintSent, row.kbHintSent].map((num) => num || 0)
      );

      const totalHintsFollowed = sum(
        [row.behavioralHintFollowed, row.checklistHintFollowed, row.gwfHintFollowed, row.kbHintFollowed].map(
          (num) => num || 0
        )
      );

      row.overallEngagement = totalAssistanceUsed / totalAssistanceSent || undefined;
      row.convosPoweredByAARate =
        Math.min(row.numOfConvosPoweredByAA || 0, row.numOfConvos || 0) / (row.numOfConvos || 0);
      row.totalHintsFollowedRate = totalHintsFollowed / totalHintsSent || undefined;
      row.totalHintsFollowed = totalHintsFollowed;
      row.totalHintsSent = totalHintsSent;
    }

    return Array.from(agentIdToData.values());
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

  const columns = useMemo<ColumnDef<AgentAssistanceUsedLeaderboardRow>[]>(() => {
    let stickyHeadersWidth = 0;
    const stickyColumns = [];

    if (visibleColumns.has('name')) {
      stickyColumns.push(
        columnHelper.accessor('name', {
          id: 'icon',
          header: t('icon', 'Icon'),
          cell: (cell) => (
            <ProfileAvatarWithLoginStatus displayName={cell.getValue()} username={cell.row.original.username} />
          ),
          meta: { sticky: true, fixedWidth: ICON_COL_WIDTH },
        })
      );
      stickyHeadersWidth += ICON_COL_WIDTH;
    }

    if (visibleColumns.has('name')) {
      stickyColumns.push(
        columnHelper.accessor('name', {
          header: t('name', 'Name'),
          cell: (cell: CellContext<AgentAssistanceUsedLeaderboardRow, string>) => {
            const row = cell.row.original;
            if (displayAsAgent) {
              return (
                <SharedTooltip label={`${cell.getValue()} (${row.username})`} withArrow position="top-start">
                  <span>{cell.getValue()}</span>
                </SharedTooltip>
              );
            }
            return (
              <SharedTooltip
                label={`Show agent overview for ${cell.getValue()} (${row.username})`}
                withArrow
                position="top-start"
              >
                <Link
                  to={generateEncodedPath(ABS_ROUTES.COACHING.WORKFLOW.AGENT.OVERVIEW, {
                    agentId: parseUserResourceId(row.resourceName),
                  })}
                >
                  {cell.getValue()}
                </Link>
              </SharedTooltip>
            );
          },
          meta: { sticky: true, fixedWidth: NAME_COL_WIDTH },
        })
      );
      stickyHeadersWidth += NAME_COL_WIDTH;
    }
    if (visibleColumns.has('team')) {
      stickyColumns.push(
        columnHelper.accessor('team', {
          header: t('team', 'Team'),
          meta: { sticky: true, fixedWidth: TEAM_COL_WIDTH },
        })
      );
      stickyHeadersWidth += TEAM_COL_WIDTH;
    }

    if (visibleColumns.has('avgConvosPerDay')) {
      stickyColumns.push(
        columnHelper.accessor('avgConvosPerDay', {
          header: t('convos-per-day-per-agent', '# of convos (per day)'),
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
          header: t('active-days', 'Active Days'),
          meta: { sticky: true, fixedWidth: STICKY_STATS_COL_WIDTH },
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

    const agentLeaderboardColumns: ColumnDef<AgentAssistanceUsedLeaderboardRow, unknown>[] = [
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
    agentLeaderboardColumns.push(assistanceUsedColumnGroup);

    const overallEngagementColumnGroup = getOverallEngagementColumns(columnHelper, isDark);
    agentLeaderboardColumns.push(overallEngagementColumnGroup);

    const ahtColumnGroup = getAHTColumns(columnHelper, visibleColumns, enableATT);
    if (ahtColumnGroup) {
      agentLeaderboardColumns.push(ahtColumnGroup);
    }

    const hintColumnGroup = getHintColumns(columnHelper, visibleColumns);
    if (hintColumnGroup) {
      agentLeaderboardColumns.push(hintColumnGroup);
    }

    const autoSummarizationColumnGroup = getAutoSummarizationColumns(columnHelper, visibleColumns);
    if (autoSummarizationColumnGroup) {
      agentLeaderboardColumns.push(autoSummarizationColumnGroup);
    }

    const genAIColumnGroup = getGenAIColumns(columnHelper, visibleColumns);
    if (genAIColumnGroup) {
      agentLeaderboardColumns.push(genAIColumnGroup);
    }

    const assistanceColumnGroup = getAssistanceColumns(columnHelper, visibleColumns);
    if (assistanceColumnGroup) {
      agentLeaderboardColumns.push(assistanceColumnGroup);
    }

    return agentLeaderboardColumns;
  }, [columnHelper, displayAsAgent, visibleColumns, t, enableATT, isDark]);

  const exportCSV = useCallback(
    (hiddenColumnIds: string[]) => {
      const headers = new Map<keyof AgentAssistanceUsedLeaderboardRow, string>([
        ['username', 'Username'],
        ['name', 'Name'],
        ...getAssistanceUsedLeaderboardRowCSVHeader(enableATT),
      ]);
      const hiddenKeys = new Set(hiddenColumnIds) as Set<keyof AgentAssistanceUsedLeaderboardRow>;

      const file = new File(
        [getCSV(rows || [], headers, hiddenKeys, visibleColumns)],
        'assistance_used_agent_leaderboard.csv'
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
