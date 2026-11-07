// pattern: Functional Core

import {describe, expect, it} from 'vitest';
import {labelActivityKind, labelActivityText, labelDataMode, labelDisplayValue, labelErrorMessage, labelRole, labelTransport} from './display-labels';

describe('display labels', () => {
  it('translates backend states without changing unknown identifiers', () => {
    expect(labelDisplayValue('ready')).toBe('已就绪');
    expect(labelDisplayValue('provider-x')).toBe('provider-x');
    expect(labelDataMode('real')).toBe('真实数据');
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
