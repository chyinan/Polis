// pattern: Imperative Shell

import {afterEach, describe, expect, it, vi} from 'vitest';
import {RealWorkbenchApi} from './real-workbench-api';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('RealWorkbenchApi mission inputs', () => {
  it('reads only revisions for the requested company and mission', async () => {
    const expected = [{
      inputId: 'input-1',
      companyId: 'company-1',
      missionId: 'mission-1',
      requestId: 'upload-1',
      revision: '1',
      sourceKind: 'upload',
      displayName: 'goal.md',
      mediaType: 'text/markdown',
      byteSize: '7',
      contentDigest: 'a'.repeat(64),
      state: 'usable',
      imageWidth: '0',
      imageHeight: '0',
      createdAt: '2026-09-23T00:00:00Z',
    }];
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => expected});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    const actual = await api.listMissionInputs({companyId: 'company-1', missionId: 'mission-1'});

    expect(actual).toEqual(expected);
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/missions/mission-1/inputs', expect.objectContaining({credentials: 'same-origin'}));
  });

  it('uploads one file as multipart without setting a JSON content type', async () => {
    const expected = {
      inputId: 'input-1',
      companyId: 'company-1',
      missionId: 'mission-1',
      revision: 1,
      requestId: 'upload-1',
      sourceKind: 'upload',
      displayName: 'goal.md',
      mediaType: 'text/markdown',
      byteSize: 7,
      contentDigest: 'a'.repeat(64),
      state: 'usable',
    };
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => expected});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');
    const file = new File(['# goal\n'], 'goal.md', {type: 'text/markdown'});

    const receipt = await api.uploadMissionInput({companyId: 'company-1', missionId: 'mission-1', inputId: null, requestId: 'upload-1', file});

    expect(receipt).toEqual(expected);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/workbench/companies/company-1/missions/mission-1/inputs');
    expect(init.method).toBe('POST');
    expect(new Headers(init.headers).get('Content-Type')).toBeNull();
    const form = init.body as FormData;
    expect(form.get('requestId')).toBe('upload-1');
    expect(form.get('inputId')).toBeNull();
    expect((form.get('file') as File).name).toBe('goal.md');
  });

  it('uploads a selected directory as ordered relative paths and files', async () => {
    const expected = {
      inputId: 'tree-1',
      companyId: 'company-1',
      missionId: 'mission-1',
      revision: 1,
      requestId: 'directory-1',
      sourceKind: 'directory_snapshot',
      displayName: 'project',
      mediaType: 'application/gzip',
      byteSize: 128,
      contentDigest: 'b'.repeat(64),
      state: 'usable',
    };
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => expected});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');
    const readme = new File(['# Project\n'], 'README.md', {type: 'text/markdown'});
    const source = new File(['package main\n'], 'main.go', {type: 'text/plain'});

    const receipt = await api.uploadMissionDirectoryInput({
      companyId: 'company-1', missionId: 'mission-1', inputId: 'tree-1', requestId: 'directory-1',
      files: [{relativePath: 'project/README.md', file: readme}, {relativePath: 'project/src/main.go', file: source}],
    });

    expect(receipt).toEqual(expected);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/workbench/companies/company-1/missions/mission-1/inputs/directory');
    expect(init.method).toBe('POST');
    expect(new Headers(init.headers).get('Content-Type')).toBeNull();
    const form = init.body as FormData;
    expect(form.getAll('paths')).toEqual(['project/README.md', 'project/src/main.go']);
    expect((form.getAll('files')[0] as File).name).toBe('README.md');
    expect((form.getAll('files')[1] as File).name).toBe('main.go');
  });

  it('reads a digest-bound Task input manifest and preserves its not-delivered state', async () => {
    const expected = {
      companyId: 'company-1',
      missionId: 'mission-1',
      taskId: 'task-1',
      manifestDigest: 'c'.repeat(64),
      deliveryStatus: 'not_delivered',
      payloadDigest: null,
      includedInputIds: [],
      includedInputPaths: [],
      inputExclusions: [],
      manifest: {
        schemaVersion: 'polis-model-input-manifest@1',
        companyId: 'company-1',
        missionId: 'mission-1',
        taskId: 'task-1',
        candidateInputs: [{inputId: 'input-1', revision: 2, requestId: 'upload-2', sourceKind: 'upload', displayName: 'goal.md', mediaType: 'text/markdown', byteSize: 8, contentDigest: 'd'.repeat(64), state: 'usable'}],
        excludedInputs: [],
        deliveryStatus: 'not_delivered',
      },
      createdAt: '2026-09-23T00:00:00Z',
    };
    const fetchMock = vi.fn().mockResolvedValue({ok: true, json: async () => expected});
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    const actual = await api.getTaskInputManifest({companyId: 'company-1', taskId: 'task-1'});

    expect(actual).toEqual(expected);
    expect(fetchMock).toHaveBeenCalledWith('/api/workbench/companies/company-1/tasks/task-1/input-manifest', expect.objectContaining({credentials: 'same-origin'}));
  });

  it('surfaces provider delivery evidence separately from the immutable manifest snapshot', async () => {
    const expected = {
      companyId: 'company-1', missionId: 'mission-1', taskId: 'task-1', manifestDigest: 'c'.repeat(64),
      deliveryStatus: 'provider_delivered',
      payloadDigest: 'e'.repeat(64),
      includedInputIds: ['input-1'],
      includedInputPaths: [{inputId: 'input-1', relativePath: '', mediaType: 'text/markdown', byteSize: 8, contentDigest: 'd'.repeat(64)}],
      inputExclusions: [],
      manifest: {
        schemaVersion: 'polis-model-input-manifest@1', companyId: 'company-1', missionId: 'mission-1', taskId: 'task-1',
        candidateInputs: [{inputId: 'input-1', revision: 1, requestId: 'upload-1', sourceKind: 'upload', displayName: 'goal.md', mediaType: 'text/markdown', byteSize: 8, contentDigest: 'd'.repeat(64), state: 'usable'}],
        excludedInputs: [], deliveryStatus: 'not_delivered',
      },
      createdAt: '2026-09-24T00:00:00Z',
    };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ok: true, json: async () => expected}));
    const api = new RealWorkbenchApi('/api/workbench');

    const actual = await api.getTaskInputManifest({companyId: 'company-1', taskId: 'task-1'});

    expect(actual.deliveryStatus).toBe('provider_delivered');
    expect(actual.manifest.deliveryStatus).toBe('not_delivered');
  });

  it('rejects a cross-mission input response', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => [{inputId: 'input-1', companyId: 'company-1', missionId: 'mission-other', requestId: 'upload-1', revision: '1', sourceKind: 'upload', displayName: 'goal.md', mediaType: 'text/markdown', byteSize: '7', contentDigest: 'a'.repeat(64), state: 'usable', imageWidth: '0', imageHeight: '0', createdAt: '2026-09-23T00:00:00Z'}],
    });
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');

    await expect(api.listMissionInputs({companyId: 'company-1', missionId: 'mission-1'})).rejects.toThrow('mission inputs');
  });

  it('rejects oversized files before making a request', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');
    const file = new File([new Uint8Array(8 * 1024 * 1024 + 1)], 'large.txt', {type: 'text/plain'});

    await expect(api.uploadMissionInput({companyId: 'company-1', missionId: 'mission-1', inputId: null, requestId: 'upload-2', file})).rejects.toThrow('8 MB');
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('rejects oversized directories before making a request', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    const api = new RealWorkbenchApi('/api/workbench');
    const large = new File([new Uint8Array(7 * 1024 * 1024 + 1)], 'large.txt', {type: 'text/plain'});

    await expect(api.uploadMissionDirectoryInput({
      companyId: 'company-1', missionId: 'mission-1', inputId: null, requestId: 'directory-2',
      files: [{relativePath: 'project/large.txt', file: large}],
    })).rejects.toThrow('7 MB');
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
