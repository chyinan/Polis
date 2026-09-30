// pattern: Functional Core

import type {DailyRoutineCommandReceipt, DailyRoutineView} from './workbench';
import type {ValidationIssue, ValidationResult} from './workbench-validation';

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isValidRoutineId(value: unknown): value is string {
  return typeof value === 'string' && /^[A-Za-z0-9_-]{1,80}$/.test(value);
}

function isLogicalDay(value: unknown): value is string {
  if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
  const parsed = new Date(`${value}T00:00:00Z`);
  return Number.isFinite(parsed.getTime()) && parsed.toISOString().slice(0, 10) === value;
}

function isDailyRoutineView(value: unknown, missionId: string): value is DailyRoutineView {
  if (!isRecord(value)) return false;
  const validInstruction = value.taskInstruction === null
    || (typeof value.taskInstruction === 'string' && value.taskInstruction.trim() === value.taskInstruction && value.taskInstruction.length > 0 && new TextEncoder().encode(value.taskInstruction).length <= 4096);
  return isValidRoutineId(value.routineId)
    && value.missionId === missionId
    && isValidRoutineId(value.employeeId)
    && typeof value.timezone === 'string'
    && value.timezone.length > 0
    && value.timezone.length <= 128
    && typeof value.localTime === 'string'
    && /^([01][0-9]|2[0-3]):[0-5][0-9]$/.test(value.localTime)
    && isLogicalDay(value.nextLogicalDay)
    && ['skip', 'coalesce_latest', 'catch_up'].includes(String(value.catchUpPolicy))
    && Number.isInteger(value.maxCatchUp)
    && Number(value.maxCatchUp) >= 1
    && Number(value.maxCatchUp) <= 10
    && (value.nextDueAt === null || (typeof value.nextDueAt === 'string' && Number.isFinite(Date.parse(value.nextDueAt))))
    && validInstruction
    && Number.isSafeInteger(value.needsInstructionOccurrences)
    && Number(value.needsInstructionOccurrences) >= 0
    && Number.isSafeInteger(value.linkedTaskCount)
    && Number(value.linkedTaskCount) >= 0;
}

export function validateDailyRoutines(value: unknown, missionId: string): ValidationResult<ReadonlyArray<DailyRoutineView>> {
  if (!Array.isArray(value)) {
    return {success: false, issues: [{path: '', message: 'daily Routine response is not an array'}]};
  }
  const issues: Array<ValidationIssue> = [];
  const seen = new Set<string>();
  value.forEach((item, index) => {
    if (!isDailyRoutineView(item, missionId)) {
      issues.push({path: String(index), message: 'daily Routine contains malformed or cross-Mission data'});
      return;
    }
    if (seen.has(item.routineId)) {
      issues.push({path: String(index), message: 'daily Routine response contains a duplicate Routine ID'});
    }
    seen.add(item.routineId);
  });
  return issues.length > 0 ? {success: false, issues} : {success: true, value: value as ReadonlyArray<DailyRoutineView>};
}

export function validateDailyRoutineCommandReceipt(
  value: unknown,
  commandType: DailyRoutineCommandReceipt['commandType'],
  routineId: string,
  requestId: string,
): ValidationResult<DailyRoutineCommandReceipt> {
  const resultingState = commandType === 'routine.daily.create' ? 'active' : 'instruction_set';
  if (!isRecord(value)
    || typeof value.commandId !== 'string'
    || value.commandType !== commandType
    || value.targetType !== 'routine'
    || value.targetId !== routineId
    || value.requestId !== requestId
    || value.accepted !== true
    || typeof value.acceptedAt !== 'string'
    || value.resultingState !== resultingState) {
    return {success: false, issues: [{path: 'command', message: 'daily Routine command receipt is malformed or mismatched'}]};
  }
  return {success: true, value: value as DailyRoutineCommandReceipt};
}
