package shell

import (
	"fmt"
	"slices"

	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// The tray is a box at the desk lane's foot, under the desk and as wide
// (field-and-desk.md §4.6): sheets are set aside in it for a while. A
// sheet dragged to it from the field leaves the field for the tray, though
// it stays filed where it is. It goes out only where it is taken: dragged
// onto a group, a group's row, or the field in a group, it is filed there;
// onto the desk it opens; let go anywhere else, it stays in the tray. The tray dragged out
// whole does so with them all. Its menu is the menu for picked sheets, and
// a sheet's menu puts it back. It holds them only for now: emptied, or
// with the app closed, each is back where it was. It is a shelf
// (shelf.go), sunk and of a fixed size, filling the lane under the desk,
// its sheets drawn as the field draws them, smaller.

// trayDrag is the tray dragged out whole.
type trayDrag struct{}

func (trayDrag) shellDrag() {}

// inTray reports whether the tray holds the sheet id.
func (s *State) inTray(id string) bool {
	return slices.ContainsFunc(s.lane.held, func(sh SheetItem) bool { return sh.ID == id })
}

// trayIDs are the sheets the tray holds.
func (s *State) trayIDs() []string {
	ids := make([]string, len(s.lane.held))
	for i, sh := range s.lane.held {
		ids[i] = sh.ID
	}
	return ids
}

// toTray sets the sheets ids aside in the tray, after those it holds, as
// the field shows them.
func (s *State) toTray(ids []string) {
	for _, id := range ids {
		if sh, ok := s.shownSheet(id); ok && !s.inTray(id) {
			s.lane.held = append(slices.Clip(s.lane.held), sh)
		}
	}
	s.lane.heldRev++
}

// outOfTray puts the sheets ids back where they were.
func (s *State) outOfTray(ids []string) {
	// A new list, as a frame may still be drawing the old.
	var kept []SheetItem
	for _, sh := range s.lane.held {
		if !slices.Contains(ids, sh.ID) {
			kept = append(kept, sh)
		}
	}
	s.lane.held = kept
	s.lane.heldRev++
}

// shownSheet is the sheet id as the field shows it.
func (s *State) shownSheet(id string) (SheetItem, bool) {
	for _, cd := range s.field.found.cards {
		if b, ok := cd.kind.(block); ok {
			for _, e := range b.entries {
				if e.sheet.ID == id {
					return e.sheet, true
				}
			}
		}
	}
	return SheetItem{}, false
}

// carried is the sheets d carries: the sheet dragged, with the others
// selected with it, or all the tray holds.
func (s *State) carried(d shellDrag) []string {
	switch d := d.(type) {
	case sheetDrag:
		// A sheet selected with others goes with them, as a Mac's files
		// do; out of the tray, it goes alone.
		if !d.tray && s.isPicked("sheet:"+d.id) {
			if picked := s.pickedOf("sheet:"); len(picked) > 1 {
				return picked
			}
		}
		return []string{d.id}
	case trayDrag:
		return s.trayIDs()
	}
	return nil
}

// trashDropped puts what was dropped on the trash in it: the sheets d
// carries, out of the tray if they came from it, or a group.
func (s *State) trashDropped(d shellDrag) {
	switch d := d.(type) {
	case sheetDrag, trayDrag:
		s.trashSheets(s.carried(d))
		s.delivered(d)
	case groupDrag:
		if g, ok := s.Library.Group(d.id); ok && !g.Inbox {
			s.trashGroup(g)
		}
	}
}

// delivered lets the tray go of what d took out of it, once a drop has
// taken it.
func (s *State) delivered(d shellDrag) {
	switch d := d.(type) {
	case sheetDrag:
		if d.tray {
			s.outOfTray([]string{d.id})
		}
	case trayDrag:
		s.outOfTray(s.trayIDs())
	}
}

// trayView draws the tray in the desk lane, under where the desk rests and
// as wide, in the window h tall.
func (s *State) trayView(c *ui.Context, semantic theme.Theme, h, k float32, hidden, animate bool) {
	if s.Library == nil {
		return
	}
	held, ids := s.lane.held, s.trayIDs()
	desk := s.lane.preview
	top := desk.Y + desk.H + 48*k
	sh := shelf{
		key: "tray", name: locales.Tray(), icon: components.IconTray, label: locales.SheetsInTray(), count: len(held), right: true,
		width: desk.W, top: top, most: h - top - 24*k, fixed: true, inset: true, takes: takes{sheets: true},
	}
	if len(held) > 0 {
		sh.drag = trayDrag{}
		sh.menu = func(m *ui.Menu) {
			s.sheetsMenu(ids, func() { s.outOfTray(ids) })(m)
			m.Separator()
			if m.Item(locales.EmptyTray()).Chosen() {
				s.outOfTray(ids)
			}
		}
	}
	// Its sheets stand two abreast where it is wide enough for them.
	inner := desk.W - 2*10*k - 12*k
	cols := max(1, min(2, int((inner+8*k)/(180*k+8*k))))
	dropped, ok := s.shelfView(c, semantic, sh, &s.lane.tray, k, hidden, animate, func(out float32) {
		for at := 0; at < len(held); at += cols {
			ui.Row(c).Key(fmt.Sprint("row-", at)).WidthPercent(100).Gap(8 * k).AlignItems(ui.Stretch).Children(func() {
				for _, sheet := range held[at:min(at+cols, len(held))] {
					s.trayCard(c, semantic, sheet, (inner-float32(cols-1)*8*k)/float32(cols), out, k, animate)
				}
			})
		}
	}, func(out float32) {
		if len(held) == 0 {
			ui.Column(c).Key("empty").Grow(1).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
				shelfHint(c, semantic, locales.TrayHint(), true, 0, out, k, animate)
			})
		}
	})
	// A sheet comes in with the others picked with it.
	if d, isSheet := dropped.(sheetDrag); ok && isSheet && !d.tray {
		in := s.carried(d)
		s.field.picked = nil
		s.toTray(in)
	}
}

// trayCard is a sheet in the tray, as the field shows a sheet, width wide:
// pressed it opens on the desk, with Option alongside the sheets open;
// dragged out it goes alone; its menu is the sheet's, and puts it back.
// Its words show as much as shown says.
func (s *State) trayCard(c *ui.Context, semantic theme.Theme, sh SheetItem, width, shown, k float32, animate bool) {
	btn := ui.ButtonBase(c.Key("tray-"+sh.ID)).Width(width).Shrink(0).Padding(10*k, 12*k, 10*k, 12*k).Gap(4 * k).
		AlignItems(ui.Start).Radius(8 * k).FocusRing(false).KeepFocus().Label(sheetTitle(sh))
	hover := components.HoverAmount(btn, btn.Hovered(), animate)
	btn.Background(components.Blend(semantic.Surface.Page, semantic.GhostElement.Hover, hover)).
		Border(1, components.Blend(semantic.Border.Variant, semantic.Border.Default, hover))
	// Dragged, it shows its words, though the tray goes back under it.
	if s.dragFrom(btn, sheetDrag{id: sh.ID, tray: true}) {
		shown = 1
	}
	btn.ContextMenu(func(m *ui.Menu) {
		s.sheetMenu(sh, false)(m)
		m.Separator()
		if m.Item(locales.PutBack()).Chosen() {
			s.outOfTray([]string{sh.ID})
		}
	})
	if btn.Clicked() {
		s.Library.OpenSheet(sh.ID, btn.ClickModifiers()&ui.Alt != 0)
		s.ShowDesk()
	}
	btn.Children(func() {
		ui.Column(c).Key("card").WidthPercent(100).Gap(4 * k).Opacity(shown).Children(func() {
			components.Label(c, sheetTitle(sh), theme.TextSmall, semantic.Text.Default).SingleLine()
			if sh.Excerpt != "" {
				components.Label(c, sh.Excerpt, theme.TextXSmall, semantic.Text.Muted).MaxLines(2)
			}
			if sh.Words > 0 {
				components.Label(c, locales.WordsCount(sh.Words, thousands(sh.Words)), theme.TextXSmall, semantic.Text.Muted).SingleLine().FontFeatures("tnum")
			}
		})
	})
}

// sheetsMenu is what a menu does to the sheets ids at once, done after it
// did it.
func (s *State) sheetsMenu(ids []string, done func()) func(*ui.Menu) {
	return func(m *ui.Menu) {
		n := len(ids)
		if m.Item(locales.OpenSheets(n)).Chosen() {
			for _, sh := range ids {
				s.Library.OpenSheet(sh, true)
			}
			done()
			s.ShowDesk()
		}
		m.Separator()
		s.moveMenu(m, func(group string) {
			s.moveSheets(ids, group)
			done()
		}, "")
		m.Separator()
		if m.Item(locales.MoveSheetsToTrash(n)).Chosen() {
			s.trashSheets(ids)
			done()
		}
	}
}
