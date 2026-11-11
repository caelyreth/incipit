package manifest

import (
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Every object behaves alike, whatever its kind, inline or block: one key
// does one thing to all of them (§12.1). Each case runs against each
// object kind with a slot of its own.

// objectPage is a page holding one object between text, with the caret
// set nowhere in particular.
type objectPage struct {
	*page
	kind doc.Kind
	obj  doc.ID
}

var objectKinds = []struct {
	name  string
	build func(p *page) doc.ID
}{
	{"inline formula", func(p *page) doc.ID {
		p.typeText("ab $x$ cd")
		return p.nodeOf(profile.Formula)
	}},
	{"math block", func(p *page) doc.ID {
		p.typeText("ab")
		p.do(vocab.Break{})
		p.typeText("$$x")
		p.do(vocab.Break{})
		p.do(vocab.Break{})
		p.typeText("cd")
		return p.nodeOf(profile.Math)
	}},
	{"figure", func(p *page) doc.ID {
		p.typeText("ab")
		p.do(vocab.Break{})
		p.placeFigure()
		p.typeText("x")
		p.do(vocab.Break{})
		p.typeText("cd")
		return p.nodeOf(profile.Figure)
	}},
}

// nodeOf is the first node of kind k on the page.
func (p *page) nodeOf(k doc.Kind) doc.ID {
	p.t.Helper()
	var found doc.ID
	var walk func(n *doc.Node)
	walk = func(n *doc.Node) {
		if found != 0 {
			return
		}
		if n.Kind() == k {
			found = n.ID()
			return
		}
		for _, s := range n.Slots() {
			if s.IsInline() {
				s.Content().Each(func(_ int, it doc.Item) bool {
					if it.Node != nil {
						walk(it.Node)
					}
					return found == 0
				})
				continue
			}
			for _, c := range s.Children() {
				walk(c)
			}
		}
	}
	walk(p.e.Snapshot().Root())
	if found == 0 {
		p.t.Fatalf("no %s", profile.Schema().KindName(k))
	}
	return found
}

func (o *objectPage) beside(dir vocab.Direction) doc.Pos {
	o.t.Helper()
	pos, ok := vocab.Beside(o.e.Snapshot(), o.obj, dir)
	if !ok {
		o.t.Fatal("object has no place")
	}
	return pos
}

func (o *objectPage) own(end bool) doc.Pos {
	n, _ := o.e.Snapshot().Node(o.obj)
	pos := doc.Pos{Node: o.obj}
	if end {
		pos.Offset = n.Slot(0).Len()
	}
	return pos
}

func (o *objectPage) at(pos doc.Pos) { o.do(vocab.Place{Stop: doc.CaretAt(pos)}) }

func (o *objectPage) gone() bool {
	_, ok := o.e.Snapshot().Node(o.obj)
	return !ok
}

func (o *objectPage) wantSelected() {
	o.t.Helper()
	if s := o.selection(); s.Kind != doc.SelectNode || s.Node != o.obj {
		o.t.Fatalf("selection %+v, want the object selected", s)
	}
}

func (o *objectPage) wantCaret(pos doc.Pos) {
	o.t.Helper()
	if s := o.selection(); s.Kind != doc.SelectText || !s.IsCollapsed() || s.Head.Pos != pos {
		o.t.Fatalf("selection %+v, want the caret at %v", s, pos)
	}
}

// wantAfter wants the caret past the object, outside it.
func (o *objectPage) wantAfter() {
	o.t.Helper()
	s := o.selection()
	r := o.e.Snapshot()
	if s.Kind != doc.SelectText || s.Head.Pos.Node == o.obj || doc.Compare(r, s.Head, doc.CaretAt(o.beside(vocab.Forward))) < 0 {
		o.t.Fatalf("selection %+v, want the caret after the object", s)
	}
}

func eachObject(t *testing.T, test func(o *objectPage)) {
	for _, k := range objectKinds {
		t.Run(k.name, func(t *testing.T) {
			p := open(t)
			o := &objectPage{page: p, obj: k.build(p)}
			test(o)
		})
	}
}

var (
	back = vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	fwd  = vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Forward}
)

// Motion passes through an object's own slot: in at its start, out after.
func TestMotionPassesThroughEveryObject(t *testing.T) {
	eachObject(t, func(o *objectPage) {
		o.at(o.beside(vocab.Backward))
		var path []doc.Caret
		for range 6 {
			o.do(right)
			path = append(path, o.selection().Head)
		}
		start, end := false, false
		for _, c := range path {
			start = start || c.Pos == o.own(false)
			end = end || c.Pos == o.own(true)
		}
		if !start || !end {
			t.Fatalf("path %v never reaches both ends of the object's own slot", path)
		}
	})
}

// Deleting toward an object, by any unit, either way, selects it; the
// next delete removes it.
func TestDeletingTowardAnObjectSelectsIt(t *testing.T) {
	for _, in := range []vocab.Delete{back, fwd, {Unit: vocab.ByWord, Dir: vocab.Backward}} {
		eachObject(t, func(o *objectPage) {
			if in.Dir < 0 {
				o.at(o.beside(vocab.Forward))
				if o.kindOf() != profile.Formula {
					// A block's place after it is the gap; from the text
					// after it, as a user stands.
					o.do(right)
				}
			} else {
				o.at(o.beside(vocab.Backward))
				if o.kindOf() != profile.Formula {
					o.do(left)
				}
			}
			o.do(in)
			o.wantSelected()
			o.do(in)
			if !o.gone() {
				t.Fatal("the second delete kept the object")
			}
		})
	}
}

func (o *objectPage) kindOf() doc.Kind {
	n, _ := o.e.Snapshot().Node(o.obj)
	return n.Kind()
}

// Backspace at the start of an object's own slot, or in an empty one,
// selects the object.
func TestBackspaceAtAnObjectsStartSelectsIt(t *testing.T) {
	eachObject(t, func(o *objectPage) {
		o.at(o.own(true))
		o.do(back)
		o.wantCaret(o.own(false))
		o.do(back)
		o.wantSelected()
	})
}

// Escape in an object's own slot selects it; on the selected object it
// writes in its own slot again, at its end.
func TestEscapeTogglesAnObjectAndItsSlot(t *testing.T) {
	eachObject(t, func(o *objectPage) {
		o.at(o.own(true))
		o.do(vocab.Dismiss{})
		o.wantSelected()
		o.do(vocab.Dismiss{})
		o.wantCaret(o.own(true))
	})
}

// Select All grows from an object's own slot to the object, then on.
func TestSelectAllGrowsThroughAnObject(t *testing.T) {
	eachObject(t, func(o *objectPage) {
		o.at(o.own(true))
		o.do(vocab.SelectScope{})
		if s := o.selection(); s.Kind != doc.SelectText || s.Head.Pos.Node != o.obj || s.Anchor.Pos.Node != o.obj {
			t.Fatalf("first: %+v, want the own slot", s)
		}
		o.do(vocab.SelectScope{})
		o.wantSelected()
		o.do(vocab.SelectScope{})
		if s := o.selection(); s.Kind != doc.SelectText {
			t.Fatalf("third: %+v, want a range around the object", s)
		}
	})
}

// Leave goes after the object; Return from there comes back to its own
// slot's end.
func TestLeaveAndReturnThroughAnObject(t *testing.T) {
	eachObject(t, func(o *objectPage) {
		o.at(o.own(true))
		o.do(vocab.Leave{})
		o.wantCaret(o.beside(vocab.Forward))
		o.do(vocab.Return{})
		o.wantCaret(o.own(true))
	})
}

// Enter on a selected object, or in its own slot when that keeps no
// lines, breaks right after it.
func TestEnterBreaksBesideAnObject(t *testing.T) {
	eachObject(t, func(o *objectPage) {
		o.do(vocab.Place{Node: o.obj})
		o.do(vocab.Break{})
		if o.gone() {
			t.Fatal("Enter removed the selected object")
		}
		o.wantAfter()
	})
	eachObject(t, func(o *objectPage) {
		if spec, _ := vocab.SlotSpec(o.e.Snapshot(), doc.SlotRef{Node: o.obj}); spec.Lines {
			return
		}
		o.at(o.own(true))
		o.do(vocab.Break{})
		o.wantAfter()
	})
}

// Typing over a selected object replaces it.
func TestTypingReplacesASelectedObject(t *testing.T) {
	eachObject(t, func(o *objectPage) {
		o.do(vocab.Place{Node: o.obj})
		o.typeText("z")
		if !o.gone() {
			t.Fatal("typing kept the object")
		}
	})
}
