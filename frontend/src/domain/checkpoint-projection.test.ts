// pattern: Functional Core

import {describe, expect, it} from 'vitest';
import {selectCheckpointForTask} from './checkpoint-projection';
import type {ArtifactSummary, CheckpointSummary} from './workbench';

const qualified: CheckpointSummary = {
  checkpointId: 'checkpoint-qualified',
  taskId: 'task-1',
  employeeId: 'emp-backend',
  sessionId: 'session-2',
  sessionState: 'stopped',
  sessionEpoch: '2',
  kind: 'qualified',
  state: 'current',
  qualificationState: 'qualified',
  workspaceRevision: '4',
  workspaceDigest: 'digest-4',
  artifactId: 'artifact-1',
};

const artifact: ArtifactSummary = {
  artifactId: 'artifact-1',
  taskId: 'task-1',
  authorEmployeeId: 'emp-backend',
  digest: 'digest-4',
  bytes: '514',
  state: 'ready',
  verdict: 'candidate',
  contractRevisionId: null,
  qualificationId: 'check-1',
  checkpointId: qualified.checkpointId,
};

describe('checkpoint read projection', () => {
  it('selects the authoritative task checkpoint by relation, never from activity events', () => {
    expect(selectCheckpointForTask([qualified], 'task-1', artifact)).toEqual(qualified);
    expect(selectCheckpointForTask([qualified], 'task-2', artifact)).toBeNull();
  });
});
