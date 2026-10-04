export const MAX_WORKSPACE_PATCH_BYTES = 64 * 1024;
export const MAX_WORKSPACE_SNAPSHOT_BYTES = 4096;

export type WorkspacePatchApplication = Readonly<{
  content: string;
  addedLines: number;
  removedLines: number;
}>;

type WorkspaceText = Readonly<{
  lines: ReadonlyArray<string>;
  lineEnding: '\n' | '\r\n';
  hasTrailingNewline: boolean;
}>;

type PatchHunk = Readonly<{
  oldIndex: number;
  oldCount: number;
  newStart: number;
  newCount: number;
  oldLines: ReadonlyArray<string>;
  newLines: ReadonlyArray<string>;
  addedLines: number;
  removedLines: number;
}>;

function splitWorkspaceText(value: string, label: string): WorkspaceText {
  const lineEnding: '\n' | '\r\n' = value.includes('\r\n') ? '\r\n' : '\n';
  const withoutCRLF = value.replace(/\r\n/g, '');
  if (withoutCRLF.includes('\r') || (lineEnding === '\r\n' && withoutCRLF.includes('\n'))) {
    throw new Error(`${label} 使用了不支持的混合或裸 CR 换行。`);
  }
  const normalized = value.replace(/\r\n/g, '\n');
  const hasTrailingNewline = normalized.endsWith('\n');
  const lines = normalized === '' ? [] : normalized.split('\n');
  if (hasTrailingNewline) lines.pop();
  return {lines, lineEnding, hasTrailingNewline};
}

function parseHunkHeader(line: string): Readonly<{oldStart: number; oldCount: number; newStart: number; newCount: number}> | null {
  const match = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(?: .*)?$/.exec(line);
  if (match === null) return null;
  const oldStart = Number(match[1]);
  const oldCount = match[2] === undefined ? 1 : Number(match[2]);
  const newStart = Number(match[3]);
  const newCount = match[4] === undefined ? 1 : Number(match[4]);
  if (![oldStart, oldCount, newStart, newCount].every(Number.isSafeInteger)) return null;
  return {oldStart, oldCount, newStart, newCount};
}

function parseHunks(lines: ReadonlyArray<string>, firstHunk: number): ReadonlyArray<PatchHunk> {
  const hunks: PatchHunk[] = [];
  let index = firstHunk;
  while (index < lines.length) {
    const header = parseHunkHeader(lines[index] ?? '');
    if (header === null) throw new Error('补丁包含无法识别的 hunk 或额外文件数据。');
    const oldIndex = header.oldCount === 0 ? header.oldStart : header.oldStart - 1;
    if (oldIndex < 0) throw new Error('补丁包含无效的基线行号。');
    index += 1;
    let oldSeen = 0;
    let newSeen = 0;
    let addedLines = 0;
    let removedLines = 0;
    const oldLines: string[] = [];
    const newLines: string[] = [];
    while (oldSeen < header.oldCount || newSeen < header.newCount) {
      const line = lines[index];
      if (line === undefined || line.startsWith('\\')) throw new Error('补丁 hunk 不完整或依赖缺失行尾标记。');
      const marker = line[0];
      const content = line.slice(1);
      if (marker === ' ') {
        oldLines.push(content);
        newLines.push(content);
        oldSeen += 1;
        newSeen += 1;
      } else if (marker === '-') {
        oldLines.push(content);
        oldSeen += 1;
        removedLines += 1;
      } else if (marker === '+') {
        newLines.push(content);
        newSeen += 1;
        addedLines += 1;
      } else {
        throw new Error('补丁 hunk 包含无效行标记。');
      }
      if (oldSeen > header.oldCount || newSeen > header.newCount) throw new Error('补丁 hunk 行数与声明不一致。');
      index += 1;
    }
    hunks.push({
      oldIndex,
      oldCount: header.oldCount,
      newStart: header.newStart,
      newCount: header.newCount,
      oldLines,
      newLines,
      addedLines,
      removedLines,
    });
  }
  if (hunks.length === 0) throw new Error('补丁中没有可应用的修改。');
  if (!hunks.some(hunk => hunk.addedLines > 0 || hunk.removedLines > 0)) throw new Error('补丁没有实际内容变更。');
  return hunks;
}

export function applyWorkspaceTextPatch(baseText: string, patchText: string): WorkspacePatchApplication {
  if (new TextEncoder().encode(patchText).length > MAX_WORKSPACE_PATCH_BYTES) {
    throw new Error(`补丁超过 ${MAX_WORKSPACE_PATCH_BYTES} 字节上限。`);
  }
  const base = splitWorkspaceText(baseText, '冻结工作区');
  const normalizedPatch = patchText.replace(/\r\n/g, '\n');
  if (normalizedPatch.includes('\r')) throw new Error('补丁含有无效的裸 CR 字符。');
  const patchLines = normalizedPatch.split('\n');
  if (patchLines.at(-1) === '') patchLines.pop();
  let firstHeader = 0;
  if (patchLines[0]?.startsWith('diff --git ')) {
    if (patchLines[0] !== 'diff --git a/workspace.txt b/workspace.txt') throw new Error('补丁只能修改 workspace.txt。');
    firstHeader += 1;
    if (patchLines[firstHeader]?.startsWith('index ')) firstHeader += 1;
  }
  if (patchLines[firstHeader] !== '--- a/workspace.txt' || patchLines[firstHeader + 1] !== '+++ b/workspace.txt') {
    throw new Error('补丁必须明确限定为 a/workspace.txt 到 b/workspace.txt。');
  }
  const hunks = parseHunks(patchLines, firstHeader + 2);
  let baseCursor = 0;
  const outputLines: string[] = [];
  let addedLines = 0;
  let removedLines = 0;
  for (const hunk of hunks) {
    if (hunk.oldIndex < baseCursor || hunk.oldIndex > base.lines.length) throw new Error('补丁 hunk 与冻结基线范围不匹配。');
    outputLines.push(...base.lines.slice(baseCursor, hunk.oldIndex));
    const actualOldLines = base.lines.slice(hunk.oldIndex, hunk.oldIndex + hunk.oldCount);
    if (actualOldLines.length !== hunk.oldCount || actualOldLines.some((line, lineIndex) => line !== hunk.oldLines[lineIndex])) {
      throw new Error('补丁上下文与冻结 workspace.txt 基线不匹配；未修改候选内容。');
    }
    outputLines.push(...hunk.newLines);
    baseCursor = hunk.oldIndex + hunk.oldCount;
    const expectedNewStart = hunk.newCount === 0 ? outputLines.length : outputLines.length - hunk.newCount + 1;
    if (hunk.newStart !== expectedNewStart) throw new Error('补丁新行号与应用结果不一致。');
    addedLines += hunk.addedLines;
    removedLines += hunk.removedLines;
  }
  outputLines.push(...base.lines.slice(baseCursor));
  const normalizedContent = outputLines.join('\n') + (base.hasTrailingNewline ? '\n' : '');
  const content = base.lineEnding === '\r\n' ? normalizedContent.replace(/\n/g, '\r\n') : normalizedContent;
  if (new TextEncoder().encode(content).length > MAX_WORKSPACE_SNAPSHOT_BYTES) {
    throw new Error(`补丁结果超过 ${MAX_WORKSPACE_SNAPSHOT_BYTES} 字节工作区上限。`);
  }
  return {content, addedLines, removedLines};
}
