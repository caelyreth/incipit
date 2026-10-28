# Architecture

## Dependency rule

```text
engine/*         no MyGo imports
app/*            engine imports; no MyGo UI imports
native/*         app + engine + MyGo
cmd/*            composition only
```

This boundary is enforced by review and tests that reject imports in engine
packages.

## Execution path

```text
native input
  -> app/controller
  -> engine/editor session
  -> engine/kernel policy dispatch
  -> semantic document draft
  -> projection + surface preparation
  -> atomic publication
  -> native retained layout and paint
```

Native text-input callbacks query the controller, never the semantic document
directly. The controller returns projected text and maps edits back through the
engine's projected-edit path.

## Ownership

- `engine/model`: semantic truth, immutable text roots, anchors, marks.
- `engine/kernel`: policies, resources, transactions, dispatch, projection.
- `engine/editor`: editor behavior above the kernel.
- `engine/surface`: validated UI-neutral presentation values.
- `app/controller`: active session, preedit facade, metrics, clipboard.
- `app/workspace`: page and tab lifecycle.
- `native`: MyGo windows, layout, drawing, motion, focus, and chrome.

## Non-negotiable invariants

1. Semantic text is authoritative; generated marker text is not stored.
2. Semantic/projected/native ranges use UTF-16; shaping runs use UTF-8 bytes.
3. Draft, validate, prepare, and publish are one transaction boundary.
4. Ordered choice rollback restores the complete journal.
5. Stale projection versions and split-scalar ranges are rejected.
6. Grapheme movement is a policy decision, not a coordinate type.
7. IME preedit is transient and commits through the projected-edit planner.
8. Undo preserves semantic text, marks, selection, projected selection, and
   mark identity.
9. Observable iteration order is explicit.
10. Shaping and layout are retained per paragraph, not rebuilt per frame.

