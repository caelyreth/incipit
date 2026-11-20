package shell

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The field lane stands at the window's left edge as the desk lane does
// at its right, as wide, so the field and its pill stand in the window's
// middle (field-and-desk.md §4.6). It holds what tunes the field: Home and
// the trash at its top, the queries kept, and the settings at its foot. The queries kept rest in a
// box half out of the window, its edge and the bookmarks standing out of
// it all that shows; the pointer on it, or a ribbon dragged to it, draws
// it out. A bookmark is a smart folder of the notebook's, the newest on
// top: pressed, its query becomes the pill's; its × or its menu deletes
// it, and its menu renames it. Each is dyed at its making in the hue the
// others wear least, and keeps it. The query held stands on top as the
// bookmark it would be, drawn in dashes in the hue it would take; pressed,
// or a ribbon dropped on the box, it is kept. Bookmarks come in and go by
// folding open and shut, the box growing and shrinking with them. The lane
// goes aside with the field as the desk comes up.

// queryDrag is a condition dragged from its ribbon, to be kept as a bookmark.
type queryDrag struct{ text string }

func (queryDrag) shellDrag() {}

// bookmarks are the queries kept, the notebook's smart folders, the newest
// first: each is kept after the others.
func (s *State) bookmarks() []GroupItem {
	var out []GroupItem
	var walk func(parent string)
	walk = func(parent string) {
		if s.Library == nil {
			return
		}
		for _, g := range s.Library.Groups(parent) {
			switch {
			case g.Smart():
				out = append(out, g)
			case g.HasGroups:
				walk(g.ID)
			}
		}
	}
	walk("")
	slices.Reverse(out)
	return out
}

// saveQuery keeps text as a bookmark, named as its conditions and words
// say, dyed for good in the hue the next is given.
func (s *State) saveQuery(text string) {
	text = strings.TrimSpace(text)
	if text == "" || s.Library == nil {
		return
	}
	marks := s.bookmarks()
	for _, g := range marks {
		if sameQuery(g.Query, text) {
			return
		}
	}
	if id := s.Library.NewSmartGroup(queryName(text), text); id != "" {
		s.Library.StyleGroup(id, "", nextTint(marks))
	}
}

// nextTint is the hue a bookmark kept after marks, the newest first, is
// dyed in: the one they wear least, looking from the hue after the
// newest's, so that kept one after another they go round the hues.
func nextTint(marks []GroupItem) int {
	hues := len(theme.AllAccents)
	worn := make([]int, hues)
	from := 0
	for i, g := range marks {
		if g.Color < TintHalo || g.Color >= TintHalo+hues {
			continue
		}
		worn[g.Color-TintHalo]++
		if i == 0 {
			from = g.Color - TintHalo + 1
		}
	}
	least := from % hues
	for i := range hues {
		if h := (from + i) % hues; worn[h] < worn[least] {
			least = h
		}
	}
	return TintHalo + least
}

// queryName is a query said in words: its conditions' sentences, then its
// words quoted.
func queryName(text string) string {
	conds, words := splitQuery(text)
	parts := make([]string, 0, len(conds)+1)
	for _, c := range conds {
		if cond, ok := condition(c); ok {
			parts = append(parts, cond.Sentence())
		}
	}
	if w := strings.TrimSpace(words); w != "" {
		parts = append(parts, "“"+w+"”")
	}
	return strings.Join(parts, " · ")
}

// The kept box's measures at the reference type size.
const (
	// settingsSize is the settings button's width.
	settingsSize = 32
	// keptNotch is how deep a bookmark's tail is cut, and keptPull how much
	// further the one in use stands out than the others.
	keptNotch, keptPull = 6, 6
)

// fieldLaneView draws the field lane at the left of the window w by h, as
// look stands the field, and the kept box over it.
func (s *State) fieldLaneView(c *ui.Context, semantic theme.Theme, look stageLook, w, h float32, hidden, animate bool) {
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	lane := ui.Box(c).Key("field-lane").Absolute().Top(0).Left(0).Size(s.laneRoom(), h).Hidden(hidden).
		Role(ui.RoleGroup).Label("Field lane")
	if hidden {
		return
	}
	if look.Offset.X != 0 {
		lane.Translate(look.Offset.X, 0)
	}
	var buttonsFoot float32
	lane.Children(func() {
		barH := titleLine(c)
		ui.Box(c).Key("title-band").Absolute().Top(0).Left(0).Right(0).Height(barH).DragWindow()
		// The lane's buttons stand in one line beside the field's columns:
		// Home under the title line, the trash under it, and the settings
		// in the corner at the foot.
		size, mid := settingsSize*k, s.fieldMargin(w, h)/2
		inset := mid - size/2
		homeTop := max(inset, barH+theme.DefaultLayout.ChromeGutter*k)
		trashTop := homeTop + size + 8*k
		// How the place is shown, apart from which place it is.
		listTop := trashTop + size + 20*k
		buttonsFoot = listTop + size
		s.homeButton(c, semantic, size, animate).Absolute().Left(inset).Top(homeTop)
		s.trashButton(c, semantic, size, animate).Absolute().Left(inset).Top(trashTop)
		s.listButton(c, semantic, size, animate).Absolute().Left(inset).Top(listTop)
		s.settingsButton(c, semantic, size, animate).Absolute().Left(inset).Bottom(inset)
	})
	s.keptBox(c, semantic, look, h, k, buttonsFoot, hidden, animate)
}

// trashButton shows the trash in the field, or leaves it, filled while it
// is shown.
func (s *State) trashButton(c *ui.Context, semantic theme.Theme, size float32, animate bool) ui.Element {
	shown := s.inTrash()
	label := map[bool]string{true: "Leave Trash", false: "Show Trash"}[shown]
	btn := laneCircleButton(c, semantic, "trash", components.IconTrash, label, "Trash", shown, size, animate)
	if btn.Clicked() {
		s.toggleTrash()
	}
	// What is dropped on it goes to the trash.
	if !shown {
		if _, dropped, ok := s.dropOn(c, semantic, btn, takes{sheets: true, groups: true}, animate); ok {
			s.trashDropped(dropped)
		}
	}
	return btn
}

func (s *State) homeButton(c *ui.Context, semantic theme.Theme, size float32, animate bool) ui.Element {
	home := !s.inTrash() && !s.InSettings()
	btn := laneCircleButton(c, semantic, "home", components.IconHome, "Home", "Home", home, size, animate)
	if btn.Clicked() {
		s.goHome()
	}
	return btn
}

// keptBox draws the shelf the queries kept rest in, h being the window's
// height, aligned at the foot of a lower-lane region so it grows upward
// while leaving the upper side lane blank: a query dragged to it is kept,
// and it stays out while a bookmark in it is renamed.
func (s *State) keptBox(c *ui.Context, semantic theme.Theme, look stageLook, h, k, buttonsFoot float32, hidden, animate bool) {
	marks := s.bookmarks()
	text := s.FieldQuery()
	keep := text != "" && !s.commanding() &&
		!slices.ContainsFunc(marks, func(g GroupItem) bool { return sameQuery(g.Query, text) })
	topClearance, bottomClearance := max(80*k, buttonsFoot+36*k), 88*k
	most := min(h*0.36, max(0, h-topClearance-bottomClearance))
	bottomY := max(topClearance+most, min(h*0.86, h-bottomClearance))
	sh := shelf{
		key: "kept", icon: components.IconBookmark, label: "Kept searches", count: len(marks), listing: true,
		width: shelfWidth * k, tail: shelfTail * k, top: bottomY, most: most, bottom: true, offset: look.Offset.X,
		takes: takes{queries: true},
		out:   slices.ContainsFunc(marks, func(g GroupItem) bool { return g.ID == s.field.renaming }),
	}
	dropped, ok := s.shelfView(c, semantic, sh, &s.field.kept, k, hidden, animate, func(out float32) {
		if keep {
			s.draftBookmark(c, semantic, text, s.tintFamily(semantic, nextTint(marks)), out, k, animate)
		}
		rows := s.keptRows(marks, animate)
		gone := map[string]bool{}
		for _, r := range rows {
			if s.bookmark(c, semantic, r, sameQuery(r.g.Query, text), out, k, animate) {
				gone[r.g.ID] = true
			}
		}
		s.field.keptRows = slices.DeleteFunc(rows, func(r keptRow) bool { return gone[r.g.ID] })
	}, func(out float32) {
		if len(marks) == 0 && !keep {
			shelfHint(c, semantic, "Searches kept rest here, to be found again.", false, shelfTail*k, out, k, animate)
		}
	})
	if ok {
		s.saveQuery(dropped.(queryDrag).text)
	}
}

// keptRow is a bookmark as the box shows it, or one going, which folds
// shut where it stood.
type keptRow struct {
	g       GroupItem
	leaving bool
}

// keptRows are marks as the box shows them: those it showed and no longer
// holds stay, going, where they stood among them, so that those over one
// going come down as it folds shut, the box standing on its foot.
func (s *State) keptRows(marks []GroupItem, animate bool) []keptRow {
	at := map[string]int{}
	for i, g := range marks {
		at[g.ID] = i
	}
	var out []keptRow
	next := 0
	for _, r := range s.field.keptRows {
		if i, ok := at[r.g.ID]; ok {
			for ; next <= i; next++ {
				out = append(out, keptRow{g: marks[next]})
			}
		} else if animate {
			out = append(out, keptRow{g: r.g, leaving: true})
		}
	}
	for ; next < len(marks); next++ {
		out = append(out, keptRow{g: marks[next]})
	}
	return out
}

// bookmark is a kept query, a bookmark standing out of the box in its
// group's color: pressed, it is the pill's query; its × deletes it, and
// its menu renames or deletes it. The one the pill holds stands further
// out. Its words are shown as much as shown says, gone while the box
// rests. Going, it folds shut where it stands, taking its room with it, so
// that those over it come down as it does; it reports when it is shut.
func (s *State) bookmark(c *ui.Context, semantic theme.Theme, r keptRow, current bool, shown, k float32, animate bool) bool {
	g, tail := r.g, shelfTail*k
	btn := ui.ButtonBase(c.Key("bookmark-"+g.ID)).WidthPercent(100).Padding(7*k, tail+14*k, 7*k, 10*k).Gap(8 * k).
		AlignItems(ui.Center).FocusRing(false).KeepFocus().Label(g.Name).Tooltip(g.Name).
		Transition(bookmarkTransition(animate, k))
	staying := map[bool]float32{true: 1}[!r.leaving]
	if animate {
		staying = components.Follow(btn, "staying", staying, components.MotionPanel)
	}
	if r.leaving {
		// Its own room goes with it, and the gap over it with its room.
		h := s.field.keptH[g.ID]
		btn.Height(h*staying).Padding(7*k*staying, tail+14*k, 7*k*staying, 10*k).
			Clip().IgnoreInput(true).Opacity(staying).Margin(-6*k*(1-staying), 0, 0, 0)
		if staying < 0.01 {
			return true
		}
	} else if h := btn.LayoutBounds().H; h > 0 {
		if s.field.keptH == nil {
			s.field.keptH = map[string]float32{}
		}
		s.field.keptH[g.ID] = h
	}
	hover := components.HoverAmount(btn, btn.Hovered(), animate)
	// The ribbon is dyed full in its hue, its words cut from it in the
	// page's color; the one in use is pulled further out of the box.
	f := s.tintFamily(semantic, g.Color)
	fill := components.Blend(f.Foreground, f.Emphasis, 0.25*hover)
	pulled := map[bool]float32{true: 1}[current]
	if animate {
		pulled = components.Follow(btn, "pulled", pulled, components.MotionPanel)
	}
	btn.Draw(func(p *ui.Painter, r ui.Rect) {
		r.W -= keptPull * k * (1 - pulled)
		var ribbon ui.Path
		bookmarkOutline(&ribbon, r, 5*k, keptNotch*k)
		p.FillPath(&ribbon, fill)
	})
	btn.ContextMenu(func(m *ui.Menu) {
		if m.Item("Rename…").Chosen() {
			s.startRename(g.ID)
		}
		m.Separator()
		if m.Item("Delete").Chosen() {
			s.deleteBookmark(g.ID)
		}
	})
	if btn.Clicked() && s.field.renaming != g.ID {
		s.SetFieldQuery(g.Query)
		s.field.next = change{depth: stageDown}
		s.field.focus = true
	}
	btn.Children(func() {
		if s.field.renaming == g.ID {
			s.renameInput(c, semantic, g)
			return
		}
		components.Label(c, g.Name, theme.TextXSmall, semantic.Surface.Page).MaxLines(2).Grow(1).MinWidth(0).Opacity(shown).
			Translate(0, -components.UIOpticalLift(s.Settings.UIFontSize))
		// The pointer on it, its count gives way to its ×.
		ui.Box(c).Key("end").Shrink(0).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			components.CenteredLabel(c, thousands(g.Count), theme.TextXSmall, semantic.Surface.Page).
				Opacity(shown * (1 - hover))
			miniCloseButton(c, "delete", "Delete "+g.Name, "Delete", 18*k, 11*k,
				semantic.Surface.Page, semantic.Surface.Page.Alpha(0.25), animate,
				func() { s.deleteBookmark(g.ID) }).
				Absolute().Opacity(shown * hover).IgnoreInput(hover < 0.5)
		})
	})
	return false
}

// bookmarkTransition folds a bookmark open as it comes, out of the box's
// edge, and shut as it goes; only its colors move besides. The box stands
// on its foot and grows up over what comes in, so those kept before stay
// where they stand: a move of their own, from where they stood in the list
// as it grew, would throw them up the height of the one coming and back.
func bookmarkTransition(animate bool, k float32) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: -1}
	}
	spring := components.Spring(components.MotionPanel)
	gone := &ui.Motion{X: -12 * k, Opacity: 0, Collapse: true}
	return ui.ElementTransition{
		Animation: &spring, ExitDuration: components.MotionQuick, ExitEase: ui.EaseOut,
		Colors: true, Enter: gone, Exit: gone,
	}
}

// tintFamily is the hue a bookmark of tint is dyed in: that hue's, or the
// accent's for none.
func (s *State) tintFamily(semantic theme.Theme, tint int) theme.Family {
	if tint >= TintHalo && tint < TintHalo+len(semantic.Families) {
		return semantic.Families[tint-TintHalo]
	}
	return semantic.Families[s.Settings.Accent]
}

// bookmarkOutline adds a bookmark's outline in r to path: its head rounded by
// radius, its tail at the right cut notch deep.
func bookmarkOutline(path *ui.Path, r ui.Rect, radius, notch float32) {
	x0, y0, x1, y1 := r.X, r.Y, r.X+r.W, r.Y+r.H
	q := radius * (1 - 0.5522848)
	path.MoveTo(x0+radius, y0)
	path.LineTo(x1, y0)
	path.LineTo(x1-notch, (y0+y1)/2)
	path.LineTo(x1, y1)
	path.LineTo(x0+radius, y1)
	path.CubeTo(x0+q, y1, x0, y1-q, x0, y1-radius)
	path.LineTo(x0, y0+radius)
	path.CubeTo(x0, y0+q, x0+q, y0, x0+radius, y0)
	path.Close()
}

// draftBookmark is the query held, text, as the bookmark it would be, on
// top of those kept: its outline drawn in dashes, dimly dyed in f, the hue
// it would take, its words the name it would bear. Pressed, it is kept.
// Its words are shown as much as shown says.
func (s *State) draftBookmark(c *ui.Context, semantic theme.Theme, text string, f theme.Family, shown, k float32, animate bool) {
	tail := shelfTail * k
	btn := ui.ButtonBase(c.Key("keep")).WidthPercent(100).Padding(7*k, tail+14*k, 7*k, 10*k).Gap(6 * k).
		AlignItems(ui.Center).FocusRing(false).KeepFocus().
		Label("Keep this search").Tooltip("Keep this search").Transition(bookmarkTransition(animate, k))
	hover := components.HoverAmount(btn, btn.Hovered(), animate)
	// Kept, it would stand as far out as the one in use; until then it
	// stands back as the others do.
	btn.Draw(func(p *ui.Painter, r ui.Rect) {
		r.W -= keptPull * k
		var ribbon ui.Path
		bookmarkOutline(&ribbon, r, 5*k, keptNotch*k)
		p.FillPath(&ribbon, f.Foreground.Alpha(0.10+0.12*hover))
		var edge ui.Path
		bookmarkOutline(&edge, ui.Rect{X: r.X + 0.5, Y: r.Y + 0.5, W: r.W - 1, H: r.H - 1}, 5*k, keptNotch*k)
		p.StrokePath(edge.Cap(ui.ButtCap).Dash(0, 4*k, 3*k), 1, f.Foreground.Alpha(0.55+0.35*hover))
	})
	if btn.Clicked() {
		s.saveQuery(text)
	}
	btn.Children(func() {
		ui.Icon(c, components.IconPlus.SVG()).FontSize(11 * k).TextColor(f.Foreground).Shrink(0).Opacity(shown)
		components.Label(c, queryName(text), theme.TextXSmall, f.Foreground).MaxLines(2).Grow(1).MinWidth(0).Opacity(shown).
			Translate(0, -components.UIOpticalLift(s.Settings.UIFontSize))
	})
}

// splitQuery is text's conditions, each as the language writes it, and
// what is written besides them.
func splitQuery(text string) (conds []string, words string) {
	found := query.Conditions(query.Parse(text))
	for i := len(found) - 1; i >= 0; i-- {
		text = query.Cut(text, found[i].Span)
	}
	for _, c := range found {
		conds = append(conds, c.String())
	}
	return conds, strings.TrimLeft(text, " ")
}
