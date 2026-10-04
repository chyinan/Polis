// pattern: Imperative Shell
import {useEffect, useMemo, useState, type ReactElement} from 'react';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import type {WorkbenchApi} from '../data/workbench-api';
import {useCreateTaskTakeoverLease, useReleaseTaskTakeoverLease, useSubmitTaskTakeoverSnapshot, useTaskTakeoverLeases} from '../data/workbench-query';
import type {CompanyOverviewView, TaskTakeoverLeaseView} from '../domain/workbench';
import styles from '../styles/workbench.module.css';

type PendingSnapshotReturn = Readonly<{
  leaseId: string;
  requestId: string;
  baseWorkspaceDigest: string;
  baseWorkspaceRevision: number;
  content: string;
  humanEffortSeconds: number;
}>;

type MissionTakeoverPanelProps = Readonly<{
  api: WorkbenchApi;
  companyId: string;
  missionId: string;
  missionState: string;
  overview: CompanyOverviewView;
}>;

export function MissionTakeoverPanel({api, companyId, missionId, missionState, overview}: MissionTakeoverPanelProps): ReactElement {
  const leasesQuery = useTaskTakeoverLeases(api, companyId, missionId);
  const createLease = useCreateTaskTakeoverLease(api, companyId, missionId);
  const submitSnapshot = useSubmitTaskTakeoverSnapshot(api, companyId, missionId);
  const releaseLease = useReleaseTaskTakeoverLease(api, companyId, missionId);
  const eligibleTasks = useMemo(() => overview.tasks.filter(task => task.state !== 'completed' && task.state !== 'cancelled'), [overview.tasks]);
  const [selectedTaskId, setSelectedTaskId] = useState('');
  const [content, setContent] = useState('');
  const [humanEffortSeconds, setHumanEffortSeconds] = useState('');
  const [loadedLeaseId, setLoadedLeaseId] = useState('');
  const [localError, setLocalError] = useState('');
  const [pendingSnapshotReturn, setPendingSnapshotReturn] = useState<PendingSnapshotReturn | null>(null);
  const taskId = selectedTaskId || eligibleTasks[0]?.taskId || '';
  const activeLease = leasesQuery.data?.find(item => item.taskId === taskId && item.state === 'granted') ?? null;
  const pending = createLease.isPending || submitSnapshot.isPending || releaseLease.isPending;
  const contentBytes = new TextEncoder().encode(content).length;
  const canGrant = api.mode === 'real' && missionState === 'paused' && leasesQuery.data !== undefined && taskId !== '' && activeLease === null && pendingSnapshotReturn === null && !pending;
  const canReturn = missionState === 'paused' && activeLease !== null && content.trim() !== '' && contentBytes <= 4096 && pendingSnapshotReturn === null && !pending;

  useEffect(() => {
    if (activeLease === null || loadedLeaseId === activeLease.leaseId) return;
    let live = true;
    setLocalError('');
    void api.getWorkspace({companyId, taskId: activeLease.taskId}).then(workspace => {
      const workspaceRevision = Number(workspace.revision);
      if (workspace.digest !== activeLease.baseWorkspaceDigest || workspaceRevision !== activeLease.baseWorkspaceRevision || workspace.files.length !== 1 || workspace.files[0]?.path !== 'workspace.txt') {
        throw new Error('读取到的工作区与接管冻结版本不一致，不能用于编辑。');
      }
      if (live) setContent(workspace.files[0].content);
    }).catch(error => {
      if (live) setLocalError(error instanceof Error ? error.message : '读取冻结工作区失败。');
    }).finally(() => {
      if (live) setLoadedLeaseId(activeLease.leaseId);
    });
    return () => { live = false; };
  }, [activeLease, api, companyId, loadedLeaseId]);

  async function grant(): Promise<void> {
    if (!canGrant) return;
    setLocalError('');
    const lease = await createLease.mutateAsync({taskId, requestId: `task-takeover-grant-${Date.now()}`});
    setSelectedTaskId(lease.taskId);
    setContent('');
    setLoadedLeaseId('');
  }

  async function handBack(): Promise<void> {
    if (!canReturn || activeLease === null) return;
    const effort = humanEffortSeconds.trim() === '' ? 0 : Number(humanEffortSeconds);
    if (!Number.isSafeInteger(effort) || effort < 0 || effort > 86_400) {
      setLocalError('人工投入时间须为 0 至 86400 秒，留空表示不记录。');
      return;
    }
    setLocalError('');
    const attempt: PendingSnapshotReturn = {
      leaseId: activeLease.leaseId,
      requestId: `task-takeover-return-${activeLease.leaseId}-${Date.now()}`,
      baseWorkspaceDigest: activeLease.baseWorkspaceDigest,
      baseWorkspaceRevision: activeLease.baseWorkspaceRevision,
      content,
      humanEffortSeconds: effort,
    };
    setPendingSnapshotReturn(attempt);
    await retrySnapshotReturn(attempt);
  }

  async function retrySnapshotReturn(attempt: PendingSnapshotReturn): Promise<void> {
    setLocalError('');
    try {
      await submitSnapshot.mutateAsync(attempt);
      setPendingSnapshotReturn(null);
      setLoadedLeaseId('');
    } catch {
      // Keep the exact payload and request ID so an ambiguous response can be retried safely.
    }
  }

  async function release(): Promise<void> {
    if (activeLease === null || pending) return;
    await releaseLease.mutateAsync({leaseId: activeLease.leaseId, requestId: `task-takeover-release-${activeLease.leaseId}-${Date.now()}`});
    setContent('');
    setLoadedLeaseId('');
  }

  return <section className={styles.sectionCard}>
    <div className={styles.sectionHeader}>
      <div><span className={styles.cardEyebrow}>人工接管</span><h2 className={styles.sectionTitle}>冻结工作区并交还 snapshot</h2></div>
      <StatusBadge label={activeLease?.state === 'granted' ? '接管中' : '由操作员发起'} tone={activeLease?.state === 'granted' ? 'warning' : 'neutral'} />
    </div>
    <p className={styles.panelDescription}>仅在 Mission 已暂停且 Worker、Job 和服务端点停止后发放接管租约。交还内容绑定冻结的工作区摘要与修订号，作为 MissionInput 进入后续正式变更请求；不会直接覆盖旧 Task 工作区。</p>
    {eligibleTasks.length === 0 ? <div className={styles.emptyState}>没有可接管的未完成 Task。</div> : <>
      <div className={styles.formStack}>
        {pendingSnapshotReturn !== null ? <div className={styles.recordRow}>
          <div className={styles.recordLead}><div>
            <strong>快照交还结果尚未确认</strong>
            <span>重试会使用相同内容、冻结版本和请求 ID；系统会读取已保存回执或继续原提交。</span>
          </div></div>
          <div className={styles.recordActions}><button className={styles.commandButton} disabled={pending} onClick={() => { void retrySnapshotReturn(pendingSnapshotReturn); }} type="button">以同一请求重试交还</button></div>
        </div> : null}
        <label className={styles.formLabel}>目标 Task
          <select className={styles.formField} disabled={pendingSnapshotReturn !== null} value={taskId} onChange={event => { setSelectedTaskId(event.target.value); setContent(''); setLoadedLeaseId(''); }}>
            {eligibleTasks.map(task => <option key={task.taskId} value={task.taskId}>{task.title} · {task.taskId} · {task.state}</option>)}
          </select>
        </label>
        {missionState !== 'paused' ? <p className={styles.formHint}>暂停 Mission 后才可以取得或交还接管租约。</p> : null}
        {activeLease === null ? <button className={styles.commandButton} disabled={!canGrant} onClick={() => { void grant(); }} type="button">{createLease.isPending ? '正在确认停止边界…' : '请求人工接管'}</button> : <>
          <div className={styles.recordRow}>
            <div className={styles.recordLead}><div>
              <strong>冻结版本 {activeLease.baseWorkspaceDigest.slice(0, 16)} · r{activeLease.baseWorkspaceRevision}</strong>
              <span>租约 {activeLease.leaseId} · 需求基线 {activeLease.baseRequirementsSha256.slice(0, 16)}</span>
            </div></div>
            <div className={styles.recordActions}><button className={styles.commandButton} disabled={pending || missionState !== 'paused'} onClick={() => { void release(); }} type="button">释放租约，不提交 snapshot</button></div>
          </div>
          <label className={styles.formLabel}>完整工作区文本 snapshot
            <textarea className={styles.formField} disabled={pendingSnapshotReturn !== null} rows={10} maxLength={4096} value={content} onChange={event => setContent(event.target.value)} placeholder="正在读取冻结工作区文本…" />
          </label>
          <div className={styles.recordLead}><span>{contentBytes} / 4096 UTF-8 字节</span></div>
          <label className={styles.formLabel}>人工投入秒数（可留空）
            <input className={styles.formField} inputMode="numeric" max={86400} min={0} value={humanEffortSeconds} onChange={event => setHumanEffortSeconds(event.target.value)} />
          </label>
          {pendingSnapshotReturn === null ? <button className={styles.commandButton} disabled={!canReturn || loadedLeaseId !== activeLease.leaseId} onClick={() => { void handBack(); }} type="button">{submitSnapshot.isPending ? '正在核对并交还…' : '交还人工 snapshot'}</button> : null}
        </>}
        {leasesQuery.isError ? <p className={styles.formError} role="alert">接管历史读取失败：{leasesQuery.error.message}</p> : null}
        {createLease.isError ? <p className={styles.formError} role="alert">接管请求结果尚未确认：{createLease.error.message} 请核对已刷新的租约状态。</p> : null}
        {submitSnapshot.isError ? <p className={styles.formError} role="alert">snapshot 交还请求未确认：{submitSnapshot.error.message} 请检查上方租约状态，再以相同请求 ID 重试。</p> : null}
        {releaseLease.isError ? <p className={styles.formError} role="alert">租约释放结果尚未确认：{releaseLease.error.message} 请核对已刷新的租约状态。</p> : null}
        {localError !== '' ? <p className={styles.formError} role="alert">{localError}</p> : null}
      </div>
      <div className={styles.recordList}>
        {(leasesQuery.data ?? []).map((lease: TaskTakeoverLeaseView) => <article className={styles.recordRow} key={lease.leaseId}>
          <div className={styles.recordLead}><div><strong>{lease.taskId} · {lease.state}</strong>
            <span>{lease.snapshotDigest === null ? `冻结 r${lease.baseWorkspaceRevision} · ${lease.baseWorkspaceDigest}` : `snapshot ${lease.snapshotDigest} · ${lease.snapshotBytes} 字节`}</span>
            <span>{lease.events.map(event => `${event.state} · ${event.reasonCode} · ${event.createdAt}`).join(' / ')}</span>
          </div></div>
        </article>)}
      </div>
    </>}
  </section>;
}
