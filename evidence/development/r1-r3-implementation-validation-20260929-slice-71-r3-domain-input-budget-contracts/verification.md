# Slice 71 verification — R3 domain input and budget contracts

Date: 2026-09-29  
Schema: 50 (no migration)  
Status: pure acceptance-contract tests passed; domain workflow execution remains disabled.

## Implemented

- Content review can be evaluated against a caller-supplied, company-tagged source catalog. Verified claim references must match the exact MissionInput ID, revision and digest; a catalog entry from another company is rejected.
- Human sample evidence binds the sample-plan MissionInput, draft revision/body digest, reviewer and an explicit unique/non-empty list of current critical claims. No sample rate or content-quality threshold was introduced.
- Research protocols/results now pin a deterministic seed, declare risk-budget units, report positive consumed units and bind an output digest. Validation rejects changed seeds, missing output digests and consumption above the frozen budget. A separate validator binds the dataset and method source to exact same-company MissionInput revisions.
- Negative simulated research outcomes remain accepted when protocol, budget, output and independent evaluation evidence are complete.

## Verification

- Red/green tests added for authorized content sources, cross-company source rejection, source digest mismatch, draft-stale sampling, sampling-plan digest mismatch, unknown/duplicate sampled claims, same-company research dataset/method inputs, seed mismatch, output digest requirement and risk-budget overflow.
- `rtk bash scripts/go.sh test ./internal/domainworkflow -count=1` — passed.
- `rtk bash scripts/go.sh test ./... -count=1` — passed.
- Independent static code review found no outstanding Critical, Important or Minor findings. Review confirmed valid `inconclusive` outcomes remain inconclusive after the added evidence checks.

## Limits

- These are pure validation contracts, not persistent draft/review/publish/correction/feedback business operations. No Workbench command or page invokes the new content validators yet.
- The research checks validate a supplied receipt; they do not execute a simulation or prove claimed resource consumption. No built-in statistical method, success threshold or domain result is inferred.
- R3 reference profiles remain `not_run` and `executionEnabled=false`. No actual content/research evidence, external source, publisher or research account was used.
