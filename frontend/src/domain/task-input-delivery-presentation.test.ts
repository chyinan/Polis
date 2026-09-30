// pattern: Functional Core

import {describe, expect, it} from 'vitest';
import type {ModelInputDeliveryRef} from './mission-input';
import {countIncludedTaskInputImages, countIncludedTaskInputSources, countIncludedTaskInputTextFiles} from './task-input-delivery-presentation';

describe('task input delivery presentation', () => {
  it('counts distinct input sources when an archive contributes multiple text files', () => {
    const includedFiles: ReadonlyArray<ModelInputDeliveryRef> = [
      {inputId: 'archive-1', relativePath: 'project/README.md', mediaType: 'text/markdown', byteSize: 12, contentDigest: 'a'.repeat(64)},
      {inputId: 'archive-1', relativePath: 'project/src/main.ts', mediaType: 'text/plain', byteSize: 8, contentDigest: 'b'.repeat(64)},
      {inputId: 'text-1', relativePath: '', mediaType: 'text/plain', byteSize: 7, contentDigest: 'c'.repeat(64)},
    ];

    expect(countIncludedTaskInputSources(includedFiles)).toBe(2);
    expect(countIncludedTaskInputTextFiles(includedFiles)).toBe(3);
    expect(countIncludedTaskInputImages(includedFiles)).toBe(0);
  });

  it('counts Worker image inputs separately from text files', () => {
    const includedFiles: ReadonlyArray<ModelInputDeliveryRef> = [
      {inputId: 'archive-1', relativePath: 'project/README.md', mediaType: 'text/markdown', byteSize: 12, contentDigest: 'a'.repeat(64)},
      {inputId: 'archive-1', relativePath: 'project/screen.png', mediaType: 'image/png', byteSize: 100, contentDigest: 'b'.repeat(64)},
    ];

    expect(countIncludedTaskInputSources(includedFiles)).toBe(1);
    expect(countIncludedTaskInputTextFiles(includedFiles)).toBe(1);
    expect(countIncludedTaskInputImages(includedFiles)).toBe(1);
  });
});
