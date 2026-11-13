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
        <Link className={styles.routeCard} to="/group/new-company"><strong>新建公司</strong><span>目标、团队、环境与能力、资源、审阅</span><ArrowUpRight aria-hidden="true" size={14} /></Link>
        <Link className={styles.routeCard} to="/group/resources"><strong>集团资源</strong><span>共享资源与未知在途责任</span><ArrowUpRight aria-hidden="true" size={14} /></Link>
        <Link className={styles.routeCard} to="/group/settings"><strong>集团设置</strong><span>公共技能与 MCP 定义</span><ArrowUpRight aria-hidden="true" size={14} /></Link>
      </div>
    </section>
  </div>;
}

type WizardStep = 0 | 1 | 2 | 3 | 4;

const COMPANY_ID_PATTERN = /^[a-zA-Z0-9_-]{1,80}$/;
const MAX_COMPANY_NAME = 200;
const MAX_WORKSPACE_ROOT = 1024;
const MAX_EMPLOYEE_LABEL = 120;
const MAX_EMPLOYEE_ROLE = 120;
const MAX_MODEL_PROFILE = 200;

const defaultRoster = (): ReadonlyArray<EmployeeDraft> => [
  {id: 'emp-planning', displayName: '规划工程师', role: 'planning', modelProfile: 'deterministic/fake'},
  {id: 'emp-backend', displayName: '后端工程师', role: 'backend', modelProfile: 'deterministic/fake'},
  {id: 'emp-frontend', displayName: '前端工程师', role: 'frontend', modelProfile: 'deterministic/fake'},
  {id: 'emp-review', displayName: '独立验收员', role: 'review', modelProfile: 'deterministic/fake'},
];

function slugifyCompanyId(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[\s._]+/g, '-')
    .replace(/[^a-z0-9_-]/g, '')
    .replace(/-{2,}/g, '-')
    .replace(/^[-_]+|[-_]+$/g, '')
    .slice(0, 80);
}

function validateCompanyId(value: string): string | null {
  const id = value.trim();
  if (id === '') return '请填写公司 ID';
  if (!COMPANY_ID_PATTERN.test(id)) return '仅支持字母、数字、下划线和连字符，长度 1-80';
  return null;
}

function validateCompanyName(value: string): string | null {
  const name = value.trim();
  if (name === '') return '请填写公司名称';
  if (name.length > MAX_COMPANY_NAME) return `公司名称不能超过 ${MAX_COMPANY_NAME} 个字符`;
  return null;
}

function validateWorkspaceRoot(value: string): string | null {
  const root = value.trim();
  if (root === '') return '请填写工作区 / 项目目录';
  if (root.length > MAX_WORKSPACE_ROOT) return `工作区目录不能超过 ${MAX_WORKSPACE_ROOT} 个字符`;
  return null;
}

function validateEmployeeDraft(employee: EmployeeDraft): Readonly<{displayName: string | null; role: string | null; modelProfile: string | null}> {
  const displayName = employee.displayName.trim();
  const role = employee.role.trim();
  const modelProfile = employee.modelProfile.trim();
  return {
    displayName: displayName === '' ? '请填写显示名' : displayName.length > MAX_EMPLOYEE_LABEL ? `显示名不能超过 ${MAX_EMPLOYEE_LABEL} 个字符` : null,
    role: role === '' ? '请填写岗位' : role.length > MAX_EMPLOYEE_ROLE ? `岗位不能超过 ${MAX_EMPLOYEE_ROLE} 个字符` : null,
    modelProfile: modelProfile === '' ? '请填写模型配置' : modelProfile.length > MAX_MODEL_PROFILE ? `模型配置不能超过 ${MAX_MODEL_PROFILE} 个字符` : null,
  };
}

export function NewCompanyPage({api}: Readonly<{api: WorkbenchApi}>) {
  const [step, setStep] = useState<WizardStep>(0);
  const [companyId, setCompanyId] = useState('');
  const [idTouched, setIdTouched] = useState(false);
  const [name, setName] = useState('');
  const [workspaceRoot, setWorkspaceRoot] = useState('');
  const [roster, setRoster] = useState<ReadonlyArray<EmployeeDraft>>(() => defaultRoster());
  const [showValidation, setShowValidation] = useState(false);
  const createCompany = useCreateCompany(api);
  const navigate = useNavigate();
  const steps = ['目标与资料', '固定团队', '环境与工具', '资源与边界', '审阅与创建'];

  const companyIdError = validateCompanyId(companyId);
  const nameError = validateCompanyName(name);
  const workspaceRootError = validateWorkspaceRoot(workspaceRoot);
  const rosterErrors = roster.map(validateEmployeeDraft);
  const rosterInvalid = rosterErrors.some(error => error.displayName !== null || error.role !== null || error.modelProfile !== null);
  const basicInvalid = companyIdError !== null || nameError !== null;
  const formInvalid = basicInvalid || workspaceRootError !== null || rosterInvalid;

  async function submitCompany(): Promise<void> {
    if (formInvalid) {
      setShowValidation(true);
      if (basicInvalid) setStep(0);
      else if (rosterInvalid) setStep(1);
      else if (workspaceRootError !== null) setStep(2);
      else setStep(4);
      return;
    }
    const trimmedId = companyId.trim();
    try {
      const receipt = await createCompany.mutateAsync({
        id: trimmedId,
        name: name.trim(),
        workspaceRoot: workspaceRoot.trim(),
        roster: roster.map(employee => ({
          id: employee.id,
          displayName: employee.displayName.trim(),
          role: employee.role.trim(),
          modelProfile: employee.modelProfile.trim(),
        })),
        requestId: `company-create-${trimmedId}-${Date.now()}`,
      });
      navigate(`/companies/${receipt.targetId}/overview`);
    } catch {
      // mutation state surfaces the error
    }
  }

  function updateEmployee(employeeId: string, field: keyof EmployeeDraft, value: string): void {
    setRoster(current => current.map(employee => employee.id === employeeId ? {...employee, [field]: value} : employee));
  }

  function handleNameChange(value: string): void {
    setName(value);
    if (!idTouched) setCompanyId(slugifyCompanyId(value));
  }

  function handleCompanyIdChange(value: string): void {
    setIdTouched(true);
    setCompanyId(value);
  }

  return <div className={styles.viewStack} data-od-id="new-company-view">
    <ViewHeader eyebrow="集团 / 公司向导" title="新建公司" />
    <div className={styles.stepper}>{steps.map((label, index) => <button className={styles.stepItem + ' ' + (step === index ? styles.stepItemActive : '') + ' ' + (index < step ? styles.stepItemDone : '')} key={label} onClick={() => setStep(index as WizardStep)} type="button"><span>{String(index + 1).padStart(2, '0')}</span>{label}</button>)}</div>
    <section className={styles.sectionCard} data-od-id="company-creation-step">
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>第 {step + 1} 步 / 5</span><h2 className={styles.sectionTitle}>{steps[step]}</h2></div><StatusBadge label="本地草案" tone="neutral" /></div>
      {step === 0 ? <div className={styles.formStack}>
        <label className={styles.formLabel}>公司 ID<input className={styles.formField} value={companyId} onChange={event => handleCompanyIdChange(event.target.value)} placeholder="例如 content-lab" /></label>
        <p className={styles.formHint}>用于作用域与链接，仅支持字母、数字、下划线和连字符（1-80）。未手动修改时会根据公司名称自动生成。</p>
        {showValidation && companyIdError ? <p className={styles.formError} role="alert">{companyIdError}</p> : null}
        <label className={styles.formLabel}>公司名称<input className={styles.formField} value={name} onChange={event => handleNameChange(event.target.value)} placeholder="例如 Polis 内容实验室" /></label>
        {showValidation && nameError ? <p className={styles.formError} role="alert">{nameError}</p> : null}
        <p className={styles.formHint}>创建会保存固定组织与 4 个逻辑员工，但不会启动使命或调用模型。</p>
      </div> : null}
      {step === 1 ? <div className={styles.formStack}>
        <p className={styles.formHint}>当前产品运行时固定 4 个逻辑员工（ID 不可更改）；可在创建前调整显示名、岗位与模型配置，创建后仍可在公司设置中修改组织配置。</p>
        {roster.map((employee, index) => {
          const errors = rosterErrors[index];
          return <div className={styles.wizardChoice} key={employee.id}>
            <strong>{employee.id}</strong>
            <label className={styles.formLabel}>显示名<input className={styles.formField} value={employee.displayName} onChange={event => updateEmployee(employee.id, 'displayName', event.target.value)} /></label>
            {showValidation && errors?.displayName ? <p className={styles.formError} role="alert">{errors.displayName}</p> : null}
            <label className={styles.formLabel}>岗位<input className={styles.formField} value={employee.role} onChange={event => updateEmployee(employee.id, 'role', event.target.value)} /></label>
            {showValidation && errors?.role ? <p className={styles.formError} role="alert">{errors.role}</p> : null}
            <label className={styles.formLabel}>模型配置<input className={styles.formField} value={employee.modelProfile} onChange={event => updateEmployee(employee.id, 'modelProfile', event.target.value)} /></label>
            {showValidation && errors?.modelProfile ? <p className={styles.formError} role="alert">{errors.modelProfile}</p> : null}
          </div>;
        })}
      </div> : null}
      {step === 2 ? <div className={styles.formStack}>
        <label className={styles.formLabel}>工作区 / 项目目录<input className={styles.formField} value={workspaceRoot} onChange={event => setWorkspaceRoot(event.target.value)} placeholder="例如 C:/workspace/content-lab" /></label>
        {showValidation && workspaceRootError ? <p className={styles.formError} role="alert">{workspaceRootError}</p> : null}
        <p className={styles.formHint}>只保存授权配置，不扫描任意宿主目录、不运行脚本。</p>
        <div className={styles.wizardGrid}>
          <div className={styles.wizardChoice}>
            <Database aria-hidden="true" className={styles.icon} size={18} />
            <strong>技能 / MCP</strong>
            <span>创建公司时不会导入技能、注册 MCP、建立绑定，也不会执行脚本或连接外部服务。</span>
            <span>公司创建完成后，可在 <strong>公司设置 → 能力</strong> 中导入技能清单/库、登记 MCP 服务器、为员工配置能力绑定与工具调用资格。</span>
            <span>集团范围的公共技能与 MCP 定义可在 <strong>集团设置</strong> 中查看与管理（如已接入）。</span>
            <StatusBadge label="创建后配置" tone="info" />
          </div>
          <div className={styles.wizardChoice}>
            <Settings2 aria-hidden="true" className={styles.icon} size={18} />
            <strong>模型与运行时</strong>
            <span>使用现有运行时就绪状态；创建过程不探测模型、不发起真实提供方业务。</span>
            <span>运行时提供方、模型与凭据就绪状态可在创建后于 <strong>公司设置</strong> 中查看与调整（以当前已实现能力为准）。</span>
            <StatusBadge label="不探测 / 不启动" tone="neutral" />
          </div>
        </div>
      </div> : null}
      {step === 3 ? <div className={styles.detailRows}>
        <DataRow label="预算口径" value="现有 tool-call 口径；不伪造美元上限" />
        <DataRow label="保护收尾额" value="由运行时现有语义决定" />
        <DataRow label="外部动作" value="默认关闭；创建不发送、不探测、不发布" />
        <DataRow label="技能 / MCP" value="创建时不绑定；创建后在公司设置 → 能力中配置" />
        <DataRow label="启动状态" value="创建后保持 draft，需单独启动使命" />
      </div> : null}
      {step === 4 ? <div className={styles.formStack}>
        <div className={styles.detailRows}>
          <DataRow label="公司 ID" value={companyId.trim() || '未填写'} />
          <DataRow label="公司名称" value={name.trim() || '未填写'} />
          <DataRow label="工作区目录" value={workspaceRoot.trim() || '未填写'} />
          <DataRow label="外部效果" value="不发送、不探测、不发布、不启动使命" />
          <DataRow label="技能 / MCP" value="未绑定（创建后可在公司设置 → 能力配置）" />
        </div>
        <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>固定员工</span><h2 className={styles.sectionTitle}>{roster.length} 个逻辑员工</h2></div></div>
        <div className={styles.recordList}>{roster.map((employee, index) => {
          const errors = rosterErrors[index];
          const invalid = errors && (errors.displayName || errors.role || errors.modelProfile);
          return <div className={styles.recordRow} key={employee.id}><div className={styles.recordLead}><Building2 aria-hidden="true" size={16} /><div><strong>{employee.displayName.trim() || employee.id}</strong><span>{employee.id} · 岗位 {employee.role.trim() || '未填写'} · 模型 {employee.modelProfile.trim() || '未填写'}</span></div></div><StatusBadge label={invalid ? '需补全' : '就绪'} tone={invalid ? 'warning' : 'success'} /></div>;
        })}</div>
        {formInvalid ? <p className={styles.formError} role="alert">请先补全必填项（公司 ID、名称、工作区目录及员工信息）后再创建。</p> : null}
        {createCompany.isError ? <p className={styles.formError} role="alert">创建失败：{labelErrorMessage(createCompany.error.message)}</p> : null}
      </div> : null}
      <div className={styles.wizardActions}>
        <button className={styles.textButton} disabled={step === 0 || createCompany.isPending} onClick={() => setStep((step - 1) as WizardStep)} type="button">上一步</button>
        {step < 4 ? <button className={styles.commandButton} onClick={() => setStep((step + 1) as WizardStep)} type="button">下一步 <ArrowUpRight aria-hidden="true" size={14} /></button> : <button className={styles.commandButton} disabled={createCompany.isPending} onClick={() => { void submitCompany(); }} type="button">{createCompany.isPending ? '正在创建…' : '创建公司'} <ArrowUpRight aria-hidden="true" size={14} /></button>}
      </div>
    </section>
  </div>;
}

