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

