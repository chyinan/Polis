// pattern: Imperative Shell

import {useEffect, useState, type ReactElement} from 'react';
import {BellRing, Check, FileSearch, MessageSquareWarning} from 'lucide-react';
import type {WorkbenchApi} from '../data/workbench-api';
import {useConfigureNotificationRoute, useNotifications} from '../data/workbench-query';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import {labelErrorMessage} from '../domain/display-labels';
import styles from '../styles/workbench.module.css';

type NotificationSettingsPageProps = Readonly<{api: WorkbenchApi; companyId: string}>;

export function NotificationSettingsPage({api, companyId}: NotificationSettingsPageProps): ReactElement {
  const query = useNotifications(api, companyId);
  const configure = useConfigureNotificationRoute(api, companyId);
  const [destination, setDestination] = useState('local://workbench');
  const [message, setMessage] = useState<string | null>(null);

  useEffect(() => {
    const route = query.data?.routes[0];
    if (!route) return;
    setDestination(route.destination || 'local://workbench');
  }, [query.data]);

  if (query.isPending) return <div className={styles.viewStack}><NotificationHeader /><div className={styles.emptyState} role="status">正在读取通知配置…</div></div>;
  if (query.isError) return <div className={styles.viewStack}><NotificationHeader /><div className={styles.formError} role="alert">读取通知配置失败：{labelErrorMessage(query.error.message)}</div></div>;

  const route = query.data.routes[0] ?? null;
  const deliveryCount = query.data.deliveries.length;

  async function saveDisabledDraft(): Promise<void> {
    const trimmedDestination = destination.trim() || 'local://workbench';
    setMessage(null);
    try {
      await configure.mutateAsync({adapter: 'local', destination: trimmedDestination, enabled: false, requestId: `notification-draft-${companyId}-${Date.now()}`});
      setMessage('已保存本地通知配置草案；当前不会启用外部发送。');
    } catch (error) {
      setMessage(error instanceof Error ? `保存失败：${labelErrorMessage(error.message)}` : '保存通知配置草案失败');
    }
  }

  return <div className={styles.viewStack} data-od-id="notification-settings-view">
    <NotificationHeader action={<StatusBadge label="外部资格未验证" tone="warning" />} />
    <section className={styles.notificationSection}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>通知通道</span><h2 className={styles.sectionTitle}>通道配置</h2></div><BellRing aria-hidden="true" className={styles.icon} size={18} /></div>
      <div className={styles.notificationFormGrid}>
        <label className={styles.notificationField}><span>通知适配器</span><select value="local" disabled><option value="local">本地记录（当前可保存）</option></select></label>
        <label className={styles.notificationField}><span>目标地址</span><input onChange={event => setDestination(event.target.value)} placeholder="例如 local://workbench" value={destination} /></label>
        <label className={styles.notificationField}><span>外部通道</span><select value="qq_official" disabled><option value="qq_official">QQ 官方机器人（未接入）</option></select></label>
        <label className={styles.notificationField}><span>凭据引用</span><input disabled placeholder="需后端凭据管理与授权后配置" value="" readOnly /></label>
        <label className={styles.notificationField}><span>安全别名</span><input disabled placeholder="外部通道接入后配置" value="" readOnly /></label>
        <label className={styles.notificationField}><span>触发策略</span><select defaultValue="handoff" disabled><option value="handoff">人工接管</option><option value="critical">关键阻塞</option></select></label>
      </div>
      <div className={styles.notificationSummary}>
        <NotificationSummaryRow label="当前保存范围" value="本地记录适配器" />
        <NotificationSummaryRow label="目标地址" value={destination.trim() || route?.destination || 'local://workbench'} />
        <NotificationSummaryRow label="当前启用状态" value={route?.enabled ? '已启用' : '未启用'} />
        <NotificationSummaryRow label="外部发送" value="未授权 / 未接入" />
      </div>
      <div className={styles.notificationActions}>
        <button className={styles.textButton} disabled={configure.isPending} onClick={() => { void saveDisabledDraft(); }} type="button">{configure.isPending ? '正在保存…' : '保存本地配置草案'}</button>
        <button className={styles.commandButton} disabled title="需要外部通道资格、凭据与单独授权" type="button">发送测试</button>
        <button className={styles.textButton} onClick={() => setMessage('资格报告未运行：需要单独的 provider/外部账号授权与资格验证。')} type="button">获取资格报告</button>
        <button className={styles.textButton} disabled title="外部通道未验证，当前不可启用" type="button">启用通道</button>
      </div>
      {message ? <p className={styles.notificationNotice} role="status">{message}</p> : null}
      <p className={styles.formHint}>当前仅支持保存本地通知路由配置（目标地址与停用状态草案）。QQ 凭据、外部通道资格与真实发送不会在前端保存或触发；相关能力需在获得授权并完成后端/资格接入后启用。</p>
    </section>
    <section className={styles.notificationSection}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>投递状态</span><h2 className={styles.sectionTitle}>投递状态</h2></div><MessageSquareWarning aria-hidden="true" className={styles.icon} size={18} /></div>
      <div className={styles.notificationStateList}>
        <NotificationStateRow label="人工处理" value="工作台可接管" icon={<Check aria-hidden="true" size={15} />} />
        <NotificationStateRow label="通知投递" value={deliveryCount > 0 ? `${deliveryCount} 条记录` : '暂无记录'} icon={<BellRing aria-hidden="true" size={15} />} />
        <NotificationStateRow label="平台回执" value="按适配器实际返回为准" icon={<FileSearch aria-hidden="true" size={15} />} />
        <NotificationStateRow label="失败降级" value="保留为通知意图/记录" icon={<MessageSquareWarning aria-hidden="true" size={15} />} />
      </div>
    </section>
  </div>;
}

function NotificationHeader({action}: Readonly<{action?: ReactElement}>): ReactElement {
  return <div className={styles.viewHeader}><div><p className={styles.eyebrow}>公司 / 通知</p><h1 className={styles.pageTitle}>对外通知</h1></div>{action ? <div className={styles.viewHeaderAction}>{action}</div> : null}</div>;
}

function NotificationSummaryRow({label, value}: Readonly<{label: string; value: string}>): ReactElement {
  return <div className={styles.notificationSummaryRow}><span>{label}</span><strong>{value}</strong></div>;
}

function NotificationStateRow({icon, label, value}: Readonly<{icon: ReactElement; label: string; value: string}>): ReactElement {
  return <div className={styles.notificationStateRow}><span className={styles.notificationStateLabel}>{icon}{label}</span><strong>{value}</strong></div>;
}
