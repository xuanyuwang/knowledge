import { Flex } from '@mantine/core';
import type { ColumnDef, ColumnHelper } from '@tanstack/react-table';
import { isNaN } from 'lodash-es';

import { createBackgroundColorFn, InfoIcon } from '@cresta/director-components';
import { convertSecondsIntoMinuteAndSecondsString, formatNumberToFixed, getI18n } from '@cresta/director-utils';

import { getHeatmapColors } from '@/components/gridtable';
import { getAHTTerminology } from '@/utils/terminologyUtils';

import type { AssistanceUsedLeaderboardRow } from './types';

export function getHintColumns<TData extends AssistanceUsedLeaderboardRow>(
  columnHelper: ColumnHelper<TData>,
  visibleColumns: Set<keyof TData>
): ColumnDef<TData> | undefined {
  const i18n = getI18n();
  const t = i18n.getFixedT(null, 'director-app-insights', 'assistance.assistance-used-leaderboard.hints');
  const tCommon = i18n.getFixedT(null, 'director-app', 'common');
  const castedColumnHelper = columnHelper as unknown as ColumnHelper<AssistanceUsedLeaderboardRow>;
  const columns = [];

  if (visibleColumns.has('totalHintsFollowedRate')) {
    columns.push(
      castedColumnHelper.accessor('totalHintsFollowedRate', {
        header: t('all-hints-engagement', 'All hints engagement (%)'),
        cell: (cell) => {
          if (cell.getValue() === undefined || isNaN(cell.getValue())) {
            return tCommon('n-a', 'N/A');
          }
          const val = cell.getValue() || 0;
          const rate = val * 100;
          return `${formatNumberToFixed(rate, 1)}%`;
        },
      })
    );
  }

  if (visibleColumns.has('hintsFollowed')) {
    columns.push(
      castedColumnHelper.accessor('hintsFollowed', {
        header: t('hints-followed-per-day', 'Hints followed (per day)'),
        cell: (cell) => {
          if (cell.getValue() === undefined || isNaN(cell.getValue())) {
            return tCommon('n-a', 'N/A');
          }
          return formatNumberToFixed(cell.getValue(), 1);
        },
      })
    );
  }

  if (visibleColumns.has('behavioralHintFollowRate')) {
    columns.push(
      castedColumnHelper.accessor('behavioralHintFollowRate', {
        header: t('behavioral-hint-engagement', 'Behavioral hint engagement (%)'),
        cell: (cell) => {
          if (cell.getValue() === undefined || isNaN(cell.getValue())) {
            return tCommon('n-a', 'N/A');
          }
          const val = cell.getValue() || 0;
          const rate = val * 100;
          return `${formatNumberToFixed(rate, 1)}%`;
        },
      })
    );
  }
  if (visibleColumns.has('kbHintFollowRate')) {
    columns.push(
      castedColumnHelper.accessor('kbHintFollowRate', {
        header: t('kb-hint-engagement', 'KB hint engagement (%)'),
        cell: (cell) => {
          if (cell.getValue() === undefined || isNaN(cell.getValue())) {
            return tCommon('n-a', 'N/A');
          }
          const val = cell.getValue() || 0;
          const rate = val * 100;
          return `${formatNumberToFixed(rate, 1)}%`;
        },
      })
    );
  }
  if (visibleColumns.has('gwfHintFollowRate')) {
    columns.push(
      castedColumnHelper.accessor('gwfHintFollowRate', {
        header: t('gwf-hint-engagement', 'GWF hint engagement (%)'),
        cell: (cell) => {
          if (cell.getValue() === undefined || isNaN(cell.getValue())) {
            return tCommon('n-a', 'N/A');
          }
          const val = cell.getValue() || 0;
          const rate = val * 100;
          return `${formatNumberToFixed(rate, 1)}%`;
        },
      })
    );
  }
  if (visibleColumns.has('checklistHintFollowRate')) {
    columns.push(
      castedColumnHelper.accessor('checklistHintFollowRate', {
        header: t('checklist-engagement', 'Checklist engagement (%)'),
        cell: (cell) => {
          if (cell.getValue() === undefined || isNaN(cell.getValue())) {
            return tCommon('n-a', 'N/A');
          }
          const val = cell.getValue() || 0;
          const rate = val * 100;
          return `${formatNumberToFixed(rate, 1)}%`;
        },
      })
    );
  }

  if (!columns.length) {
    return;
  }

  return castedColumnHelper.group({
    id: 'hintsUsage',
    columns: columns,
  }) as ColumnDef<TData>;
}

export function getAHTColumns<TData extends AssistanceUsedLeaderboardRow>(
  columnHelper: ColumnHelper<TData>,
  visibleColumns: Set<keyof TData>,
  enableATT?: boolean
): ColumnDef<TData> | undefined {
  const i18n = getI18n();
  const t = i18n.getFixedT(null, 'director-app-insights', 'assistance.assistance-used-leaderboard.aht');
  const tCommon = i18n.getFixedT(null, 'director-app', 'common');
  const { fullName } = getAHTTerminology(enableATT);
  const castedColumnHelper = columnHelper as unknown as ColumnHelper<AssistanceUsedLeaderboardRow>;
  const columns = [];

  if (visibleColumns.has('averageHandleTime')) {
    columns.push(
      castedColumnHelper.accessor('averageHandleTime', {
        header: t('per-convo', { defaultValue: '{{name}} (per convo)', name: fullName }),
        cell: (cell) => {
          if (cell.getValue() === undefined) {
            return tCommon('n-a', 'N/A');
          }
          return convertSecondsIntoMinuteAndSecondsString(cell.getValue());
        },
      })
    );
  }

  return castedColumnHelper.group({
    id: 'averageHandleTime',
    columns: columns,
  }) as ColumnDef<TData>;
}

export function getAutoSummarizationColumns<TData extends AssistanceUsedLeaderboardRow>(
  columnHelper: ColumnHelper<TData>,
  visibleColumns: Set<keyof TData>
): ColumnDef<TData> | undefined {
  const i18n = getI18n();
  const t = i18n.getFixedT(null, 'director-app-insights', 'assistance.assistance-used-leaderboard.auto-summary');
  const tCommon = i18n.getFixedT(null, 'director-app', 'common');
  const castedColumnHelper = columnHelper as unknown as ColumnHelper<AssistanceUsedLeaderboardRow>;
  const columns = [];

  if (visibleColumns.has('averageConvoWithSummary')) {
    columns.push(
      castedColumnHelper.accessor('averageConvoWithSummary', {
        header: t('convos-with-realtime-summaries-per-day', 'Conversations w. Realtime Summaries (per day)'),
        cell: (cell) => {
          if (cell.getValue() === undefined) {
            return tCommon('n-a', 'N/A');
          }
          return formatNumberToFixed(cell.getValue(), 1);
        },
      })
    );
  }
  if (visibleColumns.has('summaryPerFrequency')) {
    columns.push(
      castedColumnHelper.accessor('summaryPerFrequency', {
        header: t('summary-used-per-day', 'Summary used (per day)'),
        cell: (cell) => {
          if (cell.getValue() === undefined) {
            return tCommon('n-a', 'N/A');
          }
          return `${formatNumberToFixed(cell.getValue(), 1)}`;
        },
      })
    );
  }
  if (visibleColumns.has('summaryUsageRate')) {
    columns.push(
      castedColumnHelper.accessor('summaryUsageRate', {
        header: t('percent-used', '% used'),
        cell: (cell) => {
          if (cell.getValue() === undefined || isNaN(cell.getValue())) {
            return tCommon('n-a', 'N/A');
          }
          const val = cell.getValue() || 0;
          const rate = val * 100;
          return `${formatNumberToFixed(rate, 1)}%`;
        },
      })
    );
  }

  if (!columns.length) {
    return;
  }

  return castedColumnHelper.group({
    id: 'autoSummaryUsage',
    columns: columns,
  }) as ColumnDef<TData>;
}

export function getGenAIColumns<TData extends AssistanceUsedLeaderboardRow>(
  columnHelper: ColumnHelper<TData>,
  visibleColumns: Set<keyof TData>
): ColumnDef<TData> | undefined {
  const i18n = getI18n();
  const t = i18n.getFixedT(null, 'director-app-insights', 'assistance.assistance-used-leaderboard.gen-ai');
  const tCommon = i18n.getFixedT(null, 'director-app', 'common');
  const castedColumnHelper = columnHelper as unknown as ColumnHelper<AssistanceUsedLeaderboardRow>;
  const columns = [];

  if (visibleColumns.has('genAiAnswersUsedPerFrequency')) {
    columns.push(
      castedColumnHelper.accessor('genAiAnswersUsedPerFrequency', {
        header: t('answers-used-per-day', 'GenAI Answers used (per day)'),
        cell: (cell) => {
          if (cell.getValue() === undefined) {
            return tCommon('n-a', 'N/A');
          }
          return `${formatNumberToFixed(cell.getValue(), 1)}`;
        },
      })
    );
  }
  if (visibleColumns.has('genAiAnswersUsageRate')) {
    columns.push(
      castedColumnHelper.accessor('genAiAnswersUsageRate', {
        header: t('answer-percent-used', 'GenAI Answer % used'),
        cell: (cell) => {
          if (cell.getValue() === undefined || isNaN(cell.getValue())) {
            return tCommon('n-a', 'N/A');
          }
          if (cell.getValue() === -1) {
            return '--';
          }
          const val = cell.getValue() || 0;
          const rate = val * 100;
          return `${formatNumberToFixed(rate, 1)}%`;
        },
      })
    );
  }

  if (!columns.length) {
    return;
  }

  return castedColumnHelper.group({
    id: 'genAiUsage',
    columns: columns,
  }) as ColumnDef<TData>;
}

export function getAssistanceColumns<TData extends AssistanceUsedLeaderboardRow>(
  columnHelper: ColumnHelper<TData>,
  visibleColumns: Set<keyof TData>
): ColumnDef<TData> | undefined {
  const i18n = getI18n();
  const t = i18n.getFixedT(null, 'director-app-insights', 'assistance.assistance-used-leaderboard.assistance');
  const tCommon = i18n.getFixedT(null, 'director-app', 'common');
  const castedColumnHelper = columnHelper as unknown as ColumnHelper<AssistanceUsedLeaderboardRow>;
  const columns = [];

  if (visibleColumns.has('searchesPerFrequency')) {
    columns.push(
      castedColumnHelper.accessor('searchesPerFrequency', {
        header: t('searches-per-day', 'Searches (per day)'),
        cell: (cell) => {
          if (cell.getValue() === undefined) {
            return tCommon('n-a', 'N/A');
          }
          return `${formatNumberToFixed(cell.getValue(), 1)}`;
        },
      })
    );
  }
  if (visibleColumns.has('gwPerFrequency')) {
    columns.push(
      castedColumnHelper.accessor('gwPerFrequency', {
        header: t('guided-workflow-per-day', 'Guided workflow (per day)'),
        cell: (cell) => {
          if (cell.getValue() === undefined) {
            return tCommon('n-a', 'N/A');
          }
          return `${formatNumberToFixed(cell.getValue(), 1)}`;
        },
      })
    );
  }
  if (visibleColumns.has('suggestionsPerFrequency')) {
    columns.push(
      castedColumnHelper.accessor('suggestionsPerFrequency', {
        header: t('suggestions', 'Suggestions'),
        cell: (cell) => {
          if (cell.getValue() === undefined) {
            return tCommon('n-a', 'N/A');
          }
          return `${formatNumberToFixed(cell.getValue(), 1)}`;
        },
      })
    );
  }
  if (visibleColumns.has('smartComposePerFrequency')) {
    columns.push(
      castedColumnHelper.accessor('smartComposePerFrequency', {
        header: t('smart-compose-per-day', 'Smart Compose (per day)'),
        cell: (cell) => {
          if (cell.getValue() === undefined) {
            return tCommon('n-a', 'N/A');
          }
          return `${formatNumberToFixed(cell.getValue(), 1)}`;
        },
      })
    );
  }
  if (visibleColumns.has('notesPerFrequency')) {
    columns.push(
      castedColumnHelper.accessor('notesPerFrequency', {
        header: t('notes-per-day', 'Notes (per day)'),
        cell: (cell) => {
          if (cell.getValue() === undefined) {
            return tCommon('n-a', 'N/A');
          }
          return `${formatNumberToFixed(cell.getValue(), 1)}`;
        },
      })
    );
  }

  if (!columns.length) {
    return;
  }

  return castedColumnHelper.group({
    id: 'assistanceUsage',
    columns: columns,
  }) as ColumnDef<TData>;
}

export function getOverallEngagementColumns<TData extends AssistanceUsedLeaderboardRow>(
  columnHelper: ColumnHelper<TData>,
  isDark: boolean
): ColumnDef<TData> {
  const i18n = getI18n();
  const t = i18n.getFixedT(null, 'director-app-insights', 'assistance.assistance-used-leaderboard.overall-engagement');
  const tCommon = i18n.getFixedT(null, 'director-app', 'common');
  const castedColumnHelper = columnHelper as unknown as ColumnHelper<AssistanceUsedLeaderboardRow>;

  return castedColumnHelper.group({
    id: 'overallEngagement',
    columns: [
      castedColumnHelper.accessor('overallEngagement', {
        header: () => (
          <Flex gap="xs" align="center">
            {t('label', 'Overall Engagement (%)')}
            <InfoIcon
              tooltipLabel={t(
                'tooltip',
                'Overall engagement % includes all types of hints (except for reminder hints), summary, Gen AI answers'
              )}
              size="xs"
            />
          </Flex>
        ),
        meta: {
          cellBackgroundColor: createBackgroundColorFn(getHeatmapColors('blue', isDark)),
          headerDisplayName: t('label', 'Overall Engagement (%)'),
        },
        cell: (cell) => {
          if (cell.getValue() === undefined || isNaN(cell.getValue())) {
            return tCommon('n-a', 'N/A');
          }
          const val = (cell.getValue() as number) || 0;
          const rate = val * 100;
          return `${formatNumberToFixed(rate, 1)}%`;
        },
      }),
    ],
  }) as ColumnDef<TData>;
}

export function getAssistanceUsedLeaderboardRowCSVHeader(
  enableATT?: boolean
): Map<keyof AssistanceUsedLeaderboardRow, string> {
  const i18n = getI18n();
  const t = i18n.getFixedT(null, 'director-app-insights', 'assistance.assistance-used-leaderboard.csv');
  const { fullName } = getAHTTerminology(enableATT);
  return new Map<keyof AssistanceUsedLeaderboardRow, string>([
    ['team', t('team', 'Team')],
    ['avgConvosPerDay', t('convos-per-day', '# of convos (per day)')],
    ['activeDays', t('active-days', 'Active days')],
    ['convosPoweredByAARate', t('convos-powered-by-aa', 'Convos powered by AA (%)')],
    [
      'conversationsWithAssistanceUsedPercent',
      t('conversations-with-assistance-used', 'Conversations with assistance used (%)'),
    ],
    ['assistanceUsed', t('assistance-used-per-day', '# of Assistance used (per day)')],
    ['overallEngagement', t('overall-engagement', 'Overall Engagement (%)')],
    ['averageHandleTime', `${fullName} (seconds)`], // getAHTTerminology already includes i18n
    ['totalHintsFollowedRate', t('all-hints-engagement', 'All hints engagement (%)')],
    ['hintsFollowed', t('hints-followed-per-day', 'Hints followed (per day)')],
    ['behavioralHintFollowRate', t('behavioral-hint-followed', 'Behavioral hint followed (%)')],
    ['kbHintFollowRate', t('kb-hint-followed', 'KB hint followed (%)')],
    ['gwfHintFollowRate', t('gwf-hint-followed', 'GWF hint followed (%)')],
    ['checklistHintFollowRate', t('checklist-followed', 'Checklist followed (%)')],
    [
      'averageConvoWithSummary',
      t('convos-with-realtime-summaries-per-day', 'Conversations w. Realtime Summaries (per day)'),
    ],
    ['summaryPerFrequency', t('summary-used-per-day', 'Summary Used (per day)')],
    ['summaryUsageRate', t('summary-used-percent', 'Summary Used (%)')],
    ['genAiAnswersUsedPerFrequency', t('gen-ai-answers-used-per-day', 'GenAI Answers Used (per day)')],
    ['genAiAnswersUsageRate', t('gen-ai-answers-used-percent', 'GenAI Answers Used (%)')],
    ['searchesPerFrequency', t('searches-per-day', 'Searches (per day)')],
    ['gwPerFrequency', t('guided-workflow-per-day', 'Guided Workflow (per day)')],
    ['suggestionsPerFrequency', t('suggestions-per-day', 'Suggestions (per day)')],
    ['smartComposePerFrequency', t('smart-compose-per-day', 'Smart Compose (per day)')],
    ['notesPerFrequency', t('notes-per-day', 'Notes (per day)')],
  ]);
}
