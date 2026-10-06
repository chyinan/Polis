# REQ-14 ActionIntent Operations projection

Slice296 exposes bounded denied generic ActionIntent audit records in Workbench Operations. The projection is Company-scoped, latest-event filtered, digest-only and read-only; it does not issue permits or execute external actions.
