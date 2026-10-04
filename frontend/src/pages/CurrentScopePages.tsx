// pattern: Imperative Shell

import {useEffect, useRef, useState, type ReactElement} from 'react';
import {Activity, Building2, CheckCircle2, CircleAlert, Handshake, LockKeyhole, MessageSquareText, Settings2, ShieldCheck, UsersRound} from 'lucide-react';
import type {WorkbenchApi} from '../data/workbench-api';
import {useBindEmployeeCapability, useCapabilityCatalog, useCompanyFeedback, useCompanyList, useCompanyOverview, useDecideCapability, useApproveStdioMCPRuntimeQualification, useObserveStdioMCPRuntime, useObserveStreamableHTTPMCPRuntime, useDecideGitHubFeedbackSource, useDeleteGitHubFeedbackCredential, useImportReadOnlySkillPackage, useImportStdioMCPPackage, usePollGitHubFeedbackSource, useProbeGitHubFeedbackSource, useQualifyCapability, useRegisterGitHubFeedbackSource, useRegisterMCP, useRevokeEmployeeCapability, useReviewIncompleteCapabilityRevocation, useRuntimeSettings, useSetGitHubFeedbackBacklogStatus, useSetHumanInterventionState, useStoreGitHubFeedbackCredential} from '../data/workbench-query';
import {useSetGitHubFeedbackCollectionPolicy} from '../data/workbench-query';
import type {AttentionItem, CapabilityDecisionView, CapabilityQualificationView, CompanyFeedbackView, CompanySummaryView, MCPServerDefinitionView, RuntimeSettingsView, StdioMCPPackageRevisionView} from '../domain/workbench';
import {labelActivityKind, labelActivityText, labelDisplayValue, labelErrorMessage} from '../domain/display-labels';
import {activateStagedGeneration, activateStagedSidecar, getDesktopRuntime, getWindowsStartupStatus, isDesktopHost, rollbackPreviousGeneration, rollbackPreviousSidecar, setWindowsStartupEnabled, stageRecoveryGeneration, stageSidecarUpdate, type DesktopRuntimeSnapshot, type RecoveryGenerationStageReceipt, type WindowsStartupStatus} from '../lib/desktop-bridge';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import {DomainEvidencePanel} from './DomainEvidencePanel';
import {MemoryCorrectionQueuePanel} from './MemoryCorrectionQueuePanel';
import styles from '../styles/workbench.module.css';

type ScopePageProps = Readonly<{api: WorkbenchApi; companyId: string}>;

function pendingRequestIdentity(pendingIds: Map<string, string>, operation: string, payload: unknown): Readonly<{key: string; requestId: string}> {
  const key = JSON.stringify({operation, payload});
  const requestId = pendingIds.get(key) ?? `${operation}-${crypto.randomUUID()}`;
  pendingIds.set(key, requestId);
  return {key, requestId};
}

function clearPendingRequestIdentity(pendingIds: Map<string, string>, key: string): void {
  pendingIds.delete(key);
}

export function FeedbackPage({api, companyId}: ScopePageProps): ReactElement {
  const query = useCompanyOverview(api, companyId);
  const feedbackQuery = useCompanyFeedback(api, companyId);
  const backlogStatus = useSetGitHubFeedbackBacklogStatus(api, companyId);
  const storeGitHubCredential = useStoreGitHubFeedbackCredential(api, companyId);
  const deleteGitHubCredential = useDeleteGitHubFeedbackCredential(api, companyId);
  const [githubToken, setGitHubToken] = useState('');
  const [githubCredentialMessage, setGitHubCredentialMessage] = useState<string | null>(null);
  const [backlogStatusChoice, setBacklogStatusChoice] = useState<'open' | 'triaging' | 'waiting' | 'handled' | 'archived'>('triaging');
  const [backlogRationale, setBacklogRationale] = useState('');
  const [backlogMessage, setBacklogMessage] = useState<string | null>(null);
  const pendingRequestIds = useRef(new Map<string, string>());
  if (query.isPending) return <ScopeLoading title="反馈待办" label="正在读取待处理事项" />;
  if (query.isError) return <ScopeError title="反馈待办" message={query.error.message} />;
  const overview = query.data;
  const attention = overview.attention;
  const recentActivity = overview.recentActivity.slice(0, 8);
  async function saveGitHubCredential(): Promise<void> {
    setGitHubCredentialMessage(null);
    try {
      const token = githubToken;
      const payload = {tokenDigest: await sha256Text(token)};
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'github-credential-store', payload);
      await storeGitHubCredential.mutateAsync({token, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setGitHubToken('');
      setGitHubCredentialMessage('凭据已保存到本机受保护存储；此操作没有访问 GitHub。');
    } catch (error) {
      setGitHubCredentialMessage(error instanceof Error ? error.message : 'GitHub 凭据保存失败');
    }
  }
  async function removeGitHubCredential(): Promise<void> {
    setGitHubCredentialMessage(null);
    try {
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'github-credential-delete', {});
      await deleteGitHubCredential.mutateAsync({requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setGitHubToken('');
      setGitHubCredentialMessage('本机保存的 GitHub 凭据已删除。');
    } catch (error) {
      setGitHubCredentialMessage(error instanceof Error ? error.message : 'GitHub 凭据删除失败');
    }
  }
  async function updateBacklogStatus(issue: CompanyFeedbackView['issues'][number]): Promise<void> {
    setBacklogMessage(null);
    try {
      const payload = {
        sourceId: issue.sourceId,
        providerItemId: issue.providerItemId,
        revisionSha256: issue.revisionSha256,
        status: backlogStatusChoice,
        rationale: backlogRationale.trim(),
      };
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'github-backlog', payload);
      const receipt = await backlogStatus.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setBacklogMessage(`已更新公司内部待办状态为“${backlogStatusLabel(receipt.status)}”。远端 Issue 状态“${labelDisplayValue(receipt.remoteState)}”未变更。`);
    } catch (error) {
      setBacklogMessage(error instanceof Error ? error.message : '公司待办状态更新失败');
    }
  }
  return <div className={styles.viewStack} data-od-id="feedback-view">
    <ScopeHeader eyebrow="公司 / 反馈" title="反馈待办" action={<StatusBadge label={`${attention.length} 项待处理`} tone={attention.length > 0 ? 'warning' : 'success'} />} />
    {api.mode === 'real' ? <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>GitHub / 只读凭据</span><h2 className={styles.sectionTitle}>受保护的 GitHub Token</h2></div><StatusBadge label="按需只读" tone="info" /></div>
      <p className={styles.formHint}>使用只读 Issues 权限的 token。保存和删除凭据不会访问 GitHub；只有启用只读采集后，点击权限探测或采集才会发送只读请求。凭据不会写入 Polis 数据库，也不会由 API 返回。</p>
      <div className={styles.formStack}>
        <label className={styles.formLabel}>GitHub Token<input autoComplete="new-password" className={styles.formField} onChange={event => setGitHubToken(event.target.value)} type="password" value={githubToken} /></label>
        <div className={styles.commandGroup}>
          <button className={styles.commandButton} disabled={api.mode !== 'real' || githubToken.trim() === '' || storeGitHubCredential.isPending} onClick={() => { void saveGitHubCredential(); }} type="button">{storeGitHubCredential.isPending ? '保存中…' : '保存到本机保护存储'}</button>
          <button className={styles.commandButton} disabled={deleteGitHubCredential.isPending} onClick={() => { void removeGitHubCredential(); }} type="button">{deleteGitHubCredential.isPending ? '删除中…' : '删除本机凭据'}</button>
        </div>
        {githubCredentialMessage ? <div className={styles.operationNotice} role="status">{labelErrorMessage(githubCredentialMessage)}</div> : null}
      </div>
    </section> : null}
    <section className={styles.detailGrid}>
      <article className={styles.sectionCard}>
        <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>人工介入</span><h2 className={styles.sectionTitle}>需要人处理</h2></div><CircleAlert aria-hidden="true" className={styles.icon} size={18} /></div>
        {attention.length === 0 ? <div className={styles.emptyState}><CheckCircle2 aria-hidden="true" size={18} /><strong>当前没有待处理事项</strong><span>系统没有把普通活动或日志伪装成反馈。</span></div> : <div className={styles.recordList}>{attention.map(item => <AttentionRow api={api} companyId={companyId} item={item} key={item.id} />)}</div>}
      </article>
      <article className={styles.sectionCard}>
        <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>活动信号</span><h2 className={styles.sectionTitle}>最近反馈信号</h2></div><Activity aria-hidden="true" className={styles.icon} size={18} /></div>
        {recentActivity.length === 0 ? <div className={styles.emptyState}><span>当前快照没有最近活动。</span></div> : <div className={styles.recordList}>{recentActivity.map(event => <div className={styles.recordRow} key={event.id}><div className={styles.recordLead}><Activity aria-hidden="true" size={16} /><div><strong>{labelActivityText(event.summary)}</strong><span>{labelActivityKind(event.kind)} · {labelActivityText(event.subject.label)}</span></div></div><StatusBadge label={labelDisplayValue(event.tone)} tone={event.tone === 'success' ? 'success' : event.tone === 'danger' ? 'danger' : 'info'} /></div>)}</div>}
      </article>
    </section>
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>只读来源 / GitHub Issues</span><h2 className={styles.sectionTitle}>外部反馈观察</h2></div><StatusBadge label={feedbackQuery.isSuccess ? `${feedbackQuery.data.issues.length} 条观察` : feedbackQuery.isPending ? '读取中' : '不可用'} tone={feedbackQuery.isSuccess ? 'info' : feedbackQuery.isPending ? 'neutral' : 'warning'} /></div>
      <p className={styles.formHint}>展示已持久化、按公司隔离的只读观察。来源批准、权限探测和采集覆盖状态单独显示；这些文本不会自动转成任务或命令。</p>
      <div className={styles.formStack}>
        <label className={styles.formLabel}>内部待办操作说明<input className={styles.formField} maxLength={512} onChange={event => setBacklogRationale(event.target.value)} value={backlogRationale} /></label>
        <label className={styles.formLabel}>内部待办状态<select className={styles.formField} onChange={event => setBacklogStatusChoice(event.target.value as typeof backlogStatusChoice)} value={backlogStatusChoice}><option value="open">待分诊</option><option value="triaging">分诊中</option><option value="waiting">等待补充</option><option value="handled">内部已处理</option><option value="archived">归档</option></select></label>
        <p className={styles.formHint}>这些状态只记录 Polis 公司 backlog。标记“内部已处理”或“归档”不会关闭、评论或修改 GitHub Issue；来源出现新修订时会重新标记为需复核。</p>
      </div>
      {feedbackQuery.isPending ? <div className={styles.emptyState} role="status">正在读取 GitHub 观察快照。</div> : feedbackQuery.isError ? <div className={styles.errorState} role="alert">GitHub 观察读取失败：{labelErrorMessage(feedbackQuery.error.message)}</div> : <>
        <GitHubFeedbackSourcesPanel api={api} companyId={companyId} feedback={feedbackQuery.data} />
        <div className={styles.recordList}>
          {feedbackQuery.data.issues.length === 0 ? <div className={styles.emptyState}>当前快照中没有已持久化的 Issue 观察。</div> : feedbackQuery.data.issues.map(issue => {
            const source = feedbackQuery.data.sources.find(item => item.sourceId === issue.sourceId);
            return <article className={styles.recordRow} key={`${issue.sourceId}:${issue.providerItemId}:${issue.revisionSha256}`}>
              <div className={styles.recordLead}><MessageSquareText aria-hidden="true" size={16} /><div><strong>{issue.title || `Issue #${issue.issueNumber}`}</strong><span>{source?.repository ?? issue.sourceId} · #{issue.issueNumber} · {labelDisplayValue(issue.state)} · 更新于 {issue.sourceUpdatedAt}</span><span>{issue.body || '正文为空'}{issue.bodyTruncated ? ' …（正文已截断）' : ''}</span>
                {issue.comments.map(comment => <span key={`${comment.commentId}:${comment.revisionSha256}`}>评论 · {comment.sourceUpdatedAt}：{comment.body}{comment.bodyTruncated ? ' …（评论已截断）' : ''}</span>)}
                <span>评论覆盖：{labelDisplayValue(issue.commentCoverage)}{issue.commentCoverageReason ? ` · ${issue.commentCoverageReason}` : ''} · 已读 {issue.comments.length}/{issue.commentCount}</span>
              </div></div>
              <div className={styles.recordMeta}><StatusBadge label={`公司待办：${labelDisplayValue(issue.backlogStatus)}`} tone={issue.backlogStatus === 'handled' || issue.backlogStatus === 'archived' ? 'success' : issue.backlogStatus === 'needs_review' ? 'warning' : 'info'} /><StatusBadge label={issue.commentContextPartial ? '评论上下文不完整' : issue.commentCoverage === 'complete' || issue.commentCoverage === 'not_read' ? labelDisplayValue(issue.commentCoverage) : '评论部分覆盖'} tone={issue.commentContextPartial || issue.commentCoverage === 'partial' ? 'warning' : 'info'} /><button className={styles.textButton} disabled={api.mode !== 'real' || backlogRationale.trim() === '' || backlogStatus.isPending} onClick={() => { void updateBacklogStatus(issue); }} type="button">{backlogStatus.isPending ? '更新中…' : '记录内部状态'}</button></div>
            </article>;
          })}
        </div>
        {backlogMessage ? <div className={styles.operationNotice} role="status">{labelErrorMessage(backlogMessage)}</div> : null}
      </>}
    </section>
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>处理边界</span><h2 className={styles.sectionTitle}>反馈处理边界</h2></div><ShieldCheck aria-hidden="true" className={styles.icon} size={18} /></div>
      <div className={styles.detailRows}><ScopeRow label="事实来源" value="当前公司的权威总览快照" /><ScopeRow label="处理动作" value="在工作台中查看和处理，不自动执行外部动作" /><ScopeRow label="外部通知" value="QQ / Webhook 需要单独授权与资格验证" /><ScopeRow label="当前状态" value={labelDisplayValue(overview.meta.freshness)} /></div>
    </section>
  </div>;
}

function backlogStatusLabel(status: CompanyFeedbackView['issues'][number]['backlogStatus']): string {
  switch (status) {
    case 'open': return '待分诊';
    case 'needs_review': return '需复核';
    case 'triaging': return '分诊中';
    case 'waiting': return '等待信息';
    case 'handled': return '内部已处理';
    case 'archived': return '已归档';
  }
}

function GitHubFeedbackSourcesPanel({api, companyId, feedback}: Readonly<{api: WorkbenchApi; companyId: string; feedback: CompanyFeedbackView}>): ReactElement {
  const register = useRegisterGitHubFeedbackSource(api, companyId);
  const probe = useProbeGitHubFeedbackSource(api, companyId);
  const decide = useDecideGitHubFeedbackSource(api, companyId);
  const poll = usePollGitHubFeedbackSource(api, companyId);
  const collectionPolicy = useSetGitHubFeedbackCollectionPolicy(api, companyId);
  const [repositoryId, setRepositoryId] = useState('');
  const [owner, setOwner] = useState('');
  const [name, setName] = useState('');
  const [rationale, setRationale] = useState('');
  const [collectionIntervalSeconds, setCollectionIntervalSeconds] = useState(3600);
  const [message, setMessage] = useState<string | null>(null);
  const pendingRequestIds = useRef(new Map<string, string>());
  async function registerSource(): Promise<void> {
    setMessage(null);
    try {
      const payload = {repositoryId, owner: owner.trim(), name: name.trim()};
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'github-source-register', payload);
      const result = await register.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setMessage(`已登记 ${result.repository}，目前是草案；还需单独探测权限并人工批准。`);
      setRepositoryId(''); setOwner(''); setName('');
    } catch (error) { setMessage(error instanceof Error ? error.message : 'GitHub 来源登记失败'); }
  }
  async function probeSource(sourceId: string): Promise<void> {
    setMessage(null);
    try {
      const payload = {sourceId, rationale: rationale.trim()};
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'github-source-probe', payload);
      const result = await probe.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setMessage(`权限探测：${labelDisplayValue(result.permissionStatus)} · 覆盖 ${labelDisplayValue(result.coverage)}${result.coverageReason ? ` · ${result.coverageReason}` : ''}`);
    } catch (error) { setMessage(error instanceof Error ? error.message : 'GitHub 权限探测失败'); }
  }
  async function decideSource(sourceId: string, decision: 'approved' | 'paused' | 'revoked'): Promise<void> {
    setMessage(null);
    try {
      const payload = {sourceId, decision, rationale: rationale.trim()};
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'github-source-decision', payload);
      const result = await decide.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setMessage(`${result.repository} 来源状态：${labelDisplayValue(result.state)}。`);
    } catch (error) { setMessage(error instanceof Error ? error.message : 'GitHub 来源决策失败'); }
  }
  async function pollSource(sourceId: string): Promise<void> {
    setMessage(null);
    try {
      const payload = {sourceId};
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'github-source-poll', payload);
      const result = await poll.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setMessage(`采集 ${labelDisplayValue(result.coverage)}：${result.itemCount} 个 Issue，${result.commentScanCount} 组评论上下文（${labelDisplayValue(result.commentCoverage)}）${result.replayed ? ' · 已复用结果' : ''}${result.coverageReason ? ` · ${result.coverageReason}` : ''}`);
    } catch (error) { setMessage(error instanceof Error ? error.message : 'GitHub 采集失败'); }
  }
  async function setCollection(sourceId: string, enabled: boolean): Promise<void> {
    setMessage(null);
    try {
      const payload = {sourceId, enabled, intervalSeconds: collectionIntervalSeconds, rationale: rationale.trim()};
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'github-collection-policy', payload);
      const result = await collectionPolicy.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      if (!result.enabled) setMessage('已停用公司级采集策略。已接受的单次请求可能已开始，不会重放。');
      else if (!result.externalEnabled) setMessage('公司级策略已启用；全局 GitHub 只读外发仍关闭，因此不会发送请求。');
      else if (!result.schedulerEnabled) setMessage('公司级策略已启用；此实例的定时采集器仍关闭。');
      else setMessage(`定时只读采集已启用，下次计划：${result.nextPollAt ?? '待定'}。只写入公司待办。`);
    } catch (error) { setMessage(error instanceof Error ? error.message : 'GitHub 采集策略更新失败'); }
  }
  const validRepository = /^\d+$/.test(repositoryId) && owner.trim() !== '' && name.trim() !== '';
  return <div className={styles.viewStack}>
    <div className={styles.formStack}>
      <span className={styles.cardEyebrow}>登记只读仓库</span>
      <div className={styles.detailGrid}>
        <label className={styles.formLabel}>GitHub 仓库数字 ID<input className={styles.formField} inputMode="numeric" onChange={event => setRepositoryId(event.target.value)} value={repositoryId} /></label>
        <label className={styles.formLabel}>Owner<input className={styles.formField} onChange={event => setOwner(event.target.value)} value={owner} /></label>
        <label className={styles.formLabel}>Repository<input className={styles.formField} onChange={event => setName(event.target.value)} value={name} /></label>
      </div>
      <label className={styles.formLabel}>人工决策说明<input className={styles.formField} maxLength={512} onChange={event => setRationale(event.target.value)} value={rationale} /></label>
      <button className={styles.commandButton} disabled={!validRepository || register.isPending} onClick={() => { void registerSource(); }} type="button">{register.isPending ? '登记中…' : '登记固定只读来源'}</button>
      <p className={styles.formHint}>权限探测和手动采集会向已登记仓库发送只读 GET 请求；探测/采集按钮仅在你主动点击时运行。采集不会自动转成任务或命令。</p>
    </div>
    <div className={styles.recordList}>
      {feedback.sources.length === 0 ? <div className={styles.emptyState}>当前公司尚无登记的 GitHub 只读来源。</div> : feedback.sources.map(source => <div className={styles.recordRow} key={source.sourceId}>
        <div className={styles.recordLead}><MessageSquareText aria-hidden="true" size={16} /><div><strong>{source.repository}</strong><span>来源状态：{labelDisplayValue(source.state)} · 权限：{labelDisplayValue(source.permissionStatus)}</span><span>覆盖：{labelDisplayValue(source.coverage)}{source.coverageReason ? ` · ${source.coverageReason}` : ''}{source.coveredThrough ? ` · 截止 ${source.coveredThrough}` : ''}</span></div></div>
        <div className={styles.recordMeta}>
          <StatusBadge label={source.provider} tone="info" />
          {source.state !== 'revoked' ? <button className={styles.textButton} disabled={rationale.trim() === '' || probe.isPending} onClick={() => { void probeSource(source.sourceId); }} type="button">探测权限（只读 GET）</button> : null}
          {source.permissionStatus === 'verified' && source.state !== 'approved' && source.state !== 'revoked' ? <button className={styles.textButton} disabled={rationale.trim() === '' || decide.isPending} onClick={() => { void decideSource(source.sourceId, 'approved'); }} type="button">人工批准</button> : null}
          {source.state === 'approved' ? <button className={styles.textButton} disabled={rationale.trim() === '' || decide.isPending} onClick={() => { void decideSource(source.sourceId, 'paused'); }} type="button">暂停</button> : null}
          {source.state !== 'revoked' ? <button className={styles.textButton} disabled={rationale.trim() === '' || decide.isPending} onClick={() => { void decideSource(source.sourceId, 'revoked'); }} type="button">撤销</button> : null}
          {source.state === 'approved' && source.permissionStatus === 'verified' ? <button className={styles.commandButton} disabled={poll.isPending} onClick={() => { void pollSource(source.sourceId); }} type="button">{poll.isPending ? '采集中…' : '采集快照（只读 GET）'}</button> : null}
        </div>
      </div>)}
    </div>
    {feedback.sources.some(source => source.state === 'approved' && source.permissionStatus === 'verified') ? <div className={styles.formStack}>
      <span className={styles.cardEyebrow}>Scheduled collection policy</span>
      <p className={styles.formHint}>启用需要独立的公司策略和部署级只读外发、定时采集两个开关。关闭策略会停止后续预约；已经开始的有界 GET 可能完成。采集只更新内部 backlog，不启动任务或模型。</p>
      {feedback.sources.filter(source => source.state === 'approved' && source.permissionStatus === 'verified').map(source => <div className={styles.recordRow} key={`collection-${source.sourceId}`}>
        <div className={styles.recordLead}><MessageSquareText aria-hidden="true" size={16} /><div><strong>{source.repository}</strong><span>公司策略：{source.collectionEnabled ? `已启用，每 ${source.collectionIntervalSeconds / 60} 分钟` : '未启用'}{source.collectionNextPollAt ? ` · 下次 ${source.collectionNextPollAt}` : ''}</span><span>{source.collectionLastAttempt ? `上次 ${source.collectionLastAttempt}${source.collectionLastReasonCode ? ` / ${source.collectionLastReasonCode}` : ''}` : '暂无自动采集记录'}</span></div></div>
        <div className={styles.recordMeta}>
          <label className={styles.formLabel}>间隔
            <select className={styles.formField} onChange={event => setCollectionIntervalSeconds(Number(event.target.value))} value={collectionIntervalSeconds}>
              <option value={900}>15 分钟</option><option value={3600}>1 小时</option><option value={21600}>6 小时</option><option value={86400}>24 小时</option>
            </select>
          </label>
          <button className={styles.textButton} disabled={rationale.trim() === '' || collectionPolicy.isPending} onClick={() => { void setCollection(source.sourceId, !source.collectionEnabled); }} type="button">{collectionPolicy.isPending ? '保存中…' : source.collectionEnabled ? '停用定时采集' : '启用定时采集'}</button>
        </div>
      </div>)}
    </div> : null}
    {message ? <div className={styles.operationNotice} role="status">{labelErrorMessage(message)}</div> : null}
  </div>;
}

export function GroupResourcesPage({api, companyId}: ScopePageProps): ReactElement {
  const companiesQuery = useCompanyList(api);
  const overviewQuery = useCompanyOverview(api, companyId);
  if (companiesQuery.isPending || overviewQuery.isPending) return <ScopeLoading title="集团资源" label="正在读取公司目录与当前资源快照" />;
  if (companiesQuery.isError) return <ScopeError title="集团资源" message={companiesQuery.error.message} />;
  if (overviewQuery.isError) return <ScopeError title="集团资源" message={overviewQuery.error.message} />;
  const overview = overviewQuery.data;
  const companies = companiesQuery.data;
  const activeCompanies = companies.filter(company => company.state === 'active').length;
  const employeeCount = companies.reduce((total, company) => total + company.roster.length, 0);
  return <div className={styles.viewStack} data-od-id="group-resources-view">
    <ScopeHeader eyebrow="集团 / 资源" title="集团资源" action={<StatusBadge label="当前作用域已接入" tone="success" />} />
    <section className={styles.resourceGrid}>
      <ResourceMetric icon={<Building2 aria-hidden="true" size={17} />} label="公司目录" value={`${companies.length}`} detail={`${activeCompanies} 个运行中作用域`} />
      <ResourceMetric icon={<UsersRound aria-hidden="true" size={17} />} label="固定角色" value={`${employeeCount}`} detail="来自各公司固定角色" />
      <ResourceMetric icon={<CircleAlert aria-hidden="true" size={17} />} label="待处理事项" value={`${overview.attention.length}`} detail="当前公司快照" />
    </section>
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>公司目录</span><h2 className={styles.sectionTitle}>已登记公司</h2></div><StatusBadge label={`${companies.length} 个`} tone="info" /></div>
      <div className={styles.recordList}>{companies.length === 0 ? <div className={styles.emptyState}><span>还没有登记公司。</span></div> : companies.map(company => <CompanyResourceRow company={company} key={company.id} />)}</div>
    </section>
    <section className={styles.detailGrid}>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>当前作用域</span><h2 className={styles.sectionTitle}>当前资源口径</h2></div><Settings2 aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><ScopeRow label="公司" value={overview.company.name} /><ScopeRow label="工作区" value={overview.company.description || '由公司配置维护'} /><ScopeRow label="工具预算" value={`${overview.resources.toolCallsUsed} / ${overview.resources.toolCallsLimit}`} /><ScopeRow label="截止时间" value={overview.resources.asOf} /></div></article>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>受保护边界</span><h2 className={styles.sectionTitle}>受保护资源</h2></div><LockKeyhole aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.checkList}><div><CheckCircle2 aria-hidden="true" size={15} /><span>公司之间不合并内部责任记录</span></div><div><CheckCircle2 aria-hidden="true" size={15} /><span>凭据和模型秘密不进入目录</span></div><div><CheckCircle2 aria-hidden="true" size={15} /><span>CAS / 产物仍按公司作用域读取</span></div></div></article>
    </section>
  </div>;
}

export function GroupSettingsPage({api, companyId}: ScopePageProps): ReactElement {
  const query = useRuntimeSettings(api, companyId);
  const catalogQuery = useCapabilityCatalog(api, companyId);
  const [tab, setTab] = useState<'runtime' | 'capabilities' | 'domains' | 'memory' | 'safety'>('runtime');
  if (query.isPending) return <ScopeLoading title="集团设置" label="正在读取当前 runtime readiness" />;
  if (query.isError) return <ScopeError title="集团设置" message={query.error.message} />;
  const runtime = query.data;
  return <div className={styles.viewStack} data-od-id="group-settings-view">
    <ScopeHeader eyebrow="集团 / 设置" title="集团设置" action={<StatusBadge label={readinessLabel(runtime.runtimeReadiness)} tone={runtime.runtimeReadiness === 'ready' ? 'success' : 'warning'} />} />
    <div className={styles.tabBar} role="tablist"><button aria-selected={tab === 'runtime'} className={`${styles.tabButton} ${tab === 'runtime' ? styles.tabButtonActive : ''}`} onClick={() => setTab('runtime')} role="tab" type="button">运行时就绪</button><button aria-selected={tab === 'capabilities'} className={`${styles.tabButton} ${tab === 'capabilities' ? styles.tabButtonActive : ''}`} onClick={() => setTab('capabilities')} role="tab" type="button">技能 / MCP</button><button aria-selected={tab === 'domains'} className={`${styles.tabButton} ${tab === 'domains' ? styles.tabButtonActive : ''}`} onClick={() => setTab('domains')} role="tab" type="button">领域验收</button><button aria-selected={tab === 'memory'} className={`${styles.tabButton} ${tab === 'memory' ? styles.tabButtonActive : ''}`} onClick={() => setTab('memory')} role="tab" type="button">记忆更正</button><button aria-selected={tab === 'safety'} className={`${styles.tabButton} ${tab === 'safety' ? styles.tabButtonActive : ''}`} onClick={() => setTab('safety')} role="tab" type="button">安全边界</button></div>
    {tab === 'runtime' ? <RuntimeReadinessPanel runtime={runtime} /> : tab === 'capabilities' ? <CapabilityCatalogPanel api={api} companyId={companyId} query={catalogQuery} /> : tab === 'domains' ? <DomainEvidencePanel api={api} companyId={companyId} /> : tab === 'memory' ? <MemoryCorrectionQueuePanel api={api} companyId={companyId} /> : <SafetyPanel runtime={runtime} />}
    <WindowsStartupPanel />
    <DesktopRecoveryPanel />
    <DesktopSidecarUpdatePanel />
  </div>;
}

function WindowsStartupPanel(): ReactElement | null {
  const desktopHost = isDesktopHost();
  const [status, setStatus] = useState<WindowsStartupStatus | null>(null);
  const [statusLoading, setStatusLoading] = useState(true);
  const [statusReadError, setStatusReadError] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [messageRole, setMessageRole] = useState<'alert' | 'status'>('status');

  useEffect(() => {
    if (!desktopHost) return;
    let cancelled = false;
    setStatusLoading(true);
    setStatusReadError(false);
    void getWindowsStartupStatus()
      .then(value => {
        if (cancelled) return;
        setStatus(value);
        setMessage(null);
      })
      .catch(() => {
        if (cancelled) return;
        setStatusReadError(true);
        setMessageRole('alert');
        setMessage('无法读取 Windows 登录启动状态');
      })
      .finally(() => { if (!cancelled) setStatusLoading(false); });
    return () => { cancelled = true; };
  }, [desktopHost]);

  if (!desktopHost) return null;

  async function toggleStartup(): Promise<void> {
    if (status === null || !status.supported || busy) return;
    setBusy(true);
    setMessageRole('status');
    setMessage(null);
    try {
      const next = await setWindowsStartupEnabled(!status.enabled);
      setStatus(next);
      setMessage(next.enabled ? '已启用当前 Windows 用户登录时启动 Polis' : '已关闭登录时启动 Polis');
    } catch {
      setMessageRole('alert');
      setMessage('更新 Windows 登录启动设置失败');
    } finally {
      setBusy(false);
    }
  }

  return <article className={styles.sectionCard} aria-labelledby="windows-startup-title">
    <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>POLIS DESKTOP / STARTUP</span><h2 className={styles.sectionTitle} id="windows-startup-title">Windows 登录启动</h2></div><Settings2 aria-hidden="true" className={styles.icon} size={18} /></div>
    <div className={styles.detailRows}>
      <ScopeRow label="支持状态" value={statusLoading ? '读取中' : statusReadError ? '读取失败' : status?.supported ? '当前 Windows 用户' : '此平台不支持'} />
      {status?.supported ? <ScopeRow label="登录时启动 Polis" value={status.enabled ? '已启用' : '已关闭'} /> : null}
    </div>
    <p className={styles.formHint}>这是当前 Windows 用户的本机启动偏好；启用后只打开 Polis Desktop，不会自动启动使命、Worker 或模型请求。</p>
    {message ? <p className={styles.formHint} role={messageRole}>{message}</p> : null}
    {status?.supported ? <div className={styles.wizardActions}><button aria-pressed={status.enabled} className={status.enabled ? styles.textButton : styles.commandButton} disabled={busy} onClick={() => { void toggleStartup(); }} type="button">{busy ? '正在更新…' : status.enabled ? '关闭登录启动' : '启用登录启动'}</button></div> : null}
  </article>;
}

function DesktopRecoveryPanel(): ReactElement | null {
  const desktopHost = isDesktopHost();
  const [runtime, setRuntime] = useState<DesktopRuntimeSnapshot | null>(null);
  const [packagePath, setPackagePath] = useState('');
  const [administratorPassword, setAdministratorPassword] = useState('');
  const [stagedGeneration, setStagedGeneration] = useState<RecoveryGenerationStageReceipt | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);

  useEffect(() => {
    if (!desktopHost) return;
    let cancelled = false;
    void getDesktopRuntime()
      .then(value => { if (!cancelled) setRuntime(value); })
      .catch(error => { if (!cancelled) setMessage(error instanceof Error ? error.message : '无法读取本机运行时状态'); });
    return () => { cancelled = true; };
  }, [desktopHost]);

  if (!desktopHost) return null;

  async function stageGeneration(): Promise<void> {
    setBusy(true);
    setMessage(null);
    try {
      const receipt = await stageRecoveryGeneration(packagePath.trim(), administratorPassword || undefined);
      setStagedGeneration(receipt);
      setAdministratorPassword('');
      setMessage(`恢复代际 ${receipt.generationId} 已验证并暂存，活动数据尚未切换。`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '恢复代际准备失败');
    } finally {
      setBusy(false);
    }
  }

  async function activateGeneration(): Promise<void> {
    if (stagedGeneration === null) return;
    setBusy(true);
    setMessage(null);
    try {
      const next = await activateStagedGeneration(stagedGeneration.generationId);
      setRuntime(next);
      setStagedGeneration(null);
      setMessage(`当前活动代际：${next.active_generation_id ?? 'legacy'}。`);
    } catch (error) {
      const next = await getDesktopRuntime().catch(() => null);
      if (next !== null) setRuntime(next);
      setMessage(error instanceof Error ? error.message : '代际切换失败；桌面启动器已尝试恢复上一代');
    } finally {
      setBusy(false);
    }
  }

  async function rollbackGeneration(): Promise<void> {
    if (runtime?.previous_generation_id === null || runtime?.previous_generation_id === undefined) return;
    const accepted = window.confirm(
      `切换到上一代 ${runtime.previous_generation_id}？当前代际上的新记录会保留在本机，但切回后不会显示；两个代际都不会自动删除。`,
    );
    if (!accepted) return;
    setBusy(true);
    setMessage(null);
    try {
      const next = await rollbackPreviousGeneration(true);
      setRuntime(next);
      setStagedGeneration(null);
      setMessage(`已切换到代际 ${next.active_generation_id ?? 'legacy'}。`);
    } catch (error) {
      const next = await getDesktopRuntime().catch(() => null);
      if (next !== null) setRuntime(next);
      setMessage(error instanceof Error ? error.message : '代际回退失败；桌面启动器已尝试恢复当前代际');
    } finally {
      setBusy(false);
    }
  }

  return <article className={styles.sectionCard} aria-labelledby="desktop-recovery-title">
    <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>POLIS DESKTOP / RECOVERY</span><h2 className={styles.sectionTitle} id="desktop-recovery-title">本机恢复代际</h2></div><Activity aria-hidden="true" className={styles.icon} size={18} /></div>
    <div className={styles.detailRows}>
      <ScopeRow label="活动代际" value={runtime?.active_generation_id ?? '读取中'} />
      <ScopeRow label="保留的上一代" value={runtime?.previous_generation_id ?? '无'} />
      <ScopeRow label="切换状态" value={runtime?.generation_pending ? '启动检查中' : '稳定'} />
    </div>
    <p className={styles.formHint}>准备阶段会将恢复包还原到独立数据库和 CAS 目录。激活前需要停止活动工作并通过启动健康检查；失败时会恢复旧指针。暂存与旧代际不会自动删除。</p>
    <label className={styles.formLabel}>恢复包目录<input className={styles.formField} value={packagePath} onChange={event => setPackagePath(event.target.value)} placeholder="完整本机路径，例如 C:\\Backups\\polis-recovery-…" /></label>
    <label className={styles.formLabel}>PostgreSQL 管理员密码（仅旧安装未保存时需要）<input autoComplete="current-password" className={styles.formField} type="password" value={administratorPassword} onChange={event => setAdministratorPassword(event.target.value)} /></label>
    {message ? <p className={styles.formHint} role="status">{labelErrorMessage(message)}</p> : null}
    <div className={styles.wizardActions}>
      <button className={styles.commandButton} disabled={busy || packagePath.trim() === ''} onClick={() => { void stageGeneration(); }} type="button">{busy ? '正在准备…' : '验证并暂存'}</button>
      {stagedGeneration ? <button className={styles.commandButton} disabled={busy} onClick={() => { void activateGeneration(); }} type="button">激活此代际</button> : null}
      {runtime?.previous_generation_id ? <button className={styles.textButton} disabled={busy} onClick={() => { void rollbackGeneration(); }} type="button">切回上一代</button> : null}
    </div>
  </article>;
}

function DesktopSidecarUpdatePanel(): ReactElement | null {
  const desktopHost = isDesktopHost();
  const [runtime, setRuntime] = useState<DesktopRuntimeSnapshot | null>(null);
  const [sourcePath, setSourcePath] = useState('');
  const [stagedSha256, setStagedSha256] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);

  useEffect(() => {
    if (!desktopHost) return;
    let cancelled = false;
    void getDesktopRuntime()
      .then(value => { if (!cancelled) setRuntime(value); })
      .catch(error => { if (!cancelled) setMessage(error instanceof Error ? error.message : '无法读取本机更新状态'); });
    return () => { cancelled = true; };
  }, [desktopHost]);

  if (!desktopHost) return null;

  async function refreshRuntime(): Promise<void> {
    const next = await getDesktopRuntime();
    setRuntime(next);
    setStagedSha256(next.staged_sidecar_sha256);
  }

  async function stageCandidate(): Promise<void> {
    setBusy(true);
    setMessage(null);
    try {
      const receipt = await stageSidecarUpdate(sourcePath.trim());
      setStagedSha256(receipt.sha256);
      setSourcePath('');
      setMessage(`候选已私下暂存；SHA-256 ${receipt.sha256}，大小 ${receipt.sizeBytes} 字节。`);
      await refreshRuntime();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '本地 sidecar 暂存失败');
    } finally {
      setBusy(false);
    }
  }

  async function activateCandidate(): Promise<void> {
    const sha256 = runtime?.staged_sidecar_sha256 ?? stagedSha256;
    if (sha256 === null || sha256 === undefined) return;
    const accepted = window.confirm(`激活本机 Polis sidecar？候选 SHA-256：${sha256}。当前数据库和数据代际保持原状。`);
    if (!accepted) return;
    setBusy(true);
    setMessage(null);
    try {
      const next = await activateStagedSidecar(sha256);
      setRuntime(next);
      setStagedSha256(next.staged_sidecar_sha256);
      setMessage(`sidecar 已启动并通过身份校验。活动 SHA-256：${next.active_sidecar_sha256 ?? '未知'}。`);
    } catch (error) {
      const next = await getDesktopRuntime().catch(() => null);
      if (next !== null) {
        setRuntime(next);
        setStagedSha256(next.staged_sidecar_sha256);
      }
      setMessage(error instanceof Error ? error.message : 'sidecar 激活失败；启动器会尝试恢复原二进制');
    } finally {
      setBusy(false);
    }
  }

  async function rollbackCandidate(): Promise<void> {
    if (runtime?.previous_sidecar_sha256 === null || runtime?.previous_sidecar_sha256 === undefined) return;
    const accepted = window.confirm(`切回上一版 Polis sidecar？目标 SHA-256：${runtime.previous_sidecar_sha256}。不会切换数据库代际，也不会删除候选或保留二进制。`);
    if (!accepted) return;
    setBusy(true);
    setMessage(null);
    try {
      const next = await rollbackPreviousSidecar(true);
      setRuntime(next);
      setStagedSha256(next.staged_sidecar_sha256);
      setMessage(`sidecar 已回滚并通过身份校验。活动 SHA-256：${next.active_sidecar_sha256 ?? '未知'}。`);
    } catch (error) {
      const next = await getDesktopRuntime().catch(() => null);
      if (next !== null) setRuntime(next);
      setMessage(error instanceof Error ? error.message : 'sidecar 回滚失败；启动器会尝试恢复当前二进制');
    } finally {
      setBusy(false);
    }
  }

  const selectedSha256 = runtime?.staged_sidecar_sha256 ?? stagedSha256;
  return <article className={styles.sectionCard} aria-labelledby="desktop-sidecar-update-title">
    <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>POLIS DESKTOP / LOCAL UPDATE</span><h2 className={styles.sectionTitle} id="desktop-sidecar-update-title">本机 sidecar 更新</h2></div><Activity aria-hidden="true" className={styles.icon} size={18} /></div>
    <div className={styles.detailRows}>
      <ScopeRow label="活动二进制 SHA-256" value={runtime?.active_sidecar_sha256 ?? '读取中'} />
      <ScopeRow label="暂存候选 SHA-256" value={selectedSha256 ?? '无'} />
      <ScopeRow label="保留上一版 SHA-256" value={runtime?.previous_sidecar_sha256 ?? '无'} />
      <ScopeRow label="切换状态" value={runtime?.sidecar_update_pending ? '启动恢复检查中' : '稳定'} />
    </div>
    <p className={styles.formHint}>仅从本机选定路径暂存并显示 SHA-256；发行者签名尚未验证。激活前请核对候选哈希。只接受与当前 sidecar 使用同一迁移清单的补丁；差异会在停机前拒绝，本地更新不执行数据库回滚。切换会再检查健康状态和可执行文件身份；失败时恢复上一版。数据代际与 sidecar 指针分开保存。</p>
    {runtime?.sidecar_update_blocked ? <p className={styles.formHint} role="status">已设置 POLIS_DESKTOP_POLIS_EXE，开发覆盖模式下暂存、激活和回滚均已禁用。</p> : null}
    <label className={styles.formLabel}>本机候选二进制完整路径<input className={styles.formField} value={sourcePath} onChange={event => setSourcePath(event.target.value)} placeholder="C:\\Downloads\\polis.exe" /></label>
    {message ? <p className={styles.formHint} role="status">{labelErrorMessage(message)}</p> : null}
    <div className={styles.wizardActions}>
      <button className={styles.commandButton} disabled={busy || runtime?.sidecar_update_blocked === true || sourcePath.trim() === ''} onClick={() => { void stageCandidate(); }} type="button">{busy ? '处理中…' : '暂存并计算 SHA-256'}</button>
      {selectedSha256 ? <button className={styles.commandButton} disabled={busy || runtime?.sidecar_update_blocked === true || runtime?.sidecar_update_pending === true} onClick={() => { void activateCandidate(); }} type="button">激活候选</button> : null}
      {runtime?.previous_sidecar_sha256 ? <button className={styles.textButton} disabled={busy || runtime.sidecar_update_blocked || runtime.sidecar_update_pending} onClick={() => { void rollbackCandidate(); }} type="button">回滚到上一版</button> : null}
    </div>
  </article>;
}

function CapabilityCatalogPanel({api, companyId, query}: Readonly<{api: WorkbenchApi; companyId: string; query: ReturnType<typeof useCapabilityCatalog>}>) {
  const importSkillPackage = useImportReadOnlySkillPackage(api, companyId);
  const importMCPPackage = useImportStdioMCPPackage(api, companyId);
  const observeMCPRuntime = useObserveStdioMCPRuntime(api, companyId);
  const observeStreamableHTTPRuntime = useObserveStreamableHTTPMCPRuntime(api, companyId);
  const registerMCP = useRegisterMCP(api, companyId);
  const qualify = useQualifyCapability(api, companyId);
  const decide = useDecideCapability(api, companyId);
  const approveRuntimeQualification = useApproveStdioMCPRuntimeQualification(api, companyId);
  const bind = useBindEmployeeCapability(api, companyId);
  const revokeBinding = useRevokeEmployeeCapability(api, companyId);
  const reviewRevocation = useReviewIncompleteCapabilityRevocation(api, companyId);
  const employeeOverview = useCompanyOverview(api, companyId);
  const [skillRevision, setSkillRevision] = useState('');
  const [skillBundleFile, setSkillBundleFile] = useState<File | null>(null);
  const [mcpName, setMcpName] = useState('');
  const [mcpTransport, setMcpTransport] = useState<'stdio' | 'streamable_http'>('stdio');
  const [mcpEndpoint, setMcpEndpoint] = useState('');
  const [mcpRevision, setMcpRevision] = useState('1.0.0');
  const [mcpBundleFile, setMcpBundleFile] = useState<File | null>(null);
  const [mcpServerToUpdate, setMcpServerToUpdate] = useState('');
  const [selectedEmployee, setSelectedEmployee] = useState('');
  const [rationale, setRationale] = useState('');
  const [message, setMessage] = useState<string | null>(null);
  const pendingRequestIds = useRef(new Map<string, string>());
  const pendingMCPRegistrations = useRef(new Map<string, Readonly<{id: string; requestId: string}>>());
  if (query.isPending) return <div className={styles.emptyState} role="status">正在读取能力候选目录</div>;
  if (query.isError) return <div className={styles.errorState} role="alert">能力目录读取失败：{query.error.message}</div>;
  const qualificationFor = (kind: 'skill' | 'mcp', id: string, digest: string) => query.data.qualifications.find(item => item.capabilityKind === kind && item.capabilityId === id && item.versionDigest === digest);
  async function submitSkill(): Promise<void> {
    setMessage(null);
    try {
      if (skillBundleFile === null) throw new Error('请先选择只读 Skill ZIP');
      const contentDigest = await sha256File(skillBundleFile);
      const payload = {revision: skillRevision, contentDigest};
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'skill-import', payload);
      await importSkillPackage.mutateAsync({revision: skillRevision, bundleFile: skillBundleFile, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setMessage('只读 Skill 来源包已解析并固定摘要，候选仍需人工核验、批准和员工绑定；没有运行脚本。');
    } catch (error) { setMessage(error instanceof Error ? error.message : '技能登记失败'); }
  }
  async function submitMCP(): Promise<void> {
    setMessage(null);
    try {
      if (mcpTransport === 'stdio') {
        if (mcpBundleFile === null) throw new Error('请先选择受控 stdio MCP ZIP 包');
        if (mcpRevision.trim() === '') throw new Error('请填写包版本');
        const serverId = mcpServerToUpdate || null;
        const revision = mcpRevision.trim();
        const contentDigest = await sha256File(mcpBundleFile);
        const payload = {serverId, revision, contentDigest};
        const pending = pendingRequestIdentity(pendingRequestIds.current, 'mcp-package-import', payload);
        const imported = await importMCPPackage.mutateAsync({serverId, revision, bundleFile: mcpBundleFile, requestId: pending.requestId});
        clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
        setMessage(`受控 stdio MCP 包已固定为候选版本 ${imported.revision}（${imported.manifestDigest}）。上传只解析并保存 CAS 文件，未启动本地程序；仍需元数据核验、人工批准、运行观察和员工绑定。`);
        return;
      }
      const name = mcpName.trim();
      if (name === '') throw new Error('请填写 MCP 名称');
      const endpoint = new URL(mcpEndpoint.trim()).toString();
      const descriptor = {schemaVersion: 'mcp-streamable-http-descriptor@1', name, transport: mcpTransport, endpoint, protocolVersion: '2026-07-28'};
      const descriptorDigest = await sha256Text(JSON.stringify(descriptor));
      const payload = {name, transport: mcpTransport, endpoint, descriptorDigest};
      const key = JSON.stringify(payload);
      const pendingRegistration = pendingMCPRegistrations.current.get(key) ?? {
        id: `${name.toLowerCase().replace(/[^a-z0-9_-]+/g, '-') || 'mcp'}-${crypto.randomUUID()}`,
        requestId: `mcp-register-${crypto.randomUUID()}`,
      };
      pendingMCPRegistrations.current.set(key, pendingRegistration);
      await registerMCP.mutateAsync({id: pendingRegistration.id, name, transport: mcpTransport, command: null, endpoint, args: [], descriptorDigest, requestId: pendingRegistration.requestId});
      pendingMCPRegistrations.current.delete(key);
      setMessage('Streamable HTTP 定义已登记，协议 profile 固定为 2026-07-28；登记没有访问端点。');
    } catch (error) { setMessage(error instanceof Error ? error.message : 'MCP 登记失败'); }
  }
  async function qualifyCapability(kind: 'skill' | 'mcp', id: string): Promise<void> {
    setMessage(null);
    try {
      const payload = {capabilityKind: kind, capabilityId: id};
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'capability-qualify', payload);
      const result = await qualify.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      const status = typeof result === 'object' && result !== null && 'status' in result ? String(result.status) : 'recorded';
      const server = kind === 'mcp' ? query.data?.mcpServers.find(item => item.id === id) : undefined;
      const profile = server?.transport === 'streamable_http' ? '固定版本和端点摘要' : '冻结描述';
      setMessage(`元数据核验结果：${labelDisplayValue(status)}。仅检查${profile}；不会访问 MCP 地址，也不授予运行时资格。`);
    } catch (error) { setMessage(error instanceof Error ? error.message : '能力核验失败'); }
  }
  async function decideCapability(kind: 'skill' | 'mcp', id: string, qualificationId: string, decision: 'approved' | 'revoked'): Promise<void> {
    setMessage(null);
    try {
      const payload = {capabilityKind: kind, capabilityId: id, qualificationId, decision, rationale};
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'capability-decision', payload);
      await decide.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setMessage(decision === 'approved' ? '已记录人工批准；能力运行仍被资格门控。' : '已撤销能力并撤销现有员工绑定。');
    } catch (error) { setMessage(error instanceof Error ? error.message : '能力决策失败'); }
  }
  async function reviewIncompleteRevocation(revocationId: string): Promise<void> {
    setMessage(null);
    if (rationale.trim() === '') {
      setMessage('请填写复核理由。');
      return;
    }
    const confirmed = window.confirm('确认以安装所有者身份记录：该历史撤销缺少精确会话清单，现有证据无法证明撤销时的完整 WorkerSession 集合。此复核不会补造清单、不会标记已静止，也不会停止 Worker。');
    if (!confirmed) return;
    try {
      const payload = {revocationId, rationale: rationale.trim()};
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'capability-revocation-review', payload);
      await reviewRevocation.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setMessage('已记录安装所有者复核；历史清单仍不完整，撤销仍未证明静止。');
    } catch (error) { setMessage(error instanceof Error ? error.message : '撤销记录复核失败'); }
  }
  async function approveRuntimeQualificationRecord(runtimeQualificationId: string): Promise<void> {
    setMessage(null);
    try {
      const payload = {runtimeQualificationId, rationale};
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'mcp-runtime-approval', payload);
      await approveRuntimeQualification.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setMessage('MCP 运行资格已记录人工审批；员工绑定仍是单独操作，真实 provider 调用仍关闭，主机隔离资格仍需独立验收。');
    } catch (error) { setMessage(error instanceof Error ? error.message : 'MCP 运行资格审批失败'); }
  }
  async function observeMCPRuntimePackage(serverId: string, packageRevisionId: string, capabilityQualificationId: string): Promise<void> {
    setMessage(null);
    try {
      const payload = {serverId, packageRevisionId, capabilityQualificationId};
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'mcp-runtime-observe', payload);
      await observeMCPRuntime.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setMessage('隔离运行观察完成并记录为“待人工批准”；只发现 server/tool schema，没有调用 MCP 工具。员工绑定仍是单独操作。');
    } catch (error) { setMessage(error instanceof Error ? error.message : 'MCP 隔离运行观察失败'); }
  }
  async function observeStreamableHTTPRuntimeSchema(capabilityId: string, capabilityQualificationId: string): Promise<void> {
    setMessage(null);
    try {
      const payload = {capabilityId, capabilityQualificationId};
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'mcp-http-runtime-observe', payload);
      await observeStreamableHTTPRuntime.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setMessage('已读取固定 HTTPS 端点的 tools/list 并保存 schema 摘要；这次操作没有调用工具。之后仍需人工批准运行资格并单独绑定员工。');
    } catch (error) { setMessage(error instanceof Error ? error.message : 'Streamable HTTP MCP 目录观察失败'); }
  }
  async function bindCapability(kind: 'skill' | 'mcp', id: string, qualificationId: string): Promise<void> {
    if (!selectedEmployee) return;
    setMessage(null);
    try {
      const payload = {employeeId: selectedEmployee, capabilityKind: kind, capabilityId: id, qualificationId, reason: rationale};
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'capability-bind', payload);
      await bind.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setMessage('已将固定版本绑定到员工身份；接班会保留该绑定，运行资格和 provider 调用仍各自受门控。');
    } catch (error) { setMessage(error instanceof Error ? error.message : '员工绑定失败'); }
  }
  async function unbindCapability(item: NonNullable<typeof query.data>['bindings'][number]): Promise<void> {
    try {
      const payload = {employeeId: item.employeeId, capabilityKind: item.capabilityKind, capabilityId: item.capabilityId, qualificationId: item.qualificationId, reason: rationale || 'administrator_revoked'};
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'capability-unbind', payload);
      await revokeBinding.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setMessage('员工能力绑定已撤销。');
    } catch (error) { setMessage(error instanceof Error ? error.message : '绑定撤销失败'); }
  }
  const roster = employeeOverview.data?.employees ?? [];
  const capabilityRows = [
    ...query.data.skills.map(item => ({kind: 'skill' as const, id: item.id, digest: item.contentDigest, label: item.displayName, version: item.revision, status: item.status})),
    ...query.data.mcpServers.map(item => ({kind: 'mcp' as const, id: item.id, digest: item.descriptorDigest, label: item.name, version: item.transport, status: item.status})),
  ];
  const stdioRuntimeQualifications = query.data.runtimeQualifications.filter(item => (item.transport ?? 'stdio') === 'stdio');
  return <div className={styles.viewStack}>
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>治理 / 人工批准</span><h2 className={styles.sectionTitle}>核验、批准与绑定</h2></div><StatusBadge label={`${query.data.qualifications.length} 条本地核验`} tone="info" /></div>
      <div className={styles.formStack}>
        <label className={styles.formLabel}>员工身份<select className={styles.formField} onChange={event => setSelectedEmployee(event.target.value)} value={selectedEmployee}><option value="">选择固定员工</option>{roster.map(employee => <option key={employee.employeeId} value={employee.employeeId}>{employee.displayName} · {employee.role}</option>)}</select></label>
        <label className={styles.formLabel}>批准 / 绑定理由<input className={styles.formField} onChange={event => setRationale(event.target.value)} value={rationale} /></label>
        {employeeOverview.isError ? <div className={styles.errorState} role="alert">员工列表不可用：{employeeOverview.error.message}</div> : null}
      </div>
      <div className={styles.recordList}>{capabilityRows.length === 0 ? <div className={styles.emptyState}>当前没有已登记的能力版本。</div> : capabilityRows.map(item => {
        const qualification = qualificationFor(item.kind, item.id, item.digest);
        const approved = item.status === 'approved';
        const qualifiedMetadata = qualification?.status === 'metadata_verified';
        const profileSupported = item.kind === 'skill' || query.data.mcpServers.some(server => server.id === item.id && (server.transport === 'stdio' || server.transport === 'streamable_http'));
        const employeeBinding = selectedEmployee ? query.data.bindings.find(binding => binding.employeeId === selectedEmployee && binding.capabilityKind === item.kind && binding.capabilityId === item.id) : undefined;
        const alreadyBound = employeeBinding?.state === 'bound' && employeeBinding.versionDigest === item.digest;
        return <div className={styles.recordRow} key={`${item.kind}:${item.id}`}>
          <div className={styles.recordLead}><Settings2 aria-hidden="true" size={16} /><div><strong>{item.label}</strong><span>{item.kind} · {item.version} · {item.digest}</span><span>{item.kind === 'skill' ? 'Skill 来源核验' : 'MCP 描述核验'}：{qualification ? labelDisplayValue(qualification.status) : '未运行'}</span><span>运行时资格：不可用；本地核验没有执行代码或启动 MCP。</span></div></div>
          <div className={styles.recordMeta}><StatusBadge label={labelDisplayValue(item.status)} tone={approved ? 'success' : item.status === 'revoked' ? 'danger' : 'warning'} />
            <button className={styles.textButton} disabled={!profileSupported || qualify.isPending} onClick={() => { void qualifyCapability(item.kind, item.id); }} type="button">{profileSupported ? '核验元数据' : '当前 profile 不支持核验'}</button>
            {qualification && !approved && item.status !== 'revoked' ? <button className={styles.textButton} disabled={!qualifiedMetadata || rationale.trim() === '' || decide.isPending} onClick={() => { void decideCapability(item.kind, item.id, qualification.qualificationId, 'approved'); }} type="button">人工批准</button> : null}
            {approved && qualification ? <button className={styles.textButton} disabled={rationale.trim() === '' || decide.isPending} onClick={() => { void decideCapability(item.kind, item.id, qualification.qualificationId, 'revoked'); }} type="button">撤销能力</button> : null}
            {approved && qualification && selectedEmployee ? <button className={styles.textButton} disabled={!qualifiedMetadata || bind.isPending || alreadyBound} onClick={() => { void bindCapability(item.kind, item.id, qualification.qualificationId); }} type="button">{alreadyBound ? '已绑定' : '绑定到所选员工'}</button> : null}
          </div>
        </div>;
      })}</div>
    </section>
    <section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>员工绑定 / 版本固定</span><h2 className={styles.sectionTitle}>绑定历史与接班状态</h2></div><StatusBadge label={`${query.data.bindings.filter(item => item.state === 'bound').length} 个当前绑定`} tone="info" /></div>{query.data.bindings.length > 0 ? <div className={styles.recordList}>{query.data.bindings.map(item => <div className={styles.recordRow} key={item.eventId}><div className={styles.recordLead}><Handshake aria-hidden="true" size={16} /><div><strong>{item.employeeId} · {item.capabilityKind} / {item.capabilityId}</strong><span>{item.versionDigest} · {labelDisplayValue(item.qualificationStatus)} · {labelDisplayValue(item.executionStatus)}</span><span>绑定关联逻辑员工身份，因此交接会沿用版本；运行路径仍关闭。</span></div></div><div className={styles.recordMeta}><StatusBadge label={labelDisplayValue(item.state)} tone={item.state === 'bound' ? 'warning' : 'neutral'} />{item.state === 'bound' ? <button className={styles.textButton} disabled={rationale.trim() === '' || revokeBinding.isPending} onClick={() => { void unbindCapability(item); }} type="button">撤销绑定</button> : null}</div></div>)}</div> : <div className={styles.emptyState}>尚无能力版本绑定到员工身份。</div>}</section>
    <section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>审批审计</span><h2 className={styles.sectionTitle}>人工决定记录</h2></div><StatusBadge label={`${query.data.decisions.length} 条`} tone="info" /></div>{query.data.decisions.length > 0 ? <div className={styles.recordList}>{query.data.decisions.map(item => <div className={styles.recordRow} key={item.decisionId}><div className={styles.recordLead}><ShieldCheck aria-hidden="true" size={16} /><div><strong>{item.capabilityKind} / {item.capabilityId} · {item.versionDigest}</strong><span>{item.actor} · {item.qualificationId} · {item.rationale}</span></div></div><StatusBadge label={labelDisplayValue(item.decision)} tone={item.decision === 'approved' ? 'success' : 'danger'} /></div>)}</div> : <div className={styles.emptyState}>尚无人工批准或撤销记录。</div>}</section>
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>REQ-14 / 撤销状态</span><h2 className={styles.sectionTitle}>派发门禁与受影响执行</h2></div><StatusBadge label={`${query.data.revocations.length} 条当前撤销`} tone="info" /></div>
      <p className={styles.formHint}>接受撤销后会阻止后续派发。只有会话清单完整、所有受影响 WorkerSession 均已停止，且没有 dispatching MCP 意图时才显示已静止。安装所有者可以对旧记录作“已复核但仍无法证明静止”的不可变记录；这不会补造撤销时清单、改变静止状态或停止 Worker。操作需要先登录安装所有者，并填写复核理由；结果未知会保留在明细中。</p>
      {query.data.revocations.length > 0 ? <div className={styles.recordList}>{query.data.revocations.map(item => <div className={styles.recordRow} key={item.revocationId}>
        <div className={styles.recordLead}><ShieldCheck aria-hidden="true" size={16} /><div>
          <strong>{item.scope === 'employee' ? `${item.employeeId} · ` : ''}{item.capabilityKind} / {item.capabilityId}</strong>
          <span>撤销已接受：{item.revocationAccepted ? '是' : '否'} · 对新派发生效：{item.effectiveForNewDispatch ? '是' : '否'} · {item.acceptedAt}</span>
          <span>撤销时会话清单：{item.sessionInventoryComplete ? '完整' : '不完整，需复核'}</span>
          {item.ownerReview ? <span>安装所有者复核：已记录但仍无法证明静止 · {item.ownerReview.reviewedAt} · {item.ownerReview.rationale}</span> : null}
          <span>WorkerSession：{item.liveSessionCount} 个未停止 / {item.affectedSessionCount} 个受影响 · MCP：{item.dispatchingMcpCallCount} 个派发中 / {item.mcpCallCount} 个意图</span>
          {item.sessions.map(session => <span key={session.sessionId}>会话 {session.sessionId} · {session.employeeId} · 当前 {session.state}{session.stateAtRevocation ? ` / 撤销时 ${session.stateAtRevocation}` : ''} · Skill 加载 {session.skillLoadCount} · MCP 调用 {session.mcpCallCount}</span>)}
          {item.mcpCalls.map(call => <span key={call.intentId}>MCP {call.intentId} · {call.toolName} · 当前 {call.status === 'dispatching' ? '派发中' : call.status === 'completed' ? '已完成' : `结果未知${call.reasonCode ? `（${call.reasonCode}）` : ''}`}{call.statusAtRevocation ? ` / 撤销时 ${call.statusAtRevocation}` : ''}</span>)}
          {item.sessionsTruncated || item.mcpCallsTruncated ? <span>明细已截断；数量汇总仍按完整清单计算。</span> : null}
        </div></div><div className={styles.recordMeta}><StatusBadge label={item.quiesced ? '已静止' : item.sessionInventoryComplete ? '尚未静止' : item.ownerReview ? '已复核，静止未证' : '需复核'} tone={item.quiesced ? 'success' : 'warning'} />{!item.sessionInventoryComplete && !item.ownerReview ? <button className={styles.textButton} disabled={rationale.trim() === '' || reviewRevocation.isPending} onClick={() => { void reviewIncompleteRevocation(item.revocationId); }} type="button">安装所有者复核</button> : null}</div>
      </div>)}</div> : <div className={styles.emptyState}>当前没有生效中的能力撤销。</div>}
      {query.data.revocationsTruncated ? <div className={styles.formHint}>当前仅显示最近的 64 条撤销状态。</div> : null}
    </section>
    <section className={styles.detailGrid}>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>技能版本目录</span><h2 className={styles.sectionTitle}>受控只读 Skill 导入</h2></div><StatusBadge label={`${query.data.skills.length} 个`} tone="info" /></div><div className={styles.formStack}><label className={styles.formLabel}>Skill 版本<input className={styles.formField} maxLength={64} onChange={event => setSkillRevision(event.target.value)} value={skillRevision} /></label><label className={styles.formLabel}>ZIP 来源包<input accept=".zip,application/zip" className={styles.formField} onChange={event => setSkillBundleFile(event.target.files?.[0] ?? null)} type="file" /></label><p className={styles.formHint}>ZIP 根目录或一个顶层目录中需包含 SKILL.md；仅接收 SKILL.md、references 中的 Markdown/TXT、assets 中的 Markdown/TXT/PNG/JPEG 和 LICENSE。脚本、嵌套归档、SVG、未知文件、链接与越界路径会被拒绝。上传只解析并保存 CAS 文件，不安装或运行代码。</p><button className={styles.commandButton} disabled={importSkillPackage.isPending || skillBundleFile === null || skillRevision.trim() === ''} onClick={() => { void submitSkill(); }} type="button">{importSkillPackage.isPending ? '导入并校验中…' : '导入只读 Skill 候选'}</button></div></article>
      <article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>受控 MCP 定义</span><h2 className={styles.sectionTitle}>固定传输 profile 候选</h2></div><StatusBadge label={`${query.data.mcpServers.length} 个`} tone="info" /></div><div className={styles.formStack}><label className={styles.formLabel}>传输方式<select className={styles.formField} onChange={event => setMcpTransport(event.target.value as 'stdio' | 'streamable_http')} value={mcpTransport}><option value="stdio">标准输入输出</option><option value="streamable_http">Streamable HTTP（2026-07-28）</option></select></label>{mcpTransport === 'stdio' ? <><label className={styles.formLabel}>包版本<input className={styles.formField} maxLength={64} onChange={event => setMcpRevision(event.target.value)} value={mcpRevision} /></label><label className={styles.formLabel}>更新已有定义<select className={styles.formField} onChange={event => setMcpServerToUpdate(event.target.value)} value={mcpServerToUpdate}><option value="">创建新定义</option>{query.data.mcpServers.filter(item => item.transport === 'stdio').map(item => <option key={item.id} value={item.id}>{item.name} · {item.id}</option>)}</select></label><label className={styles.formLabel}>受控 stdio MCP ZIP<input accept=".zip,application/zip" className={styles.formField} onChange={event => setMcpBundleFile(event.target.files?.[0] ?? null)} type="file" /></label><p className={styles.formHint}>ZIP 根目录需含 mcp-package.json；文件清单、命令、参数和 MCP 版本必须匹配 manifest。导入只解析并保存公司 CAS，不启动代码。新包版本会撤销旧运行观察，需重新观察并人工批准。</p></> : <><label className={styles.formLabel}>名称<input className={styles.formField} onChange={event => setMcpName(event.target.value)} value={mcpName} /></label><label className={styles.formLabel}>HTTPS 访问地址<input className={styles.formField} onChange={event => setMcpEndpoint(event.target.value)} placeholder="仅登记；元数据核验不会访问" value={mcpEndpoint} /></label><p className={styles.formHint}>Streamable HTTP 固定为 2026-07-28；登记不访问端点，连接和调用仍受资格门控。</p></>}<button className={styles.commandButton} disabled={mcpTransport === 'stdio' ? importMCPPackage.isPending || mcpBundleFile === null || mcpRevision.trim() === '' : registerMCP.isPending || mcpName.trim() === '' || mcpEndpoint.trim() === ''} onClick={() => { void submitMCP(); }} type="button">{mcpTransport === 'stdio' ? importMCPPackage.isPending ? '导入并校验中…' : '导入受控 stdio MCP 包' : registerMCP.isPending ? '登记中…' : '登记 Streamable HTTP 定义'}</button>{query.data.mcpPackages.length > 0 ? <div className={styles.recordList}>{query.data.mcpPackages.map(item => <div className={styles.recordRow} key={item.id}><div className={styles.recordLead}><ShieldCheck aria-hidden="true" size={16} /><div><strong>{item.manifest.name} · {item.revision}</strong><span>{item.serverId} · {item.manifest.serverName} {item.manifest.serverVersion}</span><span>包摘要 {item.manifestDigest} · {item.manifest.files.length} 个固定文件</span></div></div><StatusBadge label="候选 / 未执行" tone="warning" /></div>)}</div> : null}</div></article>
    </section>
    {query.data.mcpPackages.length > 0 ? <MCPRuntimeObservationPanel
      packages={query.data.mcpPackages} servers={query.data.mcpServers} qualifications={query.data.qualifications} decisions={query.data.decisions}
      available={query.data.runtimeObservationAvailable} pending={observeMCPRuntime.isPending} onObserve={observeMCPRuntimePackage}
    /> : null}
    {query.data.mcpServers.some(server => server.transport === 'streamable_http') || query.data.streamableHttpRuntimeObservationAvailable ? <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>R2 / Streamable HTTP MCP</span><h2 className={styles.sectionTitle}>远端工具目录资格</h2></div><StatusBadge label={query.data.streamableHttpRuntimeObservationAvailable ? '按需读取已启用' : '外部访问关闭'} tone={query.data.streamableHttpRuntimeObservationAvailable ? 'warning' : 'neutral'} /></div>
      <p className={styles.formHint}>只有固定 HTTPS 描述符已人工批准后，点击观察才会向该端点发送一次 tools/list 请求。观察不执行工具；外部访问由 POLIS_MCP_STREAMABLE_HTTP_ENABLED 默认关闭。schema 观察、运行资格批准和员工绑定是三个独立步骤。</p>
      {query.data.mcpServers.filter(server => server.transport === 'streamable_http').map(server => {
        const qualification = qualificationFor('mcp', server.id, server.descriptorDigest);
        const latestDecision = qualification ? query.data.decisions.find(item => item.capabilityId === server.id && item.qualificationId === qualification.qualificationId && item.versionDigest === server.descriptorDigest) : undefined;
        const canObserve = query.data.streamableHttpRuntimeObservationAvailable && server.status === 'approved' && qualification?.status === 'metadata_verified' && latestDecision?.decision === 'approved';
        const runtimeRecord = query.data.runtimeQualifications.find(item => item.transport === 'streamable_http' && item.capabilityId === server.id && item.versionDigest === server.descriptorDigest);
        return <div className={styles.recordRow} key={server.id}>
          <div className={styles.recordLead}><ShieldCheck aria-hidden="true" size={16} /><div><strong>{server.name}</strong><span>{server.id} · {server.endpoint ?? '缺少 HTTPS 地址'}</span><span>{runtimeRecord ? `schema ${runtimeRecord.toolSchemaSha256} · ${labelDisplayValue(runtimeRecord.status)}` : '尚无远端 schema 观察'}</span></div></div>
          <div className={styles.recordMeta}><button className={styles.textButton} disabled={!canObserve || observeStreamableHTTPRuntime.isPending || qualification === undefined} onClick={() => { if (qualification) void observeStreamableHTTPRuntimeSchema(server.id, qualification.qualificationId); }} type="button">{observeStreamableHTTPRuntime.isPending ? '读取中…' : '读取 tools/list'}</button></div>
        </div>;
      })}
      {query.data.runtimeQualifications.filter(item => item.transport === 'streamable_http').map(item => <div className={styles.recordRow} key={item.runtimeQualificationId}>
        <div className={styles.recordLead}><ShieldCheck aria-hidden="true" size={16} /><div><strong>{item.serverName} · 远端 profile</strong><span>{item.endpoint} · {item.hostProfile} · {item.protocolVersion}</span><span>工具 schema {item.toolSchemaSha256}</span></div></div>
        <div className={styles.recordMeta}><StatusBadge label={labelDisplayValue(item.status)} tone={item.status === 'qualified' ? 'success' : item.status === 'schema_drift' || item.status === 'revoked' ? 'danger' : 'warning'} />{item.status === 'observed_unqualified' ? <button className={styles.textButton} disabled={rationale.trim() === '' || approveRuntimeQualification.isPending} onClick={() => { void approveRuntimeQualificationRecord(item.runtimeQualificationId); }} type="button">批准运行资格</button> : null}</div>
      </div>)}
    </section> : null}
    {stdioRuntimeQualifications.length > 0 || query.data.runtimeObservationAvailable ? <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>受控 stdio MCP</span><h2 className={styles.sectionTitle}>运行资格记录</h2></div><StatusBadge label={`${stdioRuntimeQualifications.length} 条记录`} tone="info" /></div>
      <p className={styles.formHint}>运行观察会固定命令、包版本和工具 schema，并在 Windows deny-all AppContainer 停止进程后保存结果。观察状态仍需单独人工批准；真实 provider 的 MCP 调用仍关闭，观察记录也不等于整个 Windows 主机已通过发布验收。</p>
      {stdioRuntimeQualifications.length === 0 ? <div className={styles.emptyState}>尚无运行观察记录。</div> : <div className={styles.recordList}>{stdioRuntimeQualifications.map(item => <div className={styles.recordRow} key={item.runtimeQualificationId}>
        <div className={styles.recordLead}><ShieldCheck aria-hidden="true" size={16} /><div><strong>{item.serverName} · {item.serverVersion}</strong><span>{item.capabilityId} · {item.hostProfile} · {item.protocolVersion}</span><span>工具 schema {item.toolSchemaSha256}</span><span>命令 {item.commandSha256} · 包 {item.packageManifestSha256}</span></div></div>
        <div className={styles.recordMeta}><StatusBadge label={labelDisplayValue(item.status)} tone={item.status === 'qualified' ? 'success' : item.status === 'schema_drift' || item.status === 'revoked' ? 'danger' : 'warning'} />{item.status === 'observed_unqualified' ? <button className={styles.textButton} disabled={rationale.trim() === '' || approveRuntimeQualification.isPending} onClick={() => { void approveRuntimeQualificationRecord(item.runtimeQualificationId); }} type="button">批准运行资格</button> : null}</div>
      </div>)}</div>}
    </section> : null}
    {message ? <div className={styles.operationNotice} role="status">{message}</div> : null}
      <p className={styles.formHint}>Skill 脚本始终不执行。stdio 观察仅在本机显式启用时运行一次本地 discovery/tools/list，网络保持拒绝；Streamable HTTP 访问默认关闭，启用后也仅在点击观察时读取 tools/list。外部 Worker 工具调用还需要当前元数据批准、运行资格批准和员工版本绑定。</p>
  </div>;
}

function MCPRuntimeObservationPanel({packages, servers, qualifications, decisions, available, pending, onObserve}: Readonly<{
  packages: ReadonlyArray<StdioMCPPackageRevisionView>;
  servers: ReadonlyArray<MCPServerDefinitionView>;
  qualifications: ReadonlyArray<CapabilityQualificationView>;
  decisions: ReadonlyArray<CapabilityDecisionView>;
  available: boolean;
  pending: boolean;
  onObserve(serverId: string, packageRevisionId: string, capabilityQualificationId: string): Promise<void>;
}>): ReactElement {
  return <section className={styles.sectionCard}>
    <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>运行观察 / 手动操作</span><h2 className={styles.sectionTitle}>受控 stdio MCP 运行观察</h2></div><StatusBadge label={available ? '受控观察已启用' : '当前关闭'} tone={available ? 'warning' : 'neutral'} /></div>
    <p className={styles.formHint}>点击“运行隔离观察”会启动所选本地包一次，在网络拒绝的 Windows AppContainer 中执行 server discovery 和 tools/list，随后确认进程树停止。观察不会调用 MCP 工具；结果仍需人工批准，Employee 版本绑定是另一项操作。</p>
    {!available ? <div className={styles.emptyState}>本机未启用受控运行观察；当前不会启动任何 MCP 程序。</div> : null}
    <div className={styles.recordList}>{packages.map(packageRevision => {
      const server = servers.find(candidate => candidate.id === packageRevision.serverId);
      const latestPackage = packages.find(candidate => candidate.serverId === packageRevision.serverId);
      const qualification = server ? qualifications.find(candidate => candidate.capabilityKind === 'mcp' && candidate.capabilityId === server.id && candidate.versionDigest === server.descriptorDigest && candidate.status === 'metadata_verified') : undefined;
      const latestDecision = qualification ? decisions.find(candidate => candidate.capabilityKind === 'mcp' && candidate.capabilityId === packageRevision.serverId && candidate.versionDigest === server?.descriptorDigest && candidate.qualificationId === qualification.qualificationId) : undefined;
      const canObserve = available && latestPackage?.id === packageRevision.id && server?.status === 'approved' && qualification !== undefined && latestDecision?.decision === 'approved';
      return <div className={styles.recordRow} key={packageRevision.id}>
        <div className={styles.recordLead}><ShieldCheck aria-hidden="true" size={16} /><div><strong>{packageRevision.manifest.name} · {packageRevision.revision}</strong><span>{packageRevision.serverId} · {packageRevision.manifest.serverName} {packageRevision.manifest.serverVersion}</span><span>固定包摘要 {packageRevision.manifestDigest}</span></div></div>
        <div className={styles.recordMeta}>
          <StatusBadge label={latestPackage?.id === packageRevision.id ? '当前版本' : '旧版本'} tone={latestPackage?.id === packageRevision.id ? 'info' : 'neutral'} />
          <button className={styles.textButton} disabled={!canObserve || pending || qualification === undefined} onClick={() => { if (qualification) void onObserve(packageRevision.serverId, packageRevision.id, qualification.qualificationId); }} type="button">{pending ? '观察中…' : '运行隔离观察'}</button>
        </div>
      </div>;
    })}</div>
  </section>;
}

async function sha256Text(value: string): Promise<string> {
  const bytes = new TextEncoder().encode(value);
  return sha256Bytes(bytes);
}

async function sha256File(file: File): Promise<string> {
  if (file.size < 1 || file.size > 8 * 1024 * 1024) {
    throw new Error('能力包文件须为 1 字节至 8 MiB。');
  }
  return sha256Bytes(new Uint8Array(await file.arrayBuffer()));
}

async function sha256Bytes(bytes: Uint8Array<ArrayBuffer>): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', bytes);
  return Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, '0')).join('');
}

function RuntimeReadinessPanel({runtime}: Readonly<{runtime: RuntimeSettingsView}>): ReactElement {
  return <section className={styles.detailGrid}><article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>运行时配置</span><h2 className={styles.sectionTitle}>当前运行配置</h2></div><Settings2 aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.detailRows}><ScopeRow label="提供方" value={runtime.provider} /><ScopeRow label="模型" value={runtime.model} /><ScopeRow label="推理强度" value={runtime.effort} /><ScopeRow label="配置档案" value={runtime.profile} /><ScopeRow label="运行时版本" value={labelDisplayValue(runtime.runtimeVersion)} /><ScopeRow label="产品工具面" value={labelDisplayValue(runtime.productSurfaceQualification)} /></div></article><article className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>就绪状态</span><h2 className={styles.sectionTitle}>依赖状态</h2></div><StatusBadge label={readinessLabel(runtime.authReadiness)} tone={runtime.authReadiness === 'ready' || runtime.authReadiness === 'not_required' ? 'success' : 'warning'} /></div><div className={styles.detailRows}><ScopeRow label="身份认证" value={readinessLabel(runtime.authReadiness)} /><ScopeRow label="PostgreSQL" value={readinessLabel(runtime.postgresqlStatus)} /><ScopeRow label="CAS" value={readinessLabel(runtime.casStatus)} /><ScopeRow label="事件流" value={readinessLabel(runtime.eventStreamStatus)} /><ScopeRow label="工作区" value={runtime.workspaceRoot} /></div></article></section>;
}

function SafetyPanel({runtime}: Readonly<{runtime: RuntimeSettingsView}>): ReactElement {
  return <section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>安全政策</span><h2 className={styles.sectionTitle}>安全边界</h2></div><ShieldCheck aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.checkList}><div><CheckCircle2 aria-hidden="true" size={15} /><span>前端不接触数据库凭据和模型凭据</span></div><div><CheckCircle2 aria-hidden="true" size={15} /><span>浏览和就绪检查不会启动模型业务回合</span></div><div><CheckCircle2 aria-hidden="true" size={15} /><span>当前模型工具面：{labelDisplayValue(runtime.productSurfaceQualification)}</span></div><div><LockKeyhole aria-hidden="true" size={15} /><span>外部通知和 QQ 路由需要独立资格与授权</span></div></div></section>;
}

function ScopeHeader({action, eyebrow, title}: Readonly<{action?: ReactElement; eyebrow: string; title: string}>): ReactElement {
  return <div className={styles.viewHeader}><div><p className={styles.eyebrow}>{eyebrow}</p><h1 className={styles.pageTitle}>{title}</h1></div>{action ? <div className={styles.viewHeaderAction}>{action}</div> : null}</div>;
}

function ScopeLoading({label, title}: Readonly<{label: string; title: string}>): ReactElement {
  return <div className={styles.viewStack}><ScopeHeader eyebrow="读取中" title={title} /><div className={styles.emptyState} role="status">{label}</div></div>;
}

function ScopeError({message, title}: Readonly<{message: string; title: string}>): ReactElement {
  return <div className={styles.viewStack}><ScopeHeader eyebrow="只读数据读取失败" title={title} /><div className={styles.errorState} role="alert"><CircleAlert aria-hidden="true" size={18} /><div><strong>无法读取当前作用域</strong><p>{labelErrorMessage(message)}</p></div></div></div>;
}

function ResourceMetric({detail, icon, label, value}: Readonly<{detail: string; icon: ReactElement; label: string; value: string}>): ReactElement {
  return <article className={styles.resourceCard}><span className={styles.cardEyebrow}>{label}</span><strong>{icon}{value}</strong><p>{detail}</p><StatusBadge label="当前快照" tone="info" /></article>;
}

function CompanyResourceRow({company}: Readonly<{company: CompanySummaryView}>): ReactElement {
  return <div className={styles.recordRow}><div className={styles.recordLead}><Building2 aria-hidden="true" size={16} /><div><strong>{company.name}</strong><span>{company.id} · {company.workspaceRoot}</span></div></div><div className={styles.recordMeta}><span>{company.roster.length} 个角色</span><StatusBadge label={labelDisplayValue(company.state)} tone={company.state === 'active' ? 'success' : 'neutral'} /></div></div>;
}

function AttentionRow({api, companyId, item}: Readonly<{api: WorkbenchApi; companyId: string; item: AttentionItem}>): ReactElement {
  const stateMutation = useSetHumanInterventionState(api, companyId);
  const [actionError, setActionError] = useState<string | null>(null);
  const pendingRequest = useRef<Readonly<{state: 'acknowledged' | 'resolved'; requestId: string}> | null>(null);
  const isIntervention = item.subject.kind === 'human_intervention' && item.workflowState !== undefined;
  async function updateState(state: 'acknowledged' | 'resolved'): Promise<void> {
    setActionError(null);
    const pending = pendingRequest.current?.state === state
      ? pendingRequest.current
      : {state, requestId: `human-${item.subject.id}-${state}-${crypto.randomUUID()}`};
    pendingRequest.current = pending;
    try {
      await stateMutation.mutateAsync({interventionId: item.subject.id, state, requestId: pending.requestId});
      pendingRequest.current = null;
    } catch (error) {
      const detail = error instanceof Error ? error.message : '命令结果未知';
      setActionError(`人工介入状态更新结果尚未确认：${detail} 已刷新介入状态与活动，请核对后使用同一请求 ID 重试。`);
    }
  }
  return <div className={styles.recordRow}>
    <div className={styles.recordLead}><CircleAlert aria-hidden="true" size={16} /><div><strong>{item.title}</strong><span>{item.subject.label} · {item.description}</span></div></div>
    <div className={styles.recordMeta}>
      <StatusBadge label={labelDisplayValue(item.tone)} tone={item.tone === 'danger' ? 'danger' : item.tone === 'warning' ? 'warning' : 'info'} />
      {item.workflowState ? <StatusBadge label={labelDisplayValue(item.workflowState)} tone={item.workflowState === 'acknowledged' ? 'success' : 'warning'} /> : null}
      {item.notificationState ? <StatusBadge label={`QQ 提醒 · ${labelDisplayValue(item.notificationState)}`} tone={item.notificationState === 'provider_accepted' || item.notificationState === 'delivered' ? 'success' : item.notificationState === 'outcome_unknown' || item.notificationState === 'exhausted' || item.notificationState === 'rejected' ? 'danger' : 'info'} /> : null}
      {isIntervention && api.mode === 'real' ? <div className={styles.commandGroup}>
        {item.workflowState === 'open' ? <button className={styles.commandButton} disabled={stateMutation.isPending} onClick={() => { void updateState('acknowledged'); }} type="button">确认接管</button> : null}
        <button className={styles.commandButton} disabled={stateMutation.isPending} onClick={() => { void updateState('resolved'); }} type="button">标记已解决</button>
      </div> : null}
      {actionError ? <span className={styles.errorState} role="alert">{labelErrorMessage(actionError)}</span> : null}
    </div>
  </div>;
}

function ScopeRow({label, value}: Readonly<{label: string; value: string}>): ReactElement {
  return <div className={styles.detailRow}><span className={styles.fieldLabel}>{label}</span><span className={styles.detailValue}>{value}</span></div>;
}

function readinessLabel(value: string): string {
  const labels: Record<string, string> = {ready: '已就绪', not_required: '无需认证', configured: '已配置', missing: '缺少配置', invalid: '配置无效', restart_required: '需要重启', unavailable: '不可用', deferred: '已延期', not_applicable: '不适用', reported: '已报告', unavailable_data: '数据不可得'};
  return labels[value] ?? value;
}
