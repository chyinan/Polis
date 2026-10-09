// pattern: Functional Core
import type {ReactElement} from 'react';
import type {GoPatchView} from '../../domain/go-patch';
import styles from '../../styles/workbench.module.css';

/** Read-only projection. UserDisposition remains in the existing review form. */
export function GoPatchPreview(props: Readonly<{patch: GoPatchView; logText: string | null}>): ReactElement {
  const {patch, logText} = props;
  return <section className={styles.formStack} aria-label="Go Patch 人工审核">
    <h3>冻结快照补丁</h3>
    <p className={styles.formHint}>接受补丁只记录人工决定，不会自动应用到源仓库、merge 或部署。</p>
    <dl><dt>base commit / 输入</dt><dd><code>{patch.baseCommitOid} / {patch.inputSha256}</code></dd>
      <dt>候选 revision / digest</dt><dd><code>{patch.candidateRevision} / {patch.candidateSha256}</code></dd>
      <dt>Patch / 重放</dt><dd><code>{patch.patchSha256}</code> · 与候选一致</dd>
      <dt>Go JobRun / result</dt><dd><code>{patch.jobRunId} / {patch.resultSha256}</code></dd>
      <dt>执行范围 / runner</dt><dd><code>{patch.profile} / {patch.runnerSha256}</code></dd>
      <dt>日志 digest</dt><dd><code>{patch.logSha256}</code></dd></dl>
    <ul>{patch.files.map(file => <li key={file.path}><code>{file.path}</code> · {file.action}</li>)}</ul>
    <details open><summary>只读差异{patch.diffTruncated ? '（差异已截断；完整内容在正式交付包）' : ''}</summary><pre className={styles.codePreview}>{patch.diff}</pre></details>
    <p className={styles.formHint}>{patch.scope}</p>
    {logText === null ? <p>暂无已读取的测试日志。</p> : <details><summary>受保护 API 读取的有界日志</summary><pre className={styles.codePreview}>{logText}</pre></details>}
  </section>;
}
