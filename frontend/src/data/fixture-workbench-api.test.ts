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

  it('returns an empty feedback view in fixture mode without claiming external observations', async () => {
    const api = new FixtureWorkbenchApi();
    const feedback = await api.getCompanyFeedback({companyId: FIXTURE_COMPANY_ID});

    expect(feedback).toEqual({companyId: FIXTURE_COMPANY_ID, sources: [], issues: []});
  });

  it('shows R3 evidence profiles as unqualified and keeps submissions out of fixture mode', async () => {
    const api = new FixtureWorkbenchApi();
    const ledger = await api.listDomainEvidence({companyId: FIXTURE_COMPANY_ID});

    expect(ledger.profiles.map(profile => profile.qualificationStatus)).toEqual(['not_run', 'not_run']);
    expect(ledger.profiles.every(profile => profile.executionEnabled === false)).toBe(true);
    await expect(api.recordDomainEvidence({
      companyId: FIXTURE_COMPANY_ID, profileId: ledger.profiles[0].id, profileRevision: ledger.profiles[0].revision, evidence: [], requestId: 'fixture-domain-evidence',
    })).rejects.toMatchObject({code: 'SIMULATED_MODE'});
    await expect(api.getDomainEvidenceArtifactPreview({
      companyId: FIXTURE_COMPANY_ID, recordId: 'domain-evidence-1', area: 'quality', inputId: 'input-1', inputRevision: 1, sourceDigest: 'a'.repeat(64), relativePath: 'quality.md', expectedContentSHA256: 'b'.repeat(64),
    })).rejects.toMatchObject({code: 'SIMULATED_MODE'});
    await expect(api.listDomainEvidenceArtifactPreviewEntries({
      companyId: FIXTURE_COMPANY_ID, recordId: 'domain-evidence-1', area: 'quality', inputId: 'input-1', inputRevision: 1, sourceDigest: 'a'.repeat(64),
    })).rejects.toMatchObject({code: 'SIMULATED_MODE'});
  });

  it('never stores credentials in fixture mode', async () => {
    const api = new FixtureWorkbenchApi();

    await expect(api.storeGitHubFeedbackCredential({companyId: FIXTURE_COMPANY_ID, token: 'github_pat_example', requestId: 'credential-store'})).rejects.toMatchObject({code: 'SIMULATED_MODE'});
    await expect(api.deleteGitHubFeedbackCredential({companyId: FIXTURE_COMPANY_ID, requestId: 'credential-delete'})).rejects.toMatchObject({code: 'SIMULATED_MODE'});
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
