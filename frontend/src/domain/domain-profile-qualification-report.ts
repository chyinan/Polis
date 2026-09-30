import type {DomainEvidenceAreaView, DomainEvidenceRecordView, DomainWorkflowProfileView} from './workbench';

const QUALIFICATION_REPORT_SCHEMA = 'r3-domain-profile-qualification@1';
const MAX_QUALIFICATION_CASE_REFERENCES = 100;
type DomainProfileQualificationCaseCandidate = Pick<DomainEvidenceRecordView, 'recordId' | 'profileId' | 'profileRevision' | 'evidenceDigest' | 'assessment'>;

type DomainProfileQualificationCaseReference = Readonly<{
  recordId: string;
  evidenceDigest: string;
  assessmentId: string;
}>;

type DomainProfileQualificationReport = Readonly<{
  schemaVersion: typeof QUALIFICATION_REPORT_SCHEMA;
  profileId: string;
  profileRevision: string;
  areas: ReadonlyArray<Readonly<{area: DomainEvidenceAreaView; cases: ReadonlyArray<DomainProfileQualificationCaseReference>}>>;
}>;

export type DomainProfileQualificationReportResult =
  | Readonly<{success: true; content: string}>
  | Readonly<{success: false; missingAreas: ReadonlyArray<DomainEvidenceAreaView>}>
;

export function buildDomainProfileQualificationReport(
  profile: DomainWorkflowProfileView,
  submissions: ReadonlyArray<DomainProfileQualificationCaseCandidate>,
): DomainProfileQualificationReportResult {
  const maxCasesPerArea = Math.floor(MAX_QUALIFICATION_CASE_REFERENCES / profile.requiredEvidence.length);
  const casesByArea = new Map<DomainEvidenceAreaView, Map<string, DomainProfileQualificationCaseReference>>();
  for (const area of profile.requiredEvidence) casesByArea.set(area, new Map());

  for (const submission of submissions) {
    const assessment = submission.assessment;
    if (submission.profileId !== profile.id || submission.profileRevision !== profile.revision || assessment === null ||
      assessment.recordId !== submission.recordId || assessment.evidenceDigest !== submission.evidenceDigest || assessment.outcome !== 'evidence_accepted') {
      continue;
    }
    for (const areaAssessment of assessment.areaAssessments) {
      if (areaAssessment.outcome !== 'accepted') continue;
      const areaCases = casesByArea.get(areaAssessment.area);
      if (areaCases === undefined || areaCases.size >= maxCasesPerArea || areaCases.has(submission.recordId)) continue;
      areaCases.set(submission.recordId, {
        recordId: submission.recordId,
        evidenceDigest: submission.evidenceDigest,
        assessmentId: assessment.assessmentId,
      });
    }
  }

  const areas = profile.requiredEvidence.map(area => ({area, cases: [...(casesByArea.get(area)?.values() ?? [])]}));
  const missingAreas = areas.filter(area => area.cases.length === 0).map(area => area.area);
  if (missingAreas.length > 0) return {success: false, missingAreas};

  const report: DomainProfileQualificationReport = {
    schemaVersion: QUALIFICATION_REPORT_SCHEMA,
    profileId: profile.id,
    profileRevision: profile.revision,
    areas,
  };
  return {success: true, content: `${JSON.stringify(report, null, 2)}\n`};
}
