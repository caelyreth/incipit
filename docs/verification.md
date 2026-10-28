# Verification

Verified at `9cda264` before this status update:

```text
go test ./...          passed
go vet ./...           passed
go test -race ./...    passed at 4898018
```

The tree has 99 implementation commits and 348 test functions. The engine,
application, and native packages all contain focused Tester or unit coverage.

The later window/menu, pointer-drag, scroll-parking, input-target, and sidebar
changes passed the regular full suite and vet. The full race suite was not rerun
after those changes.

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

## Not Proven

- Full Rust/Go behavioral equivalence. No generated immutable oracle corpus or
  randomized differential trace suite is present yet.
- Native IME, accessibility, focus, window lifecycle, and visual acceptance on
  macOS, Windows, and Linux.
- Large-document performance and paragraph-cache invalidation benchmarks.
- Cross-platform packaging or executable release artifacts.

Do not describe the project as release-complete until the unchecked roadmap
items have authoritative evidence.
