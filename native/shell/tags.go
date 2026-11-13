package shell

import (
	"fmt"
	"strings"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// tagLineCount is how many lines a sheet's tags take at most.
const tagLineCount = 2

// tagWidthKey is a tag chip's text at a size, as its width is kept.
type tagWidthKey struct {
	text string
	size float32
}

// sheetTags shows a sheet's tags under its excerpt, most written first,
// as the page shows a tag, Halo's normal tone on its surface: chips on at
// most two lines, and the count of those that find no room in
// a chip of its own, the hidden ones named in its tooltip.
func (s *State) sheetTags(c *ui.Context, semantic theme.Theme, tags []string) {
	if len(tags) == 0 {
		return
	}
	scale := s.Settings.UIFontSize
	size := theme.TextXSmall.Size(scale)
	pad := theme.Space06.DIPs(scale)
	gap := theme.Space06.DIPs(scale)
	// The card's text width: the list's and the card's padding either
	// side, and the card's border.
	width := s.paneLayout().SheetList*scale/theme.DefaultUIFontSize - 4*theme.Space08.DIPs(scale) - 2
	chip := func(text string) float32 { return s.tagWidth(text, size) + 2*pad }
	widths := make([]float32, len(tags))
	for i, tag := range tags {
		widths[i] = chip(tagText(tag))
	}
	lines, hidden := fitTags(widths, func(n int) float32 { return chip(moreText(n)) }, width, gap, tagLineCount)
	halo := semantic.Families[theme.Halo]
	ui.Column(c).Key("tags").WidthPercent(100).Gap(theme.Space04.DIPs(scale)).
		PaddingY(theme.Space04.DIPs(scale)).Children(func() {
		for li, line := range lines {
			ui.Row(c).WidthPercent(100).Gap(gap).AlignItems(ui.Center).Children(func() {
				for _, i := range line {
					tagChip(c, tagText(tags[i]), pad, width, halo.Surface, halo.Foreground)
				}
				if li == len(lines)-1 && hidden > 0 {
					tagChip(c, moreText(hidden), pad, width, halo.Surface, semantic.Text.Muted).
						Tooltip(strings.Join(tags[len(tags)-hidden:], ", "))
				}
			})
		}
	})
}

func tagText(name string) string { return "#" + name }
func moreText(n int) string      { return fmt.Sprintf("+%d", n) }

// tagChip is one chip: its text on its fill, cut short where a line
// cannot hold it.
func tagChip(c *ui.Context, text string, pad, width float32, fill, ink ui.Color) ui.Element {
	scale := components.UIFontSize(c)
	height := theme.TextXSmall.LineHeight(scale) + theme.Space04.DIPs(scale)
	// Rounded as the page rounds a tag's background.
	chip := ui.Row(c).Height(height).Radius(3 * scale / theme.DefaultUIFontSize).PaddingX(pad).
		MaxWidth(width).MinWidth(0).AlignItems(ui.Center).Background(fill)
	chip.Children(func() {
		components.Label(c, text, theme.TextXSmall, ink).SingleLine().MinWidth(0)
	})
	return chip
}

// tagWidth is a chip text's width at a size, measured once.
func (s *State) tagWidth(text string, size float32) float32 {
	key := tagWidthKey{text, size}
	if w, ok := s.tagWidths[key]; ok {
		return w
	}
	if s.tagWidths == nil || len(s.tagWidths) > 4096 {
		s.tagWidths = map[tagWidthKey]float32{}
	}
	w, _ := ui.ShapeText(text, ui.Font{Family: theme.UIFontFamily, Size: size}, 0).Size()
	s.tagWidths[key] = w
	return w
}

// fitTags lays chips of widths, in order, on at most lines lines of width,
// gap apart: the lines and the chips each holds, and how many find no
// room. Those are counted in a chip of their own, as wide as more says,
// at the last line's end, which gives up chips until it fits. A chip wider
// than a line takes a line, cut short.
func fitTags(widths []float32, more func(n int) float32, width, gap float32, lines int) ([][]int, int) {
	var out [][]int
	used := float32(0)
	placed := 0
	for i, w := range widths {
		w = min(w, width)
		if len(out) > 0 && used+gap+w <= width {
			out[len(out)-1] = append(out[len(out)-1], i)
			used += gap + w
		} else if len(out) < lines {
			out = append(out, []int{i})
			used = w
		} else {
			break
		}
		placed++
	}
	hidden := len(widths) - placed
	if hidden == 0 {
		return out, 0
	}
	last := out[len(out)-1]
	for {
		used := float32(0)
		for j, i := range last {
			if j > 0 {
				used += gap
			}
			used += min(widths[i], width)
		}
		if len(last) == 0 || used+gap+more(hidden) <= width {
			break
		}
		last = last[:len(last)-1]
		hidden++
	}
	out[len(out)-1] = last
	return out, hidden
}
