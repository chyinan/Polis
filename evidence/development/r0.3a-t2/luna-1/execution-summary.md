# R0.3A-T2 Backend-only real requalification

Result: `INCONCLUSIVE`.

The new independent allowance was created with one Medium and zero High. The single real `gpt-5.6-luna/medium` Backend turn started, reached `turn/started`, then received structured `responseStreamDisconnected` events with `willRetry=true`. No Polis dynamic tool call, receipt, ContractRevision, Message, Obligation, workspace mutation, checkpoint, or artifact was produced. The reconnect grace expired after approximately 30 seconds; the worker process was stopped and stop proof was confirmed.

No Frontend, successor, Reviewer, or High turn was started. No retry was attempted. The provider supplied no token usage updates; token counts remain zero/unavailable. The native protocol is preserved at the session `protocol.jsonl` path beside this file.

The first generated result classified the reconnect deadline as business `failed`; that local classification was corrected to `INCONCLUSIVE` from the immutable protocol facts without rerunning the turn. There was no unresolved business side effect because zero Polis tool calls occurred.
