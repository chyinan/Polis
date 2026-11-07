// pattern: Functional Core

import type {ArtifactSummary, CheckpointSummary} from './workbench';

export function selectCheckpointForTask(checkpoints: ReadonlyArray<CheckpointSummary>, taskId: string, artifact: ArtifactSummary | null = null): CheckpointSummary | null {
  if (artifact?.taskId === taskId && artifact.checkpointId !== null) {
    const artifactCheckpoint = checkpoints.find(checkpoint => checkpoint.taskId === taskId && checkpoint.checkpointId === artifact.checkpointId);
    if (artifactCheckpoint !== undefined) {
      return artifactCheckpoint;
    }
  }
  return checkpoints.find(checkpoint => checkpoint.taskId === taskId) ?? null;
}
