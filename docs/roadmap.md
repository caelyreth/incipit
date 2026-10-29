# Migration Roadmap

## Completed foundation

- [x] Repository and package boundaries
- [x] Pinned Go/MyGo toolchain
- [x] Architecture ownership record

## Engine

- [x] Coordinate and text-root primitives
- [x] Semantic document, anchors, and marks
- [x] Control algebra and policy kernel
- [x] Resources and atomic publication
- [x] Projection and source correspondence
- [x] Editor commands, words, and statistics
- [x] Latin, Chinese, Japanese, and Korean word boundaries
- [ ] Thai, Lao, Khmer, and Burmese dictionary segmentation
- [x] Markup, pairs, CJK, and typography
- [x] Projected edits and IME composition
- [x] History and correction feedback
- [x] Editor session orchestration

## Application

- [x] Document controller and custom buffer facade
- [x] `ui.TextInputClient` adapter
- [x] Clipboard, metrics, and activity
- [x] Keymap and command routing
- [x] Workspace and parked pages
- [x] Settings snapshots and publication

## Native UI

- [x] Theme and prose metrics
- [x] Shaped paragraph cache
- [x] Selection and range geometry
- [x] Custom cursor and cursor breathing
- [x] Selection/viewport motion
- [x] Writing surface
- [x] Window shell and title bar
- [x] Sidebar, tabs, status, and toolbar
- [x] Command palette and page menu
- [x] Settings window
- [ ] Accessibility acceptance (roles and focus exist; native platform audit remains)

## Verification

- [x] Focused Rust oracle fixture generation (`v1` and regression `v2`)
- [ ] Go golden and randomized parity tests
- [x] MyGo Tester interaction tests for the implemented shell, surface, settings, overlays, focus, text input, and pointer selection
- [ ] Native IME acceptance
- [x] Performance benchmarks
- [ ] Absolute release latency and memory budgets on every target platform
- [ ] Cross-platform packaging

The checked items record implemented, tested behavior, not full Rust/native
parity acceptance. The remaining unchecked items are release gates.

