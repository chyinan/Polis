// pattern: Imperative Shell

import {useRef, useState, type ReactElement} from 'react';
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
  const [target, setTarget] = useState('');
  const [secretRef, setSecretRef] = useState('');
  const [safetyAlias, setSafetyAlias] = useState('');
  const [message, setMessage] = useState<string | null>(null);
  const pendingRequest = useRef<Readonly<{fingerprint: string; requestId: string}> | null>(null);

  if (query.isPending) return <div className={styles.viewStack}><NotificationHeader /><div className={styles.emptyState} role="status">正在读取通知配置…</div></div>;
  if (query.isError) return <div className={styles.viewStack}><NotificationHeader /><div className={styles.formError} role="alert">读取通知配置失败：{labelErrorMessage(query.error.message)}</div></div>;

  const route = query.data.routes.find(item => item.adapter === 'qq_official') ?? null;
  const deliveryCount = query.data.deliveries.length;

  async function saveDisabledDraft(): Promise<void> {
    setMessage(null);
    const fingerprint = JSON.stringify({target: target.trim(), secretRef: secretRef.trim(), safetyAlias: safetyAlias.trim()});
    const pending = pendingRequest.current?.fingerprint === fingerprint
      ? pendingRequest.current
      : {fingerprint, requestId: `notification-draft-${companyId}-${crypto.randomUUID()}`};
    pendingRequest.current = pending;
    try {
      await configure.mutateAsync({adapter: 'qq_official', destination: target.trim(), safetyAlias: safetyAlias.trim(), credentialRef: secretRef.trim(), enabled: false, requestId: pending.requestId});
      pendingRequest.current = null;
      setMessage('已保存禁用草案；当前不会产生外部发送。');
    } catch (error) {
      const detail = error instanceof Error ? error.message : '命令结果未知';
      setMessage(`禁用通知草案保存结果尚未确认：${detail} 已刷新通知状态；核对后同一草案重试会复用原请求 ID。`);
    }
  }

  return <div className={styles.viewStack} data-od-id="notification-settings-view">
    <NotificationHeader action={<StatusBadge label="资格未验证" tone="warning" />} />
    <section className={styles.notificationSection}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>通知通道</span><h2 className={styles.sectionTitle}>通道配置</h2></div><BellRing aria-hidden="true" className={styles.icon} size={18} /></div>
      <div className={styles.notificationFormGrid}>
        <label className={styles.notificationField}><span>通知适配器</span><select value="qq_official" onChange={() => undefined}><option value="qq_official">QQ 官方机器人</option></select></label>
        <label className={styles.notificationField}><span>接收目标</span><input data-testid="qq-notification-target" onChange={event => setTarget(event.target.value)} placeholder="填写目标标识" value={target} /></label>
        <label className={styles.notificationField}><span>凭据引用</span><input data-testid="qq-notification-credential-ref" onChange={event => setSecretRef(event.target.value)} placeholder="SecretRef 名称" value={secretRef} /></label>
        <label className={styles.notificationField}><span>安全别名</span><input data-testid="qq-notification-alias" onChange={event => setSafetyAlias(event.target.value)} placeholder="例如：研发一组" value={safetyAlias} /></label>
        <label className={styles.notificationField}><span>外发级别</span><select defaultValue="handoff"><option value="handoff">仅需接管</option><option disabled value="critical">严重故障</option></select></label>
      </div>
      <div className={styles.notificationSummary}>
        <NotificationSummaryRow label="当前适配器" value="QQ 官方机器人" />
        <NotificationSummaryRow label="接收路由" value={target || route?.destination || '管理员目标'} />
        <NotificationSummaryRow label="发送状态" value={route?.enabled ? '已启用' : '未启用'} />
        <NotificationSummaryRow label={'\u8d44\u683c\u72b6\u6001'} value={route?.qualificationStatus ?? 'unverified'} />
        <NotificationSummaryRow label="降级行为" value="保留在工作台" />
      </div>
      <div className={styles.notificationActions}>
        <button data-testid="qq-notification-save" className={styles.textButton} disabled={configure.isPending} onClick={() => { void saveDisabledDraft(); }} type="button">{configure.isPending ? '保存中…' : '保存禁用草案'}</button>
        <button data-testid="qq-notification-test" className={styles.commandButton} disabled title="需要独立授权后才能发送测试消息" type="button">发送测试</button>
        <button className={styles.textButton} onClick={() => setMessage('资格报告未运行：需要单独的 provider/外部账号授权。')} type="button">获取资格报告</button>
        <button className={styles.textButton} disabled title="资格验证完成前不可启用" type="button">启用提醒</button>
      </div>
      {message ? <p className={styles.notificationNotice} data-testid="qq-route-save-status" role="status">{message}</p> : null}
      <p className={styles.formHint}>当前只保存本地配置草案；QQ 凭据不会进入 React，也不会在未授权时产生外部发送。</p>
    </section>
    <section className={styles.notificationSection}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>投递状态</span><h2 className={styles.sectionTitle}>投递状态</h2></div><MessageSquareWarning aria-hidden="true" className={styles.icon} size={18} /></div>
      <div className={styles.notificationStateList}>
        <NotificationStateRow label="人工介入" value="工作台待决事项" icon={<Check aria-hidden="true" size={15} />} />
        <NotificationStateRow label="通知投递" value={deliveryCount > 0 ? `${deliveryCount} 条记录` : '未创建'} icon={<BellRing aria-hidden="true" size={15} />} />
        <NotificationStateRow label="平台回执" value="无" icon={<FileSearch aria-hidden="true" size={15} />} />
        <NotificationStateRow label="失败降级" value="不递归通知" icon={<MessageSquareWarning aria-hidden="true" size={15} />} />
      </div>
    </section>
  </div>;
}

function NotificationHeader({action}: Readonly<{action?: ReactElement}>): ReactElement {
  return <div className={styles.viewHeader}><div><p className={styles.eyebrow}>公司 / 设置</p><h1 className={styles.pageTitle}>接管通知</h1></div>{action ? <div className={styles.viewHeaderAction}>{action}</div> : null}</div>;
}

function NotificationSummaryRow({label, value}: Readonly<{label: string; value: string}>): ReactElement {
  return <div className={styles.notificationSummaryRow}><span>{label}</span><strong>{value}</strong></div>;
}

function NotificationStateRow({icon, label, value}: Readonly<{icon: ReactElement; label: string; value: string}>): ReactElement {
  return <div className={styles.notificationStateRow}><span className={styles.notificationStateLabel}>{icon}{label}</span><strong>{value}</strong></div>;
}
