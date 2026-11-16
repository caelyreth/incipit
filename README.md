# Neverland

Neverland is a writing workspace built with Go and MyGo.

The application shell stands on its own: windows, tabs, the library, the
settings window, the command palette and the notebook on disk. The rebuilt
page editor runs behind it with core text editing, Markdown notation, pairs,
lists, navigation, history, guides, word and sentence boundaries, typography,
CJK refinement, scopes and deletion, tasks, statistics, formatting state, and
persisted settings and selections. Copy and paste carry structure through the
system clipboard. Tables, figures, formula typesetting and accessibility are
still being built.

## Layout

```text
cmd/neverland        executable composition
editor/doc           document tree, operations, transactions and history
editor/vocab         kinds, intents, roles and semantic helpers
editor/policy        policy framework, facts, annotations and dispatch
editor/policies      the product's behavior policies
editor/present       presentation units, stop maps and decorations
editor/engine        per-page intent processing and publication
editor/store         node-row encoding and pointer-diff saves
app/keymap           shell commands, chords and the keymap
app/clipboard        versioned structured slice transfer
app/library          groups, chapters, sheets and the trash
app/settings         settings state and the changes it takes
app/storage          the notebook on disk: pages, library, layout, media
app/workspace        pages and tabs
internal/wordseg     ICU-tailored word boundaries
native/theme         palette and prose/UI metrics
native/components    shared controls
native/motion        animation curves
native/shell         window composition and chrome
native/overlay       command palette
native/settings      settings window
native/notices       toasts
native/formula       TeX formula setting
native/window        the app: windows, menus, routing, notebook
```

## Development

The repository uses Go 1.27.3 and MyGo v0.9.1.

```sh
direnv allow
./scripts/fetch-assets.sh
go test ./...
go tool mygo dev      # live-reloading development app
go tool mygo build    # app bundle and installer under dist/<os>-<arch>
```

The MyGo CLI is pinned as a Go tool in `go.mod`, so `go tool mygo` runs the
same v0.9.1 release as the framework. `mygo.json` holds the app configuration.

Binary assets are not stored in Git. The reference prose font, the math font
and the word segmenter's CJK dictionary are fetched into ignored local paths
and verified against pinned SHA-256 checksums; `go generate ./native/theme`
or `go generate ./internal/wordseg` performs the same step.

The Nix shell provides Go 1.27.1 and enables the Go toolchain download for the
exact 1.27.3 patch release required by MyGo.

## Text segmentation

`internal/wordseg` uses ICU-tailored Unicode word rules. Chinese and Japanese
runs use an embedded ICU dictionary, while Korean and Latin text use the same
UAX #29-compatible platform boundaries. Thai, Lao, Khmer and Burmese fall back
to grapheme boundaries. The editor reaches it through `vocab.WordSegments` for
word-wise motion, selection and deletion.
