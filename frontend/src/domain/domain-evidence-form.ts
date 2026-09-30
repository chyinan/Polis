// pattern: Functional Core

import type {DomainEvidenceAreaView, DomainEvidenceItemView, DomainWorkflowProfileView} from './workbench';

export type DomainEvidenceDraftFields = Readonly<{
  artifactId: string;
  revision: string;
  sha256: string;
  methodSHA256: string;
  assessedByEmployeeId: string;
}>;

export type DomainEvidenceFormResult =
  | Readonly<{success: true; evidence: ReadonlyArray<DomainEvidenceItemView>}>
  | Readonly<{success: false; reason: string}>;

export function buildDomainEvidenceItems(
  profile: Pick<DomainWorkflowProfileView, 'requiredEvidence'>,
  drafts: Readonly<Partial<Record<DomainEvidenceAreaView, DomainEvidenceDraftFields>>>,
): DomainEvidenceFormResult {
  const evidence: Array<DomainEvidenceItemView> = [];
  for (const area of profile.requiredEvidence) {
    const draft = drafts[area];
    if (draft === undefined) continue;
    const values = {
      artifactId: draft.artifactId.trim(),
      revisionText: draft.revision.trim(),
      sha256: draft.sha256.trim(),
      methodSHA256: draft.methodSHA256.trim(),
      assessedByEmployeeId: draft.assessedByEmployeeId.trim(),
    };
    if (Object.values(values).every(value => value === '')) continue;
    if (Object.values(values).some(value => value === '')) return {success: false, reason: `complete_or_clear_evidence_row:${area}`};
    const revision = Number(values.revisionText);
    if (!Number.isSafeInteger(revision) || revision <= 0) return {success: false, reason: `invalid_evidence_revision:${area}`};
    if (!/^[0-9a-f]{64}$/.test(values.sha256) || !/^[0-9a-f]{64}$/.test(values.methodSHA256)) return {success: false, reason: `invalid_evidence_digest:${area}`};
    evidence.push({
      area,
      artifact: {id: values.artifactId, revision, sha256: values.sha256},
      methodSHA256: values.methodSHA256,
      assessedByEmployeeId: values.assessedByEmployeeId,
    });
  }
  return {success: true, evidence};
}
