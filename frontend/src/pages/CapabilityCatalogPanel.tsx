// pattern: Imperative Shell

import {useState, type ReactElement} from 'react';
import {Settings2} from 'lucide-react';
import type {WorkbenchApi} from '../data/workbench-api';
import {useApproveSkill, useCapabilityCatalog, useDescribeVisibleTool, useGetMCPToolCallReceipt, useImportSkill, useImportSkillDocument, useImportSkillLibrary, useListMCPToolCallQualifications, useListMCPToolCallReceipts, useListVisibleResources, useListVisibleSkills, useListVisibleTools, useLoadSkill, useQualifyMCP, useReadSkillReference, useRegisterMCP, useRevokeSkill, useUpsertCapabilityBinding} from '../data/workbench-query';
import type {CapabilityCatalogView, MCPServerDefinitionView, MCPToolCallQualificationView, MCPToolCallReceiptView, SkillLoadView, SkillRevisionView, SkillSummaryPage, ToolDescriptorView, ToolSummaryPage, VisibleResourcePage} from '../domain/workbench';
import {labelTransport} from '../domain/display-labels';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import styles from '../styles/workbench.module.css';

const employees = [
  {id: 'emp-planning', label: '规划'},
  {id: 'emp-backend', label: '后端'},
  {id: 'emp-frontend', label: '前端'},
  {id: 'emp-review', label: '评审'},
] as const;

export function CapabilityCatalogPanel({api, companyId, query}: Readonly<{api: WorkbenchApi; companyId: string; query: ReturnType<typeof useCapabilityCatalog>}>): ReactElement {
  const importSkill = useImportSkill(api, companyId);
  const importDocument = useImportSkillDocument(api, companyId);
  const importLibrary = useImportSkillLibrary(api, companyId);
  const registerMCP = useRegisterMCP(api, companyId);
  const qualifyMCP = useQualifyMCP(api, companyId);
  const approveSkill = useApproveSkill(api, companyId);
  const loadSkill = useLoadSkill(api, companyId);
  const listVisible = useListVisibleSkills(api, companyId);
  const listTools = useListVisibleTools(api, companyId);
  const listResources = useListVisibleResources(api, companyId);
  const describeTool = useDescribeVisibleTool(api, companyId);
  const readReference = useReadSkillReference(api, companyId);
  const revokeSkill = useRevokeSkill(api, companyId);
  const saveBinding = useUpsertCapabilityBinding(api, companyId);
  const listCallQualifications = useListMCPToolCallQualifications(api, companyId);
  const listCallReceipts = useListMCPToolCallReceipts(api, companyId);
  const getCallReceipt = useGetMCPToolCallReceipt(api, companyId);
  const [callBindingId, setCallBindingId] = useState('');
  const [callQualifications, setCallQualifications] = useState<ReadonlyArray<MCPToolCallQualificationView>>([]);
  const [callReceipts, setCallReceipts] = useState<ReadonlyArray<MCPToolCallReceiptView>>([]);
  const [callReceiptDetail, setCallReceiptDetail] = useState<MCPToolCallReceiptView | null>(null);
  const [skillPackage, setSkillPackage] = useState('');
  const [skillName, setSkillName] = useState('');
  const [skillRevision, setSkillRevision] = useState('1');
  const [skillSource, setSkillSource] = useState('local://candidate');
  const [skillDigest, setSkillDigest] = useState('');
  const [documentText, setDocumentText] = useState('');
  const [documentPackage, setDocumentPackage] = useState('');
  const [libraryPackage, setLibraryPackage] = useState('brand-guidelines');
  const [loadTask, setLoadTask] = useState('');
  const [loadEmployee, setLoadEmployee] = useState<(typeof employees)[number]['id']>('emp-backend');
  const [loadSkillId, setLoadSkillId] = useState('');
  const [mcpName, setMcpName] = useState('');
  const [mcpTransport, setMcpTransport] = useState<'stdio' | 'streamable_http'>('stdio');
  const [mcpCommand, setMcpCommand] = useState('');
  const [mcpEndpoint, setMcpEndpoint] = useState('');
  const [mcpDigest, setMcpDigest] = useState('');
  const [employeeId, setEmployeeId] = useState<(typeof employees)[number]['id']>('emp-backend');
  const [selectedSkills, setSelectedSkills] = useState<ReadonlyArray<string>>([]);
  const [selectedMCP, setSelectedMCP] = useState<ReadonlyArray<string>>([]);
  const [message, setMessage] = useState<string | null>(null);
  if (query.isPending) return <div className={styles.emptyState} role="status">正在读取能力候选目录</div>;
  if (query.isError) return <div className={styles.errorState} role="alert">能力目录读取失败：{query.error.message}</div>;
  const catalog = query.data;

  async function run(action: () => Promise<void>, success: string): Promise<void> {
    setMessage(null);
    try {
      await action();
      setMessage(success);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '能力目录操作失败');
    }
  }

  return <div className={styles.viewStack}>
    <section className={styles.detailGrid}>
      <article className={styles.sectionCard}>
        <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>本地清单</span><h2 className={styles.sectionTitle}>导入 SKILL.md</h2></div><StatusBadge label="只读校验" tone="info" /></div>
        <div className={styles.formStack}>
          <label className={styles.formLabel}>清单正文<textarea className={styles.formField} onChange={event => setDocumentText(event.target.value)} rows={8} value={documentText} /></label>
          <label className={styles.formLabel}>版本<input className={styles.formField} onChange={event => setSkillRevision(event.target.value)} value={skillRevision} /></label>
          <label className={styles.formLabel}>包标识（名称不是合法标识时填写）<input className={styles.formField} onChange={event => setDocumentPackage(event.target.value)} value={documentPackage} /></label>
          <button className={styles.commandButton} disabled={importDocument.isPending} onClick={() => { void run(() => importDocument.mutateAsync({publisherScope: 'company', packageId: documentPackage, revision: skillRevision, sourceRef: 'operator-paste', document: documentText, requestId: `skill-manifest-${companyId}-${Date.now()}`}).then(() => undefined), '清单已登记为候选。格式已检查，脚本没有执行。'); }} type="button">{importDocument.isPending ? '导入中…' : '导入清单'}</button>
          <label className={styles.formLabel}>技能库中的单个包<input className={styles.formField} onChange={event => setLibraryPackage(event.target.value)} value={libraryPackage} /></label>
          <button className={styles.commandButton} disabled={importLibrary.isPending} onClick={() => { void run(() => importLibrary.mutateAsync({publisherScope: 'company', packageId: libraryPackage, revision: skillRevision, requestId: `skill-library-${companyId}-${Date.now()}`}).then(() => undefined), '已读取这一个技能包。脚本没有执行。'); }} type="button">{importLibrary.isPending ? '读取中…' : '读取这一个技能包'}</button>
        </div>
      </article>
      <article className={styles.sectionCard}>
        <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>手工登记</span><h2 className={styles.sectionTitle}>候选元数据</h2></div></div>
        <div className={styles.formStack}>
          <label className={styles.formLabel}>包标识<input className={styles.formField} onChange={event => setSkillPackage(event.target.value)} value={skillPackage} /></label>
          <label className={styles.formLabel}>显示名称<input className={styles.formField} onChange={event => setSkillName(event.target.value)} value={skillName} /></label>
          <label className={styles.formLabel}>来源引用<input className={styles.formField} onChange={event => setSkillSource(event.target.value)} value={skillSource} /></label>
          <label className={styles.formLabel}>内容指纹<input className={styles.formField} onChange={event => setSkillDigest(event.target.value)} placeholder="sha256:..." value={skillDigest} /></label>
          <button className={styles.commandButton} disabled={importSkill.isPending} onClick={() => { void run(() => importSkill.mutateAsync({publisherScope: 'company', packageId: skillPackage, revision: skillRevision, displayName: skillName, sourceRef: skillSource, contentDigest: skillDigest, requestId: `skill-import-${companyId}-${Date.now()}`}).then(() => undefined), '技能已登记为候选；不会执行脚本或自动安装依赖。'); }} type="button">{importSkill.isPending ? '登记中…' : '登记技能候选'}</button>
        </div>
      </article>
    </section>
    <SkillList approveSkill={approveSkill} catalog={catalog} companyId={companyId} describeTool={describeTool} listResources={listResources} listTools={listTools} listVisible={listVisible} loadEmployee={loadEmployee} loadSkill={loadSkill} loadSkillId={loadSkillId} loadTask={loadTask} readReference={readReference} revokeSkill={revokeSkill} run={run} setLoadEmployee={setLoadEmployee} setLoadSkillId={setLoadSkillId} setLoadTask={setLoadTask} />
    <section className={styles.detailGrid}>
      <article className={styles.sectionCard}>
        <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>MCP 服务定义</span><h2 className={styles.sectionTitle}>候选 MCP</h2></div><StatusBadge label={`${catalog.mcpServers.length} 个`} tone="info" /></div>
        <div className={styles.formStack}>
          <label className={styles.formLabel}>名称<input className={styles.formField} onChange={event => setMcpName(event.target.value)} value={mcpName} /></label>
          <label className={styles.formLabel}>传输方式<select className={styles.formField} onChange={event => setMcpTransport(event.target.value as 'stdio' | 'streamable_http')} value={mcpTransport}><option value="stdio">标准输入输出</option><option value="streamable_http">流式 HTTP</option></select></label>
          {mcpTransport === 'stdio' ? <label className={styles.formLabel}>启动命令<input className={styles.formField} onChange={event => setMcpCommand(event.target.value)} placeholder="仅登记，不会执行" value={mcpCommand} /></label> : <label className={styles.formLabel}>访问地址<input className={styles.formField} onChange={event => setMcpEndpoint(event.target.value)} placeholder="仅登记，不会访问" value={mcpEndpoint} /></label>}
          <label className={styles.formLabel}>描述指纹<input className={styles.formField} onChange={event => setMcpDigest(event.target.value)} placeholder="sha256:..." value={mcpDigest} /></label>
          <button className={styles.commandButton} disabled={registerMCP.isPending} onClick={() => { void run(() => registerMCP.mutateAsync({id: `${mcpName.trim().toLowerCase().replace(/[^a-z0-9_-]+/g, '-') || 'mcp'}-${Date.now()}`, name: mcpName, transport: mcpTransport, command: mcpTransport === 'stdio' ? mcpCommand : null, endpoint: mcpTransport === 'streamable_http' ? mcpEndpoint : null, args: [], descriptorDigest: mcpDigest, requestId: `mcp-register-${companyId}-${Date.now()}`}).then(() => undefined), 'MCP 已登记为未验证；不会启动服务或执行工具发现。'); }} type="button">{registerMCP.isPending ? '登记中…' : '登记 MCP 定义'}</button>
        </div>
        <div className={styles.recordList}>{catalog.mcpServers.length === 0 ? <div className={styles.emptyState}>当前没有 MCP 定义。</div> : catalog.mcpServers.map(server => <MCPRow companyId={companyId} key={server.id} qualifyMCP={qualifyMCP} run={run} server={server} />)}</div>
      </article>
      <article className={styles.sectionCard}>
        <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>员工绑定</span><h2 className={styles.sectionTitle}>能力绑定</h2></div><StatusBadge label={`${catalog.bindings.length} 条`} tone="info" /></div>
        <div className={styles.formStack}>
          <label className={styles.formLabel}>员工<select className={styles.formField} onChange={event => setEmployeeId(event.target.value as (typeof employees)[number]['id'])} value={employeeId}>{employees.map(employee => <option key={employee.id} value={employee.id}>{employee.label}</option>)}</select></label>
          <fieldset className={styles.formStack}>
            <legend className={styles.formLabel}>已批准技能</legend>
            {catalog.skills.filter(skill => skill.status === 'approved').length === 0 ? <p className={styles.formHint}>先批准技能版本，才能放进绑定。</p> : catalog.skills.filter(skill => skill.status === 'approved').map(skill => <label className={styles.formLabel} key={skill.id}><input checked={selectedSkills.includes(skill.id)} onChange={event => setSelectedSkills(toggle(selectedSkills, skill.id, event.target.checked))} type="checkbox" /> {skill.displayName} · {skill.revision}</label>)}
          </fieldset>
          <fieldset className={styles.formStack}>
            <legend className={styles.formLabel}>未连接的 MCP 定义</legend>
            {catalog.mcpServers.filter(server => server.status !== 'revoked').map(server => <label className={styles.formLabel} key={server.id}><input checked={selectedMCP.includes(server.id)} onChange={event => setSelectedMCP(toggle(selectedMCP, server.id, event.target.checked))} type="checkbox" /> {server.name}</label>)}
          </fieldset>
          <button className={styles.commandButton} disabled={saveBinding.isPending} onClick={() => { void run(() => saveBinding.mutateAsync({employeeId, skillRevisionIds: selectedSkills, mcpDefinitionIds: selectedMCP, status: 'draft', requestId: `binding-draft-${companyId}-${Date.now()}`}).then(() => undefined), '绑定已保存为草案。连接状态保持未配置。'); }} type="button">保存草案</button>
          <button className={styles.commandButton} disabled={saveBinding.isPending} onClick={() => { void run(() => saveBinding.mutateAsync({employeeId, skillRevisionIds: selectedSkills, mcpDefinitionIds: selectedMCP, status: 'approved', requestId: `binding-approve-${companyId}-${Date.now()}`}).then(() => undefined), '绑定已批准。这不会启动 MCP，也不会执行技能脚本。'); }} type="button">{saveBinding.isPending ? '保存中…' : '批准绑定'}</button>
        </div>
        <div className={styles.recordList}>{catalog.bindings.length === 0 ? <div className={styles.emptyState}>当前没有能力绑定。</div> : catalog.bindings.map(binding => <div className={styles.recordRow} key={binding.id}><div className={styles.recordLead}><Settings2 aria-hidden="true" size={16} /><div><strong>{employeeLabel(binding.employeeId)}</strong><span>{binding.skillRevisionIds.length} 个技能 · {binding.mcpDefinitionIds.length} 个 MCP · 未连接</span></div></div><StatusBadge label={bindingStatus(binding.status)} tone={binding.status === 'approved' ? 'success' : binding.status === 'revoked' ? 'danger' : 'warning'} /></div>)}</div>
      </article>
      <article className={styles.sectionCard}>
        <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>R0.15</span><h2 className={styles.sectionTitle}>工具调用资格与回执（只读，默认不执行）</h2></div><StatusBadge label="read-only" tone="info" /></div>
        <p className={styles.formHint}>仅展示已登记的 `tools.call` 资格与审计回执；不提供调用/执行按钮。外部 MCP/OAuth/网络 egress 未授权。</p>
        <div className={styles.formStack}>
          <label className={styles.formLabel}>绑定（可选）
            <select className={styles.formField} onChange={event => setCallBindingId(event.target.value)} value={callBindingId}>
              <option value="">全部（按公司）</option>
              {catalog.bindings.map(binding => <option key={binding.id} value={binding.id}>{employeeLabel(binding.employeeId)} · {binding.id}</option>)}
            </select>
          </label>
          <div style={{display: 'flex', gap: 8, flexWrap: 'wrap'}}>
            <button className={styles.commandButton} disabled={listCallQualifications.isPending} onClick={() => { void run(async () => { const items = await listCallQualifications.mutateAsync({bindingId: callBindingId || undefined, requestId: `call-qual-list-${companyId}-${Date.now()}`}); setCallQualifications(items); }, '已加载工具调用资格（只读）。'); }} type="button">{listCallQualifications.isPending ? '加载中…' : '加载资格'}</button>
            <button className={styles.commandButton} disabled={listCallReceipts.isPending} onClick={() => { void run(async () => { const items = await listCallReceipts.mutateAsync({bindingId: callBindingId || undefined, requestId: `call-receipt-list-${companyId}-${Date.now()}`}); setCallReceipts(items); }, '已加载工具调用回执（只读）。'); }} type="button">{listCallReceipts.isPending ? '加载中…' : '加载回执'}</button>
          </div>
        </div>
        {callQualifications.length > 0 ? <div className={styles.recordList}>{callQualifications.map(q => <div className={styles.recordRow} key={q.id}><div className={styles.recordLead}><div><strong>{q.handle}</strong><span>{q.effectClass} · {q.transport} · digest {q.descriptorDigest.slice(0, 18)}… · policy {q.policyRevision}</span></div></div><StatusBadge label={q.status} tone={q.status === 'approved' ? 'success' : q.status === 'revoked' || q.status === 'disabled' ? 'danger' : 'warning'} /></div>)}</div> : <div className={styles.emptyState}>未加载资格或暂无记录。</div>}
        {callReceipts.length > 0 ? <div className={styles.recordList}>{callReceipts.map(r => <div className={styles.recordRow} key={r.id}><div className={styles.recordLead}><div><strong>{r.handle}</strong><span>{r.outcome}{r.code ? ` · ${r.code}` : ''} · {r.effectClass} · {r.createdAt}</span></div></div><div style={{display:'flex',gap:8,alignItems:'center'}}><button className={styles.commandButton} disabled={getCallReceipt.isPending} onClick={() => { void run(async () => { const detail = await getCallReceipt.mutateAsync({receiptId: r.id, requestId: `call-receipt-${r.id}-${Date.now()}`}); setCallReceiptDetail(detail); }, '已加载回执详情（只读、脱敏）。'); }} type="button">详情</button><StatusBadge label={r.outcome} tone={r.outcome === 'completed' ? 'success' : r.outcome === 'denied' ? 'warning' : 'danger'} /></div></div>)}</div> : <div className={styles.emptyState}>未加载回执或暂无记录。</div>}
        {callReceiptDetail ? <div className={styles.recordList}><div className={styles.recordRow}><div className={styles.recordLead}><div><strong>回执详情 {callReceiptDetail.id}</strong><span>{callReceiptDetail.handle} · {callReceiptDetail.outcome}{callReceiptDetail.code ? ` · ${callReceiptDetail.code}` : ''} · req {callReceiptDetail.requestDigest.slice(0,18)}… · args {callReceiptDetail.argumentsDigest.slice(0,18)}…</span></div></div><button className={styles.commandButton} onClick={() => setCallReceiptDetail(null)} type="button">关闭</button></div><pre className={styles.formHint} style={{whiteSpace:'pre-wrap',margin:0}}>{JSON.stringify({resultJson: callReceiptDetail.resultJson ?? null, errorJson: callReceiptDetail.errorJson ?? null, processObserved: callReceiptDetail.processObserved, egressObserved: callReceiptDetail.egressObserved}, null, 2)}</pre></div> : null}
      </article>
    </section>
    {message ? <div className={styles.operationNotice} role="status">{message}</div> : null}
    <p className={styles.formHint}>导入、批准、绑定、连接和执行仍然分开。本页不扫描任意路径，不启动进程，不访问地址，也不执行技能脚本。</p>
  </div>;
}

function SkillList({approveSkill, catalog, companyId, describeTool, listResources, listTools, listVisible, loadEmployee, loadSkill, loadSkillId, loadTask, readReference, revokeSkill, run, setLoadEmployee, setLoadSkillId, setLoadTask}: Readonly<{approveSkill: ReturnType<typeof useApproveSkill>; catalog: CapabilityCatalogView; companyId: string; describeTool: ReturnType<typeof useDescribeVisibleTool>; listResources: ReturnType<typeof useListVisibleResources>; listTools: ReturnType<typeof useListVisibleTools>; listVisible: ReturnType<typeof useListVisibleSkills>; loadEmployee: (typeof employees)[number]['id']; loadSkill: ReturnType<typeof useLoadSkill>; loadSkillId: string; loadTask: string; readReference: ReturnType<typeof useReadSkillReference>; revokeSkill: ReturnType<typeof useRevokeSkill>; run: (action: () => Promise<void>, success: string) => Promise<void>; setLoadEmployee: (value: (typeof employees)[number]['id']) => void; setLoadSkillId: (value: string) => void; setLoadTask: (value: string) => void}>): ReactElement {
  const approved = catalog.skills.filter(skill => skill.status === 'approved');
  const servers = catalog.mcpServers.filter(server => server.status !== 'revoked');
  const [summaries, setSummaries] = useState<SkillSummaryPage | null>(null);
  const [tools, setTools] = useState<ToolSummaryPage | null>(null);
  const [resourceServer, setResourceServer] = useState('');
  const [resources, setResources] = useState<VisibleResourcePage | null>(null);
  return <section className={styles.sectionCard}>
    <div className={styles.sectionHeader}><div><span className={styles.cardEyebrow}>技能版本目录</span><h2 className={styles.sectionTitle}>技能版本</h2></div><StatusBadge label={`${catalog.skills.length} 个`} tone="info" /></div>
    <div className={styles.formStack}>
      <label className={styles.formLabel}>任务标识<input className={styles.formField} onChange={event => setLoadTask(event.target.value)} value={loadTask} /></label>
      <label className={styles.formLabel}>员工<select className={styles.formField} onChange={event => setLoadEmployee(event.target.value as (typeof employees)[number]['id'])} value={loadEmployee}>{employees.map(employee => <option key={employee.id} value={employee.id}>{employee.label}</option>)}</select></label>
      <label className={styles.formLabel}>已批准技能<select className={styles.formField} onChange={event => setLoadSkillId(event.target.value)} value={loadSkillId}><option value="">选择技能</option>{approved.map(skill => <option key={skill.id} value={skill.id}>{skill.displayName} · {skill.revision}</option>)}</select></label>
      <button className={styles.commandButton} disabled={loadSkill.isPending || loadSkillId === '' || loadTask.trim() === ''} onClick={() => { void run(() => loadSkill.mutateAsync({taskId: loadTask.trim(), employeeId: loadEmployee, skillId: loadSkillId, requestId: `skill-load-${companyId}-${Date.now()}`}).then(() => undefined), '已把技能正文载入该任务。没有执行脚本，也没有新增权限。'); }} type="button">{loadSkill.isPending ? '载入中…' : '载入正文'}</button>
      <button className={styles.commandButton} disabled={listVisible.isPending} onClick={() => { void run(() => listVisible.mutateAsync({employeeId: loadEmployee, cursor: '', limit: 32, requestId: `skill-list-${companyId}-${Date.now()}`}).then(page => { setSummaries(page); }), '已列出该员工已批准绑定中的技能摘要。没有返回正文，也没有执行脚本。'); }} type="button">{listVisible.isPending ? '列出中…' : '查看可见摘要'}</button>
      <button className={styles.commandButton} disabled={listTools.isPending} onClick={() => { void run(() => listTools.mutateAsync({employeeId: loadEmployee, cursor: '', limit: 32, requestId: `tool-list-${companyId}-${Date.now()}`}).then(page => { setTools(page); }), '已列出缓存中的工具名称。没有连接服务，也没有调用工具。'); }} type="button">{listTools.isPending ? '列出中…' : '查看可见工具'}</button>
      <label className={styles.formLabel}>已绑定服务<select className={styles.formField} onChange={event => setResourceServer(event.target.value)} value={resourceServer}><option value="">选择服务</option>{servers.map(server => <option key={server.id} value={server.id}>{server.name}</option>)}</select></label>
      <button className={styles.commandButton} disabled={listResources.isPending || resourceServer === ''} onClick={() => { void run(() => listResources.mutateAsync({employeeId: loadEmployee, serverId: resourceServer, cursor: '', limit: 32, requestId: `resource-list-${companyId}-${Date.now()}`}).then(page => { setResources(page); }), '已列出该服务缓存中的资源名称。没有连接服务，也没有读取资源。'); }} type="button">{listResources.isPending ? '列出中…' : '查看可见资源'}</button>
    </div>
    {summaries === null ? null : <div className={styles.recordList}>{summaries.items.length === 0 ? <div className={styles.emptyState}>当前员工没有可见的已批准技能。</div> : summaries.items.map(item => <div className={styles.recordRow} key={item.id}><div className={styles.recordLead}><Settings2 aria-hidden="true" size={16} /><div><strong>{item.displayName}</strong><span>{item.purpose} · {item.revision} · 未执行</span></div></div><StatusBadge label={item.executionStatus === 'unverified' ? '未执行' : item.executionStatus} tone="info" /></div>)}{summaries.nextCursor !== null ? <button className={styles.commandButton} disabled={listVisible.isPending} onClick={() => { const cursor = summaries.nextCursor ?? ''; void run(() => listVisible.mutateAsync({employeeId: loadEmployee, cursor, limit: 32, requestId: `skill-list-${companyId}-${Date.now()}`}).then(page => { setSummaries(page); }), '已列出下一页技能摘要。'); }} type="button">下一页</button> : null}</div>}
    {tools === null ? null : <div className={styles.recordList}>{tools.items.length === 0 ? <div className={styles.emptyState}>当前员工没有已缓存的可见工具。</div> : tools.items.map(item => <ToolSummaryRow companyId={companyId} describeTool={describeTool} employeeId={loadEmployee} handle={item.handle} key={item.handle} run={run} serverName={item.serverName} toolName={item.toolName} />)}</div>}
    {resources === null ? null : <div className={styles.recordList}>{resources.items.length === 0 ? <div className={styles.emptyState}>当前服务没有已缓存的可见资源。</div> : resources.items.map(item => <div className={styles.recordRow} key={item.uri}><div className={styles.recordLead}><Settings2 aria-hidden="true" size={16} /><div><strong>{item.name}</strong><span>{item.uri} · 未读取</span></div></div><StatusBadge label="未读取" tone="info" /></div>)}</div>}
    <div className={styles.recordList}>{catalog.skills.length === 0 ? <div className={styles.emptyState}>当前没有技能版本。</div> : catalog.skills.map(skill => <SkillRow approveSkill={approveSkill} companyId={companyId} key={skill.id} revokeSkill={revokeSkill} run={run} skill={skill} />)}</div>
    <div className={styles.recordList}>{catalog.loads.length === 0 ? <div className={styles.emptyState}>当前任务还没有载入技能。</div> : catalog.loads.map(load => <LoadRow companyId={companyId} key={load.id} load={load} readReference={readReference} run={run} />)}</div>
  </section>;
}

function ToolSummaryRow({companyId, describeTool, employeeId, handle, run, serverName, toolName}: Readonly<{companyId: string; describeTool: ReturnType<typeof useDescribeVisibleTool>; employeeId: (typeof employees)[number]['id']; handle: string; run: (action: () => Promise<void>, success: string) => Promise<void>; serverName: string; toolName: string}>): ReactElement {
  const [described, setDescribed] = useState<ToolDescriptorView | null>(null);
  const schema = described === null ? '' : JSON.stringify(described.inputSchema);
  return <div className={styles.recordRow}>
    <div className={styles.recordLead}><Settings2 aria-hidden="true" size={16} /><div><strong>{toolName}</strong><span>{serverName} · 未调用</span>{described !== null ? <span>{described.description} · {schema}</span> : null}</div></div>
    <div className={styles.recordMeta}>
      <button className={styles.commandButton} disabled={describeTool.isPending} onClick={() => { void run(() => describeTool.mutateAsync({employeeId, handle, requestId: `tool-describe-${companyId}-${Date.now()}`}).then(item => { setDescribed(item); }), '已读取冻结的工具描述。没有连接服务，也没有调用工具。'); }} type="button">{describeTool.isPending ? '读取中…' : '查看描述'}</button>
      <StatusBadge label="未调用" tone="info" />
    </div>
  </div>;
}

function LoadRow({companyId, load, readReference, run}: Readonly<{companyId: string; load: SkillLoadView; readReference: ReturnType<typeof useReadSkillReference>; run: (action: () => Promise<void>, success: string) => Promise<void>}>): ReactElement {
  const [reference, setReference] = useState(load.references[0] ?? '');
  const [body, setBody] = useState('');
  return <div className={styles.recordRow}>
    <div className={styles.recordLead}><Settings2 aria-hidden="true" size={16} /><div><strong>{load.taskId}</strong><span>{employeeLabel(load.employeeId)} · {load.contentDigest} · 未执行</span>{body !== '' ? <span>{body}</span> : null}</div></div>
    <div className={styles.recordMeta}>
      {load.references.length > 0 ? <label className={styles.formLabel}>引用<select className={styles.formField} onChange={event => setReference(event.target.value)} value={reference}>{load.references.map(item => <option key={item} value={item}>{item}</option>)}</select></label> : null}
      <button className={styles.commandButton} disabled={readReference.isPending || reference === ''} onClick={() => { void run(() => readReference.mutateAsync({taskId: load.taskId, employeeId: load.employeeId, skillId: load.skillRevisionId, reference, requestId: `skill-reference-${companyId}-${Date.now()}`}).then(item => { setBody(item.body); }), '已读取冻结的引用正文。没有执行脚本。'); }} type="button">{readReference.isPending ? '读取中…' : '读取引用'}</button>
      <StatusBadge label={load.repeated ? '重复载入' : '已载入'} tone="info" />
    </div>
  </div>;
}

function SkillRow({approveSkill, companyId, revokeSkill, run, skill}: Readonly<{approveSkill: ReturnType<typeof useApproveSkill>; companyId: string; revokeSkill: ReturnType<typeof useRevokeSkill>; run: (action: () => Promise<void>, success: string) => Promise<void>; skill: SkillRevisionView}>): ReactElement {
  return <div className={styles.recordRow}>
    <div className={styles.recordLead}><Settings2 aria-hidden="true" size={16} /><div><strong>{skill.displayName}</strong><span>{skill.packageId} · {skill.revision} · {qualificationText(skill.manifest)}</span></div></div>
    <div className={styles.recordMeta}>
      {skill.status === 'candidate' ? <button className={styles.commandButton} disabled={approveSkill.isPending} onClick={() => { void run(() => approveSkill.mutateAsync({skillId: skill.id, requestId: `skill-approve-${companyId}-${Date.now()}`}).then(() => undefined), '技能版本已批准。批准只改变目录状态。'); }} type="button">批准</button> : null}
      {skill.status !== 'revoked' ? <button className={styles.commandButton} disabled={revokeSkill.isPending} onClick={() => { void run(() => revokeSkill.mutateAsync({skillId: skill.id, requestId: `skill-revoke-${companyId}-${Date.now()}`}).then(() => undefined), '技能版本已撤销，不能再进入新的绑定。'); }} type="button">撤销</button> : null}
      <StatusBadge label={skill.status === 'candidate' ? '候选' : skill.status === 'approved' ? '已批准' : '已撤销'} tone={skill.status === 'approved' ? 'success' : skill.status === 'revoked' ? 'danger' : 'warning'} />
    </div>
  </div>;
}

function MCPRow({companyId, qualifyMCP, run, server}: Readonly<{companyId: string; qualifyMCP: ReturnType<typeof useQualifyMCP>; run: (action: () => Promise<void>, success: string) => Promise<void>; server: MCPServerDefinitionView}>): ReactElement {
  const probed = probeSummary(server.qualification);
  return <div className={styles.recordRow}><div className={styles.recordLead}><Settings2 aria-hidden="true" size={16} /><div><strong>{server.name}</strong><span>{labelTransport(server.transport)} · {server.command ?? server.endpoint ?? '未配置目标'} · {probed}</span></div></div><div className={styles.recordMeta}>{server.transport === 'stdio' && server.status !== 'revoked' && server.status !== 'approved' ? <button className={styles.commandButton} disabled={qualifyMCP.isPending} onClick={() => { void run(() => qualifyMCP.mutateAsync({mcpId: server.id, requestId: `mcp-qualify-${companyId}-${Date.now()}`}).then(() => undefined), '已完成工具目录探测，没有调用任何工具。'); }} type="button">检查连接</button> : null}<StatusBadge label={server.status === 'unverified' ? '未验证' : server.status === 'candidate' ? '候选' : server.status === 'approved' ? '已批准' : '已撤销'} tone={server.status === 'approved' ? 'success' : server.status === 'revoked' ? 'danger' : 'warning'} /></div></div>;
}

function probeSummary(qualification: unknown): string {
  if (!isRecord(qualification) || qualification.probeStatus !== 'passed' || !Array.isArray(qualification.toolNames)) return '未探测';
  return `已探测 ${qualification.toolNames.length} 个工具，未调用`;
}

function toggle(current: ReadonlyArray<string>, id: string, checked: boolean): ReadonlyArray<string> {
  return checked ? [...current, id] : current.filter(item => item !== id);
}

function qualificationText(manifest: unknown): string {
  if (!isRecord(manifest) || !isRecord(manifest.qualification)) return '执行未验证';
  const format = manifest.qualification.formatStatus === 'supported' ? '格式已检查' : '格式待核对';
  const scripts = manifest.qualification.scriptsPresent === true ? '发现脚本但未执行' : '未执行脚本';
  return `${format} · ${scripts}`;
}

function bindingStatus(status: string): string {
  if (status === 'draft') return '草案';
  if (status === 'approved') return '已批准';
  if (status === 'disabled') return '已停用';
  if (status === 'revoked') return '已撤销';
  return status;
}

function employeeLabel(id: string): string {
  return employees.find(employee => employee.id === id)?.label ?? id;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
