# PostgreSQL 18 package preparation attempt

The first download-script invocation was interrupted after its PostgreSQL signing-key `curl` process remained active for approximately two minutes without producing output. The process was stopped; its exact `/tmp/polis-pg18-download.*` temporary directory was removed by the script's EXIT cleanup. No `.deb` was copied, no PostgreSQL files were extracted, no apt source/index configuration was written to the system, and no database server or cluster was started.

The downloader now sets explicit connect and total-transfer timeouts before retrying the same official PostgreSQL key URL.
