package shell

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The field lane stands at the window's left edge as the desk lane does
// at its right, as wide, so the field and its pill stand in the window's
// middle (field-and-desk.md §4.6). It holds what tunes the field: the
// queries kept, and the settings at its foot. The queries kept rest in a
// box half out of the window, its edge and the bookmarks standing out of
// it all that shows; the pointer on it, or a ribbon dragged to it, draws
// it out. A bookmark is a smart folder of the notebook's: pressed, its
// query becomes the pill's; its menu renames it or lets it go. The query
// held, or a ribbon dropped on the box, is kept as one. The lane goes
// aside with the field as the desk comes up.

// queryDrag is a condition dragged from its ribbon, to be kept as a bookmark.
type queryDrag struct{ text string }

func (queryDrag) shellDrag() {}

// bookmarks are the queries kept, the notebook's smart folders, in order.
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
	return out
}

// sameQuery reports whether a and b are the one query, their conditions
// and words written alike.
func sameQuery(a, b string) bool {
	ac, aw := splitQuery(a)
	bc, bw := splitQuery(b)
	return strings.Join(ac, " ") == strings.Join(bc, " ") && strings.TrimSpace(aw) == strings.TrimSpace(bw)
}

// saveQuery keeps text as a bookmark, named as its conditions and words say.
func (s *State) saveQuery(text string) {
	text = strings.TrimSpace(text)
	if text == "" || s.Library == nil {
		return
	}
	for _, g := range s.bookmarks() {
		if sameQuery(g.Query, text) {
			return
		}
	}
	s.Library.NewSmartGroup(queryName(text), text)
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
func (s *State) fieldLaneView(c *ui.Context, semantic theme.Theme, look stage.Look, w, h float32, hidden, animate bool) {
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	lane := ui.Box(c).Key("field-lane").Absolute().Top(0).Left(0).Size(s.laneRoom(), h).Hidden(hidden).
		Role(ui.RoleGroup).Label("Field lane")
	if look.Offset.X != 0 {
		lane.Translate(look.Offset.X, 0)
	}
	lane.Children(func() {
		ui.Box(c).Key("title-band").Absolute().Top(0).Left(0).Right(0).Height(titleLine(c)).DragWindow()
		// The settings stand in the corner of what is left free beside the
		// field's columns, as far from the window's foot as from its edge.
		size, mid := settingsSize*k, s.fieldMargin(w, h)/2
		s.settingsButton(c, semantic, size, animate).Absolute().Left(mid - size/2).Bottom(mid - size/2)
	})
	s.keptBox(c, semantic, look, h, k, hidden, animate)
}

// keptBox draws the shelf the queries kept rest in, h being the window's
// height, centred as the desk rests at the other edge: a query dragged to
// it is kept, and it stays out while a bookmark in it is renamed.
func (s *State) keptBox(c *ui.Context, semantic theme.Theme, look stage.Look, h, k float32, hidden, animate bool) {
	marks := s.bookmarks()
	text := s.FieldQuery()
	keep := text != "" && !strings.HasPrefix(s.field.words, ">") &&
		!slices.ContainsFunc(marks, func(g GroupItem) bool { return sameQuery(g.Query, text) })
	sh := shelf{
		key: "kept", name: "Kept", label: "Kept searches", count: len(marks),
		width: shelfWidth * k, tail: shelfTail * k, top: h / 2, most: h - 192*k, centred: true, offset: look.Offset.X,
		takes: takes{queries: true},
		out:   slices.ContainsFunc(marks, func(g GroupItem) bool { return g.ID == s.field.renaming }),
	}
	dropped, ok := s.shelfView(c, semantic, sh, &s.field.kept, k, hidden, animate, func(out float32) {
		for _, g := range marks {
			s.bookmark(c, semantic, g, sameQuery(g.Query, text), out, k, animate)
		}
	}, func(out float32) {
		switch {
		case keep:
			s.keepSlip(c, semantic, text, (shelfWidth-32)*k, k, animate).Opacity(out).Shrink(0)
		case len(marks) == 0:
			shelfHint(c, semantic, "Searches kept rest here, to be found again.", false, shelfTail*k, out, k)
		}
	})
	if ok {
		s.saveQuery(dropped.(queryDrag).text)
	}
}

// bookmark is a kept query, a bookmark standing out of the box in its
// group's color: pressed, it is the pill's query; its menu renames it or
// lets it go. The one the pill holds stands further out. Its words are
// shown as much as shown says, gone while the box rests.
func (s *State) bookmark(c *ui.Context, semantic theme.Theme, g GroupItem, current bool, shown, k float32, animate bool) {
	tail := shelfTail * k
	btn := ui.ButtonBase(c.Key("bookmark-"+g.ID)).WidthPercent(100).Padding(7*k, tail+14*k, 7*k, 10*k).Gap(8 * k).
		AlignItems(ui.Center).FocusRing(false).KeepFocus().Label(g.Name).Tooltip(g.Name)
	hover := components.HoverAmount(btn, btn.Hovered(), animate)
	// The ribbon is dyed full in its hue, its words cut from it in the
	// page's color; the one in use is pulled further out of the box.
	f := s.bookmarkFamily(semantic, g)
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
		if m.Item("Let Go").Chosen() {
			s.Library.TrashGroup(g.ID)
		}
	})
	if btn.Clicked() && s.field.renaming != g.ID {
		s.SetFieldQuery(g.Query)
		s.field.next = change{depth: stage.Down}
		s.field.focus = true
	}
	btn.Children(func() {
		if s.field.renaming == g.ID {
			s.renameInput(c, semantic, g)
			return
		}
		components.Label(c, g.Name, theme.TextXSmall, semantic.Surface.Page).MaxLines(2).Grow(1).MinWidth(0).Opacity(shown)
		components.Label(c, thousands(g.Count), theme.TextXSmall, semantic.Surface.Page.Alpha(0.7)).SingleLine().Shrink(0).Opacity(shown)
	})
}

// bookmarkFamily is the hue a kept query's bookmark is dyed in: its
// group's, or the accent's.
func (s *State) bookmarkFamily(semantic theme.Theme, g GroupItem) theme.Family {
	if g.Color >= TintHalo && g.Color < TintHalo+len(semantic.Families) {
		return semantic.Families[g.Color-TintHalo]
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

// keepSlip is where the query held is kept from, width wide within the
// box: pressed, it is a bookmark.
func (s *State) keepSlip(c *ui.Context, semantic theme.Theme, text string, width, k float32, animate bool) ui.Element {
	btn := ui.ButtonBase(c.Key("keep")).Width(width).Padding(7*k, 8*k, 7*k, 10*k).Gap(6 * k).
		AlignItems(ui.Center).Radius(5 * k).FocusRing(false).KeepFocus().
		Label("Keep this search").Tooltip("Keep " + queryName(text))
	btn.Background(components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, components.HoverAmount(btn, btn.Hovered(), animate))).
		Draw(func(p *ui.Painter, r ui.Rect) { components.DashedOutline(p, r, 5*k, 1, semantic.Border.Default) })
	if btn.Clicked() {
		s.saveQuery(text)
	}
	btn.Children(func() {
		ui.Icon(c, components.IconPlus.SVG()).FontSize(12 * k).TextColor(semantic.Icon.Muted).Shrink(0)
		components.Label(c, "Keep this search", theme.TextXSmall, semantic.Text.Muted).SingleLine()
	})
	return btn
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
