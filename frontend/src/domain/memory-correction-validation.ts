import type {MemoryCorrectionCommandReceiptView, MemoryCorrectionQueueView, MemoryCorrectionQueueItemView} from './workbench';

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isSource(value: unknown): boolean {
  return isObject(value) && typeof value.kind === 'string' && typeof value.id === 'string'
    && Number.isSafeInteger(value.revision) && (value.revision as number) > 0
    && typeof value.sha256 === 'string' && /^[0-9a-f]{64}$/.test(value.sha256);
}

function isItem(value: unknown): value is MemoryCorrectionQueueItemView {
  if (!isObject(value) || !['proposed', 'approved', 'rejected', 'stale', 'revoked'].includes(String(value.state))) return false;
  return typeof value.correctionId === 'string' && typeof value.recordId === 'string'
    && typeof value.recordKind === 'string' && typeof value.recordScope === 'string'
    && typeof value.sensitivity === 'string' && Number.isSafeInteger(value.baseRevision)
    && Number.isSafeInteger(value.currentRevision) && typeof value.currentState === 'string'
    && isSource(value.source) && typeof value.observedAt === 'string'
    && typeof value.proposedBy === 'string'
    && typeof value.proposedAt === 'string';
}

export function validateMemoryCorrectionQueue(value: unknown): MemoryCorrectionQueueView {
  if (!isObject(value) || !Array.isArray(value.items) || typeof value.truncated !== 'boolean'
    || !value.items.every(isItem)) {
    throw new Error('failed to parse memory correction queue response');
  }
  return value as unknown as MemoryCorrectionQueueView;
}

export function validateMemoryCorrectionCommandReceipt(value: unknown): MemoryCorrectionCommandReceiptView {
  if (!isObject(value) || typeof value.id !== 'string' || typeof value.status !== 'string'
    || (value.revision !== undefined && (!Number.isSafeInteger(value.revision) || (value.revision as number) < 0))) {
    throw new Error('failed to parse memory correction command receipt');
  }
  return value as unknown as MemoryCorrectionCommandReceiptView;
}
