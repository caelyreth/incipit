package shell

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The desk's switcher (M3) is the scrubber grown open: Go to Sheet (⌘P)
// on the desk opens it where the overview stands, the overview kept at its
// foot, a field to write in at its head and between them the sheets to go
// to, each list saying why it is there:
//
//   - On the desk: the desk's columns, in the strip's order, numbered as
//     the overview's cells are;
//   - Closed lately: what left the desk since the window opened, the
//     latest first;
//   - Near it in a group: the sheets filed next to the one in front;
//   - Last changed: the notebook's other sheets, the latest first.
//
// With words written, the columns they find, then what else they find,
// best first. Return goes to the one the keys are on, in front where it is
// open, else in place of the sheet in front; ⌘Return opens it beside the
// one in front, a column open split again. While open it holds the
// keyboard: a guard over the desk takes the pointer, so passing over a
// column brings none in front, and a press on it, or Escape, puts the
// switcher away and does nothing else. It stays on the desk: the field is
// never shown.

const (
	// switchWidth is the open switcher's width at the reference type size.
	switchWidth = 420
	// switchClosed and switchNear are how many sheets it lists closed
	// lately and near the one in front, switchSheets how many in all past
	// the desk's; switchFound is how many of what words find it lists.
	switchClosed = 3
	switchNear   = 3
	switchSheets = 8
	switchFound  = 12
	// switchRecalled is how many sheets closed it remembers.
	switchRecalled = 8
	// switchPause is how long writing pauses before the lists follow it.
	switchPause = 150 * time.Millisecond
)

// The switcher's lists, named by why their sheets are there.
const (
	listOnDesk  = "On the desk"
	listClosed  = "Closed lately"
	listChanged = "Last changed"
	listFound   = "Found"
)

// switchState is the switcher's between frames.
type switchState struct {
	open, focus bool
	// grown is how far open it is, 0 to 1, as last drawn: the guard over
	// the desk, and the dock fading under it, follow it.
	grown float32
	// height is its open height, and list its lists', both set as it
	// opens.
	height, list float32
	// applied is what is written as the lists last took it, typed when it
	// was last changed: the lists follow once the writing pauses.
	applied string
	typed   time.Time
	// query is what is written, input its field's; at is the row the
	// keys are on, reveal set as the keys move it, to bring it in view.
	query  string
	input  queryInput
	at     int
	reveal bool
	// rows are the rows listed for rowsFor, at the library's revision
	// rowsAt; total is how many sheets what is written finds.
	rows    []switchRow
	rowsFor string
	rowsAt  uint64
	listed  bool
	total   int
	// seen are the sheets on the desk as last seen, and closed those that
	// left it since the window opened, the latest first: kept for the
	// session, never saved.
	seen, closed []string
}

// switchRow is a sheet the switcher lists: open in a column, numbered by
// its place on the strip, or not; section is the list it is in, line the
// line of its text what is written finds.
type switchRow struct {
	column  string
	number  int
	sheet   SheetItem
	group   GroupItem
	section string
	line    string
	front   bool
}

// OpenSwitcher opens the desk's switcher, the keys on the first column
// other than the one in front; it reports whether it opened, as it does
// only on the desk.
func (s *State) OpenSwitcher() bool {
	if !s.OnDesk() {
		return false
	}
	sw := &s.desk.switcher
	sw.open, sw.focus, sw.query, sw.applied, sw.at, sw.reveal = true, true, "", "", 0, true
	sw.height = 0
	for i, row := range s.switchRows() {
		if !row.front {
			sw.at = i
			break
		}
	}
	return true
}

// closeSwitcher puts the switcher away, the sheet in front taking the
// keyboard again.
func (s *State) closeSwitcher() {
	if !s.desk.switcher.open {
		return
	}
	s.desk.switcher.open = false
	s.desk.claim = s.ActiveColumn
}

// noteClosed notes the sheets that left the desk since the last frame,
// for Closed lately.
func (s *State) noteClosed() {
	sw := &s.desk.switcher
	if slices.EqualFunc(sw.seen, s.Open, func(id string, p Page) bool { return id == p.ID }) {
		return
	}
	for _, id := range sw.seen {
		if !slices.ContainsFunc(s.Open, func(p Page) bool { return p.ID == id }) {
			sw.closed = slices.DeleteFunc(sw.closed, func(c string) bool { return c == id })
			sw.closed = slices.Insert(sw.closed, 0, id)
		}
	}
	sw.closed = sw.closed[:min(len(sw.closed), switchRecalled)]
	sw.seen = sw.seen[:0]
	for _, p := range s.Open {
		sw.seen = append(sw.seen, p.ID)
	}
}

// switchRows are what the switcher lists (see the top of the file), read
// again only as what is written, the desk or the library changes.
func (s *State) switchRows() []switchRow {
	sw := &s.desk.switcher
	written := strings.TrimSpace(sw.applied)
	var rev uint64
	if s.Library != nil {
		rev = s.Library.Revision()
	}
	var key strings.Builder
	key.WriteString(written + "\x00" + s.ActiveColumn)
	for _, p := range s.Open {
		key.WriteString("\x00" + p.Column)
	}
	key.WriteString("\x01" + strings.Join(sw.closed, "\x00"))
	if sw.listed && sw.rowsFor == key.String() && sw.rowsAt == rev {
		return sw.rows
	}
	sw.rows, sw.rowsFor, sw.rowsAt, sw.listed, sw.total = nil, key.String(), rev, true, 0
	if s.Library == nil {
		for i, page := range s.Open {
			sw.rows = append(sw.rows, switchRow{column: page.Column, number: i + 1, section: listOnDesk,
				sheet: SheetItem{ID: page.ID, Title: page.Title}, front: page.Column == s.ActiveColumn})
		}
		return sw.rows
	}
	all := s.Library.Sheets(Place{Kind: PlaceAll})
	byID := make(map[string]SheetItem, len(all))
	for _, sh := range all {
		byID[sh.ID] = sh
	}
	groups := map[string]GroupItem{}
	groupOf := func(sheet string) GroupItem {
		id := s.Library.SheetGroup(sheet)
		g, ok := groups[id]
		if !ok {
			g, _ = s.Library.Group(id)
			groups[id] = g
		}
		return g
	}
	listed := map[string]bool{}
	add := func(sh SheetItem, section, line string) {
		listed[sh.ID] = true
		sw.rows = append(sw.rows, switchRow{sheet: sh, group: groupOf(sh.ID), section: section, line: line})
	}

	var found []FoundSheet
	if written != "" {
		f := s.Library.Search(written)
		found, sw.total = f.Sheets, f.Total
	}
	foundID := map[string]bool{}
	for _, f := range found {
		foundID[f.ID] = true
	}
	for i, page := range s.Open {
		listed[page.ID] = true
		if written != "" && !foundID[page.ID] {
			continue
		}
		sh, ok := byID[page.ID]
		if !ok {
			sh = SheetItem{ID: page.ID, Title: page.Title}
		}
		sw.rows = append(sw.rows, switchRow{column: page.Column, number: i + 1, sheet: sh, group: groupOf(page.ID),
			section: listOnDesk, front: page.Column == s.ActiveColumn})
	}
	if written != "" {
		n := 0
		for _, f := range found {
			if !listed[f.ID] && n < switchFound {
				add(f.SheetItem, listFound, f.Line)
				n++
			}
		}
		return sw.rows
	}

	left := switchSheets
	n := 0
	for _, id := range sw.closed {
		if sh, ok := byID[id]; ok && !listed[id] && n < switchClosed {
			add(sh, listClosed, "")
			n++
		}
	}
	left -= n
	if s.ActivePage != "" {
		// The sheets filed nearest the one in front, the one after it
		// first.
		group := groupOf(s.ActivePage)
		filed := s.Library.Sheets(Place{Kind: PlaceGroup, Group: s.Library.SheetGroup(s.ActivePage)})
		if at := slices.IndexFunc(filed, func(sh SheetItem) bool { return sh.ID == s.ActivePage }); at >= 0 {
			section := "Near it"
			if group.Name != "" {
				section += " in " + group.Name
			}
			most := min(switchNear, left)
			n = 0
			for d := 1; n < most && (at+d < len(filed) || at-d >= 0); d++ {
				for _, i := range []int{at + d, at - d} {
					if i >= 0 && i < len(filed) && !listed[filed[i].ID] && n < most {
						add(filed[i], section, "")
						n++
					}
				}
			}
			left -= n
		}
	}
	latest := slices.Clone(all)
	slices.SortStableFunc(latest, func(a, b SheetItem) int { return b.Updated.Compare(a.Updated) })
	for _, sh := range latest {
		if left == 0 {
			break
		}
		if !listed[sh.ID] {
			add(sh, listChanged, "")
			left--
		}
	}
	return sw.rows
}

// switchTo goes to a row: in front where it is open, else in place of the
// sheet in front; beside, next to the sheet in front, a column open split
// again.
func (s *State) switchTo(row switchRow, beside bool) {
	switch {
	case row.column != "" && beside:
		if s.SplitColumnID != nil {
			s.SplitColumnID(row.column)
		}
	case row.column != "":
		if s.SelectColumn != nil {
			s.SelectColumn(row.column)
		}
	case s.Library != nil:
		s.Library.OpenSheet(row.sheet.ID, beside)
	}
	s.closeSwitcher()
	s.ShowDesk()
}

// switchKey is what the keys do in the switcher's field past its
// completion: Up and Down go between the rows, Return goes to the one the
// keys are on, ⌘Return beside. Escape is the desk's (deskEscape).
func (s *State) switchKey(k ui.TextKey) bool {
	sw := &s.desk.switcher
	if k.Key == ui.KeyEnter {
		// Return goes by what is written, paused or not.
		s.applySwitchQuery()
	}
	rows := s.switchRows()
	switch {
	case (k.Key == ui.KeyDown || k.Key == ui.KeyUp) && k.Mods == 0:
		if len(rows) > 0 {
			step := map[ui.Key]int{ui.KeyDown: 1, ui.KeyUp: -1}[k.Key]
			sw.at, sw.reveal = (sw.at+step+len(rows))%len(rows), true
		}
		return true
	case k.Key == ui.KeyEnter && (k.Mods == 0 || k.Mods == ui.Cmd):
		if sw.at >= 0 && sw.at < len(rows) {
			s.switchTo(rows[sw.at], k.Mods == ui.Cmd)
		}
		return true
	}
	return false
}

// switcherView is the open switcher's body, laid out at its open width
// whatever the scrubber's size on the way, so nothing in it reflows as it
// grows: its field, and its lists, at most list high, over a rule; the
// overview and the hint of its keys follow it at the foot (deskFoot).
func (s *State) switcherView(c *ui.Context, semantic theme.Theme, width, list float32, animate bool) ui.Element {
	sw := &s.desk.switcher
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	inset := theme.Space04.DIPs(scale)
	body := ui.Column(c).Key("switch-body").Width(width).Shrink(0).Gap(theme.Space06.DIPs(scale)).
		Padding(inset, inset, 0)
	body.Children(func() {
		field, input := queryField{
			Key: "desk-switch", Label: "Go to sheet", Placeholder: "Go to a sheet…",
			Value: &sw.query, In: &sw.input, Scope: query.InNotebook,
			Focus: sw.focus && sw.open, Animate: animate, Keys: s.switchKey,
		}.View(s, c, semantic)
		field.Shrink(0)
		// The keyboard stays in the field while the switcher is open.
		sw.focus = sw.open && !input.Focused()
		if input.Changed() {
			sw.typed = c.Now()
		}
		if sw.query != sw.applied {
			// The lists follow what is written once it pauses, or at once
			// when it is cleared.
			if wait := switchPause - c.Now().Sub(sw.typed); wait > 0 && strings.TrimSpace(sw.query) != "" {
				c.After(wait)
			} else {
				s.applySwitchQuery()
			}
		}
		rows := s.switchRows()
		sw.at = max(0, min(sw.at, len(rows)-1))
		ui.Scroll(c).Key("switch-scroll").Height(list).Shrink(0).Scrollbars(false).Children(func() {
			ui.Column(c).Key("switch-rows").Role(ui.RoleList).Label("Sheets").Children(func() {
				section := ""
				for i, row := range rows {
					if row.section != section {
						section = row.section
						s.switchSection(c, semantic, section)
					}
					s.switchRowView(c, semantic, i, row, animate)
				}
				if len(rows) == 0 {
					components.CenteredLabel(c, "No sheet holds these words", theme.TextSmall, semantic.Text.Muted).
						Key("switch-none").Height(metrics.Height * 1.5).PaddingX(theme.Space08.DIPs(scale))
				}
			})
		})
		sw.reveal = false
		ui.Box(c).Key("switch-rule").Height(1).Shrink(0).MarginX(-inset).Background(semantic.Border.Variant)
	})
	return body
}

// applySwitchQuery has the lists take what is written, the keys on the
// first row.
func (s *State) applySwitchQuery() {
	sw := &s.desk.switcher
	if sw.applied != sw.query {
		sw.applied, sw.at, sw.reveal = sw.query, 0, true
	}
}

// switchListHeight is how high the lists stand: as the rows listed as it
// opens take, at least five rows, at most most.
func (s *State) switchListHeight(most float32) float32 {
	metrics := theme.DefaultControls.Scaled(s.Settings.UIFontSize)
	row := metrics.Height * 1.5
	rows, section := float32(0), ""
	for _, r := range s.switchRows() {
		if r.section != section {
			section = r.section
			rows += metrics.InlineHeight
		}
		rows += row
	}
	return min(max(rows, metrics.InlineHeight+5*row), most)
}

// switchBodyHeight is how high switcherView lays its body out around
// lists list high: every part of it is of a set height.
func (s *State) switchBodyHeight(list float32) float32 {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	return theme.Space04.DIPs(scale) + metrics.Height + 2*theme.Space06.DIPs(scale) + list + 1
}

// switchSection heads a list with why its sheets are there and, for the
// desk and what words find, how many.
func (s *State) switchSection(c *ui.Context, semantic theme.Theme, name string) {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	note := ""
	switch name {
	case listOnDesk:
		n := 0
		for _, row := range s.desk.switcher.rows {
			if row.column != "" {
				n++
			}
		}
		note = fmt.Sprint(n)
	case listFound:
		note = fmt.Sprint(s.desk.switcher.total)
	}
	ui.Row(c).Key("section-" + name).Height(metrics.InlineHeight).AlignItems(ui.Center).
		PaddingX(theme.Space08.DIPs(scale)).Gap(theme.Space08.DIPs(scale)).Children(func() {
		components.CenteredLabel(c, name, theme.TextXSmall, semantic.Text.Muted).Grow(1).MinWidth(0)
		if note != "" {
			components.CenteredLabel(c, note, theme.TextXSmall, semantic.Text.Muted).Shrink(0).FontFeatures("tnum")
		}
	})
}

// switchRowView is a row: a mark saying where the sheet is (its column's
// number, as the overview's cell, or its group's icon), its title, and
// under it where it is filed, when it changed and how long it is, or the
// line of it what is written finds; the row the keys are on shows the key
// that goes to it.
func (s *State) switchRowView(c *ui.Context, semantic theme.Theme, i int, row switchRow, animate bool) {
	sw := &s.desk.switcher
	scale := s.Settings.UIFontSize
	k := scale / theme.DefaultUIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	key := "sheet-" + row.sheet.ID
	if row.column != "" {
		key = "column-" + row.column
	}
	item := ui.ButtonBase(c.Key(key)).Role(ui.RoleOption).Label(sheetTitle(row.sheet)).Selected(i == sw.at).
		KeepFocus().FocusRing(false).Height(metrics.Height * 1.5).PaddingX(theme.Space08.DIPs(scale)).
		Gap(theme.Space12.DIPs(scale)).AlignItems(ui.Center).Radius(metrics.Radius)
	if item.Hovered() && s.desk.pointed {
		sw.at = i
	}
	at := i == sw.at
	if at && sw.reveal {
		item.ScrollIntoView()
	}
	lit := float32(0)
	if at {
		lit = 1
	}
	if animate {
		lit = components.Follow(item, "lit", lit, components.HoverIn)
	}
	item.Background(semantic.GhostElement.Hover.Alpha(lit))
	if item.Clicked() {
		s.switchTo(row, item.ClickModifiers()&ui.Cmd != 0)
	}
	side := 24 * k
	item.Children(func() {
		mark := ui.Box(c).Key("mark").Size(side, side).Shrink(0).Radius(6 * k).
			AlignItems(ui.Center).Justify(ui.Center)
		if row.column != "" {
			face, ink := semantic.Element.Hover, semantic.Text.Muted
			if at {
				face, ink = semantic.Accent.Muted, semantic.Accent.Foreground
			}
			mark.Background(face).Label("Column " + fmt.Sprint(row.number)).Children(func() {
				components.CenteredLabel(c, fmt.Sprint(row.number), theme.TextXSmall, ink).FontFeatures("tnum")
			})
		} else {
			mark.Background(semantic.Surface.Inset).Border(1, semantic.Border.Variant).Children(func() {
				ui.Icon(c, groupIcon(row.group).SVG()).FontSize(14 * k).
					TextColor(groupInk(semantic, row.group, semantic.Icon.Muted))
			})
		}
		title := semantic.Text.Default
		if at {
			title = semantic.Text.Heading
		}
		ui.Column(c).Key("entry").Grow(1).MinWidth(0).Gap(theme.Space02.DIPs(scale)).Children(func() {
			components.Label(c, sheetTitle(row.sheet), theme.TextSmall, title).SingleLine().WidthPercent(100)
			components.Label(c, switchMeta(row, c), theme.TextXSmall, semantic.Text.Muted).SingleLine().WidthPercent(100)
		})
		if row.front {
			ui.Row(c).Key("front").Shrink(0).Height(18 * k).PaddingX(theme.Space06.DIPs(scale)).Radius(9 * k).
				AlignItems(ui.Center).Background(semantic.Element.Hover).Children(func() {
				components.CenteredLabel(c, "In front", theme.TextXSmall, semantic.Text.Muted)
			})
		} else if at {
			components.Keycaps(c, components.KeycapStyle(semantic, scale, true), []string{"↩"}, "").Shrink(0)
		}
	})
}

// switchMeta is a row's second line: the line found, or where the sheet
// is filed, when it last changed and how many words it holds.
func switchMeta(row switchRow, c *ui.Context) string {
	if line := strings.TrimSpace(row.line); line != "" {
		return line
	}
	var parts []string
	if row.group.Name != "" {
		parts = append(parts, row.group.Name)
	}
	if at := when(row.sheet.Updated, c.Now()); at != "" {
		parts = append(parts, at)
	}
	parts = append(parts, words(row.sheet.Words))
	return strings.Join(parts, " · ")
}

// switchHints are the switcher's keys, at the foot beside the overview.
func (s *State) switchHints(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := s.Settings.UIFontSize
	style := components.KeycapStyle(semantic, scale, true)
	return ui.Row(c).Key("switch-keys").Shrink(0).Gap(theme.Space04.DIPs(scale)).AlignItems(ui.Center).Children(func() {
		for i, hint := range []struct {
			keys  []string
			label string
		}{{[]string{"↩"}, "Go"}, {[]string{"⌘", "↩"}, "Beside"}, {[]string{"esc"}, "Close"}} {
			if i > 0 {
				ui.Box(c).Key(fmt.Sprint("gap-", i)).Width(theme.Space06.DIPs(scale))
			}
			components.Keycaps(c, style, hint.keys, "")
			components.CenteredLabel(c, hint.label, theme.TextXSmall, semantic.Text.Muted)
		}
	})
}
