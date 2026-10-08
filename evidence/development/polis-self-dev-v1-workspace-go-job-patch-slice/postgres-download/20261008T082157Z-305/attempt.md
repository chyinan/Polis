# PostgreSQL 18 package preparation attempt

The second invocation again remained in the curl transfer for the official PostgreSQL signing key despite a 45-second per-transfer timeout and two retries. It was interrupted after roughly three minutes; the temporary `/tmp/polis-pg18-download.*` directory was removed by the script EXIT cleanup. No key was installed or copied, no Debian package was downloaded, no `.tools/pg` extraction occurred, and no global apt source or package state changed.

A separate bounded verbose GET of the same official URL succeeded from WSL in about two seconds. The downloader has been narrowed to HTTP/1.1, one retry, and a 30-second per-transfer timeout before the next attempt.
