// pattern: Imperative Shell

import {useEffect, useState, type ReactElement} from 'react';
import {BellRing, Check, FileSearch, MessageSquareWarning} from 'lucide-react';
import type {WorkbenchApi} from '../data/workbench-api';
import {useConfigureNotificationRoute, useNotifications, useTestNotification} from '../data/workbench-query';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import {labelDisplayValue, labelErrorMessage, labelNotificationAdapter} from '../domain/display-labels';
import styles from '../styles/workbench.module.css';

type NotificationSettingsPageProps = Readonly<{api: WorkbenchApi; companyId: string}>;

export function NotificationSettingsPage({api, companyId}: NotificationSettingsPageProps): ReactElement {
  const query = useNotifications(api, companyId);
  const configure = useConfigureNotificationRoute(api, companyId);
  const test = useTestNotification(api, companyId);
  const [destination, setDestination] = useState('local://workbench');
  const [enabled, setEnabled] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [messageTone, setMessageTone] = useState<'status' | 'error'>('status');

  useEffect(() => {
    const route = query.data?.routes[0];
    if (!route) return;
    setDestination(route.destination || 'local://workbench');
    setEnabled(route.enabled);
  }, [query.data]);

  if (query.isPending) return <div className={styles.viewStack}><NotificationHeader /><div className={styles.emptyState} role="status">正在读取通知配置…</div></div>;
  if (query.isError) return <div className={styles.viewStack}><NotificationHeader /><div className={styles.formError} role="alert">读取通知配置失败：{labelErrorMessage(query.error.message)}</div></div>;

  const route = query.data.routes[0] ?? null;
  const deliveryCount = query.data.deliveries.length;
  const savedEnabled = route?.enabled ?? false;
  const canSendTest = !configure.isPending && !test.isPending && savedEnabled;

  async function saveRoute(): Promise<void> {
    const trimmedDestination = destination.trim() || 'local://workbench';
    setMessage(null);
    try {
      await configure.mutateAsync({adapter: 'local', destination: trimmedDestination, enabled, requestId: `notification-route-${companyId}-${Date.now()}`});
      setMessageTone('status');
      setMessage(enabled ? '本地通知路由已保存并启用（仅本地记录，不会外部发送）。' : '本地通知路由已保存并停用。');
    } catch (error) {
      setMessageTone('error');
      setMessage(error instanceof Error ? `保存失败：${labelErrorMessage(error.message)}` : '保存通知路由失败');
    }
  }

  async function sendTest(): Promise<void> {
    setMessage(null);
    try {
      await test.mutateAsync({requestId: `notification-test-${companyId}-${Date.now()}`});
      setMessageTone('status');
      setMessage('已创建本地测试通知投递记录。');
    } catch (error) {
      setMessageTone('error');
      setMessage(error instanceof Error ? `发送测试失败：${labelErrorMessage(error.message)}` : '发送测试通知失败');
    }
  }

  return <div className={styles.viewStack} data-od-id="notification-settings-view">
    <NotificationHeader action={<StatusBadge label={savedEnabled ? '本地路由已启用' : '本地路由未启用'} tone={savedEnabled ? 'success' : 'warning'} />} />
    <section className={styles.notificationSection}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>通知通道</span><h2 className={styles.sectionTitle}>通道配置</h2></div><BellRing aria-hidden="true" className={styles.icon} size={18} /></div>
      <div className={styles.notificationFormGrid}>
        <label className={styles.notificationField}><span>通知适配器</span><select value="local" disabled><option value="local">本地记录（当前可用）</option></select></label>
        <label className={styles.notificationField}><span>目标地址</span><input onChange={event => setDestination(event.target.value)} placeholder="例如 local://workbench" value={destination} /></label>
        <label className={styles.notificationField}><span>启用本地路由</span><label className={styles.formHint}><input checked={enabled} onChange={event => setEnabled(event.target.checked)} type="checkbox" /> 启用后可创建本地投递记录</label></label>
        <label className={styles.notificationField}><span>外部通道</span><select value="qq_official" disabled><option value="qq_official">QQ 官方机器人（未接入）</option></select></label>
        <label className={styles.notificationField}><span>凭据引用</span><input disabled placeholder="需后端凭据管理与授权后配置" value="" readOnly /></label>
        <label className={styles.notificationField}><span>触发策略</span><select defaultValue="handoff" disabled><option value="handoff">人工接管</option><option value="critical">关键阻塞</option></select></label>
      </div>
      <div className={styles.notificationSummary}>
        <NotificationSummaryRow label="当前保存范围" value="本地记录适配器" />
        <NotificationSummaryRow label="目标地址" value={destination.trim() || route?.destination || 'local://workbench'} />
        <NotificationSummaryRow label="已保存状态" value={savedEnabled ? '已启用' : '未启用'} />
        <NotificationSummaryRow label="外部发送" value="未授权 / 未接入" />
      </div>
      <div className={styles.notificationActions}>
        <button className={styles.commandButton} disabled={configure.isPending || test.isPending} onClick={() => { void saveRoute(); }} type="button">{configure.isPending ? '正在保存…' : '保存本地路由'}</button>
        <button className={styles.textButton} disabled={!canSendTest} onClick={() => { void sendTest(); }} title={savedEnabled ? '创建一条本地测试投递记录' : '请先保存并启用本地路由'} type="button">{test.isPending ? '正在发送…' : '发送测试通知'}</button>
        <button className={styles.textButton} onClick={() => { setMessageTone('status'); setMessage('当前仅支持本地通知记录。外部通道资格/凭据与真实发送需要单独接入与授权。'); }} type="button">查看外部通道说明</button>
      </div>
      {message ? <p className={messageTone === 'error' ? styles.formError : styles.notificationNotice} role={messageTone === 'error' ? 'alert' : 'status'}>{message}</p> : null}
      <p className={styles.formHint}>当前支持保存并启用本地通知路由，以及创建本地测试投递记录。不会触发 QQ 或其他外部账号发送；外部通道需要凭据、资格验证与后端适配后才能启用。</p>
    </section>
    <section className={styles.notificationSection}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>投递状态</span><h2 className={styles.sectionTitle}>投递结果</h2></div><MessageSquareWarning aria-hidden="true" className={styles.icon} size={18} /></div>
      <div className={styles.notificationStateList}>
        <NotificationStateRow label="人工处理" value="工作台可接管" icon={<Check aria-hidden="true" size={15} />} />
        <NotificationStateRow label="通知投递" value={deliveryCount > 0 ? `${deliveryCount} 条记录` : '暂无记录'} icon={<BellRing aria-hidden="true" size={15} />} />
        <NotificationStateRow label="平台回执" value="按适配器实际返回为准" icon={<FileSearch aria-hidden="true" size={15} />} />
        <NotificationStateRow label="失败降级" value="保留为通知意图/记录" icon={<MessageSquareWarning aria-hidden="true" size={15} />} />
      </div>
      <div className={styles.recordList}>
        {query.data.deliveries.length === 0 ? <div className={styles.emptyState}>暂无通知投递记录。</div> : query.data.deliveries.map(delivery => <div className={styles.recordRow} key={delivery.deliveryId}><div className={styles.recordLead}><BellRing aria-hidden="true" size={16} /><div><strong>{labelNotificationAdapter(delivery.adapter)} · {labelDisplayValue(delivery.state)}</strong><span>{delivery.deliveryId} · 意图 {delivery.intentId}</span></div></div><div className={styles.recordMeta}><StatusBadge label={delivery.errorCode ?? '无错误'} tone={delivery.errorCode ? 'danger' : 'success'} /></div></div>)}
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
