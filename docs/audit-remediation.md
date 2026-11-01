# Architecture and UI Audit Remediation

This is the tracked status of the 2026-11-01 architecture, reuse, and native UI
audit at revision `531f68a25e4d76f2e61af1f17ea2bad14454c53b`. The detailed
audit, including evidence and proposed tests, is in the local `references/`
archive. This ledger remains in Git so that unfinished decisions and platform
validation do not disappear when a worktree or local archive is removed.

Status is **open**, **in progress**, **settled**, or **deferred**. A settled
entry must name its implementation and verification; a deferred entry must
explain the reason and what evidence or decision would allow it to proceed.
Candidate refactors should preserve intentionally different behavior rather
than force unrelated operations through a generic abstraction.

## Findings

| ID | Concern | Status | Notes |
| --- | --- | --- | --- |
| A1 | Page-scoped interactions and semantic caches | Settled | One lifecycle boundary resets page-owned state and detaches native input; control identity and block units follow the page. Regression tests cover resize, stale units and late callbacks. Native IME page-switch acceptance remains outstanding. |
| A2 | Async image operation identity and targets | Settled | Image operations capture page, session, revision, caret and optional node; detached/stale operations report failure. Delayed-intake, remount, colliding-ID, history and autosave regressions are covered. Native file-dialog acceptance remains outstanding; unused stored media can remain until the existing sweep. |
| A3 | Command inventory and entry-point routing | In progress | `codex/audit-routing` worktree |
| A4 | IME preview surface metadata | Settled | A shared range transform preserves table, figure and room metadata; tests include empty/cell-end composition and candidate updates. Cross-cell composition remains unsupported by the editor; native IME acceptance remains outstanding. |
| A5 | Formatting coverage and mark identity | In progress | `codex/audit-marks` worktree |
| A6 | Visible keyboard focus | Settled | Independently focusable tabs and group disclosures paint a focused border at the actual keyboard target. MyGo tester pixel checks distinguish disclosure focus from its focusable ancestor; native high-contrast and assistive-technology acceptance remain outstanding. |
| B1 | Layout equality and saver ownership | Settled | Storage compares selections by value and clones queued layouts and library records. Added unchanged-frame, ownership and concurrent-save tests. |
| B2 | Mark canonicalization and identity allocation | In progress | `codex/audit-marks` worktree |
| B3 | Semantic text normalization | Deferred | Edits now share checked LF normalization and the kernel uses the actual inserted length. Noncanonical legacy records are quarantined rather than silently shifting offsets; page rows remain on disk, but those pages are unavailable until recovered. A versioned migration must map marks and any persisted selection/anchor offsets, be tested against real old notebooks and backups, and publish atomically before this compatibility gap can close. |
| B4 | After-rule co-schedulability | Settled | One validation checks interfering equal-order wildcard and keyed after-rules without changing handler trigger semantics. |
| B5 | Atomic library operations and navigation reconciliation | In progress | `codex/audit-library` worktree |
| B6 | Durable file publication | Settled | Media, verified SQLite backups, recovery copies and rescue files share staged sync/replace publication; failed rescue writes are reported alongside the save error. Existing destination modes are retained and newly staged files stay private (including new rescue files). Failure-injection tests cover write/sync/close/verify/rename/cleanup and post-rename directory-sync failure; Windows cross-compiles with write-through replacement. Native Windows, crash/power-loss and newly created parent-directory durability still need platform acceptance. |
| B7 | Native text hit affinity | In progress | `codex/audit-affinity` worktree |
| B8 | Spring tolerances by value domain | Settled | Dimensionless opacity, zoom and reveal springs have finer stopping thresholds; caption and table 100x workarounds are removed. Tests cover terminal jumps and equivalence with the former scaled geometry. |
| B9 | Layout and shaping invalidation | Deferred | Equal, immutable visual flows retain their layout; nested table data is detached from callers. A 100-row MyGo tester benchmark improved from about 560 us/424 KB to 59 us/126 KB per stable frame on Apple M5 Pro. Theme-only changes still reshape: MyGo v0.7.3 retains paint spans in text layouts, so omitting colors from the cache key would show stale colors. Revisit with a framework paint-only span API and theme-transition benchmarks. |
| B10 | Scroll input physics | Settled | Shared axis physics exposes explicit ownership, bounds and edge tolerance; editor and MyGo containers retain their distinct notch behavior. Tests cover phases, crossing, tolerance and arbitrary bounds. |
| B11 | Keycaps and inline shortcut chips | Deferred | Both render caps, but the static run uses text-layout line boxes while sentence chips use glyph baselines, different width rules, clipping, and animated replacement. Unifying geometry without approving those differences would change wrapping and appearance; first capture baselines/widths at multiple scales, for symbols and words during recording, then extract only a parameterized primitive that preserves them. |
| B12 | Inline rename lifecycle | Settled | Group and chapter fields share focus-arrival selection, Escape, submit and blur handling while retaining separate row visuals and model actions. MyGo tester covers Unicode replacement, empty drafts, submit, cancellation and click-away. |
| B13 | Elevated materials and small controls | Settled | Dialogs, notices and popovers share a material with explicit existing shadow variants. Pixel comparisons cover both themes and UI scales; switch documentation and unused swatch animation input now match behavior. |
| B14 | Selection-track lifetimes | Deferred | The group-place and active-sheet highlights are independently visible, and groups, Trash and chips occupy separate clipping/scroll spaces. A single shared track would conflate selections or escape a clip. Decide which cross-container transitions should travel, then implement an ancestor overlay with explicit coordinate/clip mapping and tests for search, drag, multiselect and reduced motion. Existing list-local glides remain. |
| B15 | Navigation accessibility and focus | Deferred | Visible focus and tree/list semantics are separate contracts: MyGo focus groups include nested disclosure controls, and a role alone does not implement tree keys or multiselect. Define row-only roving, expand/collapse and keyboard-accessible context actions; validate the actual accessibility tree and VoiceOver/Narrator/Orca before changing navigation semantics. |
| B16 | Group menu presentation host | Settled | Rename, style and child creation reveal the library, expand all ancestors and focus/scroll the target row or field. Tester coverage exercises row and sheet-title menus with the sidebar visible/hidden and deeply nested groups. |
| B17 | Toolbar shortcut metadata | In progress | `codex/audit-routing` worktree |
| C1 | Presentation assembly and publication | Deferred | Presentation assembly differs across formatting, edits, reconfiguration and history in caret/visual-anchor restoration and transaction ordering. No single replacement is yet proven equivalent; first tabulate those contracts and add marker-only, rollback, per-event handled, active-mark and undo/reconfigure tests before extracting a common builder. |
| C2 | Policy rules, outcomes, spans and composition ownership | Deferred | The editable-span query is shared. Rule descriptors, handled-versus-changed outcomes and composition's semantic/projected targets still have different owners and no reproduced target mismatch. Map stale checking, handler gates and no-op/native-error outcomes with regression tests before consolidating; a generic wrapper would conceal rather than solve these contracts. |
| C3 | Settings policy host and coherent snapshots | Settled | One synchronized app-level host validates candidates, publishes through the workspace, and exposes owned coherent snapshots. Failed validation/publication leaves the host unchanged. Tests cover publication failures, ownership and concurrent snapshot reads; settings edits themselves remain UI-thread operations. |
| C4 | Media format metadata | Settled | One app-level registry describes input aliases, dialog extensions, decoder names, canonical stored MIME/extensions and PNG normalization; TIFF remains accepted and stored as PNG. Tests cover aliases, isolation, sorted filters, unknown types and actual converted bytes. Native dialog acceptance remains outstanding. |
| C5 | Library projections, search and counts | Deferred | Sheet rows, entries and search use a fresh operation-local session index instead of repeated linear scans; badge counts avoid preparing titles, excerpts and word counts. A synthetic 1,000-session lookup benchmark improved from about 2.31 ms to 18 us on Apple M5 Pro, not an end-to-end search measurement. Search still scans text and place details are assembled repeatedly; benchmark realistic large notebooks before adding a cache, and test invalidation for edits, filing, query, settings and the Recent time boundary. |
| C6 | Geometry, role mappings and deterministic helpers | Settled | Surface color-role mapping, setting-key sorting and notation lookup are shared. Viewport/content conversion now has one explicit inset/visible-scroll transform while retaining separate controller and painting adapters; tests cover negative overscroll, fractional offsets and existing geometry. |
| C7 | Public target rebasing and affinity | Settled | Position affinity is distinct from mark-boundary affinity; collapsed targets remain collapsed, including when given opposing boundary affinities. |
| C8 | Resource cloning and dispatch predicates | Settled | Row/collection reads clone only returned values while preserving authority and isolation. Compiled and linear plans share trigger predicates, with independent expected-plan coverage. |
| C9 | Settings row variants and timing gates | Deferred | Shared effective motion policy covers settings, shell, overlays and table feedback; state and origin tags share geometry without merging interaction or paint. Settings rows have different wrapping/density and the settings tree's fixed-height fold/disclosure differs from the measured fold and independently focusable group disclosure. Preserve these behaviors until frame-by-frame, focus and narrow-width comparisons establish a safe shared variant. |
| C10 | Place-title flight geometry | Settled | Header, band and flight share type, gap and weight descriptors derived from theme metrics; menu anchors remain separate. Scale and interrupted-flight tests pass. |

## Final Verification

- Before the remaining worktree integrations, the full uncached suite and
  `go vet ./...` passed; the full `go test -race ./...` also passed on the
  integration branch through the library projection changes.
- Run the full test suite, race detector, and `go vet` on the integrated branch.
- Exercise changed UI behavior and snapshots at relevant sizes and motion
  settings; distinguish automated MyGo tester coverage from native acceptance.
- Record any outstanding native IME, accessibility, dialog, crash-durability,
  performance, or platform coverage explicitly rather than implying it ran.
