package shell

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/internal/text/query"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

type ribbonKind uint8

const (
	ribbonCondition ribbonKind = iota
	ribbonKey
	ribbonValue
	ribbonMore
	// ribbonMoreKeys stands after the keys first offered, for the rest.
	ribbonMoreKeys
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

// keysOffered is how many keys the pill offers opened with nothing
// written, before More Filters.
const keysOffered = 4

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
	// A list stands over the pill, its ground; what the pill offers stands
	// over it too, where the room is.
	listed := s.listed()
	hold := &s.field.ribbonHold
	// The actions standing about the pill's end take room as the ribbons
	// do, so what stands near moves aside for them rather than under them;
	// opening or closing them is a change of what stands.
	acting := s.field.acting || s.actionsStand()
	stand := fmt.Sprint(s.field.open, listed, acting)
	if of := s.completing(); of != nil {
		stand += " " + of.Name
	} else {
		stand += " " + strings.Join(s.field.conditions, " ")
	}
	if hold.stand != stand {
		*hold = ribbonHold{stand: stand, widths: map[string]float32{}}
	}
	items := s.field.ribbonItems[:0]
	defer func() { s.field.ribbonItems = items[:0] }()
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
		if listed {
			side = 1
		}
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
	switch {
	case len(values) == 0:
	case listed:
		// A step further out, so the values read as the key's, apart.
		place(values, -(out.above + 3*gap + rh), -(rh + gap))
		out.above += 2*gap + float32(len(values))*(rh+gap)
	default:
		place(values, h+out.below+3*gap, rh+gap)
		out.below += 2*gap + float32(len(values))*(rh+gap)
	}
	if acting {
		reach := actionsReach(len(s.fieldActions()), h, rh, gap)
		out.above, out.below = max(out.above, reach), max(out.below, reach)
	}
	hold.above, hold.below = max(hold.above, out.above), max(hold.below, out.below)
	out.above, out.below = hold.above, hold.below
	return out
}

// actionsReach is how far n actions rh tall, standing about the end of a
// pill h tall (arc), reach past its top and its foot, a gap past them.
func actionsReach(n int, h, rh, gap float32) float32 {
	if n == 0 {
		return 0
	}
	return max(0, float32(n-1)/2*(rh+gap)-(h-rh)/2+gap)
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
	if !s.field.open || s.field.acting {
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
	names := filterKeys
	switch {
	case s.InSettings():
		// The settings are found by their words, and the shortcuts also by
		// their keys.
		names = settingsKeys
	case s.listed():
		// A list is read in an order: its sort is offered first.
		names = append([]string{"sort"}, slices.DeleteFunc(slices.Clone(filterKeys), func(n string) bool { return n == "sort" })...)
	}
	for _, name := range names {
		// A sort is the list's: the field is offered none.
		if name == "sort" && !s.listed() {
			continue
		}
		if k := mustKey(name); k != nil && (k.Takes != query.TakesNothing || !s.hasCondition(name)) {
			add(k)
		}
	}
	// Opened with nothing written, the pill offers the keys most reached
	// for, and the rest behind one more: a first click to search meets a
	// few ways in, not the whole of the query's language.
	if len(suggested) == 0 && !s.field.allKeys && len(out) > keysOffered+1 {
		out = append(out[:keysOffered], ribbonPlace{kind: ribbonMoreKeys, i: -1})
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
	// Each value says how many sheets it would find; a setting's, none.
	if s.InSettings() {
		return out
	}
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

// settingsKeys are the keys the open pill offers among the settings.
var settingsKeys = []string{"key"}

// filterKeys are the keys the open pill offers to narrow by, in order.
var filterKeys = []string{"tag", "in", "updated", "created", "words", "has", "title", "heading", "code", "quote", "link", "inbox", "open", "bands", "sort"}

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
		l.label, l.icon = p.key.Label(), components.IconPlus
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
		l.label, l.icon, l.edge = locales.More(thousands(p.i)), components.IconChevronDown, edgeNone
		l.text = semantic.Text.Muted
	case ribbonMoreKeys:
		l.label, l.icon, l.edge = locales.MoreFilters(), components.IconChevronDown, edgeNone
		l.text = semantic.Text.Muted
	}
	// An order says which view it is read in, and shows it once chosen.
	if (p.kind == ribbonKey || p.kind == ribbonValue) && viewOf[p.key.Name] != "" {
		l.count = locales.ViewList()
		if viewOf[p.key.Name] == "field" {
			l.count = locales.ViewField()
		}
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
				l.icon, l.label = settingsCategoryIcon(cond.Value), s.settingsCategoryTitle(cond.Value)
				break
			}
			if s.Library != nil {
				if g, ok := s.groupNamed(cond.Value); ok {
					shown := cond
					shown.Value = g.Title()
					l.label = shown.Sentence()
					l.icon = groupIcon(g)
					l.ink = groupInk(semantic, g, l.ink)
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

// viewOf is the view each of the keys that order things is read in: a
// sort is the list's, exact; bands the field's, near and far.
var viewOf = map[string]string{"sort": "list", "bands": "field"}

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
	case ribbonMoreKeys:
		return "more-keys"
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
	shown := l.shown * s.field.offered
	if p.kind != ribbonCondition {
		shown *= 1 - s.field.away
	}
	above := p.y < 0
	originY := float32(0)
	if above {
		originY = 1
	}
	// Each view, and each row, has ribbons of its own: a ribbon moves along
	// its row as those before it come and go, but one that goes to another
	// row, or another view, goes where it stood and comes in where it now
	// stands, never travelling across the others.
	key := s.ribbonID(p) + fmt.Sprintf("@%.0f", p.y/(h+ribbonGap*k))
	if s.listed() {
		key += "@list"
	}
	// Put away while a change moves them, they go to where they will stand
	// unseen, and come back there still.
	moves := animate && s.field.offered > 0.001
	btn := ui.ButtonBase(c.Key(key)).Absolute().Left(p.x).Top(p.y).Size(p.w, h).
		TransformOrigin(0.5, originY).
		PaddingStart(ribbonStart * k).Gap(6 * k).AlignItems(ui.Center).Radius(h / 2).Opacity(shown).
		IgnoreInput(shown < 0.5).FocusRing(false).KeepFocus().Label(l.label).Transition(ribbonTransition(moves, above, k))
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
		rim := semantic.Border.Default
		btn.Draw(func(pt *ui.Painter, r ui.Rect) { components.DashedOutline(pt, r, h/2, 1, rim) })
	}
	switch p.kind {
	case ribbonCondition:
		if s.InSettings() && (conditionKey(p.text) == "settings" || conditionKey(p.text) == "in") {
			if btn.Clicked() {
				if i := slices.Index(s.field.conditions, p.text); i >= 0 {
					s.broaden(i)
				}
			}
			break
		}
		btn.Drag(queryDrag{text: p.text})
		btn.ContextMenu(s.ribbonMenu(p.text))
		// The group the field is in takes what is dropped on its ribbon, as
		// on its head: its sheets each stand alone, with no block to drop on.
		if g, ok := s.scopeGroup(); ok && conditionKey(p.text) == "in" {
			if _, dropped, ok := s.dropOn(c, semantic, btn, filing(g), animate); ok {
				s.fileInto(g, dropped)
			}
		}
		if btn.Clicked() {
			if i := slices.Index(s.field.conditions, p.text); i >= 0 && btn.ClickModifiers()&ui.Alt != 0 {
				s.turn(i)
			} else if i >= 0 {
				s.rewrite(i)
			}
		}
	case ribbonMore:
		if btn.Clicked() {
			s.field.allValues = true
		}
	case ribbonMoreKeys:
		if btn.Clicked() {
			s.field.allKeys = true
		}
	default:
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
		miniCloseButton(c, semantic, "remove", locales.RemoveNamed(l.label), locales.Remove(), ribbonClose*k, 10*k,
			semantic.Icon.Muted, l.ink.Alpha(theme.TintFill), animate,
			func() {
				if i := slices.Index(s.field.conditions, p.text); i >= 0 {
					s.broaden(i)
				}
			}).Shrink(0).KeepFocus()
	})
}

// pillKey is what a key does in the pill's input before the input takes it.

// ribbonMenu is the menu of a condition's ribbon: every way of changing
// it named, where pressing it with Option and dragging it are the
// shortcuts.
func (s *State) ribbonMenu(text string) func(*ui.Menu) {
	return func(m *ui.Menu) {
		i := slices.Index(s.field.conditions, text)
		if i < 0 {
			return
		}
		if components.MenuItem(m.Item(locales.ChangeEllipsis()), components.IconPen).Chosen() {
			s.rewrite(i)
		}
		c, _ := condition(text)
		turn := map[bool]string{true: locales.Include(), false: locales.Exclude()}[c.Not]
		if m.Item(turn).Shortcut(ui.Alt, 0).Chosen() {
			s.turn(i)
		}
		if s.mode() == modeLibrary && !s.inTrash() && components.MenuItem(m.Item(locales.KeepAsBookmark()), components.IconBookmark).Chosen() {
			s.saveQuery(text)
		}
		if g, ok := s.scopeGroup(); ok && conditionKey(text) == "in" && !s.inTrash() {
			s.keepItem(m, g)
		}
		m.Separator()
		if components.MenuItem(m.Item(locales.Remove()), components.IconClose).Chosen() {
			s.broaden(i)
		}
	}
}
