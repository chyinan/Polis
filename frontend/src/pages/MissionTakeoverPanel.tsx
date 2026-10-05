// pattern: Imperative Shell
import {useEffect, useMemo, useRef, useState, type ChangeEvent, type ReactElement} from 'react';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import type {WorkbenchApi} from '../data/workbench-api';
import {useCreateTaskTakeoverLease, useReleaseTaskTakeoverLease, useSubmitTaskTakeoverSnapshot, useTaskTakeoverLeases} from '../data/workbench-query';
import type {CompanyOverviewView, TaskTakeoverLeaseView, TaskTakeoverWorkspaceManifestEntryView, TaskTakeoverWorkspaceManifestView} from '../domain/workbench';
import {applyWorkspaceTextPatch, MAX_WORKSPACE_PATCH_BYTES, MAX_WORKSPACE_SNAPSHOT_BYTES} from '../domain/workspace-patch';
import styles from '../styles/workbench.module.css';

type PendingSnapshotReturn = Readonly<{
  leaseId: string;
  requestId: string;
  baseWorkspaceDigest: string;
  baseWorkspaceRevision: number;
  content: string;
  humanEffortSeconds: number;
}>;

type TakeoverWorkspaceDraftFile = Readonly<{
  entry: TaskTakeoverWorkspaceManifestEntryView;
  baseContent: string;
  content: string;
}>;

type MissionTakeoverPanelProps = Readonly<{
  api: WorkbenchApi;
  companyId: string;
  missionId: string;
  missionState: string;
  overview: CompanyOverviewView;
}>;

async function loadBoundWorkspaceFiles(api: WorkbenchApi, companyId: string, missionId: string, lease: TaskTakeoverLeaseView): Promise<Readonly<{manifest: TaskTakeoverWorkspaceManifestView; files: ReadonlyArray<TakeoverWorkspaceDraftFile>}>> {
  const manifest = await api.getTaskTakeoverWorkspaceManifest({companyId, missionId, leaseId: lease.leaseId});
  const binding = lease.workspaceTree;
  if (binding === undefined || manifest.taskId !== lease.taskId || manifest.workspaceTree.rootBindingId !== binding.rootBindingId
    || manifest.workspaceTree.revision !== binding.revision || manifest.workspaceTree.manifestSha256 !== binding.manifestSha256
    || manifest.workspaceTree.fileCount !== binding.fileCount || manifest.workspaceTree.bytes !== binding.bytes) {
    throw new Error('接管租约返回的冻结文件清单与租约固定摘要不一致。');
  }
  const loaded: Array<TakeoverWorkspaceDraftFile | undefined> = new Array(manifest.entries.length);
  let cursor = 0;
  const workers = Array.from({length: Math.min(4, manifest.entries.length)}, async () => {
    while (true) {
      const index = cursor;
      cursor += 1;
      const entry = manifest.entries[index];
      if (entry === undefined) return;
      const file = await api.readTaskTakeoverWorkspaceFile({
        companyId,
        missionId,
        leaseId: lease.leaseId,
        relativePath: entry.relativePath,
        manifestSha256: manifest.workspaceTree.manifestSha256,
      });
      if (file.taskId !== lease.taskId || file.relativePath !== entry.relativePath || file.sha256 !== entry.sha256 || file.bytes !== entry.bytes
        || file.fileRevision !== entry.fileRevision || file.workspaceRevision !== manifest.workspaceTree.revision || file.contentType !== entry.contentType) {
        throw new Error(`冻结文件 ${entry.relativePath} 与租约清单不一致。`);
      }
      loaded[index] = {entry, baseContent: file.content, content: file.content};
    }
  });
  await Promise.all(workers);
  if (loaded.some(file => file === undefined)) throw new Error('冻结工作区有文件未能加载完成。');
  return {manifest, files: loaded as ReadonlyArray<TakeoverWorkspaceDraftFile>};
}

export function MissionTakeoverPanel({api, companyId, missionId, missionState, overview}: MissionTakeoverPanelProps): ReactElement {
  const leasesQuery = useTaskTakeoverLeases(api, companyId, missionId);
  const createLease = useCreateTaskTakeoverLease(api, companyId, missionId);
  const submitSnapshot = useSubmitTaskTakeoverSnapshot(api, companyId, missionId);
  const releaseLease = useReleaseTaskTakeoverLease(api, companyId, missionId);
  const eligibleTasks = useMemo(() => overview.tasks.filter(task => task.state !== 'completed' && task.state !== 'cancelled'), [overview.tasks]);
  const [selectedTaskId, setSelectedTaskId] = useState('');
  const [content, setContent] = useState('');
  const [baseContent, setBaseContent] = useState<string | null>(null);
  const [workspaceTreeManifest, setWorkspaceTreeManifest] = useState<TaskTakeoverWorkspaceManifestView | null>(null);
  const [workspaceTreeFiles, setWorkspaceTreeFiles] = useState<ReadonlyArray<TakeoverWorkspaceDraftFile>>([]);
  const [selectedWorkspacePath, setSelectedWorkspacePath] = useState('');
  const [humanEffortSeconds, setHumanEffortSeconds] = useState('');
  const [loadedLeaseId, setLoadedLeaseId] = useState('');
  const [localError, setLocalError] = useState('');
  const [patchNotice, setPatchNotice] = useState('');
  const [patchError, setPatchError] = useState('');
  const [workspaceLoadFailed, setWorkspaceLoadFailed] = useState(false);
  const [workspaceLoadAttempt, setWorkspaceLoadAttempt] = useState(0);
  const [pendingSnapshotReturn, setPendingSnapshotReturn] = useState<PendingSnapshotReturn | null>(null);
  const pendingRequestIds = useRef(new Map<string, string>());
  const taskId = selectedTaskId || eligibleTasks[0]?.taskId || '';
  const activeLease = leasesQuery.data?.find(item => item.taskId === taskId && item.state === 'granted') ?? null;
  const selectedWorkspaceFile = workspaceTreeFiles.find(file => file.entry.relativePath === selectedWorkspacePath) ?? null;
  const workspaceTreeDraftBytes = workspaceTreeFiles.reduce((total, file) => total + new TextEncoder().encode(file.content).length, 0);
  const singleFileReturnSupported = workspaceTreeManifest === null || (workspaceTreeManifest.entries.length === 1 && workspaceTreeManifest.entries[0]?.relativePath === 'workspace.txt');
  const pending = createLease.isPending || submitSnapshot.isPending || releaseLease.isPending;
  const contentBytes = new TextEncoder().encode(content).length;
  const canGrant = api.mode === 'real' && missionState === 'paused' && leasesQuery.data !== undefined && taskId !== '' && activeLease === null && pendingSnapshotReturn === null && !pending;
  const canReturn = missionState === 'paused' && activeLease !== null && singleFileReturnSupported && baseContent !== null && loadedLeaseId === activeLease.leaseId && content.trim() !== '' && contentBytes <= MAX_WORKSPACE_SNAPSHOT_BYTES && pendingSnapshotReturn === null && !pending;

  useEffect(() => {
    if (activeLease === null || loadedLeaseId === activeLease.leaseId) return;
    let live = true;
    setLocalError('');
    setBaseContent(null);
    setContent('');
    setWorkspaceTreeManifest(null);
    setWorkspaceTreeFiles([]);
    setSelectedWorkspacePath('');
    setPatchNotice('');
    setPatchError('');
    setWorkspaceLoadFailed(false);
    if (activeLease.workspaceTree !== undefined) {
      void loadBoundWorkspaceFiles(api, companyId, missionId, activeLease).then(({manifest, files}) => {
        if (!live) return;
        setWorkspaceTreeManifest(manifest);
        setWorkspaceTreeFiles(files);
        setSelectedWorkspacePath(files[0]?.entry.relativePath ?? '');
        if (manifest.entries.length === 1 && manifest.entries[0]?.relativePath === 'workspace.txt' && files[0] !== undefined) {
          setBaseContent(files[0].baseContent);
          setContent(files[0].content);
        }
        setLoadedLeaseId(activeLease.leaseId);
      }).catch(error => {
        if (!live) return;
        setWorkspaceLoadFailed(true);
        setLocalError(error instanceof Error ? error.message : '读取租约冻结文件失败。');
      });
      return () => { live = false; };
    }
    void api.getWorkspace({companyId, taskId: activeLease.taskId}).then(workspace => {
      const workspaceRevision = Number(workspace.revision);
      if (workspace.digest !== activeLease.baseWorkspaceDigest || workspaceRevision !== activeLease.baseWorkspaceRevision || workspace.files.length !== 1 || workspace.files[0]?.path !== 'workspace.txt') {
        throw new Error('读取到的工作区与接管冻结版本不一致，不能用于编辑。');
      }
      if (live) {
        setBaseContent(workspace.files[0].content);
        setContent(workspace.files[0].content);
        setLoadedLeaseId(activeLease.leaseId);
      }
    }).catch(error => {
      if (live) {
        setWorkspaceLoadFailed(true);
        setLocalError(error instanceof Error ? error.message : '读取冻结工作区失败。');
      }
    });
    return () => { live = false; };
  }, [activeLease, api, companyId, missionId, loadedLeaseId, workspaceLoadAttempt]);

  function retryFrozenWorkspaceRead(): void {
    if (activeLease === null || pending || pendingSnapshotReturn !== null) return;
    setWorkspaceLoadFailed(false);
    setLocalError('');
    setWorkspaceLoadAttempt(attempt => attempt + 1);
  }

  function updateTreeFile(path: string, nextContent: string): void {
    setWorkspaceTreeFiles(files => files.map(file => file.entry.relativePath === path ? {...file, content: nextContent} : file));
    if (workspaceTreeManifest?.entries.length === 1 && path === 'workspace.txt') {
      setContent(nextContent);
      setPatchNotice('');
    }
  }

  function requestIdFor(key: string): string {
    const existing = pendingRequestIds.current.get(key);
    if (existing !== undefined) return existing;
    const requestId = `${key}-${crypto.randomUUID()}`;
    pendingRequestIds.current.set(key, requestId);
    return requestId;
  }

  function clearRequestId(key: string): void {
    pendingRequestIds.current.delete(key);
  }

  async function grant(): Promise<void> {
    if (!canGrant) return;
    setLocalError('');
    const key = `task-takeover-grant-${taskId}`;
    try {
      const lease = await createLease.mutateAsync({taskId, requestId: requestIdFor(key)});
      clearRequestId(key);
      setSelectedTaskId(lease.taskId);
      setContent('');
      setBaseContent(null);
      setLoadedLeaseId('');
    } catch (error) {
      const detail = error instanceof Error ? error.message : '命令结果未知';
      setLocalError(`接管租约结果尚未确认：${detail} 已刷新租约状态；若仍可申请，重试会复用原请求 ID。`);
    }
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
      requestId: `task-takeover-return-${activeLease.leaseId}-${crypto.randomUUID()}`,
      baseWorkspaceDigest: activeLease.baseWorkspaceDigest,
      baseWorkspaceRevision: activeLease.baseWorkspaceRevision,
      content,
      humanEffortSeconds: effort,
    };
    setPendingSnapshotReturn(attempt);
    await retrySnapshotReturn(attempt);
  }

  async function importPatch(event: ChangeEvent<HTMLInputElement>): Promise<void> {
    const file = event.currentTarget.files?.item(0) ?? null;
    event.currentTarget.value = '';
    if (file === null) return;
    setPatchError('');
    setPatchNotice('');
    if (baseContent === null || activeLease === null || loadedLeaseId !== activeLease.leaseId) {
      setPatchError('冻结工作区尚未加载完成，不能导入补丁。');
      return;
    }
    if (file.size > MAX_WORKSPACE_PATCH_BYTES) {
      setPatchError(`补丁超过 ${MAX_WORKSPACE_PATCH_BYTES} 字节上限。`);
      return;
    }
    try {
      const patchText = new TextDecoder('utf-8', {fatal: true}).decode(await file.arrayBuffer());
      const applied = applyWorkspaceTextPatch(baseContent, patchText);
      setContent(applied.content);
      setWorkspaceTreeFiles(files => files.map(item => item.entry.relativePath === 'workspace.txt' ? {...item, content: applied.content} : item));
      setPatchNotice(`补丁已按冻结基线核对：新增 ${applied.addedLines} 行、删除 ${applied.removedLines} 行。下方可对比冻结文本与候选文本。`);
    } catch (error) {
      setPatchError(error instanceof Error ? error.message : '补丁无效，候选内容未更改。');
    }
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
    // An unresolved snapshot return owns the exact lease/request identity until
    // its receipt is confirmed. Releasing here could strand a same-payload retry.
    if (activeLease === null || pending || pendingSnapshotReturn !== null) return;
    setLocalError('');
    const key = `task-takeover-release-${activeLease.leaseId}`;
    try {
      await releaseLease.mutateAsync({leaseId: activeLease.leaseId, requestId: requestIdFor(key)});
      clearRequestId(key);
      setContent('');
      setBaseContent(null);
      setLoadedLeaseId('');
    } catch (error) {
      const detail = error instanceof Error ? error.message : '命令结果未知';
      setLocalError(`租约释放结果尚未确认：${detail} 已刷新租约与活动状态；若仍可释放，重试会复用原请求 ID。`);
    }
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
            <div className={styles.recordActions}><button className={styles.commandButton} disabled={pending || pendingSnapshotReturn !== null || missionState !== 'paused'} onClick={() => { void release(); }} type="button">释放租约，不提交 snapshot</button></div>
          </div>
          {workspaceLoadFailed ? <div className={styles.recordRow}>
            <div className={styles.recordLead}><div>
              <strong>冻结工作区尚未加载</strong>
              <span>重试会继续读取同一租约绑定的摘要和修订号；不会创建新租约。</span>
            </div></div>
            <div className={styles.recordActions}><button className={styles.commandButton} disabled={pending || pendingSnapshotReturn !== null} onClick={retryFrozenWorkspaceRead} type="button">重试读取冻结工作区</button></div>
          </div> : null}
          {workspaceTreeManifest !== null ? <>
            <p className={styles.formHint}>冻结清单固定了 {workspaceTreeManifest.workspaceTree.fileCount} 个文件、{workspaceTreeManifest.workspaceTree.bytes} 字节；编辑器读取的都是该租约清单中的 CAS 内容。</p>
            <label className={styles.formLabel}>工作区文件
              <select className={styles.formField} disabled={pendingSnapshotReturn !== null} value={selectedWorkspacePath} onChange={event => setSelectedWorkspacePath(event.target.value)}>
                {workspaceTreeFiles.map(file => <option key={file.entry.relativePath} value={file.entry.relativePath}>{file.entry.relativePath}{file.content === file.baseContent ? '' : ' · 已修改'}</option>)}
              </select>
            </label>
            <label className={styles.formLabel}>文件内容 · {selectedWorkspaceFile?.entry.relativePath ?? '正在加载'}
              <textarea className={styles.formField} disabled={pendingSnapshotReturn !== null || selectedWorkspaceFile === null} rows={12} value={selectedWorkspaceFile?.content ?? ''} onChange={event => { if (selectedWorkspaceFile !== null) updateTreeFile(selectedWorkspaceFile.entry.relativePath, event.target.value); }} placeholder="正在读取租约冻结文件…" />
            </label>
            {workspaceTreeManifest.entries.length > 1 ? <p className={styles.formHint}>当前租约包含多个文件；交还按钮会在完整树回传格式接入后开放。现在可以逐个查看和修改文件，所有修改暂留在本面板中。</p> : null}
            {workspaceTreeManifest.entries.length === 1 && workspaceTreeManifest.entries[0]?.relativePath === 'workspace.txt' ? <>
              <label className={styles.formLabel}>导入基线补丁（仅限 workspace.txt）
                <input accept=".diff,.patch,text/plain" className={styles.formField} disabled={pendingSnapshotReturn !== null || baseContent === null || loadedLeaseId !== activeLease.leaseId} onChange={event => { void importPatch(event); }} type="file" />
              </label>
              <p className={styles.formHint}>只接受 UTF-8 unified diff，必须仅修改 workspace.txt，且所有 hunk 必须匹配本租约冻结内容。补丁不会执行脚本、启用 Skill/MCP 或写入旧 Task；过期基线和冲突会拒绝导入。</p>
              {baseContent !== null ? <div className={styles.wizardGrid}>
                <label className={styles.formLabel}>冻结基线（只读）
                  <textarea aria-label="冻结基线预览" className={styles.formField} readOnly rows={8} value={baseContent} />
                </label>
                <label className={styles.formLabel}>待交还候选（只读预览）
                  <textarea aria-label="待交还候选预览" className={styles.formField} readOnly rows={8} value={content} />
                </label>
              </div> : null}
            </> : null}
          </> : <>
            <label className={styles.formLabel}>完整工作区文本 snapshot
              <textarea className={styles.formField} disabled={pendingSnapshotReturn !== null || baseContent === null} rows={10} maxLength={MAX_WORKSPACE_SNAPSHOT_BYTES} value={content} onChange={event => { setContent(event.target.value); setPatchNotice(''); }} placeholder="正在读取冻结工作区文本…" />
            </label>
            <label className={styles.formLabel}>导入基线补丁（仅限 workspace.txt）
              <input accept=".diff,.patch,text/plain" className={styles.formField} disabled={pendingSnapshotReturn !== null || baseContent === null || loadedLeaseId !== activeLease.leaseId} onChange={event => { void importPatch(event); }} type="file" />
            </label>
            <p className={styles.formHint}>只接受 UTF-8 unified diff，必须仅修改 workspace.txt，且所有 hunk 必须匹配本租约冻结内容。补丁不会执行脚本、启用 Skill/MCP 或写入旧 Task；过期基线和冲突会拒绝导入。</p>
            {baseContent !== null ? <div className={styles.wizardGrid}>
              <label className={styles.formLabel}>冻结基线（只读）
                <textarea aria-label="冻结基线预览" className={styles.formField} readOnly rows={8} value={baseContent} />
              </label>
              <label className={styles.formLabel}>待交还候选（只读预览）
                <textarea aria-label="待交还候选预览" className={styles.formField} readOnly rows={8} value={content} />
              </label>
            </div> : null}
          </>}
          {patchError !== '' ? <p className={styles.formError} role="alert">{patchError}</p> : null}
          {patchNotice !== '' ? <p className={styles.operationNotice} role="status">{patchNotice}</p> : null}
          <div className={styles.recordLead}><span>{workspaceTreeManifest !== null && workspaceTreeManifest.entries.length > 1 ? `候选 ${workspaceTreeDraftBytes} / 16777216 UTF-8 字节` : `${contentBytes} / 4096 UTF-8 字节`}</span></div>
          <label className={styles.formLabel}>人工投入秒数（可留空）
            <input className={styles.formField} disabled={pendingSnapshotReturn !== null} inputMode="numeric" max={86400} min={0} value={humanEffortSeconds} onChange={event => setHumanEffortSeconds(event.target.value)} />
          </label>
          {pendingSnapshotReturn === null ? <button className={styles.commandButton} disabled={!canReturn || loadedLeaseId !== activeLease.leaseId} onClick={() => { void handBack(); }} type="button">{submitSnapshot.isPending ? '正在核对并交还…' : workspaceTreeManifest !== null && workspaceTreeManifest.entries.length > 1 ? '多文件交还尚未就绪' : '交还人工 snapshot'}</button> : null}
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
