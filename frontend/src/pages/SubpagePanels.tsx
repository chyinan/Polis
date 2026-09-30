// pattern: Imperative Shell

import {BadgeCheck, Check, FileCheck2, FolderOpen, Handshake, History, LockKeyhole, MessageSquare, Settings2, ShieldAlert, Wrench} from 'lucide-react';
import {useState, type ChangeEvent, type ReactNode} from 'react';
import type {ActivityEvent, CompanyOverviewView, CrossBackendHandoverView, EmployeeSummary, JobRunView, ProjectEnvironmentRevisionView, TaskSummary} from '../domain/workbench';
import {isInputArchiveSource, type MissionInputState, type MissionInputView} from '../domain/mission-input';
import {MAX_MISSION_DIRECTORY_BYTES, MAX_MISSION_DIRECTORY_FILES, MAX_MISSION_INPUT_BYTES} from '../data/workbench-api';
import type {WorkbenchApi} from '../data/workbench-api';
import {useArtifactDeliveryManifest, useArtifactDetail, useCreateProjectJobBrowserSession, useCreateTaskEnvironmentHandover, useDecideProjectEnvironmentExecutorQualification, useDecideProjectEnvironmentPolicy, useEnsureProjectEnvironment, useMissionInputs, useProjectEnvironments, useStartTaskJobRun, useStopTaskJobRun, useTaskCrossBackendHandovers, useTaskInputManifest, useTaskJobLogs, useTaskJobRuns, useTaskWorkspace, useUploadMissionDirectoryInput, useUploadMissionInput} from '../data/workbench-query';
import type {ServiceBrowserSessionView} from '../domain/workbench';
import {formatEntityId} from '../domain/activity-presentation';
import {labelDisplayValue, labelProjectEnvironmentPolicy, labelRole} from '../domain/display-labels';
import {selectCheckpointForTask} from '../domain/checkpoint-projection';
import {countIncludedTaskInputImages, countIncludedTaskInputSources, countIncludedTaskInputTextFiles} from '../domain/task-input-delivery-presentation';
import {ActivityTimeline} from '../components/activity-timeline/ActivityTimeline';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import styles from '../styles/workbench.module.css';

function DataRow({label, value, mono = false}: Readonly<{label: string; value: ReactNode; mono?: boolean}>) {
  return <div className={styles.detailRow}><span className={styles.fieldLabel}>{label}</span><span className={mono ? styles.detailValueMono : styles.detailValue}>{value}</span></div>;
}

function EmptyPanel({title, detail}: Readonly<{title: string; detail: string}>) {
  return <div className={styles.emptyState}><span className={styles.emptyStateIcon}><LockKeyhole aria-hidden="true" size={18} /></span><strong>{title}</strong><span>{detail}</span></div>;
}

function qualificationTone(status: EmployeeSummary['qualification']['status']): 'info' | 'neutral' | 'success' | 'warning' | 'danger' {
  if (status === 'supported') return 'success';
  if (status === 'limited') return 'warning';
  if (status === 'unsupported') return 'danger';
  return 'info';
}

function taskTone(task: TaskSummary): 'info' | 'neutral' | 'success' | 'warning' | 'danger' {
  if (task.state === 'working') return 'info';
  if (task.state === 'candidate') return 'warning';
  if (task.state === 'completed') return 'success';
  if (task.state === 'blocked') return 'danger';
  return 'neutral';
}

function employeeEvents(overview: CompanyOverviewView, employeeId: string): ReadonlyArray<ActivityEvent> {
  return overview.recentActivity.filter(event => event.actor.id === employeeId || event.subject.id === employeeId);
}

export type MissionSubpageTab = 'inputs' | 'guidance' | 'deliveries';

export function MissionSubpage({api, companyId, overview, tab}: Readonly<{api: WorkbenchApi; companyId: string; overview: CompanyOverviewView; tab: MissionSubpageTab}>) {
  const mission = overview.mission;
  if (tab === 'inputs') return <MissionInputsPanel api={api} companyId={companyId} missionId={mission.missionId} missionState={mission.state} />;
  if (tab === 'guidance') {
    const obligations = overview.obligations;
    return <section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>指导 / 责任</span><h2 className={styles.sectionTitle}>指导与责任变化</h2></div><MessageSquare aria-hidden="true" className={styles.icon} size={18} /></div>{obligations.length > 0 ? <div className={styles.recordList}>{obligations.map(item => <div className={styles.recordRow} key={item.obligationId}><div className={styles.recordLead}><MessageSquare aria-hidden="true" size={16} /><div><strong>{item.note || '责任 ' + formatEntityId(item.obligationId)}</strong><span>{formatEntityId(item.messageId)} · 负责人 {item.ownerEmployeeId}</span></div></div><div className={styles.recordMeta}><code>{item.evidenceRef ? formatEntityId(item.evidenceRef) : '暂无证据'}</code><StatusBadge label={labelDisplayValue(item.state)} tone={item.state === 'fulfilled' ? 'success' : item.state === 'declined' ? 'danger' : 'warning'} /></div></div>)}</div> : <EmptyPanel detail="当前使命快照没有责任记录。" title="暂无指导责任" />}</section>;
  }
  return <section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>交付边界</span><h2 className={styles.sectionTitle}>使命交付</h2></div><BadgeCheck aria-hidden="true" className={styles.icon} size={18} /></div>{overview.artifacts.length > 0 ? <div className={styles.recordList}>{overview.artifacts.map(artifact => <div className={styles.recordRow} key={artifact.artifactId}><div className={styles.recordLead}><FileCheck2 aria-hidden="true" size={16} /><div><strong>{formatEntityId(artifact.artifactId)}</strong><span>{artifact.authorEmployeeId} · {artifact.bytes} · {labelDisplayValue(artifact.state)}</span></div></div><div className={styles.recordMeta}><code>{artifact.contractRevisionId ? formatEntityId(artifact.contractRevisionId) : '暂无合同版本'}</code><StatusBadge label={labelDisplayValue(artifact.verdict)} tone={artifact.verdict === 'passed' ? 'success' : artifact.verdict === 'failed' ? 'danger' : 'warning'} /></div></div>)}</div> : <EmptyPanel detail="当前使命快照没有产物；不把任务完成状态推断为交付。" title="暂无交付产物" />}</section>;
}

type MissionInputsPanelProps = Readonly<{
  api: WorkbenchApi;
  companyId: string;
  missionId: string;
  missionState: string;
}>;

const missionInputStateLabels: Readonly<Record<MissionInputState, string>> = {
  uploading: '上传未完成',
  stored: '原件已保存',
  usable: '文本表示可用',
  partial: '已保存，处理受限',
  unsupported: '格式不支持',
  rejected: '已拒绝',
};

function MissionInputsPanel({api, companyId, missionId, missionState}: MissionInputsPanelProps) {
  const query = useMissionInputs(api, companyId, missionId);
  const upload = useUploadMissionInput(api, companyId, missionId);
  const directoryUpload = useUploadMissionDirectoryInput(api, companyId, missionId);
  const [inputId, setInputId] = useState('');
  const [notice, setNotice] = useState<string | null>(null);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [retryTarget, setRetryTarget] = useState<MissionInputView | null>(null);
  const inputs = query.data ?? [];
  const latestByID = new Map<string, MissionInputView>();
  for (const input of inputs) {
    if (!latestByID.has(input.inputId)) latestByID.set(input.inputId, input);
  }
  const canUpload = ['draft', 'active', 'paused'].includes(missionState);

  async function handleFileChange(event: ChangeEvent<HTMLInputElement>): Promise<void> {
    const file = event.currentTarget.files?.item(0) ?? null;
    event.currentTarget.value = '';
    if (file === null) return;
    setNotice(null);
    setUploadError(null);
    try {
      const retryInputID = retryTarget === null ? null : Number(retryTarget.revision) > 1 ? retryTarget.inputId : null;
      const receipt = await upload.mutateAsync({
        inputId: retryTarget === null ? (inputId === '' ? null : inputId) : retryInputID,
        requestId: retryTarget?.requestId ?? 'mission-input-' + crypto.randomUUID(),
        file,
      });
      setInputId('');
      setRetryTarget(null);
      setNotice(receipt.displayName + ' 已保存为第 ' + receipt.revision + ' 版。保存不会启动使命，也不代表模型已读取。');
    } catch (error) {
      setUploadError(error instanceof Error ? error.message : '资料上传失败');
    }
  }

  async function handleDirectoryChange(event: ChangeEvent<HTMLInputElement>): Promise<void> {
    const selectedFiles = Array.from(event.currentTarget.files ?? []);
    event.currentTarget.value = '';
    if (selectedFiles.length === 0) return;
    setNotice(null);
    setUploadError(null);
    try {
      const files = selectedFiles.map(file => ({relativePath: file.webkitRelativePath, file}));
      const receipt = await directoryUpload.mutateAsync({
        inputId: inputId === '' ? null : inputId,
        requestId: 'mission-directory-' + crypto.randomUUID(),
        files,
      });
      setInputId('');
      setNotice(receipt.displayName + ' 目录快照已保存为第 ' + receipt.revision + ' 版。上传不会启动使命，也不代表模型已读取。');
    } catch (error) {
      setUploadError(error instanceof Error ? error.message : '目录快照上传失败');
    }
  }

  return <section className={styles.detailGrid}>
    <article className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div><span className={styles.cardEyebrow}>使命输入</span><h2 className={styles.sectionTitle}>资料版本</h2></div>
        <FolderOpen aria-hidden="true" className={styles.icon} size={18} />
      </div>
      <div className={styles.formStack}>
        <label className={styles.formLabel}>保存为
          <select className={styles.formField} value={inputId} onChange={event => setInputId(event.target.value)} disabled={!canUpload || upload.isPending || retryTarget !== null}>
            <option value="">新建资料</option>
            {[...latestByID.values()].map(input => <option key={input.inputId} value={input.inputId}>{input.displayName} · 当前版本 {input.revision}</option>)}
          </select>
        </label>
        <label className={styles.formLabel}>{retryTarget === null ? '选择文本、图片、PDF 或 ZIP 项目包' : '选择同一原件以继续未完成上传'}
          <input
            accept=".txt,.md,.go,.ts,.tsx,.js,.jsx,.html,.css,.json,.yaml,.yml,.toml,.sql,.rs,.py,.sh,.ps1,.csv,.png,.jpg,.jpeg,.pdf,.zip"
            className={styles.formField}
            data-testid="mission-input-file"
            disabled={!canUpload || upload.isPending || directoryUpload.isPending}
            onChange={event => { void handleFileChange(event); }}
            type="file"
          />
        </label>
        {retryTarget === null ? <label className={styles.formLabel}>选择目录快照
          <input
            className={styles.formField}
            data-testid="mission-directory-input"
            disabled={!canUpload || directoryUpload.isPending || upload.isPending}
            multiple
            onChange={event => { void handleDirectoryChange(event); }}
            ref={element => element?.setAttribute('webkitdirectory', '')}
            type="file"
          />
        </label> : null}
        <p className={styles.formHint}>单文件上限 {MAX_MISSION_INPUT_BYTES / (1024 * 1024)} MB；PDF 原件最多 6 MB；目录最多 {MAX_MISSION_DIRECTORY_FILES} 个文件、合计 {MAX_MISSION_DIRECTORY_BYTES / (1024 * 1024)} MB。单文件支持 UTF-8 文本、代码、Markdown、CSV、PNG、JPEG 和 PDF。PDF 原件与有界文本抽取记录会一起保存，抽取文本可进入 Worker；扫描页不做 OCR，原 PDF 不会作为文本发送。ZIP 项目包仅在内存中有界检查，支持文本可进入 Worker 上下文，未支持文件会单独标示；上传只保存版本，不启动员工，也不代表模型已读取。</p>
        {retryTarget !== null ? <button className={styles.textButton} disabled={upload.isPending || directoryUpload.isPending} onClick={() => setRetryTarget(null)} type="button">取消续传</button> : null}
        {upload.isPending || directoryUpload.isPending ? <div className={styles.emptyState} role="status">正在保存输入快照…</div> : null}
        {uploadError ? <p className={styles.formError} role="alert">{uploadError}</p> : null}
        {notice ? <p className={styles.operationNotice} data-testid="mission-input-upload-status" role="status">{notice}</p> : null}
      </div>
    </article>
    <article className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>来源与状态</span><h2 className={styles.sectionTitle}>已登记资料</h2></div><StatusBadge label={inputs.length + ' 个版本'} tone="info" /></div>
      {query.isPending ? <div className={styles.emptyState}>正在读取资料版本</div> : null}
      {query.isError ? <div className={styles.errorState} role="alert">资料列表读取失败：{query.error.message}</div> : null}
      {!query.isPending && !query.isError && inputs.length === 0 ? <EmptyPanel detail="上传只保存原件和版本元数据，不启动使命，也不宣称模型已经读取。" title="暂无资料" /> : null}
      {inputs.length > 0 ? <div className={styles.recordList} data-testid="mission-input-revisions">{inputs.map(input => <div className={styles.recordRow} key={input.inputId + ':' + input.revision}>
        <div className={styles.recordLead}><FolderOpen aria-hidden="true" size={16} /><div><strong>{input.displayName} · 版本 {input.revision}</strong><span>{labelDisplayValue(input.sourceKind)} · {input.mediaType} · {input.byteSize} bytes</span></div></div>
        <div className={styles.recordMeta}><StatusBadge label={missionInputStateLabels[input.state]} tone={input.state === 'usable' ? 'success' : input.state === 'partial' || input.state === 'stored' || input.state === 'uploading' ? 'warning' : 'danger'} /><code title="MissionInput ID">{input.inputId} @ {input.revision}</code><code title={input.contentDigest}>{input.contentDigest}</code>{input.state === 'uploading' || input.state === 'stored' ? <button className={styles.textButton} onClick={() => { setRetryTarget(input); setInputId(''); }} type="button">继续上传</button> : null}</div>
      </div>)}</div> : null}
    </article>
  </section>;
}

export type TaskSubpageTab = 'jobs' | 'browser';

function TaskInputManifestPanel({api, companyId, taskId}: Readonly<{api: WorkbenchApi; companyId: string; taskId: string}>) {
  const query = useTaskInputManifest(api, companyId, taskId);
  if (query.isPending) return <article className={styles.sectionCard}><div className={styles.emptyState} role="status">正在读取任务输入绑定</div></article>;
  if (query.isError) return <article className={styles.sectionCard}><div className={styles.errorState} role="alert">任务输入清单不可用：{query.error.message}</div></article>;
  const view = query.data;
  const delivered = view.deliveryStatus === 'provider_delivered' || view.deliveryStatus === 'local_context_loaded' || view.deliveryStatus === 'not_required';
  const tone = delivered ? 'success' : view.deliveryStatus === 'outcome_unknown' ? 'danger' : 'warning';
  const deliveryStatusLabel = view.deliveryStatus === 'not_required' ? '无需额外任务输入' : labelDisplayValue(view.deliveryStatus);
  const includedInputSourceCount = countIncludedTaskInputSources(view.includedInputPaths);
  const includedTextFileCount = countIncludedTaskInputTextFiles(view.includedInputPaths);
  const includedImageCount = countIncludedTaskInputImages(view.includedInputPaths);
  const includedPathsByInput = new Map<string, typeof view.includedInputPaths[number][]>();
  for (const item of view.includedInputPaths) includedPathsByInput.set(item.inputId, [...(includedPathsByInput.get(item.inputId) ?? []), item]);
  const exclusionsByInput = new Map<string, typeof view.inputExclusions[number][]>();
  for (const item of view.inputExclusions) exclusionsByInput.set(item.inputId, [...(exclusionsByInput.get(item.inputId) ?? []), item]);
  const workerDeliveryLabel = view.deliveryStatus === 'provider_delivered' ? '已发送到模型提供方'
    : view.deliveryStatus === 'local_context_loaded' ? '已载入本地 Worker 上下文'
      : view.deliveryStatus === 'not_required' ? '无需向 Worker 发送额外任务输入'
      : view.deliveryStatus === 'sending' ? '正在发送'
        : view.deliveryStatus === 'outcome_unknown' ? '发送结果未知'
          : view.deliveryStatus === 'not_sent' ? '准备完成但未送达'
            : '尚无 Worker 交付记录';
  const deliveryLabel = (inputId: string) => {
    const includedPaths = includedPathsByInput.get(inputId) ?? [];
    const exclusions = exclusionsByInput.get(inputId) ?? [];
    const sourceKind = view.manifest.candidateInputs.find(input => input.inputId === inputId)?.sourceKind;
    const isArchive = sourceKind !== undefined && isInputArchiveSource(sourceKind);
    if (includedPaths.length > 0 && isArchive) {
      const textCount = countIncludedTaskInputTextFiles(includedPaths);
      const imageCount = countIncludedTaskInputImages(includedPaths);
      const includedParts = [textCount > 0 ? `${textCount} 个文本文件` : '', imageCount > 0 ? `${imageCount} 张图片` : ''].filter(Boolean).join('、');
      return `${includedParts}纳入本轮输入 · ${workerDeliveryLabel}${exclusions.length > 0 ? ` · ${exclusions.length} 个文件未纳入` : ''}`;
    }
    if (includedPaths.length > 0) return workerDeliveryLabel;
    const reason = exclusions[0]?.reason;
    if (reason === 'context_limit') return '未纳入：超过上下文限制';
    if (reason === 'representation_not_supported') return '未纳入：当前表示不支持';
    return '尚无 Worker 交付记录';
  };
  return <article className={styles.sectionCard} data-testid="task-input-manifest"><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>任务 / 输入</span><h2 className={styles.sectionTitle}>固定的输入版本</h2></div><StatusBadge label={deliveryStatusLabel} tone={tone} /></div><div className={styles.detailRows}><DataRow label="清单摘要" value={view.manifestDigest} mono /><DataRow label="输入负载摘要" value={view.payloadDigest ?? '尚无交付回执'} mono /><DataRow label="候选输入" value={view.manifest.candidateInputs.length} /><DataRow label="未纳入输入" value={view.manifest.excludedInputs.length} /><DataRow label="模型读取状态" value={deliveryStatusLabel} /><DataRow label="有内容纳入的输入源" value={`${includedInputSourceCount} / ${view.manifest.candidateInputs.length}`} /><DataRow label="纳入文本文件" value={includedTextFileCount} /><DataRow label="纳入图片" value={includedImageCount} /><DataRow label="上下文限制排除项" value={view.inputExclusions.filter(item => item.reason === 'context_limit').length} /></div>{view.manifest.candidateInputs.length > 0 ? <div className={styles.recordList}>{view.manifest.candidateInputs.map(input => { const includedFiles = includedPathsByInput.get(input.inputId) ?? []; const excludedFiles = exclusionsByInput.get(input.inputId) ?? []; return <div className={styles.recordRow} key={input.inputId}><div className={styles.recordLead}><FolderOpen aria-hidden="true" size={16} /><div><strong>{input.displayName} · 版本 {input.revision}</strong><span>{labelDisplayValue(input.sourceKind)} · {input.mediaType} · {input.byteSize} bytes</span><span>{deliveryLabel(input.inputId)}</span>{includedFiles.length > 0 ? <span>Worker 输入：{includedFiles.slice(0, 3).map(file => file.relativePath || input.displayName).join('、')}{includedFiles.length > 3 ? `，另有 ${includedFiles.length - 3} 个` : ''}</span> : null}{excludedFiles.length > 0 ? <span>未纳入：{excludedFiles.slice(0, 3).map(file => `${file.relativePath || input.displayName}（${labelDisplayValue(file.reason)}）`).join('、')}{excludedFiles.length > 3 ? `，另有 ${excludedFiles.length - 3} 个` : ''}</span> : null}</div></div><div className={styles.recordMeta}><code title={input.contentDigest}>{input.contentDigest}</code></div></div>; })}</div> : <EmptyPanel detail={view.deliveryStatus === 'not_required' ? '此 Task 创建时没有可纳入的已验证输入；Worker 已记录无需额外输入。' : '创建此 Task 时没有可纳入的已验证输入版本。'} title={view.deliveryStatus === 'not_required' ? '无需额外输入' : '没有候选输入'} />}{view.manifest.excludedInputs.length > 0 ? <p className={styles.formHint}>另有 {view.manifest.excludedInputs.length} 个版本因状态不是 usable 而未纳入候选。</p> : null}</article>;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function decodeBase64Text(value: string): string {
  const binary = atob(value);
  const bytes = Uint8Array.from(binary, character => character.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

function formatProjectJobLogs(content: string): string {
  try {
    const parsed: unknown = JSON.parse(decodeBase64Text(content));
    if (!isRecord(parsed) || parsed.schemaVersion !== 'project-job-logs@1' || typeof parsed.stdout !== 'string' || typeof parsed.stderr !== 'string') {
      return '日志清单格式无效';
    }
    return `stdout:\n${decodeBase64Text(parsed.stdout)}\nstderr:\n${decodeBase64Text(parsed.stderr)}`;
  } catch {
    return '日志清单无法解码';
  }
}

export function TaskSubpage({api, companyId, overview, task, tab}: Readonly<{api: WorkbenchApi; companyId: string; overview: CompanyOverviewView; task: TaskSummary | null; tab: TaskSubpageTab}>) {
  const workspaceQuery = useTaskWorkspace(api, companyId, task?.taskId ?? null);
  const projectEnvironmentsQuery = useProjectEnvironments(api, companyId, tab === 'jobs');
  const jobRunsQuery = useTaskJobRuns(api, companyId, task?.taskId ?? null, tab === 'jobs');
  const crossBackendHandoversQuery = useTaskCrossBackendHandovers(api, companyId, task?.taskId ?? null, tab === 'jobs');
  const startProjectJob = useStartTaskJobRun(api);
  const stopProjectJob = useStopTaskJobRun(api);
  const createServiceBrowserSession = useCreateProjectJobBrowserSession(api);
  const createCrossBackendHandover = useCreateTaskEnvironmentHandover(api);
  const [jobScriptPath, setJobScriptPath] = useState('scripts/build.mjs');
  const [jobArguments, setJobArguments] = useState('');
  const [selectedEnvironmentRevisionId, setSelectedEnvironmentRevisionId] = useState('');
  const [selectedJobLogId, setSelectedJobLogId] = useState<string | null>(null);
  const [selectedHandoverID, setSelectedHandoverID] = useState('');
  const [handoverTargetRevisionID, setHandoverTargetRevisionID] = useState('');
  const [jobActionMessage, setJobActionMessage] = useState<string | null>(null);
  const [jobActionError, setJobActionError] = useState<string | null>(null);
  const [serviceBrowserSession, setServiceBrowserSession] = useState<Readonly<{jobId: string; session: ServiceBrowserSessionView}> | null>(null);
  const jobLogsQuery = useTaskJobLogs(api, companyId, selectedJobLogId, tab === 'jobs');
  const environmentPolicy = useDecideProjectEnvironmentPolicy(api, companyId);
  const environmentExecutorQualification = useDecideProjectEnvironmentExecutorQualification(api, companyId);
  const ensureEnvironment = useEnsureProjectEnvironment(api, companyId);
  const artifactForTask = task === null ? null : overview.artifacts.find(item => item.taskId === task.taskId) ?? null;
  const artifactQuery = useArtifactDetail(api, companyId, artifactForTask?.artifactId ?? null);
  const deliveryManifestQuery = useArtifactDeliveryManifest(api, companyId, artifactForTask?.artifactId ?? null);
  const [downloadingArtifact, setDownloadingArtifact] = useState(false);
  const [artifactDownloadError, setArtifactDownloadError] = useState<string | null>(null);
  const [environmentRationale, setEnvironmentRationale] = useState('');
  const [executorEvidenceInputId, setExecutorEvidenceInputId] = useState('');
  const [executorEvidenceInputRevision, setExecutorEvidenceInputRevision] = useState('');
  const [environmentActionMessage, setEnvironmentActionMessage] = useState<string | null>(null);
  const [environmentActionError, setEnvironmentActionError] = useState<string | null>(null);
  if (task === null) return <section className={styles.sectionCard}><EmptyPanel detail="当前快照没有可选任务对象。" title="暂无任务详情" /></section>;
  const downloadArtifact = async () => {
    setDownloadingArtifact(true);
    setArtifactDownloadError(null);
    try {
      const blob = await api.downloadArtifactPackage({companyId, artifactId: artifactForTask?.artifactId ?? ''});
      const objectUrl = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = objectUrl;
      anchor.download = 'polis-delivery.zip';
      document.body.append(anchor);
      anchor.click();
      anchor.remove();
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 0);
    } catch (error) {
      setArtifactDownloadError(error instanceof Error ? error.message : '交付包下载失败');
    } finally {
      setDownloadingArtifact(false);
    }
  };
  const decideEnvironmentPolicy = async (revisionId: string, decision: 'approved' | 'revoked') => {
    setEnvironmentActionMessage(null);
    setEnvironmentActionError(null);
    if (environmentRationale.trim() === '') {
      setEnvironmentActionError('请填写环境策略决策理由。');
      return;
    }
    try {
      await environmentPolicy.mutateAsync({revisionId, decision, rationale: environmentRationale, requestId: 'environment-policy-' + crypto.randomUUID()});
      setEnvironmentActionMessage(decision === 'approved' ? '环境策略批准已记入审计；隔离执行资格仍单独门控。' : '环境策略已撤销。');
    } catch (error) {
      setEnvironmentActionError(error instanceof Error ? error.message : '环境策略决策失败');
    }
  };
  const decideEnvironmentExecutorQualification = async (revisionId: string, decision: 'qualified' | 'revoked') => {
    setEnvironmentActionMessage(null);
    setEnvironmentActionError(null);
    if (environmentRationale.trim() === '' || executorEvidenceInputId.trim() === '' || !/^[1-9]\d*$/.test(executorEvidenceInputRevision)) {
      setEnvironmentActionError('请填写资格证据的 MissionInput ID、版本和审批理由。');
      return;
    }
    try {
      await environmentExecutorQualification.mutateAsync({
        revisionId, decision, evidenceInputId: executorEvidenceInputId.trim(), evidenceInputRevision: executorEvidenceInputRevision,
        rationale: environmentRationale, requestId: 'environment-executor-qualification-' + crypto.randomUUID(),
      });
      setEnvironmentActionMessage(decision === 'qualified' ? '资格报告已按当前执行器、主机、隔离策略和工具链指纹记入审计。' : '当前执行器资格已撤销。');
    } catch (error) {
      setEnvironmentActionError(error instanceof Error ? error.message : '执行器资格决策失败');
    }
  };
  const requestEnvironmentPreparation = async (revisionId: string) => {
    setEnvironmentActionMessage(null);
    setEnvironmentActionError(null);
    try {
      const run = await ensureEnvironment.mutateAsync({revisionId, requestId: 'environment-ensure-' + crypto.randomUUID()});
      setEnvironmentActionMessage(`准备请求 ${formatEntityId(run.runId)}：${labelDisplayValue(run.state)} · ${labelDisplayValue(run.reasonCode)}`);
    } catch (error) {
      setEnvironmentActionError(error instanceof Error ? error.message : '环境准备请求失败');
    }
  };
  const readyJobEnvironments = (projectEnvironmentsQuery.data ?? []).filter(item => item.sourceBindingStatus === 'bound'
    && item.policyDecision === 'approved' && item.executorQualification === 'qualified' && item.preparationState === 'ready');
  const selectedJobEnvironment = readyJobEnvironments.find(item => item.revisionId === selectedEnvironmentRevisionId) ?? readyJobEnvironments[0] ?? null;
  const taskJobs = jobRunsQuery.data ?? [];
  const taskHandovers = crossBackendHandoversQuery.data ?? [];
  const activeTaskSession = overview.employees.find(employee => employee.employeeId === task.ownerEmployeeId
    && employee.currentTask?.id === task.taskId && employee.sessionState === 'active' && employee.sessionId !== null) ?? null;
  const latestTaskJob = taskJobs[0] ?? null;
  const latestSourceEnvironment = latestTaskJob === null ? null : projectEnvironmentsQuery.data?.find(item => item.revisionId === latestTaskJob.environmentRevisionId) ?? null;
  const crossBackendHandoverRequired = latestSourceEnvironment !== null && selectedJobEnvironment !== null && latestSourceEnvironment.profileId !== selectedJobEnvironment.profileId;
  const selectableHandovers = crossBackendHandoverRequired && latestTaskJob !== null && selectedJobEnvironment !== null
    ? taskHandovers.filter(item => item.sourceJobId === latestTaskJob.jobId && item.targetEnvironmentRevisionId === selectedJobEnvironment.revisionId)
    : [];
  const selectedHandover = selectableHandovers.find(item => item.handoverId === selectedHandoverID) ?? selectableHandovers[0] ?? null;
  const latestJobKnownTerminalOrNone = latestTaskJob === null || latestTaskJob.state === 'exited' || latestTaskJob.state === 'failed' || latestTaskJob.state === 'cancelled';
  const taskHasUnknownJob = taskJobs.some(job => job.state === 'outcome_unknown');
  const canCreateProjectJobSession = overview.mission.state === 'active' && latestJobKnownTerminalOrNone && !taskHasUnknownJob;
  const canStartProjectJob = task.kind === 'compat' && task.state === 'working' && overview.mission.state === 'active' && selectedJobEnvironment !== null
    && latestJobKnownTerminalOrNone && !taskHasUnknownJob && (activeTaskSession !== null || canCreateProjectJobSession)
    && (!crossBackendHandoverRequired || selectedHandover !== null);
  const handoverTargetsForJob = (job: JobRunView): ReadonlyArray<ProjectEnvironmentRevisionView> => {
    const sourceEnvironment = projectEnvironmentsQuery.data?.find(item => item.revisionId === job.environmentRevisionId);
    if (sourceEnvironment === undefined) return [];
    return readyJobEnvironments.filter(target => target.revisionId !== job.environmentRevisionId
      && target.missionId === sourceEnvironment.missionId
      && target.profileId !== sourceEnvironment.profileId
      && target.sourceRevisionSha256 === sourceEnvironment.sourceRevisionSha256
      && target.packageJsonSha256 === sourceEnvironment.packageJsonSha256
      && target.lockfileSha256 === sourceEnvironment.lockfileSha256);
  };
  const handoverSourceOptions = latestTaskJob !== null && (latestTaskJob.state === 'exited' || latestTaskJob.state === 'failed' || latestTaskJob.state === 'cancelled')
    ? [latestTaskJob]
    : [];
  const handoverSourceJob = handoverSourceOptions[0] ?? null;
  const handoverTargets = handoverSourceJob === null ? [] : handoverTargetsForJob(handoverSourceJob);
  const selectedHandoverTarget = handoverTargets.find(item => item.revisionId === handoverTargetRevisionID) ?? handoverTargets[0] ?? null;
  const createTaskEnvironmentHandover = async (job: JobRunView, target: ProjectEnvironmentRevisionView) => {
    setJobActionError(null);
    setJobActionMessage(null);
    try {
      const handover: CrossBackendHandoverView = await createCrossBackendHandover.mutateAsync({
        companyId, taskId: task.taskId, sourceJobId: job.jobId, targetEnvironmentRevisionId: target.revisionId,
        requestId: `handover-create-${crypto.randomUUID()}`,
      });
      setSelectedEnvironmentRevisionId(target.revisionId);
      setSelectedHandoverID(handover.handoverId);
      setJobActionMessage(`接续记录 ${formatEntityId(handover.handoverId)} 已创建；工作区版本 ${handover.workspaceRevision}；记录 SHA-256 ${handover.recordSha256}`);
    } catch (error) {
      setJobActionError(error instanceof Error ? error.message : '跨后端接续记录创建失败');
    }
  };
  const startProjectJobRun = async () => {
    setJobActionError(null);
    setJobActionMessage(null);
    if (selectedJobEnvironment === null || !canStartProjectJob) {
      setJobActionError('需要活动 Mission 和已批准、已准备的隔离环境；跨平台启动还需要有效接续记录。');
      return;
    }
    try {
      const args = jobArguments.split('\n').map(argument => argument.trim()).filter(argument => argument !== '');
      const receipt = await startProjectJob.mutateAsync({
        companyId, taskId: task.taskId, sessionId: activeTaskSession?.sessionId ?? '',
        environmentRevisionId: selectedJobEnvironment.revisionId, scriptPath: jobScriptPath, args,
        ...(crossBackendHandoverRequired && selectedHandover !== null ? {handoverId: selectedHandover.handoverId} : {}),
        requestId: `job-start-${crypto.randomUUID()}`,
      });
      setJobActionMessage(`JobRun ${formatEntityId(receipt.jobId)}：${labelDisplayValue(receipt.state)}`);
      setSelectedJobLogId(receipt.jobId);
    } catch (error) {
      setJobActionError(error instanceof Error ? error.message : '隔离批处理启动失败');
    }
  };
  const startProjectServiceJobRun = async (serviceId: string) => {
    setJobActionError(null);
    setJobActionMessage(null);
    if (selectedJobEnvironment === null || !canStartProjectJob) {
      setJobActionError('需要活动 Mission 和已批准、已准备的隔离环境；跨平台启动还需要有效接续记录。');
      return;
    }
    try {
      const receipt = await startProjectJob.mutateAsync({
        companyId, taskId: task.taskId, sessionId: activeTaskSession?.sessionId ?? '',
        environmentRevisionId: selectedJobEnvironment.revisionId, kind: 'service', serviceId,
        ...(crossBackendHandoverRequired && selectedHandover !== null ? {handoverId: selectedHandover.handoverId} : {}),
        requestId: `service-start-${crypto.randomUUID()}`,
      });
      setJobActionMessage(`服务 JobRun ${formatEntityId(receipt.jobId)}：${labelDisplayValue(receipt.state)} / ${labelDisplayValue(receipt.readiness)}`);
    } catch (error) {
      setJobActionError(error instanceof Error ? error.message : '隔离服务启动失败');
    }
  };
  const stopProjectJobRun = async (jobId: string) => {
    setJobActionError(null);
    setJobActionMessage(null);
    try {
      const receipt = await stopProjectJob.mutateAsync({companyId, jobId, requestId: `job-stop-${crypto.randomUUID()}`});
      if (serviceBrowserSession?.jobId === jobId) setServiceBrowserSession(null);
      setJobActionMessage(`JobRun ${formatEntityId(receipt.jobId)}：${labelDisplayValue(receipt.state)}`);
    } catch (error) {
      setJobActionError(error instanceof Error ? error.message : '隔离作业停止失败');
    }
  };
  const requestServiceBrowserSession = async (jobId: string) => {
    setJobActionError(null);
    setJobActionMessage(null);
    try {
      const session = await createServiceBrowserSession.mutateAsync({companyId, jobId, requestId: `browser-session-${crypto.randomUUID()}`});
      setServiceBrowserSession({jobId, session});
      setJobActionMessage('独立浏览器入口已生成，五分钟后过期；停止服务会立即撤销。');
    } catch (error) {
      setJobActionError(error instanceof Error ? error.message : '独立浏览器入口创建失败');
    }
  };
  const taskEvents = overview.recentActivity.filter(event => event.subject.id === task.taskId);
  const artifact = artifactForTask;
  const checkpoint = selectCheckpointForTask(overview.checkpoints, task.taskId, artifact);
  if (tab === 'jobs') {
    return <section className={styles.detailGrid}>
      <article className={styles.sectionCard}>
        <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>R2 / 跨后端接续</span><h2 className={styles.sectionTitle}>已保存状态接班</h2></div><Handshake aria-hidden="true" className={styles.icon} size={18} /></div>
        <p className={styles.formHint}>接续只记录已保存的输入清单和工作区版本。创建时要求 Mission 已暂停、所有 WorkerSession 已停止、旧作业均为已知终态，且目标 Windows/Linux Node 环境已批准、具备资格并准备就绪。</p>
        {crossBackendHandoversQuery.isError ? <div className={styles.errorState} role="alert">接续记录读取失败：{crossBackendHandoversQuery.error.message}</div> : null}
        <div className={styles.formStack}>
          <label className={styles.formLabel}>最新已知终态源作业<input className={styles.formField} readOnly value={handoverSourceJob === null ? '当前没有可接续的终态 JobRun' : `${formatEntityId(handoverSourceJob.jobId)} · ${handoverSourceJob.state} · ${projectEnvironmentsQuery.data?.find(item => item.revisionId === handoverSourceJob.environmentRevisionId)?.profileId ?? '未知环境'}`} /></label>
          <label className={styles.formLabel}>已核准目标环境<select className={styles.formField} disabled={handoverTargets.length === 0} onChange={event => setHandoverTargetRevisionID(event.target.value)} value={selectedHandoverTarget?.revisionId ?? ''}>
            {handoverTargets.length === 0 ? <option value="">没有同源且已就绪的另一平台环境</option> : handoverTargets.map(target => <option key={target.revisionId} value={target.revisionId}>{target.profileId} · {formatEntityId(target.revisionId)}</option>)}
          </select></label>
          <button className={styles.textButton} disabled={overview.mission.state !== 'paused' || handoverSourceJob === null || selectedHandoverTarget === null || createCrossBackendHandover.isPending} onClick={() => { if (handoverSourceJob !== null && selectedHandoverTarget !== null) void createTaskEnvironmentHandover(handoverSourceJob, selectedHandoverTarget); }} type="button">{createCrossBackendHandover.isPending ? '正在记录接续…' : '创建一次性交接记录'}</button>
          {overview.mission.state !== 'paused' ? <p className={styles.formHint}>请先正式暂停 Mission；控制面会再次核验所有会话与作业状态。</p> : null}
          {crossBackendHandoverRequired ? <label className={styles.formLabel}>用于下一次 JobRun 的接续记录<select className={styles.formField} disabled={selectableHandovers.length === 0} onChange={event => setSelectedHandoverID(event.target.value)} value={selectedHandover?.handoverId ?? ''}>
            {selectableHandovers.length === 0 ? <option value="">请为最新源作业创建接续记录</option> : selectableHandovers.map(item => <option key={item.handoverId} value={item.handoverId}>{formatEntityId(item.handoverId)} · {item.sourceProfileId} → {item.targetProfileId}</option>)}
          </select></label> : null}
        </div>
        {taskHandovers.length > 0 ? <div className={styles.recordList}>{taskHandovers.map(item => <div className={styles.recordRow} key={item.handoverId}>
          <div className={styles.recordLead}><Handshake aria-hidden="true" size={16} /><div><strong>{item.sourceProfileId} → {item.targetProfileId}</strong><span>源作业 {formatEntityId(item.sourceJobId)} · 目标环境 {formatEntityId(item.targetEnvironmentRevisionId)}</span><span>工作区版本 {item.workspaceRevision} · 清单 {item.taskInputManifestSha256}</span><span>不可变记录 SHA-256 {item.recordSha256}</span></div></div>
          <StatusBadge label={selectableHandovers.some(option => option.handoverId === item.handoverId) ? '可用于下一次 JobRun' : '已记录'} tone="info" />
        </div>)}</div> : null}
      </article>
      <article className={styles.sectionCard}>
        <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>作业 / 执行</span><h2 className={styles.sectionTitle}>运行作业</h2></div><Wrench aria-hidden="true" className={styles.icon} size={18} /></div>
        <div className={styles.detailRows}><DataRow label="任务状态" value={labelDisplayValue(task.state)} /><DataRow label="生成代数" value={task.generation} mono /><DataRow label="工作区版本" value={task.workspaceRevision ?? '不可得'} mono /><DataRow label="检查点" value={checkpoint?.checkpointId ?? '暂无检查点'} mono /><DataRow label="检查点状态" value={checkpoint ? `${checkpoint.kind} / ${labelDisplayValue(checkpoint.qualificationState)}` : '暂无检查点'} /><DataRow label="活动 WorkerSession" value={activeTaskSession?.sessionId ? formatEntityId(activeTaskSession.sessionId) : '不可用'} mono /></div>
        {workspaceQuery.isPending ? <div className={styles.emptyState}>正在读取授权工作区</div> : workspaceQuery.isError ? <div className={styles.errorState} role="alert">工作区读取失败：{workspaceQuery.error.message}</div> : workspaceQuery.data ? <div className={styles.detailRows}><DataRow label="CAS 指纹" value={workspaceQuery.data.digest} mono /><DataRow label="变更文件" value={workspaceQuery.data.changedFiles.join(', ')} /><DataRow label="预览" value={workspaceQuery.data.files[0]?.content || '内容不可得'} /></div> : null}
        <div className={styles.formStack}>
          {task.kind === 'compat' ? <>
            <label className={styles.formLabel}>已资格环境<select className={styles.formField} disabled={readyJobEnvironments.length === 0} onChange={event => setSelectedEnvironmentRevisionId(event.target.value)} value={selectedJobEnvironment?.revisionId ?? ''}>{readyJobEnvironments.length === 0 ? <option value="">没有可用环境</option> : readyJobEnvironments.map(item => <option key={item.revisionId} value={item.revisionId}>{formatEntityId(item.revisionId)} · {item.profileId}</option>)}</select></label>
            <label className={styles.formLabel}>项目内脚本路径<input className={styles.formField} maxLength={260} onChange={event => setJobScriptPath(event.target.value)} value={jobScriptPath} /></label>
            <label className={styles.formLabel}>参数（每行一个）<textarea className={styles.formField} maxLength={8192} onChange={event => setJobArguments(event.target.value)} rows={3} value={jobArguments} /></label>
            <p className={styles.formHint}>仅在 Task 工作中、Mission 活动且 Windows/Linux Node 来源、策略、执行资格和准备状态均通过时开放批处理脚本。缺少活动 WorkerSession 时，控制面为本次 JobRun 建立仅用于受控项目作业的会话，不会发起模型调用。每行作为独立参数传入 Node；执行环境拒绝网络访问。</p>
            <button className={styles.textButton} disabled={!canStartProjectJob || startProjectJob.isPending} onClick={() => { void startProjectJobRun(); }} type="button">{startProjectJob.isPending ? '正在启动…' : '启动隔离批处理'}</button>
            {selectedJobEnvironment?.profileId === 'windows-node-npm@1' ? selectedJobEnvironment.policyManifest?.services?.map(service => <button className={styles.textButton} disabled={!canStartProjectJob || startProjectJob.isPending} key={service.id} onClick={() => { void startProjectServiceJobRun(service.id); }} type="button">{startProjectJob.isPending ? '正在启动…' : `启动隔离服务 ${service.id}`}</button>) : null}
          </> : <p className={styles.formHint}>当前 Worker Task 类型没有已资格的批处理 JobRun 配置。</p>}
          {jobActionMessage ? <p className={styles.formHint} role="status">{jobActionMessage}</p> : null}
          {jobActionError ? <div className={styles.errorState} role="alert">{jobActionError}</div> : null}
          {taskJobs.filter(job => job.kind === 'service' && job.state === 'running' && job.readiness === 'ready').map(job => <div className={styles.formStack} key={`browser-${job.jobId}`}>
            <button className={styles.textButton} disabled={createServiceBrowserSession.isPending} onClick={() => { void requestServiceBrowserSession(job.jobId); }} type="button">{createServiceBrowserSession.isPending ? '正在创建入口…' : '创建独立浏览器验证入口'}</button>
            {serviceBrowserSession?.jobId === job.jobId && Date.parse(serviceBrowserSession.session.expiresAt) > Date.now() ? <p className={styles.formHint}>仅供本机访问；有效至 {new Date(serviceBrowserSession.session.expiresAt).toLocaleTimeString()}。停止服务会立即撤销。 <a href={serviceBrowserSession.session.url} rel="noopener noreferrer" target="_blank">在浏览器中打开</a></p> : null}
          </div>)}
        </div>
      </article>
      <article className={styles.sectionCard}>
        <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>环境 / 资格</span><h2 className={styles.sectionTitle}>项目环境版本</h2></div><StatusBadge label={`${projectEnvironmentsQuery.data?.length ?? 0} 个版本`} tone="info" /></div>
        {projectEnvironmentsQuery.isPending ? <div className={styles.emptyState} role="status">正在读取项目环境</div> : projectEnvironmentsQuery.isError ? <div className={styles.errorState} role="alert">环境状态读取失败：{projectEnvironmentsQuery.error.message}</div> : projectEnvironmentsQuery.data?.length ? <>
          <div className={styles.formStack}>
            <label className={styles.formLabel}>环境策略决策理由<input className={styles.formField} maxLength={512} onChange={event => setEnvironmentRationale(event.target.value)} value={environmentRationale} /></label>
            <p className={styles.formHint}>策略批准只绑定此版本的源码、lockfile、依赖来源、Node/npm 工具链和策略摘要；尚未通过隔离资格时不会运行 npm。</p>
            <label className={styles.formLabel}>主机资格报告 MissionInput ID<input className={styles.formField} maxLength={96} onChange={event => setExecutorEvidenceInputId(event.target.value)} value={executorEvidenceInputId} /></label>
            <label className={styles.formLabel}>主机资格报告版本<input className={styles.formField} inputMode="numeric" maxLength={19} onChange={event => setExecutorEvidenceInputRevision(event.target.value)} value={executorEvidenceInputRevision} /></label>
            <p className={styles.formHint}>先将 `polis-node-executor-qualification@1` JSON 报告和其引用的检查证据作为 MissionInputs 上传到该环境所属的 Mission，再填写报告的 ID 和版本。此操作只记录人工审查；不会运行 WFP、Node/npm 或项目代码。</p>
            {environmentActionMessage ? <p className={styles.formHint} role="status">{environmentActionMessage}</p> : null}
            {environmentActionError ? <div className={styles.errorState} role="status">{environmentActionError}</div> : null}
          </div>
          <div className={styles.recordList}>{projectEnvironmentsQuery.data.map(item => <div className={styles.recordRow} key={item.revisionId}>
            <div className={styles.recordLead}><Wrench aria-hidden="true" size={16} /><div>
              <strong>{item.profileId} · {formatEntityId(item.revisionId)}</strong>
              <span>源码 {item.sourceRevisionSha256.slice(0, 12)} · lockfile {item.lockfileSha256.slice(0, 12)}</span>
              <span>来源 {item.sourceBindingStatus === 'bound' ? `${formatEntityId(item.missionId)} / ${formatEntityId(item.sourceInputId)} @${item.sourceInputRevision} · ${item.projectRootRelative}` : '未绑定到已保存的目录输入'}</span>
              <span>策略摘要 {item.policySha256.slice(0, 12)} · 工具链 {item.toolchainSha256.slice(0, 12)}</span>
              <span>执行器 {item.executorFingerprintSha256?.slice(0, 12) ?? '不可用'} · 主机 {item.hostFingerprintSha256?.slice(0, 12) ?? '不可用'} · 隔离策略 {item.isolationPolicySha256?.slice(0, 12) ?? '不可用'}</span>
              {item.executorEvidenceInputId && item.executorEvidenceInputRevision && item.executorEvidenceSha256 ? <span>资格报告 {formatEntityId(item.executorEvidenceInputId)} @{item.executorEvidenceInputRevision} · SHA-256 {item.executorEvidenceSha256}</span> : null}
              {item.policyManifest ? <span>{labelProjectEnvironmentPolicy(item.policyManifest)} · {item.policyManifest.installPolicy} · lifecycle scripts {item.policyManifest.lifecycleScriptsPolicy} · timeout {item.policyManifest.timeoutMs} ms</span> : <span>策略清单未验证，不能批准</span>}
              {item.policyManifest?.services?.map(service => <span key={service.id}>Service {service.id} - {service.scriptPath} - health {service.probe.bindAddress}:{service.probe.port}{service.probe.path} - HTTP {service.probe.expectedStatusCode} - response SHA-256 {service.probe.expectedBodySha256}</span>)}
              <span>策略 {labelDisplayValue(item.policyDecision)} · 执行器 {labelDisplayValue(item.executorQualification)}</span>
              <span>准备状态 {labelDisplayValue(item.preparationState)} · {labelDisplayValue(item.preparationReason)}</span>
              {item.preparationRunId ? <span>准备运行 {formatEntityId(item.preparationRunId)}</span> : null}
            </div></div>
            <div className={styles.recordMeta}>
              <StatusBadge label={labelDisplayValue(item.preparationState)} tone={item.preparationState === 'ready' && item.policyDecision === 'approved' && item.executorQualification === 'qualified' ? 'success' : item.preparationState === 'failed' || item.preparationState === 'outcome_unknown' ? 'danger' : 'warning'} />
              {item.policyDecision === 'approved' || item.policyDecision === 'revocation_required'
                ? <button className={styles.textButton} disabled={environmentPolicy.isPending || environmentRationale.trim() === ''} onClick={() => { void decideEnvironmentPolicy(item.revisionId, 'revoked'); }} type="button">撤销策略</button>
                : <button className={styles.textButton} disabled={environmentPolicy.isPending || environmentRationale.trim() === '' || item.sourceBindingStatus !== 'bound' || item.policyManifest === null || item.policyDecision === 'unverified' || item.policyDecision === 'source_unverified'} onClick={() => { void decideEnvironmentPolicy(item.revisionId, 'approved'); }} type="button">批准此版本策略</button>}
              {item.sourceBindingStatus === 'bound' && item.policyManifest !== null
                ? <button className={styles.textButton} disabled={environmentExecutorQualification.isPending || environmentRationale.trim() === '' || executorEvidenceInputId.trim() === '' || !/^[1-9]\d*$/.test(executorEvidenceInputRevision)} onClick={() => { void decideEnvironmentExecutorQualification(item.revisionId, item.executorQualification === 'qualified' ? 'revoked' : 'qualified'); }} type="button">{item.executorQualification === 'qualified' ? '撤销此执行器资格' : '审查并记录执行器资格'}</button>
                : null}
              {item.policyDecision === 'approved' && item.sourceBindingStatus === 'bound' && item.policyManifest !== null && item.executorQualification === 'qualified' && item.preparationState !== 'ready'
                ? <button className={styles.textButton} disabled={ensureEnvironment.isPending} onClick={() => { void requestEnvironmentPreparation(item.revisionId); }} type="button">准备此环境</button>
                : null}
            </div>
          </div>)}</div>
        </> : <EmptyPanel detail="尚未登记项目环境版本；环境准备与文件/网络隔离资格未就绪时不会启动项目命令。" title="没有项目环境版本" />}
      </article>
      <article className={styles.sectionCard}>
        <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>作业 / 持久状态</span><h2 className={styles.sectionTitle}>JobRun 与服务</h2></div><StatusBadge label={`${jobRunsQuery.data?.length ?? 0} 条记录`} tone="info" /></div>
        {jobRunsQuery.isPending ? <div className={styles.emptyState} role="status">正在读取 JobRun</div> : jobRunsQuery.isError ? <div className={styles.errorState} role="alert">JobRun 读取失败：{jobRunsQuery.error.message}</div> : jobRunsQuery.data?.length ? <div className={styles.recordList}>{jobRunsQuery.data.map(job => <div className={styles.recordRow} key={job.jobId}><div className={styles.recordLead}><Wrench aria-hidden="true" size={16} /><div><strong>{job.kind} · {formatEntityId(job.jobId)}</strong><span>状态 {labelDisplayValue(job.state)} · 就绪 {labelDisplayValue(job.readiness)}</span>{job.serviceId === 'legacy:unattributed' ? <span>历史服务 · 未归属</span> : job.serviceId ? <span>固定服务定义 {formatEntityId(job.serviceId)}</span> : null}<span>环境 {formatEntityId(job.environmentRevisionId)} · 会话 {formatEntityId(job.sessionId)}</span><span>stdout offset {job.stdoutOffset} · stderr offset {job.stderrOffset}{job.logsTruncated ? ' · 日志已截断' : ''}{job.logGap ? ' · 日志存在缺口' : ''}</span>{job.logManifestSha256 ? <span>日志清单 {job.logManifestSha256}</span> : null}{job.serviceEndpoint ? <span>服务 generation {job.serviceEndpoint.generation} · {job.serviceEndpoint.bindAddress}:{job.serviceEndpoint.port} · {labelDisplayValue(job.serviceEndpoint.readiness)}</span> : null}{job.exitCode !== null ? <span>退出码 {job.exitCode}</span> : null}<div className={styles.recordActions}>{job.logManifestSha256 ? <button className={styles.textButton} onClick={() => setSelectedJobLogId(job.jobId)} type="button">读取日志</button> : null}{job.state === 'running' || job.state === 'starting' || job.state === 'outcome_unknown' || (job.kind === 'service' && job.serviceId !== 'legacy:unattributed' && (job.state === 'exited' || job.state === 'failed' || job.state === 'cancelled') && job.serviceEndpoint?.readiness === 'unhealthy') ? <button className={styles.textButton} disabled={stopProjectJob.isPending} onClick={() => { void stopProjectJobRun(job.jobId); }} type="button">{job.state === 'exited' || job.state === 'failed' || job.state === 'cancelled' ? '重试端点撤销' : job.state === 'outcome_unknown' ? '重试停止' : '停止'}</button> : null}</div></div></div><StatusBadge label={labelDisplayValue(job.state)} tone={job.state === 'exited' && job.exitCode === 0 ? 'success' : job.state === 'exited' || job.state === 'failed' || job.state === 'cancelled' || job.state === 'outcome_unknown' ? 'danger' : 'warning'} /></div>)}</div> : <EmptyPanel detail="当前任务没有持久 JobRun；没有合格隔离执行器时不会启动项目命令。" title="暂无运行作业" />}
        {selectedJobLogId ? <div className={styles.formStack}><div>{jobLogsQuery.isPending ? <div className={styles.emptyState} role="status">正在读取作业日志</div> : jobLogsQuery.isError ? <div className={styles.errorState} role="alert">日志读取失败：{jobLogsQuery.error.message}</div> : jobLogsQuery.data ? <><DataRow label="Manifest SHA-256" value={jobLogsQuery.data.manifestSha256 || '暂无日志内容'} mono /><pre className={styles.codePreview}>{formatProjectJobLogs(jobLogsQuery.data.content)}</pre></> : null}</div></div> : null}
      </article>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>任务活动</span><h2 className={styles.sectionTitle}>任务相关事件</h2></div><History aria-hidden="true" className={styles.icon} size={18} /></div>{taskEvents.length > 0 ? <ActivityTimeline compact events={taskEvents} /> : <EmptyPanel detail="当前活动快照没有直接绑定到该任务的事件。" title="暂无任务事件" />}</article>
      {task.kind === 'compat' ? <TaskInputManifestPanel api={api} companyId={companyId} taskId={task.taskId} /> : null}
    </section>;
  }
  const artifacts = overview.artifacts.filter(artifact => artifact.taskId === task.taskId);
  return <section className={styles.detailGrid}>
    <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>浏览器 / 产物</span><h2 className={styles.sectionTitle}>测试与预览边界</h2></div><FolderOpen aria-hidden="true" className={styles.icon} size={18} /></div>{artifacts.length > 0 ? <div className={styles.recordList}>{artifacts.map(artifact => <div className={styles.recordRow} key={artifact.artifactId}><div className={styles.recordLead}><BadgeCheck aria-hidden="true" size={16} /><div><strong>{formatEntityId(artifact.artifactId)}</strong><span>{artifact.digest} · {artifact.bytes}</span></div></div><StatusBadge label={labelDisplayValue(artifact.verdict)} tone={artifact.verdict === 'passed' ? 'success' : artifact.verdict === 'failed' ? 'danger' : 'warning'} /></div>)}</div> : <EmptyPanel detail="当前只读数据没有该任务的预览地址或产物。" title="暂无可预览产物" />}{artifactQuery.isPending ? <div className={styles.emptyState}>正在读取产物内容</div> : artifactQuery.isError ? <div className={styles.errorState} role="alert">产物读取失败：{artifactQuery.error.message}</div> : artifactQuery.data ? <div className={styles.detailRows}><DataRow label="内容是否可用" value={artifactQuery.data.contentAvailable ? '是' : '否'} /><DataRow label="内容预览" value={artifactQuery.data.content || '内容不可得'} /></div> : null}{deliveryManifestQuery.isPending ? <div className={styles.emptyState}>正在读取交付清单</div> : deliveryManifestQuery.isError ? <div className={styles.errorState} role="alert">交付清单不可用：{deliveryManifestQuery.error.message}</div> : deliveryManifestQuery.data ? <><div className={styles.detailRows}><DataRow label="Manifest SHA-256" value={deliveryManifestQuery.data.manifestSha256} mono /><DataRow label="内容摘要" value={deliveryManifestQuery.data.manifest.content.sha256} mono /><DataRow label="内容大小" value={deliveryManifestQuery.data.manifest.content.byteSize + ' bytes'} /><DataRow label="检查点 / 验证回执" value={`${deliveryManifestQuery.data.manifest.qualification.checkpointId} / ${deliveryManifestQuery.data.manifest.qualification.validationReceiptId}`} mono /><DataRow label="工作区版本 / Runner" value={`${deliveryManifestQuery.data.manifest.qualification.workspaceRevision} / ${deliveryManifestQuery.data.manifest.qualification.runnerRevision}`} /></div><button className={styles.textButton} disabled={downloadingArtifact} onClick={() => { void downloadArtifact(); }} type="button">{downloadingArtifact ? '正在准备交付包…' : '下载含完整 Manifest 的 ZIP'}</button>{artifactDownloadError ? <div className={styles.errorState} role="alert">{artifactDownloadError}</div> : null}</> : null}</article>
    <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>验收边界</span><h2 className={styles.sectionTitle}>验收不是预览</h2></div><ShieldAlert aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="验收状态" value={labelDisplayValue(task.acceptance)} /><DataRow label="合同版本" value={task.contractRevisionId ?? '不可得'} mono /><DataRow label="检查点" value={checkpoint?.checkpointId ?? '暂无检查点'} mono /><DataRow label="预览状态" value="未提供" /><DataRow label="最终验收" value={task.acceptance === 'passed' ? '已通过' : '未通过'}/></div></article>
  </section>;
}

export type EmployeeSubpageTab = 'responsibilities' | 'collaboration' | 'memory' | 'handover' | 'tools' | 'workspace';

export function EmployeeSubpage({overview, employee, tab}: Readonly<{overview: CompanyOverviewView; employee: EmployeeSummary; tab: EmployeeSubpageTab}>) {
  const currentTask = employee.currentTask === null ? null : overview.tasks.find(task => task.taskId === employee.currentTask?.id) ?? null;
  const currentArtifact = currentTask === null ? null : overview.artifacts.find(artifact => artifact.taskId === currentTask.taskId) ?? null;
  const currentCheckpoint = currentTask === null ? null : selectCheckpointForTask(overview.checkpoints, currentTask.taskId, currentArtifact);
  if (tab === 'responsibilities') {
    return <section className={styles.detailGrid}>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>职责</span><h2 className={styles.sectionTitle}>责任边界</h2></div><Handshake aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="员工身份" value={employee.employeeId} mono /><DataRow label="岗位" value={labelRole(employee.role)} /><DataRow label="岗位版本" value={employee.roleRevision ?? '不可得'} mono /><DataRow label="当前任务" value={employee.currentTask?.label ?? '当前无任务'} /><DataRow label="待处理责任" value={employee.openObligationCount} /></div></article>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>资格</span><h2 className={styles.sectionTitle}>资格与证据</h2></div><BadgeCheck aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="状态" value={<StatusBadge label={labelDisplayValue(employee.qualification.status)} tone={qualificationTone(employee.qualification.status)} />} /><DataRow label="证据" value={employee.qualification.evidenceId ? formatEntityId(employee.qualification.evidenceId) : '不可得'} mono /><DataRow label="政策版本" value={employee.qualification.policyRevision ?? '不可得'} mono /><DataRow label="状态原因" value={employee.status.reason} /></div></article>
    </section>;
  }
  if (tab === 'collaboration') {
    const events = employeeEvents(overview, employee.employeeId);
    return <section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>协作</span><h2 className={styles.sectionTitle}>消息与协作事件</h2></div><MessageSquare aria-hidden="true" className={styles.icon} size={18} /></div>{events.length > 0 ? <ActivityTimeline events={events} /> : <EmptyPanel detail="当前快照没有与该员工绑定的消息或活动事件。" title="暂无协作事件" />}</section>;
  }
  if (tab === 'memory') {
    return <section className={styles.detailGrid}>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>记忆 / 连续性</span><h2 className={styles.sectionTitle}>连续性边界</h2></div><History aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="员工会话" value={employee.sessionId ? formatEntityId(employee.sessionId) : '未启动'} mono /><DataRow label="会话周期" value={employee.epoch} mono /><DataRow label="当前任务" value={employee.currentTask?.label ?? '无'} /><DataRow label="记忆读取接口" value="未提供" /><DataRow label="读取行为" value="不从界面推断记忆内容" /></div></article>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>检查点</span><h2 className={styles.sectionTitle}>可恢复记录</h2></div><FileCheck2 aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="快照游标" value={formatEntityId(overview.meta.snapshotCursor)} mono /><DataRow label="实体版本" value={overview.meta.entityRevision} mono /><DataRow label="检查点" value={currentCheckpoint?.checkpointId ?? '暂无检查点'} mono /><DataRow label="检查点状态" value={currentCheckpoint ? `${currentCheckpoint.kind} / ${labelDisplayValue(currentCheckpoint.qualificationState)}` : '暂无检查点'} /><DataRow label="最终验收" value="独立字段，不由检查点替代" /></div></article>
    </section>;
  }
  if (tab === 'handover') {
    return <section className={styles.detailGrid}>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>交接</span><h2 className={styles.sectionTitle}>交接连续性</h2></div><Handshake aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="逻辑身份" value={employee.employeeId} mono /><DataRow label="会话状态" value={labelDisplayValue(employee.sessionState)} /><DataRow label="会话周期" value={employee.epoch} mono /><DataRow label="交接就绪" value={employee.sessionState === 'stopped' ? '可继续查看' : '未声明'} /><DataRow label="接班人" value="当前只读数据未提供" /></div></article>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>写入隔离</span><h2 className={styles.sectionTitle}>旧写入者边界</h2></div><ShieldAlert aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.checkList}><div><Check aria-hidden="true" size={15} /><span>会话周期代表一次会话实例，不是新员工</span></div><div><Check aria-hidden="true" size={15} /><span>停止与接班恢复分开观察</span></div><div><ShieldAlert aria-hidden="true" size={15} /><span>旧写入者拒绝不简化为网络失败</span></div></div></article>
    </section>;
  }
  if (tab === 'tools') {
    return <section className={styles.resourceGrid}>
      <article className={styles.resourceCard}><span className={styles.cardEyebrow}>模型配置</span><strong>{employee.profile ?? '不可得'}</strong><p>员工会话配置</p><StatusBadge label={employee.status.activeModelRequests + ' 个进行中'} tone={employee.status.activeModelRequests === '0' ? 'neutral' : 'warning'} /></article>
      <article className={styles.resourceCard}><span className={styles.cardEyebrow}>工具调用</span><strong>{employee.status.inFlightTools}</strong><p>执行中的工具</p><StatusBadge label={labelDisplayValue(employee.toolBudget.quality)} tone={employee.toolBudget.quality === 'reported' ? 'success' : 'warning'} /></article>
      <article className={styles.resourceCard}><span className={styles.cardEyebrow}>工具预算</span><strong>{employee.toolBudget.used === null ? '不可得' : employee.toolBudget.used + ' / ' + employee.toolBudget.limit}</strong><p>剩余 {employee.toolBudget.remaining ?? '不可得'}</p><StatusBadge label="只读" tone="info" /></article>
      <article className={styles.resourceCard}><span className={styles.cardEyebrow}>资格</span><strong>{labelDisplayValue(employee.qualification.status)}</strong><p>{employee.qualification.evidenceId ? formatEntityId(employee.qualification.evidenceId) : '暂无证据引用'}</p><StatusBadge label="证据边界" tone={qualificationTone(employee.qualification.status)} /></article>
    </section>;
  }
  const ownedTasks = overview.tasks.filter(task => task.ownerEmployeeId === employee.employeeId);
  return <section className={styles.detailGrid}>
    <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>工作区</span><h2 className={styles.sectionTitle}>责任工作区</h2></div><FolderOpen aria-hidden="true" className={styles.icon} size={18} /></div>{ownedTasks.length > 0 ? <div className={styles.recordList}>{ownedTasks.map(task => <div className={styles.recordRow} key={task.taskId}><div className={styles.recordLead}><FolderOpen aria-hidden="true" size={16} /><div><strong>{task.title}</strong><span>{task.taskId} · 工作区版本 {task.workspaceRevision ?? '不可得'}</span></div></div><StatusBadge label={labelDisplayValue(task.acceptance)} tone={taskTone(task)} /></div>)}</div> : <EmptyPanel detail="当前员工没有负责人绑定的任务。" title="暂无责任工作区" />}</article>
    <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>工作区边界</span><h2 className={styles.sectionTitle}>运行目录</h2></div><LockKeyhole aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="工作区读取接口" value="当前未提供" /><DataRow label="工作区修改" value="关闭" /><DataRow label="任务检出" value="由后端状态决定" /><DataRow label="产物发布" value="独立于工作区更新" /></div></article>
  </section>;
}

export function CapabilitiesSubpage({overview}: Readonly<{overview: CompanyOverviewView}>) {
  const qualified = overview.employees.filter(employee => employee.qualification.status === 'supported').length;
  return <div className={styles.viewStack}><section className={styles.resourceGrid}><article className={styles.resourceCard}><span className={styles.cardEyebrow}>员工配置</span><strong>{overview.employees.length}</strong><p>当前快照中的员工会话配置</p><StatusBadge label="当前快照" tone="info" /></article><article className={styles.resourceCard}><span className={styles.cardEyebrow}>资格</span><strong>{qualified + ' / ' + overview.employees.length}</strong><p>已支持 / 全部员工</p><StatusBadge label={qualified === overview.employees.length ? '全部支持' : '混合状态'} tone={qualified === overview.employees.length ? 'success' : 'warning'} /></article><article className={styles.resourceCard}><span className={styles.cardEyebrow}>工具预算</span><strong>{overview.resources.toolCallsUsed + ' / ' + overview.resources.toolCallsLimit}</strong><p>公司资源摘要</p><StatusBadge label={labelDisplayValue(overview.resources.toolBudgetQuality)} tone={overview.resources.toolBudgetQuality === 'reported' ? 'success' : 'warning'} /></article></section><section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>能力边界</span><h2 className={styles.sectionTitle}>能力目录</h2></div><Settings2 aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><DataRow label="模型 / 运行时目录" value="当前只读接口未提供" /><DataRow label="工具目录" value="当前只读接口未提供" /><DataRow label="资格证据" value="按员工资格独立显示" /><DataRow label="外部动作" value="关闭" /></div></section></div>;
}
