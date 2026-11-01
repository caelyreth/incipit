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
| A6 | Visible keyboard focus | In progress | `codex/audit-ui-focus` worktree |
| B1 | Layout equality and saver ownership | Settled | Storage compares selections by value and clones queued layouts and library records. Added unchanged-frame, ownership and concurrent-save tests. |
| B2 | Mark canonicalization and identity allocation | In progress | `codex/audit-marks` worktree |
| B3 | Semantic text normalization | Settled | Edits share checked LF normalization and the kernel uses the actual inserted length. A noncanonical legacy record is quarantined as damaged rather than silently shifting offsets; its page rows remain on disk, but automatic migration is not implemented. |
| B4 | After-rule co-schedulability | Settled | One validation checks interfering equal-order wildcard and keyed after-rules without changing handler trigger semantics. |
| B5 | Atomic library operations and navigation reconciliation | In progress | `codex/audit-library` worktree |
| B6 | Durable file publication | In progress | `codex/audit-durability` worktree |
| B7 | Native text hit affinity | In progress | `codex/audit-affinity` worktree |
| B8 | Spring tolerances by value domain | Settled | Dimensionless opacity, zoom and reveal springs have finer stopping thresholds; caption and table 100x workarounds are removed. Tests cover terminal jumps and equivalence with the former scaled geometry. |
| B9 | Layout and shaping invalidation | Open | Profile before changing cache semantics |
| B10 | Scroll input physics | Settled | Shared axis physics exposes explicit ownership, bounds and edge tolerance; editor and MyGo containers retain their distinct notch behavior. Tests cover phases, crossing, tolerance and arbitrary bounds. |
| B11 | Keycaps and inline shortcut chips | Open | Preserve interactive inline behavior |
| B12 | Inline rename lifecycle | Open | |
| B13 | Elevated materials and small controls | Settled | Dialogs, notices and popovers share a material with explicit existing shadow variants. Pixel comparisons cover both themes and UI scales; switch documentation and unused swatch animation input now match behavior. |
| B14 | Selection-track lifetimes | Open | Keep independent selections and clip spaces |
| B15 | Navigation accessibility and focus | Open | Platform acceptance required |
| B16 | Group menu presentation host | In progress | `codex/audit-ui-focus` worktree |
| B17 | Toolbar shortcut metadata | In progress | `codex/audit-routing` worktree |
| C1 | Presentation assembly and publication | Open | Preserve transaction and history semantics |
| C2 | Policy rules, outcomes, spans and composition ownership | Open | |
| C3 | Settings policy host and coherent snapshots | Open | |
| C4 | Media format metadata | Open | Preserve accepted versus stored formats |
| C5 | Library projections, search and counts | Open | Profile large notebooks |
| C6 | Geometry, role mappings and deterministic helpers | In progress | Surface color-role mapping and setting-key sorting are shared; coordinate and notation-run helpers remain open. |
| C7 | Public target rebasing and affinity | Settled | Position affinity is distinct from mark-boundary affinity; collapsed targets remain collapsed, including when given opposing boundary affinities. |
| C8 | Resource cloning and dispatch predicates | Settled | Row/collection reads clone only returned values while preserving authority and isolation. Compiled and linear plans share trigger predicates, with independent expected-plan coverage. |
| C9 | Settings row variants and timing gates | In progress | Shared effective motion policy now covers settings, shell, overlays and table feedback; row/disclosure extraction remains open. |
| C10 | Place-title flight geometry | Settled | Header, band and flight share type, gap and weight descriptors derived from theme metrics; menu anchors remain separate. Scale and interrupted-flight tests pass. |

## Final Verification

- Run the full test suite, race detector, and `go vet` on the integrated branch.
- Exercise changed UI behavior and snapshots at relevant sizes and motion
  settings; distinguish automated MyGo tester coverage from native acceptance.
- Record any outstanding native IME, accessibility, dialog, crash-durability,
  performance, or platform coverage explicitly rather than implying it ran.
