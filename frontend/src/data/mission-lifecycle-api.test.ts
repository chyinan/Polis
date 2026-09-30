// pattern: Imperative Shell

import {afterEach, describe, expect, it, vi} from 'vitest';
import {RealWorkbenchApi} from './real-workbench-api';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('RealWorkbenchApi formal Mission lifecycle commands', () => {
  it('sends explicit pause and resume commands with distinct idempotency keys', async () => {
    const pauseReceipt = {commandId: 'pause-1', commandType: 'mission.pause', targetType: 'mission', targetId: 'mission-1', requestId: 'pause-1', accepted: true, acceptedAt: '2026-09-23T00:00:00Z', resultingState: 'paused'};
    const resumeReceipt = {commandId: 'resume-1', commandType: 'mission.resume', targetType: 'mission', targetId: 'mission-1', requestId: 'resume-1', accepted: true, acceptedAt: '2026-09-23T00:01:00Z', resultingState: 'active'};
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ok: true, json: async () => pauseReceipt})
      .mockResolvedValueOnce({ok: true, json: async () => resumeReceipt});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    expect(await api.pauseMission({companyId: 'company-1', missionId: 'mission-1', requestId: 'pause-1'})).toEqual(pauseReceipt);
    expect(await api.resumeMission({companyId: 'company-1', missionId: 'mission-1', requestId: 'resume-1'})).toEqual(resumeReceipt);

    const calls = fetchMock.mock.calls as Array<[string, RequestInit]>;
    expect(calls[0]?.[0]).toBe('/api/workbench/companies/company-1/missions/mission-1/pause');
    expect(JSON.parse(String(calls[0]?.[1].body))).toEqual({requestId: 'pause-1'});
    expect(calls[1]?.[0]).toBe('/api/workbench/companies/company-1/missions/mission-1/resume');
    expect(JSON.parse(String(calls[1]?.[1].body))).toEqual({requestId: 'resume-1'});
  });

  it('sends human intervention acknowledgement and resolution as scoped commands', async () => {
    const acknowledged = {commandId: 'ack-1', commandType: 'human_intervention.acknowledged', targetType: 'human_intervention', targetId: 'intervention-1', requestId: 'ack-1', accepted: true, acceptedAt: '2026-09-23T00:00:00Z', resultingState: 'acknowledged'};
    const resolved = {commandId: 'resolve-1', commandType: 'human_intervention.resolved', targetType: 'human_intervention', targetId: 'intervention-1', requestId: 'resolve-1', accepted: true, acceptedAt: '2026-09-23T00:01:00Z', resultingState: 'resolved'};
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ok: true, json: async () => acknowledged})
      .mockResolvedValueOnce({ok: true, json: async () => resolved});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    expect(await api.setHumanInterventionState({companyId: 'company-1', interventionId: 'intervention-1', state: 'acknowledged', requestId: 'ack-1'})).toEqual(acknowledged);
    expect(await api.setHumanInterventionState({companyId: 'company-1', interventionId: 'intervention-1', state: 'resolved', requestId: 'resolve-1'})).toEqual(resolved);

    const calls = fetchMock.mock.calls as Array<[string, RequestInit]>;
    expect(calls[0]?.[0]).toBe('/api/workbench/companies/company-1/human-interventions/intervention-1/acknowledge');
    expect(JSON.parse(String(calls[0]?.[1].body))).toEqual({requestId: 'ack-1'});
    expect(calls[1]?.[0]).toBe('/api/workbench/companies/company-1/human-interventions/intervention-1/resolve');
    expect(JSON.parse(String(calls[1]?.[1].body))).toEqual({requestId: 'resolve-1'});
  });
});
