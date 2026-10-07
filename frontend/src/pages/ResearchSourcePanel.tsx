// pattern: Imperative Shell

import {useRef, useState, type ReactElement} from 'react';
import type {WorkbenchApi} from '../data/workbench-api';
import {useSetResearchSourceAuthorization} from '../data/workbench-query';
import styles from '../styles/workbench.module.css';

type ResearchSourcePanelProps = Readonly<{
  api: WorkbenchApi;
  companyId: string;
  missionId: string;
}>;

export function ResearchSourcePanel({api, companyId, missionId}: ResearchSourcePanelProps): ReactElement {
  const authorizeSource = useSetResearchSourceAuthorization(api, companyId);
  const pendingRequestIds = useRef(new Map<string, string>());
  const [sourceId, setSourceId] = useState('');
  const [origin, setOrigin] = useState('');
  const [searchEndpoint, setSearchEndpoint] = useState('');
  const [credentialRef, setCredentialRef] = useState('');
  const [identitySha256, setIdentitySha256] = useState('');
  const [dataSha256, setDataSha256] = useState('');
  const [rationale, setRationale] = useState('');
  const [state, setState] = useState<'authorized' | 'revoked'>('authorized');
  const [message, setMessage] = useState<string | null>(null);

  async function submit(): Promise<void> {
    setMessage(null);
    const payload = {sourceId, missionId, origin, searchEndpoint, searchCredentialRef: credentialRef, searchRankingRevision: searchEndpoint === '' ? '' : 'research-ranking@1', profileRevision: 'research-source@1', identitySha256, dataSha256, state, rationale: rationale.trim()};
    if (sourceId.trim() === '' || missionId.trim() === '' || rationale.trim() === '' || (state === 'authorized' && (origin.trim() === '' || identitySha256.trim() === '' || dataSha256.trim() === ''))) {
      setMessage('请填写来源 ID、Mission、理由；授权时还必须填写 origin 和两个 SHA-256。');
      return;
    }
    const key = JSON.stringify(payload);
    const requestId = pendingRequestIds.current.get(key) ?? `research-source-${crypto.randomUUID()}`;
    pendingRequestIds.current.set(key, requestId);
    try {
      const result = await authorizeSource.mutateAsync({...payload, requestId});
      pendingRequestIds.current.delete(key);
      setMessage(result.state === 'authorized' ? `研究来源 ${result.sourceId} 已登记。` : `研究来源 ${result.sourceId} 已撤销。`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '研究来源操作失败。');
    }
  }

  return <section className={styles.sectionCard}>
    <div className={styles.sectionHeader}>
      <div><span className={styles.cardEyebrow}>REQ-38 / 来源登记</span><h2 className={styles.sectionTitle}>研究来源</h2></div>
      <span className={styles.statusBadge}>不执行检索</span>
    </div>
    <p className={styles.formHint}>这里只登记 Mission-scoped 来源元数据。登记不会启动搜索、抓取、浏览器或 Provider；来源必须经过后续运行资格。</p>
    <div className={styles.formStack}>
      <label className={styles.formLabel}>来源 ID<input className={styles.formField} value={sourceId} onChange={event => setSourceId(event.target.value)} placeholder="source-docs" /></label>
      <label className={styles.formLabel}>Mission ID<input className={styles.formField} value={missionId} readOnly /></label>
      <label className={styles.formLabel}>状态<select className={styles.formField} value={state} onChange={event => setState(event.target.value as 'authorized' | 'revoked')}><option value="authorized">authorized</option><option value="revoked">revoked</option></select></label>
      <label className={styles.formLabel}>HTTPS origin<input className={styles.formField} value={origin} onChange={event => setOrigin(event.target.value)} placeholder="https://example.test" /></label>
      <label className={styles.formLabel}>搜索 endpoint（可选）<input className={styles.formField} value={searchEndpoint} onChange={event => setSearchEndpoint(event.target.value)} placeholder="https://example.test/search" /></label>
      <label className={styles.formLabel}>credential ref（不填秘密）<input className={styles.formField} value={credentialRef} onChange={event => setCredentialRef(event.target.value)} placeholder="search-token-ref" /></label>
      <label className={styles.formLabel}>identity SHA-256<input className={styles.formField} value={identitySha256} onChange={event => setIdentitySha256(event.target.value)} /></label>
      <label className={styles.formLabel}>data SHA-256<input className={styles.formField} value={dataSha256} onChange={event => setDataSha256(event.target.value)} /></label>
      <label className={styles.formLabel}>理由<textarea className={styles.formField} value={rationale} onChange={event => setRationale(event.target.value)} rows={2} /></label>
    </div>
    <button className={styles.commandButton} type="button" disabled={api.mode !== 'real' || authorizeSource.isPending} onClick={() => { void submit(); }}>{authorizeSource.isPending ? '正在提交…' : state === 'authorized' ? '登记研究来源' : '撤销研究来源'}</button>
    {message ? <div className={styles.operationNotice} role="status">{message}</div> : null}
  </section>;
}
