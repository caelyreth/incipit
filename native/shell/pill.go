package shell

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
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
func (s *State) horizonView(c *ui.Context, semantic theme.Theme, r stage.Rect, rl ribbonLayout, w, k float32, animate bool) {
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
	count, words rollingNumber
	lastWords    int
}

type rollingNumber struct {
	ready   bool
	current int
	seq     int
	entries []rollEntry
}

type rollEntry struct {
	seq      int
	value    int
	mountDir float32 // +1 enters from below, -1 enters from above, 0 initial
	exitDir  float32 // +1 exits upward (-lineH), -1 exits downward (+lineH)
	active   bool
	mounted  bool
}

func (n *rollingNumber) view(c *ui.Context, key string, value int, size, lineH float32, ink ui.Color, animate bool) float32 {
	if !n.ready {
		n.ready = true
		n.current = value
		n.seq = 1
		n.entries = []rollEntry{{seq: 1, value: value, active: true}}
	} else if value != n.current {
		dir := float32(-1)
		if value > n.current {
			dir = 1
		}
		n.current = value
		if !animate {
			n.seq++
			n.entries = []rollEntry{{seq: n.seq, value: value, active: true}}
		} else {
			for i := range n.entries {
				if n.entries[i].active {
					n.entries[i].active = false
					n.entries[i].exitDir = dir
				}
			}
			n.seq++
			n.entries = append(n.entries, rollEntry{
				seq:      n.seq,
				value:    value,
				mountDir: dir,
				active:   true,
			})
		}
	}
	if !animate && len(n.entries) > 1 {
		n.entries = []rollEntry{{seq: n.seq, value: n.current, active: true, mounted: true}}
	}
	targetW, _ := c.MeasureText(0, ui.Span{Text: thousands(n.current), Font: theme.UIFontFamily, Size: size, Features: "tnum"})
	spring := components.Spring(components.MotionPanel)
	box := ui.Box(c.Key(key)).Height(lineH).Clip()
	w := targetW
	if animate {
		w = box.AnimateTo("w", targetW, spring)
	} else {
		box.Jump("w", targetW)
	}
	w = max(0, w)
	kept := n.entries[:0]
	box.Width(w).Children(func() {
		for i := range n.entries {
			e := n.entries[i]
			// Use NoWrap without SingleLine so clipped width animation
			// reveals growing numbers smoothly without ellipsis truncation.
			el := ui.Text(c.Key(e.seq), thousands(e.value)).
				Font(theme.UIFontFamily).FontSize(size).FixedLineHeight(lineH).
				FontFeatures("tnum").TextColor(ink).NoWrap().
				Absolute().Top(0).Left(0)
			var y, a float32
			if !animate {
				e.mounted = true
				y = 0
				a = 1
				el.Jump("y", 0)
				el.Jump("a", 1)
			} else {
				if !e.mounted {
					e.mounted = true
					el.Jump("y", e.mountDir)
					if e.mountDir == 0 {
						el.Jump("a", 1)
					} else {
						el.Jump("a", 0)
					}
				}
				yTarget := float32(0)
				aTarget := float32(1)
				if !e.active {
					yTarget = -e.exitDir
					aTarget = 0
				}
				y = el.AnimateTo("y", yTarget, spring)
				a = el.AnimateTo("a", aTarget, spring)
			}
			el.Translate(0, y*lineH).Opacity(max(0, min(1, a)))
			if e.active || (animate && !(el.Settled("y") && el.Settled("a"))) {
				kept = append(kept, e)
			}
		}
	})
	n.entries = kept
	return w
}

func animatedText(c *ui.Context, key, text string, size, lineH float32, ink ui.Color, animate bool) float32 {
	targetW, _ := c.MeasureText(0, ui.Span{Text: text, Font: theme.UIFontFamily, Size: size})
	box := ui.Box(c.Key(key)).Height(lineH).Clip()
	w := targetW
	if animate {
		w = box.AnimateTo("w", targetW, components.Spring(components.MotionPanel))
	} else {
		box.Jump("w", targetW)
	}
	w = max(0, w)
	box.Width(w).Children(func() {
		ui.Text(c, text).Font(theme.UIFontFamily).FontSize(size).FixedLineHeight(lineH).
			TextColor(ink).NoWrap().Absolute().Top(0).Left(0)
	})
	return w
}

func nounForm(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func (s *State) horizonStats(c *ui.Context, semantic theme.Theme, r stage.Rect, w, gap float32, animate bool) {
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
		s.field.stats.count.view(c, "stat-count", st.count, size, lineH, ink, animate)
		ui.Box(c.Key("stat-unit-gap")).Width(spaceW).Shrink(0)
		animatedText(c, "stat-unit", unit, size, lineH, ink, animate)

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
				wordsW = s.field.stats.words.view(c, "stat-words", wordCount, size, lineH, ink, animate)
				ui.Box(c.Key("stat-word-gap")).Width(spaceW).Shrink(0)
				wordUnitW = animatedText(c, "stat-word-unit", wordUnit, size, lineH, ink, animate)
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

func (s *State) pillView(c *ui.Context, semantic theme.Theme, r stage.Rect, acts []fieldAction, k float32, animate bool) {
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
type ribbonKind uint8

const (
	ribbonCondition ribbonKind = iota
	ribbonKey
	ribbonValue
	ribbonMore
)

// ribbonPlace is where a ribbon stands: what it is, the condition, the
// suggestion or how many more at i, the key it names, what it holds, x from the field's
// left, y from the pill's top (above it, less than 0), w wide.
type ribbonPlace struct {
	kind ribbonKind
	i    int
	key  *query.Key
	// text is the condition, and item the suggestion, as they were as the
	// ribbon was laid out: a ribbon pressed may change the query before
	// the ribbons after it are drawn.
	text    string
	item    query.Suggestion
	x, y, w float32
	// count is how many sheets a value would find, -1 where it is not
	// counted.
	count int
}

// ribbonLayout is where the ribbons stand around the pill, and how far
// they reach above its top and below its foot.
type ribbonLayout struct {
	places       []ribbonPlace
	above, below float32
	// heldAbove and heldBelow are how far the query's own conditions
	// reach, which stay with the pill wherever it is.
	heldAbove, heldBelow float32
}

// idleMost is how many values stand at once; past it, one says how many
// more there are, and what is written narrows them.
const idleMost = 24

// ribbons lays the ribbons out around a pill w wide and h tall, centred at
// mid in the field: the query's conditions, then, while the pill is open,
// each key the query can be narrowed by, idle. Each key's ribbons stand on
// its own side of the pill (ribbonSide), so a condition coming or going
// moves only those on its side; each side in rows as wide as the pill,
// the conditions nearest it. While a key's value is being written, its
// values stand in rows of their own under all of them.
func (s *State) ribbons(c *ui.Context, k, mid, w, h float32) ribbonLayout {
	var out ribbonLayout
	rh, gap := ribbonH*k, ribbonGap*k
	// What the ribbons hold changes as it is written or taken, and the
	// room they take only grows while the pill stays as it is: open or
	// let go of, with the same key's values standing, or, with none, the
	// same conditions. Any of those changing, it is what they take again.
	hold := &s.field.ribbonHold
	stand := fmt.Sprint(s.field.open, s.field.acting)
	if of := s.completing(); of != nil {
		stand += " " + of.Name
	} else {
		stand += " " + strings.Join(s.field.conditions, " ")
	}
	if hold.stand != stand {
		*hold = ribbonHold{stand: stand, widths: map[string]float32{}}
	}
	items := make([]ribbonPlace, 0, len(s.field.conditions)+len(filterKeys))
	for i := range s.field.conditions {
		items = append(items, ribbonPlace{kind: ribbonCondition, i: i, text: s.field.conditions[i]})
	}
	items = append(items, s.keyRibbons()...)
	var sides [2][][]ribbonPlace
	for _, p := range items {
		name := conditionKey(p.text)
		if p.kind == ribbonKey {
			name = p.key.Name
		}
		side := ribbonSide(name)
		sides[side] = s.flow(c, sides[side], p, k, w)
	}
	var values [][]ribbonPlace
	for _, p := range s.valueRibbons() {
		values = s.flow(c, values, p, k, w)
	}
	place := func(rows [][]ribbonPlace, y0, step float32) {
		for r, row := range rows {
			x := mid - rowWidth(row, gap)/2
			for _, p := range row {
				p.x, p.y = x, y0+float32(r)*step
				x += p.w + gap
				out.places = append(out.places, p)
			}
		}
	}
	place(sides[0], h+gap, rh+gap)
	place(sides[1], -(rh + gap), -(rh + gap))
	out.below = float32(len(sides[0])) * (rh + gap)
	out.above = float32(len(sides[1])) * (rh + gap)
	for _, p := range out.places {
		if p.kind == ribbonCondition {
			out.heldAbove, out.heldBelow = max(out.heldAbove, -p.y), max(out.heldBelow, p.y+rh-h)
		}
	}
	if len(values) > 0 {
		// A step further out, so the values read as the key's, apart.
		place(values, h+out.below+3*gap, rh+gap)
		out.below += 2*gap + float32(len(values))*(rh+gap)
	}
	// The place's actions open, their arc about the pill's end takes room
	// in the band over and under it as ribbons do.
	if s.field.acting {
		reach := float32(len(s.fieldActions())-1)/2*(rh+gap) - (h-rh)/2
		out.below, out.above = max(out.below, reach), max(out.above, reach)
	}
	hold.above, hold.below = max(hold.above, out.above), max(hold.below, out.below)
	out.above, out.below = hold.above, hold.below
	return out
}

// ribbonHold is what the ribbons hold to while the pill stands as stand
// says: how far they reach above and below it, and each value's width,
// by its ribbon, so that a count changing never moves those after it.
type ribbonHold struct {
	stand        string
	above, below float32
	widths       map[string]float32
}

// ribbonSide is the side of the pill a key's ribbons stand on, its
// conditions' and its own, 0 under it and 1 over it: the keys alternate
// as the open pill offers them, any other under it.
func ribbonSide(name string) int {
	return max(0, slices.Index(filterKeys, name)) % 2
}

// flow puts p at the end of rows, in a new row where the last is full.
func (s *State) flow(c *ui.Context, rows [][]ribbonPlace, p ribbonPlace, k, w float32) [][]ribbonPlace {
	p.w = min(s.ribbonWidth(c, p, k), w)
	if p.kind == ribbonValue {
		id := s.ribbonID(p)
		p.w = max(p.w, s.field.ribbonHold.widths[id])
		s.field.ribbonHold.widths[id] = p.w
	}
	if n := len(rows); n == 0 || rowWidth(rows[n-1], ribbonGap*k)+ribbonGap*k+p.w > w {
		rows = append(rows, nil)
	}
	rows[len(rows)-1] = append(rows[len(rows)-1], p)
	return rows
}

// keyRibbons are the keys the query can be narrowed by and is not, while
// the pill is open; a key that what is written may start is that
// suggestion's, for the keys to go along.
func (s *State) keyRibbons() []ribbonPlace {
	// The settings are found by their words alone.
	if !s.field.open || s.field.acting || s.InSettings() {
		return nil
	}
	suggested := map[string]int{}
	if s.offered() && s.completing() == nil {
		for i, item := range s.field.completion.Items {
			suggested[keyOf(item)] = i
		}
	}
	var out []ribbonPlace
	add := func(k *query.Key) {
		i, ok := suggested[k.Name]
		if !ok {
			i = -1
		}
		delete(suggested, k.Name)
		p := ribbonPlace{kind: ribbonKey, i: i, key: k}
		if ok {
			p.item = s.field.completion.Items[i]
		}
		out = append(out, p)
	}
	// A key that takes a value stays, for more of its values; one that
	// stands alone goes once it is taken.
	for _, name := range filterKeys {
		if k := mustKey(name); k != nil && (k.Takes != query.TakesNothing || !s.hasCondition(name)) {
			add(k)
		}
	}
	// Keys written that the pill does not offer of itself come after.
	for _, item := range s.field.completion.Items {
		if _, ok := suggested[keyOf(item)]; ok {
			if k := mustKey(keyOf(item)); k != nil {
				add(k)
			}
		}
	}
	return out
}

// valueRibbons are the values of the key being written: no more than
// idleMost of them, until all are asked for.
func (s *State) valueRibbons() []ribbonPlace {
	of := s.completing()
	if !s.offered() || of == nil {
		return nil
	}
	var out []ribbonPlace
	for i, item := range s.field.completion.Items {
		out = append(out, ribbonPlace{kind: ribbonValue, i: i, key: of, item: item, count: -1})
	}
	if len(out) > idleMost && !s.field.allValues {
		out = append(out[:idleMost-1], ribbonPlace{kind: ribbonMore, i: len(out) - idleMost + 1})
	}
	// Each value says how many sheets it would find.
	var values []string
	for _, p := range out {
		if p.kind == ribbonValue {
			values = append(values, rawValue(p.item))
		}
	}
	if s.Library != nil && len(values) > 0 {
		counts := s.Library.CountValues(s.countedWith(of), of.Name, values)
		for i, p := range out {
			if n, ok := counts[rawValue(p.item)]; ok && p.kind == ribbonValue {
				out[i].count = n
			}
		}
	}
	return out
}

// keyOf is the key a key's suggestion names.
func keyOf(item query.Suggestion) string {
	return strings.TrimSuffix(strings.TrimPrefix(item.Text, "$"), ":")
}

func rowWidth(row []ribbonPlace, gap float32) float32 {
	w := float32(0)
	for i, p := range row {
		if i > 0 {
			w += gap
		}
		w += p.w
	}
	return w
}

// filterKeys are the keys the open pill offers to narrow by, in order.
var filterKeys = []string{"tag", "in", "updated", "created", "words", "has", "title", "heading", "code", "quote", "link", "inbox", "open", "sort"}

// hasCondition reports whether the query has a condition of the key name.
func (s *State) hasCondition(name string) bool {
	for _, text := range s.field.conditions {
		if conditionKey(text) == name {
			return true
		}
	}
	return false
}

// ribbonEdge is how a ribbon is outlined: not, in dashes, or in full.
type ribbonEdge uint8

const (
	edgeNone ribbonEdge = iota
	edgeDashed
	edgeSolid
)

// ribbonLook is how a ribbon shows: its words, how many it would find,
// and icon, the ink of its icon, its words' ink, its fill, which none is
// transparent, its outline, how much of it shows, and whether it has a ×
// to take it away.
type ribbonLook struct {
	label  string
	count  string
	icon   components.IconName
	ink    ui.Color
	text   ui.Color
	fill   ui.Color
	edge   ribbonEdge
	shown  float32
	closes bool
}

// look is how the ribbon at p shows, by what it is and how it stands:
//   - a condition, filled in the color of what it names, with its ×;
//   - a key idle, outlined in dashes, a + before it; the key whose values
//     are being written, filled in the accent, opened; while a key is
//     being written, the keys it cannot start faded;
//   - a value idle, outlined in dashes, how many sheets it would find
//     after it, faded where it would find none; what is written, a pen
//     before it; a value the query has taken, filled, a tick before it;
//   - how many more there are, plain;
//   - what the keys are on, outlined in full over any of these.
func (s *State) look(semantic theme.Theme, p ribbonPlace) ribbonLook {
	l := ribbonLook{ink: semantic.Icon.Muted, text: semantic.Text.Default, edge: edgeDashed, shown: 1}
	switch p.kind {
	case ribbonCondition:
		l = s.conditionLook(semantic, p.text)
	case ribbonKey:
		l.label, l.icon = p.key.Label, components.IconPlus
		if of := s.completing(); of != nil && of.Name == p.key.Name {
			l.icon, l.edge = components.IconChevronDown, edgeNone
			l.ink, l.text, l.fill = semantic.Accent.Foreground, semantic.Accent.Foreground, semantic.Accent.Muted
		} else if s.offered() && s.completing() == nil && p.i < 0 {
			l.shown = 0.4
		}
	case ribbonValue:
		v := rawValue(p.item)
		l.label = query.Describe(p.key, v)
		if strings.HasPrefix(p.item.Label, "#") {
			l.label = "#" + v
		}
		if p.count >= 0 {
			l.count = thousands(p.count)
			if p.count == 0 {
				l.shown = 0.5
			}
		}
		if p.item.Written {
			l.icon = components.IconPen
		}
		if s.takenAt(p.key, v) >= 0 {
			l.icon, l.edge = components.IconCheck, edgeNone
			l.ink, l.text, l.fill = semantic.Accent.Foreground, semantic.Text.Heading, semantic.Surface.Chosen
		}
	case ribbonMore:
		l.label, l.icon, l.edge = thousands(p.i)+" more", components.IconChevronDown, edgeNone
		l.text = semantic.Text.Muted
	}
	if p.kind != ribbonCondition && p.kind != ribbonMore && p.i >= 0 && p.i == s.field.suggestion {
		l.edge = edgeSolid
	}
	return l
}

// conditionLook is how the condition text shows as a ribbon: its words,
// and the icon and color of what it names.
func (s *State) conditionLook(semantic theme.Theme, text string) ribbonLook {
	l := ribbonLook{label: text, ink: semantic.Text.Default, text: semantic.Text.Heading, shown: 1, closes: true}
	if cond, ok := condition(text); ok {
		l.label = cond.Sentence()
		switch cond.Key.Name {
		case "in":
			if s.InSettings() {
				l.icon, l.label = settingsCategoryIcon(cond.Value), cond.Value
				break
			}
			if s.Library != nil {
				if g, ok := s.groupNamed(cond.Value); ok {
					l.icon = groupIcon(g)
					if tint := groupTint(semantic, g); tint != nil {
						l.ink = *tint
					}
				}
			}
		case "settings":
			l.icon = components.IconSettings
		case "trash":
			l.icon = components.IconTrash
		}
	}
	l.fill = l.ink.Alpha(theme.TintFill)
	return l
}

// The parts of a ribbon, at the reference type size: its padding at the
// start, its icon and the room after it, and its ×, which stands as far in
// from the ribbon's end as from its top and foot, a circle in the rounded
// end; ribbonX is the × and the room before it.
const (
	ribbonStart, ribbonIcon = 10, 14 + 6
	ribbonClose             = 16
	ribbonEnd               = (ribbonH - ribbonClose) / 2
	ribbonX                 = 6 + ribbonClose
)

// ribbonWidth is how wide the ribbon at p is: what it says, whole.
func (s *State) ribbonWidth(c *ui.Context, p ribbonPlace, k float32) float32 {
	l := s.look(theme.Theme{}, p)
	tw, _ := c.MeasureText(0, ui.Span{Text: l.label, Font: theme.UIFontFamily, Size: theme.TextSmall.Size(components.UIFontSize(c))})
	w := float32(math.Ceil(float64(tw))) + 1 + ribbonStart*k
	if l.count != "" {
		cw, _ := c.MeasureText(0, ui.Span{Text: l.count, Font: theme.UIFontFamily, Size: theme.TextXSmall.Size(components.UIFontSize(c)), Features: "tnum"})
		w += float32(math.Ceil(float64(cw))) + 1 + 6*k
	}
	if l.icon != "" {
		w += ribbonIcon * k
	}
	if l.closes {
		return w + ribbonX*k + ribbonEnd*k
	}
	return w + ribbonStart*k
}

// ribbonID is the ribbon's identity as the ribbons move about.
func (s *State) ribbonID(p ribbonPlace) string {
	switch p.kind {
	case ribbonKey:
		return "key-" + p.key.Name
	case ribbonValue:
		// What is written keeps one ribbon as it is written.
		if p.item.Written {
			return "value-" + p.key.Name + "-written"
		}
		return "value-" + p.key.Name + "-" + p.item.Text
	case ribbonMore:
		return "more"
	}
	// A key that holds one value keeps one ribbon as its value changes.
	if c, ok := condition(p.text); ok && written(c).Key.OneValue() {
		return "ribbon-$" + written(c).Key.Name
	}
	return "ribbon-" + p.text
}

// ribbon is the ribbon at p around the pill, as it looks (look):
// a condition, pressed to be written again or with Option turned around,
// its × taking it away; a key, pressed to be written; a value, pressed to
// be taken or let go; how many more, pressed to show them all.
func (s *State) ribbon(c *ui.Context, semantic theme.Theme, p ribbonPlace, k float32, animate bool) {
	l := s.look(semantic, p)
	h := ribbonH * k
	// It stands where the layout puts it about the pill, and only that
	// moves it on its spring; the ribbons container carries the pill's
	// vertical place at once as the field scrolls. Away from the horizon,
	// all but the query's own conditions are put away.
	shown := l.shown
	if p.kind != ribbonCondition {
		shown *= 1 - s.field.away
	}
	above := p.y < 0
	originY := float32(0)
	if above {
		originY = 1
	}
	btn := ui.ButtonBase(c.Key(s.ribbonID(p))).Absolute().Left(p.x).Top(p.y).Size(p.w, h).
		TransformOrigin(0.5, originY).
		PaddingStart(ribbonStart * k).Gap(6 * k).AlignItems(ui.Center).Radius(h / 2).Opacity(shown).
		IgnoreInput(shown < 0.5).FocusRing(false).KeepFocus().Label(l.label).Transition(ribbonTransition(animate, above, k))
	hover := components.HoverAmount(btn, btn.Hovered(), animate)
	if l.closes {
		btn.PaddingEnd(ribbonEnd * k)
	} else {
		btn.PaddingEnd(ribbonStart * k)
	}
	if l.fill.A > 0 {
		btn.Background(components.Blend(l.fill, l.fill.Alpha(min(1, float32(l.fill.A)/255+0.1)), hover))
	} else {
		btn.Background(components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover))
	}
	switch l.edge {
	case edgeSolid:
		btn.Border(1, semantic.Border.Focused)
	case edgeDashed:
		btn.Draw(func(pt *ui.Painter, r ui.Rect) { components.DashedOutline(pt, r, h/2, 1, semantic.Border.Default) })
	}
	switch p.kind {
	case ribbonCondition:
		if s.InSettings() && (conditionKey(p.text) == "settings" || conditionKey(p.text) == "in") {
			btn.Tooltip("Remove " + l.label)
			if btn.Clicked() {
				if i := slices.Index(s.field.conditions, p.text); i >= 0 {
					s.broaden(i)
				}
			}
			break
		}
		btn.Tooltip("Change " + l.label + "; with Option, turn it around; drag it to the lane to keep it").
			Drag(queryDrag{text: p.text})
		if btn.Clicked() {
			if i := slices.Index(s.field.conditions, p.text); i >= 0 && btn.ClickModifiers()&ui.Alt != 0 {
				s.turn(i)
			} else if i >= 0 {
				s.rewrite(i)
			}
		}
	case ribbonMore:
		btn.Tooltip("Show them all")
		if btn.Clicked() {
			s.field.allValues = true
		}
	default:
		if p.i >= 0 {
			btn.Tooltip(p.item.Help)
		} else {
			btn.Tooltip(p.key.Help)
		}
		if btn.Clicked() {
			if p.i >= 0 {
				s.activate("suggest:"+strconv.Itoa(p.i), false)
			} else {
				s.activate("key:"+p.key.Name, false)
			}
		}
	}
	btn.Children(func() {
		if l.icon != "" {
			ui.Icon(c, l.icon.SVG()).FontSize(14 * k).TextColor(l.ink).Shrink(0)
		}
		words := components.BaselineRow(c).Key("words").Grow(1).MinWidth(0).Gap(6 * k)
		words.Children(func() {
			components.Label(c, l.label, theme.TextSmall, l.text).SingleLine().Shrink(0).Grow(1)
			if l.count != "" {
				components.Label(c, l.count, theme.TextXSmall, semantic.Text.Muted).SingleLine().Shrink(0).FontFeatures("tnum")
			}
		})
		if !l.closes {
			return
		}
		gone := ui.ButtonBase(c.Key("remove")).Size(ribbonClose*k, ribbonClose*k).Radius(ribbonClose * k / 2).Shrink(0).
			AlignItems(ui.Center).Justify(ui.Center).FocusRing(false).KeepFocus().
			Label("Remove " + l.label).Tooltip("Remove")
		gone.Background(components.Blend(l.ink.Alpha(0), l.ink.Alpha(theme.TintFill), components.HoverAmount(gone, gone.Hovered(), animate)))
		if i := slices.Index(s.field.conditions, p.text); i >= 0 && gone.Clicked() {
			s.broaden(i)
		}
		gone.Children(func() {
			ui.Icon(c, components.IconClose.SVG()).FontSize(10 * k).TextColor(semantic.Icon.Muted)
		})
	})
}

// pillKey is what a key does in the pill's input before the input takes it.
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
			for _, row := range s.field.order {
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
