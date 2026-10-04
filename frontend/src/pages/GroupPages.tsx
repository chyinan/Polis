// pattern: Imperative Shell

import {useState, type ReactNode} from 'react';
import {ArrowUpRight, Building2, Database, LockKeyhole, Plus, Radio, Settings2} from 'lucide-react';
import {Link, useNavigate} from 'react-router-dom';
import type {WorkbenchApi} from '../data/workbench-api';
import {useCompanyList, useCompanyOverview, useCreateCompany} from '../data/workbench-query';
import type {CompanyOverviewView, EmployeeDraft} from '../domain/workbench';
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
      <div className={styles.recordList}>{companies.map(company => <Link className={styles.recordRow} key={company.id} to={`/companies/${company.id}/overview`}><div className={styles.recordLead}><Building2 aria-hidden="true" size={16} /><div><strong>{company.name}</strong><span>{company.id} · {company.workspaceRoot}</span></div></div><StatusBadge label={labelDisplayValue(company.state)} tone={company.state === 'active' ? 'success' : 'neutral'} /></Link>)}</div>
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

export function NewCompanyPage({api}: Readonly<{api: WorkbenchApi}>) {
  const [step, setStep] = useState<WizardStep>(0);
  const [companyId, setCompanyId] = useState('');
  const [name, setName] = useState('');
  const [workspaceRoot, setWorkspaceRoot] = useState('');
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
    const receipt = await createCompany.mutateAsync({id: companyId, name, workspaceRoot, roster, requestId: `company-create-${companyId}`});
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
      {step === 1 ? <div className={styles.formStack}>{roster.map(employee => <div className={styles.wizardChoice} key={employee.id}><strong>{employee.id}</strong><label className={styles.formLabel}>显示名<input className={styles.formField} value={employee.displayName} onChange={event => updateEmployee(employee.id, 'displayName', event.target.value)} /></label><label className={styles.formLabel}>岗位<input className={styles.formField} value={employee.role} onChange={event => updateEmployee(employee.id, 'role', event.target.value)} /></label><label className={styles.formLabel}>模型配置<input className={styles.formField} value={employee.modelProfile} onChange={event => updateEmployee(employee.id, 'modelProfile', event.target.value)} /></label></div>)}</div> : null}
      {step === 2 ? <div className={styles.formStack}><label className={styles.formLabel}>工作区 / 项目目录<input className={styles.formField} value={workspaceRoot} onChange={event => setWorkspaceRoot(event.target.value)} placeholder="例如 C:/workspace/content-lab" /></label><p className={styles.formHint}>只保存授权配置，不扫描任意宿主目录、不运行脚本。</p><div className={styles.wizardGrid}><div className={styles.wizardChoice}><Database aria-hidden="true" className={styles.icon} size={18} /><strong>技能 / MCP</strong><span>当前阶段默认不绑定外部能力。</span><StatusBadge label="默认关闭" tone="neutral" /></div><div className={styles.wizardChoice}><Settings2 aria-hidden="true" className={styles.icon} size={18} /><strong>模型提供方</strong><span>使用现有运行时就绪状态，不在创建时启动。</span><StatusBadge label="不探测" tone="info" /></div></div></div> : null}
      {step === 3 ? <div className={styles.detailRows}><DataRow label="预算口径" value="现有 tool-call 口径；不伪造美元上限" /><DataRow label="保护收尾额" value="由运行时现有语义决定" /><DataRow label="外部动作" value="默认关闭" /><DataRow label="启动状态" value="创建后保持 draft" /></div> : null}
      {step === 4 ? <div className={styles.detailRows}><DataRow label="公司 ID" value={companyId || '未填写'} /><DataRow label="公司名称" value={name || '未填写'} /><DataRow label="工作区目录" value={workspaceRoot || '未填写'} /><DataRow label="固定员工" value={`${roster.length} 个逻辑员工`} /><DataRow label="外部效果" value="不发送、不探测、不发布" />{createCompany.isError ? <p className={styles.formError} role="alert">创建结果尚未确认：{createCompany.error.message} 请先到集团公司目录核对，再决定是否重试。</p> : null}</div> : null}
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
