package shell

import (
	"math"
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
// what is written always has the pill's whole width. A condition comes in
// by a click on the field, a suggestion taken, or written whole and
// followed by a space. A ribbon's × takes it away, as Backspace with
// nothing written takes the last; pressed, a ribbon goes back into the
// pill to be written again, and pressed with Option it turns to its
// opposite. Return opens the row the keys are on, Tab takes the suggestion
// they are on or the first, the arrows move them along the field's rows,
// and Escape puts the suggestions away, then the words, then the
// conditions, then goes to the desk. Written after ">", what the pill
// holds names the app's commands. Beside it on the horizon stand
// Neverland's name, before it, and the settings, after it.

// horizonSide is how much room, at the reference type size, the mark and
// the settings each take beside the pill.
const horizonSide = 140

// horizonView is what stands on the horizon at r: the pill, the ribbons
// around it as rl lays them out, Neverland's name just before it, and the
// settings just after.
func (s *State) horizonView(c *ui.Context, semantic theme.Theme, r stage.Rect, rl ribbonLayout, w, k float32, animate bool) {
	gap := 12 * k
	ui.Row(c).Key("wordmark").Absolute().Top(r.Y).Right(w - r.X + gap).Height(r.H).
		AlignItems(ui.Center).IgnoreInput(true).Role(ui.RoleImage).Label("Neverland").
		Children(func() {
			ui.Row(c).Key("name").AlignItems(ui.End).Children(func() {
				size := 22 * k
				ui.Text(c, "Neverland").Font(theme.UIFontFamily).FontSize(size).FontWeight(700).
					LetterSpacing(-0.02 * size).TextColor(semantic.Text.Default).SingleLine()
				ui.Text(c, ".").Font(theme.UIFontFamily).FontSize(size).FontWeight(700).
					TextColor(semantic.Accent.Foreground).SingleLine()
			})
		})
	// The ribbons first, as they were laid out for what the pill held as
	// the frame began: the pill, taking the keys, may change it.
	for _, p := range rl.places {
		s.ribbon(c, semantic, p, r.Y, k, animate)
	}
	s.pillView(c, semantic, r, k, animate)
	settings := ui.ButtonBase(c.Key("settings")).Absolute().Left(r.X+r.W+gap).Top(r.Y).Size(r.H, r.H).
		Radius(r.H / 2).AlignItems(ui.Center).Justify(ui.Center).FocusRing(false).
		Label("Open settings").Tooltip("Settings")
	hover := components.HoverAmount(settings, settings.Hovered(), animate)
	settings.Background(components.Blend(semantic.Surface.Page, semantic.GhostElement.Hover, hover)).
		Border(1, semantic.Border.Variant)
	if settings.Clicked() && s.OpenSettings != nil {
		s.OpenSettings()
	}
	settings.Children(func() {
		ui.Icon(c, components.IconSettings.SVG()).FontSize(16 * k).TextColor(semantic.Icon.Muted)
	})
}

func (s *State) pillView(c *ui.Context, semantic theme.Theme, r stage.Rect, k float32, animate bool) {
	pill := ui.Row(c).Key("pill").Absolute().Left(r.X).Top(r.Y).Size(r.W, r.H).
		Radius(r.H / 2).Background(semantic.Surface.Page).
		PaddingStart(r.H * 0.4).PaddingEnd(r.H * 0.2).Gap(8 * k).AlignItems(ui.Center).
		Cursor(ui.CursorText).Role(ui.RoleSearch).Label("Search")
	var input ui.Element
	pill.Children(func() {
		input = ui.TextInputBase(c.Key("pill-input"), &s.field.words).
			Grow(1).MinWidth(0).Font(theme.UIFontFamily).
			FontSize(theme.TextDefault.Size(s.Settings.UIFontSize)).
			TextColor(semantic.Text.Heading).Placeholder("Where to, today?").Label("Search")
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
		// A condition written whole, a space after it, becomes a ribbon.
		if strings.HasSuffix(s.field.words, " ") && strings.TrimSpace(s.field.words) != "" {
			before := len(s.field.conditions)
			s.takeConditions(s.field.words)
			if len(s.field.conditions) > before {
				s.setWords(s.field.words)
			}
		}
		s.confirmButton(c, semantic, emptyTrashKey, "", "Empty Trash", animate, func() { s.Library.EmptyTrash() })
	})
	_, caret := input.TextSelection()
	if s.offer(byteOffset(s.field.words, caret)) {
		c.AnimationFrame()
	}
	focused := input.Focused()
	// Open, the pill shows what it can narrow by around it; away from its
	// horizon, or let go of, only what the query holds.
	open := focused && (math.Abs(float64(s.field.scroll)) < 1 || s.field.home)
	if open != s.field.open {
		s.field.open = open
		c.AnimationFrame()
	}
	if !focused && pill.Clicked() {
		s.field.focus = true
	}
	// Pressed with the field scrolled away, the pill brings it back.
	if (pill.Pressed() || input.Pressed()) && s.field.scroll != 0 && !s.field.home {
		s.field.home = true
	}
	target := float32(0)
	if focused {
		target = 1
	}
	active := target
	if animate {
		duration := components.HoverOut
		if focused {
			duration = components.HoverIn
		}
		active = components.Follow(pill, "focus", target, duration)
	}
	pill.Border(1, components.Blend(semantic.Border.Variant, semantic.Border.Focused, active))
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
// suggestion or how many more at i, the key it names, x from the field's
// left, y from the pill's top (above it, less than 0), w wide.
type ribbonPlace struct {
	kind    ribbonKind
	i       int
	key     *query.Key
	x, y, w float32
}

// ribbonLayout is where the ribbons stand around the pill, and how far
// they reach above its top and below its foot.
type ribbonLayout struct {
	places       []ribbonPlace
	above, below float32
}

// idleMost is how many values stand at once; past it, one says how many
// more there are, and what is written narrows them.
const idleMost = 24

// ribbons lays the ribbons out around a pill w wide and h tall, centred at
// mid in the field: the query's conditions, then, while the pill is open,
// each key the query can be narrowed by and is not, idle. They go alternately under and over the pill, each
// side in rows as wide as it, the first row nearest it. While a key's
// value is being written, its values stand in rows of their own under all
// of them.
func (s *State) ribbons(c *ui.Context, k, mid, w, h float32) ribbonLayout {
	var out ribbonLayout
	rh, gap := ribbonH*k, ribbonGap*k
	items := make([]ribbonPlace, 0, len(s.field.conditions)+len(filterKeys))
	for i := range s.field.conditions {
		items = append(items, ribbonPlace{kind: ribbonCondition, i: i})
	}
	items = append(items, s.keyRibbons()...)
	var sides [2][][]ribbonPlace
	for n, p := range items {
		sides[n%2] = s.flow(c, sides[n%2], p, k, w)
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
	if len(values) > 0 {
		// A step further out, so the values read as the key's, apart.
		place(values, h+out.below+3*gap, rh+gap)
		out.below += 2*gap + float32(len(values))*(rh+gap)
	}
	return out
}

// flow puts p at the end of rows, in a new row where the last is full.
func (s *State) flow(c *ui.Context, rows [][]ribbonPlace, p ribbonPlace, k, w float32) [][]ribbonPlace {
	p.w = min(s.ribbonWidth(c, p, k), w)
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
	if !s.field.open {
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
		out = append(out, ribbonPlace{kind: ribbonKey, i: i, key: k})
	}
	for _, name := range filterKeys {
		if k := mustKey(name); k != nil && !s.hasCondition(name) {
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

// valueRibbons are the values of the key being written, no more than
// idleMost of them.
func (s *State) valueRibbons() []ribbonPlace {
	of := s.completing()
	if !s.offered() || of == nil {
		return nil
	}
	var out []ribbonPlace
	for i := range s.field.completion.Items {
		out = append(out, ribbonPlace{kind: ribbonValue, i: i, key: of})
	}
	if len(out) > idleMost {
		out = append(out[:idleMost-1], ribbonPlace{kind: ribbonMore, i: len(out) - idleMost + 1})
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
var filterKeys = []string{"tag", "updated", "created", "words", "has", "title", "link", "inbox", "open", "sort", "trash"}

// hasCondition reports whether the query has a condition of the key name.
func (s *State) hasCondition(name string) bool {
	for _, text := range s.field.conditions {
		if conditionKey(text) == name {
			return true
		}
	}
	return false
}

// ribbonFace is how a ribbon shows: its words, its icon, and the ink of
// what it names. An idle key is its name, and a value as its key says it.
func (s *State) ribbonFace(semantic theme.Theme, p ribbonPlace) (label string, icon components.IconName, ink ui.Color) {
	switch p.kind {
	case ribbonKey:
		return p.key.Label, components.IconPlus, semantic.Text.Muted
	case ribbonValue:
		item := s.field.completion.Items[p.i]
		if strings.HasPrefix(item.Label, "#") {
			return item.Label, "", semantic.Text.Muted
		}
		return query.Describe(p.key, item.Label), "", semantic.Text.Muted
	case ribbonMore:
		return thousands(p.i) + " more", "", semantic.Text.Muted
	}
	text := s.field.conditions[p.i]
	label, ink = text, semantic.Text.Default
	cond, ok := condition(text)
	if !ok {
		return label, icon, ink
	}
	label = cond.Sentence()
	switch cond.Key.Name {
	case "in":
		if s.Library != nil {
			if g, ok := s.groupNamed(cond.Value); ok {
				icon = groupIcon(g)
				if tint := groupTint(semantic, g); tint != nil {
					ink = *tint
				}
			}
		}
	case "trash":
		icon = components.IconTrash
	}
	return label, icon, ink
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
	label, icon, _ := s.ribbonFace(theme.Theme{}, p)
	tw, _ := c.MeasureText(0, ui.Span{Text: label, Font: theme.UIFontFamily, Size: theme.TextSmall.Size(components.UIFontSize(c))})
	w := float32(math.Ceil(float64(tw))) + 1 + ribbonStart*k
	if icon != "" {
		w += ribbonIcon * k
	}
	if p.kind != ribbonCondition {
		return w + ribbonStart*k
	}
	return w + ribbonX*k + ribbonEnd*k
}

// ribbonID is the ribbon's identity as the ribbons move about.
func (s *State) ribbonID(p ribbonPlace) string {
	switch p.kind {
	case ribbonKey:
		return "key-" + p.key.Name
	case ribbonValue:
		return "value-" + p.key.Name + "-" + s.field.completion.Items[p.i].Text
	case ribbonMore:
		return "more"
	}
	return "ribbon-" + s.field.conditions[p.i]
}

// ribbon is the ribbon at p, the pill's top at top: a condition, pressed
// to be written again or with Option turned around, its × taking it away;
// or something idle, pressed to be written.
func (s *State) ribbon(c *ui.Context, semantic theme.Theme, p ribbonPlace, top, k float32, animate bool) {
	label, icon, ink := s.ribbonFace(semantic, p)
	h := ribbonH * k
	btn := ui.ButtonBase(c.Key(s.ribbonID(p))).Absolute().Left(p.x).Top(top+p.y).Size(p.w, h).
		PaddingStart(ribbonStart * k).Gap(6 * k).AlignItems(ui.Center).Radius(h / 2).
		FocusRing(false).KeepFocus().Label(label).Transition(ribbonTransition(animate))
	hover := components.HoverAmount(btn, btn.Hovered(), animate)
	if p.kind == ribbonCondition {
		btn.PaddingEnd(ribbonEnd * k).Tooltip("Change " + label + "; with Option, turn it around").
			Background(components.Blend(ink.Alpha(0.14), ink.Alpha(0.24), hover))
		if btn.Clicked() {
			if btn.ClickModifiers()&ui.Alt != 0 {
				s.turn(p.i)
			} else {
				s.rewrite(p.i)
			}
		}
	} else {
		// Idle, a ribbon is only outlined, in dashes; the key whose value
		// is being written, and the suggestion the keys are on, in full.
		btn.PaddingEnd(ribbonStart * k).
			Background(components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover))
		of := s.completing()
		if of != nil && p.kind == ribbonKey && of.Name == p.key.Name || p.kind != ribbonMore && p.i >= 0 && p.i == s.suggested() {
			btn.Border(1, semantic.Border.Focused)
		} else {
			btn.Draw(func(pt *ui.Painter, r ui.Rect) { components.DashedOutline(pt, r, h/2, 1, semantic.Border.Default) })
		}
		switch {
		case p.kind == ribbonMore:
			btn.IgnoreInput(true)
		case p.i >= 0:
			btn.Tooltip(s.field.completion.Items[p.i].Help)
			if btn.Clicked() {
				s.activate("suggest:"+strconv.Itoa(p.i), false)
			}
		default:
			btn.Tooltip(p.key.Help)
			if btn.Clicked() {
				s.activate("key:"+p.key.Name, false)
			}
		}
	}
	btn.Children(func() {
		if icon != "" {
			ui.Icon(c, icon.SVG()).FontSize(14 * k).TextColor(ink).Shrink(0)
		}
		text := semantic.Text.Heading
		if p.kind != ribbonCondition {
			text = semantic.Text.Default
		}
		components.Label(c, label, theme.TextSmall, text).SingleLine().Shrink(0)
		if p.kind != ribbonCondition {
			return
		}
		ui.Box(c).Grow(1)
		gone := ui.ButtonBase(c.Key("remove")).Size(ribbonClose*k, ribbonClose*k).Radius(ribbonClose * k / 2).Shrink(0).
			AlignItems(ui.Center).Justify(ui.Center).FocusRing(false).KeepFocus().
			Label("Remove " + label).Tooltip("Remove")
		gone.Background(components.Blend(ink.Alpha(0), ink.Alpha(0.2), components.HoverAmount(gone, gone.Hovered(), animate)))
		if gone.Clicked() {
			s.broaden(p.i)
		}
		gone.Children(func() {
			ui.Icon(c, components.IconClose.SVG()).FontSize(10 * k).TextColor(semantic.Icon.Muted)
		})
	})
}

// pillKey is what a key does in the pill's input before the input takes it.
func (s *State) pillKey(key ui.TextKey) bool {
	plain := key.Mods == 0
	switch key.Key {
	case ui.KeyEscape:
		s.escapeField()
		return true
	case ui.KeyBackspace:
		if s.field.words == "" && len(s.field.conditions) > 0 {
			s.broaden(len(s.field.conditions) - 1)
			return true
		}
	case ui.KeyTab:
		if plain && s.offered() {
			s.activate("suggest:"+strconv.Itoa(s.suggested()), false)
			return true
		}
	case ui.KeyEnter:
		if s.offered() {
			s.activate("suggest:"+strconv.Itoa(s.suggested()), false)
			return true
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
			n := len(s.field.completion.Items)
			s.field.suggestion = (s.suggested() + map[ui.Key]int{ui.KeyUp: n - 1, ui.KeyDown: 1}[key.Key]) % n
			return true
		}
		if plain {
			s.choose(map[ui.Key]int{ui.KeyUp: -1, ui.KeyDown: 1}[key.Key])
			return true
		}
	}
	return false
}

// ribbonTransition brings a ribbon in as it grows from its place, moves
// it as the ribbons around it come and go, and takes it away as it fades.
func ribbonTransition(animate bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	spring := components.Spring(components.MotionScene)
	gone := &ui.Motion{Opacity: 0, Scale: 0.9}
	return ui.ElementTransition{Animation: &spring, Position: true, Colors: true, Enter: gone, Exit: gone}
}
