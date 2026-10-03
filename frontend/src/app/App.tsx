// pattern: Imperative Shell

import {useEffect, useMemo, useState} from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {BrowserRouter, Navigate, Route, Routes} from 'react-router-dom';
import {useActivityStream, useCompanyOverview} from '../data/workbench-query';
import {AppShell} from '../components/workbench-shell/AppShell';
import {FixtureWorkbenchApi, FIXTURE_COMPANY_ID} from '../data/fixture-workbench-api';
import {RealWorkbenchApi} from '../data/real-workbench-api';
import type {ActivityStreamStatus, WorkbenchApi} from '../data/workbench-api';
import type {EmployeeDraft, Freshness} from '../domain/workbench';
import {labelErrorMessage} from '../domain/display-labels';
import {getDesktopRuntime, isDesktopHost, listenForDesktopEvent, openDesktopLogs, quitDesktop, requestQuit, restartLocalServices, setLastCompany, type DesktopRuntimeSnapshot} from '../lib/desktop-bridge';
import {ActivityPage} from '../pages/ActivityPage';
import {CompanyOverviewPage} from '../pages/CompanyOverviewPage';
import {CollaborationPage, DecisionsPage, EmployeesPage, EvidencePage, MissionPage, OperationsPage, OrgChartPage, ResourcesPage, SettingsPage, TaskPage} from '../pages/WorkbenchPages';
import {FeedbackPage, GroupResourcesPage, GroupSettingsPage} from '../pages/CurrentScopePages';
import {NotificationSettingsPage} from '../pages/NotificationSettingsPage';
import {InstallationOwnerPage} from '../pages/InstallationOwnerPage';
import {GroupOverviewPage, NewCompanyPage} from '../pages/GroupPages';
import styles from './app.module.css';

const queryClient = new QueryClient();
const browserApiBaseUrl = import.meta.env.VITE_WORKBENCH_API_BASE_URL ?? '/api/workbench';
const browserApi: WorkbenchApi = import.meta.env.VITE_WORKBENCH_MODE === 'fixture'
  ? new FixtureWorkbenchApi()
  : new RealWorkbenchApi(browserApiBaseUrl, import.meta.env.VITE_WORKBENCH_LIVE_UPDATES !== 'deferred');

function NotAvailablePage() {
  return <div className={styles.notAvailable}><div className={styles.notAvailableCode}>404</div><h1>页面未找到</h1><p>请从当前公司工作台导航返回已接入的页面。</p></div>;
}

export function App() {
  if (isDesktopHost()) return <DesktopRuntimeGate />;
  const companyId = browserApi.mode === 'real' ? import.meta.env.VITE_WORKBENCH_COMPANY_ID ?? null : FIXTURE_COMPANY_ID;
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        {companyId === null ? <RealModeSetupPage /> : <WorkbenchRuntime api={browserApi} apiBaseUrl={browserApiBaseUrl} companyId={companyId} />}
      </BrowserRouter>
    </QueryClientProvider>
  );
}

function WorkbenchRuntime({api, apiBaseUrl, companyId}: Readonly<{api: WorkbenchApi; apiBaseUrl: string; companyId: string}>) {
  const overviewQuery = useCompanyOverview(api, companyId);
  const meta = overviewQuery.data?.meta ?? null;
  const streamStatus = useActivityStream(api, companyId, meta?.snapshotCursor ?? null);
  const companyName = overviewQuery.data?.company.name ?? '当前公司';
  const freshness: Freshness = meta?.freshness ?? (overviewQuery.isError ? 'stale' : 'unknown');
  const shellFreshness = freshnessFromStream(freshness, streamStatus);
  const dataMode = meta?.dataMode ?? api.mode;
  return (
    <AppShell apiBaseUrl={apiBaseUrl} companyId={companyId} companyName={companyName} dataMode={dataMode} freshness={shellFreshness} observedAt={meta?.observedAt ?? null} recoveryState={meta?.recoveryState ?? 'unknown'} streamStatus={streamStatus}>
      <Routes>
        <Route element={<Navigate replace to={`/companies/${companyId}/overview`} />} path="/" />
        <Route element={<CompanyOverviewPage api={api} companyId={companyId} />} path="/companies/:companyId/overview" />
        <Route element={<ActivityPage api={api} companyId={companyId} snapshotCursor={meta?.snapshotCursor ?? null} streamStatus={streamStatus} />} path="/companies/:companyId/activity" />
        <Route element={<CollaborationPage api={api} companyId={companyId} />} path="/companies/:companyId/collaboration" />
        <Route element={<OperationsPage api={api} companyId={companyId} />} path="/companies/:companyId/operations" />
        <Route element={<MissionPage api={api} companyId={companyId} />} path="/companies/:companyId/mission" />
        <Route element={<TaskPage api={api} companyId={companyId} />} path="/companies/:companyId/tasks" />
        <Route element={<EmployeesPage api={api} companyId={companyId} />} path="/companies/:companyId/employees" />
        <Route element={<OrgChartPage api={api} companyId={companyId} />} path="/companies/:companyId/org-chart" />
        <Route element={<ResourcesPage api={api} companyId={companyId} />} path="/companies/:companyId/resources" />
        <Route element={<EvidencePage api={api} companyId={companyId} />} path="/companies/:companyId/evidence" />
        <Route element={<DecisionsPage api={api} companyId={companyId} />} path="/companies/:companyId/decisions" />
        <Route element={<FeedbackPage api={api} companyId={companyId} />} path="/companies/:companyId/feedback" />
        <Route element={<NotificationSettingsPage api={api} companyId={companyId} />} path="/companies/:companyId/notifications" />
        <Route element={<SettingsPage api={api} companyId={companyId} />} path="/companies/:companyId/settings" />
        <Route element={<GroupOverviewPage api={api} companyId={companyId} />} path="/group/overview" />
        <Route element={<NewCompanyPage api={api} />} path="/group/new-company" />
        <Route element={<GroupResourcesPage api={api} companyId={companyId} />} path="/group/resources" />
        <Route element={<GroupSettingsPage api={api} companyId={companyId} />} path="/group/settings" />
        <Route element={<InstallationOwnerPage workbenchApiBaseUrl={apiBaseUrl} />} path="/group/installation" />
        <Route element={<NotAvailablePage />} path="*" />
      </Routes>
    </AppShell>
  );
}

function DesktopRuntimeGate() {
  const [runtime, setRuntime] = useState<DesktopRuntimeSnapshot | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    let cancelled = false;
    let timer: number | undefined;
    async function refresh(): Promise<void> {
      try {
        const next = await getDesktopRuntime();
        if (cancelled) return;
        setRuntime(next);
        setError(null);
        if (next.startup_stage !== 'ready' && next.startup_stage !== 'failed' && next.startup_stage !== 'stopped') {
          timer = window.setTimeout(() => { void refresh(); }, 250);
        }
      } catch (refreshError) {
        if (!cancelled) setError(refreshError instanceof Error ? refreshError.message : '读取桌面运行时失败');
      }
    }
    void refresh();
    return () => {
      cancelled = true;
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, []);

  useEffect(() => {
    const unlisten = [
      listenForDesktopEvent('desktop-runtime-status', () => undefined),
      listenForDesktopEvent('desktop-restart-requested', () => { void restartLocalServices().then(setRuntime).catch(errorValue => setError(errorValue instanceof Error ? errorValue.message : '重启本地服务失败')); }),
      listenForDesktopEvent('desktop-open-logs', () => { void openDesktopLogs(); }),
      listenForDesktopEvent('desktop-quit-requested', () => { void handleDesktopQuit(); }),
    ];
    return () => { unlisten.forEach(promise => { void promise.then(dispose => dispose()); }); };
    async function handleDesktopQuit(): Promise<void> {
      try {
        const active = await requestQuit();
        const force = active ? window.confirm('Polis 仍有进行中的工作。确认安全停止并退出吗？') : false;
        if (!active || force) await quitDesktop(force);
      } catch (quitError) {
        setError(quitError instanceof Error ? quitError.message : '安全退出 Polis 失败');
      }
    }
  }, []);

  if (error !== null || runtime?.startup_stage === 'failed') {
    return <DesktopFailurePage message={error ?? runtime?.diagnostics ?? '桌面服务启动失败'} onRetry={() => { void restartLocalServices().then(setRuntime).catch(errorValue => setError(errorValue instanceof Error ? errorValue.message : '重启本地服务失败')); }} onOpenLogs={() => { void openDesktopLogs(); }} />;
  }
  if (runtime === null || runtime.startup_stage !== 'ready' || runtime.api_base_url === null || runtime.session_token === null) {
    return <DesktopStartupPage runtime={runtime} />;
  }
  const api = new RealWorkbenchApi(`${runtime.api_base_url}/api/workbench`, true, runtime.session_token);
  if (runtime.first_run_required) {
    return <QueryClientProvider client={queryClient}><BrowserRouter><DesktopFirstRunWizard api={api} runtime={runtime} onComplete={setRuntime} /></BrowserRouter></QueryClientProvider>;
  }
  return <DesktopCompanyResolver api={api} runtime={runtime} />;
}

function DesktopCompanyResolver({api, runtime}: Readonly<{api: WorkbenchApi; runtime: DesktopRuntimeSnapshot}>) {
  const [companyId, setCompanyId] = useState<string | null>(runtime.last_company_id);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    if (companyId !== null) return;
    void api.listCompanies().then(companies => {
      const first = companies.find(company => company.state === 'active') ?? companies[0];
      if (first === undefined) {
      setError('当前没有公司，请重新进入首次设置');
        return;
      }
      setCompanyId(first.id);
      void setLastCompany(first.id);
    }).catch(readError => setError(readError instanceof Error ? readError.message : '读取公司目录失败'));
  }, [api, companyId]);
  if (error !== null) return <DesktopFailurePage message={error} onRetry={() => window.location.reload()} onOpenLogs={() => { void openDesktopLogs(); }} />;
  if (companyId === null) return <DesktopStartupPage runtime={runtime} />;
  return <QueryClientProvider client={queryClient}><BrowserRouter><WorkbenchRuntime api={api} apiBaseUrl={`${runtime.api_base_url}/api/workbench`} companyId={companyId} /></BrowserRouter></QueryClientProvider>;
}

function DesktopStartupPage({runtime}: Readonly<{runtime: DesktopRuntimeSnapshot | null}>) {
  const rows = runtime === null ? [] : [
    ['数据库', runtime.database.state],
    ['Polis 运行时', runtime.backend.state],
    ['AI 运行时', runtime.provider.state === 'ready' ? 'ready' : runtime.provider.auth_state],
    ['事件流', runtime.event_stream.state],
  ];
  return <main className={styles.desktopGate}><p className={styles.eyebrow}>POLIS DESKTOP / 启动</p><h1>正在准备 Polis</h1><p className={styles.desktopGateLead}>{startupStageLabel(runtime?.startup_stage ?? 'starting_database')}</p><div className={styles.desktopStatusList}>{rows.map(([label, state]) => <div className={styles.desktopStatusRow} key={label}><span>{label}</span><strong data-state={state}>{desktopStateLabel(state)}</strong></div>)}</div>{runtime?.provider.reason ? <p className={styles.formHint}>{labelErrorMessage(runtime.provider.reason)}</p> : null}<p className={styles.desktopGateMeta}>{runtime?.data_root ?? '正在解析本地数据目录'}</p></main>;
}

function DesktopFailurePage({message, onRetry, onOpenLogs}: Readonly<{message: string; onRetry: () => void; onOpenLogs: () => void}>) {
  return <main className={styles.desktopGate}><p className={styles.eyebrow}>POLIS DESKTOP / 诊断</p><h1>本地服务启动失败</h1><p className={styles.desktopGateLead}>{labelErrorMessage(message)}</p><div className={styles.wizardActions}><button className={styles.commandButton} onClick={onRetry} type="button">重试</button><button className={styles.textButton} onClick={onOpenLogs} type="button">打开诊断日志</button></div></main>;
}

function DesktopFirstRunWizard({api, runtime, onComplete}: Readonly<{api: WorkbenchApi; runtime: DesktopRuntimeSnapshot; onComplete: (runtime: DesktopRuntimeSnapshot) => void}>) {
  const [name, setName] = useState('');
  const [workspaceRoot, setWorkspaceRoot] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const companyId = useMemo(() => slugify(name) || 'company', [name]);
  async function createCompany(): Promise<void> {
    if (name.trim() === '' || workspaceRoot.trim() === '') { setError('公司名称和工作区目录不能为空'); return; }
    setSubmitting(true);
    setError(null);
    try {
      const receipt = await api.createCompany({id: companyId, name: name.trim(), workspaceRoot: workspaceRoot.trim(), roster: desktopRoster, requestId: `desktop-company-${companyId}`});
      await setLastCompany(receipt.targetId);
      onComplete({...runtime, first_run_required: false, last_company_id: receipt.targetId});
    } catch (createError) {
      setError(createError instanceof Error ? createError.message : '创建公司失败');
    } finally {
      setSubmitting(false);
    }
  }
  return <main className={styles.desktopGate}><p className={styles.eyebrow}>POLIS DESKTOP / 首次设置</p><h1>欢迎使用 Polis</h1><p className={styles.desktopGateLead}>本地数据库和 Polis 运行时已准备好。完成一次配置后，之后双击即可恢复上次公司。</p><section className={styles.desktopWizardCard}><div className={styles.desktopStatusList}><div className={styles.desktopStatusRow}><span>PostgreSQL</span><strong data-state={runtime.database.state}>{desktopStateLabel(runtime.database.state)}</strong></div><div className={styles.desktopStatusRow}><span>Polis 运行时</span><strong data-state={runtime.backend.state}>{desktopStateLabel(runtime.backend.state)}</strong></div><div className={styles.desktopStatusRow}><span>AI 运行时</span><strong data-state={runtime.provider.state}>{runtime.provider.state === 'ready' ? '已就绪' : '需要设置'}</strong></div></div><label className={styles.formLabel}>公司名称<input className={styles.formField} value={name} onChange={event => setName(event.target.value)} placeholder="例如：内容实验室" /></label><label className={styles.formLabel}>工作区 / 项目目录<input className={styles.formField} value={workspaceRoot} onChange={event => setWorkspaceRoot(event.target.value)} placeholder="例如：C:\\workspace\\content-lab" /></label><p className={styles.formHint}>将使用当前产品固定角色模板：规划、后端、前端、审查。不会执行 SQL，也不会发起真实模型业务。</p>{error ? <p className={styles.formError} role="alert">{labelErrorMessage(error)}</p> : null}<div className={styles.wizardActions}><button className={styles.commandButton} disabled={submitting} onClick={() => { void createCompany(); }} type="button">{submitting ? '正在创建…' : '创建公司并进入工作台'}</button></div></section></main>;
}

const desktopRoster: ReadonlyArray<EmployeeDraft> = [
  {id: 'emp-planning', displayName: '规划工程师', role: 'planning', modelProfile: 'deterministic/fake'},
  {id: 'emp-backend', displayName: '后端工程师', role: 'backend', modelProfile: 'deterministic/fake'},
  {id: 'emp-frontend', displayName: '前端工程师', role: 'frontend', modelProfile: 'deterministic/fake'},
  {id: 'emp-review', displayName: '审查员', role: 'review', modelProfile: 'deterministic/fake'},
];

function slugify(value: string): string {
  return value.trim().toLowerCase().replace(/[^a-z0-9_-]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 80);
}

function startupStageLabel(stage: string): string {
  const labels: Record<string, string> = {starting_database: '正在启动数据库…', migrating_database: '正在迁移数据库…', starting_polis_runtime: '正在启动 Polis 运行时…', checking_ai_runtime: '正在检查 AI 运行时…', ready: '已就绪', failed: '启动失败', stopped: '已停止'};
  return labels[stage] ?? '正在准备本地服务…';
}

function desktopStateLabel(state: string): string {
  const labels: Record<string, string> = {ready: '已就绪', starting: '启动中', failed: '失败', stopped: '已停止', degraded: '降级', setup_required: '需要设置', authentication_required: '需要认证', runtime_missing: '运行时缺失'};
  return labels[state] ?? state;
}

function freshnessFromStream(freshness: Freshness, streamStatus: ActivityStreamStatus): Freshness {
  if (streamStatus === 'reconnecting') {
    return 'reconnecting';
  }
  if (streamStatus === 'incompatible') {
    return 'stale';
  }
  return freshness;
}

function RealModeSetupPage() {
    return <div className={styles.notAvailable}><div className={styles.notAvailableCode}>实时数据模式 / 需要作用域</div><h1>尚未配置公司作用域</h1><p>实时工作台需要通过 VITE_WORKBENCH_COMPANY_ID 明确指定公司；不会把示例公司的 ID 冒充成实时作用域。</p></div>;
}
