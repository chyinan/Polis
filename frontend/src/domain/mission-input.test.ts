import {describe, expect, it} from 'vitest';
import {isInputArchiveSource} from './mission-input';

describe('mission input archive classification', () => {
  it('treats directory, archive, and Git snapshots as multi-file inputs', () => {
    expect(isInputArchiveSource('directory_snapshot')).toBe(true);
    expect(isInputArchiveSource('zip_snapshot')).toBe(true);
    expect(isInputArchiveSource('pdf_snapshot')).toBe(true);
    expect(isInputArchiveSource('git_snapshot')).toBe(true);
    expect(isInputArchiveSource('upload')).toBe(false);
  });
});
