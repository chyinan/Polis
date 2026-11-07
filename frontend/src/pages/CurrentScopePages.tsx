// pattern: Imperative Shell

import {useState, type ReactElement} from 'react';
import {Activity, Building2, CheckCircle2, CircleAlert, LockKeyhole, Settings2, ShieldCheck, UsersRound} from 'lucide-react';
import type {WorkbenchApi} from '../data/workbench-api';
import {useCapabilityCatalog, useCompanyList, useCompanyOverview, useRuntimeSettings} from '../data/workbench-query';
import type {AttentionItem, CompanySummaryView, RuntimeSettingsView} from '../domain/workbench';
import {labelActivityKind, labelActivityText, labelDisplayValue, labelErrorMessage} from '../domain/display-labels';
import {CapabilityCatalogPanel} from './CapabilityCatalogPanel';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import styles from '../styles/workbench.module.css';

type ScopePageProps = Readonly<{api: WorkbenchApi; companyId: string}>;

export function FeedbackPage({api, companyId}: ScopePageProps): ReactElement {
  const query = useCompanyOverview(api, companyId);
  if (query.isPending) return <ScopeLoading title="反馈待办" label="正在读取待处理事项" />;
  if (query.isError) return <ScopeError title="反馈待办" message={query.error.message} />;
  const overview = query.data;
  const attention = overview.attention;
  const recentActivity = overview.recentActivity.slice(0, 8);
  return <div className={styles.viewStack} data-od-id="feedback-view">
    <ScopeHeader eyebrow="公司 / 反馈" title="反馈待办" action={<StatusBadge label={`${attention.length} 项待处理`} tone={attention.length > 0 ? 'warning' : 'success'} />} />
    <section className={styles.detailGrid}>
      <article className={styles.sectionCard}>
        <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>人工介入</span><h2 className={styles.sectionTitle}>需要人处理</h2></div><CircleAlert aria-hidden="true" className={styles.icon} size={18} /></div>
        {attention.length === 0 ? <div className={styles.emptyState}><CheckCircle2 aria-hidden="true" size={18} /><strong>当前没有待处理事项</strong><span>系统没有把普通活动或日志伪装成反馈。</span></div> : <div className={styles.recordList}>{attention.map(item => <AttentionRow item={item} key={item.id} />)}</div>}
      </article>
      <article className={styles.sectionCard}>
        <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>活动信号</span><h2 className={styles.sectionTitle}>最近反馈信号</h2></div><Activity aria-hidden="true" className={styles.icon} size={18} /></div>
        {recentActivity.length === 0 ? <div className={styles.emptyState}><span>当前快照没有最近活动。</span></div> : <div className={styles.recordList}>{recentActivity.map(event => <div className={styles.recordRow} key={event.id}><div className={styles.recordLead}><Activity aria-hidden="true" size={16} /><div><strong>{labelActivityText(event.summary)}</strong><span>{labelActivityKind(event.kind)} · {labelActivityText(event.subject.label)}</span></div></div><StatusBadge label={labelDisplayValue(event.tone)} tone={event.tone === 'success' ? 'success' : event.tone === 'danger' ? 'danger' : 'info'} /></div>)}</div>}
      </article>
    </section>
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>处理边界</span><h2 className={styles.sectionTitle}>反馈处理边界</h2></div><ShieldCheck aria-hidden="true" className={styles.icon} size={18} /></div>
      <div className={styles.detailRows}><ScopeRow label="事实来源" value="当前公司的权威总览快照" /><ScopeRow label="处理动作" value="在工作台中查看和处理，不自动执行外部动作" /><ScopeRow label="外部通知" value="QQ / Webhook 需要单独授权与资格验证" /><ScopeRow label="当前状态" value={labelDisplayValue(overview.meta.freshness)} /></div>
    </section>
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
  const [tab, setTab] = useState<'runtime' | 'capabilities' | 'safety'>('runtime');
  if (query.isPending) return <ScopeLoading title="集团设置" label="正在读取当前 runtime readiness" />;
  if (query.isError) return <ScopeError title="集团设置" message={query.error.message} />;
  const runtime = query.data;
  return <div className={styles.viewStack} data-od-id="group-settings-view">
    <ScopeHeader eyebrow="集团 / 设置" title="集团设置" action={<StatusBadge label={readinessLabel(runtime.runtimeReadiness)} tone={runtime.runtimeReadiness === 'ready' ? 'success' : 'warning'} />} />
    <div className={styles.tabBar} role="tablist"><button aria-selected={tab === 'runtime'} className={`${styles.tabButton} ${tab === 'runtime' ? styles.tabButtonActive : ''}`} onClick={() => setTab('runtime')} role="tab" type="button">运行时就绪</button><button aria-selected={tab === 'capabilities'} className={`${styles.tabButton} ${tab === 'capabilities' ? styles.tabButtonActive : ''}`} onClick={() => setTab('capabilities')} role="tab" type="button">技能 / MCP</button><button aria-selected={tab === 'safety'} className={`${styles.tabButton} ${tab === 'safety' ? styles.tabButtonActive : ''}`} onClick={() => setTab('safety')} role="tab" type="button">安全边界</button></div>
    {tab === 'runtime' ? <RuntimeReadinessPanel runtime={runtime} /> : tab === 'capabilities' ? <CapabilityCatalogPanel api={api} companyId={companyId} query={catalogQuery} /> : <SafetyPanel runtime={runtime} />}
  </div>;
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

function AttentionRow({item}: Readonly<{item: AttentionItem}>): ReactElement {
  return <div className={styles.recordRow}><div className={styles.recordLead}><CircleAlert aria-hidden="true" size={16} /><div><strong>{item.title}</strong><span>{item.subject.label} · {item.description}</span></div></div><StatusBadge label={labelDisplayValue(item.tone)} tone={item.tone === 'danger' ? 'danger' : item.tone === 'warning' ? 'warning' : 'info'} /></div>;
}

function ScopeRow({label, value}: Readonly<{label: string; value: string}>): ReactElement {
  return <div className={styles.detailRow}><span className={styles.fieldLabel}>{label}</span><span className={styles.detailValue}>{value}</span></div>;
}

function readinessLabel(value: string): string {
  const labels: Record<string, string> = {ready: '已就绪', not_required: '无需认证', configured: '已配置', missing: '缺少配置', invalid: '配置无效', restart_required: '需要重启', unavailable: '不可用', deferred: '已延期', not_applicable: '不适用', reported: '已报告', unavailable_data: '数据不可得'};
  return labels[value] ?? value;
}
