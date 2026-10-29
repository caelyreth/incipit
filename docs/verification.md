# Verification

Updated on 2026-10-28 from `codex/native-contracts` after adding the
immutable Rust oracle v2 corpus.

Verified commands:

```text
go test -count=1 ./engine/editor ./engine/kernel passed
go vet ./...                                      passed
cargo test -q -p shoreward-engine                 passed
```

The full `go test -count=1 ./...` run at this checkpoint reaches an unrelated
in-progress failure in `native/documentview`:
`TestOffscreenCaretDoesNotScheduleBreathing`. The oracle packages pass, but the
full suite must be rerun after the concurrent native work lands.

## Rust oracle v2

`fixtures/rust-oracle/v2.json` was generated from Rust exporter commit
`2b37fca6a1f43764e4a122d013a29d1ee859e4fe`:

```text
SHA-256 9ceb7feeac7bc27e0abeb63b4320ebb9fbdd87cf97538a5b0dc688dcf74ee1b4
```

The fixture records:

- ICU word-like results for `_`, `123,456`, `a·b`, Thai dictionary
  segmentation, and a standalone combining acute accent;
- final compound IME replacement inside a generated strong pair;
- undo restoration of a marker-only projected selection;
- `allow-strong = false` control state and exact command rejection;
- Unicode `Other_Alphabetic` pair and typography decisions for `U+0345`;
- kernel undeclared document read/write variants and exact user-facing text.

The Go consumers are in `engine/editor/rust_oracle_v2_test.go` and
`engine/kernel/rust_oracle_v2_test.go`. Current session behavior matches the
compound IME, marker-only undo, and strong-rejection records. The word-like and
`Other_Alphabetic` pair/typography records deliberately mark the remaining
implementation divergences as known skips until those policies are corrected.
The kernel consumer currently verifies the immutable error contract; the Go
kernel error kinds must be aligned before it can be a behavioral comparison.

## Proven

- Semantic document transactions, anchors, marks, projection, policy runtime,
  deterministic dispatch, observer validation, and rollback.
- Grapheme, word, sentence, CJK, typography, pair, markup, projected-edit,
  composition, history, feedback, and statistics behavior.
- UTF-16 native boundaries, projected replacement, IME preview retention,
  cancellation, and policy-decline handling.
- Keymap routing, platform primary modifiers, composition gates, menus,
  clipboard behavior, and command-palette action routing.
- Page lifecycle, parked sessions, configuration publication, settings
  reducers, and per-page scroll restoration.
- MyGo paragraph-local shaping, retained layout, caret/selection geometry,
  custom painting, cursor breathing, viewport springs, caret reveal, pointer
  drag/double-click selection, native sidebar navigation, and accessible input
  roles/focus/value.
- Immutable Rust v1 coordinate, anchor, mark, projection, metric, and seeded
  trace evidence, plus the focused v2 regression corpus above.

## Not Proven

- Full Rust/Go behavioral equivalence. The focused v2 corpus exposes known
  ICU word-like and `Other_Alphabetic` policy differences, and no randomized
  current-source differential suite is present yet.
- Behavioral parity for kernel undeclared document access until the Go error
  variants and formatting contract are implemented and consumed by tests.
- Native IME, accessibility, focus, window lifecycle, and visual acceptance on
  macOS, Windows, and Linux.
- Large-document performance and paragraph-cache invalidation benchmarks.
- Cross-platform packaging or executable release artifacts.

Do not describe the project as release-complete until the unchecked roadmap
items have authoritative evidence.
