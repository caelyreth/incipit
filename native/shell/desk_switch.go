package shell

import (
	"strings"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The desk's switcher (M3) is the scrubber grown open: Go to Sheet (⌘P)
// on the desk opens it where the overview stands, the overview kept at its
// foot, a field to write in at its head and between them the sheets to go
// to: the desk's columns, in their order, then the notebook's other
// sheets, the latest first, or what the field's words find, best first.
// Return goes to the one the keys are on, in front where it is open,
// else in place of the sheet in front; ⌘Return opens it beside the one in
// front, a column open split again. Escape, or a press elsewhere, puts it
// away. It stays on the desk: the field is never shown.

const (
	// switchWidth is the open switcher's width at the reference type size.
	switchWidth = 360
	// switchSheets is how many of the notebook's sheets it lists.
	switchSheets = 8
)

// switchState is the switcher's between frames.
type switchState struct {
	open, focus bool
	// query is what is written, input its field's; at is the row the
	// keys are on.
	query string
	input queryInput
	at    int
	// found are the notebook's sheets listed for foundFor, at the
	// library's revision foundAt.
	found    []SheetItem
	foundFor string
	foundAt  uint64
	searched bool
}

// switchRow is a sheet the switcher lists: open in a column, or not.
type switchRow struct {
	column, sheet, title string
	front                bool
}

// OpenSwitcher opens the desk's switcher, the keys on the first column
// other than the one in front; it reports whether it opened, as it does
// only on the desk.
func (s *State) OpenSwitcher() bool {
	if !s.OnDesk() {
		return false
	}
	sw := &s.desk.switcher
	sw.open, sw.focus, sw.query, sw.at = true, true, "", 0
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

// switchRows are what the switcher lists: the columns, in the strip's
// order, those of sheets what is written finds where anything is, then
// the notebook's other sheets.
func (s *State) switchRows() []switchRow {
	sw := &s.desk.switcher
	written := strings.TrimSpace(sw.query)
	sheets := s.switchSheets(written)
	found := map[string]bool{}
	for _, sheet := range sheets {
		found[sheet.ID] = true
	}
	var rows []switchRow
	opened := map[string]bool{}
	for _, page := range s.Open {
		opened[page.ID] = true
		if written != "" && !found[page.ID] {
			continue
		}
		rows = append(rows, switchRow{column: page.Column, sheet: page.ID, title: titled(page.Title),
			front: page.Column == s.ActiveColumn})
	}
	listed := 0
	for _, sheet := range sheets {
		if opened[sheet.ID] || listed == switchSheets {
			continue
		}
		rows = append(rows, switchRow{sheet: sheet.ID, title: titled(sheet.Title)})
		listed++
	}
	return rows
}

// switchSheets are the notebook's sheets the switcher reads: what written
// finds, or the latest; read again only as either changes.
func (s *State) switchSheets(written string) []SheetItem {
	sw := &s.desk.switcher
	if s.Library == nil {
		return nil
	}
	rev := s.Library.Revision()
	if sw.searched && sw.foundFor == written && sw.foundAt == rev {
		return sw.found
	}
	sw.found, sw.foundFor, sw.foundAt, sw.searched = nil, written, rev, true
	if written == "" {
		sw.found = s.Library.Sheets(Place{Kind: PlaceRecent})
	} else {
		for _, f := range s.Library.Search(written).Sheets {
			sw.found = append(sw.found, f.SheetItem)
		}
	}
	return sw.found
}

func titled(title string) string {
	if title == "" {
		return "Untitled"
	}
	return title
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
		s.Library.OpenSheet(row.sheet, beside)
	}
	s.closeSwitcher()
	s.ShowDesk()
}

// switchKey is what the keys do in the switcher's field past its
// completion: Up and Down go between the rows, Return goes to the one the
// keys are on, ⌘Return beside. Escape is the desk's (deskEscape).
func (s *State) switchKey(k ui.TextKey) bool {
	sw := &s.desk.switcher
	rows := s.switchRows()
	switch {
	case k.Key == ui.KeyDown && k.Mods == 0, k.Key == ui.KeyUp && k.Mods == 0:
		if len(rows) > 0 {
			step := map[ui.Key]int{ui.KeyDown: 1, ui.KeyUp: -1}[k.Key]
			sw.at = (sw.at + step + len(rows)) % len(rows)
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

// switcherView is the open switcher's content, in the grown scrubber:
// its field, its rows, and the hint of its keys; the overview follows it
// at the foot (deskFoot).
func (s *State) switcherView(c *ui.Context, semantic theme.Theme, animate bool) {
	sw := &s.desk.switcher
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	rows := s.switchRows()
	sw.at = max(0, min(sw.at, len(rows)-1))
	field, input := queryField{
		Key: "desk-switch", Label: "Go to sheet", Placeholder: "Go to a sheet",
		Value: &sw.query, In: &sw.input, Scope: query.InNotebook,
		Focus: sw.focus, Animate: animate, Keys: s.switchKey,
	}.View(s, c, semantic)
	field.Shrink(0)
	sw.focus = false
	if input.Changed() {
		// What is written lists other rows: the keys go to the first.
		sw.at = 0
	}
	list := ui.Column(c).Key("switch-rows").Role(ui.RoleList).Label("Sheets").Shrink(0)
	list.Children(func() {
		section := ""
		for i, row := range rows {
			if name := map[bool]string{true: "On the desk", false: "Sheets"}[row.column != ""]; name != section {
				section = name
				components.CenteredLabel(c, name, theme.TextXSmall, semantic.Text.Muted).
					Key("section-" + name).Height(metrics.InlineHeight).PaddingX(theme.Space08.DIPs(scale))
			}
			key := "sheet-" + row.sheet
			if row.column != "" {
				key = "column-" + row.column
			}
			item := ui.ButtonBase(c.Key(key)).Role(ui.RoleOption).Label(row.title).Selected(i == sw.at).
				KeepFocus().FocusRing(false).Height(metrics.Height).PaddingX(theme.Space08.DIPs(scale)).
				Gap(theme.Space08.DIPs(scale)).AlignItems(ui.Center).Radius(metrics.Radius)
			if item.Hovered() {
				sw.at = i
			}
			lit := float32(0)
			if i == sw.at {
				lit = 1
			}
			if animate {
				lit = components.Follow(item, "lit", lit, components.HoverIn)
			}
			item.Background(semantic.GhostElement.Hover.Alpha(lit))
			if item.Clicked() {
				s.switchTo(row, item.ClickModifiers()&ui.Cmd != 0)
			}
			item.Children(func() {
				label := components.CenteredLabel(c, row.title, theme.TextSmall, semantic.Text.Default)
				label.Grow(1).MinWidth(0)
				if row.front {
					components.CenteredLabel(c, "In front", theme.TextXSmall, semantic.Text.Muted).Shrink(0)
				}
			})
		}
		if len(rows) == 0 {
			components.CenteredLabel(c, "No sheets found", theme.TextSmall, semantic.Text.Muted).
				Key("switch-none").Height(metrics.Height).PaddingX(theme.Space08.DIPs(scale))
		}
	})
	style := components.KeycapStyle(semantic, scale, true)
	ui.Row(c).Key("switch-keys").Shrink(0).Gap(theme.Space08.DIPs(scale)).AlignItems(ui.Center).
		PaddingX(theme.Space08.DIPs(scale)).Children(func() {
		components.Keycaps(c, style, []string{"↩"}, "")
		components.CenteredLabel(c, "Go", theme.TextXSmall, semantic.Text.Muted)
		components.Keycaps(c, style, []string{"⌘", "↩"}, "")
		components.CenteredLabel(c, "Open beside", theme.TextXSmall, semantic.Text.Muted)
	})
}
