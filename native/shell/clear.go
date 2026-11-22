package shell

import (
	"github.com/caelyreth/neverland/locales"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// statsClear ends the counts beside the pill while a query is written, or
// narrowed by: "· Clear", in Afterglow, which clears it, as the field
// lane's Home does. It stands by what it changes, the count of what the
// query finds, leaving the pill its room for what is written; it comes and
// goes as the word count does, and takes no press while the place's
// actions stand where the counts are (away).
func (s *State) statsClear(c *ui.Context, semantic theme.Theme, size, lineH float32, ink ui.Color, spaceW, sepW, away float32, animate bool) {
	target := map[bool]float32{true: 1}[s.queryWritten()]
	sec := ui.Row(c).Key("stat-clear-group").Height(lineH).AlignItems(ui.Center).Clip()
	shown := target
	if animate {
		shown = sec.AnimateTo("shown", target, components.Spring(components.MotionPanel))
	} else {
		sec.Jump("shown", target)
	}
	shown = max(0, min(1, shown))
	sec.Opacity(shown)
	if shown <= 0.001 {
		sec.Width(0)
		return
	}
	var clearW float32
	sec.Children(func() {
		ui.Text(c, "·").Font(theme.UIFontFamily).FontSize(size).FixedLineHeight(lineH).
			TextColor(ink).NoWrap().Shrink(0).MarginStart(2 * spaceW).MarginEnd(2 * spaceW)
		btn := ui.ButtonBase(c.Key("clear-search")).Height(lineH).Shrink(0).AlignItems(ui.Center).
			FocusRing(false).KeepFocus().Cursor(ui.CursorDefault).Label(locales.ClearSearch()).
			IgnoreInput(target < 1 || away > 0.5)
		components.Hint(c, semantic, btn, locales.ClearSearch(), s.binding(VerbHome), components.HintAbove)
		if btn.Clicked() {
			s.goHome()
		}
		red := semantic.Status.Danger.Foreground
		hover := map[bool]float32{true: 1}[btn.Hovered()]
		if animate {
			hover = components.Follow(btn, "hover", hover, components.MotionQuick)
		}
		btn.Children(func() {
			ui.Text(c, locales.ClearWord()).Font(theme.UIFontFamily).FontSize(size).FixedLineHeight(lineH).
				TextColor(red.Alpha(0.8 + 0.2*hover)).NoWrap()
		})
		clearW, _ = c.MeasureText(0, ui.Span{Text: locales.ClearWord(), Font: theme.UIFontFamily, Size: size})
	})
	if shown < 0.999 {
		sec.Width((4*spaceW + sepW + clearW) * shown)
	}
}
