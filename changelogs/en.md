### 1.0.0-alpha.2

A polish release: smoother theme changes, a tidier Desk, and proper window controls on Windows and Linux.

- **Seamless light and dark switching:** Switching between Starfield and Daybreak now blends the whole window as one. The Desk's toolbar, sheet headers and the Desk lane in the Field no longer lag behind or catch up afterwards, and controls change from dark to light halfway through the blend instead of the moment it starts.
- **A more consistent Desk:** The Desk now has the same rounded corners as the Tray and Kept, plus a fine border. As it flies between the Field and full screen, its border fades out early and its corners square off gradually, so no seam shows against the window edge. The Tray's empty hint is now centered.
- **Search suggestions stay in view:** When the search field sits at the bottom of the window, suggestions such as Key move to the side that has room instead of disappearing.
- **Native window controls on Windows and Linux:** The Close button no longer overlaps the system's window controls, and the gap reserved for the macOS traffic lights now appears only on macOS.
- **More reliable updates:** If the main update server can't be reached, updates are downloaded from GitHub Releases instead.

### 1.0.0-alpha.1

Welcome to the first public alpha of Incipit — a quiet, local-first writing workspace built with Go.

#### Writing & Editor

- **Live Markdown shaping:** Format headings, lists, task items, block quotes, tables, and captioned figures cleanly as you type, choosing whether syntax marks stay visible or tuck away.
- **Formulas:** Write inline math and display blocks using either TeX or Typst syntax.
- **Code blocks:** Fenced blocks sit on their own panel with language-aware syntax highlighting from the info string, smart indentation on Enter, and clean fence navigation.
- **Typography & CJK typesetting:** Convert `--` and `...` on the fly, curl quotes, auto-pair brackets and quotes, tune CJK punctuation width and CJK–Latin spacing, see subtle correction pulses, and keep your current line centered in Typewriter mode.
- **Precision editing & navigation:** Act quickly with the floating selection toolbar, keyboard block moves (`↑`/`↓`) and block/list shortcuts, indentation and table-row guides, structural scope expansion, triple-click sentence selection, two-step deletion protection, live in-sheet find, and real-time word and CJK character counts.
- **Smart clipboard:** Paste cleanly from the web with automatic HTML-to-Markdown translation or Paste and Match Style, and copy plain text out as clean Markdown.
- **macOS writing aids:** Check spelling and grammar as you write, auto-correct with click-to-restore, accept inline ghost predictions with Tab, and look up words via Force Click or the context menu.

#### Library, Search & Workspace

- **Field, List, Tray & Desk:** Browse your notebook spatially in Field or linearly in List, organize with nested groups, color swatches, tags, pins, and sheet image previews, stage notes in the Tray, and write across split columns on the Desk.
- **Search, filters & commands:** Search full text and filter with `$tag`, `$in`, `$updated`, `$words`, `$has`, and `$task`, bookmark frequent queries, jump via Go to Sheet, or trigger actions from the Command Palette.
- **Local-first safety & shortcuts:** Keep everything in a local SQLite notebook folder with Trash recovery, automatic verified backups, crash-safe restore, orphaned image cleanup on launch, single-instance window handoff, and customizable shortcuts with live `$key` chord capture.

#### Onboarding, Appearance & Delivery

- **Guided first run:** Choose your notebook folder (with XDG support on Linux) and preview your writing feel and look before starting.
- **Themes, typography & motion:** Switch between Starfield (dark), Daybreak (light), and System with group-tinted accents and reduced-motion-friendly transitions; pick Serif or Sans prose, browse a searchable font picker with live specimens, select inline CJK glyph forms, and download Noto typefaces on demand.
- **Languages & export:** Use Incipit natively in English, Japanese, Simplified Chinese, or Traditional Chinese, and export sheets or groups to Markdown (with optional YAML front matter) and paginated PDF.
- **Cross-platform signed updates:** Install and update cleanly via signed manifests and binary delta patches across macOS (Apple Silicon & Intel), Windows (`amd64` & `arm64`), and Linux (`tar.gz`, `.deb`, `.rpm`, and `install.sh`).
