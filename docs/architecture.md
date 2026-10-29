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
- `engine/editor/internal/wordsegmenter`: vendored ICU-tailored Latin/CJK word
  boundaries; not a public API.
- `engine/surface`: validated UI-neutral presentation values.
- `app/controller`: active session, preedit facade, metrics, clipboard.
- `app/workspace`: page and tab lifecycle.
- `native`: MyGo windows, layout, drawing, motion, focus, and chrome.

## Actions and keys

Policies, including the host's surface policy, declare actions by identity,
name and scope only (`editor.Command`, read through `editor.Commands` and
`editor.CommandFor`). Default keys live in one layer, `engine/editor/keymap.go`,
linked with the catalog by `kernel.LinkPolicyCatalog(descriptors, defaults...)`.
A configuration's remaps sit above the defaults, and preparation checks the
combined keymap for conflicts.

Settings never spell a key in prose. Text names an action as `{command.id}`
(`appsettings.Phrase`), and the view draws the key it has now, which records
a new one in place. A policy card edits the actions its text names right
there in the sentence, and lists its other actions as rows.

The Keymap groups actions by `Scope`: writing, pages, window, app. Built-in
actions come first, and a key a switchable feature adds is marked with that
feature, which leads to its card. Every key on either page records through
the same recorder, which opens under the row or sentence holding the key,
and goes through the same conflict check.

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

