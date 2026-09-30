# R0.6 D checkpoint — Human intervention / operator controls

> Date: 2026-09-21  
> Status: PASSED targeted baseline

## Delivered

- Added durable `operator_instructions` records scoped to Company/Mission with optional Task and Employee targets.
- Added a formal command path for human instructions; active Mission is required, target IDs are checked server-side, and request IDs use the existing receipt/event transaction boundary.
- Instruction state starts at `pending`; the API and UI do not claim employee application until an authoritative future application record exists.
- Added Activity event emission, read projection, HTTP list/create endpoints, and Mission-page composer/history with loading, error, empty and pending states.
- Did not expose the internal `TXSetPaused` flip as Pause/Resume; no unsupported pause semantics are claimed.

## Verification evidence

- `rtk bash scripts/go.sh test ./internal/kernel ./internal/control ./internal/workbench ./cmd/polis` — passed.
- `rtk npm run typecheck` from `frontend/` — passed.
- `rtk npm test -- --run src/domain/workbench-validation.test.ts` — 11 tests passed.
- Disposable PostgreSQL 18 cluster under `/tmp/polis-r06-d-pg` — migration `000010_r06_operator_instructions.sql` applied; `TestOperatorInstructionIsPersistedForActiveMission` passed.
- The same disposable cluster re-ran `TestPostgresStreamReplaysCompanySequenceEvents` — passed.

## Known boundaries

- `pending` is honest acceptance state, not proof that a worker has already applied the instruction.
- Pause/Resume remains deferred until the runtime has a formal product contract.
