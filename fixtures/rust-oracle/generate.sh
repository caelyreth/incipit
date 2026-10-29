#!/bin/sh
set -eu

SHOREWARD=${SHOREWARD:-/Users/caelyreth/Craftbench/TideBook/shoreward}
NEVERLAND=${NEVERLAND:-/Users/caelyreth/Craftbench/TideBook/neverland}
RUST_FLAKE=${RUST_FLAKE:-$HOME/.config/flakes/single-rust}
VERSION=${VERSION:-v2}
OUTPUT=${OUTPUT:-$NEVERLAND/fixtures/rust-oracle/$VERSION.json}

case "$VERSION" in
  v1|v2) ;;
  *)
    echo "unsupported oracle version: $VERSION" >&2
    exit 2
    ;;
esac

if [ -n "$(git -C "$SHOREWARD" status --porcelain -- crates/shoreward-engine/src/bin/go_oracle.rs)" ]; then
  echo "commit the Rust exporter before generating immutable oracle evidence" >&2
  exit 1
fi

SHOREWARD_ORACLE_COMMIT=${SHOREWARD_ORACLE_COMMIT:-$(git -C "$SHOREWARD" rev-parse HEAD)}
export SHOREWARD_ORACLE_COMMIT

cd "$SHOREWARD"
nix develop "$RUST_FLAKE" --command cargo run \
  -p shoreward-engine --bin go_oracle -- "$VERSION" \
  "$OUTPUT"

shasum -a 256 "$OUTPUT"
