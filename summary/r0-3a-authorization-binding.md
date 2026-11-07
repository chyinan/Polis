# R0.3A authorization binding hardening

## Result

Local authorization binding hardening passed. No Medium, High, allowance,
provider egress or Backend execution was started. T21B remains unchanged,
including its raw empty `t21_fingerprint` field.

## Binding contract

Future business startup now requires an exact `BusinessAuthorizationBinding`
containing opaque `execution_fingerprint`, `employee_id`, `problem_key`,
`purpose`, `model`, `profile`, `effort` and `allowance_limits` (Medium, High,
concurrency). The pure authorization core denies stale qualification and every
missing/mismatched field with a distinct reason code.

The Backend-only `R03AT2` path invokes read-only binding revalidation after the
existing qualification gate and before problem-key persistence, allowance
creation or worker creation. The command accepts the future binding path and
current execution fingerprint through `POLIS_BUSINESS_AUTHORIZATION_BINDING`
and `POLIS_BUSINESS_EXECUTION_FINGERPRINT`; missing values are denied at
startup.

## Regression coverage

Tests pass for exact fingerprint allow; empty historical T21B fingerprint;
wrong fingerprint; stale qualification; wrong employee; wrong ProblemKey;
wrong purpose; model/profile/effort drift; allowance-limit drift; missing
binding fields; shell JSON binding load; and Backend worker-start context
construction. No historical evidence was rewritten.

## Next stage

Infrastructure is frozen. The next step is a separately authorized real
Backend allowance bound to the exact current execution fingerprint and
employee/purpose/profile/limits. No T21C/T21D or further transport/tool
forensics are planned unless the Backend exposes a blocking infrastructure
defect.
