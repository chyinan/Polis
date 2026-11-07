// pattern: Functional Core
import {describe, expect, it} from 'vitest';
import {archiveInvalidationKeys} from './workbench-query';

describe('archiveInvalidationKeys', () => {
  it('invalidates the company collection and archived overview', () => {
    expect(archiveInvalidationKeys('real', 'company-1')).toEqual([
      ['workbench', 'real', 'companies'],
      ['workbench', 'real', 'company-overview', 'company-1'],
    ]);
  });
});
