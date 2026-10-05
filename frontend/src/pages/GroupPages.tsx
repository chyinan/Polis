// pattern: Imperative Shell

import {useState, type ReactNode} from 'react';
import {ArrowUpRight, Building2, Database, LockKeyhole, Plus, Radio, Settings2} from 'lucide-react';
import {Link, useNavigate} from 'react-router-dom';
import teamCoverage from '../../../spec/design-v0.4.5/product/TEAM_COVERAGE.json';
import teamCoverageSource from '../../../spec/design-v0.4.5/product/TEAM_COVERAGE.json?raw';
import type {WorkbenchApi} from '../data/workbench-api';
import {useCompanyList, useCompanyOverview, useCreateCompany, useUpdateCompany} from '../data/workbench-query';
import type {CompanyOverviewView, CompanySummaryView, EmployeeDraft} from '../domain/workbench';
import {labelDisplayValue, labelErrorMessage} from '../domain/display-labels';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import styles from '../styles/workbench.module.css';

type GroupPageProps = Readonly<{api: WorkbenchApi; companyId: string}>;

function ViewHeader({eyebrow, title, action}: Readonly<{eyebrow: string; title: string; action?: ReactNode}>) {
  return <div className={styles.viewHeader}><div><p className={styles.eyebrow}>{eyebrow}</p><h1 className={styles.pageTitle}>{title}</h1></div>{action ? <div className={styles.viewHeaderAction}>{action}</div> : null}</div>;
}

function DataRow({label, value}: Readonly<{label: string; value: ReactNode}>) {
  return <div className={styles.detailRow}><span className={styles.fieldLabel}>{label}</span><span className={styles.detailValue}>{value}</span></div>;
}

function EmptyGroupState({title, detail}: Readonly<{title: string; detail: string}>) {
  return <section className={styles.sectionCard}><div className={styles.emptyState}><span className={styles.emptyStateIcon}><LockKeyhole aria-hidden="true" size={18} /></span><strong>{title}</strong><span>{detail}</span></div></section>;
}

function GroupSummary({overview, companyId, companyCount}: Readonly<{overview: CompanyOverviewView; companyId: string; companyCount: number}>) {
  return <section className={styles.groupSummary}>
    <article className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>当前作用域</span><h2 className={styles.sectionTitle}>{companyCount} 个已登记公司</h2></div><StatusBadge label="真实目录" tone="info" /></div>
      <p className={styles.panelDescription}>公司目录来自控制面；切换公司后页面数据会按公司作用域重新读取。</p>
      <Link className={styles.textButton} to={'/companies/' + companyId + '/overview'}>进入 {overview.company.name} <ArrowUpRight aria-hidden="true" size={14} /></Link>
    </article>
    <article className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>需要人处理</span><h2 className={styles.sectionTitle}>{overview.attention.length} 项</h2></div><Database aria-hidden="true" className={styles.icon} size={18} /></div>
      <p className={styles.panelDescription}>集团只显示当前授权范围内的责任摘要，不合并公司内部事件。</p>
      <Link className={styles.textButton} to="/group/settings">打开集团设置 <ArrowUpRight aria-hidden="true" size={14} /></Link>
    </article>
  </section>;
}

function ExistingCompanyCoverageConfirmation({api, company}: Readonly<{api: WorkbenchApi; company: CompanySummaryView}>) {
  const updateCompany = useUpdateCompany(api);
  const [acknowledged, setAcknowledged] = useState(false);
  const [error, setError] = useState('');
  if (api.mode !== 'real' || company.teamCoverageConfirmed) return null;

  async function confirmCoverage(): Promise<void> {
    if (!acknowledged) return;
    try {
      const confirmationSha256 = await digestTeamCoverageSource();
      await updateCompany.mutateAsync({
        companyId: company.id,
        name: company.name,
        workspaceRoot: company.workspaceRoot,
        roster: company.roster,
        requestId: `company-team-coverage-confirm-${crypto.randomUUID()}`,
        teamCoverageConfirmationSha256: confirmationSha256,
      });
      setAcknowledged(false);
      setError('');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '团队覆盖确认未能保存。请检查公司目录后再决定是否重试。');
    }
  }

  return <details className={styles.coverageConfirmation}>
    <summary>审阅并确认固定矩阵</summary>
    <p className={styles.formHint}>只有当前安装负责人可以保存确认。请核对以下固定映射；保存后仍需分别验证岗位资格，执行开关保持关闭。</p>
    <div className={styles.coverageConfirmationRows}>
      {teamCoverage.coverage.map(coverage => <div className={styles.coverageConfirmationRow} key={coverage.task_type}>
        <strong>{taskTypeLabels[coverage.task_type] ?? coverage.task_type} · {coverage.owner}（{employeeLabels[coverage.owner] ?? coverage.owner}）</strong>
        <span>验收路径：{acceptancePathLabels[coverage.acceptance_path] ?? coverage.acceptance_path}；独立复核：{coverage.eligible_independent_checkers.map(checker => employeeLabels[checker] ?? checker).join('、')}</span>
        <StatusBadge compact label="岗位资格未验证" tone="neutral" />
      </div>)}
    </div>
    <label className={styles.reviewCheck}><input checked={acknowledged} onChange={event => {setAcknowledged(event.target.checked); setError('');}} type="checkbox" /><span>我作为安装负责人，已核对并确认采用上方 TEAM_COVERAGE 固定岗位映射与验收路径。</span></label>
    {error ? <p className={styles.formError} role="alert">{error} 请先核对公司目录，避免重复提交。</p> : null}
    <button className={styles.commandButton} disabled={!acknowledged || updateCompany.isPending} onClick={() => {void confirmCoverage();}} type="button">{updateCompany.isPending ? '正在保存确认…' : '以安装负责人身份确认固定矩阵'}</button>
  </details>;
}

export function GroupOverviewPage({api, companyId}: GroupPageProps) {
  const query = useCompanyOverview(api, companyId);
  const companiesQuery = useCompanyList(api);
  if (query.isPending || companiesQuery.isPending) return <div className={styles.viewStack}><ViewHeader eyebrow="集团 / 总览" title="公司总览" /><div className={styles.emptyState} role="status"><Radio aria-hidden="true" className={styles.loadingIcon} size={18} /><span>正在读取公司目录</span></div></div>;
  if (query.isError || companiesQuery.isError) return <div className={styles.viewStack}><ViewHeader eyebrow="集团 / 总览" title="公司总览" /><EmptyGroupState detail={labelErrorMessage(query.error?.message ?? companiesQuery.error?.message ?? '公司目录读取失败')} title="公司目录读取失败" /></div>;
  const overview = query.data;
  const companies = companiesQuery.data;
  return <div className={styles.viewStack} data-od-id="group-overview-view">
    <ViewHeader action={<Link className={styles.commandButton} to="/group/new-company">新建公司 <Plus aria-hidden="true" size={14} /></Link>} eyebrow="集团 / 总览" title="公司总览" />
    <GroupSummary companyId={companyId} overview={overview} companyCount={companies.length} />
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>公司目录</span><h2 className={styles.sectionTitle}>已登记公司</h2></div><StatusBadge label={`${companies.length} 个`} tone="info" /></div>
      <div className={styles.recordList}>{companies.map(company => <div className={styles.companyDirectoryEntry} key={company.id}>
        <Link className={styles.recordRow} to={`/companies/${company.id}/overview`}><div className={styles.recordLead}><Building2 aria-hidden="true" size={16} /><div><strong>{company.name}</strong><span>{company.id} · {company.workspaceRoot}</span></div></div><div className={styles.recordMeta}><StatusBadge compact label={labelDisplayValue(company.state)} tone={company.state === 'active' ? 'success' : 'neutral'} /><StatusBadge compact label={company.teamCoverageConfirmed ? '矩阵已确认 · 岗位资格未验证' : '矩阵待负责人确认'} tone={company.teamCoverageConfirmed ? 'info' : 'neutral'} /></div></Link>
        <ExistingCompanyCoverageConfirmation api={api} company={company} />
      </div>)}</div>
    </section>
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>导航覆盖</span><h2 className={styles.sectionTitle}>集团级入口</h2></div><Building2 aria-hidden="true" className={styles.icon} size={18} /></div>
      <div className={styles.routeGrid}>
        <Link className={styles.routeCard} to="/group/new-company"><strong>新建公司</strong><span>目标、团队、环境、资源、审阅</span><ArrowUpRight aria-hidden="true" size={14} /></Link>
        <Link className={styles.routeCard} to="/group/resources"><strong>集团资源</strong><span>共享资源与未知在途责任</span><ArrowUpRight aria-hidden="true" size={14} /></Link>
        <Link className={styles.routeCard} to="/group/settings"><strong>集团设置</strong><span>公共技能与 MCP 定义</span><ArrowUpRight aria-hidden="true" size={14} /></Link>
      </div>
    </section>
  </div>;
}

type WizardStep = 0 | 1 | 2 | 3 | 4;

const taskTypeLabels: Readonly<Record<string, string>> = {
  planning: '规划分析',
  frontend: '前端实现',
  backend: '后端实现',
  environment_plan: '环境方案',
  regression_test_overlay: '回归测试覆盖层',
  design_change: '设计变更',
  delivery_assembly: '交付组装',
};

const acceptancePathLabels: Readonly<Record<string, string>> = {
  owner_protected_goal_contract: '负责人保护的目标合同',
  trusted_baseline_plus_independent_review: '可信基线 + 独立复核',
  current_environment_policy: '当前环境策略',
  separate_test_quality_review_not_final_production_acceptance: '独立测试质量复核（不能替代最终生产验收）',
  risk_policy_owner_if_outside_scope: '超出范围时由风险策略负责人处理',
  immutable_manifest_and_deterministic_checks: '不可变清单 + 确定性检查',
};

const employeeLabels: Readonly<Record<string, string>> = {
  'emp-planning': '规划工程师',
  'emp-backend': '后端工程师',
  'emp-frontend': '前端工程师',
  'emp-review': '独立验收员',
};

async function digestTeamCoverageSource(): Promise<string> {
  if (!globalThis.crypto?.subtle) throw new Error('当前浏览器无法校验团队覆盖规范摘要，请使用本机受支持的浏览器重试。');
  const digest = await globalThis.crypto.subtle.digest('SHA-256', new TextEncoder().encode(teamCoverageSource));
  return Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, '0')).join('');
}

export function NewCompanyPage({api}: Readonly<{api: WorkbenchApi}>) {
  const [step, setStep] = useState<WizardStep>(0);
  const [companyId, setCompanyId] = useState('');
  const [name, setName] = useState('');
  const [workspaceRoot, setWorkspaceRoot] = useState('');
  const [confirmTeamCoverage, setConfirmTeamCoverage] = useState(false);
  const [confirmationError, setConfirmationError] = useState('');
  const [roster, setRoster] = useState<ReadonlyArray<EmployeeDraft>>(() => [
    {id: 'emp-planning', displayName: '规划工程师', role: 'planning', modelProfile: 'deterministic/fake'},
    {id: 'emp-backend', displayName: '后端工程师', role: 'backend', modelProfile: 'deterministic/fake'},
    {id: 'emp-frontend', displayName: '前端工程师', role: 'frontend', modelProfile: 'deterministic/fake'},
    {id: 'emp-review', displayName: '独立验收员', role: 'review', modelProfile: 'deterministic/fake'},
  ]);
  const createCompany = useCreateCompany(api);
  const navigate = useNavigate();
  const steps = ['目标与资料', '固定团队', '环境与工具', '资源与边界', '审阅与创建'];
  async function submitCompany(): Promise<void> {
    let teamCoverageConfirmationSHA256: string | undefined;
    if (confirmTeamCoverage) {
      try {
        teamCoverageConfirmationSHA256 = await digestTeamCoverageSource();
        setConfirmationError('');
      } catch (error) {
        setConfirmationError(error instanceof Error ? error.message : '团队覆盖规范摘要校验失败。');
        return;
      }
    }
    const receipt = await createCompany.mutateAsync({id: companyId, name, workspaceRoot, roster, requestId: `company-create-${companyId}`, ...(teamCoverageConfirmationSHA256 ? {teamCoverageConfirmationSha256: teamCoverageConfirmationSHA256} : {})});
    navigate(`/companies/${receipt.targetId}/overview`);
  }
  function updateEmployee(employeeId: string, field: keyof EmployeeDraft, value: string): void {
    setRoster(current => current.map(employee => employee.id === employeeId ? {...employee, [field]: value} : employee));
  }
  return <div className={styles.viewStack} data-od-id="new-company-view">
    <ViewHeader eyebrow="集团 / 公司向导" title="新建公司" />
    <div className={styles.stepper}>{steps.map((label, index) => <button className={styles.stepItem + ' ' + (step === index ? styles.stepItemActive : '') + ' ' + (index < step ? styles.stepItemDone : '')} key={label} onClick={() => setStep(index as WizardStep)} type="button"><span>{String(index + 1).padStart(2, '0')}</span>{label}</button>)}</div>
    <section className={styles.sectionCard} data-od-id="company-creation-step">
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>第 {step + 1} 步 / 5</span><h2 className={styles.sectionTitle}>{steps[step]}</h2></div><StatusBadge label="本地草案" tone="neutral" /></div>
      {step === 0 ? <div className={styles.formStack}><label className={styles.formLabel}>公司 ID<input className={styles.formField} value={companyId} onChange={event => setCompanyId(event.target.value)} placeholder="例如 content-lab" /></label><label className={styles.formLabel}>公司名称<input className={styles.formField} value={name} onChange={event => setName(event.target.value)} placeholder="例如 Polis 内容实验室" /></label><p className={styles.formHint}>创建会保存固定组织，但不会启动使命或调用模型。</p></div> : null}
      {step === 1 ? <div className={styles.formStack}>{roster.map(employee => <div className={styles.wizardChoice} key={employee.id}><strong>{employee.id}</strong><label className={styles.formLabel}>显示名<input className={styles.formField} value={employee.displayName} onChange={event => updateEmployee(employee.id, 'displayName', event.target.value)} /></label><label className={styles.formLabel}>岗位<input className={styles.formField} value={employee.role} onChange={event => updateEmployee(employee.id, 'role', event.target.value)} /></label><label className={styles.formLabel}>模型配置<input className={styles.formField} value={employee.modelProfile} onChange={event => updateEmployee(employee.id, 'modelProfile', event.target.value)} /></label></div>)}<section aria-label="固定团队任务覆盖草案" className={styles.wizardChoice} data-od-id="company-team-coverage-draft"><div><strong>固定团队任务覆盖（设计草案 v{teamCoverage.document_version}）</strong><p className={styles.formHint}>负责人确认前不作为执行合同。覆盖表中的岗位资格均未验证；按草案要求，任务准入需要先由人工处理。</p></div><div aria-label="七类任务覆盖" className={styles.recordList} role="list">{teamCoverage.coverage.map(coverage => <div className={styles.recordRow} key={coverage.task_type} role="listitem"><div className={styles.recordLead}><div><strong>{taskTypeLabels[coverage.task_type] ?? coverage.task_type} · {coverage.owner}（{employeeLabels[coverage.owner] ?? coverage.owner}）</strong><span>验收路径：{acceptancePathLabels[coverage.acceptance_path] ?? coverage.acceptance_path}；独立复核：{coverage.eligible_independent_checkers.map(checker => employeeLabels[checker] ?? checker).join('、')}</span></div></div><div className={styles.recordMeta}><StatusBadge compact label={coverage.qualification === 'qualified' ? '已资格验证' : '未资格验证'} tone={coverage.qualification === 'qualified' ? 'success' : 'neutral'} /></div></div>)}</div><p className={styles.formHint}>执行开关：{teamCoverage.execution_enabled ? '开启' : '关闭'}。创建公司只登记组织信息，不代表负责人已确认该矩阵，也不会放行真实提供方执行。</p></section></div> : null}
      {step === 2 ? <div className={styles.formStack}><label className={styles.formLabel}>工作区 / 项目目录<input className={styles.formField} value={workspaceRoot} onChange={event => setWorkspaceRoot(event.target.value)} placeholder="例如 C:/workspace/content-lab" /></label><p className={styles.formHint}>只保存授权配置，不扫描任意宿主目录、不运行脚本。</p><div className={styles.wizardGrid}><div className={styles.wizardChoice}><Database aria-hidden="true" className={styles.icon} size={18} /><strong>技能 / MCP</strong><span>当前阶段默认不绑定外部能力。</span><StatusBadge label="默认关闭" tone="neutral" /></div><div className={styles.wizardChoice}><Settings2 aria-hidden="true" className={styles.icon} size={18} /><strong>模型提供方</strong><span>使用现有运行时就绪状态，不在创建时启动。</span><StatusBadge label="不探测" tone="info" /></div></div></div> : null}
      {step === 3 ? <div className={styles.detailRows}><DataRow label="预算口径" value="现有 tool-call 口径；不伪造美元上限" /><DataRow label="保护收尾额" value="由运行时现有语义决定" /><DataRow label="外部动作" value="默认关闭" /><DataRow label="启动状态" value="创建后保持 draft" /></div> : null}
      {step === 4 ? <div className={styles.detailRows}>
        <DataRow label="公司 ID" value={companyId || '未填写'} />
        <DataRow label="公司名称" value={name || '未填写'} />
        <DataRow label="工作区目录" value={workspaceRoot || '未填写'} />
        <DataRow label="固定员工" value={`${roster.length} 个逻辑员工`} />
        <DataRow label="团队执行准入" value={<StatusBadge compact label={confirmTeamCoverage ? '提交后记录矩阵确认 · 资格未验证' : '需负责人确认 · 资格未验证'} tone="neutral" />} />
        <DataRow label="外部效果" value="不发送、不探测、不发布" />
        <p className={styles.formHint}>岗位资格仍需独立验证。未勾选时可创建团队矩阵未确认的公司；勾选后将记录本次显示的 TEAM_COVERAGE 摘要，且需要当前安装负责人会话。</p>
        {api.mode === 'real' ? <label className={styles.reviewCheck}><input checked={confirmTeamCoverage} onChange={event => {setConfirmTeamCoverage(event.target.checked); setConfirmationError('');}} type="checkbox" /><span>我作为安装负责人，已核对并确认采用上方固定团队任务覆盖草案。此确认只确认岗位映射与验收路径，不代表任何岗位已通过能力或真实提供方资格验证。</span></label> : <p className={styles.formHint}>模拟模式不会记录负责人确认。</p>}
        {confirmationError ? <p className={styles.formError} role="alert">{confirmationError}</p> : null}
        {createCompany.isError ? <p className={styles.formError} role="alert">创建结果尚未确认：{createCompany.error.message} 请先到集团公司目录核对，再决定是否重试。</p> : null}
      </div> : null}
      <div className={styles.wizardActions}><button className={styles.textButton} disabled={step === 0 || createCompany.isPending} onClick={() => setStep((step - 1) as WizardStep)} type="button">上一步</button>{step < 4 ? <button className={styles.commandButton} onClick={() => setStep((step + 1) as WizardStep)} type="button">下一步 <ArrowUpRight aria-hidden="true" size={14} /></button> : <button className={styles.commandButton} disabled={createCompany.isPending} onClick={() => { void submitCompany(); }} type="button">{createCompany.isPending ? '正在创建…' : '创建公司'} <ArrowUpRight aria-hidden="true" size={14} /></button>}</div>
    </section>
  </div>;
}

export function GroupResourcesPage() {
  return <div className={styles.viewStack}><ViewHeader eyebrow="集团 / 资源" title="集团资源" /><section className={styles.resourceGrid}><article className={styles.resourceCard}><span className={styles.cardEyebrow}>共享资源</span><strong>不可得</strong><p>集团资源读取接口尚未接入</p><StatusBadge label="需核对" tone="neutral" /></article><article className={styles.resourceCard}><span className={styles.cardEyebrow}>在途责任</span><strong>不可得</strong><p>不跨公司合并责任</p><StatusBadge label="未知" tone="neutral" /></article><article className={styles.resourceCard}><span className={styles.cardEyebrow}>凭据目录</span><strong>隐藏</strong><p>不向普通观察页面暴露秘密</p><StatusBadge label="受保护" tone="info" /></article></section><EmptyGroupState detail="页面结构已保留；接入集团资源读取接口后只替换数据适配器。" title="集团资源尚未接入" /></div>;
}

export function GroupSettingsPage() {
  const [tab, setTab] = useState<'skills' | 'mcp'>('skills');
  return <div className={styles.viewStack}><ViewHeader eyebrow="集团 / 设置" title="集团设置" /><div className={styles.tabBar} role="tablist"><button aria-selected={tab === 'skills'} className={styles.tabButton + ' ' + (tab === 'skills' ? styles.tabButtonActive : '')} onClick={() => setTab('skills')} role="tab" type="button">公共技能</button><button aria-selected={tab === 'mcp'} className={styles.tabButton + ' ' + (tab === 'mcp' ? styles.tabButtonActive : '')} onClick={() => setTab('mcp')} role="tab" type="button">MCP 定义</button></div><section className={styles.sectionCard}><div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>{tab === 'skills' ? '技能版本目录' : 'MCP 资格面板'}</span><h2 className={styles.sectionTitle}>{tab === 'skills' ? '公共技能' : 'MCP 定义'}</h2></div><Settings2 aria-hidden="true" className={styles.icon} size={18} /></div><div className={styles.recordList}><div className={styles.emptyState}><span>当前没有已登记的{tab === 'skills' ? '公共技能' : 'MCP 定义'}。</span></div></div></section></div>;
}
