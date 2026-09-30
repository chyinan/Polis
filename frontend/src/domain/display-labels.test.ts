// pattern: Functional Core

import {describe, expect, it} from 'vitest';
import {labelActivityKind, labelActivityText, labelDataMode, labelDisplayValue, labelErrorMessage, labelProjectEnvironmentPolicy, labelRole, labelTransport} from './display-labels';

describe('display labels', () => {
  it('translates backend states without changing unknown identifiers', () => {
    expect(labelDisplayValue('ready')).toBe('已就绪');
    expect(labelDisplayValue('provider-x')).toBe('provider-x');
    expect(labelDataMode('real')).toBe('真实数据');
  });

  it('labels environment policy and isolation gates explicitly', () => {
    expect(labelDisplayValue('unverified')).toBe('未验证');
    expect(labelDisplayValue('revocation_required')).toBe('需要撤销旧批准');
    expect(labelDisplayValue('blocked_policy')).toBe('等待策略批准');
    expect(labelDisplayValue('blocked_unqualified')).toBe('等待隔离执行资格');
  });

  it('distinguishes Linux deny-all and Windows registry policies', () => {
    const linux = {schemaVersion: 'project-environment-policy@1', profileId: 'linux-node-npm@1', registryHosts: ['registry.npmjs.org'], installPolicy: 'npm ci --ignore-scripts --no-audit --no-fund --offline', lifecycleScriptsPolicy: 'ignore', networkPolicy: 'deny_all', timeoutMs: 180000, outputLimitBytes: 1048576} as const;
    const windows = {schemaVersion: 'project-environment-policy@1', profileId: 'windows-node-npm@1', registryHosts: ['registry.npmjs.org'], installPolicy: 'npm ci --ignore-scripts --no-audit --no-fund', lifecycleScriptsPolicy: 'ignore', networkPolicy: 'registry_allowlist', timeoutMs: 180000, outputLimitBytes: 1048576} as const;

    expect(labelProjectEnvironmentPolicy(linux)).toBe('Linux/Node · deny_all · registry metadata registry.npmjs.org');
    expect(labelProjectEnvironmentPolicy(windows)).toBe('Windows/Node · Registry registry.npmjs.org');
  });

  it('translates activity protocol names and role values', () => {
    expect(labelActivityKind('workspace_updated')).toBe('工作区已更新');
    expect(labelActivityText('workspace_updated · controller.recovered')).toBe('工作区已更新 · 控制器已恢复');
    expect(labelRole('backend')).toBe('后端');
    expect(labelTransport('streamable_http')).toBe('流式 HTTP');
  });

  it('translates known runtime failures for the user-facing diagnostics surface', () => {
    expect(labelErrorMessage('failed to load companies')).toBe('读取公司目录失败');
    expect(labelErrorMessage('unrelated diagnostic')).toBe('unrelated diagnostic');
  });
});
