package shell

import (
	"fmt"
	"math"
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The map is the canvas's half that holds the notebook: every group a
// region of its sheets' cards, the regions packed into columns, the
// fullest first to the shortest column, so the map stays compact whatever
// its groups hold. Every size here is in the canvas's zoomed type
// (components.UIFontSize), so the map grows and shrinks with it.

// The map's measures at the reference type size: a card, the cards a
// region's row holds, and the room between and around them.
const (
	cardWidth   = 184
	cardHeight  = 88
	regionCards = 2
	cardGap     = 8
	regionPad   = 12
	regionGap   = 24
	// regionHead is about how tall a region's head stands over its cards,
	// for packing the columns.
	regionHead = 64
)

// region is a group as the map shows it: its own sheets, without its
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
// group after its parent, and the Inbox first, as the library leads with
// it.
func (s *State) regions(now time.Time) []region {
	var out []region
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
			if group.Inbox {
				out = append([]region{r}, out...)
			} else {
				out = append(out, r)
			}
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

// pack deals the regions out to at most n columns, about as many as make
// the map as wide as it is tall, each to the column shortest so far, in
// order.
func pack(regions []region, n int) [][]region {
	n = max(1, min(n, int(math.Ceil(math.Sqrt(float64(len(regions)))))))
	columns := make([][]region, n)
	heights := make([]float64, n)
	for _, r := range regions {
		shortest := 0
		for i := range heights {
			if heights[i] < heights[shortest] {
				shortest = i
			}
		}
		rows := math.Ceil(float64(len(r.sheets)) / regionCards)
		columns[shortest] = append(columns[shortest], r)
		heights[shortest] += regionHead + rows*(cardHeight+cardGap) + regionGap
	}
	return columns
}

// regionWidth is a region's width at the reference type size.
const regionWidth = regionCards*cardWidth + (regionCards-1)*cardGap + 2*regionPad

// mapColumns is how many columns of regions the map has: as many as the
// strip's window holds at its zoom, up to four, so the map shows whole
// across, scrolling down.
func (s *State) mapColumns(k, pad float32) int {
	room := s.canvas.viewport.W - 2*pad
	return max(1, min(4, int((room+regionGap*k)/((regionWidth+regionGap)*k))))
}

// canvasMap draws the map, the strip's first column: the regions in their
// columns, scrolling down. lit, while a search is lighting sheets, is what
// it found.
func (s *State) canvasMap(c *ui.Context, semantic theme.Theme, lit map[string]bool, pad, origin float32, animate bool) {
	k := components.UIFontSize(c) / theme.DefaultUIFontSize
	now := c.Now()
	columns := pack(s.regions(now), s.mapColumns(k, pad))
	width := float32(len(columns))*(regionWidth+regionGap)*k - regionGap*k
	s.column(c, "canvas-map", width, origin, ui.Start, animate).Children(func() {
		ui.Scroll(c).Key("map-scroll").WidthPercent(100).Grow(1).MinHeight(0).Children(func() {
			ui.Row(c).Key("map-columns").AlignItems(ui.Start).Gap(regionGap * k).Children(func() {
				for i, column := range columns {
					ui.Column(c).Key(fmt.Sprintf("map-column-%d", i)).Width(regionWidth * k).Shrink(0).
						Gap(regionGap * k).Children(func() {
						for _, r := range column {
							s.canvasRegion(c, semantic, r, lit, now, animate)
						}
					})
				}
			})
		})
	})
}

// canvasRegion is a group's region: its mark and name, what it holds, its
// sheets' cards, and its strip of activity at its foot. Sheets dragged
// onto it are filed in it.
func (s *State) canvasRegion(
	c *ui.Context,
	semantic theme.Theme,
	r region,
	lit map[string]bool,
	now time.Time,
	animate bool,
) {
	size := components.UIFontSize(c)
	k := size / theme.DefaultUIFontSize
	tint := groupTint(semantic, r.group)
	ink := semantic.Text.Muted
	if tint != nil {
		ink = *tint
	}
	box := ui.Column(c).Key("region-"+r.group.ID).WidthPercent(100).
		Padding(regionPad*k).Gap(cardGap*k).Radius(10*k).
		Background(semantic.Surface.Panel).Border(1, semantic.Border.Variant).
		Transition(listTransition(animate, true))
	box.Opacity(litAmount(box, lit == nil || anyLit(r.sheets, lit), 0.45, animate))
	box.Children(func() {
		ui.Row(c).WidthPercent(100).AlignItems(ui.Center).Gap(6 * k).Children(func() {
			glyph := ui.Icon(c, groupIcon(r.group).SVG()).FontSize(16 * k).Shrink(0)
			glyph.TextColor(ink)
			// A nested group says where it is, before its name, giving way
			// to it.
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
			ui.Row(c).Key("cards").WidthPercent(100).Wrap().Gap(cardGap * k).Children(func() {
				for _, sheet := range r.sheets {
					s.canvasCard(c, semantic, sheet, lit, now, animate)
				}
			})
		}
		activityStrip(c, semantic, r.days, ink)
	})
	box.ContextMenu(s.groupMenu(r.group))
	// A region takes sheets: what moves between groups on the canvas. A
	// smart folder takes none, as what it holds, its query finds.
	if dropped, ok := s.dropOn(c, semantic, box, takes{sheets: !r.group.Smart()}, animate); ok {
		s.Library.MoveSheet(dropped.(sheetDrag).id, r.group.ID, Last)
	}
}

// anyLit reports whether a search lit any of sheets.
func anyLit(sheets []SheetItem, lit map[string]bool) bool {
	for _, sheet := range sheets {
		if lit[sheet.ID] {
			return true
		}
	}
	return false
}

// activityStrip is a region's baseline: its last days across its width, a
// segment each, today last, in the region's ink where a sheet was last
// changed that day.
func activityStrip(c *ui.Context, semantic theme.Theme, days [activityDays]int, ink ui.Color) {
	k := components.UIFontSize(c) / theme.DefaultUIFontSize
	ui.Row(c).Key("activity").WidthPercent(100).Shrink(0).Gap(2*k).Margin(4*k, 0, 0, 0).
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

// canvasCard is a sheet on the map: its title, the start of its text, its
// words and last change. Open in a column, it carries the accent's mark;
// the one in front, the accent's edge. Clicked, it opens in a column;
// dragged to another region, it is filed there.
func (s *State) canvasCard(
	c *ui.Context,
	semantic theme.Theme,
	sheet SheetItem,
	lit map[string]bool,
	now time.Time,
	animate bool,
) {
	k := components.UIFontSize(c) / theme.DefaultUIFontSize
	open := s.isOpen(sheet.ID)
	card := ui.Column(c).Key("card-" + sheet.ID).Role(ui.RoleButton).Label(sheet.Title).Focusable().
		Width(cardWidth * k).Height(cardHeight * k).Shrink(0).Clip().
		Padding(8 * k).Gap(2 * k).Radius(6 * k).Transition(listTransition(animate, true))
	hover := components.HoverAmount(card, card.Hovered(), animate)
	card.Background(components.Blend(semantic.Surface.Page, semantic.Element.Hover, hover))
	switch {
	case sheet.ID == s.ActivePage:
		card.Border(1, semantic.Accent.Foreground)
	case card.FocusVisible():
		card.Border(1, semantic.Border.Focused)
	default:
		card.Border(1, semantic.Border.Variant)
	}
	card.Children(func() {
		ui.Row(c).WidthPercent(100).AlignItems(ui.Center).Gap(6 * k).Children(func() {
			if open {
				ui.Box(c).Size(6*k, 6*k).Radius(3 * k).Shrink(0).Background(semantic.Accent.Foreground)
			}
			components.Label(c, sheet.Title, theme.TextSmall, semantic.Text.Heading).
				SingleLine().MinWidth(0).Grow(1)
		})
		components.Label(c, sheet.Excerpt, theme.TextXSmall, semantic.Text.Muted).
			WidthPercent(100).MaxLines(2).Grow(1)
		components.Label(c, words(sheet.Words)+" · "+when(sheet.Updated, now), theme.TextXSmall,
			semantic.Text.Muted).SingleLine().FontFeatures("tnum").Opacity(0.8)
	})
	card.Opacity(litAmount(card, lit == nil || lit[sheet.ID], 0.25, animate))
	card.Drag(sheetDrag{id: sheet.ID})
	card.ContextMenu(s.sheetMenu(sheet, false))
	if card.Clicked() || card.Shortcut(0, ui.KeyEnter) {
		s.openOnCanvas(sheet.ID)
	}
}
