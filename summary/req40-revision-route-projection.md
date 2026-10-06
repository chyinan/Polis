# REQ-40 revision-route projection

Slice289 adds the optional Schema119 `revisionRoutes` read projection. It joins route records to immutable Manifest/disposition rows, validates `changes_requested`, bounds results to 16, and exposes pending/successor/task-ready states read-only in Workbench. It does not create or execute work.
