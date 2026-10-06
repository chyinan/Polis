# REQ-40 explicit DeliveryManifest invalidation and withdrawal

Slice281 adds an installation-owner, CSRF-protected command path for appending an `invalidated` or `withdrawn` DeliveryManifest revision.

The command is bound to the exact current manifest revision, preserves immutable Artifact content, records the owner reason in the new revision's disposition audit row, resets feedback state to `not_requested` for that non-deliverable revision, and is idempotent through the existing receipt key. Terminal manifest revisions and stale revisions are rejected. It has no Worker, Mission, Task or external side effect.
