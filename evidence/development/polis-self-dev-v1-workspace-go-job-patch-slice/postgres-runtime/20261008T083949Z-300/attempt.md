# Disposable PostgreSQL 18 harness attempt

The harness created an isolated PostgreSQL 18.6 Unix-socket-only cluster and a unique test database. The first database name did not use the repository's required `polis_r0_` prefix, so `cmd/polis migrate` refused to run before applying migrations. The harness stopped the exact temporary cluster and removed only its `/tmp/polis-self-dev-go-pg.*` directory. No existing database or cluster was contacted or changed.
