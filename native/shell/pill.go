package shell

import (
	"math"
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The pill is where the field's query is written, and the query's
// conditions stand around it as ribbons, each saying its condition in
// words in the color of what it names: the first under the pill, the next
// over it, and so on, outward on either side as the field's cards go, so
// what is written always has the pill's whole width. Nothing of the
// query's language is ever written in the pill for them: a condition
// comes in by a click on the field, or by a key opened from the ribbons
// and one of its values taken (completion.go). A ribbon's × takes it away,
// as Backspace with nothing written takes the last; pressed, a ribbon
// opens its key to change it, and pressed with Option it turns to its
// opposite. Return opens the row the keys are on or takes what an open key
// holds, Tab takes a suggestion, the arrows move along the suggestions or
// the field's rows, and Escape closes an open key, then puts the
// suggestions away, then lets go of the pill; on the field, it puts the
// words away, then the conditions, then goes to the desk. Written after ">", what the pill holds names the app's commands.
// Beside it on the horizon stands Neverland's name, before it.

// horizonSide is how much room, at the reference type size, the field
// keeps on either side of the pill: for Neverland's name before it, and as
// much after it, so the pill stands in the middle.
const horizonSide = 140

// horizonView is what stands on the horizon at r: the pill, the ribbons
// around it as rl lays them out, Neverland's name before it, and the
// field's live statistics after it.
func (s *State) horizonView(c *ui.Context, semantic theme.Theme, r stageRect, rl ribbonLayout, w, k float32, animate bool) {
	gap := 12 * k
	// Pressed, Neverland's name goes home, as the field lane's Home does.
	mark := ui.ButtonBase(c.Key("wordmark")).Absolute().Top(r.Y).Right(w - r.X + gap).Height(r.H).
		AlignItems(ui.Center).FocusRing(false).KeepFocus().Label("Neverland").Tooltip("Home")
	if mark.Clicked() {
		s.goHome()
	}
	mark.Children(func() {
		ui.Row(c).Key("name").AlignItems(ui.End).Children(func() {
			size := 22 * k
			ui.Text(c, "Neverland").Font(theme.UIFontFamily).FontSize(size).FontWeight(700).
				LetterSpacing(-0.02 * size).TextColor(semantic.Text.Default).SingleLine()
			ui.Text(c, ".").Font(theme.UIFontFamily).FontSize(size).FontWeight(700).
				TextColor(semantic.Accent.Foreground).SingleLine()
		})
	})
	acts := s.fieldActions()
	s.noticeView(c, semantic, r, w, gap, k, animate)
	s.horizonStats(c, semantic, r, w, gap, animate)
	// The ribbons first, as they were laid out for what the pill held as
	// the frame began: the pill, taking the keys, may change it.
	// Carry the pill's vertical offset on an unclipped parent so each
	// ribbon's Scale transition never scales the horizon translation.
	ui.Box(c).Key("ribbons").Absolute().Left(0).Top(r.Y).Children(func() {
		for _, p := range rl.places {
			s.ribbon(c, semantic, p, k, animate)
		}
	})
	s.pillView(c, semantic, r, acts, k, animate)
	s.actionsView(c, semantic, acts, r, w, gap, k, animate)
}

type fieldStatsState struct {
	count, words components.RollingNumber
	lastWords    int
}

func nounForm(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func (s *State) horizonStats(c *ui.Context, semantic theme.Theme, r stageRect, w, gap float32, animate bool) {
	if s.Library == nil {
		return
	}
	maxW := max(0, w-(r.X+r.W+gap)-gap)
	if maxW <= 0 {
		return
	}
	st := s.field.found.stats
	size := theme.TextXSmall.Size(s.Settings.UIFontSize)
	lineH := theme.TextXSmall.LineHeight(s.Settings.UIFontSize)
	ink := semantic.Text.Muted
	spaceGapW, _ := c.MeasureText(0, ui.Span{Text: "0 0", Font: theme.UIFontFamily, Size: size})
	pairW, _ := c.MeasureText(0, ui.Span{Text: "00", Font: theme.UIFontFamily, Size: size})
	spaceW := max(size*0.25, spaceGapW-pairW)
	sepW, _ := c.MeasureText(0, ui.Span{Text: "·", Font: theme.UIFontFamily, Size: size})

	var unit string
	switch st.mode {
	case statCommands:
		unit = nounForm(st.count, "command", "commands")
	case statSettings:
		unit = nounForm(st.count, "section", "sections")
	default:
		unit = nounForm(st.count, "sheet", "sheets")
		s.field.stats.lastWords = st.words
	}
	row := ui.Row(c).Key("field-stats").Absolute().Top(r.Y).Left(r.X+r.W+gap).Height(r.H).
		MaxWidth(maxW).AlignItems(ui.Center).IgnoreInput(true).Clip().
		Translate(0, -components.UIOpticalLift(s.Settings.UIFontSize))
	// The place's actions open, they stand where the stats did.
	acting := map[bool]float32{true: 1}[s.field.acting]
	if animate {
		acting = components.Follow(row, "acting", acting, components.MotionQuick)
	}
	s.field.actingShown = acting
	// A notice stands where they do, as long as it stands.
	row.Opacity(1 - max(acting, s.field.told))
	row.Children(func() {
		s.field.stats.count.View(c, "stat-count", st.count, size, lineH, ink, animate)
		ui.Box(c.Key("stat-unit-gap")).Width(spaceW).Shrink(0)
		components.AnimatedText(c, "stat-unit", unit, size, lineH, ink, animate)

		wordCount := s.field.stats.lastWords
		wordUnit := nounForm(wordCount, "word", "words")
		targetShown := float32(0)
		if st.mode == statLibrary {
			targetShown = 1
		}
		sec := ui.Row(c).Key("stat-words-group").Height(lineH).AlignItems(ui.Center).Clip()
		shown := targetShown
		if animate {
			shown = sec.AnimateTo("shown", targetShown, components.Spring(components.MotionPanel))
		} else {
			sec.Jump("shown", targetShown)
		}
		shown = max(0, min(1, shown))
		sec.Opacity(shown)
		var wordsW, wordUnitW float32
		if shown > 0.001 {
			sec.Children(func() {
				ui.Text(c, "·").Font(theme.UIFontFamily).FontSize(size).FixedLineHeight(lineH).
					TextColor(ink).NoWrap().Shrink(0).MarginStart(2 * spaceW).MarginEnd(2 * spaceW)
				wordsW = s.field.stats.words.View(c, "stat-words", wordCount, size, lineH, ink, animate)
				ui.Box(c.Key("stat-word-gap")).Width(spaceW).Shrink(0)
				wordUnitW = components.AnimatedText(c, "stat-word-unit", wordUnit, size, lineH, ink, animate)
			})
			if shown < 0.999 {
				naturalW := 4*spaceW + sepW + wordsW + spaceW + wordUnitW
				sec.Width(naturalW * shown)
			}
		} else {
			sec.Width(0)
		}
	})
}

func (s *State) pillView(c *ui.Context, semantic theme.Theme, r stageRect, acts []fieldAction, k float32, animate bool) {
	pill := ui.Row(c).Key("pill").Absolute().Left(r.X).Top(r.Y).Size(r.W, r.H).
		Radius(r.H / 2).Background(semantic.Surface.Page).
		PaddingStart(r.H * 0.4).Gap(8 * k).AlignItems(ui.Center).
		Cursor(ui.CursorText).Role(ui.RoleSearch).Label("Search")
	var input ui.Element
	pill.Children(func() {
		input = ui.TextInputBase(c.Key("pill-input"), &s.field.words).
			Grow(1).MinWidth(0).Font(theme.UIFontFamily).
			FontSize(theme.TextDefault.Size(s.Settings.UIFontSize)).
			TextColor(semantic.Text.Heading).Placeholder(s.pillPlaceholder()).Label("Search")
		input.OnTextKey(s.pillKey)
		if edit := s.field.edit; edit != nil {
			s.field.edit = nil
			// The caret goes after what was set, as it would after typing it.
			input.ReplaceText(0, math.MaxInt, *edit).SetTextSelection(math.MaxInt, math.MaxInt)
		}
		if s.field.focus && !s.OnDesk() && !s.field.hidden {
			s.field.focus = false
			input.Focus()
		}
		s.spaceKey()
		// A condition written whole, a space after it, becomes a ribbon.
		if strings.HasSuffix(s.field.words, " ") && strings.TrimSpace(s.field.words) != "" {
			if s.takeConditions(s.field.words) {
				s.setWords(s.field.words)
			}
		}
		s.actionsButton(c, semantic, acts, r.H, k, animate)
	})
	_, caret := input.TextSelection()
	if s.offer(byteOffset(s.field.words, caret)) {
		c.AnimationFrame()
	}
	focused := input.Focused()
	// Open, the pill shows what it can narrow by around it; let go of, only
	// what the query holds. Scrolling never opens or closes it, so what
	// the field makes room for stays as it is under the hand.
	open := focused
	if open != s.field.open {
		s.field.open = open
		if !open {
			s.settleQuery()
		}
		c.AnimationFrame()
	}
	if !focused && pill.Clicked() {
		s.field.focus = true
	}
	// Pressed with the field scrolled away, the pill brings it back.
	if (pill.Pressed() || input.Pressed()) && s.field.scroll != 0 && !s.field.home {
		s.field.home, s.field.homeTo = true, 0
	}
	target := float32(0)
	if focused {
		target = 1
	}
	active := target
	if animate {
		active = components.Follow(pill, "focus", target, components.MotionQuick)
	}
	pill.Border(1, components.Blend(semantic.Border.Variant, semantic.Border.Focused, active))
}

// pillPlaceholder is what the pill says with nothing written: what an
// open key takes, or where to go.
func (s *State) pillPlaceholder() string {
	switch {
	case s.field.opened != nil:
		// What the key says of its value: "Title has…", "Updated…".
		return strings.TrimSpace(s.field.opened.Label+" "+s.field.opened.Is) + "…"
	case s.InSettings():
		return "Find a setting…"
	}
	return "Where to, today?"
}

// ribbonH and ribbonGap are a ribbon's height and the room around it, at
// the reference type size.
const ribbonH, ribbonGap = 28, 6

// ribbonKind is what a ribbon is: one of the query's conditions; a key it
// can be narrowed by, or a value of the key being written, either idle
// until pressed; or how many more there are than stand.
func (s *State) pillKey(key ui.TextKey) bool {
	plain := key.Mods == 0
	if plain && s.actionKey(key.Key) {
		return true
	}
	switch key.Key {
	case ui.KeyEscape:
		if len(s.field.picked) > 0 || s.field.acting || s.field.opened != nil || s.offered() {
			s.escapeField()
		} else {
			s.field.quiet = true
		}
		return true
	case ui.KeyBackspace:
		switch {
		case s.field.words != "":
		case s.field.opened != nil:
			s.closeKey()
			return true
		case len(s.field.conditions) > 0:
			s.broaden(len(s.field.conditions) - 1)
			return true
		}
	case ui.KeyTab, ui.KeyEnter:
		if key.Key == ui.KeyTab && !plain {
			break
		}
		switch {
		case s.field.opened != nil:
			s.takeOpened()
			return true
		case s.offered():
			s.activate("suggest:"+strconv.Itoa(s.suggested()), false)
			return true
		}
		if key.Key == ui.KeyTab {
			break
		}
		id := s.field.chosen
		if id == "" {
			for _, row := range s.field.ranked {
				if strings.HasPrefix(row, "sheet:") || strings.HasPrefix(row, "command:") {
					id = row
					break
				}
			}
		}
		s.activate(id, key.Mods&(ui.Cmd|ui.Alt) != 0)
		return true
	case ui.KeyDown, ui.KeyUp:
		if plain && s.offered() {
			// While something is being written, the arrows go along what
			// may stand there.
			s.stepSuggestion(map[ui.Key]int{ui.KeyUp: -1, ui.KeyDown: 1}[key.Key])
			return true
		}
	}
	return s.fieldKey(key)
}

// ribbonTransition brings a ribbon in outward from the pill, moves it as
// the ribbons around it come and go, and tucks it back toward the pill as
// it leaves, mirroring the vertical travel between upper and bottom rows.
func ribbonTransition(animate, above bool, k float32) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	spring := components.Spring(components.MotionScene)
	shift := -8 * k
	if above {
		shift = 8 * k
	}
	gone := &ui.Motion{Y: shift, Opacity: 0, Scale: 0.9}
	return ui.ElementTransition{
		Animation: &spring, ExitDuration: components.MotionQuick, ExitEase: ui.EaseOut,
		Position: true, Colors: true, Enter: gone, Exit: gone,
	}
}
