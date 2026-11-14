// pattern: Imperative Shell

import {useEffect, useMemo, useState, type ReactElement} from 'react';
import {BellRing, Check, FileSearch, Link2, MessageSquareWarning} from 'lucide-react';
import type {WorkbenchApi} from '../data/workbench-api';
import {useConfigureNotificationRoute, useDispatchNotifications, useNotifications, useTestNotification} from '../data/workbench-query';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import {labelDisplayValue, labelErrorMessage, labelNotificationAdapter} from '../domain/display-labels';
import type {NotificationRouteView} from '../domain/workbench';
import styles from '../styles/workbench.module.css';

type NotificationSettingsPageProps = Readonly<{api: WorkbenchApi; companyId: string}>;
type Adapter = 'local' | 'webhook' | 'qq_official';

export function NotificationSettingsPage({api, companyId}: NotificationSettingsPageProps): ReactElement {
  const query = useNotifications(api, companyId);
  const configure = useConfigureNotificationRoute(api, companyId);
  const test = useTestNotification(api, companyId);
  const dispatch = useDispatchNotifications(api, companyId);

  const [adapter, setAdapter] = useState<Adapter>('local');
  const [enabled, setEnabled] = useState(false);
  const [destination, setDestination] = useState('local://workbench');
  const [webhookSecret, setWebhookSecret] = useState('');
  const [clearWebhookSecret, setClearWebhookSecret] = useState(false);
  const [qqAppId, setQqAppId] = useState('');
  const [qqAppSecret, setQqAppSecret] = useState('');
  const [clearQqSecret, setClearQqSecret] = useState(false);
  const [qqTargetKind, setQqTargetKind] = useState<'user' | 'group'>('user');
  const [qqSandbox, setQqSandbox] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [messageTone, setMessageTone] = useState<'status' | 'error'>('status');

  const routesByAdapter = useMemo(() => {
    const map = new Map<Adapter, NotificationRouteView>();
    for (const route of query.data?.routes ?? []) {
      if (route.adapter === 'local' || route.adapter === 'webhook' || route.adapter === 'qq_official') map.set(route.adapter, route);
    }
    return map;
  }, [query.data]);

  useEffect(() => {
    const route = routesByAdapter.get(adapter);
    if (!route) {
      setEnabled(false);
      if (adapter === 'local') setDestination('local://workbench');
      if (adapter === 'webhook') setDestination('');
      if (adapter === 'qq_official') { setDestination(''); setQqAppId(''); setQqTargetKind('user'); setQqSandbox(false); }
      setWebhookSecret('');
      setClearWebhookSecret(false);
      setQqAppSecret('');
      setClearQqSecret(false);
      return;
    }
    setEnabled(route.enabled);
    if (adapter === 'local') {
      setDestination(route.destination || 'local://workbench');
    } else {
      // webhook/qq destinations are redacted in view for safety; require re-entry to change/save target
      setDestination('');
    }
    if (adapter === 'webhook') {
      setWebhookSecret('');
      setClearWebhookSecret(false);
    }
    if (adapter === 'qq_official') {
      const cfg = route.config as {appId?: unknown; targetKind?: unknown; sandbox?: unknown};
      setQqAppId(typeof cfg.appId === 'string' ? cfg.appId : '');
      setQqTargetKind(cfg.targetKind === 'group' ? 'group' : 'user');
      setQqSandbox(cfg.sandbox === true);
      setQqAppSecret('');
      setClearQqSecret(false);
    }
  }, [adapter, routesByAdapter]);

  if (query.isPending) return <div className={styles.viewStack}><NotificationHeader /><div className={styles.emptyState} role="status">正在读取通知配置…</div></div>;
  if (query.isError) return <div className={styles.viewStack}><NotificationHeader /><div className={styles.formError} role="alert">读取通知配置失败：{labelErrorMessage(query.error.message)}</div></div>;

  const currentRoute = routesByAdapter.get(adapter) ?? null;
  const savedEnabled = currentRoute?.enabled ?? false;
  const deliveryCount = query.data.deliveries.length;
  const pendingIntentCount = query.data.pendingIntentCount;
  const canSendTest = !configure.isPending && !test.isPending && !dispatch.isPending && savedEnabled;
  const canDispatch = !configure.isPending && !test.isPending && !dispatch.isPending && pendingIntentCount > 0;

  async function saveRoute(): Promise<void> {
    setMessage(null);
    try {
      const requestId = `notification-route-${companyId}-${adapter}-${Date.now()}`;
      if (adapter === 'local') {
        await configure.mutateAsync({adapter, destination: (destination.trim() || 'local://workbench'), enabled, config: {}, requestId});
      } else if (adapter === 'webhook') {
        const url = destination.trim();
        if (url === '') throw new Error('请填写 Webhook URL（为安全不回显已保存地址）');
        await configure.mutateAsync({adapter, destination: url, enabled, config: {}, secret: webhookSecret.trim() || undefined, clearSecret: clearWebhookSecret || undefined, requestId});
        setWebhookSecret('');
        setClearWebhookSecret(false);
      } else {
        const target = destination.trim();
        const appId = qqAppId.trim();
        if (target === '') throw new Error('请填写目标 OpenID（为安全不回显已保存目标）');
        if (appId === '') throw new Error('请填写 QQ 机器人 AppID');
        await configure.mutateAsync({adapter, destination: target, enabled, config: {appId, targetKind: qqTargetKind, sandbox: qqSandbox}, secret: qqAppSecret.trim() || undefined, clearSecret: clearQqSecret || undefined, requestId});
        setQqAppSecret('');
        setClearQqSecret(false);
      }
      setMessageTone('status');
      setMessage(enabled ? `${labelNotificationAdapter(adapter)} 路由已保存并启用。` : `${labelNotificationAdapter(adapter)} 路由已保存并停用。`);
    } catch (error) {
      setMessageTone('error');
      setMessage(error instanceof Error ? `保存失败：${labelErrorMessage(error.message)}` : '保存通知路由失败');
    }
  }

  async function sendTest(): Promise<void> {
    setMessage(null);
    try {
      const result = await test.mutateAsync({adapter, requestId: `notification-test-${companyId}-${adapter}-${Date.now()}`});
      setMessageTone('status');
      setMessage(`已对 ${labelNotificationAdapter(result.adapter)} 发送测试通知：${labelDisplayValue(result.state)}。`);
    } catch (error) {
      setMessageTone('error');
      setMessage(error instanceof Error ? `发送测试失败：${labelErrorMessage(error.message)}` : '发送测试通知失败');
    }
  }

  async function dispatchPending(): Promise<void> {
    setMessage(null);
    try {
      const result = await dispatch.mutateAsync({requestId: `notification-dispatch-${companyId}-${Date.now()}`});
      setMessageTone('status');
      setMessage(`已处理待发送通知：扫描 ${result.intents} 条，投递尝试 ${result.dispatched} 次（成功 ${result.delivered}，失败 ${result.failed}）。`);
    } catch (error) {
      setMessageTone('error');
      setMessage(error instanceof Error ? `处理待发送通知失败：${labelErrorMessage(error.message)}` : '处理待发送通知失败');
    }
  }

  const secretConfigured = currentRoute?.secretConfigured ?? false;

  return <div className={styles.viewStack} data-od-id="notification-settings-view">
    <NotificationHeader action={<StatusBadge label={savedEnabled ? `${labelNotificationAdapter(adapter)} 已启用` : '未启用'} tone={savedEnabled ? 'success' : 'warning'} />} />
    <section className={styles.notificationSection}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>通知通道</span><h2 className={styles.sectionTitle}>通道配置</h2></div><BellRing aria-hidden="true" className={styles.icon} size={18} /></div>
      <div className={styles.notificationFormGrid}>
        <label className={styles.notificationField}><span>通知适配器</span>
          <select value={adapter} onChange={event => setAdapter(event.target.value as Adapter)}>
            <option value="local">本地记录</option>
            <option value="webhook">Webhook（HTTP）</option>
            <option value="qq_official">QQ 官方机器人</option>
          </select>
        </label>

        {adapter === 'local' ? <>
          <label className={styles.notificationField}><span>目标地址</span><input onChange={event => setDestination(event.target.value)} placeholder="例如 local://workbench" value={destination} /></label>
        </> : null}

        {adapter === 'webhook' ? <>
          <label className={styles.notificationField}><span>Webhook URL</span><input onChange={event => setDestination(event.target.value)} placeholder="https://example.com/webhook（已保存地址不回显）" value={destination} /></label>
          <label className={styles.notificationField}><span>签名密钥（可选，HMAC-SHA256）</span><input autoComplete="off" onChange={event => { setWebhookSecret(event.target.value); setClearWebhookSecret(false); }} placeholder={secretConfigured && !clearWebhookSecret ? '已保存，留空则不修改' : '留空则不使用签名'} type="password" value={webhookSecret} /></label>
          {secretConfigured ? <label className={styles.notificationField}><span>密钥操作</span><label className={styles.formHint}><input checked={clearWebhookSecret} onChange={event => { setClearWebhookSecret(event.target.checked); if (event.target.checked) setWebhookSecret(''); }} type="checkbox" /> 清除已保存密钥</label></label> : null}
        </> : null}

        {adapter === 'qq_official' ? <>
          <label className={styles.notificationField}><span>AppID</span><input onChange={event => setQqAppId(event.target.value)} placeholder="QQ 开放平台机器人 AppID" spellCheck={false} value={qqAppId} /></label>
          <label className={styles.notificationField}><span>AppSecret（写入后不回显）</span><input autoComplete="off" onChange={event => { setQqAppSecret(event.target.value); setClearQqSecret(false); }} placeholder={secretConfigured && !clearQqSecret ? '已保存，留空则不修改' : '请输入 AppSecret'} type="password" value={qqAppSecret} /></label>
          <label className={styles.notificationField}><span>目标类型</span>
            <select value={qqTargetKind} onChange={event => setQqTargetKind(event.target.value as 'user' | 'group')}>
              <option value="user">单聊用户（openid）</option>
              <option value="group">群聊（group_openid）</option>
            </select>
          </label>
          <label className={styles.notificationField}><span>目标 OpenID</span><input onChange={event => setDestination(event.target.value)} placeholder="目标用户/群 OpenID（已保存目标不回显）" spellCheck={false} value={destination} /></label>
          <label className={styles.notificationField}><span>环境</span><label className={styles.formHint}><input checked={qqSandbox} onChange={event => setQqSandbox(event.target.checked)} type="checkbox" /> 使用沙箱接口（sandbox.api.sgroup.qq.com）</label></label>
          {secretConfigured ? <label className={styles.notificationField}><span>密钥操作</span><label className={styles.formHint}><input checked={clearQqSecret} onChange={event => { setClearQqSecret(event.target.checked); if (event.target.checked) setQqAppSecret(''); }} type="checkbox" /> 清除已保存 AppSecret</label></label> : null}
        </> : null}

        <label className={styles.notificationField}><span>启用路由</span><label className={styles.formHint}><input checked={enabled} onChange={event => setEnabled(event.target.checked)} type="checkbox" /> 启用后可用于测试发送（按已保存配置执行）</label></label>
      </div>

      <div className={styles.notificationSummary}>
        <NotificationSummaryRow label="适配器" value={labelNotificationAdapter(adapter)} />
        <NotificationSummaryRow label="已保存状态" value={savedEnabled ? '已启用' : '未启用'} />
        <NotificationSummaryRow label="目标" value={adapter === 'local' ? (destination.trim() || currentRoute?.destination || 'local://workbench') : (currentRoute ? '已保存（不回显）' : '未配置')} />
        <NotificationSummaryRow label="密钥" value={adapter === 'local' ? '不适用' : (secretConfigured ? '已配置' : '未配置')} />
      </div>

      <div className={styles.notificationActions}>
        <button className={styles.commandButton} disabled={configure.isPending || test.isPending || dispatch.isPending} onClick={() => { void saveRoute(); }} type="button">{configure.isPending ? '正在保存…' : '保存路由'}</button>
        <button className={styles.textButton} disabled={!canSendTest} onClick={() => { void sendTest(); }} title={savedEnabled ? `按当前选中的 ${labelNotificationAdapter(adapter)} 路由发送测试通知` : '请先保存并启用当前适配器路由'} type="button">{test.isPending ? '正在发送…' : '发送测试通知'}</button>
        <button className={styles.textButton} disabled={!canDispatch} onClick={() => { void dispatchPending(); }} title={pendingIntentCount > 0 ? '将待发送通知投递到当前已启用的路由' : '当前没有待发送通知意图'} type="button">{dispatch.isPending ? '正在处理…' : `处理待发送通知${pendingIntentCount > 0 ? ` (${pendingIntentCount})` : ''}`}</button>
        <button className={styles.textButton} onClick={() => { setMessageTone('status'); setMessage(adapter === 'qq_official' ? 'QQ 适配器按官方 v2 API 实现：获取 access_token（bots.qq.com）后向 api.sgroup.qq.com 发送消息。主动消息能力/额度受平台权限、目标关系与风控限制，需使用已授权的 AppID/AppSecret 与有效 OpenID。' : adapter === 'webhook' ? 'Webhook 适配器会向配置的 URL 发送 JSON POST；若配置密钥，将附带 HMAC-SHA256 签名头 X-Polis-Signature-256 与时间戳。' : '本地适配器仅记录通知，不发起外部请求。'); }} type="button">适配器说明</button>
      </div>
      {message ? <p className={messageTone === 'error' ? styles.formError : styles.notificationNotice} role={messageTone === 'error' ? 'alert' : 'status'}>{message}</p> : null}
      <p className={styles.formHint}>Webhook/QQ 目标地址与密钥在后端保存；前端不会回显密钥，已保存的敏感目标以脱敏形式展示。测试发送按当前选中且已保存启用的适配器路由执行；“处理待发送通知”会尝试投递系统产生的待发送通知意图。实际到达性取决于目标服务/平台权限、网络与凭据有效性。</p>
    </section>

    <section className={styles.notificationSection}>
      <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>投递状态</span><h2 className={styles.sectionTitle}>投递结果</h2></div><Link2 aria-hidden="true" className={styles.icon} size={18} /></div>
      <div className={styles.notificationStateList}>
        <NotificationStateRow label="待发送意图" value={pendingIntentCount > 0 ? `${pendingIntentCount} 条待处理` : '暂无待处理'} icon={<MessageSquareWarning aria-hidden="true" size={15} />} />
        <NotificationStateRow label="人工处理" value="工作台可接管" icon={<Check aria-hidden="true" size={15} />} />
        <NotificationStateRow label="通知投递" value={deliveryCount > 0 ? `${deliveryCount} 条记录` : '暂无记录'} icon={<BellRing aria-hidden="true" size={15} />} />
        <NotificationStateRow label="平台回执" value="按适配器实际返回为准" icon={<FileSearch aria-hidden="true" size={15} />} />
      </div>
      <div className={styles.recordList}>
        {query.data.deliveries.length === 0 ? <div className={styles.emptyState}>暂无通知投递记录。</div> : query.data.deliveries.map(delivery => <div className={styles.recordRow} key={delivery.deliveryId}><div className={styles.recordLead}><BellRing aria-hidden="true" size={16} /><div><strong>{labelNotificationAdapter(delivery.adapter)} · {labelDisplayValue(delivery.state)}</strong><span>{delivery.deliveryId} · 意图 {delivery.intentId}</span></div></div><div className={styles.recordMeta}><StatusBadge label={delivery.errorCode ?? '无错误'} tone={delivery.errorCode ? 'danger' : delivery.state === 'delivered' ? 'success' : 'warning'} /></div></div>)}
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
