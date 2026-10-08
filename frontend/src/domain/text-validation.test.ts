import {describe, expect, it} from 'vitest';
import {containsC0OrDelControlCharacters} from './text-validation';

describe('containsC0OrDelControlCharacters', () => {
  it('accepts ordinary text and non-ASCII text', () => {
    expect(containsC0OrDelControlCharacters('workspace/文件.go')).toBe(false);
  });

  it.each(['\u0000', '\u001f', '\u007f'])('rejects control code %j', character => {
    expect(containsC0OrDelControlCharacters(`before${character}after`)).toBe(true);
  });
});
