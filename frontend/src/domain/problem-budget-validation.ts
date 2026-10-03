import type {ProblemToolCallAllocationReceiptView, ProblemToolCallBudgetListView, ProblemToolCallBudgetView, ProblemToolCallClosingReserveReceiptView, TaskToolCallAllocationReceiptView, TaskToolCallBudgetView, TaskToolCallIncompleteClosureReceiptView} from './workbench';

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isCount(value: unknown): value is number {
  return Number.isSafeInteger(value) && Number(value) >= 0;
}

function isTaskBudget(value: unknown): value is TaskToolCallBudgetView {
  if (!isRecord(value) || typeof value.taskId !== 'string' || !/^[A-Za-z0-9_-]{1,80}$/.test(value.taskId)
    || typeof value.kind !== 'string' || !isCount(value.toolCallsUsed)
    || typeof value.toolCallsRemaining !== 'number' || !Number.isSafeInteger(value.toolCallsRemaining) || value.toolCallsRemaining < -1
    || !Number.isSafeInteger(value.allocationRevision) || Number(value.allocationRevision) < 1
    || typeof value.allocationEligible !== 'boolean' || !isCount(value.budgetRejectionCount)
    || typeof value.closedIncomplete !== 'boolean' || typeof value.closureEligible !== 'boolean') return false;
  if (value.toolCallLimit !== null && !isCount(value.toolCallLimit)) return false;
  if (value.lastAllocationReason !== undefined && typeof value.lastAllocationReason !== 'string') return false;
  if (value.lastAllocatedAt !== undefined && typeof value.lastAllocatedAt !== 'string') return false;
  if (value.lastRejectionReason !== undefined && !['session_limit', 'task_limit', 'problem_limit', 'closing_reserve', 'initial_closing_reserve'].includes(String(value.lastRejectionReason))) return false;
  if (value.lastRejectionAt !== undefined && typeof value.lastRejectionAt !== 'string') return false;
  if (value.closureReason !== undefined && typeof value.closureReason !== 'string') return false;
  if (value.closedAt !== undefined && typeof value.closedAt !== 'string') return false;
  if (Number(value.budgetRejectionCount) === 0 && (value.lastRejectionReason !== undefined || value.lastRejectionAt !== undefined)) return false;
  if (Number(value.budgetRejectionCount) > 0 && (value.lastRejectionReason === undefined || typeof value.lastRejectionAt !== 'string')) return false;
  if (value.closedIncomplete && (typeof value.closureReason !== 'string' || value.closureReason.trim() === '' || typeof value.closedAt !== 'string' || value.closureEligible || value.allocationEligible)) return false;
  if (!value.closedIncomplete && (value.closureReason !== undefined || value.closedAt !== undefined)) return false;
  if (value.closureEligible && (Number(value.budgetRejectionCount) === 0 || value.closedIncomplete || value.lastRejectionReason === 'session_limit')) return false;
  if (value.toolCallLimit === null) return value.toolCallsRemaining === 0;
  if (value.toolCallLimit === 0) return value.toolCallsRemaining === -1;
  return value.toolCallsRemaining === Math.max(value.toolCallLimit - value.toolCallsUsed, 0);
}

function isProblemBudget(value: unknown): value is ProblemToolCallBudgetView {
  if (!isRecord(value) || typeof value.problemKey !== 'string' || !/^problem:[A-Za-z0-9_-]{1,80}$/.test(value.problemKey)
    || typeof value.missionId !== 'string' || !/^[A-Za-z0-9_-]{1,80}$/.test(value.missionId)
    || !isCount(value.taskCount) || !isCount(value.workerSessionAttempts) || !isCount(value.toolCallsUsed)
    || typeof value.toolCallsRemaining !== 'number' || !Number.isSafeInteger(value.toolCallsRemaining)
    || !Number.isSafeInteger(value.allocationRevision) || Number(value.allocationRevision) < 1
    || !isCount(value.closingReserveToolCalls) || !isCount(value.closingReserveRemaining)
    || !isCount(value.closingReserveRevision)
    || !isCount(value.budgetRejectionCount)
    || typeof value.tasksTruncated !== 'boolean' || !Array.isArray(value.tasks) || value.tasks.length > 20 || !value.tasks.every(isTaskBudget)
    || !['pending', 'unbounded', 'available', 'closing_reserved', 'exhausted'].includes(String(value.state))) return false;
  if (value.toolCallLimit !== null && !isCount(value.toolCallLimit)) return false;
  if (value.lastAllocationReason !== undefined && typeof value.lastAllocationReason !== 'string') return false;
  if (value.lastAllocatedAt !== undefined && typeof value.lastAllocatedAt !== 'string') return false;
  if (value.lastClosingReserveReason !== undefined && typeof value.lastClosingReserveReason !== 'string') return false;
  if (value.lastClosingReserveAt !== undefined && typeof value.lastClosingReserveAt !== 'string') return false;
  if (value.lastRejectionAt !== undefined && typeof value.lastRejectionAt !== 'string') return false;
  if (value.lastRejectionRoute !== undefined && !['worker_admission', 'worker_tool_call'].includes(String(value.lastRejectionRoute))) return false;
  if (value.lastRejectionReason !== undefined && !['session_limit', 'task_limit', 'problem_limit', 'closing_reserve', 'initial_closing_reserve'].includes(String(value.lastRejectionReason))) return false;
  if (value.lastRejectionTaskId !== undefined && (typeof value.lastRejectionTaskId !== 'string' || !/^[A-Za-z0-9_-]{1,80}$/.test(value.lastRejectionTaskId))) return false;
  if (Number(value.budgetRejectionCount) === 0 && (value.lastRejectionAt !== undefined || value.lastRejectionRoute !== undefined || value.lastRejectionReason !== undefined || value.lastRejectionTaskId !== undefined)) return false;
  if (Number(value.budgetRejectionCount) > 0 && (typeof value.lastRejectionAt !== 'string' || value.lastRejectionRoute === undefined || value.lastRejectionReason === undefined || value.lastRejectionTaskId === undefined)) return false;
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

export function validateTaskToolCallAllocationReceipt(value: unknown, taskId: string, revision: number): TaskToolCallAllocationReceiptView {
  if (!isRecord(value) || value.id !== taskId || value.status !== 'task_budget_allocated' || value.revision !== revision) {
    throw new Error('failed to parse Task budget allocation receipt');
  }
  return value as unknown as TaskToolCallAllocationReceiptView;
}

export function validateTaskToolCallIncompleteClosureReceipt(value: unknown, taskId: string): TaskToolCallIncompleteClosureReceiptView {
  if (!isRecord(value) || value.id !== taskId || value.status !== 'closed_incomplete' || value.revision !== 1) {
    throw new Error('failed to parse Task incomplete budget closure receipt');
  }
  return value as unknown as TaskToolCallIncompleteClosureReceiptView;
}
