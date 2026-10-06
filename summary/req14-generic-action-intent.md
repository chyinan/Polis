# REQ-14 generic ActionIntent default-deny foundation

Slice290 adds Schema120 generic ActionIntent persistence with strict ResourceKey and digest validation, exact Task/WorkerSession scope and idempotency conflict handling. It only records a durable denied event because no generic resource policy or DispatchPermit executor is qualified.
