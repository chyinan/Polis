# Polis parser fork

This directory is a local fork of `github.com/giraffesyo/pdf` v0.7.0, upstream
commit `8a26970ec43650cea9f50bed558edf9c26a10479`. Its upstream MIT `LICENSE`
is preserved. `go.mod` replaces only that exact module path with this directory,
so imports of the upstream module's `internal/*` packages remain within their
original module boundary.

Keep changes limited to bounded parser capabilities needed by Polis. Preserve
the existing `Extract` behavior and historical snapshot verification unless a
new explicitly versioned option/schema is selected. When refreshing upstream,
compare the full source and tests; do not overwrite Polis extensions.

The PDF fixtures use Git's binary attribute so whitespace inside byte-exact
xref tables is not mistaken for source whitespace during diff checks.

Slice339 adds opt-in `Options.AuditContentOperators`, which inventories sorted
unique operator names with fixed count/name-length caps. It is diagnostic only;
it exposes no operands or graphics state and is not evidence that a page can be
rendered. The zero-value option preserves upstream extraction behavior.

Run this module's tests with `scripts/test-pdf-parser-fork.sh` from the repo
root. The root Go suite is also required because `internal/intake` consumes this
fork through the replacement.
