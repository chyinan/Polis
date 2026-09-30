// pattern: Functional Core

import {describe, expect, it} from 'vitest';
import {managementAccessForOrigins} from './management-access';

describe('management access presentation', () => {
  it.each(['tauri://localhost', 'http://localhost:4173', 'http://127.0.0.1:8080', 'http://[::1]:8080'])('labels %s as local management', origin => {
    expect(managementAccessForOrigins(origin, '/api/workbench')).toEqual({mode: 'local_management', label: '本机管理'});
  });

  it.each(['https://polis.example.com', 'http://192.168.1.30:8080', 'not a URL'])('keeps %s remote and unqualified', origin => {
    expect(managementAccessForOrigins(origin, '/api/workbench')).toEqual({mode: 'remote_management_unqualified', label: '远程管理 · 未验证'});
  });

  it('labels a local page with a remote API as remote and unqualified', () => {
    expect(managementAccessForOrigins('http://localhost:4173', 'https://polis.example.com/api/workbench'))
      .toEqual({mode: 'remote_management_unqualified', label: '远程管理 · 未验证'});
  });

  it('keeps a remote page unqualified when its API uses another origin', () => {
    expect(managementAccessForOrigins('https://polis.example.com', 'http://127.0.0.1:8080/api/workbench'))
      .toEqual({mode: 'remote_management_unqualified', label: '远程管理 · 未验证'});
  });
});
