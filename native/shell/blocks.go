package shell

import (
	"fmt"
	"math"
	"strings"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A block is a group as the field shows it: its head, then what it holds
// in full, row by row, as the sheet list showed it, one column wide and no
// taller than its half of the field holds, the rest counted. A block is as
// tall as what it holds, laid out once and moved as a whole
// (stage.Frame), its layer kept.

// shape is how a block stands: its rows, column by column, its size, and
// how many of its sheets it leaves out.
type shape struct {
	size stage.Size
	cols [][]row
	more int
}

// row is one row of a card: a group inside it, a sheet, a command, a
// settings section, the area that makes a sheet there, or what it leaves
// out.
type row struct {
	group   *GroupItem
	entry   *entry
	command *Command
	setting *SettingsSection
	create  bool
	h       float32
}

// blockMetrics are the measures blocks are laid out to, at the type's
// size, for a field w by h.
type blockMetrics struct {
	scale, k       float32
	col, pad, head float32
	// headGap is the room between a card's head and its first row, and
	// rowGap between rows: no two things that light under the pointer
	// touch (field-and-desk.md §4.7.2).
	headGap float32
	// tall is how tall a block's rows may stand in its half of the field,
	// and alone how tall where its row is all the field holds, nothing
	// over the pill.
	rowGap, tall      float32
	alone             float32
	groupRow, control float32
	horizon           stage.Horizon
}

// measureStep is the step, at the reference type size, the field's
// measures are taken in from the window's.
const measureStep = 8

// fieldGap is the room between columns in the field and between the field
// and the desk lane, at the reference type size.
const fieldGap = 14

// fieldMetrics are the normal field's column measures, fixed for the
// window's size and type scale regardless of what query is shown.
func (s *State) fieldMetrics(w, h float32) blockMetrics {
	return s.columnMetrics(w, h, 260, 340)
}

func (s *State) blockMetrics(w, h float32) blockMetrics {
	// Live settings cards are wider, for a setting's words and its control
	// to stand side by side; category cards share the library's measure.
	if s.settingsLive() {
		return s.columnMetrics(w, h, 400, 560)
	}
	return s.fieldMetrics(w, h)
}

func (s *State) columnMetrics(w, h, least, most float32) blockMetrics {
	scale := s.Settings.UIFontSize
	k := scale / theme.DefaultUIFontSize
	pad, gap := 6*k, fieldGap*k
	cols, col := stage.Columns(w-48*k, least*k, most*k, gap)
	// What the window gives is taken in steps, so that resizing it lays
	// the cards out again a step at a time and between steps only moves
	// them, as they were drawn.
	step := measureStep * k
	if col < most*k {
		col = max(step, float32(math.Floor(float64(col/step)))*step)
	}
	band := 40*k + 2*gap
	// What a card takes past its rows: its head, the room under it, and
	// its padding.
	head, headGap := 36*k, 6*k
	frame := head + headGap + 2*pad
	return blockMetrics{
		scale: scale, k: k,
		col: col - 2*pad, pad: pad, head: head, headGap: headGap, rowGap: 4 * k,
		// A block is no taller than its half of the field holds, or, alone
		// in its row, than the window holds about the pill.
		tall:     max(160*k, float32(math.Floor(float64((h/2-band/2-gap-frame)/step)))*step),
		alone:    max(160*k, float32(math.Floor(float64((h-band-2*gap-frame-(48+24)*k)/step)))*step),
		groupRow: 2*theme.Space06.DIPs(scale) + theme.TextSmall.LineHeight(scale),
		control:  theme.DefaultControls.Scaled(scale).Height,
		horizon:  stage.Horizon{Cols: cols, Col: col, Gap: gap, Above: band / 2, Below: band / 2},
	}
}

// shapes are the cards' shapes, in their order, each worked out again only
// when what they hold or the measures change.
func (s *State) shapes(c *ui.Context, f found, m blockMetrics) []shape {
	key := fmt.Sprintf("%s|%d|%s|%d|%d|%v", f.query, f.rev, f.open, f.held, s.field.measures, m)
	if s.field.shapesKey != key {
		s.field.shapes, s.field.shapesKey = map[string]shape{}, key
	}
	out := make([]shape, len(f.cards))
	for i, cd := range f.cards {
		sh, ok := s.field.shapes[cd.id]
		if !ok {
			sh = cd.kind.shape(s, c, m)
			s.field.shapes[cd.id] = sh
		}
		out[i] = sh
	}
	return out
}

func (b block) shape(s *State, c *ui.Context, m blockMetrics) shape {
	switch {
	case b.single:
		h := entryHeight(c, b.entries[0], m, 4)
		return shape{size: stage.Size{W: m.col + 2*m.pad, H: h + 2*m.pad}, cols: [][]row{{{entry: &b.entries[0], h: h}}}}
	case b.create:
		return shape{size: stage.Size{W: m.col + 2*m.pad, H: m.control + 2*m.pad}, cols: [][]row{{{create: true, h: m.control}}}}
	}
	rows := make([]row, 0, len(b.groups)+len(b.entries)+1)
	for i := range b.groups {
		rows = append(rows, row{group: &b.groups[i], h: m.groupRow})
	}
	for i := range b.entries {
		rows = append(rows, row{entry: &b.entries[i], h: entryHeight(c, b.entries[i], m, 2)})
	}
	// Where sheets can be made, the area that makes one ends the block.
	if g := b.group; g != nil && !b.trashed && !g.Smart() {
		rows = append(rows, row{create: true, h: m.control})
	}
	return stack(rows, m)
}

func (b block) rows() []string {
	ids := make([]string, 0, len(b.groups)+len(b.entries))
	for _, g := range b.groups {
		ids = append(ids, "group:"+g.ID)
	}
	for _, e := range b.entries {
		ids = append(ids, "sheet:"+e.sheet.ID)
	}
	return ids
}

func (b block) ground() bool { return !b.create }

// A block is drawn anew while its group's name is being written.
func (b block) drawsLive(s *State) bool {
	return b.group != nil && s.field.renaming == b.group.ID && !b.create
}

// The parts of a sheet's row, as the sheet list set them.
func rowPad(scale float32) float32 { return theme.Space08.DIPs(scale) }
func rowGap(scale float32) float32 { return theme.Space02.DIPs(scale) }

// tagsHeight is the height of a row's line of tags.
func tagsHeight(scale float32) float32 {
	return theme.TextXSmall.LineHeight(scale) + 3*theme.Space04.DIPs(scale)
}

// entryHeight is how tall a sheet's row stands: its title and time, its
// opening, or where it matches, in up to lines lines, its tags, and its
// words.
func entryHeight(c *ui.Context, e entry, m blockMetrics, lines int) float32 {
	pad, gap := rowPad(m.scale), rowGap(m.scale)
	xs := theme.TextXSmall.LineHeight(m.scale)
	h := 2*pad + theme.TextSmall.LineHeight(m.scale) + gap + xs
	if body := entryBody(e); body != "" {
		h += gap + float32(bodyLines(c, body, m.col-2*pad, m.scale, lines))*xs
	}
	if len(e.sheet.Tags) > 0 {
		h += gap + tagsHeight(m.scale)
	}
	return h
}

func entryBody(e entry) string {
	if e.line != "" {
		return e.line
	}
	return e.sheet.Excerpt
}

// bodyLines is how many lines, up to most, body takes width wide.
func bodyLines(c *ui.Context, body string, width, scale float32, most int) int {
	span := ui.Span{Text: body, Font: theme.UIFontFamily, Size: theme.TextXSmall.Size(scale)}
	_, one := c.MeasureText(0, ui.Span{Text: "Ag", Font: span.Font, Size: span.Size})
	_, h := c.MeasureText(width, span)
	return max(1, min(most, int(h/one+0.5)))
}

// view is what a block holds: a group's head and rows, taking sheets and
// groups dropped on it.
func (b block) view(s *State, c *ui.Context, semantic theme.Theme, box ui.Element, fr stage.Frame, sh shape, current bool, m blockMetrics, animate bool) {
	if g := b.group; current && g != nil && !b.trashed && !g.Smart() && !b.create {
		if dropped, ok := s.dropOn(c, semantic, box, filing(*g), animate); ok {
			s.fileInto(*g, dropped)
		}
	}
	box.Gap(m.headGap).Children(func() {
		if !b.single && !b.create {
			s.blockHead(c, semantic, b, fr, m, animate)
		}
		cardRows(c, sh, m, func(r row) {
			switch {
			case r.group != nil:
				s.groupRow(c, semantic, *r.group, fr, m, animate)
			case r.entry != nil:
				s.entryRow(c, semantic, *r.entry, b.trashed, r.h, b.single, m, animate)
			case r.create:
				g := *b.group
				components.CreateArea{Key: "create", Label: "New sheet", Animate: animate, Action: func() {
					s.Library.NewSheet(g.ID, Last)
					s.ShowDesk()
				}}.View(c, semantic).Height(r.h)
			default:
				s.moreRow(c, semantic, b, sh.more, fr, m, animate)
			}
		})
	})
}

func (b block) name() string {
	if b.group != nil {
		return b.group.Name
	}
	return "Trash"
}

// blockHead is a block's head: its group's icon, the groups it is in from
// the one the query is in, its name and count, pressed to go into it.
func (s *State) blockHead(c *ui.Context, semantic theme.Theme, b block, fr stage.Frame, m blockMetrics, animate bool) {
	ui.Row(c).Key("head").Height(m.head).WidthPercent(100).AlignItems(ui.Center).Gap(4 * m.k).Shrink(0).Children(func() {
		g := b.group
		icon, ink := components.IconTrash, semantic.Icon.Muted
		if g != nil {
			icon = groupIcon(*g)
			if tint := groupTint(semantic, *g); tint != nil {
				ink = *tint
			}
		}
		head := ui.ButtonBase(c.Key("name")).Grow(1).MinWidth(0).FillHeight().PaddingX(8 * m.k).Gap(8 * m.k).
			AlignItems(ui.Center).Radius(8 * m.k).FocusRing(false).KeepFocus().Label(b.name())
		hover := components.HoverAmount(head, head.Hovered(), animate)
		head.Background(components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover))
		if g != nil {
			head.ContextMenu(s.groupMenu(*g, b.trashed))
			if !b.trashed && !g.Inbox {
				head.Drag(groupDrag{id: g.ID})
			}
			if !b.trashed && s.field.renaming != g.ID && head.Clicked() {
				s.into(*g, fr.Rect)
			}
		}
		head.Children(func() {
			ui.Icon(c, icon.SVG()).FontSize(15 * m.k).TextColor(ink).Shrink(0)
			if g != nil && s.field.renaming == g.ID {
				s.renameInput(c, semantic, *g)
				return
			}
			components.BaselineRow(c).Key("words").Grow(1).MinWidth(0).Gap(8 * m.k).Children(func() {
				ui.Row(c).Key("title").Grow(1).MinWidth(0).Gap(4 * m.k).AlignItems(ui.Baseline).Children(func() {
					if len(b.path) > 0 {
						components.Label(c, strings.Join(b.path, " › ")+" ›", theme.TextDefault, semantic.Text.Muted).MinWidth(0).SingleLine()
					}
					components.Label(c, b.name(), theme.TextDefault, semantic.Text.Heading).Shrink(0).MaxWidthPercent(100).SingleLine()
				})
				if g != nil {
					components.Label(c, thousands(g.Count), theme.TextXSmall, semantic.Text.Muted).
						SingleLine().Shrink(0).FontFeatures("tnum")
				}
			})
		})
		if g != nil && b.trashed && s.armed(c, deleteKey("group:"+g.ID)) {
			s.confirmButton(c, semantic, deleteKey("group:"+g.ID), "", "Delete", animate, func() { s.Library.DeleteGroup(g.ID) })
		}
	})
}

// rowFill is a row's ground: the selection's where the keys are on it,
// the picked fill where it is picked, a step of the ink under the
// pointer; and pressed, it gives a little. A sheet open on the desk is
// never a fill, which would read as a hover left behind, but its strip
// (openMark).
func (s *State) rowFill(c *ui.Context, semantic theme.Theme, el ui.Element, id string, animate bool) {
	hover := components.HoverAmount(el, el.Hovered(), animate)
	selection := components.NavigationSelection(c, semantic)
	fill := components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover)
	switch {
	case s.isPicked(id):
		fill = components.Blend(selection.Picked, selection.PickedHover, hover)
	case s.field.chosen == id:
		fill = selection.Fill
	}
	el.Background(fill)
	press := float32(0)
	if el.Pressed() {
		press = 1
	}
	if animate {
		press = components.Follow(el, "press", press, components.MotionQuick)
	}
	if press > 0.001 {
		el.Scale(1-0.012*press, 1-0.012*press)
	}
	if el.FocusVisible() {
		el.Border(1, semantic.Border.Focused)
	}
}

// groupRow is a group inside a block: pressed, the query goes into it.
func (s *State) groupRow(c *ui.Context, semantic theme.Theme, g GroupItem, fr stage.Frame, m blockMetrics, animate bool) {
	id := "group:" + g.ID
	btn := ui.ButtonBase(c.Key("row-" + id)).WidthPercent(100).Height(m.groupRow).PaddingX(rowPad(m.scale)).
		Gap(8 * m.k).AlignItems(ui.Center).Radius(theme.DefaultControls.Scaled(m.scale).Radius).
		FocusRing(false).KeepFocus().Label(g.Name)
	s.rowFill(c, semantic, btn, id, animate)
	// What is dropped on the row is filed in its group, not in the block's.
	if dropped, ok := s.dropOn(c, semantic, btn, filing(g), animate); ok {
		s.fileInto(g, dropped)
	}
	btn.ContextMenu(s.groupMenu(g, false))
	if !g.Inbox {
		btn.Drag(groupDrag{id: g.ID})
	}
	if btn.Clicked() {
		s.into(g, fr.Rect)
	}
	ink := semantic.Icon.Muted
	if tint := groupTint(semantic, g); tint != nil {
		ink = *tint
	}
	btn.Children(func() {
		ui.Icon(c, groupIcon(g).SVG()).FontSize(14 * m.k).TextColor(ink).Shrink(0)
		components.BaselineRow(c).Key("words").Grow(1).MinWidth(0).Gap(8 * m.k).Children(func() {
			components.Label(c, g.Name, theme.TextSmall, semantic.Text.Default).Grow(1).MinWidth(0).SingleLine()
			components.Label(c, thousands(g.Count), theme.TextXSmall, semantic.Text.Muted).SingleLine().Shrink(0).FontFeatures("tnum")
		})
	})
}

// filing is what g takes to be filed in it: sheets, the tray, and groups
// but in the Inbox.
func filing(g GroupItem) takes {
	return takes{sheets: true, groups: !g.Inbox}
}

// fileInto files what was dropped on g in it: the sheets it carries, out
// of the tray if they came from it, or a group.
func (s *State) fileInto(g GroupItem, dropped shellDrag) {
	switch d := dropped.(type) {
	case sheetDrag, trayDrag:
		s.moveSheets(s.carried(d), g.ID)
		s.delivered(d)
	case groupDrag:
		if d.id != g.ID {
			s.moveGroup(d.id, g.ID)
		}
	}
}

// entryRow is a sheet in a block, in full, as the sheet list showed it:
// pressed it opens on the desk, with Option alongside the sheets open;
// Cmd picks it with others or lets it go, Shift picks the run to it, and
// a menu on a picked sheet is for all the picked.
func (s *State) entryRow(c *ui.Context, semantic theme.Theme, e entry, trashed bool, h float32, single bool, m blockMetrics, animate bool) {
	sh := e.sheet
	id := "sheet:" + sh.ID
	pad, gap := rowPad(m.scale), rowGap(m.scale)
	confirming := trashed && (s.armed(c, deleteKey(id)) || s.armed(c, deletePickedKey(id)))
	front := sh.ID == s.ActivePage
	btn := ui.ButtonBase(c.Key("row-" + id)).WidthPercent(100).Height(h).Shrink(0).Padding(pad).
		AlignItems(ui.Start).Radius(theme.DefaultControls.Scaled(m.scale).Radius).
		FocusRing(false).KeepFocus().Label(sheetTitle(sh)).Selected(front)
	s.rowFill(c, semantic, btn, id, animate)
	if s.isPicked(id) && len(s.field.picked) > 1 {
		btn.ContextMenu(s.pickedMenu(id, trashed))
	} else {
		btn.ContextMenu(s.sheetMenu(sh, trashed))
	}
	if !trashed {
		btn.Drag(sheetDrag{id: sh.ID})
	}
	if !confirming && btn.Clicked() {
		mods := btn.ClickModifiers()
		switch {
		case mods&ui.Cmd != 0:
			s.pickToggle(id)
		case mods&ui.Shift != 0:
			s.pickTo(id)
		default:
			s.field.picked, s.field.anchor = nil, id
			s.activate(id, mods&ui.Alt != 0)
		}
	} else if !confirming && btn.Shortcut(0, ui.KeyEnter) {
		s.activate(id, false)
	}
	lines := 2
	if single {
		lines = 4
	}
	title := semantic.Text.Default
	if front || s.isPicked(id) {
		title = semantic.Text.Heading
	}
	btn.Children(func() {
		if at := s.deskPlace(sh.ID); at > 0 {
			openMark(c, semantic, at, h, pad, m)
		}
		ui.Column(c).Key("entry").Grow(1).MinWidth(0).Gap(gap).Children(func() {
			if (trashed && s.armed(c, deletePickedKey(id))) || confirming {
				ui.Row(c).Key("title").WidthPercent(100).AlignItems(ui.Center).Gap(pad).Children(func() {
					components.CenteredLabel(c, sheetTitle(sh), theme.TextSmall, title).Grow(1).MinWidth(0)
					if trashed && s.armed(c, deletePickedKey(id)) {
						label := fmt.Sprintf("Delete %d", len(s.field.picked))
						s.confirmButton(c, semantic, deletePickedKey(id), "", label, animate, s.deletePicked)
					} else {
						s.confirmButton(c, semantic, deleteKey(id), "", "Delete", animate, func() { s.Library.DeleteSheet(sh.ID) })
					}
				})
			} else {
				components.BaselineRow(c).Key("title").WidthPercent(100).Gap(pad).Children(func() {
					components.Label(c, sheetTitle(sh), theme.TextSmall, title).Grow(1).MinWidth(0).SingleLine()
					if !sh.Updated.IsZero() {
						components.Label(c, when(sh.Updated, c.Now()), theme.TextXSmall, semantic.Text.Muted).
							SingleLine().Shrink(0).FontFeatures("tnum")
					}
				})
			}
			if body := entryBody(e); body != "" {
				components.Label(c, body, theme.TextXSmall, semantic.Text.Muted).WidthPercent(100).MaxLines(lines)
			}
			s.rowTags(c, semantic, sh.Tags, m)
			foot := words(sh.Words)
			if e.in != "" {
				foot = e.in + " · " + foot
			}
			components.Label(c, foot, theme.TextXSmall, semantic.Text.Muted).SingleLine().FontFeatures("tnum").Opacity(0.7)
		})
	})
}

// deskPlace is where the sheet id stands on the desk, counted from one as
// the desk's columns go, 0 where it is not open.
func (s *State) deskPlace(id string) int {
	for i, page := range s.Open {
		if page.ID == id {
			return i + 1
		}
	}
	return 0
}

// openMark marks a sheet's row h tall, padded pad, as open on the desk
// (field-and-desk.md §4.7.4): a strip in the accent along its leading
// edge, in the card's margin, as the sheet list's was, and at its foot's
// end a badge of the desk and the number for its column, in Halo.
func openMark(c *ui.Context, semantic theme.Theme, at int, h, pad float32, m blockMetrics) {
	thin, inset := 3*m.k, 6*m.k
	ui.Box(c.Key("open-strip")).Absolute().Left(-m.pad/2-thin/2).Top(inset).Size(thin, h-2*inset).Radius(thin / 2).
		Background(semantic.Accent.Foreground).IgnoreInput(true)
	line := theme.TextXSmall.LineHeight(m.scale)
	ink := semantic.Accent.OnAccent
	ui.Row(c.Key("open-badge")).Absolute().Right(pad).Bottom(pad).Height(line).PaddingX(5 * m.k).Gap(3 * m.k).
		Radius(line / 2).AlignItems(ui.Center).Background(semantic.Families[theme.Halo].Foreground).IgnoreInput(true).
		Label("Open on the desk, column " + fmt.Sprint(at)).Children(func() {
		ui.Icon(c, components.IconDesk.SVG()).FontSize(line * 0.7).TextColor(ink)
		components.CenteredLabel(c, fmt.Sprint(at), theme.TextXSmall, ink).SingleLine().FontFeatures("tnum")
	})
}

// rowTags are a sheet's tags on one line, as many as fit, the rest
// counted.
func (s *State) rowTags(c *ui.Context, semantic theme.Theme, tags []string, m blockMetrics) {
	if len(tags) == 0 {
		return
	}
	pad, gap := theme.Space06.DIPs(m.scale), theme.Space06.DIPs(m.scale)
	size := theme.TextXSmall.Size(m.scale)
	room := m.col - 2*rowPad(m.scale)
	chip := func(text string) float32 {
		w, _ := c.MeasureText(0, ui.Span{Text: text, Font: theme.UIFontFamily, Size: size})
		return w + 2*pad
	}
	shown, used := 0, float32(0)
	for i, tag := range tags {
		w := chip("#" + tag)
		rest := float32(0)
		if i < len(tags)-1 {
			rest = gap + chip(fmt.Sprintf("+%d", len(tags)-i-1))
		}
		if used+w+rest > room && shown > 0 {
			break
		}
		used += w + gap
		shown++
	}
	ui.Row(c).Key("tags").WidthPercent(100).Height(tagsHeight(m.scale)).Gap(gap).AlignItems(ui.Center).
		PaddingY(theme.Space04.DIPs(m.scale)).Clip().Children(func() {
		for _, tag := range tags[:shown] {
			components.TagChip(c, "#"+tag, theme.TextXSmall, pad, room, semantic.Tag.Background, semantic.Tag.Foreground).Shrink(0)
		}
		if hidden := len(tags) - shown; hidden > 0 {
			components.TagChip(c, fmt.Sprintf("+%d", hidden), theme.TextXSmall, pad, room, semantic.Tag.Background, semantic.Text.Muted).
				Shrink(0).Tooltip(strings.Join(tags[shown:], ", "))
		}
	})
}

// moreRow says how many sheets a block leaves out; pressed, the query goes
// into its group, where it shows more.
func (s *State) moreRow(c *ui.Context, semantic theme.Theme, b block, more int, fr stage.Frame, m blockMetrics, animate bool) {
	label := fmt.Sprintf("%s more", thousands(more))
	btn := ui.ButtonBase(c.Key("more")).WidthPercent(100).Height(m.groupRow).PaddingX(rowPad(m.scale)).
		AlignItems(ui.Center).Radius(theme.DefaultControls.Scaled(m.scale).Radius).FocusRing(false).KeepFocus().Label(label)
	s.rowFill(c, semantic, btn, "", animate)
	if b.group != nil && btn.Clicked() {
		s.into(*b.group, fr.Rect)
	}
	btn.Children(func() {
		components.CenteredLabel(c, label, theme.TextXSmall, semantic.Text.Muted)
	})
}

func sheetTitle(sh SheetItem) string {
	if sh.Title == "" {
		return "Untitled"
	}
	return sh.Title
}

func deleteKey(id string) string { return "delete-" + id }

func (s *State) renameInput(c *ui.Context, semantic theme.Theme, g GroupItem) {
	input := ui.TextInputBase(c.Key("rename-"+g.ID), &s.field.renameName).
		Grow(1).MinWidth(0).Font(theme.UIFontFamily).
		FontSize(theme.TextDefault.Size(s.Settings.UIFontSize)).
		TextColor(semantic.Text.Heading).Label("Group name")
	if s.field.renameFocus {
		s.field.renameFocus = false
		input.Focus()
	}
	switch inlineName(input) {
	case nameCancel:
		s.field.renaming = ""
	case nameCommit:
		s.field.renaming = ""
		name := strings.TrimSpace(s.field.renameName)
		if name == "" || name == g.Name {
			return
		}
		// The query going on in the group follows its new name.
		for i, text := range s.field.conditions {
			if c, ok := condition(text); ok && c.Key.Name == "in" && strings.EqualFold(c.Value, g.Name) {
				c.Value = name
				s.field.conditions[i] = c.String()
			}
		}
		s.field.conditions = ordered(s.field.conditions)
		s.Library.RenameGroup(g.ID, name)
	}
}

func (s *State) groupMenu(g GroupItem, trashed bool) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if trashed {
			if m.Item("Put Back").Chosen() {
				s.restoreGroup(g)
			}
			m.Separator()
			if m.Item(deleteGroupLabel(g)).Chosen() {
				s.arm(deleteKey("group:" + g.ID))
			}
			return
		}
		if !g.Smart() {
			if m.Item("New Sheet").Chosen() {
				s.Library.NewSheet(g.ID, Last)
				s.ShowDesk()
			}
		}
		if g.Inbox {
			return
		}
		if !g.Smart() && m.Item("New Group Inside").Chosen() {
			s.newGroup(g.ID)
		}
		m.Separator()
		if m.Item("Rename…").Chosen() {
			s.startRename(g.ID)
		}
		m.Submenu("Color", func(m *ui.Menu) {
			for _, opt := range []struct {
				label string
				color int
			}{
				{"Default", TintMoonveil}, {"Violet", TintHalo}, {"Blue", TintHalo + 1}, {"Teal", TintHalo + 2},
				{"Amber", TintHalo + 3}, {"Rose", TintHalo + 4}, {"Indigo", TintHalo + 5},
			} {
				if m.Item(opt.label).Checked(g.Color == opt.color).Chosen() {
					s.Library.StyleGroup(g.ID, g.Icon, opt.color)
				}
			}
		})
		s.moveMenu(m, func(target string) { s.moveGroup(g.ID, target) }, g.ID)
		m.Separator()
		if m.Item(trashGroupLabel(g)).Chosen() {
			s.trashGroup(g)
		}
	}
}

func (s *State) sheetMenu(sh SheetItem, trashed bool) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if trashed {
			if m.Item("Put Back").Chosen() {
				s.restoreSheets([]string{sh.ID})
			}
			m.Separator()
			if m.Item("Delete Immediately…").Chosen() {
				s.arm(deleteKey("sheet:" + sh.ID))
			}
			return
		}
		if m.Item("Open").Chosen() {
			s.Library.OpenSheet(sh.ID, false)
			s.ShowDesk()
		}
		if m.Item("Open Alongside").Chosen() {
			s.Library.OpenSheet(sh.ID, true)
			s.ShowDesk()
		}
		m.Separator()
		if m.Item("Duplicate").Chosen() {
			s.Library.DuplicateSheet(sh.ID)
		}
		s.moveMenu(m, func(group string) { s.moveSheets([]string{sh.ID}, group) }, "")
		m.Separator()
		if m.Item("Move to Trash").Chosen() {
			s.trashSheets([]string{sh.ID})
		}
	}
}
