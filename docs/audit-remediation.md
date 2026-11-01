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
| A2 | Async image operation identity and targets | In progress | `codex/audit-async` worktree |
| A3 | Command inventory and entry-point routing | In progress | `codex/audit-routing` worktree |
| A4 | IME preview surface metadata | In progress | `codex/audit-ime` worktree |
| A5 | Formatting coverage and mark identity | In progress | `codex/audit-marks` worktree |
| A6 | Visible keyboard focus | In progress | `codex/audit-ui-focus` worktree |
| B1 | Layout equality and saver ownership | In progress | `codex/audit-persistence` worktree |
| B2 | Mark canonicalization and identity allocation | In progress | `codex/audit-marks` worktree |
| B3 | Semantic text normalization | In progress | `codex/audit-invariants` worktree |
| B4 | After-rule co-schedulability | In progress | `codex/audit-invariants` worktree |
| B5 | Atomic library operations and navigation reconciliation | In progress | `codex/audit-library` worktree |
| B6 | Durable file publication | In progress | `codex/audit-durability` worktree |
| B7 | Native text hit affinity | In progress | `codex/audit-affinity` worktree |
| B8 | Spring tolerances by value domain | Open | |
| B9 | Layout and shaping invalidation | Open | Profile before changing cache semantics |
| B10 | Scroll input physics | Open | Preserve editor and scroll-view differences |
| B11 | Keycaps and inline shortcut chips | Open | Preserve interactive inline behavior |
| B12 | Inline rename lifecycle | Open | |
| B13 | Elevated materials and small controls | Open | Preserve intentional visual variants |
| B14 | Selection-track lifetimes | Open | Keep independent selections and clip spaces |
| B15 | Navigation accessibility and focus | Open | Platform acceptance required |
| B16 | Group menu presentation host | In progress | `codex/audit-ui-focus` worktree |
| B17 | Toolbar shortcut metadata | In progress | `codex/audit-routing` worktree |
| C1 | Presentation assembly and publication | Open | Preserve transaction and history semantics |
| C2 | Policy rules, outcomes, spans and composition ownership | Open | |
| C3 | Settings policy host and coherent snapshots | Open | |
| C4 | Media format metadata | Open | Preserve accepted versus stored formats |
| C5 | Library projections, search and counts | Open | Profile large notebooks |
| C6 | Geometry, role mappings and deterministic helpers | Open | |
| C7 | Public target rebasing and affinity | In progress | `codex/audit-invariants` worktree; contract needs care |
| C8 | Resource cloning and dispatch predicates | Open | Preserve authority and oracle paths |
| C9 | Settings row variants and timing gates | Open | Distinguish app and MyGo motion policy |
| C10 | Place-title flight geometry | Open | Preserve endpoint appearance and interaction |

## Final Verification

- Run the full test suite, race detector, and `go vet` on the integrated branch.
- Exercise changed UI behavior and snapshots at relevant sizes and motion
  settings; distinguish automated MyGo tester coverage from native acceptance.
- Record any outstanding native IME, accessibility, dialog, crash-durability,
  performance, or platform coverage explicitly rather than implying it ran.
