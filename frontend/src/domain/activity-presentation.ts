// pattern: Functional Core

import type {ActivityEventKind, StatusTone} from './workbench';
import {labelActivityKind} from './display-labels';

export type ActivityPresentation = Readonly<{
  label: string;
  symbol: string;
  tone: StatusTone;
}>;

type ActivityFilter = 'all' | 'collaboration' | 'lifecycle' | 'evidence';

const ACTIVITY_PRESENTATIONS: Readonly<Record<ActivityEventKind, ActivityPresentation>> = {
  mission_created: {label: '使命已创建', symbol: '＋', tone: 'info'},
  mission_started: {label: '使命已启动', symbol: '▶', tone: 'success'},
  mission_cancelled: {label: '使命已取消', symbol: '■', tone: 'neutral'},
  provider_turn_completed: {label: '模型回合已完成', symbol: '✓', tone: 'success'},
  provider_turn_failed: {label: '模型回合失败', symbol: '!', tone: 'danger'},
  provider_runtime_initialization_failed: {label: '运行时初始化失败', symbol: '!', tone: 'danger'},
  provider_runtime_thread_start_failed: {label: '运行线程启动失败', symbol: '!', tone: 'danger'},
  employee_started_task: {label: '开始任务', symbol: '▶', tone: 'info'},
  contract_revision_proposed: {label: '合同修订提议', symbol: '◇', tone: 'info'},
  contract_revision_accepted: {label: '合同修订接受', symbol: '✓', tone: 'success'},
  peer_message_sent: {label: '同事消息发送', symbol: '→', tone: 'info'},
  obligation_created: {label: '责任创建', symbol: '!', tone: 'warning'},
  obligation_observed: {label: '责任已观察', symbol: '◌', tone: 'info'},
  workspace_updated: {label: '工作区更新', symbol: '↗', tone: 'info'},
  checkpoint_saved: {label: '检查点已保存', symbol: '□', tone: 'info'},
  employee_stopped: {label: '员工停止', symbol: '■', tone: 'neutral'},
  successor_resumed: {label: '接班恢复', symbol: '↻', tone: 'success'},
  old_writer_rejected: {label: '旧写入者被拒绝', symbol: '⊘', tone: 'danger'},
  artifact_submitted: {label: '产物提交', symbol: '▣', tone: 'info'},
  acceptance_passed: {label: '验收通过', symbol: '✓', tone: 'success'},
};

export function getActivityPresentation(kind: ActivityEventKind): ActivityPresentation {
  return ACTIVITY_PRESENTATIONS[kind] ?? {label: labelActivityKind(kind), symbol: '•', tone: 'info'};
}

export function formatEventTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return '时间不可用';
  }
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
    timeZone: 'Asia/Shanghai',
    timeZoneName: 'short',
  }).format(date);
}

export function formatEntityId(id: string): string {
  if (id.length <= 18) {
    return id;
  }
  return `${id.slice(0, 8)}…${id.slice(-6)}`;
}

export function filterActivityEvents(
  events: ReadonlyArray<import('./workbench').ActivityEvent>,
  filter: ActivityFilter,
): Array<import('./workbench').ActivityEvent> {
  if (filter === 'all') {
    return [...events];
  }
  const kindsByFilter: Readonly<Record<ActivityFilter, ReadonlyArray<import('./workbench').ActivityEventKind>>> = {
    all: [],
    collaboration: ['peer_message_sent', 'obligation_created', 'obligation_observed'],
    lifecycle: ['mission_created', 'mission_started', 'mission_cancelled', 'provider_turn_completed', 'provider_turn_failed', 'provider_runtime_initialization_failed', 'provider_runtime_thread_start_failed', 'employee_started_task', 'employee_stopped', 'successor_resumed', 'old_writer_rejected'],
    evidence: ['contract_revision_proposed', 'contract_revision_accepted', 'workspace_updated', 'checkpoint_saved', 'artifact_submitted', 'acceptance_passed'],
  };
  const allowedKinds = kindsByFilter[filter];
  return events.filter(event => allowedKinds.includes(event.kind));
}
