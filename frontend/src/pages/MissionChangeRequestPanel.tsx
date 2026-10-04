// pattern: Imperative Shell
import {useState, type ReactElement} from 'react';
import type {WorkbenchApi} from '../data/workbench-api';
import {useApplyMissionChangeRequest, useConsiderMissionChangeRequest, useCreateMissionChangeRequest, useDeclineMissionChangeRequest, useMissionChangeRequests} from '../data/workbench-query';
import type {CompanyOverviewView, MissionChangeRequestView, StatusTone} from '../domain/workbench';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import styles from '../styles/workbench.module.css';

type MissionChangeRequestPanelProps = Readonly<{
  api: WorkbenchApi;
  companyId: string;
  missionId: string;
  missionState: string;
  overview: CompanyOverviewView;
}>;

export function MissionChangeRequestPanel({api, companyId, missionId, missionState, overview}: MissionChangeRequestPanelProps): ReactElement {
  const requestsQuery = useMissionChangeRequests(api, companyId, missionId);
  const createRequest = useCreateMissionChangeRequest(api, companyId, missionId);
  const considerRequest = useConsiderMissionChangeRequest(api, companyId, missionId);
  const declineRequest = useDeclineMissionChangeRequest(api, companyId, missionId);
  const applyRequest = useApplyMissionChangeRequest(api, companyId, missionId);
  const [summary, setSummary] = useState('');
  const [proposedTitle, setProposedTitle] = useState(overview.mission.title);
  const [proposedGoal, setProposedGoal] = useState(overview.mission.goal);
  const [criteriaText, setCriteriaText] = useState(overview.mission.acceptanceContract?.required_text.join('\n') ?? '');
  const [blockPreviousResults, setBlockPreviousResults] = useState(true);
  const criteria = criteriaText.split(/\r?\n/).map(value => value.trim()).filter(value => value !== '');
  const proposedAcceptanceContract = criteria.length === 0 ? null : {revision: 'text-acceptance@1' as const, required_text: criteria};
  const hasAcceptanceContract = proposedAcceptanceContract !== null || overview.mission.acceptanceContract !== null;
  const mutationPending = createRequest.isPending || considerRequest.isPending || declineRequest.isPending || applyRequest.isPending;
  const activeRequest = requestsQuery.data?.find(request => ['received', 'queued', 'considered'].includes(request.state)) ?? null;
  const canRequestFormalChange = missionState === 'active' || missionState === 'paused';
  const canCreate = canRequestFormalChange && activeRequest === null && summary.trim() !== '' && proposedTitle.trim() !== '' && proposedGoal.trim() !== '' && criteria.length <= 8 && hasAcceptanceContract;

  async function submit(): Promise<void> {
    if (!canCreate) return;
    await createRequest.mutateAsync({
      requestId: `mission-change-create-${Date.now()}`,
      changeSummary: summary.trim(), proposedTitle: proposedTitle.trim(), proposedGoal: proposedGoal.trim(),
      proposedAcceptanceContract, blockPreviousResults,
    });
    setSummary('');
  }

  async function consider(request: MissionChangeRequestView): Promise<void> {
    await considerRequest.mutateAsync({changeRequestId: request.changeRequestId, requestId: `mission-change-consider-${request.changeRequestId}-${Date.now()}`});
  }

  async function decline(request: MissionChangeRequestView): Promise<void> {
    await declineRequest.mutateAsync({changeRequestId: request.changeRequestId, requestId: `mission-change-decline-${request.changeRequestId}-${Date.now()}`});
  }

  async function apply(request: MissionChangeRequestView): Promise<void> {
    await applyRequest.mutateAsync({changeRequestId: request.changeRequestId, impactSha256: request.impactSha256, requestId: `mission-change-apply-${request.changeRequestId}-${Date.now()}`});
  }

  return <section className={styles.sectionCard}>
    <div className={styles.sectionHeader}>
      <div><span className={styles.cardEyebrow}>正式需求变更</span><h2 className={styles.sectionTitle}>版本、影响与旧结果保护</h2></div>
      <StatusBadge label={activeRequest === null ? (canRequestFormalChange ? '可登记' : '使命已结束') : missionChangeStateLabel(activeRequest.state)} tone={activeRequest === null ? (canRequestFormalChange ? 'neutral' : 'warning') : missionChangeStateTone(activeRequest.state)} />
    </div>
    <p className={styles.panelDescription}>提交会固定当前目标、验收标准和输入版本。应用变更会关闭旧 Mission，并创建一个含新输入版本的草稿 successor；旧验收绑定和产物历史不会被覆盖。</p>
    <div className={styles.formStack}>
      <label className={styles.formLabel}>变更说明
        <textarea className={styles.formField} rows={2} maxLength={4096} value={summary} onChange={event => setSummary(event.target.value)} placeholder="说明正式目标、优先级或验收标准需要如何变化。" />
      </label>
      <div className={styles.wizardGrid}>
        <label className={styles.formLabel}>后继使命标题
          <input className={styles.formField} maxLength={200} value={proposedTitle} onChange={event => setProposedTitle(event.target.value)} />
        </label>
        <label className={styles.formLabel}>后继使命目标
          <input className={styles.formField} maxLength={4096} value={proposedGoal} onChange={event => setProposedGoal(event.target.value)} />
        </label>
      </div>
      <label className={styles.formLabel}>新的验收标准（每行一条；留空表示沿用现行标准）
        <textarea className={styles.formField} rows={4} value={criteriaText} onChange={event => setCriteriaText(event.target.value)} placeholder={overview.mission.acceptanceContract === null ? '当前使命没有可继承的标准，请至少添加一条。' : 'Task summary:\nEmpty results:\nValidation:'} />
      </label>
      {!hasAcceptanceContract ? <p className={styles.formError} role="alert">后继使命需要显式验收标准。</p> : null}
      <label className={styles.formLabel}>
        <input checked={blockPreviousResults} onChange={event => setBlockPreviousResults(event.target.checked)} type="checkbox" />
        在变更结论前阻止旧产物清单和下载
      </label>
      {criteria.length > 8 ? <p className={styles.formError} role="alert">最多 8 条验收标准。</p> : null}
      {createRequest.isError ? <p className={styles.formError} role="alert">变更请求结果尚未确认：{createRequest.error.message} 请核对已刷新的变更历史。</p> : null}
      <button className={styles.commandButton} disabled={!canCreate || mutationPending} onClick={() => { void submit(); }} type="button">{createRequest.isPending ? '正在登记…' : '登记正式变更'}</button>
      <p className={styles.formHint}>“影响分析”只列出系统能核对的任务、输入、产物和在途写者；它不会自动理解所有自然语言依赖。应用前必须暂停并确认写者停止。</p>
    </div>
    <div className={styles.recordList}>
      {requestsQuery.isPending ? <div className={styles.emptyState} role="status">正在读取需求变更历史</div> : requestsQuery.isError ? <div className={styles.errorState} role="alert">需求变更历史读取失败：{requestsQuery.error.message}</div> : requestsQuery.data.length === 0 ? <div className={styles.emptyState}>当前没有正式需求变更。</div> : requestsQuery.data.map(request => <MissionChangeRequestRecord key={request.changeRequestId} request={request} missionState={missionState} mutationPending={mutationPending} onConsider={() => { void consider(request); }} onDecline={() => { void decline(request); }} onApply={() => { void apply(request); }} />)}
    </div>
    {considerRequest.isError ? <p className={styles.formError} role="alert">影响复核结果尚未确认：{considerRequest.error.message} 请查看已刷新的请求状态。</p> : null}
    {declineRequest.isError ? <p className={styles.formError} role="alert">拒绝结果尚未确认：{declineRequest.error.message} 请查看已刷新的请求状态。</p> : null}
    {applyRequest.isError ? <p className={styles.formError} role="alert">变更应用结果尚未确认：{applyRequest.error.message} 请查看已刷新的 Mission 与请求状态。</p> : null}
  </section>;
}

type MissionChangeRequestRecordProps = Readonly<{
  request: MissionChangeRequestView;
  missionState: string;
  mutationPending: boolean;
  onConsider: () => void;
  onDecline: () => void;
  onApply: () => void;
}>;

function MissionChangeRequestRecord({request, missionState, mutationPending, onConsider, onDecline, onApply}: MissionChangeRequestRecordProps): ReactElement {
  const hasOpenWriters = request.impact.activeWorkerSessions.length > 0 || request.impact.nonterminalJobRuns.length > 0 || request.impact.activeServiceEndpoints.length > 0;
  return <article className={styles.recordRow}>
    <div className={styles.recordLead}><div>
      <strong>{request.changeSummary}</strong>
      <span>{request.changeRequestId} · {request.baseRequirementsSha256.slice(0, 12)} · {request.blockPreviousResults ? '旧结果已阻止交付' : '旧结果继续可取'}</span>
      <span>影响快照 rev{request.impactRevision} · {request.impact.inputRevisions.length} 个输入 · {request.impact.tasks.length} 个任务 · {request.impact.artifacts.length} 个产物</span>
      <span>应用时会复制 {request.inputRevisionMap.filter(item => item.origin === 'mission_input').length} 个最新输入修订和 {request.inputRevisionMap.filter(item => item.origin === 'task_workspace').length} 个未完成工作区快照到后继使命。</span>
      <span>{hasOpenWriters ? `${request.impact.activeWorkerSessions.length} 个 Worker、${request.impact.nonterminalJobRuns.length} 个 Job、${request.impact.activeServiceEndpoints.length} 个服务端点仍需核对。` : '当前快照没有已知在途 Worker、Job 或服务端点。'}</span>
      <span>自然语言依赖尚未评估；候选后继：{request.proposedTitle} · {request.proposedGoal}</span>
      {request.successorMissionId !== null ? <span>后继使命草稿：{request.successorMissionId}</span> : null}
      {request.events.map(event => <span key={event.eventId}>{missionChangeStateLabel(event.state)} · {event.reasonCode} · {event.createdAt}</span>)}
    </div></div>
    <div className={styles.recordActions}>
      <StatusBadge label={missionChangeStateLabel(request.state)} tone={missionChangeStateTone(request.state)} />
      {['received', 'queued', 'considered'].includes(request.state) ? <>
        <button className={styles.commandButton} disabled={missionState !== 'paused' || mutationPending} onClick={onConsider} type="button">{missionState === 'paused' ? '复核当前影响' : '暂停后复核'}</button>
        <button className={styles.commandButton} disabled={mutationPending} onClick={onDecline} type="button">拒绝变更</button>
      </> : null}
      {request.state === 'considered' ? <button className={styles.commandButton} disabled={missionState !== 'paused' || mutationPending || hasOpenWriters} onClick={onApply} type="button">应用并创建后继使命</button> : null}
    </div>
  </article>;
}

function missionChangeStateLabel(state: MissionChangeRequestView['state']): string {
  switch (state) {
    case 'received': return '已登记';
    case 'queued': return '等待安全边界';
    case 'considered': return '影响已复核';
    case 'applied': return '已创建后继使命';
    case 'declined': return '已拒绝';
    case 'superseded': return '已被后续变更替代';
  }
}

function missionChangeStateTone(state: MissionChangeRequestView['state']): StatusTone {
  switch (state) {
    case 'considered': return 'info';
    case 'applied': return 'success';
    case 'declined':
    case 'superseded': return 'neutral';
    case 'received':
    case 'queued': return 'warning';
  }
}
