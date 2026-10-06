# REQ-40 delivery completion UI

- Slice 276 exposes the existing owner/CSRF `delivery/complete` backend command through `WorkbenchApi`, `useCompleteDurableDeliveryManifest`, receipt validation and the real Workbench delivery panel.
- The form is shown only for a real API and an `assembling` Manifest. It collects bounded source/build/instruction/limitation/license evidence and never implies acceptance or starts execution.
- Legacy delivery responses without `feedbackBacklog` remain compatible through client normalization.
