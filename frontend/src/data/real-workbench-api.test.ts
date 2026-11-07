// pattern: Imperative Shell

import {afterEach, describe, expect, it, vi} from 'vitest';
import {FixtureWorkbenchApi, FIXTURE_COMPANY_ID} from './fixture-workbench-api';
import {RealWorkbenchApi} from './real-workbench-api';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('RealWorkbenchApi', () => {
  it('rejects an unbounded activity limit before making a request', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    await expect(api.listActivityEvents({companyId: 'company-1', cursor: null, limit: 101, snapshotCursor: 'snapshot-1'})).rejects.toThrow('between 1 and 100');
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('does not fall back to fixture data when the real API fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('control plane unavailable')));
    const api = new RealWorkbenchApi('/api/workbench');

    await expect(api.getCompanyOverview({companyId: 'company-1'})).rejects.toThrow('control plane unavailable');
  });

  it('rejects a malformed successful response instead of rendering unknown values', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ok: true, json: async () => ({meta: {companyId: 'company-1'}})}));
    const api = new RealWorkbenchApi('/api/workbench');

    await expect(api.getCompanyOverview({companyId: 'company-1'})).rejects.toThrow('overview');
  });

  it('preserves the authoritative checkpoint relation from the real overview DTO', async () => {
    const fixture = new FixtureWorkbenchApi();
    const expected = await fixture.getCompanyOverview({companyId: FIXTURE_COMPANY_ID});
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ok: true, json: async () => expected}));
    const api = new RealWorkbenchApi('/api/workbench');

    const actual = await api.getCompanyOverview({companyId: FIXTURE_COMPANY_ID});

    expect(actual.checkpoints).toHaveLength(1);
    expect(actual.checkpoints[0].checkpointId).toBe(actual.artifacts[0].checkpointId);
    expect(actual.checkpoints[0].taskId).toBe(actual.artifacts[0].taskId);
  });

  it('keeps live updates explicitly deferred unless the SSE capability is enabled', () => {
    const statuses: Array<string> = [];
    const api = new RealWorkbenchApi('/api/workbench');

    const unsubscribe = api.subscribeToActivityEvents({companyId: 'company-1', cursor: 'company-seq:1'}, () => undefined, status => statuses.push(status));
    unsubscribe();

    expect(statuses).toEqual(['closed']);
  });

  it('submits a mission goal through the real command endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        commandId: 'request-1',
        commandType: 'mission.create',
        targetType: 'mission',
        targetId: 'mission-1',
        requestId: 'request-1',
        accepted: true,
        acceptedAt: '2026-09-16T00:00:00Z',
        resultingState: 'draft',
      }),
    });
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    const acceptanceContract = {revision: 'text-acceptance@1' as const, required_text: ['Mission ID: {{mission_id}}']};
    const receipt = await api.createMission({companyId: 'company-1', title: 'goal', goal: 'body', acceptanceContract, requestId: 'request-1'});

    expect(receipt.targetId).toBe('mission-1');
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/missions', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({title: 'goal', goal: 'body', acceptanceContract, requestId: 'request-1'}),
    }));
  });

  it('preserves a structured conflict from a real command response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      json: async () => ({code: 'REVISION_CONFLICT', error: 'mission is already active', targetId: 'mission-1', currentState: 'active'}),
    }));
    const api = new RealWorkbenchApi('/api/workbench');

    await expect(api.startMission({companyId: 'company-1', missionId: 'mission-1', requestId: 'request-2'})).rejects.toMatchObject({code: 'REVISION_CONFLICT', status: 409, currentState: 'active'});
  });

  it('surfaces a command transport outage instead of treating it as accepted', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('command backend unavailable')));
    const api = new RealWorkbenchApi('/api/workbench');

    await expect(api.cancelMission({companyId: 'company-1', missionId: 'mission-1', requestId: 'request-3'})).rejects.toThrow('command backend unavailable');
  });
});
