// pattern: Functional Core
import {createElement} from 'react';
import {renderToStaticMarkup} from 'react-dom/server';
import {expect, it} from 'vitest';
import type {GoPatchView} from '../../domain/go-patch';
import {GoPatchPreview} from './GoPatchPreview';

it('shows bounded diff, identities and decision-only scope without executing anything', () => {
  // Synthetic presentation data only; no JobRun or qualification is created.
  const sha = 'a'.repeat(64);
  const patch: GoPatchView = {baseCommitOid: 'b'.repeat(40), inputSha256: sha, baseSnapshotSha256: sha, candidateSha256: sha, candidateRevision: '2', patchSha256: sha, jobRunId: 'job-fixed', resultSha256: sha, profile: 'polis-go-test-fixture@1', runnerSha256: sha, logSha256: sha, diff: '<script>not executable</script>', diffTruncated: true, files: [{path: 'formatter.go', action: 'change'}], scope: 'Fixed tests only', replayVerified: true, automaticApply: false};
  const markup = renderToStaticMarkup(createElement(GoPatchPreview, {patch, logText: 'bounded test log'}));
  expect(markup).toContain('formatter.go');
  expect(markup).toContain('job-fixed');
  expect(markup).toContain('不会自动应用');
  expect(markup).toContain('差异已截断');
  expect(markup).toContain('bounded test log');
  expect(markup).not.toContain('<script>');
});
