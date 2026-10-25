import { MantineProvider } from '@mantine/core';
import type { RenderHookResult } from '@testing-library/react';
import { saveAs } from 'file-saver';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mock } from 'vitest-mock-extended';

import type {
  AgentStatsGroupResult,
  AgentStatsResponse,
  Attribute,
  ConversationAssistanceUsageStatsResponse,
  ConversationStatsGroupResult,
  ConversationStatsResponse,
  Group,
  User,
} from '@cresta/director-api';
import { DirectorEnvironment } from '@cresta/director-api';
import { AttributeHintType } from '@cresta/web-client';

import { renderHook } from '@/testing';
import { parseCsv } from '@/utils';

import type {
  ConversationStatsQueryResult,
  ManagedAgentStats,
  ManagedKnowledgeAssistStats,
} from '../../../../../hooks';
import type { ManagedHintStats } from '../../../../../types';

import { useAssistanceUsedByAgentLeaderboard } from './useAssistanceUsedByAgentLeaderboard';
import { useAssistanceUsedByTeamLeaderboard } from './useAssistanceUsedByTeamLeaderboard';

vi.mock('file-saver', () => ({ saveAs: vi.fn() }));

async function readCSV(blob: Blob): Promise<string[][]> {
  const text = await new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result !== 'string') {
        reject(new Error('Expected CSV text'));
        return;
      }
      resolve(reader.result);
    };
    reader.onerror = () => reject(reader.error);
    reader.readAsText(blob);
  });
  return parseCsv(text);
}

function setup(
  usage?: ConversationAssistanceUsageStatsResponse,
  baselineAttributes?: Attribute
): RenderHookResult<
  {
    agent: ReturnType<typeof useAssistanceUsedByAgentLeaderboard>;
    team: ReturnType<typeof useAssistanceUsedByTeamLeaderboard>;
  },
  unknown
> {
  const assistanceStats = { data: undefined, isLoading: false, isPending: false, error: null };
  const conversationStats = mock<ConversationStatsQueryResult>({
    data: baselineAttributes
      ? mock<ConversationStatsResponse>({
          resultGroups: [
            mock<ConversationStatsGroupResult>({
              groupedByAttributes: baselineAttributes,
              totalConversationCount: 10,
              averageConversationCountPerFrequencyPerUser: 2,
              averageHandleTime: 120,
            }),
          ],
        })
      : undefined,
    isPending: false,
    error: null,
  });
  const knowledgeAssistStats = mock<ManagedKnowledgeAssistStats>({ data: undefined, isLoading: false, error: null });
  const agentStats = mock<ManagedAgentStats>({
    data: baselineAttributes
      ? mock<AgentStatsResponse>({
          resultsGroupedByAttributeTypes: [
            mock<AgentStatsGroupResult>({
              groupedByAttributes: baselineAttributes,
              averageActiveDayCountPerUser: 5,
            }),
          ],
        })
      : undefined,
    isLoading: false,
    error: null,
  });
  const hints = {
    [AttributeHintType.HINT_TYPE_UNSPECIFIED]: mock<ManagedHintStats>({
      data: undefined,
      isLoading: false,
      error: null,
    }),
    [AttributeHintType.REGULAR_HINT]: mock<ManagedHintStats>({ data: undefined, isLoading: false, error: null }),
    [AttributeHintType.BEHAVIOR_HINT]: mock<ManagedHintStats>({ data: undefined, isLoading: false, error: null }),
    [AttributeHintType.CHECKLIST_HINT]: mock<ManagedHintStats>({ data: undefined, isLoading: false, error: null }),
    [AttributeHintType.KB_HINT]: mock<ManagedHintStats>({ data: undefined, isLoading: false, error: null }),
    [AttributeHintType.GW_HINT]: mock<ManagedHintStats>({ data: undefined, isLoading: false, error: null }),
  };
  return renderHook(
    () => ({
      agent: useAssistanceUsedByAgentLeaderboard(
        DirectorEnvironment.VOICE,
        assistanceStats,
        hints,
        false,
        knowledgeAssistStats,
        false,
        conversationStats,
        conversationStats,
        agentStats,
        usage
      ),
      team: useAssistanceUsedByTeamLeaderboard(
        DirectorEnvironment.VOICE,
        assistanceStats,
        hints,
        false,
        knowledgeAssistStats,
        conversationStats,
        conversationStats,
        agentStats,
        false,
        usage
      ),
    }),
    { wrapper: ({ children }) => <MantineProvider>{children}</MantineProvider> }
  );
}

describe('Assistance Used conversation usage rows', () => {
  afterEach(() => vi.clearAllMocks());

  it('includes an eligible agent with measured zero usage and an eligible team with positive usage', () => {
    const { result } = setup({
      resultGroups: [
        {
          groupedByAttributes: {
            user: mock<User>({
              name: 'customers/acme/users/agent-1',
              fullName: 'Agent One',
              username: 'agent1',
              isDevUser: false,
            }),
          },
          totalConversationCount: 5,
          conversationsWithAssistanceUsedCount: 0,
          conversationsWithAssistanceUsedPercent: 0,
        },
        {
          groupedByAttributes: {
            group: mock<Group>({ name: 'customers/acme/groups/team-1', displayName: 'Team One' }),
          },
          totalConversationCount: 8,
          conversationsWithAssistanceUsedCount: 3,
          conversationsWithAssistanceUsedPercent: 37.5,
        },
      ],
    });
    expect(result.current.agent.rows).toEqual([
      expect.objectContaining({
        resourceName: 'customers/acme/users/agent-1',
        name: 'Agent One',
        conversationsWithAssistanceUsedPercent: 0,
      }),
    ]);
    expect(result.current.team.rows).toEqual([
      expect.objectContaining({
        resourceName: 'customers/acme/groups/team-1',
        team: 'Team One',
        conversationsWithAssistanceUsedPercent: 37.5,
      }),
    ]);
  });

  it('exports numeric percentages for both tables and honors hidden columns', async () => {
    const { result } = setup({
      resultGroups: [
        {
          groupedByAttributes: {
            user: mock<User>({ name: 'customers/acme/users/agent-1', fullName: 'Agent One', isDevUser: false }),
            group: mock<Group>({ name: 'customers/acme/groups/team-1', displayName: 'Team One' }),
          },
          totalConversationCount: 8,
          conversationsWithAssistanceUsedCount: 3,
          conversationsWithAssistanceUsedPercent: 37.5,
        },
      ],
    });

    for (const table of [result.current.agent, result.current.team]) {
      table.exportCSV([]);
      const file = vi.mocked(saveAs).mock.lastCall?.[0];
      if (!(file instanceof Blob)) {
        throw new Error('Expected CSV file');
      }
      const [headers, row] = await readCSV(file);
      const index = headers?.indexOf('Conversations with assistance used (%)') ?? -1;
      expect(index).toBeGreaterThanOrEqual(0);
      expect(row?.[index]).toBe('37.5');

      table.exportCSV(['conversationsWithAssistanceUsedPercent']);
      const hiddenFile = vi.mocked(saveAs).mock.lastCall?.[0];
      if (!(hiddenFile instanceof Blob)) {
        throw new Error('Expected CSV file');
      }
      const [hiddenHeaders] = await readCSV(hiddenFile);
      expect(hiddenHeaders).not.toContain('Conversations with assistance used (%)');
    }
  });

  it('retains the existing conversation and active-day metrics for rows with no other assistance activity', () => {
    const groupedByAttributes = {
      user: mock<User>({ name: 'customers/acme/users/agent-1', fullName: 'Agent One', isDevUser: false }),
      group: mock<Group>({ name: 'customers/acme/groups/team-1', displayName: 'Team One' }),
    };
    const { result } = setup(
      {
        resultGroups: [
          {
            groupedByAttributes,
            totalConversationCount: 8,
            conversationsWithAssistanceUsedCount: 0,
            conversationsWithAssistanceUsedPercent: 0,
          },
        ],
      },
      groupedByAttributes
    );

    for (const table of [result.current.agent, result.current.team]) {
      expect(table.rows?.[0]).toMatchObject({
        avgConvosPerDay: 2,
        averageHandleTime: 120,
        activeDays: 5,
        convosPoweredByAARate: 1,
        conversationsWithAssistanceUsedPercent: 0,
      });
    }
  });
});
