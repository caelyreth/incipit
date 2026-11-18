package shell

import (
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A drag lights only what it can be dropped on (field-and-desk.md
// §4.7.3): a group's head or row, the Group ribbon of the group the field
// is in, the trash in the lane, the desk, the tray and the kept box. While
// one goes on, each target that takes it shows the accent's hint, and the
// one under the pointer its tint; what is dragged dims where it was, its
// title alone, and a group held over long enough is gone into, the drag
// going on there.

// springFor is how long something is held over a group before the field
// goes into it.
const springFor = 700 * time.Millisecond

// dragFrom makes el the source of v, dimmed in place while it is dragged
// (MyGo's copy under the pointer keeps its own opacity), and reports
// whether it is.
func (s *State) dragFrom(el ui.Element, v shellDrag) bool {
	el.Drag(v)
	if !el.Dragging() {
		return false
	}
	s.field.dragNext = v
	el.Opacity(0.4)
	return true
}

// springLoad goes into g, the row id at from, once what it takes has been
// held over it springFor: so filing deep needs no letting go.
func (s *State) springLoad(c *ui.Context, id string, g GroupItem, over bool, from stage.Rect) {
	sp := &s.field.spring
	switch {
	case !over:
		if sp.id == id {
			*sp = springing{}
		}
		return
	case sp.id != id:
		*sp = springing{id: id, at: c.Now()}
	}
	if left := springFor - c.Now().Sub(sp.at); left > 0 {
		c.After(left)
		return
	}
	*sp = springing{}
	s.into(g, from)
}

// springing is the group something has been held over, since when.
type springing struct {
	id string
	at time.Time
}

// shellDrag is anything the shell drags: a sheet, the tray whole, a group,
// or a condition. A
// MyGo element takes the values of one type alone, the type of the last
// ui.Drop or ui.DragOver it was asked, so a target that takes several
// kinds asks for this one type, and sorts out what it takes (takes).
type shellDrag interface{ shellDrag() }

func (sheetDrag) shellDrag() {}
func (groupDrag) shellDrag() {}

// takes is what a drop target takes of the shell's drags.
type takes struct {
	sheets, groups, queries bool
}

// of reports whether t takes v.
func (t takes) of(v shellDrag) bool {
	switch v.(type) {
	case sheetDrag, trayDrag:
		return t.sheets
	case groupDrag:
		return t.groups
	case queryDrag:
		return t.queries
	}
	return false
}

// dropOn makes target take what t allows of the shell's drags: it lights
// the target while one it takes goes on, more while it is over it, and
// reports whether one is, and the one dropped on it.
func (s *State) dropOn(c *ui.Context, semantic theme.Theme, target ui.Element, t takes, animate bool) (over bool, dropped shellDrag, ok bool) {
	over, dropped, ok = dragOn(target, t)
	s.dropHighlight(c, semantic, target, s.field.drag != nil && t.of(s.field.drag), over, animate)
	return over, dropped, ok
}

// dragOn makes target take what t allows of the shell's drags, for a
// target that shows it takes them its own way: it reports whether one is
// over it, and returns the one dropped on it.
func dragOn(target ui.Element, t takes) (over bool, dropped shellDrag, ok bool) {
	held, holding := ui.DragOver[shellDrag](target)
	if dropped, ok := ui.Drop[shellDrag](target); ok && t.of(dropped) {
		return false, dropped, true
	}
	return holding && t.of(held), nil, false
}

// dropHighlight lights a target: the accent's hint while something it
// takes is dragged, its tint and rim while it is over it.
func (s *State) dropHighlight(c *ui.Context, semantic theme.Theme, target ui.Element, takes, over bool, animate bool) {
	can := map[bool]float32{true: 1}[takes || over]
	amount := map[bool]float32{true: 1}[over]
	if animate {
		can = components.Follow(target, "drop-can", can, components.MotionQuick)
		amount = components.Follow(target, "drop", amount, components.MotionQuick)
	}
	if can > 0.001 {
		hint := components.Blend(ui.Transparent, semantic.Accent.Hint, can)
		target.Background(components.Blend(hint, semantic.Accent.Tint, amount))
		target.Border(1, components.Blend(ui.Transparent, semantic.Accent.Foreground, amount))
	}
}
