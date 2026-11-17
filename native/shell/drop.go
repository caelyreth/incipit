package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

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
// the target while one is over it, and returns the one dropped on it.
func (s *State) dropOn(c *ui.Context, semantic theme.Theme, target ui.Element, t takes, animate bool) (shellDrag, bool) {
	over, dropped, ok := dragOn(target, t)
	s.dropHighlight(c, semantic, target, over, animate)
	return dropped, ok
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

// dropHighlight tints a target while something it takes is over it.
func (s *State) dropHighlight(c *ui.Context, semantic theme.Theme, target ui.Element, over bool, animate bool) {
	amount := float32(0)
	if over {
		amount = 1
	}
	if animate {
		amount = components.Follow(target, "drop", amount, components.MotionQuick)
	}
	if amount > 0.001 {
		target.Background(components.Blend(ui.Transparent, semantic.Accent.Muted, amount))
		target.Border(1, components.Blend(ui.Transparent, semantic.Accent.Foreground, amount))
	}
}
