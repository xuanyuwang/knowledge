import dayjs from 'dayjs';
import timezone from 'dayjs/plugin/timezone';
import utc from 'dayjs/plugin/utc';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AttributeType, Frequency, MetadataDayOfWeek } from '@cresta/web-client';

import { initClientVersionHeaders } from '../../shared';

import { InsightsApi } from './insightsApi';
import type { InsightsRequestParams } from './types';

dayjs.extend(utc);
dayjs.extend(timezone);

function setup(response: object): { api: InsightsApi; fetch: ReturnType<typeof vi.fn>; params: InsightsRequestParams } {
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(response)));
  vi.stubGlobal('fetch', fetch);
  const api = new InsightsApi();
  api.setCustomerId('acme');
  api.setProfileId('sales');
  api.setStreamingApiEnabled(false);
  api.setAuth('token');
  api.setTimezone('America/Toronto');
  return {
    api,
    fetch,
    params: {
      dateRange: [dayjs.tz('2026-10-01', 'America/Toronto'), dayjs.tz('2026-10-20', 'America/Toronto')],
      filterByAttribute: { users: [{ name: 'customers/acme/users/agent-1' }] },
      groupByAttributeTypes: [{ attributeType: AttributeType.ATTRIBUTE_TYPE_AGENT }],
      frequency: Frequency.FREQUENCY_UNSPECIFIED,
      metadata: { startDayOfWeek: MetadataDayOfWeek.MONDAY },
      includePeerUserStats: true,
      filterToAgentsOnly: true,
    },
  };
}

describe('InsightsApi.retrieveConversationAssistanceUsageStats', () => {
  beforeEach(() => {
    process.env.REACT_APP_TESTING_DOMAIN = 'director-test.cresta.ai';
    initClientVersionHeaders({ clientName: 'director-app', clientVersion: 'main_test' });
  });

  afterEach(() => {
    delete process.env.REACT_APP_TESTING_DOMAIN;
    vi.unstubAllGlobals();
  });

  it.each([AttributeType.ATTRIBUTE_TYPE_AGENT, AttributeType.ATTRIBUTE_TYPE_GROUP])(
    'sends the dedicated API the selected cohort, timezone, role filters, and single grouping %s',
    async (attributeType) => {
      const { api, fetch, params } = setup({});
      await api.retrieveConversationAssistanceUsageStats({
        ...params,
        groupByAttributeTypes: [{ attributeType }],
      }).promise;

      const [url, init] = fetch.mock.calls[0] as [string, RequestInit];
      expect(url).toBe(
        'https://api-director-test.cresta.ai/v1/customers/acme/profiles/sales/conversationAssistanceUsageStats:retrieve'
      );
      expect(init.method).toBe('POST');
      expect(init.headers).toMatchObject({ Authorization: 'Bearer token' });
      if (typeof init.body !== 'string') {
        throw new Error('Expected a JSON request body');
      }
      expect(JSON.parse(init.body)).toEqual({
        parent: 'customers/acme/profiles/sales',
        filterByAttribute: params.filterByAttribute,
        filterByTimeRange: {
          startTimestamp: '2026-10-01T04:00:00.000Z',
          endTimestamp: '2026-10-20T04:00:00.000Z',
        },
        groupByAttributeTypes: [attributeType],
        frequency: Frequency.FREQUENCY_UNSPECIFIED,
        metadata: { timeZoneId: 'America/Toronto', startDayOfWeek: MetadataDayOfWeek.MONDAY },
        includePeerUserStats: true,
        filterToAgentsOnly: true,
      });
    }
  );

  it('uses each dedicated result denominator and resolves agent/team row identity', async () => {
    const { api, params } = setup({
      totalConversationCount: 100,
      conversationsWithAssistanceUsedCount: 90,
      conversationAssistanceUsageStatsResults: [
        {
          attribute: { users: [{ name: 'customers/acme/users/agent-1', fullName: 'Agent One' }] },
          totalConversationCount: 4,
          conversationsWithAssistanceUsedCount: 1,
        },
        {
          attribute: { groups: [{ name: 'customers/acme/groups/team-1', displayName: 'Team One' }] },
          totalConversationCount: 10,
          conversationsWithAssistanceUsedCount: 7,
        },
      ],
    });
    const result = await api.retrieveConversationAssistanceUsageStats(params).promise;
    expect(result?.resultGroups[0]).toMatchObject({
      groupedByAttributes: { user: { name: 'customers/acme/users/agent-1' } },
      totalConversationCount: 4,
      conversationsWithAssistanceUsedCount: 1,
      conversationsWithAssistanceUsedPercent: 25,
    });
    expect(result?.resultGroups[1]).toMatchObject({
      groupedByAttributes: { group: { name: 'customers/acme/groups/team-1' } },
      conversationsWithAssistanceUsedPercent: 70,
    });
  });

  it('preserves a computed zero when protobuf JSON omits the zero numerator', async () => {
    const { api, params } = setup({ conversationAssistanceUsageStatsResults: [{ totalConversationCount: 5 }] });
    const result = await api.retrieveConversationAssistanceUsageStats(params).promise;
    expect(result?.resultGroups[0]?.conversationsWithAssistanceUsedPercent).toBe(0);
  });

  it('leaves a zero-denominator percentage unavailable', async () => {
    const { api, params } = setup({ conversationAssistanceUsageStatsResults: [{}] });
    const result = await api.retrieveConversationAssistanceUsageStats(params).promise;
    expect(result?.resultGroups[0]?.conversationsWithAssistanceUsedPercent).toBeUndefined();
  });

  it('does not create measured zero rows from a population-only response', async () => {
    const { api, params } = setup({ totalConversationCount: 100 });
    const result = await api.retrieveConversationAssistanceUsageStats(params).promise;
    expect(result?.resultGroups).toEqual([]);
  });

  it('surfaces an unavailable endpoint as an error rather than a zero result', async () => {
    const { api, fetch, params } = setup({});
    fetch.mockResolvedValue(new Response(JSON.stringify({ code: 12, message: 'Unimplemented' }), { status: 501 }));
    await expect(api.retrieveConversationAssistanceUsageStats(params).promise).rejects.toThrow();
  });
});
