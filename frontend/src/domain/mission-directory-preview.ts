// pattern: Functional Core

export const MAX_MISSION_INPUT_BYTES = 8 * 1024 * 1024;
export const MAX_MISSION_DIRECTORY_BYTES = 7 * 1024 * 1024;
export const MAX_MISSION_DIRECTORY_FILES = 250;

export type MissionDirectorySelectionFile = Readonly<{
  relativePath: string;
  byteSize: number;
}>;

export type MissionDirectorySelectionIssue =
  | 'no_files'
  | 'too_many_files'
  | 'invalid_file_size'
  | 'directory_too_large'
  | 'invalid_path'
  | 'reserved_path'
  | 'root_mismatch'
  | 'path_collision'
  | 'file_parent_collision';

export type MissionDirectoryPreviewEntry = MissionDirectorySelectionFile & Readonly<{
  sensitiveHint: boolean;
}>;

export type MissionDirectorySelectionPreview = Readonly<{
  entries: ReadonlyArray<MissionDirectoryPreviewEntry>;
  totalBytes: number;
  issues: ReadonlyArray<MissionDirectorySelectionIssue>;
  valid: boolean;
}>;

const WINDOWS_RESERVED_NAMES = new Set(['CON', 'PRN', 'AUX', 'NUL']);
const WINDOWS_FORBIDDEN_CHARACTERS = /[<>:"|?*]/u;
const SENSITIVE_DIRECTORIES = new Set(['.ssh', '.aws', '.azure', '.kube', '.docker']);
const SENSITIVE_EXTENSIONS = ['.pem', '.key', '.p12', '.pfx', '.p8'];

/** Summarizes user-selected directory entries without reading file contents. */
export function previewMissionDirectorySelection(files: ReadonlyArray<MissionDirectorySelectionFile>): MissionDirectorySelectionPreview {
  const issues = new Set<MissionDirectorySelectionIssue>();
  const entries: Array<MissionDirectoryPreviewEntry> = [];
  const validPaths: Array<Readonly<{path: string; root: string}>> = [];
  let totalBytes = 0;

  if (files.length === 0) issues.add('no_files');
  if (files.length > MAX_MISSION_DIRECTORY_FILES) issues.add('too_many_files');

  for (const file of files) {
    if (!Number.isSafeInteger(file.byteSize) || file.byteSize <= 0 || file.byteSize > MAX_MISSION_INPUT_BYTES) {
      issues.add('invalid_file_size');
    } else {
      totalBytes += file.byteSize;
    }
    const parsedPath = validateSelectedDirectoryPath(file.relativePath);
    if (parsedPath === null) {
      issues.add('invalid_path');
    } else {
      if (parsedPath.reserved) issues.add('reserved_path');
      if (!parsedPath.reserved) validPaths.push({path: parsedPath.path.toLowerCase(), root: parsedPath.root});
    }
    entries.push({...file, sensitiveHint: isSensitiveMissionDirectoryPath(file.relativePath)});
  }

  if (totalBytes > MAX_MISSION_DIRECTORY_BYTES) issues.add('directory_too_large');
  if (validPaths.length > 0 && validPaths.some(file => file.root !== validPaths[0]?.root)) issues.add('root_mismatch');
  for (let index = 0; index < validPaths.length; index += 1) {
    const current = validPaths[index];
    if (current === undefined) continue;
    for (let priorIndex = 0; priorIndex < index; priorIndex += 1) {
      const prior = validPaths[priorIndex];
      if (prior === undefined) continue;
      if (current.path === prior.path) issues.add('path_collision');
      else if (current.path.startsWith(`${prior.path}/`) || prior.path.startsWith(`${current.path}/`)) issues.add('file_parent_collision');
    }
  }

  const issueOrder: ReadonlyArray<MissionDirectorySelectionIssue> = [
    'no_files', 'too_many_files', 'invalid_file_size', 'directory_too_large', 'invalid_path',
    'reserved_path', 'root_mismatch', 'path_collision', 'file_parent_collision',
  ];
  const orderedIssues = issueOrder.filter(issue => issues.has(issue));
  return {entries, totalBytes, issues: orderedIssues, valid: orderedIssues.length === 0};
}

/** Flags common credential/key locations from names only; no file bytes are read. */
export function isSensitiveMissionDirectoryPath(relativePath: string): boolean {
  const parts = relativePath.split('/').map(part => part.toLowerCase());
  const baseName = parts.at(-1) ?? '';
  if (baseName === '.env' || (baseName.startsWith('.env.') && baseName !== '.env.example' && baseName !== '.env.sample') ||
    baseName === '.npmrc' || baseName === '.netrc' || baseName === '.git-credentials' ||
    baseName === 'id_rsa' || baseName.startsWith('id_rsa.') || baseName === 'id_ed25519' || baseName.startsWith('id_ed25519.') ||
    baseName.includes('secret') || baseName.includes('credential')) {
    return true;
  }
  const extension = baseName.slice(baseName.lastIndexOf('.'));
  if (SENSITIVE_EXTENSIONS.includes(extension) || parts.some(part => SENSITIVE_DIRECTORIES.has(part))) return true;
  return parts.some(part => part === '.git') && baseName === 'config';
}

function validateSelectedDirectoryPath(value: string): Readonly<{path: string; root: string; reserved: boolean}> | null {
  if (value.length === 0 || new TextEncoder().encode(value).byteLength > 1024 || value.startsWith('/') || value.includes('\\') || containsControlCharacters(value)) return null;
  const parts = value.split('/');
  if (parts.length < 2 || parts.some(part => part === '' || part === '.' || part === '..' || part.endsWith('.') || part.endsWith(' ') || WINDOWS_FORBIDDEN_CHARACTERS.test(part))) return null;
  const reserved = parts.some(part => {
    const extensionStart = part.lastIndexOf('.');
    const baseName = (extensionStart < 0 ? part : part.slice(0, extensionStart)).toUpperCase();
    return WINDOWS_RESERVED_NAMES.has(baseName) || /^(COM|LPT)[1-9]$/u.test(baseName);
  });
  return {path: value, root: parts[0] ?? '', reserved};
}

function containsControlCharacters(value: string): boolean {
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index);
    if (code <= 0x1f || (code >= 0x7f && code <= 0x9f)) return true;
  }
  return false;
}
