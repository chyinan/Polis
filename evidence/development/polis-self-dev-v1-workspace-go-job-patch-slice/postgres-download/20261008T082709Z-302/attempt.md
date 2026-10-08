# PostgreSQL 18 package preparation attempt

This attempt successfully downloaded the official PostgreSQL PGDG key, verified its documented fingerprint, and completed signed apt metadata refresh using the temporary Jammy/PGDG source list. It then stopped at the candidate-version decision with a shell syntax error before requesting package downloads. No `.deb` was copied or extracted; no system apt source or package state was changed.

GnuPG initialized an empty user-level `~/.gnupg` keybox/trustdb during this attempt because its temporary home was configured only after the process had started. That directory was left untouched. Subsequent runs now set `GNUPGHOME` inside the exact disposable `/tmp` directory so it is removed by the script's guarded cleanup.
