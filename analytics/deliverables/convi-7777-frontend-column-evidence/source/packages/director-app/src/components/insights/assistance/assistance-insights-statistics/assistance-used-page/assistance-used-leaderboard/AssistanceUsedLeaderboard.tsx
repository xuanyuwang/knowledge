import type React from 'react';
import { memo, useCallback, useMemo, useState } from 'react';
import { Flex, Tabs } from '@mantine/core';

import type { ConversationAssistanceUsageStatsResponse, DirectorEnvironment } from '@cresta/director-api';
import { DirectorTable, DTPageSize, DTPaginationType } from '@cresta/director-components';
import type { AttributeHintType } from '@cresta/web-client';

import { DownloadButton } from '@/components/download-button';
import {
  getConfigurableColumns,
  TableColumnsConfigMenu,
  useTableColumnsConfig,
} from '@/components/table-columns-config';
import { useTranslation } from '@/hooks/useTranslation';

import type {
  AssistanceStatsBaseResult,
  ConversationStatsQueryResult,
  ManagedAgentStats,
  ManagedKnowledgeAssistStats,
} from '../../../../hooks';
import type { ManagedHintStats } from '../../../../types';

import { useAssistanceUsedByAgentLeaderboard, useAssistanceUsedByTeamLeaderboard } from './hooks';

import styles from './AssistanceUsedLeaderboard.module.css';

interface AssistanceUsedLeaderboardProps {
  appType: DirectorEnvironment;
  conversationAssistanceUsageStats: ConversationAssistanceUsageStatsResponse | undefined;
  conversationAssistanceUsageStatsForTeam: ConversationAssistanceUsageStatsResponse | undefined;
  assistanceStats: AssistanceStatsBaseResult;
  hintStatsByHintType: Record<AttributeHintType, ManagedHintStats>;
  allHintTypeLoading: boolean;
  hintStatsByHintTypeForTeam: Record<AttributeHintType, ManagedHintStats>;
  allHintTypeLoadingForTeam: boolean;
  knowledgeAssistStats: ManagedKnowledgeAssistStats;
  displayAsAgent?: boolean;
  conversationStats: ConversationStatsQueryResult;
  conversationStatsWithAA: ConversationStatsQueryResult;
  agentStats: ManagedAgentStats;
  assistanceStatsForTeam: AssistanceStatsBaseResult;
  knowledgeAssistStatsForTeam: ManagedKnowledgeAssistStats;
  conversationStatsForTeam: ConversationStatsQueryResult;
  conversationStatsWithAAForTeam: ConversationStatsQueryResult;
  agentStatsForTeam: ManagedAgentStats;
}

enum AssistanceUsedLeaderboardTabs {
  AGENT = 'AGENT',
  TEAM = 'TEAM',
}

export const AssistanceUsedLeaderboard = memo(function AssistanceUsedLeaderboard(
  props: AssistanceUsedLeaderboardProps
): React.ReactElement {
  const {
    appType,
    conversationAssistanceUsageStats,
    conversationAssistanceUsageStatsForTeam,
    assistanceStats,
    hintStatsByHintType,
    allHintTypeLoading,
    hintStatsByHintTypeForTeam,
    allHintTypeLoadingForTeam,
    knowledgeAssistStats,
    displayAsAgent = false,
    conversationStats,
    conversationStatsWithAA,
    agentStats,
    assistanceStatsForTeam,
    knowledgeAssistStatsForTeam,
    conversationStatsForTeam,
    conversationStatsWithAAForTeam,
    agentStatsForTeam,
  } = props;

  const {
    directorTableProps: agentLeaderboardTableProps,
    rows: agentLeaderboardRows,
    columns: agentLeaderboardColumns,
    exportCSV: agentLeaderboardExportCSV,
  } = useAssistanceUsedByAgentLeaderboard(
    appType,
    assistanceStats,
    hintStatsByHintType,
    allHintTypeLoading,
    knowledgeAssistStats,
    displayAsAgent,
    conversationStats,
    conversationStatsWithAA,
    agentStats,
    conversationAssistanceUsageStats
  );

  const {
    directorTableProps: teamLeaderboardTableProps,
    rows: teamLeaderboardRows,
    columns: teamLeaderboardColumns,
    exportCSV: teamLeaderboardExportCSV,
  } = useAssistanceUsedByTeamLeaderboard(
    appType,
    assistanceStatsForTeam,
    hintStatsByHintTypeForTeam,
    allHintTypeLoadingForTeam,
    knowledgeAssistStatsForTeam,
    conversationStatsForTeam,
    conversationStatsWithAAForTeam,
    agentStatsForTeam,
    displayAsAgent,
    conversationAssistanceUsageStatsForTeam
  );

  const { t } = useTranslation('director-app-insights', { keyPrefix: 'assistance.used-leaderboard' });
  const [activeTab, setActiveTab] = useState<string | null>(AssistanceUsedLeaderboardTabs.AGENT);

  const agentConfigurableColumns = useMemo(
    () => getConfigurableColumns(agentLeaderboardColumns),
    [agentLeaderboardColumns]
  );
  const teamConfigurableColumns = useMemo(
    () => getConfigurableColumns(teamLeaderboardColumns),
    [teamLeaderboardColumns]
  );

  const agentColumnsConfig = useTableColumnsConfig({ tableKey: 'assistance-used-leaderboard-agent' });
  const teamColumnsConfig = useTableColumnsConfig({ tableKey: 'assistance-used-leaderboard-team' });

  const isAgentTab = activeTab === AssistanceUsedLeaderboardTabs.AGENT;
  const columnsConfig = isAgentTab ? agentColumnsConfig : teamColumnsConfig;
  const configurableColumns = isAgentTab ? agentConfigurableColumns : teamConfigurableColumns;

  const handleClickCSVExport = useCallback(() => {
    if (isAgentTab) {
      agentLeaderboardExportCSV(agentColumnsConfig.hiddenColumnIds);
      return;
    }
    teamLeaderboardExportCSV(teamColumnsConfig.hiddenColumnIds);
  }, [
    isAgentTab,
    agentLeaderboardExportCSV,
    teamLeaderboardExportCSV,
    agentColumnsConfig.hiddenColumnIds,
    teamColumnsConfig.hiddenColumnIds,
  ]);

  return (
    <Flex direction="column" gap="3em" data-testid="assistance-used-leaderboard">
      <Flex justify="space-between">
        <Flex className={styles.title} wrap="wrap" align="center">
          <span>{t('title', 'Leaderboard')}</span>
          <span className={styles.title__dot}>•</span>
          <span>{t('subtitle', 'by overall assistance used')}</span>
          <Tabs onChange={setActiveTab} value={activeTab} variant="segmented" radius="md" ms="lg">
            <Tabs.List>
              <Tabs.Tab value={AssistanceUsedLeaderboardTabs.AGENT}>{t('tab.agent', 'Agent')}</Tabs.Tab>
              <Tabs.Tab value={AssistanceUsedLeaderboardTabs.TEAM}>{t('tab.team', 'Team')}</Tabs.Tab>
            </Tabs.List>
          </Tabs>
        </Flex>
        <Flex align="center" gap="sm">
          <TableColumnsConfigMenu
            columns={configurableColumns}
            hiddenColumnIds={columnsConfig.hiddenColumnIds}
            onToggleColumn={columnsConfig.toggleColumn}
            onResetColumns={columnsConfig.resetColumns}
            dataTestId="assistance-used-leaderboard-columns-menu"
          />
          <DownloadButton
            className={styles.downloadButton}
            onClick={handleClickCSVExport}
            tooltipMessage={t('download-tooltip', 'Download Assistance Used leaderboard')}
          />
        </Flex>
      </Flex>
      {isAgentTab ? (
        <DirectorTable
          {...agentLeaderboardTableProps}
          data={agentLeaderboardRows}
          columns={agentLeaderboardColumns}
          columnVisibility={agentColumnsConfig.columnVisibility}
          variant="muted"
          compact
          columnDivider="groups"
          paginationType={DTPaginationType.FRONT_END}
          initialPageSize={DTPageSize.TEN}
          aria-label={t('aria-label', 'Leaderboard - by overall assistance used')}
          dataTestId="assistance-used-leaderboard-agent"
        />
      ) : (
        <DirectorTable
          {...teamLeaderboardTableProps}
          data={teamLeaderboardRows}
          columns={teamLeaderboardColumns}
          columnVisibility={teamColumnsConfig.columnVisibility}
          variant="muted"
          compact
          columnDivider="groups"
          paginationType={DTPaginationType.FRONT_END}
          initialPageSize={DTPageSize.TEN}
          aria-label={t('aria-label', 'Leaderboard - by overall assistance used')}
          dataTestId="assistance-used-leaderboard-team"
        />
      )}
    </Flex>
  );
});
