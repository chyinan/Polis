#!/usr/bin/env python3
"""Verify original delivered files without network or package dependencies."""
from pathlib import Path
import hashlib, sys
root = Path(__file__).resolve().parent
bad = []
count = 0
for line in (root / "SHA256SUMS.txt").read_text(encoding="utf-8").splitlines():
    expected, name = line.split("  ", 1)
    path = (root / name).resolve()
    if not path.is_relative_to(root) or not path.is_file():
        bad.append(name + ": missing or invalid path")
        continue
    actual = hashlib.sha256(path.read_bytes()).hexdigest()
    count += 1
    if actual != expected:
        bad.append(name + ": checksum mismatch")
for problem in bad:
    print(problem)
print(f"{count} files checked; {len(bad)} problem(s).")
sys.exit(1 if bad else 0)
