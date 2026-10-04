// pattern: Imperative Shell

import {useRef, useState, type ReactElement} from 'react';
import type {MissionInputView} from '../domain/mission-input';
import type {ResearchSimulationRunView} from '../domain/workbench';
import type {WorkbenchApi} from '../data/workbench-api';
import {useMissionInputs, useRunResearchSimulation} from '../data/workbench-query';
import {StatusBadge} from '../components/status-badge/StatusBadge';
import styles from '../styles/workbench.module.css';

type ResearchSimulationPanelProps = Readonly<{
  api: WorkbenchApi;
  companyId: string;
  missionId: string;
  runs: ReadonlyArray<ResearchSimulationRunView>;
}>;

const MAX_DATASET_BYTES = 1_048_576;
const MAX_METHOD_BYTES = 16_384;
const MAX_RISK_BUDGET_UNITS = 5_120_000;

function pendingRequestIdentity(pendingIds: Map<string, string>, payload: unknown): Readonly<{key: string; requestId: string}> {
  const key = JSON.stringify({operation: 'research-simulation', payload});
  const requestId = pendingIds.get(key) ?? `research-simulation-${crypto.randomUUID()}`;
  pendingIds.set(key, requestId);
  return {key, requestId};
}

function isUsableSimulationInput(input: MissionInputView, maximumBytes: number): boolean {
  const byteSize = Number(input.byteSize);
  return input.state === 'usable'
    && (input.mediaType === 'application/json' || input.mediaType.startsWith('text/'))
    && Number.isSafeInteger(byteSize) && byteSize > 0 && byteSize <= maximumBytes;
}

function inputOptionLabel(input: MissionInputView): string {
  return `${input.inputId}@${input.revision} · ${input.displayName} · ${input.mediaType} · SHA-256 ${input.contentDigest.slice(0, 12)}`;
}

function inputKey(input: MissionInputView): string {
  return JSON.stringify([input.inputId, input.revision]);
}

type Props = ResearchSimulationPanelProps;

export function ResearchSimulationPanel({api, companyId, missionId, runs}: Props): ReactElement {
  const inputsQuery = useMissionInputs(api, companyId, missionId);
  const runSimulation = useRunResearchSimulation(api, companyId);
  const [datasetInputKey, setDatasetInputKey] = useState('');
  const [methodInputKey, setMethodInputKey] = useState('');
  const [seed, setSeed] = useState('42');
  const [controlDefinition, setControlDefinition] = useState('');
  const [riskBudgetUnits, setRiskBudgetUnits] = useState('128');
  const [message, setMessage] = useState<string | null>(null);
  const pendingRequestIds = useRef(new Map<string, string>());

  const allInputs = inputsQuery.data ?? [];
  const datasets = allInputs.filter(input => isUsableSimulationInput(input, MAX_DATASET_BYTES));
  const methods = allInputs.filter(input => isUsableSimulationInput(input, MAX_METHOD_BYTES));
  const selectedDataset = datasets.find(input => inputKey(input) === datasetInputKey) ?? null;
  const selectedMethod = methods.find(input => inputKey(input) === methodInputKey) ?? null;

  async function submit(): Promise<void> {
    setMessage(null);
    const budget = Number(riskBudgetUnits);
    if (selectedDataset === null || selectedMethod === null) {
      setMessage('请从当前 Mission 输入中选择一个可用数据集和一个可用方法。');
      return;
    }
    if (!/^(0|[1-9]\d{0,19})$/.test(seed) || BigInt(seed) > 18446744073709551615n) {
      setMessage('Seed 必须是无符号 64 位整数。');
      return;
    }
    if (!Number.isSafeInteger(budget) || budget <= 0 || budget > MAX_RISK_BUDGET_UNITS) {
      setMessage(`风险预算必须在 1 到 ${MAX_RISK_BUDGET_UNITS} 次样本抽取之间。`);
      return;
    }
    if (controlDefinition.trim().length === 0 || Array.from(controlDefinition.trim()).length > 512) {
      setMessage('请用 1 到 512 个字符描述对照组。');
      return;
    }
    try {
      const payload = {
        datasetInputId: selectedDataset.inputId,
        datasetInputRevision: selectedDataset.revision,
        methodInputId: selectedMethod.inputId,
        methodInputRevision: selectedMethod.revision,
        seed,
        controlDefinition: controlDefinition.trim(),
        riskBudgetUnits: budget,
      };
      const pending = pendingRequestIdentity(pendingRequestIds.current, payload);
      const run = await runSimulation.mutateAsync({...payload, requestId: pending.requestId});
      pendingRequestIds.current.delete(pending.key);
      setMessage(`模拟记录 ${run.runId} 已保存。领域资格仍为 not_run，执行仍保持关闭。`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '研究模拟失败。');
    }
  }

  return <section className={styles.sectionCard} aria-labelledby="research-simulation-title">
    <div className={styles.sectionHeader}>
      <div><span className={styles.cardEyebrow}>R3 / 确定性研究模拟</span><h2 className={styles.sectionTitle} id="research-simulation-title">研究模拟</h2></div>
      <StatusBadge label="仅模拟" tone="warning" />
    </div>
    <p className={styles.formHint}>此功能只运行有界、确定性的 bootstrap 计算并记录估计值；不会调用模型、发布结果、访问账户、推断研究结论、授予领域资格或开放执行。</p>
    <div className={styles.formStack}>
      <label className={styles.formLabel}>数据集 MissionInput
        <select className={styles.formField} value={datasetInputKey} onChange={event => setDatasetInputKey(event.target.value)}>
          <option value="">选择当前 Mission 中的输入</option>
          {datasets.map(input => <option key={inputKey(input)} value={inputKey(input)}>{inputOptionLabel(input)}</option>)}
        </select>
      </label>
      <label className={styles.formLabel}>方法 MissionInput
        <select className={styles.formField} value={methodInputKey} onChange={event => setMethodInputKey(event.target.value)}>
          <option value="">选择已固定版本的方法输入</option>
          {methods.map(input => <option key={inputKey(input)} value={inputKey(input)}>{inputOptionLabel(input)}</option>)}
        </select>
      </label>
      <div className={styles.detailRows}>
        <label className={styles.formLabel}>随机种子 Seed
          <input className={styles.formField} inputMode="numeric" value={seed} onChange={event => setSeed(event.target.value)} />
        </label>
        <label className={styles.formLabel}>风险预算（样本抽取次数）
          <input className={styles.formField} inputMode="numeric" value={riskBudgetUnits} onChange={event => setRiskBudgetUnits(event.target.value)} />
        </label>
      </div>
      <label className={styles.formLabel}>对照组定义
        <input className={styles.formField} maxLength={512} value={controlDefinition} onChange={event => setControlDefinition(event.target.value)} />
      </label>
      <div className={styles.formHint}>数据集 JSON：<code>{'{"control":[1,2,3],"treatment":[2,3,4]}'}</code>。方法 JSON：<code>{'{"schemaVersion":"polis-research-method@1","algorithm":"bootstrap-mean-difference@1","iterations":16,"sampleSize":4}'}</code>。数据集上限 1 MiB；方法上限 16 KiB。</div>
      {inputsQuery.isError ? <div className={styles.errorState} role="alert">当前 Mission 输入读取失败：{inputsQuery.error.message}</div> : null}
      {missionId === '' ? <div className={styles.formHint}>请先创建或选择 Mission，再选择固定版本的输入。</div> : null}
      <button className={styles.commandButton} type="button" disabled={api.mode !== 'real' || runSimulation.isPending || selectedDataset === null || selectedMethod === null || controlDefinition.trim() === ''} onClick={() => { void submit(); }}>
        {runSimulation.isPending ? '正在运行有界模拟…' : '运行确定性模拟'}
      </button>
      {api.mode !== 'real' ? <div className={styles.formHint}>Fixture 模式为只读；模拟命令需要通过认证的公司账本。</div> : null}
      {message !== null ? <div className={styles.operationNotice} role="status">{message}</div> : null}
    </div>
    <div className={styles.detailRows}>
      <div><span>领域资格</span><strong>not_run</strong></div>
      <div><span>执行状态</span><strong>关闭</strong></div>
    </div>
    <h3 className={styles.sectionTitle}>已保存的模拟记录</h3>
    {runs.length === 0 ? <div className={styles.emptyState}>此公司尚无研究模拟记录。</div> : <div className={styles.recordList}>
      {runs.slice(0, 10).map(run => <article className={styles.recordRow} key={run.runId}>
        <div className={styles.recordLead}><div><strong>{run.runId}</strong><span>{run.createdAt} · 种子 {run.seed} · 使用 {run.riskConsumedUnits}/{run.riskBudgetUnits} 次样本抽取</span><span>数据集 {run.datasetInputId}@{run.datasetInputRevision} · {run.datasetSha256}</span><span>方法 {run.methodInputId}@{run.methodInputRevision} · {run.methodSha256}</span><span>对照组：{run.controlDefinition}</span></div></div>
        <div className={styles.recordMeta}><StatusBadge label="仅估计值" tone="info" /><span>均值差 {String(run.output.meanDifference)}</span><span>范围 {String(run.output.minimumDifference)} 至 {String(run.output.maximumDifference)}</span><span>结果 SHA-256 {run.outputSha256}</span></div>
      </article>)}
    </div>}
  </section>;
}
