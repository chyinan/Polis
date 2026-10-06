// pattern: Imperative Shell

import {afterEach, describe, expect, it, vi} from 'vitest';
import {FixtureWorkbenchApi, FIXTURE_COMPANY_ID} from './fixture-workbench-api';
import {RealWorkbenchApi} from './real-workbench-api';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('RealWorkbenchApi', () => {
  it('lists, creates, and repairs mission-scoped daily Routines through authenticated routes', async () => {
    const routine = {
      routineId: 'routine-1', missionId: 'mission-1', employeeId: 'emp-backend', timezone: 'Asia/Shanghai', localTime: '09:00',
      nextLogicalDay: '2026-09-30', catchUpPolicy: 'coalesce_latest', maxCatchUp: 1, nextDueAt: null,
      taskInstruction: 'Review the approved queue.', needsInstructionOccurrences: 0, linkedTaskCount: 0,
    };
    const receipt = {
      commandId: 'command-1', commandType: 'routine.daily.create', targetType: 'routine', targetId: 'routine-1',
      requestId: 'routine-request-1', accepted: true, acceptedAt: '2026-09-30T00:00:00Z', resultingState: 'active',
    };
    const repairReceipt = {...receipt, commandType: 'routine.daily.instruction', requestId: 'routine-repair-1', resultingState: 'instruction_set'};
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ok: true, json: async () => [routine]})
      .mockResolvedValueOnce({ok: true, json: async () => receipt})
      .mockResolvedValueOnce({ok: true, json: async () => repairReceipt});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench', false, 'routine-token');

    await expect(api.listDailyRoutines({companyId: 'company-1', missionId: 'mission-1'})).resolves.toEqual([routine]);
    await expect(api.createDailyRoutine({
      companyId: 'company-1', missionId: 'mission-1', routineId: 'routine-1', employeeId: 'emp-backend',
      taskInstruction: 'Review the approved queue.', timezone: 'Asia/Shanghai', localTime: '09:00',
      nextLogicalDay: '2026-09-30', catchUpPolicy: 'coalesce_latest', maxCatchUp: 1, requestId: 'routine-request-1',
    })).resolves.toEqual(receipt);
    await expect(api.setDailyRoutineTaskInstruction({
      companyId: 'company-1', missionId: 'mission-1', routineId: 'routine-1',
      taskInstruction: 'Review the legacy queue.', requestId: 'routine-repair-1',
    })).resolves.toEqual(repairReceipt);

    expect(fetchMock.mock.calls.map(call => call[0])).toEqual([
      '/api/workbench/companies/company-1/missions/mission-1/routines',
      '/api/workbench/companies/company-1/missions/mission-1/routines',
      '/api/workbench/companies/company-1/missions/mission-1/routines/routine-1/instruction',
    ]);
    expect(JSON.parse(String((fetchMock.mock.calls[1]?.[1] as RequestInit).body))).toMatchObject({
      routineId: 'routine-1', employeeId: 'emp-backend', taskInstruction: 'Review the approved queue.', requestId: 'routine-request-1',
    });
  });

  it('sends runtime qualification approval through the authenticated company command route', async () => {
    const receipt = {id: 'runtime-qualification-1', status: 'qualified'};
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => receipt});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench', false, 'runtime-approval-token');

    await expect(api.approveStdioMCPRuntimeQualification({
      companyId: 'company-1', runtimeQualificationId: 'runtime-qualification-1',
      rationale: 'Reviewed the pinned process and schema evidence.', requestId: 'runtime-approval-1',
    })).resolves.toEqual(receipt);
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/capabilities/runtime-approve', expect.objectContaining({
      credentials: 'same-origin', headers: expect.objectContaining({'X-Polis-Desktop-Token': 'runtime-approval-token'}),
      body: JSON.stringify({runtimeQualificationId: 'runtime-qualification-1', rationale: 'Reviewed the pinned process and schema evidence.', requestId: 'runtime-approval-1'}),
    }));
  });

  it('posts stdio MCP runtime observation with pinned revision and metadata qualification', async () => {
    const response = {companyId: 'company-1', runtimeQualificationId: 'runtime-qualification-1', capabilityId: 'mcp-1', status: 'observed_unqualified'};
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => response});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench', false, 'runtime-observation-token');

    await expect(api.observeStdioMCPRuntime({
      companyId: 'company-1', serverId: 'mcp-1', packageRevisionId: 'package-revision-1',
      capabilityQualificationId: 'qualification-1', requestId: 'runtime-observe-1',
    })).resolves.toEqual(response);

    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/capabilities/runtime-observe', expect.objectContaining({
      method: 'POST', credentials: 'same-origin',
      headers: expect.objectContaining({'X-Request-ID': 'runtime-observe-1', 'X-Polis-Desktop-Token': 'runtime-observation-token'}),
      body: JSON.stringify({serverId: 'mcp-1', packageRevisionId: 'package-revision-1', capabilityQualificationId: 'qualification-1', requestId: 'runtime-observe-1'}),
    }));
  });

  it('posts Streamable HTTP tools/list observation only through the authenticated runtime route', async () => {
    const response = {companyId: 'company-1', runtimeQualificationId: 'http-runtime-1', capabilityId: 'mcp-http-1', transport: 'streamable_http', status: 'observed_unqualified'};
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => response});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench', false, 'http-runtime-token');

    await expect(api.observeStreamableHTTPMCPRuntime({
      companyId: 'company-1', capabilityId: 'mcp-http-1', capabilityQualificationId: 'http-qualification-1', requestId: 'http-runtime-observe-1',
    })).resolves.toEqual(response);
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/capabilities/runtime-observe-http', expect.objectContaining({
      method: 'POST', credentials: 'same-origin',
      headers: expect.objectContaining({'X-Request-ID': 'http-runtime-observe-1', 'X-Polis-Desktop-Token': 'http-runtime-token'}),
      body: JSON.stringify({capabilityId: 'mcp-http-1', capabilityQualificationId: 'http-qualification-1', requestId: 'http-runtime-observe-1'}),
    }));
  });

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

  it('lists, creates, and applies a formal Mission change using the exact reviewed impact digest', async () => {
    const impact = {
      schemaVersion: 'polis-mission-change-impact@1', missionId: 'mission-1', baseRequirementsSha256: 'a'.repeat(64),
      inputRevisions: [], tasks: [], artifacts: [], activeWorkerSessions: [], nonterminalJobRuns: [], activeServiceEndpoints: [], activeTaskTakeoverLeases: [], returnedHumanTakeoverSnapshots: [],
      naturalLanguageImpactStatus: 'not_assessed',
    };
    const request = {
      changeRequestId: 'change-1', missionId: 'mission-1', clientRequestId: 'change-create-1', baseRequirementsSha256: 'a'.repeat(64),
      changeSummary: 'Define empty-result behavior.', proposedTitle: 'API revision', proposedGoal: 'Return an empty result list.',
      proposedAcceptanceContract: {revision: 'text-acceptance@1' as const, required_text: ['Task summary:', 'Empty results:']},
      blockPreviousResults: true, state: 'queued', impactRevision: 1, impactSha256: 'b'.repeat(64), impact,
      successorMissionId: null, inputRevisionMap: [], createdAt: '2026-09-26T00:00:00Z',
      events: [{eventId: 'event-1', state: 'queued', impactRevision: 1, successorMissionId: null, reasonCode: 'awaiting_safe_boundary', createdAt: '2026-09-26T00:00:00Z', inputRevisionMap: []}],
    };
    const applied = {
      ...request, state: 'applied', successorMissionId: 'mission-successor',
      events: [{eventId: 'event-2', state: 'applied', impactRevision: 1, successorMissionId: 'mission-successor', reasonCode: 'successor_mission_created', createdAt: '2026-09-26T00:01:00Z', inputRevisionMap: []}],
    };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ok: true, json: async () => [request]})
      .mockResolvedValueOnce({ok: true, json: async () => request})
      .mockResolvedValueOnce({ok: true, json: async () => applied});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    expect(await api.listMissionChangeRequests({companyId: 'company-1', missionId: 'mission-1'})).toHaveLength(1);
    await api.createMissionChangeRequest({
      companyId: 'company-1', missionId: 'mission-1', requestId: 'change-create-1', changeSummary: request.changeSummary,
      proposedTitle: request.proposedTitle, proposedGoal: request.proposedGoal, proposedAcceptanceContract: request.proposedAcceptanceContract,
      blockPreviousResults: true,
    });
    const result = await api.applyMissionChangeRequest({
      companyId: 'company-1', missionId: 'mission-1', changeRequestId: 'change-1', requestId: 'change-apply-1', impactSha256: 'b'.repeat(64),
    });

    expect(result.successorMissionId).toBe('mission-successor');
    expect(fetchMock.mock.calls.map(call => call[0])).toEqual([
      '/api/workbench/companies/company-1/missions/mission-1/change-requests',
      '/api/workbench/companies/company-1/missions/mission-1/change-requests',
      '/api/workbench/companies/company-1/missions/mission-1/change-requests/change-1/apply',
    ]);
    const applyRequest = fetchMock.mock.calls[2]?.[1] as RequestInit;
    expect(JSON.parse(String(applyRequest.body))).toEqual({requestId: 'change-apply-1', impactSha256: 'b'.repeat(64)});
  });

  it('lists and returns Task takeover leases with the frozen workspace version', async () => {
    const granted = {
      leaseId: 'lease-1', missionId: 'mission-1', taskId: 'task-1', clientRequestId: 'takeover-grant-1',
      baseRequirementsSha256: 'a'.repeat(64), baseWorkspaceDigest: 'b'.repeat(64), baseWorkspaceRevision: 7,
      state: 'granted', snapshotInputId: null, snapshotRevision: null, snapshotDigest: null, snapshotBytes: null,
      humanEffortSeconds: null, diffSummary: null, createdAt: '2026-09-26T00:00:00Z',
      events: [{eventId: 'event-1', state: 'granted', snapshotInputId: null, snapshotRevision: null, snapshotDigest: null, snapshotBytes: null, humanEffortSeconds: null, diffSummary: null, reasonCode: 'worker_tree_stopped', createdAt: '2026-09-26T00:00:00Z'}],
    };
    const returned = {
      ...granted, state: 'returned', snapshotInputId: 'input-1', snapshotRevision: 1, snapshotDigest: 'c'.repeat(64), snapshotBytes: 19, humanEffortSeconds: 30,
      diffSummary: {model: 'task-workspace-diff@1', baseWorkspaceDigest: 'b'.repeat(64), baseWorkspaceRevision: 7, submittedContentDigest: 'c'.repeat(64), baseBytes: 16, submittedBytes: 19, removedLines: 1, addedLines: 1, changed: true},
      events: [granted.events[0], {eventId: 'event-2', state: 'returned', snapshotInputId: 'input-1', snapshotRevision: 1, snapshotDigest: 'c'.repeat(64), snapshotBytes: 19, humanEffortSeconds: 30, diffSummary: {model: 'task-workspace-diff@1', baseWorkspaceDigest: 'b'.repeat(64), baseWorkspaceRevision: 7, submittedContentDigest: 'c'.repeat(64), baseBytes: 16, submittedBytes: 19, removedLines: 1, addedLines: 1, changed: true}, reasonCode: 'snapshot_base_verified', createdAt: '2026-09-26T00:01:00Z'}],
    };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ok: true, json: async () => [granted]})
      .mockResolvedValueOnce({ok: true, json: async () => granted})
      .mockResolvedValueOnce({ok: true, json: async () => returned})
      .mockResolvedValueOnce({ok: true, json: async () => ({...granted, state: 'released', events: [granted.events[0], {...granted.events[0], eventId: 'event-3', state: 'released', reasonCode: 'operator_returned_no_snapshot'}]})});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench', false, 'takeover-token');

    expect(await api.listTaskTakeoverLeases({companyId: 'company-1', missionId: 'mission-1'})).toHaveLength(1);
    await api.createTaskTakeoverLease({companyId: 'company-1', missionId: 'mission-1', taskId: 'task-1', requestId: 'takeover-grant-1'});
    expect((await api.submitTaskTakeoverSnapshot({companyId: 'company-1', missionId: 'mission-1', leaseId: 'lease-1', requestId: 'takeover-return-1', baseWorkspaceDigest: 'b'.repeat(64), baseWorkspaceRevision: 7, content: '# handover', humanEffortSeconds: 30})).state).toBe('returned');
    expect((await api.releaseTaskTakeoverLease({companyId: 'company-1', missionId: 'mission-1', leaseId: 'lease-1', requestId: 'takeover-release-1'})).state).toBe('released');
    const [snapshotUrl, snapshotRequest] = fetchMock.mock.calls[2] as [string, RequestInit];
    expect(snapshotUrl).toBe('/api/workbench/companies/company-1/missions/mission-1/takeover-leases/lease-1/snapshot');
    expect(snapshotRequest.headers).toMatchObject({'X-Request-ID': 'takeover-return-1', 'X-Polis-Desktop-Token': 'takeover-token'});
    expect(JSON.parse(String(snapshotRequest.body))).toMatchObject({baseWorkspaceDigest: 'b'.repeat(64), baseWorkspaceRevision: 7, content: '# handover'});
  });

  it('imports a read-only Skill ZIP with a same-origin session token and server-computed digest', async () => {
    const response = {companyId: 'company-1', id: 'skill-1', packageId: 'design-review', revision: '1.0.0', status: 'candidate'};
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => response});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench', false, 'skill-session-token');
    const bundleFile = new File(['zip bytes'], 'design-review.zip', {type: 'application/zip'});

    expect(await api.importReadOnlySkillPackage({companyId: 'company-1', revision: '1.0.0', bundleFile, requestId: 'skill-import-1'})).toEqual(response);

    const [url, request] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/workbench/companies/company-1/capabilities/skills');
    expect(request.method).toBe('POST');
    expect(request.credentials).toBe('same-origin');
    expect(request.headers).toMatchObject({'X-Request-ID': 'skill-import-1', 'X-Polis-Desktop-Token': 'skill-session-token'});
    expect((request.headers as Record<string, string>)['Content-Type']).toBeUndefined();
    const form = request.body as FormData;
    expect(form.get('revision')).toBe('1.0.0');
    expect(form.get('requestId')).toBe('skill-import-1');
    const uploadedBundle = form.get('bundle');
    expect(uploadedBundle).toBeInstanceOf(File);
    expect((uploadedBundle as File).name).toBe(bundleFile.name);
    await expect((uploadedBundle as File).text()).resolves.toBe('zip bytes');
  });

  it('imports a controlled stdio MCP package with server-side descriptor and package fingerprints', async () => {
    const response = {
      companyId: 'company-1', id: 'package-1', serverId: 'mcp-1', revision: '1.0.0', manifestDigest: 'a'.repeat(64), createdAt: '2026-09-28T00:00:00Z',
      manifest: {
        schemaVersion: 'polis-controlled-stdio-mcp@1', name: 'fixture-mcp', serverName: 'fixture-server', serverVersion: '1.0.0',
        command: 'runtime/node.exe', entryPoint: 'server/main.mjs', args: ['server/main.mjs'],
        files: [{relativePath: 'runtime/node.exe', mediaType: 'application/octet-stream', byteSize: 12, contentSHA256: 'b'.repeat(64)}, {relativePath: 'server/main.mjs', mediaType: 'text/javascript', byteSize: 16, contentSHA256: 'c'.repeat(64)}],
      },
    };
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => response});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench', false, 'mcp-package-session-token');
    const bundleFile = new File(['package zip'], 'fixture-mcp.zip', {type: 'application/zip'});

    expect(await api.importStdioMCPPackage({companyId: 'company-1', serverId: null, revision: '1.0.0', bundleFile, requestId: 'mcp-package-import-1'})).toEqual(response);

    const [url, request] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/workbench/companies/company-1/capabilities/mcp-packages');
    expect(request.method).toBe('POST');
    expect(request.credentials).toBe('same-origin');
    expect(request.headers).toMatchObject({'X-Request-ID': 'mcp-package-import-1', 'X-Polis-Desktop-Token': 'mcp-package-session-token'});
    expect((request.headers as Record<string, string>)['Content-Type']).toBeUndefined();
    const form = request.body as FormData;
    expect(form.get('serverId')).toBeNull();
    expect(form.get('revision')).toBe('1.0.0');
    expect(form.get('requestId')).toBe('mcp-package-import-1');
    const uploadedBundle = form.get('bundle');
    expect(uploadedBundle).toBeInstanceOf(File);
    expect((uploadedBundle as File).name).toBe(bundleFile.name);
    await expect((uploadedBundle as File).text()).resolves.toBe('package zip');
  });

  it('rejects oversized controlled MCP packages before sending them', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');
    const oversized = new File([new Uint8Array((8 << 20) + 1)], 'large-mcp.zip', {type: 'application/zip'});

    await expect(api.importStdioMCPPackage({companyId: 'company-1', serverId: null, revision: '1', bundleFile: oversized, requestId: 'mcp-package-import-large'})).rejects.toThrow('8 MiB');
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('pins an imported stdio MCP package revision to the selected server definition', async () => {
    const response = {
      companyId: 'company-1', id: 'package-2', serverId: 'mcp-1', revision: '2.0.0', manifestDigest: 'a'.repeat(64), createdAt: '2026-09-28T00:00:00Z',
      manifest: {
        schemaVersion: 'polis-controlled-stdio-mcp@1', name: 'fixture-mcp', serverName: 'fixture-server', serverVersion: '1.0.0',
        command: 'runtime/node.exe', entryPoint: 'server/main.mjs', args: ['server/main.mjs'],
        files: [{relativePath: 'runtime/node.exe', mediaType: 'application/octet-stream', byteSize: 12, contentSHA256: 'b'.repeat(64)}, {relativePath: 'server/main.mjs', mediaType: 'text/javascript', byteSize: 16, contentSHA256: 'c'.repeat(64)}],
      },
    };
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => response});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');
    const bundleFile = new File(['package zip'], 'fixture-mcp-v2.zip', {type: 'application/zip'});

    await api.importStdioMCPPackage({companyId: 'company-1', serverId: 'mcp-1', revision: '2.0.0', bundleFile, requestId: 'mcp-package-import-2'});

    const [, request] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect((request.body as FormData).get('serverId')).toBe('mcp-1');
    expect((request.body as FormData).get('revision')).toBe('2.0.0');
  });

  it('rejects an oversized Skill source before sending it', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');
    const oversized = new File([new Uint8Array((8 << 20) + 1)], 'large.zip', {type: 'application/zip'});

    await expect(api.importReadOnlySkillPackage({companyId: 'company-1', revision: '1', bundleFile: oversized, requestId: 'skill-import-large'})).rejects.toThrow('8 MiB');
    expect(fetchMock).not.toHaveBeenCalled();
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

  it('reads the company domain evidence ledger and records only the requested profile revision', async () => {
    const profile = {
      id: 'content-operations-reference', revision: 'content-operations@1', domain: 'content_operations',
      qualificationStatus: 'not_run', executionEnabled: false,
      stages: ['authorized_sources'],
      requiredEvidence: ['quality', 'intervention', 'recovery', 'cost', 'organization_benefit'],
    };
    const ledger = {companyId: 'company-1', profiles: [profile], submissions: [], qualifications: [], researchSimulationRuns: [], contentSourceEvents: [], contentDrafts: [], contentReviews: [], contentPublications: [], contentCorrections: [], contentFeedback: []};
    const record = {
      companyId: 'company-1', recordId: 'domain-evidence-1', profileId: profile.id, profileRevision: profile.revision,
      readinessStatus: 'incomplete', qualificationStatus: 'not_run', executionEnabled: false,
      evidenceDigest: 'a'.repeat(64), submission: {profileId: profile.id, profileRevision: profile.revision, evidence: []},
      review: null, assessment: null,
      reasonCodes: ['domain_evidence_area_missing'], requestId: 'domain-evidence-request-1', createdAt: '2026-09-25T00:00:00Z',
    };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ok: true, json: async () => ledger})
      .mockResolvedValueOnce({ok: true, json: async () => record});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    expect(await api.listDomainEvidence({companyId: 'company-1'})).toEqual(ledger);
    await api.recordDomainEvidence({
      companyId: 'company-1', profileId: profile.id, profileRevision: profile.revision, evidence: [], requestId: 'domain-evidence-request-1',
    });

    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/workbench/companies/company-1/domain-workflows', expect.objectContaining({credentials: 'same-origin'}));
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/workbench/companies/company-1/domain-evidence', expect.objectContaining({
      method: 'POST', body: JSON.stringify({profileId: profile.id, profileRevision: profile.revision, evidence: [], requestId: 'domain-evidence-request-1'}),
    }));
  });

  it('rejects a research simulation receipt whose risk budget differs from the requested run', async () => {
    const output = {
      schemaVersion: 'polis-research-simulation-output@1', algorithm: 'bootstrap-mean-difference@1', protocolRevision: 1,
      datasetSha256: 'a'.repeat(64), methodSha256: 'b'.repeat(64), seed: '42', controlDefinition: 'control cohort',
      iterations: 16, sampleSize: 4, riskConsumedUnits: 128, riskUnit: 'sample_draw',
      controlMean: 2.5, treatmentMean: 3.5, meanDifference: 1, minimumDifference: 0, maximumDifference: 2,
    };
    const receipt = {
      companyId: 'company-1', runId: 'research-run-1', profileId: 'research-simulation-reference', profileRevision: 'research-simulation@1',
      protocolRevision: '1', datasetInputId: 'dataset-1', datasetInputRevision: '1', datasetSha256: 'a'.repeat(64),
      methodInputId: 'method-1', methodInputRevision: '1', methodSha256: 'b'.repeat(64), seed: '42', controlDefinition: 'control cohort',
      riskUnit: 'sample_draw', riskBudgetUnits: '129', riskConsumedUnits: '128', outputSha256: 'c'.repeat(64), output,
      requestId: 'research-request-1', createdAt: '2026-09-29T00:00:00Z',
    };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ok: true, json: async () => receipt}));
    const api = new RealWorkbenchApi('/api/workbench');

    await expect(api.runResearchSimulation({
      companyId: 'company-1', datasetInputId: 'dataset-1', datasetInputRevision: '1', methodInputId: 'method-1', methodInputRevision: '1',
      seed: '42', controlDefinition: 'control cohort', riskBudgetUnits: 128, requestId: 'research-request-1',
    })).rejects.toThrow('research simulation receipt');
  });

  it('records company-scoped content source, draft, and independent review commands', async () => {
    const sourceEvent = {
      companyId: 'company-1', eventSeq: '1', eventId: 'content-source-event-1', inputId: 'source-1', revision: '2',
      sha256: 'a'.repeat(64), state: 'authorized', rationale: 'approved source', requestId: 'content-source-request', createdAt: '2026-09-29T00:00:00Z',
    };
    const draft = {
      companyId: 'company-1', draftId: 'content-draft-1', draftInputId: 'draft-1', draftRevision: '3', draftSha256: 'b'.repeat(64),
      writerEmployeeId: 'emp-backend', criticalClaims: ['claim-1'], constraintsPassed: true, requestId: 'content-draft-request', createdAt: '2026-09-29T00:01:00Z',
    };
    const review = {
      companyId: 'company-1', reviewId: 'content-review-1', draftInputId: 'draft-1', draftRevision: '3', draftSha256: 'b'.repeat(64),
      checkerEmployeeId: 'emp-review', correctionId: '', outcome: 'accepted', stale: false,
      review: {draftRevision: '3', checkerEmployeeId: 'emp-review', humanSampled: true, claims: [{claimId: 'claim-1', finding: 'verified', sources: [{inputId: 'source-1', revision: '2', sha256: 'a'.repeat(64)}], limitation: ''}]},
      sample: {draftRevision: '3', draftSha256: 'b'.repeat(64), plan: {inputId: 'plan-1', revision: '1', sha256: 'c'.repeat(64)}, sampledClaimIds: ['claim-1'], sampledByEmployeeId: 'emp-review'},
      reasonCodes: [], requestId: 'content-review-request', createdAt: '2026-09-29T00:02:00Z',
    } as const;
    const publication = {
      companyId: 'company-1', publicationId: 'content-publication-1', reviewId: review.reviewId,
      draftInputId: draft.draftInputId, draftRevision: draft.draftRevision, draftSha256: draft.draftSha256,
      mode: 'simulation', externalSideEffects: false, receiptSha256: 'd'.repeat(64),
      receipt: {schemaVersion: 'polis-content-publication-simulation@1', mode: 'simulation', publicationId: 'content-publication-1', reviewId: review.reviewId, draftInputId: draft.draftInputId, draftRevision: draft.draftRevision, draftSha256: draft.draftSha256, externalSideEffects: false},
      requestId: 'content-publication-request', createdAt: '2026-09-29T00:03:00Z',
    } as const;
    const correction = {
      companyId: 'company-1', correctionId: 'content-correction-1', publicationId: publication.publicationId,
      correctionDraftInputId: draft.draftInputId, correctionDraftRevision: '4', correctionDraftSha256: 'e'.repeat(64),
      rationale: 'revise the statement', state: 'review_required', requestId: 'content-correction-request', createdAt: '2026-09-29T00:04:00Z',
    } as const;
    const feedback = {
      companyId: 'company-1', feedbackId: 'content-feedback-1', publicationId: publication.publicationId,
      category: 'correction_requested', note: 'source was withdrawn', state: 'review_required',
      requestId: 'content-feedback-request', createdAt: '2026-09-29T00:05:00Z',
    } as const;
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ok: true, json: async () => sourceEvent})
      .mockResolvedValueOnce({ok: true, json: async () => draft})
      .mockResolvedValueOnce({ok: true, json: async () => review})
      .mockResolvedValueOnce({ok: true, json: async () => publication})
      .mockResolvedValueOnce({ok: true, json: async () => correction})
      .mockResolvedValueOnce({ok: true, json: async () => feedback});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    await expect(api.setContentSourceAuthorization({
      companyId: 'company-1', sourceInputId: 'source-1', sourceInputRevision: '2', sourceSha256: 'a'.repeat(64),
      state: 'authorized', rationale: 'approved source', requestId: 'content-source-request',
    })).resolves.toEqual(sourceEvent);
    await expect(api.registerContentDraft({
      companyId: 'company-1', draftInputId: 'draft-1', draftInputRevision: '3', writerEmployeeId: 'emp-backend',
      criticalClaims: ['claim-1'], constraintsPassed: true, requestId: 'content-draft-request',
    })).resolves.toEqual(draft);
    await expect(api.recordContentReview({
      companyId: 'company-1', draftInputId: 'draft-1', draftRevision: '3', correctionId: '', review: review.review,
      sample: review.sample, requestId: 'content-review-request',
    })).resolves.toEqual(review);
    await expect(api.simulateContentPublication({companyId: 'company-1', reviewId: review.reviewId, requestId: 'content-publication-request'})).resolves.toEqual(publication);
    await expect(api.recordContentCorrection({
      companyId: 'company-1', publicationId: publication.publicationId, correctionDraftInputId: draft.draftInputId,
      correctionDraftRevision: '4', rationale: 'revise the statement', requestId: 'content-correction-request',
    })).resolves.toEqual(correction);
    await expect(api.recordContentFeedback({
      companyId: 'company-1', publicationId: publication.publicationId, category: 'correction_requested',
      note: 'source was withdrawn', requestId: 'content-feedback-request',
    })).resolves.toEqual(feedback);

    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/workbench/companies/company-1/domain-workflows/content-sources', expect.objectContaining({
      method: 'POST', body: JSON.stringify({sourceInputId: 'source-1', sourceInputRevision: '2', sourceSha256: 'a'.repeat(64), state: 'authorized', rationale: 'approved source', requestId: 'content-source-request'}),
    }));
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/workbench/companies/company-1/domain-workflows/content-drafts', expect.objectContaining({method: 'POST'}));
    expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/workbench/companies/company-1/domain-workflows/content-reviews', expect.objectContaining({method: 'POST'}));
    expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/workbench/companies/company-1/domain-workflows/content-publications', expect.objectContaining({method: 'POST'}));
    expect(fetchMock).toHaveBeenNthCalledWith(5, '/api/workbench/companies/company-1/domain-workflows/content-corrections', expect.objectContaining({method: 'POST'}));
    expect(fetchMock).toHaveBeenNthCalledWith(6, '/api/workbench/companies/company-1/domain-workflows/content-feedback', expect.objectContaining({method: 'POST'}));
  });

  it('records an independent domain evidence reference review with the scoped decision payload', async () => {
    const review = {
      companyId: 'company-1', reviewId: 'domain-review-1', recordId: 'domain-evidence-1', outcome: 'more_evidence_required',
      reviewerEmployeeId: 'emp-review', rationale: 'attach the recovery evidence log', reviewContractRevision: 1,
      previewedEvidence: [{area: 'quality' as const, relativePath: 'quality.md', sourceDigest: 'a'.repeat(64), contentDigest: 'b'.repeat(64), mediaType: 'text/markdown'}],
      requestId: 'domain-review-request-1', createdAt: '2026-09-25T01:00:00Z',
    };
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => review});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    expect(await api.recordDomainEvidenceReview({
      companyId: 'company-1', recordId: 'domain-evidence-1', outcome: 'more_evidence_required',
      reviewerEmployeeId: 'emp-review', rationale: 'attach the recovery evidence log', previewedEvidence: review.previewedEvidence, requestId: 'domain-review-request-1',
    })).toEqual(review);
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/domain-evidence/domain-evidence-1/review', expect.objectContaining({
      method: 'POST', body: JSON.stringify({outcome: 'more_evidence_required', reviewerEmployeeId: 'emp-review', rationale: 'attach the recovery evidence log', previewedEvidence: review.previewedEvidence, requestId: 'domain-review-request-1'}),
    }));
  });

  it('records a substantive per-area domain evidence assessment without claiming profile qualification', async () => {
    const areaAssessments = [
      {area: 'quality' as const, outcome: 'accepted' as const, rationale: 'quality evidence meets the scoped criterion'},
      {area: 'intervention' as const, outcome: 'insufficient' as const, rationale: 'additional intervention log is required'},
      {area: 'recovery' as const, outcome: 'accepted' as const, rationale: 'recovery evidence meets the scoped criterion'},
      {area: 'cost' as const, outcome: 'accepted' as const, rationale: 'cost evidence meets the scoped criterion'},
      {area: 'organization_benefit' as const, outcome: 'accepted' as const, rationale: 'organization evidence meets the scoped criterion'},
    ];
    const previewedEvidence = areaAssessments.map(item => ({area: item.area, relativePath: `${item.area}.md`, sourceDigest: 'a'.repeat(64), contentDigest: 'b'.repeat(64), mediaType: 'text/markdown'}));
    const assessment = {
      companyId: 'company-1', assessmentId: 'domain-substantive-1', recordId: 'domain-evidence-1', evidenceDigest: 'c'.repeat(64),
      outcome: 'more_evidence_required', reviewerEmployeeId: 'emp-independent', areaAssessments, previewedEvidence,
      requestId: 'domain-assessment-request-1', createdAt: '2026-09-26T02:00:00Z',
    };
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => assessment});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    expect(await api.recordDomainEvidenceSubstantiveAssessment({
      companyId: 'company-1', recordId: 'domain-evidence-1', evidenceDigest: 'c'.repeat(64), reviewerEmployeeId: 'emp-independent',
      areaAssessments, previewedEvidence, requestId: 'domain-assessment-request-1',
    })).toEqual(assessment);
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/domain-evidence/domain-evidence-1/assessment', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({evidenceDigest: 'c'.repeat(64), reviewerEmployeeId: 'emp-independent', areaAssessments, previewedEvidence, requestId: 'domain-assessment-request-1'}),
    }));
  });

  it('records a company-scoped global domain qualification decision bound to its CAS report', async () => {
    const decision = {
      companyId: 'company-1', eventId: 'domain-profile-qualification-event-1',
      profileId: 'content-operations-reference', profileRevision: 'content-operations@1', decision: 'qualified' as const,
      evidenceInputId: 'domain-qualification-report', evidenceInputRevision: '2', evidenceSha256: 'a'.repeat(64),
      rationale: 'reviewed accepted evidence across every content operations area', actor: 'local-owner' as const,
      requestId: 'domain-profile-qualification-request-1', createdAt: '2026-09-29T04:00:00Z',
    };
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => decision});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    expect(await api.recordDomainProfileQualification({
      companyId: 'company-1', profileId: decision.profileId, profileRevision: decision.profileRevision, decision: 'qualified',
      evidenceInputId: decision.evidenceInputId, evidenceInputRevision: decision.evidenceInputRevision,
      rationale: decision.rationale, requestId: decision.requestId,
    })).toEqual(decision);
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/domain-workflows/content-operations-reference/qualification', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({decision: 'qualified', profileRevision: decision.profileRevision, evidenceInputId: decision.evidenceInputId, evidenceInputRevision: '2', rationale: decision.rationale, requestId: decision.requestId}),
    }));
  });

  it('loads only the selected company evidence input and verifies both source and preview digests', async () => {
    const bytes = new TextEncoder().encode('reviewable quality evidence');
    const blob = new Blob([bytes], {type: 'text/markdown'});
    const sourceDigest = 'a'.repeat(64);
    const contentDigest = await crypto.subtle.digest('SHA-256', bytes).then(value => Array.from(new Uint8Array(value), byte => byte.toString(16).padStart(2, '0')).join(''));
    const fetchMock = vi.fn().mockResolvedValue(new Response(blob, {headers: {
      'Content-Type': 'text/markdown', 'Content-Length': String(bytes.length),
      'X-Source-SHA256': sourceDigest, 'X-Content-SHA256': contentDigest, 'X-Polis-Preview-Filename': 'quality.md',
      'X-Polis-Input-ID': 'input-1', 'X-Polis-Input-Revision': '2',
    }}));
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench', false, 'preview-session-token');

    const preview = await api.getDomainEvidenceArtifactPreview({
      companyId: 'company-1', recordId: 'domain-evidence-1', area: 'quality', inputId: 'input-1', inputRevision: 2, sourceDigest, relativePath: 'reports/quality.md', expectedContentSHA256: contentDigest,
    });

    expect(preview.sourceDigest).toBe(sourceDigest);
    expect(preview.contentDigest).toBe(contentDigest);
    expect(preview.fileName).toBe('quality.md');
    expect(preview.mediaType).toBe('text/markdown');
    expect(preview.textContent).toBe('reviewable quality evidence');
    expect(preview.relativePath).toBe('reports/quality.md');
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/domain-evidence/domain-evidence-1/evidence/quality/preview?path=reports%2Fquality.md', expect.objectContaining({
      credentials: 'same-origin', headers: expect.objectContaining({'X-Polis-Desktop-Token': 'preview-session-token'}),
    }));
  });

  it('reads a validated preview-entry manifest bound to the exact input revision', async () => {
    const manifest = {
      companyId: 'company-1', recordId: 'domain-evidence-1', area: 'quality', inputId: 'input-1', inputRevision: 2, sourceDigest: 'a'.repeat(64),
      entries: [
        {relativePath: 'reports/quality.md', fileName: 'quality.md', mediaType: 'text/markdown', byteSize: 25, contentSHA256: 'b'.repeat(64), previewable: true, reasonCode: ''},
        {relativePath: 'reports/unsafe.svg', fileName: 'unsafe.svg', mediaType: 'application/octet-stream', byteSize: 20, contentSHA256: 'c'.repeat(64), previewable: false, reasonCode: 'evidence_preview_unsupported'},
      ],
    };
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => manifest});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    expect(await api.listDomainEvidenceArtifactPreviewEntries({
      companyId: 'company-1', recordId: 'domain-evidence-1', area: 'quality', inputId: 'input-1', inputRevision: 2, sourceDigest: 'a'.repeat(64),
    })).toEqual(manifest);
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/domain-evidence/domain-evidence-1/evidence/quality/preview/entries', expect.objectContaining({credentials: 'same-origin'}));
  });

  it('rejects a preview with an unbound source hash or changed preview bytes', async () => {
    const bytes = new TextEncoder().encode('source text');
    const expectedDigest = 'a'.repeat(64);
    const actualDigest = await crypto.subtle.digest('SHA-256', bytes).then(value => Array.from(new Uint8Array(value), byte => byte.toString(16).padStart(2, '0')).join(''));
    const headers = {
      'Content-Type': 'text/plain', 'Content-Length': String(bytes.length), 'X-Polis-Preview-Filename': 'quality.txt',
      'X-Polis-Input-ID': 'input-1', 'X-Polis-Input-Revision': '1', 'X-Content-SHA256': actualDigest,
    };
    const apiOptions = {companyId: 'company-1', recordId: 'domain-evidence-1', area: 'quality' as const, inputId: 'input-1', inputRevision: 1, sourceDigest: expectedDigest, relativePath: 'quality.txt', expectedContentSHA256: actualDigest};
    const tamperedBytes = new TextEncoder().encode('source test');
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(new Blob([bytes]), {headers: {...headers, 'X-Source-SHA256': 'b'.repeat(64)}}))
      .mockResolvedValueOnce(new Response(new Blob([bytes]), {headers: {...headers, 'X-Source-SHA256': expectedDigest, 'X-Content-SHA256': 'c'.repeat(64)}}))
      .mockResolvedValueOnce(new Response(new Blob([tamperedBytes]), {headers: {...headers, 'X-Source-SHA256': expectedDigest}}));
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    await expect(api.getDomainEvidenceArtifactPreview(apiOptions)).rejects.toThrow('metadata does not match');
    await expect(api.getDomainEvidenceArtifactPreview(apiOptions)).rejects.toThrow('metadata does not match');
    await expect(api.getDomainEvidenceArtifactPreview(apiOptions)).rejects.toThrow('bytes do not match');
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

  it('verifies the downloaded artifact against its canonical manifest and checksum list', async () => {
    const artifact = new TextEncoder().encode('qualified artifact payload');
    const manifest = {
      schemaVersion: 'polis-delivery-manifest@1',
      companyId: 'company-1',
      artifactId: 'artifact-1',
      taskId: 'task-1',
      content: {fileName: 'artifact.bin', contentType: 'application/octet-stream', byteSize: String(artifact.byteLength), sha256: await sha256Hex(artifact)},
      state: 'ready',
      verdict: 'passed',
      qualification: {
        checkpointId: 'checkpoint-1',
        validationReceiptId: 'receipt-1',
        taskValidationBindingDigest: 'a'.repeat(64),
        workspaceDigest: 'b'.repeat(64),
        workspaceRevision: '7',
        runnerRevision: 'runner-test@1',
      },
      createdAt: '2026-09-24T00:00:00Z',
    };
    const manifestBytes = new TextEncoder().encode(JSON.stringify(manifest));
    const manifestDigest = await sha256Hex(manifestBytes);
    const sums = new TextEncoder().encode(`${manifestDigest}  manifest.json\n${manifest.content.sha256}  artifact.bin\n`);
    const archive = buildStoredZip([['artifact.bin', artifact], ['manifest.json', manifestBytes], ['SHA256SUMS', sums]]);
    const packageDigest = await sha256Hex(archive);
    const fetchMock = vi.fn().mockResolvedValue(zipResponse(archive, packageDigest, manifestDigest));
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    const blob = await api.downloadArtifactPackage({companyId: 'company-1', artifactId: 'artifact-1'});

    expect(blob.type).toBe('application/zip');
    expect(blob.size).toBe(archive.byteLength);
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/artifacts/artifact-1/download', expect.objectContaining({credentials: 'same-origin'}));
  });

  it('treats a legacy durable delivery response without backlog as an empty backlog', async () => {
    const manifest = {
      schemaVersion: 'polis-durable-delivery-manifest@1', deliveryId: 'artifact-1', revision: '1', companyId: 'company-1',
      missionId: 'mission-1', taskId: 'task-1', artifactId: 'artifact-1', state: 'assembling',
      artifact: {fileName: 'artifact.bin', byteSize: '1', sha256: 'a'.repeat(64)},
      sections: [{key: 'file_inventory', state: 'available', detail: 'artifact'}], createdAt: '2026-10-06T00:00:00Z',
    };
    const manifestSha256 = await sha256Hex(new TextEncoder().encode(JSON.stringify(manifest)));
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => ({
      manifest, manifestSha256,
      userDisposition: {revision: '1', manifestRevision: '1', state: 'not_requested', actor: 'system', reason: 'not requested', requestId: 'request-1', feedbackDeadline: '', createdAt: '2026-10-06T00:00:00Z'},
    })});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    await expect(api.getDurableDelivery({companyId: 'company-1', artifactId: 'artifact-1'})).resolves.toMatchObject({feedbackBacklog: []});
  });

  it('posts assembling delivery evidence through the owner completion route', async () => {
    const receipt = {requestId: 'complete-1', companyId: 'company-1', deliveryId: 'artifact-1', manifestRevision: '2', dispositionRevision: '1', state: 'ready', actor: 'system', createdAt: '2026-10-06T00:00:00Z'};
    const evidence = {reference: 'ref-1', digest: 'a'.repeat(64), detail: 'bounded evidence'};
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => receipt});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench', false, 'owner-token');

    await expect(api.completeDurableDeliveryManifest({
      companyId: 'company-1', artifactId: 'artifact-1', requestId: 'complete-1', expectedManifestRevision: '1',
      sourceInputs: evidence, environmentBuild: evidence, runInstructions: evidence, limitations: evidence, licenseSource: evidence,
    })).resolves.toEqual(receipt);
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/artifacts/artifact-1/delivery/complete', expect.objectContaining({method: 'POST', body: expect.stringContaining('complete-1')}));
  });

  it('sends the desktop session token with delivery manifest and package requests', async () => {
    const artifact = new TextEncoder().encode('qualified artifact payload');
    const manifest = {
      schemaVersion: 'polis-delivery-manifest@1', companyId: 'company-1', artifactId: 'artifact-1', taskId: 'task-1',
      content: {fileName: 'artifact.bin', contentType: 'application/octet-stream', byteSize: String(artifact.byteLength), sha256: await sha256Hex(artifact)},
      state: 'ready', verdict: 'passed',
      qualification: {checkpointId: 'checkpoint-1', validationReceiptId: 'receipt-1', taskValidationBindingDigest: 'a'.repeat(64), workspaceDigest: 'b'.repeat(64), workspaceRevision: '7', runnerRevision: 'runner-test@1'},
      createdAt: '2026-09-24T00:00:00Z',
    };
    const manifestBytes = new TextEncoder().encode(JSON.stringify(manifest));
    const manifestDigest = await sha256Hex(manifestBytes);
    const sums = new TextEncoder().encode(`${manifestDigest}  manifest.json\n${manifest.content.sha256}  artifact.bin\n`);
    const archive = buildStoredZip([['artifact.bin', artifact], ['manifest.json', manifestBytes], ['SHA256SUMS', sums]]);
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ok: true, json: async () => ({manifest, manifestSha256: manifestDigest})})
      .mockResolvedValueOnce(zipResponse(archive, await sha256Hex(archive), manifestDigest));
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench', false, 'desktop-session-fixture');

    await api.getArtifactDeliveryManifest({companyId: 'company-1', artifactId: 'artifact-1'});
    await api.downloadArtifactPackage({companyId: 'company-1', artifactId: 'artifact-1'});

    const expectedHeaders = expect.objectContaining({'X-Polis-Desktop-Token': 'desktop-session-fixture'});
    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/workbench/companies/company-1/artifacts/artifact-1/manifest', expect.objectContaining({headers: expectedHeaders}));
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/workbench/companies/company-1/artifacts/artifact-1/download', expect.objectContaining({headers: expectedHeaders}));
  });

  it('rejects a download whose package bytes do not match the response digest', async () => {
    const artifact = new TextEncoder().encode('qualified artifact payload');
    const manifest = {
      schemaVersion: 'polis-delivery-manifest@1', companyId: 'company-1', artifactId: 'artifact-1', taskId: 'task-1',
      content: {fileName: 'artifact.bin', contentType: 'application/octet-stream', byteSize: String(artifact.byteLength), sha256: await sha256Hex(artifact)},
      state: 'ready', verdict: 'passed',
      qualification: {checkpointId: 'checkpoint-1', validationReceiptId: 'receipt-1', taskValidationBindingDigest: 'a'.repeat(64), workspaceDigest: 'b'.repeat(64), workspaceRevision: '7', runnerRevision: 'runner-test@1'},
      createdAt: '2026-09-24T00:00:00Z',
    };
    const manifestBytes = new TextEncoder().encode(JSON.stringify(manifest));
    const manifestDigest = await sha256Hex(manifestBytes);
    const sums = new TextEncoder().encode(`${manifestDigest}  manifest.json\n${manifest.content.sha256}  artifact.bin\n`);
    const archive = buildStoredZip([['artifact.bin', artifact], ['manifest.json', manifestBytes], ['SHA256SUMS', sums]]);
    const changed = new Uint8Array(archive);
    changed[changed.length - 1] ^= 1;
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(zipResponse(changed, await sha256Hex(archive), manifestDigest)));
    const api = new RealWorkbenchApi('/api/workbench');

    await expect(api.downloadArtifactPackage({companyId: 'company-1', artifactId: 'artifact-1'})).rejects.toThrow('package checksum does not match');
  });

  it('reads scoped project environment and Task JobRun lifecycle projections', async () => {
    const environment = {
      companyId: 'company-1', revisionId: 'environment-1', missionId: 'mission-1', sourceInputId: 'input-1', sourceInputRevision: '1', projectRootRelative: 'repo', sourceBindingStatus: 'bound', profileId: 'windows-node-npm@1',
      sourceRevisionSha256: 'a'.repeat(64), packageJsonSha256: 'b'.repeat(64), lockfileSha256: 'c'.repeat(64),
      policySha256: 'd'.repeat(64), policyManifest: {schemaVersion: 'project-environment-policy@1', profileId: 'windows-node-npm@1', registryHosts: ['registry.npmjs.org'], installPolicy: 'npm ci --ignore-scripts --no-audit --no-fund', lifecycleScriptsPolicy: 'ignore', networkPolicy: 'registry_allowlist', timeoutMs: 180000, outputLimitBytes: 1048576},
      toolchainSha256: 'e'.repeat(64), executorFingerprintSha256: 'f'.repeat(64), hostFingerprintSha256: '1'.repeat(64), isolationPolicySha256: '2'.repeat(64), executorEvidenceSha256: null, executorEvidenceInputId: null, executorEvidenceInputRevision: null, policyDecision: 'approved',
      executorQualification: 'unqualified', preparationState: 'blocked_unqualified',
      preparationReason: 'environment_executor_not_qualified', preparationRunId: 'run-1', createdAt: '2026-09-24T00:00:00Z',
    };
    const job = {
      companyId: 'company-1', jobId: 'job-1', taskId: 'task-1', sessionId: 'session-1', environmentRevisionId: 'environment-1', handoverId: '',
      kind: 'service', serviceId: 'web', state: 'running', readiness: 'ready', exitCode: null, reasonCode: 'none', stdoutOffset: '128', stderrOffset: '0',
      stdoutBytes: '64', stderrBytes: '0', logsTruncated: false, logGap: false, logManifestSha256: 'f'.repeat(64),
      serviceEndpoint: {generation: '1', bindAddress: '127.0.0.1', port: '3000', readiness: 'ready', sourceRevisionSha256: 'a'.repeat(64), healthcheckSha256: 'b'.repeat(64), leaseExpiresAt: '2026-09-24T00:01:00Z'},
      createdAt: '2026-09-24T00:00:00Z', updatedAt: '2026-09-24T00:00:01Z',
    };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ok: true, json: async () => [environment]})
      .mockResolvedValueOnce({ok: true, json: async () => [job]});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    const environments = await api.listProjectEnvironments({companyId: 'company-1'});
    const jobs = await api.listTaskJobRuns({companyId: 'company-1', taskId: 'task-1'});

    expect(environments[0].preparationState).toBe('blocked_unqualified');
    expect(jobs[0].readiness).toBe('ready');
    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/workbench/companies/company-1/environments', expect.anything());
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/workbench/companies/company-1/tasks/task-1/jobs', expect.anything());
  });

  it('lists and creates a company-scoped cross-backend handover', async () => {
    const handover = {
      companyId: 'company-1', handoverId: 'handover-1', missionId: 'mission-1', taskId: 'task-1', sourceJobId: 'job-1',
      sourceSessionId: 'session-1', sourceRuntimeIncarnation: 'runtime-windows-1', sourceEnvironmentRevisionId: 'windows-env-1',
      sourceProfileId: 'windows-node-npm@1', targetEnvironmentRevisionId: 'linux-env-1', targetProfileId: 'linux-node-npm@1',
      projectSourceSha256: 'a'.repeat(64), packageJsonSha256: 'b'.repeat(64), lockfileSha256: 'c'.repeat(64),
      workspaceDigest: 'd'.repeat(64), workspaceRevision: 7, taskInputManifestSha256: 'e'.repeat(64),
      targetPolicySha256: 'f'.repeat(64), targetToolchainSha256: '1'.repeat(64), requestId: 'handover-create-1',
      recordSha256: '2'.repeat(64), createdAt: '2026-09-26T00:00:00Z',
    };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ok: true, json: async () => [handover]})
      .mockResolvedValueOnce({ok: true, json: async () => handover});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    expect(await api.listTaskCrossBackendHandovers({companyId: 'company-1', taskId: 'task-1'})).toHaveLength(1);
    await api.createTaskEnvironmentHandover({companyId: 'company-1', taskId: 'task-1', sourceJobId: 'job-1', targetEnvironmentRevisionId: 'linux-env-1', requestId: 'handover-create-1'});

    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/workbench/companies/company-1/tasks/task-1/environment-handovers', expect.anything());
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/workbench/companies/company-1/tasks/task-1/environment-handovers', expect.objectContaining({method: 'POST'}));
  });

  it('starts, stops, and reads logs for a company-scoped Task JobRun', async () => {
    const started = {companyId: 'company-1', jobId: 'job-1', taskId: 'task-1', kind: 'batch', serviceId: '', state: 'running', readiness: 'not_applicable', reasonCode: 'project_job_running'};
    const stopped = {...started, state: 'cancelled', reasonCode: 'project_job_cancelled'};
    const logs = {companyId: 'company-1', jobId: 'job-1', manifestSha256: 'a'.repeat(64), content: 'bWFuaWZlc3Q='};
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ok: true, json: async () => started})
      .mockResolvedValueOnce({ok: true, json: async () => stopped})
      .mockResolvedValueOnce({ok: true, json: async () => logs});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    const start = await api.startTaskJobRun({companyId: 'company-1', taskId: 'task-1', sessionId: '', environmentRevisionId: 'environment-1', handoverId: 'handover-1', scriptPath: 'scripts/build.mjs', args: ['--check'], requestId: 'job-start-1'});
    const stop = await api.stopTaskJobRun({companyId: 'company-1', jobId: 'job-1', requestId: 'job-stop-1'});
    const artifact = await api.getTaskJobLogs({companyId: 'company-1', jobId: 'job-1'});

    expect(start.state).toBe('running');
    expect(stop.state).toBe('cancelled');
    expect(artifact.manifestSha256).toBe('a'.repeat(64));
    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/workbench/companies/company-1/tasks/task-1/jobs', expect.objectContaining({method: 'POST', body: expect.stringContaining('handover-1')}));
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toMatchObject({sessionId: '', handoverId: 'handover-1'});
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/workbench/companies/company-1/jobs/job-1/stop', expect.objectContaining({method: 'POST'}));
    expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/workbench/companies/company-1/jobs/job-1/logs', expect.anything());
  });

  it('starts an approved service by ID without accepting a caller command', async () => {
    const response = {companyId: 'company-1', jobId: 'service-job-1', taskId: 'task-1', kind: 'service', serviceId: 'web', state: 'running', readiness: 'ready', reasonCode: 'service_endpoint_ready'};
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => response});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    await api.startTaskJobRun({companyId: 'company-1', taskId: 'task-1', sessionId: 'session-1', environmentRevisionId: 'environment-1', kind: 'service', serviceId: 'web', requestId: 'service-start-1'});

    const request = JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body)) as Record<string, unknown>;
    expect(request).toEqual({taskId: 'task-1', sessionId: 'session-1', environmentRevisionId: 'environment-1', kind: 'service', serviceId: 'web', requestId: 'service-start-1'});
  });

  it('creates a bounded loopback browser session for a service JobRun', async () => {
    const response = {url: `http://127.0.0.1:43123/_polis/open/${'a'.repeat(64)}`, expiresAt: new Date(Date.now() + 60_000).toISOString()};
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => response});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    const session = await api.createProjectJobBrowserSession({companyId: 'company-1', jobId: 'job-1', requestId: 'browser-session-1'});

    expect(session.url).toBe(response.url);
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/jobs/job-1/browser-session', expect.objectContaining({method: 'POST'}));
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({requestId: 'browser-session-1'});
  });

  it('rejects a non-loopback browser session response', async () => {
    const response = {url: `https://example.invalid/_polis/open/${'a'.repeat(64)}`, expiresAt: new Date(Date.now() + 60_000).toISOString()};
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ok: true, json: async () => response}));
    const api = new RealWorkbenchApi('/api/workbench');

    await expect(api.createProjectJobBrowserSession({companyId: 'company-1', jobId: 'job-1', requestId: 'browser-session-1'})).rejects.toThrow('service browser session');
  });

  it('reads the company-scoped bounded GitHub feedback projection', async () => {
    const feedback = {
      companyId: 'company-1',
      sources: [{sourceId: 'source-1', provider: 'github', repositoryId: '1296269', repository: 'acme/widget', profileRevision: 'github-issues-readonly@1', state: 'approved', permissionStatus: 'verified', coverage: 'complete', coverageReason: '', coveredThrough: '2026-09-24T00:00:00Z', lastScanAt: '2026-09-24T00:00:00Z', collectionEnabled: false, collectionIntervalSeconds: 0, collectionRationale: '', collectionNextPollAt: null, collectionLastAttempt: '', collectionLastReasonCode: ''}],
      issues: [{sourceId: 'source-1', providerItemId: '101', issueNumber: '7', revisionSha256: 'a'.repeat(64), backlogStatus: 'open', backlogReason: 'new_issue_observed', backlogUpdatedAt: '2026-09-24T00:00:01Z', title: 'issue', titleTruncated: false, body: 'untrusted body', bodySha256: 'b'.repeat(64), bodyTruncated: false, state: 'open', sourceUpdatedAt: '2026-09-24T00:00:00Z', observedAt: '2026-09-24T00:00:01Z', htmlUrl: 'https://github.com/acme/widget/issues/7', commentCoverage: 'not_read', commentCoverageReason: '', commentCount: '0', commentContextPartial: false, comments: []}],
    };
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => feedback});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    const actual = await api.getCompanyFeedback({companyId: 'company-1'});

    expect(actual.issues[0]?.providerItemId).toBe('101');
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/feedback', expect.anything());
  });

  it('rejects a feedback response from another company scope', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ok: true, json: async () => ({companyId: 'company-2', sources: [], issues: []})}));
    const api = new RealWorkbenchApi('/api/workbench');

    await expect(api.getCompanyFeedback({companyId: 'company-1'})).rejects.toThrow('feedback');
  });

  it('stores and deletes a GitHub token without returning secret material', async () => {
    const token = `github_pat_${'A'.repeat(40)}`;
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ok: true, json: async () => ({credentialRef: 'default-readonly', stored: true})})
      .mockResolvedValueOnce({ok: true, json: async () => ({credentialRef: 'default-readonly', stored: false})});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    const stored = await api.storeGitHubFeedbackCredential({companyId: 'company-1', token, requestId: 'github-store-1'});
    const deleted = await api.deleteGitHubFeedbackCredential({companyId: 'company-1', requestId: 'github-delete-1'});

    expect(stored).toEqual({credentialRef: 'default-readonly', stored: true});
    expect(deleted).toEqual({credentialRef: 'default-readonly', stored: false});
    expect(JSON.stringify(stored)).not.toContain(token);
    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/workbench/companies/company-1/feedback/credentials', expect.objectContaining({method: 'POST', body: JSON.stringify({token})}));
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/workbench/companies/company-1/feedback/credentials/delete', expect.objectContaining({method: 'POST', body: '{}'}));
  });

  it('runs explicit GitHub source registration, permission probe, manual approval and bounded poll commands', async () => {
    const sourceReceipt = {companyId: 'company-1', sourceId: 'source-1', repositoryId: '1296269', repository: 'acme/widget', profileRevision: 'github-issues-readonly@1', filterRevision: 'github-issues-overlap-updated-asc@1', configurationSha256: 'a'.repeat(64), state: 'draft', permissionStatus: 'unverified', createdAt: '2026-09-24T00:00:00Z'};
    const probeReceipt = {companyId: 'company-1', sourceId: 'source-1', requestId: 'probe-1', permissionStatus: 'verified', coverage: 'complete', coverageReason: ''};
    const pollReceipt = {companyId: 'company-1', sourceId: 'source-1', requestId: 'poll-1', scanId: 'scan-1', coverage: 'partial', coverageReason: 'github_rate_limited', coveredThrough: null, pageCount: 1, itemCount: 1, commentScanCount: 1, commentCoverage: 'partial', commentCoverageReason: 'github_rate_limited', replayed: false};
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ok: true, json: async () => sourceReceipt})
      .mockResolvedValueOnce({ok: true, json: async () => probeReceipt})
      .mockResolvedValueOnce({ok: true, json: async () => ({...sourceReceipt, state: 'approved', permissionStatus: 'verified'})})
      .mockResolvedValueOnce({ok: true, json: async () => pollReceipt});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    const registered = await api.registerGitHubFeedbackSource({companyId: 'company-1', repositoryId: '1296269', owner: 'acme', name: 'widget', requestId: 'register-1'});
    const probed = await api.probeGitHubFeedbackSource({companyId: 'company-1', sourceId: 'source-1', rationale: 'verify selected repository read access', requestId: 'probe-1'});
    const approved = await api.decideGitHubFeedbackSource({companyId: 'company-1', sourceId: 'source-1', decision: 'approved', rationale: 'approve selected source', requestId: 'approve-1'});
    const polled = await api.pollGitHubFeedbackSource({companyId: 'company-1', sourceId: 'source-1', requestId: 'poll-1'});

    expect(registered.state).toBe('draft');
    expect(probed.permissionStatus).toBe('verified');
    expect(approved.state).toBe('approved');
    expect(polled.coverage).toBe('partial');
    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/workbench/companies/company-1/feedback/sources', expect.objectContaining({method: 'POST', body: JSON.stringify({sourceId: '', repositoryId: 1296269, owner: 'acme', name: 'widget', requestId: 'register-1'})}));
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/workbench/companies/company-1/feedback/sources/source-1/probe', expect.objectContaining({method: 'POST'}));
    expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/workbench/companies/company-1/feedback/sources/source-1/decision', expect.objectContaining({method: 'POST'}));
    expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/workbench/companies/company-1/feedback/sources/source-1/scan', expect.objectContaining({method: 'POST'}));
  });

  it('records an explicit bounded GitHub collection policy and verifies both global gates', async () => {
    const receipt = {companyId: 'company-1', sourceId: 'source-1', enabled: true, intervalSeconds: 3600, rationale: 'bounded collection', updatedAt: '2026-09-29T00:00:00Z', nextPollAt: '2026-09-29T01:00:00Z', lastAttempt: '', lastReasonCode: '', requestId: 'policy-1', externalEnabled: false, schedulerEnabled: false};
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => receipt});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    const actual = await api.setGitHubFeedbackCollectionPolicy({companyId: 'company-1', sourceId: 'source-1', enabled: true, intervalSeconds: 3600, rationale: 'bounded collection', requestId: 'policy-1'});

    expect(actual).toEqual(receipt);
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/feedback/collection-policy', expect.objectContaining({method: 'POST', body: JSON.stringify({sourceId: 'source-1', enabled: true, intervalSeconds: 3600, rationale: 'bounded collection', requestId: 'policy-1'})}));
    await expect(api.setGitHubFeedbackCollectionPolicy({companyId: 'company-1', sourceId: 'source-1', enabled: true, intervalSeconds: 899, rationale: 'too fast', requestId: 'policy-2'})).rejects.toThrow('between 15 minutes and 24 hours');
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('changes only the company backlog state and verifies the remote issue is unchanged', async () => {
    const receipt = {
      companyId: 'company-1', sourceId: 'source-1', providerItemId: '101', issueNumber: 7,
      revisionSha256: 'a'.repeat(64), status: 'handled', rationale: 'triaged internally',
      eventId: 'backlog-event-1', requestId: 'backlog-1', remoteState: 'open', remoteUnchanged: true,
    };
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => receipt});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    const result = await api.setGitHubFeedbackBacklogStatus({
      companyId: 'company-1', sourceId: 'source-1', providerItemId: '101', revisionSha256: 'a'.repeat(64), status: 'handled',
      rationale: 'triaged internally', requestId: 'backlog-1',
    });

    expect(result.remoteUnchanged).toBe(true);
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/feedback/backlog', expect.objectContaining({
      method: 'POST', body: JSON.stringify({sourceId: 'source-1', providerItemId: '101', revisionSha256: 'a'.repeat(64), status: 'handled', rationale: 'triaged internally', requestId: 'backlog-1'}),
    }));
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ok: true, json: async () => ({...receipt, remoteUnchanged: false})}));
    await expect(api.setGitHubFeedbackBacklogStatus({
      companyId: 'company-1', sourceId: 'source-1', providerItemId: '101', revisionSha256: 'a'.repeat(64), status: 'handled',
      rationale: 'triaged internally', requestId: 'backlog-1',
    })).rejects.toThrow('backlog receipt');
  });

  it('records environment policy decisions and returns blocked preparation receipts without executing', async () => {
    const preparation = {companyId: 'company-1', runId: 'run-1', revisionId: 'environment-1', state: 'blocked_unqualified', reasonCode: 'environment_executor_not_qualified', requestId: 'ensure-1', createdAt: '2026-09-24T00:00:00Z'};
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ok: true, json: async () => ({id: 'policy-event-1', status: 'approved'})})
      .mockResolvedValueOnce({ok: true, json: async () => preparation});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    const decision = await api.decideProjectEnvironmentPolicy({companyId: 'company-1', revisionId: 'environment-1', decision: 'approved', rationale: 'reviewed fixed lockfile and source policy', requestId: 'policy-1'});
    const run = await api.ensureProjectEnvironment({companyId: 'company-1', revisionId: 'environment-1', requestId: 'ensure-1'});

    expect(decision.status).toBe('approved');
    expect(run.state).toBe('blocked_unqualified');
    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/workbench/companies/company-1/environments/environment-1/policy', expect.objectContaining({method: 'POST', body: JSON.stringify({decision: 'approved', rationale: 'reviewed fixed lockfile and source policy', requestId: 'policy-1'})}));
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/workbench/companies/company-1/environments/environment-1/preparation', expect.objectContaining({method: 'POST', body: JSON.stringify({requestId: 'ensure-1'})}));
  });

  it('records executor qualification against a same-company evidence revision', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => ({id: 'executor-qualification-1', status: 'qualified'})});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    const receipt = await api.decideProjectEnvironmentExecutorQualification({
      companyId: 'company-1', revisionId: 'environment-1', decision: 'qualified', evidenceInputId: 'input-evidence-1', evidenceInputRevision: '2',
      rationale: 'reviewed the host qualification report', requestId: 'executor-qualification-request-1',
    });

    expect(receipt.status).toBe('qualified');
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/environments/environment-1/executor-qualification', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({decision: 'qualified', evidenceInputId: 'input-evidence-1', evidenceInputRevision: '2', qualifiedUntil: undefined, rationale: 'reviewed the host qualification report', requestId: 'executor-qualification-request-1'}),
    }));
  });

  it('surfaces a command transport outage instead of treating it as accepted', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('command backend unavailable')));
    const api = new RealWorkbenchApi('/api/workbench');

    await expect(api.cancelMission({companyId: 'company-1', missionId: 'mission-1', requestId: 'request-3'})).rejects.toThrow('command backend unavailable');
  });
});

async function sha256Hex(content: Uint8Array): Promise<string> {
  const copy = new Uint8Array(content.byteLength);
  copy.set(content);
  const digest = await crypto.subtle.digest('SHA-256', copy.buffer);
  return Array.from(new Uint8Array(digest), value => value.toString(16).padStart(2, '0')).join('');
}

function zipResponse(bytes: Uint8Array, packageDigest: string, manifestDigest: string) {
  const copy = new Uint8Array(bytes);
  return {
    ok: true,
    status: 200,
    headers: {get: (name: string) => ({'content-type': 'application/zip', 'x-content-sha256': packageDigest, 'x-polis-manifest-sha256': manifestDigest}[name.toLowerCase()] ?? null)},
    arrayBuffer: async () => copy.buffer,
  };
}

function buildStoredZip(entries: ReadonlyArray<readonly [string, Uint8Array]>): Uint8Array {
  const encoder = new TextEncoder();
  const localParts: Uint8Array[] = [];
  const centralParts: Uint8Array[] = [];
  let localOffset = 0;
  for (const [name, content] of entries) {
    const nameBytes = encoder.encode(name);
    const checksum = crc32(content);
    const local = new Uint8Array(30 + nameBytes.length + content.length);
    const localView = new DataView(local.buffer);
    localView.setUint32(0, 0x04034b50, true);
    localView.setUint16(4, 20, true);
    localView.setUint16(8, 0, true);
    localView.setUint32(14, checksum, true);
    localView.setUint32(18, content.length, true);
    localView.setUint32(22, content.length, true);
    localView.setUint16(26, nameBytes.length, true);
    local.set(nameBytes, 30);
    local.set(content, 30 + nameBytes.length);

    const central = new Uint8Array(46 + nameBytes.length);
    const centralView = new DataView(central.buffer);
    centralView.setUint32(0, 0x02014b50, true);
    centralView.setUint16(4, 20, true);
    centralView.setUint16(6, 20, true);
    centralView.setUint32(16, checksum, true);
    centralView.setUint32(20, content.length, true);
    centralView.setUint32(24, content.length, true);
    centralView.setUint16(28, nameBytes.length, true);
    centralView.setUint32(42, localOffset, true);
    central.set(nameBytes, 46);
    localParts.push(local);
    centralParts.push(central);
    localOffset += local.length;
  }
  const centralOffset = localOffset;
  const centralBytes = concatenate(centralParts);
  const end = new Uint8Array(22);
  const endView = new DataView(end.buffer);
  endView.setUint32(0, 0x06054b50, true);
  endView.setUint16(8, entries.length, true);
  endView.setUint16(10, entries.length, true);
  endView.setUint32(12, centralBytes.length, true);
  endView.setUint32(16, centralOffset, true);
  return concatenate([...localParts, centralBytes, end]);
}

function concatenate(parts: ReadonlyArray<Uint8Array>): Uint8Array {
  const result = new Uint8Array(parts.reduce((total, part) => total + part.length, 0));
  let offset = 0;
  for (const part of parts) {
    result.set(part, offset);
    offset += part.length;
  }
  return result;
}

function crc32(content: Uint8Array): number {
  let checksum = 0xffffffff;
  for (const byte of content) {
    checksum ^= byte;
    for (let bit = 0; bit < 8; bit++) checksum = (checksum >>> 1) ^ ((checksum & 1) === 1 ? 0xedb88320 : 0);
  }
  return (checksum ^ 0xffffffff) >>> 0;
}
