# REQ-37 BorrowerLease tool projection

Slice284 exposes the Schema116 control-plane BorrowerLease lifecycle through fake-only product surface `polis-product-tool-surface@17`. `jobs_borrow` accepts one exact ready service generation; `jobs_touch` refreshes only the bounded idle grace; `jobs_release` ends one exact borrower lease. All three require the current Task and WorkerSession and never start, stop or extend the owner service endpoint.

Owner/session stop, recovery reconciliation, endpoint revocation and generation changes append durable revocation events. Native service/restart and Provider qualification remain separate.
