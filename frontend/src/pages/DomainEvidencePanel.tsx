// pattern: Imperative Shell

import {useEffect, useRef, useState, type ReactElement} from 'react';
import {useCompanyOverview, useDomainEvidence, useRecordDomainEvidence, useRecordDomainEvidenceReview, useRecordDomainEvidenceSubstantiveAssessment, useRecordDomainProfileQualification} from '../data/workbench-query';
import type {WorkbenchApi} from '../data/workbench-api';
import type {DomainEvidenceArtifactPreviewEntryView, DomainEvidenceArtifactPreviewManifestView, DomainEvidenceArtifactPreviewView, DomainEvidenceAreaAssessmentOutcomeView, DomainEvidenceAreaView, DomainEvidenceRecordView, DomainEvidenceReviewOutcomeView, EmployeeSummary} from '../domain/workbench';
import {buildDomainEvidenceItems, type DomainEvidenceDraftFields} from '../domain/domain-evidence-form';
import {buildDomainProfileQualificationReport} from '../domain/domain-profile-qualification-report';
import {labelDisplayValue} from '../domain/display-labels';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import {ResearchSimulationPanel} from './ResearchSimulationPanel';
import {ContentOperationsPanel} from './ContentOperationsPanel';
import {ResearchSourcePanel} from './ResearchSourcePanel';
import styles from '../styles/workbench.module.css';

type DomainEvidencePanelProps = Readonly<{api: WorkbenchApi; companyId: string}>;

const EMPTY_DRAFT: DomainEvidenceDraftFields = {
  artifactId: '', revision: '', sha256: '', methodSHA256: '', assessedByEmployeeId: '',
};

const AREA_LABELS: Readonly<Record<DomainEvidenceAreaView, string>> = {
  quality: '质量结果',
  intervention: '人工介入',
  recovery: '恢复能力',
  cost: '成本测量',
  organization_benefit: '组织收益',
};

type DomainEvidenceReviewDraft = Readonly<{
  outcome: DomainEvidenceReviewOutcomeView | '';
  reviewerEmployeeId: string;
  rationale: string;
}>;

const EMPTY_REVIEW_DRAFT: DomainEvidenceReviewDraft = {outcome: '', reviewerEmployeeId: '', rationale: ''};

const REVIEW_OUTCOME_LABELS: Readonly<Record<DomainEvidenceReviewOutcomeView, string>> = {
  evidence_references_accepted: '引用字段已核对',
  evidence_references_rejected: '引用字段被拒绝',
  more_evidence_required: '需要补充证据',
};

type DomainEvidencePreviewState = Readonly<{
  loading: boolean;
  error: string | null;
  preview: DomainEvidenceArtifactPreviewView | null;
  relativePath: string | null;
  objectUrl: string | null;
}>;

type DomainEvidencePreviewManifestState = Readonly<{
  loading: boolean;
  error: string | null;
  manifest: DomainEvidenceArtifactPreviewManifestView | null;
}>;

const EMPTY_PREVIEW_STATE: DomainEvidencePreviewState = {loading: false, error: null, preview: null, relativePath: null, objectUrl: null};
const EMPTY_PREVIEW_MANIFEST_STATE: DomainEvidencePreviewManifestState = {loading: false, error: null, manifest: null};

function pendingRequestIdentity(pendingIds: Map<string, string>, operation: string, payload: unknown): Readonly<{key: string; requestId: string}> {
  const key = JSON.stringify({operation, payload});
  const requestId = pendingIds.get(key) ?? `${operation}-${crypto.randomUUID()}`;
  pendingIds.set(key, requestId);
  return {key, requestId};
}

function clearPendingRequestIdentity(pendingIds: Map<string, string>, key: string): void {
  pendingIds.delete(key);
}

function domainEvidenceAreaKey(companyId: string, recordId: string, area: DomainEvidenceAreaView): string {
  return `${companyId}:${recordId}:${area}`;
}

function evidencePreviewUnavailableReason(entry: DomainEvidenceArtifactPreviewEntryView): string {
  if (entry.mediaType === 'application/pdf') return 'PDF 原件不在应用中运行；如果包内有文本提取结果，请预览该文本文件。';
  if (entry.relativePath === 'pdf/extraction.json' || entry.fileName === '.polis-git-source.json') return '这是解析/来源元数据，不作为领域证据内容。';
  return '此文件类型不在安全预览列表中。';
}

export function DomainEvidencePanel({api, companyId}: DomainEvidencePanelProps): ReactElement {
  const query = useDomainEvidence(api, companyId);
  const overviewQuery = useCompanyOverview(api, companyId);
  const recordEvidence = useRecordDomainEvidence(api, companyId);
  const recordReview = useRecordDomainEvidenceReview(api, companyId);
  const recordQualification = useRecordDomainProfileQualification(api, companyId);
  const [profileId, setProfileId] = useState('content-operations-reference');
  const [drafts, setDrafts] = useState<Readonly<Partial<Record<DomainEvidenceAreaView, DomainEvidenceDraftFields>>>>({});
  const [message, setMessage] = useState<string | null>(null);
  const [qualificationInputId, setQualificationInputId] = useState('');
  const [qualificationInputRevision, setQualificationInputRevision] = useState('');
  const [qualificationRationale, setQualificationRationale] = useState('');
  const [qualificationMessage, setQualificationMessage] = useState<string | null>(null);
  const [reviewDrafts, setReviewDrafts] = useState<Readonly<Record<string, DomainEvidenceReviewDraft>>>({});
  const [previews, setPreviews] = useState<Readonly<Record<string, DomainEvidencePreviewState>>>({});
  const [previewManifests, setPreviewManifests] = useState<Readonly<Record<string, DomainEvidencePreviewManifestState>>>({});
  const pendingRequestIds = useRef(new Map<string, string>());
  const previewUrls = useRef(new Set<string>());
  const previewGeneration = useRef(0);

  useEffect(() => {
    const generation = previewGeneration.current + 1;
    previewGeneration.current = generation;
    setPreviews({});
    setPreviewManifests({});
    const objectUrls = previewUrls.current;
    return () => {
      if (previewGeneration.current === generation) previewGeneration.current++;
      for (const objectUrl of objectUrls) URL.revokeObjectURL(objectUrl);
      objectUrls.clear();
    };
  }, [companyId]);

  if (query.isPending) return <section className={styles.sectionCard} role="status">正在读取领域证据记录…</section>;
  if (query.isError) return <section className={styles.errorState} role="alert">领域证据读取失败：{query.error.message}</section>;

  const profile = query.data.profiles.find(item => item.id === profileId) ?? query.data.profiles[0];
  if (profile === undefined) return <section className={styles.emptyState}>当前版本没有领域验收 profile。</section>;
  const employees = overviewQuery.data?.employees ?? [];
  const reviewers = employees.filter(employee => employee.role === 'review');
  const profileSubmissions = query.data.submissions.filter(item => item.profileId === profile.id);
  const currentQualification = query.data.qualifications.find(item => item.profileId === profile.id) ?? null;
  const qualificationReport = buildDomainProfileQualificationReport(profile, profileSubmissions);

  function updateDraft(area: DomainEvidenceAreaView, field: keyof DomainEvidenceDraftFields, value: string): void {
    setDrafts(current => ({...current, [area]: {...(current[area] ?? EMPTY_DRAFT), [field]: value}}));
  }

  async function submitEvidence(): Promise<void> {
    setMessage(null);
    const result = buildDomainEvidenceItems(profile, drafts);
    if (!result.success) {
      const [, area] = result.reason.split(':');
      setMessage(result.reason.startsWith('complete_or_clear')
        ? `请补完“${AREA_LABELS[area as DomainEvidenceAreaView] ?? area}”的证据行，或清空该行。`
        : `“${AREA_LABELS[area as DomainEvidenceAreaView] ?? area}”的修订号或摘要格式无效。`);
      return;
    }
    if (employees.length === 0 && result.evidence.length > 0) {
      setMessage('公司员工名单尚未加载，无法选择评估人。');
      return;
    }
    try {
      const payload = {
        profileId: profile.id,
        profileRevision: profile.revision,
        evidence: result.evidence,
      };
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'domain-evidence', payload);
      const record = await recordEvidence.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setDrafts({});
      setMessage(record.readinessStatus === 'ready_for_review'
        ? '证据引用已记入不可变记录，当前进入待人工审核；这不会取得资格或开启执行。'
        : '已记录为不完整证据；该领域仍未取得资格，执行保持关闭。');
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '领域证据提交失败');
    }
  }

  async function submitProfileQualification(decision: 'qualified' | 'revoked'): Promise<void> {
    setQualificationMessage(null);
    if (qualificationRationale.trim() === '' || Array.from(qualificationRationale).length > 2000) {
      setQualificationMessage('请填写不超过 2000 字的管理者决定理由。');
      return;
    }
    if (decision === 'qualified' && (qualificationInputId.trim() === '' || !/^[1-9]\d*$/.test(qualificationInputRevision))) {
      setQualificationMessage('资格决定需要引用聚合评估报告 MissionInput ID 和修订号。');
      return;
    }
    try {
      const payload = {
        profileId: profile.id,
        profileRevision: profile.revision,
        decision,
        ...(decision === 'qualified' ? {evidenceInputId: qualificationInputId.trim(), evidenceInputRevision: qualificationInputRevision} : {}),
        rationale: qualificationRationale.trim(),
      };
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'domain-profile-qualification', payload);
      const record = await recordQualification.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setQualificationRationale('');
      setQualificationMessage(decision === 'qualified'
        ? `已记录全局领域资格决定 ${record.eventId}；产品执行仍保持关闭。`
        : `已撤销领域资格 ${record.eventId}；产品执行仍保持关闭。`);
    } catch (error) {
      setQualificationMessage(error instanceof Error ? error.message : '领域资格决定失败');
    }
  }

  function downloadProfileQualificationReport(): void {
    if (!qualificationReport.success) return;
    const file = new Blob([qualificationReport.content], {type: 'application/json'});
    const downloadURL = URL.createObjectURL(file);
    const link = document.createElement('a');
    link.href = downloadURL;
    link.download = `r3-${profile.id}-${profile.revision.replace(/[^a-z0-9-]/gi, '-')}-qualification.json`;
    document.body.appendChild(link);
    link.click();
    link.remove();
    window.setTimeout(() => URL.revokeObjectURL(downloadURL), 1000);
  }

  function updateReviewDraft(recordId: string, field: keyof DomainEvidenceReviewDraft, value: string): void {
    setReviewDrafts(current => ({
      ...current,
      [recordId]: {...(current[recordId] ?? EMPTY_REVIEW_DRAFT), [field]: value},
    }));
  }

  async function submitReview(record: DomainEvidenceRecordView): Promise<void> {
    const recordId = record.recordId;
    const hasPreviewedEveryItem = record.submission.evidence.every(item => previews[domainEvidenceAreaKey(companyId, recordId, item.area)]?.preview !== null && previews[domainEvidenceAreaKey(companyId, recordId, item.area)]?.preview !== undefined);
    setMessage(null);
    if (!hasPreviewedEveryItem) {
      setMessage('请先成功预览本提交的每项证据，再记录审核。');
      return;
    }
    const draft = reviewDrafts[recordId] ?? EMPTY_REVIEW_DRAFT;
    if (draft.outcome === '' || draft.reviewerEmployeeId === '' || draft.rationale.trim() === '' || Array.from(draft.rationale).length > 2000) {
      setMessage('请选择 review 岗审核人和引用审核结果，并填写不超过 2000 字的理由。');
      return;
    }
    try {
      const previewedEvidence = record.submission.evidence.map(item => {
        const preview = previews[domainEvidenceAreaKey(companyId, recordId, item.area)]?.preview;
        return preview === null || preview === undefined ? null : {
          area: item.area,
          relativePath: preview.relativePath,
          sourceDigest: preview.sourceDigest,
          contentDigest: preview.contentDigest,
          mediaType: preview.mediaType,
        };
      }).filter((item): item is NonNullable<typeof item> => item !== null);
      const payload = {
        recordId,
        outcome: draft.outcome,
        reviewerEmployeeId: draft.reviewerEmployeeId,
        rationale: draft.rationale,
        previewedEvidence,
      };
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'domain-evidence-review', payload);
      await recordReview.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setMessage('引用审核已写入不可变记录；它不代表底层内容质量，不取得领域资格，也不会开启执行。');
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '领域证据引用审核失败');
    }
  }

  async function loadEvidencePreviewManifest(recordId: string, item: DomainEvidenceRecordView['submission']['evidence'][number]): Promise<void> {
    const key = domainEvidenceAreaKey(companyId, recordId, item.area);
    const generation = previewGeneration.current;
    setPreviewManifests(current => ({...current, [key]: {loading: true, error: null, manifest: null}}));
    try {
      const manifest = await api.listDomainEvidenceArtifactPreviewEntries({
        companyId, recordId, area: item.area, inputId: item.artifact.id,
        inputRevision: item.artifact.revision, sourceDigest: item.artifact.sha256,
      });
      if (generation !== previewGeneration.current) return;
      setPreviewManifests(current => ({...current, [key]: {loading: false, error: null, manifest}}));
    } catch (error) {
      if (generation !== previewGeneration.current) return;
      setPreviewManifests(current => ({...current, [key]: {loading: false, error: error instanceof Error ? error.message : '证据文件列表读取失败', manifest: null}}));
    }
  }

  async function loadEvidencePreview(recordId: string, item: DomainEvidenceRecordView['submission']['evidence'][number], previewEntry: DomainEvidenceArtifactPreviewEntryView): Promise<void> {
    const key = domainEvidenceAreaKey(companyId, recordId, item.area);
    const generation = previewGeneration.current;
    const previous = previews[key];
    if (previous?.objectUrl !== null && previous?.objectUrl !== undefined) {
      URL.revokeObjectURL(previous.objectUrl);
      previewUrls.current.delete(previous.objectUrl);
    }
    setPreviews(current => ({...current, [key]: {loading: true, error: null, preview: null, relativePath: previewEntry.relativePath, objectUrl: null}}));
    try {
      const preview = await api.getDomainEvidenceArtifactPreview({
        companyId,
        recordId,
        area: item.area,
        inputId: item.artifact.id,
        inputRevision: item.artifact.revision,
        sourceDigest: item.artifact.sha256,
        relativePath: previewEntry.relativePath,
        expectedContentSHA256: previewEntry.contentSHA256,
      });
      if (generation !== previewGeneration.current) return;
      const objectUrl = preview.textContent === null ? URL.createObjectURL(preview.content) : null;
      if (objectUrl !== null) previewUrls.current.add(objectUrl);
      setPreviews(current => ({...current, [key]: {loading: false, error: null, preview, relativePath: previewEntry.relativePath, objectUrl}}));
    } catch (error) {
      if (generation !== previewGeneration.current) return;
      setPreviews(current => ({...current, [key]: {loading: false, error: error instanceof Error ? error.message : '证据预览失败', preview: null, relativePath: previewEntry.relativePath, objectUrl: null}}));
    }
  }

  return <div className={styles.viewStack} data-od-id="domain-evidence-panel">
    <ResearchSimulationPanel key={`${companyId}:${overviewQuery.data?.mission.missionId ?? ''}`} api={api} companyId={companyId} missionId={overviewQuery.data?.mission.missionId ?? ''} runs={query.data.researchSimulationRuns} />
    <ContentOperationsPanel key={`${companyId}:${overviewQuery.data?.mission.missionId ?? ''}`} api={api} companyId={companyId} missionId={overviewQuery.data?.mission.missionId ?? ''} employees={employees} sourceEvents={query.data.contentSourceEvents} drafts={query.data.contentDrafts} reviews={query.data.contentReviews} publications={query.data.contentPublications} corrections={query.data.contentCorrections} feedback={query.data.contentFeedback} />
    <ResearchSourcePanel key={`${companyId}:${overviewQuery.data?.mission.missionId ?? ''}:research-source`} api={api} companyId={companyId} missionId={overviewQuery.data?.mission.missionId ?? ''} />
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div><span className={styles.cardEyebrow}>R3 / 独立领域证据</span><h2 className={styles.sectionTitle}>领域验收准备</h2></div>
        <StatusBadge label={`证据资格 ${labelDisplayValue(profile.qualificationStatus)}`} tone={profile.qualificationStatus === 'qualified' ? 'success' : profile.qualificationStatus === 'revoked' ? 'danger' : 'warning'} />
      </div>
      <p className={styles.formHint}>案例审核只评估单次提交。全局领域资格需要管理者审查聚合报告和逐领域已接受案例；即使记录资格，当前也没有可开放的内容/研究运行时，执行仍关闭。</p>
      <label className={styles.formLabel}>参考工作流
        <select className={styles.formField} onChange={event => setProfileId(event.target.value)} value={profile.id}>
          {query.data.profiles.map(item => <option key={item.id} value={item.id}>{item.domain === 'content_operations' ? '内容运营' : '研究模拟'} · {item.revision}</option>)}
        </select>
      </label>
      <div className={styles.detailRows}>
        <div><span>Qualification</span><strong>{labelDisplayValue(profile.qualificationStatus)}</strong></div>
        <div><span>Execution</span><strong>关闭</strong></div>
        <div><span>必需证据项</span><strong>{profile.requiredEvidence.length}</strong></div>
      </div>
      <div className={styles.checkList}>
        {profile.requiredEvidence.map(area => <div key={area}><span aria-hidden="true">•</span><span>{AREA_LABELS[area]}</span></div>)}
      </div>
      <div className={styles.formStack}>
        <div className={styles.detailRows}>
          <div><span>运行时执行</span><strong>关闭</strong></div>
          <div><span>资格决定事件</span><strong><code>{currentQualification?.eventId ?? '尚无管理者决定'}</code></strong></div>
          {currentQualification?.decision === 'qualified' ? <div><span>聚合报告 SHA-256</span><strong><code>{currentQualification.evidenceSha256}</code></strong></div> : null}
        </div>
        {profile.qualificationStatus === 'qualified' ? <>
          <label className={styles.formLabel}>撤销理由<textarea className={styles.formField} maxLength={2000} onChange={event => setQualificationRationale(event.target.value)} rows={3} value={qualificationRationale} /></label>
          <button className={styles.textButton} disabled={api.mode !== 'real' || recordQualification.isPending || qualificationRationale.trim() === ''} onClick={() => { void submitProfileQualification('revoked'); }} type="button">{recordQualification.isPending ? '正在撤销…' : '撤销此公司领域资格'}</button>
        </> : <>
          <p className={styles.formHint}>先生成聚合报告，再把下载的 JSON 上传为 MissionInput。报告只汇总案例 ID、摘要和审核回执引用，不包含证据正文。</p>
          {qualificationReport.success ? <button className={styles.textButton} onClick={downloadProfileQualificationReport} type="button">生成并下载聚合资格报告</button> : <p className={styles.formHint}>尚缺已接受的逐项案例评审：{qualificationReport.missingAreas.map(area => AREA_LABELS[area]).join('、')}</p>}
          <p className={styles.formHint}>报告格式 {`r3-domain-profile-qualification@1`}；后端会重读 CAS，并逐项核对每个案例摘要和独立评审回执。</p>
          <label className={styles.formLabel}>聚合资格报告 MissionInput ID<input className={styles.formField} onChange={event => setQualificationInputId(event.target.value)} value={qualificationInputId} /></label>
          <label className={styles.formLabel}>报告修订号<input className={styles.formField} inputMode="numeric" onChange={event => setQualificationInputRevision(event.target.value)} value={qualificationInputRevision} /></label>
          <label className={styles.formLabel}>管理者决定理由<textarea className={styles.formField} maxLength={2000} onChange={event => setQualificationRationale(event.target.value)} rows={3} value={qualificationRationale} /></label>
          <button className={styles.textButton} disabled={api.mode !== 'real' || recordQualification.isPending || qualificationRationale.trim() === '' || qualificationInputId.trim() === '' || !/^[1-9]\d*$/.test(qualificationInputRevision)} onClick={() => { void submitProfileQualification('qualified'); }} type="button">{recordQualification.isPending ? '正在记录…' : '审查并记录领域资格'}</button>
        </>}
        {qualificationMessage ? <p className={styles.formHint} role="status">{qualificationMessage}</p> : null}
      </div>
    </section>

    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>证据登记 / 不会自动解锁</span><h2 className={styles.sectionTitle}>提交证据引用</h2></div></div>
      <p className={styles.formHint}>先在使命输入中上传证据文件，再引用其 MissionInput ID、修订号和 SHA-256。后端会核对该引用确实属于当前公司；同时填写评估方法摘要和评估员工。留空的证据项会登记为不完整。</p>
      <div className={styles.recordList}>
        {profile.requiredEvidence.map(area => {
          const draft = drafts[area] ?? EMPTY_DRAFT;
          return <article className={styles.recordRow} key={area}>
            <div className={styles.recordLead}><div><strong>{AREA_LABELS[area]}</strong><span>该项必须引用一个不可变工件和评估方法。</span></div></div>
            <div className={styles.formStack}>
              <label className={styles.formLabel}>MissionInput ID<input className={styles.formField} onChange={event => updateDraft(area, 'artifactId', event.target.value)} value={draft.artifactId} /></label>
              <div className={styles.detailRows}>
                <label className={styles.formLabel}>修订号<input className={styles.formField} inputMode="numeric" onChange={event => updateDraft(area, 'revision', event.target.value)} value={draft.revision} /></label>
                <label className={styles.formLabel}>工件 SHA-256<input className={styles.formField} onChange={event => updateDraft(area, 'sha256', event.target.value)} value={draft.sha256} /></label>
              </div>
              <div className={styles.detailRows}>
                <label className={styles.formLabel}>评估方法 SHA-256<input className={styles.formField} onChange={event => updateDraft(area, 'methodSHA256', event.target.value)} value={draft.methodSHA256} /></label>
                <label className={styles.formLabel}>评估员工身份
                  <select className={styles.formField} onChange={event => updateDraft(area, 'assessedByEmployeeId', event.target.value)} value={draft.assessedByEmployeeId}>
                    <option value="">选择评估人</option>
                    {employees.map(employee => <option key={employee.employeeId} value={employee.employeeId}>{employee.displayName} · {employee.employeeId}</option>)}
                  </select>
                </label>
              </div>
            </div>
          </article>;
        })}
      </div>
      <div className={styles.commandGroup}>
        <button className={styles.commandButton} disabled={api.mode !== 'real' || recordEvidence.isPending} onClick={() => { void submitEvidence(); }} type="button">
          {recordEvidence.isPending ? '提交中…' : '记录证据提交'}
        </button>
        {api.mode !== 'real' ? <span className={styles.formHint}>Fixture 数据模式不支持写入领域证据。</span> : null}
      </div>
      {message ? <div className={styles.operationNotice} role="status">{message}</div> : null}
    </section>

    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>不可变历史</span><h2 className={styles.sectionTitle}>近期提交</h2></div><StatusBadge label={`${profileSubmissions.length} 条记录`} tone="info" /></div>
      {profileSubmissions.length === 0 ? <div className={styles.emptyState}>该参考工作流尚无证据提交。</div> : <div className={styles.recordList}>
        {profileSubmissions.map(record => <div className={styles.recordRow} key={record.recordId}>
          <div className={styles.recordLead}><div><strong>{labelDisplayValue(record.readinessStatus)} · {record.recordId}</strong><span>{record.createdAt} · {record.evidenceDigest}</span><span>{record.submission.evidence.map(item => `${AREA_LABELS[item.area]}: ${item.artifact.id}@${item.artifact.revision}`).join(' · ') || '没有提交证据项'}</span></div></div>
          <div className={styles.recordMeta}><StatusBadge label="资格未运行" tone="warning" /><span>执行关闭</span></div>
          {record.readinessStatus === 'ready_for_review' ? <div className={styles.formStack}>
            <div className={styles.formHint}>预览只展示被引用的 MissionInput 内容。PDF 原件和解析元数据不在应用内执行；如果有文本提取结果，可单独预览。目录、ZIP、Git 归档会列出已验证文件，只允许预览安全的文本和图片内容。引用审核结果仍不代表领域质量或资格。</div>
            <div className={styles.detailRows}>
              {record.submission.evidence.map(item => {
                const key = domainEvidenceAreaKey(companyId, record.recordId, item.area);
                const previewState = previews[key] ?? EMPTY_PREVIEW_STATE;
                const manifestState = previewManifests[key] ?? EMPTY_PREVIEW_MANIFEST_STATE;
                return <div key={item.area} className={styles.evidencePreviewRow}>
                  <div><strong>{AREA_LABELS[item.area]}：{item.artifact.id}@{item.artifact.revision}</strong><span>MissionInput SHA-256 {item.artifact.sha256} · 方法 {item.methodSHA256} · 评估人 {item.assessedByEmployeeId}</span></div>
                  <button className={styles.textButton} disabled={api.mode !== 'real' || manifestState.loading} onClick={() => { void loadEvidencePreviewManifest(record.recordId, item); }} type="button">
                    {manifestState.loading ? '读取清单…' : manifestState.manifest === null ? '读取证据文件清单' : '重新读取文件清单'}
                  </button>
                  {manifestState.error !== null ? <div className={styles.errorState} role="alert">{manifestState.error}</div> : null}
                  {manifestState.manifest?.entries.map(entry => {
                    const isSelected = previewState.relativePath === entry.relativePath;
                    return <div className={styles.evidencePreviewEntry} key={entry.relativePath}>
                      <div><strong>{entry.relativePath}</strong><span>{entry.mediaType} · {entry.byteSize} bytes · SHA-256 {entry.contentSHA256}</span></div>
                      {entry.previewable ? <button className={styles.textButton} disabled={api.mode !== 'real' || previewState.loading} onClick={() => { void loadEvidencePreview(record.recordId, item, entry); }} type="button">
                        {previewState.loading && isSelected ? '载入中…' : isSelected ? '重新预览' : '预览此文件'}
                      </button> : <><StatusBadge label="不可预览" tone="neutral" /><span className={styles.formHint}>{evidencePreviewUnavailableReason(entry)}</span></>}
                      {previewState.error !== null && isSelected ? <div className={styles.errorState} role="alert">{previewState.error}</div> : null}
                      {isSelected && previewState.preview?.textContent !== null && previewState.preview !== null ? <pre className={styles.evidencePreviewText}>{previewState.preview.textContent}</pre> : null}
                      {isSelected && previewState.preview?.mediaType.startsWith('image/') === true && previewState.objectUrl !== null ? <img className={styles.evidencePreviewImage} alt={`${AREA_LABELS[item.area]}证据预览 ${entry.fileName}`} src={previewState.objectUrl} /> : null}
                      {isSelected && previewState.preview !== null ? <span className={styles.formHint}>{previewState.preview.fileName} · 内容 SHA-256 {previewState.preview.contentDigest} · MissionInput SHA-256 {previewState.preview.sourceDigest}</span> : null}
                    </div>;
                  })}
                </div>;
              })}
            </div>
            {record.review === null ? <>
              <div className={styles.detailRows}>
                <label className={styles.formLabel}>引用审核结果
                  <select className={styles.formField} onChange={event => updateReviewDraft(record.recordId, 'outcome', event.target.value)} value={(reviewDrafts[record.recordId] ?? EMPTY_REVIEW_DRAFT).outcome}>
                    <option value="">选择结果</option>
                    <option value="evidence_references_accepted">引用字段已核对</option>
                    <option value="evidence_references_rejected">引用字段被拒绝</option>
                    <option value="more_evidence_required">需要补充证据</option>
                  </select>
                </label>
                <label className={styles.formLabel}>独立审核人
                  <select className={styles.formField} onChange={event => updateReviewDraft(record.recordId, 'reviewerEmployeeId', event.target.value)} value={(reviewDrafts[record.recordId] ?? EMPTY_REVIEW_DRAFT).reviewerEmployeeId}>
                    <option value="">选择 review 岗员工</option>
                    {reviewers.filter(employee => !record.submission.evidence.some(item => item.assessedByEmployeeId === employee.employeeId)).map(employee => <option key={employee.employeeId} value={employee.employeeId}>{employee.displayName} · {employee.employeeId}</option>)}
                  </select>
                </label>
              </div>
              <label className={styles.formLabel}>审核理由<textarea className={styles.formField} maxLength={2000} onChange={event => updateReviewDraft(record.recordId, 'rationale', event.target.value)} value={(reviewDrafts[record.recordId] ?? EMPTY_REVIEW_DRAFT).rationale} /></label>
              <button className={styles.commandButton} disabled={api.mode !== 'real' || recordReview.isPending || !record.submission.evidence.every(item => previews[domainEvidenceAreaKey(companyId, record.recordId, item.area)]?.preview !== null && previews[domainEvidenceAreaKey(companyId, record.recordId, item.area)]?.preview !== undefined)} onClick={() => { void submitReview(record); }} type="button">
                {recordReview.isPending ? '提交审核…' : '记录独立引用审核'}
              </button>
              {!record.submission.evidence.every(item => previews[domainEvidenceAreaKey(companyId, record.recordId, item.area)]?.preview !== null && previews[domainEvidenceAreaKey(companyId, record.recordId, item.area)]?.preview !== undefined) ? <span className={styles.formHint}>每项证据至少成功预览一个可预览文件后，才能记录审核。</span> : null}
              {reviewers.length === 0 ? <span className={styles.formHint}>公司没有 review 岗员工，不能提交审核。</span> : null}
            </> : <div className={styles.detailRows}>
              <StatusBadge label={REVIEW_OUTCOME_LABELS[record.review.outcome]} tone={record.review.outcome === 'evidence_references_accepted' ? 'success' : 'warning'} />
              <span>{record.review.reviewerEmployeeId} · {record.review.createdAt} · {record.review.rationale}</span>
              <span>{record.review.reviewContractRevision === 1 ? `预览过的内容：${record.review.previewedEvidence.map(item => `${AREA_LABELS[item.area]} / ${item.relativePath} · ${item.contentDigest}`).join(' · ')}` : '历史引用决议没有绑定预览内容。'}</span>
            </div>}
          </div> : null}
          {record.review?.outcome === 'evidence_references_accepted' ? <DomainEvidenceSubstantiveAssessmentPanel api={api} companyId={companyId} record={record} reviewers={reviewers} previews={previews} /> : null}
        </div>)}
      </div>}
    </section>
  </div>;
}

type DomainEvidenceSubstantiveAssessmentPanelProps = Readonly<{
  api: WorkbenchApi;
  companyId: string;
  record: DomainEvidenceRecordView;
  reviewers: ReadonlyArray<EmployeeSummary>;
  previews: Readonly<Record<string, Readonly<{preview: DomainEvidenceArtifactPreviewView | null}>>>;
}>;

type DomainEvidenceSubstantiveDraft = Readonly<{
  outcome: DomainEvidenceAreaAssessmentOutcomeView | '';
  rationale: string;
}>;

const EMPTY_SUBSTANTIVE_DRAFT: DomainEvidenceSubstantiveDraft = {outcome: '', rationale: ''};

const SUBSTANTIVE_OUTCOME_LABELS: Readonly<Record<DomainEvidenceAreaAssessmentOutcomeView, string>> = {
  accepted: '证据支持该领域标准',
  rejected: '证据与该领域标准冲突',
  insufficient: '证据不足，需补充材料',
};

function DomainEvidenceSubstantiveAssessmentPanel({api, companyId, record, reviewers, previews}: DomainEvidenceSubstantiveAssessmentPanelProps): ReactElement {
  const recordAssessment = useRecordDomainEvidenceSubstantiveAssessment(api, companyId);
  const eligibleReviewers = reviewers.filter(employee => !record.submission.evidence.some(item => item.assessedByEmployeeId === employee.employeeId));
  const [reviewerEmployeeId, setReviewerEmployeeId] = useState('');
  const [drafts, setDrafts] = useState<Readonly<Partial<Record<DomainEvidenceAreaView, DomainEvidenceSubstantiveDraft>>>>({});
  const [message, setMessage] = useState<string | null>(null);
  const pendingRequestIds = useRef(new Map<string, string>());
  const previewedEvidence = record.submission.evidence.map(item => previews[domainEvidenceAreaKey(companyId, record.recordId, item.area)]?.preview)
    .filter((preview): preview is DomainEvidenceArtifactPreviewView => preview !== null && preview !== undefined)
    .map(preview => ({area: preview.area, relativePath: preview.relativePath, sourceDigest: preview.sourceDigest, contentDigest: preview.contentDigest, mediaType: preview.mediaType}));
  const hasEveryPreview = previewedEvidence.length === record.submission.evidence.length;

  function update(area: DomainEvidenceAreaView, field: keyof DomainEvidenceSubstantiveDraft, value: string): void {
    setDrafts(current => ({...current, [area]: {...(current[area] ?? EMPTY_SUBSTANTIVE_DRAFT), [field]: value}}));
  }

  async function submit(): Promise<void> {
    setMessage(null);
    if (record.assessment !== null) return;
    if (reviewerEmployeeId === '' || !eligibleReviewers.some(employee => employee.employeeId === reviewerEmployeeId)) {
      setMessage('请选择未参与评估的 review 员工。');
      return;
    }
    if (!hasEveryPreview) {
      setMessage('请在本次审阅中预览每个领域至少一个证据文件。');
      return;
    }
    const areaAssessments = record.submission.evidence.map(item => {
      const draft = drafts[item.area] ?? EMPTY_SUBSTANTIVE_DRAFT;
      return {area: item.area, outcome: draft.outcome, rationale: draft.rationale.trim()};
    });
    if (areaAssessments.some(item => item.outcome === '' || item.rationale === '' || Array.from(item.rationale).length > 1000)) {
      setMessage('请为每个必需领域选择结果并填写不超过 1000 字的理由。');
      return;
    }
    try {
      const payload = {
        recordId: record.recordId,
        evidenceDigest: record.evidenceDigest,
        reviewerEmployeeId,
        areaAssessments: areaAssessments as ReadonlyArray<{area: DomainEvidenceAreaView; outcome: DomainEvidenceAreaAssessmentOutcomeView; rationale: string}>,
        previewedEvidence,
      };
      const pending = pendingRequestIdentity(pendingRequestIds.current, 'domain-substantive-assessment', payload);
      const assessment = await recordAssessment.mutateAsync({...payload, requestId: pending.requestId});
      clearPendingRequestIdentity(pendingRequestIds.current, pending.key);
      setMessage(`逐领域判定已记录：${substantiveOutcomeLabel(assessment.outcome)}。该结论只适用于本次证据提交，未改变参考 profile 资格或执行状态。`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '领域证据实质审阅失败。');
    }
  }

  if (record.assessment !== null) {
    return <section className={styles.formStack}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>逐领域实质判定</span><h3 className={styles.sectionTitle}>证据包审阅结果</h3></div>
        <StatusBadge label={substantiveOutcomeLabel(record.assessment.outcome)} tone={record.assessment.outcome === 'evidence_accepted' ? 'success' : record.assessment.outcome === 'evidence_rejected' ? 'danger' : 'warning'} />
      </div>
      <p className={styles.formHint}>本结果只判定这份绑定摘要的证据包，不授予参考 profile 全局资格，也不开放执行。</p>
      <div className={styles.recordList}>{record.assessment.areaAssessments.map(item => <div className={styles.recordRow} key={item.area}>
        <div className={styles.recordLead}><div><strong>{AREA_LABELS[item.area]} · {SUBSTANTIVE_OUTCOME_LABELS[item.outcome]}</strong><span>{item.rationale}</span></div></div>
      </div>)}</div>
      <div className={styles.formHint}>reviewer {record.assessment.reviewerEmployeeId} · {record.assessment.createdAt} · evidence {record.assessment.evidenceDigest}</div>
    </section>;
  }

  return <section className={styles.formStack}>
    <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>逐领域实质判定</span><h3 className={styles.sectionTitle}>审阅质量、恢复、成本与组织收益证据</h3></div></div>
    <p className={styles.formHint}>参考引用审查已通过。请按每个领域的实际证据分别判定。这里记录人工结论，不自动换算指标阈值、profile 资格或执行权限。</p>
    <label className={styles.formLabel}>独立领域 reviewer
      <select className={styles.formField} value={reviewerEmployeeId} onChange={event => setReviewerEmployeeId(event.target.value)}>
        <option value="">选择未参与评估的 review 员工</option>
        {eligibleReviewers.map(employee => <option key={employee.employeeId} value={employee.employeeId}>{employee.displayName} · {employee.employeeId}</option>)}
      </select>
    </label>
    {record.submission.evidence.map(item => {
      const draft = drafts[item.area] ?? EMPTY_SUBSTANTIVE_DRAFT;
      return <article className={styles.recordRow} key={item.area}>
        <div className={styles.recordLead}><div><strong>{AREA_LABELS[item.area]}</strong><span>输入 {item.artifact.id}@{item.artifact.revision} · 方法 {item.methodSHA256}</span></div></div>
        <div className={styles.formStack}>
          <label className={styles.formLabel}>领域判断
            <select className={styles.formField} value={draft.outcome} onChange={event => update(item.area, 'outcome', event.target.value)}>
              <option value="">选择判断</option>
              <option value="accepted">证据支持该领域标准</option>
              <option value="rejected">证据与该领域标准冲突</option>
              <option value="insufficient">证据不足，需补充材料</option>
            </select>
          </label>
          <label className={styles.formLabel}>判定理由<textarea className={styles.formField} rows={2} maxLength={1000} value={draft.rationale} onChange={event => update(item.area, 'rationale', event.target.value)} /></label>
        </div>
      </article>;
    })}
    {!hasEveryPreview ? <p className={styles.formHint}>需在本次领域审阅中成功预览每个领域至少一个安全文件。</p> : null}
    {eligibleReviewers.length === 0 ? <p className={styles.formHint}>没有未参与评估的 review 员工，不能提交独立领域判定。</p> : null}
    <button className={styles.commandButton} disabled={api.mode !== 'real' || recordAssessment.isPending || !hasEveryPreview || eligibleReviewers.length === 0} onClick={() => { void submit(); }} type="button">
      {recordAssessment.isPending ? '记录判定中…' : '记录逐领域判定'}
    </button>
    {message !== null ? <div className={styles.operationNotice} role="status">{message}</div> : null}
  </section>;
}

function substantiveOutcomeLabel(outcome: 'evidence_accepted' | 'evidence_rejected' | 'more_evidence_required'): string {
  switch (outcome) {
    case 'evidence_accepted': return '证据包接受';
    case 'evidence_rejected': return '证据包拒绝';
    case 'more_evidence_required': return '需要补充证据';
  }
}
