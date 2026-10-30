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

## Pages and blocks

A page is a document whose root holds blocks in order. Each block is a node
with a kind, attributes, its own text rope and inline marks; marks are typed
(`model.MarkKind`) and carry a level, and the model gives kinds no meaning.
Block boundaries are structure, never text: a block's text holds no line
break (code, which keeps its lines, aside), and splitting or joining blocks moves positions with transfer edits
(`model.NewTextTransfer`), so anchors, pulses and selections follow the text
into the block it lands in.

The projection lays the whole page out, block after block, with one line
break of generated text between blocks. Text policies work inside one block.
An edit whose range reaches across a block boundary becomes a span input
(`kernel.SpanInputKey`), which the text policy turns into taking out the
blocks between and joining the ends; a line break typed or pasted splits the
block. An input may change its block, the blocks beside it and their page,
nothing farther. History records whole-page snapshots and restores them with
`DocumentDraft.RestoreDocument`.

## Writing Profile

Every block and mark kind is built in, in `engine/editor/writing.go`: the
engine below it knows only kinds, attributes and marks, and core editing
(`editor.text`) projects their notation, parses edits of it, and types into
each kind as the kind has it. The writing policy (`editor.writing`) adds only
what typing a marker makes, one switch per trigger, and the formatting
commands; turning it off leaves every heading and mark as it is. A page stores what its parts are: a heading and its `level`, a
quote, bullet and ordered items, a code block and its `info`, a rule; marks
of kind emphasis, code and strike. Notation is never stored. The projection
generates it: a block's opener as the attribute segment `block` ("## ",
"> ", "1. ", a fence with its line break), a code block's closing fence as
`fence`, and each mark's markers as `mark/<id>`. Ordered items are numbered
by their place in their run.

With the policy on, typing reads notation as structure: "# " to "###### ", "> ", "- " or "+ "
and "1. " at a paragraph's start; "```" and Return; "---"; and the markers
`*` `_` `` ` `` `~`. Return follows the block's kind, and code takes text
literally: no marks, pairs or refinement read into it. An edit of projected
notation is parsed back with the same grammar (`parseOpener`,
`inlineLevel`); notation that no longer reads as syntax unfolds into the
literal text it shows, so deleting the space of "## " leaves "##" in a
paragraph, and typing it back makes the heading again. The policy's settings
gate each trigger, never a kind.

Select All selects by scope (`engine/editor/scopes.go`): the block's text,
then the whole page, notation included. While a scope is selected, a guide
marks the next one with the keys that select it: an area primitive
(`surface.AreaPrimitive`) names the ranges and keys, and the view shows it
as a bracket in the margin beside those rows, keycaps at its left, grown
out of the selection's rows; it never reads as a selection.

A page is named after its first heading, whatever its level, or "Untitled"
when it has none (`Session.Title`); nothing else names it.

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

