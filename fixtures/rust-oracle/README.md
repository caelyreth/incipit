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

## Fixture generations

`v1.json` is immutable evidence originally generated from the Rust engine at
`3fff58e089e57529bc725935cd37a2da63071f0b`. It is retained byte-for-byte and
must not be regenerated.

`v2.json` is generated from the committed Rust exporter and records its exact
source revision in every fixture:

```sh
VERSION=v2 fixtures/rust-oracle/generate.sh
```

The generator refuses to run when the Rust exporter has uncommitted changes.

`v1` covers:

- UTF-8/UTF-16/scalar boundary conversions;
- native UTF-16 query alignment;
- anchor rebasing under insertions, replacements, and deletions;
- inline mark rebasing, identity, level changes, and deletion;
- semantic character and paragraph counts.
- seeded document traces with exact revisions, text, mark IDs, ranges, and levels;
- exact generated markup projection text and segment correspondence.

`v2` adds focused regression evidence for:

- ICU word-like classifications for underscore, numeric punctuation, middle
  dot, Thai dictionary segmentation, and a standalone combining mark;
- final compound IME replacement inside a generated strong pair;
- undo restoration of a marker-only projected selection;
- the `allow-strong = false` command gate and exact rejection;
- Unicode `Other_Alphabetic` pair and typography decisions for `U+0345`;
- kernel undeclared document read/write variants and user-facing formatting.

Go consumers live in `engine/model/rust_oracle_test.go`,
`engine/model/rust_trace_test.go`, `engine/editor/rust_oracle_test.go`, and
the focused `*_rust_oracle_v2_test.go` files.

