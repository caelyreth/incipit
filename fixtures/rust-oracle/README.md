# Rust Oracle Fixtures

This directory retains machine-readable behavior traces generated from the Rust
Shoreward implementation before or during the Go port.

Fixtures must include:

- coordinate conversion tables;
- anchor and mark rebase tables;
- projection construction and mapping cases;
- exact projection patches;
- control-plan traces;
- transaction and publication outcomes;
- randomized editor traces with fixed seeds.

Raw captures are ignored under `raw/`. Committed fixtures should be small,
deterministic, and referenced by Go tests.

## Current fixture

`v1.json` is generated from the Rust engine at
`3fff58e089e57529bc725935cd37a2da63071f0b` with:

```sh
fixtures/rust-oracle/generate.sh
```

It covers:

- UTF-8/UTF-16/scalar boundary conversions;
- native UTF-16 query alignment;
- anchor rebasing under insertions, replacements, and deletions;
- inline mark rebasing, identity, level changes, and deletion;
- semantic character and paragraph counts.
- seeded document traces with exact revisions, text, mark IDs, ranges, and levels;
- exact generated markup projection text and segment correspondence.

Go consumers live in `engine/model/rust_oracle_test.go`,
`engine/model/rust_trace_test.go`, and `engine/editor/rust_oracle_test.go`.

