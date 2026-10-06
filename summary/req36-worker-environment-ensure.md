# REQ-36 fake-only Worker environment ensure

Slice279 adds isolated fake-only product surface `polis-product-tool-surface@16`, extending @14 with `polis_environment_ensure`.

The tool accepts only an exact `revision_id` from the current Mission's environment catalog. Kernel rechecks the WorkerSession, current Task/Mission and revision Mission inside the same transaction that records the idempotent preparation request; receipt replay re-runs the binding guard through `TXWrite`. Host, command, package, registry and network selection remain unavailable to the Worker. Existing Control/Workbench preparation and executor gates remain authoritative, and real provider @4 is unchanged.
