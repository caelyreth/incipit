package shell

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The canvas is the notebook whole on one surface, where the columns show
// one place at a time: every group a region, every sheet a card in it,
// arranged by the notebook itself rather than placed by hand. It is for
// seeing the notebook, how big each part is and what moved lately, and
// for filing by dragging; opening a card goes back to the columns, at the
// sheet.

// zoom is how near the canvas is: what each card shows. Only the detail
// changes as it goes nearer; the regions keep their order.
type zoom uint8

const (
	// zoomFar shows each sheet as a mark sized by its words.
	zoomFar zoom = iota
	// zoomMiddle shows each sheet's title, words and last change.
	zoomMiddle
	// zoomNear shows each sheet as a card: its excerpt and tags too.
	zoomNear
)

// pinchStep is how far a pinch goes before it changes the zoom by one.
const pinchStep = 1.3

type canvasState struct {
	open bool
	zoom zoom
	// pinch is how far the pinches have gone since the zoom last changed,
	// 0 for not at all.
	pinch float32
	// query lights the sheets it finds, and dims the rest, in place;
	// input is its field's, and focus asks the field for the focus.
	query  string
	input  queryInput
	focus  bool
	typing bool
	scroll ui.ScrollState
}

// CanvasOpen reports whether the canvas shows in place of the columns.
func (s *State) CanvasOpen() bool { return s != nil && s.canvas.open && s.Library != nil }

// ToggleCanvas goes to the canvas, or back to the columns as they were.
func (s *State) ToggleCanvas() {
	if s == nil {
		return
	}
	s.canvas.open = !s.canvas.open
	s.canvas.typing = false
}

// closeCanvas goes back to the columns, as choosing panes does.
func (s *State) closeCanvas() {
	s.canvas.open, s.canvas.typing = false, false
}

// region is a group as the canvas shows it: its own sheets, without its
// groups', which are regions of their own, and what they come to.
type region struct {
	group GroupItem
	// path names the groups it is in, outermost first.
	path   string
	sheets []SheetItem
	words  int
	// week is how many of its sheets changed in the last seven days, and
	// days how many were last changed on each of the last activityDays,
	// today last.
	week int
	days [activityDays]int
}

// activityDays is how many days a region's strip of activity covers.
const activityDays = 14

// regions are the notebook's groups in the library's order, each nested
// group after its parent and the Inbox first, as the library leads with
// it; the smart folders stand where they do there.
func (s *State) regions(now time.Time) []region {
	var out []region
	defer func() {
		for i, r := range out {
			if r.group.Inbox {
				copy(out[1:i+1], out[:i])
				out[0] = r
				break
			}
		}
	}()
	today := day(now)
	var walk func(parent, path string)
	walk = func(parent, path string) {
		for _, group := range s.Library.Groups(parent) {
			r := region{group: group, path: path}
			r.sheets = s.Library.Sheets(Place{Kind: PlaceGroup, Group: group.ID})
			for _, sheet := range r.sheets {
				r.words += sheet.Words
				if sheet.Updated.IsZero() {
					continue
				}
				ago := int(today.Sub(day(sheet.Updated)).Hours() / 24)
				if ago < 7 {
					r.week++
				}
				if ago >= 0 && ago < activityDays {
					r.days[activityDays-1-ago]++
				}
			}
			out = append(out, r)
			if group.HasGroups {
				inner := group.Name
				if path != "" {
					inner = path + " / " + group.Name
				}
				walk(group.ID, inner)
			}
		}
	}
	walk("", "")
	return out
}

// day is the local midnight that starts t's day.
func day(t time.Time) time.Time {
	t = t.Local()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// canvasView draws the canvas: a head with the query that lights the
// sheets and the zoom, over the regions, which wrap to the window's width
// and scroll down.
func (s *State) canvasView(c *ui.Context, semantic theme.Theme, layout theme.Layout, animate bool) {
	scale := s.Settings.UIFontSize
	now := c.Now()
	regions := s.regions(now)
	var lit map[string]bool
	found := 0
	if text := strings.TrimSpace(s.canvas.query); text != "" {
		result := s.Library.Search(s.canvas.query)
		lit = make(map[string]bool, len(result.Sheets))
		for _, sheet := range result.Sheets {
			lit[sheet.ID] = true
		}
		found = result.Total
	}
	gutter := layout.ContentGutter * scale / theme.DefaultUIFontSize
	canvas := ui.Column(c).Key("canvas").Grow(1).MinWidth(0).Background(semantic.Surface.Page).
		Transition(fadeTransition(animate))
	canvas.Children(func() {
		s.canvasHead(c, semantic, gutter, lit != nil, found, animate)
		scroll := ui.Scroll(c).Key("canvas-scroll").Grow(1).MinHeight(0).TrackScroll(&s.canvas.scroll)
		scroll.Children(func() {
			field := ui.Row(c).Key("canvas-field").WidthPercent(100).Wrap().AlignItems(ui.Start).
				Gap(theme.Space16.DIPs(scale)).Padding(theme.Space08.DIPs(scale), gutter, gutter, gutter)
			// A pinch goes nearer or farther, a step at a time.
			field.HandleInput(func(ev ui.InputEvent) bool {
				if ev.Kind != ui.InputZoom {
					return false
				}
				if s.canvas.pinch == 0 {
					s.canvas.pinch = 1
				}
				s.canvas.pinch *= ev.Scale
				switch {
				case s.canvas.pinch >= pinchStep && s.canvas.zoom < zoomNear:
					s.canvas.zoom++
					s.canvas.pinch = 1
				case s.canvas.pinch <= 1/pinchStep && s.canvas.zoom > zoomFar:
					s.canvas.zoom--
					s.canvas.pinch = 1
				}
				return true
			})
			field.Children(func() {
				for _, r := range regions {
					s.canvasRegion(c, semantic, r, lit, now, animate)
				}
			})
		})
	})
	// Escape, wherever the focus is, lets go of the search, then of the
	// canvas.
	if c.Shortcut(0, ui.KeyEscape) {
		if s.canvas.query != "" {
			s.canvas.query = ""
		} else {
			s.closeCanvas()
		}
	}
}

// canvasHead is the canvas's line under the title bar: the query, what it
// found, and how near the canvas is.
func (s *State) canvasHead(c *ui.Context, semantic theme.Theme, gutter float32, searching bool, found int, animate bool) {
	scale := s.Settings.UIFontSize
	ui.Row(c).Key("canvas-head").WidthPercent(100).Shrink(0).AlignItems(ui.Center).
		Gap(theme.Space12.DIPs(scale)).PaddingX(gutter).PaddingY(theme.Space12.DIPs(scale)).Children(func() {
		field, input := queryField{
			Key: "canvas-search", Label: "Light sheets", Placeholder: "Light the sheets a search finds",
			Value: &s.canvas.query, In: &s.canvas.input, Scope: query.InNotebook,
			Focus: s.canvas.focus, Animate: animate,
		}.View(s, c, semantic)
		field.Grow(1).MaxWidth(440 * scale / theme.DefaultUIFontSize)
		s.canvas.focus = false
		s.canvas.typing = input.Focused()
		if searching {
			components.Label(c, foundCount(found), theme.TextXSmall, semantic.Text.Muted).
				SingleLine().Shrink(0).FontFeatures("tnum")
		}
		ui.Box(c).Grow(1)
		options := make([]components.SegmentOption, 0, 3)
		for _, z := range [...]struct {
			zoom  zoom
			label string
		}{{zoomFar, "Far"}, {zoomMiddle, "Middle"}, {zoomNear, "Near"}} {
			options = append(options, components.SegmentOption{
				Key: "zoom-" + strings.ToLower(z.label), Label: z.label, Selected: s.canvas.zoom == z.zoom,
				Action: func() { s.canvas.zoom, s.canvas.pinch = z.zoom, 1 },
			})
		}
		components.Segmented{Key: "canvas-zoom", Label: "Zoom", Animate: animate, Options: options}.View(c, semantic)
	})
}

// foundCount says how many sheets a search lit.
func foundCount(n int) string {
	switch n {
	case 0:
		return "Nothing found"
	case 1:
		return "1 sheet found"
	}
	return thousands(n) + " sheets found"
}

// regionWidth is how wide a region of n sheets stands at a zoom: the
// columns of cards it takes to be about as tall as it is wide, at most
// three, so a large group reads large without one region taking a row.
func (s *State) regionWidth(z zoom, n int) float32 {
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	pad := 2 * theme.Space12.DIPs(s.Settings.UIFontSize)
	gap := theme.Space08.DIPs(s.Settings.UIFontSize)
	card, ratio := float32(0), float32(1)
	switch z {
	case zoomFar:
		return 232 * k
	case zoomMiddle:
		card, ratio = 184*k, 3
	case zoomNear:
		card, ratio = 232*k, 1.4
	}
	cols := int(math.Ceil(math.Sqrt(float64(n) / float64(ratio))))
	cols = max(1, min(cols, 3))
	return pad + float32(cols)*card + float32(cols-1)*gap
}

// canvasRegion is a group's region: its mark, name and what it holds over
// its strip of activity, then its sheets. Sheets dragged onto it are filed
// in it.
func (s *State) canvasRegion(
	c *ui.Context,
	semantic theme.Theme,
	r region,
	lit map[string]bool,
	now time.Time,
	animate bool,
) {
	scale := s.Settings.UIFontSize
	z := s.canvas.zoom
	tint := groupTint(semantic, r.group)
	ink := semantic.Text.Muted
	if tint != nil {
		ink = *tint
	}
	box := ui.Column(c).Key("region-"+r.group.ID).Width(s.regionWidth(z, len(r.sheets))).Shrink(0).
		Padding(theme.Space12.DIPs(scale)).Gap(theme.Space08.DIPs(scale)).
		Radius(10*scale/theme.DefaultUIFontSize).Background(semantic.Surface.Panel).
		Border(1, semantic.Border.Variant).Transition(listTransition(animate, true))
	if lit != nil {
		some := false
		for _, sheet := range r.sheets {
			some = some || lit[sheet.ID]
		}
		if !some {
			box.Opacity(0.45)
		}
	}
	box.Children(func() {
		ui.Row(c).WidthPercent(100).AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).Children(func() {
			s.rowIcon(c, semantic, groupIcon(r.group), tint, animate)
			// A nested group says where it is, before its name, giving
			// way to it.
			if r.path != "" {
				components.Label(c, r.path+" /", theme.TextSmall, semantic.Text.Muted).
					SingleLine().MinWidth(0).Shrink(1)
			}
			components.Label(c, r.group.Name, theme.TextSmall, semantic.Text.Heading).
				FontWeight(600).SingleLine().Shrink(0).MaxWidthPercent(100)
		})
		facts := sheetCount(len(r.sheets)) + " · " + words(r.words)
		if r.week > 0 {
			facts += fmt.Sprintf(" · %d this week", r.week)
		}
		components.Label(c, facts, theme.TextXSmall, semantic.Text.Muted).
			WidthPercent(100).SingleLine().FontFeatures("tnum")
		if len(r.sheets) == 0 {
			text := "No sheets"
			if r.group.Smart() {
				text = "Nothing found for “" + r.group.Query + "”"
			}
			components.Label(c, text, theme.TextXSmall, semantic.Text.Muted).Opacity(0.7)
		} else {
			gap := theme.Space08.DIPs(scale)
			if z == zoomFar {
				gap = theme.Space04.DIPs(scale)
			}
			ui.Row(c).Key("cards").WidthPercent(100).Wrap().AlignItems(ui.Center).Gap(gap).Children(func() {
				for _, sheet := range r.sheets {
					s.canvasCard(c, semantic, sheet, ink, lit, now, animate)
				}
			})
		}
		s.activityStrip(c, semantic, r.days, ink)
	})
	box.ContextMenu(s.groupMenu(r.group))
	// A region takes sheets: what moves between groups on the canvas. A
	// smart folder takes none, as what it holds, its query finds.
	if dropped, ok := s.dropOn(c, semantic, box, takes{sheets: !r.group.Smart()}, animate); ok {
		s.Library.MoveSheet(dropped.(sheetDrag).id, r.group.ID, Last)
	}
}

// activityStrip is the region's baseline: its last days across its
// width, a segment each, today last, in the region's ink where a sheet was
// last changed that day.
func (s *State) activityStrip(c *ui.Context, semantic theme.Theme, days [activityDays]int, ink ui.Color) {
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	ui.Row(c).Key("activity").WidthPercent(100).Shrink(0).Gap(2*k).Margin(theme.Space04.DIPs(s.Settings.UIFontSize), 0, 0, 0).
		Label("Changed in the last two weeks").Children(func() {
		for _, n := range days {
			fill := semantic.Border.Variant
			if n > 0 {
				fill = components.Blend(semantic.Border.Variant, ink, min(1, 0.5+0.25*float32(n)))
			}
			ui.Box(c).Grow(1).Basis(0).Height(3 * k).Radius(1.5 * k).Background(fill)
		}
	})
}

// canvasCard is a sheet on the canvas, as near as the canvas is: a mark
// sized by its words, its title line, or its card. A click opens it in
// the columns; it drags to another region to be filed there.
func (s *State) canvasCard(
	c *ui.Context,
	semantic theme.Theme,
	sheet SheetItem,
	ink ui.Color,
	lit map[string]bool,
	now time.Time,
	animate bool,
) {
	scale := s.Settings.UIFontSize
	k := scale / theme.DefaultUIFontSize
	open := sheet.ID == s.ActivePage
	card := ui.Column(c).Key("card-" + sheet.ID).Role(ui.RoleButton).Label(sheet.Title).
		Focusable().Shrink(0).Transition(listTransition(animate, true))
	hover := components.HoverAmount(card, card.Hovered(), animate)
	switch s.canvas.zoom {
	case zoomFar:
		// From a dot for a line to a block for a chapter's worth.
		size := (6 + 12*min(1, float32(math.Log10(float64(sheet.Words)+1))/4)) * k
		fill := components.Blend(semantic.Border.Variant, ink, 0.7+0.3*hover)
		card.Size(size, size).Radius(2 * k).Background(fill).
			Tooltip(sheet.Title + " · " + words(sheet.Words))
		if open {
			card.Border(1, semantic.Accent.Foreground)
		}
	case zoomMiddle, zoomNear:
		width := float32(184)
		if s.canvas.zoom == zoomNear {
			width = 232
		}
		card.Width(width*k).Padding(theme.Space08.DIPs(scale)).Gap(theme.Space02.DIPs(scale)).
			Radius(6*k).Background(components.Blend(semantic.Surface.Page, semantic.Element.Hover, hover)).
			Border(1, semantic.Border.Variant)
		if open {
			card.Border(1, semantic.Accent.Foreground)
		}
		card.Children(func() {
			components.Label(c, sheet.Title, theme.TextSmall, semantic.Text.Heading).
				WidthPercent(100).SingleLine()
			if s.canvas.zoom == zoomNear {
				if sheet.Excerpt != "" {
					components.Label(c, sheet.Excerpt, theme.TextXSmall, semantic.Text.Muted).
						WidthPercent(100).MaxLines(3)
				}
				s.sheetTags(c, semantic, sheet.Tags)
			}
			components.Label(c, words(sheet.Words)+" · "+when(sheet.Updated, now), theme.TextXSmall,
				semantic.Text.Muted).SingleLine().FontFeatures("tnum").Opacity(0.8)
		})
	}
	if lit != nil && !lit[sheet.ID] {
		card.Opacity(0.25)
	}
	if card.FocusVisible() {
		card.Border(1, semantic.Border.Focused)
	}
	card.Drag(sheetDrag{id: sheet.ID})
	card.ContextMenu(s.sheetMenu(sheet, false))
	if card.Clicked() || card.Shortcut(0, ui.KeyEnter) {
		s.openFromCanvas(sheet.ID, card.ClickModifiers()&ui.Alt != 0)
	}
}

// openFromCanvas opens a sheet and goes back to the columns, at its group.
func (s *State) openFromCanvas(id string, newTab bool) {
	s.closeCanvas()
	s.reveal(id)
	s.Library.OpenSheet(id, newTab)
}
