// pattern: Functional Core

import {describe, expect, it} from 'vitest';
import {isSensitiveMissionDirectoryPath, MAX_MISSION_DIRECTORY_BYTES, MAX_MISSION_DIRECTORY_FILES, previewMissionDirectorySelection} from './mission-directory-preview';

describe('previewMissionDirectorySelection', () => {
  it('summarizes user-selected files and flags sensitive names without reading contents', () => {
    const preview = previewMissionDirectorySelection([
      {relativePath: 'project/README.md', byteSize: 80},
      {relativePath: 'project/.env', byteSize: 12},
      {relativePath: 'project/src/main.go', byteSize: 32},
    ]);

    expect(preview.valid).toBe(true);
    expect(preview.totalBytes).toBe(124);
    expect(preview.entries.map(entry => entry.sensitiveHint)).toEqual([false, true, false]);
    expect(preview.issues).toEqual([]);
  });

  it('rejects empty, oversized, invalid, reserved, and colliding selections', () => {
    expect(previewMissionDirectorySelection([]).issues).toContain('no_files');
    expect(previewMissionDirectorySelection([{relativePath: 'project/large.bin', byteSize: MAX_MISSION_DIRECTORY_BYTES + 1}]).issues).toContain('directory_too_large');
    expect(previewMissionDirectorySelection([{relativePath: 'project/../outside.txt', byteSize: 1}]).issues).toContain('invalid_path');
    expect(previewMissionDirectorySelection([{relativePath: 'project/CON.txt', byteSize: 1}]).issues).toContain('reserved_path');
    expect(previewMissionDirectorySelection([
      {relativePath: 'project/Readme.md', byteSize: 1},
      {relativePath: 'project/readme.md', byteSize: 1},
    ]).issues).toContain('path_collision');
    expect(previewMissionDirectorySelection([
      {relativePath: 'project/src', byteSize: 1},
      {relativePath: 'project/src/main.go', byteSize: 1},
    ]).issues).toContain('file_parent_collision');
    expect(previewMissionDirectorySelection([
      {relativePath: 'project/a.txt', byteSize: 1},
      {relativePath: 'other/b.txt', byteSize: 1},
    ]).issues).toContain('root_mismatch');
  });

  it('enforces file-count and aggregate byte limits', () => {
    const tooMany = Array.from({length: MAX_MISSION_DIRECTORY_FILES + 1}, (_, index) => ({relativePath: `project/file-${index}.txt`, byteSize: 1}));
    expect(previewMissionDirectorySelection(tooMany).issues).toContain('too_many_files');
    expect(previewMissionDirectorySelection([
      {relativePath: 'project/first.bin', byteSize: MAX_MISSION_DIRECTORY_BYTES},
      {relativePath: 'project/second.bin', byteSize: 1},
    ]).issues).toContain('directory_too_large');
    expect(previewMissionDirectorySelection([{relativePath: 'project/empty.txt', byteSize: 0}]).issues).toContain('invalid_file_size');
  });
});

describe('isSensitiveMissionDirectoryPath', () => {
  it.each(['project/.env', 'project/.ssh/id_ed25519', 'project/certs/client.pem', 'project/.git/config', 'project/aws.credentials'])('flags likely sensitive paths: %s', path => {
    expect(isSensitiveMissionDirectoryPath(path)).toBe(true);
  });

  it.each(['project/.env.example', 'project/README.md', 'project/src/main.go'])('does not flag ordinary/example paths: %s', path => {
    expect(isSensitiveMissionDirectoryPath(path)).toBe(false);
  });
});
