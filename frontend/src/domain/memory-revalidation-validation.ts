// pattern: Functional Core

import type {MemoryTaskImpactView, MemoryTaskRevalidationPreviewView, MemoryTaskRevalidationReceipt, MemoryTaskStatusView} from './workbench';
import type {ValidationIssue, ValidationResult} from './workbench-validation';

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isValidID(value: unknown): value is string {
  return typeof value === 'string' && /^[A-Za-z0-9_-]{1,80}$/.test(value);
}

function isDigest(value: unknown): value is string {
  return typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
}

function isImpact(value: unknown): value is MemoryTaskImpactView {
  return isRecord(value)
    && isValidID(value.dependencyId)
    && isValidID(value.recordId)
    && Number.isSafeInteger(value.recordRevision) && Number(value.recordRevision) > 0
    && Number.isSafeInteger(value.replacementRevision) && Number(value.replacementRevision) > 0
    && ['ordinary', 'high', 'critical'].includes(String(value.riskLevel))
    && ['dirty', 'frozen'].includes(String(value.state))
    && isValidID(value.correctionId)
    && typeof value.reason === 'string';
}

export function validateMemoryTaskStatus(value: unknown): ValidationResult<MemoryTaskStatusView> {
  if (!isRecord(value) || !['clear', 'dirty', 'frozen'].includes(String(value.state)) || !Array.isArray(value.impacts) || !value.impacts.every(isImpact)) {
    return {success: false, issues: [{path: 'memoryTaskStatus', message: 'memory impact response is malformed'}]};
  }
  const impacts = value.impacts as ReadonlyArray<MemoryTaskImpactView>;
  const expectedState = impacts.some(item => item.state === 'frozen') ? 'frozen' : impacts.length > 0 ? 'dirty' : 'clear';
  if (value.state !== expectedState) {
    return {success: false, issues: [{path: 'state', message: 'memory impact aggregate state does not match its entries'}]};
  }
  const seen = new Set<string>();
  for (const impact of impacts) {
    const key = `${impact.dependencyId}:${impact.correctionId}`;
    if (seen.has(key)) return {success: false, issues: [{path: 'impacts', message: 'memory impact response contains a duplicate dependency/correction'}]};
    seen.add(key);
  }
  return {success: true, value: value as unknown as MemoryTaskStatusView};
}

export function validateMemoryTaskRevalidationPreview(value: unknown, taskId: string, dependencyId: string, correctionId: string): ValidationResult<MemoryTaskRevalidationPreviewView> {
  const issues: Array<ValidationIssue> = [];
  if (!isRecord(value)) return {success: false, issues: [{path: '', message: 'memory revalidation preview is malformed'}]};
  const source = value.correctionSource;
  const validSource = isRecord(source) && typeof source.kind === 'string' && source.kind.length > 0 && source.kind.length <= 64
    && isValidID(source.id) && Number.isSafeInteger(source.revision) && Number(source.revision) > 0 && isDigest(source.sha256);
  if (value.taskId !== taskId || value.dependencyId !== dependencyId || value.correctionId !== correctionId) issues.push({path: 'scope', message: 'memory revalidation preview scope does not match request'});
  if (!['ready', 'working'].includes(String(value.taskState)) || !['active', 'paused'].includes(String(value.missionState))) issues.push({path: 'state', message: 'memory revalidation preview includes an ineligible Task or Mission'});
  if (!Number.isSafeInteger(value.taskGeneration) || Number(value.taskGeneration) < 0 || !isDigest(value.taskPlanSha256) || typeof value.taskPlan === 'undefined' || new TextEncoder().encode(JSON.stringify(value.taskPlan) ?? '').length > 64 * 1024) issues.push({path: 'taskPlan', message: 'Task plan binding is malformed'});
  if (!['dirty', 'frozen'].includes(String(value.dependencyState)) || !['ordinary', 'high', 'critical'].includes(String(value.riskLevel))) issues.push({path: 'dependency', message: 'memory dependency state is malformed'});
  if (!validSource || typeof value.correctionObservedAt !== 'string' || value.correctionObservedAt === '' || typeof value.correctionProposerReason !== 'string' || typeof value.correctionReviewReason !== 'string') issues.push({path: 'correction', message: 'approved correction evidence is malformed'});
  if (!isValidID(value.previousRecordId) || !Number.isSafeInteger(value.previousRevision) || Number(value.previousRevision) < 1 || !Number.isSafeInteger(value.replacementRevision) || Number(value.replacementRevision) <= Number(value.previousRevision) || typeof value.replacementContent !== 'string' || new TextEncoder().encode(value.replacementContent).length > 32768 || !isDigest(value.replacementContentSha256)) issues.push({path: 'replacement', message: 'replacement memory revision is malformed'});
  if (typeof value.targetKind !== 'string' || value.targetKind === '' || !isValidID(value.targetId) || !Number.isSafeInteger(value.targetRevision) || Number(value.targetRevision) < 1 || !isDigest(value.targetSha256)) issues.push({path: 'target', message: 'memory dependency target binding is malformed'});
  if (!isValidID(value.stoppedSessionId) || !isDigest(value.workspaceDigest) || !Number.isSafeInteger(value.workspaceRevision) || Number(value.workspaceRevision) < 1 || typeof value.workspaceContent !== 'string' || new TextEncoder().encode(value.workspaceContent).length > 128 * 1024 || !isDigest(value.contextSha256)) issues.push({path: 'workspace', message: 'stopped session or workspace binding is malformed'});
  if (value.otherImpacts !== undefined && (!Array.isArray(value.otherImpacts) || !value.otherImpacts.every(isImpact))) issues.push({path: 'otherImpacts', message: 'other memory impacts are malformed'});
  if (issues.length > 0) return {success: false, issues};
  return {success: true, value: {...value, otherImpacts: (value.otherImpacts ?? [])} as MemoryTaskRevalidationPreviewView};
}

export function validateMemoryTaskRevalidationReceipt(value: unknown): ValidationResult<MemoryTaskRevalidationReceipt> {
  if (!isRecord(value) || !isValidID(value.id) || value.status !== 'revalidated' || !Number.isSafeInteger(value.revision) || Number(value.revision) < 1) {
    return {success: false, issues: [{path: '', message: 'memory revalidation receipt is malformed'}]};
  }
  return {success: true, value: value as unknown as MemoryTaskRevalidationReceipt};
}
