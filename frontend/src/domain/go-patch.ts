// pattern: Functional Core

export type GoPatchView = Readonly<{
  baseCommitOid: string; inputSha256: string; baseSnapshotSha256: string;
  candidateSha256: string; candidateRevision: string; patchSha256: string;
  jobRunId: string; resultSha256: string; profile: 'polis-go-test-fixture@1';
  runnerSha256: string; logSha256: string; diff: string; diffTruncated: boolean;
  files: ReadonlyArray<Readonly<{path: string; action: 'add' | 'change' | 'delete'}>>;
  scope: string; replayVerified: true; automaticApply: false;
}>;

/** Validates a bounded read model; it does not confer execution qualification. */
export function isGoPatchView(value: unknown): value is GoPatchView {
  if (typeof value !== 'object' || value === null) return false;
  const item = value as Record<string, unknown>; // Checked unknown record; each field is validated below.
  const hashes = ['inputSha256', 'baseSnapshotSha256', 'candidateSha256', 'patchSha256', 'resultSha256', 'runnerSha256', 'logSha256'];
  if (!hashes.every(key => typeof item[key] === 'string' && /^[a-f0-9]{64}$/.test(item[key]))) return false;
  if (typeof item.baseCommitOid !== 'string' || !/^([a-f0-9]{40}|[a-f0-9]{64})$/.test(item.baseCommitOid)
    || typeof item.candidateRevision !== 'string' || !/^[1-9][0-9]{0,18}$/.test(item.candidateRevision)
    || typeof item.jobRunId !== 'string' || !/^[A-Za-z0-9_-]{1,80}$/.test(item.jobRunId)
    || item.profile !== 'polis-go-test-fixture@1' || item.replayVerified !== true || item.automaticApply !== false
    || typeof item.diff !== 'string' || new TextEncoder().encode(item.diff).byteLength > 65536 || typeof item.diffTruncated !== 'boolean'
    || typeof item.scope !== 'string' || item.scope.length < 1 || item.scope.length > 2048
    || !Array.isArray(item.files) || item.files.length < 1 || item.files.length > 512) return false;
  return item.files.every((file: unknown) => {
    if (typeof file !== 'object' || file === null || !('path' in file) || !('action' in file)) return false;
    return typeof file.path === 'string' && file.path.length <= 512 && !/[\\/:]/.test(file.path) && !Array.from(file.path).some(character => character.charCodeAt(0) < 32) && !file.path.startsWith('.')
      && (file.path === 'formatter.go' || file.path.endsWith('_test.go')) && typeof file.action === 'string' && ['add', 'change', 'delete'].includes(file.action);
  });
}
