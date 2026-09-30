import {describe, expect, it} from 'vitest';
import {buildDomainProfileQualificationReport} from './domain-profile-qualification-report';
import type {DomainEvidenceSubstantiveAssessmentRecordView, DomainWorkflowProfileView} from './workbench';

const profile: DomainWorkflowProfileView = {
  id: 'content-operations-reference', revision: 'content-operations@1', domain: 'content_operations',
  qualificationStatus: 'not_run', executionEnabled: false, stages: ['draft', 'review'],
  requiredEvidence: ['quality', 'intervention', 'recovery', 'cost', 'organization_benefit'],
};

function acceptedCase(areaAssessments: DomainEvidenceSubstantiveAssessmentRecordView['areaAssessments']): {
  recordId: string;
  profileId: string;
  profileRevision: string;
  evidenceDigest: string;
  assessment: DomainEvidenceSubstantiveAssessmentRecordView | null;
} {
  const assessment = {
    companyId: 'company-1', assessmentId: 'domain-assessment-1', recordId: 'domain-record-1', evidenceDigest: 'a'.repeat(64),
    outcome: 'evidence_accepted' as const, reviewerEmployeeId: 'emp-review', areaAssessments,
    previewedEvidence: [], requestId: 'domain-assessment-request-1', createdAt: '2026-09-29T04:00:00Z',
  } satisfies DomainEvidenceSubstantiveAssessmentRecordView;
  return {recordId: assessment.recordId, profileId: profile.id, profileRevision: profile.revision, evidenceDigest: assessment.evidenceDigest, assessment};
}

describe('domain profile qualification report', () => {
  it('builds a pinned aggregate report only when every required area has an accepted case', () => {
    const cases = [acceptedCase(profile.requiredEvidence.map(area => ({area, outcome: 'accepted' as const, rationale: 'reviewed'})))];
    const result = buildDomainProfileQualificationReport(profile, cases);
    expect(result.success).toBe(true);
    if (!result.success) return;
    const report = JSON.parse(result.content) as {
      schemaVersion: string;
      profileId: string;
      profileRevision: string;
      areas: ReadonlyArray<{area: string; cases: ReadonlyArray<{recordId: string; evidenceDigest: string; assessmentId: string}>}>;
    };
    expect(report.schemaVersion).toBe('r3-domain-profile-qualification@1');
    expect(report.profileId).toBe(profile.id);
    expect(report.profileRevision).toBe(profile.revision);
    expect(report.areas.map(area => area.area)).toEqual(profile.requiredEvidence);
    expect(report.areas.every(area => area.cases.length === 1 && area.cases[0].assessmentId === 'domain-assessment-1')).toBe(true);
  });

  it('reports missing or insufficient areas without manufacturing qualification evidence', () => {
    const partial = acceptedCase([{area: 'quality', outcome: 'accepted', rationale: 'reviewed'}]);
    const result = buildDomainProfileQualificationReport(profile, [partial]);
    expect(result).toEqual({success: false, missingAreas: ['intervention', 'recovery', 'cost', 'organization_benefit']});
    const insufficient = acceptedCase([{area: 'quality', outcome: 'insufficient', rationale: 'more evidence required'}]);
    expect(buildDomainProfileQualificationReport(profile, [insufficient])).toEqual({success: false, missingAreas: profile.requiredEvidence});
  });

  it('pins only distinct profile-matching accepted cases', () => {
    const accepted = acceptedCase(profile.requiredEvidence.map(area => ({area, outcome: 'accepted' as const, rationale: 'reviewed'})));
    const result = buildDomainProfileQualificationReport(profile, [accepted, accepted]);
    expect(result.success).toBe(true);
    if (result.success) {
      const report = JSON.parse(result.content) as {areas: ReadonlyArray<{cases: ReadonlyArray<{recordId: string}>}>};
      expect(report.areas.every(area => area.cases.length === 1)).toBe(true);
    }
    const wrongProfile = {...accepted, profileId: 'research-simulation-reference', profileRevision: 'research-simulation@1'};
    expect(buildDomainProfileQualificationReport(profile, [wrongProfile])).toEqual({success: false, missingAreas: profile.requiredEvidence});
  });
});
