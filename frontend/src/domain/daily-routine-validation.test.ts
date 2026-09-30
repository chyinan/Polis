import {describe, expect, it} from 'vitest';
import {validateDailyRoutines} from './daily-routine-validation';

const validRoutine = {
  routineId: 'routine-1',
  missionId: 'mission-1',
  employeeId: 'emp-backend',
  timezone: 'Asia/Shanghai',
  localTime: '09:00',
  nextLogicalDay: '2026-09-30',
  catchUpPolicy: 'coalesce_latest',
  maxCatchUp: 1,
  nextDueAt: '2026-09-30T01:00:00Z',
  taskInstruction: 'Review the assigned queue.',
  needsInstructionOccurrences: 0,
  linkedTaskCount: 2,
};

describe('validateDailyRoutines', () => {
  it('accepts company-readable Routine records only for the requested Mission', () => {
    const result = validateDailyRoutines([validRoutine], 'mission-1');
    expect(result.success).toBe(true);
  });

  it('rejects cross-Mission and duplicate Routine records', () => {
    const crossMission = validateDailyRoutines([{...validRoutine, missionId: 'mission-other'}], 'mission-1');
    const duplicate = validateDailyRoutines([validRoutine, validRoutine], 'mission-1');
    expect(crossMission.success).toBe(false);
    expect(duplicate.success).toBe(false);
  });

  it('rejects unsupported recurrence profile values and malformed day strings', () => {
    const invalidPolicy = validateDailyRoutines([{...validRoutine, catchUpPolicy: 'unbounded'}], 'mission-1');
    const invalidDay = validateDailyRoutines([{...validRoutine, nextLogicalDay: '2026-02-31'}], 'mission-1');
    expect(invalidPolicy.success).toBe(false);
    expect(invalidDay.success).toBe(false);
  });
});
