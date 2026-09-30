# Reference domain workflows

Last verified: 2026-09-29

## Purpose

Define domain-specific acceptance rules without inferring content or research quality from software results.

## Contracts

- `ReferenceWorkflows` returns immutable content-operations and research-simulation templates with `not_run` qualification and execution disabled. The Kernel may overlay the latest company-scoped profile decision in the evidence ledger; a qualified evidence decision still does not enable execution without an implemented runtime.
- Content review binds to one immutable draft revision, requires an independent checker and human sampling, and accepts a verified claim only with a pinned source reference.
- Research evaluation binds to a dataset revision and method digest, records a control and risk budget, requires an independent evaluator, and treats negative results as valid findings.
- Research execution must remain simulation-only. Real publication, account access, or trading is outside this package.

## Invariants

- A newer draft revision makes the prior fact check stale.
- Unavailable/conflicting sources stay inconclusive; they are never upgraded to verified.
- A company-scoped global qualification report must cover every profile area and reference the same company/profile revision, exact case evidence digests, and accepted substantive assessment IDs. Case submission, reference review, and per-submission assessment alone never qualify the profile.
- No domain profile is qualified for product claims unless a real owner-reviewed aggregate report meets its own quality, recovery, cost, organization-benefit, and content-intervention evidence requirements. Research and content results never transfer between domains.
