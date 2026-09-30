// pattern: Imperative Shell

import {useState, type ReactElement} from 'react';
import type {MissionInputView} from '../domain/mission-input';
import type {DomainContentClaimFindingView, DomainContentClaimReviewView, DomainContentDraftView, DomainContentFeedbackCategoryView, DomainContentFeedbackView, DomainContentPublicationView, DomainContentReviewView, DomainContentSourceEventView, EmployeeSummary} from '../domain/workbench';
import type {WorkbenchApi} from '../data/workbench-api';
import {useMissionInputs, useRecordContentCorrection, useRecordContentFeedback, useRecordContentReview, useRegisterContentDraft, useSetContentSourceAuthorization, useSimulateContentPublication} from '../data/workbench-query';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import styles from '../styles/workbench.module.css';

type ContentOperationsPanelProps = Readonly<{
  api: WorkbenchApi;
  companyId: string;
  missionId: string;
  employees: ReadonlyArray<EmployeeSummary>;
  sourceEvents: ReadonlyArray<DomainContentSourceEventView>;
  drafts: ReadonlyArray<DomainContentDraftView>;
  reviews: ReadonlyArray<DomainContentReviewView>;
  publications: ReadonlyArray<DomainContentPublicationView>;
  corrections: ReadonlyArray<Readonly<{correctionId: string; publicationId: string; state: 'review_required'; correctionDraftInputId: string; correctionDraftRevision: string; rationale: string; createdAt: string}>>;
  feedback: ReadonlyArray<DomainContentFeedbackView>;
}>;

type ClaimDraft = Readonly<{
  finding: DomainContentClaimFindingView | '';
  sourceKey: string;
  limitation: string;
}>;

const EMPTY_CLAIM_DRAFT: ClaimDraft = {finding: '', sourceKey: '', limitation: ''};
const MAX_CONTENT_INPUT_BYTES = 1_048_576;

function inputReferenceKey(inputId: string, revision: string): string {
  return JSON.stringify([inputId, revision]);
}

function inputOptionLabel(input: MissionInputView): string {
  return `${input.inputId}@${input.revision} · ${input.displayName} · ${input.mediaType} · SHA-256 ${input.contentDigest.slice(0, 12)}`;
}

function isContentInput(input: MissionInputView): boolean {
  const byteSize = Number(input.byteSize);
  return input.state === 'usable' && (input.mediaType === 'application/json' || input.mediaType.startsWith('text/'))
    && Number.isSafeInteger(byteSize) && byteSize > 0 && byteSize <= MAX_CONTENT_INPUT_BYTES;
}

export function ContentOperationsPanel({api, companyId, missionId, employees, sourceEvents, drafts, reviews, publications, corrections, feedback}: ContentOperationsPanelProps): ReactElement {
  const inputsQuery = useMissionInputs(api, companyId, missionId);
  const authorizeSource = useSetContentSourceAuthorization(api, companyId);
  const registerDraft = useRegisterContentDraft(api, companyId);
  const recordReview = useRecordContentReview(api, companyId);
  const simulatePublication = useSimulateContentPublication(api, companyId);
  const recordCorrection = useRecordContentCorrection(api, companyId);
  const recordFeedback = useRecordContentFeedback(api, companyId);
  const [sourceInputKey, setSourceInputKey] = useState('');
  const [sourceState, setSourceState] = useState<'authorized' | 'revoked'>('authorized');
  const [sourceRationale, setSourceRationale] = useState('');
  const [sourceMessage, setSourceMessage] = useState<string | null>(null);
  const [draftInputKey, setDraftInputKey] = useState('');
  const [writerEmployeeId, setWriterEmployeeId] = useState('');
  const [criticalClaimsText, setCriticalClaimsText] = useState('');
  const [constraintsPassed, setConstraintsPassed] = useState(false);
  const [draftMessage, setDraftMessage] = useState<string | null>(null);
  const [selectedDraftKey, setSelectedDraftKey] = useState('');
  const [checkerEmployeeId, setCheckerEmployeeId] = useState('');
  const [samplePlanKey, setSamplePlanKey] = useState('');
  const [sampledClaimIDsText, setSampledClaimIDsText] = useState('');
  const [claimDrafts, setClaimDrafts] = useState<Readonly<Record<string, ClaimDraft>>>({});
  const [reviewMessage, setReviewMessage] = useState<string | null>(null);
  const [selectedCorrectionPublicationID, setSelectedCorrectionPublicationID] = useState('');
  const [correctionDraftID, setCorrectionDraftID] = useState('');
  const [correctionRationale, setCorrectionRationale] = useState('');
  const [correctionMessage, setCorrectionMessage] = useState<string | null>(null);
  const [selectedFeedbackPublicationID, setSelectedFeedbackPublicationID] = useState('');
  const [feedbackCategory, setFeedbackCategory] = useState<DomainContentFeedbackCategoryView>('correction_requested');
  const [feedbackNote, setFeedbackNote] = useState('');
  const [feedbackMessage, setFeedbackMessage] = useState<string | null>(null);

  const inputs = inputsQuery.data ?? [];
  const usableInputs = inputs.filter(isContentInput);
  const latestSources = new Map<string, DomainContentSourceEventView>();
  for (const event of sourceEvents) {
    const key = inputReferenceKey(event.inputId, event.revision);
    if (!latestSources.has(key)) latestSources.set(key, event);
  }
  const authorizedSources = Array.from(latestSources.values()).filter(event => event.state === 'authorized');
  const authorizedByKey = new Map(authorizedSources.map(event => [inputReferenceKey(event.inputId, event.revision), event]));
  const missionInputKeys = new Set(usableInputs.map(input => inputReferenceKey(input.inputId, input.revision)));
  const historicalSources = Array.from(latestSources.values()).filter(event => !missionInputKeys.has(inputReferenceKey(event.inputId, event.revision)));
  const selectedSourceInput = usableInputs.find(input => inputReferenceKey(input.inputId, input.revision) === sourceInputKey) ?? null;
  const selectedDraftInput = usableInputs.find(input => inputReferenceKey(input.inputId, input.revision) === draftInputKey) ?? null;
  const canSubmitSourceAuthorization = sourceState === 'authorized'
    ? selectedSourceInput !== null || latestSources.has(sourceInputKey)
    : authorizedByKey.has(sourceInputKey);
  const selectedDraft = drafts.find(draft => inputReferenceKey(draft.draftInputId, draft.draftRevision) === selectedDraftKey) ?? null;
  const pendingCorrection = selectedDraft === null ? null : corrections.find(correction => correction.correctionDraftInputId === selectedDraft.draftInputId
    && correction.correctionDraftRevision === selectedDraft.draftRevision
    && !reviews.some(review => review.correctionId === correction.correctionId && review.outcome === 'accepted')) ?? null;
  const reviewAlreadyRecorded = selectedDraft !== null && (pendingCorrection !== null
    ? reviews.some(review => review.correctionId === pendingCorrection.correctionId)
    : reviews.some(review => review.draftInputId === selectedDraft.draftInputId && review.draftRevision === selectedDraft.draftRevision));
  const reviewers = employees.filter(employee => employee.role === 'review');
  const selectedCorrectionPublication = publications.find(item => item.publicationId === selectedCorrectionPublicationID) ?? null;
  const correctionDraftOptions = selectedCorrectionPublication === null ? [] : drafts.filter(draft =>
    draft.draftInputId === selectedCorrectionPublication.draftInputId
    && BigInt(draft.draftRevision) > BigInt(selectedCorrectionPublication.draftRevision)
    && !drafts.some(candidate => candidate.draftInputId === draft.draftInputId && BigInt(candidate.draftRevision) > BigInt(draft.draftRevision)));

  async function submitSourceAuthorization(): Promise<void> {
    setSourceMessage(null);
    const selectedPriorSource = authorizedByKey.get(sourceInputKey);
    const selectedHistoricalSource = latestSources.get(sourceInputKey);
    const sourceForAuthorization = selectedSourceInput ?? selectedHistoricalSource;
    const sourceInputID = sourceState === 'authorized' ? sourceForAuthorization?.inputId : selectedPriorSource?.inputId;
    const sourceRevision = sourceState === 'authorized' ? sourceForAuthorization?.revision : selectedPriorSource?.revision;
    const sourceSHA256 = sourceState === 'authorized' ? selectedSourceInput?.contentDigest ?? selectedHistoricalSource?.sha256 : selectedPriorSource?.sha256;
    if (sourceInputID === undefined || sourceRevision === undefined || sourceSHA256 === undefined || sourceRationale.trim() === '') {
      setSourceMessage('请选择输入，并填写 1 到 1000 个字符的授权理由。');
      return;
    }
    try {
      const event = await authorizeSource.mutateAsync({
        sourceInputId: sourceInputID, sourceInputRevision: sourceRevision,
        sourceSha256: sourceSHA256, state: sourceState, rationale: sourceRationale.trim(),
        requestId: `content-source-${crypto.randomUUID()}`,
      });
      setSourceRationale('');
      setSourceMessage(event.state === 'authorized' ? `已授权来源 ${event.inputId}@${event.revision}。` : `已撤销来源 ${event.inputId}@${event.revision}。`);
    } catch (error) {
      setSourceMessage(error instanceof Error ? error.message : '内容来源授权操作失败。');
    }
  }

  async function submitDraft(): Promise<void> {
    setDraftMessage(null);
    const criticalClaims = criticalClaimsText.split(/[\r\n,，]+/).map(item => item.trim()).filter(item => item !== '');
    if (selectedDraftInput === null || writerEmployeeId === '' || criticalClaims.length === 0 || new Set(criticalClaims).size !== criticalClaims.length) {
      setDraftMessage('请选择草稿输入和作者，并填写不重复的关键主张 ID。');
      return;
    }
    try {
      const draft = await registerDraft.mutateAsync({
        draftInputId: selectedDraftInput.inputId, draftInputRevision: selectedDraftInput.revision,
        writerEmployeeId, criticalClaims, constraintsPassed,
        requestId: `content-draft-${crypto.randomUUID()}`,
      });
      setDraftMessage(`草稿版本 ${draft.draftInputId}@${draft.draftRevision} 已固定。后续编辑须上传为新的 MissionInput 版本。`);
    } catch (error) {
      setDraftMessage(error instanceof Error ? error.message : '内容草稿登记失败。');
    }
  }

  function chooseDraft(key: string): void {
    setSelectedDraftKey(key);
    const draft = drafts.find(item => inputReferenceKey(item.draftInputId, item.draftRevision) === key);
    if (draft === undefined) {
      setClaimDrafts({});
      setSampledClaimIDsText('');
      return;
    }
    setClaimDrafts(Object.fromEntries(draft.criticalClaims.map(claimId => [claimId, EMPTY_CLAIM_DRAFT])));
    setSampledClaimIDsText(draft.criticalClaims.join(', '));
  }

  function updateClaimDraft(claimId: string, field: keyof ClaimDraft, value: string): void {
    setClaimDrafts(current => ({...current, [claimId]: {...(current[claimId] ?? EMPTY_CLAIM_DRAFT), [field]: value}}));
  }

  async function submitReview(): Promise<void> {
    setReviewMessage(null);
    if (selectedDraft === null || reviewAlreadyRecorded || checkerEmployeeId === '' || !reviewers.some(employee => employee.employeeId === checkerEmployeeId)) {
      setReviewMessage('请选择尚未核查的草稿和 review 岗员工。');
      return;
    }
    const claims = selectedDraft.criticalClaims.map(claimId => {
      const draft = claimDrafts[claimId] ?? EMPTY_CLAIM_DRAFT;
      const source = authorizedByKey.get(draft.sourceKey);
      return {
        claimId,
        finding: draft.finding,
        sources: draft.finding === 'verified' && source !== undefined ? [{inputId: source.inputId, revision: source.revision, sha256: source.sha256}] : [],
        limitation: draft.limitation.trim(),
      };
    });
    if (claims.some(claim => claim.finding === '' || (claim.finding === 'verified' && claim.sources.length === 0) || (claim.finding === 'inconclusive' && claim.limitation === ''))) {
      setReviewMessage('每条关键主张都需要核查结论；已核实主张须选授权来源，无法确定的主张须说明原因。');
      return;
    }
    const reviewedClaims = claims as ReadonlyArray<DomainContentClaimReviewView>;
    const samplePlan = authorizedByKey.get(samplePlanKey);
    if (samplePlan === undefined) {
      setReviewMessage('请选择已授权的人工作样计划来源。');
      return;
    }
    const sampledClaimIds = sampledClaimIDsText.split(/[\r\n,，]+/).map(item => item.trim()).filter(item => item !== '');
    if (sampledClaimIds.length === 0 || new Set(sampledClaimIds).size !== sampledClaimIds.length || sampledClaimIds.some(claimId => !selectedDraft.criticalClaims.includes(claimId))) {
      setReviewMessage('人工抽样必须包含草稿中的一个或多个不重复关键主张 ID。');
      return;
    }
    try {
      const review = await recordReview.mutateAsync({
        draftInputId: selectedDraft.draftInputId, draftRevision: selectedDraft.draftRevision,
        correctionId: pendingCorrection?.correctionId ?? '',
        review: {draftRevision: selectedDraft.draftRevision, checkerEmployeeId, humanSampled: true, claims: reviewedClaims},
        sample: {
          draftRevision: selectedDraft.draftRevision, draftSha256: selectedDraft.draftSha256,
          plan: {inputId: samplePlan.inputId, revision: samplePlan.revision, sha256: samplePlan.sha256},
          sampledClaimIds, sampledByEmployeeId: checkerEmployeeId,
        },
        requestId: `content-review-${crypto.randomUUID()}`,
      });
      setReviewMessage(`独立核查已保存：${review.outcome}${review.stale ? '（草稿版本已过期）' : ''}。这不授予内容运营资格。`);
    } catch (error) {
      setReviewMessage(error instanceof Error ? error.message : '内容事实核查失败。');
    }
  }

  async function submitPublication(review: DomainContentReviewView): Promise<void> {
    setReviewMessage(null);
    if (review.outcome !== 'accepted' || review.stale || publications.some(item => item.reviewId === review.reviewId)) {
      setReviewMessage('只有当前版本且接受的核查可以进行一次本地模拟发布。');
      return;
    }
    try {
      const publication = await simulatePublication.mutateAsync({reviewId: review.reviewId, requestId: `content-publication-${crypto.randomUUID()}`});
      setReviewMessage(`模拟发布 ${publication.publicationId} 已记录。没有向外部平台发布内容。`);
    } catch (error) {
      setReviewMessage(error instanceof Error ? error.message : '模拟发布失败。');
    }
  }

  async function submitCorrection(): Promise<void> {
    setCorrectionMessage(null);
    const correctionDraft = correctionDraftOptions.find(item => item.draftId === correctionDraftID);
    if (selectedCorrectionPublication === null || correctionDraft === undefined || correctionRationale.trim() === '') {
      setCorrectionMessage('请选择已模拟发布的内容、新版本草稿，并填写纠错理由。');
      return;
    }
    try {
      const correction = await recordCorrection.mutateAsync({
        publicationId: selectedCorrectionPublication.publicationId,
        correctionDraftInputId: correctionDraft.draftInputId,
        correctionDraftRevision: correctionDraft.draftRevision,
        rationale: correctionRationale.trim(), requestId: `content-correction-${crypto.randomUUID()}`,
      });
      setCorrectionMessage(`纠错记录 ${correction.correctionId} 已保存；新草稿需要重新核查。`);
      setCorrectionRationale('');
    } catch (error) {
      setCorrectionMessage(error instanceof Error ? error.message : '纠错记录失败。');
    }
  }

  async function submitFeedback(): Promise<void> {
    setFeedbackMessage(null);
    if (selectedFeedbackPublicationID === '' || feedbackNote.trim() === '') {
      setFeedbackMessage('请选择模拟发布记录并填写反馈说明。');
      return;
    }
    try {
      const item = await recordFeedback.mutateAsync({
        publicationId: selectedFeedbackPublicationID, category: feedbackCategory, note: feedbackNote.trim(),
        requestId: `content-feedback-${crypto.randomUUID()}`,
      });
      setFeedbackMessage(item.state === 'review_required' ? '反馈已记录并标记为需要人工复核；没有自动创建任务。' : '反馈已作为内部观察记录。');
      setFeedbackNote('');
    } catch (error) {
      setFeedbackMessage(error instanceof Error ? error.message : '反馈记录失败。');
    }
  }

  return <section className={styles.sectionCard} aria-labelledby="content-operations-title">
    <div className={styles.sectionHeader}>
      <div><span className={styles.cardEyebrow}>R3 / 来源绑定内容流程</span><h2 className={styles.sectionTitle} id="content-operations-title">内容来源、版本草稿与独立核查</h2></div>
      <StatusBadge label="领域资格未运行" tone="warning" />
    </div>
    <p className={styles.formHint}>此处记录同公司 MissionInput 来源授权、固定版本草稿和人工事实核查。通过软件校验不代表内容质量合格或领域资格已授予；不会向外部平台发布。</p>
    <div className={styles.detailRows}>
      <div><span>Mission</span><strong>{missionId || '未选择'}</strong></div>
      <div><span>资格状态</span><strong>not_run</strong></div>
      <div><span>执行</span><strong>关闭</strong></div>
    </div>
    {missionId === '' ? <p className={styles.formHint}>请先创建或选择 Mission，再管理其中的文本输入。</p> : null}
    {inputsQuery.isError ? <div className={styles.errorState} role="alert">Mission 输入读取失败：{inputsQuery.error.message}</div> : null}
    <div className={styles.detailRows}>
      <div><span>已授权来源事件</span><strong>{sourceEvents.length}</strong></div>
      <div><span>固定草稿版本</span><strong>{drafts.length}</strong></div>
      <div><span>事实核查</span><strong>{reviews.length}</strong></div>
    </div>

    <div className={styles.formStack}>
      <h3 className={styles.sectionTitle}>授权或撤销同公司输入来源</h3>
      <label className={styles.formLabel}>来源 MissionInput
        <select className={styles.formField} value={sourceInputKey} onChange={event => setSourceInputKey(event.target.value)}>
          <option value="">{sourceState === 'authorized' ? '选择当前 Mission 的可用文本/JSON 输入' : '选择此前已授权的来源版本'}</option>
          {usableInputs.map(input => <option key={inputReferenceKey(input.inputId, input.revision)} value={inputReferenceKey(input.inputId, input.revision)}>{inputOptionLabel(input)}</option>)}
          {historicalSources.map(event => <option key={inputReferenceKey(event.inputId, event.revision)} value={inputReferenceKey(event.inputId, event.revision)}>{event.inputId}@{event.revision} · 历史{event.state === 'authorized' ? '已授权' : '已撤销'} · SHA-256 {event.sha256.slice(0, 12)}</option>)}
        </select>
      </label>
      <label className={styles.formLabel}>授权状态
        <select className={styles.formField} value={sourceState} onChange={event => setSourceState(event.target.value as 'authorized' | 'revoked')}>
          <option value="authorized">授权用于本地核查</option>
          <option value="revoked">撤销此版本</option>
        </select>
      </label>
      <label className={styles.formLabel}>理由<textarea className={styles.formField} maxLength={1000} value={sourceRationale} onChange={event => setSourceRationale(event.target.value)} /></label>
      <button className={styles.commandButton} type="button" disabled={api.mode !== 'real' || authorizeSource.isPending || !canSubmitSourceAuthorization} onClick={() => { void submitSourceAuthorization(); }}>
        {authorizeSource.isPending ? '保存中…' : '保存来源授权状态'}
      </button>
      {sourceMessage !== null ? <div className={styles.operationNotice} role="status">{sourceMessage}</div> : null}
      {sourceEvents.slice(0, 8).map(event => <div className={styles.recordRow} key={event.eventId}>
        <div className={styles.recordLead}><div><strong>{event.state === 'authorized' ? '已授权' : '已撤销'} · {event.inputId}@{event.revision}</strong><span>{event.createdAt} · {event.sha256} · {event.rationale}</span></div></div>
      </div>)}
    </div>

    <div className={styles.formStack}>
      <h3 className={styles.sectionTitle}>登记不可变草稿版本</h3>
      <label className={styles.formLabel}>草稿 MissionInput
        <select className={styles.formField} value={draftInputKey} onChange={event => setDraftInputKey(event.target.value)}>
          <option value="">选择当前 Mission 的可用草稿输入</option>
          {usableInputs.map(input => <option key={inputReferenceKey(input.inputId, input.revision)} value={inputReferenceKey(input.inputId, input.revision)}>{inputOptionLabel(input)}</option>)}
        </select>
      </label>
      <label className={styles.formLabel}>作者
        <select className={styles.formField} value={writerEmployeeId} onChange={event => setWriterEmployeeId(event.target.value)}>
          <option value="">选择公司员工</option>
          {employees.map(employee => <option key={employee.employeeId} value={employee.employeeId}>{employee.displayName} · {employee.employeeId}</option>)}
        </select>
      </label>
      <label className={styles.formLabel}>关键主张 ID（逗号或换行分隔）<textarea className={styles.formField} value={criticalClaimsText} onChange={event => setCriticalClaimsText(event.target.value)} /></label>
      <label className={styles.formLabel}><span><input type="checkbox" checked={constraintsPassed} onChange={event => setConstraintsPassed(event.target.checked)} /> 作者已检查并声明机械约束通过</span></label>
      <p className={styles.formHint}>机械约束声明是人工记录，不由此面板推断内容质量。修改正文后请上传为新的 MissionInput 版本，再登记新草稿。</p>
      <button className={styles.commandButton} type="button" disabled={api.mode !== 'real' || registerDraft.isPending || selectedDraftInput === null || writerEmployeeId === ''} onClick={() => { void submitDraft(); }}>
        {registerDraft.isPending ? '登记中…' : '固定草稿版本'}
      </button>
      {draftMessage !== null ? <div className={styles.operationNotice} role="status">{draftMessage}</div> : null}
      {drafts.slice(0, 10).map(draft => <div className={styles.recordRow} key={draft.draftId}>
        <div className={styles.recordLead}><div><strong>{draft.draftInputId}@{draft.draftRevision}</strong><span>作者 {draft.writerEmployeeId} · SHA-256 {draft.draftSha256} · 主张 {draft.criticalClaims.join(', ')}</span></div></div>
        <div className={styles.recordMeta}><StatusBadge label={draft.constraintsPassed ? '约束已声明通过' : '约束待完成'} tone={draft.constraintsPassed ? 'info' : 'warning'} /></div>
      </div>)}
    </div>

    <div className={styles.formStack}>
      <h3 className={styles.sectionTitle}>独立事实核查与人工抽样</h3>
      <label className={styles.formLabel}>固定草稿
        <select className={styles.formField} value={selectedDraftKey} onChange={event => chooseDraft(event.target.value)}>
          <option value="">选择已登记草稿</option>
          {drafts.map(draft => <option key={draft.draftId} value={inputReferenceKey(draft.draftInputId, draft.draftRevision)}>{draft.draftInputId}@{draft.draftRevision} · {draft.writerEmployeeId}</option>)}
        </select>
      </label>
      <label className={styles.formLabel}>独立 reviewer
        <select className={styles.formField} value={checkerEmployeeId} onChange={event => setCheckerEmployeeId(event.target.value)}>
          <option value="">选择 review 岗员工</option>
          {reviewers.filter(employee => employee.employeeId !== selectedDraft?.writerEmployeeId).map(employee => <option key={employee.employeeId} value={employee.employeeId}>{employee.displayName} · {employee.employeeId}</option>)}
        </select>
      </label>
      {selectedDraft?.criticalClaims.map(claimId => {
        const claimDraft = claimDrafts[claimId] ?? EMPTY_CLAIM_DRAFT;
        return <article className={styles.recordRow} key={claimId}>
          <div className={styles.recordLead}><div><strong>主张 {claimId}</strong></div></div>
          <div className={styles.formStack}>
            <label className={styles.formLabel}>核查结论
              <select className={styles.formField} value={claimDraft.finding} onChange={event => updateClaimDraft(claimId, 'finding', event.target.value)}>
                <option value="">选择结论</option><option value="verified">来源支持</option><option value="inconclusive">无法确定</option><option value="contradicted">来源矛盾</option>
              </select>
            </label>
            {claimDraft.finding === 'verified' ? <label className={styles.formLabel}>已授权来源
              <select className={styles.formField} value={claimDraft.sourceKey} onChange={event => updateClaimDraft(claimId, 'sourceKey', event.target.value)}>
                <option value="">选择精确来源版本</option>
                {authorizedSources.map(source => <option key={inputReferenceKey(source.inputId, source.revision)} value={inputReferenceKey(source.inputId, source.revision)}>{source.inputId}@{source.revision} · {source.sha256}</option>)}
              </select>
            </label> : null}
            {claimDraft.finding === 'inconclusive' ? <label className={styles.formLabel}>无法确定的理由<input className={styles.formField} maxLength={1000} value={claimDraft.limitation} onChange={event => updateClaimDraft(claimId, 'limitation', event.target.value)} /></label> : null}
          </div>
        </article>;
      })}
      <label className={styles.formLabel}>人工作样计划（须先授权）
        <select className={styles.formField} value={samplePlanKey} onChange={event => setSamplePlanKey(event.target.value)}>
          <option value="">选择抽样计划输入</option>
          {authorizedSources.map(source => <option key={inputReferenceKey(source.inputId, source.revision)} value={inputReferenceKey(source.inputId, source.revision)}>{source.inputId}@{source.revision} · {source.sha256}</option>)}
        </select>
      </label>
      <label className={styles.formLabel}>抽样主张 ID（逗号或换行分隔）<textarea className={styles.formField} value={sampledClaimIDsText} onChange={event => setSampledClaimIDsText(event.target.value)} /></label>
      {selectedDraft === null ? null : <div className={styles.formHint}>草稿 {selectedDraft.draftInputId}@{selectedDraft.draftRevision} · {selectedDraft.draftSha256} · {pendingCorrection !== null ? `此核查将关联纠错 ${pendingCorrection.correctionId}` : reviewAlreadyRecorded ? '此草稿版本已有核查记录' : '核查后如产生新草稿版本，旧核查将标为过期'}</div>}
      <button className={styles.commandButton} type="button" disabled={api.mode !== 'real' || recordReview.isPending || selectedDraft === null || reviewAlreadyRecorded || checkerEmployeeId === ''} onClick={() => { void submitReview(); }}>
        {recordReview.isPending ? '记录核查中…' : '记录独立事实核查与人工作样'}
      </button>
      {reviewMessage !== null ? <div className={styles.operationNotice} role="status">{reviewMessage}</div> : null}
      {reviews.slice(0, 10).map(review => <div className={styles.recordRow} key={review.reviewId}>
        <div className={styles.recordLead}><div><strong>{review.draftInputId}@{review.draftRevision} · {review.outcome}</strong><span>Checker {review.checkerEmployeeId} · {review.createdAt} · {review.reviewId}{review.correctionId === '' ? '' : ` · 纠错 ${review.correctionId}`}</span></div></div>
        <div className={styles.recordMeta}>
          <StatusBadge label={review.stale ? '草稿已更新，核查过期' : '绑定精确草稿版本'} tone={review.stale ? 'warning' : 'info'} />
          {review.outcome === 'accepted' && !review.stale ? <button className={styles.textButton} type="button" disabled={api.mode !== 'real' || simulatePublication.isPending || publications.some(item => item.reviewId === review.reviewId)} onClick={() => { void submitPublication(review); }}>
            {publications.some(item => item.reviewId === review.reviewId) ? '已模拟发布' : '本地模拟发布'}
          </button> : null}
        </div>
      </div>)}
      {api.mode !== 'real' ? <div className={styles.formHint}>Fixture 模式为只读，不接受内容运营写入。</div> : null}
    </div>

    <div className={styles.formStack}>
      <h3 className={styles.sectionTitle}>发布模拟与纠错</h3>
      <p className={styles.formHint}>模拟发布只保存精确核查版本的本地回执，不会连接或写入任何外部平台。</p>
      {publications.slice(0, 10).map(item => <div className={styles.recordRow} key={item.publicationId}>
        <div className={styles.recordLead}><div><strong>{item.publicationId} · simulation</strong><span>核查 {item.reviewId} · 草稿 {item.draftInputId}@{item.draftRevision} · {item.draftSha256}</span><span>回执 SHA-256 {item.receiptSha256} · externalSideEffects=false</span></div></div>
        <div className={styles.recordMeta}><StatusBadge label="无外部发布" tone="info" /></div>
      </div>)}
      <label className={styles.formLabel}>待纠错的模拟发布记录
        <select className={styles.formField} value={selectedCorrectionPublicationID} onChange={event => { setSelectedCorrectionPublicationID(event.target.value); setCorrectionDraftID(''); }}>
          <option value="">选择已模拟发布内容</option>
          {publications.map(item => <option key={item.publicationId} value={item.publicationId}>{item.publicationId} · {item.draftInputId}@{item.draftRevision}</option>)}
        </select>
      </label>
      <label className={styles.formLabel}>纠正后的新草稿版本
        <select className={styles.formField} value={correctionDraftID} onChange={event => setCorrectionDraftID(event.target.value)}>
          <option value="">选择比已发布版本更新的草稿</option>
          {correctionDraftOptions.map(item => <option key={item.draftId} value={item.draftId}>{item.draftInputId}@{item.draftRevision} · {item.draftSha256}</option>)}
        </select>
      </label>
      <label className={styles.formLabel}>纠错理由<textarea className={styles.formField} maxLength={2000} value={correctionRationale} onChange={event => setCorrectionRationale(event.target.value)} /></label>
      <button className={styles.commandButton} type="button" disabled={api.mode !== 'real' || recordCorrection.isPending || correctionDraftOptions.length === 0} onClick={() => { void submitCorrection(); }}>
        {recordCorrection.isPending ? '保存纠错中…' : '记录纠错并要求重新核查'}
      </button>
      {correctionMessage !== null ? <div className={styles.operationNotice} role="status">{correctionMessage}</div> : null}
      {corrections.slice(0, 10).map(item => <div className={styles.recordRow} key={item.correctionId}><div className={styles.recordLead}><div><strong>纠错 {item.correctionId} · {item.state}</strong><span>{item.publicationId} → {item.correctionDraftInputId}@{item.correctionDraftRevision} · {item.rationale}</span></div></div></div>)}
    </div>

    <div className={styles.formStack}>
      <h3 className={styles.sectionTitle}>内部反馈</h3>
      <label className={styles.formLabel}>关联模拟发布
        <select className={styles.formField} value={selectedFeedbackPublicationID} onChange={event => setSelectedFeedbackPublicationID(event.target.value)}>
          <option value="">选择模拟发布记录</option>
          {publications.map(item => <option key={item.publicationId} value={item.publicationId}>{item.publicationId} · {item.draftInputId}@{item.draftRevision}</option>)}
        </select>
      </label>
      <label className={styles.formLabel}>观察类别
        <select className={styles.formField} value={feedbackCategory} onChange={event => setFeedbackCategory(event.target.value as DomainContentFeedbackCategoryView)}>
          <option value="correction_requested">要求纠错</option><option value="negative">负面观察</option><option value="mixed">混合观察</option><option value="inconclusive">无法确定</option><option value="positive">正面观察</option>
        </select>
      </label>
      <label className={styles.formLabel}>反馈<textarea className={styles.formField} maxLength={2000} value={feedbackNote} onChange={event => setFeedbackNote(event.target.value)} /></label>
      <button className={styles.commandButton} type="button" disabled={api.mode !== 'real' || recordFeedback.isPending || selectedFeedbackPublicationID === '' || feedbackNote.trim() === ''} onClick={() => { void submitFeedback(); }}>
        {recordFeedback.isPending ? '保存反馈中…' : '记录内部反馈'}
      </button>
      {feedbackMessage !== null ? <div className={styles.operationNotice} role="status">{feedbackMessage}</div> : null}
      {feedback.slice(0, 10).map(item => <div className={styles.recordRow} key={item.feedbackId}><div className={styles.recordLead}><div><strong>{item.category} · {item.state}</strong><span>{item.publicationId} · {item.note}</span></div></div></div>)}
    </div>
  </section>;
}
