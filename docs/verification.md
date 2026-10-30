# Verification

Updated on 2026-10-28 from the completion pass on `codex/native-contracts`.

## Required Checks

```text
go test -count=1 ./...                               PASS
go test -race -count=1 ./...                         PASS
go vet ./...                                         PASS
go build ./cmd/neverland                             PASS
go tool mygo vet .                                   PASS
CGO_ENABLED=0 GOOS=windows GOARCH=amd64
  go test -c -o /tmp/editor-windows.test.exe ./engine/editor
                                                       PASS
CGO_ENABLED=0 GOOS=windows GOARCH=amd64
  go test -c -o /tmp/shell-windows.test.exe ./native/shell
                                                       PASS
CGO_ENABLED=0 GOOS=linux GOARCH=arm64
  go test -c -o /tmp/editor-linux.test ./engine/editor
                                                       PASS
CGO_ENABLED=0 GOOS=linux GOARCH=amd64
  go build -o /tmp/neverland-linux ./cmd/neverland   PASS
gofmt -l engine app cmd native                       clean
git diff --check                                     PASS
```

The suite contains 522 Go test functions. All linked development worktrees
created for this pass were removed after integration.

## Rust Oracle

The repository retains two deterministic Rust-generated corpora:

```text
fixtures/rust-oracle/v1.json
fixtures/rust-oracle/v2.json
```

Version 1 covers:

- UTF-8, UTF-16, scalar, and native-range conversion;
- anchor and mark rebasing;
- semantic metrics;
- generated markup projection;
- seeded document traces.

Version 2 covers:

- platform word-like behavior for grouped numbers, middle dots, CJK
  dictionary boundaries, and standalone combining marks;
- explicit platform overrides for a bare underscore and deferred Thai lexical
  segmentation;
- compound IME final replacement inside nonempty generated pairs;
- undo restoration of marker-only projected selections;
- `allow-strong = false` state and exact rejection;
- pair and typography decisions for Unicode `Other_Alphabetic`;
- kernel undeclared document read/write errors.

All v2 cases are mandatory assertions. The two intentional word-boundary
overrides execute against the current platform policy rather than being
skipped.

## Proven

- Semantic document transactions, anchors, marks, projection, policy runtime,
  deterministic dispatch, observer validation, rollback, and atomic
  publication.
- Grapheme, ICU-tailored Latin/CJK word, sentence, typography, pair, markup,
  projected-edit, composition, history, feedback, and statistics behavior.
- UTF-16 native boundaries, compound projected replacement, IME preview
  retention, final compound commit, cancellation, and policy-decline handling.
- Rust-compatible `editor.surface` bindings for visual movement, clipboard,
  pages, presentation, CJK, and application commands.
- Remapped shortcuts resolve through the same prepared policy configuration
  used by the editor, settings UI, and command palette.
- Vertical visual-row movement and Shift extension preserve a preferred
  column across wrapped rows.
- Page lifecycle, parked sessions, focus restoration, configuration
  publication, settings reducers, and per-page scroll restoration.
- Custom-document caret, click, drag, double-click, triple-click, Shift-click,
  accessibility selection, and offscreen caret scheduling behavior.
- Modal mounting and text-input focus for command search.
- Portable MyGo test locking and Windows test compilation.

## Segmentation Benchmarks

Local reference on Apple M5 Pro with Go 1.27.3:

```text
BenchmarkWordLikeRanges/Latin          ~1.46 us/op   2864 B/op  8 allocs/op
BenchmarkWordLikeRanges/CJK            ~6.90 us/op   4160 B/op 37 allocs/op
BenchmarkWordLikeRanges/ThaiFallback   ~1.06 us/op   1504 B/op  7 allocs/op
```

These are comparative development measurements, not release latency budgets.

## Performance Evidence

Long-prose, heavy-policy, retained-layout, and end-to-end typing benchmarks are
reproducible with:

```sh
./scripts/bench-performance.sh
```

The reference result reduces 256 KiB end-to-end keystroke-to-paint from
about `87.8 ms / 21.8 MB` to `3.77 ms / 1.03 MB`, and retained one-paragraph
typing from `44-47 ms` to `0.58 ms`. The benchmark methodology, complete
before/after matrix, profiles, and remaining bottlenecks are recorded in
`docs/performance.md` and the Git-ignored
`references/performance-2026-10-29/` archive.

## MyGo v0.7.1 Accessibility Boundary

MyGo supports custom-document focus, role, label, value, and accessibility
selection input. Its richer `Element.Access` callback exposes an internal-only
platform node type, and its generic path does not synthesize announced
selection ranges or edit actions for external `TextInputClient` values.

This exact framework limitation is documented and pinned by a capability test
in `native/documentview/accessibility_test.go`.

## Not Yet Proven

- Full randomized Rust/Go equivalence beyond the focused v1/v2 oracle corpus.
- Real macOS, Windows, and Linux IME candidate, commit, cancellation, and
  focus-loss matrices.
- Platform accessibility audits on every target OS.
- Windows and Linux native launch, paint, input, and window-restore acceptance.
- Large-document interaction budgets and paragraph-cache memory limits.
- Production installer/update distribution flows.

Do not describe the project as release-complete until those acceptance gates
have authoritative evidence.

