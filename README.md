<img width="2400" height="800" alt="header" src="https://github.com/user-attachments/assets/760ac5ff-5f79-48c1-a032-e7105120d40f" />

### Incipit

Quiet, local-first native writing workspace written in Go (`CGO_ENABLED=0` across macOS, Windows, and Linux), storing notebooks in SQLite and rendering directly through [Evil MyGo](https://github.com/caelyreth/evil-mygo) without Electron or webviews.

> [!WARNING]
> **Early Alpha:** Incipit is in early-stage testing. Expect breaking changes; keep backups of important writing.

#### Highlights

- **Editor:** Live Markdown shaping, TeX/Typst formulas, language-highlighted code panels, Typewriter mode, smart typography, and CJK punctuation/spacing backed by UAX #29 + ICU dictionary segmentation.
- **Workspace & Search:** Field and List views, Tray staging, split-column Desk, Bleve full-text search (`$tag`, `$in`, `$updated`, `$words`, `$has`, `$task`, `$key`), Command Palette, and customizable shortcuts.
- **Storage & Delivery:** SQLite notebooks with verified automatic backups, crash recovery, Markdown/PDF export, and signed cross-platform updates with delta patches.

#### Editor Engine

[`internal/editor`](internal/editor) is a headless core isolated from storage and UI:

- **Immutable tree ([`doc`](internal/editor/doc)):** Typed-slot AST addressed by 64-bit `doc.ID` identities, adaptive persistent chunked vectors (`chunkAt`), interned `MarkSet`s, and seven reversible primitives per transaction.
- **Deterministic engine ([`engine`](internal/editor/engine)):** `Engine.Do` and `Engine.Preview` dispatch handlers, rewrite phases, and observers atomically with no clock reads (explicit deadlines via `Timer` intents) for full intent-log replayability.
- **Isolated policies ([`policy`](internal/editor/policy), [`policies`](internal/editor/policies)):** 24 self-contained policies communicate strictly via [`vocab`](internal/editor/vocab) roles with zero cross-imports, ordered only at [`internal/app/manifest`](internal/app/manifest).
- **Memoised presentation & persistence ([`present`](internal/editor/present), [`store`](internal/editor/store)):** `Presenter.Present` caches immutable units by node pointer and view key; `Changes` and `Verify` persist pointer diffs in `O(Δ)` with unknown fields preserved.

#### Development

```sh
go tool fetch && go test ./...
go run ./cmd/incipit
```

Nix environment and Linux packages are defined in [`flake.nix`](flake.nix).

### License

Copyright © 2026 Yu (Caelyreth)

- **Source code:** [AGPL-3.0-only](LICENSE).
- **Brand & artwork:** The Incipit name and assets under [`assets/`](assets) are excluded (all rights reserved); see [`BRAND.md`](BRAND.md).
- **Community & third-party:** [Code of Conduct](CODE_OF_CONDUCT.md) and ICU/Unicode notices in [`internal/text/wordseg/LICENSES`](internal/text/wordseg/LICENSES).
