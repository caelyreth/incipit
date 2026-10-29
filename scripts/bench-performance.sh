#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out="${1:-$root/references/performance-$(date +%F)}"
benchtime="${BENCHTIME:-1s}"
count="${BENCH_COUNT:-3}"

mkdir -p "$out"
cd "$root"

{
  printf 'date=%s\n' "$(date -Iseconds)"
  printf 'go=%s\n' "$(go version)"
  printf 'uname=%s\n' "$(uname -a)"
  printf 'cpu=%s\n' "$(sysctl -n machdep.cpu.brand_string 2>/dev/null || true)"
  printf 'benchtime=%s\n' "$benchtime"
  printf 'count=%s\n' "$count"
} | tee "$out/environment.txt"

GOMAXPROCS=1 go test -run '^$' \
  -bench '^BenchmarkSessionTyping(LongProse|HeavyPolicies)$' \
  -benchmem -benchtime="$benchtime" -count="$count" \
  ./engine/editor | tee "$out/editor-session.txt"

go test -run '^$' \
  -bench '^Benchmark(FirstLayout|RetainedLayout|TypingLastParagraph|CaretGeometry|SelectionGeometry)$' \
  -benchmem -benchtime="$benchtime" -count="$count" \
  ./native/textlayout | tee "$out/native-textlayout.txt"

go test -run '^$' \
  -bench '^Benchmark(TypingFrameEndToEnd|HeadlessPaint)$' \
  -benchmem -benchtime="$benchtime" -count="$count" \
  ./native/documentview | tee "$out/native-documentview.txt"

go test -run '^$' \
  -bench '^BenchmarkPolicy' \
  -benchmem -benchtime="$benchtime" -count="$count" \
  ./engine/kernel | tee "$out/kernel-policy.txt"
