// pattern: Imperative Shell

import {afterEach, describe, expect, it, vi} from 'vitest';
import {RealWorkbenchApi} from './real-workbench-api';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('RealWorkbenchApi QQ notification configuration', () => {
  it('sends only the C2C target and non-secret references when saving a disabled draft', async () => {
    const receipt = {commandId: 'route-1', commandType: 'notification.route.update', targetType: 'company', targetId: 'company-1', requestId: 'route-1', accepted: true, acceptedAt: '2026-09-23T00:00:00Z', resultingState: 'configured'};
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => receipt});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    await api.configureNotificationRoute({companyId: 'company-1', adapter: 'qq_official', destination: 'admin-openid', safetyAlias: 'Team A', credentialRef: 'default', enabled: false, requestId: 'route-1'});

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/workbench/companies/company-1/notifications/route');
    const payload = JSON.parse(String(init.body)) as Record<string, unknown>;
    expect(payload).toEqual({adapter: 'qq_official', destination: 'admin-openid', safetyAlias: 'Team A', credentialRef: 'default', enabled: false, requestId: 'route-1'});
    expect(payload).not.toHaveProperty('clientSecret');
    expect(payload).not.toHaveProperty('accessToken');
  });

  it('reads masked target, route revision and explicit qualification state', async () => {
    const response = {
      routes: [{routeId: 'default-qq_official', adapter: 'qq_official', enabled: false, destination: 'admin C2C ••••1234', safetyAlias: 'Team A', credentialRef: 'default', status: 'configured', routeRevision: '1', qualificationStatus: 'unverified', qualifiedUntil: ''}],
      deliveries: [],
    };
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => response});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    const actual = await api.listNotifications({companyId: 'company-1'});

    expect(actual.routes[0]?.adapter).toBe('qq_official');
    expect(actual.routes[0]?.destination).toContain('••••1234');
    expect(actual.routes[0]?.qualificationStatus).toBe('unverified');
    expect(actual.routes[0]?.enabled).toBe(false);
  });
});
