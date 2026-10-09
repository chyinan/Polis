// pattern: Functional Core
import {describe, expect, it} from 'vitest';
import {validateArtifactDetail} from './workbench-validation';

function patchArtifact() {
  const sha = 'a'.repeat(64);
  return {artifactId: 'artifact', taskId: 'task', digest: sha, bytes: '20000', state: 'ready', verdict: 'passed', content: 'Patch bundle', contentAvailable: true, patch: {
    baseCommitOid: 'b'.repeat(40), inputSha256: sha, baseSnapshotSha256: sha, candidateSha256: sha, candidateRevision: '2', patchSha256: sha,
    jobRunId: 'job-fixed', resultSha256: sha, profile: 'polis-go-test-fixture@1', runnerSha256: sha, logSha256: sha,
    diff: '+ candidate change', diffTruncated: false, files: [{path: 'formatter.go', action: 'change'}], scope: 'Fixed test set only', replayVerified: true, automaticApply: false,
  }};
}

describe('Go patch read boundary', () => {
  it('accepts a bounded digest-matched patch read model', () => {
    expect(validateArtifactDetail(patchArtifact()).success).toBe(true);
  });
  it('rejects claims of automatic source application', () => {
    const artifact = patchArtifact();
    artifact.patch.automaticApply = true;
    expect(validateArtifactDetail(artifact).success).toBe(false);
  });
  it('rejects a patch digest that differs from the Artifact', () => {
    const artifact = patchArtifact();
    artifact.patch.patchSha256 = 'f'.repeat(64);
    expect(validateArtifactDetail(artifact).success).toBe(false);
  });
  it('rejects an unbounded diff or escaped path', () => {
    const artifact = patchArtifact();
    artifact.patch.diff = 'x'.repeat(65537);
    expect(validateArtifactDetail(artifact).success).toBe(false);
    artifact.patch.diff = 'bounded'; artifact.patch.files = [{path: '../secret.go', action: 'change'}];
    expect(validateArtifactDetail(artifact).success).toBe(false);
  });
});
