// pattern: Imperative Shell

import {useRef, useState, type FormEvent} from 'react';
import type {DailyRoutineCatchUpPolicy, DailyRoutineView, EmployeeSummary, MissionSummary} from '../domain/workbench';
import {CommandApiError, type WorkbenchApi} from '../data/workbench-api';
import {useCreateDailyRoutine, useDailyRoutines, useSetDailyRoutineTaskInstruction} from '../data/workbench-query';
import styles from '../styles/workbench.module.css';

type DailyRoutinePanelProps = Readonly<{
  api: WorkbenchApi;
  companyId: string;
  mission: MissionSummary;
  employees: ReadonlyArray<EmployeeSummary>;
}>;

type CreateDraft = Readonly<{
  employeeId: string;
  taskInstruction: string;
  timezone: string;
  localTime: string;
  nextLogicalDay: string;
  catchUpPolicy: DailyRoutineCatchUpPolicy;
  maxCatchUp: string;
}>;

type PendingCreate = Readonly<{
  fingerprint: string;
  routineId: string;
  requestId: string;
}>;

function currentLogicalDay(timezone: string): string {
  const parts = new Intl.DateTimeFormat('en', {timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit'}).formatToParts(new Date());
  const year = parts.find(part => part.type === 'year')?.value;
  const month = parts.find(part => part.type === 'month')?.value;
  const day = parts.find(part => part.type === 'day')?.value;
  return year && month && day ? `${year}-${month}-${day}` : '';
}

function routineCatchUpLabel(policy: DailyRoutineCatchUpPolicy): string {
  const labels: Readonly<Record<DailyRoutineCatchUpPolicy, string>> = {
    skip: '跳过较早实例',
    coalesce_latest: '合并为最近一次',
    catch_up: '按上限补做',
  };
  return labels[policy];
}

function routineTimeLabel(value: string | null): string {
  if (value === null) return '未安排';
  const date = new Date(value);
  return Number.isFinite(date.getTime()) ? date.toLocaleString() : '时间不可读';
}

export function DailyRoutinePanel({api, companyId, mission, employees}: DailyRoutinePanelProps) {
  const routinesQuery = useDailyRoutines(api, companyId, mission.missionId);
  const createMutation = useCreateDailyRoutine(api, companyId, mission.missionId);
  const [taskInstruction, setTaskInstruction] = useState('');
  const [employeeId, setEmployeeId] = useState(employees[0]?.employeeId ?? '');
  const [timezone, setTimezone] = useState('Asia/Shanghai');
  const [localTime, setLocalTime] = useState('09:00');
  const [nextLogicalDay, setNextLogicalDay] = useState(() => currentLogicalDay('Asia/Shanghai'));
  const [catchUpPolicy, setCatchUpPolicy] = useState<DailyRoutineCatchUpPolicy>('coalesce_latest');
  const [maxCatchUp, setMaxCatchUp] = useState('1');
  const [message, setMessage] = useState<string | null>(null);
  const [isError, setIsError] = useState(false);
  const pendingCreate = useRef<PendingCreate | null>(null);
  const canCreate = mission.state === 'active' || mission.state === 'paused';

  if (api.mode !== 'real') return null;

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const instruction = taskInstruction.trim();
    if (instruction === '' || new TextEncoder().encode(instruction).length > 4096) {
      setIsError(true);
      setMessage('任务说明必须为 1–4096 字节。');
      return;
    }
    if (employeeId === '' || timezone.trim() === '' || localTime === '' || nextLogicalDay === '') {
      setIsError(true);
      setMessage('请填写负责人、时区、触发时间和首次执行日期。');
      return;
    }
    const parsedMaxCatchUp = Number(maxCatchUp);
    if (!Number.isInteger(parsedMaxCatchUp) || parsedMaxCatchUp < 1 || parsedMaxCatchUp > 10) {
      setIsError(true);
      setMessage('补做上限必须在 1 到 10 之间。');
      return;
    }
    const draft: CreateDraft = {employeeId, taskInstruction: instruction, timezone, localTime, nextLogicalDay, catchUpPolicy, maxCatchUp};
    const fingerprint = JSON.stringify(draft);
    const pending = pendingCreate.current?.fingerprint === fingerprint
      ? pendingCreate.current
      : {fingerprint, routineId: crypto.randomUUID(), requestId: crypto.randomUUID()};
    pendingCreate.current = pending;
    setIsError(false);
    setMessage(null);
    try {
      await createMutation.mutateAsync({
        routineId: pending.routineId,
        employeeId,
        taskInstruction: instruction,
        timezone,
        localTime,
        nextLogicalDay,
        catchUpPolicy,
        maxCatchUp: parsedMaxCatchUp,
        requestId: pending.requestId,
      });
      pendingCreate.current = null;
      setTaskInstruction('');
      setMessage('每日职责已保存；到期后会创建 Task，系统不会自动启动 Worker。');
    } catch (error) {
      setIsError(true);
      const detail = error instanceof CommandApiError || error instanceof Error ? error.message : '命令结果未知';
      setMessage(`每日职责结果尚未确认：${detail} 请先核对已刷新的 Routine 列表，再继续。`);
    }
  }

  return (
    <section className={styles.sectionBlock} data-testid="daily-routine-panel">
      <div className={styles.sectionBlockHeader}>
        <div><p className={styles.sectionKicker}>周期职责</p><h2 className={styles.sectionTitle}>每日 Routine</h2></div>
      </div>
      <article className={styles.sectionCard}>
        <p className={styles.panelDescription}>每次到期会生成一条固定负责人 Task。补做遵循所选上限和时区；暂停 Mission 时不会准入 Worker。</p>
        {routinesQuery.isPending ? <p className={styles.formHint}>正在读取每日职责…</p> : null}
        {routinesQuery.isError ? <p className={styles.errorText} role="alert">{routinesQuery.error.message}</p> : null}
        {routinesQuery.data?.length === 0 ? <p className={styles.emptyState}>当前 Mission 尚未安排每日职责。</p> : null}
        {routinesQuery.data?.map(routine => <DailyRoutineRow api={api} companyId={companyId} missionId={mission.missionId} employees={employees} canRepair={canCreate} routine={routine} key={routine.routineId} />)}
        {canCreate ? <form className={styles.formStack} onSubmit={handleCreate}>
          <div className={styles.subsectionHeader}><span>新增每日职责</span><code>固定岗位 / 每日一次</code></div>
          <label className={styles.formLabel}>负责人<select className={styles.formField} value={employeeId} onChange={event => setEmployeeId(event.target.value)} required>
            <option value="">选择固定成员</option>
            {employees.map(employee => <option key={employee.employeeId} value={employee.employeeId}>{employee.displayName} · {employee.employeeId}</option>)}
          </select></label>
          <label className={styles.formLabel}>任务说明（保存后固定）<textarea className={styles.formField} rows={3} maxLength={4096} value={taskInstruction} onChange={event => setTaskInstruction(event.target.value)} required placeholder="写清每次到期要完成的有限工作" /></label>
          <div className={styles.formStack}>
            <label className={styles.formLabel}>时区<input className={styles.formField} value={timezone} onChange={event => setTimezone(event.target.value)} required /></label>
            <label className={styles.formLabel}>本地触发时间<input className={styles.formField} type="time" value={localTime} onChange={event => setLocalTime(event.target.value)} required /></label>
            <label className={styles.formLabel}>首次执行日期<input className={styles.formField} type="date" value={nextLogicalDay} onChange={event => setNextLogicalDay(event.target.value)} required /></label>
            <label className={styles.formLabel}>补做策略<select className={styles.formField} value={catchUpPolicy} onChange={event => setCatchUpPolicy(event.target.value as DailyRoutineCatchUpPolicy)}>
              <option value="skip">跳过较早实例</option><option value="coalesce_latest">合并为最近一次</option><option value="catch_up">按上限补做</option>
            </select></label>
            {catchUpPolicy === 'catch_up' ? <label className={styles.formLabel}>最多补做<input className={styles.formField} type="number" min={1} max={10} value={maxCatchUp} onChange={event => setMaxCatchUp(event.target.value)} required /></label> : null}
          </div>
          <button className={styles.commandButton} type="submit" disabled={createMutation.isPending}>{createMutation.isPending ? '保存中…' : '保存每日职责'}</button>
        </form> : <p className={styles.formHint}>只有进行中或已暂停的 Mission 可以安排每日职责。</p>}
        {message !== null ? <p className={isError ? styles.errorText : styles.formHint} role={isError ? 'alert' : 'status'}>{message}</p> : null}
      </article>
    </section>
  );
}

type DailyRoutineRowProps = Readonly<{
  api: WorkbenchApi;
  companyId: string;
  missionId: string;
  employees: ReadonlyArray<EmployeeSummary>;
  canRepair: boolean;
  routine: DailyRoutineView;
}>;

function DailyRoutineRow({api, companyId, missionId, employees, canRepair, routine}: DailyRoutineRowProps) {
  const repairMutation = useSetDailyRoutineTaskInstruction(api, companyId, missionId);
  const [instruction, setInstruction] = useState(routine.taskInstruction ?? '');
  const [message, setMessage] = useState<string | null>(null);
  const [isError, setIsError] = useState(false);
  const pendingRepair = useRef<Readonly<{instruction: string; requestId: string}> | null>(null);
  const employeeName = employees.find(employee => employee.employeeId === routine.employeeId)?.displayName ?? routine.employeeId;
  const needsRepair = routine.taskInstruction === null && (routine.needsInstructionOccurrences > 0 || canRepair);

  async function handleRepair() {
    const trimmed = instruction.trim();
    if (trimmed === '' || new TextEncoder().encode(trimmed).length > 4096) {
      setIsError(true);
      setMessage('任务说明必须为 1–4096 字节。');
      return;
    }
    const pending = pendingRepair.current?.instruction === trimmed
      ? pendingRepair.current
      : {instruction: trimmed, requestId: crypto.randomUUID()};
    pendingRepair.current = pending;
    setIsError(false);
    setMessage(null);
    try {
      await repairMutation.mutateAsync({routineId: routine.routineId, taskInstruction: trimmed, requestId: pending.requestId});
      pendingRepair.current = null;
      setMessage('旧实例已补充说明并生成 Task。');
    } catch (error) {
      setIsError(true);
      const detail = error instanceof CommandApiError || error instanceof Error ? error.message : '命令结果未知';
      setMessage(`旧实例补充结果尚未确认：${detail} 请先核对已刷新的 Routine 和 Task 状态，再继续。`);
    }
  }

  return (
    <article className={styles.boundaryItem} data-routine-id={routine.routineId}>
      <div><strong>{employeeName} · {routine.localTime} ({routine.timezone})</strong><p>下次：{routineTimeLabel(routine.nextDueAt)} · {routineCatchUpLabel(routine.catchUpPolicy)} · 已关联 {routine.linkedTaskCount} 条 Task</p>
        {routine.taskInstruction !== null ? <p>{routine.taskInstruction}</p> : null}
        {needsRepair ? <div className={styles.formStack}>
          <p className={styles.formHint}>{routine.needsInstructionOccurrences > 0 ? `有 ${routine.needsInstructionOccurrences} 条旧实例缺少任务说明；补充后会为它们各创建一条 Task。` : '旧 Routine 还没有任务说明；到期前补充后才会生成 Task。'}</p>
          <label className={styles.formLabel}>补充任务说明<textarea className={styles.formField} rows={2} maxLength={4096} value={instruction} onChange={event => setInstruction(event.target.value)} required /></label>
          <button className={styles.textButton} type="button" disabled={!canRepair || repairMutation.isPending} onClick={() => void handleRepair()}>{repairMutation.isPending ? '补充中…' : '补充并交付旧实例'}</button>
        </div> : null}
        {message !== null ? <p className={isError ? styles.errorText : styles.formHint} role={isError ? 'alert' : 'status'}>{message}</p> : null}
      </div>
    </article>
  );
}
