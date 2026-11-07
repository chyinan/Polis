// pattern: Functional Core

import {describe, expect, it} from 'vitest';
import {isAcceptanceContract, validateActivityView, validateCompanyList, validateCompanyOverview, validateMissionCommandReceipt, validateRuntimeSettings} from './workbench-validation';

const companyId = 'company-1';

const meta = {
  schemaVersion: 2,
  companyId,
  entityRevision: '9007199254740993',
  snapshotCursor: 'cursor-1',
  observedAt: '2026-09-14T00:00:00Z',
  dataMode: 'simulated' as const,
  freshness: 'fresh' as const,
  sourceLabel: 'fixture',
  recoveryState: 'operational' as const,
};

const event = {
  id: 'event-1',
  companySeq: '1',
  occurredAt: '2026-09-14T00:00:00Z',
  kind: 'checkpoint_saved' as const,
  actor: {kind: 'system' as const, id: 'controller', label: '控制面'},
  subject: {kind: 'checkpoint', id: 'checkpoint-1', label: 'checkpoint'},
  summary: 'checkpoint 已保存',
  detail: 'evidence',
  tone: 'info' as const,
  evidenceRefs: ['checkpoint-1'],
  metadata: {workspace_revision: '2'},
};

describe('Workbench view validation', () => {
  it('accepts only public, bounded Mission acceptance criteria and documented identity placeholders', () => {
    expect(isAcceptanceContract({revision: 'text-acceptance@1', required_text: ['Mission ID: {{mission_id}}', 'Task ID: {{task_id}}']})).toBe(true);
    expect(isAcceptanceContract({revision: 'text-acceptance@1', required_text: ['{{employee_id}}']})).toBe(false);
    expect(isAcceptanceContract({revision: 'text-acceptance@1', required_text: ['  criterion  ']})).toBe(false);
  });

  it('accepts a scoped activity view and preserves null page termination', () => {
    const result = validateActivityView({meta, items: [event], nextCursor: null}, companyId);

    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.value.meta.entityRevision).toBe('9007199254740993');
      expect(result.value.nextCursor).toBeNull();
    }
  });

  it('accepts provider initialization failure as a distinct runtime event', () => {
    const result = validateActivityView({meta, items: [{...event, kind: 'provider_runtime_initialization_failed', tone: 'danger'}], nextCursor: null}, companyId);

    expect(result.success).toBe(true);
  });

  it('rejects a response from another company scope', () => {
    const result = validateActivityView({meta: {...meta, companyId: 'company-2'}, items: [event], nextCursor: null}, companyId);

    expect(result.success).toBe(false);
  });

  it('rejects a numeric next cursor instead of coercing it', () => {
    const result = validateActivityView({meta, items: [event], nextCursor: 2}, companyId);

    expect(result.success).toBe(false);
  });

  it('rejects a blank next cursor instead of treating it as a page', () => {
    const result = validateActivityView({meta, items: [event], nextCursor: '   '}, companyId);

    expect(result.success).toBe(false);
  });

  it('rejects an unknown activity kind and tone', () => {
    const result = validateActivityView({meta, items: [{...event, kind: 'future_event', tone: 'success-ish'}], nextCursor: null}, companyId);

    expect(result.success).toBe(false);
  });

  it('requires the overview to carry the same company identity as its meta', () => {
    const result = validateCompanyOverview({
      meta,
      company: {companyId, name: 'Polis', description: 'fixture'},
      mission: {
        missionId: 'mission-1',
        title: 'mission',
        goal: 'goal',
        state: 'active',
        contract: 'contract@1',
        acceptanceContract: null,
        currentContractRevision: null,
        nextMilestone: 'next',
        verifiedMilestones: '0',
        milestoneTotal: '1',
        milestones: [{id: 'milestone-1', label: 'next', state: 'current'}],
      },
      team: {total: '0', working: '0', sleeping: '0', waiting: '0', stopped: '0'},
      employees: [],
      tasks: [],
      obligations: [],
      artifacts: [],
      checkpoints: [],
      resources: {
        toolCallsUsed: '0',
        toolCallsLimit: '0',
        toolBudgetQuality: 'unavailable',
        moneyQuality: 'unavailable',
        moneyAmount: null,
        currency: null,
        asOf: '2026-09-14T00:00:00Z',
        note: 'fixture',
      },
      attention: [],
      recentActivity: [event],
    }, companyId);

    expect(result.success).toBe(true);
  });

  it('accepts a typed mission command receipt and rejects an unaccepted response', () => {
    const result = validateMissionCommandReceipt({commandId: 'request-1', commandType: 'mission.start', targetType: 'mission', targetId: 'mission-1', requestId: 'request-1', accepted: true, acceptedAt: '2026-09-16T00:00:00Z', resultingState: 'active'});
    const rejected = validateMissionCommandReceipt({commandId: 'request-1', commandType: 'mission.start', targetType: 'mission', targetId: 'mission-1', requestId: 'request-1', accepted: false, acceptedAt: '2026-09-16T00:00:00Z', resultingState: 'active'});

    expect(result.success).toBe(true);
    expect(rejected.success).toBe(false);
  });

  it('accepts a scoped company list with a fixed logical roster', () => {
    const result = validateCompanyList([{id: companyId, name: 'Polis', workspaceRoot: 'C:/workspace', state: 'active', roster: [{id: 'emp-planning', displayName: 'Planner', role: 'planning', modelProfile: 'deterministic/fake'}]}]);

    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.value[0]?.id).toBe(companyId);
    }
  });

  it('accepts safe runtime settings statuses without credential material', () => {
    const result = validateRuntimeSettings({companyId, workerMode: 'deterministic', provider: 'deterministic', model: 'deterministic/fake', effort: 'bounded', profile: 'deterministic/fake', authReadiness: 'not_required', runtimeVersion: 'local', runtimeReadiness: 'ready', productSurfaceQualification: 'not_applicable', workspaceRoot: 'C:/workspace', postgresqlStatus: 'ready', casStatus: 'ready', eventStreamStatus: 'deferred'});

    expect(result.success).toBe(true);
  });
});
