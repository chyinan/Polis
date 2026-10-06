// pattern: Functional Core

import {describe, expect, it} from 'vitest';
import {isAcceptanceContract, validateActivityView, validateCapabilityCatalog, validateCompanyFeedback, validateCompanyList, validateCompanyOverview, validateDomainEvidenceArtifactPreviewManifest, validateDomainEvidenceLedger, validateDomainEvidenceRecord, validateDomainEvidenceReviewRecord, validateDomainProfileQualificationRecord, validateDurableDelivery, validateDurableDeliveryManifestCompletionReceipt, validateDurableDeliveryManifestInvalidationReceipt, validateGitHubCredentialReceipt, validateGitHubFeedbackPollReceipt, validateGitHubFeedbackProbeReceipt, validateGitHubFeedbackSourceReceipt, validateJobRunCommandReceipt, validateJobRunLogArtifact, validateMissionChangeRequest, validateMissionChangeRequests, validateMissionCommandReceipt, validateOperatorInstructions, validateProjectEnvironmentRevisions, validateRuntimeSettings, validateServiceBrowserSession, validateTaskCrossBackendHandovers, validateTaskInputManifest, validateTaskJobRuns, validateTaskTakeoverLease, validateTaskTakeoverLeases} from './workbench-validation';
import {validateGitHubFeedbackCollectionPolicyReceipt} from './workbench-validation';

const companyId = 'company-1';

describe('durable delivery backlog projection', () => {
  it('rejects a lifecycle response that omits the scoped feedback backlog', () => {
    const result = validateDurableDelivery({
      manifest: {
        schemaVersion: 'polis-durable-delivery-manifest@1', deliveryId: 'artifact-1', revision: '1', companyId,
        missionId: 'mission-1', taskId: 'task-1', artifactId: 'artifact-1', state: 'assembling',
        artifact: {fileName: 'artifact.bin', byteSize: '1', sha256: 'a'.repeat(64)},
        sections: [{key: 'file_inventory', state: 'available', detail: 'artifact'}], createdAt: '2026-10-06T00:00:00Z',
      },
      manifestSha256: 'b'.repeat(64),
      userDisposition: {revision: '1', manifestRevision: '1', state: 'not_requested', actor: 'system', reason: 'not requested', requestId: 'request-1', feedbackDeadline: '', createdAt: '2026-10-06T00:00:00Z'},
    }, companyId, 'artifact-1');
    expect(result.success).toBe(false);
  });
});

describe('durable delivery completion receipt', () => {
  it('accepts only a ready system completion receipt in the requested company scope', () => {
    const result = validateDurableDeliveryManifestCompletionReceipt({
      requestId: 'complete-1', companyId, deliveryId: 'artifact-1', manifestRevision: '2', dispositionRevision: '1',
      state: 'ready', actor: 'system', createdAt: '2026-10-06T00:00:00Z',
    }, {requestId: 'complete-1', companyId, artifactId: 'artifact-1', expectedManifestRevision: '1'});
    expect(result.success).toBe(true);
  });
});

describe('durable delivery invalidation receipt', () => {
  it('accepts only the next manifest revision in the requested scope', () => {
    const valid = validateDurableDeliveryManifestInvalidationReceipt({
      requestId: 'invalidate-1', companyId, deliveryId: 'artifact-1', manifestRevision: '3', dispositionRevision: '1',
      state: 'withdrawn', actor: 'installation-owner', reason: 'owner withdrawal', createdAt: '2026-10-06T00:00:00Z',
    }, {requestId: 'invalidate-1', companyId, artifactId: 'artifact-1', expectedManifestRevision: '2', state: 'withdrawn', reason: 'owner withdrawal'});
    expect(valid.success).toBe(true);
    const stale = validateDurableDeliveryManifestInvalidationReceipt({
      requestId: 'invalidate-1', companyId, deliveryId: 'artifact-1', manifestRevision: '2', dispositionRevision: '1',
      state: 'withdrawn', actor: 'installation-owner', reason: 'owner withdrawal', createdAt: '2026-10-06T00:00:00Z',
    }, {requestId: 'invalidate-1', companyId, artifactId: 'artifact-1', expectedManifestRevision: '2', state: 'withdrawn', reason: 'owner withdrawal'});
    expect(stale.success).toBe(false);
  });
});

describe('operator instruction responses', () => {
  it('accepts the immutable response projection and clarification state', () => {
    const result = validateOperatorInstructions([{
      instructionId: 'instruction-1', missionId: 'mission-1', taskId: null, employeeId: 'emp-backend',
      content: 'Prioritize the latest failing check.', state: 'needs_clarification', createdAt: '2026-09-26T00:00:00Z',
      responseOutcome: 'needs_clarification', responseSummary: 'The failing check refers to an unavailable environment.',
      respondedByEmployeeId: 'emp-backend', respondedAt: '2026-09-26T00:01:00Z',
      responses: [{employeeId: 'emp-backend', outcome: 'needs_clarification', summary: 'The failing check refers to an unavailable environment.', respondedAt: '2026-09-26T00:01:00Z'}],
    }]);
    expect(result.success).toBe(true);
  });

  it('rejects response metadata that does not match a known outcome', () => {
    const result = validateOperatorInstructions([{
      instructionId: 'instruction-1', missionId: 'mission-1', taskId: null, employeeId: null,
      content: 'Continue.', state: 'applied', createdAt: '2026-09-26T00:00:00Z',
      responseOutcome: 'unknown', responseSummary: null, respondedByEmployeeId: null, respondedAt: null,
      responses: [],
    }]);
    expect(result.success).toBe(false);
  });

  it('rejects response metadata inconsistent with instruction state', () => {
    const result = validateOperatorInstructions([{
      instructionId: 'instruction-1', missionId: 'mission-1', taskId: null, employeeId: 'emp-backend',
      content: 'Continue.', state: 'pending', createdAt: '2026-09-26T00:00:00Z',
      responseOutcome: 'applied', responseSummary: 'Applied.', respondedByEmployeeId: 'emp-backend', respondedAt: '2026-09-26T00:01:00Z',
      responses: [{employeeId: 'emp-backend', outcome: 'applied', summary: 'Applied.', respondedAt: '2026-09-26T00:01:00Z'}],
    }]);
    expect(result.success).toBe(false);
  });

  it('preserves legacy terminal instructions without response metadata', () => {
    const result = validateOperatorInstructions([{
      instructionId: 'instruction-1', missionId: 'mission-1', taskId: null, employeeId: null,
      content: 'Continue.', state: 'applied', createdAt: '2026-09-26T00:00:00Z',
      responseOutcome: null, responseSummary: null, respondedByEmployeeId: null, respondedAt: null,
      responses: [],
    }]);
    expect(result.success).toBe(true);
  });

  it('rejects a clarification state without immutable response evidence', () => {
    const result = validateOperatorInstructions([{
      instructionId: 'instruction-1', missionId: 'mission-1', taskId: null, employeeId: 'emp-backend',
      content: 'Continue.', state: 'needs_clarification', createdAt: '2026-09-26T00:00:00Z',
      responseOutcome: null, responseSummary: null, respondedByEmployeeId: null, respondedAt: null,
      responses: [],
    }]);
    expect(result.success).toBe(false);
  });
});

describe('formal mission change requests', () => {
  const request = {
    changeRequestId: 'change-1', missionId: 'mission-1', clientRequestId: 'change-command-1',
    baseRequirementsSha256: 'a'.repeat(64), changeSummary: 'Define empty-dataset behavior.',
    proposedTitle: 'API revision', proposedGoal: 'Return an empty list when no records exist.',
    proposedAcceptanceContract: {revision: 'text-acceptance@1', required_text: ['Task summary:', 'Empty results:']},
    blockPreviousResults: true, state: 'queued', impactRevision: 1, impactSha256: 'b'.repeat(64),
    planningAssessment: null,
    impact: {
      schemaVersion: 'polis-mission-change-impact@1', missionId: 'mission-1', baseRequirementsSha256: 'a'.repeat(64),
      inputRevisions: [{inputId: 'input-1', revision: 2, contentDigest: 'c'.repeat(64), state: 'usable'}],
      tasks: [{taskId: 'task-1', ownerEmployeeId: 'emp-backend', kind: 'compat', state: 'working', generation: 1, workspaceDigest: 'd'.repeat(64), workspaceRevision: 3}],
      artifacts: [{artifactId: 'artifact-1', taskId: 'task-1', digest: 'e'.repeat(64), verdict: 'candidate'}],
      activeWorkerSessions: [{sessionId: 'session-1', taskId: 'task-1', employeeId: 'emp-backend', state: 'active'}],
      nonterminalJobRuns: [], activeServiceEndpoints: [], activeTaskTakeoverLeases: [], returnedHumanTakeoverSnapshots: [], naturalLanguageImpactStatus: 'not_assessed',
    },
    successorMissionId: null, inputRevisionMap: [], createdAt: '2026-09-26T00:00:00Z',
    events: [{eventId: 'event-1', state: 'received', impactRevision: 1, successorMissionId: null, reasonCode: 'request_received', createdAt: '2026-09-26T00:00:00Z', inputRevisionMap: []}],
  };

  it('accepts a mission-scoped impact snapshot and state history', () => {
    expect(validateMissionChangeRequest(request, 'mission-1').success).toBe(true);
    expect(validateMissionChangeRequests([request], 'mission-1').success).toBe(true);
  });

  it('rejects cross-mission impacts and applied changes without a successor', () => {
    expect(validateMissionChangeRequest({...request, impact: {...request.impact, missionId: 'other-mission'}}, 'mission-1').success).toBe(false);
    expect(validateMissionChangeRequest({...request, state: 'applied'}, 'mission-1').success).toBe(false);
    expect(validateMissionChangeRequests([request, request], 'mission-1').success).toBe(false);
  });
});

describe('Task human takeover lease validation', () => {
  const lease = {
    leaseId: 'lease-1', missionId: 'mission-1', taskId: 'task-1', clientRequestId: 'request-1',
    baseRequirementsSha256: 'a'.repeat(64), baseWorkspaceDigest: 'b'.repeat(64), baseWorkspaceRevision: 4,
    state: 'granted', snapshotInputId: null, snapshotRevision: null, snapshotDigest: null, snapshotBytes: null,
    humanEffortSeconds: null, diffSummary: null, createdAt: '2026-09-26T00:00:00Z',
    events: [{eventId: 'event-1', state: 'granted', snapshotInputId: null, snapshotRevision: null, snapshotDigest: null, snapshotBytes: null, humanEffortSeconds: null, diffSummary: null, reasonCode: 'worker_tree_stopped', createdAt: '2026-09-26T00:00:00Z'}],
  } as const;

  it('accepts a scoped frozen lease and rejects inconsistent handback data', () => {
    expect(validateTaskTakeoverLease(lease, 'mission-1').success).toBe(true);
    expect(validateTaskTakeoverLeases([lease], 'mission-1').success).toBe(true);
    expect(validateTaskTakeoverLease({...lease, missionId: 'other-mission'}, 'mission-1').success).toBe(false);
    expect(validateTaskTakeoverLeases([lease, lease], 'mission-1').success).toBe(false);
    expect(validateTaskTakeoverLease({...lease, state: 'returned'}, 'mission-1').success).toBe(false);
    expect(validateTaskTakeoverLease({...lease, events: [{...lease.events[0], state: 'released'}]}, 'mission-1').success).toBe(false);
  });
});

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

function companyOverviewWithEmployee(employee: unknown): Readonly<Record<string, unknown>> {
  return {
    meta,
    company: {companyId, name: 'Polis', description: 'fixture'},
    mission: {
      missionId: 'mission-1', title: 'mission', goal: 'goal', state: 'active', contract: 'contract@1',
      acceptanceContract: null, currentContractRevision: null, nextMilestone: 'next', verifiedMilestones: '0', milestoneTotal: '1',
      milestones: [{id: 'milestone-1', label: 'next', state: 'current'}],
    },
    team: {total: '1', working: '0', sleeping: '1', waiting: '0', stopped: '0'},
    employees: [employee],
    tasks: [], obligations: [], artifacts: [], checkpoints: [],
    resources: {
      toolCallsUsed: '0', toolCallsLimit: '0', toolBudgetQuality: 'unavailable', moneyQuality: 'unavailable',
      moneyAmount: null, currency: null, asOf: '2026-09-14T00:00:00Z', note: 'fixture',
    },
    attention: [], recentActivity: [event],
  };
}

describe('Workbench view validation', () => {
  it('accepts the R3 profiles only while qualification and execution remain closed', () => {
    const contentProfile = {
      id: 'content-operations-reference', revision: 'content-operations@1', domain: 'content_operations',
      qualificationStatus: 'not_run', executionEnabled: false, stages: ['authorized_sources'],
      requiredEvidence: ['quality', 'intervention', 'recovery', 'cost', 'organization_benefit'],
    };
    const researchProfile = {
      id: 'research-simulation-reference', revision: 'research-simulation@1', domain: 'research_simulation',
      qualificationStatus: 'not_run', executionEnabled: false, stages: ['pin_dataset'],
      requiredEvidence: ['quality', 'recovery', 'cost', 'organization_benefit'],
    };
    const ledger = {companyId, profiles: [contentProfile, researchProfile], submissions: [], qualifications: [], researchSimulationRuns: [], contentSourceEvents: [], contentDrafts: [], contentReviews: [], contentPublications: [], contentCorrections: [], contentFeedback: []};
    expect(validateDomainEvidenceLedger(ledger, companyId).success).toBe(true);
    expect(validateDomainEvidenceLedger({...ledger, profiles: [{...contentProfile, executionEnabled: true}, researchProfile]}, companyId).success).toBe(false);
    expect(validateDomainEvidenceLedger({...ledger, profiles: [{...contentProfile, revision: 'research-simulation@1'}, researchProfile]}, companyId).success).toBe(false);
  });

  it('validates company-scoped content source, versioned draft and stale-aware review projections', () => {
    const profile = {
      id: 'content-operations-reference', revision: 'content-operations@1', domain: 'content_operations',
      qualificationStatus: 'not_run', executionEnabled: false, stages: ['authorized_sources'],
      requiredEvidence: ['quality', 'intervention', 'recovery', 'cost', 'organization_benefit'],
    };
    const source = {
      companyId, eventSeq: '1', eventId: 'content-source-event-1', inputId: 'source-1', revision: '1', sha256: 'a'.repeat(64),
      state: 'authorized', rationale: 'approved for this synthetic review fixture', requestId: 'source-authorization-1', createdAt: '2026-09-29T00:00:00Z',
    };
    const draft = {
      companyId, draftId: 'content-draft-1', draftInputId: 'draft-input-1', draftRevision: '1', draftSha256: 'b'.repeat(64),
      writerEmployeeId: 'emp-backend', criticalClaims: ['claim-1'], constraintsPassed: true,
      requestId: 'content-draft-1-request', createdAt: '2026-09-29T00:01:00Z',
    };
    const review = {
      companyId, reviewId: 'content-review-1', draftInputId: draft.draftInputId, draftRevision: '1', draftSha256: draft.draftSha256,
      checkerEmployeeId: 'emp-review', correctionId: '', outcome: 'accepted', stale: false,
      review: {draftRevision: '1', checkerEmployeeId: 'emp-review', humanSampled: true, claims: [{claimId: 'claim-1', finding: 'verified', sources: [{inputId: source.inputId, revision: source.revision, sha256: source.sha256}], limitation: ''}]},
      sample: {draftRevision: '1', draftSha256: draft.draftSha256, plan: {inputId: 'plan-1', revision: '2', sha256: 'c'.repeat(64)}, sampledClaimIds: ['claim-1'], sampledByEmployeeId: 'emp-review'},
      reasonCodes: [], requestId: 'content-review-1-request', createdAt: '2026-09-29T00:02:00Z',
    };
    const ledger = {
      companyId, profiles: [profile], submissions: [], qualifications: [], researchSimulationRuns: [],
      contentSourceEvents: [source], contentDrafts: [draft], contentReviews: [review], contentPublications: [], contentCorrections: [], contentFeedback: [],
    };

    expect(validateDomainEvidenceLedger(ledger, companyId).success).toBe(true);
    expect(validateDomainEvidenceLedger({...ledger, contentSourceEvents: [{...source, companyId: 'other-company'}]}, companyId).success).toBe(false);
    expect(validateDomainEvidenceLedger({...ledger, contentReviews: [{...review, sample: {...review.sample, draftSha256: 'd'.repeat(64)}}]}, companyId).success).toBe(false);
  });

  it('accepts only bounded company-scoped research simulation receipts in the ledger', () => {
    const researchProfile = {
      id: 'research-simulation-reference', revision: 'research-simulation@1', domain: 'research_simulation',
      qualificationStatus: 'not_run', executionEnabled: false, stages: ['pin_dataset'],
      requiredEvidence: ['quality', 'recovery', 'cost', 'organization_benefit'],
    };
    const output = {
      schemaVersion: 'polis-research-simulation-output@1', algorithm: 'bootstrap-mean-difference@1', protocolRevision: 1,
      datasetSha256: 'a'.repeat(64), methodSha256: 'b'.repeat(64), seed: '42', controlDefinition: 'control cohort',
      iterations: 16, sampleSize: 4, riskConsumedUnits: 128, riskUnit: 'sample_draw',
      controlMean: 2.5, treatmentMean: 3.5, meanDifference: 1, minimumDifference: 0, maximumDifference: 2,
    };
    const run = {
      companyId, runId: 'research-run-1', profileId: researchProfile.id, profileRevision: researchProfile.revision,
      protocolRevision: '1', datasetInputId: 'dataset-1', datasetInputRevision: '1', datasetSha256: output.datasetSha256,
      methodInputId: 'method-1', methodInputRevision: '1', methodSha256: output.methodSha256, seed: '42', controlDefinition: 'control cohort',
      riskUnit: 'sample_draw', riskBudgetUnits: '128', riskConsumedUnits: '128', outputSha256: 'c'.repeat(64), output,
      requestId: 'research-request-1', createdAt: '2026-09-29T00:00:00Z',
    };
    const ledger = {companyId, profiles: [researchProfile], submissions: [], qualifications: [], researchSimulationRuns: [run], contentSourceEvents: [], contentDrafts: [], contentReviews: [], contentPublications: [], contentCorrections: [], contentFeedback: []};

    expect(validateDomainEvidenceLedger(ledger, companyId).success).toBe(true);
    expect(validateDomainEvidenceLedger({...ledger, researchSimulationRuns: [{...run, riskBudgetUnits: '5120001'}]}, companyId).success).toBe(false);
    expect(validateDomainEvidenceLedger({...ledger, researchSimulationRuns: [{...run, companyId: 'other-company'}]}, companyId).success).toBe(false);
    const underreported = {...run, riskBudgetUnits: '1', riskConsumedUnits: '1', output: {...output, riskConsumedUnits: 1}};
    expect(validateDomainEvidenceLedger({...ledger, researchSimulationRuns: [underreported]}, companyId).success).toBe(false);
  });

  it('matches the profile status to an evidence-bound owner qualification decision while execution stays disabled', () => {
    const contentProfile = {
      id: 'content-operations-reference', revision: 'content-operations@1', domain: 'content_operations',
      qualificationStatus: 'qualified' as const, executionEnabled: false as const, stages: ['authorized_sources'],
      requiredEvidence: ['quality', 'intervention', 'recovery', 'cost', 'organization_benefit'],
    };
    const researchProfile = {
      id: 'research-simulation-reference', revision: 'research-simulation@1', domain: 'research_simulation',
      qualificationStatus: 'not_run' as const, executionEnabled: false as const, stages: ['pin_dataset'],
      requiredEvidence: ['quality', 'recovery', 'cost', 'organization_benefit'],
    };
    const qualification = {
      companyId, eventId: 'domain-profile-qualification-event-1', profileId: contentProfile.id,
      profileRevision: contentProfile.revision, decision: 'qualified', evidenceInputId: 'domain-qualification-report',
      evidenceInputRevision: '2', evidenceSha256: 'a'.repeat(64), rationale: 'reviewed supporting samples',
      actor: 'local-owner', requestId: 'domain-profile-qualification-request', createdAt: '2026-09-29T04:00:00Z',
    };
    const ledger = {companyId, profiles: [contentProfile, researchProfile], submissions: [], qualifications: [qualification], researchSimulationRuns: [], contentSourceEvents: [], contentDrafts: [], contentReviews: [], contentPublications: [], contentCorrections: [], contentFeedback: []};
    expect(validateDomainEvidenceLedger(ledger, companyId).success).toBe(true);
    expect(validateDomainEvidenceLedger({...ledger, profiles: [{...contentProfile, qualificationStatus: 'not_run'}, researchProfile]}, companyId).success).toBe(false);
    expect(validateDomainProfileQualificationRecord(qualification, companyId, contentProfile.id, qualification.requestId).success).toBe(true);
  });

  it('validates company-scoped evidence records without accepting qualified or enabled status', () => {
    const submission = {profileId: 'content-operations-reference', profileRevision: 'content-operations@1', evidence: []};
    const record = {
      companyId, recordId: 'domain-evidence-1', profileId: submission.profileId, profileRevision: submission.profileRevision,
      readinessStatus: 'incomplete', qualificationStatus: 'not_run', executionEnabled: false,
      evidenceDigest: 'a'.repeat(64), submission, review: null, assessment: null, reasonCodes: ['domain_evidence_area_missing'], requestId: 'request-1', createdAt: '2026-09-25T00:00:00Z',
    };
    expect(validateDomainEvidenceRecord(record, companyId, 'request-1').success).toBe(true);
    expect(validateDomainEvidenceRecord({...record, companyId: 'other-company'}, companyId, 'request-1').success).toBe(false);
    expect(validateDomainEvidenceRecord({...record, qualificationStatus: 'qualified'}, companyId, 'request-1').success).toBe(false);
    const requiredEvidence = ['quality', 'intervention', 'recovery', 'cost', 'organization_benefit'] as const;
    const readyRecord = {
      ...record,
      readinessStatus: 'ready_for_review',
      reasonCodes: ['domain_evidence_requires_human_qualification_review'],
      submission: {
        ...submission,
        evidence: requiredEvidence.map((area, index) => ({
          area, artifact: {id: `artifact-${index}`, revision: 1, sha256: 'a'.repeat(64)},
          methodSHA256: 'b'.repeat(64), assessedByEmployeeId: 'emp-review',
        })),
      },
    };
    expect(validateDomainEvidenceRecord(readyRecord, companyId, 'request-1').success).toBe(true);
    expect(validateDomainEvidenceRecord({...readyRecord, submission}, companyId, 'request-1').success).toBe(false);
    const review = {
      companyId, reviewId: 'domain-review-1', recordId: readyRecord.recordId, outcome: 'evidence_references_accepted',
      reviewerEmployeeId: 'emp-domain-review', rationale: 'reference and method digest reviewed', reviewContractRevision: 1,
      previewedEvidence: readyRecord.submission.evidence.map(item => ({
        area: item.area, relativePath: `${item.area}.md`, sourceDigest: item.artifact.sha256, contentDigest: 'c'.repeat(64), mediaType: 'text/markdown',
      })),
      requestId: 'review-request-1', createdAt: '2026-09-25T01:00:00Z',
    };
    expect(validateDomainEvidenceReviewRecord(review, companyId, readyRecord.recordId, 'review-request-1').success).toBe(true);
    expect(validateDomainEvidenceReviewRecord({...review, companyId: 'other-company'}, companyId, readyRecord.recordId, 'review-request-1').success).toBe(false);
    expect(validateDomainEvidenceReviewRecord({...review, outcome: 'approved_for_execution'}, companyId, readyRecord.recordId, 'review-request-1').success).toBe(false);
    expect(validateDomainEvidenceRecord({...readyRecord, review}, companyId, 'request-1').success).toBe(true);
    const assessment = {
      companyId, assessmentId: 'domain-substantive-1', recordId: readyRecord.recordId, evidenceDigest: readyRecord.evidenceDigest,
      outcome: 'evidence_accepted', reviewerEmployeeId: review.reviewerEmployeeId,
      areaAssessments: requiredEvidence.map(area => ({area, outcome: 'accepted', rationale: 'reviewed against the scoped criterion'})),
      previewedEvidence: review.previewedEvidence, requestId: 'assessment-request-1', createdAt: '2026-09-25T02:00:00Z',
    };
    expect(validateDomainEvidenceRecord({...readyRecord, review, assessment}, companyId, 'request-1').success).toBe(true);
    expect(validateDomainEvidenceRecord({...readyRecord, review, assessment: {...assessment, outcome: 'more_evidence_required'}}, companyId, 'request-1').success).toBe(false);
  });

  it('validates bounded domain evidence preview manifests against the exact input reference', () => {
    const options = {companyId, recordId: 'domain-evidence-1', area: 'quality' as const, inputId: 'input-1', inputRevision: 2, sourceDigest: 'a'.repeat(64)};
    const manifest = {
      ...options,
      entries: [{relativePath: 'reports/quality.md', fileName: 'quality.md', mediaType: 'text/markdown', byteSize: 18, contentSHA256: 'b'.repeat(64), previewable: true, reasonCode: ''}],
    };
    expect(validateDomainEvidenceArtifactPreviewManifest(manifest, options).success).toBe(true);
    expect(validateDomainEvidenceArtifactPreviewManifest({...manifest, sourceDigest: 'c'.repeat(64)}, options).success).toBe(false);
    expect(validateDomainEvidenceArtifactPreviewManifest({...manifest, entries: [{...manifest.entries[0], relativePath: '../outside.md'}]}, options).success).toBe(false);
    expect(validateDomainEvidenceArtifactPreviewManifest({...manifest, entries: [{...manifest.entries[0], mediaType: 'text/html'}]}, options).success).toBe(false);
  });

  it('accepts the explicitly versioned Streamable HTTP MCP qualification profile', () => {
    const result = validateCapabilityCatalog({
      skills: [], mcpServers: [], mcpPackages: [], bindings: [], decisions: [], runtimeQualifications: [],
      qualifications: [{companyId, qualificationId: 'qualification-1', capabilityKind: 'mcp', capabilityId: 'mcp-1', versionDigest: 'a'.repeat(64), profile: 'streamable_http_mcp_2026_07_28@1', status: 'metadata_verified', evidenceDigest: 'b'.repeat(64), createdAt: '2026-09-25T00:00:00Z'}],
    }, companyId);
    expect(result.success).toBe(true);
  });

  it('defaults controlled runtime observation closed and validates its availability flag', () => {
    const catalog = {skills: [], mcpServers: [], mcpPackages: [], qualifications: [], bindings: [], decisions: [], runtimeQualifications: []};
    const defaultClosed = validateCapabilityCatalog(catalog, companyId);
    const enabled = validateCapabilityCatalog({...catalog, runtimeObservationAvailable: true}, companyId);
    const malformed = validateCapabilityCatalog({...catalog, runtimeObservationAvailable: 'yes'}, companyId);

    expect(defaultClosed.success && defaultClosed.value.runtimeObservationAvailable).toBe(false);
    expect(enabled.success && enabled.value.runtimeObservationAvailable).toBe(true);
    expect(malformed.success).toBe(false);
  });

  it('projects pinned stdio MCP runtime qualification without implying Worker dispatch', () => {
    const runtimeQualification = {
      companyId, runtimeQualificationId: 'runtime-qualification-1', capabilityId: 'mcp-1', capabilityQualificationId: 'qualification-1',
      versionDigest: 'a'.repeat(64), descriptorDigest: 'a'.repeat(64), runtimeProfile: 'polis-controlled-stdio-mcp-2026-07-28@1',
      hostOs: 'windows', hostProfile: 'windows_appcontainer_deny_all@1', commandSha256: 'b'.repeat(64), packageManifestSha256: 'c'.repeat(64),
      serverName: 'fixture-mcp', serverVersion: '1.0.0', protocolVersion: '2026-07-28', toolSchemaSha256: 'd'.repeat(64),
      status: 'qualified', evidenceDigest: 'e'.repeat(64), createdAt: '2026-09-27T00:00:00Z',
    };
    const binding = {
      companyId, eventId: 'binding-1', employeeId: 'emp-backend', capabilityKind: 'mcp', capabilityId: 'mcp-1',
      versionDigest: 'a'.repeat(64), qualificationId: 'qualification-1', qualificationStatus: 'metadata_verified',
      state: 'bound', executionStatus: 'runtime_qualified_dispatch_unavailable', reason: 'fixture', createdAt: '2026-09-27T00:00:00Z',
    };
    const catalog = {
      skills: [], mcpServers: [{companyId, id: 'mcp-1', name: 'Fixture MCP', transport: 'stdio', endpoint: null, command: 'runtime/node.exe', args: ['server/main.mjs'], descriptorDigest: 'a'.repeat(64), status: 'approved', createdAt: '2026-09-27T00:00:00Z'}], mcpPackages: [], bindings: [binding], decisions: [], runtimeQualifications: [runtimeQualification],
      qualifications: [{companyId, qualificationId: 'qualification-1', capabilityKind: 'mcp', capabilityId: 'mcp-1', versionDigest: 'a'.repeat(64), profile: 'stdio_mcp@1', status: 'metadata_verified', evidenceDigest: 'f'.repeat(64), createdAt: '2026-09-27T00:00:00Z'}],
    };
    expect(validateCapabilityCatalog(catalog, companyId).success).toBe(true);
    expect(validateCapabilityCatalog({...catalog, runtimeQualifications: [{...runtimeQualification, toolSchemaSha256: 'invalid'}]}, companyId).success).toBe(false);
    expect(validateCapabilityCatalog({...catalog, mcpServers: []}, companyId).success).toBe(false);
    expect(validateCapabilityCatalog({...catalog, bindings: [{...binding, executionStatus: 'runtime_unqualified'}]}, companyId).success).toBe(true);
  });

  it('validates Streamable HTTP runtime qualifications against the current endpoint descriptor', () => {
    const runtimeQualification = {
      companyId, runtimeQualificationId: 'http-runtime-1', capabilityId: 'mcp-http-1', capabilityQualificationId: 'http-qualification-1',
      transport: 'streamable_http', endpoint: 'https://mcp.example.com/v1/mcp', versionDigest: 'a'.repeat(64), descriptorDigest: 'a'.repeat(64),
      runtimeProfile: 'polis-streamable-http-mcp-2026-07-28@1', hostOs: 'remote', hostProfile: 'https_public_dns_pinned@1',
      commandSha256: '', packageManifestSha256: '', serverName: 'Read-only reference', serverVersion: 'unreported', protocolVersion: '2026-07-28',
      toolSchemaSha256: 'd'.repeat(64), status: 'observed_unqualified', evidenceDigest: 'e'.repeat(64), createdAt: '2026-09-28T00:00:00Z',
    };
    const catalog = {
      skills: [], mcpServers: [{companyId, id: 'mcp-http-1', name: 'Read-only reference', transport: 'streamable_http', endpoint: 'https://mcp.example.com/v1/mcp', command: null, args: [], descriptorDigest: 'a'.repeat(64), status: 'approved', createdAt: '2026-09-28T00:00:00Z'}],
      mcpPackages: [], qualifications: [{companyId, qualificationId: 'http-qualification-1', capabilityKind: 'mcp', capabilityId: 'mcp-http-1', versionDigest: 'a'.repeat(64), profile: 'streamable_http_mcp_2026_07_28@1', status: 'metadata_verified', evidenceDigest: 'f'.repeat(64), createdAt: '2026-09-28T00:00:00Z'}],
      bindings: [], decisions: [], runtimeQualifications: [runtimeQualification], runtimeObservationAvailable: false, streamableHttpRuntimeObservationAvailable: true,
    };
    expect(validateCapabilityCatalog(catalog, companyId).success).toBe(true);
    expect(validateCapabilityCatalog({...catalog, runtimeQualifications: [{...runtimeQualification, endpoint: 'https://other.example.com/mcp'}]}, companyId).success).toBe(false);
    expect(validateCapabilityCatalog({...catalog, streamableHttpRuntimeObservationAvailable: 'yes'}, companyId).success).toBe(false);
  });

  it('validates imported stdio MCP package manifests against their server descriptor', () => {
    const manifest = {
      schemaVersion: 'polis-controlled-stdio-mcp@1', name: 'fixture-mcp', serverName: 'fixture-server', serverVersion: '1.0.0',
      command: 'runtime/node.exe', entryPoint: 'server/main.mjs', args: ['server/main.mjs'],
      files: [
        {relativePath: 'runtime/node.exe', mediaType: 'application/octet-stream', byteSize: 128, contentSHA256: 'b'.repeat(64)},
        {relativePath: 'server/main.mjs', mediaType: 'text/javascript', byteSize: 64, contentSHA256: 'c'.repeat(64)},
      ],
    };
    const base = {
      skills: [], mcpServers: [{companyId, id: 'mcp-1', name: 'fixture-mcp', transport: 'stdio', endpoint: null, command: 'runtime/node.exe', args: ['server/main.mjs'], descriptorDigest: 'a'.repeat(64), status: 'unverified', createdAt: '2026-09-28T00:00:00Z'}],
      mcpPackages: [{companyId, id: 'package-1', serverId: 'mcp-1', revision: '1.0.0', manifestDigest: 'd'.repeat(64), manifest, createdAt: '2026-09-28T00:00:00Z'}],
      qualifications: [], bindings: [], decisions: [], runtimeQualifications: [],
    };
    expect(validateCapabilityCatalog(base, companyId).success).toBe(true);
    expect(validateCapabilityCatalog({...base, mcpPackages: [{...base.mcpPackages[0], manifest: {...manifest, command: '../outside.exe'}}]}, companyId).success).toBe(false);
    expect(validateCapabilityCatalog({...base, mcpPackages: [{...base.mcpPackages[0], manifest: {...manifest, args: []}}]}, companyId).success).toBe(false);
  });

  it('accepts MCP package paths through the ZIP extractor byte limit', () => {
    const manifest = {
      schemaVersion: 'polis-controlled-stdio-mcp@1', name: 'fixture-mcp', serverName: 'fixture-server', serverVersion: '1.0.0',
      command: 'runtime/node.exe', entryPoint: 'server/main.mjs', args: ['server/main.mjs'],
      files: [
        {relativePath: 'runtime/node.exe', mediaType: 'application/octet-stream', byteSize: 128, contentSHA256: 'b'.repeat(64)},
        {relativePath: 'server/main.mjs', mediaType: 'text/javascript', byteSize: 64, contentSHA256: 'c'.repeat(64)},
      ],
    };
    const base = {
      skills: [], mcpServers: [{companyId, id: 'mcp-1', name: 'fixture-mcp', transport: 'stdio', endpoint: null, command: 'runtime/node.exe', args: ['server/main.mjs'], descriptorDigest: 'a'.repeat(64), status: 'unverified', createdAt: '2026-09-28T00:00:00Z'}],
      mcpPackages: [{companyId, id: 'package-1', serverId: 'mcp-1', revision: '1.0.0', manifestDigest: 'd'.repeat(64), manifest, createdAt: '2026-09-28T00:00:00Z'}],
      qualifications: [], bindings: [], decisions: [], runtimeQualifications: [],
    };
    const longPath = `assets/${'d/'.repeat(260)}main.mjs`;
    const longManifest = {
      ...manifest,
      files: [...manifest.files, {relativePath: longPath, mediaType: 'text/javascript', byteSize: 1, contentSHA256: 'e'.repeat(64)}]
        .sort((left, right) => left.relativePath < right.relativePath ? -1 : left.relativePath > right.relativePath ? 1 : 0),
    };
    const longCatalog = {...base, mcpPackages: [{...base.mcpPackages[0], manifest: longManifest}]};
    expect(validateCapabilityCatalog(longCatalog, companyId).success).toBe(true);

    const overlongManifest = {
      ...manifest,
      files: [...manifest.files, {relativePath: `${'d/'.repeat(512)}x`, mediaType: 'text/javascript', byteSize: 1, contentSHA256: 'f'.repeat(64)}]
        .sort((left, right) => left.relativePath < right.relativePath ? -1 : left.relativePath > right.relativePath ? 1 : 0),
    };
    expect(validateCapabilityCatalog({...base, mcpPackages: [{...base.mcpPackages[0], manifest: overlongManifest}]}, companyId).success).toBe(false);
  });

  it('validates the separate text and image Worker delivery limits', () => {
    const candidates = [
      ...Array.from({length: 8}, (_, index) => ({inputId: `text-${index}`, revision: 1, requestId: `text-request-${index}`, sourceKind: 'upload', displayName: `text-${index}.txt`, mediaType: 'text/plain', byteSize: 8192, contentDigest: String(index).padStart(64, 'a'), state: 'usable'})),
      ...Array.from({length: 4}, (_, index) => ({inputId: `image-${index}`, revision: 1, requestId: `image-request-${index}`, sourceKind: 'upload', displayName: `image-${index}.png`, mediaType: 'image/png', byteSize: 2 * 1024 * 1024, contentDigest: String(index + 1).padStart(64, 'b'), state: 'partial'})),
    ];
    const included = candidates.map(input => ({inputId: input.inputId, relativePath: '', mediaType: input.mediaType, byteSize: input.byteSize, contentDigest: input.contentDigest}));
    const includedInputIds = included.map(item => item.inputId);
	const manifest = {schemaVersion: 'polis-model-input-manifest@1', companyId, missionId: 'mission-1', taskId: 'task-1', candidateInputs: candidates, excludedInputs: [], deliveryStatus: 'not_delivered'};
	const view = {companyId, missionId: 'mission-1', taskId: 'task-1', manifestDigest: 'f'.repeat(64), deliveryStatus: 'local_context_loaded', payloadDigest: 'e'.repeat(64), includedInputIds, includedInputPaths: included, inputExclusions: [], manifest, createdAt: '2026-09-25T00:00:00Z'};

    expect(validateTaskInputManifest(view, companyId, 'task-1').success).toBe(true);
    const extraCandidate = {...candidates[11], inputId: 'image-4', requestId: 'image-request-4', displayName: 'image-4.png'};
    const overLimit = {
      ...view,
      includedInputIds: [...includedInputIds, 'image-4'],
      includedInputPaths: [...included, {...included[11], inputId: 'image-4'}],
      manifest: {...manifest, candidateInputs: [...candidates, extraCandidate]},
    };
    expect(validateTaskInputManifest(overLimit, companyId, 'task-1').success).toBe(false);
  });

  it('accepts a partial fixed-commit Git input candidate with delivered text files', () => {
    const candidate = {inputId: 'git-input', revision: 1, requestId: 'git-import', sourceKind: 'git_snapshot', displayName: 'repo@abcdef012345', mediaType: 'application/gzip', byteSize: 2048, contentDigest: 'a'.repeat(64), state: 'partial'};
    const view = {
      companyId, missionId: 'mission-1', taskId: 'task-git', manifestDigest: 'b'.repeat(64), deliveryStatus: 'local_context_loaded', payloadDigest: 'd'.repeat(64),
      includedInputIds: ['git-input'], includedInputPaths: [{inputId: 'git-input', relativePath: 'repo/README.md', mediaType: 'text/markdown', byteSize: 128, contentDigest: 'c'.repeat(64)}], inputExclusions: [],
      manifest: {schemaVersion: 'polis-model-input-manifest@1', companyId, missionId: 'mission-1', taskId: 'task-git', candidateInputs: [candidate], excludedInputs: [], deliveryStatus: 'not_delivered'},
      createdAt: '2026-09-25T00:00:00Z',
    };

    expect(validateTaskInputManifest(view, companyId, 'task-git').success).toBe(true);
  });

  it('distinguishes an empty required-input delivery from a not-yet-delivered manifest', () => {
    const manifest = {
      schemaVersion: 'polis-model-input-manifest@1', companyId, missionId: 'mission-empty', taskId: 'task-empty',
      candidateInputs: [], excludedInputs: [], deliveryStatus: 'not_delivered',
    };
    const view = {
      companyId, missionId: 'mission-empty', taskId: 'task-empty', manifestDigest: 'a'.repeat(64), deliveryStatus: 'not_required',
      payloadDigest: 'b'.repeat(64), includedInputIds: [], includedInputPaths: [], inputExclusions: [], manifest,
      createdAt: '2026-09-25T00:00:00Z',
    };
    expect(validateTaskInputManifest(view, companyId, 'task-empty').success).toBe(true);
    expect(validateTaskInputManifest({...view, payloadDigest: null}, companyId, 'task-empty').success).toBe(false);
    expect(validateTaskInputManifest({...view, deliveryStatus: 'not_delivered', payloadDigest: null}, companyId, 'task-empty').success).toBe(true);
    expect(validateTaskInputManifest({...view, deliveryStatus: 'not_delivered', payloadDigest: 'b'.repeat(64)}, companyId, 'task-empty').success).toBe(false);
  });

  it('validates company-scoped GitHub feedback and rejects oversized source text', () => {
    const feedback = {
      companyId,
      sources: [{sourceId: 'source-1', provider: 'github', repositoryId: '1296269', repository: 'acme/widget', profileRevision: 'github-issues-readonly@1', state: 'approved', permissionStatus: 'verified', coverage: 'complete', coverageReason: '', coveredThrough: '2026-09-24T00:00:00Z', lastScanAt: '2026-09-24T00:00:00Z', collectionEnabled: false, collectionIntervalSeconds: 0, collectionRationale: '', collectionNextPollAt: null, collectionLastAttempt: '', collectionLastReasonCode: ''}],
      issues: [{sourceId: 'source-1', providerItemId: '101', issueNumber: '7', revisionSha256: 'a'.repeat(64), backlogStatus: 'open', backlogReason: 'new_issue_observed', backlogUpdatedAt: '2026-09-24T00:00:01Z', title: 'bounded issue', titleTruncated: false, body: 'untrusted issue body', bodySha256: 'b'.repeat(64), bodyTruncated: false, state: 'open', sourceUpdatedAt: '2026-09-24T00:00:00Z', observedAt: '2026-09-24T00:00:01Z', htmlUrl: 'https://github.com/acme/widget/issues/7', commentCoverage: 'complete', commentCoverageReason: '', commentCount: '0', commentContextPartial: false, comments: []}],
    };

    expect(validateCompanyFeedback(feedback, companyId).success).toBe(true);
    expect(validateCompanyFeedback({...feedback, companyId: 'company-2'}, companyId).success).toBe(false);
    expect(validateCompanyFeedback({...feedback, issues: [{...feedback.issues[0], body: '界'.repeat(6000)}]}, companyId).success).toBe(false);
    expect(validateCompanyFeedback({...feedback, issues: [{...feedback.issues[0], backlogStatus: 'remote_closed'}]}, companyId).success).toBe(false);
  });

  it('accepts only non-secret GitHub credential receipts', () => {
    const stored = validateGitHubCredentialReceipt({credentialRef: 'default-readonly', stored: true}, true);
    const deleted = validateGitHubCredentialReceipt({credentialRef: 'default-readonly', stored: false}, false);
    const malformed = validateGitHubCredentialReceipt({credentialRef: 'default-readonly', stored: false}, true);

    expect(stored.success && stored.value).toEqual({credentialRef: 'default-readonly', stored: true});
    expect(deleted.success && deleted.value).toEqual({credentialRef: 'default-readonly', stored: false});
    expect(malformed.success).toBe(false);
  });

  it('validates company-bound GitHub source, probe and poll command receipts', () => {
    const source = {companyId, sourceId: 'source-1', repositoryId: '1296269', repository: 'acme/widget', profileRevision: 'github-issues-readonly@1', filterRevision: 'github-issues-overlap-updated-asc@1', configurationSha256: 'a'.repeat(64), state: 'draft', permissionStatus: 'unverified', createdAt: '2026-09-24T00:00:00Z'};
    const probe = {companyId, sourceId: 'source-1', requestId: 'probe-1', permissionStatus: 'verified', coverage: 'complete', coverageReason: ''};
    const poll = {companyId, sourceId: 'source-1', requestId: 'poll-1', scanId: 'scan-1', coverage: 'partial', coverageReason: 'github_rate_limited', coveredThrough: null, pageCount: 1, itemCount: 1, commentScanCount: 1, commentCoverage: 'partial', commentCoverageReason: 'github_rate_limited', replayed: false};

    expect(validateGitHubFeedbackSourceReceipt(source, companyId).success).toBe(true);
    expect(validateGitHubFeedbackSourceReceipt({...source, companyId: 'company-2'}, companyId).success).toBe(false);
    expect(validateGitHubFeedbackProbeReceipt(probe, companyId, 'source-1', 'probe-1').success).toBe(true);
    expect(validateGitHubFeedbackPollReceipt(poll, companyId, 'source-1', 'poll-1').success).toBe(true);
    expect(validateGitHubFeedbackPollReceipt({...poll, sourceId: 'source-2'}, companyId, 'source-1', 'poll-1').success).toBe(false);
  });

  it('validates a company collection policy receipt and both global gates', () => {
    const receipt = {companyId, sourceId: 'source-1', enabled: true, intervalSeconds: 3600, rationale: 'bounded collection', updatedAt: '2026-09-29T00:00:00Z', nextPollAt: '2026-09-29T01:00:00Z', lastAttempt: '', lastReasonCode: '', requestId: 'policy-1', externalEnabled: false, schedulerEnabled: false};
    const expected = {companyId, sourceId: 'source-1', requestId: 'policy-1'};
    expect(validateGitHubFeedbackCollectionPolicyReceipt(receipt, expected).success).toBe(true);
    expect(validateGitHubFeedbackCollectionPolicyReceipt({...receipt, companyId: 'company-2'}, expected).success).toBe(false);
    expect(validateGitHubFeedbackCollectionPolicyReceipt({...receipt, intervalSeconds: 60}, expected).success).toBe(false);
  });

  it('accepts a scoped environment status projection and rejects cross-company records', () => {
    const revision = {
      companyId, revisionId: 'environment-1', missionId: 'mission-1', sourceInputId: 'input-1', sourceInputRevision: '1', projectRootRelative: 'repo', sourceBindingStatus: 'bound', profileId: 'windows-node-npm@1',
      sourceRevisionSha256: 'a'.repeat(64), packageJsonSha256: 'b'.repeat(64), lockfileSha256: 'c'.repeat(64),
      policySha256: 'd'.repeat(64), policyManifest: {schemaVersion: 'project-environment-policy@1', profileId: 'windows-node-npm@1', registryHosts: ['registry.npmjs.org'], installPolicy: 'npm ci --ignore-scripts --no-audit --no-fund', lifecycleScriptsPolicy: 'ignore', networkPolicy: 'registry_allowlist', timeoutMs: 180000, outputLimitBytes: 1048576},
      toolchainSha256: 'e'.repeat(64), executorFingerprintSha256: 'f'.repeat(64), hostFingerprintSha256: '1'.repeat(64), isolationPolicySha256: '2'.repeat(64), executorEvidenceSha256: null, executorEvidenceInputId: null, executorEvidenceInputRevision: null, policyDecision: 'approved',
      executorQualification: 'unqualified', preparationState: 'blocked_unqualified',
      preparationReason: 'environment_executor_not_qualified', preparationRunId: 'run-1', createdAt: '2026-09-24T00:00:00Z',
    };
    expect(validateProjectEnvironmentRevisions([revision], companyId).success).toBe(true);
    expect(validateProjectEnvironmentRevisions([{...revision, executorQualification: 'identity_stale'}], companyId).success).toBe(true);
    expect(validateProjectEnvironmentRevisions([{...revision, executorQualification: 'qualified'}], companyId).success).toBe(false);
    expect(validateProjectEnvironmentRevisions([{...revision, companyId: 'company-2'}], companyId).success).toBe(false);
  });

  it('validates approved service definitions inside an immutable environment policy', () => {
    const revision = {
      companyId, revisionId: 'environment-service-1', missionId: 'mission-1', sourceInputId: 'input-1', sourceInputRevision: '1', projectRootRelative: 'repo', sourceBindingStatus: 'bound', profileId: 'windows-node-npm@1',
      sourceRevisionSha256: 'a'.repeat(64), packageJsonSha256: 'b'.repeat(64), lockfileSha256: 'c'.repeat(64),
      policySha256: 'd'.repeat(64), policyManifest: {schemaVersion: 'project-environment-policy@1', profileId: 'windows-node-npm@1', registryHosts: ['registry.npmjs.org'], installPolicy: 'npm ci --ignore-scripts --no-audit --no-fund', lifecycleScriptsPolicy: 'ignore', networkPolicy: 'registry_allowlist', timeoutMs: 180000, outputLimitBytes: 1048576,
        services: [{id: 'ui', scriptPath: 'ui/server.mjs', probe: {bindAddress: '127.0.0.1', port: 3400, path: '/healthz', expectedStatusCode: 200, expectedBodySha256: 'e'.repeat(64), timeoutMs: 500, leaseDurationMs: 30000}}]},
      toolchainSha256: 'f'.repeat(64), executorFingerprintSha256: null, hostFingerprintSha256: null, isolationPolicySha256: null, executorEvidenceSha256: null, executorEvidenceInputId: null, executorEvidenceInputRevision: null, policyDecision: 'approved', executorQualification: 'unqualified', preparationState: 'blocked_unqualified',
      preparationReason: 'environment_executor_not_qualified', preparationRunId: 'run-1', createdAt: '2026-09-24T00:00:00Z',
    };
    expect(validateProjectEnvironmentRevisions([revision], companyId).success).toBe(true);
    expect(validateProjectEnvironmentRevisions([{...revision, policyManifest: {...revision.policyManifest, services: [{...revision.policyManifest.services[0], probe: {...revision.policyManifest.services[0].probe, bindAddress: '192.0.2.1'}}]}}], companyId).success).toBe(false);
    expect(validateProjectEnvironmentRevisions([{...revision, policyManifest: {...revision.policyManifest, services: [{...revision.policyManifest.services[0], probe: {...revision.policyManifest.services[0].probe, timeoutMs: 100, leaseDurationMs: 100}}]}}], companyId).success).toBe(false);
    expect(validateProjectEnvironmentRevisions([{...revision, policyManifest: {...revision.policyManifest, services: [{...revision.policyManifest.services[0], probe: {...revision.policyManifest.services[0].probe, path: '/' + '界'.repeat(86)}}]}}], companyId).success).toBe(false);
    expect(validateProjectEnvironmentRevisions([{...revision, policyManifest: {...revision.policyManifest, services: [{...revision.policyManifest.services[0], probe: {...revision.policyManifest.services[0].probe, path: '/\u0085'}}]}}], companyId).success).toBe(false);
  });

  it('accepts a Linux Node environment with its deny-all offline policy', () => {
    const revision = {
      companyId, revisionId: 'linux-environment-1', missionId: 'mission-1', sourceInputId: 'input-1', sourceInputRevision: '1', projectRootRelative: 'repo', sourceBindingStatus: 'bound', profileId: 'linux-node-npm@1',
      sourceRevisionSha256: 'a'.repeat(64), packageJsonSha256: 'b'.repeat(64), lockfileSha256: 'c'.repeat(64),
      policySha256: 'd'.repeat(64), policyManifest: {schemaVersion: 'project-environment-policy@1', profileId: 'linux-node-npm@1', registryHosts: ['registry.npmjs.org'], installPolicy: 'npm ci --ignore-scripts --no-audit --no-fund --offline', lifecycleScriptsPolicy: 'ignore', networkPolicy: 'deny_all', timeoutMs: 180000, outputLimitBytes: 1048576},
      toolchainSha256: 'e'.repeat(64), executorFingerprintSha256: 'f'.repeat(64), hostFingerprintSha256: '1'.repeat(64), isolationPolicySha256: '2'.repeat(64), executorEvidenceSha256: null, executorEvidenceInputId: null, executorEvidenceInputRevision: null, policyDecision: 'approved', executorQualification: 'unqualified', preparationState: 'blocked_unqualified',
      preparationReason: 'environment_executor_not_qualified', preparationRunId: 'run-linux-1', createdAt: '2026-09-25T00:00:00Z',
    };
    expect(validateProjectEnvironmentRevisions([revision], companyId).success).toBe(true);
    expect(validateProjectEnvironmentRevisions([{...revision, policyManifest: {...revision.policyManifest, registryHosts: []}}], companyId).success).toBe(false);
    expect(validateProjectEnvironmentRevisions([{...revision, policyManifest: {...revision.policyManifest, networkPolicy: 'registry_allowlist'}}], companyId).success).toBe(false);
  });

  it('accepts a scoped JobRun and local service endpoint, rejecting non-local addresses', () => {
    const job = {
      companyId, jobId: 'job-1', taskId: 'task-1', sessionId: 'session-1', environmentRevisionId: 'environment-1', handoverId: '',
      kind: 'service', serviceId: 'web', state: 'running', readiness: 'ready', exitCode: null, reasonCode: 'none',
      stdoutOffset: '128', stderrOffset: '0', stdoutBytes: '64', stderrBytes: '0', logsTruncated: false, logGap: false,
      logManifestSha256: 'f'.repeat(64),
      serviceEndpoint: {generation: '1', bindAddress: '127.0.0.1', port: '3000', readiness: 'ready', sourceRevisionSha256: 'a'.repeat(64), healthcheckSha256: 'b'.repeat(64), leaseExpiresAt: '2026-09-24T01:00:00Z'},
      createdAt: '2026-09-24T00:00:00Z', updatedAt: '2026-09-24T00:00:01Z',
    };
    expect(validateTaskJobRuns([job], companyId, 'task-1').success).toBe(true);
    expect(validateTaskJobRuns([{...job, serviceId: 'legacy:unattributed'}], companyId, 'task-1').success).toBe(true);
    expect(validateTaskJobRuns([{...job, serviceEndpoint: {...job.serviceEndpoint, bindAddress: '0.0.0.0'}}], companyId, 'task-1').success).toBe(false);
    expect(validateTaskJobRuns([{...job, serviceEndpoint: null}], companyId, 'task-1').success).toBe(false);
    expect(validateTaskJobRuns([{...job, serviceEndpoint: {...job.serviceEndpoint, leaseExpiresAt: null}}], companyId, 'task-1').success).toBe(false);
    expect(validateTaskJobRuns([{...job, readiness: 'unhealthy'}], companyId, 'task-1').success).toBe(false);
    expect(validateTaskJobRuns([{...job, readiness: 'unhealthy', serviceEndpoint: {...job.serviceEndpoint, readiness: 'revoked', leaseExpiresAt: null}}], companyId, 'task-1').success).toBe(true);
    expect(validateTaskJobRuns([{...job, readiness: 'not_ready', serviceEndpoint: null}], companyId, 'task-1').success).toBe(true);
    expect(validateTaskJobRuns([{...job, state: 'exited'}], companyId, 'task-1').success).toBe(false);
    expect(validateTaskJobRuns([{...job, state: 'exited', readiness: 'unhealthy', serviceEndpoint: {...job.serviceEndpoint, readiness: 'unhealthy'}}], companyId, 'task-1').success).toBe(true);
    expect(validateTaskJobRuns([{...job, kind: 'batch', serviceId: '', readiness: 'not_applicable', serviceEndpoint: null}], companyId, 'task-1').success).toBe(true);
    expect(validateTaskJobRuns([{...job, kind: 'batch', serviceEndpoint: null}], companyId, 'task-1').success).toBe(false);
    expect(validateTaskJobRuns([{...job, readiness: 'unhealthy', serviceEndpoint: null}], companyId, 'task-1').success).toBe(false);
    expect(validateTaskJobRuns([{...job, readiness: 'not_applicable', serviceEndpoint: null}], companyId, 'task-1').success).toBe(false);
  });

  it('validates a digest-bound cross-backend handover against Task and company scope', () => {
    const handover = {
      companyId, handoverId: 'handover-1', missionId: 'mission-1', taskId: 'task-1', sourceJobId: 'job-1',
      sourceSessionId: 'session-1', sourceRuntimeIncarnation: 'runtime-windows-1', sourceEnvironmentRevisionId: 'windows-env-1',
      sourceProfileId: 'windows-node-npm@1', targetEnvironmentRevisionId: 'linux-env-1', targetProfileId: 'linux-node-npm@1',
      projectSourceSha256: 'a'.repeat(64), packageJsonSha256: 'b'.repeat(64), lockfileSha256: 'c'.repeat(64),
      workspaceDigest: 'd'.repeat(64), workspaceRevision: 7, taskInputManifestSha256: 'e'.repeat(64),
      targetPolicySha256: 'f'.repeat(64), targetToolchainSha256: '1'.repeat(64), requestId: 'handover-request-1',
      recordSha256: '2'.repeat(64), createdAt: '2026-09-26T00:00:00Z',
    };
    expect(validateTaskCrossBackendHandovers([handover], companyId, 'task-1').success).toBe(true);
    expect(validateTaskCrossBackendHandovers([{...handover, taskId: 'task-2'}], companyId, 'task-1').success).toBe(false);
    expect(validateTaskCrossBackendHandovers([{...handover, targetProfileId: 'windows-node-npm@1'}], companyId, 'task-1').success).toBe(false);
    expect(validateTaskCrossBackendHandovers([{...handover, recordSha256: 'bad'}], companyId, 'task-1').success).toBe(false);
  });

  it('validates JobRun command and log responses against company and job scope', () => {
    const receipt = {companyId, jobId: 'job-1', taskId: 'task-1', kind: 'batch', serviceId: '', state: 'running', readiness: 'not_applicable', reasonCode: 'project_job_running'};
    const logs = {companyId, jobId: 'job-1', manifestSha256: 'a'.repeat(64), content: 'bWFuaWZlc3Q='};
    expect(validateJobRunCommandReceipt(receipt, companyId, 'task-1').success).toBe(true);
    expect(validateJobRunCommandReceipt({...receipt, companyId: 'company-2'}, companyId).success).toBe(false);
    expect(validateJobRunCommandReceipt({...receipt, kind: 'service', serviceId: 'web'}, companyId).success).toBe(true);
    expect(validateJobRunCommandReceipt({...receipt, kind: 'service', serviceId: ''}, companyId).success).toBe(false);
    expect(validateJobRunLogArtifact(logs, companyId, 'job-1').success).toBe(true);
    expect(validateJobRunLogArtifact({...logs, jobId: 'job-2'}, companyId, 'job-1').success).toBe(false);
    expect(validateJobRunLogArtifact({...logs, content: 'not base64!'}, companyId, 'job-1').success).toBe(false);
  });

  it('accepts only short-lived loopback service browser sessions', () => {
    const expiresAt = new Date(Date.now() + 60_000).toISOString();
    const session = {url: `http://127.0.0.1:43123/_polis/open/${'a'.repeat(64)}`, expiresAt};
    expect(validateServiceBrowserSession(session).success).toBe(true);
    expect(validateServiceBrowserSession({...session, url: session.url.replace('127.0.0.1', '192.0.2.1')}).success).toBe(false);
    expect(validateServiceBrowserSession({...session, url: session.url + '?redirect=https://example.com'}).success).toBe(false);
    expect(validateServiceBrowserSession({...session, expiresAt: new Date(Date.now() - 1000).toISOString()}).success).toBe(false);
    expect(validateServiceBrowserSession({...session, expiresAt: new Date(Date.now() + 10 * 60_000).toISOString()}).success).toBe(false);
  });

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

  it('rejects unsupported employee schedule states in the company projection', () => {
    const employee = {
      employeeId: 'emp-planning', displayName: 'Planning', role: 'planning', roleRevision: null, epoch: '1',
      sessionId: null, sessionState: null, profile: null, currentTask: null,
      status: {primary: 'sleeping', tone: 'neutral', reason: 'No pending work.', activeModelRequests: '0', inFlightTools: '0', observedAt: meta.observedAt},
      toolBudget: {limit: null, used: null, remaining: null, quality: 'unavailable'},
      qualification: {status: 'unverified', evidenceId: null, policyRevision: null}, openObligationCount: '0',
    };
    const valid = validateCompanyOverview(companyOverviewWithEmployee({
      ...employee, schedule: {state: 'sleeping', workGeneration: '3', checkedGeneration: '1', nextDueAt: null, pauseReason: ''},
    }), companyId);
    const invalid = validateCompanyOverview(companyOverviewWithEmployee({
      ...employee, schedule: {state: 'executing', workGeneration: '3', checkedGeneration: '1', nextDueAt: null, pauseReason: ''},
    }), companyId);

    expect(valid.success, JSON.stringify(valid)).toBe(true);
    expect(invalid.success).toBe(false);
  });

  it('accepts only a SHA-256 Employee RoleRevision digest', () => {
    const employee = {
      employeeId: 'emp-planning', displayName: 'Planning', role: 'planning', roleRevision: 'a'.repeat(64), epoch: '1',
      sessionId: null, sessionState: null, profile: null, currentTask: null,
      status: {primary: 'sleeping', tone: 'neutral', reason: 'No pending work.', activeModelRequests: '不可得', inFlightTools: '不可得', observedAt: meta.observedAt},
      schedule: null, toolBudget: {limit: null, used: null, remaining: null, quality: 'unavailable'},
      qualification: {status: 'unverified', evidenceId: null, policyRevision: null}, openObligationCount: '0',
    };
    const valid = validateCompanyOverview(companyOverviewWithEmployee(employee), companyId);
    const invalid = validateCompanyOverview(companyOverviewWithEmployee({...employee, roleRevision: 'not-a-digest'}), companyId);
    expect(valid.success, JSON.stringify(valid)).toBe(true);
    expect(invalid.success).toBe(false);
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
