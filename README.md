# Neverland

Neverland is a writing workspace built with Go and MyGo.

The port follows the research dossier recorded under the ignored local
`references/` tree:

- the headless semantic and policy engine remains authoritative;
- generated markup remains a projection rather than stored text;
- MyGo supplies native windows, text input, shaping, paint, focus, motion,
  accessibility, and application chrome;
- Neverland supplies the custom editor buffer, selection affinity, projected
  edits, IME policy, history, and text interaction.

## Layout

```text
cmd/neverland        executable composition
engine/model         semantic document, text roots, anchors, marks
engine/kernel        policy catalog, planning, dispatch, projection, publication
engine/editor        editor policies, session, composition, history
engine/surface       UI-neutral frame and style values
app/controller       engine-backed document controller and TextInputClient
app/workspace        pages, tabs, mounted/parked sessions
app/settings         declarative settings snapshots and changes
native/theme         Neverland palette and prose/UI metrics
native/textlayout    MyGo shaping and retained paragraph geometry
native/motion        cursor, selection, viewport, and feedback animation
native/documentview  custom writing surface
native/shell         window composition and chrome
native/overlay       command palette
fixtures             Rust oracle traces and generated compatibility data
```

## Development

The repository uses Go 1.27.3 and MyGo v0.7.1.

```sh
direnv allow
./scripts/fetch-fonts.sh
go test ./...
go tool mygo dev      # live-reloading development app
go tool mygo build    # app bundle and installer under dist/<os>-<arch>
```

The MyGo CLI is pinned as a Go tool in `go.mod`, so `go tool mygo` runs the
same v0.7.1 release as the framework. `mygo.json` holds the app configuration.

The reference prose font is fetched into an ignored local path and verified
against a pinned SHA-256 checksum. `go generate ./native/theme` performs the
same step.

The Nix shell provides Go 1.27.1 and enables the Go toolchain download for the
exact 1.27.3 patch release required by MyGo.

## Text segmentation

Word selection, movement, and statistics use ICU-tailored Unicode word rules.
Chinese and Japanese runs use an embedded ICU dictionary, while Korean and
Latin text use the same UAX #29-compatible platform boundaries.

Thai, Lao, Khmer, and Burmese remain valid editable text and use Unicode
grapheme fallback boundaries. Their language-specific dictionary segmentation
is deferred and is not part of the initial support contract.

## Status

The engine, application boundary, custom writing surface, window shell,
settings window, command palette, pointer selection, and contractual motion
are implemented and covered by headless tests. The complete merged suite passes
with the race detector and `go vet`. The focused Rust oracle corpus is
mandatory and has no skipped parity cases.

Still required before release: broader randomized differential traces, native
IME/accessibility acceptance on every target OS, performance acceptance, and
cross-platform runtime packaging. See
`docs/architecture.md` and `docs/roadmap.md`.

