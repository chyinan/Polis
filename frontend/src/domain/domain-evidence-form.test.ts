// pattern: Functional Core

import {describe, expect, it} from 'vitest';
import {buildDomainEvidenceItems, type DomainEvidenceDraftFields} from './domain-evidence-form';

const areas = ['quality', 'intervention', 'recovery', 'cost', 'organization_benefit'] as const;

function completeDraft(): DomainEvidenceDraftFields {
  return {
    artifactId: 'mission-input-report', revision: '2', sha256: 'a'.repeat(64),
    methodSHA256: 'b'.repeat(64), assessedByEmployeeId: 'emp-review',
  };
}

describe('domain evidence submission form core', () => {
  it('maps complete evidence rows while keeping untouched required areas absent', () => {
    const result = buildDomainEvidenceItems({requiredEvidence: areas}, {quality: completeDraft()});

    expect(result).toEqual({
      success: true,
      evidence: [{area: 'quality', artifact: {id: 'mission-input-report', revision: 2, sha256: 'a'.repeat(64)}, methodSHA256: 'b'.repeat(64), assessedByEmployeeId: 'emp-review'}],
    });
  });

  it('rejects partially filled rows and malformed revisions or digests', () => {
    const partial = buildDomainEvidenceItems({requiredEvidence: areas}, {quality: {...completeDraft(), methodSHA256: ''}});
    expect(partial).toEqual({success: false, reason: 'complete_or_clear_evidence_row:quality'});

    const badRevision = buildDomainEvidenceItems({requiredEvidence: areas}, {quality: {...completeDraft(), revision: '0'}});
    expect(badRevision).toEqual({success: false, reason: 'invalid_evidence_revision:quality'});

    const badDigest = buildDomainEvidenceItems({requiredEvidence: areas}, {quality: {...completeDraft(), sha256: 'not-a-digest'}});
    expect(badDigest).toEqual({success: false, reason: 'invalid_evidence_digest:quality'});
  });
});
