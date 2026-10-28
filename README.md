# Neverland

Neverland is the Go/MyGo port of Shoreward's writing workspace.

The port follows the research dossier recorded under the ignored local
`references/` tree:

- the headless semantic and policy engine remains authoritative;
- generated markup remains a projection rather than stored text;
- MyGo supplies native windows, text input, shaping, paint, focus, motion,
  accessibility, and application chrome;
- Shoreward supplies the custom editor buffer, selection affinity, projected
  edits, IME policy, history, and text interaction.

## Layout

```text
cmd/shoreward        executable composition
engine/model         semantic document, text roots, anchors, marks
engine/kernel        policy catalog, planning, dispatch, projection, publication
engine/editor        editor policies, session, composition, history
engine/surface       UI-neutral frame and style values
app/controller       engine-backed document controller and TextInputClient
app/workspace        pages, tabs, mounted/parked sessions
app/settings         declarative settings snapshots and changes
native/theme         Shoreward palette and prose/UI metrics
native/textlayout    MyGo shaping and retained paragraph geometry
native/motion        cursor, selection, viewport, and feedback animation
native/documentview  custom writing surface
native/shell         window composition and chrome
native/overlay       command palette, menus, and title editing
fixtures             Rust oracle traces and generated compatibility data
```

## Development

The repository uses Go 1.27.3 and MyGo v0.7.1.

```sh
direnv allow
go test ./...
go run ./cmd/shoreward
```

The Nix shell provides Go 1.27.1 and enables the Go toolchain download for the
exact 1.27.3 patch release required by MyGo.

## Status

The engine, application boundary, custom writing surface, window shell,
settings window, command palette, pointer selection, and contractual motion
are implemented and covered by headless tests. The complete merged suite passes
with the race detector and `go vet`.

Still required before release: generated Rust oracle fixtures, randomized
differential traces, native IME/accessibility acceptance on every target OS,
performance benchmarks, and cross-platform packaging. See
`docs/architecture.md` and `docs/roadmap.md`.

