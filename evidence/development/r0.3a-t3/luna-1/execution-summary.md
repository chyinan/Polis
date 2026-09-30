# R0.3A-T3 Backend-only real requalification

Result: `INCONCLUSIVE`.

The new T3 allowance reserved exactly one `gpt-5.6-luna/medium` Backend turn and zero High turns. The Backend reached `turn/started`, then received structured `responseStreamDisconnected` events with `willRetry=true` before first valid output. The phase-aware policy correctly did not terminate at the old 30-second grace. It waited until the configured first-valid-output deadline, approximately `90.005327644s`, then bounded the turn and confirmed the worker stop proof.

No Polis tool call, receipt, token usage update, ContractRevision, Message, Obligation, workspace mutation, checkpoint, or artifact was produced. No Frontend, successor, Reviewer, High, retry, or reset occurred. There was no unresolved business side effect.

The initial runner classification was corrected from business `failed` to transport `INCONCLUSIVE` using the immutable protocol facts; the Backend turn was not rerun.
