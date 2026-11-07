// pattern: Imperative Shell

import {describe, expect, it} from 'vitest';
import {FixtureWorkbenchApi, FIXTURE_COMPANY_ID} from './fixture-workbench-api';

describe('FixtureWorkbenchApi', () => {
  it('returns a simulated overview with distinct employee and session identities', async () => {
    const api = new FixtureWorkbenchApi();
    const overview = await api.getCompanyOverview({companyId: FIXTURE_COMPANY_ID});

    expect(overview.meta.dataMode).toBe('simulated');
    const frontend = overview.employees.find(employee => employee.employeeId === 'emp-frontend');
    expect(frontend?.epoch).toBe('4');
    expect(frontend?.sessionId).not.toBe(frontend?.employeeId);
    expect(overview.obligations[0].state).toBe('fulfilled');
    expect(overview.artifacts[0].verdict).toBe('passed');
  });

  it('propagates the opaque next cursor without repeating the page boundary', async () => {
    const api = new FixtureWorkbenchApi();
    const snapshotCursor = 'fixture://r03a-handover-v2/company-seq-70';
    const firstPage = await api.listActivityEvents({companyId: FIXTURE_COMPANY_ID, cursor: null, limit: 3, snapshotCursor});
    const secondPage = await api.listActivityEvents({companyId: FIXTURE_COMPANY_ID, cursor: firstPage.nextCursor, limit: 3, snapshotCursor});

    expect(firstPage.items.map(item => item.companySeq)).toEqual(['70', '69', '68']);
    expect(firstPage.nextCursor).toBe('67');
    expect(secondPage.items.map(item => item.companySeq)).toEqual(['67', '66', '65']);
  });

  it('rejects an unknown company and an unrecognized cursor', async () => {
    const api = new FixtureWorkbenchApi();

    await expect(api.getCompanyOverview({companyId: 'unknown-company'})).rejects.toThrow('company scope');
    await expect(api.listActivityEvents({companyId: FIXTURE_COMPANY_ID, cursor: 'not-a-cursor', limit: 3, snapshotCursor: 'fixture://r03a-handover-v2/company-seq-70'})).rejects.toThrow('cursor');
  });

  it('requires the scoped snapshot watermark for activity subscriptions', () => {
    const api = new FixtureWorkbenchApi();

    expect(() => api.subscribeToActivityEvents({companyId: FIXTURE_COMPANY_ID, cursor: 'wrong-watermark'}, () => undefined)).toThrow('snapshot cursor');
  });
});
