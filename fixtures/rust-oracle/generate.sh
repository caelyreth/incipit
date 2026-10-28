#!/bin/sh
set -eu

SHOREWARD=${SHOREWARD:-/Users/caelyreth/Craftbench/TideBook/shoreward}
NEVERLAND=${NEVERLAND:-/Users/caelyreth/Craftbench/TideBook/neverland}
RUST_FLAKE=${RUST_FLAKE:-$HOME/.config/flakes/single-rust}

cd "$SHOREWARD"
nix develop "$RUST_FLAKE" --command cargo run \
  -p shoreward-engine --bin go_oracle -- all \
  "$NEVERLAND/fixtures/rust-oracle/v1.json"

shasum -a 256 "$NEVERLAND/fixtures/rust-oracle/v1.json"
