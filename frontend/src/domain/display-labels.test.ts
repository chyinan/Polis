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

  it('labels dependency materialization states without implying package installation', () => {
    expect(labelDisplayValue('materialized')).toBe('已生成后继环境版本');
    expect(labelDisplayValue('resolver_outcome_unknown')).toBe('锁文件解析结果未知');
    expect(labelDisplayValue('runtime_restarted_during_resolution')).toBe('解析期间服务重启；不会自动重放');
  });

  it('labels fixed-role reconciliation outcomes without implying qualification', () => {
    expect(labelDisplayValue('owner_reconciliation_required')).toBe('等待安装负责人审阅固定岗位差异');
    expect(labelDisplayValue('active_work_present')).toBe('仍有活动或未决工作');
    expect(labelDisplayValue('reconciled')).toBe('固定岗位已恢复');
    expect(labelDisplayValue('unverified')).toBe('未验证');
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
