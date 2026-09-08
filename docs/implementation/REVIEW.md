# Read-only implementation review

One read-only development reviewer examined the R0 files; the primary agent remained the only code writer. This was development review, not a Polis runtime employee/model experiment.

Two concrete findings were reproduced and fixed:

1. A retired controller could invoke the public recovery method through its still-live pool and overwrite its successor's incarnation. Recovery is now a private startup path; its reset transaction uses the same physical connection that holds the advisory lock. A closed lease cannot run the transaction. `TestLostLeaseCannotRecoverOverSuccessor` reproduces the old overwrite and verifies the new controller's valid neighbor.
2. `Close` held the lease mutex while waiting for borrowed pool connections, which could themselves be waiting for that mutex in `guard`. It now detaches the lease under the mutex and unlocks before waiting on the pool. `TestCloseReleasesLeaseMutexBeforeWaitingForTransactions` supplies the held-transaction interleaving.

The failing run is retained as `evidence/development/go-tests-review-red.txt`; fixed normal/race runs are retained separately. The same reviewer re-read the fixes and closed both findings, reporting no further definite defects in the exposed slice. This does not imply absence of unknown defects or completion of the full design gates.
