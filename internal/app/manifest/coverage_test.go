package manifest

import (
	"fmt"
	"testing"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// The basic intents have an outcome in every kind of block and at every
// place the caret stands in it: typing, deleting either way and Enter
// change the document or the selection. The only place one may do
// nothing is an edge of the page with nothing beyond it to delete.
// A swallowed intent counts as handled, so this is what keeps a kind
// from silently ignoring a key.

// blockPages build pages whose last block is of one kind, holding "abc"
// with the caret at its end.
var blockPages = map[string]func(p *page){
	"paragraph": func(p *page) { p.typeText("abc") },
	"second paragraph": func(p *page) {
		p.typeText("x")
		p.do(vocab.Break{})
		p.typeText("abc")
	},
	"heading": func(p *page) { p.typeText("## abc") },
	"item":    func(p *page) { p.typeText("- abc") },
	"ordered": func(p *page) { p.typeText("1. abc") },
	"quote":   func(p *page) { p.typeText("> abc") },
	"code":    func(p *page) { p.typeText("```"); p.do(vocab.Break{}); p.typeText("abc") },
	"nested item": func(p *page) {
		p.typeText("- x")
		p.do(vocab.Break{})
		p.typeText("abc")
		p.do(profile.Nest{Dir: vocab.Forward})
	},
	"heading after a list": func(p *page) {
		p.typeText("- x")
		p.do(vocab.Break{})
		p.do(vocab.Break{})
		p.typeText("# abc")
	},
}

var basicIntents = map[string]vocab.Intent{
	"type":      vocab.Type{Text: "z"},
	"backspace": backspace,
	"delete":    forward,
	"enter":     vocab.Break{},
}

func TestBasicIntentsHaveAnOutcomeInEveryBlock(t *testing.T) {
	for _, reveal := range reveals {
		for name, build := range blockPages {
			// Each place the caret stands in the block: the stops from its
			// end back to its first, before any marker.
			places := 0
			var prev doc.Caret
			for back := 0; ; back++ {
				p := revealed(t, reveal)
				build(p)
				block := p.head().Pos.Node
				for range back {
					p.do(left)
				}
				if p.head().Pos.Node != block || (back > 0 && p.head() == prev) {
					break
				}
				prev = p.head()
				places++
				at := p.shown()
				for iname, in := range basicIntents {
					q := revealed(t, reveal)
					build(q)
					for range back {
						q.do(left)
					}
					before := q.e.Snapshot()
					sel := q.head()
					stops, _ := q.e.Stops(1)
					first, _ := stops.Edge(vocab.Backward)
					last, _ := stops.Edge(vocab.Forward)
					q.do(in)
					changed := q.e.Snapshot() != before || q.head() != sel
					edge := (iname == "backspace" && sel == first) || (iname == "delete" && sel == last)
					if !changed && !edge {
						t.Errorf("%v %s at %q: %s does nothing\n%s", reveal, name, at, iname, outline(before.Root()))
					}
				}
			}
			if places < 4 {
				t.Errorf("%v %s: only %d places in the block", reveal, name, places)
			}
		}
	}
}

// Every published state after a basic intent keeps the caret on a stop
// whose bound is a position in the document.
func TestBasicIntentsLeaveTheCaretOnABound(t *testing.T) {
	for name, build := range blockPages {
		for iname, in := range basicIntents {
			t.Run(fmt.Sprint(name, "/", iname), func(t *testing.T) {
				p := open(t)
				build(p)
				p.do(in)
				stops, _ := p.e.Stops(1)
				b := stops.Bound(p.head())
				if n, ok := p.e.Snapshot().Node(b.Node); !ok || b.Slot >= n.NumSlots() {
					t.Fatalf("caret %v bound to %v", p.head(), b)
				}
			})
		}
	}
}
