package shell

import (
	"github.com/caelyreth/neverland/native/scrollfx"
	"slices"
	"strings"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The field shows what its query finds (results.go) in blocks
// (blocks.go) around the pill the query is written in (pill.go).
// Everything done on the field changes the query, and the field moves to
// what the changed query finds on one value (stage.Shift), from where the
// action says: a group's head pressed opens its blocks out of it, a
// condition taken away folds them back into the block it came from.

type fieldState struct {
	// conditions are the query's conditions, each as the language writes
	// it, shown as ribbons on the pill; words is what is written after
	// them, and edit what the pill's input is to be set to.
	conditions []string
	words      string
	edit       *string
	// focus asks the pill's input for the keyboard.
	focus bool
	found found
	// shapes are found's blocks' shapes, for the measures shapesKey names.
	shapes    []shape
	shapesKey string
	// shown are the blocks drawn, those going out included, by id, as
	// shaped.
	shown      map[string]block
	shownShape map[string]shape
	// rest is the layout's frames as last laid out, at rest, drawn what
	// was last drawn, and from what the change under way started from.
	rest, drawn, from []stage.Frame
	// next is how the next change moves, as the action making it says,
	// and now the change under way.
	next, now change
	// scroll is how far the field is scrolled down from its horizon at
	// the window's middle (up, below 0), as far as lo and hi; home brings
	// it back there, homing once it has begun.
	scroll, lo, hi float32
	home, homing   bool
	// over is how far past its ends the field is pulled, as the panels and
	// tables pull, and viewH the field's height, for its reach.
	over  scrollfx.Elastic
	viewH float32
	// hidden is whether the desk fills the window over the field, which
	// takes no focus meanwhile.
	hidden bool
	// hold keeps a change still for its first frame, which lays out and
	// draws what it brings, so the motion starts on the frame after.
	hold bool
	// picked is the sheets picked together, anchor where a Shift-click's
	// run starts (picks.go).
	picked []string
	anchor string
	// order is the rows the field shows, in order, and chosen the one the
	// keys are on.
	order  []string
	chosen string
	// renaming is the group whose name is being written on its head.
	renaming, renameName string
	renameFocus          bool
}

// change is how the field moves to a new layout: which way, the place its
// new results come from, and the result its old ones go back into.
type change struct {
	depth stage.Depth
	enter stage.Rect
	exit  string
}

// FieldQuery is the field's query: its conditions, then its words.
func (s *State) FieldQuery() string {
	return strings.TrimSpace(strings.Join(append(slices.Clone(s.field.conditions), s.field.words), " "))
}

// SetFieldQuery sets the field's query: its conditions to ribbons, the
// rest to the words written.
func (s *State) SetFieldQuery(text string) {
	s.field.conditions, s.field.words = nil, ""
	s.takeConditions(text)
	s.setWords(s.field.words)
}

// takeConditions takes the conditions written in text as ribbons, and
// leaves the rest as the words.
func (s *State) takeConditions(text string) {
	conds := query.Conditions(query.Parse(text))
	for i := len(conds) - 1; i >= 0; i-- {
		text = query.Cut(text, conds[i].Span)
	}
	for _, c := range conds {
		s.field.conditions = append(s.field.conditions, c.String())
	}
	s.field.words = strings.TrimLeft(text, " ")
}

func (s *State) setWords(words string) {
	s.field.words = words
	s.field.edit = &words
}

// narrow puts cond in the query, in place of one of the same key, going
// down into what was chosen at from.
func (s *State) narrow(cond string, from stage.Rect) {
	key := conditionKey(cond)
	s.field.conditions = slices.DeleteFunc(s.field.conditions, func(c string) bool { return conditionKey(c) == key })
	s.field.conditions = append(s.field.conditions, cond)
	s.field.next = change{depth: stage.Down, enter: from}
	s.field.chosen = ""
	s.field.focus = true
}

// broaden takes the query's condition at i away, coming back up into the
// block of the group it was in, if it was one.
func (s *State) broaden(i int) {
	if i < 0 || i >= len(s.field.conditions) {
		return
	}
	next := change{depth: stage.Up}
	if c, ok := condition(s.field.conditions[i]); ok && c.Key.Name == "in" && s.Library != nil {
		if g, ok := s.groupNamed(c.Value); ok {
			next.exit = "group:" + g.ID
		}
	}
	s.field.conditions = slices.Delete(s.field.conditions, i, i+1)
	s.field.next = next
	s.field.chosen = ""
}

// into narrows the query to a group, or to what a smart folder finds.
func (s *State) into(g GroupItem, from stage.Rect) {
	if g.Smart() {
		s.SetFieldQuery(g.Query)
		s.field.next = change{depth: stage.Down, enter: from}
		return
	}
	s.narrow((&query.Condition{Key: mustKey("in"), Value: g.Name}).String(), from)
}

// Search goes to the field, its pill taking the keyboard.
func (s *State) Search() { s.ShowField() }

// condition reads a condition as the language writes it.
func condition(text string) (query.Condition, bool) {
	conds := query.Conditions(query.Parse(text))
	if len(conds) != 1 {
		return query.Condition{}, false
	}
	return conds[0], true
}

func conditionKey(text string) string {
	if c, ok := condition(text); ok {
		return c.Key.Name
	}
	return text
}

func mustKey(name string) *query.Key {
	k, _ := query.KeyNamed(name)
	return k
}

// fieldView draws the field in the window's room left of the desk lane, w
// by h, as the scene stands it: what the query finds in slots on either
// side of its horizon (stage.Horizon), the pill in the horizon, and the
// whole scrolling up and down as one, the pill keeping near the window's
// top or foot once its horizon has gone past them; pressed there, the
// pill brings the field back to its horizon. Hidden, as the desk fills the window, it is still built, so it
// keeps where everything stands and comes back as it was.
func (s *State) fieldView(c *ui.Context, semantic theme.Theme, look stage.Look, w, h float32, hidden, animate bool) {
	m := s.blockMetrics(w, h)
	k := m.k
	view := ui.Box(c).Key("field").Absolute().Top(0).Left(0).Size(w, h).Clip().Hidden(hidden).
		Role(ui.RoleGroup).Label("Field")
	if look.Offset.X != 0 {
		view.Translate(look.Offset.X, 0)
	}
	view.ContextMenu(s.fieldMenu)
	view.HandleInput(s.fieldInput)
	s.field.hidden = hidden
	if !s.OnDesk() && !s.Typing() && c.Shortcut(0, ui.KeyEscape) {
		s.escapeField()
	}
	f := s.results()
	shapes := s.shapes(c, f, m)
	heights := make([]float32, len(shapes))
	for i, sh := range shapes {
		heights[i] = sh.size.H
	}
	// As many columns as what it shows takes, so the field stands even
	// about its pill.
	z := m.horizon
	z.Cols = max(1, min(z.Cols, (len(heights)+1)/2))
	rects := z.Place(heights)
	frames := make([]stage.Frame, len(rects))
	for i, r := range rects {
		frames[i] = stage.Frame{ID: f.blocks[i].id, Size: shapes[i].size, Rect: r, Shown: 1}
	}
	below, above := stage.Reach(rects)
	s.field.lo, s.field.hi = -max(0, above+3*m.pad-h/2), max(0, below+3*m.pad-h/2)
	s.field.order = s.field.order[:0]
	for _, b := range f.blocks {
		for _, cmd := range b.commands {
			s.field.order = append(s.field.order, "command:"+cmd.ID)
		}
		for _, g := range b.groups {
			s.field.order = append(s.field.order, "group:"+g.ID)
		}
		for _, e := range b.entries {
			s.field.order = append(s.field.order, "sheet:"+e.sheet.ID)
		}
	}
	s.keepPicked()
	// What the query shows changed, the field comes back to its horizon on
	// the change's own spring.
	if s.field.rest != nil && !slices.Equal(frames, s.field.rest) && s.field.scroll != 0 {
		s.field.home = true
	}
	// Coming home, the field goes from wherever it was, past its new reach
	// too; otherwise it is kept within it.
	if !s.field.home {
		s.field.scroll = max(s.field.lo, min(s.field.scroll, s.field.hi))
	}
	drawn := s.shift(view, frames, f, shapes, animate)
	s.comeHome(view, animate)
	if s.field.hold {
		s.field.hold = false
		c.AnimationFrame()
	}
	live := make(map[string]bool, len(frames))
	for _, fr := range frames {
		live[fr.ID] = true
	}
	pill := stage.Size{W: min(440*k, w-2*(m.pad+horizonSide*k)), H: 40 * k}
	// The pill stands at the horizon, moving with the field as it
	// scrolls, but no further than near the window's top or foot: there it
	// stays, and what the field holds passes under it.
	s.field.viewH = h
	_, pulling := s.field.over.Step(c.Now(), animate)
	if pulling {
		c.AnimationFrame()
	}
	horizon := h/2 - (s.field.scroll - s.field.over.Offset())
	top, foot := 48*k, 24*k
	py := max(top, min(horizon-pill.H/2, h-foot-pill.H))
	view.Children(func() {
		at := stage.Point{X: w / 2, Y: horizon}
		for _, fr := range drawn {
			b, ok := s.field.shown[fr.ID]
			if !ok {
				continue
			}
			s.blockView(c, semantic, fr, b, s.field.shownShape[fr.ID], at, live[fr.ID], m, animate)
		}
		s.horizonView(c, semantic, stage.Rect{X: w/2 - pill.W/2, Y: py, W: pill.W, H: pill.H}, w, k, animate)
	})
}

// fieldInput scrolls the field up and down, as far as it holds, and
// pulls it past its ends as the panels and tables are pulled.
func (s *State) fieldInput(ev ui.InputEvent) bool {
	if ev.Kind != ui.InputScroll {
		return false
	}
	s.field.home, s.field.homing = false, false
	s.field.scroll = components.OverscrollPanel.Scroll(&s.field.over, s.field.scroll, ev.DY, s.field.lo, s.field.hi,
		s.field.viewH, s.Settings.UIFontSize, ev)
	return true
}

// comeHome brings the field back to its horizon at the window's middle on
// one spring, from wherever it was scrolled to.
func (s *State) comeHome(el ui.Element, animate bool) {
	if !s.field.home {
		return
	}
	if !animate {
		s.field.scroll, s.field.home = 0, false
		return
	}
	if !s.field.homing {
		el.Jump("scroll", s.field.scroll)
		s.field.homing = true
		if s.field.hold {
			return
		}
	}
	s.field.scroll = el.AnimateTo("scroll", 0, components.Spring(components.MotionScene))
	if s.field.scroll == 0 {
		s.field.home, s.field.homing = false, false
	}
}

// shift moves the field to frames, the layout at rest of what f found: at
// once, or from what it drew on one spring, the change taking over from
// wherever one under way has got to.
func (s *State) shift(field ui.Element, frames []stage.Frame, f found, shapes []shape, animate bool) []stage.Frame {
	if s.field.shown == nil {
		s.field.shown, s.field.shownShape = map[string]block{}, map[string]shape{}
	}
	if !slices.Equal(frames, s.field.rest) {
		// The field's first layout stands at once: there is nothing for it
		// to come from.
		first := s.field.rest == nil
		s.field.from = s.field.drawn
		s.field.now = s.field.next
		s.field.next = change{}
		s.field.rest = frames
		live := make(map[string]bool, len(frames)+len(s.field.from))
		for _, fr := range frames {
			live[fr.ID] = true
		}
		for _, fr := range s.field.from {
			live[fr.ID] = true
		}
		for id := range s.field.shown {
			if !live[id] {
				delete(s.field.shown, id)
				delete(s.field.shownShape, id)
			}
		}
		field.Jump("shift", 0)
		if first {
			field.Jump("shift", 1)
		} else {
			s.field.hold = true
		}
	}
	for i, b := range f.blocks {
		s.field.shown[b.id] = b
		s.field.shownShape[b.id] = shapes[i]
	}
	t := float32(1)
	switch {
	case animate && s.field.hold:
		// Its first frame draws where everything was; the spring starts on
		// the next, so the time this one takes is not a leap.
		t = 0
	case animate:
		t = field.AnimateTo("shift", 1, components.Spring(components.MotionScene))
	}
	if t >= 1 {
		s.field.from = nil
		s.field.drawn = frames
		return frames
	}
	var exit stage.Rect
	for _, fr := range frames {
		if fr.ID == s.field.now.exit {
			exit = fr.Rect
		}
	}
	s.field.drawn = stage.Shift(s.field.from, frames, s.field.now.enter, exit, s.field.now.depth, t)
	return s.field.drawn
}

// escapeField takes the words away, then the last condition, and with
// none left goes back to the desk.
func (s *State) escapeField() {
	switch {
	case len(s.field.picked) > 0:
		s.field.picked = nil
	case s.field.words != "":
		s.setWords("")
	case len(s.field.conditions) > 0:
		s.broaden(len(s.field.conditions) - 1)
	default:
		s.ShowDesk()
	}
}

// choose moves the keys by by along the rows the field shows.
func (s *State) choose(by int) {
	ids := s.field.order
	if len(ids) == 0 {
		return
	}
	at := slices.Index(ids, s.field.chosen)
	if at < 0 {
		s.field.chosen = ids[0]
		return
	}
	s.field.chosen = ids[max(0, min(at+by, len(ids)-1))]
}

// activate does what the row id does chosen: a group narrows the query
// to it, and a sheet opens on the desk, in place of the one in front or
// alongside it.
func (s *State) activate(id string, alongside bool) {
	if s.Library == nil {
		return
	}
	kind, key, _ := strings.Cut(id, ":")
	switch kind {
	case "command":
		// The command runs as the field was before it was asked for.
		s.setWords("")
		if s.RunCommand != nil {
			s.RunCommand(key)
		}
	case "group":
		if g, ok := s.Library.Group(key); ok {
			s.into(g, stage.Rect{})
		}
	case "sheet":
		for _, b := range s.field.found.blocks {
			if b.trashed {
				continue
			}
			for _, e := range b.entries {
				if e.sheet.ID != key {
					continue
				}
				if s.field.words != "" {
					s.Library.OpenFound(key, s.FieldQuery(), alongside)
				} else {
					s.Library.OpenSheet(key, alongside)
				}
				s.ShowDesk()
				return
			}
		}
	}
}

// fieldMenu is what the field's ground offers.
func (s *State) fieldMenu(m *ui.Menu) {
	if s.Library == nil {
		return
	}
	if m.Item("New Sheet").Chosen() {
		s.Library.NewSheet(s.CurrentGroup(), Last)
		s.ShowDesk()
	}
	if m.Item("New Group").Chosen() {
		s.newGroup(s.scope())
	}
	m.Separator()
	trash := slices.IndexFunc(s.field.conditions, func(c string) bool { return conditionKey(c) == "trash" })
	if trash >= 0 {
		if m.Item("Leave Trash").Chosen() {
			s.broaden(trash)
		}
		if m.Item("Empty Trash…").Chosen() {
			s.arm(emptyTrashKey)
		}
		return
	}
	if m.Item("Show Trash").Chosen() {
		s.field.conditions = nil
		s.setWords("")
		s.narrow((&query.Condition{Key: mustKey("trash")}).String(), stage.Rect{})
	}
}

// scope is the group the query is in, "" for none.
func (s *State) scope() string {
	if s.Library == nil {
		return ""
	}
	scope, _, _ := s.browsing(query.Parse(s.FieldQuery()))
	return scope
}

// CurrentGroup is the group a new sheet goes in: the one the field is in,
// or the Inbox.
func (s *State) CurrentGroup() string {
	if g := s.scope(); g != "" {
		return g
	}
	if s.Library == nil {
		return ""
	}
	for _, g := range s.Library.Groups("") {
		if g.Inbox {
			return g.ID
		}
	}
	return ""
}

func (s *State) newGroup(parent string) {
	id := s.Library.NewGroup(parent, Last)
	s.startRename(id)
}

func (s *State) startRename(id string) {
	g, ok := s.Library.Group(id)
	if !ok || g.Inbox {
		return
	}
	s.field.renaming, s.field.renameName, s.field.renameFocus = id, g.Name, true
}

// ReconcileLibrary drops what the library no longer has: a group being
// renamed, and a condition naming a group gone.
func (s *State) ReconcileLibrary() {
	if s.Library == nil {
		return
	}
	if s.field.renaming != "" {
		if _, ok := s.Library.Group(s.field.renaming); !ok {
			s.field.renaming = ""
		}
	}
	s.field.conditions = slices.DeleteFunc(s.field.conditions, func(text string) bool {
		c, ok := condition(text)
		if !ok || c.Key.Name != "in" {
			return false
		}
		_, found := s.groupNamed(c.Value)
		return !found
	})
}
