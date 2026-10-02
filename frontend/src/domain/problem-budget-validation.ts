import type {ProblemToolCallAllocationReceiptView, ProblemToolCallBudgetListView, ProblemToolCallBudgetView, ProblemToolCallClosingReserveReceiptView} from './workbench';

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isCount(value: unknown): value is number {
  return Number.isSafeInteger(value) && Number(value) >= 0;
}

function isProblemBudget(value: unknown): value is ProblemToolCallBudgetView {
  if (!isRecord(value) || typeof value.problemKey !== 'string' || !/^problem:[A-Za-z0-9_-]{1,80}$/.test(value.problemKey)
    || typeof value.missionId !== 'string' || !/^[A-Za-z0-9_-]{1,80}$/.test(value.missionId)
    || !isCount(value.taskCount) || !isCount(value.workerSessionAttempts) || !isCount(value.toolCallsUsed)
    || typeof value.toolCallsRemaining !== 'number' || !Number.isSafeInteger(value.toolCallsRemaining)
    || !Number.isSafeInteger(value.allocationRevision) || Number(value.allocationRevision) < 1
    || !isCount(value.closingReserveToolCalls) || !isCount(value.closingReserveRemaining)
    || !isCount(value.closingReserveRevision)
    || !['pending', 'unbounded', 'available', 'closing_reserved', 'exhausted'].includes(String(value.state))) return false;
  if (value.toolCallLimit !== null && !isCount(value.toolCallLimit)) return false;
  if (value.lastAllocationReason !== undefined && typeof value.lastAllocationReason !== 'string') return false;
  if (value.lastAllocatedAt !== undefined && typeof value.lastAllocatedAt !== 'string') return false;
  if (value.lastClosingReserveReason !== undefined && typeof value.lastClosingReserveReason !== 'string') return false;
  if (value.lastClosingReserveAt !== undefined && typeof value.lastClosingReserveAt !== 'string') return false;
  if (Number(value.closingReserveRemaining) > Number(value.closingReserveToolCalls)) return false;
  if (value.state === 'pending') return value.toolCallLimit === null
    && value.toolCallsRemaining === 0;
  if (typeof value.toolCallLimit !== 'number') return false;
  if (value.state === 'unbounded') return value.toolCallLimit === 0 && value.toolCallsRemaining === -1
    && value.closingReserveToolCalls === 0 && value.closingReserveRemaining === 0;
  if (value.state === 'available') return value.toolCallLimit > 0 && value.toolCallsRemaining === value.toolCallLimit - value.toolCallsUsed
    && value.toolCallsRemaining > value.closingReserveRemaining;
  if (value.state === 'closing_reserved') return value.toolCallLimit > 0 && value.toolCallsRemaining > 0
    && value.toolCallsRemaining === value.toolCallLimit - value.toolCallsUsed
    && value.toolCallsRemaining <= value.closingReserveRemaining;
  return value.toolCallLimit > 0 && value.toolCallsRemaining === 0 && value.toolCallsUsed >= value.toolCallLimit;
}

export function validateProblemToolCallBudgetList(value: unknown): ProblemToolCallBudgetListView {
  if (!isRecord(value) || !Array.isArray(value.items) || typeof value.truncated !== 'boolean' || !value.items.every(isProblemBudget)) {
    throw new Error('failed to parse ProblemKey budget response');
  }
  return value as unknown as ProblemToolCallBudgetListView;
}

export function validateProblemToolCallAllocationReceipt(value: unknown, problemKey: string, revision: number): ProblemToolCallAllocationReceiptView {
  if (!isRecord(value) || value.id !== problemKey || value.status !== 'allocated' || value.revision !== revision) {
    throw new Error('failed to parse ProblemKey budget allocation receipt');
  }
  return value as unknown as ProblemToolCallAllocationReceiptView;
}

export function validateProblemToolCallClosingReserveReceipt(value: unknown, problemKey: string, revision: number): ProblemToolCallClosingReserveReceiptView {
  if (!isRecord(value) || value.id !== problemKey || value.status !== 'reserve_updated' || value.revision !== revision) {
    throw new Error('failed to parse ProblemKey closing reserve receipt');
  }
  return value as unknown as ProblemToolCallClosingReserveReceiptView;
}
