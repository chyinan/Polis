// pattern: Imperative Shell

import {useCallback, useEffect, useMemo, useState, type FormEvent} from 'react';
import {KeyRound, LogOut, RefreshCw, ShieldCheck} from 'lucide-react';
import {dispatchAuthorizationInvalidated, observeProtectedResponse} from '../lib/authorization-state';
import styles from './InstallationOwnerPage.module.css';

type OwnerSession = Readonly<{authenticated: boolean; expiresAt: string | null}>;
type OwnerSetupStatus = Readonly<{initialized: boolean}>;
type ObservedProviderAccount = Readonly<{
  providerClass: string;
  locatorFingerprint: string;
  firstSeenAt: string;
  lastSeenAt: string;
  companyCount: number;
  workerSessionCount: number;
}>;
type ObservedProviderAccountList = Readonly<{schemaVersion: string; accounts: ReadonlyArray<ObservedProviderAccount>}>;
type InstallationWorkerSlotPolicy = Readonly<{
  status: 'unconfigured' | 'configured' | 'over_capacity';
  configured: boolean;
  maxActiveSlots?: number;
  protectedSlots?: number;
  revision: number;
  activeSlots: number;
  ordinaryActiveSlots: number;
  protectedActiveSlots: number;
  action?: string;
}>;

class OwnerRequestError extends Error {
  constructor(message: string, readonly status: number) {
    super(message);
  }
}

async function requestJSON<T>(url: string, init: RequestInit = {}, protectedRequest = false): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  if (init.body !== undefined) headers.set('Content-Type', 'application/json');
  const response = await fetch(url, {...init, cache: 'no-store', credentials: 'include', headers});
  if (protectedRequest) observeProtectedResponse(url, response.status);
  let body: unknown = null;
  try {
    body = await response.json();
  } catch {
    body = null;
  }
  if (!response.ok) {
    const message = isRecord(body) && typeof body.error === 'string' ? body.error : `请求失败（HTTP ${response.status}）`;
    throw new OwnerRequestError(message, response.status);
  }
  if (!isRecord(body)) throw new Error('服务器返回了无效的安装账户数据');
  return body as T;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function readCSRFCookie(): string {
  const prefix = 'polis_owner_csrf=';
  const entry = document.cookie.split(';').map(value => value.trim()).find(value => value.startsWith(prefix));
  return entry === undefined ? '' : decodeURIComponent(entry.slice(prefix.length));
}

function formatDate(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '不可得' : new Intl.DateTimeFormat('zh-CN', {dateStyle: 'medium', timeStyle: 'short'}).format(date);
}

export function InstallationOwnerPage({workbenchApiBaseUrl}: Readonly<{workbenchApiBaseUrl: string}>) {
  const apiBase = useMemo(() => workbenchApiBaseUrl.replace(/\/+$/, ''), [workbenchApiBaseUrl]);
  const apiRoot = apiBase.endsWith('/api/workbench') ? apiBase.slice(0, -'/api/workbench'.length) : apiBase;
  const ownerBase = `${apiRoot}/api/installation/owner`;
  const [session, setSession] = useState<OwnerSession | null>(null);
  const [setupStatus, setSetupStatus] = useState<OwnerSetupStatus | null>(null);
  const [accounts, setAccounts] = useState<ReadonlyArray<ObservedProviderAccount> | null>(null);
  const [workerSlots, setWorkerSlots] = useState<InstallationWorkerSlotPolicy | null>(null);
  const [workerSlotsError, setWorkerSlotsError] = useState<string | null>(null);
  const [maxActiveSlots, setMaxActiveSlots] = useState('');
  const [protectedSlots, setProtectedSlots] = useState('');
  const [workerSlotsRequestID, setWorkerSlotsRequestID] = useState('');
  const [bootstrapCode, setBootstrapCode] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [loginPassword, setLoginPassword] = useState('');
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadProtectedOwnerData = useCallback(async () => {
    const result = await requestJSON<ObservedProviderAccountList>(`${apiBase}/installation/provider-accounts`, {}, true);
    setAccounts(result.accounts);
    setSetupStatus({initialized: true});
    try {
      const policy = await requestJSON<InstallationWorkerSlotPolicy>(`${apiBase}/installation/worker-slots`, {}, true);
      setWorkerSlots(policy);
      setWorkerSlotsError(null);
      setMaxActiveSlots(policy.configured && policy.maxActiveSlots !== undefined ? String(policy.maxActiveSlots) : '');
      setProtectedSlots(policy.configured && policy.protectedSlots !== undefined ? String(policy.protectedSlots) : '');
    } catch (policyError) {
      setWorkerSlots(null);
      setWorkerSlotsError(policyError instanceof Error ? policyError.message : '无法读取全局 Worker 槽位策略');
    }
  }, [apiBase]);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const current = await requestJSON<OwnerSession>(`${ownerBase}/session`);
      setSession(current);
      if (current.authenticated) {
        await loadProtectedOwnerData();
      } else {
        setAccounts(null);
        setWorkerSlots(null);
        setWorkerSlotsError(null);
        setSetupStatus(null);
        if (window.location.protocol !== 'https:') {
          try {
            setSetupStatus(await requestJSON<OwnerSetupStatus>(`${ownerBase}/status`));
          } catch (statusError) {
            if (!(statusError instanceof OwnerRequestError && statusError.status === 403)) throw statusError;
          }
        }
      }
    } catch (loadError) {
      setSession(null);
      setAccounts(null);
      setError(loadError instanceof Error ? loadError.message : '无法读取安装级账户状态');
    } finally {
      setLoading(false);
    }
  }, [loadProtectedOwnerData, ownerBase]);

  useEffect(() => { void refresh(); }, [refresh]);

  async function signIn(password: string): Promise<void> {
    await requestJSON<OwnerSession>(`${ownerBase}/login`, {method: 'POST', body: JSON.stringify({password})});
    setLoginPassword('');
    const current = await requestJSON<OwnerSession>(`${ownerBase}/session`);
    if (!current.authenticated) throw new Error('密码已验证，但当前浏览器没有保留登录会话 Cookie');
    setSession(current);
    await loadProtectedOwnerData();
  }

  async function handleLogin(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await signIn(loginPassword);
    } catch (loginError) {
      setError(loginError instanceof Error ? loginError.message : '登录失败');
    } finally {
      setSubmitting(false);
    }
  }

  async function handleBootstrap(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    const passwordBytes = new TextEncoder().encode(newPassword).length;
    if (passwordBytes < 14 || passwordBytes > 1024) {
      setError('密码长度需要为 14 到 1024 个 UTF-8 字节');
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await requestJSON(`${ownerBase}/bootstrap`, {method: 'POST', body: JSON.stringify({code: bootstrapCode.trim(), password: newPassword})});
      setNewPassword('');
      setBootstrapCode('');
      setSetupStatus({initialized: true});
      await signIn(newPassword);
    } catch (bootstrapError) {
      setError(bootstrapError instanceof Error ? bootstrapError.message : '首次设置失败');
      if (bootstrapError instanceof OwnerRequestError && bootstrapError.status === 409) setSetupStatus({initialized: true});
    } finally {
      setSubmitting(false);
    }
  }

  async function handleLogout(): Promise<void> {
    setSubmitting(true);
    setError(null);
    try {
      const csrf = readCSRFCookie();
      if (csrf === '') throw new Error('找不到 CSRF Cookie，请刷新页面后重试');
      await requestJSON(`${ownerBase}/logout`, {method: 'POST', headers: {'X-Polis-CSRF-Token': csrf}, body: '{}'}, true);
      dispatchAuthorizationInvalidated();
      setSession({authenticated: false, expiresAt: null});
      setAccounts(null);
      setWorkerSlots(null);
      setWorkerSlotsError(null);
      if (window.location.protocol !== 'https:') {
        try { setSetupStatus(await requestJSON<OwnerSetupStatus>(`${ownerBase}/status`)); } catch { setSetupStatus(null); }
      }
    } catch (logoutError) {
      setError(logoutError instanceof Error ? logoutError.message : '退出登录失败');
    } finally {
      setSubmitting(false);
    }
  }

  async function handleWorkerSlotsSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    const max = Number(maxActiveSlots);
    const protectedCount = Number(protectedSlots);
    if (!Number.isSafeInteger(max) || max < 1 || !Number.isSafeInteger(protectedCount) || protectedCount < 0 || protectedCount > max) {
      setWorkerSlotsError('总槽位必须是正整数；保护槽位必须是 0 到总槽位之间的整数。');
      return;
    }
    setSubmitting(true);
    setWorkerSlotsError(null);
    const requestID = workerSlotsRequestID || crypto.randomUUID();
    setWorkerSlotsRequestID(requestID);
    try {
      const csrf = readCSRFCookie();
      if (csrf === '') throw new Error('找不到 CSRF Cookie，请刷新页面后重试');
      const policy = await requestJSON<InstallationWorkerSlotPolicy>(`${apiBase}/installation/worker-slots`, {
        method: 'POST',
        headers: {'X-Polis-CSRF-Token': csrf},
        body: JSON.stringify({
          requestId: requestID,
          expectedRevision: workerSlots?.revision ?? 0,
          maxActiveSlots: max,
          protectedSlots: protectedCount,
        }),
      }, true);
      setWorkerSlots(policy);
      setWorkerSlotsError(null);
      setWorkerSlotsRequestID('');
    } catch (policyError) {
      setWorkerSlotsError(policyError instanceof Error ? policyError.message : '保存 Worker 槽位策略失败');
      if (policyError instanceof OwnerRequestError && policyError.status === 409) {
        setWorkerSlotsRequestID('');
        try {
          const current = await requestJSON<InstallationWorkerSlotPolicy>(`${apiBase}/installation/worker-slots`, {}, true);
          setWorkerSlots(current);
          setMaxActiveSlots(current.configured && current.maxActiveSlots !== undefined ? String(current.maxActiveSlots) : '');
          setProtectedSlots(current.configured && current.protectedSlots !== undefined ? String(current.protectedSlots) : '');
        } catch (readError) {
          setWorkerSlotsError(readError instanceof Error ? readError.message : '无法刷新 Worker 槽位策略');
        }
      } else if (policyError instanceof OwnerRequestError && policyError.status < 500) {
        setWorkerSlotsRequestID('');
      }
    } finally {
      setSubmitting(false);
    }
  }

  const needsBootstrap = setupStatus?.initialized === false;

  return (
    <section className={styles.page}>
      <header className={styles.heading}>
        <div className={styles.mark}><KeyRound aria-hidden="true" size={20} /></div>
        <div><p className={styles.eyebrow}>INSTALLATION / OWNER ACCESS</p><h1>安装级账户</h1><p>查看 Polis 已观测到的 ProviderAccount 定位信息。</p></div>
        <button aria-label="刷新安装账户状态" className={styles.refreshButton} disabled={loading || submitting} onClick={() => { void refresh(); }} type="button"><RefreshCw aria-hidden="true" size={16} />刷新</button>
      </header>

      {error ? <p className={styles.error} role="alert">{error}</p> : null}
      {loading ? <p className={styles.notice}>正在检查本地安装会话…</p> : null}

      {!loading && session?.authenticated && accounts !== null ? <>
        <div className={styles.sessionCard}>
          <ShieldCheck aria-hidden="true" size={18} /><span>安装所有者已登录</span>
          {session.expiresAt ? <small>会话到期：{formatDate(session.expiresAt)}</small> : null}
          <button className={styles.secondaryButton} disabled={submitting} onClick={() => { void handleLogout(); }} type="button"><LogOut aria-hidden="true" size={15} />退出登录</button>
        </div>
        <section className={styles.card}>
          <div className={styles.cardHeading}><div><h2>已观测账户</h2><p>只显示经过哈希处理的定位指纹和汇总计数。</p></div><span>{accounts.length} 个</span></div>
          {accounts.length === 0 ? <p className={styles.notice}>目前没有 ProviderAccount 观测记录。新 WorkerSession 建立后才会出现。</p> : <div className={styles.tableWrap}>
            <table><thead><tr><th>提供方</th><th>定位指纹</th><th>公司数</th><th>会话数</th><th>首次观测</th><th>最近观测</th></tr></thead>
              <tbody>{accounts.map(account => <tr key={`${account.providerClass}:${account.locatorFingerprint}`}><td>{account.providerClass}</td><td><code>{account.locatorFingerprint}</code></td><td>{account.companyCount}</td><td>{account.workerSessionCount}</td><td>{formatDate(account.firstSeenAt)}</td><td>{formatDate(account.lastSeenAt)}</td></tr>)}</tbody>
            </table>
          </div>}
          <p className={styles.disclaimer}>定位指纹是使用观测，不等同于账单归属证明；此页面不会设置预算、扣款或执行跨公司变更。</p>
        </section>
        <form className={styles.card} onSubmit={event => { void handleWorkerSlotsSubmit(event); }}>
          <div className={styles.cardHeading}><div><h2>全局 Worker 并发上限</h2><p>由安装所有者为整个安装明确配置容量。策略未配置时，新 WorkerSession admission 会 fail closed。</p></div><span>{workerSlots?.status ?? '不可用'}</span></div>
          {workerSlots ? <p className={styles.notice}>当前占用 {workerSlots.activeSlots} / {workerSlots.configured ? workerSlots.maxActiveSlots : '未配置'} 个槽位；普通任务 {workerSlots.ordinaryActiveSlots} 个，保护类任务 {workerSlots.protectedActiveSlots} 个。保护槽位供 review 与 peer-review 工作使用。策略版本：{workerSlots.revision}。</p> : null}
          {workerSlotsError ? <p className={styles.error} role="alert">{workerSlotsError}</p> : null}
          {workerSlots?.action ? <p className={styles.help}>{workerSlots.action}</p> : null}
          {workerSlotsRequestID ? <p className={styles.help}>上一条保存请求结果尚未确认；重试会复用相同请求身份和参数。也可刷新页面读取服务端状态。</p> : null}
          <label>总活动 Worker 槽位<input className={styles.input} disabled={submitting || workerSlotsRequestID !== ''} inputMode="numeric" min="1" onChange={event => setMaxActiveSlots(event.target.value)} required type="number" value={maxActiveSlots} /></label>
          <label>保护类槽位<input className={styles.input} disabled={submitting || workerSlotsRequestID !== ''} inputMode="numeric" min="0" onChange={event => setProtectedSlots(event.target.value)} required type="number" value={protectedSlots} /></label>
          <p className={styles.help}>保护槽位数不会自动选择。降低上限不会停止现有会话；若当前占用超过新上限，状态会显示超容量，新的 admission 保持关闭。</p>
          <button className={styles.primaryButton} disabled={submitting || workerSlots === null} type="submit">{submitting ? '正在保存…' : '保存全局槽位策略'}</button>
        </form>
      </> : null}

      {!loading && session?.authenticated === false ? <div className={styles.forms}>
        {needsBootstrap ? <form className={styles.card} onSubmit={event => { void handleBootstrap(event); }}>
          <div className={styles.cardHeading}><div><h2>首次设置安装所有者</h2><p>先在 Polis 主机终端运行 `polis owner-bootstrap`，再输入一次性代码。</p></div><span>仅首次</span></div>
          <label>一次性设置代码<input autoComplete="off" className={styles.input} onChange={event => setBootstrapCode(event.target.value)} required value={bootstrapCode} /></label>
          <label>新密码<input autoComplete="new-password" className={styles.input} onChange={event => setNewPassword(event.target.value)} required type="password" value={newPassword} /></label>
          <p className={styles.help}>密码需要 14 到 1024 个 UTF-8 字节。设置代码 10 分钟后过期。</p>
          <button className={styles.primaryButton} disabled={submitting} type="submit">{submitting ? '正在初始化…' : '创建安装所有者'}</button>
        </form> : null}
        {!needsBootstrap ? <form className={styles.card} onSubmit={event => { void handleLogin(event); }}>
          <div className={styles.cardHeading}><div><h2>所有者登录</h2><p>登录后可查看安装级账户观测。</p></div><span>{setupStatus?.initialized ? '账户已初始化' : '远程登录'}</span></div>
          <label>所有者密码<input autoComplete="current-password" className={styles.input} onChange={event => setLoginPassword(event.target.value)} required type="password" value={loginPassword} /></label>
          <button className={styles.primaryButton} disabled={submitting} type="submit">{submitting ? '正在登录…' : '登录'}</button>
          {setupStatus === null && window.location.protocol === 'https:' ? <p className={styles.help}>首次设置需要在 Polis 主机本地完成；远程 Workbench 仅接受已创建的所有者账户。</p> : null}
        </form> : null}
      </div> : null}

      {!loading && session === null ? <div className={styles.card}><p className={styles.notice}>无法确认安装会话状态。请确认 Polis 服务已启动，再刷新。</p></div> : null}
    </section>
  );
}
